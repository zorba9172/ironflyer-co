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
  ArrowBackRounded,
  ArrowForwardRounded,
  AutoAwesomeRounded,
  BoltRounded,
  CheckRounded,
  CheckCircleRounded,
  CodeRounded,
  DataObjectRounded,
  ExpandMoreRounded,
  GitHub,
  HubRounded,
  LayersRounded,
  MailOutlineRounded,
  MonitorHeartOutlined,
  RocketLaunchRounded,
  RuleFolderRounded,
  SecurityRounded,
  SendRounded,
  SettingsSuggestRounded,
  ShieldOutlined,
  StorageRounded,
  TimelineRounded,
  VerifiedRounded,
  VisibilityRounded,
} from "@mui/icons-material";
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Button,
  Card,
  Stack,
  Typography,
} from "@mui/material";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import {
  Suspense,
  useCallback,
  useEffect,
  useRef,
  useState,
  type MutableRefObject,
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
import { TemplatesGalleryPreview } from "../src/components/home/TemplatesGalleryPreview";
import { BrandBackdrop, ProductTheater } from "../src/components/marketing";
import { useAuth } from "../src/lib/auth";
import { extractErrorMessage } from "../src/lib/errors";
import { formatMoney } from "../src/lib/format";
import { useDescribeIdeaMutation } from "../src/lib/gql/__generated__";
import type { HomeCopy } from "../src/lib/i18n/content";
import { useI18n } from "../src/lib/i18n/useI18n";
import { Autoplay, Navigation } from "swiper/modules";
import { Swiper, SwiperSlide } from "swiper/react";

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
  const [prompt, setPrompt] = useState(
    "CRM for contacts, deals, kanban, notes and follow-ups.",
  );
  const [budgetUSD, setBudgetUSD] = useState<number | null>(27);
  const [planFirst, setPlanFirst] = useState(true);

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
        router.push(`/signup?redirect=${encodeURIComponent("/studio")}`);
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
        router.push(
          `/p/${encodeURIComponent(project.id)}?${params.toString()}`,
        );
      } catch (err) {
        const message = extractErrorMessage(err);
        const isFunds =
          /payment.required|insufficient|wallet|top.?up|budget/i.test(message);
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

  const timing = search?.get("theme") === "dark" ? "dark" : "light";

  return (
    <Box
      data-home-timing={timing}
      sx={{
        position: "relative",
        width: "100%",
        minWidth: 0,
        overflow: "clip",
        bgcolor: timing === "light" ? "#fbfaff" : tokens.color.bg.base,
      }}
    >
      <Hero
        timing={timing}
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

      <Box
        sx={{
          color: timing === "light" ? "#090d3d" : tokens.color.text.primary,
          bgcolor: timing === "light" ? "#fbfaff" : tokens.color.bg.base,
          backgroundImage:
            timing === "light"
              ? "radial-gradient(820px 420px at 92% 12%, rgba(226,69,205,0.08), transparent 70%), radial-gradient(720px 360px at 6% 38%, rgba(127,77,255,0.07), transparent 72%)"
              : "radial-gradient(820px 420px at 92% 12%, rgba(145,75,255,0.13), transparent 70%), radial-gradient(720px 360px at 6% 38%, rgba(39,134,255,0.07), transparent 72%)",
        }}
      >
        <FlowPanel timing={timing} />

        <CapabilityGrid timing={timing} />

        <TemplateShowcase timing={timing} onPick={pickSeed} />

        <VscodeExtensionBand timing={timing} />

        <TestimonialBand timing={timing} />

        <PricingCards timing={timing} />

        <ProofFooterBand timing={timing} />

        <FaqShowcase timing={timing} />

        <FinalShipCTA timing={timing} />

        <Footer copy={homeCopy.footer} />
      </Box>
    </Box>
  );
}

// ── Layout primitive ────────────────────────────────────────────────────

function Section({ children, sx }: { children: ReactNode; sx?: object }) {
  return (
    <Box sx={[{ width: "100%", minWidth: 0, px: { xs: 2, md: 4 } }, sx ?? {}]}>
      <Box sx={{ maxWidth: 1280, minWidth: 0, mx: "auto" }}>{children}</Box>
    </Box>
  );
}

function homeTone(timing: OrbitalTiming) {
  const light = timing === "light";
  return {
    light,
    text: light ? "#080b3f" : tokens.color.text.primary,
    secondary: light ? "#5e6689" : tokens.color.text.secondary,
    muted: light ? "#858ca8" : tokens.color.text.muted,
    border: light ? "rgba(127,77,255,0.18)" : tokens.color.border.subtle,
    strong: light ? "rgba(127,77,255,0.30)" : tokens.color.border.strong,
    surface: light
      ? "rgba(255,255,255,0.78)"
      : `${tokens.color.bg.surfaceRaised}d9`,
    surfaceStrong: light
      ? "rgba(255,255,255,0.92)"
      : tokens.color.bg.surfaceRaised,
    inset: light ? "rgba(248,246,255,0.92)" : tokens.color.bg.inset,
  };
}

// ── 2026-05-27 Orbital Home ────────────────────────────────────────────

type OrbitalTiming = "dark" | "light";

interface OrbitalPalette {
  mode: OrbitalTiming;
  bg: string;
  surface: string;
  surface2: string;
  inset: string;
  text: string;
  secondary: string;
  muted: string;
  border: string;
  strong: string;
  glow: string;
  purple: string;
  pink: string;
  orange: string;
  blue: string;
  cardShadow: string;
}

function orbitalPalette(mode: OrbitalTiming): OrbitalPalette {
  if (mode === "light") {
    return {
      mode,
      bg: "#fbfaff",
      surface: "rgba(255,255,255,0.88)",
      surface2: "rgba(255,255,255,0.74)",
      inset: "#ffffff",
      text: "#0b1040",
      secondary: "#46507d",
      muted: "#7e84a7",
      border: "rgba(112,77,255,0.16)",
      strong: "rgba(147,76,255,0.32)",
      glow: "rgba(158,77,255,0.18)",
      purple: "#8a35ff",
      pink: "#ee46c8",
      orange: "#ff6259",
      blue: "#3b82ff",
      cardShadow: "0 26px 80px rgba(70,51,160,0.12)",
    };
  }
  return {
    mode,
    bg: "#030614",
    surface: "rgba(13,16,44,0.78)",
    surface2: "rgba(16,20,56,0.66)",
    inset: "rgba(6,8,26,0.82)",
    text: "#ffffff",
    secondary: "#c9cdf5",
    muted: "#858bb5",
    border: "rgba(139,107,255,0.24)",
    strong: "rgba(184,82,255,0.58)",
    glow: "rgba(163,59,255,0.42)",
    purple: "#a73dff",
    pink: "#f03bce",
    orange: "#ff6b4a",
    blue: "#1fb6ff",
    cardShadow: "0 26px 100px rgba(123,42,255,0.28)",
  };
}

function gradient(t: OrbitalPalette) {
  return `linear-gradient(100deg, ${t.orange}, ${t.pink} 52%, ${t.purple})`;
}

function OrbitalHome({
  timing,
  copy,
  prompt,
  onPromptChange,
  onSubmit,
  submitting,
  onPickSeed,
  inputRef,
  welcomeOpen,
  onWelcomeContinue,
  onWelcomeDismiss,
}: {
  timing: OrbitalTiming;
  copy: HomeCopy;
  prompt: string;
  onPromptChange: (next: string) => void;
  onSubmit: (payload: HeroPromptSubmitPayload) => void;
  submitting: boolean;
  onPickSeed: (seed: string) => void;
  inputRef: MutableRefObject<HeroPromptInputHandle | null>;
  welcomeOpen: boolean;
  onWelcomeContinue: () => void;
  onWelcomeDismiss: () => void;
}) {
  const t = orbitalPalette(timing);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);

  useEffect(() => {
    inputRef.current = {
      focus: () => textareaRef.current?.focus(),
    };
    return () => {
      inputRef.current = null;
    };
  }, [inputRef]);

  const submitPrompt = useCallback(() => {
    if (prompt.trim().length < 8 || submitting) return;
    onSubmit({ text: prompt.trim(), budgetUSD: null, planFirst: false });
  }, [onSubmit, prompt, submitting]);

  return (
    <Box
      data-home-timing={timing}
      sx={{
        minHeight: "100vh",
        overflow: "clip",
        color: t.text,
        bgcolor: t.bg,
        backgroundImage:
          timing === "light"
            ? [
                `radial-gradient(ellipse 760px 460px at 80% 2%, rgba(226,69,205,0.13), transparent 72%)`,
                `radial-gradient(ellipse 780px 420px at 10% 14%, rgba(125,74,255,0.10), transparent 70%)`,
                `radial-gradient(circle at 1px 1px, rgba(132,72,255,0.14) 1px, transparent 1.6px)`,
              ].join(", ")
            : [
                `radial-gradient(ellipse 860px 520px at 78% 4%, rgba(128,63,255,0.33), transparent 72%)`,
                `radial-gradient(ellipse 700px 380px at 4% 15%, rgba(0,128,255,0.12), transparent 70%)`,
                `radial-gradient(circle at 1px 1px, rgba(126,183,255,0.22) 1px, transparent 1.6px)`,
              ].join(", "),
        backgroundSize: "auto, auto, 32px 32px",
      }}
    >
      <Box
        sx={{
          maxWidth: 1220,
          mx: "auto",
          px: { xs: 2, md: 2.4 },
          pb: { xs: 4, md: timing === "dark" ? 1 : 3 },
        }}
      >
        <OrbitalNav t={t} />

        <Box
          component="section"
          sx={{
            position: "relative",
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "0.84fr 1.16fr" },
            gap: { xs: 3, md: 3.5 },
            alignItems: "center",
            minHeight: { xs: "auto", md: 258 },
            pt: { xs: 2, lg: 2 },
          }}
        >
          <HeroCopy t={t} timing={timing} copy={copy.hero} />
          <OrbitalVisual t={t} />
        </Box>

        {welcomeOpen && (
          <Box sx={{ mt: 2 }}>
            <WelcomeBanner
              onContinue={onWelcomeContinue}
              onDismiss={onWelcomeDismiss}
            />
          </Box>
        )}

        <OrbitalBuilderPanel
          t={t}
          prompt={prompt}
          onPromptChange={onPromptChange}
          onSubmit={submitPrompt}
          submitting={submitting}
          textareaRef={textareaRef}
          onPickSeed={onPickSeed}
        />

        <OrbitalCapabilities t={t} />
        <OrbitalTestimonial t={t} />
        <OrbitalTemplates t={t} onPickSeed={onPickSeed} />
        <OrbitalPricingFaq t={t} />
        <OrbitalFinalCta t={t} />
        {timing === "light" && <OrbitalFooter t={t} copy={copy.footer} />}
      </Box>
    </Box>
  );
}

function OrbitalNav({ t }: { t: OrbitalPalette }) {
  const links = [
    "Product",
    "Solutions",
    "Templates",
    "Pricing",
    "Resources",
    "Enterprise",
  ];
  return (
    <Stack
      component="nav"
      direction="row"
      alignItems="center"
      sx={{
        height: { xs: 58, md: 56 },
        gap: { xs: 1, md: 3 },
        color: t.text,
      }}
    >
      <BrandLogo inverse={t.mode === "dark"} size={26} href="/" />
      <Stack
        direction="row"
        spacing={2.3}
        sx={{ display: { xs: "none", md: "flex" }, ml: 2.6 }}
      >
        {links.map((label) => (
          <Box
            key={label}
            component={Link}
            href={`/${label.toLowerCase() === "product" ? "product" : label.toLowerCase()}`}
            sx={{
              color: t.text,
              opacity: 0.92,
              fontSize: 12.4,
              fontWeight: 800,
              "&:hover": { color: t.pink },
            }}
          >
            {label}
            {label === "Solutions" || label === "Resources" ? "⌄" : ""}
          </Box>
        ))}
      </Stack>
      <Box sx={{ flex: 1 }} />
      <Button
        component={Link}
        href="/login"
        size="small"
        sx={{
          color: t.text,
          bgcolor:
            t.mode === "dark"
              ? "rgba(38,42,100,0.62)"
              : "rgba(255,255,255,0.58)",
          border: `1px solid ${t.border}`,
          minHeight: 34,
          display: { xs: "none", sm: "inline-flex" },
        }}
      >
        Log in
      </Button>
      <Button
        component={Link}
        href="/signup"
        variant="contained"
        endIcon={<ArrowForwardRounded sx={{ fontSize: 16 }} />}
        sx={{
          ml: 1,
          minHeight: 38,
          px: { xs: 1.6, md: 2 },
          background: gradient(t),
          color: "#fff",
          fontWeight: 900,
          borderRadius: 1.5,
          boxShadow: `0 12px 34px ${t.glow}`,
        }}
      >
        Start a project free
      </Button>
    </Stack>
  );
}

function HeroCopy({
  t,
  copy,
  timing,
}: {
  t: OrbitalPalette;
  copy: HomeCopy["hero"];
  timing: OrbitalTiming;
}) {
  return (
    <Stack spacing={2.4} sx={{ pt: { xs: 2, lg: 0 }, maxWidth: 570 }}>
      <Box sx={orbitalPillSx(t)}>
        <AutoAwesomeRounded sx={{ fontSize: 15 }} />
        AI-powered product builder
      </Box>
      <Typography
        component="h1"
        sx={{
          fontSize: { xs: 38, md: 35 },
          lineHeight: 1.04,
          fontWeight: 900,
          letterSpacing: 0,
          color: t.text,
          textShadow:
            timing === "dark" ? "0 10px 38px rgba(0,0,0,0.45)" : "none",
        }}
      >
        Build, review and
        <br />
        ship production apps
        <br />
        from a{" "}
        <Box
          component="span"
          sx={{
            backgroundImage: gradient(t),
            WebkitBackgroundClip: "text",
            WebkitTextFillColor: "transparent",
          }}
        >
          single prompt.
        </Box>
      </Typography>
      <Stack direction={{ xs: "column", sm: "row" }} spacing={2}>
        <Button
          component={Link}
          href="/signup"
          variant="contained"
          endIcon={<ArrowForwardRounded sx={{ fontSize: 18 }} />}
          sx={{
            minHeight: 44,
            px: 2.4,
            background: gradient(t),
            color: "#fff",
            borderRadius: 1.3,
            fontWeight: 900,
          }}
        >
          Start building for free
        </Button>
        <Button
          component={Link}
          href="/templates"
          variant="outlined"
          sx={{
            minHeight: 44,
            px: 2.4,
            color: t.text,
            borderColor: t.strong,
            bgcolor:
              t.mode === "light" ? "rgba(255,255,255,0.55)" : "transparent",
            fontWeight: 900,
          }}
        >
          See templates
        </Button>
      </Stack>
      <Stack
        direction="row"
        useFlexGap
        flexWrap="nowrap"
        gap={1.4}
        sx={{ overflow: "hidden" }}
      >
        {[
          "No credit card required",
          "Setup in 30 seconds",
          "SOC 2 compliant",
          "GDPR ready",
        ].map((item) => (
          <Stack key={item} direction="row" spacing={0.6} alignItems="center">
            <CheckCircleRounded
              sx={{
                fontSize: 13,
                color: t.mode === "light" ? t.purple : "#dfe4ff",
              }}
            />
            <Typography
              sx={{
                color: t.secondary,
                fontSize: 9.5,
                fontWeight: 800,
                whiteSpace: "nowrap",
              }}
            >
              {item}
            </Typography>
          </Stack>
        ))}
      </Stack>
      {t.mode === "light" && (
        <Stack spacing={1.4} sx={{ pt: 1 }}>
          <Typography sx={{ color: t.muted, fontSize: 12 }}>
            Trusted by modern teams worldwide
          </Typography>
          <Stack
            direction="row"
            useFlexGap
            flexWrap="wrap"
            gap={3.2}
            sx={{
              color: t.secondary,
              fontSize: 18,
              fontWeight: 800,
              opacity: 0.8,
            }}
          >
            {["Google", "Microsoft", "airbnb", "amazon", "Spotify"].map(
              (name) => (
                <span key={name}>{name}</span>
              ),
            )}
          </Stack>
        </Stack>
      )}
    </Stack>
  );
}

function OrbitalVisual({ t }: { t: OrbitalPalette }) {
  const icons = [
    <CodeRounded key="code" />,
    <CheckRounded key="check" />,
    <DataObjectRounded key="data" />,
    <RocketLaunchRounded key="rocket" />,
    <SecurityRounded key="security" />,
    <HubRounded key="hub" />,
  ];
  return (
    <Box
      aria-hidden
      sx={{
        position: "relative",
        height: { xs: 420, md: 276 },
        display: { xs: "none", md: "block" },
        perspective: "1200px",
      }}
    >
      <Box
        sx={{
          position: "absolute",
          inset: "6% 2% 0 4%",
          borderRadius: "50%",
          background: `radial-gradient(circle at 50% 58%, ${t.purple}66, transparent 26%), radial-gradient(circle at 50% 62%, ${t.blue}55, transparent 14%)`,
          filter: "blur(0.2px)",
        }}
      />
      <Box sx={orbitSx(t, 18, 44, 112, 0)} />
      <Box sx={orbitSx(t, 42, 42, 92, -18)} />
      <Box
        sx={{
          position: "absolute",
          left: "36%",
          top: "47%",
          width: 138,
          height: 138,
          borderRadius: "50%",
          background: `radial-gradient(circle at 34% 25%, #fff8, ${t.blue}66 13%, ${t.purple}8a 42%, ${t.pink}a3 68%, ${t.bg} 100%)`,
          boxShadow: `0 0 44px ${t.blue}99, 0 0 86px ${t.pink}8a, inset -28px -32px 44px rgba(7,10,38,0.74)`,
        }}
      />
      <Box
        sx={{
          position: "absolute",
          left: "36%",
          top: "47%",
          width: 166,
          height: 36,
          borderRadius: "50%",
          borderTop: `5px solid ${t.blue}`,
          borderBottom: `5px solid ${t.pink}`,
          transform: "rotate(-3deg)",
          filter: "drop-shadow(0 0 18px #a93dff)",
        }}
      />
      {[0, 1, 2].map((idx) => (
        <Box
          key={idx}
          sx={{
            position: "absolute",
            left: `${31 + idx * 10}%`,
            top: `${20 - idx * 4}%`,
            width: idx === 1 ? 108 : 82,
            height: idx === 1 ? 120 : 96,
            borderRadius: 3,
            border: `1px solid ${idx === 1 ? t.pink : t.strong}`,
            bgcolor:
              t.mode === "light"
                ? "rgba(255,255,255,0.72)"
                : "rgba(13,15,55,0.76)",
            boxShadow: `0 0 38px ${idx === 1 ? t.pink : t.blue}66`,
            transform: `rotateY(${-18 + idx * 12}deg) rotateZ(${-5 + idx * 5}deg) translateZ(${idx * 18}px)`,
            p: 2,
            overflow: "hidden",
            "&::before": {
              content: '""',
              position: "absolute",
              inset: 0,
              background: `linear-gradient(135deg, ${t.blue}22, ${t.pink}22)`,
            },
          }}
        >
          <Stack spacing={1} sx={{ position: "relative" }}>
            <Box
              sx={{
                width: "48%",
                height: 8,
                borderRadius: 99,
                bgcolor: t.blue,
              }}
            />
            <Box
              sx={{
                width: "82%",
                height: 7,
                borderRadius: 99,
                bgcolor: t.pink,
              }}
            />
            <Box
              sx={{
                width: "70%",
                height: 7,
                borderRadius: 99,
                bgcolor: t.blue,
                opacity: 0.8,
              }}
            />
            <Box
              sx={{
                width: "56%",
                height: 7,
                borderRadius: 99,
                bgcolor: t.purple,
                opacity: 0.85,
              }}
            />
          </Stack>
        </Box>
      ))}
      {icons.map((icon, idx) => (
        <Box
          key={idx}
          sx={{
            position: "absolute",
            left: `${18 + (idx % 3) * 23}%`,
            top: `${16 + Math.floor(idx / 3) * 28}%`,
            width: 40,
            height: 40,
            borderRadius: 2,
            display: "grid",
            placeItems: "center",
            color: t.mode === "light" ? t.purple : "#79d5ff",
            bgcolor:
              t.mode === "light"
                ? "rgba(255,255,255,0.82)"
                : "rgba(7,15,53,0.72)",
            border: `1px solid ${t.strong}`,
            boxShadow: `0 0 28px ${t.glow}`,
            transform: `translateY(${idx % 2 ? 20 : -12}px)`,
            "& svg": { fontSize: 21 },
          }}
        >
          {icon}
        </Box>
      ))}
    </Box>
  );
}

function OrbitalBuilderPanel({
  t,
  prompt,
  onPromptChange,
  onSubmit,
  submitting,
  textareaRef,
  onPickSeed,
}: {
  t: OrbitalPalette;
  prompt: string;
  onPromptChange: (next: string) => void;
  onSubmit: () => void;
  submitting: boolean;
  textareaRef: MutableRefObject<HTMLTextAreaElement | null>;
  onPickSeed: (seed: string) => void;
}) {
  const examples = [
    "SaaS dashboard with users & billing",
    "Marketplace with search & checkout",
    "Internal tool for approvals",
  ];
  return (
    <Box
      sx={{
        position: "relative",
        mt: { xs: 3, md: 0.15 },
        p: { xs: 1.8, md: 1.35 },
        borderRadius: 4,
        border: `1px solid ${t.strong}`,
        bgcolor: t.surface,
        boxShadow: `${t.cardShadow}, 0 0 0 1px ${t.mode === "dark" ? "rgba(255,255,255,0.04)" : "rgba(255,255,255,0.8)"}, 0 0 45px ${t.glow}`,
        backdropFilter: "blur(18px)",
        overflow: "hidden",
        "&::before": {
          content: '""',
          position: "absolute",
          inset: 0,
          pointerEvents: "none",
          background: `radial-gradient(ellipse 420px 150px at 12% 2%, ${t.pink}22, transparent 70%), radial-gradient(ellipse 500px 160px at 88% 96%, ${t.purple}24, transparent 72%)`,
        },
      }}
    >
      <Stack
        direction="row"
        alignItems="center"
        justifyContent="space-between"
        sx={{ position: "relative", mb: 1.5, px: { xs: 0.5, md: 1.5 } }}
      >
        <Stack direction="row" alignItems="center" spacing={1.2}>
          <BoltRounded sx={{ color: t.pink, fontSize: 20 }} />
          <Typography sx={{ fontSize: 16, fontWeight: 900, color: t.text }}>
            AI Product Builder
          </Typography>
        </Stack>
        <Button
          size="small"
          sx={{
            color: t.text,
            border: `1px solid ${t.border}`,
            bgcolor: t.surface2,
            fontWeight: 800,
          }}
        >
          Enhance prompt ✨
        </Button>
      </Stack>
      <Box
        sx={{
          position: "relative",
          p: { xs: 1.3, md: 2 },
          minHeight: { xs: 155, md: 96 },
          display: "grid",
          gridTemplateColumns: { xs: "1fr", md: "1fr auto" },
          gap: 1.5,
          alignItems: "center",
          borderRadius: 2,
          border: `1px solid ${t.border}`,
          bgcolor: t.inset,
        }}
      >
        <Box>
          <Box
            component="textarea"
            ref={textareaRef}
            value={prompt}
            onChange={(e) => onPromptChange(e.target.value)}
            placeholder="Describe your app in plain English..."
            onKeyDown={(e) => {
              if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
                e.preventDefault();
                onSubmit();
              }
            }}
            sx={{
              width: "100%",
              height: 32,
              resize: "none",
              border: 0,
              outline: 0,
              bgcolor: "transparent",
              color: t.text,
              font: "inherit",
              fontSize: { xs: 20, md: 18 },
              lineHeight: 1.3,
              textAlign: "center",
              "&::placeholder": { color: t.secondary, opacity: 0.95 },
            }}
          />
          <Stack
            direction="row"
            justifyContent="center"
            useFlexGap
            flexWrap="wrap"
            gap={1}
          >
            {[
              "Include features",
              "Integrations",
              "Roles",
              "Data",
              "Workflows",
            ].map((chip) => (
              <Box
                key={chip}
                sx={{
                  px: 1.4,
                  py: 0.55,
                  borderRadius: 999,
                  border: `1px solid ${t.border}`,
                  bgcolor: t.surface2,
                  color: t.secondary,
                  fontSize: 12,
                  fontWeight: 800,
                }}
              >
                {chip}
              </Box>
            ))}
          </Stack>
        </Box>
        <Button
          disabled={submitting || prompt.trim().length < 8}
          onClick={onSubmit}
          sx={{
            width: 50,
            height: 50,
            minWidth: 50,
            borderRadius: "50%",
            justifySelf: "center",
            color: "#fff",
            background: `linear-gradient(135deg, ${t.purple}, ${t.pink})`,
            boxShadow: `0 0 34px ${t.glow}`,
            "&.Mui-disabled": { opacity: 0.45, color: "#fff" },
          }}
        >
          <SendRounded sx={{ fontSize: 21 }} />
        </Button>
      </Box>
      <Stack
        direction={{ xs: "column", md: "row" }}
        spacing={1.4}
        alignItems={{ md: "center" }}
        sx={{ position: "relative", mt: 1.6, px: { md: 1.5 } }}
      >
        <Typography sx={{ color: t.secondary, fontSize: 13, minWidth: 135 }}>
          Try an example:
        </Typography>
        <Stack direction="row" useFlexGap flexWrap="wrap" gap={1}>
          {examples.map((ex) => (
            <Button
              key={ex}
              size="small"
              onClick={() => onPickSeed(ex)}
              sx={{
                color: t.text,
                border: `1px solid ${t.border}`,
                bgcolor: t.surface2,
                fontSize: 12,
                fontWeight: 800,
              }}
            >
              {ex}
            </Button>
          ))}
        </Stack>
      </Stack>
      <Box
        sx={{
          position: "relative",
          display: "grid",
          gridTemplateColumns: {
            xs: "1fr",
            sm: "repeat(2, 1fr)",
            md: "repeat(4, 1fr)",
          },
          gap: 1.1,
          mt: 1.25,
        }}
      >
        {[
          [
            "AI generates code",
            "Production-ready code from your prompt",
            <LayersRounded key="l" />,
          ],
          [
            "Review & test",
            "Preview, test and refine before shipping",
            <CheckCircleRounded key="c" />,
          ],
          [
            "Deploy anywhere",
            "One-click deploy to cloud or infrastructure",
            <RocketLaunchRounded key="r" />,
          ],
          [
            "Monitor & iterate",
            "Logs, metrics and real-time observability.",
            <VisibilityRounded key="v" />,
          ],
        ].map(([title, body, icon]) => (
          <Stack
            key={title as string}
            direction="row"
            spacing={1}
            alignItems="center"
            sx={{
              p: 1.1,
              minHeight: 68,
              borderRadius: 2,
              border: `1px solid ${t.border}`,
              bgcolor: t.surface2,
            }}
          >
            <Box
              sx={{
                width: 34,
                height: 34,
                display: "grid",
                placeItems: "center",
                borderRadius: 2,
                color: t.pink,
                background: `radial-gradient(circle, ${t.pink}38, ${t.purple}18)`,
                border: `1px solid ${t.strong}`,
                "& svg": { fontSize: 19 },
              }}
            >
              {icon}
            </Box>
            <Box>
              <Typography
                sx={{ color: t.text, fontSize: 11.4, fontWeight: 900 }}
              >
                {title}
              </Typography>
              <Typography
                sx={{
                  color: t.secondary,
                  mt: 0.2,
                  fontSize: 9.5,
                  lineHeight: 1.28,
                }}
              >
                {body}
              </Typography>
            </Box>
          </Stack>
        ))}
      </Box>
    </Box>
  );
}

function OrbitalCapabilities({ t }: { t: OrbitalPalette }) {
  const items = [
    [
      "AI Product Architect",
      "Plans, designs and generates your entire prompt",
      <RocketLaunchRounded key="a" />,
    ],
    [
      "Visual App Builder",
      "Drag, drop and build with components",
      <VisibilityRounded key="b" />,
    ],
    [
      "Code You Own",
      "Clean, production-ready React, TypeScript & APIs",
      <CodeRounded key="c" />,
    ],
    [
      "Data & Integrations",
      "Connect databases, APIs and 3rd-party services",
      <DataObjectRounded key="d" />,
    ],
    [
      "Team & Roles",
      "Invite teammates, set roles and manage access",
      <HubRounded key="e" />,
    ],
    [
      "Environments",
      "Dev, staging and production with one click",
      <SettingsSuggestRounded key="f" />,
    ],
    [
      "Observability",
      "Real-time logs, traces, metrics and alerts",
      <TimelineRounded key="g" />,
    ],
    [
      "Enterprise Ready",
      "SSO, audit logs, RBAC, backups and compliance",
      <SecurityRounded key="h" />,
    ],
  ];
  return (
    <OrbitalSection
      t={t}
      title="Everything you need to build and ship"
      sx={{ mt: { xs: 3, md: 1.6 } }}
    >
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: {
            xs: "1fr",
            sm: "repeat(2, 1fr)",
            md: "repeat(4, 1fr)",
          },
          gap: 1.2,
        }}
      >
        {items.map(([title, body, icon]) => (
          <OrbitalFeatureCard
            key={title as string}
            t={t}
            title={title as string}
            body={body as string}
            icon={icon as ReactNode}
          />
        ))}
      </Box>
    </OrbitalSection>
  );
}

function OrbitalFeatureCard({
  t,
  title,
  body,
  icon,
}: {
  t: OrbitalPalette;
  title: string;
  body: string;
  icon: ReactNode;
}) {
  return (
    <Stack
      direction="row"
      spacing={1.2}
      sx={{
        p: 1.55,
        minHeight: 76,
        borderRadius: 2,
        border: `1px solid ${t.border}`,
        bgcolor: t.surface,
        boxShadow:
          t.mode === "light" ? "0 14px 34px rgba(51,46,130,0.06)" : "none",
      }}
    >
      <Box
        sx={{
          width: 38,
          height: 38,
          flexShrink: 0,
          borderRadius: 2,
          display: "grid",
          placeItems: "center",
          color: t.pink,
          bgcolor: t.mode === "light" ? "#fbf7ff" : "rgba(37,20,86,0.6)",
          border: `1px solid ${t.border}`,
          "& svg": { fontSize: 21 },
        }}
      >
        {icon}
      </Box>
      <Box>
        <Typography sx={{ color: t.text, fontSize: 12.7, fontWeight: 900 }}>
          {title}
        </Typography>
        <Typography
          sx={{ color: t.secondary, mt: 0.3, fontSize: 11, lineHeight: 1.35 }}
        >
          {body}
        </Typography>
      </Box>
    </Stack>
  );
}

function OrbitalTestimonial({ t }: { t: OrbitalPalette }) {
  return (
    <Box
      sx={{
        mt: 2,
        p: { xs: 2.4, md: 2 },
        borderRadius: 3,
        border: `1px solid ${t.strong}`,
        bgcolor: t.surface,
        display: "grid",
        gridTemplateColumns: { xs: "1fr", md: "1.35fr repeat(4, 0.7fr)" },
        gap: 1.5,
        alignItems: "center",
        boxShadow: `0 0 40px ${t.glow}`,
      }}
    >
      <Box>
        <Typography sx={{ color: t.pink, fontSize: 30, lineHeight: 0.6 }}>
          “
        </Typography>
        <Typography
          sx={{
            color: t.text,
            fontSize: { xs: 18, md: 14.5 },
            fontWeight: 900,
            lineHeight: 1.22,
            maxWidth: 360,
          }}
        >
          We shipped our client portal in a week with IronFlyer. The AI plan was
          spot-on and the code was clean and easy to extend.
        </Typography>
      </Box>
      {[
        ["7 days", "to production", <TimelineRounded key="t" />],
        ["92%", "code accuracy", <ShieldOutlined key="s" />],
        ["3x", "faster delivery", <RocketLaunchRounded key="r" />],
        ["10K+", "projects built", <MailOutlineRounded key="m" />],
      ].map(([value, label, icon]) => (
        <Stack
          key={value as string}
          spacing={0.45}
          alignItems="center"
          sx={{
            borderLeft: { md: `1px solid ${t.border}` },
            minHeight: 90,
            justifyContent: "center",
          }}
        >
          <Box sx={{ color: t.pink, "& svg": { fontSize: 27 } }}>{icon}</Box>
          <Typography
            sx={{ color: t.text, fontSize: 22, fontWeight: 900, lineHeight: 1 }}
          >
            {value}
          </Typography>
          <Typography sx={{ color: t.secondary, fontSize: 10.8 }}>
            {label}
          </Typography>
        </Stack>
      ))}
    </Box>
  );
}

function OrbitalTemplates({
  t,
  onPickSeed,
}: {
  t: OrbitalPalette;
  onPickSeed: (seed: string) => void;
}) {
  const templates = [
    ["SaaS Starter", "Auth", "Billing", "Roles", "/market/console.png"],
    ["AI Chat App", "Chat", "OpenAI", "Pages", "/market/ai-replies-loop.mp4"],
    ["Marketplace", "Listings", "Search", "Checkout", "/market/data-flow.jpg"],
    [
      "Internal Tool",
      "Approvals",
      "Reports",
      "Workflows",
      "/market/repository.png",
    ],
    [
      "Admin Dashboard",
      "Analytics",
      "Charts",
      "Tables",
      "/market/ai-generated-code.png",
    ],
  ];
  return (
    <Box
      sx={{
        mt: 1.5,
        p: { xs: 2, md: 1.25 },
        borderRadius: 3,
        border: `1px solid ${t.border}`,
        bgcolor: t.surface,
      }}
    >
      <Stack
        direction="row"
        justifyContent="space-between"
        alignItems="center"
        sx={{ mb: 1.8 }}
      >
        <Typography sx={{ color: t.text, fontSize: 16, fontWeight: 900 }}>
          Start from a proven template
        </Typography>
        <Button
          component={Link}
          href="/templates"
          size="small"
          endIcon={<ArrowForwardRounded sx={{ fontSize: 15 }} />}
          sx={{ color: t.pink, fontWeight: 900 }}
        >
          View all templates
        </Button>
      </Stack>
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: {
            xs: "1fr",
            sm: "repeat(2, 1fr)",
            md: "repeat(5, 1fr)",
          },
          gap: 1.1,
        }}
      >
        {templates.map(([title, a, b, c, media]) => (
          <Box
            key={title}
            onClick={() =>
              onPickSeed(
                `Build a ${title.toLowerCase()} with ${a}, ${b}, ${c}, admin analytics and deploy-ready code.`,
              )
            }
            sx={{
              cursor: "pointer",
              p: 1,
              borderRadius: 2,
              border: `1px solid ${t.strong}`,
              bgcolor: t.mode === "light" ? "#fff" : "rgba(8,11,34,0.75)",
            }}
          >
            <Box
              sx={{
                height: 54,
                borderRadius: 1.5,
                overflow: "hidden",
                backgroundImage: `linear-gradient(135deg, ${t.purple}33, ${t.blue}22), url('${media}')`,
                backgroundSize: "cover",
                backgroundPosition: "center",
                border: `1px solid ${t.border}`,
              }}
            />
            <Typography
              sx={{ color: t.text, mt: 0.65, fontSize: 11.2, fontWeight: 900 }}
            >
              {title}
            </Typography>
            <Stack direction="row" gap={0.45} sx={{ mt: 0.6 }}>
              {[a, b, c].map((tag) => (
                <Box
                  key={tag}
                  sx={{
                    px: 0.65,
                    py: 0.25,
                    borderRadius: 999,
                    bgcolor: t.surface2,
                    color: t.secondary,
                    fontSize: 8.5,
                    fontWeight: 800,
                  }}
                >
                  {tag}
                </Box>
              ))}
            </Stack>
          </Box>
        ))}
      </Box>
    </Box>
  );
}

function OrbitalPricingFaq({ t }: { t: OrbitalPalette }) {
  const plans = [
    [
      "Free",
      "$0",
      "Forever",
      ["1 workspace", "2 projects", "Community support"],
      "Get started",
    ],
    [
      "Pro",
      "$29",
      "per month",
      ["Unlimited projects", "AI generations", "Email support"],
      "Start free trial",
    ],
    [
      "Team",
      "$79",
      "per month",
      ["SSO & RBAC", "Environments", "Priority support"],
      "Start free trial",
    ],
    [
      "Enterprise",
      "Custom",
      "Let's talk",
      ["Advanced security", "SLA & support", "Custom integrations"],
      "Contact sales",
    ],
  ];
  const faqs = [
    "Can I export the code?",
    "How does pricing work?",
    "Is my data secure?",
    "Do you offer onboarding?",
  ];
  return (
    <Box
      sx={{
        mt: 1.5,
        display: "grid",
        gridTemplateColumns: { xs: "1fr", md: "1fr 3fr 1.15fr" },
        gap: 1.05,
      }}
    >
      <Box
        sx={{
          p: 1.1,
          borderRadius: 2.5,
          border: `1px solid ${t.border}`,
          bgcolor: t.surface,
          minHeight: 152,
        }}
      >
        <Typography
          sx={{
            color: t.text,
            fontSize: 16,
            lineHeight: 1.12,
            fontWeight: 900,
          }}
        >
          Simple,
          <br />
          transparent pricing
        </Typography>
        <Stack direction="row" spacing={1} sx={{ mt: 2 }}>
          <Box
            sx={{
              px: 1.3,
              py: 0.55,
              borderRadius: 999,
              bgcolor: t.purple,
              color: "#fff",
              fontSize: 12,
              fontWeight: 900,
            }}
          >
            Monthly
          </Box>
          <Box
            sx={{
              px: 1.3,
              py: 0.55,
              borderRadius: 999,
              border: `1px solid ${t.border}`,
              color: t.secondary,
              fontSize: 12,
              fontWeight: 800,
            }}
          >
            Yearly
          </Box>
        </Stack>
      </Box>
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: {
            xs: "1fr",
            sm: "repeat(2, 1fr)",
            md: "repeat(4, 1fr)",
          },
          gap: 1.1,
        }}
      >
        {plans.map(([name, price, cadence, features, cta], idx) => (
          <Box
            key={name as string}
            sx={{
              position: "relative",
              p: 1,
              borderRadius: 2,
              border: `1px solid ${idx === 2 ? t.pink : t.border}`,
              bgcolor: t.surface,
              boxShadow: idx === 2 ? `0 0 34px ${t.glow}` : "none",
            }}
          >
            {idx === 2 && (
              <Box
                sx={{
                  position: "absolute",
                  right: 12,
                  top: -13,
                  px: 1.2,
                  py: 0.5,
                  borderRadius: 999,
                  bgcolor: t.purple,
                  color: "#fff",
                  fontSize: 11,
                  fontWeight: 900,
                }}
              >
                Most popular
              </Box>
            )}
            <Typography sx={{ color: t.text, fontSize: 11.5, fontWeight: 900 }}>
              {name}
            </Typography>
            <Typography
              sx={{
                color: t.text,
                mt: 0.75,
                fontSize: price === "Custom" ? 18 : 23,
                fontWeight: 900,
                lineHeight: 1,
              }}
            >
              {price}
            </Typography>
            <Typography sx={{ color: t.secondary, mt: 0.25, fontSize: 10 }}>
              {cadence}
            </Typography>
            <Stack spacing={0.35} sx={{ my: 1.15 }}>
              {(features as string[]).map((f) => (
                <Typography key={f} sx={{ color: t.secondary, fontSize: 8.9 }}>
                  ✓ {f}
                </Typography>
              ))}
            </Stack>
            <Button
              fullWidth
              component={Link}
              href="/signup"
              variant={idx === 2 ? "contained" : "outlined"}
              sx={{
                minHeight: 32,
                px: 0.6,
                color: idx === 2 ? "#fff" : t.text,
                background: idx === 2 ? gradient(t) : "transparent",
                borderColor: t.border,
                fontWeight: 900,
                fontSize: 10.2,
                whiteSpace: "nowrap",
              }}
            >
              {cta}
            </Button>
          </Box>
        ))}
      </Box>
      <Box
        sx={{
          p: 1.1,
          borderRadius: 2.5,
          border: `1px solid ${t.border}`,
          bgcolor: t.surface,
        }}
      >
        <Typography
          sx={{ color: t.text, fontSize: 14, fontWeight: 900, mb: 0.8 }}
        >
          Frequently asked questions
        </Typography>
        <Stack spacing={0.55}>
          {faqs.map((q) => (
            <Box
              key={q}
              sx={{
                p: 0.85,
                display: "flex",
                justifyContent: "space-between",
                borderRadius: 1.3,
                bgcolor: t.surface2,
                border: `1px solid ${t.border}`,
                color: t.text,
                fontSize: 10,
                fontWeight: 900,
              }}
            >
              <span>{q}</span>
              <span>+</span>
            </Box>
          ))}
        </Stack>
      </Box>
    </Box>
  );
}

function OrbitalFinalCta({ t }: { t: OrbitalPalette }) {
  return (
    <Box
      sx={{
        mt: 0.9,
        p: { xs: 2.4, md: 0.8 },
        borderRadius: 4,
        border: `1px solid ${t.strong}`,
        background: `linear-gradient(105deg, ${t.mode === "light" ? "#35128f" : "#16084f"}, ${t.purple}, ${t.pink})`,
        color: "#fff",
        display: "grid",
        gridTemplateColumns: { xs: "1fr", md: "58px 1fr auto" },
        gap: 1.1,
        alignItems: "center",
        boxShadow: `0 0 40px ${t.glow}`,
      }}
    >
      <Box
        sx={{
          width: 40,
          height: 40,
          borderRadius: "50%",
          display: "grid",
          placeItems: "center",
          border: "2px solid rgba(255,255,255,0.42)",
          boxShadow: "0 0 28px rgba(255,255,255,0.35)",
          bgcolor: "rgba(255,255,255,0.1)",
        }}
      >
        <BoltRounded sx={{ fontSize: 21 }} />
      </Box>
      <Box>
        <Typography sx={{ fontSize: { xs: 24, md: 18 }, fontWeight: 900 }}>
          Stop stitching tools. Start shipping products.
        </Typography>
        <Typography sx={{ opacity: 0.82, mt: 0.5 }}>
          One prompt. One workspace. One launch.
        </Typography>
      </Box>
      <Stack direction={{ xs: "column", sm: "row" }} spacing={1.4}>
        <Button
          component={Link}
          href="/signup"
          variant="contained"
          endIcon={<ArrowForwardRounded />}
          sx={{ bgcolor: "#fff", color: t.purple, fontWeight: 900 }}
        >
          Start building for free
        </Button>
        <Button
          component={Link}
          href="/enterprise"
          variant="outlined"
          sx={{
            color: "#fff",
            borderColor: "rgba(255,255,255,0.55)",
            fontWeight: 900,
          }}
        >
          Talk to sales
        </Button>
      </Stack>
    </Box>
  );
}

function OrbitalFooter({
  t,
  copy,
}: {
  t: OrbitalPalette;
  copy: HomeCopy["footer"];
}) {
  const cols = ["Product", "Solutions", "Resources", "Company", "Legal"];
  return (
    <Box
      component="footer"
      sx={{
        pt: 2.2,
        pb: 1,
        display: "grid",
        gridTemplateColumns: { xs: "1fr", md: "1.5fr repeat(5, 1fr)" },
        gap: 1.8,
        color: t.secondary,
      }}
    >
      <Box>
        <BrandLogo inverse={t.mode === "dark"} size={26} href="/" />
        <Typography
          sx={{ mt: 1.3, maxWidth: 280, fontSize: 12.5, lineHeight: 1.55 }}
        >
          {copy.body}
        </Typography>
        <GitHub sx={{ mt: 1.5, fontSize: 18 }} />
      </Box>
      {cols.map((col) => (
        <Stack key={col} spacing={0.8}>
          <Typography
            sx={{
              color: t.text,
              fontSize: 11,
              fontWeight: 900,
              textTransform: "uppercase",
            }}
          >
            {col}
          </Typography>
          {["Features", "Templates", "Pricing"].map((l) => (
            <Typography key={l} sx={{ fontSize: 12 }}>
              {l}
            </Typography>
          ))}
        </Stack>
      ))}
    </Box>
  );
}

function OrbitalSection({
  t,
  title,
  children,
  sx,
}: {
  t: OrbitalPalette;
  title: string;
  children: ReactNode;
  sx?: object;
}) {
  return (
    <Box sx={sx}>
      <Typography
        sx={{
          color: t.text,
          textAlign: "center",
          fontSize: { xs: 24, md: 20 },
          fontWeight: 900,
          mb: 1.6,
        }}
      >
        <Box component="span" sx={{ color: t.pink, mx: 1 }}>
          ←
        </Box>
        {title}
        <Box component="span" sx={{ color: t.pink, mx: 1 }}>
          →
        </Box>
      </Typography>
      {children}
    </Box>
  );
}

function orbitalPillSx(t: OrbitalPalette) {
  return {
    display: "inline-flex",
    alignItems: "center",
    gap: 0.8,
    alignSelf: "flex-start",
    px: 1.5,
    py: 0.65,
    borderRadius: 999,
    border: `1px solid ${t.strong}`,
    bgcolor:
      t.mode === "light" ? "rgba(255,255,255,0.8)" : "rgba(73,28,124,0.38)",
    color: t.purple,
    fontSize: 12,
    fontWeight: 900,
    boxShadow: `0 0 24px ${t.glow}`,
  };
}

function orbitSx(
  t: OrbitalPalette,
  top: number,
  left: number,
  width: number,
  rotate: number,
) {
  return {
    position: "absolute",
    top: `${top}%`,
    left: `${left - width / 2}%`,
    width: `${width}%`,
    height: 86,
    borderRadius: "50%",
    border: `2px solid ${t.blue}`,
    borderLeftColor: t.pink,
    borderRightColor: t.purple,
    transform: `rotate(${rotate}deg)`,
    filter: `drop-shadow(0 0 16px ${t.blue})`,
    opacity: t.mode === "light" ? 0.55 : 0.95,
  } as const;
}

// ── Hero ────────────────────────────────────────────────────────────────

interface HeroProps {
  timing: OrbitalTiming;
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
  const light = props.timing === "light";
  const hero = {
    bg: light ? "#fbfaff" : tokens.color.bg.base,
    text: light ? "#080b3f" : tokens.color.text.primary,
    secondary: light ? "#5e6689" : tokens.color.text.secondary,
    muted: light ? "#79809f" : tokens.color.text.muted,
    border: light ? "rgba(127,77,255,0.18)" : tokens.color.border.subtle,
    surface: light ? "rgba(255,255,255,0.74)" : "rgba(16,18,44,0.68)",
    surfaceStrong: light
      ? "rgba(255,255,255,0.92)"
      : tokens.color.bg.surfaceRaised,
    chip: light ? "rgba(255,255,255,0.68)" : "rgba(18,20,48,0.78)",
    shadow: light
      ? "0 26px 90px rgba(150,80,255,0.14), 0 18px 80px rgba(234,75,189,0.10)"
      : "0 28px 110px rgba(104,42,255,0.28), 0 0 80px rgba(225,73,201,0.10)",
  };

  return (
    <Section
      sx={{
        pt: { xs: 2, md: 3.7 },
        pb: { xs: 4, md: 4.4 },
        position: "relative",
        overflow: "hidden",
        minHeight: { md: 790 },
        color: hero.text,
        bgcolor: hero.bg,
        backgroundImage: light
          ? [
              "radial-gradient(780px 360px at 4% 84%, rgba(230,70,199,0.14), transparent 72%)",
              "radial-gradient(780px 420px at 82% 34%, rgba(255,111,76,0.10), transparent 75%)",
            ].join(",")
          : [
              "radial-gradient(780px 360px at 4% 84%, rgba(181,108,255,0.16), transparent 72%)",
              "radial-gradient(780px 420px at 82% 34%, rgba(255,111,76,0.12), transparent 75%)",
            ].join(","),
      }}
    >
      <Stack
        spacing={{ xs: 2.1, md: 2.25 }}
        sx={{ position: "relative", zIndex: 1 }}
      >
        <Stack spacing={1.45} alignItems="center" sx={{ pt: { md: 0.8 } }}>
          {props.welcomeOpen && (
            <WelcomeBanner
              onContinue={props.onWelcomeContinue}
              onDismiss={props.onWelcomeDismiss}
            />
          )}
          <Stack
            direction="row"
            spacing={1}
            alignItems="center"
            sx={referencePillSx(hero)}
          >
            <AutoAwesomeRounded sx={{ fontSize: 14 }} />
            <span>{props.copy.eyebrow}</span>
          </Stack>
          <Typography
            component="h1"
            sx={{
              color: hero.text,
              fontSize: { xs: 39, sm: 52, md: 56 },
              fontWeight: 900,
              letterSpacing: 0,
              lineHeight: 1.02,
              maxWidth: 880,
              textAlign: "center",
            }}
          >
            Build, review and ship
            <br />
            production apps
            <br />
            from a{" "}
            <Box
              component="span"
              sx={{
                backgroundImage: `linear-gradient(100deg, ${tokens.color.accent.coral}, ${tokens.color.brand.magenta} 52%, ${tokens.color.accent.violet})`,
                WebkitBackgroundClip: "text",
                WebkitTextFillColor: "transparent",
              }}
            >
              single prompt.
            </Box>
          </Typography>
          <Box
            sx={{
              width: "100%",
              maxWidth: 846,
              mx: "auto",
              pt: { xs: 1.2, md: 3.1 },
            }}
          >
            <HeroPromptInput
              ref={props.inputRef}
              timing={props.timing}
              value={props.prompt}
              onChange={props.onPromptChange}
              onSubmit={props.onSubmit}
              submitting={props.submitting}
              budgetUSD={props.budgetUSD ?? 27}
              onBudgetChange={props.onBudgetChange}
              planFirst={props.planFirst}
              onPlanFirstChange={props.onPlanFirstChange}
            />
          </Box>
          <Box sx={{ width: "100%", maxWidth: 900 }}>
            <CategoryChips timing={props.timing} onPick={props.onPickSeed} />
          </Box>
          <HeroCapabilityRail timing={props.timing} colors={hero} />
          <HeroTrustedLogos timing={props.timing} colors={hero} />
        </Stack>
      </Stack>
    </Section>
  );
}

function HomeTimingToggle({ timing }: { timing: OrbitalTiming }) {
  const light = timing === "light";
  return (
    <Stack
      direction="row"
      spacing={0.4}
      sx={{
        position: "absolute",
        top: { xs: 8, md: 10 },
        right: { xs: 0, md: 2 },
        zIndex: 3,
        p: 0.35,
        borderRadius: 999,
        border: `1px solid ${light ? "rgba(127,77,255,0.18)" : tokens.color.border.subtle}`,
        bgcolor: light ? "rgba(255,255,255,0.78)" : "rgba(10,12,30,0.74)",
        boxShadow: light
          ? "0 10px 30px rgba(89,59,160,0.10)"
          : "0 14px 36px rgba(0,0,0,0.24)",
        backdropFilter: "blur(14px)",
      }}
    >
      {(["light", "dark"] as const).map((mode) => {
        const active = timing === mode;
        return (
          <Button
            key={mode}
            component={Link}
            href={mode === "light" ? "/?theme=light" : "/?theme=dark"}
            size="small"
            sx={{
              minHeight: 28,
              px: 1.15,
              borderRadius: 999,
              fontSize: 11.5,
              fontWeight: 900,
              color: active
                ? "#fff"
                : light
                  ? "#555d83"
                  : tokens.color.text.secondary,
              bgcolor: active ? tokens.color.accent.violet : "transparent",
              "&:hover": {
                bgcolor: active
                  ? tokens.color.accent.violet
                  : light
                    ? "rgba(143,77,255,0.10)"
                    : tokens.color.bg.surfaceHover,
              },
            }}
          >
            {mode === "light" ? "Light" : "Dark"}
          </Button>
        );
      })}
    </Stack>
  );
}

function referencePillSx(hero: { border: string; surface: string }) {
  return {
    px: 1.65,
    py: 0.75,
    borderRadius: 999,
    border: `1px solid ${hero.border}`,
    bgcolor: hero.surface,
    color: tokens.color.brand.magenta,
    fontSize: 13,
    fontWeight: 900,
    boxShadow: "0 12px 30px rgba(181,108,255,0.10)",
  } as const;
}

function HeroCapabilityRail({
  timing,
  colors,
}: {
  timing: OrbitalTiming;
  colors: {
    border: string;
    surface: string;
    surfaceStrong: string;
    text: string;
    secondary: string;
    shadow: string;
  };
}) {
  const items = [
    {
      title: "AI generates code",
      body: "Production-ready code from your prompt",
      icon: <DataObjectRounded />,
    },
    {
      title: "Review & test",
      body: "Preview, test and refine before shipping",
      icon: <MonitorHeartOutlined />,
    },
    {
      title: "Deploy anywhere",
      body: "One-click deploy to cloud or your infrastructure",
      icon: <RocketLaunchRounded />,
    },
    {
      title: "Monitor & iterate",
      body: "Logs, metrics and real-time observability",
      icon: <TimelineRounded />,
    },
  ];
  const light = timing === "light";
  return (
    <Box
      sx={{
        width: "100%",
        maxWidth: 1090,
        display: "grid",
        gridTemplateColumns: {
          xs: "1fr",
          sm: "repeat(2, minmax(0, 1fr))",
          lg: "repeat(4, minmax(0, 1fr))",
        },
        border: `1px solid ${colors.border}`,
        borderRadius: 2.2,
        bgcolor: colors.surface,
        boxShadow: light
          ? "0 14px 48px rgba(78,64,130,0.08)"
          : "0 18px 70px rgba(0,0,0,0.24)",
        overflow: "hidden",
      }}
    >
      {items.map((item, index) => (
        <Stack
          key={item.title}
          direction="row"
          spacing={1.4}
          sx={{
            minHeight: 92,
            p: 2,
            borderRight: {
              lg:
                index < items.length - 1
                  ? `1px solid ${colors.border}`
                  : "none",
            },
            borderBottom: {
              xs:
                index < items.length - 1
                  ? `1px solid ${colors.border}`
                  : "none",
              sm: index < 2 ? `1px solid ${colors.border}` : "none",
              lg: "none",
            },
          }}
        >
          <Box
            sx={{
              width: 50,
              height: 50,
              flex: "0 0 auto",
              display: "grid",
              placeItems: "center",
              borderRadius: 1.8,
              border: `1px solid ${colors.border}`,
              bgcolor: colors.surfaceStrong,
              color: tokens.color.accent.violet,
              "& svg": { fontSize: 27 },
            }}
          >
            {item.icon}
          </Box>
          <Box sx={{ minWidth: 0 }}>
            <Typography
              sx={{ color: colors.text, fontSize: 14, fontWeight: 900 }}
            >
              {item.title}
            </Typography>
            <Typography
              sx={{
                mt: 0.45,
                color: colors.secondary,
                fontSize: 12.4,
                lineHeight: 1.45,
              }}
            >
              {item.body}
            </Typography>
          </Box>
        </Stack>
      ))}
    </Box>
  );
}

function HeroTrustedLogos({
  colors,
}: {
  timing: OrbitalTiming;
  colors: { muted: string; secondary: string };
}) {
  const logos = ["Google", "Microsoft", "Airbnb", "Amazon", "Spotify"];
  return (
    <Stack spacing={1.3} alignItems="center" sx={{ pt: { xs: 0.2, md: 0.8 } }}>
      <Typography sx={{ color: colors.muted, fontSize: 14, fontWeight: 700 }}>
        Trusted by modern teams worldwide
      </Typography>
      <Stack
        direction="row"
        useFlexGap
        flexWrap="wrap"
        justifyContent="center"
        sx={{ gap: { xs: 2.2, sm: 4.2 }, color: colors.secondary }}
      >
        {logos.map((name) => (
          <Stack
            key={name}
            direction="row"
            alignItems="center"
            sx={{ gap: 0.7, opacity: 0.86 }}
          >
            <Box
              sx={{
                width: 7,
                height: 7,
                borderRadius: "50%",
                bgcolor: "currentColor",
                color: colors.secondary,
              }}
            />
            <Typography
              sx={{
                fontSize: { xs: 17, md: 20 },
                fontWeight: 900,
                letterSpacing: 0,
                color: colors.secondary,
              }}
            >
              {name}
            </Typography>
          </Stack>
        ))}
      </Stack>
    </Stack>
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
      <Box sx={{ display: "inline-flex", color: tokens.color.accent.violet }}>
        {icon}
      </Box>
      <Typography
        sx={{ fontFamily: tokens.font.mono, fontSize: 11.5, fontWeight: 700 }}
      >
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
      <CheckCircleRounded
        sx={{ fontSize: 18, color: tokens.color.accent.success }}
      />
      <Box sx={{ flex: 1, minWidth: 220, textAlign: "left" }}>
        <Typography
          sx={{
            fontSize: 13.5,
            fontWeight: 700,
            color: tokens.color.text.primary,
          }}
        >
          Welcome aboard. Your prompt is ready to launch.
        </Typography>
        <Typography sx={{ fontSize: 12, color: tokens.color.text.secondary }}>
          We saved what you typed before signing up. Continue when you are ready
          and we will hold the wallet budget.
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
              <Stack
                direction="row"
                alignItems="center"
                spacing={0.75}
                sx={{ color: tokens.color.accent.violet }}
              >
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
              <Typography
                sx={{ fontSize: 11.5, color: tokens.color.text.secondary }}
              >
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
      <Stack
        direction="row"
        alignItems="baseline"
        justifyContent="space-between"
        useFlexGap
        flexWrap="wrap"
        sx={{ gap: 1 }}
      >
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
  const icons = [
    <AutoAwesomeRounded key="idea" />,
    <RuleFolderRounded key="patch" />,
    <RocketLaunchRounded key="ship" />,
  ];
  return (
    <Section sx={{ py: { xs: 6, md: 8 } }}>
      <Stack spacing={1} sx={{ textAlign: "center", mb: { xs: 4, md: 5 } }}>
        <Typography
          sx={{
            fontSize: { xs: 26, md: 32 },
            fontWeight: 800,
            letterSpacing: 0,
          }}
        >
          {copy.title}
        </Typography>
        <Typography
          sx={{
            color: tokens.color.text.secondary,
            fontSize: 14,
            maxWidth: 600,
            mx: "auto",
          }}
        >
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
            <Typography
              sx={{ mt: 2, fontSize: 20, fontWeight: 800, letterSpacing: 0 }}
            >
              {s.title}
            </Typography>
            <Typography
              sx={{
                mt: 1,
                color: tokens.color.text.secondary,
                fontSize: 13.5,
                lineHeight: 1.55,
              }}
            >
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
          <Stack
            direction="row"
            alignItems="center"
            spacing={1}
            sx={{ color: tokens.color.accent.violet }}
          >
            <BoltRounded sx={{ fontSize: 18 }} />
            <Typography
              sx={{
                fontSize: 11.5,
                fontWeight: 800,
                letterSpacing: 1,
                textTransform: "uppercase",
              }}
            >
              {copy.eyebrow}
            </Typography>
          </Stack>
          <Typography
            sx={{
              mt: 1.5,
              fontSize: { xs: 26, md: 32 },
              fontWeight: 800,
              letterSpacing: 0,
            }}
          >
            {copy.title}
          </Typography>
          <Typography
            sx={{
              mt: 1.5,
              fontSize: 14,
              color: tokens.color.text.secondary,
              lineHeight: 1.6,
              maxWidth: 520,
            }}
          >
            {copy.body}
          </Typography>
          <Stack
            direction={{ xs: "column", sm: "row" }}
            spacing={1.5}
            sx={{ mt: 3 }}
          >
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
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 12,
                  color: tokens.color.text.muted,
                }}
              >
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
            gridTemplateColumns: {
              xs: "repeat(2, 1fr)",
              sm: "repeat(3, 1fr)",
              md: "repeat(6, 1fr)",
            },
            gap: 2,
            color: tokens.color.text.secondary,
          }}
        >
          {logos.map((logo) => (
            <Typography
              key={logo}
              sx={{ textAlign: "center", fontSize: 15, fontWeight: 900 }}
            >
              {logo}
            </Typography>
          ))}
        </Box>
      </Stack>
    </Section>
  );
}

function FlowPanel({ timing }: { timing: OrbitalTiming }) {
  const light = timing === "light";
  const c = {
    text: light ? "#080b3f" : tokens.color.text.primary,
    secondary: light ? "#5e6689" : tokens.color.text.secondary,
    border: light ? "rgba(127,77,255,0.18)" : tokens.color.border.subtle,
    surface: light
      ? "rgba(255,255,255,0.78)"
      : `${tokens.color.bg.surfaceRaised}d9`,
    icon: light ? "rgba(143,77,255,0.10)" : `${tokens.color.accent.violet}19`,
  };
  const steps = [
    {
      title: "Plan",
      body: "Turn a prompt into a structured product plan with roles, flows, data and acceptance criteria.",
      icon: <RuleFolderRounded />,
    },
    {
      title: "Build",
      body: "Generate a production-ready app with code, APIs, screens and a design system.",
      icon: <SettingsSuggestRounded />,
    },
    {
      title: "Review",
      body: "Test visually, review logic, track tasks and iterate with confidential AI feedback.",
      icon: <VisibilityRounded />,
    },
    {
      title: "Deploy",
      body: "One click deploys to staging or production with environments, logs and rollback.",
      icon: <RocketLaunchRounded />,
    },
  ];
  return (
    <Section sx={{ py: { xs: 4, md: 4.8 } }}>
      <Box
        sx={{
          position: "relative",
          p: { xs: 3, md: 4 },
          borderRadius: 2,
          border: `1px solid ${c.border}`,
          bgcolor: c.surface,
          overflow: "hidden",
          boxShadow: light
            ? "0 24px 90px rgba(103,65,180,0.08)"
            : `0 26px 90px ${tokens.color.accent.purple}1c`,
        }}
      >
        <MiniPrism
          sx={{ right: { xs: 18, md: 42 }, top: { xs: 18, md: 28 } }}
        />
        <Stack spacing={1} alignItems="center" sx={{ mb: { xs: 3, md: 4 } }}>
          <Typography
            sx={{
              color: c.text,
              fontSize: { xs: 25, md: 32 },
              lineHeight: 1.05,
              fontWeight: 900,
              textAlign: "center",
            }}
          >
            From idea to launch in one flow
          </Typography>
          <Typography
            sx={{
              color: c.secondary,
              fontSize: 13.5,
              textAlign: "center",
            }}
          >
            Plan with clarity. Build with speed. Ship with confidence.
          </Typography>
        </Stack>
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: {
              xs: "1fr",
              sm: "repeat(2, 1fr)",
              md: "repeat(4, 1fr)",
            },
            gap: { xs: 2, md: 2.5 },
          }}
        >
          {steps.map((step) => (
            <Stack
              key={step.title}
              spacing={1.2}
              alignItems="center"
              sx={{ textAlign: "center", minWidth: 0 }}
            >
              <Box
                sx={{
                  width: 44,
                  height: 44,
                  borderRadius: 1,
                  display: "grid",
                  placeItems: "center",
                  color: tokens.color.accent.violet,
                  bgcolor: c.icon,
                  border: `1px solid ${c.border}`,
                  "& svg": { fontSize: 21 },
                }}
              >
                {step.icon}
              </Box>
              <Typography sx={{ fontSize: 13.5, fontWeight: 900 }}>
                {step.title}
              </Typography>
              <Typography
                sx={{
                  color: c.secondary,
                  fontSize: 12,
                  lineHeight: 1.55,
                  maxWidth: 210,
                }}
              >
                {step.body}
              </Typography>
            </Stack>
          ))}
        </Box>
      </Box>
    </Section>
  );
}

function CapabilityGrid({ timing }: { timing: OrbitalTiming }) {
  const light = timing === "light";
  const c = {
    text: light ? "#080b3f" : tokens.color.text.primary,
    secondary: light ? "#5e6689" : tokens.color.text.secondary,
    border: light ? "rgba(127,77,255,0.18)" : tokens.color.border.subtle,
    surface: light
      ? "rgba(255,255,255,0.78)"
      : `${tokens.color.bg.surfaceRaised}d9`,
  };
  const capabilities = [
    {
      title: "AI Product Architect",
      body: "Understands your goal and creates a complete app plan.",
      icon: <AutoAwesomeRounded />,
    },
    {
      title: "Visual App Builder",
      body: "Generate responsive screens with a modern design system.",
      icon: <VisibilityRounded />,
    },
    {
      title: "Code You Own",
      body: "Export clean production-ready React and TypeScript.",
      icon: <CodeRounded />,
    },
    {
      title: "Data & Integrations",
      body: "Models, APIs, auth, storage and third-party connectors.",
      icon: <DataObjectRounded />,
    },
    {
      title: "Team & Roles",
      body: "Invite teammates, set roles and manage access.",
      icon: <HubRounded />,
    },
    {
      title: "Environments",
      body: "Dev, staging and prod with secrets and config.",
      icon: <SettingsSuggestRounded />,
    },
    {
      title: "Observability",
      body: "Logs, traces, metrics and error tracking by default.",
      icon: <TimelineRounded />,
    },
    {
      title: "Enterprise Ready",
      body: "SSO, audit logs, RBAC and isolated backends.",
      icon: <SecurityRounded />,
    },
  ];
  return (
    <Section sx={{ py: { xs: 4, md: 4.8 } }}>
      <Stack spacing={3}>
        <Typography
          sx={{
            fontSize: { xs: 25, md: 32 },
            fontWeight: 900,
            textAlign: "center",
            color: c.text,
          }}
        >
          Everything you need to build and ship
        </Typography>
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: {
              xs: "1fr",
              sm: "repeat(2, 1fr)",
              lg: "repeat(4, 1fr)",
            },
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
                border: `1px solid ${c.border}`,
                bgcolor: c.surface,
                transition: `transform ${tokens.motion.fast} ${tokens.motion.snap}, border-color ${tokens.motion.fast} ${tokens.motion.snap}`,
                "&:hover": {
                  transform: "translateY(-3px)",
                  borderColor: tokens.color.border.strong,
                },
              }}
            >
              <Stack direction="row" spacing={1.1} alignItems="center">
                <Box
                  sx={{
                    color: tokens.color.accent.violet,
                    display: "grid",
                    "& svg": { fontSize: 18 },
                  }}
                >
                  {item.icon}
                </Box>
                <Typography
                  sx={{ color: c.text, fontSize: 13.5, fontWeight: 900 }}
                >
                  {item.title}
                </Typography>
              </Stack>
              <Typography
                sx={{
                  mt: 1,
                  fontSize: 12,
                  lineHeight: 1.5,
                  color: c.secondary,
                }}
              >
                {item.body}
              </Typography>
            </Box>
          ))}
        </Box>
      </Stack>
    </Section>
  );
}

function TemplateShowcase({
  timing,
  onPick,
}: {
  timing: OrbitalTiming;
  onPick: (seed: string) => void;
}) {
  const light = timing === "light";
  const c = {
    text: light ? "#080b3f" : tokens.color.text.primary,
    secondary: light ? "#5e6689" : tokens.color.text.secondary,
    border: light ? "rgba(127,77,255,0.18)" : tokens.color.border.subtle,
    surface: light
      ? "rgba(255,255,255,0.78)"
      : `${tokens.color.bg.surfaceRaised}d4`,
    card: light ? "rgba(255,255,255,0.86)" : tokens.color.bg.surface,
    inset: light ? "rgba(247,244,255,0.92)" : tokens.color.bg.inset,
  };
  const templates = [
    ["SaaS Starter", "Auth, billing, team settings", "92", "Live billing"],
    ["Client Portal", "Projects, files, approvals", "88", "Approvals queue"],
    ["Marketplace", "Listings, search, checkout", "94", "Order flow"],
    ["Internal Tool", "Workflows, approvals, reports", "91", "Ops cockpit"],
    ["Education App", "Lessons, progress, analytics", "86", "Progress map"],
  ];
  return (
    <Section sx={{ py: { xs: 4, md: 4.8 } }}>
      <Box
        sx={{
          p: { xs: 2.4, md: 3 },
          borderRadius: 2,
          border: `1px solid ${c.border}`,
          bgcolor: c.surface,
          boxShadow: light
            ? "0 24px 90px rgba(103,65,180,0.09)"
            : "0 24px 80px rgba(0,0,0,0.18)",
          overflow: "hidden",
        }}
      >
        <Stack
          direction="row"
          alignItems="center"
          justifyContent="space-between"
          useFlexGap
          flexWrap="wrap"
          sx={{ gap: 1.5, mb: 2 }}
        >
          <Box>
            <Typography sx={{ color: c.text, fontSize: 20, fontWeight: 900 }}>
              Start from a proven template
            </Typography>
            <Typography sx={{ mt: 0.5, color: c.secondary, fontSize: 13 }}>
              Swipe through polished starting points for common product
              patterns.
            </Typography>
          </Box>
          <Button
            component={Link}
            href="/templates"
            size="small"
            endIcon={<ArrowForwardRounded sx={{ fontSize: 16 }} />}
            sx={{ color: tokens.color.accent.violet }}
          >
            Browse all templates
          </Button>
          <Stack direction="row" spacing={0.8}>
            <IconSwiperButton className="template-prev">
              <ArrowBackRounded sx={{ fontSize: 17 }} />
            </IconSwiperButton>
            <IconSwiperButton className="template-next">
              <ArrowForwardRounded sx={{ fontSize: 17 }} />
            </IconSwiperButton>
          </Stack>
        </Stack>
        <Box
          sx={{
            ".swiper": {
              overflow: "visible",
              width: "100%",
            },
            ".swiper-wrapper": {
              alignItems: "stretch",
              display: "flex",
              width: "100%",
            },
            ".swiper-slide": {
              display: "block",
              flexShrink: 0,
              height: "auto",
            },
          }}
        >
          <Swiper
            modules={[Autoplay, Navigation]}
            slidesPerView={1.12}
            spaceBetween={14}
            autoplay={{ delay: 2800, disableOnInteraction: false }}
            navigation={{
              prevEl: ".template-prev",
              nextEl: ".template-next",
            }}
            breakpoints={{
              640: { slidesPerView: 2.2, spaceBetween: 14 },
              960: { slidesPerView: 3.2, spaceBetween: 16 },
              1200: { slidesPerView: 4.25, spaceBetween: 16 },
            }}
          >
            {templates.map(([title, body, score, seed]) => (
              <SwiperSlide key={title}>
                <Box
                  onClick={() =>
                    onPick(
                      `Build a ${title.toLowerCase()} with ${body.toLowerCase()}, roles, payments and deploy-ready code.`,
                    )
                  }
                  sx={{
                    height: "100%",
                    cursor: "pointer",
                    borderRadius: 1.5,
                    border: `1px solid ${c.border}`,
                    bgcolor: c.card,
                    overflow: "hidden",
                    transition: `transform ${tokens.motion.fast} ${tokens.motion.snap}, border-color ${tokens.motion.fast} ${tokens.motion.snap}`,
                    "&:hover": {
                      transform: "translateY(-4px)",
                      borderColor: tokens.color.border.strong,
                    },
                  }}
                >
                  <Box
                    sx={{
                      p: 1,
                      background: light
                        ? "linear-gradient(135deg, rgba(143,77,255,0.12), rgba(255,91,184,0.10))"
                        : `${tokens.color.accent.purple}24`,
                    }}
                  >
                    <Box
                      sx={{
                        height: 88,
                        borderRadius: 1.1,
                        bgcolor: c.inset,
                        p: 1.1,
                        position: "relative",
                        overflow: "hidden",
                        transform: "perspective(700px) rotateX(4deg)",
                        boxShadow: light
                          ? "inset 0 0 0 1px rgba(255,255,255,0.8)"
                          : "inset 0 0 0 1px rgba(255,255,255,0.04)",
                      }}
                    >
                      <Stack direction="row" spacing={0.5} sx={{ mb: 1 }}>
                        {[0, 1, 2].map((dot) => (
                          <Box
                            key={dot}
                            sx={{
                              width: 6,
                              height: 6,
                              borderRadius: "50%",
                              bgcolor: light
                                ? "rgba(83,89,123,0.36)"
                                : tokens.color.text.muted,
                            }}
                          />
                        ))}
                      </Stack>
                      <Box
                        sx={{
                          width: "70%",
                          height: 12,
                          borderRadius: 0.7,
                          bgcolor: `${tokens.color.accent.violet}88`,
                          mb: 0.8,
                        }}
                      />
                      <Box
                        sx={{
                          width: "50%",
                          height: 12,
                          borderRadius: 0.7,
                          bgcolor: `${tokens.color.accent.violet}55`,
                        }}
                      />
                      <Box
                        sx={{
                          position: "absolute",
                          right: 12,
                          bottom: 12,
                          width: 42,
                          height: 30,
                          borderRadius: "50%",
                          background:
                            "linear-gradient(135deg, rgba(255,105,93,0.75), rgba(179,77,255,0.9))",
                          transform: "rotateX(64deg) rotateZ(-18deg)",
                          filter: "blur(0.2px)",
                        }}
                      />
                      <Typography
                        sx={{
                          position: "absolute",
                          right: 13,
                          top: 34,
                          color: tokens.color.accent.violet,
                          fontFamily: tokens.font.mono,
                          fontWeight: 900,
                          fontSize: 14,
                        }}
                      >
                        {score}
                      </Typography>
                    </Box>
                  </Box>
                  <Box sx={{ p: 1.55 }}>
                    <Typography
                      sx={{ color: c.text, fontSize: 13.5, fontWeight: 900 }}
                    >
                      {title}
                    </Typography>
                    <Typography
                      sx={{
                        mt: 0.5,
                        fontSize: 11.8,
                        color: c.secondary,
                      }}
                    >
                      {body}
                    </Typography>
                    <Typography
                      sx={{
                        mt: 1,
                        fontSize: 10.5,
                        color: light ? "#8a91ad" : tokens.color.text.muted,
                        fontFamily: tokens.font.mono,
                      }}
                    >
                      {seed}
                    </Typography>
                  </Box>
                </Box>
              </SwiperSlide>
            ))}
          </Swiper>
        </Box>
      </Box>
    </Section>
  );
}

function IconSwiperButton({
  className,
  children,
}: {
  className: string;
  children: ReactNode;
}) {
  return (
    <Button
      className={className}
      aria-label={
        className.includes("prev") ? "Previous templates" : "Next templates"
      }
      sx={{
        minWidth: 34,
        width: 34,
        height: 34,
        borderRadius: "50%",
        border: `1px solid ${tokens.color.border.subtle}`,
        color: tokens.color.accent.violet,
        bgcolor: "rgba(255,255,255,0.10)",
        p: 0,
        "&:hover": {
          bgcolor: `${tokens.color.accent.violet}16`,
        },
      }}
    >
      {children}
    </Button>
  );
}

function VscodeExtensionBand({ timing }: { timing: OrbitalTiming }) {
  const light = timing === "light";
  const c = {
    text: light ? "#080b3f" : tokens.color.text.primary,
    secondary: light ? "#5e6689" : tokens.color.text.secondary,
    border: light ? "rgba(127,77,255,0.18)" : tokens.color.border.subtle,
    surface: light ? "rgba(255,255,255,0.80)" : "rgba(16,18,44,0.72)",
    inset: light ? "#ffffff" : tokens.color.bg.inset,
  };
  return (
    <Section sx={{ py: { xs: 3, md: 4 } }}>
      <Stack
        direction={{ xs: "column", lg: "row" }}
        sx={{
          alignItems: "stretch",
          border: `1px solid ${c.border}`,
          borderRadius: `${tokens.radius.sm}px`,
          bgcolor: c.surface,
          boxShadow: light
            ? "0 24px 80px rgba(103,65,180,0.09)"
            : "0 24px 80px rgba(0,0,0,0.22)",
          overflow: "hidden",
        }}
      >
        <Stack spacing={1.6} sx={{ flex: 1, p: { xs: 2.4, md: 3.2 } }}>
          <Typography
            sx={{
              color: tokens.color.accent.violet,
              fontSize: 12,
              fontWeight: 950,
              textTransform: "uppercase",
            }}
          >
            VS Code extension
          </Typography>
          <Typography
            sx={{
              color: c.text,
              fontSize: { xs: 28, md: 38 },
              fontWeight: 950,
              lineHeight: 1.08,
            }}
          >
            Keep the AI build loop inside your editor.
          </Typography>
          <Typography
            sx={{
              color: c.secondary,
              fontSize: 16,
              lineHeight: 1.55,
              maxWidth: 680,
            }}
          >
            Review generated patches, inspect gates, open previews and ask
            IronFlyer to fix diagnostics without leaving VS Code.
          </Typography>
          <Stack
            direction="row"
            useFlexGap
            flexWrap="wrap"
            sx={{ gap: 1, pt: 0.4 }}
          >
            {["Patch diffs", "Live gates", "Run output", "Secure sign-in"].map(
              (item) => (
                <Box
                  key={item}
                  sx={{
                    border: `1px solid ${c.border}`,
                    borderRadius: 999,
                    color: c.secondary,
                    px: 1.3,
                    py: 0.55,
                    fontSize: 12.5,
                    fontWeight: 800,
                  }}
                >
                  {item}
                </Box>
              ),
            )}
          </Stack>
          <Stack
            direction={{ xs: "column", sm: "row" }}
            spacing={1.2}
            sx={{ pt: 0.8 }}
          >
            <Button
              component={Link}
              href="/vscode"
              variant="contained"
              endIcon={<ArrowForwardRounded />}
            >
              See the extension
            </Button>
            <Button
              component={Link}
              href="/studio"
              variant="outlined"
              sx={{ color: c.text, borderColor: c.border }}
            >
              Open Studio
            </Button>
          </Stack>
        </Stack>
        <Box
          sx={{
            borderLeft: { lg: `1px solid ${c.border}` },
            bgcolor: c.inset,
            display: "grid",
            flex: { lg: "0 0 420px" },
            minHeight: { xs: 260, lg: "auto" },
            p: 2,
            placeItems: "center",
          }}
        >
          <Box
            sx={{
              border: `1px solid ${c.border}`,
              borderRadius: `${tokens.radius.sm}px`,
              bgcolor: light ? "#f7f4ff" : "#080918",
              color: c.text,
              fontFamily: tokens.font.mono,
              overflow: "hidden",
              width: "100%",
            }}
          >
            <Stack
              direction="row"
              sx={{
                borderBottom: `1px solid ${c.border}`,
                gap: 0.7,
                px: 1.5,
                py: 1,
              }}
            >
              {["#ff5f57", "#ffbd2e", "#28c840"].map((color) => (
                <Box
                  key={color}
                  sx={{
                    width: 10,
                    height: 10,
                    borderRadius: "50%",
                    bgcolor: color,
                  }}
                />
              ))}
              <Box sx={{ flex: 1 }} />
              <Typography
                sx={{
                  color: c.secondary,
                  fontFamily: tokens.font.mono,
                  fontSize: 11,
                }}
              >
                IronFlyer
              </Typography>
            </Stack>
            {[
              ["Projects", "ClientFlow pinned"],
              ["Patches", "4 changes ready"],
              ["Gates", "Build passed"],
              ["Preview", "Live"],
            ].map(([label, value]) => (
              <Stack
                key={label}
                direction="row"
                sx={{
                  alignItems: "center",
                  borderBottom: `1px solid ${c.border}`,
                  px: 1.6,
                  py: 1.1,
                }}
              >
                <CodeRounded
                  sx={{
                    color: tokens.color.accent.violet,
                    fontSize: 17,
                    mr: 1,
                  }}
                />
                <Typography
                  sx={{ flex: 1, fontFamily: tokens.font.mono, fontSize: 12.5 }}
                >
                  {label}
                </Typography>
                <Typography
                  sx={{
                    color: c.secondary,
                    fontFamily: tokens.font.mono,
                    fontSize: 12,
                  }}
                >
                  {value}
                </Typography>
              </Stack>
            ))}
          </Box>
        </Box>
      </Stack>
    </Section>
  );
}

function TestimonialBand({ timing }: { timing: OrbitalTiming }) {
  const c = homeTone(timing);
  return (
    <Section sx={{ py: { xs: 4, md: 4.8 } }}>
      <Box
        sx={{
          position: "relative",
          p: { xs: 3, md: 4 },
          borderRadius: 2,
          border: `1px solid ${c.strong}`,
          bgcolor: c.surface,
          overflow: "hidden",
          boxShadow: `0 18px 80px ${tokens.color.accent.purple}20`,
        }}
      >
        <MiniPrism sx={{ right: 30, bottom: 22 }} />
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "1.6fr 1fr" },
            gap: 3,
            alignItems: "center",
          }}
        >
          <Box>
            <Typography
              sx={{
                fontFamily: tokens.font.mono,
                fontSize: 10.5,
                color: tokens.color.accent.violet,
                fontWeight: 800,
                textTransform: "uppercase",
              }}
            >
              How teams build faster
            </Typography>
            <Typography
              sx={{
                mt: 1,
                maxWidth: 690,
                fontSize: { xs: 24, md: 32 },
                lineHeight: 1.08,
                fontWeight: 900,
                color: c.text,
              }}
            >
              "We shipped our client portal in a week with Ironflyer. The AI
              plan was spot-on and the code was clean and easy to extend."
            </Typography>
          </Box>
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: "repeat(3, 1fr)",
              gap: 2,
            }}
          >
            {[
              ["7 days", "To production"],
              ["92%", "Code kept"],
              ["3x", "Faster delivery"],
            ].map(([value, label]) => (
              <Stack key={label} spacing={0.5}>
                <Typography
                  sx={{
                    color: tokens.color.accent.violet,
                    fontSize: 28,
                    fontWeight: 900,
                  }}
                >
                  {value}
                </Typography>
                <Typography sx={{ color: c.secondary, fontSize: 12 }}>
                  {label}
                </Typography>
              </Stack>
            ))}
          </Box>
        </Box>
      </Box>
    </Section>
  );
}

function PricingCards({ timing }: { timing: OrbitalTiming }) {
  const c = homeTone(timing);
  const plans = [
    [
      "Free",
      "$0",
      "Forever",
      ["1 workspace", "2 projects", "Community support"],
      "Get started",
    ],
    [
      "Pro",
      "$29",
      "Per user / month",
      ["Unlimited projects", "AI templates", "Email support"],
      "Start free trial",
    ],
    [
      "Team",
      "$79",
      "Per user / month",
      ["SSO & RBAC", "Environments", "Priority support"],
      "Start free trial",
    ],
    [
      "Enterprise",
      "Custom",
      "Let's talk",
      ["Advanced security", "SLA & support", "Custom integrations"],
      "Contact sales",
    ],
  ];
  return (
    <Section sx={{ py: { xs: 4, md: 5.2 } }}>
      <Stack spacing={3} alignItems="center">
        <Stack spacing={1} alignItems="center">
          <Typography
            sx={{
              color: c.text,
              fontSize: { xs: 25, md: 32 },
              fontWeight: 900,
            }}
          >
            Simple, transparent pricing
          </Typography>
          <Typography sx={{ color: c.secondary, fontSize: 13 }}>
            Start free. Scale on your terms.
          </Typography>
        </Stack>
        <Box
          sx={{
            width: "100%",
            maxWidth: 900,
            display: "grid",
            gridTemplateColumns: {
              xs: "1fr",
              sm: "repeat(2, 1fr)",
              lg: "repeat(4, 1fr)",
            },
            gap: 1.5,
          }}
        >
          {plans.map(([name, price, cadence, features, cta], index) => (
            <Box
              key={name as string}
              sx={{
                position: "relative",
                p: 2.2,
                borderRadius: 1,
                border: `1px solid ${index === 2 ? tokens.color.accent.violet : c.border}`,
                bgcolor: c.surface,
              }}
            >
              {index === 2 && (
                <Box
                  sx={{
                    position: "absolute",
                    right: 12,
                    top: 12,
                    px: 0.8,
                    py: 0.25,
                    borderRadius: 999,
                    bgcolor: tokens.color.accent.violet,
                    color: "#fff",
                    fontSize: 10,
                    fontWeight: 900,
                  }}
                >
                  Most popular
                </Box>
              )}
              <Typography sx={{ color: c.text, fontSize: 12, fontWeight: 900 }}>
                {name}
              </Typography>
              <Typography
                sx={{
                  mt: 1.4,
                  fontSize: price === "Custom" ? 29 : 34,
                  fontWeight: 900,
                  lineHeight: 1,
                  color: c.text,
                }}
              >
                {price}
              </Typography>
              <Typography
                sx={{
                  mt: 0.7,
                  color: c.secondary,
                  fontSize: 11,
                }}
              >
                {cadence}
              </Typography>
              <Stack spacing={0.7} sx={{ my: 2.2 }}>
                {(features as string[]).map((feature) => (
                  <Typography
                    key={feature}
                    sx={{ color: c.secondary, fontSize: 12 }}
                  >
                    - {feature}
                  </Typography>
                ))}
              </Stack>
              <Button
                component={Link}
                href="/signup"
                fullWidth
                variant={index === 2 ? "contained" : "text"}
                color="primary"
                sx={{
                  bgcolor:
                    index === 2 ? undefined : `${tokens.color.accent.purple}1f`,
                }}
              >
                {cta}
              </Button>
            </Box>
          ))}
        </Box>
      </Stack>
    </Section>
  );
}

function ProofFooterBand({ timing }: { timing: OrbitalTiming }) {
  const c = homeTone(timing);
  const rows = [
    [
      "Build in natural language",
      "Shorten the gap from idea to working product.",
    ],
    ["Ship with confidence", "Built-in reviews, tests and observability."],
    ["Own your code", "Export anytime. You are never locked in."],
    ["Secure by default", "Enterprise-grade security and compliance."],
  ];
  return (
    <Section sx={{ py: { xs: 2.5, md: 3.8 } }}>
      <Box
        sx={{
          p: 2.2,
          borderRadius: 2,
          border: `1px solid ${c.border}`,
          bgcolor: c.surface,
          display: "grid",
          gridTemplateColumns: {
            xs: "1fr",
            sm: "repeat(2, 1fr)",
            lg: "repeat(4, 1fr)",
          },
          gap: 2,
        }}
      >
        {rows.map(([title, body]) => (
          <Stack key={title} direction="row" spacing={1.2}>
            <BoltRounded
              sx={{ fontSize: 16, color: tokens.color.accent.violet, mt: 0.25 }}
            />
            <Box>
              <Typography
                sx={{ color: c.text, fontSize: 12.5, fontWeight: 900 }}
              >
                {title}
              </Typography>
              <Typography
                sx={{
                  mt: 0.4,
                  fontSize: 11.5,
                  color: c.secondary,
                }}
              >
                {body}
              </Typography>
            </Box>
          </Stack>
        ))}
      </Box>
    </Section>
  );
}

function FaqShowcase({ timing }: { timing: OrbitalTiming }) {
  const c = homeTone(timing);
  const questions = [
    [
      "Can I export the code?",
      "Yes. Export clean React and TypeScript when the project is ready, including the generated app structure.",
    ],
    [
      "How does pricing work?",
      "Start free, then upgrade when you need more projects, private workspaces, team controls or production deploys.",
    ],
    [
      "Is my data secure?",
      "Projects stay scoped to your workspace. Team roles, audit logs and enterprise controls are available on paid plans.",
    ],
    [
      "Do you offer onboarding?",
      "Yes. Teams can get a guided setup for templates, roles, deploy targets and VS Code workflows.",
    ],
  ];
  return (
    <Section sx={{ py: { xs: 4, md: 5.2 } }}>
      <Stack spacing={3}>
        <Typography
          sx={{
            textAlign: "center",
            fontSize: { xs: 25, md: 32 },
            fontWeight: 900,
            color: c.text,
          }}
        >
          Frequently asked questions
        </Typography>
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
            gap: 2,
          }}
        >
          <Stack spacing={1.1}>
            {questions.map(([question, answer]) => (
              <Accordion
                key={question}
                disableGutters
                sx={{
                  borderRadius: 1,
                  border: `1px solid ${c.border}`,
                  bgcolor: c.surface,
                  boxShadow: "none",
                  color: c.text,
                  overflow: "hidden",
                  "&::before": { display: "none" },
                }}
              >
                <AccordionSummary
                  expandIcon={
                    <ExpandMoreRounded
                      sx={{ color: c.secondary, fontSize: 20 }}
                    />
                  }
                  sx={{
                    minHeight: 54,
                    px: 2,
                    "& .MuiAccordionSummary-content": { my: 1.2 },
                  }}
                >
                  <Typography
                    sx={{ color: c.text, fontSize: 13, fontWeight: 900 }}
                  >
                    {question}
                  </Typography>
                </AccordionSummary>
                <AccordionDetails sx={{ px: 2, pt: 0, pb: 2 }}>
                  <Typography
                    sx={{ color: c.secondary, fontSize: 13, lineHeight: 1.55 }}
                  >
                    {answer}
                  </Typography>
                </AccordionDetails>
              </Accordion>
            ))}
          </Stack>
          <Box
            sx={{
              p: 2,
              borderRadius: 1,
              border: `1px solid ${c.border}`,
              bgcolor: c.inset,
              position: "relative",
              overflow: "hidden",
            }}
          >
            <MiniPrism sx={{ right: 22, bottom: 18 }} />
            <Stack
              direction="row"
              justifyContent="space-between"
              sx={{
                mb: 2,
                fontFamily: tokens.font.mono,
                color: c.muted,
                fontSize: 11,
              }}
            >
              <span>App.tsx</span>
              <span>schema.prisma</span>
            </Stack>
            <Typography
              component="pre"
              sx={{
                m: 0,
                p: 0,
                border: 0,
                bgcolor: `${tokens.color.bg.base}00`,
                color: c.secondary,
                fontFamily: tokens.font.mono,
                fontSize: 12,
                lineHeight: 1.7,
                whiteSpace: "pre-wrap",
              }}
            >
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

function FinalShipCTA({ timing }: { timing: OrbitalTiming }) {
  const c = homeTone(timing);
  return (
    <Section sx={{ py: { xs: 3.5, md: 4.2 } }}>
      <Box
        sx={{
          p: { xs: 2, md: 3 },
          borderRadius: 2,
          border: `1px solid ${c.border}`,
          bgcolor:
            timing === "light"
              ? "rgba(236,222,255,0.78)"
              : `${tokens.color.accent.purple}2e`,
          display: "grid",
          gridTemplateColumns: { xs: "1fr", md: "160px 1fr auto" },
          gap: { xs: 2, md: 3 },
          alignItems: "center",
        }}
      >
        <Box
          sx={{
            height: 98,
            borderRadius: 1,
            backgroundImage: "url('/market/data-flow.jpg')",
            backgroundSize: "cover",
            backgroundPosition: "center",
            border: `1px solid ${tokens.color.border.subtle}`,
          }}
        />
        <Box>
          <Typography
            sx={{
              color: c.text,
              fontSize: { xs: 22, md: 28 },
              fontWeight: 900,
            }}
          >
            Stop stitching tools. Start shipping products.
          </Typography>
          <Typography sx={{ mt: 0.6, color: c.secondary, fontSize: 13 }}>
            One prompt. One workspace. One launch.
          </Typography>
        </Box>
        <Stack direction={{ xs: "column", sm: "row" }} spacing={1.2}>
          <Button
            component={Link}
            href="/signup"
            variant="contained"
            color="primary"
            endIcon={<ArrowForwardRounded sx={{ fontSize: 16 }} />}
          >
            Start building for free
          </Button>
          <Button
            component={Link}
            href="/enterprise"
            sx={{ color: c.text, fontWeight: 800 }}
          >
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
        ["Product", "/"],
        ["Templates", "/templates"],
        ["VS Code", "/vscode"],
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
    <Section
      sx={{
        pt: { xs: 4, md: 5 },
        pb: { xs: 4, md: 4.5 },
        borderTop: `1px solid ${tokens.color.border.subtle}`,
      }}
    >
      <Stack
        direction={{ xs: "column", md: "row" }}
        spacing={{ xs: 4, md: 6 }}
        sx={{ alignItems: { md: "flex-start" } }}
      >
        <Stack spacing={1.5} sx={{ flex: 1, maxWidth: 360 }}>
          <BrandLogo inverse size={28} href="/" />
          <Typography
            sx={{
              color: tokens.color.text.secondary,
              fontSize: 13,
              lineHeight: 1.55,
            }}
          >
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
              "&:hover": {
                color: tokens.color.text.primary,
                bgcolor: "transparent",
              },
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
