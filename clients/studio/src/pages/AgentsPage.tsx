import { Box, Card, Chip, Stack, Typography } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { AGENTS, agentStatus, mockProject } from '../studioData';
import { agentColor } from '../components/statusColor';

const statusText: Record<string, string> = { working: 'Working', done: 'Done', blocked: 'Blocked', idle: 'Idle' };

// The orchestrator's specialist agents. Status is derived from the gate each
// one owns — the roster mirrors the real finisher run.
export function AgentsPage() {
  const theme = useTheme();
  const gates = mockProject.gates;

  return (
    <Box sx={{ p: { xs: 3, md: 5 }, maxWidth: 1100, mx: 'auto' }}>
      <Typography variant="h3" sx={{ fontSize: '2.5rem', mb: 1 }}>Agents</Typography>
      <Typography sx={{ color: 'text.secondary', mb: 4 }}>One orchestrator routes work to specialists. Each closes a finisher gate, reviews its own patches, and reports cost as it goes.</Typography>

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', lg: 'repeat(3, 1fr)' }, gap: 2 }}>
        {AGENTS.map((a) => {
          const status = agentStatus(a, gates);
          const color = agentColor(theme, status);
          return (
            <Card key={a.id} sx={{ p: 2.5 }}>
              <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1.25 }}>
                <Stack direction="row" alignItems="center" spacing={1.5}>
                  <Box sx={(t) => ({ width: 36, height: 36, borderRadius: 2, display: 'grid', placeItems: 'center', color: '#fff', backgroundImage: t.brand.gradient.signature, fontWeight: 700 })}>{a.name[0]}</Box>
                  <Typography variant="h6" sx={{ fontSize: '1.02rem' }}>{a.name}</Typography>
                </Stack>
                <Chip size="small" label={statusText[status]} sx={{ height: 20, fontSize: '0.66rem', bgcolor: `${color}22`, color }} />
              </Stack>
              <Typography sx={{ color: 'text.secondary', fontSize: '0.88rem' }}>{a.role}</Typography>
              {a.gateId && (
                <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.7rem', color: 'text.disabled', mt: 1.5 })}>owns the {a.gateId} gate</Typography>
              )}
            </Card>
          );
        })}
      </Box>
    </Box>
  );
}
