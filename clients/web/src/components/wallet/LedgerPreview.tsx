"use client";

// LedgerPreview — "Recent activity" panel on /wallet. The codegen
// surface does not yet expose a ledger query for the calling tenant, so
// we approximate the rolled-up cost ledger from the 10 most recent
// executions: each line is the execution's spent ($providerCost +
// $sandboxCost + …) at a glance. Once a ledger hook lands the parent
// can swap this in place without touching the page.

import { Box, Stack, Typography } from "@mui/material";
import Link from "next/link";
import { useMemo } from "react";
import { useExecutionsQuery } from "../../lib/gql/__generated__";
import { formatMoney } from "../../lib/format";
import { relativeTime } from "../../lib/relativeTime";
import { tokens } from "../../theme";
import { ErrorPanel } from "../cockpit/ErrorPanel";
import { LoadingPanel } from "../cockpit/LoadingPanel";

interface LedgerRow {
  id: string;
  amountUSD: number;
  label: string;
  href: string;
  createdAt: string;
}

export function LedgerPreview() {
  const { data, loading, error, refetch } = useExecutionsQuery({
    variables: { limit: 20, offset: 0 },
    fetchPolicy: "cache-and-network",
  });

  const rows = useMemo<LedgerRow[]>(() => {
    const list = data?.executions ?? [];
    return list
      .map((e) => ({
        id: e.id,
        amountUSD:
          (e.providerCostUSD || 0) +
          (e.sandboxCostUSD || 0) +
          (e.storageCostUSD || 0) +
          (e.deploymentCostUSD || 0),
        label: e.promptSummary?.trim() || `exec ${e.id.slice(0, 8)}`,
        href: `/execution/${e.id}`,
        createdAt: e.endedAt || e.startedAt || e.admittedAt || e.createdAt,
      }))
      .filter((r) => r.amountUSD > 0)
      .slice(0, 10);
  }, [data]);

  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.surface,
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 1,
        p: { xs: 2.5, md: 3 },
      }}
    >
      <Typography
        component="h2"
        sx={{
          fontSize: 14,
          fontWeight: 700,
          color: tokens.color.text.primary,
          mb: 2,
        }}
      >
        Recent activity
      </Typography>

      {loading && rows.length === 0 ? (
        <LoadingPanel minHeight={120} label="Loading activity" />
      ) : error ? (
        <ErrorPanel
          error={error}
          onRetry={() => void refetch()}
          title="Could not load activity"
        />
      ) : rows.length === 0 ? (
        <Typography sx={{ fontSize: 13, color: tokens.color.text.muted, py: 2 }}>
          No paid activity yet. Your provider cost lines will appear here.
        </Typography>
      ) : (
        <Stack
          divider={<Box sx={{ borderTop: `1px solid ${tokens.color.border.subtle}` }} />}
        >
          {rows.map((row) => (
            <Stack
              key={row.id}
              direction="row"
              alignItems="center"
              spacing={1.5}
              sx={{ py: 1.25 }}
            >
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontWeight: 700,
                  fontSize: 13.5,
                  color: tokens.color.text.primary,
                  minWidth: 72,
                }}
              >
                {formatMoney(row.amountUSD)}
              </Typography>
              <Typography
                sx={{
                  fontSize: 12,
                  color: tokens.color.text.muted,
                  whiteSpace: "nowrap",
                }}
              >
                provider cost
              </Typography>
              <Box
                component={Link}
                href={row.href}
                sx={{
                  flex: 1,
                  minWidth: 0,
                  fontFamily: tokens.font.mono,
                  fontSize: 12,
                  color: tokens.color.text.secondary,
                  textDecoration: "none",
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                  "&:hover": { color: tokens.color.accent.violet },
                }}
              >
                {row.label}
              </Box>
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 11,
                  color: tokens.color.text.muted,
                  letterSpacing: 0.4,
                  whiteSpace: "nowrap",
                }}
              >
                {relativeTime(row.createdAt)}
              </Typography>
            </Stack>
          ))}
        </Stack>
      )}
    </Box>
  );
}
