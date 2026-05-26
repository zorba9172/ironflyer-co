"use client";

// /dashboard — the cockpit Overview surface.
//
// V22 framing: this is the first page every authenticated caller lands on
// after signing in. It is deliberately tight — wallet + execution health
// up top, the latest paid runs immediately under, plus the operator-only
// margin/scale dashboards stacked on the bottom for callers who run the
// platform. Anything richer than four KPIs + one table is operator
// territory and lives on /operator.
//
// Hooks wired:
//   - useWalletQuery → balance + lifetime spend
//   - useExecutionsQuery → KPI rollups + recent table
//   - useProfitDashboardQuery / useScaleDashboardQuery via the existing
//     dashboard panels (operator-only)

import { ArrowForwardRounded, BoltRounded } from "@mui/icons-material";
import { Box, Button, Stack } from "@mui/material";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useMemo } from "react";
import { RequireAuth, useAuth } from "../../src/lib/auth";
import {
  EmptyState,
  ErrorPanel,
  LoadingPanel,
  MetricCard,
  PageHeader,
} from "../../src/components/cockpit";
import { ExecutionsTable } from "../../src/components/executions";
import {
  ProfitDashboard,
  ScaleDashboard,
  WindowSelector,
  windowToRange,
  type DashboardWindow,
  WINDOW_OPTIONS,
} from "../../src/components/dashboard";
import { useExecutionsQuery } from "../../src/lib/gql/__generated__";
import { useWalletBalance } from "../../src/lib/hooks";
import { formatMoney, formatPercent } from "../../src/lib/format";

const DEFAULT_WINDOW: DashboardWindow = "7d";

const SUCCEEDED_STATES = new Set(["succeeded", "success"]);
const TERMINAL_STATES = new Set(["succeeded", "success", "failed", "killed", "stopped", "refunded"]);

function isOperator(plan?: string | null): boolean {
  if (!plan) return false;
  const p = plan.toLowerCase();
  return p === "operator" || p === "admin" || p === "owner";
}

function parseWindow(value: string | null): DashboardWindow {
  if (!value) return DEFAULT_WINDOW;
  if ((WINDOW_OPTIONS as readonly string[]).includes(value)) {
    return value as DashboardWindow;
  }
  return DEFAULT_WINDOW;
}

export default function DashboardPage() {
  return (
    <RequireAuth>
      <DashboardInner />
    </RequireAuth>
  );
}

function DashboardInner() {
  const { user } = useAuth();
  const router = useRouter();
  const search = useSearchParams();
  const operator = isOperator(user?.plan);

  const window: DashboardWindow = parseWindow(search.get("window"));
  const setWindow = (next: DashboardWindow) => {
    const params = new URLSearchParams(search?.toString());
    params.set("window", next);
    router.replace(`/dashboard?${params.toString()}`);
  };
  const { since, until } = useMemo(() => windowToRange(window), [window]);

  const walletQ = useWalletBalance();
  const execQ = useExecutionsQuery({
    variables: { limit: 50, offset: 0 },
    fetchPolicy: "cache-and-network",
    pollInterval: 30000,
    notifyOnNetworkStatusChange: true,
  });

  const executions = execQ.data?.executions ?? [];

  // KPI rollups computed client-side from the same executions list the
  // table renders. Keeps the surface as one query rather than five.
  const rollup = useMemo(() => {
    const weekAgo = Date.now() - 7 * 24 * 60 * 60 * 1000;
    let weekCount = 0;
    let terminalCount = 0;
    let succeededCount = 0;
    let marginNumerator = 0;
    let marginDenominator = 0;
    for (const e of executions) {
      const ts = new Date(e.endedAt ?? e.startedAt ?? e.admittedAt ?? e.createdAt).getTime();
      if (Number.isFinite(ts) && ts >= weekAgo) weekCount += 1;
      const s = e.status.toLowerCase();
      if (TERMINAL_STATES.has(s)) {
        terminalCount += 1;
        if (SUCCEEDED_STATES.has(s)) succeededCount += 1;
      }
      if (e.grossMarginPct !== null) {
        marginNumerator += e.grossMarginPct;
        marginDenominator += 1;
      }
    }
    const successRate = terminalCount === 0 ? null : (succeededCount / terminalCount) * 100;
    const avgMargin = marginDenominator === 0 ? null : marginNumerator / marginDenominator;
    return { weekCount, successRate, avgMargin };
  }, [executions]);

  const recent = useMemo(() => executions.slice(0, 10), [executions]);

  const walletReady = walletQ.totalUSD > 0 || walletQ.availableUSD > 0 || walletQ.reservedUSD > 0 || !walletQ.loading;
  const loading = walletQ.loading && !walletReady && execQ.loading && !execQ.data;
  const fatalError = walletQ.error && !walletReady ? walletQ.error : execQ.error && !execQ.data ? execQ.error : null;

  if (loading) return <LoadingPanel label="Loading cockpit" />;
  if (fatalError) {
    return (
      <ErrorPanel
        error={fatalError}
        title="Could not load cockpit"
        onRetry={() => {
          void walletQ.refetch();
          void execQ.refetch();
        }}
      />
    );
  }

  return (
    <Box>
      <PageHeader
        eyebrow="Overview"
        title={user?.name ? `Welcome back, ${user.name.split(" ")[0]}` : "Welcome back"}
        description="Wallet, execution health, and the most recent paid runs. Drill into any tile to see the ledger entries behind it."
        actions={
          <Stack direction="row" spacing={1.25}>
            <Button
              component={Link}
              href="/executions"
              variant="outlined"
              sx={{
                color: "text.primary",
                borderColor: "divider",
              }}
            >
              All executions
            </Button>
            <Button
              component={Link}
              href="/"
              variant="contained"
              color="primary"
              startIcon={<BoltRounded sx={{ fontSize: 18 }} />}
            >
              Start a build
            </Button>
          </Stack>
        }
      />

      <Box
        sx={{
          display: "grid",
          gap: { xs: 2, md: 2.5 },
          gridTemplateColumns: { xs: "1fr 1fr", md: "repeat(4, 1fr)" },
          mb: { xs: 3, md: 4 },
        }}
      >
        <MetricCard
          accent="purple"
          label="Wallet available"
          value={formatMoney(walletQ.availableUSD)}
          hint={
            walletReady
              ? `${formatMoney(walletQ.reservedUSD)} held`
              : "Wallet pending"
          }
        />
        <MetricCard
          accent="sky"
          label="Executions · 7d"
          value={rollup.weekCount.toString()}
          hint={executions.length > 0 ? `${executions.length} total in window` : "No paid runs yet"}
        />
        <MetricCard
          accent={rollup.successRate === null ? "neutral" : rollup.successRate >= 80 ? "lime" : "yellow"}
          label="Success rate"
          value={rollup.successRate === null ? "—" : formatPercent(rollup.successRate)}
          hint="Terminal runs · succeeded / completed"
        />
        <MetricCard
          accent={rollup.avgMargin === null ? "neutral" : rollup.avgMargin >= 0 ? "lime" : "coral"}
          label="Avg gross margin"
          value={rollup.avgMargin === null ? "—" : formatPercent(rollup.avgMargin)}
          hint="Across runs with reported margin"
        />
      </Box>

      <Stack spacing={2.5}>
        <Box>
          <Stack
            direction="row"
            alignItems="center"
            justifyContent="space-between"
            sx={{ mb: 1.5 }}
          >
            <Box sx={{ fontWeight: 800, fontSize: 16 }}>Recent executions</Box>
            <Button
              component={Link}
              href="/executions"
              size="small"
              endIcon={<ArrowForwardRounded sx={{ fontSize: 14 }} />}
              sx={{ color: "text.secondary" }}
            >
              See all
            </Button>
          </Stack>
          {recent.length === 0 ? (
            <EmptyState
              title="No paid executions yet"
              body="Describe an idea from the home composer to admit your first execution. Once it runs you'll see the ledger entries and gate verdicts here."
              cta={{ label: "Start a build", href: "/" }}
            />
          ) : (
            <ExecutionsTable rows={recent} />
          )}
        </Box>

        {operator && (
          <Box>
            <PageHeader
              eyebrow="Operator console"
              title="Margin & scale"
              description="Margin gates scale. Scale gates cohorts. These panels stay healthy or every other tile drifts."
              actions={<WindowSelector value={window} onChange={setWindow} />}
              sx={{ mb: 2 }}
            />
            <Stack spacing={2.5}>
              <ProfitDashboard since={since} until={until} windowLabel={window} />
              <ScaleDashboard />
            </Stack>
          </Box>
        )}
      </Stack>
    </Box>
  );
}
