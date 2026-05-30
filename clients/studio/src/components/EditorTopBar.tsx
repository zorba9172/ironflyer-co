import { useState } from 'react';
import { Box, Button, CircularProgress, IconButton, Menu, MenuItem, Stack, Tooltip, Typography } from '@mui/material';
import { useNavigate } from 'react-router-dom';
import { toast } from '@ironflyer/ui-web/fx';
import { Icon } from '../icons';
import { LogoMark } from './LogoMark';
import { AccountMenu } from './AccountMenu';
import { CostHUD } from './CostHUD';
import { PrivateModeChip } from './PrivateModeChip';
import { useStudio } from '../store';
import { useSaveProject } from '../hooks/useSaveProject';
import { useThemeMode } from '../theme';
import { text } from '@ironflyer/design-tokens/brand';

export type EditorTab =
  | 'preview' | 'map' | 'security' | 'code'
  | 'dashboard' | 'documents' | 'logs' | 'quality' | 'team'
  | 'data' | 'users' | 'grow' | 'settings';

type EditorTabGroupId = 'build' | 'preview' | 'review' | 'operate' | 'business';
type EditorTabTone = 'success' | 'warning' | 'error' | 'info';

export interface EditorDeployReadiness {
  tone: EditorTabTone;
  label: string;
  detail: string;
}

interface EditorTabItem { value: EditorTab; label: string }

const TAB_GROUPS: { id: EditorTabGroupId; label: string; items: EditorTabItem[] }[] = [
  {
    id: 'build',
    label: 'Build',
    items: [
      { value: 'code', label: 'Code' },
      { value: 'team', label: 'Execution team' },
      { value: 'documents', label: 'Documents' },
    ],
  },
  {
    id: 'preview',
    label: 'Preview',
    items: [
      { value: 'preview', label: 'Live build' },
    ],
  },
  {
    id: 'review',
    label: 'Review',
    items: [
      { value: 'dashboard', label: 'Dashboard' },
      { value: 'map', label: 'Gate map' },
      { value: 'quality', label: 'Quality' },
      { value: 'security', label: 'Security' },
      { value: 'logs', label: 'Logs' },
    ],
  },
  {
    id: 'operate',
    label: 'Operate',
    items: [
      { value: 'data', label: 'Data' },
      { value: 'users', label: 'Users' },
      { value: 'settings', label: 'Settings' },
    ],
  },
  {
    id: 'business',
    label: 'Business',
    items: [
      { value: 'grow', label: 'Grow' },
    ],
  },
];

export function EditorTopBar({ projectName, tab, onTab, onDeploy, deployReadiness }: { projectName: string; tab: EditorTab; onTab: (t: EditorTab) => void; onDeploy: () => void; deployReadiness?: EditorDeployReadiness }) {
  const navigate = useNavigate();
  const [groupAnchor, setGroupAnchor] = useState<null | { id: EditorTabGroupId; el: HTMLElement }>(null);
  const activeGroup = TAB_GROUPS.find((g) => g.items.some((d) => d.value === tab)) ?? TAB_GROUPS[0]!;
  const openGroup = TAB_GROUPS.find((g) => g.id === groupAnchor?.id);
  const fileCount = useStudio((s) => s.generatedFiles.length);
  const saved = useStudio((s) => s.saved);
  const { save, saving } = useSaveProject();
  const { mode, toggle } = useThemeMode();
  const readinessColor = deployReadiness ? `${deployReadiness.tone}.main` : 'text.disabled';

  const onSave = async () => {
    const r = await save();
    if (r.ok) toast('Project saved.', 'success');
    else toast(r.error ?? 'Save failed.', 'error');
  };

  return (
    <Box
      sx={{
        display: 'grid',
        gridTemplateColumns: { xs: 'auto minmax(0, 1fr) auto', md: '1fr auto 1fr' },
        gap: { xs: 0.5, md: 1 },
        alignItems: 'center',
        px: { xs: 0.75, md: 2 },
        height: 56,
        borderBottom: 1,
        borderColor: 'divider',
        bgcolor: 'background.paper',
      }}
    >
      <Stack direction="row" alignItems="center" spacing={1.5} sx={{ minWidth: 0 }}>
        <IconButton size="small" onClick={() => navigate('/')} aria-label="Home"><LogoMark size={22} /></IconButton>
        <Box sx={{ minWidth: 0, display: { xs: 'none', md: 'block' } }}>
          <Typography sx={{ fontWeight: 600, fontSize: text.s90 }} noWrap>{projectName}</Typography>
          <Stack direction="row" alignItems="center" spacing={0.75} sx={{ minWidth: 0 }}>
            {fileCount > 0 && <Box sx={{ width: 6, height: 6, borderRadius: 99, bgcolor: saved ? 'success.main' : 'warning.main' }} />}
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s66, color: 'text.disabled' })} noWrap>
              {fileCount === 0 ? 'My Workspace' : saved ? 'All changes saved' : 'Unsaved changes'}
            </Typography>
            {deployReadiness && (
              <>
                <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s66, color: 'text.disabled' })}>/</Typography>
                <Tooltip title={deployReadiness.detail} arrow disableInteractive>
                  <Stack direction="row" alignItems="center" spacing={0.55} sx={{ minWidth: 0 }}>
                    <Box sx={{ width: 6, height: 6, borderRadius: 99, bgcolor: readinessColor, flexShrink: 0 }} />
                    <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s66, color: 'text.secondary' })} noWrap>
                      {deployReadiness.label}
                    </Typography>
                  </Stack>
                </Tooltip>
              </>
            )}
          </Stack>
        </Box>
      </Stack>

      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          gap: 0.25,
          bgcolor: 'action.hover',
          borderRadius: 99,
          p: 0.5,
          minWidth: 0,
          overflowX: 'auto',
          scrollbarWidth: 'none',
          '&::-webkit-scrollbar': { display: 'none' },
        }}
      >
        {TAB_GROUPS.map((group) => {
          const selected = activeGroup.id === group.id;
          const activeLabel = group.items.length > 1 ? group.items.find((d) => d.value === tab)?.label : null;
          return (
            <Button
              key={group.id}
              size="small"
              onClick={(e) => {
                if (group.items.length === 1) {
                  onTab(group.items[0]!.value);
                  return;
                }
                setGroupAnchor({ id: group.id, el: e.currentTarget });
              }}
              endIcon={group.items.length > 1 ? <Icon name="chevronDown" size={12} /> : undefined}
              sx={{
                borderRadius: 99,
                px: { xs: 0.9, md: 1.4 },
                py: 0.5,
                minWidth: 0,
                flexShrink: 0,
                order: { xs: selected ? -1 : 0, md: 0 },
                textTransform: 'none',
                fontSize: { xs: text.s74, md: text.s82 },
                whiteSpace: 'nowrap',
                color: selected ? 'text.primary' : 'text.secondary',
                bgcolor: selected ? 'background.paper' : 'transparent',
                boxShadow: selected ? 1 : 0,
              }}
            >
              <Box component="span" sx={{ minWidth: 0, whiteSpace: 'nowrap' }}>{group.label}</Box>
              {selected && activeLabel && (
                <Box component="span" sx={{ display: { xs: 'none', lg: 'inline' }, ml: 0.75, color: 'text.secondary', maxWidth: 110, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                  {activeLabel}
                </Box>
              )}
            </Button>
          );
        })}
        <Menu anchorEl={groupAnchor?.el ?? null} open={!!groupAnchor} onClose={() => setGroupAnchor(null)} slotProps={{ paper: { sx: { mt: 1, minWidth: 190, border: 1, borderColor: 'divider', backgroundImage: 'none' } } }}>
          {(openGroup?.items ?? []).map((d) => (
            <MenuItem key={d.value} selected={tab === d.value} onClick={() => { onTab(d.value); setGroupAnchor(null); }}>{d.label}</MenuItem>
          ))}
        </Menu>
      </Box>

      <Stack direction="row" alignItems="center" justifyContent="flex-end" spacing={{ xs: 0.5, md: 1 }} sx={{ minWidth: 0 }}>
        <Box sx={{ display: { xs: 'none', lg: 'block' } }}><PrivateModeChip /></Box>
        <Box sx={{ display: { xs: 'none', sm: 'block' } }}><CostHUD /></Box>
        <Button
          sx={{ display: { xs: 'none', md: 'inline-flex' } }}
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
        <Tooltip title={mode === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}>
          <IconButton
            size="small"
            onClick={toggle}
            aria-label={mode === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
            sx={{
              display: { xs: 'none', sm: 'inline-flex' },
              width: 30,
              height: 30,
              border: 1,
              borderColor: 'divider',
              color: 'text.secondary',
              bgcolor: 'action.hover',
              '&:hover': { color: 'text.primary', bgcolor: 'background.paper' },
            }}
          >
            {mode === 'dark' ? <Icon name="sun" size={15} /> : <Icon name="moon" size={15} />}
          </IconButton>
        </Tooltip>
        <AccountMenu size={28} />
      </Stack>
    </Box>
  );
}
