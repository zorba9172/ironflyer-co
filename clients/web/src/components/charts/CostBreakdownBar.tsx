// CostBreakdownBar — horizontal stacked bar of how an execution's
// (Pure SVG/MUI presentation — no hooks or events; renders as an RSC.
// The studio Dashboard pane wraps several of these per execution, so
// keeping them server-side trims meaningful JS off the studio bundle.)
// reserved budget is being consumed.
//
// Five segments, left → right:
//   provider   coral   (Anthropic / OpenAI / Gemini ...)
//   sandbox    sky     (Docker runtime CPU)
//   storage    amber   (S3 / R2 / artifact bytes)
//   deploy     purple  (Vercel + DNS)
//   headroom   muted   (budget − spent, dashed)
//
// Pure SVG, no canvas — drops in next to MetricCard without adding to
// the chart chunk. The segment legend underneath shows the absolute
// dollar value and percent share per category; bars under 1% of the
// budget are widened to a minimum so they stay visible.

import { Box, Stack, Typography } from "@mui/material";
import { formatMoney } from "../../lib/format";
import { tokens } from "../../theme";

export interface CostBreakdownBarProps {
  budgetUSD: number;
  providerCostUSD: number;
  sandboxCostUSD: number;
  storageCostUSD: number;
  deploymentCostUSD: number;
  // Optional total override; when omitted we sum the four cost legs.
  spentUSD?: number;
}

interface Segment {
  key: string;
  label: string;
  value: number;
  color: string;
}

// MIN_PCT — segments below this share are widened up to this percentage
// so a tiny cost line is still readable. The remaining width is shared
// proportionally among the other (larger) segments.
const MIN_PCT = 0.012;

export function CostBreakdownBar({
  budgetUSD,
  providerCostUSD,
  sandboxCostUSD,
  storageCostUSD,
  deploymentCostUSD,
  spentUSD,
}: CostBreakdownBarProps) {
  const segments: Segment[] = [
    { key: "provider", label: "Provider", value: providerCostUSD, color: tokens.color.accent.coral },
    { key: "sandbox", label: "Sandbox", value: sandboxCostUSD, color: tokens.color.accent.sky },
    { key: "storage", label: "Storage", value: storageCostUSD, color: tokens.color.brand.amber },
    { key: "deploy", label: "Deploy", value: deploymentCostUSD, color: tokens.color.accent.purple },
  ];

  const spent =
    spentUSD ??
    segments.reduce((acc, s) => acc + (Number.isFinite(s.value) ? s.value : 0), 0);
  const budget = Math.max(0, budgetUSD);
  const headroom = Math.max(0, budget - spent);
  const overBudget = spent > budget;
  const denom = Math.max(0.0001, overBudget ? spent : budget);

  // Compute display widths with min-floor for non-zero segments so a
  // tiny cost is still visible. Headroom is rendered separately as a
  // dashed remainder so it never collides with the min-floor logic.
  const visible = segments.filter((s) => s.value > 0);
  const rawPcts = visible.map((s) => s.value / denom);
  const floored = rawPcts.map((p) => (p > 0 ? Math.max(p, MIN_PCT) : 0));
  const totalFloored = floored.reduce((a, b) => a + b, 0);
  // Renormalize so floored segments sum to the proportion of the bar
  // they should occupy (spent/denom) — never exceed it.
  const target = Math.min(1, spent / denom);
  const scale = totalFloored > 0 ? target / totalFloored : 0;
  const widths = floored.map((f) => f * scale);

  return (
    <Box>
      <Stack
        direction="row"
        alignItems="baseline"
        justifyContent="space-between"
        sx={{ mb: 0.75 }}
      >
        <Typography
          variant="overline"
          sx={{ color: tokens.color.text.secondary, letterSpacing: 1.2 }}
        >
          Spend vs budget
        </Typography>
        <Stack direction="row" spacing={1.5} alignItems="baseline">
          <Typography
            sx={{
              color: tokens.color.text.primary,
              fontFamily: tokens.font.mono,
              fontSize: 14,
              fontWeight: 800,
            }}
          >
            {formatMoney(spent)}
          </Typography>
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 11.5,
            }}
          >
            of {formatMoney(budget)} ·{" "}
            <Box
              component="span"
              sx={{
                color: overBudget
                  ? tokens.color.accent.danger
                  : budget > 0 && spent / budget >= 0.85
                    ? tokens.color.brand.amber
                    : tokens.color.accent.success,
                fontWeight: 700,
              }}
            >
              {budget > 0 ? `${((spent / budget) * 100).toFixed(0)}%` : "—"}
            </Box>
          </Typography>
        </Stack>
      </Stack>

      <Box
        sx={{
          position: "relative",
          height: 14,
          width: "100%",
          borderRadius: 999,
          overflow: "hidden",
          bgcolor: tokens.color.bg.inset,
          border: `1px solid ${tokens.color.border.subtle}`,
        }}
        role="img"
        aria-label="Cost breakdown over reserved budget"
      >
        <Stack direction="row" sx={{ height: "100%", width: "100%" }}>
          {visible.map((s, i) => (
            <Box
              key={s.key}
              sx={{
                width: `${(widths[i] * 100).toFixed(3)}%`,
                bgcolor: s.color,
                borderRight:
                  i < visible.length - 1
                    ? `1px solid ${tokens.color.bg.inset}`
                    : undefined,
                transition: "width 240ms ease-out",
              }}
            />
          ))}
          {!overBudget && headroom > 0 && (
            <Box
              sx={{
                flex: 1,
                backgroundImage: `repeating-linear-gradient(45deg, ${tokens.color.border.subtle} 0 2px, transparent 2px 6px)`,
                opacity: 0.55,
              }}
            />
          )}
        </Stack>
      </Box>

      <Stack
        direction="row"
        flexWrap="wrap"
        spacing={1.5}
        useFlexGap
        sx={{ mt: 1 }}
      >
        {segments.map((s) => {
          const pct = denom > 0 ? (s.value / denom) * 100 : 0;
          return (
            <Stack
              key={s.key}
              direction="row"
              alignItems="center"
              spacing={0.75}
            >
              <Box
                sx={{
                  width: 8,
                  height: 8,
                  borderRadius: 999,
                  bgcolor: s.color,
                  // Dim the legend chip when this category has zero spend.
                  opacity: s.value > 0 ? 1 : 0.35,
                }}
              />
              <Typography
                sx={{
                  color:
                    s.value > 0 ? tokens.color.text.secondary : tokens.color.text.muted,
                  fontSize: 11.5,
                  fontFamily: tokens.font.mono,
                }}
              >
                {s.label} {formatMoney(s.value)}{" "}
                <Box component="span" sx={{ color: tokens.color.text.muted }}>
                  · {pct.toFixed(pct < 1 && pct > 0 ? 2 : 0)}%
                </Box>
              </Typography>
            </Stack>
          );
        })}
        {!overBudget && headroom > 0 && (
          <Stack direction="row" alignItems="center" spacing={0.75}>
            <Box
              sx={{
                width: 8,
                height: 8,
                borderRadius: 999,
                border: `1px dashed ${tokens.color.border.strong}`,
              }}
            />
            <Typography
              sx={{
                color: tokens.color.text.muted,
                fontSize: 11.5,
                fontFamily: tokens.font.mono,
              }}
            >
              Headroom {formatMoney(headroom)}
            </Typography>
          </Stack>
        )}
        {overBudget && (
          <Typography
            sx={{
              color: tokens.color.accent.danger,
              fontFamily: tokens.font.mono,
              fontSize: 11.5,
              fontWeight: 700,
            }}
          >
            Over budget by {formatMoney(spent - budget)}
          </Typography>
        )}
      </Stack>
    </Box>
  );
}
