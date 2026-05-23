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
  { value: 'account',      label: 'Account',      icon: <AccountCircle fontSize="small" /> },
  { value: 'billing',      label: 'Billing',      icon: <AccountBalanceWallet fontSize="small" /> },
  { value: 'integrations', label: 'Integrations', icon: <Hub fontSize="small" /> },
  { value: 'vault',        label: 'Vault',        icon: <Shield fontSize="small" /> },
  { value: 'danger',       label: 'Danger zone',  icon: <ErrorOutline fontSize="small" /> },
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
        eyebrow="Settings"
        title="Workspace controls"
        subtitle="Account, billing, integrations, budget vault, and destructive actions."
      />
      <Box sx={{ mb: 1.5 }}>
        <BillingStatusBanner compact />
      </Box>

      {error && (
        <Box sx={{ mb: 1.6 }}>
          <ErrorBox title="Loading error" description={error} onRetry={() => void refresh()} />
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
        <Typography variant="h6" sx={{ fontWeight: 900 }}>Profile</Typography>
        <Typography variant="body2" sx={{ color: '#686158', mt: 0.4 }}>
          Personal details shown in the dashboard. Display name and avatar are stored locally for now; server sync will come later.
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
                label="Display name"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                InputLabelProps={{ shrink: true }}
              />
              <TextField
                label="Email"
                value={user?.email ?? ''}
                InputLabelProps={{ shrink: true }}
                disabled
                helperText="Email is set when the account is created and cannot be edited here."
              />
              <TextField
                label="Avatar URL"
                placeholder="https://..."
                value={avatarUrl}
                onChange={(e) => setAvatarUrl(e.target.value)}
                InputLabelProps={{ shrink: true }}
              />
              <Stack direction="row" spacing={1} sx={{ mt: 0.6 }}>
                <Button variant="contained" startIcon={<Save />} onClick={save}>Save</Button>
                {savedAt && (
                  <Typography variant="caption" sx={{ alignSelf: 'center', color: '#6f7e00', fontWeight: 800 }}>
                    Saved at {new Date(savedAt).toLocaleTimeString('en-US')}
                  </Typography>
                )}
              </Stack>
            </>
          )}
        </Stack>
      </Surface>

      <Surface sx={{ p: 2.4 }}>
        <Typography variant="h6" sx={{ fontWeight: 900 }}>Session and security</Typography>
        <Typography variant="body2" sx={{ color: '#686158', mt: 0.4 }}>
          Signing out removes this browser session. Use the same email and password to sign in on another device.
        </Typography>
        <Stack spacing={1} sx={{ mt: 2.4 }}>
          <Stack direction="row" justifyContent="space-between" alignItems="center" sx={infoRowSx}>
            <Box>
              <Typography variant="subtitle2" sx={{ fontWeight: 900 }}>Sign-in status</Typography>
              <Typography variant="caption" sx={{ color: '#86807a' }}>JWT active in this browser.</Typography>
            </Box>
            <StatusPill kind="passed" label="Active" />
          </Stack>
          <Stack direction="row" justifyContent="space-between" alignItems="center" sx={infoRowSx}>
            <Box>
              <Typography variant="subtitle2" sx={{ fontWeight: 900 }}>Two-factor authentication</Typography>
              <Typography variant="caption" sx={{ color: '#86807a' }}>Available after Single Sign-On is connected.</Typography>
            </Box>
            <StatusPill kind="idle" label="Soon" />
          </Stack>
          <Button onClick={onLogout} variant="outlined" startIcon={<Logout />} sx={{ mt: 1, alignSelf: 'flex-start' }}>
            Sign out of this workspace
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
            <Typography variant="overline" sx={{ color: '#9fb500' }}>Current plan</Typography>
            <Typography variant="h5" sx={{ fontWeight: 900, mt: 0.4 }}>
              {currentPlan?.name ?? (tier === 'free' ? 'Free workspace' : tier)}
            </Typography>
            <Typography variant="body2" sx={{ color: '#686158', mt: 0.4 }}>
              {currentPlan?.monthlyPrice
                ? `${currentPlan.monthlyPrice} per month — renews automatically`
                : 'Free plan with a hard cost cap.'}
            </Typography>
          </Box>
          <Tooltip title="Refresh">
            <IconButton size="small" onClick={onRefresh}><Refresh fontSize="small" /></IconButton>
          </Tooltip>
        </Stack>

        <Box sx={{ mt: 2.4 }}>
          <Stack direction="row" justifyContent="space-between">
            <Typography variant="caption" sx={{ color: '#686158' }}>Spend this month</Typography>
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
          <Typography variant="subtitle2" sx={{ fontWeight: 900 }}>Usage over the last 30 days</Typography>
          {loading ? (
            <Box sx={{ mt: 1 }}><SkeletonBlock height={96} radius={8} /></Box>
          ) : (
            <Box sx={{ mt: 1 }}>
              <UsageSpark
                points={points}
                height={110}
                emptyHint="No verified charges this month"
              />
            </Box>
          )}
        </Box>

        <Divider sx={{ my: 2.2 }} />
        {stripeEnabled ? (
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1}>
            <UpgradeButton tier="pro" label="Upgrade to Pro" size="small" />
            <UpgradeButton tier="team" label="Open Team plan" size="small" variant="outlined" />
            <UpgradeButton tier="enterprise" label="Contact Enterprise" size="small" variant="outlined" />
          </Stack>
        ) : (
          <Box sx={{ p: 1.6, borderRadius: '8px', bgcolor: 'rgba(255,196,0,0.16)', border: '1px solid rgba(122,91,0,0.26)' }}>
            <Stack direction="row" spacing={1} alignItems="center">
              <Warning fontSize="small" sx={{ color: '#7a5b00' }} />
              <Typography variant="body2" sx={{ color: '#5b4500', fontWeight: 800 }}>
                Stripe is disabled in this environment. The team will handle upgrades manually.
              </Typography>
            </Stack>
          </Box>
        )}
      </Surface>

      <Surface sx={{ p: 2.4 }}>
        <Typography variant="h6" sx={{ fontWeight: 900 }}>Available plans</Typography>
        <Stack spacing={1} sx={{ mt: 1.8 }}>
          {(plans.length ? plans : fallbackPlans).map((plan) => (
            <PlanRow key={plan.tier} plan={plan} active={plan.tier === tier} />
          ))}
        </Stack>
        <Divider sx={{ my: 2 }} />
        <Typography variant="caption" sx={{ color: '#686158' }}>
          Charges are based on verified usage inside the plan cap. Every run is tied to your account and visible in the <Link href="/app/settings?tab=vault" style={{ color: '#6f7e00', fontWeight: 800 }}>Vault</Link>.
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
          {active && <StatusPill kind="passed" label="Active" />}
        </Stack>
        <Typography variant="caption" sx={{ color: '#686158' }}>
          Cost cap ${Number(plan.costCapUSD).toFixed(2)}/month{plan.hardStop ? ' · hard stop' : ''}
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
        setError('The GitHub connector is disabled in this environment');
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
        description="Connect a personal repository for code sync, pull requests, and automatic runtime cloning."
        status={loading ? 'loading' : (github?.connected ? 'connected' : 'disconnected')}
        meta={github?.login ? `Connected as ${github.login}` : 'Not connected'}
        primaryLabel={github?.connected ? 'Disconnect' : 'Connect GitHub'}
        onPrimary={github?.connected ? disconnect : connect}
        busy={busy}
        error={error}
      />
      <IntegrationCard
        title="Vercel / deploy targets"
        icon={<Hub />}
        description="Deployment account linking will be available after GitHub is connected. For now, deploys run through the internal deploy gate."
        status="idle"
        meta="Coming soon"
      />
      <IntegrationCard
        title="AI provider override"
        icon={<Shield />}
        description="Override Ironflyer defaults with your own Anthropic or OpenAI key. Provider usage bills directly to you."
        status="idle"
        meta="Managed from Connectors"
        primaryLabel="Open connectors"
        onPrimary={() => { window.location.href = '/app/connectors'; }}
      />
      <IntegrationCard
        title="Webhooks"
        icon={<Hub />}
        description="Send gate and deploy events to any endpoint you configure. Useful for Slack, Discord, or internal dashboards."
        status="idle"
        meta="Coming soon"
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
    status === 'connected' ? 'Connected' :
    status === 'disconnected' ? 'Not connected' :
    status === 'loading' ? 'Loading' :
    'Soon';
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
            <Typography variant="overline" sx={{ color: '#9fb500' }}>{scope === 'user' ? 'Your vault' : 'Workspace vault'}</Typography>
            <Typography variant="h5" sx={{ fontWeight: 900, mt: 0.4 }}>Revenue, provider cost, margin</Typography>
            <Typography variant="body2" sx={{ color: '#686158', mt: 0.4, maxWidth: 540 }}>
              Full visibility: revenue minus provider cost equals margin. Values come directly from the Ironflyer ledger and update on every charge.
            </Typography>
          </Box>
          <Button variant="outlined" startIcon={<Refresh />} onClick={() => void refresh()}>Refresh</Button>
        </Stack>
        {error && (
          <Box sx={{ mt: 1.4 }}>
            <ErrorBox title="Could not load vault" description={error} onRetry={() => void refresh()} />
          </Box>
        )}
      </Surface>

      <VaultStat label="Revenue" value={snapshot?.revenue} loading={loading} accent="lime" />
      <VaultStat label="Provider cost" value={snapshot?.providerCost} loading={loading} accent="coral" negative />
      <VaultStat label="Margin" value={snapshot?.margin} loading={loading} accent="sky" />
      <VaultStat label="Refunds" value={snapshot?.refunds} loading={loading} accent="neutral" />
      <VaultStat label="Adjustments" value={snapshot?.adjustments} loading={loading} accent="neutral" />
      <Surface sx={{ p: 2.2 }}>
        <Typography variant="subtitle2" sx={{ fontWeight: 900 }}>Margin formula</Typography>
        <Typography variant="body2" sx={{ color: '#686158', mt: 0.6 }}>
          margin = revenue − providerCost − refunds + adjustments
        </Typography>
        <Typography variant="caption" sx={{ color: '#86807a', display: 'block', mt: 1 }}>
          Vault values are identical across screens because every pricing view reads from the same snapshot.
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
      setFeedback({ kind: 'ok', msg: 'All projects were deleted. Refresh the dashboard to see a clean workspace.' });
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
      setFeedback({ kind: 'ok', msg: 'Account deleted. Signing out...' });
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
            <Typography variant="h6" sx={{ fontWeight: 900 }}>Delete all projects</Typography>
            <Typography variant="body2" sx={{ color: '#5b554b', mt: 0.4 }}>
              This removes all {projectCount} projects, including gate history and patches. This cannot be undone.
            </Typography>
          </Box>
          <Stack direction="row" spacing={1}>
            {stage?.kind === 'projects' && stage.step === 1 && (
              <Button
                variant="contained"
                color="error"
                onClick={() => setStage({ kind: 'projects', step: 2 })}
              >
                First confirmation — continue
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
                Delete all {projectCount} projects now
              </Button>
            )}
            {(!stage || stage.kind !== 'projects') && (
              <Button
                variant="outlined"
                color="error"
                onClick={() => setStage({ kind: 'projects', step: 1 })}
                startIcon={<Delete fontSize="small" />}
              >
                Delete all projects
              </Button>
            )}
            {stage && (
              <Button onClick={() => setStage(null)}>Cancel</Button>
            )}
          </Stack>
        </Stack>
      </Surface>

      <Surface sx={{ p: 2.4, borderColor: 'rgba(255,24,24,0.32) !important', bgcolor: 'rgba(255,24,24,0.04)' }}>
        <Stack direction={{ xs: 'column', md: 'row' }} spacing={1.4} justifyContent="space-between" alignItems={{ xs: 'flex-start', md: 'center' }}>
          <Box>
            <Typography variant="h6" sx={{ fontWeight: 900, color: '#7a0e0e' }}>Delete account</Typography>
            <Typography variant="body2" sx={{ color: '#5b554b', mt: 0.4 }}>
              Permanently closes the account, all projects, and history. Two confirmations are required. This cannot be undone.
            </Typography>
          </Box>
          <Stack direction="row" spacing={1}>
            {stage?.kind === 'account' && stage.step === 1 && (
              <Button variant="contained" color="error" onClick={() => setStage({ kind: 'account', step: 2 })}>
                I understand — continue
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
                Delete account
              </Button>
            )}
            {(!stage || stage.kind !== 'account') && (
              <Button variant="outlined" color="error" onClick={() => setStage({ kind: 'account', step: 1 })} startIcon={<Delete fontSize="small" />}>
                Delete account
              </Button>
            )}
            {stage?.kind === 'account' && (
              <Button onClick={() => setStage(null)}>Cancel</Button>
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
            <ErrorBox title="Action failed" description={feedback.msg} />
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
