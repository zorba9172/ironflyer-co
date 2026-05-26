// app/pricing/page.tsx — public marketing route for the V22 wallet
// pricing model. Server component; no client state required. The
// existing client PricingPage (with live GraphQL Plans/Rates) lives
// at /src/components/PricingPage and is reachable from the dashboard
// once a visitor signs in.

import {
  AccountBalanceWalletRounded,
  AutoGraphRounded,
  BoltRounded,
  CheckRounded,
  LockRounded,
  ReceiptLongRounded,
  ShieldRounded,
  TerminalRounded,
} from "@mui/icons-material";
import { Box, Button, Stack, Typography } from "@mui/material";
import type { Metadata } from "next";
import Link from "next/link";
import { tokens } from "../../../../packages/design-tokens";
import {
  ComparisonTable,
  CtaBand,
  FaqAccordion,
  MarketingHero,
  MarketingSection,
  MechanicCard,
} from "../../src/components/marketing";

export const metadata: Metadata = {
  title: "Pricing — Ironflyer",
  description:
    "Prepaid wallet, not a subscription. Pay only for completed executions. ProfitGuard meters every expensive call before it runs.",
  openGraph: {
    title: "Pricing — Ironflyer",
    description:
      "Prepaid wallet, debit per execution, ProfitGuard guards every expensive call. Free, Pro, and Enterprise tiers.",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "Pricing — Ironflyer",
    description:
      "Prepaid wallet, debit per execution, ProfitGuard guards every expensive call.",
  },
};

interface PlanTier {
  name: string;
  price: string;
  cadence: string;
  pitch: string;
  features: string[];
  cta: { label: string; href: string };
  highlight?: boolean;
  badge?: string;
}

const PLANS: PlanTier[] = [
  {
    name: "Free",
    price: "$0",
    cadence: "no card required",
    pitch: "Stand up a workspace and feel the gates without spending a cent.",
    features: [
      "Mock Docker workspace",
      "GateBudget capped at a $5 wallet",
      "1 project, public demo seed",
      "ProfitGuard preview mode",
      "Community support",
    ],
    cta: { label: "Start free", href: "/signup" },
  },
  {
    name: "Pro",
    price: "$29",
    cadence: "per month + wallet top-ups",
    pitch: "For founders shipping paid product through real Docker workspaces.",
    badge: "Most chosen",
    highlight: true,
    features: [
      "Real Docker workspaces",
      "EAS mobile cloud build (Expo + Android)",
      "10 projects with OwnerID isolation",
      "ProfitGuard standard tier",
      "Wallet top-ups from $10",
      "Email support",
    ],
    cta: { label: "Start Pro", href: "/signup?tier=pro" },
  },
  {
    name: "Enterprise",
    price: "Talk to us",
    cadence: "annual + per-tenant wallet",
    pitch: "When security, SSO, and Mac pool iOS native are mandatory.",
    features: [
      "Mac pool for iOS native builds",
      "SSO + RBAC",
      "Per-tenant ledger isolation",
      "pgvector long-term memory",
      "Audit log export",
      "Self-host the orchestrator",
    ],
    cta: { label: "Talk to founder", href: "/enterprise" },
  },
];

const WALLET_STEPS: Array<{
  step: string;
  title: string;
  body: string;
  icon: React.ReactNode;
}> = [
  {
    step: "01",
    title: "Top up",
    body: "Stripe Checkout deposits credit into the prepaid wallet. The deposit lands in the ledger as a positive entry before any reasoning runs.",
    icon: <AccountBalanceWalletRounded />,
  },
  {
    step: "02",
    title: "Reserve",
    body: "Before an execution starts, the orchestrator places a wallet hold sized by ProfitGuard. Without sufficient balance, the API returns 402 with a top_up_url.",
    icon: <LockRounded />,
  },
  {
    step: "03",
    title: "Debit on materialization",
    body: "Provider tokens, sandbox minutes, and build minutes debit the ledger only as real cost lands. Unused hold releases on commit.",
    icon: <ReceiptLongRounded />,
  },
];

const RATE_ROWS = [
  {
    key: "tokens",
    item: "Provider tokens (LLM)",
    unit: "per 1M prompt / completion tokens",
    ledger: "EntryProviderTokens",
    pricing: "Pass-through provider rate sheet",
  },
  {
    key: "sandbox",
    item: "Docker workspace minutes",
    unit: "per minute, vCPU-scaled",
    ledger: "EntrySandboxMin",
    pricing: "$0.012 / min (Pro standard tier)",
  },
  {
    key: "mobile",
    item: "Mobile build minutes",
    unit: "per minute, separate from sandbox",
    ledger: "EntryMobileBuildMin",
    pricing: "$0.035 / min (Expo + Android)",
  },
  {
    key: "ios",
    item: "Mac workspace minutes (iOS native)",
    unit: "per minute, MacStadium pool",
    ledger: "EntryMacWorkspaceMin",
    pricing: "$0.180 / min (Enterprise only)",
  },
  {
    key: "deploy",
    item: "Deploy artifacts",
    unit: "per artifact persisted to S3",
    ledger: "EntryDeployArtifact",
    pricing: "$0.04 / artifact",
  },
  {
    key: "artifact",
    item: "Large artifact writes",
    unit: "per GB stored in your bucket",
    ledger: "EntryArtifactStorageGB",
    pricing: "$0.025 / GB / month",
  },
];

const PRICING_FAQ = [
  {
    question: "Is the wallet refundable?",
    answer:
      "Unused wallet balance is refundable on subscription cancellation, minus any provider cost already materialized in the ledger. Reservations that never run release fully on commit; nothing is debited on a phantom execution.",
  },
  {
    question: "What's the minimum top-up?",
    answer:
      "$10 on Pro, $100 on Enterprise. Free tier caps at $5 in lifetime wallet credit so you can feel the gates before deciding to pay.",
  },
  {
    question: "What cancels a wallet reservation?",
    answer:
      "Any gate verdict that blocks the execution (GateBudget, GateProfitGuard, GatePatchReview, GateBuild, GateMobileBuild, GateSecurityScan) releases the reservation. Cost is debited only when work materializes.",
  },
  {
    question: "Does ProfitGuard ever let an execution lose money?",
    answer:
      "No. ProfitGuard refuses any reasoning, sandbox, mobile build, or deploy that would push the user's wallet negative against expected ROI. The decision lands in the ledger before the spend.",
  },
  {
    question: "Can I bring my own provider keys?",
    answer:
      "Enterprise customers can BYO LLM keys, BYO S3, and BYO Postgres. The orchestrator still meters the ledger so margin remains observable per execution.",
  },
  {
    question: "How are mobile builds metered?",
    answer:
      "Mobile builds debit EntryMobileBuildMin, separate from sandbox minutes. iOS native builds debit EntryMacWorkspaceMin against the Mac pool. EAS cloud credits are debited as EntryEASBuildCredit.",
  },
  {
    question: "What happens at $0 wallet during an execution?",
    answer:
      "The next gate refuses, the execution pauses with a paid_blocked status, and the user gets a top_up_url. Already-materialized cost stays debited; future work waits until balance returns.",
  },
  {
    question: "Do you train on customer code?",
    answer:
      "No. Customer code, prompts, and ledger entries are never used as training data for any provider. BYO keys make this contractual; pass-through providers see only the prompt their API requires.",
  },
];

export default function PricingPage() {
  return (
    <Box sx={{ width: "100%", minWidth: 0 }}>
      <MarketingHero
        eyebrow="pricing"
        title="Pay for completed executions, not for promises."
        subhead="Ironflyer runs on a prepaid wallet, debits the ledger only as cost materializes, and lets ProfitGuard refuse every expensive call that would push margin negative. No seat-based smoke. No surprise overage."
        primary={{ href: "/signup", label: "Start with $0 wallet" }}
        secondary={{ href: "#wallet", label: "How the wallet works" }}
        proofChips={[
          "ProfitGuard before every call",
          "Append-only ledger",
          "402 Payment Required, not silent failure",
        ]}
      />

      <MarketingSection
        eyebrow="three tiers"
        title="Pick the workspace floor, then top up the wallet."
        subhead="Free is for kicking the tires on the gate system. Pro is the default for shipping paid product. Enterprise is for teams that need iOS native, SSO, and tenant isolation."
      >
        <Box
          sx={{
            display: "grid",
            gap: { xs: 2.5, md: 3 },
            gridTemplateColumns: { xs: "1fr", md: "repeat(3, 1fr)" },
          }}
        >
          {PLANS.map((plan) => (
            <Box
              key={plan.name}
              sx={{
                position: "relative",
                p: { xs: 3, md: 3.5 },
                borderRadius: `${tokens.radius.lg}px`,
                border: `1px solid ${plan.highlight ? tokens.color.accent.violet : tokens.color.border.subtle}`,
                bgcolor: plan.highlight
                  ? `${tokens.color.bg.surfaceRaised}f2`
                  : `${tokens.color.bg.surface}d9`,
                display: "flex",
                flexDirection: "column",
                gap: 2,
                boxShadow: plan.highlight ? tokens.shadow.lg : "none",
              }}
            >
              {plan.badge && (
                <Box
                  sx={{
                    position: "absolute",
                    top: -12,
                    right: 16,
                    px: 1.4,
                    py: 0.5,
                    borderRadius: 999,
                    background: `linear-gradient(100deg, ${tokens.color.accent.coral}, ${tokens.color.brand.magenta}, ${tokens.color.accent.purple})`,
                    fontFamily: tokens.font.mono,
                    fontSize: 10.5,
                    letterSpacing: 0.8,
                    textTransform: "uppercase",
                    color: tokens.color.text.primary,
                    fontWeight: 800,
                  }}
                >
                  {plan.badge}
                </Box>
              )}
              <Stack spacing={0.6}>
                <Typography
                  sx={{
                    fontSize: 14,
                    fontFamily: tokens.font.mono,
                    letterSpacing: 0.6,
                    textTransform: "uppercase",
                    color: plan.highlight
                      ? tokens.color.accent.violet
                      : tokens.color.text.muted,
                    fontWeight: 700,
                  }}
                >
                  {plan.name}
                </Typography>
                <Typography
                  sx={{
                    fontFamily: tokens.font.mono,
                    fontSize: { xs: 36, md: 44 },
                    fontWeight: 800,
                    letterSpacing: -1,
                    lineHeight: 1,
                    color: tokens.color.text.primary,
                  }}
                >
                  {plan.price}
                </Typography>
                <Typography
                  sx={{
                    color: tokens.color.text.muted,
                    fontSize: 12.5,
                  }}
                >
                  {plan.cadence}
                </Typography>
              </Stack>
              <Typography
                sx={{
                  color: tokens.color.text.secondary,
                  fontSize: 14,
                  lineHeight: 1.55,
                  minHeight: 42,
                }}
              >
                {plan.pitch}
              </Typography>
              <Stack spacing={1.2} sx={{ flex: 1 }}>
                {plan.features.map((f) => (
                  <Stack
                    key={f}
                    direction="row"
                    alignItems="flex-start"
                    spacing={1}
                  >
                    <CheckRounded
                      sx={{
                        color: plan.highlight
                          ? tokens.color.accent.violet
                          : tokens.color.accent.success,
                        fontSize: 18,
                        mt: 0.2,
                        flexShrink: 0,
                      }}
                    />
                    <Typography
                      sx={{
                        color: tokens.color.text.primary,
                        fontSize: 14,
                        fontWeight: 500,
                      }}
                    >
                      {f}
                    </Typography>
                  </Stack>
                ))}
              </Stack>
              <Button
                component={Link}
                href={plan.cta.href}
                variant={plan.highlight ? "contained" : "outlined"}
                color={plan.highlight ? "primary" : "secondary"}
                size="large"
                fullWidth
                sx={
                  plan.highlight
                    ? { mt: 1 }
                    : {
                        mt: 1,
                        borderColor: tokens.color.border.strong,
                        color: tokens.color.text.primary,
                        "&:hover": {
                          borderColor: tokens.color.accent.violet,
                          bgcolor: tokens.color.bg.surfaceHover,
                        },
                      }
                }
              >
                {plan.cta.label}
              </Button>
            </Box>
          ))}
        </Box>
      </MarketingSection>

      <MarketingSection
        id="wallet"
        eyebrow="how the wallet works"
        title="Top up. Reserve. Debit on materialization."
        subhead="Three states, recorded as ledger entries. No invoices reconstructed at month end — every cent is attributable to one execution."
        bgVariant="inset"
      >
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "repeat(3, 1fr)" },
            gap: { xs: 2.5, md: 3 },
            position: "relative",
          }}
        >
          {WALLET_STEPS.map((step) => (
            <Box
              key={step.step}
              sx={{
                p: { xs: 2.8, md: 3.2 },
                borderRadius: `${tokens.radius.md}px`,
                border: `1px solid ${tokens.color.border.subtle}`,
                bgcolor: `${tokens.color.bg.surfaceRaised}d9`,
                display: "flex",
                flexDirection: "column",
                gap: 1.5,
              }}
            >
              <Stack direction="row" alignItems="center" spacing={1.5}>
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
                  }}
                >
                  {step.icon}
                </Box>
                <Typography
                  sx={{
                    fontFamily: tokens.font.mono,
                    fontSize: 13,
                    color: tokens.color.accent.violet,
                    fontWeight: 700,
                    letterSpacing: 1,
                  }}
                >
                  {step.step}
                </Typography>
              </Stack>
              <Typography
                sx={{
                  fontSize: 19,
                  fontWeight: 800,
                  color: tokens.color.text.primary,
                  letterSpacing: -0.2,
                }}
              >
                {step.title}
              </Typography>
              <Typography
                sx={{
                  color: tokens.color.text.secondary,
                  fontSize: 14,
                  lineHeight: 1.6,
                }}
              >
                {step.body}
              </Typography>
            </Box>
          ))}
        </Box>
      </MarketingSection>

      <MarketingSection
        eyebrow="what you actually pay for"
        title="Itemized against the ledger. No bundled meters."
        subhead="Every line below maps to a named EntryType in core/orchestrator/internal/business/ledger. You can audit the bill with a SQL query."
      >
        <ComparisonTable
          columns={[
            { key: "item", label: "Cost line", highlight: true, width: "1.4fr" },
            { key: "unit", label: "Unit", width: "1.2fr" },
            { key: "ledger", label: "Ledger entry", width: "1fr" },
            { key: "pricing", label: "Price", width: "1.1fr" },
          ]}
          rows={RATE_ROWS.map((r) => ({
            key: r.key,
            cells: {
              item: r.item,
              unit: r.unit,
              ledger: (
                <Box
                  component="span"
                  sx={{
                    fontFamily: tokens.font.mono,
                    fontSize: 12.5,
                    color: tokens.color.accent.violet,
                  }}
                >
                  {r.ledger}
                </Box>
              ),
              pricing: r.pricing,
            },
          }))}
          caption="Provider token rates are pass-through against published vendor pricing. Sandbox/mobile/Mac rates are platform-managed and may move with infrastructure cost; changes ship through a public changelog."
        />
      </MarketingSection>

      <MarketingSection
        eyebrow="ProfitGuard"
        title="The decision that runs before every expensive call."
        subhead="ProfitGuard is the third law of the V22 spec: no scale is healthy unless margin is protected. It sits in front of premium model calls, sandbox allocation, mobile builds, deploy artifacts, and retry loops."
      >
        <Box
          sx={{
            p: { xs: 3, md: 4 },
            borderRadius: `${tokens.radius.lg}px`,
            border: `1px solid ${tokens.color.border.accent}`,
            background: `linear-gradient(180deg, ${tokens.color.bg.surfaceRaised}f2, ${tokens.color.bg.inset}f5)`,
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "1.2fr 0.8fr" },
            gap: { xs: 3, md: 5 },
            alignItems: "center",
          }}
        >
          <Stack spacing={2}>
            <Stack direction="row" alignItems="center" spacing={1.5}>
              <ShieldRounded
                sx={{
                  color: tokens.color.accent.violet,
                  fontSize: 28,
                }}
              />
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 14,
                  color: tokens.color.accent.violet,
                  fontWeight: 700,
                  letterSpacing: 0.8,
                }}
              >
                GateProfitGuard
              </Typography>
            </Stack>
            <Typography
              sx={{
                fontSize: { xs: 22, md: 28 },
                fontWeight: 800,
                color: tokens.color.text.primary,
                letterSpacing: -0.4,
                lineHeight: 1.2,
              }}
            >
              Expected ROI in. Decision out. Ledger entry recorded.
            </Typography>
            <Typography
              sx={{
                color: tokens.color.text.secondary,
                fontSize: 15,
                lineHeight: 1.65,
              }}
            >
              ProfitGuard sizes the wallet reservation against expected
              ROI, blocks when expected cost exceeds margin headroom,
              and writes its verdict to the ledger before the spend.
              The Vault snapshot remains the source of truth for
              <Box
                component="span"
                sx={{
                  fontFamily: tokens.font.mono,
                  color: tokens.color.accent.violet,
                  mx: 0.5,
                }}
              >
                revenue − providerCost = margin
              </Box>
              at the platform aggregate.
            </Typography>
          </Stack>
          <Stack spacing={1.5}>
            {[
              {
                icon: <AutoGraphRounded />,
                label: "Premium model calls",
              },
              {
                icon: <TerminalRounded />,
                label: "Sandbox + mobile builds",
              },
              {
                icon: <BoltRounded />,
                label: "Deploy + retry loops",
              },
              {
                icon: <ReceiptLongRounded />,
                label: "Long verification runs",
              },
            ].map((item) => (
              <Stack
                key={item.label}
                direction="row"
                alignItems="center"
                spacing={1.5}
                sx={{
                  p: 1.5,
                  borderRadius: `${tokens.radius.sm}px`,
                  border: `1px solid ${tokens.color.border.subtle}`,
                  bgcolor: `${tokens.color.bg.base}80`,
                }}
              >
                <Box
                  sx={{
                    color: tokens.color.accent.violet,
                    "& svg": { fontSize: 20 },
                  }}
                >
                  {item.icon}
                </Box>
                <Typography
                  sx={{
                    fontSize: 13.5,
                    fontWeight: 700,
                    color: tokens.color.text.primary,
                  }}
                >
                  {item.label}
                </Typography>
              </Stack>
            ))}
          </Stack>
        </Box>
      </MarketingSection>

      <MarketingSection
        eyebrow="frequently asked"
        title="Pricing questions, answered against the mechanics."
        subhead="If the answer is not here, the orchestrator probably refuses the request. Reach out and we will name the gate that blocks."
      >
        <FaqAccordion items={PRICING_FAQ} />
      </MarketingSection>

      <Box sx={{ maxWidth: 1180, mx: "auto", width: "100%", minWidth: 0 }}>
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "repeat(3, 1fr)" },
            gap: 2,
            mb: { xs: 4, md: 6 },
          }}
        >
          <MechanicCard
            name="GateBudget"
            description="Refuses execution start without sufficient wallet hold. Returns 402 Payment Required with top_up_url, never a silent fail."
            icon={<LockRounded />}
          />
          <MechanicCard
            name="EntryProviderTokens"
            description="One ledger entry per provider call. Token cost lands as it materializes, not at month end."
            icon={<ReceiptLongRounded />}
            accent="coral"
          />
          <MechanicCard
            name="Vault snapshot"
            description="Append-only ledger snapshot. Source of truth for revenue − providerCost = margin at the platform aggregate."
            icon={<ShieldRounded />}
            accent="mint"
          />
        </Box>
      </Box>

      <CtaBand
        heading="Start with $0 wallet. Top up only when you ship."
        sub="Sign up free, stand up a workspace, feel the gates block before they reach production. Upgrade when an execution actually earns the spend."
        primary={{ href: "/signup", label: "Create account" }}
        secondary={{ href: "/product", label: "How the engine works" }}
        chips={[
          "No card on Free",
          "Stripe Checkout for top-ups",
          "Cancel anytime",
        ]}
      />
    </Box>
  );
}
