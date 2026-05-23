// /docs/concepts/finisher-gates — the doctrine page. The gates are the
// product differentiator, so this page leans on the actual implementation
// notes: order, blocking semantics, recovery, the runtime applier.

import type { Metadata } from 'next';
import { DocPage } from '../../../../components/docs/DocPage';
import { CodeBlock } from '../../../../components/docs/CodeBlock';

export const metadata: Metadata = {
  title: 'Finisher Gates',
  description: 'Nine blocking gates take a prompt from idea to deployed app. Here is what each one does.',
  openGraph: {
    title: 'Finisher Gates · Ironflyer',
    description: 'Spec, UX, Architecture, Code, Lint, Tests, Security, Budget, Deploy — every one blocks until it passes.',
    images: ['/opengraph-image'],
  },
};

const toc = [
  { id: 'why-gates', label: 'Why gates' },
  { id: 'order-and-blocking', label: 'Order & blocking' },
  { id: 'the-nine', label: 'The nine gates' },
  { id: 'auto-recovery', label: 'Auto-recovery' },
  { id: 'runtime-applier', label: 'Runtime applier' },
  { id: 'beyond-the-gates', label: 'Beyond the gates' },
];

export default function FinisherGatesPage() {
  return (
    <DocPage
      eyebrow="Concepts"
      title="Finisher Gates"
      description="The gates are the contract. They block until the code is genuinely done — not vibes-done."
      toc={toc}
    >
      <h2 id="why-gates">Why gates</h2>
      <p>
        Every other AI app builder we have benchmarked optimises for the demo: a screen recording where
        a prompt becomes a UI in ninety seconds. Try to actually ship that output and the seams are
        everywhere — missing types, no tests, a security hole the model never thought about, a deploy
        config that does not match the framework. The finisher gates exist to close those seams. They
        block. They emit issues you can read. They run again after a patch.
      </p>
      <p>
        That is also why we call the company a <em>Finisher</em>, not a generator: the value is in the
        last 20% that nobody else does well.
      </p>

      <h2 id="order-and-blocking">Order & blocking</h2>
      <p>
        Gates always run in order — earlier gates produce the inputs that later gates consume. A red
        gate halts the loop and the orchestrator emits a <code>gate_failed</code> event with the issues
        the gate found. There is no “best-effort skip”; if Lint fails, Tests do not run.
      </p>
      <CodeBlock language="text">{`Spec → UX → Architecture → Code → Lint → Tests → Security → Budget → Deploy
         ▲                                                              │
         └──────────────────  Auto-recovery patch  ─────────────────────┘`}</CodeBlock>

      <h2 id="the-nine">The nine gates</h2>
      <h3>1. Spec</h3>
      <p>
        Turns the raw prompt into a structured <code>ProductSpec</code> — what the product is, who it is
        for, the success criteria, the stack constraints. A failed Spec gate almost always means the
        prompt is genuinely ambiguous; the recovery agent asks for the missing piece.
      </p>
      <h3>2. UX</h3>
      <p>
        Generates an information architecture and an interaction map. We do not produce a Figma file;
        we produce machine-readable screen specs that the Code gate later renders into components.
      </p>
      <h3>3. Architecture</h3>
      <p>
        Chooses the stack, file layout, and data model. The architect agent considers the spec, the user
        budget tier, and the runtime drivers available. It is also where we honour our pricing margin —
        Power-tier reasoning is allowed here for paying users, Lite-tier biases to faster + cheaper.
      </p>
      <h3>4. Code</h3>
      <p>
        The longest gate. The coder agent emits patches through <code>patch.Engine.Propose</code>; the
        engine validates each patch, applies it to the in-memory project, and writes through to the
        runtime sandbox so the live preview reflects reality.
      </p>
      <h3>5. Lint</h3>
      <p>
        Runs the language-appropriate linter in the sandbox — <code>eslint</code>, <code>tsc --noEmit</code>,
        <code>go vet</code>, <code>ruff</code>. The exit code is the gate result; the stderr is parsed
        into structured issues so the dashboard can group them by file.
      </p>
      <h3>6. Tests</h3>
      <p>
        Runs the project’s test suite. We do not generate tests behind your back — if the spec implies
        tests are required, the coder is instructed to write them. If you turn off the Tests gate in
        your account settings, you also have to acknowledge that your project no longer ships under the
        Ironflyer finisher promise.
      </p>
      <h3>7. Security</h3>
      <p>
        Scans patches for known footguns — secrets, dangerous shell-out, SSRF-prone fetches, JWT
        misuse — and refuses patches that contain them. The list is small and curated; we would rather
        block fewer false positives than play whack-a-mole.
      </p>
      <h3>8. Budget</h3>
      <p>
        The only gate that no LLM can repair. It compares the project&rsquo;s accumulated provider cost to
        the user&rsquo;s plan cap; if the spend has crossed the line, the gate blocks deploy until the
        plan tier rises, the project is split, or remaining iterations are pruned. Soft-warns from 80% so
        the dashboard can surface a yellow chip before the wall hits.
      </p>
      <h3>9. Deploy</h3>
      <p>
        Plans + materialises the deploy artifacts. <code>fly.toml</code> for Fly, <code>railway.json</code>
        for Railway, a workflow file for the GitHub export. The artifacts go through the patch lifecycle
        like everything else, so the Security gate gets a chance to look at them first.
      </p>

      <h2 id="auto-recovery">Auto-recovery</h2>
      <p>
        When a gate fails, the loop does not stop forever — it triggers a recovery agent that targets
        only the failing gate. The recovery agent gets the gate’s issues, the relevant file slices, and
        a strict prompt: <em>do not touch anything outside the failing gate’s scope</em>. Recovery
        patches go through the same Security + Lint gates on re-run, so a bad fix gets caught too.
      </p>

      <h2 id="runtime-applier">Runtime applier</h2>
      <p>
        Applied patches do not just update an in-memory model. The orchestrator’s <em>RuntimeApplier</em>
        writes the changes through to the workspace sandbox where the preview is running, so the next
        page reload reflects reality. This is what lets the live preview be the source of truth instead
        of an aspirational mock.
      </p>

      <h2 id="beyond-the-gates">Beyond the gates</h2>
      <p>
        The gates decide what ships. Two complementary systems decide how the loop learns and how the
        loop is trusted. The <strong>Memory Engine</strong> persists four stores — project, execution,
        user, and business memory — and links every failure to the fix that resolved it. On every retry,
        the relevant slice is auto-injected into the coder’s context, so the same mistake does not get
        repeated across runs. The engine is queryable through <code>/api/memory</code>, which means humans
        and downstream agents can read what the loop has actually learned.
      </p>
      <p>
        The <strong>Audit Log</strong> is the compliance counterpart. Every gate verdict lands as an
        immutable, SHA-256 hash-chained entry, so the sequence of decisions cannot be silently rewritten
        after the fact. <code>GET /api/audit</code> returns the chain; <code>GET /api/audit/verify</code>
        attests that the chain is intact. Together, memory turns the loop into something that improves,
        and the audit log turns it into something a reviewer can defend.
      </p>
    </DocPage>
  );
}
