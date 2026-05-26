// app/security/page.tsx — public marketing route.
//
// How we treat code, keys, and data. Production posture spelled out
// against the V22 contract: OwnerID isolation, prepaid wallet,
// append-only ledger, BYO keys and bucket.

import type { Metadata } from "next";
import Link from "next/link";
import {
  AccountBalanceWalletOutlined,
  AdminPanelSettingsRounded,
  ArrowForwardRounded,
  CloudOutlined,
  GavelRounded,
  KeyRounded,
  LockOutlined,
  MemoryRounded,
  PolicyRounded,
  ReceiptLongRounded,
  ReportProblemRounded,
  SecurityRounded,
  ShieldOutlined,
} from "@mui/icons-material";
import { Box, Button, Stack, Typography } from "@mui/material";
import type { ReactNode } from "react";
import { tokens } from "../../../../packages/design-tokens";
import { MarketingHero } from "../../src/components/marketing/MarketingHero";
import { MarketingSection } from "../../src/components/marketing/MarketingSection";

export const metadata: Metadata = {
  title: "Security — Ironflyer",
  description:
    "Production-grade by default. OwnerID isolation, prepaid wallet hard-block, append-only ledger, BYO LLM keys, BYO S3 bucket.",
  alternates: { canonical: "https://ironflyer.com/security" },
  openGraph: {
    title: "Security — Ironflyer",
    description:
      "Production-grade by default. OwnerID isolation, prepaid wallet hard-block, append-only ledger, BYO LLM keys, BYO S3 bucket.",
    url: "https://ironflyer.com/security",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "Security — Ironflyer",
    description:
      "Production-grade by default. OwnerID isolation, prepaid wallet hard-block, append-only ledger, BYO LLM keys, BYO S3 bucket.",
  },
};

interface SecurityCard {
  icon: ReactNode;
  title: string;
  body: string;
}

const CARDS: SecurityCard[] = [
  {
    icon: <ShieldOutlined />,
    title: "OwnerID isolation",
    body: "Every project, workspace, wallet, and ledger row carries an OwnerID. requireProjectAccess returns 404 on non-owner reads — never a 403 leak.",
  },
  {
    icon: <AccountBalanceWalletOutlined />,
    title: "Prepaid wallet hard-block",
    body: "No execution starts without budget. The orchestrator returns HTTP 402 with a top_up_url before any LLM, sandbox, or build call leaves the gate.",
  },
  {
    icon: <ReceiptLongRounded />,
    title: "Append-only ledger + Vault snapshot",
    body: "Every cost line is immutable and signed into a Vault snapshot. revenue − providerCost = margin is replayable from the ledger at any time.",
  },
  {
    icon: <KeyRounded />,
    title: "BYO LLM API keys",
    body: "Bring your own Anthropic, OpenAI, Gemini, HuggingFace, or DeepSeek key. We never share a customer key across tenants, ever.",
  },
  {
    icon: <CloudOutlined />,
    title: "BYO S3 bucket",
    body: "S3_BACKEND=aws|r2|minio lets you point artifacts at your own bucket with your own retention policy. Zero-egress R2 supported.",
  },
  {
    icon: <MemoryRounded />,
    title: "Per-tenant memory store",
    body: "The memory backend (in-process, SurrealDB, or pgvector) is namespaced per OwnerID. Vector reads are scoped before they hit the index.",
  },
  {
    icon: <PolicyRounded />,
    title: "Signed patch lifecycle",
    body: "The AI never writes files directly. patch.Engine.Propose → review gates → apply. Every patch carries a signed provenance record.",
  },
  {
    icon: <LockOutlined />,
    title: "No training on customer code",
    body: "We do not train, fine-tune, or share customer source or prompts with any third party. Provider calls run with their no-training flags set.",
  },
];

const NEVER_LIST = [
  "No shadow training on customer code, prompts, or patches.",
  "No third-party sharing of customer LLM keys, secrets, or bucket credentials.",
  "No public artifact buckets — every signed URL is short-lived and scoped.",
  "No skipping gates for paying customers. Gates block regardless of plan tier.",
];

export default function SecurityPage() {
  return (
    <Box>
      <MarketingHero
        eyebrow="security"
        title="Production-grade by default."
        subhead="The V22 contract spells out how Ironflyer treats your code, your keys, and your data. Eight controls below, no asterisks."
        primary={{ href: "/enterprise", label: "Talk to security" }}
        secondary={{ href: "#disclosure", label: "Report a vulnerability" }}
        proofChips={["OwnerID enforced", "Wallet hard-block", "Append-only ledger"]}
      />

      <MarketingSection
        eyebrow="controls"
        title="Eight controls, named and enforced."
        subhead="Every control below maps to a concrete file or middleware in the orchestrator. No hand-wave 'we take security seriously' filler."
      >
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: {
              xs: "1fr",
              sm: "repeat(2, minmax(0, 1fr))",
              lg: "repeat(4, minmax(0, 1fr))",
            },
            gap: 2.4,
          }}
        >
          {CARDS.map((card) => (
            <Box
              key={card.title}
              sx={{
                p: 2.6,
                borderRadius: 2,
                border: `1px solid ${tokens.color.border.subtle}`,
                bgcolor: tokens.color.bg.surface,
                display: "flex",
                flexDirection: "column",
                gap: 1.2,
                transition: `border-color ${tokens.motion.fast} ${tokens.motion.snap}`,
                "&:hover": { borderColor: tokens.color.border.strong },
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
                }}
              >
                {card.icon}
              </Box>
              <Typography
                sx={{
                  fontSize: 16,
                  fontWeight: 800,
                  color: tokens.color.text.primary,
                  letterSpacing: -0.2,
                }}
              >
                {card.title}
              </Typography>
              <Typography
                sx={{ fontSize: 14, lineHeight: 1.55, color: tokens.color.text.secondary }}
              >
                {card.body}
              </Typography>
            </Box>
          ))}
        </Box>
      </MarketingSection>

      <MarketingSection
        bgVariant="inset"
        eyebrow="compliance"
        title="Compliance posture."
        subhead="What is in flight today and what is honored by design."
      >
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "repeat(3, minmax(0, 1fr))" },
            gap: 2.4,
          }}
        >
          {[
            {
              icon: <GavelRounded />,
              title: "SOC 2 Type II",
              status: "In progress",
              body: "Type I audit complete with the controls above; Type II observation window opens once production sees commercial workloads.",
            },
            {
              icon: <SecurityRounded />,
              title: "GDPR-aware",
              status: "By design",
              body: "Data residency selectable per project. PII never leaves the tenant boundary defined by OwnerID and bucket selection.",
            },
            {
              icon: <AdminPanelSettingsRounded />,
              title: "Role-based access",
              status: "Shipping",
              body: "Operator / Admin / Owner roles, per-project owner checks, and an audit log for every secret read by the runtime.",
            },
          ].map((card) => (
            <Box
              key={card.title}
              sx={{
                p: 2.6,
                borderRadius: 2,
                border: `1px solid ${tokens.color.border.subtle}`,
                bgcolor: tokens.color.bg.surface,
              }}
            >
              <Stack direction="row" spacing={1.2} alignItems="center" sx={{ mb: 1 }}>
                <Box
                  sx={{
                    width: 34,
                    height: 34,
                    borderRadius: 1.5,
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    bgcolor: `${tokens.color.brand.mint}1a`,
                    color: tokens.color.brand.mint,
                  }}
                >
                  {card.icon}
                </Box>
                <Stack>
                  <Typography sx={{ fontWeight: 800, color: tokens.color.text.primary }}>
                    {card.title}
                  </Typography>
                  <Typography
                    sx={{
                      fontFamily: tokens.font.mono,
                      fontSize: 11,
                      letterSpacing: 0.8,
                      textTransform: "uppercase",
                      color: tokens.color.brand.mint,
                      fontWeight: 700,
                    }}
                  >
                    {card.status}
                  </Typography>
                </Stack>
              </Stack>
              <Typography sx={{ fontSize: 14, lineHeight: 1.55, color: tokens.color.text.secondary }}>
                {card.body}
              </Typography>
            </Box>
          ))}
        </Box>
      </MarketingSection>

      <MarketingSection id="disclosure" eyebrow="disclosure" title="Reporting a vulnerability.">
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "1.2fr 1fr" },
            gap: 3,
          }}
        >
          <Box
            sx={{
              p: { xs: 3, md: 4 },
              borderRadius: 3,
              border: `1px solid ${tokens.color.border.strong}`,
              bgcolor: tokens.color.bg.surface,
            }}
          >
            <Stack spacing={1.8}>
              <Stack direction="row" spacing={1.4} alignItems="center">
                <Box
                  sx={{
                    width: 38,
                    height: 38,
                    borderRadius: 1.5,
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    bgcolor: `${tokens.color.accent.warning}1f`,
                    color: tokens.color.accent.warning,
                  }}
                >
                  <ReportProblemRounded />
                </Box>
                <Typography
                  sx={{ fontSize: 20, fontWeight: 800, color: tokens.color.text.primary }}
                >
                  Coordinated disclosure
                </Typography>
              </Stack>
              <Typography sx={{ fontSize: 15, lineHeight: 1.6, color: tokens.color.text.secondary }}>
                We honor a 90-day coordinated disclosure window. Submit a writeup
                with a reproduction path and we acknowledge within one business
                day. Critical findings are triaged the same day.
              </Typography>
              <Box
                component="a"
                href="mailto:security@ironflyer.com"
                sx={{
                  alignSelf: "flex-start",
                  display: "inline-flex",
                  alignItems: "center",
                  gap: 1,
                  px: 1.6,
                  py: 0.9,
                  borderRadius: 999,
                  border: `1px solid ${tokens.color.border.strong}`,
                  bgcolor: tokens.color.bg.inset,
                  color: tokens.color.text.primary,
                  textDecoration: "none",
                  fontFamily: tokens.font.mono,
                  fontSize: 13,
                  "&:hover": { bgcolor: tokens.color.bg.surfaceHover },
                }}
              >
                security@ironflyer.com
              </Box>
            </Stack>
          </Box>
          <Box
            sx={{
              p: { xs: 3, md: 4 },
              borderRadius: 3,
              border: `1px solid ${tokens.color.border.subtle}`,
              bgcolor: tokens.color.bg.surface,
            }}
          >
            <Typography
              sx={{
                fontFamily: tokens.font.mono,
                fontSize: 11.5,
                letterSpacing: 1.4,
                textTransform: "uppercase",
                color: tokens.color.accent.danger,
                fontWeight: 700,
                mb: 1.4,
              }}
            >
              What we never do
            </Typography>
            <Stack component="ul" spacing={1} sx={{ pl: 2.5, m: 0 }}>
              {NEVER_LIST.map((line) => (
                <Typography
                  component="li"
                  key={line}
                  sx={{ fontSize: 14.5, lineHeight: 1.6, color: tokens.color.text.secondary }}
                >
                  {line}
                </Typography>
              ))}
            </Stack>
          </Box>
        </Box>

        <Stack
          direction={{ xs: "column", sm: "row" }}
          spacing={1.5}
          sx={{ pt: 4, justifyContent: "center" }}
        >
          <Button
            component={Link}
            href="/enterprise"
            variant="contained"
            color="primary"
            size="large"
            endIcon={<ArrowForwardRounded sx={{ fontSize: 18 }} />}
          >
            Talk to security
          </Button>
          <Button
            component={Link}
            href="/pricing"
            variant="text"
            size="large"
            sx={{ color: tokens.color.accent.violet }}
          >
            Review pricing
          </Button>
        </Stack>
      </MarketingSection>
    </Box>
  );
}
