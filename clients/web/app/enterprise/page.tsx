// app/enterprise/page.tsx — public marketing surface for the
// enterprise tier. Server component. The contact CTA at the bottom
// is a mailto link; replace with a calendar booking link when one
// exists.

import {
  AccountTreeRounded,
  AdminPanelSettingsRounded,
  ApiRounded,
  ArchitectureRounded,
  AssignmentTurnedInRounded,
  CloudRounded,
  GavelRounded,
  HandshakeRounded,
  KeyRounded,
  LockRounded,
  MemoryRounded,
  PolicyRounded,
  ReceiptLongRounded,
  ShieldRounded,
  StorageRounded,
  VerifiedUserRounded,
  VpnKeyRounded,
} from "@mui/icons-material";
import { Box, Stack, Typography } from "@mui/material";
import type { Metadata } from "next";
import type { ReactNode } from "react";
import { tokens } from "../../../../packages/design-tokens";
import {
  ComparisonTable,
  CtaBand,
  MarketingHero,
  MarketingSection,
  MechanicCard,
} from "../../src/components/marketing";
import { getRequestContent } from "../../src/lib/i18n/request";

export const metadata: Metadata = {
  title: "Enterprise — Ironflyer",
  description:
    "Bring AI-built code under your existing controls. SSO, per-tenant ledger isolation, audit log export, self-host the orchestrator, BYO keys and storage.",
  openGraph: {
    title: "Enterprise — Ironflyer",
    description:
      "SSO, per-tenant isolation, audit log export, BYO keys, self-host. AI-built code under your existing security controls.",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "Enterprise — Ironflyer",
    description:
      "SSO, per-tenant isolation, audit log export, BYO keys, self-host the orchestrator.",
  },
};

const TRUST_CHIPS = [
  "SSO",
  "Per-tenant ledger isolation",
  "Audit log export",
  "Self-host the orchestrator",
  "Postgres + pgvector or SurrealDB",
];

interface SecurityFeature {
  name: string;
  body: string;
  icon: ReactNode;
}

const SECURITY: SecurityFeature[] = [
  {
    name: "OwnerID isolation",
    body: "Every project, workspace, wallet, execution, and ledger entry carries OwnerID. requireProjectAccess returns 404 on non-owner — never a leaky 403.",
    icon: <PolicyRounded />,
  },
  {
    name: "Append-only ledger + Vault snapshot",
    body: "Every cost line, refund, and ProfitGuard verdict is an append-only entry. Vault snapshot is the source of truth for revenue − providerCost = margin.",
    icon: <ReceiptLongRounded />,
  },
  {
    name: "Gate registry = the only path to prod",
    body: "No code reaches a deploy artifact without crossing the gate chain. Custom gates plug into finisher.DefaultGates per tenant.",
    icon: <ShieldRounded />,
  },
  {
    name: "BYO LLM keys",
    body: "Bring your Anthropic, OpenAI, Gemini, and HuggingFace credentials. The orchestrator still meters cost, but the contract is between you and the provider.",
    icon: <KeyRounded />,
  },
  {
    name: "BYO S3 bucket",
    body: "S3_BACKEND=aws | r2 | minio. Artifacts, deploy bundles, and large writes land in your bucket — never in shared platform storage.",
    icon: <StorageRounded />,
  },
  {
    name: "Per-tenant memory",
    body: "IRONFLYER_MEMORY_BACKEND=pgvector | surreal | memory. Long-term memory is partitioned by tenant; no cross-tenant retrieval is permitted.",
    icon: <MemoryRounded />,
  },
];

interface DeploymentRow {
  key: string;
  option: string;
  orchestrator: string;
  runtime: string;
  ledger: string;
  ideal: string;
}

const DEPLOYMENTS: DeploymentRow[] = [
  {
    key: "cloud",
    option: "Cloud (managed)",
    orchestrator: "Ironflyer-hosted",
    runtime: "Ironflyer-hosted Docker + KVM",
    ledger: "Per-tenant DB schema",
    ideal: "Teams adopting fast, no infra ownership desired.",
  },
  {
    key: "hybrid",
    option: "Hybrid",
    orchestrator: "On-prem (Helm)",
    runtime: "Ironflyer-hosted runtime + Mac pool",
    ledger: "On-prem Postgres + pgvector",
    ideal: "Security org wants the brain on-prem; mobile builds can stay in cloud.",
  },
  {
    key: "self-hosted",
    option: "Self-hosted",
    orchestrator: "Your Kubernetes (Helm chart)",
    runtime: "Your Docker pool + your Mac pool",
    ledger: "Your Postgres + pgvector",
    ideal: "Regulated industries, on-prem mandate, or BYO infra contract.",
  },
];

const COMPLIANCE: Array<{ title: string; body: string; icon: ReactNode }> = [
  {
    title: "SOC 2 — in progress",
    body: "Type II audit underway. Penetration test + control mapping published quarterly under NDA.",
    icon: <VerifiedUserRounded />,
  },
  {
    title: "GDPR-aware data residency",
    body: "Per-tenant region pinning on hosted; on-prem keeps data in your boundary by definition. DPA available.",
    icon: <PolicyRounded />,
  },
  {
    title: "No training on customer code",
    body: "Customer code, prompts, and ledger entries are never used as training data. BYO keys make this contractual.",
    icon: <LockRounded />,
  },
  {
    title: "Audit log export",
    body: "Append-only audit events stream to your SIEM via webhook, S3 export, or scheduled GraphQL dump.",
    icon: <ReceiptLongRounded />,
  },
];

const PROCUREMENT: Array<{ step: string; title: string; body: string; icon: ReactNode }> = [
  {
    step: "01",
    title: "Intro call",
    body: "Founder-led. 30 minutes. We name the gates that solve your problem, not a generic deck.",
    icon: <HandshakeRounded />,
  },
  {
    step: "02",
    title: "POC on your stack",
    body: "Two weeks. Real workspace, real ledger, real gates. Outcome metric: one paid execution from intent to deploy artifact.",
    icon: <ArchitectureRounded />,
  },
  {
    step: "03",
    title: "Security review",
    body: "Your security team gets the architecture doc, audit log sample, BYO-keys path, and the on-prem Helm chart.",
    icon: <AdminPanelSettingsRounded />,
  },
  {
    step: "04",
    title: "MSA",
    body: "Annual, per-tenant wallet floor, named gates SLA. Termination preserves the ledger export and bucket contents.",
    icon: <GavelRounded />,
  },
];

export default async function EnterprisePage() {
  const { pages } = await getRequestContent();
  const hero = pages.enterprise;

  return (
    <Box sx={{ width: "100%", minWidth: 0 }}>
      <MarketingHero
        eyebrow={hero.eyebrow}
        title={hero.title}
        accentText={hero.titleAccent}
        subhead={hero.subhead}
        primary={{ href: "mailto:founder@ironflyer.dev?subject=Enterprise%20intro", label: hero.primary }}
        secondary={{ href: "/product", label: hero.secondary }}
        proofChips={hero.proofChips}
      />

      <MarketingSection
        eyebrow="what your security team gets"
        title="Six controls. Every one is a named mechanic."
        subhead="Not a feature list. Each row is a function in the orchestrator with a typed contract, a ledger trace, and a gate verdict."
      >
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: {
              xs: "1fr",
              md: "repeat(2, 1fr)",
              lg: "repeat(3, 1fr)",
            },
            gap: 2,
          }}
        >
          {SECURITY.map((row) => (
            <MechanicCard
              key={row.name}
              name={row.name}
              description={row.body}
              icon={row.icon}
            />
          ))}
        </Box>
      </MarketingSection>

      <MarketingSection
        eyebrow="deployment options"
        title="Three shapes. One finisher engine."
        subhead="Pick where the brain runs, where the runtime runs, and where the ledger lives. The gate chain is identical."
        bgVariant="inset"
      >
        <ComparisonTable
          columns={[
            {
              key: "option",
              label: "Option",
              highlight: true,
              width: "1fr",
            },
            { key: "orchestrator", label: "Orchestrator", width: "1.1fr" },
            { key: "runtime", label: "Runtime", width: "1.2fr" },
            { key: "ledger", label: "Ledger", width: "1fr" },
            { key: "ideal", label: "Ideal for", width: "1.4fr" },
          ]}
          rows={DEPLOYMENTS.map((d) => ({
            key: d.key,
            cells: {
              option: d.option,
              orchestrator: d.orchestrator,
              runtime: d.runtime,
              ledger: d.ledger,
              ideal: d.ideal,
            },
          }))}
          caption="Self-hosted ships as a Helm chart. Hybrid is a supported topology, not a custom build."
        />
      </MarketingSection>

      <MarketingSection
        eyebrow="compliance posture"
        title="Honest about where we are."
        subhead="Two laws drive procurement-grade trust: provable isolation and a paper trail that survives. Everything below either holds today or is dated."
      >
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "repeat(2, 1fr)" },
            gap: 2,
          }}
        >
          {COMPLIANCE.map((c) => (
            <Box
              key={c.title}
              sx={{
                p: { xs: 2.6, md: 3 },
                borderRadius: `${tokens.radius.md}px`,
                border: `1px solid ${tokens.color.border.subtle}`,
                bgcolor: `${tokens.color.bg.surface}d9`,
                display: "flex",
                gap: 2,
                alignItems: "flex-start",
              }}
            >
              <Box
                sx={{
                  display: "inline-grid",
                  placeItems: "center",
                  width: 40,
                  height: 40,
                  borderRadius: `${tokens.radius.sm}px`,
                  bgcolor: `${tokens.color.accent.violet}1f`,
                  color: tokens.color.accent.violet,
                  "& svg": { fontSize: 22 },
                  flexShrink: 0,
                }}
              >
                {c.icon}
              </Box>
              <Stack spacing={0.6} sx={{ minWidth: 0 }}>
                <Typography
                  sx={{
                    fontSize: 16.5,
                    fontWeight: 800,
                    color: tokens.color.text.primary,
                  }}
                >
                  {c.title}
                </Typography>
                <Typography
                  sx={{
                    color: tokens.color.text.secondary,
                    fontSize: 14,
                    lineHeight: 1.6,
                  }}
                >
                  {c.body}
                </Typography>
              </Stack>
            </Box>
          ))}
        </Box>
      </MarketingSection>

      <MarketingSection
        eyebrow="how procurement works"
        title="Four steps. No theater."
        subhead="From first intro to signed MSA, the path is short, founder-led, and engineered around your security review."
        bgVariant="inset"
      >
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: {
              xs: "1fr",
              sm: "repeat(2, 1fr)",
              lg: "repeat(4, 1fr)",
            },
            gap: { xs: 2, md: 2.4 },
          }}
        >
          {PROCUREMENT.map((p) => (
            <Box
              key={p.step}
              sx={{
                p: 2.6,
                borderRadius: `${tokens.radius.md}px`,
                border: `1px solid ${tokens.color.border.subtle}`,
                bgcolor: `${tokens.color.bg.surfaceRaised}d9`,
                display: "flex",
                flexDirection: "column",
                gap: 1.2,
                minHeight: 220,
              }}
            >
              <Stack direction="row" alignItems="center" spacing={1.2}>
                <Box
                  sx={{
                    display: "inline-grid",
                    placeItems: "center",
                    width: 32,
                    height: 32,
                    borderRadius: `${tokens.radius.sm}px`,
                    bgcolor: `${tokens.color.accent.violet}26`,
                    color: tokens.color.accent.violet,
                    "& svg": { fontSize: 18 },
                  }}
                >
                  {p.icon}
                </Box>
                <Typography
                  sx={{
                    fontFamily: tokens.font.mono,
                    fontSize: 11.5,
                    color: tokens.color.accent.violet,
                    fontWeight: 700,
                    letterSpacing: 1,
                  }}
                >
                  {p.step}
                </Typography>
              </Stack>
              <Typography
                sx={{
                  fontSize: 18,
                  fontWeight: 800,
                  color: tokens.color.text.primary,
                  letterSpacing: -0.2,
                }}
              >
                {p.title}
              </Typography>
              <Typography
                sx={{
                  color: tokens.color.text.secondary,
                  fontSize: 13.5,
                  lineHeight: 1.6,
                  flex: 1,
                }}
              >
                {p.body}
              </Typography>
            </Box>
          ))}
        </Box>
      </MarketingSection>

      <MarketingSection
        eyebrow="integrations"
        title="Where Ironflyer plugs in."
        subhead="Identity, storage, model providers, observability — all configurable. None of it is platform-locked."
      >
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", sm: "repeat(2, 1fr)", md: "repeat(4, 1fr)" },
            gap: 2,
          }}
        >
          {[
            { icon: <VpnKeyRounded />, title: "SAML + OIDC SSO" },
            { icon: <ApiRounded />, title: "GraphQL + webhook exports" },
            { icon: <CloudRounded />, title: "AWS / R2 / MinIO object store" },
            { icon: <AccountTreeRounded />, title: "Postgres + pgvector / SurrealDB" },
            { icon: <KeyRounded />, title: "BYO Anthropic + OpenAI + Gemini" },
            { icon: <AssignmentTurnedInRounded />, title: "Stripe webhook for top-ups" },
            { icon: <PolicyRounded />, title: "OPA / custom gate plugins" },
            { icon: <ReceiptLongRounded />, title: "SIEM-ready audit stream" },
          ].map((row) => (
            <Stack
              key={row.title}
              direction="row"
              alignItems="center"
              spacing={1.5}
              sx={{
                p: 1.8,
                borderRadius: `${tokens.radius.sm}px`,
                border: `1px solid ${tokens.color.border.subtle}`,
                bgcolor: `${tokens.color.bg.surface}cc`,
              }}
            >
              <Box
                sx={{
                  color: tokens.color.accent.violet,
                  "& svg": { fontSize: 20 },
                }}
              >
                {row.icon}
              </Box>
              <Typography
                sx={{
                  fontSize: 13,
                  fontWeight: 700,
                  color: tokens.color.text.primary,
                }}
              >
                {row.title}
              </Typography>
            </Stack>
          ))}
        </Box>
      </MarketingSection>

      <CtaBand
        heading="Talk to the founder. Skip the BDR queue."
        sub="Bring your security questionnaire, your stack, and your worst AI-code incident. We will tell you which gate would have blocked it."
        primary={{
          href: "mailto:founder@ironflyer.dev?subject=Enterprise%20intro",
          label: "Talk to founder",
        }}
        secondary={{ href: "/pricing", label: "Wallet model" }}
        chips={[
          "Founder-led intro",
          "Two-week POC",
          "Helm chart on request",
        ]}
      />
    </Box>
  );
}
