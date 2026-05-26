"use client";

// /wallet/topup — wallet top-up surface.
//
// This route serves two purposes:
//
//   1. Picker (default). When the user arrives without a session_id we
//      render the preset chips + custom amount input + summary + Stripe
//      Checkout CTA. Wired to walletCreateTopUp.
//   2. Stripe return handler. Stripe Checkout success_url points back
//      here as /wallet/topup?session_id=cs_…. On arrival we poll the
//      wallet every 2s for up to 30s, watch availableUSD, and render the
//      confirmation pane when a credit lands.
//
// Both modes render inside the cockpit frame (CockpitFrame is the
// layout shell, which renders the cockpit Nav above the page).

import {
  ArrowBackRounded,
  CheckCircleRounded,
  HourglassEmptyRounded,
  OpenInNewRounded,
} from "@mui/icons-material";
import {
  Alert,
  Box,
  Button,
  Card,
  CircularProgress,
  Divider,
  Stack,
  TextField,
  Typography,
} from "@mui/material";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useMemo, useRef, useState } from "react";
import {
  LoadingPanel,
  PageHeader,
} from "../../../src/components/cockpit";
import { TopUpHistory } from "../../../src/components/wallet/TopUpHistory";
import { RequireAuth } from "../../../src/lib/auth";
import { formatMoney } from "../../../src/lib/format";
import {
  useWalletCreateTopUpMutation,
  useWalletQuery,
} from "../../../src/lib/gql/__generated__";
import { useWalletBalance } from "../../../src/lib/hooks";
import { tokens } from "../../../src/theme";

const PRESETS = [20, 50, 100, 250, 500] as const;
const DEFAULT_PRESET: (typeof PRESETS)[number] = 50;
const MIN_CUSTOM = 5;
const POLL_INTERVAL_MS = 2_000;
const POLL_TIMEOUT_MS = 30_000;
const SUCCESS_REDIRECT_MS = 3_000;

export default function WalletTopUpPage() {
  return (
    <RequireAuth>
      <TopUpRouter />
    </RequireAuth>
  );
}

function TopUpRouter() {
  const params = useSearchParams();
  const sessionId = params?.get("session_id") || null;

  return sessionId ? (
    <StripeReturnView sessionId={sessionId} />
  ) : (
    <PickerView />
  );
}

// -----------------------------------------------------------------------------
// Picker mode
// -----------------------------------------------------------------------------

function PickerView() {
  const wallet = useWalletBalance();
  const [selected, setSelected] = useState<number>(DEFAULT_PRESET);
  const [custom, setCustom] = useState<string>("");
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [createTopUp, { loading: submitting }] =
    useWalletCreateTopUpMutation();

  const useCustom = custom.trim().length > 0;
  const customAmount = Number(custom);
  const customValid =
    useCustom && Number.isFinite(customAmount) && customAmount >= MIN_CUSTOM;
  const amount = useCustom ? customAmount : selected;
  const validAmount = useCustom ? customValid : true;

  const balanceTone: "healthy" | "low" | "zero" =
    wallet.availableUSD <= 0
      ? "zero"
      : wallet.lowBalance
        ? "low"
        : "healthy";

  const onContinue = async () => {
    if (submitting || !validAmount) return;
    setSubmitError(null);
    try {
      const res = await createTopUp({ variables: { amountUSD: amount } });
      const url = res.data?.walletCreateTopUp.url;
      if (!url) {
        setSubmitError("Stripe did not return a checkout URL. Try again.");
        return;
      }
      window.location.href = url;
    } catch (err) {
      setSubmitError(
        err instanceof Error
          ? err.message
          : "Could not open Stripe Checkout. Please try again.",
      );
    }
  };

  return (
    <Box>
      <PageHeader
        eyebrow="Wallet"
        title="Top up wallet"
        description="Add prepaid credits so every paid execution has the headroom it needs to admit and finish."
        breadcrumbs={[{ label: "Wallet", href: "/wallet" }, { label: "Top up" }]}
        actions={
          <Button
            component={Link}
            href="/wallet"
            variant="outlined"
            size="small"
            startIcon={<ArrowBackRounded sx={{ fontSize: 16 }} />}
          >
            Back to wallet
          </Button>
        }
      />

      <Box
        sx={{
          display: "grid",
          gap: { xs: 2.5, md: 3 },
          gridTemplateColumns: { xs: "1fr", lg: "3fr 2fr" },
        }}
      >
        {/* Left column: balance + preset picker */}
        <Stack spacing={{ xs: 2.5, md: 3 }}>
          <BalanceCard
            availableUSD={wallet.availableUSD}
            tone={balanceTone}
            loading={
              wallet.loading &&
              wallet.totalUSD === 0 &&
              wallet.availableUSD === 0
            }
          />

          <Card sx={{ p: { xs: 2.5, md: 3 } }}>
            <Typography
              variant="overline"
              sx={{
                color: tokens.color.accent.violet,
                letterSpacing: 1.2,
                fontWeight: 700,
              }}
            >
              Pick an amount
            </Typography>
            <Typography
              sx={{
                mt: 0.5,
                fontSize: 18,
                fontWeight: 700,
                color: tokens.color.text.primary,
              }}
            >
              Presets
            </Typography>

            <Stack
              direction="row"
              spacing={1}
              sx={{ mt: 2, flexWrap: "wrap", gap: 1 }}
            >
              {PRESETS.map((value) => {
                const active = !useCustom && value === selected;
                return (
                  <Button
                    key={value}
                    onClick={() => {
                      setSelected(value);
                      setCustom("");
                      setSubmitError(null);
                    }}
                    size="small"
                    variant={active ? "contained" : "outlined"}
                    color={active ? "primary" : "inherit"}
                    sx={{
                      minWidth: 80,
                      fontFamily: tokens.font.mono,
                      fontWeight: 700,
                      ...(active
                        ? {}
                        : {
                            bgcolor: tokens.color.bg.surfaceRaised,
                            color: tokens.color.text.primary,
                            borderColor: tokens.color.border.subtle,
                            "&:hover": {
                              bgcolor: tokens.color.bg.surfaceHover,
                              borderColor: tokens.color.border.strong,
                            },
                          }),
                    }}
                  >
                    ${value}
                  </Button>
                );
              })}
            </Stack>

            <Divider sx={{ my: 2.5 }} />

            <Typography
              variant="overline"
              sx={{
                color: tokens.color.text.muted,
                letterSpacing: 1.2,
                fontWeight: 700,
              }}
            >
              Custom amount (min ${MIN_CUSTOM})
            </Typography>
            <TextField
              value={custom}
              onChange={(e) => {
                const next = e.target.value.replace(/[^0-9.]/g, "");
                setCustom(next);
                setSubmitError(null);
              }}
              placeholder="e.g. 75"
              inputMode="decimal"
              size="small"
              sx={{
                mt: 1,
                maxWidth: 240,
                "& .MuiOutlinedInput-root": {
                  bgcolor: tokens.color.bg.surfaceRaised,
                  fontFamily: tokens.font.mono,
                },
              }}
              slotProps={{
                input: {
                  startAdornment: (
                    <Typography
                      sx={{
                        color: tokens.color.text.muted,
                        mr: 0.5,
                        fontFamily: tokens.font.mono,
                      }}
                    >
                      $
                    </Typography>
                  ),
                },
              }}
            />
            {useCustom && !customValid && (
              <Typography
                sx={{
                  mt: 1,
                  fontSize: 12,
                  color: tokens.color.accent.warning,
                }}
              >
                Minimum top-up is ${MIN_CUSTOM}.
              </Typography>
            )}
          </Card>

          <Box sx={{ display: { xs: "block", lg: "none" } }}>
            <TopUpHistory />
          </Box>
        </Stack>

        {/* Right column: order summary + CTA */}
        <Stack spacing={{ xs: 2.5, md: 3 }}>
          <Card sx={{ p: { xs: 2.5, md: 3 } }}>
            <Typography
              variant="overline"
              sx={{
                color: tokens.color.accent.violet,
                letterSpacing: 1.2,
                fontWeight: 700,
              }}
            >
              Order summary
            </Typography>
            <Stack spacing={1.5} sx={{ mt: 1.5 }}>
              <SummaryRow
                label="Top-up amount"
                value={formatMoney(validAmount ? amount : 0)}
              />
              <SummaryRow
                label="Processing fee"
                value="Included"
                muted
              />
              <Divider />
              <SummaryRow
                label="Total"
                value={formatMoney(validAmount ? amount : 0)}
                bold
              />
            </Stack>

            <Typography
              sx={{
                mt: 2,
                fontSize: 12,
                color: tokens.color.text.muted,
              }}
            >
              You will be redirected to Stripe Checkout to complete payment.
              Credits land in your wallet as soon as the webhook fires.
            </Typography>

            {submitError && (
              <Alert
                severity="error"
                variant="outlined"
                sx={{
                  mt: 2,
                  border: `1px solid ${tokens.color.accent.danger}55`,
                  color: tokens.color.text.primary,
                  bgcolor: `${tokens.color.accent.danger}10`,
                }}
              >
                {submitError}
              </Alert>
            )}

            <Button
              onClick={onContinue}
              disabled={submitting || !validAmount}
              variant="contained"
              color="primary"
              size="large"
              fullWidth
              sx={{ mt: 2 }}
              startIcon={
                submitting ? (
                  <CircularProgress
                    size={14}
                    thickness={5}
                    sx={{ color: "inherit" }}
                  />
                ) : (
                  <OpenInNewRounded sx={{ fontSize: 16 }} />
                )
              }
            >
              Continue to Stripe
            </Button>
          </Card>

          <Box sx={{ display: { xs: "none", lg: "block" } }}>
            <TopUpHistory />
          </Box>
        </Stack>
      </Box>
    </Box>
  );
}

function BalanceCard({
  availableUSD,
  tone,
  loading,
}: {
  availableUSD: number;
  tone: "healthy" | "low" | "zero";
  loading: boolean;
}) {
  const toneColor =
    tone === "zero"
      ? tokens.color.accent.danger
      : tone === "low"
        ? tokens.color.accent.warning
        : tokens.color.brand.mint;
  const toneLabel =
    tone === "zero"
      ? "no credits"
      : tone === "low"
        ? "running low"
        : "healthy";

  return (
    <Card sx={{ p: { xs: 2.5, md: 3 } }}>
      <Typography
        variant="overline"
        sx={{
          color: tokens.color.text.muted,
          letterSpacing: 1.2,
          fontWeight: 700,
        }}
      >
        Current balance
      </Typography>
      {loading ? (
        <LoadingPanel minHeight={100} label="Loading wallet" />
      ) : (
        <>
          <Typography
            sx={{
              mt: 1,
              fontFamily: tokens.font.mono,
              fontWeight: 800,
              fontSize: { xs: 40, md: 52 },
              color: tokens.color.text.primary,
              letterSpacing: -1.2,
              lineHeight: 1,
            }}
          >
            {formatMoney(availableUSD)}
          </Typography>
          <Stack
            direction="row"
            alignItems="center"
            spacing={1}
            sx={{ mt: 1.5 }}
          >
            <Box
              sx={{
                width: 8,
                height: 8,
                borderRadius: tokens.radius.pill,
                bgcolor: toneColor,
              }}
            />
            <Typography
              sx={{
                fontFamily: tokens.font.mono,
                fontSize: 12,
                color: toneColor,
                letterSpacing: 0.5,
                textTransform: "uppercase",
                fontWeight: 700,
              }}
            >
              {toneLabel}
            </Typography>
            <Typography
              sx={{ fontSize: 12, color: tokens.color.text.muted }}
            >
              available for paid executions
            </Typography>
          </Stack>
        </>
      )}
    </Card>
  );
}

function SummaryRow({
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
    <Stack direction="row" justifyContent="space-between" alignItems="baseline">
      <Typography
        sx={{
          fontSize: 13,
          color: muted ? tokens.color.text.muted : tokens.color.text.secondary,
          fontWeight: bold ? 700 : 500,
        }}
      >
        {label}
      </Typography>
      <Typography
        sx={{
          fontFamily: tokens.font.mono,
          fontSize: bold ? 16 : 13.5,
          fontWeight: bold ? 800 : 600,
          color: muted ? tokens.color.text.muted : tokens.color.text.primary,
        }}
      >
        {value}
      </Typography>
    </Stack>
  );
}

// -----------------------------------------------------------------------------
// Stripe return mode
// -----------------------------------------------------------------------------

function StripeReturnView({ sessionId }: { sessionId: string }) {
  const router = useRouter();

  const walletQuery = useWalletQuery({
    fetchPolicy: "network-only",
    pollInterval: POLL_INTERVAL_MS,
  });

  const baselineRef = useRef<number | null>(null);
  const startedAtRef = useRef<number>(Date.now());
  const [creditedUSD, setCreditedUSD] = useState<number | null>(null);
  const [timedOut, setTimedOut] = useState(false);

  const wallet = walletQuery.data?.wallet;

  // Baseline once we have a wallet payload.
  useEffect(() => {
    if (wallet && baselineRef.current === null) {
      baselineRef.current = wallet.availableUSD;
    }
  }, [wallet]);

  // Watch for the credit.
  useEffect(() => {
    if (!wallet || baselineRef.current === null) return;
    const delta = wallet.availableUSD - baselineRef.current;
    if (delta > 0.001 && creditedUSD === null) {
      setCreditedUSD(delta);
      walletQuery.stopPolling();
    }
  }, [wallet, creditedUSD, walletQuery]);

  // Bounce home after we observe a credit.
  useEffect(() => {
    if (creditedUSD === null) return;
    const t = setTimeout(() => router.push("/"), SUCCESS_REDIRECT_MS);
    return () => clearTimeout(t);
  }, [creditedUSD, router]);

  // Timeout — webhook will reconcile asynchronously.
  useEffect(() => {
    if (creditedUSD !== null) return;
    const remaining = Math.max(
      0,
      POLL_TIMEOUT_MS - (Date.now() - startedAtRef.current),
    );
    const t = setTimeout(() => {
      setTimedOut(true);
      walletQuery.stopPolling();
    }, remaining);
    return () => clearTimeout(t);
  }, [creditedUSD, walletQuery]);

  const breadcrumbs = useMemo(
    () => [{ label: "Wallet", href: "/wallet" }, { label: "Top up" }],
    [],
  );

  return (
    <Box>
      <PageHeader
        eyebrow="Wallet"
        title="Top-up confirmation"
        description={`Stripe session ${sessionId.slice(0, 14)}…`}
        breadcrumbs={breadcrumbs}
      />

      {creditedUSD !== null ? (
        <SuccessPanel amountUSD={creditedUSD} />
      ) : timedOut ? (
        <TimeoutPanel />
      ) : (
        <LoadingPanel label="Confirming your top-up..." />
      )}
    </Box>
  );
}

function SuccessPanel({ amountUSD }: { amountUSD: number }) {
  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.surface,
        border: `1px solid ${tokens.color.accent.success}55`,
        borderRadius: 1,
        p: { xs: 3, md: 4 },
      }}
    >
      <Stack alignItems="center" spacing={2}>
        <CheckCircleRounded
          sx={{ fontSize: 56, color: tokens.color.accent.success }}
        />
        <Typography
          sx={{
            fontSize: { xs: 22, md: 28 },
            fontWeight: 800,
            color: tokens.color.text.primary,
            letterSpacing: -0.4,
          }}
        >
          + {formatMoney(amountUSD)} added to your wallet
        </Typography>
        <Typography sx={{ fontSize: 13, color: tokens.color.text.secondary }}>
          Redirecting you back to the home composer…
        </Typography>
        <Stack direction="row" spacing={1}>
          <Button component={Link} href="/" variant="contained" color="primary">
            Start a build
          </Button>
          <Button
            component={Link}
            href="/wallet"
            variant="outlined"
            sx={{
              color: tokens.color.text.primary,
              borderColor: tokens.color.border.strong,
              "&:hover": {
                borderColor: tokens.color.accent.violet,
                bgcolor: "transparent",
              },
            }}
          >
            Back to wallet
          </Button>
        </Stack>
      </Stack>
    </Box>
  );
}

function TimeoutPanel() {
  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.surface,
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 1,
        p: { xs: 3, md: 4 },
      }}
    >
      <Stack alignItems="center" spacing={2}>
        <HourglassEmptyRounded
          sx={{ fontSize: 48, color: tokens.color.accent.warning }}
        />
        <Typography
          sx={{
            fontSize: { xs: 20, md: 24 },
            fontWeight: 800,
            color: tokens.color.text.primary,
            letterSpacing: -0.3,
            textAlign: "center",
          }}
        >
          Top-up is being processed
        </Typography>
        <Typography
          sx={{
            fontSize: 13.5,
            color: tokens.color.text.secondary,
            maxWidth: 460,
            textAlign: "center",
          }}
        >
          Your balance will update shortly. Stripe and the orchestrator webhook
          finalize the credit asynchronously.
        </Typography>
        <Button
          component={Link}
          href="/wallet"
          variant="contained"
          color="primary"
        >
          Back to wallet
        </Button>
      </Stack>
    </Box>
  );
}
