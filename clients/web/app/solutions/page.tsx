// app/solutions/page.tsx — by-stack marketing route. Same gates, same
// ledger; different runtime path per target. Server component.

import {
  AndroidRounded,
  ArrowForwardRounded,
  BadgeRounded,
  BoltRounded,
  BusinessCenterRounded,
  ConstructionRounded,
  DashboardRounded,
  DesignServicesRounded,
  FormatQuoteRounded,
  GroupsRounded,
  LanguageRounded,
  PhoneIphoneRounded,
  RocketLaunchRounded,
  WebRounded,
  WorkspacePremiumRounded,
} from "@mui/icons-material";
import { Box, Button, Stack, Typography } from "@mui/material";
import type { Metadata } from "next";
import Link from "next/link";
import type { ReactNode } from "react";
import { tokens } from "../../../../packages/design-tokens";
import {
  CtaBand,
  MarketingHero,
  MarketingSection,
} from "../../src/components/marketing";

export const metadata: Metadata = {
  title: "Solutions — Ironflyer",
  description:
    "Pick your stack: Indie SaaS, Expo mobile, native Android, native iOS, internal tools, marketing sites. Every solution ships through the same gate chain.",
  openGraph: {
    title: "Solutions — Ironflyer",
    description:
      "Stack-by-stack solutions for indie founders, mobile teams, agencies, and internal-tool builders — all running the same finisher engine.",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "Solutions — Ironflyer",
    description:
      "Stack-by-stack solutions, one finisher engine — gates that block, patches that review.",
  },
};

interface Solution {
  id: string;
  name: string;
  audience: string;
  runtime: string;
  gated: string[];
  cta: { href: string; label: string };
  icon: ReactNode;
  accent?: "violet" | "coral" | "mint";
}

const SOLUTIONS: Solution[] = [
  {
    id: "indie-saas",
    name: "Indie SaaS",
    audience: "Solo founders shipping web products charged on subscription.",
    runtime: "Docker workspace + Vercel deploy + Postgres/Surreal selected per project.",
    gated: [
      "GateBuild — Next.js build inside the sandbox",
      "GateE2E — Playwright run against preview URL",
      "GateProfitGuard — wallet ROI guard on premium calls",
    ],
    cta: { href: "/signup?stack=indie", label: "Start with web stack" },
    icon: <RocketLaunchRounded />,
    accent: "violet",
  },
  {
    id: "expo",
    name: "Mobile app (Expo)",
    audience: "Founders shipping cross-platform mobile without owning a Mac.",
    runtime: "Expo Router + EAS Build cloud signing. KVM Android emulator on the runtime.",
    gated: [
      "GateMobileBuild — Expo manifest + EAS cloud build",
      "GateLicense — bundle-id reverse-DNS check",
      "GateProfitGuard — EntryEASBuildCredit reserved upfront",
    ],
    cta: { href: "/signup?stack=expo", label: "Ship with Expo" },
    icon: <PhoneIphoneRounded />,
    accent: "coral",
  },
  {
    id: "android-native",
    name: "Native Android",
    audience: "Teams shipping Kotlin + Jetpack Compose with full Android SDK access.",
    runtime: "Linux sandbox with Android SDK 35 + KVM-passthrough emulator + gradlew assembleDebug.",
    gated: [
      "GateMobileBuild — gradlew assembleDebug must produce APK",
      "GateSecurityScan — signing config + secrets in Project.Secrets",
      "GateOwnerCheck — per-user emulator isolation",
    ],
    cta: { href: "/signup?stack=android", label: "Ship Kotlin native" },
    icon: <AndroidRounded />,
    accent: "mint",
  },
  {
    id: "ios-native",
    name: "Native iOS (Pro tier)",
    audience: "Teams that need Swift + SwiftUI and an App Store-signed IPA.",
    runtime: "MacStadium / Scaleway / AWS mac2.metal pool with xcodebuild build. Mac pool required.",
    gated: [
      "GateMobileBuild — xcodebuild build inside Mac workspace",
      "GateProfitGuard — refuses Mac allocation that breaks wallet ROI",
      "EntryMacWorkspaceMin — separate ledger entry for Mac minutes",
    ],
    cta: { href: "/enterprise", label: "Enable Mac pool" },
    icon: <WorkspacePremiumRounded />,
    accent: "violet",
  },
  {
    id: "internal-tools",
    name: "Internal tools",
    audience: "Operators replacing brittle admin spreadsheets with reviewable code.",
    runtime: "Docker workspace + Next.js + Postgres. Same gates as Indie SaaS, deployed inside the tenant.",
    gated: [
      "GateOwnerCheck — per-tenant access enforced on every entity",
      "GateSecurityScan — secrets blocked from leaving the workspace",
      "GateE2E — admin flows verified before publish",
    ],
    cta: { href: "/signup?stack=internal", label: "Replace the spreadsheet" },
    icon: <DashboardRounded />,
    accent: "coral",
  },
  {
    id: "marketing-sites",
    name: "Marketing sites",
    audience: "Agencies billing fixed-price for static + headless launches.",
    runtime: "Next.js + MDX + S3 image pipeline. Vercel deploy via GateDeploy artifact.",
    gated: [
      "GateLint — accessibility + content-policy lint",
      "GateBuild — production bundle audit",
      "GateDeploy — deploy artifact persists to your bucket",
    ],
    cta: { href: "/signup?stack=marketing", label: "Spin up a site" },
    icon: <LanguageRounded />,
    accent: "mint",
  },
];

const MANIFESTO: Array<{ title: string; body: string; icon: ReactNode }> = [
  {
    title: "Solo founders",
    body: "Ship the product yourself. The wallet, ProfitGuard, and gate chain replace the manager you don't have.",
    icon: <BadgeRounded />,
  },
  {
    title: "Paid product teams",
    body: "Onboard the AI as a junior engineer that cannot merge red TS, secrets, or a broken build. Reviewable patches, per-user OwnerID, ledger every minute.",
    icon: <GroupsRounded />,
  },
  {
    title: "Agencies billing for completion",
    body: "Quote on shipped, not hours. The ledger and OutcomeEvent stream prove what closed and what didn't — to your client and to your books.",
    icon: <BusinessCenterRounded />,
  },
];

const QUOTES: Array<{ quote: string; role: string }> = [
  {
    quote:
      "I stopped reading AI-written diffs line by line. GatePatchReview rejected the three patches that would have leaked an API key — before I ever opened them.",
    role: "Solo founder, B2B SaaS",
  },
  {
    quote:
      "Our wallet runs at $400/mo and we ship more than the last team that ran $4k of unmetered Claude. ProfitGuard does not let a retry loop burn the wallet.",
    role: "Tech lead, fintech operator",
  },
  {
    quote:
      "We bill on OutcomeEvent. The client sees a deploy artifact, a gate-green verdict, and an itemized ledger. Invoicing disputes went to zero.",
    role: "Founder, product agency",
  },
];

export default function SolutionsPage() {
  return (
    <Box sx={{ width: "100%", minWidth: 0 }}>
      <MarketingHero
        eyebrow="solutions"
        title="Pick your stack. Ship through the same gates."
        subhead="Web, Expo, native Android, native iOS, internal tools, marketing sites — every solution lands in the same finisher engine. The gate verdicts, the ledger, the ProfitGuard reservation are identical."
        primary={{ href: "/signup", label: "Start a project" }}
        secondary={{ href: "/product", label: "How the engine works" }}
        proofChips={[
          "6 production stacks",
          "1 gate chain",
          "1 append-only ledger",
        ]}
      />

      <MarketingSection
        eyebrow="stacks"
        title="Six runtime paths, all routed through the finisher engine."
        subhead="The orchestrator picks the runtime by StackDecision. The agents and the gates know which path you're on."
      >
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: {
              xs: "1fr",
              md: "repeat(2, 1fr)",
              lg: "repeat(3, 1fr)",
            },
            gap: { xs: 2.4, md: 3 },
          }}
        >
          {SOLUTIONS.map((s) => {
            const accentColor =
              s.accent === "coral"
                ? tokens.color.accent.coral
                : s.accent === "mint"
                  ? tokens.color.accent.success
                  : tokens.color.accent.violet;
            return (
              <Box
                key={s.id}
                id={s.id}
                sx={{
                  p: { xs: 2.8, md: 3.2 },
                  borderRadius: `${tokens.radius.lg}px`,
                  border: `1px solid ${tokens.color.border.subtle}`,
                  bgcolor: `${tokens.color.bg.surface}d9`,
                  display: "flex",
                  flexDirection: "column",
                  gap: 2,
                  transition: `border-color ${tokens.motion.base} ${tokens.motion.curve}`,
                  "&:hover": {
                    borderColor: `${accentColor}66`,
                  },
                }}
              >
                <Stack direction="row" alignItems="center" spacing={1.5}>
                  <Box
                    sx={{
                      display: "inline-grid",
                      placeItems: "center",
                      width: 44,
                      height: 44,
                      borderRadius: `${tokens.radius.sm}px`,
                      bgcolor: `${accentColor}1f`,
                      color: accentColor,
                      "& svg": { fontSize: 24 },
                    }}
                  >
                    {s.icon}
                  </Box>
                  <Typography
                    sx={{
                      fontSize: 19,
                      fontWeight: 800,
                      color: tokens.color.text.primary,
                      letterSpacing: -0.2,
                    }}
                  >
                    {s.name}
                  </Typography>
                </Stack>
                <Typography
                  sx={{
                    color: tokens.color.text.secondary,
                    fontSize: 13.5,
                    lineHeight: 1.6,
                  }}
                >
                  {s.audience}
                </Typography>
                <Box
                  sx={{
                    p: 1.6,
                    borderRadius: `${tokens.radius.sm}px`,
                    border: `1px dashed ${tokens.color.border.subtle}`,
                    bgcolor: `${tokens.color.bg.base}80`,
                  }}
                >
                  <Typography
                    sx={{
                      fontFamily: tokens.font.mono,
                      fontSize: 10.5,
                      letterSpacing: 0.6,
                      textTransform: "uppercase",
                      color: tokens.color.text.muted,
                      fontWeight: 700,
                    }}
                  >
                    Runtime path
                  </Typography>
                  <Typography
                    sx={{
                      mt: 0.5,
                      fontFamily: tokens.font.mono,
                      fontSize: 12.5,
                      color: tokens.color.text.primary,
                      lineHeight: 1.55,
                    }}
                  >
                    {s.runtime}
                  </Typography>
                </Box>
                <Stack spacing={0.8}>
                  <Typography
                    sx={{
                      fontFamily: tokens.font.mono,
                      fontSize: 10.5,
                      letterSpacing: 0.6,
                      textTransform: "uppercase",
                      color: accentColor,
                      fontWeight: 700,
                    }}
                  >
                    What gets gated
                  </Typography>
                  {s.gated.map((g) => (
                    <Stack
                      key={g}
                      direction="row"
                      alignItems="flex-start"
                      spacing={1}
                    >
                      <Box
                        sx={{
                          width: 6,
                          height: 6,
                          borderRadius: "50%",
                          bgcolor: accentColor,
                          mt: 0.8,
                          flexShrink: 0,
                        }}
                      />
                      <Typography
                        sx={{
                          fontSize: 13,
                          color: tokens.color.text.secondary,
                          lineHeight: 1.55,
                        }}
                      >
                        {g}
                      </Typography>
                    </Stack>
                  ))}
                </Stack>
                <Button
                  component={Link}
                  href={s.cta.href}
                  variant="text"
                  endIcon={<ArrowForwardRounded sx={{ fontSize: 16 }} />}
                  sx={{
                    alignSelf: "flex-start",
                    mt: "auto",
                    color: accentColor,
                    fontWeight: 700,
                    px: 0,
                    "&:hover": { bgcolor: "transparent", opacity: 0.85 },
                  }}
                >
                  {s.cta.label}
                </Button>
              </Box>
            );
          })}
        </Box>
      </MarketingSection>

      <MarketingSection
        eyebrow="who Ironflyer is for"
        title="Builders that bill for completion."
        subhead="The platform is opinionated. It rewards operators who care about margin and reviewability. It is not for the vibe-coding crowd."
        bgVariant="inset"
      >
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "repeat(3, 1fr)" },
            gap: { xs: 2.5, md: 3 },
          }}
        >
          {MANIFESTO.map((m) => (
            <Box
              key={m.title}
              sx={{
                p: { xs: 3, md: 3.4 },
                borderRadius: `${tokens.radius.lg}px`,
                border: `1px solid ${tokens.color.border.subtle}`,
                bgcolor: `${tokens.color.bg.surfaceRaised}cc`,
                display: "flex",
                flexDirection: "column",
                gap: 1.8,
              }}
            >
              <Box
                sx={{
                  display: "inline-grid",
                  placeItems: "center",
                  width: 48,
                  height: 48,
                  borderRadius: `${tokens.radius.md}px`,
                  bgcolor: `${tokens.color.accent.violet}1f`,
                  color: tokens.color.accent.violet,
                  "& svg": { fontSize: 26 },
                }}
              >
                {m.icon}
              </Box>
              <Typography
                sx={{
                  fontSize: 22,
                  fontWeight: 900,
                  color: tokens.color.text.primary,
                  letterSpacing: -0.3,
                }}
              >
                {m.title}
              </Typography>
              <Typography
                sx={{
                  color: tokens.color.text.secondary,
                  fontSize: 14.5,
                  lineHeight: 1.65,
                }}
              >
                {m.body}
              </Typography>
            </Box>
          ))}
        </Box>
      </MarketingSection>

      <MarketingSection
        eyebrow="builders on Ironflyer"
        title="Operators name the mechanic, not the vibe."
        subhead="Quotes from operators in private testing. Roles are real; company names withheld until the public launch."
      >
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "repeat(3, 1fr)" },
            gap: { xs: 2.4, md: 3 },
          }}
        >
          {QUOTES.map((q, i) => (
            <Box
              key={i}
              sx={{
                p: { xs: 2.8, md: 3.2 },
                borderRadius: `${tokens.radius.lg}px`,
                border: `1px solid ${tokens.color.border.subtle}`,
                bgcolor: `${tokens.color.bg.surface}cc`,
                display: "flex",
                flexDirection: "column",
                gap: 2,
              }}
            >
              <FormatQuoteRounded
                sx={{
                  fontSize: 32,
                  color: tokens.color.accent.violet,
                }}
              />
              <Typography
                sx={{
                  fontSize: 15.5,
                  lineHeight: 1.65,
                  color: tokens.color.text.primary,
                  fontWeight: 500,
                  flex: 1,
                }}
              >
                {q.quote}
              </Typography>
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 11.5,
                  letterSpacing: 0.6,
                  textTransform: "uppercase",
                  color: tokens.color.text.muted,
                  fontWeight: 700,
                }}
              >
                {q.role}
              </Typography>
            </Box>
          ))}
        </Box>
      </MarketingSection>

      <MarketingSection
        eyebrow="starter templates"
        title="Real, runnable starters in the templates folder."
        subhead="Every solution maps to a starter under templates/starters/. Same conventions, same gates, English UI copy and design-token colors."
      >
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", sm: "repeat(2, 1fr)", md: "repeat(4, 1fr)" },
            gap: 2,
          }}
        >
          {[
            { name: "react-native-expo", icon: <PhoneIphoneRounded /> },
            { name: "android-kotlin", icon: <AndroidRounded /> },
            { name: "ios-swift", icon: <WorkspacePremiumRounded /> },
            { name: "next-app", icon: <WebRounded /> },
          ].map((s) => (
            <Box
              key={s.name}
              sx={{
                p: 2.2,
                borderRadius: `${tokens.radius.md}px`,
                border: `1px solid ${tokens.color.border.subtle}`,
                bgcolor: `${tokens.color.bg.surface}cc`,
                display: "flex",
                alignItems: "center",
                gap: 1.5,
              }}
            >
              <Box
                sx={{
                  color: tokens.color.accent.violet,
                  "& svg": { fontSize: 22 },
                }}
              >
                {s.icon}
              </Box>
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 12.5,
                  color: tokens.color.text.primary,
                  fontWeight: 700,
                }}
              >
                templates/starters/{s.name}/
              </Typography>
            </Box>
          ))}
        </Box>
      </MarketingSection>

      <MarketingSection
        eyebrow="the work we don't do for you"
        title="Honest about the seams."
        subhead="Ironflyer is opinionated, not magic. These are the parts you still own."
      >
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "repeat(3, 1fr)" },
            gap: 2,
          }}
        >
          {[
            {
              icon: <DesignServicesRounded />,
              title: "Brand decisions",
              body: "We render the design tokens; the brand identity is still yours to call.",
            },
            {
              icon: <ConstructionRounded />,
              title: "Domain knowledge",
              body: "Pinned memory + blueprints help, but the rules of your industry land through your patches.",
            },
            {
              icon: <BoltRounded />,
              title: "Provider keys on Free",
              body: "Free tier is platform-managed providers. BYO keys come with Pro and Enterprise.",
            },
          ].map((row) => (
            <Box
              key={row.title}
              sx={{
                p: 2.4,
                borderRadius: `${tokens.radius.md}px`,
                border: `1px solid ${tokens.color.border.subtle}`,
                bgcolor: `${tokens.color.bg.surfaceRaised}cc`,
              }}
            >
              <Box
                sx={{
                  color: tokens.color.accent.violet,
                  mb: 1.2,
                  "& svg": { fontSize: 22 },
                }}
              >
                {row.icon}
              </Box>
              <Typography
                sx={{
                  fontSize: 16,
                  fontWeight: 800,
                  color: tokens.color.text.primary,
                  mb: 0.6,
                }}
              >
                {row.title}
              </Typography>
              <Typography
                sx={{
                  color: tokens.color.text.secondary,
                  fontSize: 13.5,
                  lineHeight: 1.6,
                }}
              >
                {row.body}
              </Typography>
            </Box>
          ))}
        </Box>
      </MarketingSection>

      <CtaBand
        heading="Your stack is supported. Your gates are the same."
        sub="Pick a runtime path and stand up the first execution. The finisher engine takes care of the rest."
        primary={{ href: "/signup", label: "Start a project" }}
        secondary={{ href: "/templates", label: "Browse starters" }}
        chips={[
          "Indie + agencies welcome",
          "Mobile + web from one workspace",
          "Same ledger, same gates",
        ]}
      />
    </Box>
  );
}
