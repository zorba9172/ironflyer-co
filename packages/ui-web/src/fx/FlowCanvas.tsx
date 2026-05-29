import { lazy, Suspense, type ReactNode } from 'react';
import type { Node, Edge, NodeMouseHandler, NodeTypes } from '@xyflow/react';
import { useMounted } from './useMounted';

export type FlowNode = Node;
export type FlowEdge = Edge;
export type { NodeMouseHandler, NodeTypes };
export type { HandleSpec } from './FlowCanvasInner';

const Inner = lazy(() => import('./FlowCanvasInner'));

export interface FlowCanvasProps {
  nodes: FlowNode[];
  edges: FlowEdge[];
  onNodeClick?: NodeMouseHandler;
  height?: number | string;
  /** custom node renderers keyed by `node.type` (merged with the built-in `card`/`lane`) */
  nodeTypes?: NodeTypes;
  /** render the navigator minimap (bottom-left) */
  minimap?: boolean;
  /** lay edges left→right (handles on the sides) instead of top→bottom */
  horizontal?: boolean;
  fitViewPadding?: number;
  /** dim everything except the path through the hovered/focused node */
  focusable?: boolean;
  /** node id to keep focused (e.g. the selected gate) when nothing is hovered */
  focusId?: string | null;
  /** legend / key rendered as a Panel inside the canvas (top-left) */
  legend?: ReactNode;
}

// Lazy React Flow canvas — the heavy graph lib loads only on the client, only
// when this mounts (constitutional: viz libs never land in the cold bundle).
export function FlowCanvas({ height = '100%', ...rest }: FlowCanvasProps) {
  const mounted = useMounted();
  return (
    <div style={{ width: '100%', height }}>
      {mounted && (
        <Suspense fallback={null}>
          <Inner {...rest} />
        </Suspense>
      )}
    </div>
  );
}
