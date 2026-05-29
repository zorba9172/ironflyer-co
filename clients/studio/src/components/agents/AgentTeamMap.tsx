import { useCallback, useMemo } from 'react';
import { Box } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { FlowCanvas, type FlowNode, type FlowEdge, type NodeMouseHandler, type HandleSpec } from '@ironflyer/ui-web/fx';
import { VscRobot, VscHubot, VscGitMerge } from 'react-icons/vsc';
import { agentStatus, statusLabel, type Agent, type AgentStatus, type Gate } from '../../studioData';
import { autonomyLabel } from '../../agentLibrary';
import { nodePalette, type MapColors } from '../map/nodes';
import { agentColor, statusColor } from '../statusColor';

// Layout lanes — orchestrator hub up top, the specialist + custom roster in the
// middle, the finisher gates they own beneath. Hand-off edges arc agent→agent.
const COL = 210;
const ORCH_Y = -280;
const AGENT_Y = -80;
const GATE_Y = 170;

const H_ORCH: HandleSpec[] = [{ id: 'b', type: 'source', side: 'bottom' }];
const H_AGENT: HandleSpec[] = [
  { id: 't', type: 'target', side: 'top' },
  { id: 'b', type: 'source', side: 'bottom' },
  { id: 'hl', type: 'target', side: 'left' },
  { id: 'hr', type: 'source', side: 'right' },
];
const H_GATE: HandleSpec[] = [{ id: 't', type: 'target', side: 'top' }];

const STATUS_TONE: Record<AgentStatus, string> = { idle: 'Idle', working: 'Working', blocked: 'Blocked', done: 'Done' };

// Orchestration map for the agent catalog. A visual mirror of the team: the
// orchestrator routes to every agent (solid), each agent tethers to the gate it
// owns (dashed down), and delegation shows as agent→agent hand-off arcs. Click
// a node to open it in the builder.
export function AgentTeamMap({ agents, gates, onEdit }: { agents: Agent[]; gates: Gate[]; onEdit?: (a: Agent) => void }) {
  const theme = useTheme();
  const c = useMemo(() => nodePalette(theme), [theme]);

  const orchestrator = agents.find((a) => a.id === 'orchestrator');
  const roster = useMemo(() => agents.filter((a) => a.id !== 'orchestrator'), [agents]);
  const gateIds = useMemo(() => new Set(gates.map((g) => g.id)), [gates]);

  // Content signature — gate the expensive graph rebuild on exactly what we draw
  // so re-renders that don't change the team keep the same node references.
  const sig = useMemo(
    () => [
      orchestrator?.id ?? '',
      roster.map((a) => `${a.id}:${a.gateId ?? ''}:${agentStatus(a, gates)}:${(a.handoffTo ?? []).join('+')}`).join('|'),
      gates.map((g) => `${g.id}|${g.status}|${g.blocking ? 1 : 0}`).join(';'),
    ].join('~'),
    [orchestrator?.id, roster, gates],
  );

  const colOf = useMemo(() => {
    const m = new Map<string, number>();
    roster.forEach((a, i) => m.set(a.id, i));
    return m;
  }, [roster]);

  const onNodeClick = useCallback<NodeMouseHandler>((_e, node) => {
    if (node.id.startsWith('agent_')) {
      const a = agents.find((x) => x.id === node.id.slice('agent_'.length));
      if (a) onEdit?.(a);
    }
  }, [agents, onEdit]);

  const nodes = useMemo<FlowNode[]>(() => {
    const cols = Math.max(roster.length, 1);
    const laneW = cols * COL + 40;
    const centerX = (cols * COL) / 2 - 98;

    const lanes: FlowNode[] = [
      { id: 'lane_agents', type: 'lane', position: { x: -40, y: AGENT_Y - 40 }, draggable: false, selectable: false, zIndex: -1, data: { label: 'Specialists' }, style: { width: laneW, height: 150, borderRadius: 18, border: `1.5px dashed ${c.accent}`, opacity: 0.35, pointerEvents: 'none' } },
      { id: 'lane_gates', type: 'lane', position: { x: -40, y: GATE_Y - 40 }, draggable: false, selectable: false, zIndex: -1, data: { label: 'Finisher gates' }, style: { width: laneW, height: 150, borderRadius: 18, border: `1.5px dashed ${c.muted}`, opacity: 0.4, pointerEvents: 'none' } },
    ];

    const out: FlowNode[] = [...lanes];

    if (orchestrator) {
      const color = agentColor(theme, agentStatus(orchestrator, gates));
      out.push({
        id: `agent_${orchestrator.id}`, type: 'card', position: { x: centerX, y: ORCH_Y },
        data: { label: <OrchestratorBody a={orchestrator} c={c} color={color} />, handles: H_ORCH, tone: color },
        style: { width: 196, padding: 13, borderRadius: 14, border: `1.5px solid ${color}`, background: c.paper, boxShadow: `0 0 0 4px ${color}1f` },
      });
    }

    for (const a of roster) {
      const status = agentStatus(a, gates);
      const color = agentColor(theme, status);
      out.push({
        id: `agent_${a.id}`, type: 'card', position: { x: (colOf.get(a.id) ?? 0) * COL, y: AGENT_Y },
        data: { label: <AgentBody a={a} status={status} color={color} c={c} />, handles: H_AGENT, tone: color },
        style: { width: 188, padding: 12, borderRadius: 12, border: `1.5px solid ${color}`, background: c.paper, boxShadow: `0 0 0 4px ${color}1a` },
      });
    }

    // Gates owned by a roster agent sit under that agent's column.
    for (const a of roster) {
      if (!a.gateId || !gateIds.has(a.gateId)) continue;
      const g = gates.find((x) => x.id === a.gateId)!;
      const color = statusColor(theme, g.status);
      out.push({
        id: `gate_${g.id}`, type: 'card', position: { x: (colOf.get(a.id) ?? 0) * COL, y: GATE_Y },
        data: { label: <GateBody g={g} color={color} c={c} />, handles: H_GATE, tone: color },
        style: { width: 188, padding: 12, borderRadius: 12, border: `1.5px solid ${color}`, background: c.paper, boxShadow: `0 0 0 4px ${color}1a` },
      });
    }

    return out;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sig, theme, c]);

  const edges = useMemo<FlowEdge[]>(() => {
    const out: FlowEdge[] = [];
    for (const a of roster) {
      const status = agentStatus(a, gates);
      const color = agentColor(theme, status);
      // Orchestrator routes to every agent.
      if (orchestrator) {
        out.push({
          id: `route_${a.id}`, source: `agent_${orchestrator.id}`, target: `agent_${a.id}`,
          sourceHandle: 'b', targetHandle: 't', animated: status === 'working',
          style: { stroke: color, strokeWidth: 1.3, opacity: 0.4 },
        });
      }
      // Agent owns a gate.
      if (a.gateId && gateIds.has(a.gateId)) {
        out.push({
          id: `own_${a.id}`, source: `agent_${a.id}`, target: `gate_${a.gateId}`,
          sourceHandle: 'b', targetHandle: 't', animated: status === 'working',
          style: { stroke: color, strokeWidth: 1.5, strokeDasharray: '4 5', opacity: 0.8 },
        });
      }
      // Delegation hand-offs.
      for (const to of a.handoffTo ?? []) {
        if (to === orchestrator?.id || !roster.some((x) => x.id === to)) continue;
        out.push({
          id: `handoff_${a.id}_${to}`, source: `agent_${a.id}`, target: `agent_${to}`,
          sourceHandle: 'hr', targetHandle: 'hl', animated: false,
          label: 'hands off', labelStyle: { fontSize: 8, fill: c.accent },
          style: { stroke: c.accent, strokeWidth: 1.4, strokeDasharray: '2 4', opacity: 0.7 },
        });
      }
    }
    return out;
  }, [roster, gates, gateIds, orchestrator, theme, c.accent]);

  return (
    <Box sx={{ height: { xs: 460, md: 600 }, borderRadius: 3, border: 1, borderColor: 'divider', overflow: 'hidden', bgcolor: 'background.paper' }}>
      <FlowCanvas nodes={nodes} edges={edges} onNodeClick={onNodeClick} minimap />
    </Box>
  );
}

function OrchestratorBody({ a, c, color }: { a: Agent; c: MapColors; color: string }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 6, textAlign: 'left', width: '100%' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
        <VscHubot size={14} color={color} />
        <span style={{ fontFamily: c.mono, fontSize: 9, fontWeight: 700, letterSpacing: '0.1em', color }}>ORCHESTRATOR</span>
      </div>
      <div style={{ fontSize: 13.5, fontWeight: 600, color: c.primary, lineHeight: 1.15 }}>{a.name}</div>
      <div style={{ fontSize: 9.5, color: c.secondary, lineHeight: 1.25 }}>Routes work to every specialist below.</div>
    </div>
  );
}

function AgentBody({ a, status, color, c }: { a: Agent; status: AgentStatus; color: string; c: MapColors }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 5, textAlign: 'left', width: '100%' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
        <VscRobot size={12} color={color} />
        <span style={{ fontFamily: c.mono, fontSize: 8.5, fontWeight: 700, letterSpacing: '0.08em', color }}>{STATUS_TONE[status].toUpperCase()}</span>
        <span style={{ flex: 1 }} />
        {a.canDelegate && <VscGitMerge size={11} color={c.accent} />}
        {a.custom && <span style={{ fontFamily: c.mono, fontSize: 8, color: c.muted }}>CUSTOM</span>}
      </div>
      <div style={{ fontSize: 12.5, fontWeight: 600, color: c.primary, lineHeight: 1.15 }}>{a.name || 'Untitled agent'}</div>
      <div style={{ fontSize: 9, color: c.secondary, lineHeight: 1.25, display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden', minHeight: 11 }}>{a.role}</div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 5, marginTop: 1 }}>
        <span style={{ fontFamily: c.mono, fontSize: 8, color: c.muted }}>{(a.skills ?? []).length} skills</span>
        <span style={{ fontFamily: c.mono, fontSize: 8, color: c.muted }}>·</span>
        <span style={{ fontFamily: c.mono, fontSize: 8, color: c.muted }}>{autonomyLabel(a.autonomy)}</span>
      </div>
    </div>
  );
}

function GateBody({ g, color, c }: { g: Gate; color: string; c: MapColors }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 5, textAlign: 'left', width: '100%' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
        <span style={{ fontFamily: c.mono, fontSize: 9, color: c.muted }}>{g.no}</span>
        <span style={{ flex: 1 }} />
        <span style={{ fontFamily: c.mono, fontSize: 8.5, fontWeight: 700, letterSpacing: '0.06em', color }}>{statusLabel[g.status].toUpperCase()}</span>
      </div>
      <div style={{ fontSize: 12, fontWeight: 600, color: c.primary, lineHeight: 1.15 }}>{g.name}</div>
      <div style={{ height: 4, borderRadius: 99, background: c.track, overflow: 'hidden' }}>
        <div style={{ height: '100%', width: `${Math.round(g.level * 100)}%`, background: color, transition: 'width .4s ease' }} />
      </div>
      <div style={{ fontSize: 9, lineHeight: 1.3, color: g.blocking ? c.secondary : c.success, display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden', minHeight: 11 }}>
        {g.blocking ? `● ${g.blocking}` : '● Closed end-to-end'}
      </div>
    </div>
  );
}
