// /docs/concepts/ai-stack — the AI substrate. Five providers behind a
// capability-tagged router, four memory stores with semantic re-ranking,
// the hash-chained audit log, the Context7 docs tool, multi-agent voting
// on critical gates, and the post-patch reflection loop.

import type { Metadata } from 'next';
import { DocPage } from '../../../../components/docs/DocPage';
import { CodeBlock } from '../../../../components/docs/CodeBlock';

export const metadata: Metadata = {
  title: 'AI stack',
  description: 'Five providers, four memory stores, a hash-chained audit log, Context7 docs lookup, multi-agent voting, and a reflection loop.',
  openGraph: {
    title: 'AI stack · Ironflyer',
    description: 'The AI substrate behind the finisher: routing, memory, audit, docs tool, voting, and reflection.',
    images: ['/opengraph-image'],
  },
};

const toc = [
  { id: 'five-providers-one-router', label: 'Five providers, one router' },
  { id: 'memory-engine', label: 'Memory engine' },
  { id: 'audit-log', label: 'Audit log' },
  { id: 'context7-docs-lookup', label: 'Context7 docs lookup' },
  { id: 'multi-agent-voting', label: 'Multi-agent voting' },
  { id: 'reflection-loop', label: 'Reflection loop' },
];

export default function AIStackPage() {
  return (
    <DocPage
      eyebrow="Concepts"
      title="AI stack"
      description="The AI substrate behind the finisher loop: five providers, four memory stores with semantic re-ranking, a hash-chained audit log, the Context7 docs tool, multi-agent voting, and a post-patch reflection step."
      toc={toc}
    >
      <h2 id="five-providers-one-router">Five providers, one router</h2>
      <p>
        The orchestrator treats <strong>Anthropic</strong>, <strong>OpenAI</strong>,{' '}
        <strong>Google Gemini</strong>, <strong>HuggingFace</strong>, and the in-process{' '}
        <strong>Mock</strong> provider as first-class endpoints behind a single capability-tagged router. Each
        gate declares what it needs; the router resolves the tags to a concrete model on a concrete provider
        and runs the call through the billing guard before any tokens move.
      </p>
      <p>
        HuggingFace is the open-model lane: <code>meta-llama/Llama-3.3-70B-Instruct</code>,{' '}
        <code>Qwen/Qwen2.5-7B-Instruct</code>, <code>deepseek-ai/DeepSeek-V3</code>,{' '}
        <code>mistralai/Mixtral-8x22B-Instruct-v0.1</code>, and <code>meta-llama/Meta-Llama-3.1-405B-Instruct</code>{' '}
        — the only lane carrying the <code>CapPrivate</code> tag, because the models can run inside your own
        Inference Endpoint instead of a third-party API.
      </p>
      <p>The full capability tag set the router resolves against:</p>
      <CodeBlock language="text">{`CapReasoning    Heavier chain-of-thought work (architecture, critic).
CapCode         Patch generation, repair runs, refactors.
CapJSON         Structured output for spec, gate verdicts, telemetry.
CapVision       Image-attached prompts, screenshot diffing.
CapCheap        Recovery runs, cheap re-tries, lint chatter.
CapPrivate      Stays inside HuggingFace Inference Endpoints.
CapThinking     Extended-thinking models (Anthropic, OpenAI o-series).
CapTools        Tool-use loops with the Coder.
CapCache        Prompt caching where the provider supports it.
CapSpeculative  Two-provider race; first usable response wins.`}</CodeBlock>
      <p>
        On top of the capability filter, a <strong>UCB1 bandit</strong> learns from telemetry which provider
        actually ships best per task. Every call writes its outcome — latency, cost, gate pass — back to the
        bandit, and the next decision balances exploitation of the current winner with bounded exploration of
        the runners-up.
      </p>

      <h2 id="memory-engine">Memory engine</h2>
      <p>
        The memory engine persists four stores so the loop has structured context to draw on across runs:
      </p>
      <ul>
        <li>
          <strong>Project memory</strong> — facts about the codebase the agents are working on: the data
          model, the chosen stack, conventions, repeated decisions.
        </li>
        <li>
          <strong>Execution memory</strong> — what happened during the loop: failures, the fixes that resolved
          them, the gate verdicts those fixes flipped. Failure-to-fix lineage is captured automatically.
        </li>
        <li>
          <strong>User memory</strong> — preferences and constraints the user has expressed across projects.
        </li>
        <li>
          <strong>Business memory</strong> — operational facts about the account: tier, cap, top-spending
          models, the ones the router has been steering away from.
        </li>
      </ul>
      <p>
        Capture hooks run on the events that matter: a failing gate plus the patch that turned it green
        becomes one linked execution memory; a recurring decision becomes a project memory. The Planner,
        Architect, UXer, and Coder each get the relevant slice injected into their context on every call —
        the Coder sees the failure-to-fix lineage, the Architect sees the data model conventions, and so on.
      </p>
      <p>
        Retrieval is semantic. The default vector store wraps the HuggingFace embedding model{' '}
        <code>BAAI/bge-small-en-v1.5</code>; queries are embedded, candidate memories are cosine-re-ranked,
        and the top matches are what gets injected. The <code>VectorStore</code> wrapper makes the embedder
        pluggable, so a self-hosted endpoint can replace the default without touching the capture path.
      </p>
      <p>
        Persistence is split. The in-memory store is the default for local development; production deploys
        select SurrealDB via <code>IRONFLYER_DB_DRIVER=surrealdb</code>, which preserves memories across
        restarts and exposes SurrealQL queries on memory tags for downstream reporting.
      </p>

      <h2 id="audit-log">Audit log</h2>
      <p>
        Every gate verdict, patch admission, and provider call lands as one row in the audit log. Each row
        carries its own <code>SHA-256</code> hash plus the previous row&rsquo;s hash, so the sequence is a
        chain — silent edits break the chain at the first tampered row.
      </p>
      <CodeBlock language="text">{`row N-1   { index, ts, kind, payload, prevHash, hash }
row N     { index, ts, kind, payload, prevHash = row N-1.hash, hash }
row N+1   { index, ts, kind, payload, prevHash = row N.hash,   hash }`}</CodeBlock>
      <p>
        The chain is persistent — with the SurrealDB driver, the log survives restarts and replays cleanly
        from disk on boot. The verification endpoint walks every row and confirms each hash matches its
        contents and its predecessor:
      </p>
      <CodeBlock language="bash">{`GET /api/audit/verify
{ "intact": true, "firstBadIndex": null }`}</CodeBlock>
      <p>
        If a row is tampered with after the fact, <code>intact</code> goes false and{' '}
        <code>firstBadIndex</code> points at the first broken link — a reviewer can read the rows from there
        and see exactly where the chain diverges from its claimed history.
      </p>

      <h2 id="context7-docs-lookup">Context7 docs lookup</h2>
      <p>
        Context7 is auto-registered as an MCP server every time the orchestrator boots. The Coder gets a
        builtin <code>lookup_docs(library, query)</code> tool that does the resolve-then-fetch in a single
        call — the agent names the library it wants docs for, names the question, and the tool returns the
        relevant section straight from the upstream documentation.
      </p>
      <CodeBlock language="text">{`tool: lookup_docs
  library: "next.js"
  query:   "app router server actions revalidate"
→ returns the current Next.js docs section, fresh, not training-cutoff.`}</CodeBlock>
      <p>
        The tool is available on every Coder call, not just on opt-in. That means the patch loop can quote
        the current API surface of the library it&rsquo;s coding against instead of leaning on whatever the
        model memorised at training time.
      </p>

      <h2 id="multi-agent-voting">Multi-agent voting</h2>
      <p>
        Critical gates wrap the agent in <code>RunVoted</code>: three independent Critic runs race in
        parallel, each emits a verdict, and the majority result wins. When the majority is sub-confidence —
        no two runs agree, or the agreement is weak — the wrapper falls back to a single-shot run with a
        deeper prompt rather than committing to a coin-flip.
      </p>
      <p>
        Voting only fires where the cost of a wrong verdict outweighs the cost of three calls: the Critic on
        gate verdicts is the canonical case. Cheaper gates run single-shot. The router still routes each of
        the three runs independently, so a vote can mix providers if the capability tags allow it.
      </p>

      <h2 id="reflection-loop">Reflection loop</h2>
      <p>
        After a Code-gate patch lands, the Reviewer reads the original story&rsquo;s acceptance criteria
        alongside the applied patch and emits one of three verdicts:
      </p>
      <ul>
        <li>
          <strong>accomplished</strong> — the patch hits the acceptance criteria. Loop moves on.
        </li>
        <li>
          <strong>partial</strong> — some criteria are met, some are still open. The remaining work becomes a
          follow-up.
        </li>
        <li>
          <strong>drift</strong> — the patch went somewhere the story did not ask for. The drift is recorded
          as an <code>Execution</code> memory tagged <code>reflection</code>, so the next planning pass sees
          it as context: <em>last time the Coder added X when the story asked for Y</em>.
        </li>
      </ul>
      <p>
        Reflection memories feed the same retrieval path as the rest of execution memory — the next Coder
        call against the same project gets the drift entry injected, and the loop stops repeating the same
        scope mistake across runs.
      </p>
    </DocPage>
  );
}
