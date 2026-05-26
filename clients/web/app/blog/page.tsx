// app/blog/page.tsx — public marketing route.
//
// Index page only. Cards link to placeholder slugs (`#`) since post
// detail pages are out of scope. Newsletter form is UI-only.

import type { Metadata } from "next";
import { Box, Button, Stack, Typography } from "@mui/material";
import { tokens } from "../../../../packages/design-tokens";
import { MarketingHero } from "../../src/components/marketing/MarketingHero";
import { MarketingSection } from "../../src/components/marketing/MarketingSection";

export const metadata: Metadata = {
  title: "Blog — Ironflyer",
  description:
    "Field notes on shipping AI-built software: gates, patches, wallet economics, ProfitGuard, and the hidden cost of vibe-coding tools.",
  alternates: { canonical: "https://ironflyer.com/blog" },
  openGraph: {
    title: "Blog — Ironflyer",
    description:
      "Field notes on shipping AI-built software: gates, patches, wallet economics, ProfitGuard, and the hidden cost of vibe-coding tools.",
    url: "https://ironflyer.com/blog",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "Blog — Ironflyer",
    description:
      "Field notes on shipping AI-built software: gates, patches, wallet economics, ProfitGuard, and the hidden cost of vibe-coding tools.",
  },
};

interface Post {
  slug: string;
  date: string;
  category: string;
  title: string;
  dek: string;
  author: string;
}

const POSTS: Post[] = [
  {
    slug: "profit-guard-before-tokens",
    date: "2026-05-20",
    category: "Economics",
    title: "Why ProfitGuard exists before the first token is spent",
    dek: "Pre-spend authorization beats post-hoc accounting. We hard-block expensive reasoning before the wallet sees it.",
    author: "Ironflyer team",
  },
  {
    slug: "against-in-browser-sandboxes",
    date: "2026-05-12",
    category: "Runtime",
    title: "The case against in-browser AI sandboxes",
    dek: "WebContainers are a demo. Real Linux workspaces with Docker drivers are the production unit.",
    author: "Ironflyer team",
  },
  {
    slug: "patches-the-only-reviewable-unit",
    date: "2026-05-04",
    category: "Engineering",
    title: "Patches are the only reviewable AI unit",
    dek: "If the model writes files directly, you cannot review what it did. patch.Engine.Propose exists for a reason.",
    author: "Ironflyer team",
  },
  {
    slug: "append-only-ledger-postgres",
    date: "2026-04-26",
    category: "Architecture",
    title: "Append-only ledgers and why we picked Postgres",
    dek: "Ledgers are the contract between revenue and provider cost. They have to be boring, durable, and replayable.",
    author: "Ironflyer team",
  },
  {
    slug: "eas-mobile-builds-real",
    date: "2026-04-15",
    category: "Mobile",
    title: "What we learned shipping mobile native builds via EAS",
    dek: "Expo + EAS is the cheapest way to ship a real iOS binary without owning Mac hardware. Here is the trade.",
    author: "Ironflyer team",
  },
  {
    slug: "wallet-not-subscription",
    date: "2026-04-08",
    category: "Economics",
    title: "Wallet, not subscription: pricing for paid execution",
    dek: "Subscriptions reward inactivity. Wallets reward shipping. Here is how we model the difference per execution.",
    author: "Ironflyer team",
  },
  {
    slug: "gates-that-actually-block",
    date: "2026-03-30",
    category: "Engineering",
    title: "Gates that actually block, not just warn",
    dek: "A warning the operator can dismiss is not a gate. Verdicts in finisher.DefaultGates() return blocking severities.",
    author: "Ironflyer team",
  },
  {
    slug: "owner-id-everywhere",
    date: "2026-03-18",
    category: "Security",
    title: "OwnerID on every row, owner check on every read",
    dek: "Multi-tenant isolation is not a feature flag. It is a column, a middleware, and a 404.",
    author: "Ironflyer team",
  },
  {
    slug: "streaming-first-provider",
    date: "2026-03-04",
    category: "Engineering",
    title: "Why every provider implements CompleteStream first",
    dek: "Non-streaming completions are a wrapper. Streaming is the contract that makes BillingGuard land cost live.",
    author: "Ironflyer team",
  },
];

export default function BlogPage() {
  return (
    <Box>
      <MarketingHero
        eyebrow="blog"
        title="Field notes on shipping AI-built software."
        subhead="Opinionated essays on gates, patches, wallet economics, ProfitGuard, and the hidden cost of vibe-coding tools."
        proofChips={["No fluff", "Mechanic-first", "Written by builders"]}
      />

      <MarketingSection>
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
          {POSTS.map((post) => (
            <Box
              key={post.slug}
              component="a"
              href="#"
              title="Coming soon"
              sx={{
                display: "flex",
                flexDirection: "column",
                p: 2.6,
                gap: 1.4,
                borderRadius: 2,
                border: `1px solid ${tokens.color.border.subtle}`,
                bgcolor: tokens.color.bg.surface,
                textDecoration: "none",
                transition: `border-color ${tokens.motion.fast} ${tokens.motion.snap}, background-color ${tokens.motion.fast} ${tokens.motion.snap}`,
                "&:hover": {
                  borderColor: tokens.color.border.strong,
                  bgcolor: tokens.color.bg.surfaceHover,
                },
              }}
            >
              <Stack direction="row" spacing={1.2} alignItems="center">
                <Typography
                  sx={{
                    fontFamily: tokens.font.mono,
                    fontSize: 11.5,
                    color: tokens.color.text.muted,
                    letterSpacing: 0.4,
                  }}
                >
                  {post.date}
                </Typography>
                <Box
                  sx={{
                    width: 3,
                    height: 3,
                    borderRadius: "50%",
                    bgcolor: tokens.color.text.muted,
                  }}
                />
                <Typography
                  sx={{
                    fontFamily: tokens.font.mono,
                    fontSize: 11,
                    color: tokens.color.accent.violet,
                    letterSpacing: 1,
                    textTransform: "uppercase",
                    fontWeight: 700,
                  }}
                >
                  {post.category}
                </Typography>
              </Stack>
              <Typography
                component="h3"
                sx={{
                  fontSize: 19,
                  fontWeight: 800,
                  letterSpacing: -0.2,
                  lineHeight: 1.25,
                  color: tokens.color.text.primary,
                }}
              >
                {post.title}
              </Typography>
              <Typography
                sx={{
                  fontSize: 14,
                  lineHeight: 1.55,
                  color: tokens.color.text.secondary,
                  flex: 1,
                }}
              >
                {post.dek}
              </Typography>
              <Stack
                direction="row"
                spacing={1}
                alignItems="center"
                sx={{ pt: 0.5, borderTop: `1px solid ${tokens.color.border.subtle}`, mt: 0.5 }}
              >
                <Box
                  sx={{
                    width: 22,
                    height: 22,
                    borderRadius: "50%",
                    background: `linear-gradient(135deg, ${tokens.color.accent.violet} 0%, ${tokens.color.accent.purple} 100%)`,
                  }}
                />
                <Typography
                  sx={{
                    fontSize: 12.5,
                    color: tokens.color.text.secondary,
                    fontWeight: 600,
                  }}
                >
                  {post.author}
                </Typography>
              </Stack>
            </Box>
          ))}
        </Box>
      </MarketingSection>

      <MarketingSection bgVariant="inset">
        <Box
          sx={{
            maxWidth: 720,
            mx: "auto",
            p: { xs: 3, md: 5 },
            borderRadius: 3,
            border: `1px solid ${tokens.color.border.strong}`,
            bgcolor: tokens.color.bg.surface,
            textAlign: "center",
          }}
        >
          <Stack spacing={2}>
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
              Newsletter
            </Typography>
            <Typography
              component="h2"
              sx={{
                fontSize: { xs: 24, md: 30 },
                fontWeight: 900,
                letterSpacing: -0.4,
                color: tokens.color.text.primary,
              }}
            >
              Get the next field note in your inbox.
            </Typography>
            <Typography
              sx={{
                fontSize: 15,
                lineHeight: 1.6,
                color: tokens.color.text.secondary,
              }}
            >
              No drip campaigns. One mechanic-first essay per release. Unsubscribe with a single click.
            </Typography>
            <Box
              component="form"
              action="#"
              data-pending="true"
              sx={{
                display: "flex",
                flexDirection: { xs: "column", sm: "row" },
                gap: 1.5,
                mt: 1,
              }}
            >
              <Box
                component="input"
                type="email"
                placeholder="you@company.com"
                aria-label="Email address"
                sx={{
                  flex: 1,
                  px: 1.8,
                  py: 1.2,
                  borderRadius: 2,
                  border: `1px solid ${tokens.color.border.subtle}`,
                  bgcolor: tokens.color.bg.inset,
                  color: tokens.color.text.primary,
                  fontFamily: tokens.font.family,
                  fontSize: 15,
                  outline: "none",
                  "&:focus": { borderColor: tokens.color.border.strong },
                }}
              />
              <Button
                type="submit"
                variant="contained"
                color="primary"
                size="large"
                sx={{ px: 3 }}
              >
                Subscribe
              </Button>
            </Box>
          </Stack>
        </Box>
      </MarketingSection>
    </Box>
  );
}
