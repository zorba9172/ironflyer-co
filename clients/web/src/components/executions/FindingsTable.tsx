"use client";

// FindingsTable — severity-coloured table of security findings with a
// collapsible remediation row. The table is the centrepiece of the
// /execution/[id]/security page.

import { ExpandLessRounded, ExpandMoreRounded } from "@mui/icons-material";
import {
  Box,
  Chip,
  IconButton,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@mui/material";
import { Fragment, useMemo, useState } from "react";
import type { ExecutionSecurityReportQuery } from "../../lib/gql/__generated__";
import { tokens } from "../../theme";

type Finding = ExecutionSecurityReportQuery["executionSecurityReport"]["findings"][number];

const SEVERITY_RANK: Record<string, number> = {
  critical: 0,
  high: 1,
  medium: 2,
  low: 3,
  info: 4,
};

const SEVERITY_COLOR: Record<string, { bg: string; fg: string; border: string }> = {
  critical: {
    bg: `${tokens.color.accent.danger}1f`,
    fg: tokens.color.accent.danger,
    border: `${tokens.color.accent.danger}66`,
  },
  high: {
    bg: `${tokens.color.accent.danger}14`,
    fg: tokens.color.accent.danger,
    border: `${tokens.color.accent.danger}55`,
  },
  medium: {
    bg: `${tokens.color.accent.warning}1f`,
    fg: tokens.color.accent.warning,
    border: `${tokens.color.accent.warning}66`,
  },
  low: {
    bg: `${tokens.color.accent.sky}1c`,
    fg: tokens.color.accent.sky,
    border: `${tokens.color.accent.sky}55`,
  },
  info: {
    bg: tokens.color.bg.surfaceRaised,
    fg: tokens.color.text.secondary,
    border: tokens.color.border.subtle,
  },
};

const headSx = {
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
  borderBottom: `1px solid ${tokens.color.border.subtle}`,
  verticalAlign: "top" as const,
};

export interface FindingsTableProps {
  findings: Finding[];
}

export function FindingsTable({ findings }: FindingsTableProps) {
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  // Memoised — expand/collapse toggles setExpanded, which would
  // otherwise re-sort every finding on every click.
  const sorted = useMemo(() => {
    return [...findings].sort((a, b) => {
      const ra = SEVERITY_RANK[a.severity.toLowerCase()] ?? 9;
      const rb = SEVERITY_RANK[b.severity.toLowerCase()] ?? 9;
      if (ra !== rb) return ra - rb;
      return a.path.localeCompare(b.path);
    });
  }, [findings]);

  if (sorted.length === 0) {
    return (
      <Box
        sx={{
          border: `1px dashed ${tokens.color.border.subtle}`,
          borderRadius: 1,
          p: 4,
          textAlign: "center",
          color: tokens.color.text.muted,
          fontFamily: tokens.font.mono,
          fontSize: 13,
        }}
      >
        No security findings — clean run.
      </Box>
    );
  }

  return (
    <TableContainer
      sx={{
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 1,
        bgcolor: tokens.color.bg.surface,
      }}
    >
      <Table size="small">
        <TableHead>
          <TableRow>
            <TableCell sx={{ ...headSx, width: 28 }} />
            <TableCell sx={headSx}>Severity</TableCell>
            <TableCell sx={headSx}>Rule</TableCell>
            <TableCell sx={headSx}>Category</TableCell>
            <TableCell sx={headSx}>Path</TableCell>
            <TableCell sx={headSx} align="right">Line</TableCell>
            <TableCell sx={headSx}>Summary</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {sorted.map((f) => {
            const sev = f.severity.toLowerCase();
            const styles = SEVERITY_COLOR[sev] ?? SEVERITY_COLOR.info;
            const open = !!expanded[f.id];
            return (
              <Fragment key={f.id}>
                <TableRow hover>
                  <TableCell sx={cellSx}>
                    <IconButton
                      size="small"
                      onClick={() => setExpanded((m) => ({ ...m, [f.id]: !m[f.id] }))}
                      aria-label={open ? "Hide remediation" : "Show remediation"}
                      sx={{ color: tokens.color.text.secondary }}
                    >
                      {open ? <ExpandLessRounded fontSize="small" /> : <ExpandMoreRounded fontSize="small" />}
                    </IconButton>
                  </TableCell>
                  <TableCell sx={cellSx}>
                    <Chip
                      size="small"
                      label={sev.toUpperCase()}
                      sx={{
                        bgcolor: styles.bg,
                        color: styles.fg,
                        border: `1px solid ${styles.border}`,
                        fontFamily: tokens.font.mono,
                        fontWeight: 700,
                        fontSize: 10.5,
                        letterSpacing: 0.8,
                        height: 20,
                        borderRadius: 0.75,
                        "& .MuiChip-label": { px: 1 },
                      }}
                    />
                  </TableCell>
                  <TableCell sx={{ ...cellSx, fontFamily: tokens.font.mono, color: tokens.color.text.secondary }}>
                    {f.ruleID}
                  </TableCell>
                  <TableCell sx={cellSx}>{f.category}</TableCell>
                  <TableCell sx={{ ...cellSx, fontFamily: tokens.font.mono }}>{f.path}</TableCell>
                  <TableCell sx={{ ...cellSx, fontFamily: tokens.font.mono }} align="right">
                    {f.line > 0 ? f.line : "—"}
                  </TableCell>
                  <TableCell sx={cellSx}>{f.summary}</TableCell>
                </TableRow>
                {open && (
                  <TableRow>
                    <TableCell />
                    <TableCell
                      colSpan={6}
                      sx={{
                        ...cellSx,
                        bgcolor: tokens.color.bg.inset,
                      }}
                    >
                      <Stack spacing={0.5}>
                        <Typography
                          variant="overline"
                          sx={{ color: tokens.color.text.muted, letterSpacing: 1.2 }}
                        >
                          Remediation
                        </Typography>
                        <Typography
                          sx={{
                            color: tokens.color.text.primary,
                            fontFamily: tokens.font.mono,
                            fontSize: 12.5,
                            whiteSpace: "pre-wrap",
                          }}
                        >
                          {f.remediation || "No remediation guidance provided."}
                        </Typography>
                      </Stack>
                    </TableCell>
                  </TableRow>
                )}
              </Fragment>
            );
          })}
        </TableBody>
      </Table>
    </TableContainer>
  );
}
