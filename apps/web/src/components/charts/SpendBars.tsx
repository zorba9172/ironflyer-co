"use client";

// SpendBars — token-bucketed spend over time as a bar chart.
//
// Replaces the inline SparklineSVG strip on the wallet 7-day and
// dashboard 24-hour cards with a chart that actually reads the
// magnitude per bucket. Bars use the violet primary; the cumulative
// total renders as a thin mint line on a secondary axis so the user
// can read "today vs the week" at a glance.

import { useMemo } from "react";
import {
  axisDefaults,
  chartPalette,
  EChart,
  hexAlpha,
  tooltipDefaults,
} from "./EChart";

export interface SpendBarPoint {
  // Display label for the bucket (e.g. "09:00", "Mon").
  label: string;
  // Bucket spend in USD.
  value: number;
}

export interface SpendBarsProps {
  points: SpendBarPoint[];
  height?: number;
  // Show the cumulative-total line overlay. Defaults true; pass false
  // on dense strips (e.g. 24h hourly) where the second axis is noise.
  showCumulative?: boolean;
  ariaLabel?: string;
}

export function SpendBars({
  points,
  height = 180,
  showCumulative = true,
  ariaLabel = "Spend over time",
}: SpendBarsProps) {
  const option = useMemo(() => {
    const labels = points.map((p) => p.label);
    const values = points.map((p) => Number(p.value.toFixed(4)));
    const cumulative: number[] = [];
    values.reduce((acc, v, i) => {
      const next = acc + v;
      cumulative[i] = Number(next.toFixed(4));
      return next;
    }, 0);

    const axis = axisDefaults();
    const tooltip = tooltipDefaults();
    const violet = chartPalette.series.primary;
    const mint = chartPalette.series.success;

    return {
      grid: { left: 36, right: showCumulative ? 36 : 12, top: 14, bottom: 22 },
      tooltip: {
        ...tooltip,
        trigger: "axis",
        axisPointer: { type: "shadow" },
        valueFormatter: (v: number) => `$${Number(v).toFixed(4)}`,
      },
      xAxis: {
        type: "category",
        data: labels,
        ...axis,
        boundaryGap: true,
      },
      yAxis: [
        {
          type: "value",
          ...axis,
          axisLabel: {
            ...(axis.axisLabel as object),
            formatter: (v: number) => `$${v.toFixed(2)}`,
          },
        },
        ...(showCumulative
          ? [
              {
                type: "value",
                ...axis,
                axisLabel: {
                  ...(axis.axisLabel as object),
                  formatter: (v: number) => `$${v.toFixed(2)}`,
                },
                splitLine: { show: false },
              },
            ]
          : []),
      ],
      series: [
        {
          type: "bar",
          name: "Spend",
          data: values,
          barMaxWidth: 18,
          itemStyle: {
            color: {
              type: "linear",
              x: 0,
              y: 0,
              x2: 0,
              y2: 1,
              colorStops: [
                { offset: 0, color: violet },
                { offset: 1, color: hexAlpha(violet, 0.35) },
              ],
            },
            borderRadius: [3, 3, 0, 0],
          },
          emphasis: { itemStyle: { color: violet } },
        },
        ...(showCumulative
          ? [
              {
                type: "line",
                name: "Cumulative",
                data: cumulative,
                yAxisIndex: 1,
                smooth: true,
                showSymbol: false,
                lineStyle: { width: 1.5, color: mint },
                areaStyle: {
                  color: {
                    type: "linear",
                    x: 0,
                    y: 0,
                    x2: 0,
                    y2: 1,
                    colorStops: [
                      { offset: 0, color: hexAlpha(mint, 0.22) },
                      { offset: 1, color: hexAlpha(mint, 0) },
                    ],
                  },
                },
              },
            ]
          : []),
      ],
    };
  }, [points, showCumulative]);

  return <EChart option={option} height={height} ariaLabel={ariaLabel} />;
}
