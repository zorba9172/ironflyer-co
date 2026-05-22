'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  AutoAwesome, Code, DataObject, GitHub, Hub, Lock, Refresh, RocketLaunch,
  Security, Storage, TravelExplore,
} from '@mui/icons-material';
import {
  Box, Button, Chip, CircularProgress, LinearProgress, Stack, Typography,
} from '@mui/material';
import { api, Project } from '../../../lib/api';
import { tokens } from '../../../lib/theme';
import { RequireAuth, useAuth } from '../../auth-context';
import { githubApi, GitHubStatus } from '../../../lib/github';
import { AppShell, PageTitle, Surface } from '../workspace-shell';
import { EmptyState, ErrorBox, StatusPill } from '../../../components/dashboard';

type ConnectorState = 'connected' | 'available' | 'disabled' | 'soon';

interface ConnectorCard {
  id: string;
  name: string;
  group: 'apps' | 'deploy' | 'chat' | 'ai' | 'runtime';
  desc: string;
  icon: React.ReactNode;
  state: ConnectorState;
  meta?: string;
  primaryLabel?: string;
  onPrimary?: () => void | Promise<void>;
  secondary?: React.ReactNode;
  error?: string | null;
  busy?: boolean;
}

const groups: { value: 'all' | ConnectorCard['group']; label: string }[] = [
  { value: 'all',     label: 'הכל' },
  { value: 'apps',    label: 'אפליקציה' },
  { value: 'deploy',  label: 'פריסה' },
  { value: 'ai',      label: 'ספקי AI' },
  { value: 'chat',    label: 'צ׳אט' },
  { value: 'runtime', label: 'ריצה' },
];

export default function ConnectorsPage() {
  return (
    <RequireAuth>
      <ConnectorsInner />
    </RequireAuth>
  );
}

function ConnectorsInner() {
  const { user, logout } = useAuth();
  const [projects, setProjects] = useState<Project[]>([]);
  const [github, setGithub] = useState<GitHubStatus | null>(null);
  const [githubError, setGithubError] = useState<string | null>(null);
  const [githubLoading, setGithubLoading] = useState(true);
  const [githubBusy, setGithubBusy] = useState(false);
  const [group, setGroup] = useState<typeof groups[number]['value']>('all');
  const [query, setQuery] = useState('');

  useEffect(() => {
    void api.listProjects().then(setProjects).catch(() => setProjects([]));
  }, []);

  const loadGithub = useCallback(async () => {
    setGithubLoading(true);
    setGithubError(null);
    try {
      setGithub(await githubApi.me());
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      if (msg === 'github-disabled') {
        setGithub({ connected: false });
        setGithubError('המחבר ל־GitHub מנוטרל בסביבה זו');
      } else {
        setGithubError(msg);
      }
    } finally {
      setGithubLoading(false);
    }
  }, []);

  useEffect(() => { void loadGithub(); }, [loadGithub]);

  async function connectGithub() {
    setGithubBusy(true);
    setGithubError(null);
    try { await githubApi.startConnect(); }
    catch (e) {
      setGithubError(e instanceof Error ? e.message : String(e));
      setGithubBusy(false);
    }
  }

  async function disconnectGithub() {
    setGithubBusy(true);
    setGithubError(null);
    try {
      await githubApi.disconnect();
      await loadGithub();
    } catch (e) {
      setGithubError(e instanceof Error ? e.message : String(e));
    } finally {
      setGithubBusy(false);
    }
  }

  const connectors: ConnectorCard[] = useMemo(() => [
    {
      id: 'github',
      name: 'GitHub',
      group: 'apps',
      icon: <GitHub />,
      desc: 'סנכרון רפו, יצירת Pull Requests, וקלון אוטומטי של פרויקטים לסביבת הריצה.',
      state: githubLoading ? 'available' : github?.connected ? 'connected' : 'available',
      meta: githubLoading ? 'טוען מצב...' : github?.login ? `מחובר כ־${github.login}` : 'דרושה הסכמת OAuth',
      primaryLabel: githubLoading ? 'בודק...' : github?.connected ? 'נתק' : 'התחבר',
      onPrimary: githubLoading ? undefined : github?.connected ? disconnectGithub : connectGithub,
      busy: githubBusy,
      error: githubError,
    },
    {
      id: 'runtime',
      name: 'סביבת ריצה',
      group: 'runtime',
      icon: <Code />,
      desc: 'תצוגה מקדימה, טרמינל ו־file API לכל פרויקט. רץ ב־Mock או Docker בהתאם לסביבה.',
      state: 'connected',
      meta: 'מופעל אוטומטית לכל פרויקט',
    },
    {
      id: 'vercel',
      name: 'Vercel',
      group: 'deploy',
      icon: <RocketLaunch />,
      desc: 'יעד פריסה אוטומטי לפרויקטים מבוססי Next.js. מתחבר לאחר חיבור GitHub.',
      state: github?.connected ? 'available' : 'soon',
      meta: github?.connected ? 'התקנה ידנית בקרוב' : 'דורש חיבור GitHub תחילה',
    },
    {
      id: 'anthropic',
      name: 'Anthropic Override',
      group: 'ai',
      icon: <AutoAwesome />,
      desc: 'השתמש במפתח Claude משלך כדי לעקוף את ברירת המחדל וחיוב ישיר אל החשבון שלך.',
      state: 'available',
      meta: 'נוסף דרך הגדרות → אינטגרציות',
      primaryLabel: 'פתח הגדרות',
      onPrimary: () => { window.location.href = '/app/settings?tab=integrations'; },
    },
    {
      id: 'openai',
      name: 'OpenAI Override',
      group: 'ai',
      icon: <AutoAwesome />,
      desc: 'מפתח OpenAI אישי לשימוש בגייטים שתומכים בכך. החיוב מועבר אלייך ישירות.',
      state: 'available',
      meta: 'נוסף דרך הגדרות → אינטגרציות',
      primaryLabel: 'פתח הגדרות',
      onPrimary: () => { window.location.href = '/app/settings?tab=integrations'; },
    },
    {
      id: 'supabase',
      name: 'Supabase',
      group: 'apps',
      icon: <Storage />,
      desc: 'מסד נתונים, אימות, אחסון ו־RLS. מתאים לפרויקטים שמצריכים backend מנוהל.',
      state: 'available',
      meta: 'דורש הוספת סודות בפרויקט',
    },
    {
      id: 'figma',
      name: 'Figma',
      group: 'chat',
      icon: <DataObject />,
      desc: 'הפניית פריימים, צילומי מסך ומערכת עיצוב כקונטקסט לזמן הבנייה.',
      state: 'soon',
      meta: 'בקרוב',
    },
    {
      id: 'search',
      name: 'חיפוש רשת',
      group: 'chat',
      icon: <TravelExplore />,
      desc: 'חיפוש Web חי לעיגון מחקר, תיעוד ועובדות בזמן בנייה.',
      state: 'connected',
      meta: 'מופעל לכל המשתמשים',
    },
    {
      id: 'mcp',
      name: 'MCP Servers',
      group: 'chat',
      icon: <Hub />,
      desc: 'כלי קונטקסט פרטיים לצוותים ו־Enterprise. נוסף דרך מדיניות מנהל בלבד.',
      state: 'soon',
      meta: 'יתאפשר בחבילת Team',
    },
  ], [github, githubBusy, githubLoading, githubError]);

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    return connectors.filter((c) => {
      if (group !== 'all' && c.group !== group) return false;
      if (!q) return true;
      return `${c.name} ${c.desc} ${c.meta ?? ''}`.toLowerCase().includes(q);
    });
  }, [connectors, group, query]);

  const connectedCount = connectors.filter((c) => c.state === 'connected').length;
  const readiness = Math.round((connectedCount / connectors.length) * 100);

  return (
    <AppShell
      userEmail={user?.email ?? 'workspace'}
      recents={projects.slice(0, 5)}
      onLogout={logout}
      query={query}
      setQuery={setQuery}
    >
      <PageTitle
        eyebrow="מחברים"
        title="חיבורים חיצוניים"
        subtitle="חיבורים שמשתלבים עם בנייה, תצוגה מקדימה והפריסה. כל חיבור עם סטטוס בזמן אמת."
        action={
          <Button variant="outlined" startIcon={<Refresh fontSize="small" />} onClick={() => void loadGithub()}>
            רענן סטטוס
          </Button>
        }
      />

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1.1fr 0.9fr' }, gap: 1.4, mb: 1.6 }}>
        <Surface sx={{ p: 1.8 }}>
          <Stack direction="row" spacing={1.2} alignItems="center">
            <Box sx={{ width: 42, height: 42, borderRadius: '8px', bgcolor: tokens.color.accent.lime, display: 'grid', placeItems: 'center' }}>
              <Security />
            </Box>
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Typography variant="h6" sx={{ fontWeight: 900 }}>מוכנות מחברים</Typography>
              <Typography variant="body2" color="text.secondary">
                {connectedCount} מתוך {connectors.length} כלים מוכנים לשימוש מיידי
              </Typography>
            </Box>
            <Typography variant="h5" sx={{ color: tokens.color.text.inverse, fontFamily: tokens.font.mono }}>{readiness}%</Typography>
          </Stack>
          <LinearProgress variant="determinate" value={readiness} sx={{
            mt: 1.5,
            height: 7,
            borderRadius: '999px',
            bgcolor: 'rgba(17,17,17,0.1)',
            '& .MuiLinearProgress-bar': { bgcolor: tokens.color.accent.lime },
          }} />
        </Surface>
        <Surface sx={{ p: 1.8 }}>
          <Stack direction="row" spacing={1.2} alignItems="flex-start">
            <Lock sx={{ color: tokens.color.accent.coral }} />
            <Box>
              <Typography variant="h6" sx={{ fontWeight: 900 }}>בטוח כברירת מחדל</Typography>
              <Typography variant="body2" color="text.secondary">
                אתה שולט במה שמתחבר. כל חיבור נשמר בהיקף החשבון שלך עד שתבחר לקשר אותו לפרויקט ספציפי.
              </Typography>
            </Box>
          </Stack>
        </Surface>
      </Box>

      <Stack direction="row" spacing={0.8} useFlexGap flexWrap="wrap" sx={{ mb: 1.4 }}>
        {groups.map((item) => (
          <Chip
            key={item.value}
            label={item.label}
            onClick={() => setGroup(item.value)}
            sx={{
              borderRadius: '8px',
              bgcolor: group === item.value ? tokens.color.accent.lime : '#fffaf1',
              color: tokens.color.text.inverse,
              border: `1px solid ${group === item.value ? tokens.color.accent.lime : 'rgba(17,17,17,0.14)'}`,
              fontWeight: 800,
            }}
          />
        ))}
      </Stack>

      {githubError && (
        <Box sx={{ mb: 1.4 }}>
          <ErrorBox title="סטטוס GitHub" description={githubError} onRetry={() => void loadGithub()} />
        </Box>
      )}

      {visible.length === 0 ? (
        <EmptyState
          illustration="orbit"
          title="אין מחברים תואמים"
          description="נסה לבחור קבוצה אחרת או לרוקן את החיפוש."
          primaryLabel="ניקוי חיפוש"
          onPrimary={() => { setQuery(''); setGroup('all'); }}
        />
      ) : (
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' }, gap: 1.4 }}>
          {visible.map((c) => <ConnectorCardView key={c.id} card={c} />)}
        </Box>
      )}
    </AppShell>
  );
}

function ConnectorCardView({ card }: { card: ConnectorCard }) {
  const pillKind =
    card.state === 'connected' ? 'connected' :
    card.state === 'available' ? 'available' :
    card.state === 'disabled' ? 'failed' :
    'idle';
  const pillLabel =
    card.state === 'connected' ? 'מחובר' :
    card.state === 'available' ? 'זמין' :
    card.state === 'disabled' ? 'מנוטרל' :
    'בקרוב';
  return (
    <Surface sx={{ p: 2, minHeight: 196, display: 'flex', flexDirection: 'column' }}>
      <Stack direction="row" justifyContent="space-between" alignItems="center">
        <Box sx={{
          width: 42, height: 42, borderRadius: '8px', display: 'grid', placeItems: 'center',
          bgcolor: card.state === 'connected' ? 'rgba(229,255,0,0.32)' : '#fffaf1',
          border: '1px solid rgba(17,17,17,0.12)',
          color: tokens.color.text.inverse,
        }}>{card.icon}</Box>
        <StatusPill kind={pillKind as any} label={pillLabel} />
      </Stack>
      <Typography variant="h6" sx={{ mt: 1.6, fontWeight: 900 }}>{card.name}</Typography>
      <Typography variant="body2" sx={{ color: '#686158', mt: 0.6, flex: 1 }}>{card.desc}</Typography>
      {card.meta && (
        <Typography variant="caption" sx={{ color: '#86807a', mt: 1, fontFamily: tokens.font.mono }}>{card.meta}</Typography>
      )}
      {card.error && (
        <Typography variant="caption" sx={{ color: '#9b1010', mt: 0.6 }}>{card.error}</Typography>
      )}
      {card.primaryLabel && card.onPrimary && (
        <Stack direction="row" spacing={1} sx={{ mt: 1.4 }}>
          <Button
            onClick={() => void card.onPrimary?.()}
            disabled={card.busy}
            variant={card.state === 'connected' ? 'outlined' : 'contained'}
            size="small"
            startIcon={card.busy ? <CircularProgress size={14} color="inherit" /> : undefined}
          >
            {card.primaryLabel}
          </Button>
          {card.secondary}
        </Stack>
      )}
    </Surface>
  );
}
