// CohortTable — sticky-header table of monthly cohort rollups. Used by
// (Pure presentation; no hooks or events — renders as an RSC.)
//
// the operator dashboard. Pure presentation; the parent owns the
// query.

import { Box, Card, Stack, Typography } from "@mui/material";
import {
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
} from "@mui/material";
import type { CohortDashboardQuery } from "../../lib/gql/__generated__";
import { formatMoney, formatNumber, formatPercent } from "../../lib/format";
import { tokens } from "../../theme";

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

export interface CohortTableProps {
  cohorts: CohortDashboardQuery["cohortDashboard"]["cohorts"];
}

export function CohortTable({ cohorts }: CohortTableProps) {
  const rows = [...cohorts].sort((a, b) => (a.month < b.month ? 1 : -1));
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
          Cohorts
        </Typography>
        <Typography
          variant="overline"
          sx={{ color: tokens.color.text.muted, letterSpacing: 1.2 }}
        >
          last 6 months
        </Typography>
      </Stack>
      <Box sx={{ maxHeight: 420, overflow: "auto" }}>
        <TableContainer>
          <Table size="small" stickyHeader>
            <TableHead>
              <TableRow>
                <TableCell sx={headSx}>Month</TableCell>
                <TableCell sx={headSx} align="right">New paying</TableCell>
                <TableCell sx={headSx} align="right">2nd exec</TableCell>
                <TableCell sx={headSx} align="right">D7 repeat</TableCell>
                <TableCell sx={headSx} align="right">D30 repeat</TableCell>
                <TableCell sx={headSx} align="right">Avg spend</TableCell>
                <TableCell sx={headSx} align="right">Gross margin</TableCell>
                <TableCell sx={headSx} align="right">Completion</TableCell>
                <TableCell sx={headSx} align="right">Refund rate</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {rows.length === 0 && (
                <TableRow>
                  <TableCell colSpan={9} align="center" sx={{ ...cellSx, py: 4, color: tokens.color.text.muted }}>
                    No cohort data in window.
                  </TableCell>
                </TableRow>
              )}
              {rows.map((r) => (
                <TableRow key={r.month}>
                  <TableCell sx={cellSx}>{formatCohortMonth(r.month)}</TableCell>
                  <TableCell sx={cellSx} align="right">{formatNumber(r.newPayingUsers)}</TableCell>
                  <TableCell sx={cellSx} align="right">{formatNumber(r.secondExecutionUsers)}</TableCell>
                  <TableCell sx={cellSx} align="right">{formatNumber(r.day7RepeatUsers)}</TableCell>
                  <TableCell sx={cellSx} align="right">{formatNumber(r.day30RepeatUsers)}</TableCell>
                  <TableCell sx={cellSx} align="right">{formatMoney(r.avgSpendUSD)}</TableCell>
                  <TableCell sx={cellSx} align="right">{formatPercent(r.grossMarginPct)}</TableCell>
                  <TableCell sx={cellSx} align="right">{formatPercent(r.completionRate, "fraction")}</TableCell>
                  <TableCell sx={cellSx} align="right">{formatPercent(r.refundRate, "fraction")}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      </Box>
    </Card>
  );
}

function formatCohortMonth(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleDateString(undefined, { year: "numeric", month: "short" });
}
