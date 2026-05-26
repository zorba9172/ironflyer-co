// Public-information comparison data for /vs/[slug] SEO landing pages.
//
// Facts about each competitor reflect their publicly stated product as
// of 2026-05. We intentionally credit each competitor with three honest
// strengths before listing the production mechanics Ironflyer adds on
// top. This is a comparison page, not a hit piece.

export type IronflyerWin = {
  mechanic: string;
  oneLineWhy: string;
};

export type Competitor = {
  slug: string;
  name: string;
  tagline: string;
  category: string;
  whatTheyDoWell: string[];
  whereTheyFallShort: string[];
  ironflyerWins: IronflyerWin[];
  switchingNotes: string[];
};

export const competitors: Competitor[] = [
  {
    slug: "lovable",
    name: "Lovable",
    tagline: "Describe your idea, get a working app.",
    category: "AI app builder",
    whatTheyDoWell: [
      "Fast first render: the prompt-to-preview loop produces a recognisable web app in minutes.",
      "Generous component library and reasonable visual defaults for marketing sites and CRUD dashboards.",
      "Friendly onboarding for non-engineers — the editor stays out of the way until you ask for it.",
    ],
    whereTheyFallShort: [
      "No prepaid wallet enforcement — runs do not block on a pre-execution funds check, so cost surprises happen on the bill, not the gate.",
      "No public gate registry: pre-execution checks for budget, secrets, mobile build manifest, and deploy readiness are not a first-class surface you can inspect or extend.",
      "Patch review is implicit. Ironflyer surfaces every AI edit as a reviewable patch with selective accept, comment, and revert.",
      "Web previews run in a managed sandbox rather than a real Linux Docker workspace with shell, PTY, and persistent file system.",
      "No native mobile build target — Expo / Android Gradle / iOS xcodebuild artifacts are not produced as part of the finisher pipeline.",
    ],
    ironflyerWins: [
      {
        mechanic: "Wallet + 402",
        oneLineWhy: "An underfunded execution returns 402 Payment Required with a top-up URL before any premium token is spent.",
      },
      {
        mechanic: "Gate registry",
        oneLineWhy: "Budget, ProfitGuard, MobileBuild, Deploy and Secrets gates run in a documented order and block the patch when they fail.",
      },
      {
        mechanic: "Patch lifecycle",
        oneLineWhy: "Every AI write is a patch you can review, accept selectively, comment on, or revert — never silent on-disk edits.",
      },
      {
        mechanic: "Docker workspace",
        oneLineWhy: "Real Linux container per user with PTY WebSocket, file API, and persistent volume — not a hosted in-browser preview.",
      },
      {
        mechanic: "ProfitGuard",
        oneLineWhy: "Premium model calls and Mac-pool mobile builds are gated by expected-ROI math before they fire.",
      },
      {
        mechanic: "Append-only ledger",
        oneLineWhy: "Revenue, provider cost, and gross margin are recorded per-execution and reconcilable against your wallet down to the cent.",
      },
    ],
    switchingNotes: [
      "Export your code from Lovable as a Git repository or zip download.",
      "Point Ironflyer at the repo and let the importer reproduce the stack decision.",
      "Top up your wallet and run the first gated execution to see Budget, Patch, and Deploy gates fire.",
      "Move your environment variables into the Secrets vault so the SecretsGate stops blocking.",
    ],
  },
  {
    slug: "bolt",
    name: "Bolt",
    tagline: "Prompt, run, deploy — entirely in the browser.",
    category: "In-browser AI dev environment",
    whatTheyDoWell: [
      "WebContainers are genuinely impressive: a Node runtime inside the browser tab with near-instant boot.",
      "Round-trip from prompt to interactive preview is among the fastest in the category.",
      "Strong fit for self-contained JS/TS demos, small front-end prototypes, and learning material.",
    ],
    whereTheyFallShort: [
      "WebContainers are a browser-sandboxed Node — not a real Linux box. Native binaries, system packages, GPU access, and arbitrary daemons are out of reach.",
      "Workspaces are tab-scoped. Ironflyer keeps a persistent server-side Docker workspace per user that survives reloads, devices, and team handoff.",
      "No prepaid wallet model — there is no ledger entry per execution mapping revenue, provider cost, and gross margin.",
      "No gate registry: the orchestrator-style budget, ProfitGuard, MobileBuild and Deploy gates that block bad runs before they spend tokens are absent.",
      "No first-class native mobile build path — Android Gradle and iOS xcodebuild are not part of the finisher pipeline.",
    ],
    ironflyerWins: [
      {
        mechanic: "Real Docker",
        oneLineWhy: "Per-user Linux container with shell, PTY WebSocket, file API, and persistent volume — not a browser-tab runtime.",
      },
      {
        mechanic: "Wallet + 402",
        oneLineWhy: "Execution refuses to start without reservation; underfunded callers get a top-up URL, not a silent failure.",
      },
      {
        mechanic: "Gate registry",
        oneLineWhy: "Budget, ProfitGuard, MobileBuild, Secrets, and Deploy gates run in a documented order and block on failure.",
      },
      {
        mechanic: "Mobile build",
        oneLineWhy: "Expo / Android Gradle / iOS xcodebuild artifacts are produced under a real MobileBuild gate, not implied by a web preview.",
      },
      {
        mechanic: "ProfitGuard",
        oneLineWhy: "Premium model calls are gated by expected ROI so margin stays positive in steady state.",
      },
      {
        mechanic: "Append-only ledger",
        oneLineWhy: "Per-execution revenue, cost, and margin are reconcilable against wallet and Stripe vault snapshot.",
      },
    ],
    switchingNotes: [
      "Export your StackBlitz / Bolt project as a Git repository.",
      "Point Ironflyer at the repo; the importer rebuilds the stack decision against a real Linux Docker workspace.",
      "Top up your wallet and run the first gated execution to see Budget, Patch, and Deploy verdicts fire.",
      "Move sensitive env vars from the tab to the Secrets vault so SecretsGate unblocks deploys.",
    ],
  },
  {
    slug: "replit-agent",
    name: "Replit Agent",
    tagline: "Build, run, and deploy with an agent inside Replit.",
    category: "AI dev environment with agent",
    whatTheyDoWell: [
      "Real container-backed workspace with a usable shell, package manager, and persistent file system.",
      "Multiplayer editing and a healthy template ecosystem make handoff and demos easy.",
      "Deploy targets are integrated end-to-end, including Replit Deployments and built-in databases.",
    ],
    whereTheyFallShort: [
      "Pricing is subscription-and-credit, not a wallet-prepaid 402 contract. Ironflyer refuses to start a run that cannot pay for itself.",
      "No public ProfitGuard surface — premium model spend and long verification loops are not gated by expected-ROI math.",
      "Gate registry is not exposed. Ironflyer publishes Budget, MobileBuild, Secrets, Deploy and ProfitGuard gates as inspectable verdicts.",
      "Patch-level review of AI edits is not the default lifecycle: edits land on disk first, then are reviewed downstream.",
      "No append-only platform ledger with per-execution revenue, provider cost, and gross margin reconciled against a Stripe vault snapshot.",
    ],
    ironflyerWins: [
      {
        mechanic: "Wallet + 402",
        oneLineWhy: "Hard pre-execution funds check — no run starts without reservation, returning a top-up URL instead of overspending.",
      },
      {
        mechanic: "ProfitGuard",
        oneLineWhy: "Every premium model call, sandbox allocation, and mobile build is gated by expected ROI before it fires.",
      },
      {
        mechanic: "Gate registry",
        oneLineWhy: "Budget, MobileBuild, Secrets, Deploy and ProfitGuard verdicts are inspectable and the patch is blocked on failure.",
      },
      {
        mechanic: "Patch lifecycle",
        oneLineWhy: "AI edits surface as reviewable patches with selective accept, not silent on-disk writes.",
      },
      {
        mechanic: "Append-only ledger",
        oneLineWhy: "Per-execution revenue, cost, margin recorded immutably and reconciled against Stripe vault snapshot.",
      },
      {
        mechanic: "Owner-scoped isolation",
        oneLineWhy: "Workspaces, projects, wallets, tokens — every store enforces an owner check before any read or mutation.",
      },
    ],
    switchingNotes: [
      "Export your Replit project as a Git repository (or use the GitHub mirror).",
      "Point Ironflyer at the repo and let the importer reproduce the stack decision into a Docker workspace.",
      "Top up your wallet and run the first gated execution to see Budget, ProfitGuard, and Deploy verdicts fire.",
      "Migrate Secrets out of Replit's environment into the Secrets vault so SecretsGate unblocks deploys.",
    ],
  },
  {
    slug: "v0",
    name: "v0",
    tagline: "Generate UI components and ship them to Vercel.",
    category: "UI generation tool",
    whatTheyDoWell: [
      "Best-in-class UI-component synthesis — Tailwind + shadcn output is consistent and copy-paste-ready.",
      "Deep integration with Next.js and Vercel deploy is genuinely frictionless for the happy path.",
      "Excellent for design exploration and iterating on a single screen before committing to a full stack.",
    ],
    whereTheyFallShort: [
      "Scope is UI-first, not full-app finisher: backend, auth, billing, gates, ledger, and mobile builds are not the surface.",
      "Deploy artifacts beyond Vercel (self-hosted Docker, Fly, Cloudflare, Render) are not first-class targets.",
      "No prepaid wallet model — there is no 402 pre-execution funds check or append-only platform ledger.",
      "No persistent server-side workspace — there is no Docker container per user with PTY, file API, and shell.",
      "No native mobile build path — Expo / Android Gradle / iOS xcodebuild are out of scope.",
    ],
    ironflyerWins: [
      {
        mechanic: "Full-app finisher",
        oneLineWhy: "End-to-end pipeline covers backend, auth, billing, secrets, deploy, and mobile — not just the UI layer.",
      },
      {
        mechanic: "Docker workspace",
        oneLineWhy: "Real Linux container per user with PTY WebSocket, file API, and persistent volume across sessions.",
      },
      {
        mechanic: "Wallet + 402",
        oneLineWhy: "No execution starts without reservation; ledger debits cost as it materialises so margin is always known.",
      },
      {
        mechanic: "Gate registry",
        oneLineWhy: "Budget, ProfitGuard, MobileBuild, Secrets, and Deploy gates run in a documented order and block on failure.",
      },
      {
        mechanic: "Multi-target deploy",
        oneLineWhy: "Deploy artifacts beyond Vercel — Docker, Fly, Cloudflare, and self-hosted runners are first-class targets.",
      },
      {
        mechanic: "Mobile build",
        oneLineWhy: "Expo, Android Gradle, and iOS xcodebuild artifacts produced under a real MobileBuild gate with ledger metering.",
      },
    ],
    switchingNotes: [
      "Export your v0 components into your repo as you already do today.",
      "Point Ironflyer at the repo; the finisher wires the components into a full app with auth, deploy, and gates.",
      "Top up your wallet and run the first gated execution to see Budget, Patch, and Deploy verdicts fire.",
      "Optionally keep Vercel as your deploy target — Ironflyer just adds gates and ledger on top of the same artifact.",
    ],
  },
  {
    slug: "base44",
    name: "Base44",
    tagline: "Describe and ship an app with built-in data and auth.",
    category: "AI app builder",
    whatTheyDoWell: [
      "Clean split-layout authoring experience and a sensible default stack for internal tools.",
      "Built-in data and auth lower the friction for non-engineering operators getting a CRUD app live.",
      "Reasonable iteration loop on UI and data model once the app is scaffolded.",
    ],
    whereTheyFallShort: [
      "No prepaid wallet enforcement — there is no 402 pre-execution funds check tied to a per-execution ledger entry.",
      "No public gate registry: Budget, ProfitGuard, MobileBuild, Secrets, and Deploy verdicts are not inspectable surfaces.",
      "Append-only platform ledger that reconciles revenue, provider cost, and gross margin per-execution is absent.",
      "Real Linux Docker workspaces with PTY and persistent volume per user are not exposed as the runtime.",
      "ProfitGuard-style ROI gating on premium model calls is not a published mechanic.",
    ],
    ironflyerWins: [
      {
        mechanic: "Production discipline",
        oneLineWhy: "Gates, patches, and ledger turn 'shipped' into a verifiable verdict rather than an UI claim.",
      },
      {
        mechanic: "Gate registry",
        oneLineWhy: "Budget, ProfitGuard, MobileBuild, Secrets, and Deploy gates run in a documented order and block on failure.",
      },
      {
        mechanic: "Append-only ledger",
        oneLineWhy: "Per-execution revenue, cost, and margin are recorded immutably and reconciled against the Stripe vault snapshot.",
      },
      {
        mechanic: "ProfitGuard",
        oneLineWhy: "Premium model calls, sandbox allocations, and mobile builds are gated by expected ROI before they fire.",
      },
      {
        mechanic: "Docker workspace",
        oneLineWhy: "Real Linux container per user with PTY WebSocket, file API, and persistent volume across sessions.",
      },
      {
        mechanic: "Patch lifecycle",
        oneLineWhy: "Every AI write surfaces as a reviewable patch with selective accept and revert — not silent on-disk edits.",
      },
    ],
    switchingNotes: [
      "Export your Base44 project's code and schema.",
      "Point Ironflyer at the exported repo and let the importer reproduce the stack decision into a Docker workspace.",
      "Top up your wallet and run the first gated execution to see Budget, ProfitGuard, and Deploy verdicts fire.",
      "Move secrets and connection strings into the Secrets vault so SecretsGate unblocks deploys.",
    ],
  },
];

export function getCompetitor(slug: string): Competitor | undefined {
  return competitors.find((c) => c.slug === slug);
}

export function getRelatedCompetitors(slug: string): Competitor[] {
  return competitors.filter((c) => c.slug !== slug);
}
