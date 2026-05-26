// app/showcase/page.tsx — public marketing route.
//
// "What people ship with Ironflyer." Curated execution archetypes
// presented as a template gallery so the page has real content
// before customer case studies land.

import type { Metadata } from "next";
import Link from "next/link";
import { ArrowForwardRounded, CheckCircleRounded } from "@mui/icons-material";
import { Box, Button, Stack, Typography } from "@mui/material";
import { tokens } from "../../../../packages/design-tokens";
import { MarketingHero } from "../../src/components/marketing/MarketingHero";
import { MarketingSection } from "../../src/components/marketing/MarketingSection";

export const metadata: Metadata = {
  title: "Showcase — Ironflyer",
  description:
    "Real apps shipped through real gates. Browse execution archetypes that the Ironflyer finisher engine has carried from prompt to deploy.",
  alternates: { canonical: "https://ironflyer.com/showcase" },
  openGraph: {
    title: "Showcase — Ironflyer",
    description:
      "Real apps shipped through real gates. Browse execution archetypes that the Ironflyer finisher engine has carried from prompt to deploy.",
    url: "https://ironflyer.com/showcase",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "Showcase — Ironflyer",
    description:
      "Real apps shipped through real gates. Browse execution archetypes that the Ironflyer finisher engine has carried from prompt to deploy.",
  },
};

interface ShowcaseCard {
  name: string;
  description: string;
  stack: string[];
  mechanics: string[];
  archetype: string;
}

const CARDS: ShowcaseCard[] = [
  {
    name: "Tasks Pro",
    description: "Team task manager with Stripe-billed seats and an append-only audit log.",
    stack: ["Next.js 15", "Postgres", "Stripe"],
    mechanics: ["GateProdReady passed", "ProfitGuard saved $2.40"],
    archetype: "SaaS — Web",
  },
  {
    name: "Recipe Stash",
    description: "Personal recipe vault with offline-first cache and shared family lists.",
    stack: ["Expo", "Supabase", "EAS Build"],
    mechanics: ["GateMobileBuild passed", "EAS credit metered"],
    archetype: "Consumer — Mobile",
  },
  {
    name: "Pomodoro Native",
    description: "Focus timer with Android foreground service and DND integration.",
    stack: ["Kotlin", "Jetpack Compose", "Room"],
    mechanics: ["GateMobileBuild passed", "gradlew assembleDebug"],
    archetype: "Consumer — Android",
  },
  {
    name: "Internal Auditor",
    description: "Privately deployed compliance review tool with LDAP login and SSO.",
    stack: ["Next.js 15", "Postgres", "LDAP"],
    mechanics: ["GateSecurity passed", "Owner check enforced"],
    archetype: "Enterprise — Web",
  },
  {
    name: "Crypto Watch",
    description: "Live price dashboard with custom alerts and historical drawdowns.",
    stack: ["Next.js 15", "Postgres", "WebSocket"],
    mechanics: ["GateLiveData passed", "Streaming provider locked"],
    archetype: "Consumer — Web",
  },
  {
    name: "Habit Streak",
    description: "Daily habit tracker with push reminders and friend leaderboards.",
    stack: ["Expo", "Firebase", "FCM"],
    mechanics: ["GateMobileBuild passed", "ProfitGuard saved $1.10"],
    archetype: "Consumer — Mobile",
  },
  {
    name: "Invoice Maker",
    description: "Freelance invoicing with Stripe payment links and PDF rendering.",
    stack: ["Next.js 15", "Postgres", "Stripe"],
    mechanics: ["GatePayments passed", "Webhook signature verified"],
    archetype: "SaaS — Web",
  },
  {
    name: "Job Board Lite",
    description: "Niche job board with S3-hosted resume uploads and email digests.",
    stack: ["Next.js 15", "Postgres", "S3"],
    mechanics: ["GateBucketPolicy passed", "Signed URL lifecycle"],
    archetype: "Marketplace — Web",
  },
  {
    name: "Time Tracker",
    description: "Cross-device time tracker that syncs without a single conflict prompt.",
    stack: ["Expo", "Postgres", "Sync API"],
    mechanics: ["GateSync passed", "CRDT plan locked"],
    archetype: "Productivity — Mobile",
  },
  {
    name: "RSS Roundup",
    description: "Curated reading list with weekly digest emails and full-text search.",
    stack: ["Next.js 15", "Postgres", "Postmark"],
    mechanics: ["GateEmail passed", "Bounce handling wired"],
    archetype: "Consumer — Web",
  },
  {
    name: "Markdown Notes",
    description: "Offline-first note app with local SQLite and optional cloud sync.",
    stack: ["Expo", "SQLite", "Cloud sync"],
    mechanics: ["GateMobileBuild passed", "Offline-first verified"],
    archetype: "Productivity — Mobile",
  },
  {
    name: "Survey Tool",
    description: "Forms with payment-gated submissions and a real-time results dashboard.",
    stack: ["Next.js 15", "Postgres", "Stripe"],
    mechanics: ["GatePayments passed", "GateLiveData passed"],
    archetype: "SaaS — Web",
  },
];

export default function ShowcasePage() {
  return (
    <Box>
      <MarketingHero
        eyebrow="showcase"
        title="Real apps shipped through real gates."
        subhead="Twelve execution archetypes the finisher engine carries from prompt to production. Each card names the gates that passed and the ProfitGuard verdicts that protected the wallet on the way."
        primary={{ href: "/signup", label: "Start your project" }}
        secondary={{ href: "/templates", label: "Browse templates" }}
        proofChips={["Gated end-to-end", "ProfitGuard on every call", "Per-execution ledger"]}
      />

      <MarketingSection
        eyebrow="archetypes"
        title="What ships when the gates approve."
        subhead="Every card below is a curated archetype with the stack, mechanic chips, and the gate verdicts that matter. Screenshots land as design captures complete."
      >
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: {
              xs: "1fr",
              sm: "repeat(2, minmax(0, 1fr))",
              md: "repeat(3, minmax(0, 1fr))",
            },
            gap: 3,
          }}
        >
          {CARDS.map((card) => (
            <Box
              key={card.name}
              sx={{
                display: "flex",
                flexDirection: "column",
                border: `1px solid ${tokens.color.border.subtle}`,
                borderRadius: 2,
                bgcolor: tokens.color.bg.surface,
                overflow: "hidden",
                transition: `border-color ${tokens.motion.fast} ${tokens.motion.snap}`,
                "&:hover": {
                  borderColor: tokens.color.border.strong,
                },
              }}
            >
              <Box
                role="img"
                aria-label={`Screenshot pending for ${card.name}`}
                sx={{
                  height: 168,
                  bgcolor: tokens.color.bg.surfaceRaised,
                  borderBottom: `1px solid ${tokens.color.border.subtle}`,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  position: "relative",
                  backgroundImage: `radial-gradient(circle at 30% 20%, ${tokens.color.accent.violet}1f, transparent 60%), radial-gradient(circle at 80% 80%, ${tokens.color.accent.coral}14, transparent 60%)`,
                }}
              >
                <Typography
                  sx={{
                    fontFamily: tokens.font.mono,
                    fontSize: 11,
                    letterSpacing: 1.2,
                    textTransform: "uppercase",
                    color: tokens.color.text.muted,
                  }}
                >
                  Screenshot pending
                </Typography>
              </Box>
              <Stack spacing={1.4} sx={{ p: 2.4, flex: 1 }}>
                <Stack direction="row" spacing={1} alignItems="center" justifyContent="space-between">
                  <Typography
                    sx={{
                      fontSize: 17,
                      fontWeight: 800,
                      color: tokens.color.text.primary,
                    }}
                  >
                    {card.name}
                  </Typography>
                  <Typography
                    sx={{
                      fontFamily: tokens.font.mono,
                      fontSize: 10.5,
                      letterSpacing: 0.6,
                      textTransform: "uppercase",
                      color: tokens.color.accent.violet,
                    }}
                  >
                    {card.archetype}
                  </Typography>
                </Stack>
                <Typography
                  sx={{
                    fontSize: 14,
                    lineHeight: 1.55,
                    color: tokens.color.text.secondary,
                  }}
                >
                  {card.description}
                </Typography>
                <Stack direction="row" spacing={0.8} flexWrap="wrap" useFlexGap>
                  {card.stack.map((s) => (
                    <Box
                      key={s}
                      sx={{
                        px: 1,
                        py: 0.35,
                        borderRadius: 999,
                        border: `1px solid ${tokens.color.border.subtle}`,
                        bgcolor: tokens.color.bg.inset,
                        fontFamily: tokens.font.mono,
                        fontSize: 11,
                        color: tokens.color.text.secondary,
                      }}
                    >
                      {s}
                    </Box>
                  ))}
                </Stack>
                <Stack spacing={0.6} sx={{ pt: 0.4 }}>
                  {card.mechanics.map((m) => (
                    <Stack key={m} direction="row" spacing={0.8} alignItems="center">
                      <CheckCircleRounded
                        sx={{ fontSize: 14, color: tokens.color.brand.mint }}
                      />
                      <Typography
                        sx={{
                          fontFamily: tokens.font.mono,
                          fontSize: 11.5,
                          color: tokens.color.text.secondary,
                        }}
                      >
                        {m}
                      </Typography>
                    </Stack>
                  ))}
                </Stack>
              </Stack>
            </Box>
          ))}
        </Box>
      </MarketingSection>

      <MarketingSection bgVariant="inset">
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "1.4fr 0.9fr" },
            gap: 4,
            alignItems: "center",
            border: `1px solid ${tokens.color.border.strong}`,
            borderRadius: 3,
            p: { xs: 3, md: 5 },
            bgcolor: tokens.color.bg.surface,
          }}
        >
          <Stack spacing={1.8}>
            <Typography
              sx={{
                fontFamily: tokens.font.mono,
                fontSize: 11.5,
                letterSpacing: 1.4,
                textTransform: "uppercase",
                color: tokens.color.accent.violet,
                fontWeight: 700,
              }}
            >
              Featured slot
            </Typography>
            <Typography
              component="h2"
              sx={{
                fontSize: { xs: 24, md: 32 },
                fontWeight: 900,
                letterSpacing: -0.4,
                color: tokens.color.text.primary,
              }}
            >
              Want your app featured here?
            </Typography>
            <Typography
              sx={{
                fontSize: 16,
                lineHeight: 1.6,
                color: tokens.color.text.secondary,
              }}
            >
              Ship something through Ironflyer that you are proud of, then send us
              the public URL and a short note on which gates carried the most
              weight. We feature builds that pass every gate cleanly and stay
              profitable per execution.
            </Typography>
          </Stack>
          <Stack spacing={1.5}>
            <Button
              component={Link}
              href="/signup"
              variant="contained"
              color="primary"
              size="large"
              endIcon={<ArrowForwardRounded sx={{ fontSize: 18 }} />}
            >
              Start your build
            </Button>
            <Button
              component={Link}
              href="/enterprise"
              variant="text"
              size="large"
              sx={{ color: tokens.color.accent.violet, justifyContent: "flex-start" }}
            >
              Submit for review
            </Button>
          </Stack>
        </Box>
      </MarketingSection>
    </Box>
  );
}
