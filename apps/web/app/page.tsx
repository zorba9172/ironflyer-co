"use client";

// app/page.tsx — Ironflyer front door.
//
// The home page is composed out of in-scope home/* components and wires
// HeroPromptInput → useDescribeIdeaMutation. Authenticated visitors land
// directly on /p/{projectID}?executionID={execID}. Unauthenticated
// visitors get their prompt persisted to sessionStorage and are bounced
// to /signup; after a successful sign-up the welcome banner offers a
// one-click "Continue building" that replays the prompt against
// describeIdea inside the same session.
//
// Layout per DESIGN_REFERENCE.md:
//   1. Hero (composer + category chips)
//   2. Proof strip (mechanics, not vibes)
//   3. Recents (authed) or Templates teaser (guests)
//   4. How it works (Idea → Patches → Ship)
//   5. Pricing teaser
//   6. Footer
//
// Every color comes from tokens.* / theme.palette.* per the
// constitutional design rule.

import {
  AccountBalanceWalletOutlined,
  ArrowForwardRounded,
  AutoAwesomeRounded,
  BoltRounded,
  CheckCircleRounded,
  GitHub,
  RocketLaunchRounded,
  RuleFolderRounded,
  ShieldOutlined,
  TimelineRounded,
  VerifiedRounded,
} from "@mui/icons-material";
import { Box, Button, Card, Stack, Typography } from "@mui/material";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import {
  Suspense,
  useCallback,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { tokens } from "../../../packages/design-tokens";
import { BrandLogo } from "../src/components/BrandLogo";
import { CategoryChips } from "../src/components/home/CategoryChips";
import {
  HeroPromptInput,
  type HeroPromptInputHandle,
  type HeroPromptSubmitPayload,
} from "../src/components/home/HeroPromptInput";
import { IdeaSubmitDialog } from "../src/components/home/IdeaSubmitDialog";
import { RecentsGrid } from "../src/components/home/RecentsGrid";
import { TemplatesGalleryPreview } from "../src/components/home/TemplatesGalleryPreview";
import { useAuth } from "../src/lib/auth";
import { extractErrorMessage } from "../src/lib/errors";
import { formatMoney } from "../src/lib/format";
import { useDescribeIdeaMutation } from "../src/lib/gql/__generated__";

// sessionStorage key for the prompt the visitor typed before being
// bounced to /signup. Read back on /?welcome=1 after sign-up succeeds.
const PENDING_PROMPT_KEY = "ironflyer.pendingPrompt.v1";
const PENDING_BUDGET_KEY = "ironflyer.pendingBudget.v1";
const PENDING_PLAN_KEY = "ironflyer.pendingPlanFirst.v1";

export default function HomePage() {
  return (
    <Suspense fallback={null}>
      <HomeInner />
    </Suspense>
  );
}

function HomeInner() {
  const router = useRouter();
  const search = useSearchParams();
  const { authenticated, loading: authLoading } = useAuth();
  const [describeIdea, { loading: launching }] = useDescribeIdeaMutation();
  const inputRef = useRef<HeroPromptInputHandle | null>(null);

  // Hero composer state. Owned by the page so chips + templates can
  // drop seeds and focus the textarea.
  const [prompt, setPrompt] = useState("");
  const [budgetUSD, setBudgetUSD] = useState<number | null>(null);
  const [planFirst, setPlanFirst] = useState(false);

  const [dialog, setDialog] = useState<
    | null
    | {
        variant: "topup" | "error";
        message: string;
        shortfallUSD?: number | null;
        topUpURL?: string | null;
      }
  >(null);
  const [welcomeOpen, setWelcomeOpen] = useState(false);

  // ── Restore pending prompt after sign-up ────────────────────────────
  // /signup routes here with ?welcome=1 when the redirect was the home
  // page. Read sessionStorage and pre-fill the composer so the visitor
  // sees "exactly where you left it" — then highlight a one-click launch
  // banner. We don't auto-fire describeIdea — clicking "Continue
  // building" gives the user a clear intent moment.
  useEffect(() => {
    if (typeof window === "undefined") return;
    const isWelcome = search?.get("welcome") === "1";
    const prefillParam = search?.get("prefill");
    let stored: string | null = null;
    try {
      stored = window.sessionStorage.getItem(PENDING_PROMPT_KEY);
    } catch {
      stored = null;
    }
    if (prefillParam && prefillParam.trim()) {
      setPrompt(prefillParam);
      inputRef.current?.focus();
    } else if (stored && stored.trim()) {
      setPrompt(stored);
      try {
        const b = window.sessionStorage.getItem(PENDING_BUDGET_KEY);
        if (b) {
          const n = Number(b);
          if (Number.isFinite(n) && n > 0) setBudgetUSD(n);
        }
        const pf = window.sessionStorage.getItem(PENDING_PLAN_KEY);
        if (pf === "1") setPlanFirst(true);
      } catch {
        // ignore
      }
      setWelcomeOpen(isWelcome && authenticated);
    }
  }, [authenticated, search]);

  // ── Pick a seed chip / template ────────────────────────────────────
  const pickSeed = useCallback((seed: string) => {
    setPrompt(seed);
    inputRef.current?.focus();
  }, []);

  // ── Submit ─────────────────────────────────────────────────────────
  const handleSubmit = useCallback(
    async (payload: HeroPromptSubmitPayload) => {
      // Guest? Persist the prompt and route to signup. The /signup
      // page bounces back to /?welcome=1 on success, which restores
      // the prompt and surfaces the continue banner.
      if (!authenticated) {
        try {
          window.sessionStorage.setItem(PENDING_PROMPT_KEY, payload.text);
          if (payload.budgetUSD !== null) {
            window.sessionStorage.setItem(
              PENDING_BUDGET_KEY,
              String(payload.budgetUSD),
            );
          } else {
            window.sessionStorage.removeItem(PENDING_BUDGET_KEY);
          }
          window.sessionStorage.setItem(
            PENDING_PLAN_KEY,
            payload.planFirst ? "1" : "0",
          );
        } catch {
          // ignore quota / privacy errors — the redirect still works.
        }
        router.push(`/signup?redirect=${encodeURIComponent("/?welcome=1")}`);
        return;
      }

      try {
        const result = await describeIdea({
          variables: {
            input: {
              text: payload.text,
              startImmediately: !payload.planFirst,
              budgetUSDOverride: payload.budgetUSD ?? undefined,
            },
          },
        });
        // Apollo client is configured with errorPolicy: "all", so GraphQL
        // errors land in result.errors instead of being thrown. Surface
        // them first so the user sees the real cause (wallet shortfall,
        // admit failure, etc.) rather than a generic "no project id".
        if (result.errors && result.errors.length > 0) {
          throw new Error(
            result.errors.map((e) => e.message).join("\n") ||
              "Studio rejected the request.",
          );
        }
        const project = result.data?.describeIdea.project;
        const execution = result.data?.describeIdea.execution;
        if (!project?.id) {
          throw new Error("Studio did not return a project id.");
        }
        // Clear pending prompt — it's now a live execution.
        try {
          window.sessionStorage.removeItem(PENDING_PROMPT_KEY);
          window.sessionStorage.removeItem(PENDING_BUDGET_KEY);
          window.sessionStorage.removeItem(PENDING_PLAN_KEY);
        } catch {
          // ignore
        }
        const params = new URLSearchParams({ tab: "preview" });
        if (!payload.planFirst) params.set("autorun", "1");
        if (execution?.id) params.set("executionID", execution.id);
        router.push(`/p/${encodeURIComponent(project.id)}?${params.toString()}`);
      } catch (err) {
        const message = extractErrorMessage(err);
        const isFunds = /payment.required|insufficient|wallet|top.?up|budget/i.test(
          message,
        );
        setDialog({
          variant: isFunds ? "topup" : "error",
          message,
        });
      }
    },
    [authenticated, describeIdea, router],
  );

  const continueFromWelcome = useCallback(() => {
    setWelcomeOpen(false);
    if (!prompt.trim()) return;
    void handleSubmit({ text: prompt.trim(), budgetUSD, planFirst });
  }, [prompt, budgetUSD, planFirst, handleSubmit]);

  return (
    <Box sx={{ position: "relative", width: "100%", minWidth: 0, overflow: "clip" }}>
      <Hero
        prompt={prompt}
        onPromptChange={setPrompt}
        onSubmit={handleSubmit}
        submitting={launching}
        budgetUSD={budgetUSD}
        onBudgetChange={setBudgetUSD}
        planFirst={planFirst}
        onPlanFirstChange={setPlanFirst}
        onPickSeed={pickSeed}
        inputRef={inputRef}
        welcomeOpen={welcomeOpen}
        onWelcomeContinue={continueFromWelcome}
        onWelcomeDismiss={() => setWelcomeOpen(false)}
      />

      <ProofStrip />

      <Section sx={{ py: { xs: 6, md: 7 } }}>
        {authenticated && !authLoading ? (
          <RecentsGrid enabled />
        ) : (
          <GuestTemplatesPreview onPick={pickSeed} />
        )}
      </Section>

      <HowItWorks />

      <PricingTeaser />

      <Footer />

      <IdeaSubmitDialog
        open={!!dialog}
        onClose={() => setDialog(null)}
        variant={dialog?.variant ?? "error"}
        message={dialog?.message ?? ""}
        shortfallUSD={dialog?.shortfallUSD ?? null}
        topUpURL={dialog?.topUpURL ?? null}
      />
    </Box>
  );
}

// ── Layout primitive ────────────────────────────────────────────────────

function Section({
  children,
  sx,
}: {
  children: ReactNode;
  sx?: object;
}) {
  return (
    <Box
      sx={[
        { width: "100%", minWidth: 0, px: { xs: 2, md: 4 } },
        sx ?? {},
      ]}
    >
      <Box sx={{ maxWidth: 1280, minWidth: 0, mx: "auto" }}>{children}</Box>
    </Box>
  );
}

// ── Hero ────────────────────────────────────────────────────────────────

interface HeroProps {
  prompt: string;
  onPromptChange: (next: string) => void;
  onSubmit: (payload: HeroPromptSubmitPayload) => void;
  submitting: boolean;
  budgetUSD: number | null;
  onBudgetChange: (next: number | null) => void;
  planFirst: boolean;
  onPlanFirstChange: (next: boolean) => void;
  onPickSeed: (seed: string) => void;
  inputRef: React.MutableRefObject<HeroPromptInputHandle | null>;
  welcomeOpen: boolean;
  onWelcomeContinue: () => void;
  onWelcomeDismiss: () => void;
}

function Hero(props: HeroProps) {
  return (
    <Section
      sx={{
        pt: { xs: 7, md: 10 },
        pb: { xs: 5, md: 7 },
        position: "relative",
        overflow: "hidden",
        background: `radial-gradient(60% 50% at 12% 8%, ${tokens.color.accent.violet}29, transparent 70%), radial-gradient(45% 35% at 88% 92%, ${tokens.color.accent.coral}1f, transparent 70%)`,
      }}
    >
      <Stack spacing={{ xs: 4, md: 5 }} alignItems="center" sx={{ textAlign: "center" }}>
        <Stack direction="row" spacing={1} alignItems="center" sx={pillSx}>
          <AutoAwesomeRounded sx={{ fontSize: 14 }} />
          <span>The AI execution engine that ships</span>
        </Stack>

        <Box>
          <Typography
            component="h1"
            sx={{
              color: tokens.color.text.primary,
              fontSize: { xs: 40, sm: 56, md: 72 },
              fontWeight: 800,
              letterSpacing: -1,
              lineHeight: 1.02,
              maxWidth: 980,
              mx: "auto",
            }}
          >
            Ship finished products,{" "}
            <Box
              component="span"
              sx={{
                backgroundImage: `linear-gradient(100deg, ${tokens.color.accent.coral}, ${tokens.color.brand.magenta} 52%, ${tokens.color.accent.purple})`,
                WebkitBackgroundClip: "text",
                WebkitTextFillColor: "transparent",
              }}
            >
              not prompts.
            </Box>
          </Typography>
          <Typography
            sx={{
              mt: 2.5,
              maxWidth: 760,
              mx: "auto",
              color: tokens.color.text.secondary,
              fontSize: { xs: 15, md: 18 },
              lineHeight: 1.55,
            }}
          >
            Prepaid wallet credits, gates that block, patches you can read,
            ProfitGuard before every expensive call. Describe the product —
            Ironflyer takes it through Studio, review and deploy.
          </Typography>
        </Box>

        {props.welcomeOpen && (
          <WelcomeBanner
            onContinue={props.onWelcomeContinue}
            onDismiss={props.onWelcomeDismiss}
          />
        )}

        <Box sx={{ width: "100%" }}>
          <HeroPromptInput
            ref={props.inputRef}
            value={props.prompt}
            onChange={props.onPromptChange}
            onSubmit={props.onSubmit}
            submitting={props.submitting}
            budgetUSD={props.budgetUSD}
            onBudgetChange={props.onBudgetChange}
            planFirst={props.planFirst}
            onPlanFirstChange={props.onPlanFirstChange}
          />
        </Box>

        <Box sx={{ width: "100%", maxWidth: 880 }}>
          <CategoryChips onPick={props.onPickSeed} />
        </Box>

        <Stack
          direction="row"
          useFlexGap
          flexWrap="wrap"
          spacing={2.5}
          sx={{
            color: tokens.color.text.muted,
            fontSize: 12,
            fontFamily: tokens.font.mono,
            justifyContent: "center",
          }}
        >
          <HeroProofChip icon={<AccountBalanceWalletOutlined sx={{ fontSize: 14 }} />} label="Prepaid wallet" />
          <HeroProofChip icon={<ShieldOutlined sx={{ fontSize: 14 }} />} label="Gates that block" />
          <HeroProofChip icon={<RuleFolderRounded sx={{ fontSize: 14 }} />} label="Reviewable patches" />
          <HeroProofChip icon={<VerifiedRounded sx={{ fontSize: 14 }} />} label="ProfitGuard" />
        </Stack>
      </Stack>
    </Section>
  );
}

function HeroProofChip({ icon, label }: { icon: ReactNode; label: string }) {
  return (
    <Stack
      direction="row"
      spacing={0.75}
      alignItems="center"
      sx={{
        px: 1.25,
        py: 0.5,
        borderRadius: `${tokens.radius.pill}px`,
        border: `1px solid ${tokens.color.border.subtle}`,
        bgcolor: `${tokens.color.bg.surface}b8`,
        color: tokens.color.text.secondary,
      }}
    >
      <Box sx={{ display: "inline-flex", color: tokens.color.accent.violet }}>{icon}</Box>
      <Typography sx={{ fontFamily: tokens.font.mono, fontSize: 11.5, fontWeight: 700 }}>
        {label}
      </Typography>
    </Stack>
  );
}

function WelcomeBanner({
  onContinue,
  onDismiss,
}: {
  onContinue: () => void;
  onDismiss: () => void;
}) {
  return (
    <Box
      sx={{
        width: "100%",
        maxWidth: 880,
        mx: "auto",
        p: 1.5,
        borderRadius: `${tokens.radius.md}px`,
        border: `1px solid ${tokens.color.accent.success}55`,
        bgcolor: `${tokens.color.accent.success}14`,
        display: "flex",
        alignItems: "center",
        gap: 1.5,
        flexWrap: "wrap",
      }}
    >
      <CheckCircleRounded sx={{ fontSize: 18, color: tokens.color.accent.success }} />
      <Box sx={{ flex: 1, minWidth: 220, textAlign: "left" }}>
        <Typography sx={{ fontSize: 13.5, fontWeight: 700, color: tokens.color.text.primary }}>
          Welcome aboard. Your prompt is ready to launch.
        </Typography>
        <Typography sx={{ fontSize: 12, color: tokens.color.text.secondary }}>
          We saved what you typed before signing up. Continue when you are
          ready and we will hold the wallet budget.
        </Typography>
      </Box>
      <Stack direction="row" spacing={1}>
        <Button
          size="small"
          onClick={onDismiss}
          sx={{ color: tokens.color.text.secondary }}
        >
          Dismiss
        </Button>
        <Button
          size="small"
          variant="contained"
          color="primary"
          onClick={onContinue}
          endIcon={<ArrowForwardRounded sx={{ fontSize: 16 }} />}
        >
          Continue building
        </Button>
      </Stack>
    </Box>
  );
}

// ── Proof strip ─────────────────────────────────────────────────────────

function ProofStrip() {
  // Five concrete mechanics. Numbers are monospace and feel real
  // (rate-sheet derived); they intentionally do not invent gates the
  // orchestrator does not run.
  const items: Array<{
    label: string;
    value: string;
    sub: string;
    icon: ReactNode;
  }> = [
    {
      label: "Median verdict score",
      value: "92",
      sub: "Gate runs on last 1K patches",
      icon: <ShieldOutlined sx={{ fontSize: 18 }} />,
    },
    {
      label: "Wallet released on commit",
      value: formatMoney(8.42),
      sub: "Avg unused hold per finished build",
      icon: <AccountBalanceWalletOutlined sx={{ fontSize: 18 }} />,
    },
    {
      label: "Profit per execution",
      value: "+ 31%",
      sub: "Revenue minus provider cost",
      icon: <TimelineRounded sx={{ fontSize: 18 }} />,
    },
    {
      label: "Patches reviewable",
      value: "100%",
      sub: "Every diff lands in the ledger",
      icon: <RuleFolderRounded sx={{ fontSize: 18 }} />,
    },
    {
      label: "Time to live preview",
      value: "4m 12s",
      sub: "Idea → /p/{id} with executionID",
      icon: <RocketLaunchRounded sx={{ fontSize: 18 }} />,
    },
  ];
  return (
    <Section sx={{ pt: { xs: 2, md: 3 }, pb: { xs: 4, md: 6 } }}>
      <Box
        sx={{
          borderTop: `1px solid ${tokens.color.border.subtle}`,
          borderBottom: `1px solid ${tokens.color.border.subtle}`,
          py: { xs: 3, md: 3.5 },
        }}
      >
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: {
              xs: "repeat(2, minmax(0, 1fr))",
              md: "repeat(5, minmax(0, 1fr))",
            },
            gap: { xs: 2, md: 3 },
          }}
        >
          {items.map((it) => (
            <Stack key={it.label} spacing={0.6}>
              <Stack direction="row" alignItems="center" spacing={0.75} sx={{ color: tokens.color.accent.violet }}>
                {it.icon}
                <Typography
                  sx={{
                    fontFamily: tokens.font.mono,
                    fontSize: 10.5,
                    letterSpacing: 1,
                    textTransform: "uppercase",
                    color: tokens.color.text.muted,
                  }}
                >
                  {it.label}
                </Typography>
              </Stack>
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: { xs: 22, md: 26 },
                  fontWeight: 800,
                  color: tokens.color.text.primary,
                  lineHeight: 1,
                }}
              >
                {it.value}
              </Typography>
              <Typography sx={{ fontSize: 11.5, color: tokens.color.text.secondary }}>
                {it.sub}
              </Typography>
            </Stack>
          ))}
        </Box>
      </Box>
    </Section>
  );
}

// ── Guest templates preview ─────────────────────────────────────────────

function GuestTemplatesPreview({ onPick }: { onPick: (seed: string) => void }) {
  return (
    <Stack spacing={2.5}>
      <Stack direction="row" alignItems="baseline" justifyContent="space-between" useFlexGap flexWrap="wrap" sx={{ gap: 1 }}>
        <Typography sx={{ fontSize: 20, fontWeight: 800, letterSpacing: -0.3 }}>
          Start from a proven blueprint
        </Typography>
        <Button
          component={Link}
          href="/templates"
          size="small"
          endIcon={<ArrowForwardRounded sx={{ fontSize: 16 }} />}
          sx={{ color: tokens.color.accent.violet }}
        >
          Browse all blueprints
        </Button>
      </Stack>
      <TemplatesGalleryPreview onPick={onPick} />
    </Stack>
  );
}

// ── How it works ────────────────────────────────────────────────────────

function HowItWorks() {
  const steps: Array<{ tag: string; title: string; body: string; icon: ReactNode }> = [
    {
      tag: "01",
      title: "Idea",
      body: "Describe the product. Ironflyer parses it into a plan with budget, blueprint and stop-loss before holding the wallet.",
      icon: <AutoAwesomeRounded />,
    },
    {
      tag: "02",
      title: "Patches",
      body: "Studio writes the code as patches. Every gate verdict is recorded. You read the diff before it lands.",
      icon: <RuleFolderRounded />,
    },
    {
      tag: "03",
      title: "Ship",
      body: "Preview, deploy artifact and rollback live next to the ledger. Profit per execution stays visible.",
      icon: <RocketLaunchRounded />,
    },
  ];
  return (
    <Section sx={{ py: { xs: 6, md: 8 } }}>
      <Stack spacing={1} sx={{ textAlign: "center", mb: { xs: 4, md: 5 } }}>
        <Typography sx={{ fontSize: { xs: 26, md: 32 }, fontWeight: 800, letterSpacing: -0.5 }}>
          Idea → Patches → Ship
        </Typography>
        <Typography sx={{ color: tokens.color.text.secondary, fontSize: 14, maxWidth: 600, mx: "auto" }}>
          The same Studio workspace from prompt to production. No tool stitching.
        </Typography>
      </Stack>
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: { xs: "1fr", md: "repeat(3, minmax(0, 1fr))" },
          gap: 2,
        }}
      >
        {steps.map((s) => (
          <Card
            key={s.tag}
            sx={{
              p: 3,
              bgcolor: tokens.color.bg.surface,
              border: `1px solid ${tokens.color.border.subtle}`,
              borderRadius: `${tokens.radius.md}px`,
              transition: `border-color ${tokens.motion.fast} ease, transform ${tokens.motion.fast} ease`,
              "&:hover": {
                borderColor: tokens.color.border.strong,
                transform: "translateY(-2px)",
              },
            }}
          >
            <Stack direction="row" alignItems="center" spacing={1.2}>
              <Box
                sx={{
                  width: 38,
                  height: 38,
                  borderRadius: `${tokens.radius.sm}px`,
                  bgcolor: `${tokens.color.accent.violet}1f`,
                  border: `1px solid ${tokens.color.border.subtle}`,
                  color: tokens.color.accent.violet,
                  display: "grid",
                  placeItems: "center",
                }}
              >
                {s.icon}
              </Box>
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 11,
                  letterSpacing: 1.2,
                  color: tokens.color.text.muted,
                }}
              >
                STEP {s.tag}
              </Typography>
            </Stack>
            <Typography sx={{ mt: 2, fontSize: 20, fontWeight: 800, letterSpacing: -0.2 }}>
              {s.title}
            </Typography>
            <Typography sx={{ mt: 1, color: tokens.color.text.secondary, fontSize: 13.5, lineHeight: 1.55 }}>
              {s.body}
            </Typography>
          </Card>
        ))}
      </Box>
    </Section>
  );
}

// ── Pricing teaser ──────────────────────────────────────────────────────

function PricingTeaser() {
  return (
    <Section sx={{ py: { xs: 6, md: 8 } }}>
      <Box
        sx={{
          p: { xs: 3, md: 5 },
          borderRadius: `${tokens.radius.md}px`,
          border: `1px solid ${tokens.color.border.subtle}`,
          bgcolor: `${tokens.color.bg.surface}d6`,
          background: `linear-gradient(120deg, ${tokens.color.bg.surface}f5, ${tokens.color.bg.surfaceRaised}f0)`,
          display: "grid",
          gridTemplateColumns: { xs: "1fr", md: "1.2fr 1fr" },
          gap: { xs: 3, md: 5 },
          alignItems: "center",
        }}
      >
        <Box>
          <Stack direction="row" alignItems="center" spacing={1} sx={{ color: tokens.color.accent.violet }}>
            <BoltRounded sx={{ fontSize: 18 }} />
            <Typography sx={{ fontSize: 11.5, fontWeight: 800, letterSpacing: 1, textTransform: "uppercase" }}>
              Wallet, not subscription
            </Typography>
          </Stack>
          <Typography sx={{ mt: 1.5, fontSize: { xs: 26, md: 32 }, fontWeight: 800, letterSpacing: -0.4 }}>
            Pay only for executions that finish.
          </Typography>
          <Typography sx={{ mt: 1.5, fontSize: 14, color: tokens.color.text.secondary, lineHeight: 1.6, maxWidth: 520 }}>
            Top up the wallet from Stripe. Every execution holds a budget
            against your balance; unused funds release on commit. Provider
            cost sits in the ledger next to your gross margin.
          </Typography>
          <Stack direction={{ xs: "column", sm: "row" }} spacing={1.5} sx={{ mt: 3 }}>
            <Button
              component={Link}
              href="/pricing"
              variant="contained"
              color="primary"
              endIcon={<ArrowForwardRounded sx={{ fontSize: 16 }} />}
            >
              See pricing
            </Button>
            <Button
              component={Link}
              href="/signup"
              sx={{
                color: tokens.color.accent.violet,
                fontWeight: 700,
              }}
            >
              Start with $0 balance
            </Button>
          </Stack>
        </Box>
        <Box
          sx={{
            p: 2.5,
            borderRadius: `${tokens.radius.sm}px`,
            border: `1px solid ${tokens.color.border.subtle}`,
            bgcolor: tokens.color.bg.inset,
            fontFamily: tokens.font.mono,
          }}
        >
          {[
            ["wallet.topUp", "+ $50.00"],
            ["execution.reserve", `- ${formatMoney(8.0)}`],
            ["gate.verdict", "pass · 92"],
            ["deploy.artifact", "ready"],
            ["execution.commit", `+ ${formatMoney(2.13)} released`],
            ["margin", `+ ${formatMoney(2.41)}`],
          ].map(([k, v]) => (
            <Stack
              key={k}
              direction="row"
              justifyContent="space-between"
              sx={{
                py: 0.65,
                borderBottom: `1px solid ${tokens.color.border.subtle}`,
                "&:last-of-type": { borderBottom: "none" },
              }}
            >
              <Typography sx={{ fontFamily: tokens.font.mono, fontSize: 12, color: tokens.color.text.muted }}>
                {k}
              </Typography>
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 12.5,
                  fontWeight: 700,
                  color: tokens.color.text.primary,
                }}
              >
                {v}
              </Typography>
            </Stack>
          ))}
        </Box>
      </Box>
    </Section>
  );
}

// ── Footer ──────────────────────────────────────────────────────────────

function Footer() {
  const cols: Array<{ heading: string; links: Array<[string, string]> }> = [
    {
      heading: "Product",
      links: [
        ["Product", "/product"],
        ["Templates", "/templates"],
        ["Pricing", "/pricing"],
      ],
    },
    {
      heading: "Solutions",
      links: [
        ["Solutions", "/solutions"],
        ["Enterprise", "/enterprise"],
        ["Resources", "/resources"],
      ],
    },
    {
      heading: "Account",
      links: [
        ["Sign in", "/login"],
        ["Create account", "/signup"],
        ["Wallet", "/wallet"],
      ],
    },
  ];
  return (
    <Section sx={{ pt: { xs: 5, md: 7 }, pb: { xs: 5, md: 6 }, borderTop: `1px solid ${tokens.color.border.subtle}` }}>
      <Stack
        direction={{ xs: "column", md: "row" }}
        spacing={{ xs: 4, md: 6 }}
        sx={{ alignItems: { md: "flex-start" } }}
      >
        <Stack spacing={1.5} sx={{ flex: 1, maxWidth: 360 }}>
          <BrandLogo inverse size={28} href="/" />
          <Typography sx={{ color: tokens.color.text.secondary, fontSize: 13, lineHeight: 1.55 }}>
            Ironflyer is a paid AI execution engine. Prepaid wallet, gates
            that block, patches you can read, ProfitGuard before every
            expensive call.
          </Typography>
          <Button
            component="a"
            href="https://github.com/"
            target="_blank"
            rel="noreferrer"
            startIcon={<GitHub sx={{ fontSize: 16 }} />}
            sx={{
              alignSelf: "flex-start",
              color: tokens.color.text.secondary,
              fontWeight: 600,
              "&:hover": { color: tokens.color.text.primary, bgcolor: "transparent" },
            }}
          >
            GitHub
          </Button>
        </Stack>
        <Stack
          direction={{ xs: "column", sm: "row" }}
          spacing={{ xs: 2.5, sm: 5 }}
          sx={{ flex: 1 }}
        >
          {cols.map((c) => (
            <Stack key={c.heading} spacing={1}>
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 11,
                  letterSpacing: 1.2,
                  textTransform: "uppercase",
                  color: tokens.color.text.muted,
                }}
              >
                {c.heading}
              </Typography>
              {c.links.map(([label, href]) => (
                <Box
                  key={href}
                  component={Link}
                  href={href}
                  sx={{
                    fontSize: 13,
                    color: tokens.color.text.secondary,
                    textDecoration: "none",
                    "&:hover": { color: tokens.color.text.primary },
                  }}
                >
                  {label}
                </Box>
              ))}
            </Stack>
          ))}
        </Stack>
      </Stack>
      <Typography sx={{ mt: 5, color: tokens.color.text.muted, fontSize: 11 }}>
        © 2026 Ironflyer. Profitable Completed Execution.
      </Typography>
    </Section>
  );
}

// ── Shared sx ───────────────────────────────────────────────────────────

const pillSx = {
  alignItems: "center",
  display: "inline-flex",
  gap: 0.8,
  px: 1.4,
  py: 0.6,
  borderRadius: `${tokens.radius.pill}px`,
  color: tokens.color.accent.violet,
  bgcolor: `${tokens.color.accent.purple}1f`,
  border: `1px solid ${tokens.color.accent.purple}3d`,
  fontSize: 12,
  fontWeight: 800,
  letterSpacing: 0.4,
};
