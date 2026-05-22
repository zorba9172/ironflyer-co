'use client';

import { useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import { ArrowForward, AutoAwesome, OpenInNew, PlayArrow } from '@mui/icons-material';
import { Box, Button, Chip, Stack, Typography } from '@mui/material';
import { api, LedgerEntry, Plan, Project, UserBudget } from '../../lib/api';
import { tokens } from '../../lib/theme';
import { RequireAuth, useAuth } from '../auth-context';
import { PromptBox } from '../prompt-box';
import { AppShell, Surface } from './workspace-shell';
import {
  ActivityTimeline, EmptyState, ErrorBox, ProjectGridCard, SkeletonCard, SkeletonGrid,
  StatusPill, UsageSpark, bucketByDay, flattenActivity, statusKindFromGate,
} from '../../components/dashboard';

const quickPrompts = [
  'בנה SaaS עם התחברות, חיוב, צוותים ולוח ניהול',
  'הקם כלי תפעול פנימי עם אישורים ודוחות',
  'השק אתר מוצר AI עם רשימת המתנה ותמחור',
  'הפוך רעיון גולמי למוצר מלוטש שמשתמשים יכולים להשתמש בו',
];

const featuredTemplates = [
  {
    title: 'Aiforge AI SaaS',
    desc: 'נחיתה, אינטגרציות, תמחור, בלוג, אונבורדינג',
    tag: 'Apps',
    prompt: 'Use the local Aiforge template as the visual foundation for an AI SaaS app with landing pages, integrations, pricing, blog, onboarding, and production launch checks.',
  },
  {
    title: 'Allstore Commerce',
    desc: 'קטלוג, באנר מבצעים, צ׳קאאוט מוכן',
    tag: 'Commerce',
    prompt: 'Use the local Allstore HTML template as the foundation for a commerce storefront with catalog pages, product detail, cart, checkout, order states, and CMS-ready content.',
  },
  {
    title: 'Davies Agency',
    desc: 'תיק עבודות, שירותים, תהליך, תמחור, קונטקט',
    tag: 'Websites',
    prompt: 'Use the local Davies template as the base for a premium agency website with portfolio demos, service sections, process, pricing, contact flows, analytics, and SEO checks.',
  },
];

export default function AppHome() {
  return (
    <RequireAuth>
      <AppHomeInner />
    </RequireAuth>
  );
}

function AppHomeInner() {
  const { user, logout } = useAuth();
  const [projects, setProjects] = useState<Project[]>([]);
  const [budget, setBudget] = useState<UserBudget | null>(null);
  const [plans, setPlans] = useState<Plan[]>([]);
  const [idea, setIdea] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [query, setQuery] = useState('');
  const [view, setView] = useState<'grid' | 'list'>('grid');

  useEffect(() => {
    void refresh();
    const pending = window.localStorage.getItem('ironflyer.pendingIdea');
    if (pending) {
      setIdea(pending);
      window.localStorage.removeItem('ironflyer.pendingIdea');
    }
  }, []);

  async function refresh() {
    setLoading(true);
    setError(null);
    try {
      const [nextProjects, nextBudget, nextPlans] = await Promise.all([
        api.listProjects(),
        api.myBudget().catch(() => null),
        api.listPlans().catch(() => [] as Plan[]),
      ]);
      setProjects(nextProjects);
      setBudget(nextBudget);
      setPlans(nextPlans);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }

  async function createFromIdea(nextIdea = idea) {
    if (!nextIdea.trim()) return;
    setBusy(true); setError(null);
    try {
      const name = nextIdea.split('\n')[0].slice(0, 60);
      const p = await api.createProject({ name, description: 'נוצר מ-prompt', idea: nextIdea });
      setIdea('');
      window.location.href = `/projects/${p.id}`;
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  const activity = useMemo(() => flattenActivity(projects, 10), [projects]);
  const continueProject = useMemo(() => {
    if (projects.length === 0) return undefined;
    return [...projects].sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt))[0];
  }, [projects]);
  const recents = projects.slice(0, 5);
  const usage = useMemo(() => summarizeUsage(budget, plans, projects, activity.length), [budget, plans, projects, activity.length]);

  return (
    <AppShell
      userEmail={user?.email ?? 'workspace'}
      recents={recents}
      onLogout={logout}
      query={query}
      setQuery={setQuery}
      view={view}
      setView={setView}
    >
      <Stack spacing={{ xs: 2.6, md: 3.2 }}>
        <HeroPrompt
          idea={idea}
          setIdea={setIdea}
          busy={busy}
          error={error}
          onSubmit={createFromIdea}
        />

        {error && !busy && (
          <ErrorBox
            title="טעינת לוח הבקרה נכשלה"
            description={error}
            onRetry={() => void refresh()}
          />
        )}

        {loading ? (
          <SkeletonGrid columns={3} count={3} minHeight={180} />
        ) : projects.length === 0 ? (
          <WelcomeCard onPick={(prompt) => setIdea(prompt)} templates={featuredTemplates} />
        ) : (
          <ContinueRow project={continueProject!} />
        )}

        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1.4fr 1fr' }, gap: 1.8 }}>
          <Surface sx={{ p: 0, overflow: 'hidden' }}>
            <SectionHeading title="פעילות אחרונה" subtitle="עדכוני גייטים, פאצ׳ים ופריסות מכל הפרויקטים" href="/app/projects" hrefLabel="כל הפרויקטים" />
            {loading ? (
              <Stack spacing={1} sx={{ p: 1.6 }}>
                <SkeletonCard lines={2} minHeight={56} />
                <SkeletonCard lines={2} minHeight={56} />
                <SkeletonCard lines={2} minHeight={56} />
              </Stack>
            ) : (
              <ActivityTimeline rows={activity} emptyHint="עוד אין פעילות. הרץ פרויקט כדי להתחיל." />
            )}
          </Surface>

          <Surface sx={{ p: 0, overflow: 'hidden' }}>
            <SectionHeading title="השימוש שלך" subtitle="חודש נוכחי" href="/app/settings?tab=billing" hrefLabel="חבילה ותשלומים" />
            <Box sx={{ p: 2 }}>
              <Stack direction="row" spacing={1} sx={{ mb: 1.4 }}>
                <UsageStat label="הרצות" value={usage.runs.toString()} />
                <UsageStat label="פאצ׳ים" value={usage.patches.toString()} />
                <UsageStat label="הוצאה" value={`$${usage.spent.toFixed(2)}`} accent />
              </Stack>
              <UsageSpark
                points={usage.points}
                emptyHint="עוד אין הוצאות החודש"
                caption={`תקרה $${usage.cap.toFixed(2)}`}
              />
              <Stack direction="row" spacing={1} sx={{ mt: 1.8 }}>
                <Button component={Link} href="/app/settings?tab=billing" variant="contained" size="small" endIcon={<ArrowForward fontSize="small" />}>
                  פתח חיוב
                </Button>
                <Button component={Link} href="/pricing" variant="outlined" size="small">
                  השווה חבילות
                </Button>
              </Stack>
            </Box>
          </Surface>
        </Box>

        {!loading && projects.length > 0 && (
          <Box>
            <SectionHeading title="הפרויקטים שלך" subtitle={`${projects.length} פעילים`} href="/app/projects" hrefLabel="כל הפרויקטים" inline />
            <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' }, gap: 1.5, mt: 1.5 }}>
              {projects.slice(0, 6).map((project) => (
                <ProjectGridCard key={project.id} project={project} />
              ))}
            </Box>
          </Box>
        )}
      </Stack>
    </AppShell>
  );
}

function HeroPrompt({
  idea, setIdea, busy, error, onSubmit,
}: {
  idea: string;
  setIdea: (v: string) => void;
  busy: boolean;
  error: string | null;
  onSubmit: (v: string) => void;
}) {
  return (
    <Stack alignItems="center" spacing={1.7} sx={{ textAlign: 'center', pt: { xs: 0.3, md: 1 }, pb: { xs: 0.2, md: 0.6 } }}>
      <Chip
        icon={<AutoAwesome sx={{ fontSize: 14 }} />}
        label="בנה עם Ironflyer"
        sx={{
          bgcolor: 'rgba(229,255,0,0.18)',
          color: '#586500',
          border: '1px solid rgba(17,17,17,0.12)',
          borderRadius: '8px',
          fontWeight: 900,
          '& .MuiChip-icon': { color: '#586500' },
        }}
      />
      <Typography variant="h2" sx={{
        maxWidth: 760,
        fontSize: { xs: '1.68rem', sm: '2.35rem', md: '3rem' },
        lineHeight: 0.96,
        textTransform: 'uppercase',
        textWrap: 'balance',
      }}>
        מה הולכים לבנות היום?
      </Typography>
      <Typography variant="body1" sx={{ maxWidth: 640, fontSize: { xs: '0.98rem', sm: '1rem' }, fontWeight: 600, color: '#686158' }}>
        תאר את האפליקציה בעברית רגילה. Ironflyer ידאג לתכנון, עיצוב, קוד, בדיקות והעלאה — שלב אחר שלב.
      </Typography>
      <Box sx={{ width: '100%', maxWidth: 820 }}>
        <PromptBox
          value={idea}
          onChange={setIdea}
          onSubmit={onSubmit}
          busy={busy}
          error={error}
          size="dashboard"
          cta="בנה אפליקציה"
          placeholder="בקש מ-Ironflyer לבנות SaaS, כלי פנימי, פורטל, או אתר..."
        />
      </Box>
      <Stack direction="row" spacing={0.9} useFlexGap flexWrap="wrap" justifyContent="center" sx={{ width: '100%', maxWidth: 900 }}>
        {quickPrompts.map((prompt) => (
          <Button
            key={prompt}
            variant="outlined"
            onClick={() => setIdea(prompt)}
            sx={{
              maxWidth: { xs: '100%', sm: 430 },
              minHeight: 38,
              px: 1.4,
              borderRadius: '8px',
              bgcolor: '#fffaf1',
              color: '#4a453e',
              borderColor: 'rgba(17,17,17,0.12)',
              justifyContent: 'flex-start',
              textAlign: 'right',
              fontWeight: 800,
              fontSize: { xs: '0.88rem', sm: '0.92rem' },
              lineHeight: 1.28,
              '&:hover': {
                borderColor: 'rgba(17,17,17,0.28)',
                bgcolor: 'rgba(229,255,0,0.24)',
              },
            }}
          >
            {prompt}
          </Button>
        ))}
      </Stack>
    </Stack>
  );
}

function WelcomeCard({ onPick, templates }: { onPick: (prompt: string) => void; templates: typeof featuredTemplates }) {
  return (
    <Surface sx={{ p: { xs: 2.2, md: 3 } }}>
      <Stack direction={{ xs: 'column', md: 'row' }} spacing={2} justifyContent="space-between" alignItems={{ xs: 'flex-start', md: 'center' }}>
        <Box sx={{ maxWidth: 520 }}>
          <Typography variant="overline" sx={{ color: '#9fb500' }}>ברוכים הבאים ל-Ironflyer</Typography>
          <Typography variant="h5" sx={{ mt: 0.4, fontWeight: 900 }}>בוא נבנה את המוצר הראשון שלך</Typography>
          <Typography variant="body2" sx={{ mt: 0.6, color: '#686158' }}>
            בחר תבנית מוכנה או הקלד רעיון בתיבת הפרומפט למעלה. כל הגייטים — ספק, ארכיטקטורה, קוד, בדיקות, אבטחה ופריסה —
            עוברים אוטומטית עד שהמוצר מוכן לשליחה.
          </Typography>
        </Box>
        <Button component={Link} href="/app/resources" variant="outlined" endIcon={<ArrowForward fontSize="small" />}>
          גלה תבניות
        </Button>
      </Stack>

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' }, gap: 1.4, mt: 2.4 }}>
        {templates.map((item) => (
          <Box
            key={item.title}
            sx={{
              p: 2,
              borderRadius: '8px',
              border: '1px solid rgba(17,17,17,0.12)',
              bgcolor: '#fffaf1',
              display: 'flex',
              flexDirection: 'column',
              transition: 'transform 200ms',
              '&:hover': { transform: 'translateY(-2px)', borderColor: 'rgba(17,17,17,0.28)' },
            }}
          >
            <Chip label={item.tag} size="small" sx={{ alignSelf: 'flex-start', borderRadius: '6px', bgcolor: 'rgba(229,255,0,0.32)', color: '#3f4900', fontWeight: 800 }} />
            <Typography variant="subtitle1" sx={{ mt: 1.1, fontWeight: 900 }}>{item.title}</Typography>
            <Typography variant="body2" sx={{ mt: 0.4, color: '#686158', flex: 1 }}>{item.desc}</Typography>
            <Button onClick={() => onPick(item.prompt)} variant="contained" size="small" sx={{ mt: 1.4, alignSelf: 'flex-start' }}>
              השתמש בתבנית
            </Button>
          </Box>
        ))}
      </Box>
    </Surface>
  );
}

function ContinueRow({ project }: { project: Project }) {
  const lastEvent = useMemo(() => {
    const events = Array.isArray(project.events) ? project.events : [];
    if (events.length === 0) return undefined;
    return events.reduce((latest, ev) => (Date.parse(ev.createdAt) > Date.parse(latest.createdAt) ? ev : latest));
  }, [project]);

  return (
    <Surface sx={{ p: { xs: 2, md: 2.4 } }}>
      <Stack direction={{ xs: 'column', md: 'row' }} spacing={2} justifyContent="space-between" alignItems={{ xs: 'flex-start', md: 'center' }}>
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Typography variant="overline" sx={{ color: '#9fb500' }}>המשך מהמקום שעצרת</Typography>
          <Stack direction="row" spacing={1.2} alignItems="center" sx={{ mt: 0.4, flexWrap: 'wrap' }}>
            <Typography variant="h5" sx={{ fontWeight: 900, minWidth: 0 }} noWrap>{project.name}</Typography>
            <StatusPill kind={statusKindFromGate(project.status)} label={project.status || 'idle'} />
          </Stack>
          <Typography variant="body2" sx={{ mt: 0.6, color: '#686158', maxWidth: 560 }}>
            {lastEvent ? lastEvent.message : project.description || project.spec?.idea || 'הפרויקט ממתין להרצה ראשונה.'}
          </Typography>
        </Box>
        <Stack direction="row" spacing={1}>
          <Button component={Link} href={`/projects/${project.id}`} variant="contained" startIcon={<OpenInNew />}>
            פתח פרויקט
          </Button>
          <Button component={Link} href={`/projects/${project.id}?action=run`} variant="outlined" startIcon={<PlayArrow />}>
            הרץ שוב
          </Button>
        </Stack>
      </Stack>
    </Surface>
  );
}

function SectionHeading({
  title, subtitle, href, hrefLabel, inline = false,
}: {
  title: string;
  subtitle?: string;
  href?: string;
  hrefLabel?: string;
  inline?: boolean;
}) {
  const inner = (
    <Stack direction="row" justifyContent="space-between" alignItems="center">
      <Box>
        <Typography variant="subtitle1" sx={{ fontWeight: 900 }}>{title}</Typography>
        {subtitle && <Typography variant="caption" sx={{ color: '#86807a' }}>{subtitle}</Typography>}
      </Box>
      {href && hrefLabel && (
        <Button component={Link} href={href} size="small" endIcon={<ArrowForward fontSize="small" />}>
          {hrefLabel}
        </Button>
      )}
    </Stack>
  );
  if (inline) return inner;
  return (
    <Box sx={{ px: 1.8, py: 1.2, borderBottom: '1px solid rgba(17,17,17,0.08)' }}>
      {inner}
    </Box>
  );
}

function UsageStat({ label, value, accent = false }: { label: string; value: string; accent?: boolean }) {
  return (
    <Box sx={{
      flex: 1,
      p: 1.2,
      borderRadius: '8px',
      bgcolor: accent ? 'rgba(229,255,0,0.22)' : '#fffaf1',
      border: '1px solid rgba(17,17,17,0.1)',
    }}>
      <Typography variant="caption" sx={{ color: '#86807a' }}>{label}</Typography>
      <Typography variant="h6" sx={{ fontWeight: 900, fontFamily: tokens.font.mono }}>{value}</Typography>
    </Box>
  );
}

function summarizeUsage(budget: UserBudget | null, plans: Plan[], projects: Project[], activityCount: number) {
  const tier = budget?.tier ?? 'free';
  const plan = plans.find((p) => p.tier === tier);
  const cap = Number(plan?.costCapUSD ?? (tier === 'team' ? 32 : tier === 'pro' ? 8 : tier === 'enterprise' ? 180 : 0.5));
  const spent = Number(budget?.spent ?? 0);
  const entries: LedgerEntry[] = Array.isArray(budget?.entries) ? budget!.entries : [];
  const points = bucketByDay(entries.map((e) => ({ createdAt: e.createdAt, costUSD: e.costUSD })), 30);

  // best-effort counts from local data
  const events = projects.flatMap((p) => (Array.isArray(p.events) ? p.events : []));
  const runs = events.filter((e) => (e.step ?? '').toLowerCase().includes('run')).length || projects.length;
  const patches = events.filter((e) => (e.step ?? '').toLowerCase().includes('patch')).length || Math.max(0, activityCount - runs);

  return { spent, cap: Math.max(cap, 0.5), points, runs, patches };
}
