"use client";

// EChart — thin wrapper around the tree-shaken echarts/core build.
//
// Only the chart types, components, and renderer we actually use are
// pulled in. The full echarts UMD bundle is ~1MB — this stays under
// ~200KB gzip even with multiple chart kinds on the page.
//
// The wrapper:
//   - owns the canvas DOM + ResizeObserver,
//   - re-applies setOption() on prop change without disposing,
//   - disposes the instance on unmount,
//   - keeps the dark theme + token palette out of the per-chart code.
//
// Call sites should lazy-load this component via next/dynamic so the
// echarts chunk is fetched only on pages that render a chart.

import { Box } from "@mui/material";
import { useEffect, useRef } from "react";

import * as echarts from "echarts/core";
import { BarChart, LineChart } from "echarts/charts";
import {
  GridComponent,
  TooltipComponent,
  LegendComponent,
  MarkLineComponent,
  MarkAreaComponent,
} from "echarts/components";
import { CanvasRenderer } from "echarts/renderers";
import type { EChartsCoreOption } from "echarts/core";

import { tokens } from "../../theme";

let registered = false;
function ensureRegistered() {
  if (registered) return;
  echarts.use([
    BarChart,
    LineChart,
    GridComponent,
    TooltipComponent,
    LegendComponent,
    MarkLineComponent,
    MarkAreaComponent,
    CanvasRenderer,
  ]);
  registered = true;
}

export interface EChartProps {
  option: EChartsCoreOption;
  height?: number | string;
  // notMerge=true on option swap. Default false (deep-merged), which
  // is correct for incremental data updates. Pass true when the option
  // shape itself changes (e.g. switching chart kinds).
  notMerge?: boolean;
  ariaLabel?: string;
}

export function EChart({
  option,
  height = 220,
  notMerge = false,
  ariaLabel,
}: EChartProps) {
  const ref = useRef<HTMLDivElement | null>(null);
  const instanceRef = useRef<echarts.ECharts | null>(null);

  // Init + dispose once per mount.
  useEffect(() => {
    ensureRegistered();
    if (!ref.current) return;
    const inst = echarts.init(ref.current, undefined, {
      renderer: "canvas",
    });
    instanceRef.current = inst;

    const ro = new ResizeObserver(() => inst.resize());
    ro.observe(ref.current);

    return () => {
      ro.disconnect();
      inst.dispose();
      instanceRef.current = null;
    };
  }, []);

  // Apply option whenever it changes. echarts diffs internally — no
  // need to dispose between renders.
  useEffect(() => {
    instanceRef.current?.setOption(option, notMerge);
  }, [option, notMerge]);

  return (
    <Box
      ref={ref}
      role="img"
      aria-label={ariaLabel}
      sx={{
        width: "100%",
        height,
        // Suppress the default echarts focus ring; the surrounding card
        // already provides keyboard affordance on its container.
        "& canvas": { outline: "none" },
      }}
    />
  );
}

// ----- Shared chart styling helpers --------------------------------

// Token-derived palette used across IronFlyer charts. Keep this in
// sync with packages/design-tokens — never hardcode hex inline.
export const chartPalette = {
  // Surface / chrome
  axisLine: tokens.color.border.subtle,
  axisText: tokens.color.text.muted,
  splitLine: tokens.color.border.subtle,
  tooltipBg: tokens.color.bg.surface,
  tooltipBorder: tokens.color.border.strong,
  tooltipText: tokens.color.text.primary,
  // Semantic series colors — match the no-lime-first identity:
  //   violet  → primary metric (revenue, count)
  //   mint    → live / success / positive delta
  //   coral   → cost / spend / negative pressure
  //   amber   → caution / blocked
  //   sky     → secondary (sandbox / infra)
  series: {
    primary: tokens.color.accent.violet,
    success: tokens.color.brand.mint,
    cost: tokens.color.accent.coral,
    warn: tokens.color.brand.amber,
    secondary: tokens.color.accent.sky,
  },
};

// hexAlpha — return a `#rrggbbAA` color from a 6-digit hex + opacity.
// Falls through unchanged for non-hex inputs (already-rgba, named).
export function hexAlpha(hex: string, alpha: number): string {
  const m = /^#([0-9a-f]{6})$/i.exec(hex);
  if (!m) return hex;
  const a = Math.round(Math.max(0, Math.min(1, alpha)) * 255)
    .toString(16)
    .padStart(2, "0");
  return `#${m[1]}${a}`;
}

// Shared tooltip styling — applied to every chart so hover affordances
// stay consistent across the product.
export function tooltipDefaults(): Record<string, unknown> {
  return {
    backgroundColor: chartPalette.tooltipBg,
    borderColor: chartPalette.tooltipBorder,
    borderWidth: 1,
    padding: [8, 10],
    textStyle: {
      color: chartPalette.tooltipText,
      fontFamily: tokens.font.family,
      fontSize: 12,
    },
    extraCssText: `box-shadow: 0 8px 24px ${hexAlpha("#000000", 0.35)}; border-radius: 6px;`,
  };
}

export function axisDefaults(): Record<string, unknown> {
  return {
    axisLine: { lineStyle: { color: chartPalette.axisLine } },
    axisTick: { lineStyle: { color: chartPalette.axisLine } },
    axisLabel: {
      color: chartPalette.axisText,
      fontFamily: tokens.font.mono,
      fontSize: 10.5,
    },
    splitLine: { lineStyle: { color: chartPalette.splitLine, type: "dashed" } },
  };
}
