"use client";

// WalletPage — prepaid balance, Stripe top-up tiles, top-up history,
// 7-day spend chart, and per-provider/model spend breakdown.
//
// GraphQL operations consumed:
//   - query Wallet               (generated)
//   - query WalletTopUps         (generated)
//   - mutation WalletCreateTopUp (generated)
//   - query WalletBudget         (inline gql — `myBudget`)

import { gql, useQuery } from "@apollo/client";
import {
  AccountBalanceWalletOutlined,
  LockOutlined,
  RefreshRounded,
} from "@mui/icons-material";
import {
  Alert,
  Box,
  Card,
  CircularProgress,
  IconButton,
  Skeleton,
  Snackbar,
  Stack,
  Tooltip,
  Typography,
} from "@mui/material";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useAuth } from "../lib/auth";
import { extractErrorMessage } from "../lib/errors";
import { formatDateTime, formatMoney, formatNumber } from "../lib/format";
import {
  useWalletCreateTopUpMutation,
  useWalletQuery,
  useWalletTopUpsQuery,
} from "../lib/gql/__generated__";
import { relativeTime } from "../lib/relativeTime";
import { tokens } from "../theme";
import {
  EmptyState,
  ErrorPanel,
  LoadingPanel,
  MetricCard,
  PageHeader,
  StatusBadge,
} from "./cockpit";
import dynamic from "next/dynamic";
import { SparklineSVG, type SparklinePoint } from "./SparklineSVG";

// Lazy-load echarts: the chart chunk loads only when the wallet
// page mounts, keeping the cockpit shell shipping a tiny initial
// bundle.
const SpendBars = dynamic(
  () => import("./charts/SpendBars").then((m) => m.SpendBars),
  { ssr: false, loading: () => <Box sx={{ height: 180 }} /> },
);

interface WalletBudgetData {
  myBudget: {
    spentUsd: string;
    entries: Array<{
      id: string;
      provider: string | null;
      model: string | null;
      promptTokens: number;
      completionTokens: number;
      costUsd: string;
      ts: string;
    }>;
  };
}
const WALLET_BUDGET = gql`
  query WalletBudget {
    myBudget {
      spentUsd
      entries {
        id
        provider
        model
        promptTokens
        completionTokens
        costUsd
        ts
      }
    }
  }
`;

const TOPUP_TIERS = [10, 25, 50, 100, 250, 500] as const;

const skelSx = {
  bgcolor: tokens.color.bg.surfaceHover,
  borderRadius: 1,
};

export function WalletPage() {
  const router = useRouter();
  const { authenticated, loading: authLoading } = useAuth();

  useEffect(() => {
    if (authLoading) return;
    if (!authenticated) {
      router.replace("/login?returnTo=" + encodeURIComponent("/wallet"));
    }
  }, [authenticated, authLoading, router]);

  const skip = !authenticated;
  const walletQ = useWalletQuery({ skip });
  const topUpsQ = useWalletTopUpsQuery({ skip, variables: { limit: 50 } });
  const budgetQ = useQuery<WalletBudgetData>(WALLET_BUDGET, { skip });
  const [createTopUp, createTopUpM] = useWalletCreateTopUpMutation();

  const [topUpError, setTopUpError] = useState<string | null>(null);
  const [snack, setSnack] = useState<string | null>(null);
  const [pendingAmount, setPendingAmount] = useState<number | null>(null);

  const handleTopUp = useCallback(
    async (amount: number) => {
      setTopUpError(null);
      setPendingAmount(amount);
      try {
        const res = await createTopUp({ variables: { amountUSD: amount } });
        const url = res.data?.walletCreateTopUp.url;
        if (!url) throw new Error("Stripe Checkout did not return a URL.");
        window.location.href = url;
      } catch (err) {
        setPendingAmount(null);
        setTopUpError(extractErrorMessage(err));
      }
    },
    [createTopUp],
  );

  // 7-day daily spend buckets
  const dailySpend = useMemo<SparklinePoint[]>(() => {
    const entries = budgetQ.data?.myBudget.entries ?? [];
    const now = Date.now();
    const day = 24 * 60 * 60 * 1000;
    const buckets: SparklinePoint[] = [];
    for (let i = 6; i >= 0; i--) {
      const start = now - i * day - day;
      buckets.push({ ts: new Date(start).toISOString(), value: 0 });
    }
    for (const e of entries) {
      const t = new Date(e.ts).getTime();
      if (Number.isNaN(t)) continue;
      const diff = Math.floor((now - t) / day);
      if (diff < 0 || diff > 6) continue;
      const idx = 6 - diff;
      buckets[idx].value += Number(e.costUsd) || 0;
    }
    return buckets;
  }, [budgetQ.data]);

  const total7d = useMemo(
    () => dailySpend.reduce((s, p) => s + p.value, 0),
    [dailySpend],
  );

  type BreakdownRow = {
    key: string;
    provider: string;
    model: string;
    costUsd: number;
    tokens: number;
    count: number;
  };
  const breakdown = useMemo<BreakdownRow[]>(() => {
    const entries = budgetQ.data?.myBudget.entries ?? [];
    const map = new Map<string, BreakdownRow>();
    for (const e of entries) {
      const provider = e.provider ?? "unknown";
      const model = e.model ?? "unknown";
      const key = `${provider}::${model}`;
      const row =
        map.get(key) ??
        ({
          key,
          provider,
          model,
          costUsd: 0,
          tokens: 0,
          count: 0,
        } as BreakdownRow);
      row.costUsd += Number(e.costUsd) || 0;
      row.tokens += (e.promptTokens || 0) + (e.completionTokens || 0);
      row.count += 1;
      map.set(key, row);
    }
    return Array.from(map.values()).sort((a, b) => b.costUsd - a.costUsd);
  }, [budgetQ.data]);

  if (authLoading || !authenticated) {
    return (
      <>
        <PageHeader title="Wallet" />
        <LoadingPanel label="Loading wallet" />
      </>
    );
  }

  const wallet = walletQ.data?.wallet;
  const topUps = topUpsQ.data?.walletTopUps ?? [];

  return (
    <Box>
      <PageHeader
        title="Wallet"
        eyebrow="prepaid · ledger-backed"
        description="Top up once, run anything. Every cent of provider, sandbox, storage, and deploy cost lands in your ledger and is visible here."
      />

      <Stack spacing={4} sx={{ pb: 6 }}>
        {/* Headline balance */}
        <Card sx={{ position: "relative", overflow: "hidden" }}>
          <Box
            sx={{
              background: `linear-gradient(135deg, ${tokens.color.accent.violet}1a 0%, transparent 60%)`,
              p: { xs: 3, md: 4 },
            }}
          >
            <Typography variant="overline" sx={{ color: tokens.color.text.secondary }}>
              Wallet available
            </Typography>
            {walletQ.loading ? (
              <Skeleton variant="text" width={280} height={84} sx={skelSx} />
            ) : (
              <Typography
                sx={{
                  color: tokens.color.accent.violet,
                  fontFamily: tokens.font.mono,
                  fontSize: { xs: 48, md: 64 },
                  fontWeight: 800,
                  letterSpacing: -1,
                  lineHeight: 1.05,
                  mt: 0.5,
                }}
              >
                {formatMoney(wallet?.availableUSD)}
              </Typography>
            )}
            <Typography
              sx={{ color: tokens.color.text.muted, fontSize: 12, mt: 1 }}
            >
              updated {relativeTime(wallet?.updatedAt)}
            </Typography>

            <Stack
              direction={{ xs: "column", sm: "row" }}
              spacing={3}
              sx={{ mt: 3, flexWrap: "wrap" }}
            >
              <MiniStat
                label="Balance"
                value={formatMoney(wallet?.balanceUSD)}
                loading={walletQ.loading}
              />
              <MiniStat
                label="On hold"
                value={formatMoney(wallet?.holdUSD)}
                loading={walletQ.loading}
              />
              <MiniStat
                label="Lifetime top-up"
                value={formatMoney(wallet?.lifetimeTopUpUSD)}
                loading={walletQ.loading}
              />
              <MiniStat
                label="Lifetime spend"
                value={formatMoney(wallet?.lifetimeSpendUSD)}
                loading={walletQ.loading}
              />
            </Stack>
          </Box>
        </Card>

        {/* Top-up tiles */}
        <Card sx={{ p: { xs: 2.5, md: 3 } }}>
          <Stack
            direction="row"
            alignItems="baseline"
            spacing={1.5}
            sx={{ mb: 2 }}
          >
            <Typography
              sx={{
                color: tokens.color.text.primary,
                fontSize: 20,
                fontWeight: 800,
              }}
            >
              Add credit
            </Typography>
            <Typography sx={{ color: tokens.color.text.muted, fontSize: 13.5 }}>
              one-tap top-up · no auto-renew
            </Typography>
          </Stack>

          <Box
            sx={{
              display: "grid",
              gap: 1.25,
              gridTemplateColumns: {
                xs: "repeat(2, 1fr)",
                sm: "repeat(3, 1fr)",
                md: "repeat(6, 1fr)",
              },
            }}
          >
            {TOPUP_TIERS.map((amount) => {
              const isPending = pendingAmount === amount && createTopUpM.loading;
              const disabled = createTopUpM.loading && !isPending;
              return (
                <Box
                  key={amount}
                  component="button"
                  onClick={() => handleTopUp(amount)}
                  disabled={disabled || isPending}
                  sx={{
                    alignItems: "center",
                    bgcolor: tokens.color.bg.surfaceRaised,
                    border: `1px solid ${tokens.color.border.subtle}`,
                    borderRadius: 1,
                    color: tokens.color.text.primary,
                    cursor: disabled || isPending ? "not-allowed" : "pointer",
                    display: "flex",
                    flexDirection: "column",
                    font: "inherit",
                    gap: 0.25,
                    opacity: disabled ? 0.5 : 1,
                    p: 2,
                    transition: `border-color ${tokens.motion.fast} ${tokens.motion.curve}, background-color ${tokens.motion.fast} ${tokens.motion.curve}`,
                    "&:hover":
                      disabled || isPending
                        ? undefined
                        : {
                            bgcolor: tokens.color.bg.surfaceHover,
                            borderColor: tokens.color.border.accent,
                          },
                  }}
                >
                  <Typography
                    sx={{
                      color: tokens.color.accent.violet,
                      fontFamily: tokens.font.mono,
                      fontSize: 22,
                      fontWeight: 800,
                      lineHeight: 1,
                    }}
                  >
                    {isPending ? (
                      <CircularProgress
                        size={18}
                        sx={{ color: tokens.color.accent.violet }}
                      />
                    ) : (
                      formatMoney(amount)
                    )}
                  </Typography>
                  <Typography
                    sx={{
                      color: tokens.color.text.muted,
                      fontSize: 11,
                      fontWeight: 700,
                      letterSpacing: 0.4,
                      textTransform: "uppercase",
                    }}
                  >
                    top up
                  </Typography>
                </Box>
              );
            })}
          </Box>

          <Stack direction="row" alignItems="center" spacing={1} sx={{ mt: 2 }}>
            <LockOutlined sx={{ color: tokens.color.text.muted, fontSize: 14 }} />
            <Typography sx={{ color: tokens.color.text.muted, fontSize: 12.5 }}>
              Powered by Stripe. You&apos;ll come back here on success.
            </Typography>
          </Stack>

          {topUpError && (
            <Box sx={{ mt: 2 }}>
              <ErrorPanel error={topUpError} title="Could not start top-up" />
            </Box>
          )}
        </Card>

        {/* 7-day spend strip */}
        <SpendStripCard
          loading={budgetQ.loading}
          points={dailySpend}
          total={total7d}
        />

        {/* Spend by model */}
        <BreakdownTable loading={budgetQ.loading} rows={breakdown} />

        {/* Top-up history */}
        <TopUpHistoryTable
          loading={topUpsQ.loading}
          error={topUpsQ.error}
          rows={topUps}
          onRefresh={() => topUpsQ.refetch()}
        />
      </Stack>

      <Snackbar
        open={!!snack}
        autoHideDuration={5000}
        onClose={() => setSnack(null)}
        anchorOrigin={{ vertical: "bottom", horizontal: "center" }}
      >
        <Alert severity="info" variant="filled" onClose={() => setSnack(null)}>
          {snack}
        </Alert>
      </Snackbar>
    </Box>
  );
}

function MiniStat({
  label,
  value,
  loading,
}: {
  label: string;
  value: string;
  loading: boolean;
}) {
  return (
    <Box>
      <Typography variant="overline" sx={{ color: tokens.color.text.muted }}>
        {label}
      </Typography>
      {loading ? (
        <Skeleton width={90} height={28} sx={skelSx} />
      ) : (
        <Typography
          sx={{
            color: tokens.color.text.primary,
            fontFamily: tokens.font.mono,
            fontSize: 18,
            fontWeight: 700,
          }}
        >
          {value}
        </Typography>
      )}
    </Box>
  );
}

function SpendStripCard({
  loading,
  points,
  total,
}: {
  loading: boolean;
  points: SparklinePoint[];
  total: number;
}) {
  return (
    <Card sx={{ p: 2.5 }}>
      <Stack
        direction={{ xs: "column", sm: "row" }}
        alignItems={{ xs: "flex-start", sm: "center" }}
        justifyContent="space-between"
        spacing={1.5}
        sx={{ mb: 1.5 }}
      >
        <Box>
          <Typography variant="overline" sx={{ color: tokens.color.text.secondary }}>
            Spend — last 7 days
          </Typography>
          <Typography
            sx={{
              color: tokens.color.text.primary,
              fontFamily: tokens.font.mono,
              fontSize: 24,
              fontWeight: 800,
            }}
          >
            {loading ? "—" : formatMoney(total)}
          </Typography>
        </Box>
        <Typography sx={{ color: tokens.color.text.muted, fontSize: 12 }}>
          per-day bucket · ledger
        </Typography>
      </Stack>
      {loading ? (
        <Skeleton variant="rectangular" height={180} sx={skelSx} />
      ) : (
        <SpendBars
          points={points.map((p, i) => ({
            label: p.ts ? shortDay(p.ts, i) : `D${i + 1}`,
            value: p.value,
          }))}
          height={180}
          ariaLabel="Spend per day over the last 7 days"
        />
      )}
    </Card>
  );
}

// shortDay — "Mon", "Tue", ... with the most recent bucket marked
// "today" so the operator instantly orients on the latest column.
function shortDay(iso: string, idx: number): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return `D${idx + 1}`;
  // The buckets render oldest→newest with 7 items; last index is today.
  return d.toLocaleDateString(undefined, { weekday: "short" });
}

function BreakdownTable({
  loading,
  rows,
}: {
  loading: boolean;
  rows: Array<{
    key: string;
    provider: string;
    model: string;
    costUsd: number;
    tokens: number;
    count: number;
  }>;
}) {
  return (
    <Box>
      <Typography
        sx={{
          color: tokens.color.text.primary,
          fontSize: 18,
          fontWeight: 800,
          mb: 1.5,
        }}
      >
        Spend by model
      </Typography>
      {loading ? (
        <Stack spacing={1}>
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} variant="rectangular" height={44} sx={skelSx} />
          ))}
        </Stack>
      ) : rows.length === 0 ? (
        <EmptyState
          icon={<AccountBalanceWalletOutlined sx={{ fontSize: 36 }} />}
          title="No spend yet"
          body="Once a run executes, you'll see exactly which model cost what."
        />
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
                "minmax(0,1fr) minmax(0,1.4fr) 110px 130px 90px",
              letterSpacing: 0.4,
              px: 2,
              py: 1.25,
              textTransform: "uppercase",
            }}
          >
            <Box>Provider</Box>
            <Box>Model</Box>
            <Box>Spent</Box>
            <Box>Tokens</Box>
            <Box>Calls</Box>
          </Box>
          {rows.map((r) => (
            <Box
              key={r.key}
              sx={{
                borderBottom: `1px solid ${tokens.color.border.subtle}`,
                display: { xs: "block", md: "grid" },
                gap: 2,
                gridTemplateColumns:
                  "minmax(0,1fr) minmax(0,1.4fr) 110px 130px 90px",
                alignItems: "center",
                px: 2,
                py: 1.5,
                "&:last-of-type": { borderBottom: 0 },
              }}
            >
              <BreakdownCell label="Provider" value={r.provider} bold />
              <BreakdownCell label="Model" value={r.model} />
              <BreakdownCell label="Spent" value={formatMoney(r.costUsd)} />
              <BreakdownCell label="Tokens" value={formatNumber(r.tokens)} muted />
              <BreakdownCell label="Calls" value={formatNumber(r.count)} muted />
            </Box>
          ))}
        </Card>
      )}
    </Box>
  );
}

function BreakdownCell({
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
    <Box sx={{ display: { xs: "flex", md: "block" }, gap: 1.5, minWidth: 0 }}>
      <Typography
        sx={{
          color: tokens.color.text.muted,
          display: { xs: "block", md: "none" },
          fontSize: 11,
          fontWeight: 700,
          letterSpacing: 0.4,
          minWidth: 70,
          textTransform: "uppercase",
        }}
      >
        {label}
      </Typography>
      <Typography
        sx={{
          color: muted ? tokens.color.text.secondary : tokens.color.text.primary,
          fontFamily: bold ? tokens.font.mono : undefined,
          fontSize: 13.5,
          fontWeight: bold ? 700 : 500,
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
        }}
      >
        {value}
      </Typography>
    </Box>
  );
}

function TopUpHistoryTable({
  loading,
  error,
  rows,
  onRefresh,
}: {
  loading: boolean;
  error: unknown;
  rows: Array<{
    id: string;
    amountUSD: number;
    status: string;
    createdAt: string;
    completedAt: string | null;
  }>;
  onRefresh: () => void;
}) {
  return (
    <Box>
      <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1.5 }}>
        <Typography
          sx={{
            color: tokens.color.text.primary,
            fontSize: 18,
            fontWeight: 800,
          }}
        >
          Top-up history
        </Typography>
        <Box sx={{ flex: 1 }} />
        <Tooltip title="Refresh">
          <IconButton onClick={onRefresh}>
            <RefreshRounded fontSize="small" />
          </IconButton>
        </Tooltip>
      </Stack>

      {error ? (
        <ErrorPanel
          error={error}
          title="Could not load top-up history"
          onRetry={onRefresh}
        />
      ) : loading ? (
        <Stack spacing={1}>
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} variant="rectangular" height={48} sx={skelSx} />
          ))}
        </Stack>
      ) : rows.length === 0 ? (
        <EmptyState
          icon={<AccountBalanceWalletOutlined sx={{ fontSize: 36 }} />}
          title="No top-ups yet"
          body="Pick an amount above to fund your first run."
        />
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
                "minmax(0,2fr) 110px 130px minmax(0,1fr) minmax(0,1fr)",
              letterSpacing: 0.4,
              px: 2,
              py: 1.25,
              textTransform: "uppercase",
            }}
          >
            <Box>Session</Box>
            <Box>Amount</Box>
            <Box>Status</Box>
            <Box>Started</Box>
            <Box>Completed</Box>
          </Box>
          {rows.map((r) => (
            <Box
              key={r.id}
              sx={{
                borderBottom: `1px solid ${tokens.color.border.subtle}`,
                display: { xs: "block", md: "grid" },
                gap: 2,
                gridTemplateColumns:
                  "minmax(0,2fr) 110px 130px minmax(0,1fr) minmax(0,1fr)",
                alignItems: "center",
                px: 2,
                py: 1.5,
                "&:last-of-type": { borderBottom: 0 },
              }}
            >
              <BreakdownCell label="Session" value={r.id} muted />
              <BreakdownCell label="Amount" value={formatMoney(r.amountUSD)} bold />
              <Box sx={{ display: { xs: "flex", md: "block" }, gap: 1.5 }}>
                <Typography
                  sx={{
                    color: tokens.color.text.muted,
                    display: { xs: "block", md: "none" },
                    fontSize: 11,
                    fontWeight: 700,
                    letterSpacing: 0.4,
                    minWidth: 70,
                    textTransform: "uppercase",
                  }}
                >
                  Status
                </Typography>
                <StatusBadge status={r.status} />
              </Box>
              <BreakdownCell
                label="Started"
                value={formatDateTime(r.createdAt)}
                muted
              />
              <BreakdownCell
                label="Completed"
                value={r.completedAt ? formatDateTime(r.completedAt) : "—"}
                muted
              />
            </Box>
          ))}
        </Card>
      )}
    </Box>
  );
}

// MetricCard import kept for symmetry with DashboardPage even though
// WalletPage builds its own headline.
void MetricCard;

export default WalletPage;
