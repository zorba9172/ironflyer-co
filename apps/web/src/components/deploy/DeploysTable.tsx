"use client";

// DeploysTable — virtualized list of deploys for /deploy. Status pill +
// target/environment columns + gate-summary chip strip + per-row link.
// The cockpit's VirtualTable owns the chrome and the row-as-link wiring.

import { Chip, Stack, TableCell, Typography } from "@mui/material";
import { VirtualTable, type VirtualTableColumn } from "../cockpit";
import { StatusBadge } from "../cockpit/StatusBadge";
import { tokens } from "../../theme";
import type { DeploysQuery } from "../../lib/gql/__generated__";
import { formatMoney } from "../../lib/format";
import { relativeTime } from "../../lib/relativeTime";

export type DeployRow = DeploysQuery["deploys"][number];

const cellSx = {
  color: tokens.color.text.primary,
  fontSize: 13,
};

const COLUMNS: VirtualTableColumn[] = [
  { key: "id", label: "Deploy", width: 110 },
  { key: "project", label: "Project", width: 110 },
  { key: "target", label: "Target", width: 110 },
  { key: "env", label: "Env", width: 100 },
  { key: "status", label: "Status", width: 120 },
  { key: "gates", label: "Gates" },
  { key: "cost", label: "Cost", align: "right", width: 100 },
  { key: "created", label: "Created", width: 130 },
  { key: "action", label: "Action", align: "right", width: 80 },
];

export interface DeploysTableProps {
  rows: DeployRow[];
}

export function DeploysTable({ rows }: DeploysTableProps) {
  return (
    <VirtualTable<DeployRow>
      rows={rows}
      columns={COLUMNS}
      rowKey={(d) => d.id}
      rowHref={(d) => `/deploy/${d.id}`}
      estimatedRowHeight={46}
      emptyLabel="No deploys match this filter."
      renderRow={(d) => (
        <>
          <TableCell sx={{ ...cellSx, fontFamily: tokens.font.mono, color: tokens.color.text.secondary }}>
            {shortId(d.id)}
          </TableCell>
          <TableCell sx={{ ...cellSx, fontFamily: tokens.font.mono, color: tokens.color.text.secondary }}>
            {shortId(d.projectID)}
          </TableCell>
          <TableCell sx={cellSx}>{d.target}</TableCell>
          <TableCell sx={cellSx}>{d.environment}</TableCell>
          <TableCell sx={cellSx}>
            <StatusBadge status={d.status} />
          </TableCell>
          <TableCell sx={cellSx}>
            <GateSummaryChips summary={d.gateSummary} />
          </TableCell>
          <TableCell sx={{ ...cellSx, fontFamily: tokens.font.mono }} align="right">
            {formatMoney(d.costUSD)}
          </TableCell>
          <TableCell sx={{ ...cellSx, color: tokens.color.text.secondary }}>
            {relativeTime(d.createdAt)}
          </TableCell>
          <TableCell
            align="right"
            sx={{ ...cellSx, color: tokens.color.accent.violet, fontFamily: tokens.font.mono }}
          >
            Open →
          </TableCell>
        </>
      )}
    />
  );
}

function shortId(id: string): string {
  if (id.length <= 12) return id;
  return `${id.slice(0, 8)}…`;
}

// GateSummaryChips — gateSummary is JSON; we tolerate any shape by
// extracting up to 3 string/number leaves and rendering them as a
// dense chip strip. Unknown shape → "—".
function GateSummaryChips({ summary }: { summary: unknown }) {
  const items = summarise(summary);
  if (items.length === 0) {
    return (
      <Typography sx={{ color: tokens.color.text.muted, fontSize: 12, fontFamily: tokens.font.mono }}>
        —
      </Typography>
    );
  }
  return (
    <Stack direction="row" spacing={0.5} flexWrap="wrap" sx={{ rowGap: 0.5 }}>
      {items.map((item, i) => (
        <Chip
          key={`${item.label}-${i}`}
          size="small"
          label={`${item.label}:${item.value}`}
          sx={{
            bgcolor: tokens.color.bg.surfaceRaised,
            color: tokens.color.text.secondary,
            border: `1px solid ${tokens.color.border.subtle}`,
            fontFamily: tokens.font.mono,
            fontSize: 10.5,
            height: 18,
            borderRadius: 0.5,
            "& .MuiChip-label": { px: 0.75 },
          }}
        />
      ))}
    </Stack>
  );
}

function summarise(value: unknown): Array<{ label: string; value: string }> {
  if (!value || typeof value !== "object") return [];
  const out: Array<{ label: string; value: string }> = [];
  for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
    if (out.length >= 3) break;
    if (typeof v === "string" || typeof v === "number" || typeof v === "boolean") {
      out.push({ label: k, value: String(v) });
    } else if (v && typeof v === "object") {
      const nested = v as Record<string, unknown>;
      if ("status" in nested && typeof nested.status === "string") {
        out.push({ label: k, value: nested.status });
      } else if ("count" in nested && typeof nested.count === "number") {
        out.push({ label: k, value: String(nested.count) });
      }
    }
  }
  return out;
}
