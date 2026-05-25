"use client";

// /execution/[id]/security — full-page security report. Uses the
// SecurityReportHeader chip strip + four quick metrics + the
// FindingsTable with collapsible remediation rows.

import { ArrowBackRounded } from "@mui/icons-material";
import { Box, Button, Stack } from "@mui/material";
import Link from "next/link";
import { use } from "react";
import {
  EmptyState,
  ErrorPanel,
  LoadingPanel,
  MetricCard,
  PageHeader,
  StatusBadge,
} from "../../../../src/components/cockpit";
import {
  FindingsTable,
  SecurityReportHeader,
} from "../../../../src/components/executions";
import { RequireAuth } from "../../../../src/lib/auth";
import { useExecutionSecurityReportQuery } from "../../../../src/lib/gql/__generated__";
import { formatNumber } from "../../../../src/lib/format";
import { tokens } from "../../../../src/theme";

export default function ExecutionSecurityPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  return (
    <RequireAuth>
      <SecurityInner id={id} />
    </RequireAuth>
  );
}

function SecurityInner({ id }: { id: string }) {
  const { data, loading, error, refetch } = useExecutionSecurityReportQuery({
    variables: { executionID: id },
    fetchPolicy: "cache-and-network",
    notifyOnNetworkStatusChange: true,
  });

  if (loading && !data) return <LoadingPanel label="Loading security report" />;
  if (error) {
    return (
      <ErrorPanel error={error} title="Security report unavailable" onRetry={() => void refetch()} />
    );
  }
  const r = data?.executionSecurityReport;
  if (!r) return null;

  const owaspCount = countOwasp(r.owaspCoverage);
  const findingsCount = r.findings.length;
  const overallLabel =
    findingsCount === 0 && r.secretsFound === 0
      ? "Clean"
      : `${findingsCount} finding${findingsCount === 1 ? "" : "s"}`;
  const overallTone: "success" | "warning" | "danger" =
    findingsCount === 0 && r.secretsFound === 0
      ? "success"
      : r.blockedDeploy
        ? "danger"
        : "warning";

  return (
    <Box>
      <PageHeader
        eyebrow="Security report"
        title={`Findings for ${id.slice(0, 10)}`}
        breadcrumbs={[
          { label: "Executions", href: "/executions" },
          { label: id.slice(0, 10), href: `/execution/${id}` },
          { label: "Security" },
        ]}
        actions={
          <Stack direction="row" spacing={1} alignItems="center">
            <StatusBadge status={overallLabel} tone={overallTone} uppercase />
            <Button
              component={Link}
              href={`/execution/${id}`}
              variant="outlined"
              startIcon={<ArrowBackRounded fontSize="small" />}
              sx={{
                borderColor: tokens.color.border.strong,
                color: tokens.color.text.primary,
              }}
            >
              Back to execution
            </Button>
          </Stack>
        }
      />
      <Stack spacing={2}>
        <SecurityReportHeader report={r} />
        <Box
          sx={{
            display: "grid",
            gap: 1.5,
            gridTemplateColumns: { xs: "1fr 1fr", md: "repeat(4, 1fr)" },
          }}
        >
          <MetricCard
            accent="coral"
            label="Total findings"
            value={formatNumber(r.findings.length)}
          />
          <MetricCard
            accent={r.secretsFound > 0 ? "coral" : "lime"}
            label="Secrets found"
            value={formatNumber(r.secretsFound)}
          />
          <MetricCard
            accent={r.outdatedDeps > 0 ? "yellow" : "lime"}
            label="Outdated deps"
            value={formatNumber(r.outdatedDeps)}
          />
          <MetricCard
            accent="sky"
            label="OWASP categories"
            value={formatNumber(owaspCount)}
            hint="covered by checks"
          />
        </Box>
        {findingsCount === 0 ? (
          <EmptyState
            title="Clean run — no security findings"
            body="No secrets, no high-severity issues, no blocking deps. The orchestrator scanned the patch set and gave it a green light."
            cta={{ label: "Back to execution", href: `/execution/${id}` }}
          />
        ) : (
          <FindingsTable findings={r.findings} />
        )}
      </Stack>
    </Box>
  );
}

function countOwasp(value: unknown): number {
  if (!value || typeof value !== "object") return 0;
  if (Array.isArray(value)) return value.length;
  return Object.keys(value as Record<string, unknown>).length;
}
