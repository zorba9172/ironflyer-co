import { useEffect, useState } from 'react';
import { Box, Button, Chip, FormControlLabel, IconButton, MenuItem, Stack, Switch, TextField, Tooltip, Typography } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { useQueryClient } from '@tanstack/react-query';
import { confirmAction, toast } from '@ironflyer/ui-web/fx';
import { useGraphQLQuery, useRequest, operations } from '@ironflyer/data';
import { useOperateProjectId } from '../hooks/useOperateProjectId';
import { PaneHeader } from '../components/operate/PaneHeader';
import { Icon } from '../icons';
import { GlassPanel, SectionHeader } from '../components/studio';
import { text } from '@ironflyer/design-tokens/brand';

interface EnvVar { key: string; valuePreview: string; secret: boolean; updatedAt: string }
interface Settings { projectID: string; displayName: string; visibility: string; region: string; supportEmail: string; envVars: EnvVar[]; updatedAt: string }

const SAMPLE: Settings = {
  projectID: '', displayName: 'TaskFlow', visibility: 'private', region: 'us-east', supportEmail: '',
  envVars: [
    { key: 'DATABASE_URL', valuePreview: '••••a91f', secret: true, updatedAt: '' },
    { key: 'NEXT_PUBLIC_APP_NAME', valuePreview: 'TaskFlow', secret: false, updatedAt: '' },
  ],
  updatedAt: '',
};
const REGIONS = ['us-east', 'us-west', 'eu-central', 'ap-south'];
const VIS = ['private', 'unlisted', 'public', 'archived'];

const VIS_DESC: Record<string, string> = {
  private: 'Only you can access this app.',
  unlisted: 'Anyone with the link can view.',
  public: 'Listed in the public directory.',
  archived: 'Stopped serving traffic.',
};

function VisibilityBadge({ vis }: { vis: string }) {
  const colorMap: Record<string, 'default' | 'success' | 'warning' | 'error'> = {
    private: 'default', unlisted: 'warning', public: 'success', archived: 'error',
  };
  return (
    <Chip
      size="small"
      label={vis}
      color={colorMap[vis] ?? 'default'}
      sx={{ height: 20, fontSize: text.s62, textTransform: 'uppercase' }}
    />
  );
}

export function SettingsPane() {
  const request = useRequest();
  const qc = useQueryClient();
  const liveProjectId = useOperateProjectId();
  const [busy, setBusy] = useState(false);
  const [draft, setDraft] = useState<Settings | null>(null);
  const [envKey, setEnvKey] = useState('');
  const [envVal, setEnvVal] = useState('');
  const [envSecret, setEnvSecret] = useState(true);

  const theme = useTheme();

  const { data: settings, isLive } = useGraphQLQuery<Settings, { appSettings: Settings }>({
    key: ['app-settings', liveProjectId ?? 'none'],
    operationName: 'AppSettings', query: operations.APP_SETTINGS,
    variables: { projectID: liveProjectId }, fallbackData: SAMPLE, enabled: !!liveProjectId,
    map: (r) => r.appSettings ?? SAMPLE,
  });
  useEffect(() => { setDraft(settings); }, [settings.updatedAt, settings.projectID]);
  const d = draft ?? settings;
  const set = (patch: Partial<Settings>) => setDraft({ ...d, ...patch });
  const refresh = () => void qc.invalidateQueries({ queryKey: ['app-settings', liveProjectId ?? 'none'] });

  const saveGeneral = async () => {
    if (!request || !liveProjectId) { toast('Connect the orchestrator to save settings.', 'error'); return; }
    setBusy(true);
    try {
      await request('UpdateAppSettings', operations.UPDATE_APP_SETTINGS, {
        projectID: liveProjectId,
        input: { displayName: d.displayName, visibility: d.visibility, region: d.region, supportEmail: d.supportEmail },
      });
      refresh(); toast('Settings saved.', 'success');
    } catch (e) { toast(e instanceof Error ? e.message : 'Save failed.', 'error'); }
    finally { setBusy(false); }
  };

  const addEnv = async () => {
    if (!envKey.trim() || !request || !liveProjectId) return;
    setBusy(true);
    try {
      await request('SetAppEnvVar', operations.SET_APP_ENV_VAR, { projectID: liveProjectId, key: envKey.trim(), value: envVal, secret: envSecret });
      setEnvKey(''); setEnvVal(''); refresh(); toast('Variable saved.', 'success');
    } catch (e) { toast(e instanceof Error ? e.message : 'Failed.', 'error'); }
    finally { setBusy(false); }
  };

  const delEnv = async (key: string) => {
    if (!request || !liveProjectId) return;
    setBusy(true);
    try { await request('DeleteAppEnvVar', operations.DELETE_APP_ENV_VAR, { projectID: liveProjectId, key }); refresh(); toast('Variable removed.', 'success'); }
    catch (e) { toast(e instanceof Error ? e.message : 'Failed.', 'error'); }
    finally { setBusy(false); }
  };

  const dangerArchive = async () => {
    if (!request || !liveProjectId) { toast('Connect the orchestrator to archive the app.', 'error'); return; }
    const ok = await confirmAction({
      title: 'Archive this app?',
      text: 'The app stops serving traffic and is hidden from the dashboard. You can restore it later by setting visibility back.',
      confirmText: 'Archive',
      danger: true,
    });
    if (!ok) return;
    setBusy(true);
    try {
      await request('UpdateAppSettings', operations.UPDATE_APP_SETTINGS, { projectID: liveProjectId, input: { visibility: 'archived' } });
      refresh(); toast('App archived — it no longer serves traffic.', 'success');
    } catch (e) { toast(e instanceof Error ? e.message : 'Archive failed.', 'error'); }
    finally { setBusy(false); }
  };

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 900, mx: 'auto' }}>
        <PaneHeader title="Settings" isLive={isLive} />

        {/* General */}
        <GlassPanel pad={2.5} sx={{ mb: 2.5 }}>
          <SectionHeader
            eyebrow="Project"
            title="General"
            subtitle="Display name, visibility, region, and support contact."
            actions={
              <Stack direction="row" alignItems="center" spacing={1.5}>
                <VisibilityBadge vis={d.visibility} />
                <Button variant="contained" disabled={busy || !liveProjectId} onClick={() => void saveGeneral()}>
                  Save
                </Button>
              </Stack>
            }
          />
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr' }, gap: 1.75 }}>
            <TextField size="small" label="Display name" value={d.displayName} onChange={(e) => set({ displayName: e.target.value })} />
            <TextField size="small" label="Support email" value={d.supportEmail} onChange={(e) => set({ supportEmail: e.target.value })} />
            <TextField size="small" select label="Visibility" value={d.visibility} onChange={(e) => set({ visibility: e.target.value })}
              helperText={VIS_DESC[d.visibility] ?? ''}>
              {VIS.map((v) => <MenuItem key={v} value={v}>{v}</MenuItem>)}
            </TextField>
            <TextField size="small" select label="Region" value={d.region} onChange={(e) => set({ region: e.target.value })}
              helperText="Primary region for your app's runtime">
              {REGIONS.map((v) => <MenuItem key={v} value={v}>{v}</MenuItem>)}
            </TextField>
          </Box>
        </GlassPanel>

        {/* Environment variables */}
        <GlassPanel pad={2.5} sx={{ mb: 2.5 }}>
          <SectionHeader eyebrow="Runtime" title="Environment variables" subtitle="Injected at build and runtime. Secret values are masked on the wire after being set." />

          {/* Existing vars list */}
          <Stack spacing={0} sx={{ mb: 2, borderRadius: (th) => `${th.studio.radius.sm}px`, overflow: 'hidden', border: 1, borderColor: 'divider' }}>
            {d.envVars.length === 0 ? (
              <Typography sx={{ fontSize: text.s80, color: 'text.disabled', p: 2 }}>No variables set.</Typography>
            ) : d.envVars.map((v, idx) => (
              <Stack
                key={v.key}
                direction="row"
                alignItems="center"
                spacing={1.5}
                sx={{
                  px: 2,
                  py: 1.25,
                  borderTop: idx > 0 ? '1px solid' : 'none',
                  borderColor: 'divider',
                  '&:hover': { bgcolor: 'action.hover' },
                }}
              >
                <Box sx={{ display: 'inline-flex', color: v.secret ? 'primary.main' : 'text.disabled', opacity: 0.7 }}>
                  <Icon name={v.secret ? 'shield' : 'eye'} size={14} />
                </Box>
                <Typography
                  sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s80, flex: 1, fontWeight: 700 })}
                  noWrap
                >
                  {v.key}
                </Typography>
                {v.secret && (
                  <Chip
                    size="small"
                    label="secret"
                    sx={(th) => ({ height: 18, fontSize: text.s56, bgcolor: `${th.studio.neon.violet}1f`, color: th.studio.neon.violet })}
                  />
                )}
                <Typography
                  sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s76, color: 'text.secondary', width: 160, textAlign: 'right' })}
                  noWrap
                >
                  {v.valuePreview}
                </Typography>
                <Tooltip title="Remove variable">
                  <IconButton
                    size="small"
                    aria-label={`Remove ${v.key}`}
                    disabled={busy}
                    onClick={() => void delEnv(v.key)}
                    sx={{ color: 'text.secondary', '&:hover': { color: 'error.main' } }}
                  >
                    <Icon name="trash" size={15} />
                  </IconButton>
                </Tooltip>
              </Stack>
            ))}
          </Stack>

          {/* Add new var form */}
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 1.4fr auto auto' }, gap: 1.25, alignItems: 'flex-end' }}>
            <TextField
              size="small"
              label="KEY"
              value={envKey}
              onChange={(e) => setEnvKey(e.target.value.toUpperCase())}
              placeholder="MY_VAR"
            />
            <TextField
              size="small"
              label="value"
              value={envVal}
              onChange={(e) => setEnvVal(e.target.value)}
              type={envSecret ? 'password' : 'text'}
            />
            <FormControlLabel
              control={<Switch size="small" checked={envSecret} onChange={(e) => setEnvSecret(e.target.checked)} />}
              label="Secret"
            />
            <Button
              variant="outlined"
              color="inherit"
              disabled={!envKey.trim() || busy || !liveProjectId}
              onClick={() => void addEnv()}
            >
              Set
            </Button>
          </Box>
          <Typography sx={{ fontSize: text.s72, color: 'text.disabled', mt: 1.25 }}>
            Secret values are masked after saving — the plaintext is never returned from the API.
          </Typography>
        </GlassPanel>

        {/* Danger zone */}
        <GlassPanel
          pad={2.5}
          accent={theme.palette.error.main}
          sx={(th) => ({ borderColor: `${th.palette.error.main}44` })}
        >
          <SectionHeader eyebrow="Danger zone" title="Destructive actions" />
          <Stack direction={{ xs: 'column', sm: 'row' }} alignItems={{ xs: 'stretch', sm: 'center' }} justifyContent="space-between" spacing={1.5}>
            <Box>
              <Typography sx={{ fontSize: text.s86, fontWeight: 700 }}>Archive this app</Typography>
              <Typography sx={{ fontSize: text.s80, color: 'text.secondary', mt: 0.25 }}>
                Stops serving traffic immediately. Reversible — set visibility back to restore.
              </Typography>
            </Box>
            <Button
              variant="outlined"
              color="error"
              disabled={busy || !liveProjectId}
              onClick={() => void dangerArchive()}
              sx={{ whiteSpace: 'nowrap', flexShrink: 0 }}
            >
              Archive app
            </Button>
          </Stack>
        </GlassPanel>
      </Box>
    </Box>
  );
}
