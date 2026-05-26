"use client";

// DependencyGraphCanvas — heavy @xyflow/react graph for the
// Dependency panel's expanded view. Pulled in via next/dynamic from
// DependencyGraphPanel so the @xyflow/react chunk + CSS never load
// on the cold cockpit page.
//
// Edges colored by allow/deny per the architecture manifest. Nodes
// laid out in concentric layers from the manifest's `layers` array
// so the operator can see at a glance which package depends on which.

import { useMemo } from "react";
import {
  Background,
  BackgroundVariant,
  Controls,
  MarkerType,
  ReactFlow,
  ReactFlowProvider,
  type Edge,
  type Node,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { Box } from "@mui/material";
import { tokens } from "../../../theme";
import type { ArchitectureLayer, ArchitectureRule } from "./types";

export interface DependencyGraphCanvasProps {
  layers: ArchitectureLayer[];
  rules: ArchitectureRule[];
}

export function DependencyGraphCanvas({ layers, rules }: DependencyGraphCanvasProps) {
  const { nodes, edges } = useMemo(() => buildGraph(layers, rules), [layers, rules]);

  return (
    <Box
      sx={{
        width: "100%",
        height: 360,
        borderRadius: 1,
        border: `1px solid ${tokens.color.border.subtle}`,
        bgcolor: tokens.color.bg.inset,
        overflow: "hidden",
      }}
    >
      <ReactFlowProvider>
        <ReactFlow
          nodes={nodes}
          edges={edges}
          fitView
          fitViewOptions={{ padding: 0.2 }}
          panOnDrag
          minZoom={0.4}
          maxZoom={1.6}
          nodesDraggable={false}
          edgesFocusable={false}
          proOptions={{ hideAttribution: true }}
        >
          <Background
            variant={BackgroundVariant.Dots}
            gap={20}
            color={tokens.color.border.subtle}
          />
          <Controls
            showInteractive={false}
            style={{
              background: tokens.color.bg.surface,
              border: `1px solid ${tokens.color.border.subtle}`,
              color: tokens.color.text.secondary,
            }}
          />
        </ReactFlow>
      </ReactFlowProvider>
    </Box>
  );
}

function buildGraph(
  layers: ArchitectureLayer[],
  rules: ArchitectureRule[],
): { nodes: Node[]; edges: Edge[] } {
  // Layered horizontal layout — one column per layer, packages stacked
  // vertically. This keeps the graph readable even at 360px tall.
  const colWidth = 220;
  const rowHeight = 56;
  const nodes: Node[] = [];
  const packageToLayer = new Map<string, string>();

  layers.forEach((layer, li) => {
    layer.packages.forEach((pkg, pi) => {
      packageToLayer.set(pkg, layer.name);
      nodes.push({
        id: pkg,
        position: { x: li * colWidth, y: pi * rowHeight },
        data: { label: pkg },
        style: {
          background: tokens.color.bg.surface,
          border: `1px solid ${tokens.color.border.strong}`,
          color: tokens.color.text.primary,
          borderRadius: 8,
          padding: 8,
          fontFamily: tokens.font.mono,
          fontSize: 11,
          width: colWidth - 32,
        },
      });
    });
    // Layer label as a header node.
    nodes.push({
      id: `__layer_${layer.name}`,
      position: { x: li * colWidth, y: -48 },
      data: { label: layer.name.toUpperCase() },
      draggable: false,
      selectable: false,
      style: {
        background: "transparent",
        border: "none",
        color: tokens.color.accent.violet,
        fontFamily: tokens.font.mono,
        fontSize: 10,
        letterSpacing: 1.2,
        fontWeight: 700,
        width: colWidth - 32,
      },
    });
  });

  const edges: Edge[] = rules.map((rule, i) => ({
    id: `${rule.from}->${rule.to}#${i}`,
    source: rule.from,
    target: rule.to,
    animated: !rule.allow,
    style: {
      stroke: rule.allow
        ? tokens.color.brand.mint
        : tokens.color.accent.coral,
      strokeWidth: 1.5,
      strokeDasharray: rule.allow ? undefined : "4 3",
    },
    markerEnd: {
      type: MarkerType.ArrowClosed,
      color: rule.allow
        ? tokens.color.brand.mint
        : tokens.color.accent.coral,
    },
  }));

  return { nodes, edges };
}
