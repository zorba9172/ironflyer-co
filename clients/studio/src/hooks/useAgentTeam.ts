import { useCallback } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useGraphQLQuery, useRequest, operations } from '@ironflyer/data';
import { toast } from '@ironflyer/ui-web/fx';
import { useStudio } from '../store';
import { useLiveProjectId } from './useLiveProjectId';
import type { Agent, AgentSchedule, Crew } from '../studioData';

// The agent layer, backed by the real orchestrator when signed in and falling
// back to the local (localStorage) store offline. Custom agents and crews are
// owner-scoped, server-persisted resources via the agentteam GraphQL surface;
// runCrew dispatches the crew's members in parallel on the backend.

interface ServerSchedule {
  mode: string; every?: string | null; at?: string | null;
  weekday?: number | null; trigger?: string | null; enabled: boolean;
}
interface ServerAgent {
  id: string; name: string; role: string; description?: string | null;
  instructions?: string | null; gateId?: string | null;
  skills: string[]; tools: string[]; responsibilities: string[];
  guardrails: string[]; knowledge: string[]; model?: string | null;
  autonomy: string; canDelegate: boolean; handoffTo: string[];
  schedule?: ServerSchedule | null;
}
interface ServerCrew {
  id: string; name: string; goal: string; process: string;
  memberIds: string[]; managerId?: string | null; schedule?: ServerSchedule | null;
}

function scheduleFromServer(s?: ServerSchedule | null): AgentSchedule | undefined {
  if (!s) return undefined;
  return {
    mode: s.mode as AgentSchedule['mode'],
    every: s.every ?? undefined, at: s.at ?? undefined,
    weekday: s.weekday ?? undefined,
    trigger: (s.trigger ?? undefined) as AgentSchedule['trigger'],
    enabled: s.enabled,
  };
}

function serverToAgent(s: ServerAgent): Agent {
  return {
    id: s.id, name: s.name, role: s.role,
    description: s.description ?? undefined, instructions: s.instructions ?? undefined,
    gateId: s.gateId ?? undefined, skills: s.skills ?? [], tools: s.tools ?? [],
    responsibilities: s.responsibilities ?? [], guardrails: s.guardrails ?? [],
    knowledge: s.knowledge ?? [], model: s.model ?? undefined,
    autonomy: s.autonomy as Agent['autonomy'], canDelegate: !!s.canDelegate,
    handoffTo: s.handoffTo ?? [], schedule: scheduleFromServer(s.schedule), custom: true,
  };
}

function serverToCrew(s: ServerCrew): Crew {
  return {
    id: s.id, name: s.name, goal: s.goal, process: s.process as Crew['process'],
    memberIds: s.memberIds ?? [], managerId: s.managerId ?? undefined,
    schedule: scheduleFromServer(s.schedule),
  };
}

function scheduleInput(s?: AgentSchedule) {
  if (!s) return undefined;
  return { mode: s.mode, every: s.every, at: s.at, weekday: s.weekday, trigger: s.trigger, enabled: s.enabled };
}

function agentInput(a: Agent) {
  return {
    id: a.id, name: a.name, role: a.role, description: a.description, instructions: a.instructions,
    gateId: a.gateId, skills: a.skills ?? [], tools: a.tools ?? [],
    responsibilities: a.responsibilities ?? [], guardrails: a.guardrails ?? [],
    knowledge: a.knowledge ?? [], model: a.model, autonomy: a.autonomy,
    canDelegate: a.canDelegate, handoffTo: a.handoffTo ?? [], schedule: scheduleInput(a.schedule),
  };
}

function crewInput(c: Crew) {
  return {
    id: c.id, name: c.name, goal: c.goal, process: c.process,
    memberIds: c.memberIds, managerId: c.managerId, schedule: scheduleInput(c.schedule),
  };
}

export interface AgentTeam {
  online: boolean;
  customAgents: Agent[];
  crews: Crew[];
  saveAgent: (a: Agent) => Promise<void>;
  deleteAgent: (id: string) => Promise<void>;
  saveCrew: (c: Crew) => Promise<void>;
  deleteCrew: (id: string) => Promise<void>;
  runCrew: (c: Crew) => Promise<void>;
}

export function useAgentTeam(): AgentTeam {
  const request = useRequest();
  const online = !!request;
  const qc = useQueryClient();
  const liveProjectId = useLiveProjectId();

  // Local store (offline fallback + optimistic source while signed out).
  const localAgents = useStudio((s) => s.customAgents);
  const localCrews = useStudio((s) => s.crews);
  const addAgent = useStudio((s) => s.addAgent);
  const updateAgent = useStudio((s) => s.updateAgent);
  const removeAgent = useStudio((s) => s.removeAgent);
  const addCrew = useStudio((s) => s.addCrew);
  const updateCrew = useStudio((s) => s.updateCrew);
  const removeCrew = useStudio((s) => s.removeCrew);

  const liveAgents = useGraphQLQuery<Agent[], { customAgents: ServerAgent[] }>({
    key: ['customAgents'], operationName: 'CustomAgents', query: operations.CUSTOM_AGENTS,
    fallbackData: [], map: (r) => (r.customAgents ?? []).map(serverToAgent), enabled: online,
  });
  const liveCrews = useGraphQLQuery<Crew[], { crews: ServerCrew[] }>({
    key: ['crews'], operationName: 'Crews', query: operations.CREWS,
    fallbackData: [], map: (r) => (r.crews ?? []).map(serverToCrew), enabled: online,
  });

  const invalidate = useCallback((k: string) => { void qc.invalidateQueries({ queryKey: [k] }); }, [qc]);

  const saveAgent = useCallback(async (a: Agent) => {
    if (online && request) {
      await request('SaveCustomAgent', operations.SAVE_CUSTOM_AGENT, { input: agentInput(a) });
      invalidate('customAgents');
    } else if (localAgents.some((x) => x.id === a.id)) { updateAgent(a); } else { addAgent(a); }
  }, [online, request, invalidate, localAgents, addAgent, updateAgent]);

  const deleteAgent = useCallback(async (id: string) => {
    if (online && request) {
      await request('DeleteCustomAgent', operations.DELETE_CUSTOM_AGENT, { id });
      invalidate('customAgents'); invalidate('crews');
    } else { removeAgent(id); }
  }, [online, request, invalidate, removeAgent]);

  const saveCrew = useCallback(async (c: Crew) => {
    if (online && request) {
      await request('SaveCrew', operations.SAVE_CREW, { input: crewInput(c) });
      invalidate('crews');
    } else if (localCrews.some((x) => x.id === c.id)) { updateCrew(c); } else { addCrew(c); }
  }, [online, request, invalidate, localCrews, addCrew, updateCrew]);

  const deleteCrew = useCallback(async (id: string) => {
    if (online && request) {
      await request('DeleteCrew', operations.DELETE_CREW, { id });
      invalidate('crews');
    } else { removeCrew(id); }
  }, [online, request, invalidate, removeCrew]);

  const runCrew = useCallback(async (c: Crew) => {
    if (!online || !request) { toast(`${c.name} dispatched — ${c.memberIds.length} agents (offline preview).`, 'info'); return; }
    if (!liveProjectId) { toast('Open a live project to run a crew against it.', 'info'); return; }
    try {
      const r = await request<{ runCrew: { totalCostUsd: number; members: { name: string }[] } }>(
        'RunCrew', operations.RUN_CREW, { id: c.id, projectId: liveProjectId },
      );
      const n = r.runCrew?.members?.length ?? c.memberIds.length;
      toast(`${c.name} ran ${n} agents · $${(r.runCrew?.totalCostUsd ?? 0).toFixed(2)} spent.`, 'success');
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Could not run crew.', 'error');
    }
  }, [online, request, liveProjectId]);

  return {
    online,
    customAgents: online ? liveAgents.data : localAgents,
    crews: online ? liveCrews.data : localCrews,
    saveAgent, deleteAgent, saveCrew, deleteCrew, runCrew,
  };
}
