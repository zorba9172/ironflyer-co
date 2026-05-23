// Package finisher — Stripe scaffold step. Same pattern as the auth
// scaffolder: when the spec wants payments, write a deterministic
// Stripe-on-Next.js skeleton (checkout session, webhook, customer
// portal) so the Coder builds product features on top of a contract we
// own. Without this, every generated app reinvents the secret-key
// handling and inevitably gets it wrong (client-side leaks are the
// number-one Stripe footgun).
//
// Trigger: ProductSpec.Stack contains "stripe" anywhere (frontend,
// backend, or auth field), OR a user story mentions "payment", "billing",
// "subscription", "checkout". Operators can also force-enable by
// supplying their own scaffolder that returns a non-empty bundle
// unconditionally.

package finisher

import (
	"context"
	"strings"

	"ironflyer/apps/orchestrator/internal/domain"
)

type StripeScaffolder interface {
	Scaffold(ctx context.Context, p *domain.Project) (StripeScaffold, error)
}

type StripeScaffold struct {
	// Provider is always "stripe" today but kept here to mirror the
	// AuthScaffold shape (room for "lemonsqueezy" / "paddle" later).
	Provider string
	Files    map[string]string
	Contract string
}

// DefaultStripeScaffolder ships the canonical Next.js + Stripe checkout
// recipe when the spec calls for payments.
type DefaultStripeScaffolder struct{}

func (DefaultStripeScaffolder) Scaffold(_ context.Context, p *domain.Project) (StripeScaffold, error) {
	if p == nil || !stripeWanted(p) {
		return StripeScaffold{}, nil
	}
	return stripeNextScaffold(), nil
}

// stripeWanted heuristically inspects the spec for payment intent. We
// keep the predicate broad so a story like "subscribe to premium" still
// triggers it even if the architect didn't name "stripe" explicitly.
func stripeWanted(p *domain.Project) bool {
	stack := strings.ToLower(p.Spec.Stack.Frontend + " " + p.Spec.Stack.Backend + " " + p.Spec.Stack.Storage + " " + p.Spec.Stack.Auth)
	if strings.Contains(stack, "stripe") {
		return true
	}
	for _, s := range p.Spec.UserStories {
		body := strings.ToLower(s.IWant + " " + s.SoThat + " " + strings.Join(s.Acceptance, " "))
		if strings.Contains(body, "payment") ||
			strings.Contains(body, "billing") ||
			strings.Contains(body, "subscription") ||
			strings.Contains(body, "checkout") ||
			strings.Contains(body, "pricing") ||
			strings.Contains(body, "paywall") {
			return true
		}
	}
	return false
}

func stripeNextScaffold() StripeScaffold {
	files := map[string]string{
		"lib/stripe/server.ts": `// Server-side Stripe client. Reads STRIPE_SECRET_KEY from the runtime
// env — NEVER expose this key in the browser bundle. The Stripe SDK is
// imported lazily so the marketing pages, which don't need it, keep a
// small bundle.
import Stripe from 'stripe';

let cached: Stripe | null = null;

export function getStripe(): Stripe {
  if (cached) return cached;
  const key = process.env.STRIPE_SECRET_KEY;
  if (!key) throw new Error('STRIPE_SECRET_KEY is not set');
  cached = new Stripe(key, { apiVersion: '2024-06-20' });
  return cached;
}
`,
		"app/api/checkout/route.ts": `// POST /api/checkout — turns a { priceId, mode? } body into a Stripe
// Checkout Session and returns the redirect URL. The client navigates
// to it; Stripe handles card collection + 3DS; the success_url comes
// back to /app/billing/return where we read the session id and confirm.
import { NextRequest, NextResponse } from 'next/server';
import { getStripe } from '../../../lib/stripe/server';
import { getCurrentUser } from '../../../lib/supabase/server';

export async function POST(req: NextRequest) {
  const user = await getCurrentUser();
  if (!user) {
    return NextResponse.json({ error: 'unauthenticated' }, { status: 401 });
  }
  const body = await req.json().catch(() => ({} as Record<string, unknown>));
  const priceId = String((body as { priceId?: unknown }).priceId ?? '');
  if (!priceId) {
    return NextResponse.json({ error: 'priceId required' }, { status: 400 });
  }
  const mode = (body as { mode?: 'payment' | 'subscription' }).mode ?? 'subscription';
  const stripe = getStripe();
  const session = await stripe.checkout.sessions.create({
    mode,
    customer_email: user.email ?? undefined,
    line_items: [{ price: priceId, quantity: 1 }],
    success_url: ` + "`${process.env.NEXT_PUBLIC_SITE_URL}/app/billing/return?session_id={CHECKOUT_SESSION_ID}`" + `,
    cancel_url:  ` + "`${process.env.NEXT_PUBLIC_SITE_URL}/app/billing`" + `,
    metadata: { userId: user.id },
  });
  return NextResponse.json({ url: session.url });
}
`,
		"app/api/stripe-webhook/route.ts": `// POST /api/stripe-webhook — verifies the Stripe signature header and
// dispatches handled event types. ALWAYS reject unverified requests:
// without the signature check, anyone can forge a "subscription paid"
// event by posting JSON. Use the STRIPE_WEBHOOK_SECRET emitted by
//   stripe listen --forward-to localhost:3000/api/stripe-webhook
// during dev, and the production secret from the Stripe dashboard.
import { NextRequest, NextResponse } from 'next/server';
import { getStripe } from '../../../lib/stripe/server';
import type Stripe from 'stripe';

export async function POST(req: NextRequest) {
  const sig = req.headers.get('stripe-signature');
  const secret = process.env.STRIPE_WEBHOOK_SECRET;
  if (!sig || !secret) {
    return NextResponse.json({ error: 'missing signature or secret' }, { status: 400 });
  }
  const raw = await req.text();
  let event: Stripe.Event;
  try {
    event = getStripe().webhooks.constructEvent(raw, sig, secret);
  } catch (e) {
    return NextResponse.json({ error: ` + "`bad signature: ${(e as Error).message}`" + ` }, { status: 400 });
  }

  switch (event.type) {
    case 'checkout.session.completed':
      // TODO: mark the user's subscription as active in your DB,
      // keyed by event.data.object.metadata.userId.
      break;
    case 'customer.subscription.updated':
    case 'customer.subscription.deleted':
      // TODO: sync the subscription status row.
      break;
    case 'invoice.payment_failed':
      // TODO: flag account, send dunning email.
      break;
  }
  return NextResponse.json({ received: true });
}

// Stripe sends raw bytes; disable Next's body parsing.
export const dynamic = 'force-dynamic';
`,
		"app/app/billing/return/page.tsx": `// Server component that runs after the user comes back from Stripe
// Checkout. We verify the session, then either show a "thank you" card
// or a friendly retry. The webhook is the source of truth for granting
// the entitlement — this page is just UX confirmation.
import { redirect } from 'next/navigation';
import { getStripe } from '../../../../lib/stripe/server';
import { getCurrentUser } from '../../../../lib/supabase/server';

export default async function BillingReturn({
  searchParams,
}: { searchParams: Promise<{ session_id?: string }> }) {
  const user = await getCurrentUser();
  if (!user) redirect('/login?next=/app/billing/return');

  const { session_id } = await searchParams;
  if (!session_id) redirect('/app/billing');

  const session = await getStripe().checkout.sessions.retrieve(session_id);
  const paid = session.payment_status === 'paid' || session.payment_status === 'no_payment_required';

  return (
    <main style={{ maxWidth: 520, margin: '120px auto', fontFamily: 'system-ui' }}>
      <h1>{paid ? 'Thank you — payment received' : 'Payment pending'}</h1>
      <p>{paid
          ? "Your subscription is being provisioned. It may take a few seconds before access updates."
          : "Stripe is still processing this payment. We'll email you the moment it clears."}
      </p>
    </main>
  );
}
`,
	}

	contract := `Stripe scaffold: Stripe Checkout + webhook on Next.js (app router).

Already provisioned by Ironflyer:
- /lib/stripe/server.ts            → lazy server-side Stripe client
- /app/api/checkout/route.ts       → POST { priceId, mode? } → Checkout URL
- /app/api/stripe-webhook/route.ts → signature-verified webhook dispatcher
- /app/app/billing/return/page.tsx → post-checkout confirmation page

Required environment variables (already declared on the runtime):
- STRIPE_SECRET_KEY        (server-only, never bundle for client)
- STRIPE_WEBHOOK_SECRET    (from "stripe listen" in dev, dashboard in prod)
- NEXT_PUBLIC_SITE_URL     (used for success_url / cancel_url)

Rules for the Coder:
1. Do NOT replace files under /lib/stripe, /app/api/checkout, /app/api/stripe-webhook — they are the contract.
2. When you add new Stripe events to react to, add cases to the switch in stripe-webhook/route.ts; never write a second webhook handler.
3. NEVER read STRIPE_SECRET_KEY in a client component or a 'use client' file. The check belongs server-side.
4. Subscription entitlement state lives in YOUR database — the webhook updates it. Do not trust client-supplied "paid" flags.
`
	return StripeScaffold{Provider: "stripe", Files: files, Contract: contract}
}

// ensureStripe mirrors ensureAuth: idempotent file upserts + a .ironflyer
// markdown drop so the Coder sees the contract in project context.
func (e *Engine) ensureStripe(ctx context.Context, projectID string) {
	if e.stripeScaffolder == nil {
		return
	}
	proj, err := e.projects.Get(projectID)
	if err != nil {
		return
	}
	scaffold, err := e.stripeScaffolder.Scaffold(ctx, &proj)
	if err != nil || (len(scaffold.Files) == 0 && scaffold.Contract == "") {
		return
	}
	_, _ = e.projects.Update(projectID, func(p *domain.Project) {
		for path, body := range scaffold.Files {
			if existing := findFile(p, path); existing != nil && existing.Content != body {
				continue
			}
			writeProjectFile(p, path, body)
		}
		if scaffold.Contract != "" {
			writeProjectFile(p, ".ironflyer/stripe.md", scaffold.Contract)
		}
	})
	if scaffold.Provider != "" {
		e.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepRun, Status: StatusDone,
			Message: "stripe_scaffolded provider=" + scaffold.Provider,
		})
	}
}
