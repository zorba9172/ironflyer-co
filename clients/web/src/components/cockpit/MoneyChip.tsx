// MoneyChip — compact $X.YZ pill used in tables, headers, feed events.
// (Pure presentation; renders as an RSC.)
// Defaults to neutral; pass color="positive" / "negative" / "warning"
// to signal margin / spend / refund context.

import { Chip, type SxProps, type Theme } from "@mui/material";
import { formatMoney, formatMoneyCompact } from "../../lib/format";
import { tokens } from "../../theme";

export type MoneyChipColor =
  | "neutral"
  | "positive"
  | "negative"
  | "warning"
  | "accent";

export interface MoneyChipProps {
  amountUSD: number | string | null | undefined;
  color?: MoneyChipColor;
  compact?: boolean;
  // Optional currency-prefix sign override; defaults to + for positive
  // colours, - for negative, none otherwise.
  showSign?: boolean;
  sx?: SxProps<Theme>;
}

const PALETTE: Record<MoneyChipColor, { bg: string; fg: string; border: string }> = {
  neutral: {
    bg: tokens.color.bg.surfaceRaised,
    fg: tokens.color.text.primary,
    border: tokens.color.border.subtle,
  },
  positive: {
    bg: `${tokens.color.accent.success}22`,
    fg: tokens.color.accent.success,
    border: `${tokens.color.accent.success}55`,
  },
  negative: {
    bg: `${tokens.color.accent.danger}22`,
    fg: tokens.color.accent.danger,
    border: `${tokens.color.accent.danger}55`,
  },
  warning: {
    bg: `${tokens.color.accent.warning}22`,
    fg: tokens.color.accent.warning,
    border: `${tokens.color.accent.warning}66`,
  },
  accent: {
    // Money positive — mint (constitution forbids lime-first identity).
    bg: `${tokens.color.accent.success}1c`,
    fg: tokens.color.accent.success,
    border: `${tokens.color.accent.success}66`,
  },
};

export function MoneyChip({
  amountUSD,
  color = "neutral",
  compact,
  showSign,
  sx,
}: MoneyChipProps) {
  const p = PALETTE[color];
  const formatted = compact ? formatMoneyCompact(amountUSD) : formatMoney(amountUSD);
  const sign =
    showSign && typeof amountUSD === "number" && amountUSD > 0 && color === "positive"
      ? "+"
      : "";
  return (
    <Chip
      size="small"
      label={`${sign}${formatted}`}
      sx={{
        bgcolor: p.bg,
        color: p.fg,
        border: `1px solid ${p.border}`,
        fontFamily: tokens.font.mono,
        fontWeight: 700,
        fontSize: 12,
        height: 24,
        borderRadius: 1,
        "& .MuiChip-label": { px: 1 },
        ...sx,
      }}
    />
  );
}
