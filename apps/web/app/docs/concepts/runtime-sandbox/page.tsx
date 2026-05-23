// /docs/concepts/runtime-sandbox — how the workspace runtime works.

import type { Metadata } from 'next';
import { DocPage } from '../../../../components/docs/DocPage';
import { CodeBlock } from '../../../../components/docs/CodeBlock';

export const metadata: Metadata = {
  title: 'Runtime Sandbox',
  description: 'Each user gets a real Linux sandbox where their generated app actually runs.',
  openGraph: {
    title: 'Runtime Sandbox · Ironflyer',
    description: 'The per-user Linux sandbox that backs the live preview, PTY, and file API.',
    images: ['/opengraph-image'],
  },
};

const toc = [
  { id: 'what-it-is', label: 'What it is' },
  { id: 'drivers', label: 'Drivers' },
  { id: 'file-api', label: 'File API' },
  { id: 'pty', label: 'PTY over WebSocket' },
  { id: 'preview-proxy', label: 'Preview proxy' },
  { id: 'isolation', label: 'Isolation' },
];

export default function RuntimeSandboxPage() {
  return (
    <DocPage
      eyebrow="Concepts"
      title="Runtime Sandbox"
      description="The thing that turns generated code into a live URL. One Linux box per user, owner-checked end to end."
      toc={toc}
    >
      <h2 id="what-it-is">What it is</h2>
      <p>
        The workspace runtime is a separate Go service (<code>apps/runtime</code>) that manages
        per-user Linux sandboxes. Each sandbox is the source of truth for a project — its file system
        is what the live preview serves, its <code>$PATH</code> is what the test gate runs against,
        its open ports are what the iframe proxies.
      </p>

      <h2 id="drivers">Drivers</h2>
      <p>
        The runtime is driver-pluggable. Two ship today:
      </p>
      <ul>
        <li><strong>Mock</strong> — an in-process fake we use in dev and tests. Files live in memory; exec is virtualised.</li>
        <li><strong>Docker</strong> — a real container per workspace, on a host pool managed by the runtime.</li>
      </ul>
      <p>
        The orchestrator never talks to the driver directly; it talks to the runtime’s HTTP API. That
        means we can swap Docker for Firecracker, Kata, or a remote VM provider without touching the
        rest of the platform.
      </p>

      <h2 id="file-api">File API</h2>
      <p>
        Every file in a sandbox is reachable through a small REST API. Listing, reading, writing,
        deleting — all auth-gated by the workspace owner.
      </p>
      <CodeBlock language="http">{`GET    /workspaces/{id}/files
GET    /workspaces/{id}/files/*path
PUT    /workspaces/{id}/files/*path
DELETE /workspaces/{id}/files/*path`}</CodeBlock>

      <h2 id="pty">PTY over WebSocket</h2>
      <p>
        For interactive workflows the runtime exposes a PTY upgrade at
        <code> /workspaces/&#123;id&#125;/terminal</code>. Clients send keystrokes as binary frames,
        receive PTY output the same way, and resize via control frames. The dashboard’s built-in
        terminal and the VSCode extension’s integrated terminal both use this protocol.
      </p>

      <h2 id="preview-proxy">Preview proxy</h2>
      <p>
        Live preview works through a signed token proxy: the runtime mints a token tied to a specific
        workspace + port + TTL, the iframe loads <code>https://preview.ironflyer.dev/&#123;ws&#125;/?t=…</code>,
        and the proxy validates the token before forwarding to the sandbox port. Because the auth lives
        in the URL query string, iframes work cleanly inside webviews where you cannot add headers.
      </p>

      <h2 id="isolation">Isolation</h2>
      <p>
        Every workspace carries an owner id. The runtime’s middleware refuses any read or write that
        does not match. The orchestrator forwards the caller’s JWT to the runtime so the runtime can
        run the same ownership check independently — defence in depth, two services agreeing on the
        same fact.
      </p>
    </DocPage>
  );
}
