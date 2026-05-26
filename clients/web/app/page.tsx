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
  CodeRounded,
  DataObjectRounded,
  GitHub,
  HubRounded,
  RocketLaunchRounded,
  RuleFolderRounded,
  SecurityRounded,
  SettingsSuggestRounded,
  ShieldOutlined,
  TimelineRounded,
  VerifiedRounded,
  VisibilityRounded,
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
import * as swal from "../src/lib/swal";
import { RecentsGrid } from "../src/components/home/RecentsGrid";
import { TemplatesGalleryPreview } from "../src/components/home/TemplatesGalleryPreview";
import { MechanicsBlock } from "../src/components/home/MechanicsBlock";
import { ComparisonTable } from "../src/components/home/ComparisonTable";
import { SocialProofStrip } from "../src/components/home/SocialProofStrip";
import { HomeFAQ } from "../src/components/home/HomeFAQ";
import { FinalCTABand } from "../src/components/home/FinalCTABand";
import { BrandBackdrop, ProductTheater } from "../src/components/marketing";
import { useAuth } from "../src/lib/auth";
import { extractErrorMessage } from "../src/lib/errors";
import { formatMoney } from "../src/lib/format";
import { useDescribeIdeaMutation } from "../src/lib/gql/__generated__";
import type { HomeCopy } from "../src/lib/i18n/content";
import { useI18n } from "../src/lib/i18n/useI18n";

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
  const { copy } = useI18n();
  const homeCopy = copy.home;
  const { authenticated, loading: authLoading } = useAuth();
  const [describeIdea, { loading: launching }] = useDescribeIdeaMutation();
  const inputRef = useRef<HeroPromptInputHandle | null>(null);

  // Hero composer state. Owned by the page so chips + templates can
  // drop seeds and focus the textarea.
  const [prompt, setPrompt] = useState("");
  const [budgetUSD, setBudgetUSD] = useState<number | null>(null);
  const [planFirst, setPlanFirst] = useState(false);

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
        const project = result.data?.describeIdea?.project;
        const execution = result.data?.describeIdea?.execution;
        if (!project?.id) {
          // The resolver always populates project on success; landing
          // here usually means the server responded with errors that
          // were stripped above OR the network proxy returned an empty
          // body. Surface the raw response so the operator can debug.
          const debugDump = JSON.stringify(result.data ?? null).slice(0, 240);
          throw new Error(
            `Studio did not return a project id.\nResponse: ${debugDump}`,
          );
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
        if (isFunds) {
          const res = await swal.fire({
            icon: "warning",
            title: "Top up your wallet to launch",
            text: message,
            showCancelButton: true,
            confirmButtonText: "Open wallet",
            cancelButtonText: "Close",
          });
          if (res.isConfirmed) router.push("/wallet");
        } else {
          await swal.error("Could not start the build", message);
        }
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
        copy={homeCopy.hero}
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

      <SocialProofStrip />

      <MechanicsBlock />

      <ComparisonTable />

      <TemplatesGalleryPreview onPick={pickSeed} />

      <HomeFAQ />

      <FinalCTABand />

      <Footer copy={homeCopy.footer} />
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
  copy: HomeCopy["hero"];
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
        pt: { xs: 3, md: 4 },
        pb: { xs: 6, md: 7 },
        position: "relative",
        overflow: "hidden",
        minHeight: { md: "calc(100vh - 70px)" },
      }}
    >
      <BrandBackdrop />
      <Stack spacing={{ xs: 3.2, md: 4.5 }} sx={{ position: "relative" }}>
        <Stack spacing={1.6} alignItems="center">
          {props.welcomeOpen && (
            <WelcomeBanner
              onContinue={props.onWelcomeContinue}
              onDismiss={props.onWelcomeDismiss}
            />
          )}
          <Box sx={{ width: "100%", maxWidth: 880, mx: "auto" }}>
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
        </Stack>

        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", lg: "minmax(0, 0.88fr) minmax(420px, 0.84fr)" },
            gap: { xs: 4, lg: 6 },
            alignItems: "center",
          }}
        >
          <Stack spacing={{ xs: 2.4, md: 2.8 }} alignItems="flex-start" sx={{ textAlign: "left" }}>
            <Stack direction="row" spacing={1} alignItems="center" sx={pillSx}>
              <AutoAwesomeRounded sx={{ fontSize: 14 }} />
              <span>{props.copy.eyebrow}</span>
            </Stack>
            <Typography
              component="h1"
              sx={{
                color: tokens.color.text.primary,
                fontSize: { xs: 40, sm: 56, md: 72 },
                fontWeight: 900,
                letterSpacing: 0,
                lineHeight: 0.98,
                maxWidth: 680,
              }}
            >
              {props.copy.titleStart}{" "}
              <Box
                component="span"
                sx={{
                  backgroundImage: `linear-gradient(100deg, ${tokens.color.accent.coral}, ${tokens.color.brand.magenta} 52%, ${tokens.color.accent.violet})`,
                  WebkitBackgroundClip: "text",
                  WebkitTextFillColor: "transparent",
                }}
              >
                {props.copy.titleAccent}
              </Box>
              {props.copy.titleEnd && ` ${props.copy.titleEnd}`}
            </Typography>
            <Typography
              sx={{
                maxWidth: 610,
                color: tokens.color.text.secondary,
                fontSize: { xs: 14.5, md: 16 },
                lineHeight: 1.58,
              }}
            >
              {props.copy.subhead}
            </Typography>
            <Stack direction={{ xs: "column", sm: "row" }} spacing={1.3}>
              <Button
                component={Link}
                href="/signup"
                variant="contained"
                color="primary"
                endIcon={<ArrowForwardRounded sx={{ fontSize: 18 }} />}
              >
                Start building for free
              </Button>
              <Button component={Link} href="/templates" sx={{ color: tokens.color.accent.violet, fontWeight: 800 }}>
                See templates
              </Button>
            </Stack>

            <Stack
              direction="row"
              useFlexGap
              flexWrap="wrap"
              spacing={1}
              sx={{
                color: tokens.color.text.muted,
                fontSize: 12,
                fontFamily: tokens.font.mono,
                justifyContent: "flex-start",
              }}
            >
              {props.copy.proofChips.map((label, index) => {
                const icons = [
                  <AccountBalanceWalletOutlined key="wallet" sx={{ fontSize: 14 }} />,
                  <ShieldOutlined key="shield" sx={{ fontSize: 14 }} />,
                  <RuleFolderRounded key="patch" sx={{ fontSize: 14 }} />,
                  <VerifiedRounded key="verified" sx={{ fontSize: 14 }} />,
                ];
                return <HeroProofChip key={label} icon={icons[index] ?? icons[0]} label={label} />;
              })}
            </Stack>

            <Typography sx={{ fontFamily: tokens.font.mono, fontSize: 11, color: tokens.color.text.muted }}>
              {props.copy.launchNote}
            </Typography>
          </Stack>
          <Box sx={{ display: { xs: "none", lg: "block" } }}>
            <ProductTheater />
          </Box>
        </Box>
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

function ProofStrip({ items }: { items: HomeCopy["proof"] }) {
  // Five concrete mechanics. Numbers are monospace and feel real
  // (rate-sheet derived); they intentionally do not invent gates the
  // orchestrator does not run.
  const icons = [
    <RocketLaunchRounded key="rocket" sx={{ fontSize: 18 }} />,
    <TimelineRounded key="timeline" sx={{ fontSize: 18 }} />,
    <RuleFolderRounded key="patch" sx={{ fontSize: 18 }} />,
    <ShieldOutlined key="shield" sx={{ fontSize: 18 }} />,
    <AccountBalanceWalletOutlined key="wallet" sx={{ fontSize: 18 }} />,
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
          {items.map((it, index) => (
            <Stack key={it.label} spacing={0.6}>
              <Stack direction="row" alignItems="center" spacing={0.75} sx={{ color: tokens.color.accent.violet }}>
                {icons[index] ?? icons[0]}
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

function GuestTemplatesPreview({
  copy,
  onPick,
}: {
  copy: HomeCopy["templates"];
  onPick: (seed: string) => void;
}) {
  return (
    <Stack spacing={2.5}>
      <Stack direction="row" alignItems="baseline" justifyContent="space-between" useFlexGap flexWrap="wrap" sx={{ gap: 1 }}>
        <Typography sx={{ fontSize: 20, fontWeight: 800, letterSpacing: 0 }}>
          {copy.title}
        </Typography>
        <Button
          component={Link}
          href="/templates"
          size="small"
          endIcon={<ArrowForwardRounded sx={{ fontSize: 16 }} />}
          sx={{ color: tokens.color.accent.violet }}
        >
          {copy.cta}
        </Button>
      </Stack>
      <TemplatesGalleryPreview onPick={onPick} />
    </Stack>
  );
}

// ── How it works ────────────────────────────────────────────────────────

function HowItWorks({ copy }: { copy: HomeCopy["how"] }) {
  const icons = [<AutoAwesomeRounded key="idea" />, <RuleFolderRounded key="patch" />, <RocketLaunchRounded key="ship" />];
  return (
    <Section sx={{ py: { xs: 6, md: 8 } }}>
      <Stack spacing={1} sx={{ textAlign: "center", mb: { xs: 4, md: 5 } }}>
        <Typography sx={{ fontSize: { xs: 26, md: 32 }, fontWeight: 800, letterSpacing: 0 }}>
          {copy.title}
        </Typography>
        <Typography sx={{ color: tokens.color.text.secondary, fontSize: 14, maxWidth: 600, mx: "auto" }}>
          {copy.subhead}
        </Typography>
      </Stack>
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: { xs: "1fr", md: "repeat(3, minmax(0, 1fr))" },
          gap: 2,
        }}
      >
        {copy.steps.map((s, index) => (
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
                {icons[index] ?? icons[0]}
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
            <Typography sx={{ mt: 2, fontSize: 20, fontWeight: 800, letterSpacing: 0 }}>
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

function PricingTeaser({ copy }: { copy: HomeCopy["pricing"] }) {
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
              {copy.eyebrow}
            </Typography>
          </Stack>
          <Typography sx={{ mt: 1.5, fontSize: { xs: 26, md: 32 }, fontWeight: 800, letterSpacing: 0 }}>
            {copy.title}
          </Typography>
          <Typography sx={{ mt: 1.5, fontSize: 14, color: tokens.color.text.secondary, lineHeight: 1.6, maxWidth: 520 }}>
            {copy.body}
          </Typography>
          <Stack direction={{ xs: "column", sm: "row" }} spacing={1.5} sx={{ mt: 3 }}>
            <Button
              component={Link}
              href="/pricing"
              variant="contained"
              color="primary"
              endIcon={<ArrowForwardRounded sx={{ fontSize: 16 }} />}
            >
              {copy.primary}
            </Button>
            <Button
              component={Link}
              href="/signup"
              sx={{
                color: tokens.color.accent.violet,
                fontWeight: 700,
              }}
            >
              {copy.secondary}
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

function TrustedStrip() {
  const logos = ["Acme", "Vertex", "Sonic", "Cortex", "Pioneer", "Nimbus"];
  return (
    <Section sx={{ pt: { xs: 2, md: 3 }, pb: { xs: 5, md: 6 } }}>
      <Stack spacing={2.2} alignItems="center">
        <Typography
          sx={{
            fontFamily: tokens.font.mono,
            fontSize: 10.5,
            letterSpacing: 1.4,
            color: tokens.color.text.muted,
            textTransform: "uppercase",
          }}
        >
          Trusted by fast-moving teams
        </Typography>
        <Box
          sx={{
            width: "100%",
            display: "grid",
            gridTemplateColumns: { xs: "repeat(2, 1fr)", sm: "repeat(3, 1fr)", md: "repeat(6, 1fr)" },
            gap: 2,
            color: tokens.color.text.secondary,
          }}
        >
          {logos.map((logo) => (
            <Typography key={logo} sx={{ textAlign: "center", fontSize: 15, fontWeight: 900 }}>
              {logo}
            </Typography>
          ))}
        </Box>
      </Stack>
    </Section>
  );
}

function FlowPanel() {
  const steps = [
    { title: "Plan", body: "Turn a prompt into a structured product plan with roles, flows, data and acceptance criteria.", icon: <RuleFolderRounded /> },
    { title: "Build", body: "Generate a production-ready app with code, APIs, screens and a design system.", icon: <SettingsSuggestRounded /> },
    { title: "Review", body: "Test visually, review logic, track tasks and iterate with confidential AI feedback.", icon: <VisibilityRounded /> },
    { title: "Deploy", body: "One click deploys to staging or production with environments, logs and rollback.", icon: <RocketLaunchRounded /> },
  ];
  return (
    <Section sx={{ py: { xs: 5, md: 6 } }}>
      <Box
        sx={{
          position: "relative",
          p: { xs: 3, md: 4 },
          borderRadius: 2,
          border: `1px solid ${tokens.color.border.subtle}`,
          bgcolor: `${tokens.color.bg.surfaceRaised}d9`,
          overflow: "hidden",
          boxShadow: `0 26px 90px ${tokens.color.accent.purple}1c`,
        }}
      >
        <MiniPrism sx={{ right: { xs: 18, md: 42 }, top: { xs: 18, md: 28 } }} />
        <Stack spacing={1} alignItems="center" sx={{ mb: { xs: 3, md: 4 } }}>
          <Typography sx={{ fontSize: { xs: 25, md: 32 }, lineHeight: 1.05, fontWeight: 900, textAlign: "center" }}>
            From idea to launch in one flow
          </Typography>
          <Typography sx={{ color: tokens.color.text.secondary, fontSize: 13.5, textAlign: "center" }}>
            Plan with clarity. Build with speed. Ship with confidence.
          </Typography>
        </Stack>
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", sm: "repeat(2, 1fr)", md: "repeat(4, 1fr)" },
            gap: { xs: 2, md: 2.5 },
          }}
        >
          {steps.map((step) => (
            <Stack key={step.title} spacing={1.2} alignItems="center" sx={{ textAlign: "center", minWidth: 0 }}>
              <Box
                sx={{
                  width: 44,
                  height: 44,
                  borderRadius: 1,
                  display: "grid",
                  placeItems: "center",
                  color: tokens.color.accent.violet,
                  bgcolor: `${tokens.color.accent.violet}19`,
                  border: `1px solid ${tokens.color.accent.violet}3d`,
                  "& svg": { fontSize: 21 },
                }}
              >
                {step.icon}
              </Box>
              <Typography sx={{ fontSize: 13.5, fontWeight: 900 }}>{step.title}</Typography>
              <Typography sx={{ color: tokens.color.text.secondary, fontSize: 12, lineHeight: 1.55, maxWidth: 210 }}>
                {step.body}
              </Typography>
            </Stack>
          ))}
        </Box>
      </Box>
    </Section>
  );
}

function CapabilityGrid() {
  const capabilities = [
    { title: "AI Product Architect", body: "Understands your goal and creates a complete app plan.", icon: <AutoAwesomeRounded /> },
    { title: "Visual App Builder", body: "Generate responsive screens with a modern design system.", icon: <VisibilityRounded /> },
    { title: "Code You Own", body: "Export clean production-ready React and TypeScript.", icon: <CodeRounded /> },
    { title: "Data & Integrations", body: "Models, APIs, auth, storage and third-party connectors.", icon: <DataObjectRounded /> },
    { title: "Team & Roles", body: "Invite teammates, set roles and manage access.", icon: <HubRounded /> },
    { title: "Environments", body: "Dev, staging and prod with secrets and config.", icon: <SettingsSuggestRounded /> },
    { title: "Observability", body: "Logs, traces, metrics and error tracking by default.", icon: <TimelineRounded /> },
    { title: "Enterprise Ready", body: "SSO, audit logs, RBAC and isolated backends.", icon: <SecurityRounded /> },
  ];
  return (
    <Section sx={{ py: { xs: 5, md: 6 } }}>
      <Stack spacing={3}>
        <Typography sx={{ fontSize: { xs: 25, md: 32 }, fontWeight: 900, textAlign: "center" }}>
          Everything you need to build and ship
        </Typography>
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", sm: "repeat(2, 1fr)", lg: "repeat(4, 1fr)" },
            gap: 1.6,
          }}
        >
          {capabilities.map((item) => (
            <Box
              key={item.title}
              sx={{
                p: 2.2,
                minHeight: 118,
                borderRadius: 1,
                border: `1px solid ${tokens.color.border.subtle}`,
                bgcolor: `${tokens.color.bg.surfaceRaised}d9`,
                transition: `transform ${tokens.motion.fast} ${tokens.motion.snap}, border-color ${tokens.motion.fast} ${tokens.motion.snap}`,
                "&:hover": {
                  transform: "translateY(-3px)",
                  borderColor: tokens.color.border.strong,
                },
              }}
            >
              <Stack direction="row" spacing={1.1} alignItems="center">
                <Box sx={{ color: tokens.color.accent.violet, display: "grid", "& svg": { fontSize: 18 } }}>{item.icon}</Box>
                <Typography sx={{ fontSize: 13.5, fontWeight: 900 }}>{item.title}</Typography>
              </Stack>
              <Typography sx={{ mt: 1, fontSize: 12, lineHeight: 1.5, color: tokens.color.text.secondary }}>
                {item.body}
              </Typography>
            </Box>
          ))}
        </Box>
      </Stack>
    </Section>
  );
}

function TemplateShowcase({ onPick }: { onPick: (seed: string) => void }) {
  const templates = [
    ["SaaS Starter", "Auth, billing, team settings", "92", "Live billing"],
    ["Client Portal", "Projects, files, approvals", "88", "Approvals queue"],
    ["Marketplace", "Listings, search, checkout", "94", "Order flow"],
    ["Internal Tool", "Workflows, approvals, reports", "91", "Ops cockpit"],
    ["Education App", "Lessons, progress, analytics", "86", "Progress map"],
  ];
  return (
    <Section sx={{ py: { xs: 5, md: 6 } }}>
      <Box
        sx={{
          p: { xs: 2.4, md: 3 },
          borderRadius: 2,
          border: `1px solid ${tokens.color.border.subtle}`,
          bgcolor: `${tokens.color.bg.surfaceRaised}d4`,
        }}
      >
        <Stack direction="row" alignItems="center" justifyContent="space-between" useFlexGap flexWrap="wrap" sx={{ gap: 1.5, mb: 2 }}>
          <Box>
            <Typography sx={{ fontSize: 20, fontWeight: 900 }}>Start from a proven template</Typography>
            <Typography sx={{ mt: 0.5, color: tokens.color.text.secondary, fontSize: 13 }}>
              Pre-built foundations for common product patterns.
            </Typography>
          </Box>
          <Button component={Link} href="/templates" size="small" endIcon={<ArrowForwardRounded sx={{ fontSize: 16 }} />} sx={{ color: tokens.color.accent.violet }}>
            Browse all templates
          </Button>
        </Stack>
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", sm: "repeat(2, 1fr)", lg: "repeat(5, 1fr)" },
            gap: 1.4,
          }}
        >
          {templates.map(([title, body, score, seed]) => (
            <Box
              key={title}
              onClick={() => onPick(`Build a ${title.toLowerCase()} with ${body.toLowerCase()}, admin dashboard, roles, payments and deploy-ready code.`)}
              sx={{
                cursor: "pointer",
                borderRadius: 1,
                border: `1px solid ${tokens.color.border.subtle}`,
                bgcolor: tokens.color.bg.surface,
                overflow: "hidden",
                transition: `transform ${tokens.motion.fast} ${tokens.motion.snap}, border-color ${tokens.motion.fast} ${tokens.motion.snap}`,
                "&:hover": { transform: "translateY(-3px)", borderColor: tokens.color.border.strong },
              }}
            >
              <Box sx={{ p: 1, bgcolor: `${tokens.color.accent.purple}24` }}>
                <Box sx={{ height: 70, borderRadius: 1, bgcolor: tokens.color.bg.inset, p: 1 }}>
                  <Stack direction="row" spacing={0.5} sx={{ mb: 1 }}>
                    {[0, 1, 2].map((dot) => (
                      <Box key={dot} sx={{ width: 6, height: 6, borderRadius: "50%", bgcolor: tokens.color.text.muted }} />
                    ))}
                  </Stack>
                  <Box sx={{ width: "68%", height: 12, borderRadius: 0.7, bgcolor: `${tokens.color.accent.violet}80`, mb: 0.8 }} />
                  <Box sx={{ width: "52%", height: 12, borderRadius: 0.7, bgcolor: `${tokens.color.accent.violet}55` }} />
                  <Typography sx={{ float: "right", mt: -2.5, mr: 1, color: tokens.color.accent.violet, fontFamily: tokens.font.mono, fontWeight: 900, fontSize: 14 }}>
                    {score}
                  </Typography>
                </Box>
              </Box>
              <Box sx={{ p: 1.4 }}>
                <Typography sx={{ fontSize: 13, fontWeight: 900 }}>{title}</Typography>
                <Typography sx={{ mt: 0.5, fontSize: 11.5, color: tokens.color.text.secondary }}>{body}</Typography>
                <Typography sx={{ mt: 1, fontSize: 10.5, color: tokens.color.text.muted, fontFamily: tokens.font.mono }}>{seed}</Typography>
              </Box>
            </Box>
          ))}
        </Box>
      </Box>
    </Section>
  );
}

function TestimonialBand() {
  return (
    <Section sx={{ py: { xs: 5, md: 6 } }}>
      <Box
        sx={{
          position: "relative",
          p: { xs: 3, md: 4 },
          borderRadius: 2,
          border: `1px solid ${tokens.color.accent.violet}80`,
          bgcolor: `${tokens.color.bg.surfaceRaised}cf`,
          overflow: "hidden",
          boxShadow: `0 18px 80px ${tokens.color.accent.purple}20`,
        }}
      >
        <MiniPrism sx={{ right: 30, bottom: 22 }} />
        <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", md: "1.6fr 1fr" }, gap: 3, alignItems: "center" }}>
          <Box>
            <Typography sx={{ fontFamily: tokens.font.mono, fontSize: 10.5, color: tokens.color.accent.violet, fontWeight: 800, textTransform: "uppercase" }}>
              How teams build faster
            </Typography>
            <Typography sx={{ mt: 1, maxWidth: 690, fontSize: { xs: 24, md: 32 }, lineHeight: 1.08, fontWeight: 900 }}>
              "We shipped our client portal in a week with Ironflyer. The AI plan was spot-on and the code was clean and easy to extend."
            </Typography>
          </Box>
          <Box sx={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 2 }}>
            {[
              ["7 days", "To production"],
              ["92%", "Code kept"],
              ["3x", "Faster delivery"],
            ].map(([value, label]) => (
              <Stack key={label} spacing={0.5}>
                <Typography sx={{ color: tokens.color.accent.violet, fontSize: 28, fontWeight: 900 }}>{value}</Typography>
                <Typography sx={{ color: tokens.color.text.secondary, fontSize: 12 }}>{label}</Typography>
              </Stack>
            ))}
          </Box>
        </Box>
      </Box>
    </Section>
  );
}

function PricingCards() {
  const plans = [
    ["Free", "$0", "Forever", ["1 workspace", "2 projects", "Community support"], "Get started"],
    ["Pro", "$29", "Per user / month", ["Unlimited projects", "AI templates", "Email support"], "Start free trial"],
    ["Team", "$79", "Per user / month", ["SSO & RBAC", "Environments", "Priority support"], "Start free trial"],
    ["Enterprise", "Custom", "Let's talk", ["Advanced security", "SLA & support", "Custom integrations"], "Contact sales"],
  ];
  return (
    <Section sx={{ py: { xs: 5, md: 7 } }}>
      <Stack spacing={3} alignItems="center">
        <Stack spacing={1} alignItems="center">
          <Typography sx={{ fontSize: { xs: 25, md: 32 }, fontWeight: 900 }}>Simple, transparent pricing</Typography>
          <Typography sx={{ color: tokens.color.text.secondary, fontSize: 13 }}>Start free. Scale on your terms.</Typography>
        </Stack>
        <Box sx={{ width: "100%", maxWidth: 900, display: "grid", gridTemplateColumns: { xs: "1fr", sm: "repeat(2, 1fr)", lg: "repeat(4, 1fr)" }, gap: 1.5 }}>
          {plans.map(([name, price, cadence, features, cta], index) => (
            <Box
              key={name as string}
              sx={{
                position: "relative",
                p: 2.2,
                borderRadius: 1,
                border: `1px solid ${index === 2 ? tokens.color.accent.violet : tokens.color.border.subtle}`,
                bgcolor: `${tokens.color.bg.surfaceRaised}e0`,
              }}
            >
              {index === 2 && (
                <Box sx={{ position: "absolute", right: 12, top: 12, px: 0.8, py: 0.25, borderRadius: 999, bgcolor: tokens.color.accent.violet, fontSize: 10, fontWeight: 900 }}>
                  Most popular
                </Box>
              )}
              <Typography sx={{ fontSize: 12, fontWeight: 900 }}>{name}</Typography>
              <Typography sx={{ mt: 1.4, fontSize: price === "Custom" ? 29 : 34, fontWeight: 900, lineHeight: 1 }}>{price}</Typography>
              <Typography sx={{ mt: 0.7, color: tokens.color.text.secondary, fontSize: 11 }}>{cadence}</Typography>
              <Stack spacing={0.7} sx={{ my: 2.2 }}>
                {(features as string[]).map((feature) => (
                  <Typography key={feature} sx={{ color: tokens.color.text.secondary, fontSize: 12 }}>
                    - {feature}
                  </Typography>
                ))}
              </Stack>
              <Button component={Link} href="/signup" fullWidth variant={index === 2 ? "contained" : "text"} color="primary" sx={{ bgcolor: index === 2 ? undefined : `${tokens.color.accent.purple}1f` }}>
                {cta}
              </Button>
            </Box>
          ))}
        </Box>
      </Stack>
    </Section>
  );
}

function ProofFooterBand() {
  const rows = [
    ["Build in natural language", "Shorten the gap from idea to working product."],
    ["Ship with confidence", "Built-in reviews, tests and observability."],
    ["Own your code", "Export anytime. You are never locked in."],
    ["Secure by default", "Enterprise-grade security and compliance."],
  ];
  return (
    <Section sx={{ py: { xs: 3, md: 5 } }}>
      <Box sx={{ p: 2.2, borderRadius: 2, border: `1px solid ${tokens.color.border.subtle}`, bgcolor: `${tokens.color.bg.surfaceRaised}d0`, display: "grid", gridTemplateColumns: { xs: "1fr", sm: "repeat(2, 1fr)", lg: "repeat(4, 1fr)" }, gap: 2 }}>
        {rows.map(([title, body]) => (
          <Stack key={title} direction="row" spacing={1.2}>
            <BoltRounded sx={{ fontSize: 16, color: tokens.color.accent.violet, mt: 0.25 }} />
            <Box>
              <Typography sx={{ fontSize: 12.5, fontWeight: 900 }}>{title}</Typography>
              <Typography sx={{ mt: 0.4, fontSize: 11.5, color: tokens.color.text.secondary }}>{body}</Typography>
            </Box>
          </Stack>
        ))}
      </Box>
    </Section>
  );
}

function FaqShowcase() {
  const questions = ["Can I export the code?", "How does pricing work?", "Is my data secure?", "Do you offer onboarding?"];
  return (
    <Section sx={{ py: { xs: 5, md: 7 } }}>
      <Stack spacing={3}>
        <Typography sx={{ textAlign: "center", fontSize: { xs: 25, md: 32 }, fontWeight: 900 }}>
          Frequently asked questions
        </Typography>
        <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" }, gap: 2 }}>
          <Stack spacing={1.1}>
            {questions.map((question) => (
              <Box key={question} sx={{ p: 2, borderRadius: 1, border: `1px solid ${tokens.color.border.subtle}`, bgcolor: `${tokens.color.bg.surfaceRaised}d9`, display: "flex", justifyContent: "space-between", gap: 2 }}>
                <Typography sx={{ fontSize: 13, fontWeight: 900 }}>{question}</Typography>
                <Typography sx={{ color: tokens.color.text.secondary }}>+</Typography>
              </Box>
            ))}
          </Stack>
          <Box sx={{ p: 2, borderRadius: 1, border: `1px solid ${tokens.color.border.subtle}`, bgcolor: tokens.color.bg.inset, position: "relative", overflow: "hidden" }}>
            <MiniPrism sx={{ right: 22, bottom: 18 }} />
            <Stack direction="row" justifyContent="space-between" sx={{ mb: 2, fontFamily: tokens.font.mono, color: tokens.color.text.muted, fontSize: 11 }}>
              <span>App.tsx</span>
              <span>schema.prisma</span>
            </Stack>
            <Typography component="pre" sx={{ m: 0, p: 0, border: 0, bgcolor: `${tokens.color.bg.base}00`, color: tokens.color.text.secondary, fontFamily: tokens.font.mono, fontSize: 12, lineHeight: 1.7, whiteSpace: "pre-wrap" }}>
{`export default function Dashboard() {
  return (
    <section className="app">
      <Hero />
      <StatsGrid />
      <ProjectsTable />
    </section>
  );
}`}
            </Typography>
          </Box>
        </Box>
      </Stack>
    </Section>
  );
}

function FinalShipCTA() {
  return (
    <Section sx={{ py: { xs: 5, md: 7 } }}>
      <Box sx={{ p: { xs: 2, md: 3 }, borderRadius: 2, border: `1px solid ${tokens.color.border.subtle}`, bgcolor: `${tokens.color.accent.purple}2e`, display: "grid", gridTemplateColumns: { xs: "1fr", md: "160px 1fr auto" }, gap: { xs: 2, md: 3 }, alignItems: "center" }}>
        <Box sx={{ height: 98, borderRadius: 1, backgroundImage: "url('/market/data-flow.jpg')", backgroundSize: "cover", backgroundPosition: "center", border: `1px solid ${tokens.color.border.subtle}` }} />
        <Box>
          <Typography sx={{ fontSize: { xs: 22, md: 28 }, fontWeight: 900 }}>Stop stitching tools. Start shipping products.</Typography>
          <Typography sx={{ mt: 0.6, color: tokens.color.text.secondary, fontSize: 13 }}>One prompt. One workspace. One launch.</Typography>
        </Box>
        <Stack direction={{ xs: "column", sm: "row" }} spacing={1.2}>
          <Button component={Link} href="/signup" variant="contained" color="primary" endIcon={<ArrowForwardRounded sx={{ fontSize: 16 }} />}>
            Start building for free
          </Button>
          <Button component={Link} href="/enterprise" sx={{ color: tokens.color.text.primary, fontWeight: 800 }}>
            Talk to sales
          </Button>
        </Stack>
      </Box>
    </Section>
  );
}

function MiniPrism({ sx }: { sx?: object }) {
  return (
    <Box
      aria-hidden
      sx={[
        {
          position: "absolute",
          width: 54,
          aspectRatio: "1 / 1",
          borderRadius: 1,
          transform: "rotateX(58deg) rotateZ(44deg)",
          background: `linear-gradient(135deg, ${tokens.color.accent.violet}66, ${tokens.color.bg.surfaceRaised}22)`,
          border: `1px solid ${tokens.color.accent.violet}66`,
          boxShadow: `0 0 34px ${tokens.color.accent.violet}45, inset 0 0 24px ${tokens.color.accent.violet}2e`,
          pointerEvents: "none",
        },
        sx ?? {},
      ]}
    />
  );
}

// ── Footer ──────────────────────────────────────────────────────────────

function Footer({ copy }: { copy: HomeCopy["footer"] }) {
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
            {copy.body}
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
        {copy.copyright}
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
