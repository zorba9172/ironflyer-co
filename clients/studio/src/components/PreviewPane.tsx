import { Box, Stack, Typography } from '@mui/material';
import { LogoMark } from './LogoMark';

// Live preview of the project. Placeholder "building" state until a runtime is
// attached; this is where the rendered app iframe will mount.
export function PreviewPane() {
  return (
    <Box sx={{ flex: 1, height: '100%', display: 'grid', placeItems: 'center', bgcolor: 'background.default', position: 'relative', overflow: 'hidden' }}>
      <Box sx={(t) => ({ position: 'absolute', inset: 0, backgroundImage: t.brand.gradient.signatureSoft, opacity: 0.5 })} />
      <Stack alignItems="center" spacing={2} sx={{ position: 'relative' }}>
        <Box sx={(t) => ({ width: 84, height: 84, borderRadius: 4, display: 'grid', placeItems: 'center', border: 1, borderColor: 'divider', bgcolor: 'background.paper', boxShadow: t.brand.shadow.md })}>
          <LogoMark size={40} />
        </Box>
        <Typography variant="h4" sx={{ fontSize: '1.6rem' }}>Building your project…</Typography>
        <Typography sx={{ color: 'text.secondary', maxWidth: 420, textAlign: 'center' }}>
          The preview mounts here once the sandbox is running. Switch to <b>Dashboard</b> to see the finisher gates.
        </Typography>
      </Stack>
    </Box>
  );
}
