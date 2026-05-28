import { Box, Container, Card, Typography, IconButton, Stack } from '@mui/material';
import { useThemeMode } from '@ironflyer/ui-web';
import { formatUSD, formatCompact } from '@ironflyer/core';

const metrics = [
  { label: 'MRR', value: formatUSD(48210, { cents: false }) },
  { label: 'Active projects', value: formatCompact(1284) },
  { label: 'Provider cost (30d)', value: formatUSD(11930, { cents: false }) },
  { label: 'Margin (30d)', value: '64%' },
];

// Admin shell scaffold — operators watch revenue, spend, and project health.
export function App() {
  const { mode, toggle } = useThemeMode();
  return (
    <Box sx={{ minHeight: '100vh', bgcolor: 'background.default' }}>
      <Box component="header" sx={{ borderBottom: 1, borderColor: 'divider', px: 3, py: 2, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Typography variant="h6">Ironflyer Backoffice</Typography>
        <IconButton onClick={toggle} size="small" aria-label="toggle theme">{mode === 'dark' ? '☼' : '☾'}</IconButton>
      </Box>

      <Container maxWidth="lg" sx={{ py: 6 }}>
        <Typography variant="h4" sx={{ fontWeight: 700, mb: 4 }}>Overview</Typography>
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', md: 'repeat(4, 1fr)' }, gap: 2 }}>
          {metrics.map((m) => (
            <Card key={m.label} sx={{ p: 3 }}>
              <Typography variant="caption" color="text.secondary" sx={{ textTransform: 'uppercase', letterSpacing: 1 }}>{m.label}</Typography>
              <Typography variant="h4" sx={{ fontWeight: 700, mt: 1 }}>{m.value}</Typography>
            </Card>
          ))}
        </Box>

        <Stack sx={{ mt: 4 }}>
          <Card sx={{ p: 3, minHeight: 280, display: 'grid', placeItems: 'center' }}>
            <Typography color="text.secondary">Charts, project table, and spend controls land here next.</Typography>
          </Card>
        </Stack>
      </Container>
    </Box>
  );
}
