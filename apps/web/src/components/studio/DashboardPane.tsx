"use client";

// DashboardPane — execution-level KPI grid: cost vs budget, gates
// passed, completion score, gross margin. Below the grid we render a
// terse event timeline pulled from the chat buffer (system + costtick
// entries) and a profit summary card.

import {
  Box,
  Card,
  Divider,
  Stack,
  Typography,
} from "@mui/material";
import { useMemo } from "react";
import { LoadingPanel } from "../cockpit/LoadingPanel";
import { MetricCard } from "../cockpit/MetricCard";
import { MoneyChip } from "../cockpit/MoneyChip";
import { StatusBadge } from "../cockpit/StatusBadge";
import { formatMoney } from "../../lib/format";
import { relativeTime } from "../../lib/relativeTime";
import {
  useExecutionSupportBundleQuery,
  type ExecutionCoreFragment,
} from "../../lib/gql/__generated__";
import { tokens } from "../../theme";
import type { StudioMessage } from "./types";

export interface DashboardPaneProps {
  execution: ExecutionCoreFragment;
  messages: StudioMessage[];
}

const TERMINAL = new Set(["succeeded", "failed", "stopped", "killed", "refunded"]);

export function DashboardPane({ execution, messages }: DashboardPaneProps) {
  const isTerminal = TERMINAL.has(execution.status);
  const query = useExecutionSupportBundleQuery({
    variables: { executionID: execution.id },
    pollInterval: isTerminal ? 0 : 5000,
    fetchPolicy: "cache-and-network",
  });
  const bundle = query.data?.executionSupportBundle;

  const gates = bundle?.gateReport.stages ?? [];
  const passedCount = gates.filter(
    (g) => g.status === "passed" || g.status === "pass",
  ).length;
  const totalGates = gates.length;
  const completion = (bundle?.gateReport.completionScore ?? execution.completionScore) || 0;
  const margin =
    bundle?.costReport.grossMarginPct ?? execution.grossMarginPct ?? null;

  const budget = execution.budgetUSD;
  const spent = execution.spentUSD;
  const budgetPct = budget > 0 ? Math.min(1, spent / budget) : 0;

  // Timeline = system + costtick entries, newest last.
  const timeline = useMemo(
    () =>
      messages.filter(
        (m) => m.role === "system" || m.role === "costtick",
      ),
    [messages],
  );

  if (!bundle && query.loading) {
    return <LoadingPanel label="Loading support bundle…" minHeight="100%" />;
  }

  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.base,
        height: "100%",
        overflowY: "auto",
        p: 2.5,
      }}
    >
      <Stack spacing={2.5}>
        <Stack direction="row" alignItems="center" spacing={1.5}>
          <Typography
            sx={{
              color: tokens.color.text.primary,
              fontSize: 18,
              fontWeight: 800,
              letterSpacing: -0.2,
            }}
          >
            Execution dashboard
          </Typography>
          <StatusBadge status={execution.status} />
          <Box sx={{ flex: 1 }} />
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
              letterSpacing: 0.6,
              textTransform: "uppercase",
            }}
          >
            id · {execution.id.slice(0, 8)}
          </Typography>
        </Stack>

        <Box
          sx={{
            display: "grid",
            gap: 1.5,
            gridTemplateColumns: {
              xs: "1fr",
              sm: "1fr 1fr",
              lg: "repeat(4, 1fr)",
            },
          }}
        >
          <MetricCard
            label="Spent vs budget"
            value={`${formatMoney(spent)}`}
            hint={`of ${formatMoney(budget)} hold (${(budgetPct * 100).toFixed(0)}%)`}
            accent="lime"
          />
          <MetricCard
            label="Gates"
            value={
              totalGates > 0
                ? `${passedCount} / ${totalGates}`
                : "—"
            }
            hint="Profit + correctness + security"
            accent="sky"
          />
          <MetricCard
            label="Completion"
            value={`${(completion * 100).toFixed(0)}%`}
            hint="Finisher score"
            accent={completion >= 0.9 ? "lime" : "yellow"}
          />
          <MetricCard
            label="Gross margin"
            value={margin === null ? "—" : `${(margin * 100).toFixed(1)}%`}
            hint="revenue − provider cost"
            accent={margin !== null && margin >= 0.2 ? "lime" : "coral"}
          />
        </Box>

        <Card sx={{ p: 2 }}>
          <Stack
            direction="row"
            alignItems="center"
            justifyContent="space-between"
            sx={{ mb: 1.5 }}
          >
            <Typography
              variant="overline"
              sx={{ color: tokens.color.text.secondary }}
            >
              Gate Report
            </Typography>
            <Typography
              sx={{
                color: tokens.color.text.muted,
                fontFamily: tokens.font.mono,
                fontSize: 11,
              }}
            >
              {totalGates} stages
            </Typography>
          </Stack>
          {totalGates === 0 ? (
            <Typography sx={{ color: tokens.color.text.muted, fontSize: 13 }}>
              No gate verdicts yet. The first verdict will land as soon as the
              orchestrator emits one.
            </Typography>
          ) : (
            <Stack divider={<Divider sx={{ borderColor: tokens.color.border.subtle }} />}>
              {gates.map((g) => (
                <Stack
                  key={`${g.name}_${g.status}`}
                  direction="row"
                  alignItems="center"
                  spacing={1}
                  sx={{ py: 0.75 }}
                >
                  <Typography
                    sx={{
                      color: tokens.color.text.primary,
                      flex: 1,
                      fontFamily: tokens.font.mono,
                      fontSize: 12.5,
                    }}
                  >
                    {g.name}
                  </Typography>
                  <Typography
                    sx={{
                      color: tokens.color.text.muted,
                      fontFamily: tokens.font.mono,
                      fontSize: 11,
                    }}
                  >
                    {g.issuesCount} issue{g.issuesCount === 1 ? "" : "s"}
                  </Typography>
                  <StatusBadge status={g.status} />
                </Stack>
              ))}
            </Stack>
          )}
        </Card>

        <Box
          sx={{
            display: "grid",
            gap: 1.5,
            gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
          }}
        >
          <Card sx={{ p: 2 }}>
            <Typography
              variant="overline"
              sx={{ color: tokens.color.text.secondary, mb: 1, display: "block" }}
            >
              Profit summary
            </Typography>
            <Stack spacing={1.25}>
              <Row label="Revenue" value={bundle?.costReport.revenueUSD ?? execution.revenueUSD} positive />
              <Row label="Provider cost" value={-(bundle?.costReport.providerCostUSD ?? execution.providerCostUSD)} negative />
              <Row label="Sandbox cost" value={-(bundle?.costReport.sandboxCostUSD ?? execution.sandboxCostUSD)} negative />
              <Row label="Storage cost" value={-(bundle?.costReport.storageCostUSD ?? execution.storageCostUSD)} negative />
              <Row label="Deployment cost" value={-(bundle?.costReport.deploymentCostUSD ?? execution.deploymentCostUSD)} negative />
              <Divider sx={{ borderColor: tokens.color.border.subtle }} />
              <Stack direction="row" alignItems="center">
                <Typography sx={{ color: tokens.color.text.primary, flex: 1, fontWeight: 700 }}>
                  Margin
                </Typography>
                <Typography
                  sx={{
                    color:
                      margin !== null && margin >= 0
                        ? tokens.color.accent.success
                        : tokens.color.accent.danger,
                    fontFamily: tokens.font.mono,
                    fontWeight: 800,
                  }}
                >
                  {margin === null ? "—" : `${(margin * 100).toFixed(1)}%`}
                </Typography>
              </Stack>
            </Stack>
          </Card>

          <Card sx={{ p: 2 }}>
            <Typography
              variant="overline"
              sx={{ color: tokens.color.text.secondary, mb: 1, display: "block" }}
            >
              Security
            </Typography>
            {bundle ? (
              <Stack spacing={1}>
                <Stack direction="row" alignItems="center" spacing={1}>
                  <Typography sx={{ color: tokens.color.text.muted, flex: 1, fontSize: 12 }}>
                    Pass rate
                  </Typography>
                  <Typography
                    sx={{
                      color: tokens.color.text.primary,
                      fontFamily: tokens.font.mono,
                      fontWeight: 800,
                    }}
                  >
                    {(bundle.securityReport.passRate * 100).toFixed(0)}%
                  </Typography>
                </Stack>
                <Stack direction="row" alignItems="center" spacing={1}>
                  <Typography sx={{ color: tokens.color.text.muted, flex: 1, fontSize: 12 }}>
                    Blocked deploy
                  </Typography>
                  <StatusBadge
                    status={bundle.securityReport.blockedDeploy ? "fail" : "pass"}
                  />
                </Stack>
                <Stack direction="row" alignItems="center" spacing={1}>
                  <Typography sx={{ color: tokens.color.text.muted, flex: 1, fontSize: 12 }}>
                    Findings
                  </Typography>
                  <Typography
                    sx={{
                      color: tokens.color.text.primary,
                      fontFamily: tokens.font.mono,
                      fontWeight: 800,
                    }}
                  >
                    {bundle.securityReport.findings.length}
                  </Typography>
                </Stack>
              </Stack>
            ) : (
              <Typography sx={{ color: tokens.color.text.muted, fontSize: 13 }}>
                Security report not generated yet.
              </Typography>
            )}
          </Card>
        </Box>

        <Card sx={{ p: 2 }}>
          <Typography
            variant="overline"
            sx={{ color: tokens.color.text.secondary, mb: 1.5, display: "block" }}
          >
            Timeline
          </Typography>
          {timeline.length === 0 ? (
            <Typography sx={{ color: tokens.color.text.muted, fontSize: 13 }}>
              No execution events yet.
            </Typography>
          ) : (
            <Stack spacing={0.6}>
              {timeline.map((m) => (
                <Stack
                  key={`tl_${m.id}`}
                  direction="row"
                  spacing={1.5}
                  sx={{ alignItems: "baseline" }}
                >
                  <Typography
                    sx={{
                      color: tokens.color.text.muted,
                      fontFamily: tokens.font.mono,
                      fontSize: 10.5,
                      letterSpacing: 0.2,
                      minWidth: 78,
                    }}
                  >
                    {relativeTime(m.createdAt)}
                  </Typography>
                  <Typography
                    sx={{
                      color:
                        m.role === "costtick"
                          ? tokens.color.accent.violet
                          : tokens.color.text.secondary,
                      flex: 1,
                      fontSize: 12.5,
                      lineHeight: 1.5,
                    }}
                  >
                    {m.body}
                  </Typography>
                </Stack>
              ))}
            </Stack>
          )}
        </Card>
      </Stack>
    </Box>
  );
}

function Row({
  label,
  value,
  positive,
  negative,
}: {
  label: string;
  value: number;
  positive?: boolean;
  negative?: boolean;
}) {
  return (
    <Stack direction="row" alignItems="center">
      <Typography sx={{ color: tokens.color.text.muted, flex: 1, fontSize: 12 }}>
        {label}
      </Typography>
      <MoneyChip
        amountUSD={value}
        color={positive ? "positive" : negative ? "negative" : "neutral"}
      />
    </Stack>
  );
}
