'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import {
  AccountBalanceWallet, AccountCircle, Delete, ErrorOutline, GitHub, Hub,
  Logout, Refresh, Save, Shield, Warning,
} from '@mui/icons-material';
import {
  Avatar, Box, Button, CircularProgress, Divider, IconButton, LinearProgress,
  Stack, Tab, Tabs, TextField, Tooltip, Typography,
} from '@mui/material';
import { api, LedgerEntry, Plan, Project, UserBudget, VaultSnapshot } from '../../../lib/api';
import { tokens } from '../../../lib/theme';
import { RequireAuth, useAuth } from '../../auth-context';
import { BillingStatusBanner } from '../../billing-status-banner';
import { UpgradeButton } from '../../upgrade-button';
import { githubApi, GitHubStatus } from '../../../lib/github';
import { accountApi, billingApi } from '../../../lib/api/billing';
import { AppShell, PageTitle, Surface } from '../workspace-shell';
import {
  ErrorBox, SkeletonBlock, StatusPill, UsageSpark, bucketByDay,
} from '../../../components/dashboard';

type TabKey = 'account' | 'billing' | 'integrations' | 'vault' | 'danger';

const tabs: { value: TabKey; label: string; icon: React.ReactElement }[] = [
  { value: 'account',      label: 'חשבון',        icon: <AccountCircle fontSize="small" /> },
  { value: 'billing',      label: 'חיוב',          icon: <AccountBalanceWallet fontSize="small" /> },
  { value: 'integrations', label: 'אינטגרציות',   icon: <Hub fontSize="small" /> },
  { value: 'vault',        label: 'כספת',          icon: <Shield fontSize="small" /> },
  { value: 'danger',       label: 'אזור מסוכן',    icon: <ErrorOutline fontSize="small" /> },
];

export default function SettingsPage() {
  return (
    <RequireAuth>
      <SettingsInner />
    </RequireAuth>
  );
}

function SettingsInner() {
  const { user, logout } = useAuth();
  const router = useRouter();
  const searchParams = useSearchParams();
  const tabParam = (searchParams.get('tab') ?? 'account') as TabKey;
  const activeTab: TabKey = tabs.some((t) => t.value === tabParam) ? tabParam : 'account';

  const [projects, setProjects] = useState<Project[]>([]);
  const [budget, setBudget] = useState<UserBudget | null>(null);
  const [plans, setPlans] = useState<Plan[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [p, b, pl] = await Promise.all([
        api.listProjects().catch(() => [] as Project[]),
        api.myBudget().catch(() => null),
        api.listPlans().catch(() => [] as Plan[]),
      ]);
      setProjects(p);
      setBudget(b);
      setPlans(pl);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void refresh(); }, [refresh]);

  function changeTab(_: unknown, next: TabKey) {
    const params = new URLSearchParams(searchParams.toString());
    params.set('tab', next);
    router.replace(`/app/settings?${params.toString()}`);
  }

  return (
    <AppShell
      userEmail={user?.email ?? 'workspace'}
      recents={projects.slice(0, 5)}
      onLogout={logout}
    >
      <PageTitle
        eyebrow="הגדרות"
        title="ניהול הסביבה"
        subtitle="חשבון, תשלומים, אינטגרציות, כספת תקציבית ופעולות מסוכנות."
      />
      <Box sx={{ mb: 1.5 }}>
        <BillingStatusBanner compact />
      </Box>

      {error && (
        <Box sx={{ mb: 1.6 }}>
          <ErrorBox title="שגיאת טעינה" description={error} onRetry={() => void refresh()} />
        </Box>
      )}

      <Surface sx={{ p: 0, mb: 2 }}>
        <Tabs
          value={activeTab}
          onChange={changeTab}
          variant="scrollable"
          scrollButtons="auto"
          sx={{
            px: 1,
            minHeight: 52,
            '& .MuiTab-root': {
              minHeight: 52,
              textTransform: 'none',
              fontWeight: 800,
              color: '#4a453e',
            },
            '& .Mui-selected': { color: tokens.color.text.inverse },
            '& .MuiTabs-indicator': { bgcolor: tokens.color.accent.lime, height: 3, borderRadius: '4px' },
          }}
        >
          {tabs.map((tab) => (
            <Tab key={tab.value} value={tab.value} label={tab.label} icon={tab.icon} iconPosition="start" />
          ))}
        </Tabs>
      </Surface>

      {activeTab === 'account' && (
        <AccountTab user={user} onLogout={logout} loading={loading} />
      )}
      {activeTab === 'billing' && (
        <BillingTab budget={budget} plans={plans} loading={loading} onRefresh={() => void refresh()} />
      )}
      {activeTab === 'integrations' && (
        <IntegrationsTab />
      )}
      {activeTab === 'vault' && (
        <VaultTab />
      )}
      {activeTab === 'danger' && (
        <DangerTab projectCount={projects.length} onLogout={logout} />
      )}
    </AppShell>
  );
}

/* ------------------------------ Account tab ------------------------------ */

function AccountTab({ user, onLogout, loading }: { user: ReturnType<typeof useAuth>['user']; onLogout: () => void; loading: boolean }) {
  const [displayName, setDisplayName] = useState('');
  const [avatarUrl, setAvatarUrl] = useState('');
  const [savedAt, setSavedAt] = useState<number | null>(null);

  useEffect(() => {
    if (!user) return;
    setDisplayName(localStorage.getItem('ironflyer.displayName') ?? user.name ?? user.email.split('@')[0] ?? '');
    setAvatarUrl(localStorage.getItem('ironflyer.avatarUrl') ?? '');
  }, [user]);

  function save() {
    localStorage.setItem('ironflyer.displayName', displayName.trim());
    localStorage.setItem('ironflyer.avatarUrl', avatarUrl.trim());
    setSavedAt(Date.now());
  }

  return (
    <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1.1fr 0.9fr' }, gap: 1.6 }}>
      <Surface sx={{ p: 2.4 }}>
        <Typography variant="h6" sx={{ fontWeight: 900 }}>פרופיל</Typography>
        <Typography variant="body2" sx={{ color: '#686158', mt: 0.4 }}>
          מידע אישי שמופיע בלוח הבקרה. השם וה־avatar נשמרים מקומית כברירת מחדל; אינטגרציה לשרת תגיע בהמשך.
        </Typography>
        <Stack spacing={1.6} sx={{ mt: 2.4 }}>
          <Stack direction="row" spacing={1.4} alignItems="center">
            <Avatar
              src={avatarUrl || undefined}
              sx={{ width: 64, height: 64, bgcolor: tokens.color.accent.lime, color: tokens.color.text.inverse, fontWeight: 900 }}
            >
              {(displayName || user?.email || '?')[0]?.toUpperCase()}
            </Avatar>
            <Box sx={{ minWidth: 0 }}>
              <Typography variant="subtitle1" sx={{ fontWeight: 900 }} noWrap>{displayName || user?.email}</Typography>
              <Typography variant="caption" sx={{ color: '#86807a' }}>{user?.email}</Typography>
            </Box>
          </Stack>
          {loading ? (
            <Stack spacing={1}><SkeletonBlock height={48} /><SkeletonBlock height={48} /></Stack>
          ) : (
            <>
              <TextField
                label="שם תצוגה"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                InputLabelProps={{ shrink: true }}
              />
              <TextField
                label="דוא״ל"
                value={user?.email ?? ''}
                InputLabelProps={{ shrink: true }}
                disabled
                helperText="הדוא״ל מוגדר בעת יצירת החשבון ולא ניתן לעריכה כאן."
              />
              <TextField
                label="קישור ל־avatar"
                placeholder="https://..."
                value={avatarUrl}
                onChange={(e) => setAvatarUrl(e.target.value)}
                InputLabelProps={{ shrink: true }}
              />
              <Stack direction="row" spacing={1} sx={{ mt: 0.6 }}>
                <Button variant="contained" startIcon={<Save />} onClick={save}>שמור</Button>
                {savedAt && (
                  <Typography variant="caption" sx={{ alignSelf: 'center', color: '#6f7e00', fontWeight: 800 }}>
                    נשמר ב־{new Date(savedAt).toLocaleTimeString('he-IL')}
                  </Typography>
                )}
              </Stack>
            </>
          )}
        </Stack>
      </Surface>

      <Surface sx={{ p: 2.4 }}>
        <Typography variant="h6" sx={{ fontWeight: 900 }}>סשן ואבטחה</Typography>
        <Typography variant="body2" sx={{ color: '#686158', mt: 0.4 }}>
          ניתוק יוצר ממך מהדפדפן הנוכחי. כדי להתחבר ממכשיר אחר השתמש באותו דוא״ל וסיסמה.
        </Typography>
        <Stack spacing={1} sx={{ mt: 2.4 }}>
          <Stack direction="row" justifyContent="space-between" alignItems="center" sx={infoRowSx}>
            <Box>
              <Typography variant="subtitle2" sx={{ fontWeight: 900 }}>מצב התחברות</Typography>
              <Typography variant="caption" sx={{ color: '#86807a' }}>JWT פעיל בדפדפן הזה.</Typography>
            </Box>
            <StatusPill kind="passed" label="פעיל" />
          </Stack>
          <Stack direction="row" justifyContent="space-between" alignItems="center" sx={infoRowSx}>
            <Box>
              <Typography variant="subtitle2" sx={{ fontWeight: 900 }}>אימות דו־שלבי</Typography>
              <Typography variant="caption" sx={{ color: '#86807a' }}>זמין לאחר חיבור חשבון Single-Sign-On.</Typography>
            </Box>
            <StatusPill kind="idle" label="בקרוב" />
          </Stack>
          <Button onClick={onLogout} variant="outlined" startIcon={<Logout />} sx={{ mt: 1, alignSelf: 'flex-start' }}>
            התנתק מסביבה זו
          </Button>
        </Stack>
      </Surface>
    </Box>
  );
}

/* ------------------------------ Billing tab ------------------------------ */

function BillingTab({
  budget, plans, loading, onRefresh,
}: {
  budget: UserBudget | null;
  plans: Plan[];
  loading: boolean;
  onRefresh: () => void;
}) {
  const tier = budget?.tier ?? 'free';
  const currentPlan = plans.find((p) => p.tier === tier);
  const cap = Number(currentPlan?.costCapUSD ?? (tier === 'team' ? 32 : tier === 'pro' ? 8 : tier === 'enterprise' ? 180 : 0.5));
  const spent = Number(budget?.spent ?? 0);
  const percent = cap > 0 ? Math.min(100, Math.max(0, (spent / cap) * 100)) : 0;
  const entries: LedgerEntry[] = Array.isArray(budget?.entries) ? budget!.entries : [];
  const points = useMemo(() => bucketByDay(entries.map((e) => ({ createdAt: e.createdAt, costUSD: e.costUSD })), 30), [entries]);
  const stripeEnabled = (process.env.NEXT_PUBLIC_STRIPE_ENABLED ?? '1') !== '0';

  return (
    <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1.1fr 0.9fr' }, gap: 1.6 }}>
      <Surface sx={{ p: 2.4 }}>
        <Stack direction="row" justifyContent="space-between" alignItems="flex-start" spacing={1}>
          <Box>
            <Typography variant="overline" sx={{ color: '#9fb500' }}>חבילה נוכחית</Typography>
            <Typography variant="h5" sx={{ fontWeight: 900, mt: 0.4 }}>
              {currentPlan?.name ?? (tier === 'free' ? 'Free workspace' : tier)}
            </Typography>
            <Typography variant="body2" sx={{ color: '#686158', mt: 0.4 }}>
              {currentPlan?.monthlyPrice
                ? `${currentPlan.monthlyPrice} לחודש — חידוש אוטומטי`
                : 'חבילה חינמית עם תקרת עלות קשיחה.'}
            </Typography>
          </Box>
          <Tooltip title="רענן">
            <IconButton size="small" onClick={onRefresh}><Refresh fontSize="small" /></IconButton>
          </Tooltip>
        </Stack>

        <Box sx={{ mt: 2.4 }}>
          <Stack direction="row" justifyContent="space-between">
            <Typography variant="caption" sx={{ color: '#686158' }}>הוצאה בחודש זה</Typography>
            <Typography variant="caption" sx={{ fontFamily: tokens.font.mono }}>${spent.toFixed(2)} / ${cap.toFixed(2)}</Typography>
          </Stack>
          <LinearProgress
            variant="determinate"
            value={percent}
            sx={{
              mt: 0.7,
              height: 8,
              borderRadius: '999px',
              bgcolor: 'rgba(17,17,17,0.1)',
              '& .MuiLinearProgress-bar': { bgcolor: percent > 82 ? tokens.color.accent.coral : tokens.color.accent.lime },
            }}
          />
        </Box>

        <Box sx={{ mt: 2.4 }}>
          <Typography variant="subtitle2" sx={{ fontWeight: 900 }}>שימוש ב־30 הימים האחרונים</Typography>
          {loading ? (
            <Box sx={{ mt: 1 }}><SkeletonBlock height={96} radius={8} /></Box>
          ) : (
            <Box sx={{ mt: 1 }}>
              <UsageSpark
                points={points}
                height={110}
                emptyHint="עוד לא בוצעו חיובים מאומתים החודש"
              />
            </Box>
          )}
        </Box>

        <Divider sx={{ my: 2.2 }} />
        {stripeEnabled ? (
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1}>
            <UpgradeButton tier="pro" label="שדרג ל־Pro" size="small" />
            <UpgradeButton tier="team" label="פתח חבילת Team" size="small" variant="outlined" />
            <UpgradeButton tier="enterprise" label="צור קשר ל־Enterprise" size="small" variant="outlined" />
          </Stack>
        ) : (
          <Box sx={{ p: 1.6, borderRadius: '8px', bgcolor: 'rgba(255,196,0,0.16)', border: '1px solid rgba(122,91,0,0.26)' }}>
            <Stack direction="row" spacing={1} alignItems="center">
              <Warning fontSize="small" sx={{ color: '#7a5b00' }} />
              <Typography variant="body2" sx={{ color: '#5b4500', fontWeight: 800 }}>
                Stripe מנוטרל בסביבה זו — שדרוג ידני יבוצע על ידי הצוות.
              </Typography>
            </Stack>
          </Box>
        )}
      </Surface>

      <Surface sx={{ p: 2.4 }}>
        <Typography variant="h6" sx={{ fontWeight: 900 }}>חבילות אפשריות</Typography>
        <Stack spacing={1} sx={{ mt: 1.8 }}>
          {(plans.length ? plans : fallbackPlans).map((plan) => (
            <PlanRow key={plan.tier} plan={plan} active={plan.tier === tier} />
          ))}
        </Stack>
        <Divider sx={{ my: 2 }} />
        <Typography variant="caption" sx={{ color: '#686158' }}>
          חיובים מתבצעים על בסיס שימוש בפועל בתוך תקרת החבילה. כל הרצה משויכת לחשבון שלך וניתנת לבדיקה ב<Link href="/app/settings?tab=vault" style={{ color: '#6f7e00', fontWeight: 800 }}>כספת</Link>.
        </Typography>
      </Surface>
    </Box>
  );
}

const fallbackPlans: Plan[] = [
  { tier: 'free', name: 'Free', monthlyPrice: '$0',  costCapUSD: '0.5',  hardStop: true },
  { tier: 'pro',  name: 'Pro',  monthlyPrice: '$20', costCapUSD: '8',    hardStop: true },
  { tier: 'team', name: 'Team', monthlyPrice: '$40', costCapUSD: '32',   hardStop: false },
];

function PlanRow({ plan, active }: { plan: Plan; active: boolean }) {
  return (
    <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{
      p: 1.2,
      borderRadius: '8px',
      border: '1px solid',
      borderColor: active ? tokens.color.accent.lime : 'rgba(17,17,17,0.12)',
      bgcolor: active ? 'rgba(229,255,0,0.12)' : '#fffaf1',
    }}>
      <Box sx={{ minWidth: 0 }}>
        <Stack direction="row" spacing={1} alignItems="center">
          <Typography variant="subtitle2" sx={{ fontWeight: 900 }}>{plan.name}</Typography>
          {active && <StatusPill kind="passed" label="פעיל" />}
        </Stack>
        <Typography variant="caption" sx={{ color: '#686158' }}>
          תקרת עלות ${Number(plan.costCapUSD).toFixed(2)}/חודש{plan.hardStop ? ' · עצירה קשיחה' : ''}
        </Typography>
      </Box>
      <Typography variant="subtitle2" sx={{ fontFamily: tokens.font.mono }}>{plan.monthlyPrice}</Typography>
    </Stack>
  );
}

/* ---------------------------- Integrations tab --------------------------- */

function IntegrationsTab() {
  const [github, setGithub] = useState<GitHubStatus | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setGithub(await githubApi.me());
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      if (msg === 'github-disabled') {
        setError('המחבר ל־GitHub מנוטרל בסביבה זו');
        setGithub({ connected: false });
      } else {
        setError(msg);
      }
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void refresh(); }, [refresh]);

  async function connect() {
    setBusy(true); setError(null);
    try { await githubApi.startConnect(); }
    catch (e) {
      setError(e instanceof Error ? e.message : String(e));
      setBusy(false);
    }
  }

  async function disconnect() {
    setBusy(true); setError(null);
    try {
      await githubApi.disconnect();
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(2, 1fr)' }, gap: 1.6 }}>
      <IntegrationCard
        title="GitHub"
        icon={<GitHub />}
        description="חיבור לרפו אישי לסנכרון קוד, יצירת Pull Requests, וקלון אוטומטי לסביבת הריצה."
        status={loading ? 'loading' : (github?.connected ? 'connected' : 'disconnected')}
        meta={github?.login ? `מחובר כ־${github.login}` : 'לא מחובר'}
        primaryLabel={github?.connected ? 'נתק חיבור' : 'התחבר ל־GitHub'}
        onPrimary={github?.connected ? disconnect : connect}
        busy={busy}
        error={error}
      />
      <IntegrationCard
        title="Vercel / יעדי פריסה"
        icon={<Hub />}
        description="הוספת חשבון פריסה תתאפשר לאחר חיבור GitHub. כרגע פריסה מבוצעת דרך תהליך ה־deploy gate הפנימי."
        status="idle"
        meta="בקרוב"
      />
      <IntegrationCard
        title="Override לספק AI"
        icon={<Shield />}
        description="עקוף את ברירת המחדל של Ironflyer במפתח Anthropic או OpenAI משלך — חיובי הספק יחויבו אליך ישירות."
        status="idle"
        meta="ניהול במסך המחברים"
        primaryLabel="פתח מחברים"
        onPrimary={() => { window.location.href = '/app/connectors'; }}
      />
      <IntegrationCard
        title="Webhooks"
        icon={<Hub />}
        description="קבל אירועי גייט ופריסה לכל endpoint שתגדיר. שימושי עבור Slack, Discord או דשבורד פנימי."
        status="idle"
        meta="בקרוב"
      />
    </Box>
  );
}

function IntegrationCard({
  title, icon, description, status, meta, primaryLabel, onPrimary, busy, error,
}: {
  title: string;
  icon: React.ReactNode;
  description: string;
  status: 'connected' | 'disconnected' | 'idle' | 'loading';
  meta?: string;
  primaryLabel?: string;
  onPrimary?: () => void;
  busy?: boolean;
  error?: string | null;
}) {
  const pillKind =
    status === 'connected' ? 'connected' :
    status === 'disconnected' ? 'idle' :
    status === 'loading' ? 'pending' :
    'idle';
  const pillLabel =
    status === 'connected' ? 'מחובר' :
    status === 'disconnected' ? 'לא מחובר' :
    status === 'loading' ? 'טוען' :
    'בקרוב';
  return (
    <Surface sx={{ p: 2.2, display: 'flex', flexDirection: 'column' }}>
      <Stack direction="row" justifyContent="space-between" alignItems="center">
        <Stack direction="row" spacing={1.2} alignItems="center">
          <Box sx={{ width: 38, height: 38, borderRadius: '8px', bgcolor: '#fffaf1', border: '1px solid rgba(17,17,17,0.12)', display: 'grid', placeItems: 'center' }}>
            {icon}
          </Box>
          <Typography variant="subtitle1" sx={{ fontWeight: 900 }}>{title}</Typography>
        </Stack>
        <StatusPill kind={pillKind as any} label={pillLabel} />
      </Stack>
      <Typography variant="body2" sx={{ color: '#686158', mt: 1.4, flex: 1 }}>{description}</Typography>
      {meta && (
        <Typography variant="caption" sx={{ color: '#86807a', mt: 1, fontFamily: tokens.font.mono }}>{meta}</Typography>
      )}
      {error && (
        <Typography variant="caption" sx={{ color: '#9b1010', mt: 0.6 }}>{error}</Typography>
      )}
      {primaryLabel && onPrimary && (
        <Button
          onClick={onPrimary}
          disabled={busy}
          variant={status === 'connected' ? 'outlined' : 'contained'}
          startIcon={busy ? <CircularProgress size={14} color="inherit" /> : undefined}
          sx={{ mt: 1.6, alignSelf: 'flex-start' }}
        >
          {primaryLabel}
        </Button>
      )}
    </Surface>
  );
}

/* ------------------------------ Vault tab ------------------------------- */

function VaultTab() {
  const [snapshot, setSnapshot] = useState<VaultSnapshot | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [scope, setScope] = useState<'user' | 'workspace'>('user');

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const next = await billingApi.myVault().catch(async () => {
        setScope('workspace');
        return billingApi.workspaceVault();
      });
      setSnapshot(next);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
      setSnapshot(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void refresh(); }, [refresh]);

  return (
    <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' }, gap: 1.6 }}>
      <Surface sx={{ p: 2.4, gridColumn: { xs: '1', md: '1 / -1' } }}>
        <Stack direction={{ xs: 'column', md: 'row' }} justifyContent="space-between" spacing={1.2} alignItems={{ xs: 'flex-start', md: 'center' }}>
          <Box>
            <Typography variant="overline" sx={{ color: '#9fb500' }}>{scope === 'user' ? 'הכספת שלך' : 'כספת סביבה'}</Typography>
            <Typography variant="h5" sx={{ fontWeight: 900, mt: 0.4 }}>הכנסות, עלות ספקים, מרווח</Typography>
            <Typography variant="body2" sx={{ color: '#686158', mt: 0.4, maxWidth: 540 }}>
              שיקוף מלא: revenue − providerCost = margin. הנתונים מגיעים ישירות מספר ה־ledger של Ironflyer ומתעדכנים בכל חיוב.
            </Typography>
          </Box>
          <Button variant="outlined" startIcon={<Refresh />} onClick={() => void refresh()}>רענן</Button>
        </Stack>
        {error && (
          <Box sx={{ mt: 1.4 }}>
            <ErrorBox title="לא ניתן לטעון את הכספת" description={error} onRetry={() => void refresh()} />
          </Box>
        )}
      </Surface>

      <VaultStat label="הכנסות (revenue)" value={snapshot?.revenue} loading={loading} accent="lime" />
      <VaultStat label="עלות ספקים (provider cost)" value={snapshot?.providerCost} loading={loading} accent="coral" negative />
      <VaultStat label="מרווח (margin)" value={snapshot?.margin} loading={loading} accent="sky" />
      <VaultStat label="זיכויים (refunds)" value={snapshot?.refunds} loading={loading} accent="neutral" />
      <VaultStat label="התאמות (adjustments)" value={snapshot?.adjustments} loading={loading} accent="neutral" />
      <Surface sx={{ p: 2.2 }}>
        <Typography variant="subtitle2" sx={{ fontWeight: 900 }}>נוסחת המרווח</Typography>
        <Typography variant="body2" sx={{ color: '#686158', mt: 0.6 }}>
          margin = revenue − providerCost − refunds + adjustments
        </Typography>
        <Typography variant="caption" sx={{ color: '#86807a', display: 'block', mt: 1 }}>
          ערכי הכספת זהים בכל מסך — כל ייצור מחיר חוזר לאותו ה־snapshot.
        </Typography>
      </Surface>
    </Box>
  );
}

function VaultStat({
  label, value, loading, accent, negative,
}: {
  label: string;
  value?: string;
  loading: boolean;
  accent: 'lime' | 'coral' | 'sky' | 'neutral';
  negative?: boolean;
}) {
  const color =
    accent === 'lime' ? tokens.color.accent.lime :
    accent === 'coral' ? tokens.color.accent.coral :
    accent === 'sky' ? tokens.color.accent.sky :
    'rgba(17,17,17,0.18)';
  const numeric = Number(value ?? 0);
  return (
    <Surface sx={{ p: 2.2 }}>
      <Typography variant="caption" sx={{ color: '#86807a' }}>{label}</Typography>
      {loading ? (
        <Box sx={{ mt: 1 }}><SkeletonBlock height={32} width="80%" /></Box>
      ) : (
        <Typography variant="h4" sx={{ fontFamily: tokens.font.mono, fontWeight: 900, mt: 0.4 }}>
          {negative && numeric > 0 ? '−' : ''}${Math.abs(numeric).toFixed(2)}
        </Typography>
      )}
      <Box sx={{ mt: 1.2, height: 4, borderRadius: '4px', bgcolor: color, opacity: 0.85 }} />
    </Surface>
  );
}

/* ------------------------------ Danger tab ------------------------------ */

function DangerTab({ projectCount, onLogout }: { projectCount: number; onLogout: () => void }) {
  const [stage, setStage] = useState<{ kind: 'projects' | 'account'; step: 1 | 2 } | null>(null);
  const [busy, setBusy] = useState(false);
  const [feedback, setFeedback] = useState<{ kind: 'ok' | 'err'; msg: string } | null>(null);

  async function deleteAllProjects() {
    setBusy(true); setFeedback(null);
    try {
      await accountApi.deleteAllProjects();
      setFeedback({ kind: 'ok', msg: 'כל הפרויקטים נמחקו. רענן את הדשבורד כדי לראות מצב נקי.' });
      setStage(null);
    } catch (e) {
      setFeedback({ kind: 'err', msg: e instanceof Error ? e.message : String(e) });
    } finally {
      setBusy(false);
    }
  }

  async function deleteAccount() {
    setBusy(true); setFeedback(null);
    try {
      await accountApi.deleteAccount();
      setFeedback({ kind: 'ok', msg: 'החשבון נמחק. מתנתק...' });
      setTimeout(onLogout, 800);
    } catch (e) {
      setFeedback({ kind: 'err', msg: e instanceof Error ? e.message : String(e) });
      setBusy(false);
    }
  }

  return (
    <Stack spacing={1.6}>
      <Surface sx={{ p: 2.4, borderColor: 'rgba(255,108,58,0.4) !important', bgcolor: 'rgba(255,108,58,0.06)' }}>
        <Stack direction={{ xs: 'column', md: 'row' }} spacing={1.4} justifyContent="space-between" alignItems={{ xs: 'flex-start', md: 'center' }}>
          <Box>
            <Typography variant="h6" sx={{ fontWeight: 900 }}>מחיקת כל הפרויקטים</Typography>
            <Typography variant="body2" sx={{ color: '#5b554b', mt: 0.4 }}>
              פעולה זו מסירה את {projectCount} הפרויקטים שלך, כולל היסטוריית גייטים ופאצ׳ים. לא ניתן לשחזר.
            </Typography>
          </Box>
          <Stack direction="row" spacing={1}>
            {stage?.kind === 'projects' && stage.step === 1 && (
              <Button
                variant="contained"
                color="error"
                onClick={() => setStage({ kind: 'projects', step: 2 })}
              >
                אישור ראשון — הבא
              </Button>
            )}
            {stage?.kind === 'projects' && stage.step === 2 && (
              <Button
                variant="contained"
                color="error"
                disabled={busy}
                startIcon={busy ? <CircularProgress size={14} color="inherit" /> : <Delete />}
                onClick={() => void deleteAllProjects()}
              >
                מחק עכשיו את כל ה־{projectCount} הפרויקטים
              </Button>
            )}
            {(!stage || stage.kind !== 'projects') && (
              <Button
                variant="outlined"
                color="error"
                onClick={() => setStage({ kind: 'projects', step: 1 })}
                startIcon={<Delete fontSize="small" />}
              >
                מחק את כל הפרויקטים
              </Button>
            )}
            {stage && (
              <Button onClick={() => setStage(null)}>ביטול</Button>
            )}
          </Stack>
        </Stack>
      </Surface>

      <Surface sx={{ p: 2.4, borderColor: 'rgba(255,24,24,0.32) !important', bgcolor: 'rgba(255,24,24,0.04)' }}>
        <Stack direction={{ xs: 'column', md: 'row' }} spacing={1.4} justifyContent="space-between" alignItems={{ xs: 'flex-start', md: 'center' }}>
          <Box>
            <Typography variant="h6" sx={{ fontWeight: 900, color: '#7a0e0e' }}>מחיקת חשבון</Typography>
            <Typography variant="body2" sx={{ color: '#5b554b', mt: 0.4 }}>
              סוגרת לצמיתות את החשבון, כל הפרויקטים, וההיסטוריה. דרושים שני אישורים. הפעולה אינה ניתנת לביטול.
            </Typography>
          </Box>
          <Stack direction="row" spacing={1}>
            {stage?.kind === 'account' && stage.step === 1 && (
              <Button variant="contained" color="error" onClick={() => setStage({ kind: 'account', step: 2 })}>
                הבנתי — המשך
              </Button>
            )}
            {stage?.kind === 'account' && stage.step === 2 && (
              <Button
                variant="contained"
                color="error"
                disabled={busy}
                startIcon={busy ? <CircularProgress size={14} color="inherit" /> : <Delete />}
                onClick={() => void deleteAccount()}
              >
                מחק את החשבון
              </Button>
            )}
            {(!stage || stage.kind !== 'account') && (
              <Button variant="outlined" color="error" onClick={() => setStage({ kind: 'account', step: 1 })} startIcon={<Delete fontSize="small" />}>
                מחק חשבון
              </Button>
            )}
            {stage?.kind === 'account' && (
              <Button onClick={() => setStage(null)}>ביטול</Button>
            )}
          </Stack>
        </Stack>
      </Surface>

      {feedback && (
        <Box>
          {feedback.kind === 'ok' ? (
            <Surface sx={{ p: 1.6, borderColor: tokens.color.accent.lime, bgcolor: 'rgba(229,255,0,0.16)' }}>
              <Typography variant="body2" sx={{ fontWeight: 800 }}>{feedback.msg}</Typography>
            </Surface>
          ) : (
            <ErrorBox title="הפעולה נכשלה" description={feedback.msg} />
          )}
        </Box>
      )}
    </Stack>
  );
}

const infoRowSx = {
  p: 1.2,
  borderRadius: '8px',
  border: '1px solid rgba(17,17,17,0.1)',
  bgcolor: '#fffaf1',
};
