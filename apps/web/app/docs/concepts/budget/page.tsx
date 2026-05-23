// /docs/concepts/budget — the transparency story. We expose the math.

import type { Metadata } from 'next';
import { DocPage } from '../../../../components/docs/DocPage';
import { CodeBlock } from '../../../../components/docs/CodeBlock';

export const metadata: Metadata = {
  title: 'Budget',
  description: 'Subscription − provider cost = margin. The math is visible to you and to us in real time.',
  openGraph: {
    title: 'Budget · Ironflyer',
    description: 'How we keep AI app pricing honest: a vault, a ledger, and a hard admit step.',
    images: ['/opengraph-image'],
  },
};

const toc = [
  { id: 'the-formula', label: 'The formula' },
  { id: 'vault-and-ledger', label: 'Vault and ledger' },
  { id: 'admit-then-charge', label: 'Admit then charge' },
  { id: 'the-cap', label: 'The cap' },
  { id: 'reading-your-budget', label: 'Reading your budget' },
];

export default function BudgetConceptPage() {
  return (
    <DocPage
      eyebrow="Concepts"
      title="Budget"
      description="A vault, a ledger, an admit step, and a hard cap — together they let the platform be honest about cost."
      toc={toc}
    >
      <h2 id="the-formula">The formula</h2>
      <p>
        Ironflyer’s pricing model is one line of arithmetic:
      </p>
      <blockquote>
        <strong>subscription revenue − provider cost = company margin</strong>
      </blockquote>
      <p>
        We picked that formulation because it forces honesty in both directions. We cannot quietly charge
        you for tokens you did not consume; you cannot accidentally burn through the budget of a paid
        plan without being told. The Vault snapshot, exposed at <code>GET /budget/vault</code>, is the
        public board for that arithmetic.
      </p>

      <h2 id="vault-and-ledger">Vault and ledger</h2>
      <p>
        Internally we keep two pieces of state:
      </p>
      <ul>
        <li><strong>Vault</strong> — the running totals: revenue, provider cost, margin. Snapshots are crawlable.</li>
        <li><strong>Ledger</strong> — every charge, tagged with user, project, model, prompt tokens, completion tokens, USD cost.</li>
      </ul>
      <p>
        The ledger is per-user; the vault is global. Together they answer two questions every developer
        wants answered: <em>how much did this prompt cost me?</em> and <em>can the company afford to keep doing this?</em>
      </p>

      <h2 id="admit-then-charge">Admit then charge</h2>
      <p>
        Every provider call goes through <code>BillingGuard</code> in the orchestrator. The flow is
        <code>Admit → CompleteStream → Charge</code>:
      </p>
      <ol>
        <li><strong>Admit</strong> rejects up front if the user’s plan cap is hit.</li>
        <li><strong>CompleteStream</strong> runs the actual provider call, streaming tokens to the caller.</li>
        <li><strong>Charge</strong> writes the realised cost into the ledger when the call finishes.</li>
      </ol>
      <p>
        That ordering matters. Charging only after we know real token counts means our margin number
        is not a forecast — it is the closed books. And admitting before we call means we never burn
        provider dollars on a request we already know we will refund.
      </p>

      <h2 id="the-cap">The cap</h2>
      <p>
        Each plan has a hard <code>CostCapUSD</code> baked into <code>budget.PlanTier</code>. The cap is
        set so that <em>subscription − cap ≥ minimum margin</em>. We will never let provider cost cross
        the cap silently — when you get close, the dashboard shows a banner; when you cross it, the
        loop pauses with a single-click upgrade.
      </p>

      <h2 id="reading-your-budget">Reading your budget</h2>
      <p>
        The protected endpoint <code>GET /budget/users/me</code> returns your spend + plan tier, with
        the most recent ledger entries. The unauthenticated <code>GET /budget/vault</code> returns the
        global snapshot. Both are stable, paginatable, and documented in the
        <a href="/docs/api/budget"> Budget API reference</a>.
      </p>
      <CodeBlock language="bash">{`curl https://api.ironflyer.dev/budget/users/me \\
  -H "Authorization: Bearer $TOKEN"

# {
#   "userId": "u_…",
#   "email": "you@example.com",
#   "tier": "pro",
#   "spent": 4.21,
#   "entries": [ … ]
# }`}</CodeBlock>
    </DocPage>
  );
}
