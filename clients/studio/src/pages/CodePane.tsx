import { Box, Chip, Stack, Typography } from '@mui/material';
import { LogoMark } from '../components/LogoMark';
import { IdeFrame } from '../components/IdeFrame';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { useStudio } from '../store';

// Code surface — the branded Eclipse Theia web IDE (clients/ide/, image
// ironflyer/theia-ide:latest), served per-workspace by the runtime. Ironflyer
// is visualization-first: this full IDE is the opt-in "for pros" layer,
// reachable in one click but never the default pane. The viz surfaces
// (Dashboard / Map / Preview) remain the landing experience; this is where a
// professional opens the hood and edits the real workspace directly.
//
// This studio top bar is the canonical chrome for the IDE — it owns the
// Ironflyer wordmark, the project name, and the Pro·code marker so the
// embedded Theia frame can stay bare (its own wordmark is redundant here).

function IdeTopBar({ projectName }: { projectName: string }) {
  return (
    <Stack
      direction="row"
      alignItems="center"
      justifyContent="space-between"
      sx={{ px: 2, py: 1, borderBottom: 1, borderColor: 'divider', bgcolor: 'background.paper', flexShrink: 0 }}
    >
      <Stack direction="row" alignItems="center" spacing={1.25} sx={{ minWidth: 0 }}>
        <LogoMark size={18} />
        <Typography variant="subtitle2">Ironflyer IDE</Typography>
        <Typography variant="subtitle2" sx={{ color: 'text.disabled' }}>/</Typography>
        <Typography variant="subtitle2" sx={{ color: 'text.secondary', fontWeight: 400 }} noWrap>
          {projectName}
        </Typography>
      </Stack>
      <Chip
        label="Pro · code"
        size="small"
        variant="outlined"
        sx={(t) => ({
          fontFamily: t.brand.font.mono,
          letterSpacing: '0.06em',
          textTransform: 'uppercase',
          color: 'text.secondary',
          '& .MuiChip-label': { ...t.typography.caption },
        })}
      />
    </Stack>
  );
}

export function CodePane() {
  const firstProjectId = useLiveProjectId();
  const storeProjectId = useStudio((s) => s.liveProjectId);
  const project = useStudio((s) => s.current);
  // Prefer the session's bound backend project, else the first live project,
  // else the in-session fixture id (so the IDE still has a workspace to target).
  const projectId = storeProjectId ?? firstProjectId ?? project.id;

  return (
    <Box sx={{ flex: 1, height: '100%', display: 'flex', flexDirection: 'column', minWidth: 0, bgcolor: 'background.default' }}>
      <IdeTopBar projectName={project.name} />
      <IdeFrame projectId={projectId} />
    </Box>
  );
}
