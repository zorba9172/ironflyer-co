"use client";

// /start — non-coder onboarding. The job: take someone with zero
// engineering background from "I have an idea" to "I clicked into the
// Studio with my idea pre-filled" in under a minute, while making the
// production-discipline promise (you approve every change, gates
// protect you, no surprise bills) legible in plain language.
//
// Self-contained on purpose: no shared marketing component, no edits to
// tracked files. It reuses the Studio's existing sessionStorage
// contract — writing the chosen example to "ironflyer.pendingPrompt.v1"
// then routing to /studio, where StudioPage already reads that key and
// pre-fills the composer (see clients/web/app/studio/page.tsx). Light/
// dark via ?theme=. Locked palette only.

import {
  AutoAwesomeRounded,
  CheckCircleRounded,
  RateReviewRounded,
  RocketLaunchRounded,
  VisibilityRounded,
} from "@mui/icons-material";
import { Box, Button, Stack, Typography } from "@mui/material";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, type ReactNode } from "react";
import { tokens } from "../../src/theme";

// Must match StudioPage's PENDING_PROMPT_KEY so the composer pre-fills.
const PENDING_PROMPT_KEY = "ironflyer.pendingPrompt.v1";

interface Step {
  n: string;
  title: string;
  body: string;
  icon: ReactNode;
}

const STEPS: Step[] = [
  {
    n: "01",
    title: "Describe it in plain words",
    body: "Tell the Studio what you want — like talking to a contractor. No diagrams, no jargon, no setup.",
    icon: <AutoAwesomeRounded />,
  },
  {
    n: "02",
    title: "Watch it get built",
    body: "Screens, data and logic are generated together. A live preview shows the real app as it takes shape.",
    icon: <VisibilityRounded />,
  },
  {
    n: "03",
    title: "Review every change",
    body: "Each change is a reviewable patch you approve or reject. Security gates run automatically and block anything unsafe — you don't have to know what to look for.",
    icon: <RateReviewRounded />,
  },
  {
    n: "04",
    title: "Publish when it's ready",
    body: "One lane from preview to live. The deploy gate refuses to ship code with critical security findings — so what goes out is safe by default.",
    icon: <RocketLaunchRounded />,
  },
];

// Non-coder-friendly example ideas. Each one is a real, buildable
// product framed in business language, not engineering language.
const EXAMPLES: { label: string; prompt: string }[] = [
  {
    label: "Booking system for my clinic",
    prompt:
      "Build an appointment booking system for a medical clinic: patients can see available slots, book and cancel, staff manage the calendar and patient list, and reminders go out automatically. Include sign-in and role-based access for staff vs patients.",
  },
  {
    label: "Inventory tracker for my store",
    prompt:
      "Build an inventory management app for a retail store: track products, stock levels, suppliers and reorder points, with low-stock alerts, a sales log, and a dashboard showing what's selling. Include staff accounts with permissions.",
  },
  {
    label: "Client portal for my agency",
    prompt:
      "Build a client portal for a services agency: clients log in to see their projects, files, invoices and approvals; the team manages everything from an admin view. Include role-based access, activity history and notifications.",
  },
  {
    label: "Membership site for my community",
    prompt:
      "Build a membership website for a community: members sign up, pay a subscription, access member-only content and events, and update their profile. Include an admin dashboard for managing members and content.",
  },
];

export default function StartPage() {
  return (
    <Suspense fallback={null}>
      <StartPageInner />
    </Suspense>
  );
}

function StartPageInner() {
  const router = useRouter();
  const search = useSearchParams();
  const light = search?.get("theme") !== "dark";

  const text = light ? "#080b3f" : tokens.color.text.primary;
  const secondary = light ? "#5d6588" : tokens.color.text.secondary;
  const muted = light ? "#8087a4" : tokens.color.text.muted;
  const bg = light ? "#fbfaff" : tokens.color.bg.base;
  const surface = light ? "#ffffff" : tokens.color.bg.surfaceRaised;
  const border = light ? "rgba(127,77,255,0.18)" : tokens.color.border.subtle;
  const chipBg = light ? "rgba(127,77,255,0.08)" : "rgba(127,77,255,0.14)";

  const themeQS = light ? "?theme=light" : "";

  const startWith = (prompt: string) => {
    try {
      window.sessionStorage.setItem(PENDING_PROMPT_KEY, prompt);
    } catch {
      // private mode / quota — the Studio still opens, just without
      // the pre-filled idea. Non-fatal.
    }
    router.push(`/studio${themeQS}`);
  };

  return (
    <Box
      sx={{
        bgcolor: bg,
        color: text,
        minHeight: "100vh",
        backgroundImage: light
          ? "radial-gradient(820px 440px at 84% 8%, rgba(231,77,202,0.12), transparent 72%), radial-gradient(760px 380px at 6% 20%, rgba(139,77,255,0.10), transparent 70%)"
          : "radial-gradient(820px 440px at 84% 8%, rgba(177,91,255,0.20), transparent 72%), radial-gradient(760px 380px at 6% 20%, rgba(37,112,255,0.12), transparent 70%)",
      }}
    >
      <Box
        sx={{
          maxWidth: 1080,
          mx: "auto",
          px: { xs: 2.5, md: 5 },
          py: { xs: 6, md: 9 },
        }}
      >
        {/* Hero */}
        <Stack spacing={2.4} sx={{ maxWidth: 820, mb: { xs: 5, md: 7 } }}>
          <Typography
            sx={{
              color: tokens.color.accent.violet,
              fontFamily: tokens.font.mono,
              fontSize: 12,
              fontWeight: 800,
              letterSpacing: 1.6,
              textTransform: "uppercase",
            }}
          >
            Start here — no code required
          </Typography>
          <Typography
            sx={{
              color: text,
              fontSize: { xs: 32, md: 50 },
              fontWeight: 900,
              letterSpacing: -0.7,
              lineHeight: 1.04,
            }}
          >
            Describe the software you need.{" "}
            <Box component="span" sx={{ color: tokens.color.accent.violet }}>
              We build it, safely.
            </Box>
          </Typography>
          <Typography
            sx={{
              color: secondary,
              fontSize: { xs: 16, md: 18.5 },
              lineHeight: 1.55,
              maxWidth: 720,
            }}
          >
            You don&apos;t need to know how to code. You describe what you want
            in plain language; Ironflyer builds a real, working app and shows
            it to you live. Every change is reviewed, every security risk is
            caught before it ships, and you never get a surprise bill. Pick an
            idea below to start in seconds — or open the Studio and write your
            own.
          </Typography>
          <Stack direction="row" spacing={1.5} flexWrap="wrap" sx={{ gap: 1.5 }}>
            <Button
              variant="contained"
              color="primary"
              component={Link}
              href={`/studio${themeQS}`}
            >
              Open the Studio
            </Button>
            <Button
              variant="outlined"
              component={Link}
              href={`/compare${themeQS}`}
              sx={{
                borderColor: light
                  ? "rgba(127,77,255,0.38)"
                  : tokens.color.border.strong,
                color: text,
              }}
            >
              How we compare
            </Button>
          </Stack>
        </Stack>

        {/* Example ideas */}
        <Typography
          sx={{
            color: text,
            fontSize: { xs: 20, md: 24 },
            fontWeight: 900,
            mb: 0.6,
          }}
        >
          Start from a real idea.
        </Typography>
        <Typography
          sx={{ color: secondary, fontSize: 15, lineHeight: 1.5, mb: 3 }}
        >
          Click one and the Studio opens with the idea already written for you.
          You can change every word before anything is built.
        </Typography>
        <Box
          sx={{
            display: "grid",
            gap: 2,
            gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" },
            mb: { xs: 5, md: 7 },
          }}
        >
          {EXAMPLES.map((ex) => (
            <Box
              key={ex.label}
              role="button"
              tabIndex={0}
              onClick={() => startWith(ex.prompt)}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  startWith(ex.prompt);
                }
              }}
              sx={{
                bgcolor: surface,
                border: `1px solid ${border}`,
                borderRadius: 2,
                cursor: "pointer",
                p: 2.6,
                transition: `transform ${tokens.motion.fast} ease, border-color ${tokens.motion.fast} ease`,
                "&:hover": {
                  borderColor: tokens.color.accent.violet,
                  transform: "translateY(-2px)",
                },
                "&:focus-visible": {
                  outline: `2px solid ${tokens.color.accent.violet}`,
                  outlineOffset: 2,
                },
              }}
            >
              <Typography
                sx={{ color: text, fontSize: 17, fontWeight: 800, mb: 0.8 }}
              >
                {ex.label}
              </Typography>
              <Typography
                sx={{ color: muted, fontSize: 13.5, lineHeight: 1.5 }}
              >
                {ex.prompt}
              </Typography>
            </Box>
          ))}
        </Box>

        {/* How it works */}
        <Typography
          sx={{
            color: text,
            fontSize: { xs: 20, md: 24 },
            fontWeight: 900,
            mb: 3,
          }}
        >
          What happens after you describe it.
        </Typography>
        <Box
          sx={{
            display: "grid",
            gap: 2,
            gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr", md: "repeat(4, 1fr)" },
            mb: { xs: 5, md: 7 },
          }}
        >
          {STEPS.map((step) => (
            <Box
              key={step.n}
              sx={{
                bgcolor: surface,
                border: `1px solid ${border}`,
                borderRadius: 2,
                display: "flex",
                flexDirection: "column",
                gap: 1.1,
                p: 2.4,
              }}
            >
              <Stack
                direction="row"
                alignItems="center"
                justifyContent="space-between"
              >
                <Box
                  sx={{
                    alignItems: "center",
                    bgcolor: chipBg,
                    borderRadius: 1.5,
                    color: tokens.color.accent.violet,
                    display: "flex",
                    height: 38,
                    justifyContent: "center",
                    width: 38,
                  }}
                >
                  {step.icon}
                </Box>
                <Typography
                  sx={{
                    color: muted,
                    fontFamily: tokens.font.mono,
                    fontSize: 13,
                    fontWeight: 800,
                    letterSpacing: 1,
                  }}
                >
                  {step.n}
                </Typography>
              </Stack>
              <Typography
                sx={{ color: text, fontSize: 16, fontWeight: 800, mt: 0.4 }}
              >
                {step.title}
              </Typography>
              <Typography
                sx={{ color: secondary, fontSize: 13.5, lineHeight: 1.5 }}
              >
                {step.body}
              </Typography>
            </Box>
          ))}
        </Box>

        {/* Reassurance band */}
        <Box
          sx={{
            bgcolor: surface,
            border: `1px solid ${border}`,
            borderRadius: 2,
            px: { xs: 3, md: 5 },
            py: { xs: 3.5, md: 4.5 },
            mb: { xs: 5, md: 6 },
          }}
        >
          <Typography
            sx={{ color: text, fontSize: { xs: 19, md: 23 }, fontWeight: 900 }}
          >
            You stay in control the whole way.
          </Typography>
          <Stack
            spacing={1.4}
            sx={{ mt: 2 }}
          >
            {[
              "Nothing is built until you send your idea — and nothing is charged until you approve a budget.",
              "Every change is shown to you as a clear before/after. Approve it or reject it.",
              "Security scanning runs on every step. Leaked passwords, vulnerable code and risky changes are blocked automatically.",
              "A prepaid wallet means you can never be billed more than you put in.",
            ].map((line) => (
              <Stack key={line} direction="row" spacing={1.2} alignItems="flex-start">
                <CheckCircleRounded
                  sx={{ color: tokens.color.brand.mint, fontSize: 20, mt: 0.2 }}
                />
                <Typography
                  sx={{ color: secondary, fontSize: 15, lineHeight: 1.5 }}
                >
                  {line}
                </Typography>
              </Stack>
            ))}
          </Stack>
        </Box>

        {/* Engineer escape hatch */}
        <Stack
          direction={{ xs: "column", sm: "row" }}
          spacing={2}
          alignItems={{ xs: "flex-start", sm: "center" }}
          justifyContent="space-between"
          sx={{
            border: `1px dashed ${border}`,
            borderRadius: 2,
            px: { xs: 2.5, md: 3.5 },
            py: 2.5,
          }}
        >
          <Box>
            <Typography sx={{ color: text, fontSize: 15.5, fontWeight: 800 }}>
              You&apos;re an engineer?
            </Typography>
            <Typography
              sx={{ color: muted, fontSize: 13.5, lineHeight: 1.5, mt: 0.3 }}
            >
              Open the code, read the gate verdicts, inspect the patches, or
              bring it all into your local VS Code. The hood opens in one click.
            </Typography>
          </Box>
          <Stack direction="row" spacing={1} flexShrink={0}>
            <Button
              variant="outlined"
              component={Link}
              href={`/appsec${themeQS}`}
              size="small"
              sx={{
                borderColor: light
                  ? "rgba(127,77,255,0.38)"
                  : tokens.color.border.strong,
                color: text,
              }}
            >
              AppSec gates
            </Button>
            <Button
              variant="outlined"
              component={Link}
              href={`/vscode${themeQS}`}
              size="small"
              sx={{
                borderColor: light
                  ? "rgba(127,77,255,0.38)"
                  : tokens.color.border.strong,
                color: text,
              }}
            >
              VS Code Extension
            </Button>
          </Stack>
        </Stack>
      </Box>
    </Box>
  );
}
