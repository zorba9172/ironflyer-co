// /docs/api/projects — the bulk of the orchestrator surface.

import type { Metadata } from 'next';
import { DocPage } from '../../../../components/docs/DocPage';
import { CodeBlock } from '../../../../components/docs/CodeBlock';

export const metadata: Metadata = {
  title: 'Projects API',
  description: 'CRUD for projects plus the finisher run, gates, files, chat stream, and GitHub linking.',
  openGraph: { title: 'Projects API · Ironflyer', description: 'Run finisher, stream events, chat, list patches, link GitHub.', images: ['/opengraph-image'] },
};

const toc = [
  { id: 'list', label: 'GET /projects' },
  { id: 'create', label: 'POST /projects' },
  { id: 'get', label: 'GET /projects/{id}' },
  { id: 'files', label: 'GET /projects/{id}/files' },
  { id: 'gates', label: 'GET /projects/{id}/gates' },
  { id: 'run', label: 'POST /projects/{id}/run' },
  { id: 'stream', label: 'GET /projects/{id}/stream' },
  { id: 'prompt', label: 'POST /projects/{id}/prompt' },
  { id: 'chat', label: 'POST /projects/{id}/chat' },
  { id: 'brainstorm', label: 'POST /projects/{id}/brainstorm' },
  { id: 'github', label: 'GitHub linking' },
];

export default function ProjectsAPIPage() {
  return (
    <DocPage
      eyebrow="API Reference"
      title="Projects"
      description="Every project endpoint is auth-protected and owner-scoped — non-owners get 404, not 403, so existence does not leak."
      toc={toc}
    >
      <h2 id="list">GET /projects</h2>
      <p>Returns the caller’s projects + any public projects they can read.</p>
      <CodeBlock language="bash">{`curl https://api.ironflyer.dev/projects \\
  -H "Authorization: Bearer $TOKEN"`}</CodeBlock>

      <h2 id="create">POST /projects</h2>
      <p>Creates a project. The id is derived from the name if not provided.</p>
      <CodeBlock language="json">{`{
  "name": "Revenue dashboard",
  "description": "Optional",
  "idea": "A revenue dashboard for indie SaaS founders. Stripe + Postgres. CSV export."
}`}</CodeBlock>

      <h2 id="get">GET /projects/&#123;id&#125;</h2>
      <p>Returns the full project including spec, files (paths only), and gate states.</p>

      <h2 id="files">GET /projects/&#123;id&#125;/files</h2>
      <p>Returns the file list. Each entry is a path + type + size; file contents are not included to keep the response small.</p>

      <h2 id="gates">GET /projects/&#123;id&#125;/gates</h2>
      <p>Returns the eight gate states in their canonical order, each one with status, last-updated, and issues.</p>

      <h2 id="run">POST /projects/&#123;id&#125;/run</h2>
      <p>
        Kicks off a finisher run. The handler forwards the caller’s bearer to the workspace runtime so
        build/test gates authenticate as the same user. Returns the final <code>report</code>; you can
        also subscribe to <code>/stream</code> for live updates.
      </p>

      <h2 id="stream">GET /projects/&#123;id&#125;/stream</h2>
      <p>
        Server-Sent Events. Emits an <code>execution</code> event per gate transition + a 15-second
        heartbeat. Closes when the run is done or the client disconnects.
      </p>
      <CodeBlock language="bash">{`curl -N https://api.ironflyer.dev/projects/PROJECT_ID/stream \\
  -H "Authorization: Bearer $TOKEN"`}</CodeBlock>

      <h2 id="prompt">POST /projects/&#123;id&#125;/prompt</h2>
      <p>
        One-shot planner call. Useful when you want a structured plan without committing to a full
        finisher run. Body: <code>&#123; "prompt": "…" &#125;</code>.
      </p>

      <h2 id="chat">POST /projects/&#123;id&#125;/chat</h2>
      <p>
        Streaming chat with an agent role of your choice. Body accepts an <code>effort</code> dial
        (<code>lite</code> | <code>economy</code> | <code>power</code>). Every token goes through the
        BillingGuard so cost lands in your ledger. Emits these SSE events: <code>turn</code>,
        <code>start</code>, <code>text</code>, <code>thinking</code>, <code>tool_use</code>,
        <code>done</code>, <code>error</code>.
      </p>
      <CodeBlock language="json">{`{
  "prompt": "Add a compact settings panel to the header.",
  "role":   "coder",
  "effort": "economy"
}`}</CodeBlock>

      <h2 id="brainstorm">POST /projects/&#123;id&#125;/brainstorm</h2>
      <p>
        Runs the strategist + brainstorm runner against a goal. Returns the chosen plan and the
        outcome of executing it. Useful for product-discovery prompts; the orchestrator picks the
        agent best suited to the goal.
      </p>

      <h2 id="github">GitHub linking</h2>
      <p>The following endpoints associate a project with a GitHub repo and clone it into the workspace runtime:</p>
      <ul>
        <li><code>POST /projects/&#123;id&#125;/connect-github</code> — body <code>&#123; owner, repo &#125;</code> or <code>&#123; fullName &#125;</code></li>
        <li><code>DELETE /projects/&#123;id&#125;/connect-github</code> — unlink</li>
        <li><code>POST /projects/&#123;id&#125;/clone-into-workspace</code> — body <code>&#123; workspaceId, ref?, subdir? &#125;</code></li>
      </ul>
    </DocPage>
  );
}
