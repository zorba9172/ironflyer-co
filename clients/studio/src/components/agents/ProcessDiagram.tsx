import { Box, Stack, Typography } from '@mui/material';
import type { CrewProcess } from '../../studioData';

// A compact, dependency-free diagram of how a crew's members collaborate.
// Parallel = fan-out workers, Sequential = a chain, Hierarchical = a manager
// over its members. Mirrors the real process so the operator sees the topology.
export function ProcessDiagram({ process, members, manager, dense }: {
  process: CrewProcess;
  members: string[];
  manager?: string;
  dense?: boolean;
}) {
  if (members.length === 0) {
    return <Typography sx={{ fontSize: '0.8rem', color: 'text.disabled' }}>Add members to see the topology.</Typography>;
  }
  if (process === 'sequential') return <Sequential members={members} dense={dense} />;
  if (process === 'hierarchical') return <Hierarchical members={members} manager={manager} dense={dense} />;
  return <Parallel members={members} dense={dense} />;
}

const pillSx = (dense?: boolean) => (t: import('@mui/material/styles').Theme) => ({
  px: dense ? 1 : 1.25,
  py: dense ? 0.4 : 0.6,
  borderRadius: 99,
  border: 1,
  borderColor: 'divider',
  bgcolor: 'background.default',
  fontSize: dense ? '0.68rem' : '0.76rem',
  fontWeight: 500,
  whiteSpace: 'nowrap',
  color: t.palette.text.primary,
});

function HubNode({ label }: { label: string }) {
  return (
    <Box sx={(t) => ({ px: 1.5, py: 0.7, borderRadius: 2, color: t.palette.primary.contrastText, backgroundImage: t.brand.gradient.signature, fontSize: '0.74rem', fontWeight: 700, whiteSpace: 'nowrap' })}>{label}</Box>
  );
}

function Arrow({ vertical }: { vertical?: boolean }) {
  return (
    <Box sx={{ display: 'flex', color: 'text.disabled' }}>
      {vertical ? (
        <svg width="14" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><path d="M12 4v14M6 13l6 6 6-6" /></svg>
      ) : (
        <svg width="16" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><path d="M4 12h14M13 6l6 6-6 6" /></svg>
      )}
    </Box>
  );
}

function Parallel({ members, dense }: { members: string[]; dense?: boolean }) {
  return (
    <Stack direction="row" alignItems="center" spacing={1.5}>
      <HubNode label="Crew" />
      <Box sx={{ display: 'flex', color: 'text.disabled' }}>
        <svg width="18" height="44" viewBox="0 0 24 60" fill="none" stroke="currentColor" strokeWidth="1.6"><path d="M2 30h8M10 30V8h12M10 30v22h12M10 30h12" strokeLinecap="round" /></svg>
      </Box>
      <Stack spacing={0.6}>
        {members.map((m) => (
          <Box key={m} sx={pillSx(dense)}>{m}</Box>
        ))}
      </Stack>
    </Stack>
  );
}

function Sequential({ members, dense }: { members: string[]; dense?: boolean }) {
  return (
    <Stack direction="row" alignItems="center" sx={{ flexWrap: 'wrap', gap: 0.75 }}>
      {members.map((m, i) => (
        <Stack key={m} direction="row" alignItems="center" spacing={0.75}>
          <Box sx={pillSx(dense)}>{m}</Box>
          {i < members.length - 1 && <Arrow />}
        </Stack>
      ))}
    </Stack>
  );
}

function Hierarchical({ members, manager, dense }: { members: string[]; manager?: string; dense?: boolean }) {
  const managerName = manager && members.includes(manager) ? manager : undefined;
  const reports = members.filter((m) => m !== managerName);
  return (
    <Stack alignItems="center" spacing={0.5}>
      <HubNode label={managerName ? `${managerName} · manager` : 'Manager (pick one)'} />
      <Arrow vertical />
      <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 0.6, justifyContent: 'center' }}>
        {reports.map((m) => (
          <Box key={m} sx={pillSx(dense)}>{m}</Box>
        ))}
      </Stack>
    </Stack>
  );
}
