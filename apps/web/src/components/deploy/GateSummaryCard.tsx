"use client";

// GateSummaryCard — collapsible JSON dump of the deploy.gateSummary
// blob. Default rendering shows a one-line summary; the operator can
// expand to inspect the full payload.

import { ExpandLessRounded, ExpandMoreRounded } from "@mui/icons-material";
import { Box, Button, Card, Stack, Typography } from "@mui/material";
import { useState } from "react";
import { tokens } from "../../theme";

export interface GateSummaryCardProps {
  summary: unknown;
}

export function GateSummaryCard({ summary }: GateSummaryCardProps) {
  const [open, setOpen] = useState(false);
  const oneline = summariseLine(summary);
  return (
    <Card sx={{ p: 2 }}>
      <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mb: 1 }}>
        <Typography sx={{ fontWeight: 800, fontSize: 15 }}>Gate summary</Typography>
        <Button
          size="small"
          variant="text"
          onClick={() => setOpen((v) => !v)}
          endIcon={open ? <ExpandLessRounded fontSize="small" /> : <ExpandMoreRounded fontSize="small" />}
          sx={{ color: tokens.color.text.secondary, fontFamily: tokens.font.mono }}
        >
          {open ? "Hide" : "Show raw"}
        </Button>
      </Stack>
      <Typography
        sx={{ color: tokens.color.text.secondary, fontFamily: tokens.font.mono, fontSize: 12.5 }}
      >
        {oneline}
      </Typography>
      {open && (
        <Box
          component="pre"
          sx={{
            mt: 1.5,
            p: 1.5,
            bgcolor: tokens.color.bg.inset,
            border: `1px solid ${tokens.color.border.subtle}`,
            borderRadius: 1,
            fontFamily: tokens.font.mono,
            fontSize: 12,
            color: tokens.color.text.primary,
            maxHeight: 360,
            overflow: "auto",
            whiteSpace: "pre-wrap",
            wordBreak: "break-word",
          }}
        >
          {safeStringify(summary)}
        </Box>
      )}
    </Card>
  );
}

function summariseLine(value: unknown): string {
  if (!value || typeof value !== "object") return "No gate data.";
  const entries = Object.entries(value as Record<string, unknown>);
  if (entries.length === 0) return "No gate stages recorded.";
  const head = entries
    .slice(0, 4)
    .map(([k, v]) => {
      if (typeof v === "string" || typeof v === "number" || typeof v === "boolean") {
        return `${k}=${v}`;
      }
      if (v && typeof v === "object" && "status" in (v as Record<string, unknown>)) {
        return `${k}=${(v as { status: string }).status}`;
      }
      return `${k}=…`;
    })
    .join(" · ");
  return entries.length > 4 ? `${head} · …` : head;
}

function safeStringify(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}
