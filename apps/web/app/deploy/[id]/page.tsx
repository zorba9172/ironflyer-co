"use client";

// /deploy/[id] — deploy detail surface with three tabs: Overview,
// Approvals, Events.
//
// State-aware actions live in DeployActionBar; see the comment in
// that component for the full state machine. The Approvals tab pulls
// from pendingDeployApprovals (the only approval query exposed by
// the orchestrator's GraphQL today) and filters client-side to the
// approvals attached to this deploy; once the schema exposes a
// per-deploy approval history field we should swap to that.

import { ArrowBackRounded, LaunchRounded } from "@mui/icons-material";
import {
  Box,
  Button,
  Card,
  Chip,
  Stack,
  Tab,
  Tabs,
  Typography,
} from "@mui/material";
import Link from "next/link";
import { use, useMemo, useState } from "react";
import {
  ErrorPanel,
  LoadingPanel,
  MetricCard,
  PageHeader,
  StatusBadge,
} from "../../../src/components/cockpit";
import {
  ApprovalsList,
  DeployActionBar,
  DeployEventStream,
  GateSummaryCard,
} from "../../../src/components/deploy";
import { RequireAuth, useAuth } from "../../../src/lib/auth";
import {
  useDeployQuery,
  usePendingDeployApprovalsQuery,
} from "../../../src/lib/gql/__generated__";
import { formatDateTime, formatMoney } from "../../../src/lib/format";
import { relativeTime } from "../../../src/lib/relativeTime";
import { tokens } from "../../../src/theme";

function isOperator(plan?: string | null): boolean {
  if (!plan) return false;
  const p = plan.toLowerCase();
  return p === "operator" || p === "admin" || p === "owner";
}

export default function DeployDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  return (
    <RequireAuth>
      <DeployInner id={id} />
    </RequireAuth>
  );
}

function DeployInner({ id }: { id: string }) {
  const [tab, setTab] = useState<"overview" | "approvals" | "events">("overview");
  const { user } = useAuth();
  const operator = isOperator(user?.plan);

  const { data, loading, error, refetch } = useDeployQuery({
    variables: { id },
    fetchPolicy: "cache-and-network",
    notifyOnNetworkStatusChange: true,
    pollInterval: 5000,
  });

  // The orchestrator does not yet expose a per-deploy approval
  // history field; we filter the operator-wide pending list down to
  // this deploy. Non-operators get an empty list (the resolver hides
  // it from them) which we soften into "no pending approvals".
  const approvalsQ = usePendingDeployApprovalsQuery({
    fetchPolicy: "cache-and-network",
    notifyOnNetworkStatusChange: true,
    skip: tab !== "approvals",
  });
  const approvals = useMemo(
    () => (approvalsQ.data?.pendingDeployApprovals ?? []).filter((a) => a.deployID === id),
    [approvalsQ.data, id],
  );

  if (loading && !data) return <LoadingPanel label="Loading deploy" />;
  if (error) {
    return (
      <ErrorPanel error={error} title="Deploy unavailable" onRetry={() => void refetch()} />
    );
  }
  const d = data?.deploy;
  if (!d) {
    return (
      <Box>
        <PageHeader
          title="Deploy not found"
          breadcrumbs={[{ label: "Deploys", href: "/deploy" }, { label: id }]}
        />
        <Typography sx={{ color: tokens.color.text.muted }}>
          This deploy does not exist or you do not have access.
        </Typography>
      </Box>
    );
  }

  return (
    <Box>
      <PageHeader
        eyebrow={`Deploy · ${shortId(d.id)}`}
        title={`${d.target} → ${d.environment}`}
        breadcrumbs={[
          { label: "Deploys", href: "/deploy" },
          { label: shortId(d.id) },
        ]}
        actions={
          <Button
            component={Link}
            href="/deploy"
            variant="outlined"
            startIcon={<ArrowBackRounded fontSize="small" />}
            sx={{
              borderColor: tokens.color.border.strong,
              color: tokens.color.text.primary,
            }}
          >
            Back
          </Button>
        }
      />
      <Stack direction="row" spacing={1.5} alignItems="center" sx={{ mb: 2 }}>
        <StatusBadge status={d.status} />
        {d.diffHash && (
          <Chip
            size="small"
            label={`diff ${d.diffHash.slice(0, 10)}`}
            sx={{
              bgcolor: tokens.color.bg.surfaceRaised,
              color: tokens.color.text.secondary,
              border: `1px solid ${tokens.color.border.subtle}`,
              fontFamily: tokens.font.mono,
              fontSize: 11,
            }}
          />
        )}
        {d.artifactHash && (
          <Chip
            size="small"
            label={`artifact ${d.artifactHash.slice(0, 10)}`}
            sx={{
              bgcolor: tokens.color.bg.surfaceRaised,
              color: tokens.color.text.secondary,
              border: `1px solid ${tokens.color.border.subtle}`,
              fontFamily: tokens.font.mono,
              fontSize: 11,
            }}
          />
        )}
      </Stack>

      <Tabs
        value={tab}
        onChange={(_, v) => setTab(v)}
        sx={{
          mb: 2,
          borderBottom: `1px solid ${tokens.color.border.subtle}`,
          "& .MuiTab-root": {
            textTransform: "none",
            fontWeight: 700,
            fontSize: 13.5,
            minHeight: 40,
          },
          "& .Mui-selected": { color: tokens.color.text.primary },
          "& .MuiTabs-indicator": { backgroundColor: tokens.color.accent.violet },
        }}
      >
        <Tab value="overview" label="Overview" />
        <Tab value="approvals" label="Approvals" />
        <Tab value="events" label="Events" />
      </Tabs>

      {tab === "overview" && (
        <Stack spacing={2}>
          <Card sx={{ p: 2.5 }}>
            <Stack direction="row" spacing={2} flexWrap="wrap" sx={{ rowGap: 1.5 }}>
              <Field label="Target" value={d.target} mono />
              <Field label="Environment" value={d.environment} mono />
              <Field
                label="Provider deploy ID"
                value={d.providerDeploymentID ?? "—"}
                mono
              />
              <Field label="Created" value={formatDateTime(d.createdAt)} />
              <Field
                label="Preview ready"
                value={d.previewReadyAt ? relativeTime(d.previewReadyAt) : "—"}
              />
              <Field
                label="Promoted"
                value={d.promotedAt ? relativeTime(d.promotedAt) : "—"}
              />
              <Field
                label="Rolled back"
                value={d.rolledBackAt ? relativeTime(d.rolledBackAt) : "—"}
              />
            </Stack>
            <Stack direction="row" spacing={1.5} sx={{ mt: 2.5 }} flexWrap="wrap">
              {d.previewURL && (
                <Button
                  component="a"
                  href={d.previewURL}
                  target="_blank"
                  rel="noopener noreferrer"
                  variant="outlined"
                  endIcon={<LaunchRounded fontSize="small" />}
                  sx={{
                    borderColor: tokens.color.border.strong,
                    color: tokens.color.text.primary,
                    fontFamily: tokens.font.mono,
                  }}
                >
                  Open preview
                </Button>
              )}
              {d.productionURL && (
                <Button
                  component="a"
                  href={d.productionURL}
                  target="_blank"
                  rel="noopener noreferrer"
                  variant="contained"
                  color="primary"
                  endIcon={<LaunchRounded fontSize="small" />}
                  sx={{ fontFamily: tokens.font.mono }}
                >
                  Open production
                </Button>
              )}
            </Stack>
          </Card>
          <GateSummaryCard summary={d.gateSummary} />
          <Card sx={{ p: 2.5 }}>
            <Typography sx={{ fontWeight: 800, fontSize: 16, mb: 1.5 }}>Cost</Typography>
            <MetricCard accent="coral" label="Deploy cost" value={formatMoney(d.costUSD)} />
          </Card>
          <Card sx={{ p: 2.5 }}>
            <Typography sx={{ fontWeight: 800, fontSize: 16, mb: 1.5 }}>Actions</Typography>
            <DeployActionBar
              deploy={d}
              canApprove={operator}
              onChanged={() => void refetch()}
            />
          </Card>
        </Stack>
      )}

      {tab === "approvals" && (
        <Box>
          {approvalsQ.loading && !approvalsQ.data ? (
            <LoadingPanel label="Loading approvals" />
          ) : approvalsQ.error ? (
            <ErrorPanel
              error={approvalsQ.error}
              title="Approvals unavailable"
              onRetry={() => void approvalsQ.refetch()}
            />
          ) : (
            <ApprovalsList approvals={approvals} canDecide={operator} />
          )}
        </Box>
      )}

      {tab === "events" && <DeployEventStream deployID={d.id} />}
    </Box>
  );
}

function Field({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <Box sx={{ minWidth: 140 }}>
      <Typography
        variant="overline"
        sx={{ color: tokens.color.text.muted, letterSpacing: 1.2, display: "block" }}
      >
        {label}
      </Typography>
      <Typography
        sx={{
          color: tokens.color.text.primary,
          fontFamily: mono ? tokens.font.mono : undefined,
          fontSize: 13.5,
          fontWeight: 600,
        }}
      >
        {value}
      </Typography>
    </Box>
  );
}

function shortId(id: string): string {
  if (id.length <= 14) return id;
  return `${id.slice(0, 10)}…`;
}
