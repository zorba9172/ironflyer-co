import { useEffect, useMemo, useState } from 'react';
import { Box, Button, IconButton, Stack, ToggleButton, ToggleButtonGroup, Typography } from '@mui/material';
import { VscWarning, VscWand, VscClose } from 'react-icons/vsc';
import { Scene3D, Lightbox, LivePreview, toast, type LivePreviewTemplate } from '@ironflyer/ui-web/fx';
import { useStudio } from '../store';
import { useThemeMode } from '../theme';
import { text } from '@ironflyer/design-tokens/brand';

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

// Live preview: renders the generated project once the agent has streamed any
// files; until then it shows the building placeholder.
export function PreviewPane() {
  const [device, setDevice] = useState<Device>('desktop');
  const [nonce, setNonce] = useState(0);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const { mode } = useThemeMode();
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

  const hasApp = count > 0;

  return (
    <Box sx={{ flex: 1, height: '100%', display: 'flex', flexDirection: 'column', bgcolor: 'background.default', minWidth: 0 }}>
      {/* toolbar */}
      <Stack direction="row" alignItems="center" spacing={1.5} sx={{ px: 2, py: 1, borderBottom: 1, borderColor: 'divider' }}>
        <ToggleButtonGroup exclusive size="small" value={device} onChange={(_, v) => v && setDevice(v)} sx={{ '& .MuiToggleButton-root': { px: 1, py: 0.5, border: 1, borderColor: 'divider' } }}>
          <ToggleButton value="desktop" aria-label="Desktop">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><rect x="3" y="4" width="18" height="12" rx="2" /><path d="M8 20h8M12 16v4" /></svg>
          </ToggleButton>
          <ToggleButton value="mobile" aria-label="Mobile">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><rect x="7" y="3" width="10" height="18" rx="2" /><path d="M11 18h2" /></svg>
          </ToggleButton>
        </ToggleButtonGroup>

        <Box sx={{ flex: 1 }} />
        {hasApp && (
          <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s72, color: previewError ? 'error.main' : 'success.main' })}>
            {previewError ? '● build failed' : `● live · ${count} file${count > 1 ? 's' : ''}`}
          </Typography>
        )}
        <IconButton size="small" aria-label="Refresh" onClick={() => setNonce((n) => n + 1)} sx={{ color: 'text.secondary' }}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><path d="M21 12a9 9 0 1 1-3-6.7L21 8M21 3v5h-5" /></svg>
        </IconButton>
      </Stack>

      {/* framed content */}
      <Box sx={{ flex: 1, display: 'grid', placeItems: 'center', p: hasApp ? 2 : 3, overflow: 'auto', position: 'relative' }}>
        <Box
          sx={(t) => ({
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
            backgroundImage: hasApp ? 'none' : t.brand.gradient.signatureSoft,
          })}
        >
          {hasApp ? (
            <>
              {previewError && (
                <Stack
                  direction="row" alignItems="center" spacing={1.5}
                  sx={{ position: 'absolute', top: 0, left: 0, right: 0, zIndex: 5, px: 2, py: 1.25, bgcolor: (t) => `${t.palette.error.main}1f`, borderBottom: 1, borderColor: 'error.main', backdropFilter: 'blur(6px)' }}
                >
                  <Box component="span" sx={{ color: 'error.main', display: 'inline-flex' }}><VscWarning size={16} /></Box>
                  <Box sx={{ flex: 1, minWidth: 0 }}>
                    <Typography sx={{ fontSize: text.s82, fontWeight: 600, color: 'error.main' }}>This preview didn't build</Typography>
                    <Typography noWrap sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s68, color: 'text.secondary' })}>{previewError}</Typography>
                  </Box>
                  <Button size="small" variant="contained" startIcon={<VscWand size={14} />} onClick={fixPreview} sx={{ flexShrink: 0 }}>Fix with agent</Button>
                  <IconButton size="small" aria-label="Dismiss" onClick={() => setPreviewError(null)} sx={{ color: 'text.secondary', flexShrink: 0 }}><VscClose size={14} /></IconButton>
                </Stack>
              )}
              <LivePreview key={nonce} files={map} template={template} dark={mode === 'dark'} onError={setPreviewError} />
            </>
          ) : (
            <>
              <Box sx={{ position: 'absolute', inset: 0, display: 'grid', placeItems: 'center', opacity: 0.55 }}>
                <Box sx={{ width: device === 'mobile' ? 240 : 360 }}><Scene3D height={device === 'mobile' ? 240 : 360} /></Box>
              </Box>
              <Stack alignItems="center" spacing={2} sx={{ position: 'relative', textAlign: 'center', px: 3 }}>
                <Typography variant="h4" sx={{ fontSize: text.s150 }}>No preview yet</Typography>
                <Typography sx={{ color: 'text.secondary', maxWidth: 420 }}>
                  Ask the agent to build something in the chat. As it streams files, your app renders here live — and the source appears in <b>Code</b>.
                </Typography>
                <Lightbox>
                  <Button component="a" href="/sample-preview.svg" data-fancybox variant="outlined" color="inherit" size="small">View a sample screen</Button>
                </Lightbox>
              </Stack>
            </>
          )}
        </Box>
      </Box>
    </Box>
  );
}
