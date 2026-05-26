// ComparisonTable — generic columns-and-rows table used by /pricing
// for the itemized rate sheet and by /enterprise for the deployment
// options matrix. Server component; cell content is plain ReactNode.

import { Box, Typography } from "@mui/material";
import type { ReactNode } from "react";
import { tokens } from "../../theme";

export interface ComparisonColumn {
  key: string;
  label: string;
  highlight?: boolean;
  width?: string;
}

export interface ComparisonRow {
  key: string;
  cells: Record<string, ReactNode>;
}

export interface ComparisonTableProps {
  columns: ComparisonColumn[];
  rows: ComparisonRow[];
  caption?: string;
}

export function ComparisonTable({
  columns,
  rows,
  caption,
}: ComparisonTableProps) {
  const gridTemplateColumns = columns
    .map((c) => c.width ?? "minmax(0, 1fr)")
    .join(" ");

  return (
    <Box
      sx={{
        borderRadius: `${tokens.radius.md}px`,
        border: `1px solid ${tokens.color.border.subtle}`,
        bgcolor: `${tokens.color.bg.surface}d9`,
        overflow: "hidden",
      }}
    >
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: { xs: "1fr", md: gridTemplateColumns },
          alignItems: "center",
          px: { xs: 2.4, md: 3 },
          py: 1.8,
          bgcolor: `${tokens.color.bg.inset}b3`,
          borderBottom: `1px solid ${tokens.color.border.subtle}`,
        }}
      >
        {columns.map((c) => (
          <Typography
            key={c.key}
            sx={{
              display: { xs: "none", md: "block" },
              fontFamily: tokens.font.mono,
              fontSize: 11,
              letterSpacing: 1.2,
              textTransform: "uppercase",
              fontWeight: 800,
              color: c.highlight
                ? tokens.color.accent.violet
                : tokens.color.text.muted,
            }}
          >
            {c.label}
          </Typography>
        ))}
      </Box>
      {rows.map((row, idx) => (
        <Box
          key={row.key}
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: gridTemplateColumns },
            alignItems: { xs: "flex-start", md: "center" },
            gap: { xs: 0.6, md: 0 },
            px: { xs: 2.4, md: 3 },
            py: { xs: 2, md: 2.2 },
            borderBottom:
              idx < rows.length - 1
                ? `1px solid ${tokens.color.border.subtle}`
                : "none",
          }}
        >
          {columns.map((c) => (
            <Box
              key={c.key}
              sx={{
                minWidth: 0,
                display: { xs: "flex", md: "block" },
                gap: 1.2,
                alignItems: "baseline",
              }}
            >
              <Typography
                sx={{
                  display: { xs: "inline", md: "none" },
                  fontFamily: tokens.font.mono,
                  fontSize: 10.5,
                  letterSpacing: 0.8,
                  textTransform: "uppercase",
                  color: tokens.color.text.muted,
                  minWidth: 110,
                  fontWeight: 700,
                }}
              >
                {c.label}
              </Typography>
              <Box
                sx={{
                  fontSize: 14,
                  color: c.highlight
                    ? tokens.color.text.primary
                    : tokens.color.text.secondary,
                  fontWeight: c.highlight ? 700 : 500,
                  minWidth: 0,
                }}
              >
                {row.cells[c.key]}
              </Box>
            </Box>
          ))}
        </Box>
      ))}
      {caption && (
        <Box
          sx={{
            px: { xs: 2.4, md: 3 },
            py: 1.4,
            bgcolor: `${tokens.color.bg.inset}b3`,
            borderTop: `1px solid ${tokens.color.border.subtle}`,
          }}
        >
          <Typography
            sx={{
              fontFamily: tokens.font.mono,
              fontSize: 11,
              color: tokens.color.text.muted,
            }}
          >
            {caption}
          </Typography>
        </Box>
      )}
    </Box>
  );
}
