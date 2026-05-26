// app/changelog/page.tsx — public marketing route.
//
// Reverse-chronological release timeline. Each entry names files or
// mechanics specific to the V22 implementation so the page reads as
// a real engineering log, not a content-marketing chore.

import type { Metadata } from "next";
import Link from "next/link";
import { RssFeedRounded } from "@mui/icons-material";
import { Box, Stack, Typography } from "@mui/material";
import { tokens } from "../../../../packages/design-tokens";
import { MarketingHero } from "../../src/components/marketing/MarketingHero";
import { MarketingSection } from "../../src/components/marketing/MarketingSection";

export const metadata: Metadata = {
  title: "Changelog — Ironflyer",
  description:
    "What we shipped to the gate. Versioned release notes spanning gates, runtime, mobile, ledger, and UI.",
  alternates: { canonical: "https://ironflyer.com/changelog" },
  openGraph: {
    title: "Changelog — Ironflyer",
    description:
      "What we shipped to the gate. Versioned release notes spanning gates, runtime, mobile, ledger, and UI.",
    url: "https://ironflyer.com/changelog",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "Changelog — Ironflyer",
    description:
      "What we shipped to the gate. Versioned release notes spanning gates, runtime, mobile, ledger, and UI.",
  },
};

type Category = "Gates" | "Runtime" | "Mobile" | "Ledger" | "UI";

interface Entry {
  date: string;
  version: string;
  category: Category;
  heading: string;
  bullets: string[];
}

const ENTRIES: Entry[] = [
  {
    date: "2026-05-27",
    version: "v22.4.7",
    category: "Mobile",
    heading: "Mac pool dispatch lands behind IRONFLYER_MAC_POOL_ENABLED",
    bullets: [
      "core/runtime/internal/suppliers/mobile/ now dispatches xcodebuild to the Mac pool when the flag is set",
      "GateMobileBuild downgrades to SeverityInfo 'deferred to EAS cloud' when no pool is attached",
      "ProfitGuard refuses Mac allocations that would push the wallet negative before allocation",
    ],
  },
  {
    date: "2026-05-21",
    version: "v22.4.6",
    category: "Ledger",
    heading: "Mobile cost lines split out from generic compute",
    bullets: [
      "Added EntryMobileBuildMin, EntryEmulatorMin, EntryMacWorkspaceMin, EntryEASBuildCredit, EntryAppetizeMin",
      "Migration 00018 extends ledger_entries.entry_type CHECK constraint for the new values",
      "Wallet UI cost panel now splits build minutes from emulator minutes",
    ],
  },
  {
    date: "2026-05-15",
    version: "v22.4.5",
    category: "Gates",
    heading: "GateMobileBuild promoted from preview to default lane",
    bullets: [
      "Registered in finisher.DefaultGates() after Budget, before Deploy",
      "Validates Expo app.json / Android build.gradle / iOS xcodegen.yml / Flutter pubspec.yaml",
      "Bundle ID enforcement via domain.AppIDPattern reverse-DNS regex",
    ],
  },
  {
    date: "2026-05-08",
    version: "v22.4.4",
    category: "Runtime",
    heading: "Per-user workspace sandboxes hardened",
    bullets: [
      "OwnerID propagated through every workspace lifecycle handler",
      "requireProjectAccess returns 404 on non-owner / non-public projects",
      "Docker driver gains a strict mount allowlist; mock driver kept in parity",
    ],
  },
  {
    date: "2026-04-29",
    version: "v22.4.3",
    category: "UI",
    heading: "Cockpit cost panel switches to visualization-first default",
    bullets: [
      "Default view of /wallet and /executions is a chart, not a table",
      "Tables move behind a 'Pro view' toggle per the viz-first rule",
      "echarts and @xyflow/react lazy-load via next/dynamic with ssr:false",
    ],
  },
  {
    date: "2026-04-21",
    version: "v22.4.2",
    category: "Ledger",
    heading: "Memory backend switches to pgvector when configured",
    bullets: [
      "IRONFLYER_MEMORY_BACKEND=pgvector enables Postgres + pgvector",
      "Migration 00017_pgvector_memory.sql installs the extension and index",
      "Default backend stays 'memory' (in-process ring buffer) to keep cold-start cheap",
    ],
  },
  {
    date: "2026-04-12",
    version: "v22.4.1",
    category: "Gates",
    heading: "Patch lifecycle gates renamed and reordered",
    bullets: [
      "Propose → Review → Apply now matches the public docs verbatim",
      "AI cannot bypass patch.Engine.Propose; direct writes blocked at the file driver",
      "GateName constants documented in core/orchestrator/internal/domain/gates.go",
    ],
  },
  {
    date: "2026-04-03",
    version: "v22.4.0",
    category: "Runtime",
    heading: "Streaming-first provider contract enforced",
    bullets: [
      "Every provider now implements CompleteStream; Complete is a thin wrapper",
      "BillingGuard wraps token deltas so cost lands in the ledger live, not at end",
      "/executions/{id}/chat/stream stays REST per the exception list",
    ],
  },
  {
    date: "2026-03-22",
    version: "v22.3.5",
    category: "UI",
    heading: "Login and signup pinned to Base44 split-layout",
    bullets: [
      "AuthShell renders left brand panel + right form panel on lg+",
      "Cockpit nav suppressed on /login and /signup; AuthShell owns full bleed",
      "Centered-card regression flagged constitutional in CLAUDE.md",
    ],
  },
  {
    date: "2026-03-10",
    version: "v22.3.4",
    category: "Mobile",
    heading: "Real Expo + EAS pipeline replaces the placeholder",
    bullets: [
      "templates/starters/react-native-expo/ ships as a runnable starter",
      "eas build dispatched from the orchestrator with credit metering",
      "EAS credit ledger entry surfaces on the cost panel per execution",
    ],
  },
  {
    date: "2026-02-26",
    version: "v22.3.3",
    category: "Gates",
    heading: "GateSecurity tightens secrets handling",
    bullets: [
      "Project.Secrets never serialised to clients; gated by an owner check",
      "Detection rules added for AWS, Stripe, Anthropic, OpenAI key shapes",
      "Audit log entries written on every secret read by the runtime",
    ],
  },
  {
    date: "2026-02-14",
    version: "v22.3.2",
    category: "Ledger",
    heading: "Append-only ledger and Vault snapshot become source of truth",
    bullets: [
      "revenue − providerCost = margin at the platform aggregate level",
      "Per-execution attribution wired into ProfitGuard reservation",
      "Operator dashboard surfaces margin first; scale dashboards second",
    ],
  },
];

const CATEGORY_COLOR: Record<Category, string> = {
  Gates: tokens.color.accent.violet,
  Runtime: tokens.color.accent.sky,
  Mobile: tokens.color.accent.coral,
  Ledger: tokens.color.brand.mint,
  UI: tokens.color.accent.purple,
};

export default function ChangelogPage() {
  return (
    <Box>
      <MarketingHero
        eyebrow="changelog"
        title="What we shipped to the gate."
        subhead="Versioned release notes from the Ironflyer team. Every entry names the file, gate, or mechanic that moved — not the marketing copy that followed."
        proofChips={["Versioned", "Mechanic-first", "No vapor"]}
      />

      <MarketingSection>
        <Stack spacing={4.5}>
          {ENTRIES.map((entry) => {
            const accent = CATEGORY_COLOR[entry.category];
            return (
              <Box
                key={entry.version}
                sx={{
                  display: "grid",
                  gridTemplateColumns: { xs: "1fr", md: "180px 1fr" },
                  gap: { xs: 2, md: 4 },
                  pb: 4,
                  borderBottom: `1px solid ${tokens.color.border.subtle}`,
                  "&:last-of-type": { borderBottom: "none" },
                }}
              >
                <Stack spacing={1} sx={{ alignItems: "flex-start" }}>
                  <Box
                    sx={{
                      px: 1.1,
                      py: 0.4,
                      borderRadius: 999,
                      border: `1px solid ${tokens.color.border.subtle}`,
                      bgcolor: tokens.color.bg.inset,
                    }}
                  >
                    <Typography
                      sx={{
                        fontFamily: tokens.font.mono,
                        fontSize: 11.5,
                        color: tokens.color.text.secondary,
                        letterSpacing: 0.4,
                      }}
                    >
                      {entry.date}
                    </Typography>
                  </Box>
                  <Typography
                    sx={{
                      fontFamily: tokens.font.mono,
                      fontSize: 12,
                      fontWeight: 700,
                      color: tokens.color.text.primary,
                      letterSpacing: 0.4,
                    }}
                  >
                    {entry.version}
                  </Typography>
                  <Box
                    sx={{
                      px: 1.1,
                      py: 0.4,
                      borderRadius: 999,
                      border: `1px solid ${accent}66`,
                      bgcolor: `${accent}14`,
                    }}
                  >
                    <Typography
                      sx={{
                        fontFamily: tokens.font.mono,
                        fontSize: 10.5,
                        letterSpacing: 1.2,
                        textTransform: "uppercase",
                        fontWeight: 700,
                        color: accent,
                      }}
                    >
                      {entry.category}
                    </Typography>
                  </Box>
                </Stack>

                <Stack spacing={1.4} sx={{ minWidth: 0 }}>
                  <Typography
                    component="h3"
                    sx={{
                      fontSize: { xs: 19, md: 22 },
                      fontWeight: 800,
                      letterSpacing: -0.3,
                      color: tokens.color.text.primary,
                    }}
                  >
                    {entry.heading}
                  </Typography>
                  <Stack component="ul" spacing={0.8} sx={{ pl: 2, m: 0 }}>
                    {entry.bullets.map((b) => (
                      <Typography
                        component="li"
                        key={b}
                        sx={{
                          fontSize: 14.5,
                          lineHeight: 1.6,
                          color: tokens.color.text.secondary,
                        }}
                      >
                        {b}
                      </Typography>
                    ))}
                  </Stack>
                </Stack>
              </Box>
            );
          })}
        </Stack>

        <Box
          sx={{
            mt: 6,
            p: 3,
            borderRadius: 2,
            border: `1px solid ${tokens.color.border.subtle}`,
            bgcolor: tokens.color.bg.surface,
            display: "flex",
            flexDirection: { xs: "column", md: "row" },
            gap: 2,
            alignItems: { xs: "flex-start", md: "center" },
            justifyContent: "space-between",
          }}
        >
          <Stack spacing={0.5}>
            <Typography sx={{ fontWeight: 800, color: tokens.color.text.primary, fontSize: 16 }}>
              Follow the changelog
            </Typography>
            <Typography sx={{ fontSize: 14, color: tokens.color.text.secondary }}>
              Subscribe via RSS to get release notes the same day we ship them.
            </Typography>
          </Stack>
          <Box
            component={Link}
            href="/changelog/rss.xml"
            sx={{
              display: "inline-flex",
              alignItems: "center",
              gap: 1,
              px: 1.6,
              py: 0.9,
              borderRadius: 999,
              border: `1px solid ${tokens.color.border.strong}`,
              bgcolor: tokens.color.bg.surfaceRaised,
              color: tokens.color.text.primary,
              textDecoration: "none",
              fontFamily: tokens.font.mono,
              fontSize: 12.5,
              "&:hover": { bgcolor: tokens.color.bg.surfaceHover },
            }}
          >
            <RssFeedRounded sx={{ fontSize: 16, color: tokens.color.accent.coral }} />
            /changelog/rss.xml
          </Box>
        </Box>
      </MarketingSection>
    </Box>
  );
}
