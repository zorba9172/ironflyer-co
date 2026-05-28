import { ReactFlow, Background, Controls, type Node, type Edge, type NodeMouseHandler } from '@xyflow/react';
import '@xyflow/react/dist/style.css';

export default function FlowCanvasInner({ nodes, edges, onNodeClick }: { nodes: Node[]; edges: Edge[]; onNodeClick?: NodeMouseHandler }) {
  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      onNodeClick={onNodeClick}
      fitView
      fitViewOptions={{ padding: 0.25 }}
      nodesDraggable={false}
      nodesConnectable={false}
      elementsSelectable
      panOnScroll
      zoomOnScroll={false}
    >
      <Background gap={22} size={1} color="var(--if-palette-divider)" />
      <Controls showInteractive={false} position="bottom-right" />
    </ReactFlow>
  );
}
