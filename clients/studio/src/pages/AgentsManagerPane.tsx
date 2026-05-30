import { useMemo, useState, type ReactNode } from 'react';
import { Box, Button, Card, Stack, Typography } from '@mui/material';
import { confirmAction, toast } from '@ironflyer/ui-web/fx';
import { AGENTS, newAgent, type Agent, type StudioProject } from '../studioData';
import { useAgentTeam } from '../hooks/useAgentTeam';
import { useDispatchAgent } from '../hooks/useDispatchAgent';
import { AgentCard } from '../components/agents/AgentCard';
import { AgentBuilder } from '../components/agents/AgentBuilder';
import { text } from '@ironflyer/design-tokens/brand';

// In-editor agent manager. Same deep builder as the catalog, scoped to the
// project being built so gates and hand-off targets are the live ones. Custom
// agents are server-persisted (owner-scoped) when signed in via useAgentTeam,
// so the in-editor manager and the standalone catalog share one source of truth.
export function AgentsManagerPane({ project }: { project: StudioProject }) {
  const { customAgents, saveAgent, deleteAgent } = useAgentTeam();
  const { dispatch } = useDispatchAgent();
  const [editing, setEditing] = useState<Agent | null>(null);

  const allAgents = useMemo(() => [...customAgents, ...AGENTS], [customAgents]);
  const editingExists = editing ? customAgents.some((a) => a.id === editing.id) : false;

  const remove = async (a: Agent) => {
    const ok = await confirmAction({ title: `Delete ${a.name}?`, text: 'This removes the agent and its schedule. The build keeps running.', confirmText: 'Delete', danger: true });
    if (ok) { await deleteAgent(a.id); toast(`${a.name} deleted.`, 'success'); }
  };

  const save = (a: Agent) => { void saveAgent(a); };

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <Stack direction="row" alignItems="flex-start" justifyContent="space-between" sx={{ mb: 2.5, gap: 2 }}>
          <Box>
            <Typography variant="h4" sx={{ fontSize: text.s160, mb: 0.5 }}>Agents</Typography>
            <Typography sx={{ color: 'text.secondary' }}>
              Define what each agent does, the skills and tools it may use, and when it runs. The orchestrator routes work to them and reports cost as they go.
            </Typography>
          </Box>
          <Button variant="contained" sx={{ flexShrink: 0 }} onClick={() => setEditing(newAgent())}>New agent</Button>
        </Stack>

        <Label>Your agents ({customAgents.length})</Label>
        {customAgents.length === 0 ? (
          <Card sx={{ p: 4, textAlign: 'center', border: '1.5px dashed', borderColor: 'divider', mb: 4 }}>
            <Typography sx={{ color: 'text.secondary', mb: 1.5 }}>No custom agents yet. Create one — e.g. an "Einstein" research agent that runs daily and grounds the build.</Typography>
            <Button variant="outlined" color="inherit" onClick={() => setEditing(newAgent())}>Create your first agent</Button>
          </Card>
        ) : (
          <Grid sx={{ mb: 4 }}>
            {customAgents.map((a) => (
              <AgentCard key={a.id} agent={a} gates={project.gates} onEdit={setEditing} onRun={(x) => void dispatch(`${x.name}'s work`)} onDelete={(x) => void remove(x)} />
            ))}
          </Grid>
        )}

        <Label>Finisher roster</Label>
        <Grid>
          {AGENTS.map((a) => <AgentCard key={a.id} agent={a} gates={project.gates} builtIn />)}
        </Grid>
      </Box>

      {editing && (
        <AgentBuilder
          agent={editing}
          gates={project.gates}
          allAgents={allAgents}
          exists={editingExists}
          onClose={() => setEditing(null)}
          onSave={save}
        />
      )}
    </Box>
  );
}

function Label({ children }: { children: ReactNode }) {
  return (
    <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s70, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.25 })}>{children}</Typography>
  );
}

function Grid({ children, sx }: { children: ReactNode; sx?: object }) {
  return (
    <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', lg: 'repeat(3, 1fr)' }, gap: 2, ...sx }}>
      {children}
    </Box>
  );
}
