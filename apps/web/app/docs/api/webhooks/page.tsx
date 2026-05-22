// /docs/api/webhooks — public webhook endpoints.

import type { Metadata } from 'next';
import { DocPage } from '../../../../components/docs/DocPage';
import { CodeBlock } from '../../../../components/docs/CodeBlock';

export const metadata: Metadata = {
  title: 'Webhooks API',
  description: 'Public webhook endpoints — Stripe today, more on the way.',
  openGraph: { title: 'Webhooks API · Ironflyer', description: 'Stripe webhook + the contract for new ones.', images: ['/opengraph-image'] },
};

const toc = [
  { id: 'overview', label: 'Overview' },
  { id: 'stripe', label: 'POST /budget/webhook (Stripe)' },
  { id: 'contract', label: 'New webhook contract' },
];

export default function WebhooksAPIPage() {
  return (
    <DocPage
      eyebrow="API Reference"
      title="Webhooks"
      description="Server-to-server callbacks. Public endpoints; authentication is per-payload (signature header, not bearer token)."
      toc={toc}
    >
      <h2 id="overview">Overview</h2>
      <p>
        Webhooks are how upstream services notify the orchestrator that something happened — Stripe
        confirms a payment, GitHub confirms a push (coming soon), Fly confirms a deploy (coming soon).
        Because the caller is a server, not a logged-in human, the route lives outside the JWT
        middleware: the body is authenticated by a provider-specific signature header.
      </p>

      <h2 id="stripe">POST /budget/webhook</h2>
      <p>
        Stripe’s callback target. The handler verifies <code>Stripe-Signature</code> against the raw
        body with a 5-minute tolerance window, then dispatches on the event type:
      </p>
      <ul>
        <li><code>checkout.session.completed</code> — user upgraded; we assign the plan + record revenue.</li>
        <li><code>customer.subscription.deleted</code> — user downgraded; we drop their plan to free.</li>
        <li><code>invoice.payment_failed</code> — flagged in the ledger so dunning logic can hold the line.</li>
      </ul>
      <p>Replay-safe: events are idempotent by Stripe event id.</p>
      <CodeBlock language="bash">{`# Local stripe-cli forwarding while developing
stripe listen --forward-to http://localhost:8080/budget/webhook`}</CodeBlock>

      <h2 id="contract">New webhook contract</h2>
      <p>When we add another provider, the contract will be the same:</p>
      <ul>
        <li>Public route, no JWT middleware.</li>
        <li>Provider-specific signature header verified inside the handler.</li>
        <li>Tolerance window ≤ 5 minutes.</li>
        <li>Idempotency on the provider event id.</li>
        <li>2xx on success, 4xx on signature failure, 5xx only on real errors so the provider retries.</li>
      </ul>
    </DocPage>
  );
}
