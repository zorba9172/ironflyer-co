import { Avatar, Box, Button, IconButton, Stack, ToggleButton, ToggleButtonGroup, Typography } from '@mui/material';
import { useNavigate } from 'react-router-dom';
import { LogoMark } from './LogoMark';

export type EditorTab = 'preview' | 'map' | 'dashboard';

export function EditorTopBar({ projectName, tab, onTab, onDeploy }: { projectName: string; tab: EditorTab; onTab: (t: EditorTab) => void; onDeploy: () => void }) {
  const navigate = useNavigate();
  return (
    <Box sx={{ display: 'grid', gridTemplateColumns: '1fr auto 1fr', alignItems: 'center', px: 2, height: 56, borderBottom: 1, borderColor: 'divider', bgcolor: 'background.paper' }}>
      <Stack direction="row" alignItems="center" spacing={1.5} sx={{ minWidth: 0 }}>
        <IconButton size="small" onClick={() => navigate('/')} aria-label="Home"><LogoMark size={22} /></IconButton>
        <Box sx={{ minWidth: 0 }}>
          <Typography sx={{ fontWeight: 600, fontSize: '0.9rem' }} noWrap>{projectName}</Typography>
          <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.66rem', color: 'text.disabled' })} noWrap>My Workspace</Typography>
        </Box>
      </Stack>

      <ToggleButtonGroup
        exclusive
        size="small"
        value={tab}
        onChange={(_, v) => v && onTab(v)}
        sx={{ bgcolor: 'action.hover', borderRadius: 99, p: 0.5, '& .MuiToggleButton-root': { border: 0, borderRadius: '99px !important', px: 2, py: 0.5, textTransform: 'none', color: 'text.secondary', '&.Mui-selected': { bgcolor: 'background.paper', color: 'text.primary', boxShadow: 1 } } }}
      >
        <ToggleButton value="preview">Preview</ToggleButton>
        <ToggleButton value="map">Map</ToggleButton>
        <ToggleButton value="dashboard">Dashboard</ToggleButton>
      </ToggleButtonGroup>

      <Stack direction="row" alignItems="center" justifyContent="flex-end" spacing={1}>
        <Avatar sx={{ width: 26, height: 26, fontSize: 12, bgcolor: 'action.selected', color: 'text.primary' }}>M</Avatar>
        <IconButton size="small" aria-label="Open repo" sx={{ color: 'text.secondary' }}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2a10 10 0 00-3.16 19.49c.5.09.68-.22.68-.48v-1.7c-2.78.6-3.37-1.34-3.37-1.34-.45-1.16-1.11-1.47-1.11-1.47-.91-.62.07-.6.07-.6 1 .07 1.53 1.03 1.53 1.03.9 1.53 2.36 1.09 2.94.83.09-.65.35-1.09.63-1.34-2.22-.25-4.55-1.11-4.55-4.94 0-1.09.39-1.98 1.03-2.68-.1-.25-.45-1.27.1-2.65 0 0 .84-.27 2.75 1.02a9.5 9.5 0 015 0c1.91-1.29 2.75-1.02 2.75-1.02.55 1.38.2 2.4.1 2.65.64.7 1.03 1.59 1.03 2.68 0 3.84-2.34 4.69-4.57 4.94.36.31.68.92.68 1.85v2.74c0 .27.18.58.69.48A10 10 0 0012 2z" /></svg>
        </IconButton>
        <Button variant="contained" size="small" onClick={onDeploy}>Deploy</Button>
      </Stack>
    </Box>
  );
}
