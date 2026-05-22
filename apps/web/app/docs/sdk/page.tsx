// /docs/sdk — introducing @ironflyer/sdk.

import type { Metadata } from 'next';
import { DocPage } from '../../../components/docs/DocPage';
import { CodeBlock } from '../../../components/docs/CodeBlock';
import { Tabs } from '../../../components/docs/Tabs';

export const metadata: Metadata = {
  title: '@ironflyer/sdk',
  description: 'Zero-dependency TypeScript client for the orchestrator and runtime.',
  openGraph: { title: '@ironflyer/sdk · Ironflyer', description: 'TypeScript SDK for both Ironflyer services.', images: ['/opengraph-image'] },
};

const toc = [
  { id: 'install', label: 'Install' },
  { id: 'hello-world', label: 'Hello world' },
  { id: 'clients', label: 'Clients' },
  { id: 'streaming', label: 'Streaming chat' },
  { id: 'errors', label: 'Errors' },
];

export default function SDKPage() {
  return (
    <DocPage
      eyebrow="SDK · ערכת פיתוח"
      title="@ironflyer/sdk"
      description="A small TypeScript client that wraps both Ironflyer services — orchestrator and runtime — with strict types and no runtime dependencies."
      toc={toc}
    >
      <h2 id="install">Install</h2>
      <p>
        The SDK is an internal monorepo package today. Depend on it from another workspace via your
        package manager.
      </p>
      <Tabs
        tabs={[
          { label: 'pnpm', content: <CodeBlock language="bash">{`pnpm add @ironflyer/sdk`}</CodeBlock> },
          { label: 'npm',  content: <CodeBlock language="bash">{`npm install @ironflyer/sdk`}</CodeBlock> },
          { label: 'yarn', content: <CodeBlock language="bash">{`yarn add @ironflyer/sdk`}</CodeBlock> },
        ]}
      />

      <h2 id="hello-world">Hello world</h2>
      <p>The same factory drives both clients; passing <code>runtimeUrl</code> is optional.</p>
      <CodeBlock language="typescript">{`import { ironflyer } from '@ironflyer/sdk';

const ifc = ironflyer({
  orchestratorUrl: 'https://api.ironflyer.dev',
  runtimeUrl:      'https://runtime.ironflyer.dev',
  getToken: () => process.env.IRONFLYER_TOKEN!,
});

const projects = await ifc.orchestrator.listProjects();
if (projects.length === 0) {
  await ifc.orchestrator.createProject({
    name: 'My first product',
    idea: 'A landing page builder with finisher gates baked in.',
  });
}`}</CodeBlock>

      <h2 id="clients">Clients</h2>
      <p>The SDK exposes two clients keyed by their underlying service.</p>
      <table>
        <thead>
          <tr><th>Client</th><th>What it wraps</th><th>Notable methods</th></tr>
        </thead>
        <tbody>
          <tr><td><code>orchestrator</code></td><td>Orchestrator HTTP API</td><td><code>signup</code>, <code>login</code>, <code>listProjects</code>, <code>createProject</code>, <code>runFinisher</code>, <code>streamEvents</code>, <code>streamChat</code>, <code>listPatches</code>, <code>applyPatch</code>, <code>myBudget</code></td></tr>
          <tr><td><code>runtime</code></td><td>Workspace runtime API</td><td><code>create</code>, <code>list</code>, <code>get</code>, <code>destroy</code>, <code>readFile</code>, <code>writeFile</code>, <code>exec</code>, <code>previewUrl</code>, <code>terminalUrl</code></td></tr>
        </tbody>
      </table>

      <h2 id="streaming">Streaming chat</h2>
      <p>
        <code>streamChat</code> parses the orchestrator’s SSE format into typed <code>ChatDelta</code>
        values. Cancel with an <code>AbortSignal</code>.
      </p>
      <CodeBlock language="typescript">{`const ctrl = new AbortController();

for await (const delta of ifc.orchestrator.streamChat(
  project.id,
  { prompt: 'Add a Hebrew RTL switch.', role: 'coder', effort: 'economy' },
  { signal: ctrl.signal },
)) {
  if (delta.type === 'text') process.stdout.write(delta.text);
  if (delta.type === 'done') console.log('\\n', delta.usage);
}`}</CodeBlock>

      <h2 id="errors">Errors</h2>
      <p>
        Every method that hits HTTP throws an <code>IronflyerError</code> on non-2xx responses. The
        error carries the HTTP status, the orchestrator error code, and the raw body, so callers can
        branch on real semantics instead of string-matching.
      </p>
    </DocPage>
  );
}
