// /docs/concepts/patches — the patch lifecycle. This is one of those topics
// developers ask about constantly: "why can't the AI just write files?"
// The answer is below.

import type { Metadata } from 'next';
import { DocPage } from '../../../../components/docs/DocPage';
import { CodeBlock } from '../../../../components/docs/CodeBlock';

export const metadata: Metadata = {
  title: 'Patches',
  description: 'Every file change in Ironflyer is a patch — proposed, gated, and applied through a single engine.',
  openGraph: {
    title: 'Patches · Ironflyer',
    description: 'Why we route every AI file change through a patch lifecycle.',
    images: ['/opengraph-image'],
  },
};

const toc = [
  { id: 'why-patches', label: 'Why patches' },
  { id: 'lifecycle', label: 'Lifecycle' },
  { id: 'file-change-shape', label: 'FileChange shape' },
  { id: 'proposing', label: 'Proposing a patch' },
  { id: 'applying', label: 'Applying a patch' },
  { id: 'human-in-the-loop', label: 'Human in the loop' },
];

export default function PatchesConceptPage() {
  return (
    <DocPage
      eyebrow="Concepts · מושגים"
      title="Patches"
      description="Every byte the AI writes goes through one engine, one set of gates, and one human-readable log."
      toc={toc}
    >
      <h2 id="why-patches">Why patches</h2>
      <p>
        The AI in Ironflyer never writes files directly. Even when the coder agent decides
        <code>src/components/Header.tsx</code> needs a new prop, the path is the same as for anything
        else: assemble a <code>Patch</code>, hand it to <code>patch.Engine.Propose</code>, wait for the
        engine to validate and gate it, and only then apply. The reason is simple — the gates only
        catch real problems if every change has to pass through them. The moment we allow a side-channel
        write, the “finished” promise becomes opt-in.
      </p>

      <h2 id="lifecycle">Lifecycle</h2>
      <p>A patch passes through four states:</p>
      <ul>
        <li><strong>proposed</strong> — submitted by an agent, stored, awaiting validation.</li>
        <li><strong>validated</strong> — the engine checked the changes against the file schema and the security gate.</li>
        <li><strong>applied</strong> — written to the in-memory project and the runtime sandbox.</li>
        <li><strong>rejected</strong> — at least one issue blocked it; the patch is kept for audit.</li>
      </ul>

      <h2 id="file-change-shape">FileChange shape</h2>
      <p>A patch is a header (project id, author, title, summary) plus an array of file changes:</p>
      <CodeBlock language="typescript">{`interface Patch {
  id: string;
  projectId: string;
  author: string;        // "agent:coder", "user:eithan@…", "deploy:user-id"
  title: string;
  summary: string;
  changes: FileChange[];
}

interface FileChange {
  op: 'create' | 'update' | 'delete';
  path: string;          // POSIX, relative to project root
  content?: string;      // omitted for 'delete'
  beforeHash?: string;   // optimistic-concurrency guard for 'update'
}`}</CodeBlock>

      <h2 id="proposing">Proposing a patch</h2>
      <p>
        Agents call the engine directly; humans call HTTP. Both routes converge inside the engine, so a
        manual patch from the dashboard gets the same security scan as one the AI produced.
      </p>
      <CodeBlock language="bash">{`curl -X POST https://api.ironflyer.dev/projects/PROJECT_ID/patches \\
  -H "Authorization: Bearer $TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{
    "title": "Add Hebrew RTL switch",
    "summary": "Wires a dir=rtl toggle into the root layout.",
    "changes": [
      {"op":"update","path":"app/layout.tsx","content":"…"}
    ]
  }'`}</CodeBlock>

      <h2 id="applying">Applying a patch</h2>
      <p>
        Once a patch is validated, <code>POST /patches/&#123;id&#125;/apply</code> performs the write.
        The orchestrator first persists the change to the in-memory project store, then asks the
        RuntimeApplier to mirror the change into the workspace sandbox. The live preview reloads on its
        own — there is no manual “refresh” step for the user.
      </p>

      <h2 id="human-in-the-loop">Human in the loop</h2>
      <p>
        Patches are not auto-applied by default for changes the security gate flags as risky. Risky
        means: edits to <code>auth/*</code>, edits to migration files, anything touching environment
        config. Those land in the dashboard as <em>pending</em> with a diff view and a single button.
        The VSCode extension surfaces the same diff inside the editor’s side-by-side diff view, and the
        apply action calls the same endpoint.
      </p>
    </DocPage>
  );
}
