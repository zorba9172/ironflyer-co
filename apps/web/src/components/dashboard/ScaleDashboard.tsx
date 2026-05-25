"use client";

// ScaleDashboard — live scale picture for the operator. Pulled from
// scaleDashboard once on mount and on the polling interval (15s) so
// the operator sees queue / utilisation shifts without a refresh.

import { Box, Card, Stack, Typography } from "@mui/material";
import { useScaleDashboardQuery } from "../../lib/gql/__generated__";
import { formatNumber, formatPercent } from "../../lib/format";
import { tokens } from "../../theme";
import { ErrorPanel, LoadingPanel, MetricCard } from "../cockpit";

export function ScaleDashboard() {
  const { data, loading, error, refetch } = useScaleDashboardQuery({
    fetchPolicy: "cache-and-network",
    pollInterval: 15000,
  });

  if (loading && !data) {
    return (
      <Card sx={{ p: 0 }}>
        <LoadingPanel label="Loading scale snapshot" minHeight={220} />
      </Card>
    );
  }
  if (error) {
    return <ErrorPanel error={error} title="Scale dashboard unavailable" onRetry={() => void refetch()} />;
  }
  const d = data?.scaleDashboard;
  if (!d) return null;

  const utilisation = clampPct(d.workerUtilizationPct);
  const health = clampPct(d.scaleHealth);
  const utilAccent = utilisation >= 90 ? "coral" : utilisation >= 70 ? "yellow" : "lime";
  const healthAccent = health >= 80 ? "lime" : health >= 50 ? "yellow" : "coral";
  const notes = collectNotes(d);

  return (
    <Card sx={{ p: 2.5 }}>
      <Stack
        direction="row"
        alignItems="baseline"
        justifyContent="space-between"
        sx={{ mb: 2 }}
      >
        <Typography
          sx={{
            fontWeight: 800,
            fontSize: 18,
            letterSpacing: -0.2,
            color: tokens.color.text.primary,
          }}
        >
          Scale
        </Typography>
        <Typography
          variant="overline"
          sx={{ color: tokens.color.text.muted, letterSpacing: 1.2 }}
        >
          live · 15s poll
        </Typography>
      </Stack>

      <Box
        sx={{
          display: "grid",
          gap: 1.5,
          gridTemplateColumns: { xs: "1fr 1fr", md: "repeat(3, 1fr)" },
        }}
      >
        <MetricCard
          accent="sky"
          label="Active executions"
          value={formatNumber(d.activeExecutions)}
        />
        <MetricCard
          accent="purple"
          label="Queued"
          value={formatNumber(d.queuedExecutions)}
          hint="Awaiting admission"
        />
        <MetricCard
          accent="neutral"
          label="Avg queue wait"
          value={`${d.queueWaitSec.toFixed(1)}s`}
        />
        <MetricCard
          accent={utilAccent}
          label="Worker utilization"
          value={formatPercent(utilisation)}
          hint="active / capacity"
        />
        <MetricCard
          accent="neutral"
          label="Sandbox capacity"
          value={formatNumber(d.sandboxCapacity)}
        />
        <MetricCard
          accent={healthAccent}
          label="Scale health"
          value={formatPercent(health)}
        />
      </Box>

      {notes.length > 0 && (
        <Box
          sx={{
            mt: 2,
            p: 1.5,
            border: `1px solid ${tokens.color.accent.warning}55`,
            bgcolor: `${tokens.color.accent.warning}10`,
            borderRadius: 1,
          }}
        >
          <Typography
            variant="overline"
            sx={{ color: tokens.color.accent.warning, letterSpacing: 1.2 }}
          >
            Degraded inputs
          </Typography>
          <Box component="ul" sx={{ pl: 2.5, m: 0, mt: 0.5 }}>
            {notes.map((n) => (
              <Typography
                key={n}
                component="li"
                sx={{ color: tokens.color.text.secondary, fontSize: 13 }}
              >
                {n}
              </Typography>
            ))}
          </Box>
        </Box>
      )}
    </Card>
  );
}

function clampPct(n: number): number {
  if (!Number.isFinite(n)) return 0;
  if (n < 0) return 0;
  if (n > 100) return 100;
  return n;
}

// collectNotes — surfaces obvious health signals so the operator sees
// the meaningful caveats without scrolling. The ScaleDashboard type
// has no `notes` field today; this is computed from the metrics we
// have, intentionally conservative.
function collectNotes(d: {
  sandboxCapacity: number;
  workerUtilizationPct: number;
  scaleHealth: number;
  queueWaitSec: number;
}): string[] {
  const out: string[] = [];
  if (d.sandboxCapacity <= 0) {
    out.push("Sandbox capacity reported as 0 — utilization is unmeasured.");
  }
  if (d.workerUtilizationPct >= 95) {
    out.push("Worker utilization above 95% — admission may stall.");
  }
  if (d.queueWaitSec > 30) {
    out.push(`Queue wait ${d.queueWaitSec.toFixed(1)}s — investigate ProfitGuard rejections.`);
  }
  if (d.scaleHealth < 50) {
    out.push("Scale health below 50% — margin protection may degrade.");
  }
  return out;
}
