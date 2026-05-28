import { useMemo } from 'react';
import { Box } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { FlowCanvas, type FlowNode, type FlowEdge, type NodeMouseHandler } from '@ironflyer/ui-web/fx';
import { statusColor } from './statusColor';
import { type StudioProject } from '../studioData';
import { useStudio } from '../store';

const POS: Record<string, { x: number; y: number }> = {
  identity: { x: 40, y: 170 },
  money: { x: 300, y: 50 },
  data: { x: 300, y: 170 },
  security: { x: 300, y: 290 },
  deploy: { x: 600, y: 170 },
  signal: { x: 860, y: 170 },
};

const DEPS: [string, string][] = [
  ['identity', 'money'],
  ['identity', 'data'],
  ['identity', 'security'],
  ['money', 'deploy'],
  ['data', 'deploy'],
  ['security', 'deploy'],
  ['deploy', 'signal'],
];

// Viz-first project map: gates as nodes, dependency edges, status as color.
export function GateMap({ project }: { project: StudioProject }) {
  const theme = useTheme();
  const selectGate = useStudio((s) => s.selectGate);

  const nodes = useMemo<FlowNode[]>(
    () =>
      project.gates.map((g) => {
        const color = statusColor(theme, g.status);
        return {
          id: g.id,
          position: POS[g.id] ?? { x: 0, y: 0 },
          data: { label: `${g.no}  ${g.name}`, blocking: g.blocking, name: g.name },
          style: {
            width: 170,
            padding: '12px 14px',
            borderRadius: 12,
            border: `1.5px solid ${color}`,
            background: theme.palette.background.paper,
            color: theme.palette.text.primary,
            fontWeight: 600,
            fontSize: 13,
            boxShadow: `0 0 0 4px ${color}1f`,
          },
        };
      }),
    [project, theme],
  );

  const edges = useMemo<FlowEdge[]>(
    () =>
      DEPS.map(([s, t]) => {
        const target = project.gates.find((g) => g.id === t);
        const live = target?.status === 'running';
        return {
          id: `${s}-${t}`,
          source: s,
          target: t,
          animated: live,
          style: { stroke: theme.palette.divider, strokeWidth: 1.5 },
        };
      }),
    [project, theme],
  );

  const onNodeClick: NodeMouseHandler = (_e, node) => selectGate(node.id);

  return (
    <Box sx={{ flex: 1, height: '100%', bgcolor: 'background.default' }}>
      <FlowCanvas nodes={nodes} edges={edges} onNodeClick={onNodeClick} />
    </Box>
  );
}
