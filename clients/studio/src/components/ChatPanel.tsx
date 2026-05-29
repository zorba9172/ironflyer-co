import { useEffect, useMemo, useRef, useState } from 'react';
import { Avatar, Box, Chip, ClickAwayListener, IconButton, InputBase, MenuItem, Paper, Popover, Stack, TextField, ToggleButton, ToggleButtonGroup, Tooltip, Typography } from '@mui/material';
import { useVirtualizer } from '@tanstack/react-virtual';
import { useChatStream, useAuth } from '@ironflyer/data';
import { formatRelativeTime, formatUSD } from '@ironflyer/core';
import {
  VscAdd, VscHistory, VscArchive, VscTrash, VscCopy, VscRefresh, VscDebugStop,
  VscEdit, VscChevronDown, VscChevronRight, VscRobot, VscLightbulb, VscShield,
  VscQuestion, VscChecklist, VscDebugStart, VscRocket, VscTerminal, VscPreview, VscSend,
} from 'react-icons/vsc';
import { useStudio, buildFocusContext, type ChatMsg } from '../store';
import { AGENTS, type Agent } from '../studioData';
import { Markdown } from './Markdown';
import { PreflightDialog } from './PreflightDialog';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { extractCodeFiles } from '../lib/extractCode';
import { text as fontScale } from '@ironflyer/design-tokens/brand';

// Quick starts shown on the empty chat — one tap dispatches a real request so
// the operator starts building in seconds instead of staring at a blank box.
const SUGGESTIONS = [
  'Scaffold a SaaS dashboard with auth',
  'Add Stripe checkout + webhook',
  'Design a landing page with a hero image',
  'Audit and close my security gates',
];

// Follow-ups offered after a reply lands — the next move is always one tap away.
const FOLLOWUPS = [
  "What's still blocking shipping?",
  'Apply the proposed patches',
  'Run the security gates',
  'Explain what just changed',
];

// Slash commands, Copilot-style. `run` executes locally; `prompt` dispatches a
// grounded request to the orchestrator.
type WorkMode = 'ask' | 'plan' | 'execute' | 'autopilot';
type TrustChatMsg = ChatMsg & {
  mode?: WorkMode;
  filesExtracted?: string[];
  costUSD?: number;
  riskHints?: string[];
};

interface SlashCommand { cmd: string; desc: string; prompt?: string; action?: 'new' | 'clear' | 'retry' | 'review' | 'stop'; mode?: WorkMode }
const SLASH: SlashCommand[] = [
  { cmd: '/new', desc: 'Start a new chat', action: 'new' },
  { cmd: '/clear', desc: 'Clear this conversation', action: 'clear' },
  { cmd: '/ask', desc: 'Switch to Ask mode', mode: 'ask' },
  { cmd: '/plan', desc: 'Switch to Plan mode', mode: 'plan' },
  { cmd: '/execute', desc: 'Switch to Execute mode', mode: 'execute' },
  { cmd: '/autopilot', desc: 'Switch to Autopilot mode', mode: 'autopilot' },
  { cmd: '/retry', desc: 'Retry the last assistant reply', action: 'retry' },
  { cmd: '/review', desc: 'Review the last answer for risks', action: 'review' },
  { cmd: '/stop', desc: 'Stop the running reply', action: 'stop' },
  { cmd: '/explain', desc: 'Explain the current code & architecture', prompt: 'Explain the current codebase and architecture in plain terms — what each part does and how it fits together.' },
  { cmd: '/security', desc: 'Audit & close security gates', prompt: 'Audit my security gates, list every finding by severity, and propose reviewable patches to close them.' },
  { cmd: '/ship', desc: "What's blocking shipping?", prompt: "What is blocking shipping right now? List each open finisher gate and exactly what's needed to close it." },
  { cmd: '/optimize', desc: 'Find performance & cost wins', prompt: 'Review the build for performance and provider-cost wins. Name concrete changes and their expected impact.' },
];

const MODES: { id: WorkMode; label: string; tip: string; Icon: typeof VscQuestion }[] = [
  { id: 'ask', label: 'Ask', Icon: VscQuestion, tip: 'Answer and explain without taking action' },
  { id: 'plan', label: 'Plan', Icon: VscChecklist, tip: 'Pause on the pre-spend plan gate before dispatch' },
  { id: 'execute', label: 'Execute', Icon: VscDebugStart, tip: 'Implement a bounded change with reviewable evidence' },
  { id: 'autopilot', label: 'Autopilot', Icon: VscRocket, tip: 'Drive the work end-to-end after confirmation' },
];

// ChatGPT-style pulsing dots while the assistant is thinking/streaming.
function TypingDots() {
  return (
    <Stack direction="row" spacing={0.6} sx={{ py: 0.75, '@keyframes ifPulse': { '0%,80%,100%': { opacity: 0.25, transform: 'scale(0.7)' }, '40%': { opacity: 1, transform: 'scale(1)' } } }}>
      {[0, 1, 2].map((i) => (
        <Box key={i} sx={{ width: 7, height: 7, borderRadius: 99, bgcolor: 'text.secondary', animation: 'ifPulse 1.2s ease-in-out infinite', animationDelay: `${i * 0.18}s` }} />
      ))}
    </Stack>
  );
}

function offlineReply(prompt: string): string {
  const q = prompt.length > 80 ? `${prompt.slice(0, 80)}…` : prompt;
  return `“${q}” is queued as a finisher request. The studio is in preview mode, so live runs are paused; the review, security, and economics surfaces are showing sample project state.`;
}

// Defense-in-depth vendor scrub. The orchestrator already sends provider-blind
// messages; the studio must NEVER render a vendor/model name even if a raw
// error somehow slips through. Any match collapses to a safe generic.
const VENDOR_RE = /\b(gemini|google\s*ai|vertex|anthropic|claude|openai|gpt-?\d|deepseek|hugging\s?face|llama|qwen|mistral|mixtral|bedrock|azure)\b/i;
const UNAVAILABLE = 'The assistant is temporarily unavailable. Please try again in a moment.';

function normalizeChatError(error: unknown): string {
  const raw = error instanceof Error ? error.message : String(error || '');
  // Known client-side transport / account states — all provider-blind.
  if (/insufficient wallet|budget_exhausted|payment required|402|out of (credit|budget)/i.test(raw)) return 'Your wallet is out of credit — top up to keep building.';
  if (/unauth|session expired|\b401\b/i.test(raw)) return 'Your session expired. Please sign in again to continue.';
  if (/offline|no orchestrator endpoint/i.test(raw)) return "You're in preview mode — connect the studio to go live.";
  if (/chat stream failed:\s*4\d\d/i.test(raw)) return 'That request was rejected. Please retry, or refresh the studio if it persists.';
  if (/chat stream failed:\s*5\d\d/i.test(raw)) return UNAVAILABLE;
  // Anything that names a vendor, looks like a raw payload, or is suspiciously
  // long is collapsed to a safe generic — never surfaced verbatim to the user.
  if (!raw.trim() || VENDOR_RE.test(raw) || raw.trim().startsWith('{') || raw.length > 200) return UNAVAILABLE;
  return raw;
}

const uid = (p: string) => `${p}${Date.now().toString(36)}${Math.random().toString(36).slice(2, 5)}`;

const slug = (s: string) => s.toLowerCase().replace(/[^a-z0-9]+/g, '');

const safeLabel = (value: string, fallback: string) => (VENDOR_RE.test(value) ? fallback : value);

function uniqueStrings(values: string[]): string[] {
  return [...new Set(values.map((v) => v.trim()).filter(Boolean))];
}

function modeInstruction(mode: WorkMode): string {
  switch (mode) {
    case 'ask':
      return 'Mode: Ask. Answer directly, explain tradeoffs, and do not claim to have changed files unless files are actually emitted.';
    case 'plan':
      return 'Mode: Plan. Produce a concise implementation plan, risks, files likely to change, and acceptance checks before any execution.';
    case 'execute':
      return 'Mode: Execute. Implement the requested bounded change, surface reviewable patches/evidence, and call out remaining checks.';
    case 'autopilot':
      return 'Mode: Autopilot. Drive the request end-to-end, route work to specialists when useful, and report evidence, cost, and risk at each gate.';
  }
}

function riskHintsFor(prompt: string, mode: WorkMode, planMode: boolean): string[] {
  const hints: string[] = [];
  if (planMode) hints.push('preflight gate');
  if (mode === 'ask') hints.push('read-only mode');
  if (mode === 'execute' || mode === 'autopilot') hints.push('review patches before apply');
  if (mode === 'autopilot') hints.push('higher autonomy');
  if (/(secret|token|key|auth|payment|stripe|billing|wallet|database|migration|delete|prod|security)/i.test(prompt)) hints.push('sensitive surface');
  return uniqueStrings(hints);
}

function reviewPromptFor(msg: TrustChatMsg): string {
  const quote = msg.text.length > 1600 ? `${msg.text.slice(0, 1600)}\n... [trimmed]` : msg.text;
  return `Review your previous answer for correctness, hidden risk, missing tests, and unsafe assumptions. Keep it provider-blind and action-oriented.\n\nPrevious answer:\n${quote}`;
}

interface Mentionable { token: string; name: string; role: string }
function mentionables(custom: Agent[]): Mentionable[] {
  return [
    ...AGENTS.map((a) => ({ token: a.id, name: a.name, role: a.role })),
    ...custom.map((a) => ({ token: slug(a.name || a.id), name: a.name || 'Untitled agent', role: a.role || 'custom agent' })),
  ];
}
// Pull @tokens out of a turn and resolve them to known agents.
function resolveMentions(text: string, all: Mentionable[]): Mentionable[] {
  const tokens = new Set((text.match(/@([a-z0-9_]+)/gi) ?? []).map((m) => m.slice(1).toLowerCase()));
  return all.filter((m) => tokens.has(m.token.toLowerCase()));
}

export function ChatPanel({ initialPrompt }: { initialPrompt?: string }) {
  const mockProjectId = useStudio((s) => s.current.id);
  const storeProjectId = useStudio((s) => s.liveProjectId);
  const launchWorkMode = useStudio((s) => s.initialWorkMode);
  const launchPreflight = useStudio((s) => s.initialPreflight);
  const constitution = useStudio((s) => s.constitution);
  const attachments = useStudio((s) => s.attachments);
  const writeGeneratedFiles = useStudio((s) => s.writeGeneratedFiles);
  const customAgents = useStudio((s) => s.customAgents);
  const chatSessions = useStudio((s) => s.chatSessions);
  const activeChatId = useStudio((s) => s.activeChatId);
  const ensureChat = useStudio((s) => s.ensureChat);
  const newChat = useStudio((s) => s.newChat);
  const selectChat = useStudio((s) => s.selectChat);
  const commitChat = useStudio((s) => s.commitChat);
  const renameChat = useStudio((s) => s.renameChat);
  const archiveChat = useStudio((s) => s.archiveChat);
  const restoreChat = useStudio((s) => s.restoreChat);
  const deleteChat = useStudio((s) => s.deleteChat);
  const repairRequest = useStudio((s) => s.repairRequest);
  const clearRepairRequest = useStudio((s) => s.clearRepairRequest);

  const { isLive, send: streamSend } = useChatStream();
  const { signOut } = useAuth();
  const liveProjectId = useLiveProjectId();

  const contextSentRef = useRef(false);
  const startedRef = useRef(false);
  const loadedIdRef = useRef<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);

  const [messages, setMessages] = useState<TrustChatMsg[]>([]);
  const [draft, setDraft] = useState('');
  const [thinking, setThinking] = useState(false);
  const [historyAnchor, setHistoryAnchor] = useState<HTMLElement | null>(null);
  const [slashOpen, setSlashOpen] = useState(false);
  const [workMode, setWorkMode] = useState<WorkMode>(launchWorkMode);
  // H2 — pre-spend plan/cost gate. When "Plan first" is on and we're connected,
  // a paid dispatch pauses on a PreflightDialog (cost preview + ProfitGuard
  // verdict) until the operator confirms. Off → the normal flow is unchanged.
  const [planMode, setPlanMode] = useState(launchPreflight);
  const [pending, setPending] = useState<{ text: string; history: TrustChatMsg[]; routed: Mentionable[]; mode: WorkMode } | null>(null);

  const activeSession = useMemo(() => chatSessions.find((c) => c.id === activeChatId) ?? null, [chatSessions, activeChatId]);
  const targetProjectId = storeProjectId ?? liveProjectId ?? mockProjectId;

  // Ensure a conversation exists as soon as the panel is shown.
  useEffect(() => { ensureChat(); }, [ensureChat]);

  // Load the active session into the live transcript whenever it changes
  // (mount, switching from history, new chat). Cancels any in-flight stream.
  useEffect(() => {
    if (!activeChatId || activeChatId === loadedIdRef.current) return;
    abortRef.current?.abort();
    loadedIdRef.current = activeChatId;
    contextSentRef.current = false;
    setThinking(false);
    setMessages(chatSessions.find((c) => c.id === activeChatId)?.messages ?? []);
  }, [activeChatId, chatSessions]);

  const commit = (msgs: TrustChatMsg[]) => { if (loadedIdRef.current) commitChat(loadedIdRef.current, msgs); };

  // Stream a reply from the orchestrator when connected; honest offline note otherwise.
  const respond = async (prompt: string, history: TrustChatMsg[], routed: Mentionable[] = [], mode: WorkMode = workMode) => {
    setThinking(true);
    if (isLive) {
      const id = uid('a');
      setMessages((m) => [...m, { id, from: 'agent', text: '', steps: [], ts: Date.now(), mode, routedTo: routed.length ? routed.map((r) => r.name) : undefined, riskHints: riskHintsFor(prompt, mode, mode === 'plan' || mode === 'autopilot') }]);
      const patch = (fn: (x: TrustChatMsg) => TrustChatMsg) => setMessages((m) => m.map((x) => (x.id === id ? fn(x) : x)));
      const parts: string[] = [];
      if (!contextSentRef.current) {
        const ctx = buildFocusContext(constitution, attachments);
        if (ctx) { parts.push(ctx); contextSentRef.current = true; }
      }
      if (routed.length > 0) {
        parts.push(`# Route this request\nHand this to the following specialist${routed.length === 1 ? '' : 's'} and answer in that role:\n${routed.map((r) => `- ${r.name} — ${r.role}`).join('\n')}`);
      }
      const prior = history.slice(-8);
      if (prior.length > 0) {
        const trim = (s: string) => (s.length > 2000 ? `${s.slice(0, 2000)}\n… [trimmed] …` : s);
        const transcript = prior.map((x) => `[${x.from === 'user' ? 'User' : 'Ironflyer'}] ${trim(x.text)}`).join('\n\n');
        parts.push(`# Conversation so far\n${transcript}`);
      }
      parts.push(`# Mode\n${modeInstruction(mode)}`);
      parts.push(`# Request\n${prompt}`);
      const serverPrompt = parts.join('\n\n---\n\n');
      const controller = new AbortController();
      abortRef.current = controller;
      let acc = '';
      try {
        await streamSend(targetProjectId, serverPrompt, (ev) => {
          if (ev.type === 'text') {
            acc += ev.text;
            patch((x) => ({ ...x, text: x.text + ev.text }));
            const files = extractCodeFiles(acc);
            if (files.length > 0) {
              writeGeneratedFiles(files);
              patch((x) => ({ ...x, filesExtracted: uniqueStrings([...(x.filesExtracted ?? []), ...files.map((f) => f.path)]) }));
            }
          } else if (ev.type === 'thinking') patch((x) => ({ ...x, thinking: (x.thinking ?? '') + ev.text }));
          else if (ev.type === 'tool') patch((x) => ({ ...x, steps: [...(x.steps ?? []), safeLabel(ev.name, 'tool step')] }));
          else if (ev.type === 'finish') patch((x) => ({ ...x, costUSD: typeof ev.costUSD === 'number' ? ev.costUSD : x.costUSD }));
          else if (ev.type === 'error') patch((x) => ({ ...x, text: x.text || `⚠ ${normalizeChatError(ev.message)}` }));
        }, controller.signal);
      } catch (e) {
        const raw = e instanceof Error ? e.message : String(e || '');
        if (controller.signal.aborted || /abort/i.test(raw)) {
          patch((x) => ({ ...x, text: x.text ? `${x.text}\n\n_⏹ stopped_` : '_⏹ stopped before any reply_' }));
        } else {
          patch((x) => ({ ...x, text: x.text || `⚠ ${normalizeChatError(e)}` }));
          if (/unauth/i.test(raw)) void signOut();
        }
      } finally {
        abortRef.current = null;
        setThinking(false);
        setMessages((m) => { commit(m); return m; });
      }
    } else {
      window.setTimeout(() => {
        setMessages((m) => {
          const next: TrustChatMsg[] = [...m, { id: uid('a'), from: 'agent' as const, text: offlineReply(prompt), ts: Date.now(), mode, routedTo: routed.length ? routed.map((r) => r.name) : undefined, riskHints: riskHintsFor(prompt, mode, mode === 'plan' || mode === 'autopilot') }];
          commit(next);
          return next;
        });
        setThinking(false);
      }, 600);
    }
  };

  // Seed the conversation with the prompt that launched the studio — once, and
  // only into an empty session (so reopening history never re-fires it).
  useEffect(() => {
    if (!initialPrompt || startedRef.current) return;
    if (messages.length > 0 || (activeSession && activeSession.messages.length > 0)) { startedRef.current = true; return; }
    if (isLive && liveProjectId === null) return;
    startedRef.current = true;
    const seed: TrustChatMsg = { id: uid('u'), from: 'user', text: initialPrompt, ts: Date.now(), mode: workMode };
    setMessages([seed]);
    commit([seed]);
    void respond(initialPrompt, [], [], workMode);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialPrompt, isLive, liveProjectId, activeSession]);

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: 'smooth' });
  }, [messages, thinking]);

  const allMentions = useMemo(() => mentionables(customAgents), [customAgents]);

  // Commit the user turn and dispatch the agent. Shared by the direct path and
  // the post-confirmation path so the pre-spend gate only wraps the decision.
  const dispatchTurn = (text: string, history: TrustChatMsg[], routed: Mentionable[], mode: WorkMode) => {
    const user: TrustChatMsg = { id: uid('u'), from: 'user', text, ts: Date.now(), mode, routedTo: routed.length ? routed.map((r) => r.name) : undefined };
    const next = [...history, user];
    setMessages(next);
    commit(next);
    void respond(text, history, routed, mode);
  };

  const send = (override?: string) => {
    const text = (override ?? draft).trim();
    if (!text || thinking) return;
    const history = messages;
    const routed = resolveMentions(text, allMentions);
    const mode = workMode;
    setDraft('');
    setSlashOpen(false);
    // Pre-spend gate: only when "Plan first" is armed and the orchestrator is
    // connected (so the dialog can show a real verdict). Otherwise dispatch now.
    if (planMode && isLive) { setPending({ text, history, routed, mode }); return; }
    dispatchTurn(text, history, routed, mode);
  };

  const confirmPending = () => {
    if (!pending) return;
    const { text, history, routed, mode } = pending;
    setPending(null);
    dispatchTurn(text, history, routed, mode);
  };

  // Edit a prior user turn in place, then re-run from there (Copilot-style).
  const editUser = (id: string, newText: string) => {
    const idx = messages.findIndex((m) => m.id === id);
    if (idx < 0 || !newText.trim()) return;
    const history = messages.slice(0, idx);
    const routed = resolveMentions(newText, allMentions);
    const mode = workMode;
    const edited: TrustChatMsg = { ...messages[idx]!, text: newText.trim(), mode, routedTo: routed.length ? routed.map((r) => r.name) : undefined };
    const trimmed = [...history, edited];
    setMessages(trimmed);
    commit(trimmed);
    void respond(newText.trim(), history, routed, mode);
  };

  const stop = () => { abortRef.current?.abort(); };

  // A repair request (e.g. the Preview broke) is queued in the store from
  // another pane; pick it up, send it through the normal chat flow, and clear.
  useEffect(() => {
    if (!repairRequest || thinking) return;
    clearRepairRequest();
    send(repairRequest);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [repairRequest, thinking]);

  const handleNewChat = () => {
    abortRef.current?.abort();
    startedRef.current = true; // a manual new chat must not re-seed the launch prompt
    newChat();
  };

  const handleClear = () => {
    abortRef.current?.abort();
    setThinking(false);
    setMessages([]);
    commit([]);
  };

  const chooseMode = (mode: WorkMode) => {
    setWorkMode(mode);
    setPlanMode(mode === 'plan' || mode === 'autopilot');
  };

  const retryLast = () => {
    if (thinking) return;
    let lastUser = -1;
    for (let i = messages.length - 1; i >= 0; i--) if (messages[i]!.from === 'user') { lastUser = i; break; }
    if (lastUser < 0) return;
    const user = messages[lastUser]!;
    const text = user.text;
    const history = messages.slice(0, lastUser);
    const trimmed = messages.slice(0, lastUser + 1); // drop the previous reply
    setMessages(trimmed);
    commit(trimmed);
    void respond(text, history, resolveMentions(text, allMentions), user.mode ?? workMode);
  };

  const reviewLast = () => {
    if (thinking) return;
    const last = [...messages].reverse().find((m) => m.from === 'agent' && m.text);
    if (last) send(reviewPromptFor(last));
  };

  const runSlash = (c: SlashCommand) => {
    setDraft('');
    setSlashOpen(false);
    if (c.mode) return chooseMode(c.mode);
    if (c.action === 'new') return handleNewChat();
    if (c.action === 'clear') return handleClear();
    if (c.action === 'retry') return retryLast();
    if (c.action === 'review') return reviewLast();
    if (c.action === 'stop') return stop();
    if (c.prompt) send(c.prompt);
  };

  const slashMatches = draft.startsWith('/') && !draft.includes(' ')
    ? SLASH.filter((c) => c.cmd.startsWith(draft.toLowerCase()))
    : [];
  const slashCommands = slashOpen ? SLASH : slashMatches;

  // @-mention autocomplete on the token at the caret end (route to a specialist).
  const mentionQuery = draft.match(/@([a-z0-9_]*)$/i);
  const mentionMatches = !slashOpen && mentionQuery
    ? allMentions.filter((m) => m.token.toLowerCase().includes(mentionQuery[1]!.toLowerCase()) || slug(m.name).includes(mentionQuery[1]!.toLowerCase())).slice(0, 6)
    : [];
  const pickMention = (m: Mentionable) => setDraft((d) => `${d.replace(/@([a-z0-9_]*)$/i, '')}@${m.token} `);

  const ctxChips = useMemo(() => {
    const chips: { label: string; title: string }[] = [];
    if (constitution.trim()) chips.push({ label: 'constitution', title: 'The project rules the finisher must honor' });
    if (attachments.length) chips.push({ label: `${attachments.length} reference${attachments.length === 1 ? '' : 's'}`, title: attachments.map((a) => a.name).join(', ') });
    return chips;
  }, [constitution, attachments]);

  const lastAgentId = useMemo(() => [...messages].reverse().find((m) => m.from === 'agent' && m.text)?.id, [messages]);
  const showFollowups = !thinking && messages.length > 0 && messages[messages.length - 1]?.from === 'agent' && !!messages[messages.length - 1]?.text;

  // Virtualize the transcript: only the visible turns are in the DOM, so a long
  // session (hundreds of messages) stays smooth. Heights are measured live, so
  // markdown/streaming bubbles size correctly.
  const rowVirtualizer = useVirtualizer({
    count: messages.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => 110,
    overscan: 8,
    getItemKey: (i) => messages[i]!.id,
  });

  return (
    <Box sx={{ width: 380, flexShrink: 0, height: '100%', borderRight: 1, borderColor: 'divider', display: 'flex', flexDirection: 'column', bgcolor: 'background.default' }}>
      <ChatHeader
        title={activeSession?.title ?? 'New chat'}
        onNew={handleNewChat}
        onOpenHistory={(el) => setHistoryAnchor(el)}
        onRename={(t) => activeChatId && renameChat(activeChatId, t)}
      />

      <Box ref={scrollRef} sx={{ flex: 1, overflowY: 'auto', p: 2 }}>
        {messages.length === 0 && !thinking && (
          <Box sx={{ pt: 1 }}>
            <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
              <Avatar sx={(t) => ({ width: 28, height: 28, backgroundImage: t.brand.gradient.signature })}> </Avatar>
              <Typography variant="h6" sx={{ fontSize: fontScale.s105 }}>Build something real</Typography>
            </Stack>
            <Typography sx={{ fontSize: fontScale.s86, color: 'text.secondary', lineHeight: 1.55, mb: 1.5 }}>
              Describe your product and I'll plan it, write the code, and drive it through the finisher gates — patches you can review, costs you can see.
            </Typography>
            <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 0.75 }}>
              {SUGGESTIONS.map((s) => (
                <Chip
                  key={s} label={s} clickable size="small" onClick={() => send(s)}
                  sx={{ height: 'auto', py: 0.6, fontSize: fontScale.s76, bgcolor: 'background.paper', border: 1, borderColor: 'divider', '& .MuiChip-label': { whiteSpace: 'normal' }, '&:hover': { borderColor: 'primary.main' } }}
                />
              ))}
            </Stack>
            <Typography sx={(t) => ({ mt: 2, fontFamily: t.brand.font.mono, fontSize: fontScale.s70, color: 'text.disabled' })}>Type <b>/</b> for commands</Typography>
          </Box>
        )}

        {messages.length > 0 && (
          <Box sx={{ position: 'relative', width: '100%', height: rowVirtualizer.getTotalSize() }}>
            {rowVirtualizer.getVirtualItems().map((vi) => {
              const m = messages[vi.index]!;
              return (
                <Box
                  key={vi.key} data-index={vi.index} ref={rowVirtualizer.measureElement}
                  sx={{ position: 'absolute', top: 0, left: 0, width: '100%', transform: `translateY(${vi.start}px)`, pb: 2.5 }}
                >
                  {m.from === 'user'
                    ? <UserBubble msg={m} disabled={thinking} onEdit={(t) => editUser(m.id, t)} />
                    : <AgentMessage msg={m} isLast={m.id === lastAgentId} thinking={thinking} onRetry={retryLast} onReview={() => send(reviewPromptFor(m))} onStop={stop} />}
                </Box>
              );
            })}
          </Box>
        )}

        <Stack spacing={2.5}>
          {thinking && messages[messages.length - 1]?.from === 'user' && (
            <Stack direction="row" spacing={1.25}>
              <Avatar sx={(t) => ({ width: 26, height: 26, backgroundImage: t.brand.gradient.signature })}> </Avatar>
              <TypingDots />
            </Stack>
          )}

          {showFollowups && (
            <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 0.75, pl: 4.5 }}>
              {FOLLOWUPS.map((f) => (
                <Chip key={f} label={f} clickable size="small" onClick={() => send(f)}
                  sx={{ height: 'auto', py: 0.5, fontSize: fontScale.s74, bgcolor: 'background.paper', border: 1, borderColor: 'divider', '& .MuiChip-label': { whiteSpace: 'normal' }, '&:hover': { borderColor: 'primary.main' } }} />
              ))}
            </Stack>
          )}
        </Stack>
      </Box>

      <Box sx={{ p: 1.5, borderTop: 1, borderColor: 'divider', position: 'relative' }}>
        {slashCommands.length > 0 && (
          <Paper elevation={0} sx={{ position: 'absolute', left: 12, right: 12, bottom: '100%', mb: 0.5, border: 1, borderColor: 'divider', borderRadius: 2, overflow: 'hidden', backgroundImage: 'none' }}>
            <Typography sx={(t) => ({ px: 1.5, pt: 1, pb: 0.5, fontFamily: t.brand.font.mono, fontSize: fontScale.s62, letterSpacing: '0.08em', textTransform: 'uppercase', color: 'text.disabled' })}>
              Slash commands
            </Typography>
            {slashCommands.map((c) => (
              <Box key={c.cmd} onClick={() => runSlash(c)} sx={{ px: 1.5, py: 1, cursor: 'pointer', '&:hover': { bgcolor: 'action.hover' } }}>
                <Typography component="span" sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s80, color: 'primary.main', mr: 1 })}>{c.cmd}</Typography>
                <Typography component="span" sx={{ fontSize: fontScale.s78, color: 'text.secondary' }}>{c.desc}</Typography>
              </Box>
            ))}
          </Paper>
        )}

        {mentionMatches.length > 0 && (
          <Paper elevation={0} sx={{ position: 'absolute', left: 12, right: 12, bottom: '100%', mb: 0.5, border: 1, borderColor: 'divider', borderRadius: 2, overflow: 'hidden', maxHeight: 240, overflowY: 'auto', backgroundImage: 'none' }}>
            {mentionMatches.map((m) => (
              <Stack key={m.token} direction="row" spacing={1} alignItems="center" onClick={() => pickMention(m)} sx={{ px: 1.5, py: 0.9, cursor: 'pointer', '&:hover': { bgcolor: 'action.hover' } }}>
                <VscRobot size={13} />
                <Typography component="span" sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s78, color: 'primary.main' })}>@{m.token}</Typography>
                <Typography component="span" noWrap sx={{ fontSize: fontScale.s74, color: 'text.secondary', minWidth: 0 }}>{m.role}</Typography>
              </Stack>
            ))}
          </Paper>
        )}

        {ctxChips.length > 0 && (
          <Stack direction="row" spacing={0.5} sx={{ mb: 0.75, flexWrap: 'wrap', gap: 0.5 }}>
            {ctxChips.map((c) => (
              <Tooltip key={c.label} title={c.title} arrow>
                <Chip size="small" label={c.label} sx={(t) => ({ height: 18, fontSize: fontScale.s60, fontFamily: t.brand.font.mono, bgcolor: 'action.hover', color: 'text.secondary' })} />
              </Tooltip>
            ))}
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s60, color: 'text.disabled', alignSelf: 'center' })}>grounding this chat</Typography>
          </Stack>
        )}

        <ToggleButtonGroup
          exclusive
          size="small"
          value={workMode}
          onChange={(_, next: WorkMode | null) => { if (next) chooseMode(next); }}
          sx={{
            mb: 0.75,
            display: 'grid',
            gridTemplateColumns: 'repeat(4, minmax(0, 1fr))',
            border: 1,
            borderColor: 'divider',
            borderRadius: 2,
            overflow: 'hidden',
            '& .MuiToggleButtonGroup-grouped': { m: 0, border: 0, borderRadius: 0 },
          }}
        >
          {MODES.map(({ id, label, tip, Icon }) => (
            <ToggleButton
              key={id}
              value={id}
              aria-label={`${label} mode`}
              sx={(t) => ({
                minWidth: 0,
                px: 0.5,
                py: 0.55,
                gap: 0.45,
                color: 'text.secondary',
                '&.Mui-selected': { color: 'primary.main', bgcolor: `${t.palette.primary.main}14` },
              })}
            >
              <Tooltip title={tip} arrow>
                <Stack component="span" direction="row" spacing={0.4} alignItems="center" sx={{ minWidth: 0 }}>
                  <Icon size={12} />
                  <Typography component="span" noWrap sx={{ fontSize: fontScale.s68, fontWeight: 700 }}>{label}</Typography>
                </Stack>
              </Tooltip>
            </ToggleButton>
          ))}
        </ToggleButtonGroup>

        <Box sx={{ border: 1, borderColor: 'divider', borderRadius: 3, bgcolor: 'background.paper', p: 1.25, '&:focus-within': { borderColor: 'primary.main' } }}>
          <InputBase
            fullWidth multiline maxRows={5}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Escape' && thinking) { e.preventDefault(); stop(); return; }
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                if (mentionMatches.length >= 1) { pickMention(mentionMatches[0]!); return; }
                if (slashMatches.length === 1) runSlash(slashMatches[0]!);
                else send();
              }
            }}
            placeholder="Ask, or @mention an agent · / for commands"
            sx={{ fontSize: fontScale.s90, px: 0.5 }}
          />
          <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mt: 0.5 }}>
            <Stack direction="row" alignItems="center" spacing={1.25} sx={{ minWidth: 0 }}>
              <Typography noWrap sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s72, color: isLive ? 'success.main' : 'text.disabled' })}>{isLive ? '● connected' : '○ offline preview'}</Typography>
              <Tooltip title="Show slash commands" arrow>
                <IconButton
                  size="small"
                  aria-label="Show slash commands"
                  onClick={() => setSlashOpen((v) => !v)}
                  sx={{ color: slashOpen ? 'primary.main' : 'text.secondary', border: 1, borderColor: slashOpen ? 'primary.main' : 'divider', width: 24, height: 24 }}
                >
                  <VscTerminal size={12} />
                </IconButton>
              </Tooltip>
              <Tooltip title={planMode ? 'Preflight is armed for this mode' : 'This mode dispatches without the pre-spend gate'} arrow>
                <Stack
                  direction="row" alignItems="center" spacing={0.5}
                  sx={(t) => ({
                    px: 0.75, py: 0.25, borderRadius: 99, border: 1,
                    borderColor: planMode ? 'primary.main' : 'divider',
                    color: planMode ? 'primary.main' : 'text.disabled',
                    userSelect: 'none',
                    bgcolor: planMode ? `${t.palette.primary.main}14` : 'transparent',
                  })}
                >
                  <VscShield size={11} />
                  <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s66, fontWeight: 600 })}>{planMode ? 'preflight' : 'direct'}</Typography>
                </Stack>
              </Tooltip>
            </Stack>
            {thinking ? (
              <Tooltip title="Stop generating" arrow>
                <IconButton onClick={stop} size="small" aria-label="Stop" sx={{ color: 'text.primary', border: 1, borderColor: 'divider', width: 30, height: 30 }}>
                  <VscDebugStop size={14} />
                </IconButton>
              </Tooltip>
            ) : (
              <IconButton onClick={() => send()} disabled={!draft.trim()} size="small" aria-label="Send" sx={(t) => ({ color: 'common.white', backgroundImage: t.brand.gradient.signature, width: 30, height: 30, '&.Mui-disabled': { backgroundImage: 'none', bgcolor: 'action.disabledBackground' } })}>
                <VscSend size={14} />
              </IconButton>
            )}
          </Stack>
        </Box>
      </Box>

      <HistoryPopover
        anchorEl={historyAnchor}
        onClose={() => setHistoryAnchor(null)}
        sessions={chatSessions}
        activeId={activeChatId}
        onSelect={(id) => { selectChat(id); setHistoryAnchor(null); }}
        onArchive={archiveChat}
        onRestore={restoreChat}
        onDelete={deleteChat}
      />

      <PreflightDialog
        open={!!pending}
        prompt={pending?.text ?? ''}
        mode={pending?.mode ?? workMode}
        projectId={targetProjectId}
        isLive={isLive}
        onConfirm={confirmPending}
        onCancel={() => setPending(null)}
      />
    </Box>
  );
}

function AgentMessage({ msg, isLast, thinking, onRetry, onReview, onStop }: { msg: TrustChatMsg; isLast: boolean; thinking: boolean; onRetry: () => void; onReview: () => void; onStop: () => void }) {
  return (
    <Stack direction="row" spacing={1.25}>
      <Avatar sx={(t) => ({ width: 26, height: 26, backgroundImage: t.brand.gradient.signature })}> </Avatar>
      <Box sx={{ minWidth: 0, flex: 1 }}>
        <Stack direction="row" spacing={0.75} alignItems="center" sx={{ mb: 0.5 }}>
          <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s70, color: 'text.disabled' })}>Orchestrator</Typography>
          {msg.mode && (
            <Chip size="small" label={msg.mode} sx={(t) => ({ height: 17, fontFamily: t.brand.font.mono, fontSize: fontScale.s58, color: 'text.secondary', bgcolor: 'action.hover' })} />
          )}
        </Stack>
        {msg.thinking && <ReasoningBlock text={msg.thinking} />}
        {msg.steps && msg.steps.length > 0 && (
          <Stack spacing={0.5} sx={{ mb: 1, p: 1, borderRadius: 2, bgcolor: 'action.hover' }}>
            {msg.steps.map((s, i) => (
              <Stack key={`${s}-${i}`} direction="row" spacing={1} alignItems="center">
                <Box component="span" sx={{ color: 'secondary.main', fontSize: fontScale.s80 }}>→</Box>
                <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s74, color: 'text.secondary' })}>{s}</Typography>
              </Stack>
            ))}
          </Stack>
        )}
        {msg.text ? <Markdown>{msg.text}</Markdown> : <TypingDots />}
        <EvidenceFooter msg={msg} />
        {msg.text && (
          <Stack direction="row" spacing={0.25} sx={{ mt: 0.25, opacity: 0.55, '&:hover': { opacity: 1 } }}>
            <Tooltip title="Copy" arrow>
              <IconButton size="small" onClick={() => void navigator.clipboard?.writeText(msg.text)} sx={{ color: 'text.secondary', p: 0.4 }}><VscCopy size={13} /></IconButton>
            </Tooltip>
            <Tooltip title="Review this answer" arrow>
              <IconButton size="small" onClick={onReview} sx={{ color: 'text.secondary', p: 0.4 }}><VscPreview size={13} /></IconButton>
            </Tooltip>
            {isLast && !thinking && (
              <Tooltip title="Retry this reply" arrow>
                <IconButton size="small" onClick={onRetry} sx={{ color: 'text.secondary', p: 0.4 }}><VscRefresh size={13} /></IconButton>
              </Tooltip>
            )}
            {isLast && thinking && (
              <Tooltip title="Stop generating" arrow>
                <IconButton size="small" onClick={onStop} sx={{ color: 'text.secondary', p: 0.4 }}><VscDebugStop size={13} /></IconButton>
              </Tooltip>
            )}
          </Stack>
        )}
      </Box>
    </Stack>
  );
}

function EvidenceFooter({ msg }: { msg: TrustChatMsg }) {
  const files = msg.filesExtracted ?? [];
  const routed = msg.routedTo ?? [];
  const steps = msg.steps ?? [];
  const risks = msg.riskHints ?? [];
  const hasEvidence = files.length > 0 || routed.length > 0 || steps.length > 0 || risks.length > 0 || typeof msg.costUSD === 'number';
  if (!hasEvidence) return null;

  return (
    <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 0.5, mt: 0.75 }}>
      {files.length > 0 && (
        <Tooltip title={files.slice(0, 8).join('\n')} arrow>
          <Chip size="small" label={`${files.length} file${files.length === 1 ? '' : 's'} extracted`} sx={(t) => ({ height: 19, fontFamily: t.brand.font.mono, fontSize: fontScale.s60, bgcolor: 'action.hover', color: 'text.secondary' })} />
        </Tooltip>
      )}
      {steps.length > 0 && (
        <Tooltip title={steps.slice(-8).join('\n')} arrow>
          <Chip size="small" label={`${steps.length} tool step${steps.length === 1 ? '' : 's'}`} sx={(t) => ({ height: 19, fontFamily: t.brand.font.mono, fontSize: fontScale.s60, bgcolor: 'action.hover', color: 'text.secondary' })} />
        </Tooltip>
      )}
      {routed.length > 0 && (
        <Tooltip title={routed.join(', ')} arrow>
          <Chip size="small" icon={<VscRobot size={11} />} label={`${routed.length} agent${routed.length === 1 ? '' : 's'} routed`} sx={(t) => ({ height: 19, fontFamily: t.brand.font.mono, fontSize: fontScale.s60, bgcolor: 'action.hover', color: 'text.secondary', '& .MuiChip-icon': { ml: 0.5 } })} />
        </Tooltip>
      )}
      {typeof msg.costUSD === 'number' && (
        <Chip size="small" label={`cost ${formatUSD(msg.costUSD)}`} sx={(t) => ({ height: 19, fontFamily: t.brand.font.mono, fontSize: fontScale.s60, bgcolor: 'action.hover', color: 'text.secondary' })} />
      )}
      {risks.map((r) => (
        <Chip key={r} size="small" icon={<VscShield size={11} />} label={r} sx={(t) => ({ height: 19, fontFamily: t.brand.font.mono, fontSize: fontScale.s60, bgcolor: `${t.palette.warning.main}16`, color: 'text.secondary', '& .MuiChip-icon': { ml: 0.5, color: 'warning.main' } })} />
      ))}
    </Stack>
  );
}

function UserBubble({ msg, disabled, onEdit }: { msg: ChatMsg; disabled: boolean; onEdit: (text: string) => void }) {
  const [editing, setEditing] = useState(false);
  const [val, setVal] = useState(msg.text);
  if (editing) {
    return (
      <Box sx={{ alignSelf: 'flex-end', width: '92%' }}>
        <TextField
          value={val} autoFocus multiline fullWidth size="small"
          onChange={(e) => setVal(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); setEditing(false); onEdit(val); } if (e.key === 'Escape') { setEditing(false); setVal(msg.text); } }}
          sx={{ '& .MuiInputBase-input': { fontSize: fontScale.s90 } }}
        />
        <Stack direction="row" spacing={1} justifyContent="flex-end" sx={{ mt: 0.5 }}>
          <Chip size="small" clickable label="Cancel" onClick={() => { setEditing(false); setVal(msg.text); }} sx={{ height: 22, fontSize: fontScale.s70, bgcolor: 'action.hover' }} />
          <Chip size="small" clickable label="Send" onClick={() => { setEditing(false); onEdit(val); }} sx={(t) => ({ height: 22, fontSize: fontScale.s70, color: 'common.white', backgroundImage: t.brand.gradient.signature })} />
        </Stack>
      </Box>
    );
  }
  return (
    <Stack direction="row" spacing={0.5} justifyContent="flex-end" alignItems="flex-start" sx={{ '&:hover .if-edit': { opacity: 1 } }}>
      <Tooltip title="Edit & resend" arrow>
        <IconButton size="small" disabled={disabled} className="if-edit" onClick={() => { setVal(msg.text); setEditing(true); }} sx={{ opacity: 0, color: 'text.disabled', p: 0.4, mt: 0.5 }}><VscEdit size={12} /></IconButton>
      </Tooltip>
      <Box sx={{ bgcolor: 'action.selected', borderRadius: 3, px: 2, py: 1.25, maxWidth: '85%' }}>
        {msg.routedTo && msg.routedTo.length > 0 && (
          <Stack direction="row" spacing={0.5} sx={{ mb: 0.5, flexWrap: 'wrap', gap: 0.5 }}>
            {msg.routedTo.map((r) => (
              <Chip key={r} size="small" icon={<VscRobot size={11} />} label={r} sx={(t) => ({ height: 18, fontSize: fontScale.s60, fontFamily: t.brand.font.mono, bgcolor: 'background.paper', '& .MuiChip-icon': { ml: 0.5 } })} />
            ))}
          </Stack>
        )}
        <Typography sx={{ fontSize: fontScale.s90, whiteSpace: 'pre-wrap' }}>{msg.text}</Typography>
      </Box>
    </Stack>
  );
}

function ReasoningBlock({ text }: { text: string }) {
  const [open, setOpen] = useState(false);
  return (
    <Box sx={{ mb: 1 }}>
      <Stack direction="row" alignItems="center" spacing={0.75} onClick={() => setOpen((v) => !v)} sx={{ cursor: 'pointer', color: 'text.secondary', '&:hover': { color: 'text.primary' } }}>
        <VscLightbulb size={13} />
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s68, letterSpacing: '0.04em' })}>Reasoning</Typography>
        {open ? <VscChevronDown size={12} /> : <VscChevronRight size={12} />}
      </Stack>
      {open && (
        <Box sx={{ mt: 0.5, p: 1, borderRadius: 2, borderLeft: 2, borderColor: 'divider', bgcolor: 'action.hover' }}>
          <Typography sx={{ fontSize: fontScale.s78, color: 'text.secondary', whiteSpace: 'pre-wrap', lineHeight: 1.5 }}>{text}</Typography>
        </Box>
      )}
    </Box>
  );
}

function ChatHeader({ title, onNew, onOpenHistory, onRename }: {
  title: string; onNew: () => void; onOpenHistory: (el: HTMLElement) => void; onRename: (t: string) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [val, setVal] = useState(title);
  useEffect(() => { setVal(title); }, [title]);
  const commit = () => { setEditing(false); if (val.trim() && val.trim() !== title) onRename(val.trim()); };
  return (
    <Stack direction="row" alignItems="center" spacing={0.5} sx={{ px: 1.5, py: 1, borderBottom: 1, borderColor: 'divider', bgcolor: 'background.paper' }}>
      {editing ? (
        <ClickAwayListener onClickAway={commit}>
          <TextField
            value={val} autoFocus variant="standard"
            onChange={(e) => setVal(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') commit(); if (e.key === 'Escape') { setEditing(false); setVal(title); } }}
            sx={{ flex: 1, '& .MuiInput-input': { fontSize: fontScale.s85, py: 0.25 } }}
          />
        </ClickAwayListener>
      ) : (
        <>
          <Typography noWrap sx={{ flex: 1, fontSize: fontScale.s85, fontWeight: 600 }}>{title}</Typography>
          <Tooltip title="Rename chat" arrow>
            <IconButton size="small" onClick={() => setEditing(true)} sx={{ color: 'text.disabled', p: 0.5 }}><VscEdit size={13} /></IconButton>
          </Tooltip>
        </>
      )}
      <Tooltip title="Chat history" arrow>
        <IconButton size="small" onClick={(e) => onOpenHistory(e.currentTarget)} sx={{ color: 'text.secondary', p: 0.5 }}><VscHistory size={15} /></IconButton>
      </Tooltip>
      <Tooltip title="New chat" arrow>
        <IconButton size="small" onClick={onNew} sx={{ color: 'text.secondary', p: 0.5 }}><VscAdd size={15} /></IconButton>
      </Tooltip>
    </Stack>
  );
}

function HistoryPopover({ anchorEl, onClose, sessions, activeId, onSelect, onArchive, onRestore, onDelete }: {
  anchorEl: HTMLElement | null; onClose: () => void;
  sessions: ReturnType<typeof useStudio.getState>['chatSessions']; activeId: string | null;
  onSelect: (id: string) => void; onArchive: (id: string) => void; onRestore: (id: string) => void; onDelete: (id: string) => void;
}) {
  const [showArchived, setShowArchived] = useState(false);
  const live = sessions.filter((c) => !c.archived).sort((a, b) => b.updatedAt - a.updatedAt);
  const archived = sessions.filter((c) => c.archived).sort((a, b) => b.updatedAt - a.updatedAt);
  return (
    <Popover
      open={!!anchorEl} anchorEl={anchorEl} onClose={onClose}
      anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }} transformOrigin={{ vertical: 'top', horizontal: 'right' }}
      slotProps={{ paper: { sx: { width: 320, maxHeight: 460, border: 1, borderColor: 'divider', backgroundImage: 'none' } } }}
    >
      <Typography sx={(t) => ({ px: 2, pt: 1.5, pb: 1, fontFamily: t.brand.font.mono, fontSize: fontScale.s62, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled' })}>Chats</Typography>
      {live.length === 0 ? (
        <Typography sx={{ px: 2, pb: 1.5, fontSize: fontScale.s80, color: 'text.secondary' }}>No conversations yet.</Typography>
      ) : (
        live.map((c) => <SessionRow key={c.id} session={c} active={c.id === activeId} onClick={() => onSelect(c.id)} action={<VscArchive size={13} />} actionTitle="Archive" onAction={() => onArchive(c.id)} />)
      )}

      {archived.length > 0 && (
        <Box sx={{ borderTop: 1, borderColor: 'divider', mt: 0.5 }}>
          <MenuItem onClick={() => setShowArchived((v) => !v)} sx={{ py: 0.75 }}>
            {showArchived ? <VscChevronDown size={13} /> : <VscChevronRight size={13} />}
            <Typography sx={{ ml: 1, fontSize: fontScale.s78, color: 'text.secondary' }}>Archived ({archived.length})</Typography>
          </MenuItem>
          {showArchived && archived.map((c) => (
            <SessionRow key={c.id} session={c} active={false} muted onClick={() => onRestore(c.id)} action={<VscTrash size={13} />} actionTitle="Delete permanently" onAction={() => onDelete(c.id)} />
          ))}
        </Box>
      )}
    </Popover>
  );
}

function SessionRow({ session, active, muted, onClick, action, actionTitle, onAction }: {
  session: ReturnType<typeof useStudio.getState>['chatSessions'][number];
  active: boolean; muted?: boolean; onClick: () => void; action: React.ReactNode; actionTitle: string; onAction: () => void;
}) {
  return (
    <Stack direction="row" alignItems="center" sx={{ px: 1, mx: 1, borderRadius: 1.5, bgcolor: active ? 'action.selected' : 'transparent', '&:hover': { bgcolor: 'action.hover' } }}>
      <Box onClick={onClick} sx={{ flex: 1, minWidth: 0, py: 0.9, px: 1, cursor: 'pointer', opacity: muted ? 0.7 : 1 }}>
        <Typography noWrap sx={{ fontSize: fontScale.s82, fontWeight: active ? 600 : 400 }}>{session.title}</Typography>
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s62, color: 'text.disabled' })}>
          {session.messages.length} msg · {formatRelativeTime(session.updatedAt)}{muted ? ' · click to restore' : ''}
        </Typography>
      </Box>
      <Tooltip title={actionTitle} arrow>
        <IconButton size="small" onClick={(e) => { e.stopPropagation(); onAction(); }} sx={{ color: 'text.disabled', p: 0.5 }}>{action}</IconButton>
      </Tooltip>
    </Stack>
  );
}
