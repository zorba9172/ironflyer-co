"use client";

// /operator — operator-only admin console.
//
// Surfaces the five live operator queries the orchestrator exposes
// today (internal/graph/resolver/operator.resolver.go):
//   - operatorScaleSnapshot — capacity / queue / worker utilization
//   - operatorPendingApprovals(tenantID?) — org-wide approval queue
//   - operatorAuditCursor(since, limit) — recent audit entries
//   - operatorWalletSnapshot(tenantID) — on-demand wallet lookup
//   - operatorAbuseScore(tenantID, userID) — abuse triage lookup
//
// Gating: the orchestrator returns FORBIDDEN for non-operators, but we
// also gate the page client-side so a non-operator sees a clean "no
// access" panel instead of red error chrome.

import {
  AccountBalanceWalletOutlined,
  GavelOutlined,
  HistoryEduOutlined,
  PrecisionManufacturingOutlined,
  ReportProblemOutlined,
  SearchRounded,
  ShieldOutlined,
} from "@mui/icons-material";
import {
  Box,
  Button,
  Card,
  Divider,
  Stack,
  Tab,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Tabs,
  TextField,
  Typography,
} from "@mui/material";
import Link from "next/link";
import { useMemo, useState } from "react";
import {
  EmptyState,
  ErrorPanel,
  LoadingPanel,
  MetricCard,
  PageHeader,
  StatusBadge,
  VirtualTable,
  type VirtualTableColumn,
} from "../../src/components/cockpit";
import { RequireAuth, useAuth } from "../../src/lib/auth";
import {
  useOperatorAbuseScoreLazyQuery,
  useOperatorAuditCursorQuery,
  useOperatorPendingApprovalsQuery,
  useOperatorScaleSnapshotQuery,
  useOperatorWalletSnapshotLazyQuery,
} from "../../src/lib/gql/__generated__";
import { formatDateTime, formatMoney, formatNumber, formatPercent } from "../../src/lib/format";
import { tokens } from "../../src/theme";

function isOperator(plan?: string | null): boolean {
  if (!plan) return false;
  const p = plan.toLowerCase();
  return p === "operator" || p === "admin" || p === "owner";
}

const HOURS_24_AGO = (): string => {
  const d = new Date();
  d.setHours(d.getHours() - 24);
  return d.toISOString();
};

export default function OperatorPage() {
  return (
    <RequireAuth>
      <OperatorView />
    </RequireAuth>
  );
}

function OperatorView() {
  const { user } = useAuth();
  if (!isOperator(user?.plan)) {
    return (
      <Box>
        <PageHeader eyebrow="Restricted" title="Operator console" />
        <EmptyState
          icon={<ShieldOutlined sx={{ fontSize: 28 }} />}
          title="Operator role required"
          body="This console is reserved for callers whose plan is 'operator', 'admin', or 'owner'. Ask the workspace owner to grant operator access."
          cta={{ label: "Back to dashboard", href: "/dashboard" }}
        />
      </Box>
    );
  }
  return <OperatorContent />;
}

function OperatorContent() {
  const [tab, setTab] = useState(0);
  const since = useMemo(() => HOURS_24_AGO(), []);

  return (
    <Box>
      <PageHeader
        eyebrow="Operator"
        title="Admin console"
        description="Live capacity snapshot, approval queue, and per-tenant lookups. Operators only — read paths are safe; writes route through the deploy approval flow."
      />

      <ScaleSnapshotRow />

      <Box sx={{ mt: 4 }}>
        <Tabs
          value={tab}
          onChange={(_, v) => setTab(v)}
          sx={{
            borderBottom: `1px solid ${tokens.color.border.subtle}`,
            "& .MuiTab-root": {
              textTransform: "none",
              fontWeight: 700,
              fontSize: 13.5,
              minHeight: 44,
            },
            "& .Mui-selected": { color: `${tokens.color.text.primary} !important` },
            "& .MuiTabs-indicator": { backgroundColor: tokens.color.accent.violet, height: 2 },
          }}
        >
          <Tab label="Approval queue" icon={<GavelOutlined sx={{ fontSize: 18 }} />} iconPosition="start" />
          <Tab label="Audit trail" icon={<HistoryEduOutlined sx={{ fontSize: 18 }} />} iconPosition="start" />
          <Tab label="Tenant lookup" icon={<SearchRounded sx={{ fontSize: 18 }} />} iconPosition="start" />
        </Tabs>

        <Box sx={{ pt: 3 }}>
          {tab === 0 && <ApprovalsTab />}
          {tab === 1 && <AuditTab since={since} />}
          {tab === 2 && <TenantLookupTab />}
        </Box>
      </Box>
    </Box>
  );
}

function ScaleSnapshotRow() {
  const { data, loading, error } = useOperatorScaleSnapshotQuery({
    fetchPolicy: "cache-and-network",
    pollInterval: 30000,
  });
  const snap = data?.operatorScaleSnapshot;

  if (loading && !snap) return <LoadingPanel label="loading capacity" minHeight={140} />;
  if (error) return <ErrorPanel error={error} title="Could not load scale snapshot" />;

  return (
    <Box
      sx={{
        display: "grid",
        gap: { xs: 2, md: 2.5 },
        gridTemplateColumns: { xs: "1fr 1fr", md: "repeat(4, 1fr)" },
      }}
    >
      <MetricCard
        accent="lime"
        label="Active executions"
        icon={<PrecisionManufacturingOutlined sx={{ fontSize: 18 }} />}
        value={formatNumber(snap?.activeExecutions ?? 0)}
      />
      <MetricCard
        accent="yellow"
        label="Queued executions"
        value={formatNumber(snap?.queuedExecutions ?? 0)}
        hint={
          (snap?.queuedExecutions ?? 0) > 0
            ? "Queue draining — check sandbox capacity"
            : "No queue pressure"
        }
      />
      <MetricCard
        accent="sky"
        label="Sandbox capacity"
        value={formatNumber(snap?.sandboxCapacity ?? 0)}
      />
      <MetricCard
        accent="purple"
        label="Worker utilization"
        value={formatPercent(snap?.workerUtilizationPct ?? 0)}
        trend={{
          direction:
            (snap?.workerUtilizationPct ?? 0) > 80
              ? "up"
              : (snap?.workerUtilizationPct ?? 0) < 30
                ? "down"
                : "flat",
          label:
            (snap?.workerUtilizationPct ?? 0) > 80
              ? "saturated"
              : (snap?.workerUtilizationPct ?? 0) < 30
                ? "headroom"
                : "steady",
          polarity: "inverse",
        }}
      />
    </Box>
  );
}

function ApprovalsTab() {
  const { data, loading, error, refetch } = useOperatorPendingApprovalsQuery({
    variables: { tenantID: null },
    fetchPolicy: "cache-and-network",
  });
  const approvals = data?.operatorPendingApprovals ?? [];

  if (loading && approvals.length === 0) return <LoadingPanel label="loading approvals" />;
  if (error) return <ErrorPanel error={error} title="Could not load approvals" onRetry={() => void refetch()} />;
  if (approvals.length === 0) {
    return (
      <EmptyState
        icon={<GavelOutlined sx={{ fontSize: 28 }} />}
        title="No pending approvals"
        body="When a deploy gate requires human sign-off it will appear here. Approvals expire after the window the planner specified."
        cta={{ label: "Open deploys", href: "/deploy" }}
      />
    );
  }

  return (
    <Card sx={{ overflow: "hidden" }}>
      <Table size="small">
        <TableHead>
          <TableRow>
            <TableCell>Deploy</TableCell>
            <TableCell>Tenant</TableCell>
            <TableCell>Status</TableCell>
            <TableCell align="right">Cost impact</TableCell>
            <TableCell>Requested</TableCell>
            <TableCell>Expires</TableCell>
            <TableCell />
          </TableRow>
        </TableHead>
        <TableBody>
          {approvals.map((a) => (
            <TableRow key={a.id} hover>
              <TableCell sx={{ fontFamily: tokens.font.mono, fontSize: 12 }}>
                {a.deployID.slice(0, 12)}…
              </TableCell>
              <TableCell sx={{ fontFamily: tokens.font.mono, fontSize: 12 }}>
                {a.tenantID.slice(0, 10)}…
              </TableCell>
              <TableCell>
                <StatusBadge status={a.status} />
              </TableCell>
              <TableCell align="right" sx={{ fontFamily: tokens.font.mono }}>
                {formatMoney(a.costImpactUSD)}
              </TableCell>
              <TableCell sx={{ fontSize: 12, color: tokens.color.text.secondary }}>
                {formatDateTime(a.requestedAt)}
              </TableCell>
              <TableCell sx={{ fontSize: 12, color: tokens.color.text.secondary }}>
                {formatDateTime(a.expiresAt)}
              </TableCell>
              <TableCell align="right">
                <Button
                  size="small"
                  component={Link}
                  href={`/deploy/${a.deployID}`}
                  variant="outlined"
                >
                  Review
                </Button>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </Card>
  );
}

function AuditTab({ since }: { since: string }) {
  const { data, loading, error, refetch } = useOperatorAuditCursorQuery({
    variables: { since, limit: 50 },
    fetchPolicy: "cache-and-network",
  });
  const entries = data?.operatorAuditCursor ?? [];

  if (loading && entries.length === 0) return <LoadingPanel label="loading audit" />;
  if (error) return <ErrorPanel error={error} title="Could not load audit cursor" onRetry={() => void refetch()} />;
  if (entries.length === 0) {
    return (
      <EmptyState
        icon={<HistoryEduOutlined sx={{ fontSize: 28 }} />}
        title="No audit entries in window"
        body="The orchestrator hasn't logged any audit-relevant action in the last 24 hours."
      />
    );
  }

  return (
    <VirtualTable
      rows={entries}
      columns={AUDIT_COLUMNS}
      rowKey={(e) => e.id}
      estimatedRowHeight={36}
      height={480}
      emptyLabel="No audit entries in window."
      renderRow={(e) => (
        <>
          <TableCell sx={{ fontSize: 12, color: tokens.color.text.secondary }}>
            {formatDateTime(e.timestamp)}
          </TableCell>
          <TableCell sx={{ fontFamily: tokens.font.mono, fontSize: 12.5 }}>
            {e.action}
          </TableCell>
          <TableCell>
            <StatusBadge status={e.outcome.toLowerCase()} />
          </TableCell>
          <TableCell
            sx={{
              fontFamily: tokens.font.mono,
              fontSize: 11.5,
              color: tokens.color.text.muted,
              maxWidth: 280,
              overflow: "hidden",
              textOverflow: "ellipsis",
            }}
          >
            {e.hash}
          </TableCell>
        </>
      )}
    />
  );
}

const AUDIT_COLUMNS: VirtualTableColumn[] = [
  { key: "time", label: "Time", width: 180 },
  { key: "action", label: "Action" },
  { key: "outcome", label: "Outcome", width: 130 },
  { key: "hash", label: "Hash" },
];

function TenantLookupTab() {
  const [tenantID, setTenantID] = useState("");
  const [userID, setUserID] = useState("");

  const [loadWallet, walletState] = useOperatorWalletSnapshotLazyQuery({
    fetchPolicy: "network-only",
  });
  const [loadAbuse, abuseState] = useOperatorAbuseScoreLazyQuery({
    fetchPolicy: "network-only",
  });

  const wallet = walletState.data?.operatorWalletSnapshot;
  const abuse = abuseState.data?.operatorAbuseScore;
  const canQueryWallet = tenantID.trim().length > 0;
  const canQueryAbuse = tenantID.trim().length > 0 && userID.trim().length > 0;

  return (
    <Stack spacing={3}>
      <Card sx={{ p: { xs: 2.5, md: 3 } }}>
        <Typography variant="overline" sx={{ color: tokens.color.accent.violet, letterSpacing: 1.2 }}>
          Lookup
        </Typography>
        <Typography sx={{ mt: 0.5, fontSize: 16, fontWeight: 700, color: tokens.color.text.primary }}>
          Per-tenant operator queries
        </Typography>
        <Typography sx={{ mt: 1, fontSize: 13.5, color: tokens.color.text.secondary }}>
          Enter a tenant ID (and a user ID for abuse score). Queries run on
          demand against the orchestrator&apos;s operator surface.
        </Typography>

        <Box
          sx={{
            display: "grid",
            gap: 2,
            gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
            mt: 2,
          }}
        >
          <TextField
            fullWidth
            size="small"
            label="Tenant ID"
            placeholder="e.g. user-abc123 or org-xyz"
            value={tenantID}
            onChange={(e) => setTenantID(e.target.value)}
            slotProps={{
              input: { sx: { fontFamily: tokens.font.mono, fontSize: 13 } },
            }}
          />
          <TextField
            fullWidth
            size="small"
            label="User ID (abuse score only)"
            placeholder="e.g. user-abc123"
            value={userID}
            onChange={(e) => setUserID(e.target.value)}
            slotProps={{
              input: { sx: { fontFamily: tokens.font.mono, fontSize: 13 } },
            }}
          />
        </Box>

        <Stack direction="row" spacing={1.25} sx={{ mt: 2.5, flexWrap: "wrap", gap: 1 }}>
          <Button
            size="small"
            variant="contained"
            color="primary"
            startIcon={<AccountBalanceWalletOutlined sx={{ fontSize: 16 }} />}
            disabled={!canQueryWallet}
            onClick={() => loadWallet({ variables: { tenantID: tenantID.trim() } })}
          >
            Wallet snapshot
          </Button>
          <Button
            size="small"
            variant="outlined"
            startIcon={<ReportProblemOutlined sx={{ fontSize: 16 }} />}
            disabled={!canQueryAbuse}
            onClick={() =>
              loadAbuse({
                variables: { tenantID: tenantID.trim(), userID: userID.trim() },
              })
            }
          >
            Abuse score
          </Button>
        </Stack>
      </Card>

      <Box
        sx={{
          display: "grid",
          gap: 2.5,
          gridTemplateColumns: { xs: "1fr", md: "minmax(0, 1.4fr) minmax(0, 1fr)" },
        }}
      >
        <Card sx={{ p: { xs: 2.5, md: 3 } }}>
          <Typography variant="overline" sx={{ color: tokens.color.text.secondary, letterSpacing: 1.2 }}>
            Wallet snapshot
          </Typography>
          {walletState.loading ? (
            <LoadingPanel minHeight={120} />
          ) : walletState.error ? (
            <ErrorPanel error={walletState.error} title="Wallet snapshot failed" />
          ) : !wallet ? (
            <Typography sx={{ mt: 2, fontSize: 13.5, color: tokens.color.text.muted }}>
              Run a wallet snapshot to see live balance, holds, and
              lifetime totals for a tenant.
            </Typography>
          ) : (
            <Box sx={{ mt: 1.5 }}>
              <Typography sx={{ fontFamily: tokens.font.mono, fontSize: 12, color: tokens.color.text.muted }}>
                {wallet.tenantID}
              </Typography>
              <Divider sx={{ my: 2 }} />
              <Box
                sx={{
                  display: "grid",
                  gap: 2,
                  gridTemplateColumns: "1fr 1fr",
                }}
              >
                <SnapshotRow label="Balance" value={formatMoney(wallet.balanceUSD)} mono />
                <SnapshotRow label="Held" value={formatMoney(wallet.holdUSD)} mono />
                <SnapshotRow label="Lifetime top-up" value={formatMoney(wallet.lifetimeTopUpUSD)} mono />
                <SnapshotRow label="Lifetime spend" value={formatMoney(wallet.lifetimeSpendUSD)} mono />
              </Box>
            </Box>
          )}
        </Card>

        <Card sx={{ p: { xs: 2.5, md: 3 } }}>
          <Typography variant="overline" sx={{ color: tokens.color.text.secondary, letterSpacing: 1.2 }}>
            Abuse score
          </Typography>
          {abuseState.loading ? (
            <LoadingPanel minHeight={120} />
          ) : abuseState.error ? (
            <ErrorPanel error={abuseState.error} title="Abuse score failed" />
          ) : !abuse ? (
            <Typography sx={{ mt: 2, fontSize: 13.5, color: tokens.color.text.muted }}>
              Pair a tenant + user ID to triage a flagged caller.
            </Typography>
          ) : (
            <Box sx={{ mt: 1.5 }}>
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 36,
                  fontWeight: 700,
                  color: tokens.color.text.primary,
                  lineHeight: 1,
                }}
              >
                {abuse.score}
              </Typography>
              <Box sx={{ mt: 1 }}>
                <StatusBadge
                  status={abuse.tier}
                  tone={abuse.score >= 75 ? "danger" : abuse.score >= 40 ? "warning" : "success"}
                />
              </Box>
              <Stack spacing={0.5} sx={{ mt: 2 }}>
                <SnapshotRow label="Tenant" value={abuse.tenantID} mono />
                <SnapshotRow label="User" value={abuse.userID} mono />
              </Stack>
            </Box>
          )}
        </Card>
      </Box>
    </Stack>
  );
}

function SnapshotRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <Stack direction="row" justifyContent="space-between" alignItems="baseline">
      <Typography sx={{ fontSize: 12, color: tokens.color.text.muted }}>{label}</Typography>
      <Typography
        sx={{
          fontSize: 13.5,
          color: tokens.color.text.primary,
          fontFamily: mono ? tokens.font.mono : undefined,
          maxWidth: "60%",
          textAlign: "right",
          wordBreak: "break-all",
        }}
      >
        {value}
      </Typography>
    </Stack>
  );
}
