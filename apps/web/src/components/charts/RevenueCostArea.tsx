"use client";

// RevenueCostArea — stacked area for the ProfitDashboard top strip.
//
// Three series across the active window:
//   Revenue        → violet area (primary metric, on top)
//   Provider cost  → coral area  (Anthropic / OpenAI / Gemini ...)
//   Sandbox cost   → sky area    (Docker runtime CPU)
//
// A dashed mark line marks the gross-margin floor (0% break-even) so
// any window where stacked cost crosses revenue is immediately
// visible. Tooltip shows the per-bucket triple plus margin %.

import { useMemo } from "react";
import {
  axisDefaults,
  chartPalette,
  EChart,
  hexAlpha,
  tooltipDefaults,
} from "./EChart";

export interface RevenueCostPoint {
  label: string;
  revenue: number;
  providerCost: number;
  sandboxCost: number;
}

export interface RevenueCostAreaProps {
  points: RevenueCostPoint[];
  height?: number;
  ariaLabel?: string;
}

export function RevenueCostArea({
  points,
  height = 220,
  ariaLabel = "Revenue versus provider and sandbox cost",
}: RevenueCostAreaProps) {
  const option = useMemo(() => {
    const labels = points.map((p) => p.label);
    const revenue = points.map((p) => Number(p.revenue.toFixed(4)));
    const providerCost = points.map((p) => Number(p.providerCost.toFixed(4)));
    const sandboxCost = points.map((p) => Number(p.sandboxCost.toFixed(4)));

    const violet = chartPalette.series.primary;
    const coral = chartPalette.series.cost;
    const sky = chartPalette.series.secondary;
    const muted = chartPalette.axisText;

    const axis = axisDefaults();
    const tooltip = tooltipDefaults();

    const areaStops = (c: string) => ({
      type: "linear",
      x: 0,
      y: 0,
      x2: 0,
      y2: 1,
      colorStops: [
        { offset: 0, color: hexAlpha(c, 0.45) },
        { offset: 1, color: hexAlpha(c, 0) },
      ],
    });

    return {
      grid: { left: 44, right: 12, top: 28, bottom: 24 },
      legend: {
        top: 0,
        right: 8,
        icon: "roundRect",
        itemWidth: 8,
        itemHeight: 8,
        textStyle: {
          color: muted,
          fontSize: 11,
        },
      },
      tooltip: {
        ...tooltip,
        trigger: "axis",
        axisPointer: { type: "line", lineStyle: { color: muted, type: "dashed" } },
        valueFormatter: (v: number) => `$${Number(v).toFixed(4)}`,
      },
      xAxis: {
        type: "category",
        data: labels,
        boundaryGap: false,
        ...axis,
      },
      yAxis: {
        type: "value",
        ...axis,
        axisLabel: {
          ...(axis.axisLabel as object),
          formatter: (v: number) => `$${v.toFixed(2)}`,
        },
      },
      series: [
        {
          name: "Revenue",
          type: "line",
          smooth: true,
          showSymbol: false,
          data: revenue,
          lineStyle: { color: violet, width: 2 },
          areaStyle: { color: areaStops(violet) },
          z: 3,
        },
        {
          name: "Provider cost",
          type: "line",
          smooth: true,
          showSymbol: false,
          data: providerCost,
          lineStyle: { color: coral, width: 1.4 },
          areaStyle: { color: areaStops(coral) },
          z: 2,
        },
        {
          name: "Sandbox cost",
          type: "line",
          smooth: true,
          showSymbol: false,
          data: sandboxCost,
          lineStyle: { color: sky, width: 1.4 },
          areaStyle: { color: areaStops(sky) },
          z: 1,
        },
      ],
    };
  }, [points]);

  return <EChart option={option} height={height} ariaLabel={ariaLabel} />;
}
