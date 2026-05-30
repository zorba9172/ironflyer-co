import { useMemo, useId } from 'react';
import { Box } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import type { Constellation3DNode, Constellation3DLink } from '@ironflyer/ui-web/fx';

export type { Constellation3DNode, Constellation3DLink };

export type NeonConstellation3DProps = {
  nodes: Constellation3DNode[];
  links: Constellation3DLink[];
  height?: number;
  /** retained for call-site compatibility; the 2D network never rotates */
  rotate?: boolean;
};

const VB = 1000; // square viewBox; container letterboxes via preserveAspectRatio

// Clean 2D node network (formerly a rotating three.js Constellation3D). The
// locked references show a calm, subtle network — soft glowing dots joined by
// faint edges — not a spinning 3D object. Each node maps to a real gate/agent
// and each edge to a real relationship; nothing here is decoration. Positions
// come from the node's x/y (a -1..1 plane); nodes without a fixed position are
// auto-laid-out on a ring so the graph is always readable. The name + props are
// kept so existing call sites stay untouched; `rotate` is accepted and ignored.
export function NeonConstellation3D({ nodes, links, height = 320 }: NeonConstellation3DProps) {
  const theme = useTheme();
  const gid = useId().replace(/[:]/g, '');
  const series = theme.studio.chart.series;

  const placed = useMemo(() => {
    const map = new Map<string, { x: number; y: number; r: number; color: string }>();
    const pad = 90;
    const span = VB - pad * 2;
    const project = (v: number) => pad + ((v + 1) / 2) * span; // -1..1 → padded box
    const ring = nodes.filter((n) => n.x == null || n.y == null);
    nodes.forEach((n, i) => {
      let px: number;
      let py: number;
      if (n.x != null && n.y != null) {
        px = project(Math.max(-1, Math.min(1, n.x)));
        py = project(Math.max(-1, Math.min(1, n.y)));
      } else {
        // Even ring layout for unpositioned nodes.
        const idx = ring.indexOf(n);
        const ang = (idx / Math.max(1, ring.length)) * Math.PI * 2 - Math.PI / 2;
        const rad = ring.length === 1 ? 0 : span * 0.42;
        px = VB / 2 + Math.cos(ang) * rad;
        py = VB / 2 + Math.sin(ang) * rad;
      }
      const v = n.value == null ? 0.5 : Math.max(0, Math.min(1, n.value));
      map.set(n.id, { x: px, y: py, r: 13 + v * 20, color: n.color ?? series[i % series.length] ?? series[0] });
    });
    return map;
  }, [nodes, series]);

  const edges = useMemo(
    () =>
      links
        .map((l) => {
          const a = placed.get(l.source);
          const b = placed.get(l.target);
          if (!a || !b) return null;
          return { a, b, color: l.color ?? a.color };
        })
        .filter(Boolean) as { a: { x: number; y: number }; b: { x: number; y: number }; color: string }[],
    [links, placed],
  );

  return (
    <Box sx={{ width: '100%', height, minHeight: 0 }}>
      <Box
        component="svg"
        viewBox={`0 0 ${VB} ${VB}`}
        preserveAspectRatio="xMidYMid meet"
        sx={{ width: '100%', height: '100%', display: 'block' }}
      >
        <defs>
          <filter id={`glow-${gid}`} x="-60%" y="-60%" width="220%" height="220%">
            <feGaussianBlur stdDeviation="9" result="b" />
            <feMerge>
              <feMergeNode in="b" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
        </defs>

        {/* Faint connective edges. */}
        <g opacity={theme.palette.mode === 'dark' ? 0.5 : 0.38}>
          {edges.map((e, i) => (
            <line
              key={i}
              x1={e.a.x}
              y1={e.a.y}
              x2={e.b.x}
              y2={e.b.y}
              stroke={e.color}
              strokeWidth={2}
              strokeLinecap="round"
            />
          ))}
        </g>

        {/* Soft glowing nodes. */}
        <g>
          {[...placed.values()].map((n, i) => (
            <g key={i}>
              <circle cx={n.x} cy={n.y} r={n.r * 1.7} fill={n.color} opacity={0.14} filter={`url(#glow-${gid})`} />
              <circle cx={n.x} cy={n.y} r={n.r} fill={n.color} opacity={0.95} />
              <circle cx={n.x} cy={n.y} r={n.r * 0.42} fill={theme.palette.background.paper} opacity={0.92} />
            </g>
          ))}
        </g>
      </Box>
    </Box>
  );
}
