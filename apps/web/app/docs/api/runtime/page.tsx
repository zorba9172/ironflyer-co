// /docs/api/runtime — the workspace runtime surface.

import type { Metadata } from 'next';
import { DocPage } from '../../../../components/docs/DocPage';
import { CodeBlock } from '../../../../components/docs/CodeBlock';

export const metadata: Metadata = {
  title: 'Runtime API',
  description: 'The workspace runtime — workspaces, files, exec, terminal WebSocket, preview tokens.',
  openGraph: { title: 'Runtime API · Ironflyer', description: 'Per-user Linux sandboxes, file I/O, exec, PTY, preview proxy.', images: ['/opengraph-image'] },
};

const toc = [
  { id: 'auth', label: 'Auth' },
  { id: 'workspaces', label: 'Workspaces' },
  { id: 'files', label: 'Files' },
  { id: 'exec', label: 'Exec' },
  { id: 'terminal', label: 'Terminal (WebSocket)' },
  { id: 'ports', label: 'Ports' },
  { id: 'preview-token', label: 'Preview tokens' },
  { id: 'apply-patch', label: 'Apply patch' },
  { id: 'git-clone', label: 'Git clone' },
];

export default function RuntimeAPIPage() {
  return (
    <DocPage
      eyebrow="API Reference"
      title="Runtime"
      description="The per-user Linux sandbox API. Same JWT as the orchestrator; the runtime independently checks owner identity."
      toc={toc}
    >
      <h2 id="auth">Auth</h2>
      <p>
        The runtime accepts the same JWTs the orchestrator issues. Pass them in <code>Authorization:
        Bearer …</code>. The preview proxy is the one exception — iframes cannot set headers, so the
        proxy verifies a signed <code>?t=…</code> token instead.
      </p>

      <h2 id="workspaces">Workspaces</h2>
      <ul>
        <li><code>GET /workspaces</code> — the caller’s workspaces.</li>
        <li><code>POST /workspaces</code> — body <code>&#123; projectId? &#125;</code>.</li>
        <li><code>GET /workspaces/&#123;id&#125;</code> — single workspace.</li>
        <li><code>DELETE /workspaces/&#123;id&#125;</code> — tear down.</li>
      </ul>

      <h2 id="files">Files</h2>
      <CodeBlock language="http">{`GET    /workspaces/{id}/files
GET    /workspaces/{id}/files/*path
PUT    /workspaces/{id}/files/*path
DELETE /workspaces/{id}/files/*path`}</CodeBlock>
      <p>
        PUT accepts text bodies. The directory tree is created on demand. DELETE refuses to remove the
        workspace root.
      </p>

      <h2 id="exec">Exec</h2>
      <p>
        <code>POST /workspaces/&#123;id&#125;/exec</code> with <code>&#123; shell: "go test ./..." &#125;</code>.
        Returns stdout, stderr, and exit code synchronously. Long-running commands should use the
        terminal WebSocket instead.
      </p>

      <h2 id="terminal">Terminal (WebSocket)</h2>
      <p>
        <code>GET /workspaces/&#123;id&#125;/terminal</code> upgrades to a WebSocket. Binary frames in
        either direction carry PTY bytes; a text frame with a JSON resize message updates the PTY
        dimensions. Standard protocol — works with xterm.js out of the box.
      </p>

      <h2 id="ports">Ports</h2>
      <ul>
        <li><code>GET /workspaces/&#123;id&#125;/ports</code> — observed open ports inside the sandbox.</li>
        <li><code>POST /workspaces/&#123;id&#125;/ports</code> — record a port for the preview proxy.</li>
      </ul>

      <h2 id="preview-token">Preview tokens</h2>
      <p>
        <code>POST /workspaces/&#123;id&#125;/preview-token</code> mints a signed token bound to a port
        and TTL. The dashboard uses this to embed a preview iframe; the VSCode extension uses the same
        URL inside its webview.
      </p>

      <h2 id="apply-patch">Apply patch</h2>
      <p>
        <code>POST /workspaces/&#123;id&#125;/apply-patch</code> is what the orchestrator calls to
        mirror a patch from the in-memory project store into the sandbox. You can call it directly if
        you want to test the patch protocol without going through the orchestrator.
      </p>

      <h2 id="git-clone">Git clone</h2>
      <p>
        <code>POST /workspaces/&#123;id&#125;/git-clone</code> clones a GitHub repo into the workspace.
        The orchestrator’s <code>/projects/&#123;id&#125;/clone-into-workspace</code> wrapper is what
        you usually want — it auto-forwards the user’s GitHub OAuth token.
      </p>
    </DocPage>
  );
}
