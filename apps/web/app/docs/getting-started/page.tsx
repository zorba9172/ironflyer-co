// /docs/getting-started — walks a new dev through the full loop: sign up,
// first prompt, finisher loop, live preview, deploy. Written long enough
// to actually be useful (~600 words) but scannable.

import type { Metadata } from 'next';
import { DocPage } from '../../../components/docs/DocPage';
import { CodeBlock } from '../../../components/docs/CodeBlock';
import { Tabs } from '../../../components/docs/Tabs';

export const metadata: Metadata = {
  title: 'Getting Started',
  description: 'Sign up, run the finisher loop, and ship a live preview in under five minutes.',
  openGraph: {
    title: 'Getting Started · Ironflyer',
    description: 'Your first project, end-to-end.',
    images: ['/opengraph-image'],
  },
};

const toc = [
  { id: '1-create-account', label: '1. Create an account' },
  { id: '2-first-prompt', label: '2. Your first prompt' },
  { id: '3-finisher-loop', label: '3. The finisher loop' },
  { id: '4-live-preview', label: '4. Live preview' },
  { id: '5-deploy', label: '5. Deploy' },
  { id: 'next', label: 'Where to next' },
];

export default function GettingStartedPage() {
  return (
    <DocPage
      eyebrow="Getting Started"
      title="Ship your first project in five minutes."
      description="From an empty screen to a live URL — the finisher does the gating, you do the deciding."
      toc={toc}
    >
      <p>
        Ironflyer is an <strong>AI Product Finisher</strong>. You write a prompt; the platform plans,
        codes, lints, tests, and security-scans the result; then it deploys behind a real URL. Every
        step is a gate, and the gates <em>block</em> until they pass. By the end of this page you will
        have signed up, kicked off your first run, watched the gates light up green, opened a live
        preview, and pushed to Fly.io.
      </p>

      <h2 id="1-create-account">1. Create an account</h2>
      <p>
        Sign up at <a href="https://app.ironflyer.dev/signup">app.ironflyer.dev/signup</a> with an email
        and password, or click <em>Continue with GitHub</em>. The free tier gives you four projects, ~50
        finisher runs per month, and a budget vault that shows exactly what each token costs. There are
        no credit traps — when you hit the cap the loop pauses; we never auto-charge for overage.
      </p>
      <p>If you would rather hit the API directly while you read along, sign up by HTTP:</p>
      <CodeBlock language="bash">{`curl -X POST https://api.ironflyer.dev/auth/signup \\
  -H "Content-Type: application/json" \\
  -d '{"email":"you@example.com","name":"You","password":"hunter22-very-long"}'`}</CodeBlock>
      <p>
        The response contains <code>user</code> and <code>token</code>. Keep the JWT — every protected
        endpoint expects it in an <code>Authorization: Bearer …</code> header.
      </p>

      <h2 id="2-first-prompt">2. Your first prompt</h2>
      <p>
        Once you are signed in, the dashboard greets you with a prompt box. Type the product you want
        in clear English. The clearest prompts mention
        a <em>who</em>, a <em>what</em>, and a <em>constraint</em>. Examples that work well:
      </p>
      <ul>
        <li><strong>SaaS dashboard</strong> — “A revenue dashboard for indie SaaS founders. Stripe + Postgres. CSV export.”</li>
        <li><strong>Internal tool</strong> — “An internal refund approvals tool with a form, review queue, and CSV export.”</li>
        <li><strong>API service</strong> — “A Go service that proxies OpenAI with per-user budgets and prom metrics.”</li>
      </ul>
      <p>
        You can also do it from the SDK. The same endpoint that powers the dashboard chat is on the
        public surface:
      </p>
      <Tabs
        tabs={[
          {
            label: 'TypeScript',
            content: (
              <CodeBlock language="typescript">{`import { ironflyer } from '@ironflyer/sdk';

const ifc = ironflyer({
  orchestratorUrl: 'https://api.ironflyer.dev',
  getToken: () => process.env.IRONFLYER_TOKEN!,
});

const project = await ifc.orchestrator.createProject({
  name: 'Revenue dashboard',
  idea: 'A revenue dashboard for indie SaaS founders. Stripe + Postgres. CSV export.',
});

await ifc.orchestrator.runFinisher(project.id);`}</CodeBlock>
            ),
          },
          {
            label: 'curl',
            content: (
              <CodeBlock language="bash">{`curl -X POST https://api.ironflyer.dev/projects \\
  -H "Authorization: Bearer $IRONFLYER_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"name":"Revenue dashboard","idea":"A revenue dashboard..."}'

curl -X POST https://api.ironflyer.dev/projects/revenue-dashboard/run \\
  -H "Authorization: Bearer $IRONFLYER_TOKEN"`}</CodeBlock>
            ),
          },
        ]}
      />

      <h2 id="3-finisher-loop">3. The finisher loop</h2>
      <p>
        As soon as you trigger a run, the nine gates start streaming. The order is fixed — Spec → UX →
        Architecture → Code → Lint → Tests → Security → Budget → Deploy — because each gate consumes the output
        of the previous one. A red gate halts the loop and surfaces the issues; a green gate emits a
        patch that the orchestrator applies through <code>patch.Engine</code>. The dashboard shows the
        feed live; the SDK exposes the same stream via SSE:
      </p>
      <CodeBlock language="typescript">{`const stream = ifc.orchestrator.streamEvents(project.id);
for await (const evt of stream) {
  if (evt.type === 'gate_passed') console.log('✅', evt.gate);
  if (evt.type === 'gate_failed') console.error('❌', evt.gate, evt.issues);
}`}</CodeBlock>
      <p>
        If a gate fails, do not panic — the loop has an auto-recovery agent that will propose a patch
        targeting only the failing gate. You can also re-run a single gate from the dashboard, the SDK,
        or the VSCode extension.
      </p>

      <h2 id="4-live-preview">4. Live preview</h2>
      <p>
        Behind every project is a real Linux sandbox running on the workspace runtime. While the gates
        run, the sandbox boots your stack (Node, Go, Python — auto-detected from the generated files)
        and exposes a port. The dashboard renders the preview in an iframe with mobile / tablet /
        desktop presets; the same iframe is available inside the VSCode extension via a webview.
      </p>
      <p>
        The preview URL is signed per session — no auth header is needed in the iframe, which is exactly
        what makes the preview work inside VSCode and Cursor. The signing key rotates every 24 hours.
      </p>

      <h2 id="5-deploy">5. Deploy</h2>
      <p>
        When every gate is green, the <strong>Deploy</strong> gate generates the artifacts your provider
        wants — <code>Dockerfile</code>, <code>fly.toml</code>, <code>railway.json</code>, a GitHub
        Actions workflow if you are exporting to a repo — and proposes them as a patch. You hit Deploy,
        the orchestrator pushes to Fly.io or Railway, and the live URL appears in the dashboard.
      </p>
      <CodeBlock language="bash">{`curl -X POST https://api.ironflyer.dev/projects/revenue-dashboard/deploy \\
  -H "Authorization: Bearer $IRONFLYER_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"provider":"fly","region":"fra"}'`}</CodeBlock>

      <h2 id="next">Where to next</h2>
      <p>
        Read the <a href="/docs/concepts/finisher-gates">Finisher Gates</a> page for the nine gates in
        detail, the <a href="/docs/concepts/budget">Budget</a> page for how revenue minus provider cost
        equals our margin (and why we show it to you), and the <a href="/docs/sdk">SDK reference</a> for
        every method available in TypeScript. If you live in your editor,
        <a href="/docs/vscode-extension"> install the VSCode extension</a> — the same loop, no browser.
      </p>
    </DocPage>
  );
}
