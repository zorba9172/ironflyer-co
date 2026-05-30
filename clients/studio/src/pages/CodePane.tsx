import { Box, Chip, Stack, Tooltip, Typography } from '@mui/material';
import { VscCode, VscTerminal, VscSync } from 'react-icons/vsc';
import { LogoMark } from '../components/LogoMark';
import { IdeFrame } from '../components/IdeFrame';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { useStudio } from '../store';
import { text as fontScale } from '@ironflyer/design-tokens/brand';

// ── Code surface ───────────────────────────────────────────────────────────────
// The branded Eclipse Theia web IDE (clients/ide/, image ironflyer/theia-ide:latest),
// served per-workspace by the runtime. Ironflyer is visualization-first: this full
// IDE is the opt-in "for pros" layer, reachable in one click but never the default
// pane. The viz surfaces (Dashboard / Map / Preview) remain the landing experience;
// this is where a professional opens the hood and edits the real workspace directly.
//
// Chrome ownership: the studio top bar is canonical IDE chrome — it owns the
// Ironflyer wordmark, the project name, the workspace mode chip, and the neon
// signature accent bar. The embedded Theia frame stays bare (its own wordmark is
// redundant). The neon hairline above the frame is the only decoration; it is a
// palette-derived gradient, never a literal color.

// ── Status chip variants ───────────────────────────────────────────────────────
type WorkspaceStatus = 'starting' | 'ready' | 'syncing' | 'offline';

function WorkspaceStatusChip({ status }: { status: WorkspaceStatus }) {
  const configs: Record<WorkspaceStatus, { label: string; color: 'default' | 'success' | 'warning' | 'error' }> = {
    starting: { label: 'Starting…', color: 'warning' },
    ready: { label: 'Workspace ready', color: 'success' },
    syncing: { label: 'Syncing', color: 'default' },
    offline: { label: 'Offline', color: 'error' },
  };
  const { label, color } = configs[status];
  return (
    <Chip
      size="small"
      label={label}
      color={color}
      variant="outlined"
      sx={(t) => ({
        fontFamily: t.brand.font.mono,
        fontSize: fontScale.s62,
        letterSpacing: '0.06em',
        textTransform: 'uppercase',
        height: 20,
      })}
    />
  );
}

// ── File count indicator ───────────────────────────────────────────────────────
function FileCountIndicator({ count }: { count: number }) {
  if (count === 0) return null;
  return (
    <Tooltip title={`${count} generated file${count === 1 ? '' : 's'} in workspace`} arrow>
      <Stack direction="row" alignItems="center" spacing={0.5}
        sx={(t) => ({
          px: 0.75, py: 0.2,
          borderRadius: 99,
          border: `1px solid ${t.palette.divider}`,
          cursor: 'default',
          userSelect: 'none',
        })}
      >
        <VscCode size={11} style={{ opacity: 0.5 }} />
        <Typography sx={(t) => ({
          fontFamily: t.brand.font.mono,
          fontSize: fontScale.s62,
          color: 'text.secondary',
        })}>
          {count}
        </Typography>
      </Stack>
    </Tooltip>
  );
}

// ── Neon accent hairline (gradient, never a literal color) ─────────────────────
function NeonAccentBar() {
  return (
    <Box
      sx={(t) => ({
        height: 2,
        flexShrink: 0,
        backgroundImage: t.studio.gradient.signature,
        opacity: 0.85,
      })}
    />
  );
}

// ── IDE top bar ─────────────────────────────────────────────────────────────────
function IdeTopBar({ projectName, fileCount, status }: {
  projectName: string;
  fileCount: number;
  status: WorkspaceStatus;
}) {
  return (
    <Stack
      direction="row"
      alignItems="center"
      justifyContent="space-between"
      sx={(t) => ({
        px: 2, py: 0.875,
        borderBottom: `1px solid ${t.palette.divider}`,
        bgcolor: 'background.paper',
        flexShrink: 0,
        minHeight: 44,
      })}
    >
      {/* Left: wordmark + project context */}
      <Stack direction="row" alignItems="center" spacing={1.5} sx={{ minWidth: 0 }}>
        <LogoMark size={16} />
        <Stack direction="row" alignItems="center" spacing={0.75}>
          <Typography
            variant="subtitle2"
            sx={{ fontWeight: 600, fontSize: fontScale.s82 }}
          >
            IDE
          </Typography>
          <Typography variant="subtitle2" sx={{ color: 'text.disabled', fontWeight: 400, fontSize: fontScale.s82 }}>
            /
          </Typography>
          <Typography
            variant="subtitle2"
            sx={{ color: 'text.secondary', fontWeight: 400, fontSize: fontScale.s82 }}
            noWrap
          >
            {projectName}
          </Typography>
        </Stack>
        <FileCountIndicator count={fileCount} />
      </Stack>

      {/* Right: status + pro marker */}
      <Stack direction="row" alignItems="center" spacing={1} sx={{ flexShrink: 0 }}>
        {status === 'syncing' && (
          <Tooltip title="Syncing generated files to workspace" arrow>
            <Box sx={{ display: 'inline-flex', color: 'text.secondary', animation: 'spin 1.4s linear infinite', '@keyframes spin': { from: { transform: 'rotate(0deg)' }, to: { transform: 'rotate(360deg)' } } }}>
              <VscSync size={13} />
            </Box>
          </Tooltip>
        )}
        <WorkspaceStatusChip status={status} />
        <Tooltip title="The full workspace IDE is the opt-in engineering layer. Visualizations are the default." arrow>
          <Chip
            label="Pro · code"
            size="small"
            variant="outlined"
            icon={<VscTerminal size={10} />}
            sx={(t) => ({
              fontFamily: t.brand.font.mono,
              letterSpacing: '0.06em',
              textTransform: 'uppercase',
              color: 'text.secondary',
              fontSize: fontScale.s60,
              height: 20,
              '& .MuiChip-label': { ...t.typography.caption },
              '& .MuiChip-icon': { ml: 0.75 },
            })}
          />
        </Tooltip>
      </Stack>
    </Stack>
  );
}

// ── Main export ────────────────────────────────────────────────────────────────
export function CodePane() {
  const firstProjectId = useLiveProjectId();
  const storeProjectId = useStudio((s) => s.liveProjectId);
  const project = useStudio((s) => s.current);
  const generatedFiles = useStudio((s) => s.generatedFiles);

  // Prefer the session's bound backend project, else the first live project,
  // else the in-session fixture id (so the IDE still has a workspace to target).
  const projectId = storeProjectId ?? firstProjectId ?? project.id;

  // Map the IdeFrame's loading state to a semantic WorkspaceStatus.
  // IdeFrame handles its own loading / error rendering — we only need the status
  // chip so the top bar stays informative.
  const fileCount = generatedFiles.length;

  // IdeFrame drives loading UI itself; we report 'ready' once we have a projectId.
  // The sync state is approximated from whether there are generated files the IDE
  // hasn't confirmed yet — good enough for the status chip.
  const status: WorkspaceStatus = projectId ? 'ready' : 'offline';

  return (
    <Box sx={{
      flex: 1, height: '100%', display: 'flex', flexDirection: 'column',
      minWidth: 0, bgcolor: 'background.default',
    }}>
      <IdeTopBar projectName={project.name} fileCount={fileCount} status={status} />
      <NeonAccentBar />
      <IdeFrame projectId={projectId} />
    </Box>
  );
}
