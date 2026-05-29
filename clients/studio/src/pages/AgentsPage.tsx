import { lazy, Suspense, useMemo, useState, type ReactNode } from 'react';
import { Box, Button, Card, CircularProgress, Stack, TextField, ToggleButton, ToggleButtonGroup, Typography } from '@mui/material';
import { confirmAction, toast } from '@ironflyer/ui-web/fx';
import { AGENTS, agentStatus, newAgent, newCrew, type Agent, type Crew } from '../studioData';
import { useStudio } from '../store';
import { useAgentTeam } from '../hooks/useAgentTeam';
import { AgentCard } from '../components/agents/AgentCard';
import { AgentBuilder } from '../components/agents/AgentBuilder';
import { CrewCard } from '../components/agents/CrewCard';
import { CrewBuilder } from '../components/agents/CrewBuilder';

// The orchestration map carries React Flow — load it only when the Map view is
// opened so the heavy canvas stays off the catalog's cold path.
const AgentTeamMap = lazy(() => import('../components/agents/AgentTeamMap').then((m) => ({ default: m.AgentTeamMap })));

// The agent catalog + builder. The orchestrator ships a finisher roster; the
// operator composes their own agents on top — each with skills, tools, areas of
// responsibility, guardrails, a model, and a schedule. This is the management
// surface; the deep builder opens full-screen.
export function AgentsPage() {
  // Custom agents + crews are server-persisted (owner-scoped) when signed in,
  // falling back to the local store offline. The built-in roster stays mock.
  const { customAgents, crews, saveAgent, deleteAgent, saveCrew: saveCrewLive, deleteCrew, runCrew } = useAgentTeam();
  const gates = useStudio((s) => s.current.gates);

  const [editing, setEditing] = useState<Agent | null>(null);
  const [editingCrew, setEditingCrew] = useState<Crew | null>(null);
  const [query, setQuery] = useState('');
  const [view, setView] = useState<'map' | 'catalog'>('map');

  const allAgents = useMemo(() => [...customAgents, ...AGENTS], [customAgents]);
  const editingExists = editing ? customAgents.some((a) => a.id === editing.id) : false;
  const editingCrewExists = editingCrew ? crews.some((c) => c.id === editingCrew.id) : false;

  const matches = (a: Agent) => {
    const q = query.trim().toLowerCase();
    if (!q) return true;
    return [a.name, a.role, a.description, ...(a.skills ?? []), ...(a.responsibilities ?? [])]
      .filter(Boolean).join(' ').toLowerCase().includes(q);
  };

  const customMatches = customAgents.filter(matches);
  const rosterMatches = AGENTS.filter(matches);

  const working = allAgents.filter((a) => agentStatus(a, gates) === 'working').length;

  const remove = async (a: Agent) => {
    const ok = await confirmAction({ title: `Delete ${a.name}?`, text: 'This removes the agent and its schedule. Any running build keeps going.', confirmText: 'Delete', danger: true });
    if (ok) { await deleteAgent(a.id); toast(`${a.name} deleted.`, 'success'); }
  };

  const save = (a: Agent) => { void saveAgent(a); };

  const removeCrewConfirm = async (c: Crew) => {
    const ok = await confirmAction({ title: `Delete ${c.name}?`, text: 'This removes the crew. The agents in it stay.', confirmText: 'Delete', danger: true });
    if (ok) { await deleteCrew(c.id); toast(`${c.name} deleted.`, 'success'); }
  };
  const saveCrew = (c: Crew) => { void saveCrewLive(c); };

  return (
    <Box sx={{ p: { xs: 3, md: 5 }, maxWidth: 1180, mx: 'auto' }}>
      <Stack direction="row" alignItems="flex-start" justifyContent="space-between" sx={{ gap: 2, mb: 1 }}>
        <Box>
          <Typography variant="h3" sx={{ fontSize: '2.5rem', mb: 1 }}>Agents</Typography>
          <Typography sx={{ color: 'text.secondary', maxWidth: 640 }}>
            One orchestrator routes work to specialists. Compose your own agents — give each a mission, skills, tools, areas of responsibility, and guardrails — then schedule when they run.
          </Typography>
        </Box>
        <Button variant="contained" sx={{ flexShrink: 0 }} onClick={() => setEditing(newAgent())}>New agent</Button>
      </Stack>

      {/* Stat strip */}
      <Stack direction="row" spacing={3} sx={{ my: 3 }}>
        <Stat n={allAgents.length} label="agents" />
        <Stat n={customAgents.length} label="yours" />
        <Stat n={working} label="working now" />
        <Stat n={AGENTS.length} label="finisher roster" />
      </Stack>

      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 3, gap: 2, flexWrap: 'wrap' }}>
        <ToggleButtonGroup size="small" exclusive value={view} onChange={(_e, v) => v && setView(v)}>
          <ToggleButton value="map" sx={{ textTransform: 'none', px: 2 }}>Map</ToggleButton>
          <ToggleButton value="catalog" sx={{ textTransform: 'none', px: 2 }}>Catalog</ToggleButton>
        </ToggleButtonGroup>
        {view === 'catalog' && (
          <TextField
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            size="small"
            placeholder="Search by name, skill, or responsibility"
            sx={{ flex: 1, maxWidth: 460 }}
          />
        )}
      </Stack>

      {view === 'map' ? (
        <Box sx={{ mb: 2 }}>
          <Typography sx={{ color: 'text.secondary', fontSize: '0.85rem', mb: 1.5 }}>
            The orchestrator routes to every specialist; each tethers to the gate it owns, and delegation shows as hand-off arcs. Click any agent to open it.
          </Typography>
          <Suspense fallback={<Box sx={{ height: { xs: 460, md: 600 }, borderRadius: 3, border: 1, borderColor: 'divider', display: 'grid', placeItems: 'center' }}><CircularProgress size={26} thickness={5} /></Box>}>
            <AgentTeamMap agents={allAgents} gates={gates} onEdit={setEditing} />
          </Suspense>
        </Box>
      ) : (
        catalogBody()
      )}

      {editing && (
        <AgentBuilder
          agent={editing}
          gates={gates}
          allAgents={allAgents}
          exists={editingExists}
          onClose={() => setEditing(null)}
          onSave={save}
        />
      )}
      {editingCrew && (
        <CrewBuilder
          crew={editingCrew}
          agents={allAgents}
          exists={editingCrewExists}
          onClose={() => setEditingCrew(null)}
          onSave={saveCrew}
        />
      )}
    </Box>
  );

  function catalogBody() {
    return (
      <>
      {/* Crews */}
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1.5 }}>
        <SectionLabel sx={{ mb: 0 }}>Crews ({crews.length})</SectionLabel>
        <Button size="small" color="inherit" onClick={() => setEditingCrew(newCrew())} sx={{ borderColor: 'divider' }} variant="outlined">New crew</Button>
      </Stack>
      {crews.length === 0 ? (
        <Card sx={{ p: 3, textAlign: 'center', border: '1.5px dashed', borderColor: 'divider', mb: 5 }}>
          <Typography sx={{ color: 'text.secondary', mb: 1.5 }}>No crews yet. Group agents to run together — in parallel as workers, in a chain, or under a manager.</Typography>
          <Button variant="outlined" color="inherit" onClick={() => setEditingCrew(newCrew())}>Create your first crew</Button>
        </Card>
      ) : (
        <Grid>
          {crews.map((c) => (
            <CrewCard key={c.id} crew={c} agents={allAgents} onEdit={setEditingCrew} onRun={(x) => void runCrew(x)} onDelete={(x) => void removeCrewConfirm(x)} />
          ))}
        </Grid>
      )}

      {/* Your agents */}
      <SectionLabel>Your agents ({customAgents.length})</SectionLabel>
      {customAgents.length === 0 ? (
        <Card sx={{ p: 4, textAlign: 'center', border: '1.5px dashed', borderColor: 'divider', mb: 5 }}>
          <Typography sx={{ color: 'text.secondary', mb: 1.5 }}>No custom agents yet. Create one — e.g. an "Einstein" research agent that runs daily and grounds the build.</Typography>
          <Button variant="outlined" color="inherit" onClick={() => setEditing(newAgent())}>Create your first agent</Button>
        </Card>
      ) : customMatches.length === 0 ? (
        <Typography sx={{ color: 'text.disabled', mb: 5 }}>No agents match "{query}".</Typography>
      ) : (
        <Grid>
          {customMatches.map((a) => (
            <AgentCard key={a.id} agent={a} gates={gates} onEdit={setEditing} onRun={(x) => toast(`${x.name} dispatched — watch the board.`, 'success')} onDelete={(x) => void remove(x)} />
          ))}
        </Grid>
      )}

      {/* Finisher roster */}
      <SectionLabel sx={{ mt: 4 }}>Finisher roster</SectionLabel>
      {rosterMatches.length === 0 ? (
        <Typography sx={{ color: 'text.disabled' }}>No roster agents match "{query}".</Typography>
      ) : (
        <Grid>
          {rosterMatches.map((a) => <AgentCard key={a.id} agent={a} gates={gates} builtIn />)}
        </Grid>
      )}
      </>
    );
  }
}

function Stat({ n, label }: { n: number; label: string }) {
  return (
    <Box>
      <Typography sx={{ fontSize: '1.6rem', fontWeight: 700, lineHeight: 1 }}>{n}</Typography>
      <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.68rem', letterSpacing: '0.08em', textTransform: 'uppercase', color: 'text.disabled', mt: 0.5 })}>{label}</Typography>
    </Box>
  );
}

function SectionLabel({ children, sx }: { children: ReactNode; sx?: object }) {
  return (
    <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5, ...sx })}>
      {children}
    </Typography>
  );
}

function Grid({ children }: { children: ReactNode }) {
  return (
    <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', lg: 'repeat(3, 1fr)' }, gap: 2, mb: 1 }}>
      {children}
    </Box>
  );
}
