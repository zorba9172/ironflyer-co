import { Box, CircularProgress, Stack, Typography } from '@mui/material';
import { useWorkspaceIde } from '@ironflyer/data';
import { useSyncWorkspaceFiles } from '../hooks/useSyncWorkspaceFiles';

// The embedded web IDE is the canonical custom-branded Eclipse Theia app
// (clients/ide/, image ironflyer/theia-ide:latest on :3030). It is the opt-in
// "open the hood" surface for professionals — Ironflyer stays
// visualization-first, so this full IDE is reachable in one click but is never
// the default landing pane. The Theia distro themes its own interior; the
// studio's IdeTopBar (see CodePane) owns the surrounding chrome (the plain
// divider that aligns with the chat rail), so our job here is only the
// lifecycle states + the edge-to-edge iframe, never the editor itself.

const SANDBOX =
  'allow-scripts allow-same-origin allow-forms allow-popups allow-downloads allow-modals';

// No decorative accent bar: the IDE chrome aligns to the same plain divider
// hairline the chat rail uses, so the toolbar border reads continuous with the
// rest of the studio (owner request 2026-05-30) rather than a colored stripe.

function CenteredPanel({ children }: { children: React.ReactNode }) {
  return (
    <Box
      sx={{
        flex: 1,
        minHeight: 0,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        p: 4,
        bgcolor: 'background.default',
      }}
    >
      {children}
    </Box>
  );
}

export function IdeFrame({ projectId }: { projectId?: string }) {
  const { url, ready, isError, error } = useWorkspaceIde(projectId);
  // Once the workspace IDE is ready, push the chat-generated files into it so
  // generated code shows up in the editor (not just an empty workspace).
  useSyncWorkspaceFiles(projectId, ready);

  // (c) No backend / error — the runtime or IDE backend isn't reachable.
  if (isError) {
    return (
      <Box sx={{ flex: 1, height: '100%', display: 'flex', flexDirection: 'column', minWidth: 0 }}>
        <CenteredPanel>
          <Stack spacing={1.5} sx={{ maxWidth: 460, textAlign: 'center' }} alignItems="center">
            <Typography variant="h6">Workspace IDE is offline</Typography>
            <Typography variant="body2" color="text.secondary">
              The runtime service didn&apos;t answer for this workspace, so the embedded IDE
              couldn&apos;t start. Make sure the runtime is running with the Docker driver so it can
              boot a real Linux workspace.
            </Typography>
            <Typography
              variant="caption"
              color="text.secondary"
              sx={(t) => ({ fontFamily: t.brand.font.mono })}
            >
              For local dev you can point the runtime at an existing IDE with the
              {' '}<Box component="span" sx={{ color: 'text.primary' }}>IRONFLYER_IDE_URL</Box>{' '}
              override.
            </Typography>
            {error?.message && (
              <Typography variant="caption" color="text.disabled" noWrap sx={{ maxWidth: '100%' }}>
                {error.message}
              </Typography>
            )}
          </Stack>
        </CenteredPanel>
      </Box>
    );
  }

  // (a) Loading / starting — poll while the IDE backend provisions.
  if (!ready || !url) {
    return (
      <Box sx={{ flex: 1, height: '100%', display: 'flex', flexDirection: 'column', minWidth: 0 }}>
        <CenteredPanel>
          <Stack spacing={2} alignItems="center">
            <CircularProgress size={32} thickness={4} color="primary" />
            <Typography variant="subtitle1">Starting your workspace IDE…</Typography>
            <Typography variant="body2" color="text.secondary">
              Booting a real Linux workspace and warming up the editor. This only takes a moment.
            </Typography>
          </Stack>
        </CenteredPanel>
      </Box>
    );
  }

  // (b) Ready — full-height, edge-to-edge Theia iframe.
  return (
    <Box sx={{ flex: 1, height: '100%', display: 'flex', flexDirection: 'column', minWidth: 0 }}>
      <Box sx={{ flex: 1, minHeight: 0, bgcolor: 'background.default' }}>
        <Box
          component="iframe"
          src={url}
          title="Ironflyer IDE"
          sandbox={SANDBOX}
          allow="clipboard-read; clipboard-write"
          sx={{ width: '100%', height: '100%', border: 'none', display: 'block' }}
        />
      </Box>
    </Box>
  );
}
