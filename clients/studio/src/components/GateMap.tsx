import { useCallback, useMemo, useState } from 'react';
import { Box, Button, Chip, IconButton, LinearProgress, Stack, Tooltip, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { FlowCanvas, type FlowNode, type FlowEdge, type NodeMouseHandler, type HandleSpec, toast } from '@ironflyer/ui-web/fx';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { formatUSD, formatRelativeTime } from '@ironflyer/core';
import { VscRobot } from 'react-icons/vsc';
import { statusColor, agentColor } from './statusColor';
import { ActivityFeed } from './ActivityFeed';
import { GateNodeLabel, FacetNodeLabel, VisionBody, ShipBody, nodePalette, type FacetNodeData } from './map/nodes';
import { agentForGate, agentStatus, scheduleLabel, statusLabel, type Agent, type Gate, type StudioProject } from '../studioData';
import { mapGate, type GateVerdict } from '../lib/liveGates';
import type { EditorTab } from './EditorTopBar';
import { useStudio } from '../store';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { useProjectExecutions } from '../hooks/useLatestExecution';
import { useSentinelForecast } from '../hooks/useEconomics';
import { useDispatchAgent } from '../hooks/useDispatchAgent';
import { useAgentTeam } from '../hooks/useAgentTeam';
import { text } from '@ironflyer/design-tokens/brand';

const VISION_ID = '__vision__';
const SHIP_ID = '__ship__';
const COL = 252; // horizontal spacing between pipeline columns
const ZIG = 78; // vertical offset for alternating gates so edges stay legible
const FACET_Y = 392; // the cross-cutting instruments lane sits below the pipeline

// Connection-point declarations. The card node renders these as invisible
// handles, so edges anchor precisely (pipeline flows L→R; agents tether down
// into a gate's top; instruments rise into ship's underside).
const H_FLOW: HandleSpec[] = [{ id: 'l', type: 'target', side: 'left' }, { id: 'r', type: 'source', side: 'right' }, { id: 't', type: 'target', side: 'top' }];
const H_VISION: HandleSpec[] = [{ id: 'r', type: 'source', side: 'right' }, { id: 't', type: 'target', side: 'top' }];
const H_SHIP: HandleSpec[] = [{ id: 'l', type: 'target', side: 'left' }, { id: 'b', type: 'target', side: 'bottom' }];
const H_FACET: HandleSpec[] = [{ id: 't', type: 'source', side: 'top' }];
const H_AGENT: HandleSpec[] = [{ id: 'b', type: 'source', side: 'bottom' }];

// Finisher gates that are performance audits — used to summarize the Perf facet.
const PERF_IDS = new Set(['lighthouse', 'bundle_size', 'perf_budget', 'mobile_size', 'mobile_bundle_analyzer', 'mem_leak', 'complexity', 'dep_graph', 'arch_boundary']);

// Performance now lives as an inner tab of the consolidated Quality workspace,
// so its facet opens 'quality' and asks it to land on the Performance sub-tab.
const FACET_TAB: Record<string, { tab: EditorTab; inner?: string }> = {
  facet_security: { tab: 'security' },
  facet_performance: { tab: 'quality', inner: 'performance' },
  facet_economics: { tab: 'dashboard' },
  facet_logs: { tab: 'logs' },
};

interface SecReport { overallScore: number; secretsFound: number; outdatedDeps: number; blockedDeploy: boolean; findings: { id: string }[] }
const EMPTY_SEC: SecReport = { overallScore: 1, secretsFound: 0, outdatedDeps: 0, blockedDeploy: false, findings: [] };

function quietGate(g: Gate, selected: boolean): Gate {
  if (selected || !g.blocking) return g;
  const openItems = g.findings.length + g.patches.filter((p) => p.state === 'proposed').length;
  return {
    ...g,
    blocking: openItems > 0
      ? `${openItems} review item${openItems === 1 ? '' : 's'} open`
      : statusLabel[g.status],
  };
}

function deployColor(t: Theme, status: StudioProject['deploy']['status']): string {
  switch (status) {
    case 'production': return t.palette.success.main;
    case 'preview': return t.brand.accent.secondary;
    case 'failed': return t.palette.error.main;
    default: return t.palette.text.disabled;
  }
}

// Viz-first project map: the whole build on one canvas — the product vision, the
// finisher pipeline, the operator's agents, and the cross-cutting instruments
// (security, performance, economics, logs) — flowing live toward ship.
export function GateMap({ project, onOpenTab }: { project: StudioProject; onOpenTab?: (t: EditorTab) => void }) {
  const theme = useTheme();
  const c = useMemo(() => nodePalette(theme), [theme]);
  const selectGate = useStudio((s) => s.selectGate);
  const setInnerTab = useStudio((s) => s.setInnerTab);
  const selectedGateId = useStudio((s) => s.selectedGateId);
  // Open a facet's destination tab, optionally deep-linking an inner sub-tab.
  const openFacet = useCallback((tab: EditorTab, inner?: string) => {
    if (inner) setInnerTab(inner);
    onOpenTab?.(tab);
  }, [onOpenTab, setInnerTab]);
  const constitution = useStudio((s) => s.constitution);
  const customAgents = useStudio((s) => s.customAgents);
  const { online, dispatch } = useDispatchAgent();
  const { saveAgent } = useAgentTeam();

  // Persist a gate→agent assignment: set the chosen custom agent's gateId so the
  // map's tether and the orchestrator's routing both reflect the ownership.
  // Built-in roster agents already own their gate, so they can't be reassigned.
  const assignAgent = useCallback((gateId: string, agentId: string) => {
    const a = customAgents.find((x) => x.id === agentId);
    if (!a) { toast('Built-in agents already own their gate — pick one of your agents to reassign.', 'info'); return; }
    void saveAgent({ ...a, gateId });
    toast(`${a.name || 'Agent'} now owns the ${gateId} gate.`, 'success');
  }, [customAgents, saveAgent]);

  const firstProjectId = useLiveProjectId();
  const storeProjectId = useStudio((s) => s.liveProjectId);
  const liveProjectId = storeProjectId ?? firstProjectId;

  const { data: liveGates, isLive } = useGraphQLQuery<Gate[], { gates: GateVerdict[] }>({
    key: ['map-gates', liveProjectId ?? 'none'],
    operationName: 'Gates', query: operations.GATES,
    variables: { projectId: liveProjectId }, fallbackData: [], enabled: !!liveProjectId,
    refetchInterval: 6000, map: (raw) => raw.gates.map(mapGate),
  });
  const { latest, economics } = useProjectExecutions(liveProjectId);
  const { forecast, isLive: forecastLive } = useSentinelForecast(liveProjectId);
  const { data: sec } = useGraphQLQuery<SecReport, { executionSecurityReport: SecReport }>({
    key: ['map-sec', latest?.id ?? 'none'], operationName: 'ExecutionSecurityReport', query: operations.EXECUTION_SECURITY_REPORT,
    variables: { executionID: latest?.id }, fallbackData: EMPTY_SEC, enabled: !!latest?.id, refetchInterval: 15000,
    map: (r) => r.executionSecurityReport ?? EMPTY_SEC,
  });

  // Each live poll returns a freshly-mapped array, so anchor `gates` to a
  // content signature: identical polls keep the same reference and the graph
  // stops rebuilding/re-fitting (the source of the tab flicker). Once live data
  // is seen we never fall back to the sample, so the map reflects the project.
  const liveSig = useMemo(() => liveGates.map((g) => `${g.id}|${g.status}|${g.level}|${g.blocking ? 1 : 0}`).join(';'), [liveGates]);
  const gates = useMemo(
    () => (liveGates.length > 0 ? liveGates : project.gates),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [liveSig, project.gates],
  );
  const closed = gates.filter((g) => !g.blocking).length;
  const open = gates.length - closed;
  const completion = gates.length ? closed / gates.length : project.completion;
  const visionText = (constitution || project.source || 'Your product vision').trim();
  const gaps = gates.filter((g) => g.blocking);

  // Agent options offered on every gate node: the owning specialist + the
  // operator's own agents (what the user explicitly asked to pick from).
  const agentOptionsFor = useMemo(() => {
    const customs = customAgents.map((a) => ({ id: a.id, name: a.name || 'Untitled agent' }));
    return (gateId: string) => {
      const owner = agentForGate(gateId);
      const base = owner ? [{ id: owner.id, name: owner.name }] : [];
      return [...base, ...customs];
    };
  }, [customAgents]);

  // --- facet summaries (security / performance / economics / logs) -------
  const secLive = isLive && !!latest?.id;
  const riskScore = secLive ? Math.round((1 - sec.overallScore) * 100) : project.security.riskScore;
  const secFindings = secLive ? sec.findings.length : project.security.findings.length;
  const secSecrets = secLive ? sec.secretsFound : project.security.findings.filter((f) => f.category === 'secret').length;
  const secBlocked = secLive ? sec.blockedDeploy : project.security.policy.effect === 'deny';
  const perfGates = gates.filter((g) => PERF_IDS.has(g.id));
  const perfPass = perfGates.filter((g) => !g.blocking).length;
  const lastEvent = project.activity[0];

  // A single content signature for everything the node graph renders. The node
  // builder runs only when this changes — so background polls that return
  // identical numbers don't churn React Flow (no flicker, stable viewport).
  const customAgentsSig = customAgents.map((a) => `${a.id}:${a.name}:${a.gateId ?? ''}:${a.schedule?.enabled ? 1 : 0}`).join('|');
  const nodeSig = [
    liveSig || gates.map((g) => `${g.id}|${g.status}|${g.level}|${g.blocking ? 1 : 0}`).join(';'),
    `econ:${economics.spentUSD}:${economics.budgetUSD}:${economics.marginPct}:${economics.providerCostUSD}:${economics.runs}`,
    `fc:${forecastLive ? 1 : 0}:${forecast.level}:${forecast.burnRatePerHourUSD}:${forecast.extrapolatedTotalUSD}`,
    `sec:${riskScore}:${secFindings}:${secSecrets}:${secBlocked ? 1 : 0}`,
    `ship:${project.deploy.status}:${project.deploy.url ?? ''}`,
    `log:${lastEvent?.id ?? ''}`,
    `vis:${visionText}`,
    `ag:${customAgentsSig}`,
    `sel:${selectedGateId ?? ''}`,
  ].join('~');

  const nodes = useMemo<FlowNode[]>(() => {
    const mid = ((gates.length - 1) * ZIG) / 2;

    // Background swimlanes orient the three reading bands: who's working
    // (agents), the build itself (pipeline), and what gates shipping
    // (instruments). They sit behind everything and never capture pointer.
    const laneW = (gates.length + 2.4) * COL + 60;
    const laneX = -64;
    const laneNodes: FlowNode[] = [
      { id: 'lane_agents', type: 'lane', position: { x: laneX, y: -312 }, draggable: false, selectable: false, zIndex: -1, data: { label: 'Agents' }, style: { width: laneW, height: 196, borderRadius: 18, border: `1px dashed ${c.muted}`, opacity: 0.22, pointerEvents: 'none' } },
      { id: 'lane_pipeline', type: 'lane', position: { x: laneX, y: -56 }, draggable: false, selectable: false, zIndex: -1, data: { label: 'Finisher pipeline' }, style: { width: laneW, height: Math.max(300, mid + 150), borderRadius: 18, border: `1px dashed ${c.accent}`, opacity: 0.2, pointerEvents: 'none' } },
      { id: 'lane_facets', type: 'lane', position: { x: laneX, y: FACET_Y - 44 }, draggable: false, selectable: false, zIndex: -1, data: { label: 'Cross-cutting instruments' }, style: { width: laneW, height: 178, borderRadius: 18, border: `1px dashed ${c.muted}`, opacity: 0.22, pointerEvents: 'none' } },
    ];

    const visionNode: FlowNode = {
      id: VISION_ID, type: 'card', position: { x: 0, y: mid - 8 },
      data: { label: <VisionBody text={visionText} c={c} />, handles: H_VISION, tone: c.accent },
      style: { width: 196, padding: 14, borderRadius: 14, border: `1.5px solid ${c.accent}`, background: c.paper, boxShadow: `0 0 0 4px ${c.accent}1f` },
    };

    const gateNodes: FlowNode[] = gates.map((g, i) => {
      const color = statusColor(theme, g.status);
      const live = g.status === 'running';
      const selected = selectedGateId === g.id;
      const displayGate = quietGate(g, selected);
      return {
        id: g.id, type: 'card', position: { x: (i + 1) * COL, y: i % 2 === 0 ? 0 : ZIG * 2 },
        data: {
          tone: color, handles: H_FLOW,
          label: (
            <GateNodeLabel d={{
              kind: 'gate', gate: displayGate, ownerName: selected ? agentForGate(g.id)?.name : undefined,
              agentOptions: agentOptionsFor(g.id), online,
              onSelectGate: selectGate, onDispatch: (scope) => void dispatch(scope),
              onAssign: assignAgent,
            }} />
          ),
        },
        style: {
          width: selected ? 218 : 196,
          padding: selected ? 13 : 11,
          borderRadius: 12,
          border: `${selected ? 2 : 1.25}px solid ${color}`,
          background: c.paper,
          boxShadow: selected ? `0 0 0 5px ${color}2b` : live ? `0 0 0 4px ${color}2b` : `0 0 0 1px ${color}12`,
        },
      };
    });

    // Operator-created agents float above the pipeline, tethered to the gate
    // they own (or to the vision when unassigned).
    const gateIndex = new Map(gates.map((g, i) => [g.id, i]));
    const colStack = new Map<number, number>();
    const agentNodes: FlowNode[] = customAgents.map((a) => {
      const status = agentStatus(a, gates);
      const color = agentColor(theme, status);
      const gi = a.gateId ? gateIndex.get(a.gateId) : undefined;
      const x = gi != null ? (gi + 1) * COL : 0;
      const stack = colStack.get(x) ?? 0;
      colStack.set(x, stack + 1);
      return {
        id: a.id, type: 'card', position: { x, y: -188 - stack * 96 },
        data: { label: <AgentBody a={a} color={color} c={c} onRun={() => void dispatch(a.name || 'the agent task')} />, handles: H_AGENT, tone: color },
        style: { width: 178, padding: 11, borderRadius: 12, border: `1.5px dashed ${color}`, background: c.paper, boxShadow: `0 0 0 3px ${color}1f` },
      };
    });

    const shipColor = deployColor(theme, project.deploy.status);
    const shippable = open === 0 && gates.length > 0;
    const shipNode: FlowNode = {
      id: SHIP_ID, type: 'card', position: { x: (gates.length + 1) * COL, y: mid - 8 },
      data: { label: <ShipBody url={project.deploy.url ?? 'Deploy target'} color={shipColor} shippable={shippable} open={open} c={c} />, handles: H_SHIP, tone: shipColor },
      style: { width: 200, padding: 14, borderRadius: 14, border: `1.5px solid ${shipColor}`, background: c.paper, boxShadow: `0 0 0 4px ${shipColor}1f` },
    };

    // Cross-cutting instruments lane — security, performance, economics, logs.
    const perfColor = perfGates.length === 0 ? c.muted : perfPass === perfGates.length ? c.success : c.warn;
    const secColor = secBlocked ? c.error : riskScore >= 30 ? c.warn : c.success;
    const facetDefs: { id: string; tab: EditorTab; inner?: string; accent: string; data: Omit<FacetNodeData, 'kind' | 'onOpen'> }[] = [
      {
        id: 'facet_security', tab: 'security', accent: secColor,
        data: {
          iconKey: 'security', title: 'Security', metric: `risk ${riskScore}`, accent: secColor,
          sub: secBlocked ? 'deploy blocked — deny by default' : `${secFindings} finding${secFindings === 1 ? '' : 's'}`,
          details: [
            { label: 'Risk score', value: String(riskScore), color: secColor },
            { label: 'Findings', value: String(secFindings) },
            { label: 'Secrets', value: String(secSecrets) },
            { label: 'Deploy', value: secBlocked ? 'denied' : 'allowed', color: secBlocked ? c.error : c.success },
          ],
        },
      },
      {
        id: 'facet_performance', tab: 'quality', inner: 'performance', accent: perfColor,
        data: {
          iconKey: 'lighthouse', title: 'Performance', accent: perfColor,
          metric: perfGates.length ? `${perfPass}/${perfGates.length}` : (forecast.level || 'ok'),
          sub: perfGates.length ? 'audits passing' : `burn ${forecastLive ? formatUSD(forecast.burnRatePerHourUSD) + '/hr' : '—'}`,
          details: [
            { label: 'Audits passing', value: perfGates.length ? `${perfPass}/${perfGates.length}` : '—' },
            { label: 'Burn rate', value: forecastLive ? `${formatUSD(forecast.burnRatePerHourUSD)}/hr` : '—' },
            { label: 'Projected total', value: forecastLive ? formatUSD(forecast.extrapolatedTotalUSD) : '—' },
            { label: 'Trajectory', value: forecast.level || 'ok' },
          ],
        },
      },
      {
        id: 'facet_economics', tab: 'dashboard', accent: c.accent,
        data: {
          iconKey: 'ledger', title: 'Economics', metric: formatUSD(economics.spentUSD), accent: c.accent,
          sub: `${economics.marginPct}% margin · of ${formatUSD(economics.budgetUSD)}`,
          details: [
            { label: 'Spent', value: formatUSD(economics.spentUSD) },
            { label: 'Budget', value: formatUSD(economics.budgetUSD) },
            { label: 'Margin', value: `${economics.marginPct}%`, color: economics.marginPct >= 0 ? c.success : c.error },
            { label: 'Provider cost', value: formatUSD(economics.providerCostUSD) },
            { label: 'Executions', value: String(economics.runs) },
          ],
        },
      },
      {
        id: 'facet_logs', tab: 'logs', accent: theme.brand.accent.secondary,
        data: {
          iconKey: 'gate', title: 'Logs', metric: lastEvent ? formatRelativeTime(lastEvent.ts) : 'idle', accent: theme.brand.accent.secondary,
          sub: lastEvent ? lastEvent.text : 'no recent activity',
          details: project.activity.slice(0, 4).map((e) => ({ label: e.kind, value: e.text.length > 22 ? `${e.text.slice(0, 22)}…` : e.text })),
        },
      },
    ];
    const facetNodes: FlowNode[] = facetDefs.map((f, i) => ({
      id: f.id, type: 'card', position: { x: COL * (0.4 + i * 1.7), y: FACET_Y },
      data: { label: <FacetNodeLabel d={{ kind: 'facet', onOpen: () => openFacet(f.tab, f.inner), ...f.data }} />, handles: H_FACET, tone: f.accent },
      style: { width: 184, padding: 13, borderRadius: 12, border: `1.5px solid ${f.accent}`, background: c.paper, boxShadow: `0 0 0 4px ${f.accent}14` },
    }));

    return [...laneNodes, visionNode, ...gateNodes, shipNode, ...agentNodes, ...facetNodes];
    // Gated on the content signature so identical polls don't rebuild the graph.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [nodeSig, theme, c, online, dispatch, selectGate, agentOptionsFor, assignAgent, openFacet]);

  const edges = useMemo<FlowEdge[]>(() => {
    const out: FlowEdge[] = [];
    if (gates.length > 0) {
      const link = (source: string, target: string, srcGate?: Gate, tgtGate?: Gate) => {
        const blocked = srcGate?.status === 'blocked';
        const live = tgtGate?.status === 'running';
        const stroke = blocked ? theme.palette.error.main : live ? theme.brand.accent.secondary : theme.palette.divider;
        const label = blocked ? 'blocked' : live ? 'running' : undefined;
        out.push({
          id: `${source}->${target}`, source, target, sourceHandle: 'r', targetHandle: 'l', animated: live, label,
          labelStyle: { fill: stroke, fontFamily: theme.brand.font.mono, fontSize: 9, fontWeight: 700 },
          labelBgStyle: { fill: theme.palette.background.paper },
          labelBgPadding: [4, 2], labelBgBorderRadius: 4,
          style: { stroke, strokeWidth: blocked || live ? 2 : 1.5 },
        });
      };
      link(VISION_ID, gates[0]!.id, undefined, gates[0]);
      for (let i = 0; i < gates.length - 1; i++) link(gates[i]!.id, gates[i + 1]!.id, gates[i], gates[i + 1]);
      link(gates[gates.length - 1]!.id, SHIP_ID, gates[gates.length - 1], undefined);
    }
    // Operator agents tether down into the gate they own (vision when unassigned).
    const gateIds = new Set(gates.map((g) => g.id));
    for (const a of customAgents) {
      const status = agentStatus(a, gates);
      const target = a.gateId && gateIds.has(a.gateId) ? a.gateId : VISION_ID;
      out.push({
        id: `${a.id}->${target}`, source: a.id, target, sourceHandle: 'b', targetHandle: 't', animated: status === 'working',
        style: { stroke: agentColor(theme, status), strokeWidth: 1.3, strokeDasharray: '2 5', opacity: 0.7 },
      });
    }
    // Tether the cross-cutting instruments up into ship (they gate shippability).
    for (const fid of ['facet_security', 'facet_performance', 'facet_economics', 'facet_logs']) {
      out.push({ id: `${fid}->ship`, source: fid, target: SHIP_ID, sourceHandle: 't', targetHandle: 'b', animated: false, style: { stroke: theme.palette.divider, strokeWidth: 1.2, strokeDasharray: '3 6', opacity: 0.55 } });
    }
    return out;
  }, [gates, theme, customAgents]);

  const onNodeClick: NodeMouseHandler = (_e, node) => {
    if (node.id === VISION_ID || node.id === SHIP_ID || node.id.startsWith('agent_') || node.id.startsWith('lane_')) return;
    const facet = FACET_TAB[node.id];
    if (facet) { openFacet(facet.tab, facet.inner); return; }
    selectGate(node.id);
  };

  return (
    <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0, height: '100%', bgcolor: 'background.default' }}>
      <MapHeader
        isLive={isLive} completion={completion} open={open} total={gates.length}
        spentUSD={economics.spentUSD} budgetUSD={economics.budgetUSD} marginPct={economics.marginPct}
        pgVerdict={project.profitGuard.verdict}
        burnUSDPerHr={forecastLive ? forecast.burnRatePerHourUSD : null}
        etaCompletionAt={forecastLive ? forecast.etaCompletionAt ?? null : null}
        onRunAll={() => void dispatch('all open gates')}
      />
      <Box sx={{ flex: 1, display: 'flex', minHeight: 0 }}>
        <Box sx={{ flex: 1, minWidth: 0, position: 'relative' }}>
          <FlowCanvas nodes={nodes} edges={edges} onNodeClick={onNodeClick} minimap horizontal focusable focusId={selectedGateId} />
          <MapLegend />
        </Box>
        <GapsRail projectId={project.id} gaps={gaps} seed={project.activity} onPick={selectGate} onDispatch={(scope) => void dispatch(scope)} />
      </Box>
    </Box>
  );
}

function AgentBody({ a, color, c, onRun }: { a: Agent; color: string; c: ReturnType<typeof nodePalette>; onRun: () => void }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 5, textAlign: 'left', width: '100%' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
        <VscRobot size={12} color={color} />
        <span style={{ fontFamily: c.mono, fontSize: 8.5, fontWeight: 700, letterSpacing: '0.1em', color }}>AGENT</span>
      </div>
      <div style={{ fontSize: 12.5, fontWeight: 600, color: c.primary, lineHeight: 1.15 }}>{a.name || 'Untitled agent'}</div>
      {a.role && <div style={{ fontSize: 9.5, color: c.secondary, lineHeight: 1.25, display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden' }}>{a.role}</div>}
      <div style={{ fontFamily: c.mono, fontSize: 8.5, color: c.muted }}>⏱ {scheduleLabel(a.schedule)}</div>
      <Box className="nodrag nopan" sx={{ mt: 0.25 }}>
        <Tooltip title="Run this agent now" arrow>
          <IconButton size="small" onClick={(e) => { e.stopPropagation(); onRun(); }} sx={{ color: 'success.main', p: 0.4 }}><VscRobot size={13} /></IconButton>
        </Tooltip>
      </Box>
    </div>
  );
}

const pgColor = (t: Theme, v: StudioProject['profitGuard']['verdict']) =>
  v === 'block' ? t.palette.error.main : v === 'hold' ? t.palette.warning.main : t.palette.success.main;

function HeaderStat({ label, value, color }: { label: string; value: string; color?: string }) {
  return (
    <Box sx={{ minWidth: 0 }}>
      <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s60, letterSpacing: '0.08em', textTransform: 'uppercase', color: 'text.disabled' })}>{label}</Typography>
      <Typography sx={{ fontSize: text.s92, fontWeight: 600, color: color ?? 'text.primary' }} noWrap>{value}</Typography>
    </Box>
  );
}

// Compact status key floating over the canvas — what the node + edge colors mean.
const LEGEND: { label: string; status: Gate['status'] }[] = [
  { label: 'Closed', status: 'closed' },
  { label: 'Running', status: 'running' },
  { label: 'Open', status: 'open' },
  { label: 'Blocked', status: 'blocked' },
  { label: 'Not started', status: 'unstarted' },
];

function MapLegend() {
  return (
    <Box sx={{ position: 'absolute', top: 12, left: 12, zIndex: 4, px: 1.25, py: 1, borderRadius: 2, border: 1, borderColor: 'divider', bgcolor: (t) => `${t.palette.background.paper}e6`, backdropFilter: 'blur(6px)' }}>
      <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s58, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 0.75 })}>Status</Typography>
      <Stack spacing={0.5}>
        {LEGEND.map((l) => (
          <Stack key={l.status} direction="row" alignItems="center" spacing={0.85}>
            <Box sx={{ width: 8, height: 8, borderRadius: 99, bgcolor: (t) => statusColor(t, l.status) }} />
            <Typography sx={{ fontSize: text.s70, color: 'text.secondary' }}>{l.label}</Typography>
          </Stack>
        ))}
      </Stack>
    </Box>
  );
}

function MapHeader(props: {
  isLive: boolean; completion: number; open: number; total: number;
  spentUSD: number; budgetUSD: number; marginPct: number;
  pgVerdict: StudioProject['profitGuard']['verdict'];
  burnUSDPerHr: number | null; etaCompletionAt: string | null; onRunAll: () => void;
}) {
  const { isLive, completion, open, total, spentUSD, budgetUSD, marginPct, pgVerdict, burnUSDPerHr, etaCompletionAt, onRunAll } = props;
  return (
    <Box sx={{ px: 3, py: 1.75, borderBottom: 1, borderColor: 'divider', bgcolor: 'background.paper' }}>
      <Stack direction="row" alignItems="center" spacing={1.5} sx={{ mb: 1.25, flexWrap: 'wrap' }}>
        <Typography variant="h5" sx={{ fontSize: text.s115 }}>Project map</Typography>
        <Chip size="small" label={isLive ? 'live' : 'sample data'} sx={(t) => ({ height: 20, fontSize: text.s62, fontFamily: t.brand.font.mono, bgcolor: isLive ? `${t.palette.success.main}22` : 'action.hover', color: isLive ? 'success.main' : 'text.disabled' })} />
        <Chip size="small" label={`ProfitGuard: ${pgVerdict}`} sx={(t) => ({ height: 20, fontSize: text.s62, fontFamily: t.brand.font.mono, bgcolor: `${pgColor(t, pgVerdict)}22`, color: pgColor(t, pgVerdict) })} />
        <Box sx={{ flex: 1 }} />
        <Typography sx={{ color: 'text.secondary', fontSize: text.s85 }}>{open} of {total} gates open · {Math.round(completion * 100)}% to shippable</Typography>
        {open > 0 && <Button size="small" variant="contained" onClick={onRunAll}>Run all open</Button>}
      </Stack>
      <LinearProgress variant="determinate" value={Math.round(completion * 100)} sx={{ height: 5, borderRadius: 99, bgcolor: 'action.hover', mb: 1.5, '& .MuiLinearProgress-bar': { borderRadius: 99, backgroundImage: (t) => t.brand.gradient.signature } }} />
      <Stack direction="row" spacing={3.5} sx={{ overflowX: 'auto' }}>
        <HeaderStat label="Spend" value={`${formatUSD(spentUSD)} / ${formatUSD(budgetUSD)}`} />
        <HeaderStat label="Margin" value={`${marginPct}%`} />
        <HeaderStat label="Burn" value={burnUSDPerHr != null ? `${formatUSD(burnUSDPerHr)}/hr` : '—'} />
        <HeaderStat label="ETA to done" value={etaCompletionAt ? formatRelativeTime(etaCompletionAt) : '—'} />
      </Stack>
    </Box>
  );
}

function GapsRail({ projectId, gaps, seed, onPick, onDispatch }: {
  projectId: string; gaps: Gate[]; seed: StudioProject['activity']; onPick: (id: string) => void; onDispatch: (scope: string) => void;
}) {
  const [open, setOpen] = useState(false);
  if (!open) {
    return (
      <Box sx={{ width: 44, borderLeft: 1, borderColor: 'divider', bgcolor: 'background.paper', display: 'flex', flexDirection: 'column', alignItems: 'center', pt: 1.5 }}>
        <Tooltip title="Show what's not closed" arrow placement="left">
          <IconButton size="small" onClick={() => setOpen(true)} sx={{ color: 'text.secondary' }}>‹</IconButton>
        </Tooltip>
        {gaps.length > 0 && (
          <Box sx={{ mt: 1, width: 22, height: 22, borderRadius: 99, display: 'grid', placeItems: 'center', bgcolor: (t) => `${t.palette.error.main}22`, color: 'error.main', fontSize: text.s70, fontWeight: 700 }}>{gaps.length}</Box>
        )}
      </Box>
    );
  }
  return (
    <Box sx={{ width: { xs: 260, lg: 320 }, flexShrink: 0, borderLeft: 1, borderColor: 'divider', bgcolor: 'background.paper', display: 'flex', flexDirection: 'column', minHeight: 0 }}>
      <Stack direction="row" alignItems="center" spacing={1} sx={{ px: 2, py: 1.5, borderBottom: 1, borderColor: 'divider' }}>
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s70, letterSpacing: '0.08em', textTransform: 'uppercase', color: 'text.disabled' })}>Not closed end-to-end</Typography>
        <Chip size="small" label={gaps.length} sx={(t) => ({ height: 18, fontSize: text.s62, fontWeight: 700, bgcolor: gaps.length ? `${t.palette.error.main}22` : `${t.palette.success.main}22`, color: gaps.length ? 'error.main' : 'success.main' })} />
        <Box sx={{ flex: 1 }} />
        <IconButton size="small" onClick={() => setOpen(false)} sx={{ color: 'text.secondary' }}>›</IconButton>
      </Stack>

      <Box sx={{ overflowY: 'auto', p: 2, flexShrink: 0, maxHeight: '52%' }}>
        {gaps.length === 0 ? (
          <Typography sx={{ fontSize: text.s85, color: 'success.main' }}>● Every gate is closed — nothing blocks shipping.</Typography>
        ) : (
          <Stack spacing={1}>
            {gaps.map((g) => (
              <Box key={g.id} sx={{ p: 1.25, borderRadius: 2, border: 1, borderColor: 'divider', transition: (t) => `border-color ${t.brand.motion.fast}`, '&:hover': { borderColor: 'text.disabled' } }}>
                <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 0.5, cursor: 'pointer' }} onClick={() => onPick(g.id)}>
                  <Box sx={{ width: 7, height: 7, borderRadius: 99, flexShrink: 0, bgcolor: (t) => statusColor(t, g.status) }} />
                  <Typography sx={{ fontSize: text.s86, fontWeight: 600, flex: 1 }} noWrap>{g.name}</Typography>
                  <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s62, color: statusColor(t, g.status) })}>{statusLabel[g.status]}</Typography>
                </Stack>
                <Typography sx={{ fontSize: text.s78, color: 'text.secondary', lineHeight: 1.35, mb: 0.75 }}>{g.blocking}</Typography>
                <Stack direction="row" spacing={0.75}>
                  <Chip size="small" clickable label="Inspect" onClick={() => onPick(g.id)} sx={{ height: 20, fontSize: text.s62, bgcolor: 'action.hover' }} />
                  <Chip size="small" clickable label="Fix" onClick={() => onDispatch(`the ${g.name} gate`)} sx={(t) => ({ height: 20, fontSize: text.s62, bgcolor: `${t.palette.primary.main}22`, color: 'primary.main' })} />
                </Stack>
              </Box>
            ))}
          </Stack>
        )}
      </Box>

      <Box sx={{ borderTop: 1, borderColor: 'divider', p: 2, flex: 1, overflowY: 'auto', minHeight: 0 }}>
        <ActivityFeed projectId={projectId} seed={seed} />
      </Box>
    </Box>
  );
}
