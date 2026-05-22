// /docs/api/deploy — the deploy package surface.

import type { Metadata } from 'next';
import { DocPage } from '../../../../components/docs/DocPage';
import { CodeBlock } from '../../../../components/docs/CodeBlock';

export const metadata: Metadata = {
  title: 'Deploy API',
  description: 'One-click deploys to Fly and Railway, ZIP exports, and GitHub repo exports.',
  openGraph: { title: 'Deploy API · Ironflyer', description: 'Push, stream, export. Fly + Railway + GitHub + ZIP.', images: ['/opengraph-image'] },
};

const toc = [
  { id: 'plan', label: 'GET /projects/{id}/deploy/plan' },
  { id: 'start', label: 'POST /projects/{id}/deploy' },
  { id: 'list', label: 'GET /projects/{id}/deployments' },
  { id: 'stream', label: 'GET /deployments/{id}/stream' },
  { id: 'export-zip', label: 'POST /projects/{id}/export/zip' },
  { id: 'export-github', label: 'POST /projects/{id}/export/github' },
];

export default function DeployAPIPage() {
  return (
    <DocPage
      eyebrow="API Reference"
      title="Deploy"
      description="Plan, start, stream, export. Every endpoint requires the same auth and project ownership as the rest of the orchestrator."
      toc={toc}
    >
      <h2 id="plan">GET /projects/&#123;id&#125;/deploy/plan</h2>
      <p>
        Returns the detected stack, the deploy artifacts that will be generated or used as-is, and
        which providers are enabled on this orchestrator instance.
      </p>
      <CodeBlock language="json">{`{
  "stack": "node",
  "artifacts": [
    { "path": "Dockerfile", "source": "generated" },
    { "path": "fly.toml",   "source": "generated" }
  ],
  "providers": { "fly": true, "railway": false }
}`}</CodeBlock>

      <h2 id="start">POST /projects/&#123;id&#125;/deploy</h2>
      <p>
        Starts a deploy. Body: <code>&#123; provider: "fly" | "railway", region?, env? &#125;</code>.
        Returns 202 with a <code>deploymentId</code> and the SSE stream URL.
      </p>
      <CodeBlock language="bash">{`curl -X POST https://api.ironflyer.dev/projects/PROJECT_ID/deploy \\
  -H "Authorization: Bearer $TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"provider":"fly","region":"fra"}'`}</CodeBlock>

      <h2 id="list">GET /projects/&#123;id&#125;/deployments</h2>
      <p>Returns the caller’s deployments for the project, newest first. In-memory today; bounded at 50 per project.</p>

      <h2 id="stream">GET /deployments/&#123;deploymentId&#125;/stream</h2>
      <p>
        SSE. Events: <code>deploy_started</code>, <code>build_started</code>, <code>push_started</code>,
        <code>log</code> (one per provider log line), <code>deployed</code>, <code>failed</code>. The
        stream replays all past events to late subscribers so the dashboard does not lose context on
        page reload.
      </p>

      <h2 id="export-zip">POST /projects/&#123;id&#125;/export/zip</h2>
      <p>
        Streams a ZIP of the project files. Useful when the caller wants to host elsewhere without
        going through Fly / Railway. Content-Disposition is set so the browser downloads automatically.
      </p>

      <h2 id="export-github">POST /projects/&#123;id&#125;/export/github</h2>
      <p>
        Creates (or updates) a GitHub repository under the caller’s account and pushes the project
        files to it. Requires a connected GitHub identity. Body: <code>&#123; repoName?, description?, private? &#125;</code>.
        Returns the new repo URL.
      </p>
    </DocPage>
  );
}
