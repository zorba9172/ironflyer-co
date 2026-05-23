'use client';

// ChatPane — center "Chat" tab. Streams assistant turns via the orchestrator
// /chat SSE endpoint and renders them with a deliberately light markdown +
// code-block renderer (no new heavy deps). Each turn carries an agent name
// and a capability badge so the user can see who is actually replying.

import { useEffect, useRef, useState } from 'react';
import {
  Box, Chip, IconButton, Stack, TextField, Tooltip, Typography,
} from '@mui/material';
import { AttachFile, Close, Send, Stop } from '@mui/icons-material';
import { streamChat, ChatAttachment, ChatDelta } from '../../lib/api';
import { tokens } from '../../lib/theme';

interface Turn {
  id: string;
  role: 'user' | 'assistant';
  agent?: string;
  capability?: string;
  text: string;
  thinking: string;
  provider?: string;
  model?: string;
  status: 'streaming' | 'done' | 'error';
  error?: string;
  // attachments echo the user's image refs on their turn bubble so the
  // history shows what was sent, not just the prompt text.
  attachments?: PendingAttachment[];
}

interface PendingAttachment {
  id: string;
  name: string;
  mediaType: 'image/png' | 'image/jpeg' | 'image/webp' | 'image/gif';
  dataUrl: string;   // for the thumbnail preview
  base64: string;    // raw bytes (no data: prefix) — what we send on the wire
  bytes: number;
}

// Total decoded-image payload the orchestrator accepts in one request.
// Mirrors the 8 MiB ceiling on the server; we check the limit client-side
// so the user gets immediate feedback instead of a 4xx after upload.
const MAX_ATTACHMENT_PAYLOAD = 8 * 1024 * 1024;

interface Props {
  projectId: string;
  defaultRole?: string;
}

const ROLES: { key: string; label: string; cap: string }[] = [
  { key: 'planner', label: 'Planner', cap: 'plan' },
  { key: 'uxer', label: 'UX', cap: 'design' },
  { key: 'architect', label: 'Architect', cap: 'arch' },
  { key: 'coder', label: 'Coder', cap: 'code' },
  { key: 'reviewer', label: 'Reviewer', cap: 'review' },
  { key: 'tester', label: 'Tester', cap: 'tests' },
  { key: 'security', label: 'Security', cap: 'sec' },
  { key: 'deployer', label: 'Deployer', cap: 'deploy' },
];

export function ChatPane({ projectId, defaultRole = 'planner' }: Props) {
  const [turns, setTurns] = useState<Turn[]>([]);
  const [draft, setDraft] = useState('');
  const [role, setRole] = useState(defaultRole);
  const [streaming, setStreaming] = useState(false);
  const [pending, setPending] = useState<PendingAttachment[]>([]);
  const [attachError, setAttachError] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: 'smooth' });
  }, [turns]);

  async function send() {
    const goal = draft.trim();
    if ((!goal && pending.length === 0) || streaming) return;
    const attachments = pending;
    setDraft('');
    setPending([]);
    setStreaming(true);
    setAttachError(null);

    const userTurn: Turn = {
      id: crypto.randomUUID(), role: 'user',
      text: goal, thinking: '', status: 'done',
      attachments,
    };
    const draftTurn: Turn = {
      id: crypto.randomUUID(), role: 'assistant',
      agent: role, capability: ROLES.find((r) => r.key === role)?.cap,
      text: '', thinking: '', status: 'streaming',
    };
    setTurns((t) => [...t, userTurn, draftTurn]);

    const wireAttachments: ChatAttachment[] = attachments.map((a) => ({
      mediaType: a.mediaType, base64: a.base64,
    }));

    abortRef.current = new AbortController();
    try {
      await streamChat(
        projectId,
        { prompt: goal, role, attachments: wireAttachments.length ? wireAttachments : undefined },
        (d: ChatDelta) => setTurns((curr) => applyDelta(curr, d)),
        abortRef.current.signal,
      );
    } finally {
      setStreaming(false);
    }
  }

  function abort() {
    abortRef.current?.abort();
    setStreaming(false);
  }

  async function onPickFiles(files: FileList | null) {
    if (!files || files.length === 0) return;
    setAttachError(null);
    const next: PendingAttachment[] = [...pending];
    let runningTotal = next.reduce((sum, a) => sum + a.bytes, 0);
    for (const f of Array.from(files)) {
      const mt = f.type.toLowerCase();
      if (mt !== 'image/png' && mt !== 'image/jpeg'
          && mt !== 'image/webp' && mt !== 'image/gif') {
        setAttachError(`Unsupported type: ${f.type || 'unknown'}`);
        continue;
      }
      if (runningTotal + f.size > MAX_ATTACHMENT_PAYLOAD) {
        setAttachError('Attachments exceed 8 MB total — drop one before adding more.');
        break;
      }
      try {
        const { dataUrl, base64 } = await readImageAsBase64(f);
        next.push({
          id: crypto.randomUUID(),
          name: f.name || 'image',
          mediaType: mt as PendingAttachment['mediaType'],
          dataUrl, base64, bytes: f.size,
        });
        runningTotal += f.size;
      } catch (e) {
        setAttachError(`Could not read ${f.name}: ${String((e as Error)?.message ?? e)}`);
      }
    }
    setPending(next);
    // Reset the input so picking the same file twice re-fires onChange.
    if (fileInputRef.current) fileInputRef.current.value = '';
  }

  function removeAttachment(id: string) {
    setPending((curr) => curr.filter((a) => a.id !== id));
  }

  return (
    <Stack spacing={1} sx={{ height: '100%', minHeight: 0, minWidth: 0, width: '100%', overflow: 'hidden' }}>
      <Box ref={scrollRef} sx={{ flex: 1, minHeight: 0, overflowY: 'auto', pr: 0.4 }}>
        {turns.length === 0 ? (
          <EmptyChat />
        ) : (
          <Stack spacing={1} sx={{ pb: 1.2 }}>
            {turns.map((t) => <TurnBubble key={t.id} turn={t} />)}
          </Stack>
        )}
      </Box>

      <Box sx={{
        borderRadius: '14px',
        border: '1px solid rgba(17,17,17,0.12)',
        bgcolor: tokens.color.bg.surface,
        p: 1,
        minWidth: 0,
      }}>
        <Stack direction="row" spacing={0.6} sx={{ overflowX: 'auto', pb: 0.6 }}>
          {ROLES.map((r) => {
            const active = r.key === role;
            return (
              <Chip
                key={r.key}
                label={r.label}
                size="small"
                onClick={() => setRole(r.key)}
                sx={{
                  borderRadius: '8px', fontWeight: 700,
                  bgcolor: active ? tokens.color.accent.lime : tokens.color.bg.inset,
                  color: active ? tokens.color.text.inverse : tokens.color.text.primary,
                  border: `1px solid ${active ? tokens.color.accent.lime : 'rgba(17,17,17,0.12)'}`,
                  '&:hover': { bgcolor: active ? tokens.color.accent.lime : tokens.color.bg.surfaceHover },
                }}
              />
            );
          })}
        </Stack>
        {pending.length > 0 && (
          <Stack direction="row" spacing={0.6} sx={{ overflowX: 'auto', pb: 0.6 }}>
            {pending.map((a) => (
              <Box key={a.id} sx={{
                position: 'relative', flexShrink: 0,
                width: 56, height: 56, borderRadius: '8px', overflow: 'hidden',
                border: '1px solid rgba(17,17,17,0.12)',
                bgcolor: tokens.color.bg.inset,
              }} title={`${a.name} · ${Math.ceil(a.bytes / 1024)} KB`}>
                <Box component="img" src={a.dataUrl} alt={a.name}
                  sx={{ width: '100%', height: '100%', objectFit: 'cover', display: 'block' }} />
                <IconButton size="small" onClick={() => removeAttachment(a.id)}
                  sx={{
                    position: 'absolute', top: 2, right: 2, p: 0.2,
                    bgcolor: 'rgba(0,0,0,0.65)', color: '#fff',
                    '&:hover': { bgcolor: 'rgba(0,0,0,0.85)' },
                  }}>
                  <Close sx={{ fontSize: 12 }} />
                </IconButton>
              </Box>
            ))}
          </Stack>
        )}
        {attachError && (
          <Typography variant="caption" sx={{ color: tokens.color.accent.danger, display: 'block', mb: 0.4 }}>
            {attachError}
          </Typography>
        )}
        <Stack direction="row" spacing={1} alignItems="flex-end" sx={{ minWidth: 0 }}>
          <Tooltip title="Attach image (screenshot, mockup, design ref)">
            <span>
              <IconButton
                disabled={streaming}
                onClick={() => fileInputRef.current?.click()}
                sx={{
                  flexShrink: 0,
                  bgcolor: tokens.color.bg.inset, color: tokens.color.text.primary,
                  '&:hover': { bgcolor: tokens.color.bg.surfaceHover },
                }}
              >
                <AttachFile fontSize="small" />
              </IconButton>
            </span>
          </Tooltip>
          <input
            ref={fileInputRef}
            type="file"
            accept="image/png,image/jpeg,image/webp,image/gif"
            multiple
            hidden
            onChange={(e) => void onPickFiles(e.target.files)}
          />
          <TextField
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            placeholder="Tell the agent what to build or fix..."
            multiline minRows={1} maxRows={6}
            fullWidth
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                void send();
              }
            }}
            sx={{
              minWidth: 0,
              '& .MuiOutlinedInput-root': {
                bgcolor: tokens.color.bg.inset,
                borderRadius: '12px',
                fontSize: 14,
              },
            }}
          />
          {streaming ? (
            <Tooltip title="Stop">
              <IconButton onClick={abort} sx={{
                bgcolor: tokens.color.accent.danger, color: '#fff',
                '&:hover': { bgcolor: tokens.color.accent.danger },
              }}>
                <Stop fontSize="small" />
              </IconButton>
            </Tooltip>
          ) : (
            <Tooltip title="Send">
              <span>
                <IconButton
                  onClick={() => void send()}
                  disabled={!draft.trim() && pending.length === 0}
                  sx={{
                    flexShrink: 0,
                    bgcolor: tokens.color.accent.lime, color: tokens.color.text.inverse,
                    '&:hover': { bgcolor: tokens.color.accent.lime },
                    '&.Mui-disabled': { bgcolor: tokens.color.bg.inset, color: tokens.color.text.muted },
                  }}
                >
                  <Send fontSize="small" />
                </IconButton>
              </span>
            </Tooltip>
          )}
        </Stack>
      </Box>
    </Stack>
  );
}

// readImageAsBase64 turns a File into (dataUrl, base64) — the dataUrl is
// used for the thumbnail in the chat history, the bare base64 string is
// what we send to the orchestrator (the server rejects "data:" prefixes).
function readImageAsBase64(file: File): Promise<{ dataUrl: string; base64: string }> {
  return new Promise((resolve, reject) => {
    const r = new FileReader();
    r.onerror = () => reject(r.error ?? new Error('FileReader error'));
    r.onload = () => {
      const dataUrl = String(r.result || '');
      const idx = dataUrl.indexOf(',');
      const base64 = idx >= 0 ? dataUrl.slice(idx + 1) : dataUrl;
      resolve({ dataUrl, base64 });
    };
    r.readAsDataURL(file);
  });
}

function applyDelta(turns: Turn[], d: ChatDelta): Turn[] {
  if (turns.length === 0) return turns;
  const idx = turns.length - 1;
  const last = turns[idx];
  if (last.role !== 'assistant') return turns;
  const next = { ...last };
  switch (d.kind) {
    case 'start':    next.provider = d.provider; next.model = d.model; break;
    case 'text':     next.text = last.text + d.text; break;
    case 'thinking': next.thinking = last.thinking + d.text; break;
    case 'done':
      next.status = 'done';
      next.provider = d.provider; next.model = d.model;
      break;
    case 'error':
      next.status = 'error'; next.error = d.error;
      break;
  }
  const out = turns.slice();
  out[idx] = next;
  return out;
}

function EmptyChat() {
  return (
    <Stack spacing={1.4} alignItems="center" sx={{ py: 6, px: { xs: 1, sm: 2 }, textAlign: 'center', width: '100%', minWidth: 0 }}>
      <Typography variant="h6" sx={{ fontWeight: 800 }}>
        What should we build today?
      </Typography>
      <Typography variant="body2" sx={{ color: tokens.color.text.muted, maxWidth: 420 }}>
        Describe the feature or failure, choose a role, and the agent will work against the project files.
      </Typography>
      <Stack direction="row" spacing={0.7} useFlexGap flexWrap="wrap" justifyContent="center" sx={{ maxWidth: 520, mt: 1 }}>
        {[
          'Plan a dashboard with 3 KPI cards',
          'Add GET /api/health',
          'Fix the failing tests',
        ].map((s) => (
          <Chip key={s} label={s} size="small" sx={{
            borderRadius: '8px', bgcolor: tokens.color.bg.inset, color: tokens.color.text.primary,
            border: '1px solid rgba(17,17,17,0.10)',
          }} />
        ))}
      </Stack>
    </Stack>
  );
}

function TurnBubble({ turn }: { turn: Turn }) {
  const isUser = turn.role === 'user';
  return (
    <Box sx={{
      px: 1.4, py: 1.2, borderRadius: '14px',
      bgcolor: isUser ? tokens.color.bg.surfaceHover : tokens.color.bg.surface,
      border: turn.status === 'streaming'
        ? `1px solid ${tokens.color.accent.lime}`
        : '1px solid rgba(17,17,17,0.10)',
    }}>
      <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mb: 0.5 }}>
        <Stack direction="row" spacing={0.8} alignItems="center">
          <Typography
            variant="caption"
            sx={{
              fontWeight: 800, textTransform: 'uppercase', letterSpacing: '0.06em',
              color: isUser ? tokens.color.accent.sky : tokens.color.accent.lime,
            }}
          >
            {isUser ? 'You' : (turn.agent ?? 'Agent')}
          </Typography>
          {!isUser && turn.capability && (
            <Chip
              label={turn.capability} size="small"
              sx={{
                height: 16, fontSize: 9, fontWeight: 800, textTransform: 'uppercase',
                bgcolor: tokens.color.bg.inset, color: tokens.color.text.muted,
                '& .MuiChip-label': { px: 0.6 },
              }}
            />
          )}
          {!isUser && turn.provider && (
            <Typography variant="caption" sx={{ color: tokens.color.text.muted, fontFamily: tokens.font.mono, fontSize: 10 }}>
              {turn.provider}/{turn.model}
            </Typography>
          )}
        </Stack>
      </Stack>

      {turn.thinking && (
        <Box sx={{
          mb: 0.7, px: 1, py: 0.7, borderRadius: 1,
          bgcolor: tokens.color.bg.inset, color: tokens.color.text.muted,
          fontStyle: 'italic', fontSize: 12, whiteSpace: 'pre-wrap',
        }}>
          {turn.thinking}
        </Box>
      )}

      {turn.attachments && turn.attachments.length > 0 && (
        <Stack direction="row" spacing={0.6} sx={{ mb: 0.7, flexWrap: 'wrap' }} useFlexGap>
          {turn.attachments.map((a) => (
            <Box key={a.id} component="img" src={a.dataUrl} alt={a.name}
              sx={{
                width: 96, height: 96, borderRadius: '8px', objectFit: 'cover',
                border: '1px solid rgba(17,17,17,0.12)',
              }} />
          ))}
        </Stack>
      )}
      <MarkdownLite source={turn.text} />

      {turn.status === 'streaming' && (
        <Box component="span" sx={{
          color: tokens.color.accent.lime,
          animation: 'cb-blink 1s steps(2) infinite',
          fontWeight: 900,
          '@keyframes cb-blink': { '50%': { opacity: 0 } },
        }}>
          ▍
        </Box>
      )}
      {turn.error && (
        <Typography variant="caption" sx={{ color: tokens.color.accent.danger, display: 'block', mt: 0.5 }}>
          {turn.error}
        </Typography>
      )}
    </Box>
  );
}

// MarkdownLite — minimal renderer. Supports fenced code blocks (```lang),
// inline `code`, bold **text**, and paragraphs. We deliberately avoid heavy
// markdown libraries here; the assistants don't lean on exotic syntax.
function MarkdownLite({ source }: { source: string }) {
  const blocks = splitCodeBlocks(source);
  return (
    <Box sx={{ fontSize: 13, lineHeight: 1.55, color: tokens.color.text.primary }}>
      {blocks.map((b, i) => b.code ? (
        <CodeBlock key={i} lang={b.lang} text={b.text} />
      ) : (
        <Paragraphs key={i} text={b.text} />
      ))}
    </Box>
  );
}

function CodeBlock({ lang, text }: { lang?: string; text: string }) {
  return (
    <Box sx={{
      mt: 0.8, mb: 0.8, borderRadius: '8px',
      bgcolor: '#0d0e0f', color: '#e7e4dc',
      border: '1px solid rgba(17,17,17,0.20)',
      overflow: 'hidden',
    }}>
      {lang && (
        <Box sx={{
          px: 1.2, py: 0.4,
          fontFamily: tokens.font.mono, fontSize: 11,
          color: tokens.color.accent.lime,
          borderBottom: '1px solid rgba(255,255,255,0.06)',
          textTransform: 'lowercase',
        }}>
          {lang}
        </Box>
      )}
      <Box component="pre" sx={{
        m: 0, p: 1.2, fontFamily: tokens.font.mono, fontSize: 12,
        overflowX: 'auto', whiteSpace: 'pre',
      }}>
        {text}
      </Box>
    </Box>
  );
}

function Paragraphs({ text }: { text: string }) {
  // Split on blank lines; render inline markup per paragraph.
  const paras = text.split(/\n{2,}/);
  return (
    <>
      {paras.map((p, i) => (
        <Box key={i} sx={{ mt: i === 0 ? 0 : 0.7, whiteSpace: 'pre-wrap' }}>
          {renderInline(p)}
        </Box>
      ))}
    </>
  );
}

function renderInline(text: string): React.ReactNode[] {
  // bold **x**, code `x`. We process code first so backticks inside bold
  // text still pass through.
  const out: React.ReactNode[] = [];
  const re = /(`[^`]+`|\*\*[^*]+\*\*)/g;
  let last = 0; let m: RegExpExecArray | null; let key = 0;
  while ((m = re.exec(text)) !== null) {
    if (m.index > last) out.push(text.slice(last, m.index));
    const tok = m[0];
    if (tok.startsWith('`')) {
      out.push(
        <Box key={key++} component="code" sx={{
          px: 0.6, py: 0.1, mx: 0.1, borderRadius: 0.6,
          fontFamily: tokens.font.mono, fontSize: 12,
          bgcolor: tokens.color.bg.inset,
          color: tokens.color.accent.lime,
        }}>
          {tok.slice(1, -1)}
        </Box>,
      );
    } else {
      out.push(
        <Box key={key++} component="strong" sx={{ fontWeight: 800 }}>
          {tok.slice(2, -2)}
        </Box>,
      );
    }
    last = m.index + tok.length;
  }
  if (last < text.length) out.push(text.slice(last));
  return out;
}

function splitCodeBlocks(source: string): { text: string; code: boolean; lang?: string }[] {
  const out: { text: string; code: boolean; lang?: string }[] = [];
  const re = /```([a-zA-Z0-9_-]+)?\n([\s\S]*?)```/g;
  let last = 0; let m: RegExpExecArray | null;
  while ((m = re.exec(source)) !== null) {
    if (m.index > last) out.push({ text: source.slice(last, m.index), code: false });
    out.push({ text: m[2], code: true, lang: m[1] });
    last = m.index + m[0].length;
  }
  if (last < source.length) out.push({ text: source.slice(last), code: false });
  if (out.length === 0) out.push({ text: source, code: false });
  return out;
}
