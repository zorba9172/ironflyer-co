// /blog/budget-transparency — the pricing philosophy post.

import type { Metadata } from 'next';
import { BlogPost } from '../../../components/docs/BlogPost';

export const metadata: Metadata = {
  title: 'Why we publish our margin — Ironflyer',
  description: 'Subscription minus provider cost equals margin. We thought the simplest way to be trusted was to do the arithmetic in public.',
  openGraph: {
    title: 'Why we publish our margin',
    description: 'The transparent budget model behind Ironflyer pricing.',
    images: ['/opengraph-image'],
  },
};

export default function BudgetTransparencyPost() {
  return (
    <BlogPost
      title="Why we publish our margin."
      subtitle="Subscription − provider cost = margin. We thought the simplest way to be trusted was to do the arithmetic in public."
      tag="Pricing"
      date="2026-04-02"
      gradient="linear-gradient(135deg, #ffc400 0%, #ff6c3a 100%)"
    >
      <p>
        Every AI tool that ships today has a margin question hanging over it. The provider cost is real
        and metered — every prompt costs the company that built the wrapper a measurable number of
        dollars to the model vendor. The retail price is usually a flat subscription. Somewhere in
        between is the margin, and most companies treat the gap between the two as a competitive secret.
      </p>
      <p>
        We decided early to make ours public. The full live snapshot lives at <code>GET /budget/vault</code>;
        every authenticated user can pull their own slice at <code>GET /budget/users/me</code>. The
        decision to publish was practical, not idealistic — it solves three problems at once.
      </p>

      <h2>Problem 1: trust</h2>
      <p>
        A new AI app builder is asking you to put your idea, your code, and your billing data on a
        platform you have not used before. The single most useful signal we can give you is the closed
        books — here is what we charged, here is what we paid the model vendor, here is the difference.
        If the difference is reasonable, the platform is sustainable; if it is unreasonable, you can
        leave. We would rather lose customers who think our margin is too high than mislead the ones
        who stay.
      </p>

      <h2>Problem 2: the credit trap</h2>
      <p>
        Most AI tools meter on credits. Credits are a great way to mask cost: the user buys a bucket,
        the bucket drains in a way that does not map cleanly to anything observable, and the user
        cannot tell whether a long conversation cost five cents or five dollars. We do not want to be
        in that business. So every charge in our ledger is in USD, every entry has the model that
        served it and the token counts behind the price, and the per-user vault shows the running
        total in the same units we are charging.
      </p>

      <h2>Problem 3: the cap</h2>
      <p>
        We bake a hard <code>CostCapUSD</code> into every plan. The cap is set so
        <code> subscription − cap ≥ minimum margin</code>, and we will not let the loop cross it
        silently. When you get close, the dashboard shows a banner; when you cross it, the loop pauses
        with a single-click upgrade. This is the part that surprises people: <em>we do not auto-charge
        for overage</em>. We would rather pause work and have a conversation than discover that we
        billed someone for ten times their plan because they accidentally left a runaway loop on.
      </p>

      <h2>The arithmetic, made boring</h2>
      <p>
        Under the hood it is genuinely simple. Every paid model call goes through <code>BillingGuard</code>
        in the orchestrator. The flow is <code>Admit → CompleteStream → Charge</code>:
      </p>
      <ol>
        <li><code>Admit</code> rejects up front if the user’s plan cap is hit.</li>
        <li><code>CompleteStream</code> runs the actual provider call.</li>
        <li><code>Charge</code> writes the realised cost into the ledger when the call finishes.</li>
      </ol>
      <p>
        That ordering matters. Charging only after we know real token counts means our margin number is
        not a forecast — it is the closed books. And admitting before we call means we never burn
        provider dollars on a request we already know we will refund.
      </p>

      <h2>What the dashboard shows</h2>
      <p>
        The budget surface in the dashboard is intentionally plain. Your lifetime spend, your plan cap,
        a sparkline of daily spend over the last 30 days, the last 30 ledger entries with model + tokens
        + USD, and a one-line link to the global vault snapshot. We do not gamify it. We do not show a
        “credits remaining” bar that drains in a way that nudges you to top up. The number is the
        number.
      </p>

      <h2>What we get back</h2>
      <p>
        The transparency has a side effect we did not anticipate: customers tell us when our prompts
        are wasteful. The first week we shipped the vault, we got three emails in two days pointing
        out that our brainstorm agent was running a Power-tier model when an Economy-tier one would
        have sufficed. We fixed it. The margin improved. Everyone wins when the math is in public.
      </p>
    </BlogPost>
  );
}
