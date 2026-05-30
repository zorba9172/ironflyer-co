import { lazy, Suspense, useMemo, useState, type ReactNode } from 'react';
import { Box, Button, Card, Chip, CircularProgress, Stack, TextField, ToggleButton, ToggleButtonGroup, Typography } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { Icon } from '../icons';
import { confirmAction, toast } from '@ironflyer/ui-web/fx';
import { AGENTS, agentStatus, newAgent, newCrew, type Agent, type AgentStatus, type Crew } from '../studioData';
import { useStudio } from '../store';
import { useAgentTeam } from '../hooks/useAgentTeam';
import { useDispatchAgent } from '../hooks/useDispatchAgent';
import { AgentCard } from '../components/agents/AgentCard';
import { AgentBuilder } from '../components/agents/AgentBuilder';
import { CrewCard } from '../components/agents/CrewCard';
import { CrewBuilder } from '../components/agents/CrewBuilder';
import { agentColor } from '../components/statusColor';
import { StudioChart, donutOption } from '../components/charts';
import { text } from '@ironflyer/design-tokens/brand';

// The orchestration map carries React Flow — load it only when the Map view is
// opened so the heavy canvas stays off the catalog's cold path.
const AgentTeamMap = lazy(() => import('../components/agents/AgentTeamMap').then((m) => ({ default: m.AgentTeamMap })));

type StatusFilter = 'all' | AgentStatus;
const STATUS_FILTERS: { value: StatusFilter; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'working', label: 'Working' },
  { value: 'done', label: 'Done' },
  { value: 'blocked', label: 'Blocked' },
  { value: 'idle', label: 'Idle' },
];

// The agent catalog + builder. The orchestrator ships a finisher roster; the
// operator composes their own agents on top — each with skills, tools, areas of
// responsibility, guardrails, a model, and a schedule. This is the management
// surface; the deep builder opens full-screen.
export function AgentsPage() {
  const theme = useTheme();

  // Custom agents + crews are server-persisted (owner-scoped) when signed in,
  // falling back to the local store offline. The built-in roster stays mock.
  const { customAgents, crews, saveAgent, deleteAgent, saveCrew: saveCrewLive, deleteCrew, runCrew } = useAgentTeam();
  const { dispatch } = useDispatchAgent();
  const gates = useStudio((s) => s.current.gates);

  const [editing, setEditing] = useState<Agent | null>(null);
  const [editingCrew, setEditingCrew] = useState<Crew | null>(null);
  const [query, setQuery] = useState('');
  const [view, setView] = useState<'map' | 'catalog'>('map');
  const [status, setStatus] = useState<StatusFilter>('all');

  const allAgents = useMemo(() => [...customAgents, ...AGENTS], [customAgents]);
  const editingExists = editing ? customAgents.some((a) => a.id === editing.id) : false;
  const editingCrewExists = editingCrew ? crews.some((c) => c.id === editingCrew.id) : false;

  const matches = (a: Agent) => {
    const q = query.trim().toLowerCase();
    const passQ = !q || [a.name, a.role, a.description, ...(a.skills ?? []), ...(a.responsibilities ?? [])]
      .filter(Boolean).join(' ').toLowerCase().includes(q);
    const passS = status === 'all' || agentStatus(a, gates) === status;
    return passQ && passS;
  };

  const customMatches = customAgents.filter(matches);
  const rosterMatches = AGENTS.filter(matches);

  // Status mix — the viz-first mirror of the roster. Donut center names the
  // operator's live signal: how many agents are working right now.
  const statusMix = useMemo(() => {
    const counts: Record<AgentStatus, number> = { working: 0, done: 0, blocked: 0, idle: 0 };
    allAgents.forEach((a) => { counts[agentStatus(a, gates)] += 1; });
    return counts;
  }, [allAgents, gates]);
  const working = statusMix.working;

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

  const donut = donutOption(theme, {
    data: [
      { name: 'Working', value: statusMix.working, color: agentColor(theme, 'working') },
      { name: 'Done', value: statusMix.done, color: agentColor(theme, 'done') },
      { name: 'Blocked', value: statusMix.blocked, color: agentColor(theme, 'blocked') },
      { name: 'Idle', value: statusMix.idle, color: agentColor(theme, 'idle') },
    ],
    centerLabel: `${working}\nworking`,
    centerColor: agentColor(theme, 'working'),
    emptyLabel: 'No agents',
  });

  return (
    <Box sx={{ position: 'relative' }}>
      {/* Hero header — ambient neon wash over the title block (mx.md › Ambient
          Effects): atmosphere only, no visible circles. */}
      <Box
        aria-hidden
        sx={(t) => ({
          position: 'absolute',
          inset: 0,
          height: 360,
          zIndex: 0,
          pointerEvents: 'none',
          background: t.studio.gradient.soft,
          opacity: 0.5,
          maskImage: `linear-gradient(to bottom, ${t.palette.common.black}, transparent)`,
          WebkitMaskImage: `linear-gradient(to bottom, ${t.palette.common.black}, transparent)`,
        })}
      />

      <Box sx={{ position: 'relative', zIndex: 1, p: { xs: 3, md: 5 }, maxWidth: 1180, mx: 'auto' }}>
        <Stack direction="row" alignItems="flex-start" justifyContent="space-between" sx={{ gap: 2, mb: 3 }}>
          <Box sx={{ minWidth: 0 }}>
            <Chip
              icon={<Icon name="bot" size={15} />}
              label="Agent team"
              sx={(t) => ({
                height: 30,
                mb: 2,
                borderRadius: t.studio.radius.pill,
                border: `1px solid ${t.palette.divider}`,
                backgroundColor: t.palette.cardBg,
                backdropFilter: `blur(${t.studio.effect.card.blur}px)`,
                color: t.palette.text.secondary,
                fontWeight: t.typography.fontWeightMedium,
                '& .MuiChip-icon': { color: t.studio.neon.blue, ml: 0.5 },
                '& .MuiChip-label': { px: 1 },
              })}
            />
            <Typography variant="h2" sx={{ mb: 1 }}>
              Your AI{' '}
              <Box component="span" sx={(t) => ({ backgroundImage: t.studio.gradient.signature, WebkitBackgroundClip: 'text', backgroundClip: 'text', WebkitTextFillColor: 'transparent', color: 'transparent' })}>agent team</Box>
            </Typography>
            <Typography variant="body1" sx={{ color: 'text.secondary', maxWidth: 640 }}>
              One orchestrator routes work to specialists. Compose your own agents — give each a mission, skills, tools, areas of responsibility, and guardrails — then schedule when they run.
            </Typography>
          </Box>
          <Button variant="contained" color="primary" startIcon={<Icon name="add" size={18} />} sx={{ flexShrink: 0 }} onClick={() => setEditing(newAgent())}>New agent</Button>
        </Stack>

        {/* Viz-first status panel: donut mirror of the live roster + stat readout. */}
        <Card
          sx={(t) => ({
            p: { xs: 2.5, md: 3 },
            mb: 4,
            display: 'flex',
            flexDirection: { xs: 'column', sm: 'row' },
            alignItems: 'center',
            gap: { xs: 2, md: 4 },
            borderRadius: `${t.studio.effect.card.radius}px`,
            backgroundColor: t.palette.cardBg,
            borderColor: t.palette.cardBorder,
            backdropFilter: `blur(${t.studio.effect.card.blur}px)`,
            WebkitBackdropFilter: `blur(${t.studio.effect.card.blur}px)`,
          })}
        >
          <Box sx={{ width: 168, flexShrink: 0 }}>
            <StudioChart option={donut} height={168} />
          </Box>
          <Box sx={(t) => ({ display: { xs: 'none', sm: 'block' }, alignSelf: 'stretch', width: '1px', backgroundColor: t.palette.divider })} />
          <Stack direction="row" sx={{ flexWrap: 'wrap', gap: { xs: 3, md: 5 }, flex: 1, justifyContent: { xs: 'center', sm: 'flex-start' } }}>
            <Stat n={allAgents.length} label="agents" accent={theme.studio.neon.violet} />
            <Stat n={customAgents.length} label="yours" accent={theme.studio.neon.blue} />
            <Stat n={working} label="working now" accent={theme.studio.neon.success} />
            <Stat n={crews.length} label="crews" accent={theme.studio.neon.pink} />
            <Stat n={AGENTS.length} label="finisher roster" accent={theme.studio.neon.purple} />
          </Stack>
        </Card>

        {/* Controls — segmented view switch + (catalog) search & status filter. */}
        <Stack direction="row" alignItems="center" sx={{ mb: 3, gap: 2, flexWrap: 'wrap' }}>
          <ToggleButtonGroup size="small" exclusive value={view} onChange={(_e, v) => v && setView(v)} sx={{ flexShrink: 0 }}>
            <ToggleButton value="map" sx={{ textTransform: 'none', px: 2, gap: 0.75 }}><Icon name="network" size={15} /> Map</ToggleButton>
            <ToggleButton value="catalog" sx={{ textTransform: 'none', px: 2, gap: 0.75 }}><Icon name="projects" size={15} /> Catalog</ToggleButton>
          </ToggleButtonGroup>
          {view === 'catalog' && (
            <>
              <TextField
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                size="small"
                placeholder="Search by name, skill, or responsibility"
                InputProps={{ startAdornment: <Box sx={{ display: 'flex', color: 'text.disabled', mr: 1 }}><Icon name="search" size={16} /></Box> }}
                sx={{ flex: 1, minWidth: 240, maxWidth: 460 }}
              />
              <ToggleButtonGroup size="small" exclusive value={status} onChange={(_e, v) => v && setStatus(v)} sx={{ flexWrap: 'wrap' }}>
                {STATUS_FILTERS.map((f) => (
                  <ToggleButton key={f.value} value={f.value} sx={{ textTransform: 'none', px: 1.5 }}>{f.label}</ToggleButton>
                ))}
              </ToggleButtonGroup>
            </>
          )}
        </Stack>

        {view === 'map' ? (
          <Box sx={{ mb: 2 }}>
            <Typography sx={{ color: 'text.secondary', fontSize: text.s85, mb: 1.5 }}>
              The orchestrator routes to every specialist; each tethers to the gate it owns, and delegation shows as hand-off arcs. Click any agent to open it.
            </Typography>
            <Suspense fallback={<Box sx={(t) => ({ height: { xs: 420, md: 480 }, borderRadius: `${t.studio.effect.card.radius}px`, border: `1px solid ${t.palette.cardBorder}`, backgroundColor: t.palette.cardBg, display: 'grid', placeItems: 'center' })}><CircularProgress size={26} thickness={5} /></Box>}>
              <AgentTeamMap agents={allAgents} gates={gates} onEdit={setEditing} />
            </Suspense>
          </Box>
        ) : (
          catalogBody()
        )}
      </Box>

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
    const filtering = query.trim() !== '' || status !== 'all';
    return (
      <>
      {/* Crews */}
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1.5 }}>
        <SectionLabel sx={{ mb: 0 }} accent={theme.studio.neon.pink}>Crews ({crews.length})</SectionLabel>
        <Button size="small" color="inherit" startIcon={<Icon name="users" size={15} />} onClick={() => setEditingCrew(newCrew())} sx={{ borderColor: 'divider' }} variant="outlined">New crew</Button>
      </Stack>
      {crews.length === 0 ? (
        <EmptyState
          icon={<Icon name="users" size={22} />}
          accent={theme.studio.neon.pink}
          text="No crews yet. Group agents to run together — in parallel as workers, in a chain, or under a manager."
          action={<Button variant="outlined" color="inherit" onClick={() => setEditingCrew(newCrew())}>Create your first crew</Button>}
        />
      ) : (
        <Grid>
          {crews.map((c) => (
            <CrewCard key={c.id} crew={c} agents={allAgents} onEdit={setEditingCrew} onRun={(x) => void runCrew(x)} onDelete={(x) => void removeCrewConfirm(x)} />
          ))}
        </Grid>
      )}

      {/* Your agents */}
      <SectionLabel accent={theme.studio.neon.blue}>Your agents ({customAgents.length})</SectionLabel>
      {customAgents.length === 0 ? (
        <EmptyState
          icon={<Icon name="bot" size={22} />}
          accent={theme.studio.neon.blue}
          text={'No custom agents yet. Create one — e.g. an "Einstein" research agent that runs daily and grounds the build.'}
          action={<Button variant="outlined" color="inherit" onClick={() => setEditing(newAgent())}>Create your first agent</Button>}
        />
      ) : customMatches.length === 0 ? (
        <Typography sx={{ color: 'text.disabled', mb: 5 }}>{filtering ? 'No agents match your filters.' : 'No agents to show.'}</Typography>
      ) : (
        <Grid>
          {customMatches.map((a) => (
            <AgentCard key={a.id} agent={a} gates={gates} onEdit={setEditing} onRun={(x) => void dispatch(`${x.name}'s work`)} onDelete={(x) => void remove(x)} />
          ))}
        </Grid>
      )}

      {/* Finisher roster */}
      <SectionLabel sx={{ mt: 4 }} accent={theme.studio.neon.violet}>Finisher roster</SectionLabel>
      {rosterMatches.length === 0 ? (
        <Typography sx={{ color: 'text.disabled' }}>{filtering ? 'No roster agents match your filters.' : 'No roster agents to show.'}</Typography>
      ) : (
        <Grid>
          {rosterMatches.map((a) => <AgentCard key={a.id} agent={a} gates={gates} builtIn />)}
        </Grid>
      )}
      </>
    );
  }
}

function Stat({ n, label, accent }: { n: number; label: string; accent: string }) {
  return (
    <Box>
      <Stack direction="row" alignItems="center" spacing={1}>
        <Box sx={{ width: 8, height: 8, borderRadius: '50%', backgroundColor: accent, boxShadow: `0 0 10px ${accent}99` }} />
        <Typography sx={{ fontSize: text.s160, fontWeight: 800, lineHeight: 1 }}>{n}</Typography>
      </Stack>
      <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s68, letterSpacing: '0.08em', textTransform: 'uppercase', color: 'text.disabled', mt: 0.75 })}>{label}</Typography>
    </Box>
  );
}

function SectionLabel({ children, sx, accent }: { children: ReactNode; sx?: object; accent?: string }) {
  return (
    <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1.5, ...sx }}>
      {accent && <Box sx={{ width: 18, height: 2, borderRadius: 1, backgroundColor: accent, boxShadow: `0 0 8px ${accent}80` }} />}
      <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s70, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled' })}>
        {children}
      </Typography>
    </Stack>
  );
}

function EmptyState({ icon, accent, text: body, action }: { icon: ReactNode; accent: string; text: string; action: ReactNode }) {
  return (
    <Card
      sx={(t) => ({
        p: 3,
        mb: 3,
        textAlign: 'center',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        gap: 1.25,
        border: `1.5px dashed ${t.palette.divider}`,
        borderRadius: `${t.studio.effect.card.radius}px`,
        backgroundColor: t.palette.cardBg,
      })}
    >
      <Box sx={{ width: 40, height: 40, borderRadius: 2, display: 'grid', placeItems: 'center', color: accent, backgroundColor: `${accent}1f`, border: `1px solid ${accent}33` }}>
        {icon}
      </Box>
      <Typography sx={{ color: 'text.secondary', maxWidth: 420 }}>{body}</Typography>
      {action}
    </Card>
  );
}

function Grid({ children }: { children: ReactNode }) {
  return (
    <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', lg: 'repeat(3, 1fr)' }, gap: 2, mb: 1 }}>
      {children}
    </Box>
  );
}
