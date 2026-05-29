import { useState } from 'react';
import { Box, Button, CircularProgress, IconButton, Menu, MenuItem, Stack, ToggleButton, ToggleButtonGroup, Typography } from '@mui/material';
import { useNavigate } from 'react-router-dom';
import { toast } from '@ironflyer/ui-web/fx';
import { LogoMark } from './LogoMark';
import { AccountMenu } from './AccountMenu';
import { CostHUD } from './CostHUD';
import { PrivateModeChip } from './PrivateModeChip';
import { useStudio } from '../store';
import { useSaveProject } from '../hooks/useSaveProject';

export type EditorTab =
  | 'theater' | 'preview' | 'map' | 'security' | 'code'
  | 'dashboard' | 'documents' | 'logs' | 'performance' | 'quality' | 'agents' | 'team'
  | 'data' | 'users' | 'analytics' | 'domains' | 'automations' | 'api' | 'marketing' | 'settings';

const DASH_GROUP: EditorTab[] = ['dashboard', 'agents', 'team', 'quality', 'performance', 'logs', 'documents'];
const dashItems: { value: EditorTab; label: string }[] = [
  { value: 'dashboard', label: 'Dashboard' },
  { value: 'agents', label: 'Execution team' },
  { value: 'team', label: 'Execution team graph' },
  { value: 'quality', label: 'Code quality' },
  { value: 'performance', label: 'Performance' },
  { value: 'logs', label: 'Logs' },
  { value: 'documents', label: 'Documents' },
];

// Operate group — the post-deploy "run the app" surfaces (Base44-parity, but
// viz-first and gate-aware). Grouped behind one dropdown so the top bar stays
// lean even as the surface count grows.
const OPERATE_GROUP: EditorTab[] = ['data', 'users', 'analytics', 'domains', 'automations', 'api', 'marketing', 'settings'];
const operateItems: { value: EditorTab; label: string }[] = [
  { value: 'data', label: 'Data' },
  { value: 'users', label: 'Users' },
  { value: 'analytics', label: 'Analytics' },
  { value: 'domains', label: 'Domains' },
  { value: 'automations', label: 'Automations' },
  { value: 'api', label: 'API' },
  { value: 'marketing', label: 'Marketing' },
  { value: 'settings', label: 'Settings' },
];

export function EditorTopBar({ projectName, tab, onTab, onDeploy }: { projectName: string; tab: EditorTab; onTab: (t: EditorTab) => void; onDeploy: () => void }) {
  const navigate = useNavigate();
  const [dashAnchor, setDashAnchor] = useState<null | HTMLElement>(null);
  const [operateAnchor, setOperateAnchor] = useState<null | HTMLElement>(null);
  const inDash = DASH_GROUP.includes(tab);
  const inOperate = OPERATE_GROUP.includes(tab);
  const dashLabel = dashItems.find((d) => d.value === tab)?.label ?? 'Dashboard';
  const operateLabel = operateItems.find((d) => d.value === tab)?.label ?? 'Operate';
  const fileCount = useStudio((s) => s.generatedFiles.length);
  const saved = useStudio((s) => s.saved);
  const { save, saving } = useSaveProject();

  const onSave = async () => {
    const r = await save();
    if (r.ok) toast('Project saved.', 'success');
    else toast(r.error ?? 'Save failed.', 'error');
  };

  return (
    <Box sx={{ display: 'grid', gridTemplateColumns: '1fr auto 1fr', alignItems: 'center', px: 2, height: 56, borderBottom: 1, borderColor: 'divider', bgcolor: 'background.paper' }}>
      <Stack direction="row" alignItems="center" spacing={1.5} sx={{ minWidth: 0 }}>
        <IconButton size="small" onClick={() => navigate('/')} aria-label="Home"><LogoMark size={22} /></IconButton>
        <Box sx={{ minWidth: 0 }}>
          <Typography sx={{ fontWeight: 600, fontSize: '0.9rem' }} noWrap>{projectName}</Typography>
          <Stack direction="row" alignItems="center" spacing={0.75}>
            {fileCount > 0 && <Box sx={{ width: 6, height: 6, borderRadius: 99, bgcolor: saved ? 'success.main' : 'warning.main' }} />}
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.66rem', color: 'text.disabled' })} noWrap>
              {fileCount === 0 ? 'My Workspace' : saved ? 'All changes saved' : 'Unsaved changes'}
            </Typography>
          </Stack>
        </Box>
      </Stack>

      <Box sx={{ display: 'flex', alignItems: 'center', bgcolor: 'action.hover', borderRadius: 99, p: 0.5 }}>
        <ToggleButtonGroup
          exclusive
          size="small"
          value={inDash ? null : tab}
          onChange={(_, v) => v && onTab(v)}
          sx={{ '& .MuiToggleButton-root': { border: 0, borderRadius: '99px !important', px: 2, py: 0.5, textTransform: 'none', color: 'text.secondary', '&.Mui-selected': { bgcolor: 'background.paper', color: 'text.primary', boxShadow: 1 } } }}
        >
          <ToggleButton value="theater">Theater</ToggleButton>
          <ToggleButton value="preview">Preview</ToggleButton>
          <ToggleButton value="map">Map</ToggleButton>
          <ToggleButton value="security">Security</ToggleButton>
          <ToggleButton value="code">Code</ToggleButton>
        </ToggleButtonGroup>
        <Button
          size="small"
          onClick={(e) => setDashAnchor(e.currentTarget)}
          endIcon={<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M6 9l6 6 6-6" /></svg>}
          sx={{ borderRadius: 99, px: 2, py: 0.5, textTransform: 'none', color: inDash ? 'text.primary' : 'text.secondary', bgcolor: inDash ? 'background.paper' : 'transparent', boxShadow: inDash ? 1 : 0 }}
        >
          {dashLabel}
        </Button>
        <Menu anchorEl={dashAnchor} open={!!dashAnchor} onClose={() => setDashAnchor(null)} slotProps={{ paper: { sx: { mt: 1, minWidth: 180, border: 1, borderColor: 'divider', backgroundImage: 'none' } } }}>
          {dashItems.map((d) => (
            <MenuItem key={d.value} selected={tab === d.value} onClick={() => { onTab(d.value); setDashAnchor(null); }}>{d.label}</MenuItem>
          ))}
        </Menu>
        <Button
          size="small"
          onClick={(e) => setOperateAnchor(e.currentTarget)}
          endIcon={<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M6 9l6 6 6-6" /></svg>}
          sx={{ borderRadius: 99, px: 2, py: 0.5, textTransform: 'none', color: inOperate ? 'text.primary' : 'text.secondary', bgcolor: inOperate ? 'background.paper' : 'transparent', boxShadow: inOperate ? 1 : 0 }}
        >
          {operateLabel}
        </Button>
        <Menu anchorEl={operateAnchor} open={!!operateAnchor} onClose={() => setOperateAnchor(null)} slotProps={{ paper: { sx: { mt: 1, minWidth: 180, border: 1, borderColor: 'divider', backgroundImage: 'none' } } }}>
          {operateItems.map((d) => (
            <MenuItem key={d.value} selected={tab === d.value} onClick={() => { onTab(d.value); setOperateAnchor(null); }}>{d.label}</MenuItem>
          ))}
        </Menu>
      </Box>

      <Stack direction="row" alignItems="center" justifyContent="flex-end" spacing={1}>
        <PrivateModeChip />
        <CostHUD />
        <IconButton size="small" aria-label="Open repo" sx={{ color: 'text.secondary' }}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2a10 10 0 00-3.16 19.49c.5.09.68-.22.68-.48v-1.7c-2.78.6-3.37-1.34-3.37-1.34-.45-1.16-1.11-1.47-1.11-1.47-.91-.62.07-.6.07-.6 1 .07 1.53 1.03 1.53 1.03.9 1.53 2.36 1.09 2.94.83.09-.65.35-1.09.63-1.34-2.22-.25-4.55-1.11-4.55-4.94 0-1.09.39-1.98 1.03-2.68-.1-.25-.45-1.27.1-2.65 0 0 .84-.27 2.75 1.02a9.5 9.5 0 015 0c1.91-1.29 2.75-1.02 2.75-1.02.55 1.38.2 2.4.1 2.65.64.7 1.03 1.59 1.03 2.68 0 3.84-2.34 4.69-4.57 4.94.36.31.68.92.68 1.85v2.74c0 .27.18.58.69.48A10 10 0 0012 2z" /></svg>
        </IconButton>
        <Button
          variant="outlined"
          color="inherit"
          size="small"
          onClick={onSave}
          disabled={saving || fileCount === 0 || saved}
          startIcon={saving ? <CircularProgress size={13} color="inherit" /> : undefined}
        >
          {saving ? 'Saving' : saved && fileCount > 0 ? 'Saved' : 'Save'}
        </Button>
        <Button variant="contained" size="small" onClick={onDeploy}>Deploy</Button>
        <AccountMenu size={28} />
      </Stack>
    </Box>
  );
}
