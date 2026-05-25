"use client";

// WalletBalanceCard — hero panel on /wallet. Headline available balance,
// hold chip, lifetime topped-up / spent rollup, and a relative "last
// updated" label. Pure presentation: parent owns the query.

import { Box, Stack, Typography } from "@mui/material";
import { formatMoney } from "../../lib/format";
import { relativeTime } from "../../lib/relativeTime";
import { tokens } from "../../theme";
import { MoneyChip } from "../cockpit/MoneyChip";

export interface WalletBalanceCardProps {
  availableUSD: number;
  holdUSD: number;
  lifetimeTopUpUSD: number;
  lifetimeSpendUSD: number;
  updatedAt: string;
}

export function WalletBalanceCard({
  availableUSD,
  holdUSD,
  lifetimeTopUpUSD,
  lifetimeSpendUSD,
  updatedAt,
}: WalletBalanceCardProps) {
  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.surface,
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 1,
        p: { xs: 3, md: 4 },
      }}
    >
      <Stack spacing={2.5}>
        <Stack spacing={0.5}>
          <Typography
            variant="overline"
            sx={{ color: tokens.color.text.muted, letterSpacing: 1.2 }}
          >
            Available
          </Typography>
          <Typography
            sx={{
              fontFamily: tokens.font.mono,
              fontWeight: 800,
              fontSize: { xs: 40, md: 56 },
              color: tokens.color.accent.violet,
              letterSpacing: -1.5,
              lineHeight: 1,
            }}
          >
            {formatMoney(availableUSD)}
          </Typography>
        </Stack>

        <Stack direction="row" spacing={1.5} alignItems="center" flexWrap="wrap" useFlexGap>
          {holdUSD > 0 && (
            <MoneyChip amountUSD={holdUSD} color="warning" />
          )}
          <Typography
            sx={{
              fontFamily: tokens.font.mono,
              fontSize: 12,
              color: tokens.color.text.muted,
              letterSpacing: 0.4,
            }}
          >
            {holdUSD > 0 ? "currently held against active runs" : "no active holds"}
          </Typography>
        </Stack>

        <Stack
          direction={{ xs: "column", sm: "row" }}
          spacing={{ xs: 1.5, sm: 4 }}
          sx={{
            borderTop: `1px solid ${tokens.color.border.subtle}`,
            pt: 2.5,
          }}
        >
          <Stat label="Lifetime topped up" value={formatMoney(lifetimeTopUpUSD)} />
          <Stat label="Lifetime spent" value={formatMoney(lifetimeSpendUSD)} />
        </Stack>

        <Typography
          sx={{
            fontFamily: tokens.font.mono,
            fontSize: 11,
            color: tokens.color.text.muted,
            letterSpacing: 0.4,
          }}
        >
          Last updated {relativeTime(updatedAt)}
        </Typography>
      </Stack>
    </Box>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <Stack spacing={0.5}>
      <Typography
        variant="overline"
        sx={{ color: tokens.color.text.muted, letterSpacing: 1.2 }}
      >
        {label}
      </Typography>
      <Typography
        sx={{
          fontFamily: tokens.font.mono,
          fontWeight: 700,
          fontSize: 18,
          color: tokens.color.text.primary,
        }}
      >
        {value}
      </Typography>
    </Stack>
  );
}
