'use client';

import { use, useEffect, useRef, useState } from 'react';
import {
  Box, Button, Card, CardContent, Chip, Divider, IconButton, LinearProgress,
  Stack, Tab, Tabs, TextField, Tooltip, Typography,
} from '@mui/material';
import KeyboardBackspaceIcon from '@mui/icons-material/KeyboardBackspace';
import {
  AutoAwesome, Code, DesktopWindows, History, Palette, RocketLaunch, Settings,
  Share, Smartphone, Visibility,
} from '@mui/icons-material';
import {
  api, BrainstormOutcome, ChatDelta, ExecutionEvent, GateState, LedgerEntry, Project,
  streamChat, UserBudget, VaultSnapshot,
} from '../../../lib/api';
import { runtime, Workspace as WS } from '../../../lib/runtime';
import { tokens } from '../../../lib/theme';
import { Terminal } from './Terminal';
import { WorkspaceFiles } from './Workspace';
import { GitHubPanel } from './GitHubPanel';
import { ChatComposer, type ComposerEffort, type ComposerMode } from './ChatComposer';
import { RequireAuth, useAuth } from '../../auth-context';

// GATE_ORDER mirrors finisher.DefaultGates() on the orchestrator. Keep in
// sync when the gate list changes — the backend treats the gate keys as
// authoritative, the UI just renders them in this order with friendlier
// labels.
const GATE_ORDER: { key: string; label: string }[] = [
  { key: 'spec', label: 'Spec' },
  { key: 'ux', label: 'UX' },
  { key: 'arch', label: 'Architecture' },
  { key: 'code', label: 'Code' },
  { key: 'lint', label: 'Lint' },
  { key: 'test', label: 'Tests' },
  { key: 'security', label: 'Security' },
  { key: 'deploy', label: 'Deploy' },
];

const ROLES = ['planner', 'uxer', 'architect', 'coder', 'reviewer', 'tester', 'security', 'deployer'] as const;

interface ChatTurn {
  id: string;
  role: string;
  status: 'streaming' | 'done' | 'error';
  text: string;
  thinking: string;
  provider?: string;
  model?: string;
  usage?: { inputTokens: number; outputTokens: number; costUSD: number; cacheReadTokens?: number; cacheCreationTokens?: number };
  error?: string;
}

export default function ProjectPage({ params }: { params: Promise<{ id: string }> }) {
  return (
    <RequireAuth>
      <ProjectPageInner params={params} />
    </RequireAuth>
  );
}

function ProjectPageInner({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const [project, setProject] = useState<Project | null>(null);
  const [events, setEvents] = useState<ExecutionEvent[]>([]);
  const [mode, setMode] = useState(0);
  const [prompt, setPrompt] = useState('');
  const [role, setRole] = useState<string>('planner');
  const [running, setRunning] = useState(false);
  const [turns, setTurns] = useState<ChatTurn[]>([]);
  const [streaming, setStreaming] = useState(false);
  const [vault, setVault] = useState<VaultSnapshot | null>(null);
  const [budget, setBudget] = useState<UserBudget | null>(null);
  const [brainstormOut, setBrainstormOut] = useState<BrainstormOutcome | null>(null);
  const [workspace, setWorkspace] = useState<WS | null>(null);
  const esRef = useRef<EventSource | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const scrollRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    void load();
    const es = new EventSource(api.streamURL(id));
    es.addEventListener('execution', (e) => {
      try {
        const evt = JSON.parse((e as MessageEvent).data) as ExecutionEvent;
        setEvents((prev) => [...prev.slice(-100), evt]);
      } catch {}
    });
    es.onerror = () => {};
    esRef.current = es;
    return () => { es.close(); abortRef.current?.abort(); };
  }, [id]);

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: 'smooth' });
  }, [turns]);

  async function load() {
    try {
      const [p, v, b] = await Promise.all([
        api.getProject(id),
        api.vault(),
        api.myBudget(),
      ]);
      setProject(p);
      setEvents(p.events ?? []);
      setVault(v);
      setBudget(b);
    } catch {}
  }

  async function runFinisher() {
    setRunning(true);
    try { await api.runFinisher(id); await load(); }
    finally { setRunning(false); }
  }

  async function sendPrompt(effort: ComposerEffort = 'economy') {
    if (!prompt.trim() || streaming) return;
    const goal = prompt;
    setPrompt('');
    setStreaming(true);

    const turnDraft: ChatTurn = { id: crypto.randomUUID(), role, status: 'streaming', text: '', thinking: '' };
    setTurns((t) => [...t, { ...turnDraft, role: 'user', text: goal, status: 'done' }, turnDraft]);

    abortRef.current = new AbortController();
    await streamChat(id, { prompt: goal, role, effort }, (d) => {
      setTurns((curr) => {
        const next = [...curr];
        const last = next[next.length - 1];
        if (!last) return curr;
        applyDelta(last, d);
        return next;
      });
    }, abortRef.current.signal);
    setStreaming(false);
    void refreshBudget();
  }

  async function refreshBudget() {
    try {
      const [v, b] = await Promise.all([api.vault(), api.myBudget()]);
      setVault(v); setBudget(b);
    } catch {}
  }

  async function runBrainstorm() {
    if (!prompt.trim()) return;
    setBrainstormOut(null);
    try {
      const out = await api.brainstorm(id, { goal: prompt, role });
      setBrainstormOut(out);
      setMode(6);
    } catch {}
  }

  if (!project) {
    return <Box sx={{ p: 4 }}><Typography color="text.secondary">Loading…</Typography></Box>;
  }

  return (
    <Box sx={{ minHeight: '100vh', bgcolor: tokens.color.bg.alabaster, overflow: 'hidden', color: tokens.color.text.inverse }}>
      <ProjectHeader p={project} running={running} onRun={runFinisher} />

      <Box sx={{
        display: 'grid',
        gridTemplateColumns: { xs: '1fr', lg: '340px minmax(0, 1fr) 280px' },
        gap: 1,
        p: { xs: 1, lg: 1.4 },
        height: { xs: 'auto', lg: 'calc(100vh - 58px)' },
        minHeight: { xs: 'calc(100vh - 58px)', lg: 'auto' },
      }}>
        {/* LEFT: streaming chat */}
        <Card sx={panelSx}>
          <CardContent sx={{ pb: 1, px: 1.6, pt: 1.5 }}>
            <Stack direction="row" justifyContent="space-between" alignItems="center">
              <Typography variant="overline" color="text.secondary">Agent chat</Typography>
              <RoleSelector value={role} onChange={setRole} />
            </Stack>
          </CardContent>
          <Box ref={scrollRef} sx={{ flex: 1, minHeight: { xs: 190, lg: 0 }, overflowY: 'auto', px: 1.5 }}>
            <ChatTimeline turns={turns} />
          </Box>
          <Divider />
          <ChatComposer
            value={prompt}
            onChange={setPrompt}
            streaming={streaming}
            onAbort={() => abortRef.current?.abort()}
            onSend={(m: ComposerMode, eff: ComposerEffort) => {
              if (m === 'plan') void runBrainstorm();
              else void sendPrompt(eff);
            }}
          />
        </Card>

        {/* CENTER: workspace tabs */}
        <Card sx={panelSx}>
          <Tabs
            value={mode}
            onChange={(_, v) => setMode(v)}
            variant="scrollable"
            scrollButtons="auto"
            allowScrollButtonsMobile
            sx={{
              px: 1,
              minHeight: 46,
              borderBottom: `1px solid ${tokens.color.border.subtle}`,
              '& .MuiTab-root': {
                minHeight: 46,
                minWidth: { xs: 78, md: 86 },
                px: 1,
                fontSize: 13,
              },
            }}
          >
            <Tab icon={<Visibility fontSize="small" />} iconPosition="start" label="Preview" />
            <Tab icon={<Code fontSize="small" />} iconPosition="start" label="Files" />
            <Tab icon={<DesktopWindows fontSize="small" />} iconPosition="start" label="IDE" />
            <Tab label="Term" />
            <Tab icon={<Palette fontSize="small" />} iconPosition="start" label="Design" />
            <Tab icon={<RocketLaunch fontSize="small" />} iconPosition="start" label="Deploy" />
            <Tab icon={<AutoAwesome fontSize="small" />} iconPosition="start" label="Plan" />
            <Tab icon={<History fontSize="small" />} iconPosition="start" label="Versions" />
            <Tab icon={<Settings fontSize="small" />} iconPosition="start" label="Config" />
          </Tabs>
          <Box sx={{ flex: 1, minHeight: { xs: 460, lg: 0 }, overflow: 'auto', p: 1.2 }}>
            {mode === 0 && <PreviewPane p={project} />}
            {mode === 1 && <WorkspaceFiles workspace={workspace} onWorkspaceChange={setWorkspace} projectId={id} />}
            {mode === 2 && <IDEPane workspace={workspace} />}
            {mode === 3 && <Terminal workspaceId={workspace?.id ?? null} />}
            {mode === 4 && <DesignPane />}
            {mode === 5 && <DeployPane p={project} workspaceId={workspace?.id ?? null} onGitHubLinked={load} />}
            {mode === 6 && <BrainstormPane out={brainstormOut} />}
            {mode === 7 && <VersionsPane p={project} events={events} />}
            {mode === 8 && <ProjectSettingsPane p={project} />}
          </Box>
        </Card>

        {/* RIGHT: gates + budget */}
        <Stack spacing={1} sx={{ overflowY: 'auto', display: { xs: 'none', lg: 'flex' } }}>
          <Card sx={panelSx}>
            <CardContent sx={{ p: 1.6 }}>
              <Typography variant="overline" color="text.secondary">Finisher Gates</Typography>
              <Stack spacing={1} sx={{ mt: 1 }}>
                {GATE_ORDER.map(({ key, label }) => (
                  <GateRow key={key} state={project.gates[key as keyof typeof project.gates] as GateState | undefined} label={label} />
                ))}
              </Stack>
            </CardContent>
          </Card>

          <BudgetCard vault={vault} budget={budget} />

          <ActivityCard events={events} />

          <Card sx={panelSx}>
            <CardContent sx={{ p: 1.6 }}>
              <Typography variant="overline" color="text.secondary">Build context</Typography>
              <Stack spacing={0.5} sx={{ mt: 1 }}>
                <RouteLine label="Mode" provider={role} />
                <RouteLine label="Files" provider={`${project.files.length}`} />
                <RouteLine label="Events" provider={`${events.length}`} />
                <RouteLine label="Access" provider="Workspace" />
                <RouteLine label="Runtime" provider={workspace ? 'Attached' : 'Not attached'} />
              </Stack>
            </CardContent>
          </Card>
        </Stack>
      </Box>
    </Box>
  );
}

function applyDelta(turn: ChatTurn, d: ChatDelta) {
  switch (d.kind) {
    case 'start':    turn.provider = d.provider; turn.model = d.model; break;
    case 'text':     turn.text += d.text; break;
    case 'thinking': turn.thinking += d.text; break;
    case 'done':
      turn.status = 'done';
      turn.usage = d.usage as ChatTurn['usage'];
      turn.provider = d.provider; turn.model = d.model;
      break;
    case 'error':    turn.status = 'error'; turn.error = d.error; break;
  }
}

function ProjectHeader({ p, running, onRun }: { p: Project; running: boolean; onRun: () => void }) {
  const passed = Object.values(p.gates).filter((g) => g.status === 'passed').length;
  const total = Object.keys(p.gates).length;
  return (
    <Box sx={{
      px: 1.5, py: 0.8, minHeight: 58, borderBottom: '1px solid rgba(17,17,17,0.12)',
      display: 'flex',
      flexDirection: { xs: 'column', sm: 'row' },
      alignItems: { xs: 'stretch', sm: 'center' },
      justifyContent: 'space-between',
      gap: { xs: 0.8, sm: 2 },
      bgcolor: 'rgba(248,244,236,0.94)',
      color: tokens.color.text.inverse,
    }}>
      <Stack direction="row" alignItems="center" spacing={2} sx={{ minWidth: 0 }}>
        <IconButton size="small" href="/app" sx={{ color: '#4a453e' }}>
          <KeyboardBackspaceIcon fontSize="small" />
        </IconButton>
        <Box sx={{ minWidth: 0 }}>
          <Typography variant="subtitle1" sx={{ fontWeight: 900 }} noWrap>{p.name}</Typography>
          <Typography variant="caption" sx={{ color: '#686158' }} noWrap>
            {p.spec.idea || p.description}
          </Typography>
        </Box>
      </Stack>
      <Stack direction="row" alignItems="center" spacing={0.8} sx={{ justifyContent: { xs: 'space-between', sm: 'flex-end' }, minWidth: 0 }}>
        <Chip label={`Gates ${passed}/${total}`} size="small" sx={{ bgcolor: '#fffaf1', color: tokens.color.text.inverse, border: '1px solid rgba(17,17,17,0.12)' }} />
        <Button variant="outlined" size="small" startIcon={<Share />} sx={headerOutlineButtonSx}>Share</Button>
        <Button variant="outlined" size="small" sx={{ ...headerOutlineButtonSx, display: { xs: 'none', sm: 'inline-flex' } }}>Export</Button>
        <Button variant="contained" size="small" onClick={onRun} disabled={running}>
          {running ? 'Running...' : 'Publish'}
        </Button>
      </Stack>
    </Box>
  );
}

function RoleSelector({ value, onChange }: { value: string; onChange: (r: string) => void }) {
  return (
    <Box>
      <select value={value} onChange={(e) => onChange(e.target.value)}
        style={{
          background: tokens.color.bg.inset, color: tokens.color.text.primary,
          border: `1px solid ${tokens.color.border.subtle}`, borderRadius: 8,
          padding: '4px 8px', fontFamily: tokens.font.family, fontSize: 12,
        }}>
        {ROLES.map((r) => <option key={r} value={r}>{r}</option>)}
      </select>
    </Box>
  );
}

function ChatTimeline({ turns }: { turns: ChatTurn[] }) {
  if (turns.length === 0) {
    return (
      <Stack spacing={1.1} sx={{ pt: 1 }}>
        {[
          'Describe a feature or bug to fix.',
          'Attach context from the + menu on the dashboard.',
          'Switch roles when you need planning, UX, code, tests, or deploy work.',
        ].map((item) => (
          <Box key={item} sx={{ p: 1.2, borderRadius: 1.4, bgcolor: tokens.color.bg.inset }}>
            <Typography variant="body2" color="text.secondary">{item}</Typography>
          </Box>
        ))}
      </Stack>
    );
  }
  return (
    <Stack spacing={1.2} sx={{ py: 1 }}>
      {turns.map((t) => (
        <Box key={t.id} sx={{
          p: 1.1, borderRadius: 1.4,
          bgcolor: t.role === 'user' ? tokens.color.bg.surfaceHover : tokens.color.bg.inset,
          border: t.status === 'streaming' ? `1px solid ${tokens.color.accent.lime}` : 'none',
        }}>
          <Stack direction="row" justifyContent="space-between" alignItems="baseline">
            <Typography variant="caption" sx={{ fontWeight: 700, letterSpacing: '0.08em', textTransform: 'uppercase', color: t.role === 'user' ? tokens.color.accent.sky : tokens.color.accent.lime }}>
              {t.role}
              {t.provider && t.role !== 'user' && (
                <Box component="span" sx={{ ml: 1, color: tokens.color.text.muted, fontWeight: 500, textTransform: 'none', letterSpacing: 0 }}>
                  · {t.provider}/{t.model}
                </Box>
              )}
            </Typography>
            {t.usage && (
              <Tooltip title={`in ${t.usage.inputTokens} / out ${t.usage.outputTokens}${t.usage.cacheReadTokens ? ` · cache ${t.usage.cacheReadTokens}` : ''}`}>
                <Typography variant="caption" color="text.secondary">
                  ${Number(t.usage.costUSD ?? 0).toFixed(5)}
                </Typography>
              </Tooltip>
            )}
          </Stack>
          {t.thinking && (
            <Box sx={{ mt: 0.5, opacity: 0.6, fontStyle: 'italic', fontSize: 12, whiteSpace: 'pre-wrap' }}>
              {t.thinking}
            </Box>
          )}
          <Box sx={{ mt: 0.5, whiteSpace: 'pre-wrap', fontSize: 13, lineHeight: 1.5 }}>
            {t.text}
            {t.status === 'streaming' && <Box component="span" sx={{ color: tokens.color.accent.lime, animation: 'blink 1s steps(2) infinite' }}>▍</Box>}
          </Box>
          {t.error && <Typography variant="caption" color="error">{t.error}</Typography>}
        </Box>
      ))}
      <style>{`@keyframes blink { 50% { opacity: 0; } }`}</style>
    </Stack>
  );
}

function GateRow({ state, label }: { state?: GateState; label: string }) {
  const status = state?.status ?? 'pending';
  const color = ({
    passed: tokens.color.accent.success,
    failed: tokens.color.accent.danger,
    repaired: tokens.color.accent.warning,
    running: tokens.color.accent.sky,
    blocked: tokens.color.accent.coral,
    pending: tokens.color.text.muted,
  } as Record<string, string>)[status] ?? tokens.color.text.muted;
  return (
    <Box sx={{
      display: 'flex', alignItems: 'center', justifyContent: 'space-between',
      px: 1.2, py: 0.75, bgcolor: tokens.color.bg.inset, borderRadius: '8px',
    }}>
      <Stack direction="row" alignItems="center" spacing={1.5}>
        <Box sx={{ width: 8, height: 8, borderRadius: '50%', bgcolor: color }} />
        <Typography variant="body2">{label}</Typography>
      </Stack>
      <Typography variant="caption" sx={{ color, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.08em' }}>
        {status}
      </Typography>
    </Box>
  );
}

function RouteLine({ label, provider }: { label: string; provider: string }) {
  return (
    <Stack direction="row" justifyContent="space-between">
      <Typography variant="body2" color="text.secondary">{label}</Typography>
      <Typography variant="body2" sx={{ fontFamily: tokens.font.mono }}>{provider}</Typography>
    </Stack>
  );
}

function BudgetCard({ vault, budget }: { vault: VaultSnapshot | null; budget: UserBudget | null }) {
  if (!vault || !budget) return null;
  const spent = Number(budget.spent || 0);
  // approximate cap by tier — could fetch from /budget/plans
  const tierCap: Record<string, number> = { free: 0.5, pro: 8, team: 32, enterprise: 180 };
  const cap = tierCap[budget.tier] ?? 0;
  const pct = cap > 0 ? Math.min(100, (spent / cap) * 100) : 0;
  return (
    <Card sx={panelSx}>
      <CardContent sx={{ p: 1.6 }}>
        <Typography variant="overline" color="text.secondary">Budget · {budget.tier}</Typography>
        <Stack spacing={0.5} sx={{ mt: 1 }}>
          <Stack direction="row" justifyContent="space-between">
            <Typography variant="body2" color="text.secondary">Spent this period</Typography>
            <Typography variant="body2" sx={{ fontFamily: tokens.font.mono }}>
              ${spent.toFixed(4)} / ${cap.toFixed(2)}
            </Typography>
          </Stack>
          <LinearProgress variant="determinate" value={pct} sx={{
            height: 6, borderRadius: 3, bgcolor: tokens.color.bg.inset,
            '& .MuiLinearProgress-bar': { bgcolor: pct > 85 ? tokens.color.accent.danger : tokens.color.accent.lime },
          }} />
        </Stack>
        <Divider sx={{ my: 1.5, borderColor: tokens.color.border.subtle }} />
        <Stack spacing={0.5}>
          <Stack direction="row" justifyContent="space-between">
            <Typography variant="body2" color="text.secondary">Org margin</Typography>
            <Typography variant="body2" sx={{ fontFamily: tokens.font.mono, color: tokens.color.accent.success }}>
              ${Number(vault.margin).toFixed(2)}
            </Typography>
          </Stack>
          <Stack direction="row" justifyContent="space-between">
            <Typography variant="caption" color="text.secondary">Revenue</Typography>
            <Typography variant="caption" sx={{ fontFamily: tokens.font.mono }}>${Number(vault.revenue).toFixed(2)}</Typography>
          </Stack>
          <Stack direction="row" justifyContent="space-between">
            <Typography variant="caption" color="text.secondary">Provider cost</Typography>
            <Typography variant="caption" sx={{ fontFamily: tokens.font.mono }}>${Number(vault.providerCost).toFixed(4)}</Typography>
          </Stack>
        </Stack>
        {topModelSpend(budget.entries).length > 0 && (
          <>
            <Divider sx={{ my: 1.5, borderColor: tokens.color.border.subtle }} />
            <Typography variant="caption" sx={{ color: tokens.color.text.muted, textTransform: 'uppercase', letterSpacing: '0.06em' }}>
              Top models
            </Typography>
            <Stack spacing={0.3} sx={{ mt: 0.6 }}>
              {topModelSpend(budget.entries).map((row) => (
                <Stack key={row.key} direction="row" justifyContent="space-between" alignItems="center">
                  <Typography variant="caption" sx={{ fontFamily: tokens.font.mono, color: tokens.color.text.primary,
                    overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 180 }} title={row.key}>
                    {row.key}
                  </Typography>
                  <Typography variant="caption" sx={{ fontFamily: tokens.font.mono, color: tokens.color.text.muted }}>
                    ${row.usd.toFixed(4)}
                  </Typography>
                </Stack>
              ))}
            </Stack>
          </>
        )}
      </CardContent>
    </Card>
  );
}

// topModelSpend aggregates ledger entries by `provider/model` and returns
// the three most expensive — making the v0-style per-model price exposure
// visible without forcing users to read a separate ledger. Empty when the
// user hasn't spent anything yet (free tier or fresh signup).
function topModelSpend(entries: LedgerEntry[]): { key: string; usd: number }[] {
  if (!entries || entries.length === 0) return [];
  const byKey: Record<string, number> = {};
  for (const e of entries) {
    const k = `${e.provider}/${e.model}`;
    byKey[k] = (byKey[k] ?? 0) + Number(e.costUSD || 0);
  }
  return Object.entries(byKey)
    .map(([key, usd]) => ({ key, usd }))
    .sort((a, b) => b.usd - a.usd)
    .slice(0, 3);
}

// ActivityCard surfaces the SSE execution stream as a compact timeline.
// Each event is a one-liner: timestamp · gate/step · short message. Lives in
// the right column so the user can watch the finisher loop run without
// switching tabs.
function ActivityCard({ events }: { events: ExecutionEvent[] }) {
  if (events.length === 0) return null;
  const recent = events.slice(-12).reverse();
  return (
    <Card sx={panelSx}>
      <CardContent sx={{ p: 1.6 }}>
        <Stack direction="row" justifyContent="space-between" alignItems="center">
          <Typography variant="overline" color="text.secondary">Activity</Typography>
          <Typography variant="caption" sx={{ color: tokens.color.text.muted, fontFamily: tokens.font.mono }}>
            {events.length} events
          </Typography>
        </Stack>
        <Stack spacing={0.6} sx={{ mt: 1 }}>
          {recent.map((e) => (
            <Stack key={e.id} direction="row" spacing={1} alignItems="flex-start">
              <Box sx={{
                mt: 0.7, width: 6, height: 6, borderRadius: '50%', flexShrink: 0,
                bgcolor: activityColor(e.status),
              }} />
              <Box sx={{ minWidth: 0, flex: 1 }}>
                <Stack direction="row" spacing={0.8} alignItems="baseline">
                  <Typography variant="caption" sx={{ color: tokens.color.text.primary, fontWeight: 700 }}>
                    {e.gate ?? e.step}
                  </Typography>
                  <Typography variant="caption" sx={{ color: tokens.color.text.muted, fontFamily: tokens.font.mono }}>
                    {formatTime(e.createdAt)}
                  </Typography>
                </Stack>
                <Typography variant="caption" sx={{
                  color: tokens.color.text.muted, display: 'block',
                  whiteSpace: 'nowrap', textOverflow: 'ellipsis', overflow: 'hidden',
                }} title={e.message}>
                  {e.message}
                </Typography>
              </Box>
            </Stack>
          ))}
        </Stack>
      </CardContent>
    </Card>
  );
}

function activityColor(status: string): string {
  switch (status) {
    case 'done':    return tokens.color.accent.success;
    case 'running': return tokens.color.accent.lime;
    case 'error':   return tokens.color.accent.danger;
    default:        return tokens.color.text.muted;
  }
}

function formatTime(iso: string): string {
  try {
    const d = new Date(iso);
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  } catch {
    return '';
  }
}

function BrainstormPane({ out }: { out: BrainstormOutcome | null }) {
  if (!out) return <Typography variant="body2" color="text.secondary">
    Hit <b>Brainstorm</b> in the chat panel. The Strategist classifies the goal as direct / brainstorm / debate / research and runs the right protocol.
  </Typography>;
  return (
    <Stack spacing={2}>
      <Box>
        <Typography variant="overline" color="text.secondary">Strategist plan</Typography>
        <Stack direction="row" spacing={1} sx={{ mt: 0.5 }}>
          <Chip label={`mode: ${out.plan.mode}`} size="small" />
          {out.plan.rounds ? <Chip label={`${out.plan.rounds} rounds`} size="small" /> : null}
          {out.plan.roles.map((r) => <Chip key={r} label={r} size="small" />)}
        </Stack>
        <Typography variant="caption" color="text.secondary" sx={{ mt: 1, display: 'block' }}>
          {out.plan.reason}
        </Typography>
      </Box>
      <Box>
        <Typography variant="overline" color="text.secondary">Synthesis</Typography>
        <Box sx={{
          mt: 1, p: 2, bgcolor: tokens.color.bg.inset, borderRadius: 2,
          whiteSpace: 'pre-wrap', fontSize: 13, lineHeight: 1.6,
        }}>
          {out.outcome.synthesis}
        </Box>
      </Box>
      {out.outcome.proposals && out.outcome.proposals.length > 0 && (
        <Box>
          <Typography variant="overline" color="text.secondary">Proposals (scored)</Typography>
          <Stack spacing={1} sx={{ mt: 1 }}>
            {out.outcome.proposals.map((p, i) => (
              <Box key={i} sx={{ p: 1.5, bgcolor: tokens.color.bg.inset, borderRadius: 2 }}>
                <Stack direction="row" justifyContent="space-between">
                  <Typography variant="body2" sx={{ fontWeight: 700 }}>{p.role} · score {p.score}</Typography>
                  <Typography variant="caption" color="text.secondary">{p.provider} · ${p.costUSD.toFixed(5)}</Typography>
                </Stack>
                <Typography variant="body2" sx={{ mt: 0.5, whiteSpace: 'pre-wrap', fontSize: 12, color: tokens.color.text.secondary }}>
                  {p.output.slice(0, 400)}{p.output.length > 400 ? '…' : ''}
                </Typography>
              </Box>
            ))}
          </Stack>
        </Box>
      )}
    </Stack>
  );
}

function PreviewPane({ p }: { p: Project }) {
  const [device, setDevice] = useState<'desktop' | 'mobile'>('desktop');
  return (
    <Box sx={{ height: '100%', display: 'flex', flexDirection: 'column', gap: 1 }}>
      <Stack direction="row" alignItems="center" justifyContent="space-between">
        <Stack direction="row" spacing={0.6}>
          <IconButton size="small" onClick={() => setDevice('desktop')} sx={{ color: device === 'desktop' ? tokens.color.accent.lime : tokens.color.text.secondary }}><DesktopWindows fontSize="small" /></IconButton>
          <IconButton size="small" onClick={() => setDevice('mobile')} sx={{ color: device === 'mobile' ? tokens.color.accent.lime : tokens.color.text.secondary }}><Smartphone fontSize="small" /></IconButton>
        </Stack>
        <Chip label={`Status: ${p.status}`} size="small" sx={{ borderRadius: 1 }} />
      </Stack>
      <Box sx={{
        flex: 1,
        minHeight: 420,
        display: 'grid',
        placeItems: 'center',
        borderRadius: '8px',
        bgcolor: '#1b1b19',
        border: `1px solid ${tokens.color.border.subtle}`,
        overflow: 'hidden',
      }}>
        <Box sx={{
          width: device === 'desktop' ? '92%' : 310,
          maxWidth: 900,
          aspectRatio: device === 'desktop' ? '16 / 10' : '9 / 16',
          borderRadius: device === 'desktop' ? '8px' : '18px',
          bgcolor: tokens.color.bg.alabaster,
          color: tokens.color.text.inverse,
          overflow: 'hidden',
          boxShadow: '0 18px 80px rgba(0,0,0,0.45)',
        }}>
          <Box sx={{ height: 34, bgcolor: '#0f0f0f', display: 'flex', alignItems: 'center', px: 1.2, gap: 0.6 }}>
            {['#ff5f57', '#ffbd2e', '#28c840'].map((color) => <Box key={color} sx={{ width: 8, height: 8, borderRadius: '50%', bgcolor: color }} />)}
            <Typography variant="caption" sx={{ ml: 1, color: '#aaa' }}>preview.ironflyer.local</Typography>
          </Box>
          <Box sx={{ p: { xs: 2, md: 3 } }}>
            <Typography sx={{ fontFamily: tokens.font.display, fontSize: device === 'desktop' ? 34 : 22, lineHeight: 1, textTransform: 'uppercase' }}>
              {p.name}
            </Typography>
            <Typography variant="body2" sx={{ mt: 1, maxWidth: 460, color: '#555' }}>
              {p.description || p.spec.idea || 'A live app preview will render from the runtime workspace.'}
            </Typography>
            <Box sx={{ mt: 2, display: 'grid', gridTemplateColumns: device === 'desktop' ? 'repeat(3, 1fr)' : '1fr', gap: 1 }}>
              {['Spec', 'Build', 'Deploy'].map((item) => (
                <Box key={item} sx={{ p: 1.3, borderRadius: '8px', bgcolor: '#fffdf7', border: '1px solid rgba(17,17,17,0.08)' }}>
                  <Typography variant="subtitle2">{item}</Typography>
                  <Typography variant="caption" sx={{ color: '#666' }}>Ready for review</Typography>
                </Box>
              ))}
            </Box>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}

// IDEPane embeds the per-user Ironflyer-branded code-server instance. The
// runtime sets `ideUrl` only when the Docker driver provisions a container
// (one IDE port per workspace); the Mock driver runs on the host and has no
// browser-accessible IDE, so we surface a hint instead of a broken iframe.
function IDEPane({ workspace }: { workspace: WS | null }) {
  if (!workspace) {
    return (
      <Stack spacing={1.5} sx={{ alignItems: 'flex-start' }}>
        <Typography variant="overline" color="text.secondary">Ironflyer IDE</Typography>
        <Typography variant="body2" color="text.secondary">
          Provision a workspace from the <b>Files</b> tab to launch a private cloud IDE.
          We boot a per-project container running our branded VS Code build.
        </Typography>
      </Stack>
    );
  }
  if (!workspace.ideUrl) {
    return (
      <Stack spacing={1.5} sx={{ alignItems: 'flex-start' }}>
        <Typography variant="overline" color="text.secondary">Ironflyer IDE</Typography>
        <Typography variant="body2" color="text.secondary">
          This workspace runs on the <Chip size="small" label={workspace.driver} sx={{ mx: 0.5 }} />
          driver, which doesn’t expose a browser IDE. Switch the runtime to
          <code style={{ marginInline: 6 }}>docker</code> to get an embedded VS Code window here.
        </Typography>
      </Stack>
    );
  }
  return (
    <Box sx={{ position: 'relative', height: '100%', minHeight: 540,
               border: `1px solid ${tokens.color.border.subtle}`, borderRadius: 1.4,
               overflow: 'hidden', bgcolor: '#0d0e0f' }}>
      <Stack direction="row" alignItems="center" justifyContent="space-between"
             sx={{ px: 1.4, py: 0.8, borderBottom: `1px solid ${tokens.color.border.subtle}` }}>
        <Stack direction="row" alignItems="center" spacing={1}>
          <Box sx={{ width: 8, height: 8, borderRadius: '50%', bgcolor: tokens.color.accent.lime }} />
          <Typography variant="caption" sx={{ color: tokens.color.text.muted, fontFamily: tokens.font.mono }}>
            ironflyer-code @ {workspace.id}
          </Typography>
        </Stack>
        <Tooltip title="Open in new tab">
          <IconButton size="small" component="a" href={workspace.ideUrl} target="_blank" rel="noopener noreferrer"
                      sx={{ color: tokens.color.text.muted }}>
            <Share fontSize="small" />
          </IconButton>
        </Tooltip>
      </Stack>
      <Box
        component="iframe"
        src={workspace.ideUrl}
        title="Ironflyer cloud IDE"
        // The runtime owns its own auth + sandbox; allow same-origin so VS Code
        // can hold cookies for itself, and forms so submission works.
        sandbox="allow-scripts allow-same-origin allow-forms allow-downloads allow-popups allow-modals"
        sx={{ width: '100%', height: 'calc(100% - 36px)', border: 0, display: 'block', background: '#0d0e0f' }}
      />
    </Box>
  );
}

function DesignPane() {
  const controls = [
    ['Selection', 'No element selected', 'Pick from preview'],
    ['Spacing', '8px grid', 'Apply to section'],
    ['Typography', 'Display + body', 'Sync tokens'],
    ['Theme', 'Alabaster / lime', 'Preview variant'],
  ];
  return (
    <Stack spacing={1.3}>
      <Typography variant="overline" color="text.secondary">Visual edit</Typography>
      <Typography variant="body2" color="text.secondary">
        Select UI elements in preview, tune layout, spacing, text, images, and ask the agent to apply targeted edits.
      </Typography>
      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' }, gap: 1 }}>
        {controls.map(([title, value, action]) => (
          <Box key={title} sx={{ p: 1.4, border: `1px solid ${tokens.color.border.subtle}`, borderRadius: '8px', bgcolor: tokens.color.bg.inset }}>
            <Typography variant="caption" color="text.secondary">{title}</Typography>
            <Typography variant="subtitle2" sx={{ mt: 0.2 }}>{value}</Typography>
            <Button variant="outlined" size="small" sx={{ mt: 1 }}>{action}</Button>
          </Box>
        ))}
      </Box>
      <Box sx={{ p: 1.4, border: `1px solid ${tokens.color.border.subtle}`, borderRadius: '8px', bgcolor: tokens.color.bg.inset }}>
        <Typography variant="subtitle2">Design handoff prompt</Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mt: 0.4 }}>
          Ask for specific layout changes, responsive states, accessibility fixes, or component token updates. The agent should apply only the selected target.
        </Typography>
      </Box>
      <Button variant="outlined" size="small" sx={{ alignSelf: 'flex-start' }}>Connect Figma</Button>
    </Stack>
  );
}

function VersionsPane({ p, events }: { p: Project; events: ExecutionEvent[] }) {
  const checkpoints = [
    ['Project created', p.createdAt, 'Initial prompt and project record'],
    ['Last saved state', p.updatedAt, 'Latest project metadata and gate updates'],
    ...events.slice(-4).reverse().map((event) => [
      event.gate ? `${event.gate} gate` : event.step,
      event.createdAt,
      event.message,
    ]),
  ];

  return (
    <Stack spacing={1.4}>
      <Typography variant="overline" color="text.secondary">Versions</Typography>
      <Typography variant="body2" color="text.secondary">
        Checkpoints make the agent workflow reviewable before deploy. Runtime snapshots can attach here when the workspace driver supports them.
      </Typography>
      <Stack spacing={1}>
        {checkpoints.map(([label, date, detail], index) => (
          <Box key={`${label}-${date}-${index}`} sx={{
            p: 1.3,
            border: `1px solid ${tokens.color.border.subtle}`,
            borderRadius: '8px',
            bgcolor: tokens.color.bg.inset,
          }}>
            <Stack direction="row" justifyContent="space-between" spacing={1}>
              <Typography variant="subtitle2">{label}</Typography>
              <Typography variant="caption" color="text.secondary">{formatTime(date)}</Typography>
            </Stack>
            <Typography variant="body2" color="text.secondary" sx={{ mt: 0.4 }}>{detail}</Typography>
            <Stack direction="row" spacing={0.8} sx={{ mt: 1 }}>
              <Button variant="outlined" size="small">Compare</Button>
              <Button variant="outlined" size="small">Restore</Button>
            </Stack>
          </Box>
        ))}
      </Stack>
    </Stack>
  );
}

function DeployPane({ p, workspaceId, onGitHubLinked }: {
  p: Project; workspaceId: string | null; onGitHubLinked: () => void;
}) {
  const passed = Object.values(p.gates).filter((gate) => gate.status === 'passed').length;
  const total = Object.keys(p.gates).length || 7;
  const checks = [
    ['Finisher gates', `${passed}/${total} passed`, passed === total],
    ['Runtime workspace', workspaceId ? 'Attached' : 'Create from Files', Boolean(workspaceId)],
    ['GitHub repository', p.github ? p.github.fullName : 'Bind a repo', Boolean(p.github)],
    ['Deploy target', 'Vercel / Fly / Railway ready', p.gates.deploy?.status === 'passed'],
  ] as const;
  const readyCount = checks.filter(([, , ready]) => ready).length;
  const readiness = Math.round((readyCount / checks.length) * 100);

  return (
    <Stack spacing={2}>
      <Box sx={{ p: 1.6, border: `1px solid ${tokens.color.border.subtle}`, borderRadius: '8px', bgcolor: tokens.color.bg.inset }}>
        <Stack direction="row" justifyContent="space-between" alignItems="center">
          <Box>
            <Typography variant="overline" color="text.secondary">Ship readiness</Typography>
            <Typography variant="body2" sx={{ mt: 0.4 }}>
              {readyCount} of {checks.length} production checks are complete.
            </Typography>
          </Box>
          <Typography variant="h4" sx={{ color: readiness === 100 ? tokens.color.accent.lime : tokens.color.accent.sky }}>
            {readiness}%
          </Typography>
        </Stack>
        <LinearProgress variant="determinate" value={readiness} sx={{
          mt: 1.4,
          height: 7,
          borderRadius: '999px',
          bgcolor: tokens.color.bg.surfaceHover,
          '& .MuiLinearProgress-bar': { bgcolor: readiness === 100 ? tokens.color.accent.lime : tokens.color.accent.sky },
        }} />
        <Stack spacing={0.7} sx={{ mt: 1.4 }}>
          {checks.map(([label, value, ready]) => (
            <Stack key={label} direction="row" justifyContent="space-between" alignItems="center" spacing={1}>
              <Stack direction="row" spacing={1} alignItems="center">
                <Box sx={{
                  width: 8,
                  height: 8,
                  borderRadius: '50%',
                  bgcolor: ready ? tokens.color.accent.lime : tokens.color.text.muted,
                }} />
                <Typography variant="body2">{label}</Typography>
              </Stack>
              <Typography variant="caption" color="text.secondary">{value}</Typography>
            </Stack>
          ))}
        </Stack>
      </Box>
      <Box>
        <Typography variant="overline" color="text.secondary">Deploy targets</Typography>
        <Stack direction="row" spacing={1} flexWrap="wrap" sx={{ mt: 1 }}>
          {['Vercel', 'Fly.io', 'Railway', 'Cloudflare', 'GitHub'].map((t) =>
            <Chip key={t} label={t} size="small" sx={{ borderRadius: '6px' }} />)}
        </Stack>
        <Typography variant="caption" color="text.secondary">
          Activates once Deploy gate passes for project {p.id}.
        </Typography>
      </Box>
      <Divider />
      <GitHubPanel
        projectId={p.id}
        github={p.github ?? null}
        workspaceId={workspaceId}
        onLinked={onGitHubLinked}
      />
    </Stack>
  );
}

function ProjectSettingsPane({ p }: { p: Project }) {
  return (
    <Stack spacing={1.4}>
      <Typography variant="overline" color="text.secondary">Project settings</Typography>
      {[
        ['Name', p.name],
        ['Visibility', 'Workspace'],
        ['Remixing', 'Enabled'],
        ['Badge', 'Hidden on paid plans'],
        ['Created', new Date(p.createdAt).toLocaleDateString()],
      ].map(([label, value]) => (
        <Box key={label} sx={{ p: 1.2, borderRadius: 1.3, bgcolor: tokens.color.bg.inset, display: 'flex', justifyContent: 'space-between', gap: 2 }}>
          <Typography variant="body2" color="text.secondary">{label}</Typography>
          <Typography variant="body2" sx={{ fontWeight: 800 }}>{value}</Typography>
        </Box>
      ))}
      <Button variant="outlined" size="small" sx={{ alignSelf: 'flex-start' }}>Rename project</Button>
    </Stack>
  );
}

const panelSx = {
  display: 'flex',
  flexDirection: 'column',
  overflow: 'hidden',
  borderRadius: '12px',
  border: '1px solid rgba(17,17,17,0.12)',
  bgcolor: tokens.color.bg.surface,
  backgroundImage: 'none',
  boxShadow: 'none',
};

const headerOutlineButtonSx = {
  minWidth: { xs: 0, sm: 88 },
  px: { xs: 1, sm: 2 },
  color: tokens.color.text.inverse,
  borderColor: 'rgba(17,17,17,0.22)',
  '&:hover': {
    borderColor: 'rgba(17,17,17,0.38)',
    bgcolor: 'rgba(17,17,17,0.04)',
  },
};
