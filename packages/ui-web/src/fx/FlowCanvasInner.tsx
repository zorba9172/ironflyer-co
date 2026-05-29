import { useMemo, useState, type ReactNode } from 'react';
import {
  ReactFlow, Background, BackgroundVariant, Controls, MiniMap, Panel, Handle, Position, MarkerType,
  type Node, type Edge, type NodeMouseHandler, type NodeTypes, type NodeProps, type EdgeMarker,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';

type Side = 'left' | 'right' | 'top' | 'bottom';
const SIDE_POS: Record<Side, Position> = {
  left: Position.Left, right: Position.Right, top: Position.Top, bottom: Position.Bottom,
};

// Connection points a card exposes. Declared as plain data by the (eager)
// calling surface so that @xyflow/react runtime values (Handle, Position,
// MarkerType) stay inside this lazy module and never reach the cold bundle.
export interface HandleSpec { id: string; type: 'source' | 'target'; side: Side }

const HANDLE_STYLE = { opacity: 0, width: 1, height: 1, minWidth: 0, minHeight: 0, border: 'none' } as const;

// Generic node renderer: paints the themed body the surface already built
// (`data.label`) plus precise, invisible connection handles from `data.handles`.
function CardNode({ data }: NodeProps) {
  const handles = (data.handles as HandleSpec[] | undefined) ?? [];
  return (
    <>
      {handles.map((h) => (
        <Handle key={h.id} id={h.id} type={h.type} position={SIDE_POS[h.side]} isConnectable={false} style={HANDLE_STYLE} />
      ))}
      {data.label as ReactNode}
    </>
  );
}

// A background swimlane band — labels a region of the canvas, never interactive.
function LaneNode({ data }: NodeProps) {
  return (
    <div style={{ width: '100%', height: '100%', pointerEvents: 'none', position: 'relative' }}>
      <span style={{
        position: 'absolute', top: 9, left: 14, fontFamily: 'var(--if-font-mono, monospace)',
        fontSize: 9, letterSpacing: '0.18em', fontWeight: 700, textTransform: 'uppercase',
        color: 'var(--if-palette-text-disabled)',
      }}>{data.label as string}</span>
    </div>
  );
}

const DEFAULT_NODE_TYPES: NodeTypes = { card: CardNode, lane: LaneNode };

export default function FlowCanvasInner({
  nodes, edges, onNodeClick, nodeTypes, minimap, horizontal, fitViewPadding = 0.25,
  focusable = false, focusId = null, legend,
}: {
  nodes: Node[];
  edges: Edge[];
  onNodeClick?: NodeMouseHandler;
  nodeTypes?: NodeTypes;
  minimap?: boolean;
  horizontal?: boolean;
  fitViewPadding?: number;
  focusable?: boolean;
  focusId?: string | null;
  legend?: ReactNode;
}) {
  const [hovered, setHovered] = useState<string | null>(null);
  const types = useMemo(() => ({ ...DEFAULT_NODE_TYPES, ...nodeTypes }), [nodeTypes]);

  // Directed adjacency so we can light the full path a node sits on.
  const adj = useMemo(() => {
    const up = new Map<string, string[]>();
    const down = new Map<string, string[]>();
    for (const e of edges) {
      (down.get(e.source) ?? down.set(e.source, []).get(e.source)!).push(e.target);
      (up.get(e.target) ?? up.set(e.target, []).get(e.target)!).push(e.source);
    }
    return { up, down };
  }, [edges]);

  const active = hovered ?? focusId;
  const focusSet = useMemo(() => {
    if (!focusable || !active) return null;
    const set = new Set<string>([active]);
    const walk = (m: Map<string, string[]>) => {
      const stack = [active];
      while (stack.length) {
        for (const nx of m.get(stack.pop()!) ?? []) if (!set.has(nx)) { set.add(nx); stack.push(nx); }
      }
    };
    walk(adj.up);
    walk(adj.down);
    return set;
  }, [focusable, active, adj]);

  const laidNodes = useMemo(() => nodes.map((n) => {
    const directed = horizontal && n.type !== 'lane'
      ? { sourcePosition: Position.Right, targetPosition: Position.Left }
      : {};
    if (!focusSet || n.type === 'lane') return { ...directed, ...n };
    const lit = focusSet.has(n.id);
    return { ...directed, ...n, style: { ...n.style, opacity: lit ? 1 : 0.16, transition: 'opacity .18s ease' } };
  }), [nodes, focusSet, horizontal]);

  const styledEdges = useMemo(() => edges.map((e) => {
    const stroke = (e.style?.stroke as string | undefined) ?? 'var(--if-palette-divider)';
    const marker: EdgeMarker = e.markerEnd as EdgeMarker ?? { type: MarkerType.ArrowClosed, width: 15, height: 15, color: stroke };
    const edge: Edge = { ...e, type: e.type ?? 'smoothstep', markerEnd: marker };
    if (!focusSet) return edge;
    const lit = focusSet.has(e.source) && focusSet.has(e.target);
    return {
      ...edge,
      animated: lit && e.id.indexOf('facet') === -1,
      style: { ...e.style, opacity: lit ? 1 : 0.1, strokeWidth: lit ? 2.4 : (e.style?.strokeWidth ?? 1.5), transition: 'opacity .18s ease' },
    };
  }), [edges, focusSet]);

  return (
    <ReactFlow
      nodes={laidNodes}
      edges={styledEdges}
      nodeTypes={types}
      onNodeClick={onNodeClick}
      onNodeMouseEnter={focusable ? (_e, n) => setHovered(n.id) : undefined}
      onNodeMouseLeave={focusable ? () => setHovered(null) : undefined}
      fitView
      fitViewOptions={{ padding: fitViewPadding }}
      nodesDraggable={false}
      nodesConnectable={false}
      elementsSelectable
      panOnScroll
      zoomOnScroll={false}
      minZoom={0.2}
      maxZoom={1.75}
      proOptions={{ hideAttribution: true }}
    >
      <Background variant={BackgroundVariant.Dots} gap={22} size={1} color="var(--if-palette-divider)" />
      <Controls showInteractive={false} position="bottom-right" />
      {legend && <Panel position="top-left">{legend}</Panel>}
      {minimap && (
        <MiniMap
          pannable
          zoomable
          position="bottom-left"
          nodeColor={(n) => (n.data?.tone as string | undefined) ?? 'var(--if-palette-text-disabled)'}
          nodeStrokeColor="var(--if-palette-divider)"
          maskColor="var(--if-palette-background-default)"
          style={{ background: 'var(--if-palette-background-paper)', border: '1px solid var(--if-palette-divider)', borderRadius: 10 }}
        />
      )}
    </ReactFlow>
  );
}
