"use client";

// Ironflyer-flavoured SVG icons. We keep them lightweight (no icon font)
// and palette-aware (currentColor). Most generic glyphs come from
// `@mui/icons-material`; the icons here express the V22 domain
// vocabulary: wallet, gates, deploys, ledger.

import type { SVGProps } from "react";

export type IconProps = SVGProps<SVGSVGElement> & {
  size?: number;
};

function base({ size = 18, ...rest }: IconProps): SVGProps<SVGSVGElement> {
  return {
    width: size,
    height: size,
    viewBox: "0 0 24 24",
    fill: "none",
    stroke: "currentColor",
    strokeWidth: 1.6,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
    "aria-hidden": true,
    ...rest,
  };
}

export function WalletIcon(props: IconProps) {
  return (
    <svg {...base(props)}>
      <path d="M3 7a3 3 0 0 1 3-3h11a2 2 0 0 1 2 2v2H6a3 3 0 0 1-3-3z" />
      <path d="M3 7v11a2 2 0 0 0 2 2h15a2 2 0 0 0 2-2V9a2 2 0 0 0-2-2H3z" />
      <circle cx="17" cy="13" r="1.2" fill="currentColor" stroke="none" />
    </svg>
  );
}

// GateIcon — three horizontal bars + a vertical rail. Mirrors the
// brand mark and reads as "review gates" in cockpit chrome.
export function GateIcon(props: IconProps) {
  return (
    <svg {...base(props)}>
      <line x1="4" y1="5" x2="4" y2="19" />
      <line x1="8" y1="7" x2="20" y2="7" />
      <line x1="8" y1="12" x2="17" y2="12" />
      <line x1="8" y1="17" x2="14" y2="17" />
    </svg>
  );
}

// GatePassIcon — gate + check.
export function GatePassIcon(props: IconProps) {
  return (
    <svg {...base(props)}>
      <line x1="4" y1="5" x2="4" y2="19" />
      <line x1="8" y1="7" x2="20" y2="7" />
      <line x1="8" y1="12" x2="17" y2="12" />
      <polyline points="9 17 12 20 20 12" />
    </svg>
  );
}

// GateBlockIcon — gate + diagonal slash, for "gate blocked deploy".
export function GateBlockIcon(props: IconProps) {
  return (
    <svg {...base(props)}>
      <line x1="4" y1="5" x2="4" y2="19" />
      <line x1="8" y1="7" x2="20" y2="7" />
      <line x1="8" y1="12" x2="17" y2="12" />
      <line x1="8" y1="17" x2="14" y2="17" />
      <line x1="3" y1="3" x2="21" y2="21" />
    </svg>
  );
}

// DeployIcon — arrow exiting a tile, suggests "ship".
export function DeployIcon(props: IconProps) {
  return (
    <svg {...base(props)}>
      <rect x="3" y="4" width="13" height="16" rx="2" />
      <polyline points="14 9 21 12 14 15" />
      <line x1="9" y1="12" x2="21" y2="12" />
    </svg>
  );
}

// LedgerIcon — stacked rows suggesting an append-only chain.
export function LedgerIcon(props: IconProps) {
  return (
    <svg {...base(props)}>
      <rect x="3" y="5" width="18" height="4" rx="1" />
      <rect x="3" y="11" width="18" height="4" rx="1" />
      <rect x="3" y="17" width="18" height="2.5" rx="1" />
    </svg>
  );
}

// SparkIcon — small lightning bolt used as a generic "running" cue.
export function SparkIcon(props: IconProps) {
  return (
    <svg {...base(props)}>
      <polygon
        points="13 2 4 14 11 14 9 22 20 10 13 10 13 2"
        fill="currentColor"
        stroke="currentColor"
      />
    </svg>
  );
}

// ShieldIcon — used for security report headers.
export function ShieldIcon(props: IconProps) {
  return (
    <svg {...base(props)}>
      <path d="M12 3 4 6v6c0 4.5 3.4 8.4 8 9 4.6-.6 8-4.5 8-9V6l-8-3z" />
    </svg>
  );
}
