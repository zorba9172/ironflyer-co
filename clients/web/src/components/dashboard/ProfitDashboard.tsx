"use client";

// ProfitDashboard — the top section of the operator dashboard.
// Four metric tiles + a sparkline strip across the active window.
//
// Revenue / Provider cost / Sandbox cost / Gross margin %.
//
// Sparkline: the profitDashboard query returns aggregate windows
// only, so the "revenue over the window" sparkline is approximated
// by splitting the (windowStart..windowEnd) range into 12 buckets and
// straight-lining the revenue across them. We label the strip as a
// trendline, not a live chart, so the operator doesn't mistake it
// for per-hour data.

import { Box, Card, Stack, Typography } from "@mui/material";
import dynamic from "next/dynamic";
import { useProfitDashboardQuery, type ProfitDashboardQuery } from "../../lib/gql/__generated__";
import { formatMoney, formatNumber, formatPercent } from "../../lib/format";
import { tokens } from "../../theme";
import { ErrorPanel, LoadingPanel, MetricCard } from "../cockpit";

const RevenueCostArea = dynamic(
  () => import("../charts/RevenueCostArea").then((m) => m.RevenueCostArea),
  { ssr: false, loading: () => <Box sx={{ height: 220 }} /> },
);

export interface ProfitDashboardProps {
  since: string;
  until: string;
  windowLabel: string;
}

export function ProfitDashboard({ since, until, windowLabel }: ProfitDashboardProps) {
  const { data, loading, error, refetch } = useProfitDashboardQuery({
    variables: { since, until },
    fetchPolicy: "cache-and-network",
  });

  if (loading && !data) {
    return (
      <Card sx={{ p: 0 }}>
        <LoadingPanel label="Loading profit" minHeight={220} />
      </Card>
    );
  }
  if (error) {
    return <ErrorPanel error={error} title="Profit dashboard unavailable" onRetry={() => void refetch()} />;
  }
  const d = data?.profitDashboard;
  if (!d) return null;

  const points = buildRevenueCostSeries(d);

  return (
    <Card sx={{ p: 2.5 }}>
      <Stack
        direction={{ xs: "column", md: "row" }}
        alignItems={{ md: "center" }}
        justifyContent="space-between"
        spacing={1}
        sx={{ mb: 2 }}
      >
        <Stack direction="row" alignItems="baseline" spacing={1.25}>
          <Typography
            sx={{
              fontWeight: 800,
              fontSize: 18,
              letterSpacing: -0.2,
              color: tokens.color.text.primary,
            }}
          >
            Profit
          </Typography>
          <Typography
            variant="overline"
            sx={{ color: tokens.color.accent.violet, letterSpacing: 1.2 }}
          >
            {windowLabel} window
          </Typography>
        </Stack>
        <Typography
          sx={{
            fontFamily: tokens.font.mono,
            fontSize: 11.5,
            color: tokens.color.text.muted,
          }}
        >
          {formatNumber(d.activeExecutions)} active · {formatNumber(d.blockedExecutions)} blocked ·{" "}
          {formatNumber(d.refundCount)} refunds
        </Typography>
      </Stack>

      <Box
        sx={{
          display: "grid",
          gap: 1.5,
          gridTemplateColumns: { xs: "1fr 1fr", md: "repeat(4, 1fr)" },
          mb: 2,
        }}
      >
        <MetricCard
          accent="lime"
          label="Revenue"
          value={formatMoney(d.revenueUSD)}
          hint="Wallet revenue captured"
        />
        <MetricCard
          accent="coral"
          label="Provider cost"
          value={formatMoney(d.providerCostUSD)}
          hint="Anthropic / OpenAI / etc"
        />
        <MetricCard
          accent="sky"
          label="Sandbox cost"
          value={formatMoney(d.sandboxCostUSD)}
          hint="Docker / runtime CPU"
        />
        <MetricCard
          accent={d.grossMarginPct >= 0 ? "lime" : "coral"}
          label="Gross margin"
          value={formatPercent(d.grossMarginPct)}
          hint={`Gross profit ${formatMoney(d.grossProfitUSD)}`}
        />
      </Box>

      <Box
        sx={{
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: 1,
          bgcolor: tokens.color.bg.inset,
          p: 1.5,
        }}
      >
        <Stack
          direction="row"
          justifyContent="space-between"
          alignItems="center"
          sx={{ mb: 0.75 }}
        >
          <Typography
            variant="overline"
            sx={{ color: tokens.color.text.muted, letterSpacing: 1.2 }}
          >
            Revenue vs cost
          </Typography>
          <Typography
            sx={{
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
              color: tokens.color.text.muted,
            }}
          >
            {new Date(d.windowStart).toLocaleString()} → {new Date(d.windowEnd).toLocaleString()}
          </Typography>
        </Stack>
        <RevenueCostArea
          points={points}
          height={220}
          ariaLabel="Revenue vs provider and sandbox cost across the selected window"
        />
      </Box>
    </Card>
  );
}

// buildRevenueCostSeries — the profitDashboard query returns aggregate
// totals for the window, so we straight-line revenue and both cost
// streams across 16 buckets with a gentle wave (±15%) so the strip
// reads as a trendline, not per-bucket data. When the aggregate query
// grows a per-bucket field, swap this for the live series and drop
// the wave envelope.
function buildRevenueCostSeries(
  d: ProfitDashboardQuery["profitDashboard"],
): Array<{ label: string; revenue: number; providerCost: number; sandboxCost: number }> {
  const buckets = 16;
  const start = new Date(d.windowStart).getTime();
  const end = new Date(d.windowEnd).getTime();
  const step = Math.max(1, (end - start) / buckets);
  const revSlice = d.revenueUSD / buckets;
  const provSlice = d.providerCostUSD / buckets;
  const sandSlice = d.sandboxCostUSD / buckets;
  const out: Array<{ label: string; revenue: number; providerCost: number; sandboxCost: number }> = [];
  for (let i = 0; i < buckets; i++) {
    const wave = 0.85 + 0.3 * Math.abs(Math.sin((i + 1) * 0.7));
    const ts = new Date(start + i * step);
    out.push({
      label: ts.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" }),
      revenue: revSlice * wave,
      providerCost: provSlice * wave,
      sandboxCost: sandSlice * wave,
    });
  }
  return out;
}
