import { lazy, Suspense } from 'react';
import type { Node, Edge, NodeMouseHandler } from '@xyflow/react';
import { useMounted } from './useMounted';

export type FlowNode = Node;
export type FlowEdge = Edge;
export type { NodeMouseHandler };

const Inner = lazy(() => import('./FlowCanvasInner'));

// Lazy React Flow canvas — the heavy graph lib loads only on the client, only
// when this mounts (constitutional: viz libs never land in the cold bundle).
export function FlowCanvas({ nodes, edges, onNodeClick, height = '100%' }: {
  nodes: FlowNode[];
  edges: FlowEdge[];
  onNodeClick?: NodeMouseHandler;
  height?: number | string;
}) {
  const mounted = useMounted();
  return (
    <div style={{ width: '100%', height }}>
      {mounted && (
        <Suspense fallback={null}>
          <Inner nodes={nodes} edges={edges} onNodeClick={onNodeClick} />
        </Suspense>
      )}
    </div>
  );
}
