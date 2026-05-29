import { useState } from 'react';
import { Box, Button, CircularProgress, IconButton, Menu, MenuItem, Stack, Tooltip, Typography } from '@mui/material';
import { useNavigate } from 'react-router-dom';
import { toast } from '@ironflyer/ui-web/fx';
import { VscChevronDown } from 'react-icons/vsc';
import { LogoMark } from './LogoMark';
import { AccountMenu } from './AccountMenu';
import { CostHUD } from './CostHUD';
import { PrivateModeChip } from './PrivateModeChip';
import { useStudio } from '../store';
import { useSaveProject } from '../hooks/useSaveProject';
import { text } from '@ironflyer/design-tokens/brand';

export type EditorTab =
  | 'preview' | 'map' | 'security' | 'code'
  | 'dashboard' | 'documents' | 'logs' | 'quality' | 'team'
  | 'data' | 'users' | 'analytics' | 'domains' | 'automations' | 'api' | 'marketing' | 'settings';

type EditorTabGroupId = 'build' | 'review' | 'operate' | 'business';
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
      { value: 'preview', label: 'Preview' },
      { value: 'code', label: 'Code' },
      { value: 'team', label: 'Execution team' },
      { value: 'documents', label: 'Documents' },
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
      { value: 'domains', label: 'Domains' },
      { value: 'automations', label: 'Automations' },
      { value: 'api', label: 'API' },
      { value: 'settings', label: 'Settings' },
    ],
  },
  {
    id: 'business',
    label: 'Business',
    items: [
      { value: 'analytics', label: 'Analytics' },
      { value: 'marketing', label: 'Marketing' },
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
  const readinessColor = deployReadiness ? `${deployReadiness.tone}.main` : 'text.disabled';

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

      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.25, bgcolor: 'action.hover', borderRadius: 99, p: 0.5, minWidth: 0 }}>
        {TAB_GROUPS.map((group) => {
          const selected = activeGroup.id === group.id;
          const activeLabel = group.items.find((d) => d.value === tab)?.label;
          return (
            <Button
              key={group.id}
              size="small"
              onClick={(e) => setGroupAnchor({ id: group.id, el: e.currentTarget })}
              endIcon={<VscChevronDown size={12} />}
              sx={{
                borderRadius: 99,
                px: 1.4,
                py: 0.5,
                minWidth: 0,
                textTransform: 'none',
                color: selected ? 'text.primary' : 'text.secondary',
                bgcolor: selected ? 'background.paper' : 'transparent',
                boxShadow: selected ? 1 : 0,
              }}
            >
              <Box component="span" sx={{ minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis' }}>{group.label}</Box>
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

      <Stack direction="row" alignItems="center" justifyContent="flex-end" spacing={1}>
        <PrivateModeChip />
        <CostHUD />
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
