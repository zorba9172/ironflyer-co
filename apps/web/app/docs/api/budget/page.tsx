// /docs/api/budget — every budget surface.

import type { Metadata } from 'next';
import { DocPage } from '../../../../components/docs/DocPage';
import { CodeBlock } from '../../../../components/docs/CodeBlock';

export const metadata: Metadata = {
  title: 'Budget API',
  description: 'Plans, rates, the vault snapshot, the per-user budget, and Stripe checkout.',
  openGraph: { title: 'Budget API · Ironflyer', description: 'Public catalogue endpoints plus the protected per-user surface.', images: ['/opengraph-image'] },
};

const toc = [
  { id: 'plans', label: 'GET /budget/plans' },
  { id: 'rates', label: 'GET /budget/rates' },
  { id: 'vault', label: 'GET /budget/vault' },
  { id: 'me', label: 'GET /budget/users/me' },
  { id: 'vault-me', label: 'GET /budget/vault/me' },
  { id: 'checkout', label: 'POST /budget/checkout' },
];

export default function BudgetAPIPage() {
  return (
    <DocPage
      eyebrow="API Reference"
      title="Budget"
      description="The public catalogue (plans, rates, the vault) plus the protected per-user surface and Stripe checkout."
      toc={toc}
    >
      <h2 id="plans">GET /budget/plans</h2>
      <p>Public. Returns the plan catalogue — free, pro, team, enterprise — with the cost cap that defines our margin floor.</p>
      <CodeBlock language="bash">{`curl https://api.ironflyer.dev/budget/plans`}</CodeBlock>

      <h2 id="rates">GET /budget/rates</h2>
      <p>Public. Per-model token rates the BillingGuard uses to compute USD cost. Refreshed when we re-negotiate provider pricing.</p>

      <h2 id="vault">GET /budget/vault</h2>
      <p>
        Public. The global vault snapshot: lifetime revenue, lifetime provider cost, margin, top
        models by cost. We expose this so anyone — investors, customers, regulators — can audit our
        margin claim.
      </p>

      <h2 id="me">GET /budget/users/me</h2>
      <p>Protected. Returns the caller’s plan tier, lifetime spend, and ledger entries.</p>
      <CodeBlock language="json">{`{
  "userId":  "u_…",
  "email":   "you@example.com",
  "tier":    "pro",
  "spent":   4.21,
  "entries": [ … ]
}`}</CodeBlock>

      <h2 id="vault-me">GET /budget/vault/me</h2>
      <p>
        Protected. The dashboard-flavoured snapshot — caller’s spend, last-30 ledger entries, plan
        cap, and the global vault snapshot for context (so the UI can render <em>you used X of Y this month</em>).
      </p>

      <h2 id="checkout">POST /budget/checkout</h2>
      <p>
        Protected. Starts a Stripe Checkout session. The orchestrator forwards the user id + email
        to Stripe so the webhook can map back. Body: <code>&#123; "tier": "pro" | "team" &#125;</code>.
      </p>
    </DocPage>
  );
}
