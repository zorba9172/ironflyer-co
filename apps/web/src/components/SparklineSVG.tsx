"use client";

// SparklineSVG — dependency-free inline SVG sparkline.
//
// Reserved for the smallest inline strips (list rows, badges) where a
// full echarts canvas would be overkill. Dashboards use the
// echarts-backed components under ./charts/ instead. Stroke defaults
// to the violet primary so the strip honors the no-lime-first identity.

import { Box } from "@mui/material";
import { tokens } from "../theme";

export interface SparklinePoint {
  // ISO timestamp for the bucket; not rendered, useful for parent layout.
  ts?: string;
  // Bucket value (USD).
  value: number;
}

export interface SparklineSVGProps {
  points: SparklinePoint[];
  width?: number;
  height?: number;
  // Stroke color override; defaults to lime accent.
  stroke?: string;
  // Soft fill under the line; defaults to lime alpha 0.18.
  fill?: string;
  // Whether to render a zero baseline rule (helps when all values are 0).
  showBaseline?: boolean;
  ariaLabel?: string;
}

export function SparklineSVG({
  points,
  width = 320,
  height = 64,
  stroke = tokens.color.accent.lime,
  fill,
  showBaseline = true,
  ariaLabel,
}: SparklineSVGProps) {
  const w = width;
  const h = height;
  const pad = 2;
  const innerW = Math.max(1, w - pad * 2);
  const innerH = Math.max(1, h - pad * 2);

  // Empty / single-point fall-throughs render a clean baseline so the
  // card never collapses to nothing.
  const safe = points.length === 0 ? [{ value: 0 }, { value: 0 }] : points;
  const max = Math.max(0, ...safe.map((p) => p.value));
  const min = 0; // floor at 0 — spend never goes negative.
  const range = Math.max(0.0001, max - min);

  const step = innerW / Math.max(1, safe.length - 1);

  const coords = safe.map((p, i) => {
    const x = pad + step * i;
    const y = pad + innerH - ((p.value - min) / range) * innerH;
    return { x, y };
  });

  const linePath = coords
    .map((c, i) => `${i === 0 ? "M" : "L"}${c.x.toFixed(2)},${c.y.toFixed(2)}`)
    .join(" ");

  // Closed polygon for the fill.
  const fillPath = `${linePath} L${(pad + innerW).toFixed(2)},${(pad + innerH).toFixed(
    2,
  )} L${pad.toFixed(2)},${(pad + innerH).toFixed(2)} Z`;

  const fillColor = fill ?? hexAlpha(stroke, 0.18);

  return (
    <Box
      component="svg"
      role="img"
      aria-label={ariaLabel ?? "Spend over time"}
      viewBox={`0 0 ${w} ${h}`}
      sx={{ display: "block", width: "100%", height: "auto", maxWidth: w }}
    >
      {showBaseline && (
        <line
          x1={pad}
          x2={pad + innerW}
          y1={pad + innerH}
          y2={pad + innerH}
          stroke={tokens.color.border.subtle}
          strokeWidth={1}
        />
      )}
      <path d={fillPath} fill={fillColor} stroke="none" />
      <path
        d={linePath}
        fill="none"
        stroke={stroke}
        strokeWidth={1.6}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      {coords.length > 0 && (
        <circle
          cx={coords[coords.length - 1].x}
          cy={coords[coords.length - 1].y}
          r={2.5}
          fill={stroke}
        />
      )}
    </Box>
  );
}

// hexAlpha — accept a #rrggbb hex and return an rgba() at the given
// opacity. Tolerant: returns the input unchanged when not a #rrggbb.
function hexAlpha(hex: string, alpha: number): string {
  const m = /^#([0-9a-f]{6})$/i.exec(hex);
  if (!m) return hex;
  const n = parseInt(m[1], 16);
  const r = (n >> 16) & 0xff;
  const g = (n >> 8) & 0xff;
  const b = n & 0xff;
  return `rgba(${r}, ${g}, ${b}, ${alpha})`;
}
