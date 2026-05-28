import { useState } from 'react';
import { Box, Button, IconButton, MenuItem, Select, Stack, ToggleButton, ToggleButtonGroup, Typography } from '@mui/material';
import { Scene3D, Lightbox } from '@ironflyer/ui-web/fx';

const routes = ['/', '/projects', '/checkout', '/settings'];
type Device = 'desktop' | 'mobile';

// Live preview with a clean base44-style toolbar: route selector, device frame,
// and refresh. The app iframe mounts in the frame once a sandbox is attached.
export function PreviewPane() {
  const [route, setRoute] = useState('/');
  const [device, setDevice] = useState<Device>('desktop');

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
        <IconButton size="small" aria-label="Refresh" sx={{ color: 'text.secondary' }}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><path d="M21 12a9 9 0 1 1-3-6.7L21 8M21 3v5h-5" /></svg>
        </IconButton>
        <Select size="small" value={route} onChange={(e) => setRoute(e.target.value)} sx={(t) => ({ minWidth: 200, fontFamily: t.brand.font.mono, fontSize: '0.82rem', '.MuiOutlinedInput-notchedOutline': { borderColor: t.palette.divider } })}>
          {routes.map((r) => <MenuItem key={r} value={r} sx={{ fontFamily: 'var(--if-font-mono)' }}>{r}</MenuItem>)}
        </Select>
        <Box sx={{ flex: 1 }} />
        <IconButton size="small" aria-label="Fullscreen" sx={{ color: 'text.secondary' }}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><path d="M8 3H3v5M16 3h5v5M3 16v5h5M21 16v5h-5" /></svg>
        </IconButton>
      </Stack>

      {/* framed content */}
      <Box sx={{ flex: 1, display: 'grid', placeItems: 'center', p: 3, overflow: 'auto', position: 'relative' }}>
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
            display: 'grid',
            placeItems: 'center',
            backgroundImage: t.brand.gradient.signatureSoft,
          })}
        >
          <Box sx={{ position: 'absolute', inset: 0, display: 'grid', placeItems: 'center', opacity: 0.55 }}>
            <Box sx={{ width: device === 'mobile' ? 240 : 360 }}><Scene3D height={device === 'mobile' ? 240 : 360} /></Box>
          </Box>
          <Stack alignItems="center" spacing={2} sx={{ position: 'relative', textAlign: 'center', px: 3 }}>
            <Typography variant="h4" sx={{ fontSize: '1.5rem' }}>Building your project…</Typography>
            <Typography sx={{ color: 'text.secondary', maxWidth: 420 }}>
              The preview for <Box component="span" sx={(t) => ({ fontFamily: t.brand.font.mono, color: 'text.primary' })}>{route}</Box> mounts here once the sandbox is running. Open <b>Map</b> or <b>Dashboard</b> to see the finisher gates.
            </Typography>
            <Lightbox>
              <Button component="a" href="/sample-preview.svg" data-fancybox variant="outlined" color="inherit" size="small">View a sample screen</Button>
            </Lightbox>
          </Stack>
        </Box>
      </Box>
    </Box>
  );
}
