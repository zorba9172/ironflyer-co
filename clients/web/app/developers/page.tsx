// app/developers/page.tsx — public marketing route.
//
// "GraphQL is the API. Sandbox is the docs." Quick-start, capability
// matrix, REST exception list, and the @ironflyer/sdk callout.

import type { Metadata } from "next";
import Link from "next/link";
import {
  ApiRounded,
  ArrowForwardRounded,
  CodeRounded,
  HubRounded,
  InsightsRounded,
  StreamRounded,
  TerminalRounded,
  WebhookRounded,
} from "@mui/icons-material";
import { Box, Button, Stack, Typography } from "@mui/material";
import type { ReactNode } from "react";
import { tokens } from "../../../../packages/design-tokens";
import { MarketingHero } from "../../src/components/marketing/MarketingHero";
import { MarketingSection } from "../../src/components/marketing/MarketingSection";
import { getRequestContent } from "../../src/lib/i18n/request";

export const metadata: Metadata = {
  title: "Developers — Ironflyer",
  description:
    "GraphQL is the API. Sandbox is the docs. SSE for chat streams, webhooks for Stripe, Runtime REST for workspaces, Prometheus for scrape.",
  alternates: { canonical: "https://ironflyer.com/developers" },
  openGraph: {
    title: "Developers — Ironflyer",
    description:
      "GraphQL is the API. Sandbox is the docs. SSE for chat streams, webhooks for Stripe, Runtime REST for workspaces, Prometheus for scrape.",
    url: "https://ironflyer.com/developers",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "Developers — Ironflyer",
    description:
      "GraphQL is the API. Sandbox is the docs. SSE for chat streams, webhooks for Stripe, Runtime REST for workspaces, Prometheus for scrape.",
  },
};

interface Capability {
  icon: ReactNode;
  title: string;
  endpoint: string;
  body: string;
}

const CAPABILITIES: Capability[] = [
  {
    icon: <ApiRounded />,
    title: "GraphQL endpoint",
    endpoint: "POST /graphql",
    body: "Single endpoint for every query, mutation, and subscription. Subscriptions arrive on the same path via graphql-transport-ws.",
  },
  {
    icon: <HubRounded />,
    title: "Apollo Sandbox",
    endpoint: "GET /graphql/sandbox",
    body: "Live, typed documentation. Run your first query in two clicks; introspection is on for authenticated developers.",
  },
  {
    icon: <StreamRounded />,
    title: "SSE chat stream",
    endpoint: "POST /executions/{id}/chat/stream",
    body: "Raw LLM deltas as Server-Sent Events. GraphQL is wrong for per-chunk streaming; orchestration events stay on executionFeed.",
  },
  {
    icon: <WebhookRounded />,
    title: "Stripe webhook",
    endpoint: "POST /budget/webhook",
    body: "Third-party callback for wallet top-ups. Signature verified against your Stripe webhook secret before any ledger write.",
  },
  {
    icon: <TerminalRounded />,
    title: "Runtime API",
    endpoint: "/v1/workspaces/{id}/...",
    body: "Per-user workspace lifecycle: File API, PTY WebSocket, mobile Metro/emulator/xcodebuild dispatch. Owner check on every call.",
  },
  {
    icon: <InsightsRounded />,
    title: "Prometheus metrics",
    endpoint: "GET /metrics",
    body: "Scrape-ready counters and histograms for wallet, gates, ProfitGuard, finisher engine. Wire into Grafana in minutes.",
  },
];

const QUICKSTART = `query Executions {
  executions(limit: 5) {
    id
    status
    promptSummary
    wallet {
      reservedUSD
      availableUSD
    }
    gateVerdicts {
      name
      severity
      message
    }
  }
}`;

const SDK_SNIPPET = `import { Client } from '@ironflyer/sdk';

const client = new Client({
  endpoint: 'https://api.ironflyer.com/graphql',
  token: process.env.IRONFLYER_API_KEY,
});

const { executions } = await client.executions({ limit: 5 });`;

const REST_EXCEPTIONS = [
  { name: "POST /budget/webhook", body: "Stripe callback (third-party signature contract)." },
  { name: "GET /healthz, /livez, /readyz, /version", body: "Kubernetes probes." },
  { name: "GET /metrics", body: "Prometheus scrape." },
  { name: "POST /executions/{id}/chat/stream", body: "AI streaming (per-chunk SSE; gqlgen overhead is wrong here)." },
];

export default async function DevelopersPage() {
  const { pages } = await getRequestContent();
  const hero = pages.developers;

  return (
    <Box>
      <MarketingHero
        eyebrow={hero.eyebrow}
        title={hero.title}
        accentText={hero.titleAccent}
        subhead={hero.subhead}
        primary={{ href: "/settings", label: hero.primary }}
        secondary={{ href: "#quickstart", label: hero.secondary }}
        proofChips={hero.proofChips}
      />

      <MarketingSection id="quickstart" eyebrow="quickstart" title="Your first query.">
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
            gap: 3,
          }}
        >
          <Stack spacing={1.4}>
            <Typography
              sx={{ fontSize: 18, fontWeight: 800, color: tokens.color.text.primary }}
            >
              List your last five executions.
            </Typography>
            <Typography sx={{ fontSize: 14.5, lineHeight: 1.6, color: tokens.color.text.secondary }}>
              Authenticate with a project-scoped API key, point at{" "}
              <Box component="code" sx={{ fontFamily: tokens.font.mono, color: tokens.color.accent.violet }}>
                POST /graphql
              </Box>
              , and ask for the fields you need. The schema is fully typed and
              wallet state ships alongside execution rows so you never call twice.
            </Typography>
            <Typography sx={{ fontSize: 14.5, lineHeight: 1.6, color: tokens.color.text.secondary }}>
              Open Apollo Sandbox at{" "}
              <Box component="code" sx={{ fontFamily: tokens.font.mono, color: tokens.color.accent.violet }}>
                /graphql/sandbox
              </Box>{" "}
              for live introspection, schema search, and saved operations.
            </Typography>
          </Stack>
          <Box
            sx={{
              p: 2.4,
              borderRadius: 2,
              bgcolor: tokens.color.bg.inset,
              border: `1px solid ${tokens.color.border.subtle}`,
              overflow: "auto",
            }}
          >
            <Typography
              component="pre"
              sx={{
                m: 0,
                fontFamily: tokens.font.mono,
                fontSize: 13,
                lineHeight: 1.55,
                color: tokens.color.text.primary,
                whiteSpace: "pre",
              }}
            >
              {QUICKSTART}
            </Typography>
          </Box>
        </Box>
      </MarketingSection>

      <MarketingSection
        bgVariant="inset"
        eyebrow="capabilities"
        title="Six surfaces, one stack."
        subhead="Every capability below maps to a real endpoint. Nothing here is roadmap copy."
      >
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", sm: "repeat(2, minmax(0, 1fr))", lg: "repeat(3, minmax(0, 1fr))" },
            gap: 2.4,
          }}
        >
          {CAPABILITIES.map((c) => (
            <Box
              key={c.title}
              sx={{
                p: 2.4,
                borderRadius: 2,
                border: `1px solid ${tokens.color.border.subtle}`,
                bgcolor: tokens.color.bg.surface,
              }}
            >
              <Box
                sx={{
                  width: 38,
                  height: 38,
                  borderRadius: 1.5,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  bgcolor: `${tokens.color.accent.violet}1f`,
                  color: tokens.color.accent.violet,
                  border: `1px solid ${tokens.color.accent.violet}44`,
                  mb: 1.4,
                }}
              >
                {c.icon}
              </Box>
              <Typography sx={{ fontSize: 16.5, fontWeight: 800, color: tokens.color.text.primary }}>
                {c.title}
              </Typography>
              <Box
                sx={{
                  display: "inline-block",
                  mt: 0.6,
                  mb: 1,
                  px: 1,
                  py: 0.3,
                  borderRadius: 0.8,
                  bgcolor: tokens.color.bg.inset,
                  border: `1px solid ${tokens.color.border.subtle}`,
                  fontFamily: tokens.font.mono,
                  fontSize: 12,
                  color: tokens.color.brand.mint,
                }}
              >
                {c.endpoint}
              </Box>
              <Typography sx={{ fontSize: 14, lineHeight: 1.6, color: tokens.color.text.secondary }}>
                {c.body}
              </Typography>
            </Box>
          ))}
        </Box>
      </MarketingSection>

      <MarketingSection
        eyebrow="sdk"
        title="@ironflyer/sdk on npm."
        subhead="A thin, typed TypeScript client that fronts both the orchestrator GraphQL and the runtime REST. Works in Node 20+, edge runtimes, and the browser."
      >
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "1.1fr 0.9fr" },
            gap: 3,
            alignItems: "stretch",
          }}
        >
          <Box
            sx={{
              p: 2.4,
              borderRadius: 2,
              bgcolor: tokens.color.bg.inset,
              border: `1px solid ${tokens.color.border.subtle}`,
              overflow: "auto",
            }}
          >
            <Typography
              component="pre"
              sx={{
                m: 0,
                fontFamily: tokens.font.mono,
                fontSize: 13,
                lineHeight: 1.55,
                color: tokens.color.text.primary,
                whiteSpace: "pre",
              }}
            >
              {SDK_SNIPPET}
            </Typography>
          </Box>
          <Stack
            spacing={1.6}
            sx={{
              p: 2.4,
              borderRadius: 2,
              border: `1px solid ${tokens.color.border.strong}`,
              bgcolor: tokens.color.bg.surface,
            }}
          >
            <Box sx={{ display: "flex", alignItems: "center", gap: 1.2 }}>
              <CodeRounded sx={{ color: tokens.color.accent.coral }} />
              <Typography sx={{ fontWeight: 800, fontSize: 17, color: tokens.color.text.primary }}>
                npm install @ironflyer/sdk
              </Typography>
            </Box>
            <Typography sx={{ fontSize: 14, lineHeight: 1.6, color: tokens.color.text.secondary }}>
              Typed GraphQL operations, runtime helpers for File API and PTY
              WebSocket, and a wallet-aware retry policy that respects the 402
              hard-block contract.
            </Typography>
            <Box
              sx={{
                p: 1.2,
                borderRadius: 1.4,
                bgcolor: `${tokens.color.accent.violet}10`,
                border: `1px solid ${tokens.color.accent.violet}44`,
              }}
            >
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 11,
                  letterSpacing: 1.2,
                  textTransform: "uppercase",
                  color: tokens.color.accent.violet,
                  fontWeight: 700,
                }}
              >
                Operator banner
              </Typography>
              <Typography sx={{ fontSize: 13.5, color: tokens.color.text.secondary, mt: 0.4 }}>
                <Box component="code" sx={{ fontFamily: tokens.font.mono }}>
                  GET /
                </Box>{" "}
                returns a JSON pointer to /graphql, /graphql/sandbox, and
                docs/V22_PLAN.md so new integrators land at the schema, not a 404.
              </Typography>
            </Box>
          </Stack>
        </Box>
      </MarketingSection>

      <MarketingSection
        bgVariant="inset"
        eyebrow="rest exceptions"
        title="Four REST routes that stay REST forever."
        subhead="Everything else is GraphQL. These four exist for a reason — they are documented here so you do not need to hunt the source for them."
      >
        <Stack spacing={1.6}>
          {REST_EXCEPTIONS.map((row) => (
            <Box
              key={row.name}
              sx={{
                p: 2.2,
                borderRadius: 2,
                border: `1px solid ${tokens.color.border.subtle}`,
                bgcolor: tokens.color.bg.surface,
                display: "grid",
                gridTemplateColumns: { xs: "1fr", md: "320px 1fr" },
                gap: 2,
              }}
            >
              <Box
                sx={{
                  px: 1.1,
                  py: 0.6,
                  borderRadius: 0.8,
                  bgcolor: tokens.color.bg.inset,
                  border: `1px solid ${tokens.color.border.subtle}`,
                  fontFamily: tokens.font.mono,
                  fontSize: 13,
                  color: tokens.color.brand.mint,
                  alignSelf: "flex-start",
                }}
              >
                {row.name}
              </Box>
              <Typography sx={{ fontSize: 14.5, lineHeight: 1.6, color: tokens.color.text.secondary }}>
                {row.body}
              </Typography>
            </Box>
          ))}
        </Stack>

        <Stack
          direction={{ xs: "column", sm: "row" }}
          spacing={1.5}
          sx={{ pt: 4, justifyContent: "center" }}
        >
          <Button
            component={Link}
            href="/settings"
            variant="contained"
            color="primary"
            size="large"
            endIcon={<ArrowForwardRounded sx={{ fontSize: 18 }} />}
          >
            Get an API key
          </Button>
          <Button
            component={Link}
            href="#"
            variant="text"
            size="large"
            sx={{ color: tokens.color.accent.violet }}
          >
            Read the docs
          </Button>
        </Stack>
      </MarketingSection>
    </Box>
  );
}
