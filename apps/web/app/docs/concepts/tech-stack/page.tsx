// /docs/concepts/tech-stack — the why-we-picked-this page. Multi-provider
// routing, OpenTelemetry traces, inline image generation, MCP both ways,
// and the project dependency graph endpoint.

import type { Metadata } from 'next';
import { DocPage } from '../../../../components/docs/DocPage';
import { CodeBlock } from '../../../../components/docs/CodeBlock';

export const metadata: Metadata = {
  title: 'Tech stack',
  description: 'Why we picked Go + Next.js + 3 AI providers + OTel.',
  openGraph: {
    title: 'Tech stack · Ironflyer',
    description: 'The stack decisions behind the finisher: Go core, Next.js 15, three AI providers, OTLP traces, and the MCP loop.',
    images: ['/opengraph-image'],
  },
};

const toc = [
  { id: 'ai-provider-routing', label: 'AI provider routing' },
  { id: 'observability', label: 'Observability' },
  { id: 'image-generation', label: 'Image generation' },
  { id: 'mcp-integration', label: 'MCP integration' },
  { id: 'project-dependency-graph', label: 'Project dependency graph' },
];

export default function TechStackPage() {
  return (
    <DocPage
      eyebrow="Concepts"
      title="Tech stack"
      description="Why we picked Go + Next.js + three AI providers + OpenTelemetry. The pieces that make the finisher loop production-ready, not just demo-ready."
      toc={toc}
    >
      <h2 id="ai-provider-routing">AI provider routing</h2>
      <p>
        The orchestrator treats Anthropic, OpenAI, and Google Gemini as first-class providers and lets the
        router pick per capability and per cost. <strong>Anthropic Claude</strong> is the default — Sonnet
        and Haiku carry the bulk of spec, code, and review work because the gate prompts were tuned against
        them. <strong>OpenAI GPT-4o</strong> is the fast-and-cheap tier: <code>gpt-4o-mini</code> for
        cheap re-runs and recovery patches, <code>gpt-4o</code> for general code, <code>o3</code> when a
        gate explicitly needs heavier reasoning. <strong>Gemini 2.5 Pro</strong> handles vision-heavy
        reasoning — design refs in chat, screenshot diff analysis, the visual-diff gate — with{' '}
        <code>gemini-2.5-flash</code> as the cheap counterpart.
      </p>
      <p>
        Routing is capability-tagged. A gate declares what it needs (reasoning, vision, long-context,
        cheap), the router resolves that to a concrete model on a concrete provider, and the billing guard
        admits the call against the user&rsquo;s cap before it runs. When latency matters more than cost,{' '}
        <code>CapSpeculative</code> races two providers on the same prompt and keeps the first usable
        response, discarding the loser&rsquo;s tokens. The ledger still charges both sides honestly.
      </p>

      <h2 id="observability">Observability</h2>
      <p>
        Every agent call is an OpenTelemetry span. Set the exporter and the endpoint, and your traces flow
        to Jaeger, Honeycomb, Datadog, Tempo, or anywhere else that speaks OTLP HTTP.
      </p>
      <CodeBlock language="bash">{`IRONFLYER_OTEL_EXPORTER=otlp
IRONFLYER_OTEL_ENDPOINT=https://otlp.example.com:4318`}</CodeBlock>
      <p>
        Every <code>BillingGuard.CompleteStream</code> emits a span with the attributes you need to debug a
        production run: <code>user.id</code>, the capability tags the call resolved against, the input and
        output token counts, the dollar cost charged to the ledger, and the concrete model the router
        picked. When a gate misroutes or a provider degrades, the trace shows it before the dashboard does.
      </p>

      <h2 id="image-generation">Image generation</h2>
      <p>
        The Coder agent has a built-in <code>generate_image</code> tool. Calls go to OpenAI Images, the
        PNG is written to <code>public/assets/&lt;name&gt;.png</code> in the workspace, and the tool returns
        the path so the next patch can reference it as a static asset. It is inline mid-patch — the Coder
        can ask for a hero image, get the path back, and reference it in the same JSX it is generating.
      </p>
      <p>
        The image is a real file in the workspace, not a data URL embedded in a string. The Security gate
        sees it like any other binary asset, and the dependency graph treats it as a normal node under{' '}
        <code>public/assets/</code>.
      </p>

      <h2 id="mcp-integration">MCP integration</h2>
      <p>
        MCP works both directions. <strong>Server:</strong> <code>POST /api/mcp</code> exposes Ironflyer
        itself as an MCP server, so any MCP-aware client (Claude Desktop, Cursor, Zed) can drive a finisher
        run as a tool. <strong>Client:</strong> set <code>IRONFLYER_MCP_SERVERS</code> to a list of
        external MCP endpoints (Notion, Linear, GitHub, your own internal tools) and the orchestrator
        attaches their tools to the Coder&rsquo;s tool loop. The Coder can read a Linear issue, fetch a
        Notion spec, or open a GitHub PR from inside the same patch turn that writes the code.
      </p>
      <CodeBlock language="bash">{`IRONFLYER_MCP_SERVERS=https://mcp.notion.so,https://mcp.linear.app,https://mcp.github.com`}</CodeBlock>

      <h2 id="project-dependency-graph">Project dependency graph</h2>
      <p>
        <code>GET /api/projects/:id/graph</code> returns the project&rsquo;s import graph — nodes are files,
        edges are imports. The parser handles TypeScript, JavaScript, Go, and Python. Vendor and build
        output are filtered out: <code>node_modules</code>, <code>dist</code>, <code>build</code>,{' '}
        <code>.next</code>, and <code>vendor</code> never appear as nodes. The graph is computed on the
        live workspace, so it reflects whatever the last patch landed — not a stale snapshot.
      </p>
      <p>
        The endpoint is what powers the dependency view in the dashboard, but it is also a primitive: agents
        use it to scope refactors to a sub-graph, and gates use it to identify which files a Code patch
        actually touched downstream.
      </p>
    </DocPage>
  );
}
