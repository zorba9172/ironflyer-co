"use client";

// PricingPage — public marketing surface.
//
// The cockpit Nav is mounted globally in app/layout.tsx via CockpitFrame;
// for unauthenticated visitors it already renders Sign-in / Get-started
// CTAs, so this page does not inline its own marketing header.
//
// GraphQL operations consumed:
//   - query PricingPlans            (inline gql — `plans`)
//   - query PricingRates            (inline gql — `rates`)
//   - mutation PricingStartCheckout (inline gql — `startCheckout`)

import { gql, useMutation, useQuery } from "@apollo/client";
import { CheckRounded } from "@mui/icons-material";
import {
  Box,
  Button,
  Card,
  CircularProgress,
  Skeleton,
  Stack,
  Typography,
} from "@mui/material";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useMemo, useState } from "react";
import { useAuth } from "../lib/auth";
import { extractErrorMessage } from "../lib/errors";
import { formatMoney } from "../lib/format";
import { tokens } from "../theme";
import { ErrorPanel, PageHeader } from "./cockpit";

interface PricingPlansData {
  plans: Array<{
    tier: string;
    name: string;
    priceUsd: string;
    costCapUsd: string;
    description: string | null;
    features: string[];
    stripePriceId: string | null;
  }>;
}
const PRICING_PLANS = gql`
  query PricingPlans {
    plans {
      tier
      name
      priceUsd
      costCapUsd
      description
      features
      stripePriceId
    }
  }
`;

interface PricingRatesData {
  rates: Array<{
    provider: string;
    model: string;
    promptPerMTok: string;
    completionPerMTok: string;
  }>;
}
const PRICING_RATES = gql`
  query PricingRates {
    rates {
      provider
      model
      promptPerMTok
      completionPerMTok
    }
  }
`;

interface PricingStartCheckoutData {
  startCheckout: { sessionId: string; url: string };
}
interface PricingStartCheckoutVars {
  input: { tier: string; successUrl?: string | null; cancelUrl?: string | null };
}
const PRICING_START_CHECKOUT = gql`
  mutation PricingStartCheckout($input: StartCheckoutInput!) {
    startCheckout(input: $input) {
      sessionId
      url
    }
  }
`;

const FAQ: Array<{ q: string; a: string }> = [
  {
    q: "Can I start for free?",
    a: "Yes. Start with a workspace, generate a preview, and upgrade when you need more projects, environments and team controls.",
  },
  {
    q: "Can I export the code?",
    a: "Yes. IronFlyer is built around code you own, with React and TypeScript output that can be extended by your team.",
  },
  {
    q: "Which plan is best for teams?",
    a: "Team is the best default for shared work: roles, environments, review flow and priority support.",
  },
  {
    q: "Is my data secure?",
    a: "Workspaces are isolated and deploy paths include secrets checks, audit trails and role-based access.",
  },
  {
    q: "Can I cancel anytime?",
    a: "Yes. You can stay on free, change plans, or talk to sales when enterprise controls are required.",
  },
  {
    q: "Do you offer onboarding?",
    a: "Yes. Team and Enterprise customers can get architecture review, setup guidance and launch support.",
  },
];

const FALLBACK_PLANS: PricingPlansData["plans"] = [
  {
    tier: "free",
    name: "Free",
    priceUsd: "0",
    costCapUsd: "5",
    description: "Start a workspace and build the first product preview.",
    features: ["1 workspace", "2 projects", "Community support"],
    stripePriceId: null,
  },
  {
    tier: "pro",
    name: "Pro",
    priceUsd: "29",
    costCapUsd: "50",
    description: "For founders shipping production-ready product flows.",
    features: ["Unlimited projects", "AI templates", "Email support"],
    stripePriceId: null,
  },
  {
    tier: "team",
    name: "Team",
    priceUsd: "79",
    costCapUsd: "250",
    description: "For teams that need review, roles and launch controls.",
    features: ["SSO and RBAC", "Environments", "Priority support"],
    stripePriceId: null,
  },
  {
    tier: "enterprise",
    name: "Enterprise",
    priceUsd: "0",
    costCapUsd: "1000",
    description: "Custom security, governance and deployment support.",
    features: ["Advanced security", "SLA and support", "Custom integrations"],
    stripePriceId: null,
  },
];

const FALLBACK_RATES: PricingRatesData["rates"] = [
  { provider: "OpenAI", model: "GPT production tier", promptPerMTok: "2.50", completionPerMTok: "10.00" },
  { provider: "Anthropic", model: "Claude production tier", promptPerMTok: "3.00", completionPerMTok: "15.00" },
  { provider: "Sandbox", model: "Build runtime", promptPerMTok: "0.80", completionPerMTok: "0.80" },
];

const skelSx = {
  bgcolor: tokens.color.bg.surfaceHover,
  borderRadius: 1,
};

export function PricingPage() {
  const router = useRouter();
  const { authenticated } = useAuth();
  const plansQ = useQuery<PricingPlansData>(PRICING_PLANS, {
    skip: !authenticated,
    errorPolicy: "all",
  });
  const ratesQ = useQuery<PricingRatesData>(PRICING_RATES, {
    skip: !authenticated,
    errorPolicy: "all",
  });
  const [startCheckout, startCheckoutM] = useMutation<
    PricingStartCheckoutData,
    PricingStartCheckoutVars
  >(PRICING_START_CHECKOUT);

  const [error, setError] = useState<string | null>(null);
  const [pendingTier, setPendingTier] = useState<string | null>(null);

  const handleCheckout = useCallback(
    async (tier: string) => {
      setError(null);
      setPendingTier(tier);
      if (!authenticated) {
        router.push(`/signup?redirect=${encodeURIComponent("/studio")}`);
        return;
      }
      try {
        const origin =
          typeof window !== "undefined" ? window.location.origin : "";
        const res = await startCheckout({
          variables: {
            input: {
              tier,
              successUrl: `${origin}/dashboard?welcome=1`,
              cancelUrl: `${origin}/pricing`,
            },
          },
        });
        const url = res.data?.startCheckout.url;
        if (!url) throw new Error("Stripe Checkout did not return a URL.");
        window.location.href = url;
      } catch (err) {
        setError(extractErrorMessage(err));
        setPendingTier(null);
      }
    },
    [authenticated, router, startCheckout],
  );

  const plans =
    plansQ.data?.plans && plansQ.data.plans.length > 0
      ? plansQ.data.plans
      : FALLBACK_PLANS;
  const highlightTier = useMemo(() => {
    if (plans.length < 2) return null;
    const sorted = [...plans].sort(
      (a, b) => Number(a.priceUsd) - Number(b.priceUsd),
    );
    return sorted[Math.floor(sorted.length / 2)]?.tier ?? null;
  }, [plans]);

  const sortedRates = useMemo(() => {
    const rows =
      ratesQ.data?.rates && ratesQ.data.rates.length > 0
        ? ratesQ.data.rates
        : FALLBACK_RATES;
    return [...rows].sort((a, b) => {
      if (a.provider !== b.provider) return a.provider.localeCompare(b.provider);
      return a.model.localeCompare(b.model);
    });
  }, [ratesQ.data]);

  return (
    <Box>
      <PageHeader
        title="Simple, transparent pricing"
        eyebrow="pricing"
        description="Start free, build in Studio, then scale when your team needs more launch lanes, environments, support and security."
      />

      <Stack spacing={6} sx={{ pb: 6 }}>
        <Card
          sx={{
            p: { xs: 2.5, md: 4 },
            overflow: "hidden",
            background: `radial-gradient(circle at 84% 10%, ${tokens.color.accent.violet}47, transparent 28%), linear-gradient(180deg, ${tokens.color.bg.surfaceRaised}eb, ${tokens.color.bg.inset}f5)`,
            borderColor: `${tokens.color.accent.violet}4d`,
          }}
        >
          <Box
            sx={{
              display: "grid",
              gap: 2,
              gridTemplateColumns: { xs: "1fr", md: "1.2fr .8fr" },
              alignItems: "center",
              minWidth: 0,
            }}
          >
            <Box sx={{ minWidth: 0 }}>
              <Typography sx={{ color: tokens.color.accent.violet, fontSize: 12, fontWeight: 900, textTransform: "uppercase" }}>
                Build now. Upgrade when the workflow earns it.
              </Typography>
              <Typography sx={{ mt: 1, fontSize: { xs: 26, md: 34 }, fontWeight: 900, lineHeight: 1.08 }}>
                One workspace, one live preview, one clear path to production.
              </Typography>
              <Typography sx={{ mt: 1.2, color: tokens.color.text.secondary, maxWidth: 660 }}>
                Plans map to the product flow from the reference: prompt, plan, code, review, deploy.
              </Typography>
            </Box>
            <Box
              sx={{
                borderRadius: 1,
                border: `1px solid ${tokens.color.border.subtle}`,
                bgcolor: `${tokens.color.bg.base}9e`,
                p: 1.5,
              }}
            >
              {["Plan locked", "Web live", "Mobile queued", "Gate 92/100"].map((label, index) => (
                <Stack
                  key={label}
                  direction="row"
                  alignItems="center"
                  spacing={1}
                  sx={{
                    borderBottom: index === 3 ? 0 : `1px solid ${tokens.color.border.subtle}`,
                    py: 1,
                  }}
                >
                  <Box
                    sx={{
                      width: 9,
                      height: 9,
                      borderRadius: "50%",
                      bgcolor: index === 3 ? tokens.color.accent.coral : tokens.color.accent.violet,
                    }}
                  />
                  <Typography sx={{ fontWeight: 800 }}>{label}</Typography>
                </Stack>
              ))}
            </Box>
          </Box>
        </Card>

        {/* Plans */}
        <Box>
          {error && (
            <Box sx={{ mb: 2 }}>
              <ErrorPanel error={error} title="Could not start checkout" />
            </Box>
          )}
          {plansQ.loading && authenticated && !plansQ.data ? (
            <Box
              sx={{
                display: "grid",
                gap: 2.5,
                gridTemplateColumns: { xs: "1fr", md: "repeat(3, 1fr)" },
              }}
            >
              {Array.from({ length: 3 }).map((_, i) => (
                <Skeleton key={i} variant="rectangular" height={360} sx={skelSx} />
              ))}
            </Box>
          ) : (
            <Box
              sx={{
                display: "grid",
                gap: 2.5,
                gridTemplateColumns: {
                  xs: "1fr",
                  md:
                    plans.length >= 3
                      ? "repeat(3, 1fr)"
                      : `repeat(${plans.length}, 1fr)`,
                },
              }}
            >
              {plans.map((p) => (
                <PlanCard
                  key={p.tier}
                  plan={p}
                  highlighted={p.tier === highlightTier}
                  busy={pendingTier === p.tier && startCheckoutM.loading}
                  disabled={startCheckoutM.loading && pendingTier !== p.tier}
                  onCheckout={() => handleCheckout(p.tier)}
                  authenticated={authenticated}
                />
              ))}
            </Box>
          )}
        </Box>

        {/* Rates */}
        <Box>
          <Typography
            sx={{
              color: tokens.color.text.primary,
              fontSize: 26,
              fontWeight: 800,
              letterSpacing: -0.4,
            }}
          >
            AI rate transparency
          </Typography>
          <Typography
            sx={{
              color: tokens.color.text.secondary,
              fontSize: 14.5,
              maxWidth: 620,
              mt: 1,
            }}
          >
            Provider costs stay visible under the product layer, so teams can
            scale with confidence after they choose a plan.
          </Typography>
          <Box sx={{ mt: 3 }}>
            {ratesQ.loading && authenticated && !ratesQ.data ? (
              <Stack spacing={1}>
                {Array.from({ length: 6 }).map((_, i) => (
                  <Skeleton key={i} variant="rectangular" height={44} sx={skelSx} />
                ))}
              </Stack>
            ) : (
              <Card sx={{ overflow: "hidden", p: 0 }}>
                <Box
                  sx={{
                    alignItems: "center",
                    borderBottom: `1px solid ${tokens.color.border.subtle}`,
                    color: tokens.color.text.muted,
                    display: { xs: "none", md: "grid" },
                    fontSize: 11.5,
                    fontWeight: 800,
                    gap: 2,
                    gridTemplateColumns:
                      "minmax(0,1fr) minmax(0,1.5fr) 160px 160px",
                    letterSpacing: 0.5,
                    px: 2.5,
                    py: 1.5,
                    textTransform: "uppercase",
                  }}
                >
                  <Box>Provider</Box>
                  <Box>Model</Box>
                  <Box>Prompt / M tok</Box>
                  <Box>Completion / M tok</Box>
                </Box>
                {sortedRates.map((r) => (
                  <Box
                    key={`${r.provider}/${r.model}`}
                    sx={{
                      alignItems: "center",
                      borderBottom: `1px solid ${tokens.color.border.subtle}`,
                      display: { xs: "block", md: "grid" },
                      gap: 2,
                      gridTemplateColumns:
                        "minmax(0,1fr) minmax(0,1.5fr) 160px 160px",
                      px: 2.5,
                      py: 1.5,
                      "&:last-of-type": { borderBottom: 0 },
                    }}
                  >
                    <PublicCell label="Provider" value={r.provider} bold />
                    <PublicCell label="Model" value={r.model} muted />
                    <PublicCell
                      label="Prompt / M tok"
                      value={formatMoney(r.promptPerMTok)}
                    />
                    <PublicCell
                      label="Completion / M tok"
                      value={formatMoney(r.completionPerMTok)}
                    />
                  </Box>
                ))}
              </Card>
            )}
          </Box>
        </Box>

        {/* FAQ */}
        <Box>
          <Typography
            sx={{
              color: tokens.color.text.primary,
              fontSize: 26,
              fontWeight: 800,
              letterSpacing: -0.4,
            }}
          >
            Frequently asked
          </Typography>
          <Box
            sx={{
              display: "grid",
              gap: 2,
              gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
              mt: 3,
            }}
          >
            {FAQ.map((item) => (
              <Card key={item.q} sx={{ p: 2.5 }}>
                <Typography
                  sx={{
                    color: tokens.color.text.primary,
                    fontSize: 16,
                    fontWeight: 800,
                  }}
                >
                  {item.q}
                </Typography>
                <Typography
                  sx={{
                    color: tokens.color.text.secondary,
                    fontSize: 14,
                    lineHeight: 1.5,
                    mt: 1,
                  }}
                >
                  {item.a}
                </Typography>
              </Card>
            ))}
          </Box>
        </Box>

        {/* Bottom CTA */}
        <Card
          sx={{
            bgcolor: tokens.color.bg.surfaceRaised,
            borderColor: `${tokens.color.accent.violet}47`,
            p: { xs: 3, md: 5 },
            textAlign: "center",
          }}
        >
          <Typography
            sx={{
              color: tokens.color.text.primary,
              fontSize: { xs: 24, md: 32 },
              fontWeight: 900,
              letterSpacing: -0.4,
            }}
          >
            Stop stitching tools. Start shipping products.
          </Typography>
          <Typography
            sx={{
              color: tokens.color.text.secondary,
              fontSize: 15,
              mt: 1,
            }}
          >
            One prompt, one workspace, one launch flow.
          </Typography>
          <Stack
            direction={{ xs: "column", sm: "row" }}
            spacing={2}
            justifyContent="center"
            sx={{ mt: 3 }}
          >
            <Button
              component={Link}
              href="/signup"
              variant="contained"
              color="primary"
              size="large"
            >
              Create an account
            </Button>
            <Button component={Link} href="/" variant="text" size="large">
              Back to overview
            </Button>
          </Stack>
        </Card>
      </Stack>
    </Box>
  );
}

function PlanCard({
  plan,
  highlighted,
  busy,
  disabled,
  onCheckout,
  authenticated,
}: {
  plan: PricingPlansData["plans"][number];
  highlighted: boolean;
  busy: boolean;
  disabled: boolean;
  onCheckout: () => void;
  authenticated: boolean;
}) {
  const price = Number(plan.priceUsd);
  const priceDisplay =
    Number.isFinite(price) && price > 0 ? formatMoney(price) : "Free";

  return (
    <Card
      sx={{
        borderColor: highlighted ? tokens.color.accent.violet : undefined,
        borderWidth: highlighted ? 2 : 1,
        display: "flex",
        flexDirection: "column",
        gap: 2,
        p: { xs: 3, md: 3.5 },
        position: "relative",
      }}
    >
      {highlighted && (
        <Box
          sx={{
            background: `linear-gradient(100deg, ${tokens.color.accent.coral}, ${tokens.color.accent.violet})`,
            borderRadius: 999,
            color: tokens.color.text.primary,
            fontSize: 11,
            fontWeight: 900,
            letterSpacing: 0.6,
            position: "absolute",
            px: 1.5,
            py: 0.5,
            right: 16,
            textTransform: "uppercase",
            top: -12,
          }}
        >
          Most chosen
        </Box>
      )}
      <Box>
        <Typography
          sx={{
            color: tokens.color.text.primary,
            fontSize: 18,
            fontWeight: 800,
          }}
        >
          {plan.name}
        </Typography>
        {plan.description && (
          <Typography
            sx={{
              color: tokens.color.text.secondary,
              fontSize: 13.5,
              mt: 0.5,
              minHeight: 40,
            }}
          >
            {plan.description}
          </Typography>
        )}
      </Box>
      <Box>
        <Typography
          sx={{
            color: tokens.color.text.primary,
            fontFamily: tokens.font.mono,
            fontSize: 44,
            fontWeight: 800,
            letterSpacing: -1,
            lineHeight: 1,
          }}
        >
          {priceDisplay}
        </Typography>
        <Typography
          sx={{
            color: tokens.color.text.muted,
            fontSize: 12.5,
            mt: 0.5,
          }}
        >
          monthly cap · cost cap {formatMoney(plan.costCapUsd)}
        </Typography>
      </Box>
      <Stack spacing={1} sx={{ flex: 1 }}>
        {plan.features.map((f) => (
          <Stack key={f} direction="row" alignItems="flex-start" spacing={1}>
            <CheckRounded
              sx={{
                color: highlighted
                  ? tokens.color.accent.violet
                  : tokens.color.text.secondary,
                fontSize: 18,
                mt: 0.25,
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
        variant="contained"
        color={highlighted ? "primary" : "secondary"}
        size="large"
        onClick={onCheckout}
        disabled={busy || disabled}
        startIcon={
          busy ? (
            <CircularProgress
              size={14}
              sx={{
                color: highlighted
                  ? tokens.color.text.inverse
                  : tokens.color.text.primary,
              }}
            />
          ) : undefined
        }
        sx={{
          mt: 1,
          ...(highlighted
            ? {}
            : {
                bgcolor: tokens.color.bg.surfaceRaised,
                color: tokens.color.text.primary,
                border: `1px solid ${tokens.color.border.strong}`,
                "&:hover": {
                  bgcolor: tokens.color.bg.surfaceHover,
                  borderColor: tokens.color.accent.violet,
                },
              }),
        }}
      >
        {busy ? "Opening Stripe…" : authenticated ? "Start checkout" : "Start building"}
      </Button>
    </Card>
  );
}

function PublicCell({
  label,
  value,
  bold,
  muted,
}: {
  label: string;
  value: string;
  bold?: boolean;
  muted?: boolean;
}) {
  return (
    <Box sx={{ display: { xs: "flex", md: "block" }, gap: 1.5 }}>
      <Typography
        sx={{
          color: tokens.color.text.muted,
          display: { xs: "block", md: "none" },
          fontSize: 11,
          fontWeight: 700,
          letterSpacing: 0.5,
          minWidth: 130,
          textTransform: "uppercase",
        }}
      >
        {label}
      </Typography>
      <Typography
        sx={{
          color: muted ? tokens.color.text.secondary : tokens.color.text.primary,
          fontFamily: bold ? tokens.font.mono : undefined,
          fontSize: 14,
          fontWeight: bold ? 800 : 600,
        }}
      >
        {value}
      </Typography>
    </Box>
  );
}

export default PricingPage;
