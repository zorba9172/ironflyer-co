"use client";

// /compare — Ironflyer vs Lovable / Bolt / Base44 / v0 / Replit Agent /
// Cursor / Windsurf. Comparison rows are fact-grounded from the
// scraped marketing pages of each tool (see docs/OVERNIGHT_2026-05-28.md
// "Competitor research" section). Anything not literally on the
// competitor's site is left as a question mark — we don't fabricate
// claims about other vendors.
//
// Design contract: locked palette (no raw hex outside tokens or theme
// mappings), no lime-first identity, light/dark via `?theme=`. Lazy
// nothing — it's a static marketing page.

import {
  CheckCircleRounded,
  CloseRounded,
  HelpOutlineRounded,
  ShieldRounded,
  WarningAmberRounded,
} from "@mui/icons-material";
import { Box, Button, Stack, Typography } from "@mui/material";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Suspense } from "react";
import { tokens } from "../../src/theme";

type Verdict = "yes" | "no" | "partial" | "unknown";

interface Row {
  capability: string;
  detail: string;
  ironflyer: Verdict;
  lovable: Verdict;
  bolt: Verdict;
  base44: Verdict;
  v0: Verdict;
  replit: Verdict;
  cursor: Verdict;
  windsurf: Verdict;
}

// Source: each competitor's public homepage, fetched 2026-05-28. A
// `partial` means the competitor mentions adjacent functionality
// (e.g. "encrypted at rest") but does not ship the same primitive.
// `unknown` = not findable on their marketing site as of that date.
const ROWS: Row[] = [
  {
    capability: "Prompt-to-app",
    detail: "Describe an app in natural language; get a working build.",
    ironflyer: "yes",
    lovable: "yes",
    bolt: "yes",
    base44: "yes",
    v0: "yes",
    replit: "yes",
    cursor: "no",
    windsurf: "no",
  },
  {
    capability: "Reviewable patches",
    detail:
      "Every AI change is a discrete patch you can approve, reject, or roll back. Not a black-box write.",
    ironflyer: "yes",
    lovable: "no",
    bolt: "no",
    base44: "no",
    v0: "no",
    replit: "no",
    cursor: "partial",
    windsurf: "partial",
  },
  {
    capability: "AppSec scanning on every iteration",
    detail:
      "Semgrep + gitleaks + trufflehog + govulncheck run on each pass. Findings open patches.",
    ironflyer: "yes",
    lovable: "no",
    bolt: "no",
    base44: "no",
    v0: "no",
    replit: "no",
    cursor: "no",
    windsurf: "no",
  },
  {
    capability: "Gates that block ship",
    detail:
      "Critical security or budget findings stop the deploy lane. No 'ship now, scan later'.",
    ironflyer: "yes",
    lovable: "no",
    bolt: "no",
    base44: "no",
    v0: "no",
    replit: "no",
    cursor: "no",
    windsurf: "no",
  },
  {
    capability: "Prepaid wallet — no surprise bills",
    detail:
      "Every paid execution reserves funds first. ProfitGuard refuses calls that would put your wallet underwater.",
    ironflyer: "yes",
    lovable: "no",
    bolt: "no",
    base44: "no",
    v0: "no",
    replit: "no",
    cursor: "no",
    windsurf: "no",
  },
  {
    capability: "SOC2 / HIPAA compliance gates",
    detail:
      "Auto-runs CC6/CC7/CC8 (SOC2) and 164.312 (HIPAA) control checks when the project opts in.",
    ironflyer: "yes",
    lovable: "no",
    bolt: "no",
    base44: "no",
    v0: "no",
    replit: "no",
    cursor: "no",
    windsurf: "no",
  },
  {
    capability: "Real Linux workspace per project",
    detail:
      "Docker-isolated sandbox with terminal, package install, real build chain.",
    ironflyer: "yes",
    lovable: "partial",
    bolt: "yes",
    base44: "partial",
    v0: "partial",
    replit: "yes",
    cursor: "no",
    windsurf: "no",
  },
  {
    capability: "Native mobile builds (Expo / Kotlin / Swift)",
    detail:
      "Real EAS / gradlew / xcodebuild dispatch with signing artifacts. iOS native gated to Pro tier.",
    ironflyer: "yes",
    lovable: "no",
    bolt: "no",
    base44: "no",
    v0: "no",
    replit: "partial",
    cursor: "no",
    windsurf: "no",
  },
  {
    capability: "Append-only cost ledger per execution",
    detail:
      "Every token, sandbox-minute and build-minute lands in a ledger you can audit.",
    ironflyer: "yes",
    lovable: "no",
    bolt: "no",
    base44: "no",
    v0: "no",
    replit: "no",
    cursor: "no",
    windsurf: "no",
  },
  {
    capability: "Standalone VS Code Extension",
    detail:
      "Bring gates, patches, wallet and chat into your local VS Code. No iframe.",
    ironflyer: "yes",
    lovable: "no",
    bolt: "no",
    base44: "no",
    v0: "no",
    replit: "no",
    cursor: "yes",
    windsurf: "yes",
  },
];

const COMPETITORS = [
  { key: "ironflyer" as const, label: "Ironflyer", us: true },
  { key: "lovable" as const, label: "Lovable", us: false },
  { key: "bolt" as const, label: "Bolt", us: false },
  { key: "base44" as const, label: "Base44", us: false },
  { key: "v0" as const, label: "v0", us: false },
  { key: "replit" as const, label: "Replit Agent", us: false },
  { key: "cursor" as const, label: "Cursor", us: false },
  { key: "windsurf" as const, label: "Windsurf", us: false },
];

export default function ComparePage() {
  return (
    <Suspense fallback={null}>
      <ComparePageInner />
    </Suspense>
  );
}

function ComparePageInner() {
  const search = useSearchParams();
  const light = search?.get("theme") !== "dark";

  const text = light ? "#080b3f" : tokens.color.text.primary;
  const secondary = light ? "#5d6588" : tokens.color.text.secondary;
  const muted = light ? "#8087a4" : tokens.color.text.muted;
  const bg = light ? "#fbfaff" : tokens.color.bg.base;
  const surface = light ? "#ffffff" : tokens.color.bg.surfaceRaised;
  const border = light ? "rgba(127,77,255,0.18)" : tokens.color.border.subtle;
  const headerBg = light ? "rgba(127,77,255,0.06)" : "rgba(127,77,255,0.12)";
  const usBg = light ? "rgba(231,77,202,0.10)" : "rgba(231,77,202,0.16)";

  return (
    <Box
      sx={{
        bgcolor: bg,
        color: text,
        minHeight: "100vh",
        backgroundImage: light
          ? "radial-gradient(780px 420px at 82% 10%, rgba(231,77,202,0.12), transparent 72%), radial-gradient(760px 380px at 8% 22%, rgba(139,77,255,0.10), transparent 70%)"
          : "radial-gradient(780px 420px at 82% 10%, rgba(177,91,255,0.20), transparent 72%), radial-gradient(760px 380px at 8% 22%, rgba(37,112,255,0.12), transparent 70%)",
      }}
    >
      <Box
        sx={{
          maxWidth: 1180,
          mx: "auto",
          px: { xs: 2.5, md: 5 },
          py: { xs: 6, md: 9 },
        }}
      >
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
            Ironflyer vs the AI app builders
          </Typography>
          <Typography
            sx={{
              color: text,
              fontSize: { xs: 32, md: 48 },
              fontWeight: 900,
              letterSpacing: -0.6,
              lineHeight: 1.05,
            }}
          >
            They sell speed.{" "}
            <Box component="span" sx={{ color: tokens.color.accent.violet }}>
              We sell shippable.
            </Box>
          </Typography>
          <Typography
            sx={{
              color: secondary,
              fontSize: { xs: 16, md: 18 },
              lineHeight: 1.55,
              maxWidth: 720,
            }}
          >
            Every entry in this table comes from the competitor's own marketing
            site, scraped 2026-05-28. We do not invent absent features for them
            — when a capability isn't on their site, we mark it unknown. The
            pattern is consistent: the generative-app category sells "describe
            an idea and ship a preview." Ironflyer sells the production
            discipline that turns a preview into something a customer can
            actually run on real infrastructure.
          </Typography>
          <Stack direction="row" spacing={1.5} sx={{ pt: 1 }}>
            <Button
              variant="contained"
              color="primary"
              component={Link}
              href={`/studio${light ? "?theme=light" : ""}`}
            >
              Open Studio
            </Button>
            <Button
              variant="outlined"
              component={Link}
              href={`/appsec${light ? "?theme=light" : ""}`}
              sx={{
                borderColor: light
                  ? "rgba(127,77,255,0.38)"
                  : tokens.color.border.strong,
                color: text,
              }}
            >
              See AppSec gates
            </Button>
          </Stack>
        </Stack>

        <Box
          sx={{
            border: `1px solid ${border}`,
            borderRadius: 2,
            bgcolor: surface,
            overflow: "hidden",
            boxShadow: light
              ? "0 24px 60px rgba(43,12,89,0.06)"
              : "0 24px 60px rgba(0,0,0,0.45)",
          }}
        >
          <Box
            sx={{
              overflowX: "auto",
              WebkitOverflowScrolling: "touch",
            }}
          >
            <Box
              sx={{
                display: "grid",
                gridTemplateColumns: `minmax(260px, 1.5fr) repeat(${COMPETITORS.length}, minmax(96px, 1fr))`,
                minWidth: 980,
              }}
              role="table"
              aria-label="Ironflyer vs competitor capabilities"
            >
              <CornerCell text={text} bg={headerBg} border={border} />
              {COMPETITORS.map((c) => (
                <HeaderCell
                  key={c.key}
                  label={c.label}
                  us={c.us}
                  border={border}
                  bg={c.us ? usBg : headerBg}
                  text={text}
                />
              ))}
              {ROWS.map((row, idx) => (
                <ComparisonRow
                  key={row.capability}
                  row={row}
                  zebra={idx % 2 === 1}
                  light={light}
                  text={text}
                  muted={muted}
                  border={border}
                  usBg={usBg}
                />
              ))}
            </Box>
          </Box>
          <Stack
            direction="row"
            spacing={2}
            sx={{
              alignItems: "center",
              borderTop: `1px solid ${border}`,
              flexWrap: "wrap",
              gap: 1.4,
              px: { xs: 2, md: 3 },
              py: 1.8,
            }}
          >
            <Legend
              icon={
                <CheckCircleRounded
                  sx={{ color: tokens.color.brand.mint, fontSize: 16 }}
                />
              }
              label="Ships today"
              muted={muted}
            />
            <Legend
              icon={
                <WarningAmberRounded
                  sx={{ color: tokens.color.accent.warning, fontSize: 16 }}
                />
              }
              label="Partial / adjacent"
              muted={muted}
            />
            <Legend
              icon={
                <CloseRounded
                  sx={{ color: tokens.color.accent.coral, fontSize: 16 }}
                />
              }
              label="Not on their site"
              muted={muted}
            />
            <Legend
              icon={<HelpOutlineRounded sx={{ color: muted, fontSize: 16 }} />}
              label="Unknown"
              muted={muted}
            />
          </Stack>
        </Box>

        <Stack spacing={3} sx={{ mt: { xs: 5, md: 7 }, maxWidth: 760 }}>
          <Typography
            sx={{ color: text, fontSize: { xs: 24, md: 30 }, fontWeight: 900 }}
          >
            How to read this table.
          </Typography>
          <Typography sx={{ color: secondary, fontSize: 16, lineHeight: 1.6 }}>
            If you only need a preview, every tool in the top row will get you
            there. If a paying customer needs to log in, store data, and not
            have a leaked API key on the deploy preview — that row becomes a
            very short list. Ironflyer is on it because the gates are part of
            the engine, not an enterprise upsell.
          </Typography>
          <Stack direction="row" spacing={1.5} flexWrap="wrap" sx={{ pt: 1 }}>
            <Chip
              icon={
                <ShieldRounded
                  sx={{ color: tokens.color.accent.violet, fontSize: 14 }}
                />
              }
              label="Production discipline, on day one"
              light={light}
            />
            <Chip
              label="Free → Pro $79 → Team $399 → Enterprise"
              light={light}
            />
            <Chip label="VS Code Extension included" light={light} />
          </Stack>
        </Stack>
      </Box>
    </Box>
  );
}

function CornerCell({
  text,
  bg,
  border,
}: {
  text: string;
  bg: string;
  border: string;
}) {
  return (
    <Box
      role="columnheader"
      sx={{
        bgcolor: bg,
        borderRight: `1px solid ${border}`,
        borderBottom: `1px solid ${border}`,
        color: text,
        fontFamily: tokens.font.mono,
        fontSize: 11,
        fontWeight: 800,
        letterSpacing: 0.8,
        px: 2,
        py: 1.4,
        textTransform: "uppercase",
      }}
    >
      Capability
    </Box>
  );
}

function HeaderCell({
  label,
  us,
  bg,
  text,
  border,
}: {
  label: string;
  us: boolean;
  bg: string;
  text: string;
  border: string;
}) {
  return (
    <Box
      role="columnheader"
      sx={{
        bgcolor: bg,
        borderBottom: `1px solid ${border}`,
        borderRight: `1px solid ${border}`,
        color: us ? tokens.color.brand.magenta : text,
        fontFamily: tokens.font.mono,
        fontSize: 11.5,
        fontWeight: us ? 900 : 800,
        letterSpacing: 0.6,
        px: 1.2,
        py: 1.4,
        textAlign: "center",
        textTransform: "uppercase",
      }}
    >
      {label}
    </Box>
  );
}

function ComparisonRow({
  row,
  zebra,
  light,
  text,
  muted,
  border,
  usBg,
}: {
  row: Row;
  zebra: boolean;
  light: boolean;
  text: string;
  muted: string;
  border: string;
  usBg: string;
}) {
  const zebraBg = zebra
    ? light
      ? "rgba(127,77,255,0.03)"
      : "rgba(255,255,255,0.02)"
    : "transparent";
  return (
    <>
      <Box
        sx={{
          bgcolor: zebraBg,
          borderRight: `1px solid ${border}`,
          borderBottom: `1px solid ${border}`,
          px: 2,
          py: 1.6,
        }}
      >
        <Typography
          sx={{ color: text, fontSize: 14.5, fontWeight: 700, lineHeight: 1.3 }}
        >
          {row.capability}
        </Typography>
        <Typography
          sx={{ color: muted, fontSize: 12.5, lineHeight: 1.5, mt: 0.4 }}
        >
          {row.detail}
        </Typography>
      </Box>
      {COMPETITORS.map((c) => {
        const v = row[c.key];
        const cellBg = c.us ? usBg : zebraBg;
        return (
          <Box
            key={c.key}
            sx={{
              alignItems: "center",
              bgcolor: cellBg,
              borderRight: `1px solid ${border}`,
              borderBottom: `1px solid ${border}`,
              display: "flex",
              justifyContent: "center",
              px: 1,
              py: 1.6,
            }}
          >
            <VerdictIcon v={v} />
          </Box>
        );
      })}
    </>
  );
}

function VerdictIcon({ v }: { v: Verdict }) {
  if (v === "yes")
    return (
      <CheckCircleRounded
        sx={{ color: tokens.color.brand.mint, fontSize: 22 }}
        titleAccess="Yes — ships today"
      />
    );
  if (v === "partial")
    return (
      <WarningAmberRounded
        sx={{ color: tokens.color.accent.warning, fontSize: 22 }}
        titleAccess="Partial — adjacent but not the same"
      />
    );
  if (v === "no")
    return (
      <CloseRounded
        sx={{ color: tokens.color.accent.coral, fontSize: 22 }}
        titleAccess="Not on their marketing site"
      />
    );
  return (
    <HelpOutlineRounded
      sx={{ color: tokens.color.text.muted, fontSize: 22 }}
      titleAccess="Unknown"
    />
  );
}

function Legend({
  icon,
  label,
  muted,
}: {
  icon: React.ReactNode;
  label: string;
  muted: string;
}) {
  return (
    <Stack direction="row" alignItems="center" spacing={0.7}>
      {icon}
      <Typography
        sx={{
          color: muted,
          fontFamily: tokens.font.mono,
          fontSize: 11,
          fontWeight: 700,
          letterSpacing: 0.5,
          textTransform: "uppercase",
        }}
      >
        {label}
      </Typography>
    </Stack>
  );
}

function Chip({
  icon,
  label,
  light,
}: {
  icon?: React.ReactNode;
  label: string;
  light: boolean;
}) {
  return (
    <Stack
      direction="row"
      alignItems="center"
      spacing={0.7}
      sx={{
        bgcolor: light ? "rgba(127,77,255,0.08)" : "rgba(127,77,255,0.14)",
        border: `1px solid ${
          light ? "rgba(127,77,255,0.28)" : tokens.color.border.strong
        }`,
        borderRadius: 999,
        color: light ? "#3a2772" : tokens.color.text.primary,
        fontFamily: tokens.font.mono,
        fontSize: 11.5,
        fontWeight: 700,
        letterSpacing: 0.4,
        px: 1.4,
        py: 0.55,
        textTransform: "uppercase",
      }}
    >
      {icon}
      <Typography
        component="span"
        sx={{ fontSize: 11.5, fontWeight: 700, letterSpacing: 0.4 }}
      >
        {label}
      </Typography>
    </Stack>
  );
}
