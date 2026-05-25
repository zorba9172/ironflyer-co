"use client";

// BlueprintDashboard — table of per-blueprint economic stats so the
// operator can see which blueprints earn / lose money at scale.

import { Box, Card, Stack, Typography } from "@mui/material";
import {
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
} from "@mui/material";
import { useBlueprintDashboardQuery } from "../../lib/gql/__generated__";
import { formatMoney, formatNumber, formatPercent } from "../../lib/format";
import { tokens } from "../../theme";
import { ErrorPanel, LoadingPanel } from "../cockpit";

const headSx = {
  position: "sticky",
  top: 0,
  bgcolor: tokens.color.bg.surface,
  color: tokens.color.text.muted,
  fontFamily: tokens.font.mono,
  fontSize: 11,
  fontWeight: 700,
  letterSpacing: 0.8,
  textTransform: "uppercase",
  borderBottom: `1px solid ${tokens.color.border.subtle}`,
  whiteSpace: "nowrap" as const,
};

const cellSx = {
  color: tokens.color.text.primary,
  fontSize: 13,
  fontFamily: tokens.font.mono,
  whiteSpace: "nowrap" as const,
  borderBottom: `1px solid ${tokens.color.border.subtle}`,
};

export function BlueprintDashboard() {
  const { data, loading, error, refetch } = useBlueprintDashboardQuery({
    fetchPolicy: "cache-and-network",
  });

  if (loading && !data) {
    return (
      <Card sx={{ p: 0 }}>
        <LoadingPanel label="Loading blueprints" minHeight={260} />
      </Card>
    );
  }
  if (error) {
    return <ErrorPanel error={error} title="Blueprint dashboard unavailable" onRetry={() => void refetch()} />;
  }
  const rows = [...(data?.blueprintDashboard.blueprints ?? [])].sort(
    (a, b) => b.executions - a.executions,
  );

  return (
    <Card sx={{ p: 0 }}>
      <Stack
        direction="row"
        alignItems="baseline"
        justifyContent="space-between"
        sx={{ px: 2.5, pt: 2.5, pb: 1.5 }}
      >
        <Typography
          sx={{
            fontWeight: 800,
            fontSize: 18,
            letterSpacing: -0.2,
            color: tokens.color.text.primary,
          }}
        >
          Blueprints
        </Typography>
        <Typography
          variant="overline"
          sx={{ color: tokens.color.text.muted, letterSpacing: 1.2 }}
        >
          all-time
        </Typography>
      </Stack>
      <Box sx={{ maxHeight: 420, overflow: "auto" }}>
        <TableContainer>
          <Table size="small" stickyHeader>
            <TableHead>
              <TableRow>
                <TableCell sx={headSx}>Blueprint</TableCell>
                <TableCell sx={headSx} align="right">Execs</TableCell>
                <TableCell sx={headSx} align="right">Avg revenue</TableCell>
                <TableCell sx={headSx} align="right">Avg cost</TableCell>
                <TableCell sx={headSx} align="right">Margin</TableCell>
                <TableCell sx={headSx} align="right">Preview ok</TableCell>
                <TableCell sx={headSx} align="right">Refunds</TableCell>
                <TableCell sx={headSx} align="right">Repairs</TableCell>
                <TableCell sx={headSx} align="right">Completion</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {rows.length === 0 && (
                <TableRow>
                  <TableCell colSpan={9} align="center" sx={{ ...cellSx, py: 4, color: tokens.color.text.muted }}>
                    No blueprint runs recorded yet.
                  </TableCell>
                </TableRow>
              )}
              {rows.map((b) => (
                <TableRow key={b.blueprintID}>
                  <TableCell sx={cellSx}>{b.blueprintID}</TableCell>
                  <TableCell sx={cellSx} align="right">{formatNumber(b.executions)}</TableCell>
                  <TableCell sx={cellSx} align="right">{formatMoney(b.avgRevenueUSD)}</TableCell>
                  <TableCell sx={cellSx} align="right">{formatMoney(b.avgCostUSD)}</TableCell>
                  <TableCell
                    sx={{
                      ...cellSx,
                      color:
                        b.grossMarginPct >= 0
                          ? tokens.color.accent.success
                          : tokens.color.accent.danger,
                      fontWeight: 700,
                    }}
                    align="right"
                  >
                    {formatPercent(b.grossMarginPct)}
                  </TableCell>
                  <TableCell sx={cellSx} align="right">{formatNumber(b.previewSuccess)}</TableCell>
                  <TableCell sx={cellSx} align="right">{formatNumber(b.refunds)}</TableCell>
                  <TableCell sx={cellSx} align="right">{formatNumber(b.repairCount)}</TableCell>
                  <TableCell sx={cellSx} align="right">{b.avgCompletionScore.toFixed(2)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      </Box>
    </Card>
  );
}
