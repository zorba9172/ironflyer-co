// /docs/api/patches — list/propose/apply.

import type { Metadata } from 'next';
import { DocPage } from '../../../../components/docs/DocPage';
import { CodeBlock } from '../../../../components/docs/CodeBlock';

export const metadata: Metadata = {
  title: 'Patches API',
  description: 'List, propose, and apply patches. Every file change in Ironflyer is one of these.',
  openGraph: { title: 'Patches API · Ironflyer', description: 'Three endpoints that gate every byte the AI writes.', images: ['/opengraph-image'] },
};

const toc = [
  { id: 'list', label: 'GET /projects/{id}/patches' },
  { id: 'propose', label: 'POST /projects/{id}/patches' },
  { id: 'apply', label: 'POST /patches/{patchId}/apply' },
];

export default function PatchesAPIPage() {
  return (
    <DocPage
      eyebrow="API Reference"
      title="Patches"
      description="Three endpoints, one engine. List the patches on a project, propose a new one, apply it after the engine has validated."
      toc={toc}
    >
      <h2 id="list">GET /projects/&#123;id&#125;/patches</h2>
      <p>
        Returns every patch on the project — applied, pending, and rejected — newest first. The pending
        ones are what your UI typically wants to surface; the others are the audit log.
      </p>
      <CodeBlock language="bash">{`curl https://api.ironflyer.dev/projects/PROJECT_ID/patches \\
  -H "Authorization: Bearer $TOKEN"`}</CodeBlock>

      <h2 id="propose">POST /projects/&#123;id&#125;/patches</h2>
      <p>Submit a patch. The engine validates and gates synchronously; a rejected patch returns 400 with the issues.</p>
      <CodeBlock language="json">{`{
  "title":   "Add compact settings panel",
  "summary": "Adds account controls to the application shell.",
  "changes": [
    { "op": "update", "path": "app/layout.tsx", "content": "…" }
  ]
}`}</CodeBlock>

      <h2 id="apply">POST /patches/&#123;patchId&#125;/apply</h2>
      <p>
        Apply a validated patch. The orchestrator updates the in-memory project store, then asks the
        RuntimeApplier to mirror the change into the workspace sandbox so the live preview reloads.
        404 if the patch does not exist or the caller is not a project member.
      </p>
      <CodeBlock language="bash">{`curl -X POST https://api.ironflyer.dev/patches/p_abc/apply \\
  -H "Authorization: Bearer $TOKEN"`}</CodeBlock>
    </DocPage>
  );
}
