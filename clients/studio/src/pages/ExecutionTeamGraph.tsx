import { useCallback, useMemo, useState } from 'react';
import { Box, Button, Chip, LinearProgress, Stack, Tooltip, Typography } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { FlowCanvas, type FlowNode, type FlowEdge, type NodeMouseHandler, type HandleSpec } from '@ironflyer/ui-web/fx';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { formatUSD } from '@ironflyer/core';
import { VscRobot, VscPlay, VscWand } from 'react-icons/vsc';
import { statusColor, agentColor } from '../components/statusColor';
import { nodePalette, type MapColors } from '../components/map/nodes';
import { TechIcon } from '../lib/techIcons';
import {
  AGENTS, agentStatus, statusLabel,
  type Agent, type AgentStatus, type Gate, type StudioProject,
} from '../studioData';
import { mapGate, type GateVerdict } from '../lib/liveGates';
import { useStudio } from '../store';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { useProjectExecutions } from '../hooks/useLatestExecution';
import { useDispatchAgent } from '../hooks/useDispatchAgent';
import { NeonConstellation3D } from '../components/studio';
import { GlassPanel, StatCard } from '../components/studio';
import { studioTokens } from '../theme';
import { text } from '@ironflyer/design-tokens/brand';
import type { Constellation3DNode, Constellation3DLink } from '@ironflyer/ui-web/fx';

// Geometry — agents sit in a top lane, the gates they own sit beneath them in
// the pipeline lane, and unowned cross-cutting agents anchor to the run itself.
const COL = 220; // horizontal spacing between team columns
const AGENT_Y = -150; // agents lane
const GATE_Y = 150; // gates lane
const RUN_ID = '__run__';

const H_AGENT: HandleSpec[] = [{ id: 'b', type: 'source', side: 'bottom' }];
const H_GATE: HandleSpec[] = [{ id: 't', type: 'target', side: 'top' }];
const H_RUN: HandleSpec[] = [{ id: 't', type: 'target', side: 'top' }];

const STATUS_TONE: Record<AgentStatus, string> = {
  idle: 'Idle', working: 'Working', blocked: 'Blocked', done: 'Done',
};

// Map agent status to a neon series color index from the studio chart palette
// so the constellation always uses the locked neon arc, never raw values.
const STATUS_IDX: Record<AgentStatus, number> = { working: 1, blocked: 6, done: 4, idle: 3 };

// Live execution-team graph: the orchestrator's specialist roster rendered as
// either a living NeonConstellation3D (default glanceable mode) or the classic
// FlowCanvas tether view. Both are mirrors of real gate state — who is working,
// who is blocked, what is not closed end-to-end.
export function ExecutionTeamGraph({
  project,
  onOpenGate,
  constellationMode = false,
}: {
  project: StudioProject;
  onOpenGate?: (gateId: string) => void;
  constellationMode?: boolean;
}) {
  const theme = useTheme();
  const c = useMemo(() => nodePalette(theme), [theme]);
  const selectGate = useStudio((s) => s.selectGate);
  const selectedGateId = useStudio((s) => s.selectedGateId);
  const customAgents = useStudio((s) => s.customAgents);
  const { dispatch } = useDispatchAgent();
  const [viewMode, setViewMode] = useState<'constellation' | 'flow'>(constellationMode ? 'constellation' : 'flow');

  const firstProjectId = useLiveProjectId();
  const storeProjectId = useStudio((s) => s.liveProjectId);
  const liveProjectId = storeProjectId ?? firstProjectId;

  const { data: liveGates, isLive } = useGraphQLQuery<Gate[], { gates: GateVerdict[] }>({
    key: ['team-gates', liveProjectId ?? 'none'],
    operationName: 'Gates', query: operations.GATES,
    variables: { projectId: liveProjectId }, fallbackData: [], enabled: !!liveProjectId,
    refetchInterval: 6000, map: (raw) => raw.gates.map(mapGate),
  });
  const { economics } = useProjectExecutions(liveProjectId);

  // Anchor `gates` to a content signature so identical polls keep the same
  // reference and the graph stops rebuilding/re-fitting.
  const liveSig = useMemo(() => liveGates.map((g) => `${g.id}|${g.status}|${g.blocking ? 1 : 0}`).join(';'), [liveGates]);
  const gates = useMemo(
    () => (liveGates.length > 0 ? liveGates : project.gates),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [liveSig, project.gates],
  );

  // The whole team = the built-in orchestrator roster + the operator's own agents.
  const team = useMemo<Agent[]>(() => [...AGENTS, ...customAgents], [customAgents]);
  const gateIds = useMemo(() => new Set(gates.map((g) => g.id)), [gates]);

  const counts = useMemo(() => {
    const tally: Record<AgentStatus, number> = { idle: 0, working: 0, blocked: 0, done: 0 };
    for (const a of team) tally[agentStatus(a, gates)] += 1;
    return tally;
  }, [team, gates]);
  const blockedGates = gates.filter((g) => g.blocking);

  // ── Constellation data ───────────────────────────────────────────────────
  // Each agent is a node sized by activity level; gate nodes are smaller and
  // positioned below. Links = ownership (agent→gate) and handoffs (agent→agent).
  // Colors from the locked neon chart series so the 3D world matches the rest.
  const series = theme.studio.chart.series as unknown as string[];

  const constellationData = useMemo<{ nodes: Constellation3DNode[]; links: Constellation3DLink[] }>(() => {
    const nodes: Constellation3DNode[] = [];
    const links: Constellation3DLink[] = [];
    const cols = team.length;
    const spread = 3;

    // Agent nodes — sized by status-derived activity
    for (let i = 0; i < team.length; i++) {
      const a = team[i]!;
      const status = agentStatus(a, gates);
      const colorIdx = STATUS_IDX[status];
      const color = series[colorIdx % series.length];
      const activity = status === 'working' ? 0.9 : status === 'done' ? 0.6 : status === 'blocked' ? 0.7 : 0.3;
      const angle = (i / cols) * Math.PI * 2;
      const r = 2.5;
      nodes.push({
        id: a.id,
        value: activity,
        color,
        x: r * Math.cos(angle),
        y: 1 + (status === 'working' ? 0.5 : 0),
        z: r * Math.sin(angle),
      });
    }

    // Gate nodes — smaller, layered below the agents
    for (let i = 0; i < gates.length; i++) {
      const g = gates[i]!;
      const gColor = g.blocking ? series[6 % series.length] : series[4 % series.length];
      const angle = (i / Math.max(gates.length, 1)) * Math.PI * 2;
      const r = 1.5;
      nodes.push({
        id: `gate_${g.id}`,
        value: g.level,
        color: gColor,
        x: r * Math.cos(angle) * spread * 0.4,
        y: -1.2,
        z: r * Math.sin(angle) * spread * 0.4,
      });
    }

    // Agent → gate ownership links
    for (const a of team) {
      if (a.gateId && gateIds.has(a.gateId)) {
        links.push({ source: a.id, target: `gate_${a.gateId}` });
      }
    }

    // Handoff links (agent → agent)
    for (const a of team) {
      for (const to of a.handoffTo ?? []) {
        if (team.some((x) => x.id === to)) {
          links.push({ source: a.id, target: to });
        }
      }
    }

    return { nodes, links };
  }, [team, gates, gateIds, series]);

  // One content signature for everything the graph renders.
  const teamSig = team.map((a) => `${a.id}:${a.gateId ?? ''}:${agentStatus(a, gates)}`).join('|');
  const nodeSig = [liveSig || gates.map((g) => `${g.id}|${g.status}|${g.blocking ? 1 : 0}`).join(';'), teamSig].join('~');

  const onRun = useCallback((scope: string) => void dispatch(scope), [dispatch]);

  // ── FlowCanvas nodes/edges (unchanged) ──────────────────────────────────
  const nodes = useMemo<FlowNode[]>(() => {
    const ownedGates = gates.filter((g) => team.some((a) => a.gateId === g.id));
    const gateCol = new Map<string, number>();
    ownedGates.forEach((g, i) => gateCol.set(g.id, i));
    let freeCol = ownedGates.length;
    const agentCol = new Map<string, number>();
    for (const a of team) {
      if (a.gateId && gateCol.has(a.gateId)) agentCol.set(a.id, gateCol.get(a.gateId)!);
      else agentCol.set(a.id, freeCol++);
    }
    const cols = Math.max(freeCol, ownedGates.length, 1);
    const laneW = cols * COL + 40;
    const laneX = -40;

    const laneNodes: FlowNode[] = [
      { id: 'lane_team', type: 'lane', position: { x: laneX, y: AGENT_Y - 44 }, draggable: false, selectable: false, zIndex: -1, data: { label: 'Execution team' }, style: { width: laneW, height: 150, borderRadius: 18, border: `1.5px dashed ${c.accent}`, opacity: 0.4, pointerEvents: 'none' } },
      { id: 'lane_gates', type: 'lane', position: { x: laneX, y: GATE_Y - 44 }, draggable: false, selectable: false, zIndex: -1, data: { label: 'Finisher gates' }, style: { width: laneW, height: 158, borderRadius: 18, border: `1.5px dashed ${c.muted}`, opacity: 0.45, pointerEvents: 'none' } },
    ];

    const agentNodes: FlowNode[] = team.map((a) => {
      const status = agentStatus(a, gates);
      const color = agentColor(theme, status);
      const x = (agentCol.get(a.id) ?? 0) * COL;
      return {
        id: `team_${a.id}`, type: 'card', position: { x, y: AGENT_Y },
        data: { label: <AgentNode a={a} status={status} color={color} c={c} onRun={() => onRun(a.gateId ? `the ${a.name} task` : a.name || 'the agent task')} />, handles: H_AGENT, tone: color },
        style: { width: 196, padding: 12, borderRadius: 12, border: `1.5px solid ${color}`, background: c.paper, boxShadow: `0 0 0 4px ${color}1f` },
      };
    });

    const gateNodes: FlowNode[] = ownedGates.map((g) => {
      const color = statusColor(theme, g.status);
      const live = g.status === 'running';
      const x = (gateCol.get(g.id) ?? 0) * COL;
      return {
        id: `gate_${g.id}`, type: 'card', position: { x, y: GATE_Y },
        data: { label: <GateNode g={g} color={color} c={c} />, handles: H_GATE, tone: color },
        style: { width: 196, padding: 12, borderRadius: 12, border: `1.5px solid ${color}`, background: c.paper, boxShadow: `0 0 0 ${live ? 5 : 4}px ${color}${live ? '38' : '1f'}` },
      };
    });

    const open = blockedGates.length;
    const runColor = open === 0 && gates.length > 0 ? c.success : open > 0 ? c.warn : c.muted;
    const runNode: FlowNode = {
      id: RUN_ID, type: 'card', position: { x: (cols * COL) / 2 - 100, y: GATE_Y + 180 },
      data: { label: <RunNode open={open} total={gates.length} spentUSD={economics.spentUSD} budgetUSD={economics.budgetUSD} color={runColor} c={c} />, handles: H_RUN, tone: runColor },
      style: { width: 220, padding: 14, borderRadius: 14, border: `1.5px solid ${runColor}`, background: c.paper, boxShadow: `0 0 0 4px ${runColor}1f` },
    };

    return [...laneNodes, ...agentNodes, ...gateNodes, runNode];
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [nodeSig, theme, c, onRun, economics.spentUSD, economics.budgetUSD]);

  const edges = useMemo<FlowEdge[]>(() => {
    const out: FlowEdge[] = [];
    for (const a of team) {
      const status = agentStatus(a, gates);
      const owns = a.gateId && gateIds.has(a.gateId);
      const target = owns ? `gate_${a.gateId}` : RUN_ID;
      const color = agentColor(theme, status);
      out.push({
        id: `team_${a.id}->${target}`, source: `team_${a.id}`, target,
        sourceHandle: 'b', targetHandle: 't', animated: status === 'working',
        style: { stroke: color, strokeWidth: status === 'blocked' ? 2 : 1.4, strokeDasharray: owns ? undefined : '3 6', opacity: owns ? 0.85 : 0.6 },
      });
    }
    return out;
  }, [team, gates, gateIds, theme]);

  const onNodeClick: NodeMouseHandler = (_e, node) => {
    if (node.id.startsWith('gate_')) {
      const id = node.id.slice('gate_'.length);
      selectGate(id);
      onOpenGate?.(id);
    }
  };

  const total = team.length;
  const open = blockedGates.length;

  return (
    <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0, height: '100%', bgcolor: 'background.default' }}>
      {/* ── Header ─────────────────────────────────────────────────────── */}
      <Box sx={{ px: 3, py: 1.75, borderBottom: 1, borderColor: 'divider', bgcolor: 'background.paper' }}>
        <Stack direction="row" alignItems="center" spacing={1.5} sx={{ mb: 1.25, flexWrap: 'wrap' }}>
          <Typography variant="h5" sx={{ fontSize: text.s115 }}>
            {viewMode === 'constellation' ? 'Execution team constellation' : 'Execution team flow'}
          </Typography>
          <Chip
            size="small"
            label={isLive ? 'live' : 'sample data'}
            sx={(t) => ({ height: 20, fontSize: text.s62, fontFamily: t.brand.font.mono, bgcolor: isLive ? `${t.palette.success.main}22` : 'action.hover', color: isLive ? 'success.main' : 'text.disabled' })}
          />
          <Box sx={{ flex: 1 }} />
          <Typography sx={{ color: 'text.secondary', fontSize: text.s85 }}>
            {counts.working} working · {counts.blocked} blocked · {open} of {gates.length} gates open
          </Typography>
          <Tooltip title={viewMode === 'constellation' ? 'Switch to flow canvas' : 'Switch to constellation view'} arrow>
            <Button
              size="small"
              variant="outlined"
              color="inherit"
              onClick={() => setViewMode((v) => v === 'constellation' ? 'flow' : 'constellation')}
              sx={(t) => ({
                fontSize: text.s70,
                borderColor: t.palette.divider,
                '&:hover': { borderColor: t.studio.neon.blue },
              })}
            >
              {viewMode === 'constellation' ? 'Flow view' : 'Constellation'}
            </Button>
          </Tooltip>
        </Stack>
        <LinearProgress
          variant="determinate"
          value={total ? Math.round((counts.done / total) * 100) : 0}
          sx={{ height: 5, borderRadius: 99, bgcolor: 'action.hover', mb: 1.5, '& .MuiLinearProgress-bar': { borderRadius: 99, backgroundImage: (t) => t.brand.gradient.signature } }}
        />
        <Stack direction="row" spacing={1} sx={{ flexWrap: 'wrap' }}>
          {(['working', 'blocked', 'idle', 'done'] as AgentStatus[]).map((s) => (
            <Chip key={s} size="small" label={`${STATUS_TONE[s]} ${counts[s]}`} sx={(t) => ({ height: 22, fontSize: text.s66, fontFamily: t.brand.font.mono, bgcolor: `${agentColor(t, s)}22`, color: agentColor(t, s) })} />
          ))}
        </Stack>
      </Box>

      {/* ── Main content ────────────────────────────────────────────────── */}
      {viewMode === 'constellation' ? (
        <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0, p: 2, gap: 2 }}>
          {/* 3D constellation — the living mirror of the AI team */}
          <GlassPanel
            accent={theme.studio.neon.violet}
            sx={{ flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}
          >
            <Typography
              sx={(t) => ({
                fontFamily: t.brand.font.mono,
                fontSize: text.s62,
                letterSpacing: '0.12em',
                textTransform: 'uppercase',
                color: 'text.disabled',
                mb: 1,
              })}
            >
              Agents · gates · handoffs — live
            </Typography>
            <Box sx={{ flex: 1, minHeight: 280 }}>
              <NeonConstellation3D
                nodes={constellationData.nodes}
                links={constellationData.links}
                height={420}
                rotate
              />
            </Box>
          </GlassPanel>

          {/* Status cards rail */}
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', sm: 'repeat(4, 1fr)' }, gap: 1.5 }}>
            <StatCard
              label="Working"
              value={counts.working}
              accent={studioTokens.neon.blue}
              hint={`of ${total} agents`}
            />
            <StatCard
              label="Blocked"
              value={counts.blocked}
              accent={counts.blocked > 0 ? studioTokens.neon.danger : studioTokens.neon.success}
              hint={counts.blocked > 0 ? 'need action' : 'none blocked'}
            />
            <StatCard
              label="Gates open"
              value={open}
              accent={open > 0 ? studioTokens.neon.warning : studioTokens.neon.success}
              hint={`of ${gates.length} total`}
            />
            <StatCard
              label="Spent"
              value={formatUSD(economics.spentUSD)}
              hint={`of ${formatUSD(economics.budgetUSD)}`}
              accent={studioTokens.neon.violet}
            />
          </Box>
        </Box>
      ) : (
        <Box sx={{ flex: 1, position: 'relative', minHeight: 0 }}>
          <FlowCanvas nodes={nodes} edges={edges} onNodeClick={onNodeClick} minimap focusable focusId={selectedGateId ? `gate_${selectedGateId}` : undefined} />
        </Box>
      )}
    </Box>
  );
}

function AgentNode({ a, status, color, c, onRun }: { a: Agent; status: AgentStatus; color: string; c: MapColors; onRun: () => void }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 6, textAlign: 'left', width: '100%' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
        <VscRobot size={13} color={color} />
        <span style={{ fontFamily: c.mono, fontSize: 9, fontWeight: 700, letterSpacing: '0.08em', color }}>{STATUS_TONE[status].toUpperCase()}</span>
        <span style={{ flex: 1 }} />
        {a.custom && <span style={{ fontFamily: c.mono, fontSize: 8, color: c.muted }}>CUSTOM</span>}
      </div>
      <div style={{ fontSize: 13, fontWeight: 600, color: c.primary, lineHeight: 1.15 }}>{a.name || 'Untitled agent'}</div>
      <div style={{ fontSize: 9.5, color: c.secondary, lineHeight: 1.25, display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden', minHeight: 12 }}>{a.role}</div>
      <Stack className="nodrag nopan" direction="row" alignItems="center" spacing={0.5} sx={{ mt: 0.25 }}>
        <Tooltip title={`Dispatch ${a.name || 'agent'}`} arrow>
          <Box component="span" role="button" onClick={(e) => { e.stopPropagation(); onRun(); }} sx={{ display: 'inline-flex', alignItems: 'center', gap: 0.5, cursor: 'pointer', color: status === 'blocked' ? 'primary.main' : 'success.main' }}>
            {status === 'blocked' ? <VscWand size={13} /> : <VscPlay size={13} />}
            <Typography sx={{ fontSize: text.s66 }}>{status === 'blocked' ? 'Unblock' : 'Run'}</Typography>
          </Box>
        </Tooltip>
      </Stack>
    </div>
  );
}

function GateNode({ g, color, c }: { g: Gate; color: string; c: MapColors }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 6, textAlign: 'left', width: '100%' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
        <Box component="span" sx={{ color: 'text.secondary', display: 'inline-flex' }}><TechIcon name={g.id} size={14} title={g.name} /></Box>
        <span style={{ fontFamily: c.mono, fontSize: 9.5, color: c.muted }}>{g.no}</span>
        <span style={{ flex: 1 }} />
        <span style={{ fontFamily: c.mono, fontSize: 9, fontWeight: 700, letterSpacing: '0.06em', color }}>{statusLabel[g.status].toUpperCase()}</span>
      </div>
      <div style={{ fontSize: 12.5, fontWeight: 600, color: c.primary, lineHeight: 1.15 }}>{g.name}</div>
      <div style={{ height: 4, borderRadius: 99, background: c.track, overflow: 'hidden' }}>
        <div style={{ height: '100%', width: `${Math.round(g.level * 100)}%`, background: color, transition: 'width .4s ease' }} />
      </div>
      <div style={{ fontSize: 10, lineHeight: 1.3, color: g.blocking ? c.secondary : c.success, display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden', minHeight: 13 }}>
        {g.blocking ? `● ${g.blocking}` : '● Closed end-to-end'}
      </div>
    </div>
  );
}

function RunNode({ open, total, spentUSD, budgetUSD, color, c }: { open: number; total: number; spentUSD: number; budgetUSD: number; color: string; c: MapColors }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 6, textAlign: 'left', width: '100%' }}>
      <span style={{ fontFamily: c.mono, fontSize: 9.5, fontWeight: 700, letterSpacing: '0.1em', color }}>EXECUTION</span>
      <div style={{ fontSize: 13.5, fontWeight: 600, color: c.primary, lineHeight: 1.1 }}>
        {open === 0 && total > 0 ? 'All gates closed — shippable' : `${open} gate${open === 1 ? '' : 's'} not closed end-to-end`}
      </div>
      <div style={{ fontSize: 10.5, color: c.secondary, lineHeight: 1.3 }}>{formatUSD(spentUSD)} spent of {formatUSD(budgetUSD)}</div>
    </div>
  );
}
