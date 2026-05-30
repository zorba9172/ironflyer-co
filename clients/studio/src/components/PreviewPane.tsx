import { useEffect, useMemo, useState } from 'react';
import { Box, Button, Chip, IconButton, Stack, ToggleButton, ToggleButtonGroup, Tooltip, Typography } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { Lightbox, LivePreview, toast, type LivePreviewTemplate } from '@ironflyer/ui-web/fx';
import { useStudio } from '../store';
import { useThemeMode } from '../theme';
import { NeonConstellation3D } from './studio';
import { Icon } from '../icons';
import type { Gate } from '../studioData';
import { text } from '@ironflyer/design-tokens/brand';
import { studioTokens } from '../theme';

type Device = 'desktop' | 'mobile';

// The request handed to the agent when the operator taps "Fix preview" — it
// carries the real bundler error so the fix is grounded, not a guess.
function repairPrompt(error: string): string {
  const trimmed = error.length > 1200 ? `${error.slice(0, 1200)}\n… [truncated]` : error;
  return `The live preview failed to build with this error:\n\n\`\`\`\n${trimmed}\n\`\`\`\n\nRead the generated files, find the root cause, and propose a patch so the preview compiles and renders cleanly.`;
}

// Maps the streamed files into a Sandpack file map: leading-slash keys, with a
// shared project root directory stripped so the bundler's entry resolves.
function toSandpackFiles(files: { path: string; content: string }[]): { map: Record<string, string>; template: LivePreviewTemplate } {
  let rel = files.map((f) => f.path.replace(/^\.?\//, ''));
  const top = (p: string) => p.split('/')[0];
  if (rel.length > 1 && rel.every((p) => p.includes('/') && top(p) === top(rel[0]!))) {
    const root = `${top(rel[0]!)}/`;
    rel = rel.map((p) => p.slice(root.length));
  }
  const map: Record<string, string> = {};
  files.forEach((f, i) => { map[`/${rel[i]}`] = f.content; });
  const hasReactEntry = Object.keys(map).some((p) => /\/src\/(main|index|app)\.(t|j)sx?$/i.test(p));
  return { map, template: hasReactEntry ? 'vite-react-ts' : 'static' };
}

// Build constellation nodes/links from real gate state so the ambient 3D
// object mirrors what the orchestrator is actually running.
function gatesToConstellation(gates: Gate[]) {
  const { neon } = studioTokens;
  const statusColor: Record<string, string> = {
    closed: neon.success,
    running: neon.blue,
    open: neon.warning,
    blocked: neon.danger,
    unstarted: neon.violet,
  };

  // Orchestrator hub is the first node; gates fan out from it.
  const nodes = [
    { id: 'orchestrator', value: 0.9, color: neon.violet, x: 0, y: 0, z: 0 },
    ...gates.map((g, i) => {
      const angle = (i / gates.length) * Math.PI * 2;
      const r = 0.65;
      return {
        id: g.id,
        value: g.level > 0 ? 0.3 + g.level * 0.5 : 0.25,
        color: statusColor[g.status] ?? neon.violet,
        x: Math.cos(angle) * r,
        y: (Math.sin(angle) * r * 0.5),
        z: Math.sin(angle) * r * 0.4,
      };
    }),
  ];

  const links = gates.map((g) => ({
    source: 'orchestrator',
    target: g.id,
    color: statusColor[g.status] ?? neon.violet,
  }));

  // Running gates get a cross-link to adjacent gates for a denser graph.
  gates.forEach((g, i) => {
    if (g.status === 'running' || g.status === 'open') {
      const next = gates[(i + 1) % gates.length];
      if (next && next.id !== g.id) {
        links.push({ source: g.id, target: next.id, color: neon.blue });
      }
    }
  });

  return { nodes, links };
}

// Live preview: renders the generated project once the agent has streamed any
// files; until then it shows the building placeholder.
export function PreviewPane({ gates = [] }: { gates?: Gate[] }) {
  const [device, setDevice] = useState<Device>('desktop');
  const [nonce, setNonce] = useState(0);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const { mode } = useThemeMode();
  const theme = useTheme();
  const generated = useStudio((s) => s.generatedFiles);
  const requestRepair = useStudio((s) => s.requestRepair);

  // A fresh build or a manual refresh starts from a clean slate; the bundler
  // re-reports an error if it's still broken.
  useEffect(() => { setPreviewError(null); }, [generated, nonce]);

  const fixPreview = () => {
    if (!previewError) return;
    requestRepair(repairPrompt(previewError));
    toast('Sent the preview error to the agent — watch the chat for the fix.', 'info');
  };

  const { map, template, count } = useMemo(() => {
    if (generated.length === 0) return { map: {}, template: 'vite-react-ts' as LivePreviewTemplate, count: 0 };
    const { map, template } = toSandpackFiles(generated.map((g) => ({ path: g.path, content: g.content })));
    return { map, template, count: Object.keys(map).length };
  }, [generated]);

  // Build the ambient network from real gate state.
  const { nodes: constellationNodes, links: constellationLinks } = useMemo(
    () => gatesToConstellation(gates),
    [gates],
  );

  const hasApp = count > 0;
  const constellationHeight = device === 'mobile' ? 240 : 320;

  return (
    <Box sx={{ flex: 1, height: '100%', display: 'flex', flexDirection: 'column', bgcolor: 'background.default', minWidth: 0 }}>
      {/* toolbar */}
      <Stack
        direction="row"
        alignItems="center"
        spacing={1.5}
        sx={{ px: 2, py: 1, borderBottom: 1, borderColor: 'divider' }}
      >
        <ToggleButtonGroup
          exclusive
          size="small"
          value={device}
          onChange={(_, v) => v && setDevice(v)}
          sx={{
            bgcolor: 'action.hover',
            borderRadius: 99,
            p: 0.25,
            '& .MuiToggleButtonGroup-grouped': {
              border: 0,
              borderRadius: '99px !important',
              px: 1,
              py: 0.5,
              color: 'text.secondary',
              '&.Mui-selected': {
                color: 'text.primary',
                bgcolor: 'background.paper',
                boxShadow: 1,
                '&:hover': { bgcolor: 'background.paper' },
              },
            },
          }}
        >
          <ToggleButton value="desktop" aria-label="Desktop view">
            <Icon name="dashboard" size={15} />
          </ToggleButton>
          <ToggleButton value="mobile" aria-label="Mobile view">
            <Icon name="smartphone" size={15} />
          </ToggleButton>
        </ToggleButtonGroup>

        <Box sx={{ flex: 1 }} />

        {hasApp && (
          <Tooltip title={previewError ? previewError : `${count} file${count > 1 ? 's' : ''} loaded`} arrow>
            <Stack
              direction="row"
              alignItems="center"
              spacing={0.75}
              sx={{ cursor: 'default', color: previewError ? 'error.main' : 'success.main' }}
            >
              <Box sx={{ width: 7, height: 7, borderRadius: 99, bgcolor: 'currentColor', flexShrink: 0 }} />
              <Typography
                sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s72, color: 'inherit' })}
              >
                {previewError ? 'build failed' : `live · ${count} file${count > 1 ? 's' : ''}`}
              </Typography>
            </Stack>
          </Tooltip>
        )}

        {/* Gate summary pill when there are open gates */}
        {gates.length > 0 && !hasApp && (
          <Stack direction="row" spacing={0.75} alignItems="center">
            {gates.filter((g) => g.status !== 'unstarted').slice(0, 4).map((g) => {
              const dotColor =
                g.status === 'closed'
                  ? theme.studio.neon.success
                  : g.status === 'running'
                  ? theme.studio.neon.blue
                  : g.status === 'open'
                  ? theme.studio.neon.warning
                  : g.status === 'blocked'
                  ? theme.studio.neon.danger
                  : theme.palette.text.disabled;
              return (
                <Tooltip key={g.id} title={`${g.name}: ${g.status}${g.blocking ? ` — ${g.blocking}` : ''}`} arrow>
                  <Box
                    sx={{ width: 8, height: 8, borderRadius: 99, bgcolor: dotColor, flexShrink: 0, cursor: 'default' }}
                  />
                </Tooltip>
              );
            })}
          </Stack>
        )}

        <Tooltip title="Refresh preview" arrow>
          <IconButton
            size="small"
            aria-label="Refresh preview"
            onClick={() => setNonce((n) => n + 1)}
            sx={{ color: 'text.secondary', '&:hover': { color: 'text.primary' } }}
          >
            <Icon name="refresh" size={15} />
          </IconButton>
        </Tooltip>
      </Stack>

      {/* framed content */}
      <Box
        sx={{
          flex: 1,
          display: 'grid',
          placeItems: 'center',
          p: hasApp ? 2 : 3,
          overflow: 'auto',
          position: 'relative',
        }}
      >
        <Box
          sx={(_t) => ({
            width: device === 'mobile' ? 390 : '100%',
            maxWidth: device === 'mobile' ? 390 : 1100,
            height: '100%',
            borderRadius: device === 'mobile' ? 6 : 4,
            border: 1,
            borderColor: 'divider',
            bgcolor: 'background.paper',
            position: 'relative',
            overflow: 'hidden',
            display: hasApp ? 'block' : 'grid',
            placeItems: 'center',
          })}
        >
          {hasApp ? (
            <>
              {previewError && (
                <Stack
                  direction="row"
                  alignItems="center"
                  spacing={1.5}
                  sx={(t) => ({
                    position: 'absolute', top: 0, left: 0, right: 0, zIndex: 5,
                    px: 2, py: 1.25,
                    bgcolor: `${t.palette.error.main}1f`,
                    borderBottom: 1,
                    borderColor: 'error.main',
                    backdropFilter: 'blur(6px)',
                  })}
                >
                  <Box component="span" sx={{ color: 'error.main', display: 'inline-flex' }}>
                    <Icon name="alert" size={16} />
                  </Box>
                  <Box sx={{ flex: 1, minWidth: 0 }}>
                    <Typography variant="subtitle2" sx={{ color: 'error.main' }}>
                      This preview didn't build
                    </Typography>
                    <Typography
                      noWrap
                      sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s68, color: 'text.secondary' })}
                    >
                      {previewError}
                    </Typography>
                  </Box>
                  <Button
                    size="small"
                    variant="contained"
                    startIcon={<Icon name="sparkles" size={14} />}
                    onClick={fixPreview}
                    sx={{ flexShrink: 0 }}
                  >
                    Fix with agent
                  </Button>
                  <IconButton
                    size="small"
                    aria-label="Dismiss error"
                    onClick={() => setPreviewError(null)}
                    sx={{ color: 'text.secondary', flexShrink: 0 }}
                  >
                    <Icon name="close" size={14} />
                  </IconButton>
                </Stack>
              )}
              <LivePreview key={nonce} files={map} template={template} dark={mode === 'dark'} onError={setPreviewError} />
            </>
          ) : (
            /* ── Empty state: calm AI-network visual bound to real gate state ── */
            <Stack
              alignItems="center"
              spacing={0}
              sx={{ position: 'relative', width: '100%', height: '100%', overflow: 'hidden' }}
            >
              {/* Ambient constellation — real gate/agent network, slowly rotating */}
              <Box
                sx={{
                  position: 'absolute',
                  inset: 0,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  opacity: 0.72,
                  pointerEvents: 'none',
                }}
              >
                <NeonConstellation3D
                  nodes={constellationNodes}
                  links={constellationLinks}
                  height={constellationHeight}
                  rotate
                />
              </Box>

              {/* Gate legend — anchored below the constellation */}
              {gates.length > 0 && (
                <Box
                  sx={{
                    position: 'absolute',
                    bottom: 80,
                    left: '50%',
                    transform: 'translateX(-50%)',
                    display: 'flex',
                    gap: 1,
                    flexWrap: 'wrap',
                    justifyContent: 'center',
                    maxWidth: 360,
                  }}
                >
                  {gates.map((g) => {
                    const dotColor =
                      g.status === 'closed'
                        ? theme.studio.neon.success
                        : g.status === 'running'
                        ? theme.studio.neon.blue
                        : g.status === 'open'
                        ? theme.studio.neon.warning
                        : g.status === 'blocked'
                        ? theme.studio.neon.danger
                        : theme.palette.text.disabled;
                    return (
                      <Chip
                        key={g.id}
                        size="small"
                        label={g.name}
                        icon={
                          <Box
                            component="span"
                            sx={{ width: 6, height: 6, borderRadius: 99, bgcolor: dotColor, flexShrink: 0, ml: 0.75 }}
                          />
                        }
                        sx={(t) => ({
                          height: 22,
                          fontSize: text.s70,
                          fontFamily: t.brand.font.mono,
                          bgcolor: `${dotColor}18`,
                          border: `1px solid ${dotColor}33`,
                          color: dotColor,
                          '& .MuiChip-icon': { ml: 0.5 },
                        })}
                      />
                    );
                  })}
                </Box>
              )}

              {/* Centered copy */}
              <Stack
                alignItems="center"
                spacing={2}
                sx={{
                  position: 'absolute',
                  bottom: gates.length > 0 ? 140 : 100,
                  left: '50%',
                  transform: 'translateX(-50%)',
                  textAlign: 'center',
                  px: 3,
                  width: '100%',
                  maxWidth: 480,
                }}
              >
                <Typography variant="h3">No preview yet</Typography>
                <Typography variant="body2" sx={{ color: 'text.secondary', maxWidth: 400, lineHeight: 1.55 }}>
                  Ask the agent to build something in the chat. As it streams files, your app
                  renders here live — and the source appears in <b>Code</b>.
                </Typography>
                <Lightbox>
                  <Button
                    component="a"
                    href="/sample-preview.svg"
                    data-fancybox
                    variant="outlined"
                    color="inherit"
                    size="small"
                  >
                    View a sample screen
                  </Button>
                </Lightbox>
              </Stack>
            </Stack>
          )}
        </Box>
      </Box>
    </Box>
  );
}
