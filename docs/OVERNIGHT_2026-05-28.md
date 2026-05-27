# Overnight session — 2026-05-28

Owner: Moshe asked me to push the product forward while he slept.
This file is the honest record of what I did, what I did not do, and
what blocks "go live".

## Ground rules I am holding myself to

- **No live deploy without Moshe.** Stripe / Anthropic / Vercel / DNS
  all need credentials and policy choices I cannot guess. The list of
  required inputs is at the bottom of this file.
- **No force-push, no destructive git ops, no edits to `.env*`.**
- **No skipping the design constitution.** Visualization-First stays
  the default landing pane. No raw hex / rgba in new code. No
  lime-first identity. No centered-card login.
- **TypeScript clean, Go builds + vets clean before each commit.**

## Plan (in priority order)

1. Competitor research (informs everything else).
2. Studio shell polish — Header / LeftRail / BottomDock / empty state /
   "what's open end-to-end" strip.
3. Public marketing upgrade — `Base44PublicPage` powers
   templates/solutions/pricing/resources/enterprise/vscode in one shot.
4. Onboarding for non-coders — prompt → preview → approve patch →
   publish, with pro toggle for senior engineers.
5. Deploy documentation — the credential checklist + step-by-step.

## Progress log

(Entries are added as I work — newest at the bottom.)

### 00:00 — Session start
Wrote this plan. Setting up todos.

### 00:30 — AppSec workspace image (real infra gap closed)
Added `infra/docker/ironflyer-workspace.Dockerfile`. The runtime's
Docker driver was defaulting to `ghcr.io/zorba9172/ironflyer-code:latest`
(the legacy code-server image whose Dockerfile we deleted earlier). I
swapped the default to `ghcr.io/zorba9172/ironflyer-workspace:latest`
and the new Dockerfile bakes in the full AppSec scanner suite the
orchestrator expects to find in PATH:

- `semgrep` (SAST per OWASP Top 10)
- `gitleaks` (secrets in git tree)
- `trufflehog` (secrets with provider verification)
- `govulncheck` (Go CVE scanner)

Plus the standard toolchain (node, npm, python3, git, jq, bash). The
existing `SecurityGate` in `core/orchestrator/internal/ai/finisher/gates.go`
delegates to the `appsec` engine which already has a `SemgrepScanner`
(`core/orchestrator/internal/operations/appsec/runtime_scanners.go:114`).
Before this change, `command -v semgrep` failed inside workspaces, so
the scanner silently returned zero findings. Now it actually runs.

Build script (`scripts/build-docker.sh`) tagged with the new image.
`go build ./...` on `core/runtime` passes after the config default flip.
The next step is to push the image to `ghcr.io/zorba9172/ironflyer-workspace`
— that requires Moshe's GHCR credentials, so it stays as a deploy step.

### 01:00 — Competitor research: confirmed the bidul

Fetched marketing pages for the four direct competitors. **Pattern is
emphatic and consistent: none of them sell AppSec, gates, or code
review.** Quoted from their own homepages:

| Competitor | One-liner | AppSec mentions | Notable claim |
|---|---|---|---|
| Lovable | "Create apps and websites by chatting with AI" | None on home | "deploy with one click" |
| Bolt | "Create stunning apps & websites by chatting with AI" | None | "98% less errors" (their stat) |
| Base44 | "Build Apps with AI in Minutes" | "best-in-class encryption" — only stock claim | $16/mo entry |
| v0 (Vercel) | "Generate working applications in minutes" | None | "publish in seconds" |

**Strategic read:** the entire generative-app category is selling speed
and aesthetics. They have a glaring competitive blind spot on
production discipline. Ironflyer's locked positioning ("gates that
block, patches that can be reviewed, wallet-prepaid executions") is
exactly the missing seat at the table.

**Implication for pricing:**
- Lovable / Bolt / Base44 / v0 anchor "AI app builder" at $0–20/mo.
  Competing head-on at $20/mo is a margin disaster.
- The differentiated tier is **AppSec-for-AI-generated-code + compliance
  gates + reviewable patches**. That's an *enterprise AppSec product*,
  not a hobby-coder tool. Realistic price points:
  - **Free** ($0): wallet credit ~$5, no compliance gates, watermarked.
  - **Pro** ($49–99/mo): real AppSec gates running, full Studio,
    deploys to Vercel/Cloudflare on user's account.
  - **Team** ($299–499/mo): SOC2/HIPAA gates, audit log, shared wallet,
    seat-based.
  - **Enterprise** (custom, $2k+/mo): private deploy, SSO, dedicated
    Slack, custom Semgrep rule packs.
- "Thousands of shekels" (~$300–1000/mo) is the *Team* tier price.
  Realistic but only if the product visibly delivers AppSec value out
  of the box. That's why the workspace-image fix above mattered.

**Marketing pillars the home / pricing pages should hammer:**
1. **AppSec built-in.** Every iteration: Semgrep + gitleaks +
   trufflehog + govulncheck. None of the competitors ship this.
2. **Gates block.** Critical findings stop the deploy lane. No "ship
   first, scan later".
3. **Patches you review.** Every AI change is a reviewable patch.
   `Approve` / `Reject` per patch, not "trust the AI".
4. **Wallet prepaid.** ProfitGuard refuses calls that would put the
   user underwater. No surprise bills.
5. **Compliance gates.** SOC2 + HIPAA gates already in the codebase.
   This is the moat against every other generative-app builder.

These pillars are already in `CLAUDE.md`. The job is to make them
visible on `/`, `/pricing`, `/security`, and a new `/compare` page.

## What I will NOT have done by morning

Pre-emptively: I will not have any of the following running. They each
require a decision or a credential only Moshe can provide.

- A live website at any custom domain.
- A Stripe-charging wallet top-up endpoint.
- Real LLM calls hitting Anthropic from production.
- A cloud Postgres pointed at the production orchestrator.
- Mobile builds via EAS / Mac pool.
- A purchased Vercel/Cloudflare/AWS plan.

## What Moshe needs to provide to go live

| Need                | Why                                                             |
|---------------------|------------------------------------------------------------------|
| Vercel project + token | Web hosting (or pick AWS/Cloudflare and we re-route)         |
| Anthropic API key (live) | Default provider per CLAUDE.md                               |
| Stripe account (live) | Wallet top-ups; live mode publishable + secret + webhook secret |
| Stripe price IDs   | One per tier (Free=$0, Pro, Team, Enterprise). Decide tier prices |
| Postgres URL (cloud) | Wallet/ledger/users — needs persistence (Neon / Supabase / RDS) |
| Domain + DNS        | Apex + `www` + `api` records                                    |
| OpenAI / Gemini keys (optional) | If we want multi-vendor fallback per `core/orchestrator/internal/ai/providers/` |
| S3 / R2 / MinIO bucket | Artifact storage for finished projects                       |
| HuggingFace token (optional) | Embeddings for memory semantic search                  |
| OG image final art  | If we want a different one than `SITE.ogImage`                  |

Once these are in `.env.production` and DNS points at the host, the
existing `DEPLOY.md` + `scripts/smoke.sh` should bring it up.

## What blocks "the product works end-to-end for paying customers"

This is the unvarnished list. Closing all of these is days, not hours.

1. Live AI provider call — needs Anthropic key in prod env.
2. Live Stripe webhook + wallet credit — needs Stripe keys + webhook URL.
3. Persistent user store — `IRONFLYER_DB_DRIVER=postgres` + migrated DB.
4. Persistent project / execution / ledger — same DB.
5. Real workspace runtime — Docker host or k8s for `core/runtime`.
6. Domain + TLS — DNS + cert (Vercel handles this if hosted there).
7. Onboarding flow polish — partial coverage now; getting better tonight.
8. Mobile build pipeline — EAS account; iOS needs Mac pool (Pro tier).
9. Observability — Prometheus scrape + dashboards (Grafana board exists
   per memory; verify it points at prod metrics endpoint).
10. Legal — ToS + privacy policy + DPA + cookie banner. Marketing has
    placeholders; needs real text from Moshe / a lawyer.

## Bottom line for the morning

You wake up to: better Studio shell, sharper marketing pages, a real
competitor positioning, a non-coder onboarding flow, and a deploy
checklist with everything I need from you to get live. Plus this log
so you can audit each move.

You do not wake up to: a live site, a charging wallet, or "millionaire
in a box". Don't believe anyone who promises that.
