"use client";

// TopUpHistory — recent Stripe Checkout attempts. Reads walletTopUps
// directly so the parent can stay slim. Empty + loading + error states
// are handled here.

import { Box, Stack, Typography } from "@mui/material";
import { useWalletTopUpsQuery } from "../../lib/gql/__generated__";
import { formatMoney } from "../../lib/format";
import { relativeTime } from "../../lib/relativeTime";
import { tokens } from "../../theme";
import { ErrorPanel } from "../cockpit/ErrorPanel";
import { LoadingPanel } from "../cockpit/LoadingPanel";
import { StatusBadge } from "../cockpit/StatusBadge";

export function TopUpHistory() {
  const { data, loading, error, refetch } = useWalletTopUpsQuery({
    variables: { limit: 20 },
    fetchPolicy: "cache-and-network",
    pollInterval: 30000,
  });

  const items = data?.walletTopUps ?? [];

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
        Recent top-ups
      </Typography>

      {loading && items.length === 0 ? (
        <LoadingPanel minHeight={120} label="Loading top-ups" />
      ) : error ? (
        <ErrorPanel error={error} onRetry={() => void refetch()} title="Could not load top-ups" />
      ) : items.length === 0 ? (
        <Typography sx={{ fontSize: 13, color: tokens.color.text.muted, py: 2 }}>
          No top-ups yet. Add your first credits to get started.
        </Typography>
      ) : (
        <Stack divider={<Box sx={{ borderTop: `1px solid ${tokens.color.border.subtle}` }} />}>
          {items.map((row) => (
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
                  fontSize: 14,
                  color: tokens.color.text.primary,
                  minWidth: 72,
                }}
              >
                {formatMoney(row.amountUSD)}
              </Typography>
              <StatusBadge status={row.status} />
              <Box sx={{ flex: 1 }} />
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 11,
                  color: tokens.color.text.muted,
                  letterSpacing: 0.4,
                }}
              >
                {relativeTime(row.completedAt || row.createdAt)}
              </Typography>
            </Stack>
          ))}
        </Stack>
      )}
    </Box>
  );
}
