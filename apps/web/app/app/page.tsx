'use client';

import { useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import {
  ArrowForward, AutoAwesome, FactCheck, OpenInNew, Paid, PlayArrow, Smartphone, Terminal,
} from '@mui/icons-material';
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
  'Build a SaaS product with auth, billing, teams, and an admin dashboard',
  'Create an internal operations tool with approvals, audit history, and reports',
  'Launch an AI product website with waitlist, pricing, and analytics',
  'Turn a raw product idea into a polished app users can actually use',
];

const featuredTemplates = [
  {
    title: 'Aiforge AI SaaS',
    desc: 'Landing pages, integrations, pricing, blog, onboarding',
    tag: 'Apps',
    prompt: 'Use the local Aiforge template as the visual foundation for an AI SaaS app with landing pages, integrations, pricing, blog, onboarding, and production launch checks.',
  },
  {
    title: 'Allstore Commerce',
    desc: 'Catalog, promo banner, cart, checkout, order states',
    tag: 'Commerce',
    prompt: 'Use the local Allstore HTML template as the foundation for a commerce storefront with catalog pages, product detail, cart, checkout, order states, and CMS-ready content.',
  },
  {
    title: 'Davies Agency',
    desc: 'Portfolio, services, process, pricing, contact flows',
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
      const p = await api.createProject({ name, description: 'Created from prompt', idea: nextIdea });
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
        <MissionControl />

        {error && !busy && (
          <ErrorBox
            title="Could not load the dashboard"
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
            <SectionHeading title="Recent activity" subtitle="Gate, patch, and deploy updates across every project" href="/app/projects" hrefLabel="All projects" />
            {loading ? (
              <Stack spacing={1} sx={{ p: 1.6 }}>
                <SkeletonCard lines={2} minHeight={56} />
                <SkeletonCard lines={2} minHeight={56} />
                <SkeletonCard lines={2} minHeight={56} />
              </Stack>
            ) : (
              <ActivityTimeline rows={activity} emptyHint="No activity yet. Run a project to start the loop." />
            )}
          </Surface>

          <Surface sx={{ p: 0, overflow: 'hidden' }}>
            <SectionHeading title="Your usage" subtitle="Current month" href="/app/settings?tab=billing" hrefLabel="Plan and billing" />
            <Box sx={{ p: 2 }}>
              <Stack direction="row" spacing={1} sx={{ mb: 1.4 }}>
                <UsageStat label="Runs" value={usage.runs.toString()} />
                <UsageStat label="Patches" value={usage.patches.toString()} />
                <UsageStat label="Spend" value={`$${usage.spent.toFixed(2)}`} accent />
              </Stack>
              <UsageSpark
                points={usage.points}
                emptyHint="No spend yet this month"
                caption={`Cap $${usage.cap.toFixed(2)}`}
              />
              <Stack direction="row" spacing={1} sx={{ mt: 1.8 }}>
                <Button component={Link} href="/app/settings?tab=billing" variant="contained" size="small" endIcon={<ArrowForward fontSize="small" />}>
                  Open billing
                </Button>
                <Button component={Link} href="/pricing" variant="outlined" size="small">
                  Compare plans
                </Button>
              </Stack>
            </Box>
          </Surface>
        </Box>

        {!loading && projects.length > 0 && (
          <Box>
            <SectionHeading title="Your projects" subtitle={`${projects.length} active`} href="/app/projects" hrefLabel="All projects" inline />
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
    <Stack
      alignItems="center"
      spacing={1.7}
      sx={{
        textAlign: 'center',
        p: { xs: 2, sm: 3, md: 4 },
        border: '1px solid rgba(226,236,248,0.12)',
        borderRadius: '8px',
        overflow: 'hidden',
        position: 'relative',
        backgroundImage: 'linear-gradient(180deg, rgba(7,9,13,0.54), rgba(7,9,13,0.92)), url(/brand/data-flow.jpg)',
        backgroundSize: 'cover',
        backgroundPosition: 'center',
        boxShadow: '0 32px 90px rgba(0,0,0,0.34)',
        '&:before': {
          content: '""',
          position: 'absolute',
          inset: 0,
          background: 'linear-gradient(90deg, rgba(83,255,189,0.1), transparent 38%, rgba(91,200,255,0.1))',
          pointerEvents: 'none',
        },
        '& > *': { position: 'relative', zIndex: 1 },
      }}
    >
      <Chip
        icon={<AutoAwesome sx={{ fontSize: 14 }} />}
        label="Finisher OS cockpit"
        sx={{
          bgcolor: 'rgba(83,255,189,0.14)',
          color: tokens.color.brand.mint,
          border: '1px solid rgba(83,255,189,0.26)',
          borderRadius: '8px',
          fontWeight: 900,
          '& .MuiChip-icon': { color: tokens.color.brand.mint },
        }}
      />
      <Typography variant="h2" sx={{
        maxWidth: 820,
        fontSize: { xs: '1.82rem', sm: '2.5rem', md: '3.35rem' },
        lineHeight: 0.96,
        textTransform: 'uppercase',
        textWrap: 'balance',
      }}>
        What needs to be finished today?
      </Typography>
      <Typography variant="body1" sx={{ maxWidth: 690, fontSize: { xs: '0.98rem', sm: '1rem' }, fontWeight: 600, color: '#c9d4df' }}>
        Describe the outcome. Ironflyer turns it into a gated build plan, reviewable patches, a live runtime, a cost ledger, and a deploy package your team can defend.
      </Typography>
      <Box sx={{ width: '100%', maxWidth: 820 }}>
        <PromptBox
          value={idea}
          onChange={setIdea}
          onSubmit={onSubmit}
          busy={busy}
          error={error}
          size="dashboard"
          cta="Build app"
          placeholder="Ask Ironflyer to build a SaaS app, internal tool, client portal, or launch site..."
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
              bgcolor: 'rgba(16,21,29,0.7)',
              color: tokens.color.text.primary,
              borderColor: 'rgba(226,236,248,0.16)',
              justifyContent: 'flex-start',
              textAlign: 'left',
              fontWeight: 800,
              fontSize: { xs: '0.88rem', sm: '0.92rem' },
              lineHeight: 1.28,
              '&:hover': {
                borderColor: 'rgba(83,255,189,0.4)',
                bgcolor: 'rgba(83,255,189,0.1)',
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

function MissionControl() {
  const items = [
    {
      icon: <FactCheck fontSize="small" />,
      label: 'Gate stack',
      value: '9 verdicts',
      text: 'Spec, UX, Architecture, Code, Lint, Tests, Security, Budget, Deploy.',
      color: tokens.color.brand.mint,
    },
    {
      icon: <Terminal fontSize="small" />,
      label: 'Runtime',
      value: 'Linux live',
      text: 'Files, terminal, preview ports, logs, and patch evidence in one workspace.',
      color: tokens.color.brand.cyan,
    },
    {
      icon: <Paid fontSize="small" />,
      label: 'Budget',
      value: 'Cap aware',
      text: 'Provider spend is visible early so the build protects margin.',
      color: tokens.color.brand.amber,
    },
    {
      icon: <Smartphone fontSize="small" />,
      label: 'Mobile',
      value: 'Approve fast',
      text: 'Review blockers, risk, preview status, and next action from the phone.',
      color: '#d7b3ff',
    },
  ];

  return (
    <Box sx={{
      display: 'grid',
      gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, 1fr)', lg: 'repeat(4, 1fr)' },
      gap: 1.2,
    }}>
      {items.map((item) => (
        <Surface
          key={item.label}
          sx={{
            p: 1.6,
            minHeight: 148,
            display: 'flex',
            flexDirection: 'column',
            justifyContent: 'space-between',
            background: `linear-gradient(180deg, ${item.color}18, rgba(16,21,29,0.86) 56%)`,
          }}
        >
          <Stack direction="row" justifyContent="space-between" alignItems="center">
            <Stack direction="row" spacing={0.8} alignItems="center">
              <Box sx={{
                width: 30,
                height: 30,
                borderRadius: '8px',
                display: 'grid',
                placeItems: 'center',
                bgcolor: `${item.color}1f`,
                color: item.color,
              }}>
                {item.icon}
              </Box>
              <Typography variant="caption" sx={{ color: tokens.color.text.secondary, fontWeight: 900, textTransform: 'uppercase' }}>
                {item.label}
              </Typography>
            </Stack>
          </Stack>
          <Box sx={{ mt: 1.6 }}>
            <Typography sx={{ fontFamily: tokens.font.display, fontSize: '1.45rem', lineHeight: 1, textTransform: 'uppercase' }}>
              {item.value}
            </Typography>
            <Typography variant="body2" sx={{ mt: 0.8, color: tokens.color.text.secondary }}>
              {item.text}
            </Typography>
          </Box>
        </Surface>
      ))}
    </Box>
  );
}

function WelcomeCard({ onPick, templates }: { onPick: (prompt: string) => void; templates: typeof featuredTemplates }) {
  return (
    <Surface sx={{ p: { xs: 2.2, md: 3 } }}>
      <Stack direction={{ xs: 'column', md: 'row' }} spacing={2} justifyContent="space-between" alignItems={{ xs: 'flex-start', md: 'center' }}>
        <Box sx={{ maxWidth: 520 }}>
          <Typography variant="overline" sx={{ color: tokens.color.brand.mint }}>Welcome to Ironflyer</Typography>
          <Typography variant="h5" sx={{ mt: 0.4, fontWeight: 900 }}>Let’s build your first finished product</Typography>
          <Typography variant="body2" sx={{ mt: 0.6, color: tokens.color.text.secondary }}>
            Pick a starter template or write your idea in the prompt box above. Ironflyer will move it through spec,
            architecture, code, tests, security, and deploy gates until the product is ready to ship.
          </Typography>
        </Box>
        <Button component={Link} href="/app/resources" variant="outlined" endIcon={<ArrowForward fontSize="small" />}>
          Explore templates
        </Button>
      </Stack>

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' }, gap: 1.4, mt: 2.4 }}>
        {templates.map((item) => (
          <Box
            key={item.title}
            sx={{
              p: 2,
              borderRadius: '8px',
              border: '1px solid rgba(226,236,248,0.12)',
              bgcolor: 'rgba(7,9,13,0.42)',
              display: 'flex',
              flexDirection: 'column',
              transition: 'transform 200ms',
              '&:hover': { transform: 'translateY(-2px)', borderColor: 'rgba(83,255,189,0.32)' },
            }}
          >
            <Chip label={item.tag} size="small" sx={{ alignSelf: 'flex-start', borderRadius: '6px', bgcolor: 'rgba(83,255,189,0.14)', color: tokens.color.brand.mint, fontWeight: 800 }} />
            <Typography variant="subtitle1" sx={{ mt: 1.1, fontWeight: 900 }}>{item.title}</Typography>
            <Typography variant="body2" sx={{ mt: 0.4, color: tokens.color.text.secondary, flex: 1 }}>{item.desc}</Typography>
            <Button onClick={() => onPick(item.prompt)} variant="contained" size="small" sx={{ mt: 1.4, alignSelf: 'flex-start' }}>
              Use template
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
          <Typography variant="overline" sx={{ color: tokens.color.brand.mint }}>Continue where you left off</Typography>
          <Stack direction="row" spacing={1.2} alignItems="center" sx={{ mt: 0.4, flexWrap: 'wrap' }}>
            <Typography variant="h5" sx={{ fontWeight: 900, minWidth: 0 }} noWrap>{project.name}</Typography>
            <StatusPill kind={statusKindFromGate(project.status)} label={project.status || 'idle'} />
          </Stack>
          <Typography variant="body2" sx={{ mt: 0.6, color: tokens.color.text.secondary, maxWidth: 560 }}>
            {lastEvent ? lastEvent.message : project.description || project.spec?.idea || 'This project is waiting for its first run.'}
          </Typography>
        </Box>
        <Stack direction="row" spacing={1}>
          <Button component={Link} href={`/projects/${project.id}`} variant="contained" startIcon={<OpenInNew />}>
            Open project
          </Button>
          <Button component={Link} href={`/projects/${project.id}?action=run`} variant="outlined" startIcon={<PlayArrow />}>
            Run again
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
        {subtitle && <Typography variant="caption" sx={{ color: tokens.color.text.secondary }}>{subtitle}</Typography>}
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
    <Box sx={{ px: 1.8, py: 1.2, borderBottom: '1px solid rgba(226,236,248,0.1)' }}>
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
      bgcolor: accent ? 'rgba(83,255,189,0.14)' : 'rgba(7,9,13,0.42)',
      border: '1px solid rgba(226,236,248,0.1)',
    }}>
      <Typography variant="caption" sx={{ color: tokens.color.text.secondary }}>{label}</Typography>
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
