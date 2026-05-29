import { useEffect, useState } from 'react';
import { Box, Button, Card, Chip, FormControlLabel, IconButton, MenuItem, Stack, Switch, TextField, Typography } from '@mui/material';
import { useQueryClient } from '@tanstack/react-query';
import { confirmAction, toast } from '@ironflyer/ui-web/fx';
import { useGraphQLQuery, useRequest, operations } from '@ironflyer/data';
import { useOperateProjectId } from '../hooks/useOperateProjectId';
import { PaneHeader } from '../components/operate/PaneHeader';

interface EnvVar { key: string; valuePreview: string; secret: boolean; updatedAt: string }
interface Settings { projectID: string; displayName: string; visibility: string; region: string; supportEmail: string; envVars: EnvVar[]; updatedAt: string }

const SAMPLE: Settings = { projectID: '', displayName: 'TaskFlow', visibility: 'private', region: 'us-east', supportEmail: '', envVars: [
  { key: 'DATABASE_URL', valuePreview: '••••a91f', secret: true, updatedAt: '' },
  { key: 'NEXT_PUBLIC_APP_NAME', valuePreview: 'TaskFlow', secret: false, updatedAt: '' },
], updatedAt: '' };
const REGIONS = ['us-east', 'us-west', 'eu-central', 'ap-south'];
const VIS = ['private', 'unlisted', 'public'];

export function SettingsPane() {
  const request = useRequest();
  const qc = useQueryClient();
  const liveProjectId = useOperateProjectId();
  const [busy, setBusy] = useState(false);
  const [draft, setDraft] = useState<Settings | null>(null);
  const [envKey, setEnvKey] = useState('');
  const [envVal, setEnvVal] = useState('');
  const [envSecret, setEnvSecret] = useState(true);

  const { data: settings, isLive } = useGraphQLQuery<Settings, { appSettings: Settings }>({
    key: ['app-settings', liveProjectId ?? 'none'], operationName: 'AppSettings', query: operations.APP_SETTINGS,
    variables: { projectID: liveProjectId }, fallbackData: SAMPLE, enabled: !!liveProjectId, map: (r) => r.appSettings ?? SAMPLE,
  });
  useEffect(() => { setDraft(settings); }, [settings.updatedAt, settings.projectID]);
  const d = draft ?? settings;
  const set = (patch: Partial<Settings>) => setDraft({ ...d, ...patch });
  const refresh = () => void qc.invalidateQueries({ queryKey: ['app-settings', liveProjectId ?? 'none'] });

  const saveGeneral = async () => {
    if (!request || !liveProjectId) { toast('Connect the orchestrator to save settings.', 'error'); return; }
    setBusy(true);
    try {
      await request('UpdateAppSettings', operations.UPDATE_APP_SETTINGS, { projectID: liveProjectId, input: { displayName: d.displayName, visibility: d.visibility, region: d.region, supportEmail: d.supportEmail } });
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
    const ok = await confirmAction({ title: 'Archive this app?', text: 'The app stops serving traffic and is hidden from the dashboard. You can restore it later.', confirmText: 'Archive', danger: true });
    if (ok) toast('Archive is owner-gated — wire the archive mutation to enable.', 'info');
  };

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 880, mx: 'auto' }}>
        <PaneHeader title="Settings" isLive={isLive} />

        <Card sx={{ p: 2.5, mb: 2 }}>
          <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.66rem', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>General</Typography>
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr' }, gap: 1.5 }}>
            <TextField size="small" label="Display name" value={d.displayName} onChange={(e) => set({ displayName: e.target.value })} />
            <TextField size="small" label="Support email" value={d.supportEmail} onChange={(e) => set({ supportEmail: e.target.value })} />
            <TextField size="small" select label="Visibility" value={d.visibility} onChange={(e) => set({ visibility: e.target.value })}>
              {VIS.map((v) => <MenuItem key={v} value={v}>{v}</MenuItem>)}
            </TextField>
            <TextField size="small" select label="Region" value={d.region} onChange={(e) => set({ region: e.target.value })}>
              {REGIONS.map((v) => <MenuItem key={v} value={v}>{v}</MenuItem>)}
            </TextField>
          </Box>
          <Stack direction="row" justifyContent="flex-end" sx={{ mt: 1.5 }}>
            <Button variant="contained" disabled={busy || !liveProjectId} onClick={() => void saveGeneral()}>Save</Button>
          </Stack>
        </Card>

        <Card sx={{ p: 2.5, mb: 2 }}>
          <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.66rem', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>Environment variables</Typography>
          <Stack spacing={0.75} sx={{ mb: 1.5 }}>
            {d.envVars.length === 0 && <Typography sx={{ fontSize: '0.8rem', color: 'text.disabled' }}>No variables set.</Typography>}
            {d.envVars.map((v) => (
              <Stack key={v.key} direction="row" alignItems="center" spacing={1} sx={{ py: 0.5, borderBottom: 1, borderColor: 'divider' }}>
                <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.78rem', flex: 1 })} noWrap>{v.key}</Typography>
                {v.secret && <Chip size="small" label="secret" sx={(th) => ({ height: 18, fontSize: '0.56rem', bgcolor: `${th.brand.accent.primary}1f`, color: th.brand.accent.primary })} />}
                <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.76rem', color: 'text.secondary', width: 160, textAlign: 'right' })} noWrap>{v.valuePreview}</Typography>
                <IconButton size="small" disabled={busy} onClick={() => void delEnv(v.key)} sx={{ color: 'text.secondary' }}><svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6" /></svg></IconButton>
              </Stack>
            ))}
          </Stack>
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 1.4fr auto auto' }, gap: 1, alignItems: 'center' }}>
            <TextField size="small" label="KEY" value={envKey} onChange={(e) => setEnvKey(e.target.value.toUpperCase())} />
            <TextField size="small" label="value" value={envVal} onChange={(e) => setEnvVal(e.target.value)} type={envSecret ? 'password' : 'text'} />
            <FormControlLabel control={<Switch size="small" checked={envSecret} onChange={(e) => setEnvSecret(e.target.checked)} />} label="Secret" />
            <Button variant="outlined" color="inherit" disabled={!envKey.trim() || busy || !liveProjectId} onClick={() => void addEnv()}>Set</Button>
          </Box>
          <Typography sx={{ fontSize: '0.72rem', color: 'text.disabled', mt: 1 }}>Secret values are masked on the wire — the plaintext is never returned after it's set.</Typography>
        </Card>

        <Card sx={{ p: 2.5, borderColor: 'error.main', borderWidth: 1, borderStyle: 'solid' }}>
          <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.66rem', textTransform: 'uppercase', color: 'error.main', mb: 1 })}>Danger zone</Typography>
          <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ flexWrap: 'wrap', gap: 1 }}>
            <Typography sx={{ fontSize: '0.84rem', color: 'text.secondary' }}>Archive the app — stops serving traffic, reversible.</Typography>
            <Button variant="outlined" color="error" onClick={() => void dangerArchive()}>Archive app</Button>
          </Stack>
        </Card>
      </Box>
    </Box>
  );
}
