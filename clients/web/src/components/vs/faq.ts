// FAQ generator for /vs/[slug] pages. Lives next to the layout so the
// page component can render both the visible FAQ section and the
// FAQPage JSON-LD structured-data payload from a single source.

import type { Competitor } from "./competitors";

export type FaqItem = { q: string; a: string };

export function buildFaq(c: Competitor): FaqItem[] {
  return [
    {
      q: `How do I migrate an existing ${c.name} project to Ironflyer?`,
      a: `Export your ${c.name} project as a Git repository, point Ironflyer at the repo, and the importer reproduces the stack decision into a real Linux Docker workspace. Your first run is a gated execution against your prepaid wallet — Budget, Patch review, and Deploy verdicts fire on the existing codebase, no rewrite required.`,
    },
    {
      q: `How does Ironflyer pricing compare to ${c.name}?`,
      a: `Ironflyer is a prepaid wallet, not a flat subscription. Every paid execution reserves funds before any premium token is spent, debits provider cost into an append-only ledger as it materialises, and releases the unused hold on commit. You pay for finished, gated executions — not seats or speculative compute.`,
    },
    {
      q: `Is my code portable if I leave Ironflyer?`,
      a: `Yes. Your project lives in a real Git repository inside your Docker workspace; you can clone, push, and self-host the same artifacts Ironflyer deploys. Deploy targets include Docker, Vercel, Fly, Cloudflare, and self-hosted runners, so there is no lock-in beyond the gate verdicts and ledger entries that live with your wallet.`,
    },
    {
      q: `Can I bring my own Anthropic, OpenAI, or Gemini API key?`,
      a: `Yes. Ironflyer's provider layer supports Anthropic (default), OpenAI, Gemini, HuggingFace, and DeepSeek. Bring-your-own-key mode bills provider cost directly to your account so the platform margin shrinks to the runtime + gates layer. ProfitGuard still gates premium calls so your own bill stays sane.`,
    },
    {
      q: `Why not just use ${c.name} plus my own engineering discipline?`,
      a: `You can. The honest answer: the discipline costs human attention. Ironflyer encodes the same discipline as gates that block — a Budget gate refuses to start without reservation, a ProfitGuard gate refuses premium calls without ROI, a MobileBuild gate refuses to deploy without a real artifact. If your team already runs that loop reliably, ${c.name} plus your own discipline is fine. If they do not, the gates do the blocking for you.`,
    },
  ];
}
