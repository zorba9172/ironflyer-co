"use client";

// ExecutionsTable — list of paid executions for /executions. The
// component renders a virtualized desktop table at md+ breakpoints
// and a card list on small viewports. Row hover lifts onto
// `bg.surfaceHover` (matches the rest of the cockpit).

import { ArrowForwardRounded } from "@mui/icons-material";
import { Box, Card, Stack, TableCell, Typography } from "@mui/material";
import Link from "next/link";
import { useMemo } from "react";
import { MoneyChip } from "../cockpit/MoneyChip";
import { StatusBadge } from "../cockpit/StatusBadge";
import { VirtualTable, type VirtualTableColumn } from "../cockpit";
import type { ExecutionsQuery } from "../../lib/gql/__generated__";
import { relativeTime } from "../../lib/relativeTime";
import { tokens } from "../../theme";

export type ExecutionRow = ExecutionsQuery["executions"][number];

const cellSx = {
  color: tokens.color.text.primary,
  fontSize: 13,
};

const COLUMNS: VirtualTableColumn[] = [
  { key: "id", label: "Execution", width: 130 },
  { key: "project", label: "Project", width: 160 },
  { key: "status", label: "Status", width: 130 },
  { key: "spend", label: "Spend", align: "right", width: 130 },
  { key: "started", label: "Started", width: 140 },
  { key: "duration", label: "Duration", width: 120, align: "right" },
  { key: "action", label: "", align: "right", width: 60 },
];

export interface ExecutionsTableProps {
  rows: ExecutionRow[];
}

export function ExecutionsTable({ rows }: ExecutionsTableProps) {
  return (
    <>
      {/* Desktop / tablet — virtualised table */}
      <Box sx={{ display: { xs: "none", sm: "block" } }}>
        <VirtualTable<ExecutionRow>
          rows={rows}
          columns={COLUMNS}
          rowKey={(r) => r.id}
          rowHref={(r) => `/execution/${r.id}`}
          estimatedRowHeight={48}
          emptyLabel="No executions match this filter."
          renderRow={(r) => (
            <>
              <TableCell
                sx={{
                  ...cellSx,
                  fontFamily: tokens.font.mono,
                  color: tokens.color.text.secondary,
                  whiteSpace: "nowrap",
                }}
              >
                {shortId(r.id)}
              </TableCell>
              <TableCell
                sx={{
                  ...cellSx,
                  fontFamily: tokens.font.mono,
                  color: tokens.color.text.secondary,
                  whiteSpace: "nowrap",
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  maxWidth: 160,
                }}
              >
                {r.projectID ? shortId(r.projectID) : r.blueprintID ?? "—"}
              </TableCell>
              <TableCell sx={cellSx}>
                <StatusBadge status={r.status} />
              </TableCell>
              <TableCell sx={cellSx} align="right">
                <MoneyChip amountUSD={r.spentUSD} color={spendColor(r)} />
              </TableCell>
              <TableCell sx={{ ...cellSx, color: tokens.color.text.secondary }}>
                {relativeTime(r.startedAt ?? r.createdAt)}
              </TableCell>
              <TableCell
                sx={{
                  ...cellSx,
                  color: tokens.color.text.secondary,
                  fontFamily: tokens.font.mono,
                }}
                align="right"
              >
                {durationLabel(r)}
              </TableCell>
              <TableCell
                align="right"
                sx={{
                  ...cellSx,
                  color: tokens.color.accent.violet,
                  fontFamily: tokens.font.mono,
                }}
              >
                →
              </TableCell>
            </>
          )}
        />
      </Box>

      {/* Mobile — card list */}
      <Box sx={{ display: { xs: "block", sm: "none" } }}>
        <MobileExecutionList rows={rows} />
      </Box>
    </>
  );
}

function MobileExecutionList({ rows }: { rows: ExecutionRow[] }) {
  if (rows.length === 0) return <ExecutionsEmpty />;
  return (
    <Stack spacing={1}>
      {rows.map((r) => (
        <Card
          key={r.id}
          component={Link}
          href={`/execution/${r.id}`}
          sx={{
            p: 1.75,
            display: "block",
            textDecoration: "none",
            color: tokens.color.text.primary,
            border: `1px solid ${tokens.color.border.subtle}`,
            transition: "background-color 160ms ease, border-color 160ms ease",
            "&:hover": {
              bgcolor: tokens.color.bg.surfaceHover,
              borderColor: tokens.color.border.strong,
            },
          }}
        >
          <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 0.75 }}>
            <Typography
              sx={{
                fontFamily: tokens.font.mono,
                fontWeight: 800,
                fontSize: 15,
                letterSpacing: 0.2,
                color: tokens.color.text.primary,
              }}
            >
              {shortId(r.id)}
            </Typography>
            <Box sx={{ flex: 1 }} />
            <ArrowForwardRounded
              sx={{ fontSize: 16, color: tokens.color.accent.violet }}
            />
          </Stack>
          <Typography
            sx={{
              fontSize: 12.5,
              color: tokens.color.text.secondary,
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
              mb: 1,
            }}
          >
            {r.promptSummary || r.blueprintID || "Untitled execution"}
          </Typography>
          <Stack direction="row" spacing={0.75} sx={{ flexWrap: "wrap", rowGap: 0.75 }}>
            <StatusBadge status={r.status} />
            <MoneyChip amountUSD={r.spentUSD} color={spendColor(r)} />
            <Typography
              sx={{
                fontFamily: tokens.font.mono,
                fontSize: 11,
                color: tokens.color.text.muted,
                alignSelf: "center",
              }}
            >
              {durationLabel(r)}
            </Typography>
            <Typography
              sx={{
                fontFamily: tokens.font.mono,
                fontSize: 11,
                color: tokens.color.text.muted,
                alignSelf: "center",
              }}
            >
              · {relativeTime(r.startedAt ?? r.createdAt)}
            </Typography>
          </Stack>
        </Card>
      ))}
    </Stack>
  );
}

function spendColor(r: ExecutionRow): "neutral" | "negative" | "warning" {
  if (r.budgetUSD > 0 && r.spentUSD >= r.budgetUSD) return "negative";
  if (r.budgetUSD > 0 && r.spentUSD >= r.budgetUSD * 0.8) return "warning";
  return "neutral";
}

function durationLabel(r: ExecutionRow): string {
  const start = r.startedAt ?? r.createdAt;
  if (!start) return "—";
  const startMs = new Date(start).getTime();
  if (!Number.isFinite(startMs)) return "—";
  const endMs = r.endedAt ? new Date(r.endedAt).getTime() : Date.now();
  if (!Number.isFinite(endMs)) return "—";
  const sec = Math.max(0, Math.floor((endMs - startMs) / 1000));
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  const s = sec % 60;
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${s.toString().padStart(2, "0")}s`;
  return `${s}s`;
}

// Kept as a named export to avoid noisy diffs in callers that still
// reference it for empty-state copy.
export function ExecutionsEmpty() {
  // useMemo holds the styles stable across re-renders so the dashed
  // border doesn't repaint on every parent state change.
  const sx = useMemo(
    () => ({
      border: `1px dashed ${tokens.color.border.subtle}`,
      borderRadius: 1,
      p: 5,
      textAlign: "center" as const,
      color: tokens.color.text.muted,
      fontSize: 13.5,
    }),
    [],
  );
  return <Box sx={sx}>No executions match this filter.</Box>;
}

function shortId(id: string): string {
  if (id.length <= 12) return id;
  return `${id.slice(0, 8)}…`;
}
