// /blog/the-eight-gates — deep dive on the gates.

import type { Metadata } from 'next';
import { BlogPost } from '../../../components/docs/BlogPost';

export const metadata: Metadata = {
  title: 'The nine gates, explained — Ironflyer',
  description: 'Spec, UX, Architecture, Code, Lint, Tests, Security, Budget, Deploy — and what each one really blocks on.',
  openGraph: {
    title: 'The nine gates, explained',
    description: 'A deep dive on what each finisher gate actually does, and why the order matters.',
    images: ['/opengraph-image'],
  },
};

export default function TheEightGatesPost() {
  return (
    <BlogPost
      title="The nine gates, explained."
      subtitle="What each finisher gate actually does — and why their order is the load-bearing part."
      tag="Engineering"
      date="2026-04-24"
      gradient="linear-gradient(135deg, #671dfc 0%, #8b5cff 60%, #e5ff00 100%)"
    >
      <p>
        The marketing line is <em>“nine gates, every one blocks”</em>. The engineering line is
        longer. Here is what each gate actually does, why we picked these nine, and what we deliberately
        left out.
      </p>

      <h2>The shape of a gate</h2>
      <p>
        Every gate is a function with the same signature — <code>Run(ctx, *GateEnv) (GateResult, error)</code>
        — and the same lifecycle. It reads the project state, runs whatever check it implements, returns
        a structured result with a status (<code>pass</code>, <code>fail</code>, <code>skip</code>) and a
        list of issues. The orchestrator persists the result, emits an SSE event, and either advances to
        the next gate or invokes the recovery agent.
      </p>
      <p>
        Two consequences fall out of that shape. First, adding a gate is mostly a config change — we
        register it in <code>finisher.DefaultGates()</code>, add it to the SDK union, and the dashboard
        renders it automatically. Second, every gate is independently testable; the recovery loop is
        the same code path regardless of which gate failed.
      </p>

      <h2>1. Spec</h2>
      <p>
        The Spec gate turns the raw prompt into a structured <code>ProductSpec</code>. The schema is
        boring on purpose — name, idea, target audience, success criteria, stack constraints. If any of
        those are still ambiguous after the planner has had a turn, the gate fails with the missing
        field as an issue. The recovery agent asks the user (or the upstream caller) for the missing
        information instead of guessing.
      </p>

      <h2>2. UX</h2>
      <p>
        UX produces an information architecture and a screen map. We do not output Figma; we output
        machine-readable screen specs that the Code gate later turns into components. UX failures are
        almost always a sign of an underspecified spec — the spec said “dashboard” without saying what
        is on it — so failure here triggers a recovery agent that loops back into Spec rather than
        re-running UX in isolation.
      </p>

      <h2>3. Architecture</h2>
      <p>
        Architecture picks the stack, file layout, and data model. This is also where we honour
        budgeting decisions — Power-tier reasoning is allowed here for paying users, Lite-tier biases
        to fast + cheap. A failed Architecture gate means the chosen stack does not actually meet the
        spec (e.g. picking a static-site stack for a product that requires server-side personalisation).
      </p>

      <h2>4. Code</h2>
      <p>
        The longest gate by far. The coder agent emits patches; the patch engine validates each one;
        the orchestrator applies them. A Code-gate failure is when the agent runs out of useful steps
        without satisfying the spec — usually a sign that the architecture forced an impossible shape.
        Recovery kicks back to Architecture in those cases.
      </p>

      <h2>5. Lint</h2>
      <p>
        Lint runs the language-appropriate linter inside the sandbox. <code>eslint</code> for TS/JS,
        <code>tsc --noEmit</code> for type-only failures, <code>go vet</code> for Go, <code>ruff</code>
        for Python. The exit code is the gate result; the stderr is parsed into structured issues so
        the dashboard can group them by file. Lint is the gate where a one-line fix solves 90% of the
        failures, which is why we put it before Tests — running tests against a project that does not
        compile is wasted budget.
      </p>

      <h2>6. Tests</h2>
      <p>
        Tests runs the project’s test suite in the sandbox. We do not synthesise tests behind your back
        — if the spec implies tests are required, the coder is instructed to write them. The Tests gate
        is the most boring one to implement and the most important one to take seriously; without it,
        every previous gate becomes optimistic.
      </p>

      <h2>7. Security</h2>
      <p>
        Security scans applied patches for known footguns: hardcoded secrets, dangerous shell-out,
        SSRF-prone fetches, JWT misuse, plain-text password storage. The list is small and curated. We
        deliberately do not run a general-purpose SAST tool — false positives drown the signal, and the
        gate becomes a wall of warnings the model learns to ignore. Better to block five things hard
        than warn about five hundred.
      </p>

      <h2>8. Budget</h2>
      <p>
        Budget is the only gate no LLM can repair. It compares the project&rsquo;s accumulated provider
        cost against the user&rsquo;s plan cap; if spend has crossed the line, the gate blocks deploy
        until the plan tier rises, the project is split, or remaining iterations are pruned. Soft-warns
        at 80% so the dashboard surfaces a yellow chip before the wall hits.
      </p>

      <h2>9. Deploy</h2>
      <p>
        Deploy plans + materialises the deploy artifacts: <code>Dockerfile</code>, <code>fly.toml</code>,
        <code>railway.json</code>, a GitHub Actions workflow if you are exporting to a repo. The artifacts
        go through the patch lifecycle like everything else, so the Security gate gets a chance to look
        at them. Deploy failures usually mean a missing environment variable or an unsupported runtime;
        recovery returns to the user with the missing piece rather than guessing.
      </p>

      <h2>What we left out</h2>
      <p>
        We deliberately did not include a <em>Performance</em> gate, a <em>Bundle Size</em> gate, or a
        <em>Browser Compatibility</em> gate. Every one of those is a fair candidate, and every one of
        those would slow the loop down enough that the rest of the gates would feel slow by association.
        We would rather add them as opt-in lints than make them hard blockers — at least until we have
        evidence that customers actually care.
      </p>

      <h2>The ordering is the contract</h2>
      <p>
        The single most useful insight from a year of running the loop: <strong>the order is the
        contract</strong>. Every gate consumes what the previous gate produced. Re-ordering them — even
        once, even just to “run Tests after Security” — turned the loop from finishing-by-default into
        guessing-with-extra-steps. The fixed order is what lets the recovery agent be scoped instead of
        free-form. It is also what lets us write the marketing line as a single sentence.
      </p>
    </BlogPost>
  );
}
