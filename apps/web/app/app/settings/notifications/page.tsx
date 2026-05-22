'use client';

// Settings → Notifications sub-route. Two tabs:
//   1. Email — toggles for each milestone topic + the receiving address.
//   2. Webhooks — list + add/delete + test the user's registered endpoints.
//
// This page lives at /app/settings/notifications so it does not have to
// modify the existing settings/page.tsx (owned by Agent G). The link to
// this page is meant to be added by that agent when convenient.

import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Alert, Box, Button, Checkbox, Chip, CircularProgress, Dialog, DialogActions,
  DialogContent, DialogTitle, FormControlLabel, IconButton, MenuItem,
  Select, Stack, Switch, Tab, Tabs, TextField, Tooltip, Typography,
} from '@mui/material';
import { Add, Delete, Email, PlayArrow, Refresh, Webhook } from '@mui/icons-material';

import { tokens } from '../../../../lib/theme';
import { RequireAuth, useAuth } from '../../../auth-context';
import { AppShell, PageTitle, Surface } from '../../workspace-shell';
import {
  CreateWebhookInput, NotificationRule, WEBHOOK_EVENT_CATALOG, Webhook as WebhookSub,
  notificationsApi, webhooksApi,
} from '../../../../lib/api/notifications';

type TabKey = 'email' | 'webhooks';

export default function NotificationsSettingsPage() {
  return (
    <RequireAuth>
      <NotificationsInner />
    </RequireAuth>
  );
}

function NotificationsInner() {
  const { user, logout } = useAuth();
  const [tab, setTab] = useState<TabKey>('email');

  return (
    <AppShell userEmail={user?.email ?? 'workspace'} onLogout={logout}>
      <PageTitle
        eyebrow="הגדרות"
        title="התראות"
        subtitle="קבע מתי Ironflyer ידחוף לאימייל או לוובהוק חיצוני."
      />
      <Tabs
        value={tab}
        onChange={(_, v) => setTab(v)}
        sx={{
          mb: 3,
          '& .Mui-selected': { color: tokens.color.text.inverse },
          '& .MuiTabs-indicator': { bgcolor: tokens.color.accent.lime, height: 3, borderRadius: '4px' },
        }}
      >
        <Tab value="email" icon={<Email fontSize="small" />} iconPosition="start" label="אימייל" />
        <Tab value="webhooks" icon={<Webhook fontSize="small" />} iconPosition="start" label="Webhooks" />
      </Tabs>

      {tab === 'email' ? <EmailTab defaultEmail={user?.email ?? ''} /> : <WebhooksTab />}
    </AppShell>
  );
}

// ---------------- Email tab ----------------

function EmailTab({ defaultEmail }: { defaultEmail: string }) {
  const [rule, setRule] = useState<NotificationRule | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [ok, setOk] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setErr(null);
    try {
      const data = await notificationsApi.getPreferences();
      if (!data.email) data.email = defaultEmail;
      setRule(data);
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, [defaultEmail]);

  useEffect(() => { void load(); }, [load]);

  const save = useCallback(async () => {
    if (!rule) return;
    setSaving(true);
    setErr(null);
    setOk(null);
    try {
      const next = await notificationsApi.setPreferences(rule);
      setRule(next);
      setOk('ההעדפות נשמרו.');
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  }, [rule]);

  if (loading || !rule) {
    return <Surface sx={{ p: 4, textAlign: 'center' }}><CircularProgress size={24} /></Surface>;
  }

  const update = (patch: Partial<NotificationRule>) => setRule({ ...rule, ...patch });

  return (
    <Stack spacing={3}>
      {err && <Alert severity="error">{err}</Alert>}
      {ok && <Alert severity="success">{ok}</Alert>}

      <Surface sx={{ p: 3 }}>
        <Typography variant="h6" sx={{ mb: 2, fontWeight: 800 }}>ערוצים</Typography>
        <Stack spacing={1.5}>
          <FormControlLabel
            control={<Switch checked={rule.channelEmail} onChange={(_, v) => update({ channelEmail: v })} />}
            label="שלח התראות באימייל"
          />
          <FormControlLabel
            control={<Switch checked={rule.channelWebhook} onChange={(_, v) => update({ channelWebhook: v })} />}
            label="הפעל גם Webhooks (מנוהל בלשונית השנייה)"
          />
        </Stack>
      </Surface>

      <Surface sx={{ p: 3 }}>
        <Typography variant="h6" sx={{ mb: 2, fontWeight: 800 }}>כתובת אימייל</Typography>
        <TextField
          fullWidth
          value={rule.email}
          onChange={(e) => update({ email: e.target.value })}
          placeholder={defaultEmail || 'you@example.com'}
          type="email"
        />
      </Surface>

      <Surface sx={{ p: 3 }}>
        <Typography variant="h6" sx={{ mb: 2, fontWeight: 800 }}>אירועים</Typography>
        <Stack spacing={1.5}>
          <FormControlLabel
            control={<Checkbox checked={rule.onRunComplete} onChange={(_, v) => update({ onRunComplete: v })} />}
            label="ריצה הסתיימה בהצלחה"
          />
          <FormControlLabel
            control={<Checkbox checked={rule.onGateFailed} onChange={(_, v) => update({ onGateFailed: v })} />}
            label="שער נכשל"
          />
          <FormControlLabel
            control={<Checkbox checked={rule.onDeployDone} onChange={(_, v) => update({ onDeployDone: v })} />}
            label="פריסה הסתיימה"
          />
          <FormControlLabel
            control={<Checkbox checked={rule.onBudgetWarning} onChange={(_, v) => update({ onBudgetWarning: v })} />}
            label="התרעת תקציב"
          />
        </Stack>
      </Surface>

      <Stack direction="row" spacing={2}>
        <Button
          variant="contained"
          disabled={saving}
          onClick={() => void save()}
          sx={{ bgcolor: tokens.color.accent.lime, color: tokens.color.text.inverse, fontWeight: 800, '&:hover': { bgcolor: '#b9d930' } }}
        >
          {saving ? 'שומר…' : 'שמור'}
        </Button>
        <Button variant="outlined" startIcon={<Refresh />} onClick={() => void load()}>רענן</Button>
      </Stack>
    </Stack>
  );
}

// ---------------- Webhooks tab ----------------

function WebhooksTab() {
  const [items, setItems] = useState<WebhookSub[]>([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);
  const [ok, setOk] = useState<string | null>(null);
  const [addOpen, setAddOpen] = useState(false);
  const [confirmId, setConfirmId] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setErr(null);
    try {
      setItems(await webhooksApi.list());
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  const onTest = useCallback(async (id: string) => {
    setErr(null);
    setOk(null);
    try {
      await webhooksApi.test(id);
      setOk('אירוע בדיקה נשלח. בדוק את ה-endpoint.');
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    }
  }, []);

  const onDelete = useCallback(async (id: string) => {
    try {
      await webhooksApi.remove(id);
      setConfirmId(null);
      await load();
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    }
  }, [load]);

  return (
    <Stack spacing={3}>
      {err && <Alert severity="error">{err}</Alert>}
      {ok && <Alert severity="success">{ok}</Alert>}

      <Stack direction="row" justifyContent="space-between" alignItems="center">
        <Typography variant="body2" color="textSecondary">
          POST חתום ב-HMAC-SHA256 אל ה-URL שלך לכל אירוע מהפרויקטים שאתה הבעלים שלהם.
        </Typography>
        <Button
          variant="contained"
          startIcon={<Add />}
          onClick={() => setAddOpen(true)}
          sx={{ bgcolor: tokens.color.accent.lime, color: tokens.color.text.inverse, fontWeight: 800, '&:hover': { bgcolor: '#b9d930' } }}
        >
          הוסף Webhook
        </Button>
      </Stack>

      {loading ? (
        <Surface sx={{ p: 4, textAlign: 'center' }}><CircularProgress size={24} /></Surface>
      ) : items.length === 0 ? (
        <Surface sx={{ p: 4, textAlign: 'center' }}>
          <Typography>עדיין אין Webhooks. לחץ "הוסף Webhook" כדי להתחיל.</Typography>
        </Surface>
      ) : (
        <Stack spacing={2}>
          {items.map((w) => (
            <WebhookRow
              key={w.id}
              w={w}
              onTest={() => void onTest(w.id)}
              onDelete={() => setConfirmId(w.id)}
            />
          ))}
        </Stack>
      )}

      <AddWebhookDialog
        open={addOpen}
        onClose={() => setAddOpen(false)}
        onCreated={async () => { setAddOpen(false); await load(); }}
      />

      <Dialog open={Boolean(confirmId)} onClose={() => setConfirmId(null)}>
        <DialogTitle>למחוק את ה-Webhook?</DialogTitle>
        <DialogContent>
          <Typography>הפעולה בלתי הפיכה. ה-endpoint יפסיק לקבל אירועים מיד.</Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirmId(null)}>ביטול</Button>
          <Button
            variant="contained"
            color="error"
            startIcon={<Delete />}
            onClick={() => confirmId && void onDelete(confirmId)}
          >
            מחק
          </Button>
        </DialogActions>
      </Dialog>
    </Stack>
  );
}

function WebhookRow({ w, onTest, onDelete }: { w: WebhookSub; onTest: () => void; onDelete: () => void }) {
  const events = w.events?.length ? w.events : ['*'];
  return (
    <Surface sx={{ p: 2.5 }}>
      <Stack direction={{ xs: 'column', md: 'row' }} justifyContent="space-between" alignItems={{ xs: 'flex-start', md: 'center' }} spacing={1.5}>
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Typography sx={{ fontWeight: 800, wordBreak: 'break-all' }}>{w.url}</Typography>
          <Stack direction="row" spacing={1} flexWrap="wrap" sx={{ mt: 1 }}>
            {events.map((e) => (
              <Chip key={e} size="small" label={e} sx={{ borderColor: 'rgba(17,17,17,0.14)' }} variant="outlined" />
            ))}
            {w.projectId && <Chip size="small" label={`פרויקט: ${w.projectId}`} variant="outlined" />}
            {w.disabled && <Chip size="small" color="error" label="הושבת" />}
          </Stack>
          <Typography variant="caption" color="textSecondary" sx={{ display: 'block', mt: 1 }}>
            {w.lastSentAt ? `נשלח לאחרונה: ${new Date(w.lastSentAt).toLocaleString('he-IL')}` : 'טרם נשלח'} · כשלונות: {w.failureCount}
          </Typography>
        </Box>
        <Stack direction="row" spacing={1}>
          <Tooltip title="שלח אירוע בדיקה">
            <IconButton onClick={onTest}><PlayArrow /></IconButton>
          </Tooltip>
          <Tooltip title="מחק">
            <IconButton onClick={onDelete}><Delete /></IconButton>
          </Tooltip>
        </Stack>
      </Stack>
    </Surface>
  );
}

function AddWebhookDialog({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: () => Promise<void> }) {
  const [url, setUrl] = useState('');
  const [secret, setSecret] = useState('');
  const [projectId, setProjectId] = useState('');
  const [events, setEvents] = useState<string[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const reset = () => {
    setUrl(''); setSecret(''); setProjectId(''); setEvents([]); setErr(null);
  };

  const submit = async () => {
    setErr(null);
    const payload: CreateWebhookInput = { url: url.trim() };
    if (secret.trim()) payload.secret = secret.trim();
    if (projectId.trim()) payload.projectId = projectId.trim();
    if (events.length) payload.events = events;
    setSubmitting(true);
    try {
      await webhooksApi.create(payload);
      reset();
      await onCreated();
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setSubmitting(false);
    }
  };

  const eventOptions = useMemo(() => WEBHOOK_EVENT_CATALOG, []);

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>הוסף Webhook</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ mt: 1 }}>
          {err && <Alert severity="error">{err}</Alert>}
          <TextField
            label="URL"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder="https://example.com/webhook"
            fullWidth
            required
          />
          <TextField
            label="Secret (חתימה HMAC, אופציונלי)"
            value={secret}
            onChange={(e) => setSecret(e.target.value)}
            fullWidth
          />
          <TextField
            label="מזהה פרויקט (ריק = כל הפרויקטים שלי)"
            value={projectId}
            onChange={(e) => setProjectId(e.target.value)}
            fullWidth
          />
          <Box>
            <Typography variant="body2" sx={{ mb: 0.5 }}>אירועים (ריק = הכל)</Typography>
            <Select
              multiple
              fullWidth
              value={events}
              onChange={(e) => {
                const v = e.target.value;
                setEvents(typeof v === 'string' ? v.split(',') : v);
              }}
              renderValue={(selected) => (selected as string[]).join(', ')}
            >
              {eventOptions.map((o) => (
                <MenuItem key={o.name} value={o.name}>
                  <Checkbox checked={events.includes(o.name)} />
                  {o.label} ({o.name})
                </MenuItem>
              ))}
            </Select>
          </Box>
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={() => { reset(); onClose(); }}>ביטול</Button>
        <Button
          variant="contained"
          disabled={submitting || !url.trim()}
          onClick={() => void submit()}
          sx={{ bgcolor: tokens.color.accent.lime, color: tokens.color.text.inverse, fontWeight: 800, '&:hover': { bgcolor: '#b9d930' } }}
        >
          {submitting ? 'שומר…' : 'צור'}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
