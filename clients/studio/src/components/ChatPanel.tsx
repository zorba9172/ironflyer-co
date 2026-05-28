import { useEffect, useRef, useState } from 'react';
import { Avatar, Box, IconButton, InputBase, Stack, Typography } from '@mui/material';
import { useChatStream, useAuth } from '@ironflyer/data';
import { useStudio, buildFocusContext } from '../store';
import { Markdown } from './Markdown';
import { useLiveProjectId } from '../hooks/useLiveProjectId';

interface Msg { id: string; from: 'user' | 'agent'; text: string; steps?: string[] }

function offlineReply(prompt: string): string {
  const q = prompt.length > 80 ? `${prompt.slice(0, 80)}…` : prompt;
  return `“${q}” — that's exactly what the finisher is for. I'm in offline preview right now, so I'm not wired to the orchestrator. Bring it up locally and set VITE_GRAPHQL_ENDPOINT, and I'll do it for real: read the code, propose reviewable patches, and close the gates. Until then the Map, Security, and Dashboard show sample data.`;
}

function providerErrorMessage(raw: string): string {
  const lower = raw.toLowerCase();
  if (
    lower.includes('api_key_invalid') ||
    lower.includes('api key expired') ||
    lower.includes('please renew the api key')
  ) {
    return 'The AI provider key has expired. The studio is still open; ask an operator to renew the Gemini API key, then retry this message.';
  }
  if (lower.includes('invalid api key') || lower.includes('api key')) {
    return 'The AI provider key is not valid right now. The studio is still open; ask an operator to check the provider credentials, then retry.';
  }
  if (lower.includes('quota') || lower.includes('rate limit') || lower.includes('resource_exhausted')) {
    return 'The AI provider is temporarily unavailable because of quota or rate limits. Wait a moment and retry.';
  }
  return '';
}

function normalizeChatError(error: unknown): string {
  const raw = error instanceof Error ? error.message : String(error || 'Something went wrong.');
  const provider = providerErrorMessage(raw);
  if (provider) return provider;
  if (/unauth/i.test(raw)) return 'Your session expired. Please sign in again to continue.';
  if (/chat stream failed:\s*4\d\d/i.test(raw)) return 'The orchestrator rejected this chat request. Please retry, or refresh the studio if it keeps happening.';
  if (/chat stream failed:\s*5\d\d/i.test(raw)) return 'The orchestrator had a temporary problem. Please retry in a moment.';
  if (raw.trim().startsWith('{') || raw.length > 240) {
    return 'The orchestrator returned an unreadable provider error. Please retry, or ask an operator to check the provider configuration.';
  }
  return raw;
}

export function ChatPanel({ initialPrompt }: { initialPrompt?: string }) {
  const mockProjectId = useStudio((s) => s.current.id);
  const constitution = useStudio((s) => s.constitution);
  const attachments = useStudio((s) => s.attachments);
  const { isLive, send: streamSend } = useChatStream();
  const { signOut } = useAuth();
  const liveProjectId = useLiveProjectId();
  const contextSentRef = useRef(false);
  const [messages, setMessages] = useState<Msg[]>(() =>
    initialPrompt ? [{ id: 'p0', from: 'user', text: initialPrompt }] : [],
  );
  const [draft, setDraft] = useState('');
  const [thinking, setThinking] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const startedRef = useRef(false);

  // When connected, chat against a real project (the first one) — not the mock.
  const targetProjectId = liveProjectId ?? mockProjectId;

  // Stream a reply from the orchestrator when connected; honest offline note otherwise.
  const respond = async (prompt: string) => {
    setThinking(true);
    if (isLive) {
      const id = `a${Date.now()}`;
      setMessages((m) => [...m, { id, from: 'agent', text: '', steps: [] }]);
      const patch = (fn: (x: Msg) => Msg) => setMessages((m) => m.map((x) => (x.id === id ? fn(x) : x)));
      // Ground the agent in the constitution + uploaded research on the first turn.
      let serverPrompt = prompt;
      if (!contextSentRef.current) {
        const ctx = buildFocusContext(constitution, attachments);
        if (ctx) {
          serverPrompt = `${ctx}\n\n---\n\n# Request\n${prompt}`;
          contextSentRef.current = true;
        }
      }
      try {
        await streamSend(targetProjectId, serverPrompt, (ev) => {
          if (ev.type === 'text') patch((x) => ({ ...x, text: x.text + ev.text }));
          else if (ev.type === 'tool') patch((x) => ({ ...x, steps: [...(x.steps ?? []), ev.name] }));
          else if (ev.type === 'error') patch((x) => ({ ...x, text: x.text || `⚠ ${normalizeChatError(ev.message)}` }));
        });
      } catch (e) {
        const raw = e instanceof Error ? e.message : String(e || '');
        const msg = normalizeChatError(e);
        patch((x) => ({ ...x, text: x.text || `⚠ ${msg}` }));
        if (/unauth/i.test(raw)) void signOut();
      } finally {
        setThinking(false);
      }
    } else {
      window.setTimeout(() => {
        setMessages((m) => [...m, { id: `a${Date.now()}`, from: 'agent', text: offlineReply(prompt) }]);
        setThinking(false);
      }, 600);
    }
  };

  // Respond to the prompt that started the session — once (guards StrictMode).
  // When online, wait until the real project id resolves so the execution is
  // created against a real project, not the mock id.
  useEffect(() => {
    if (!initialPrompt || startedRef.current) return;
    if (isLive && liveProjectId === null) return;
    startedRef.current = true;
    void respond(initialPrompt);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialPrompt, isLive, liveProjectId]);

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: 'smooth' });
  }, [messages, thinking]);

  const send = () => {
    const text = draft.trim();
    if (!text) return;
    setMessages((m) => [...m, { id: `u${Date.now()}`, from: 'user', text }]);
    setDraft('');
    void respond(text);
  };

  return (
    <Box sx={{ width: 380, flexShrink: 0, height: '100%', borderRight: 1, borderColor: 'divider', display: 'flex', flexDirection: 'column', bgcolor: 'background.default' }}>
      <Box ref={scrollRef} sx={{ flex: 1, overflowY: 'auto', p: 2 }}>
        <Stack spacing={2.5}>
          {messages.map((m) =>
            m.from === 'user' ? (
              <Stack key={m.id} direction="row" spacing={1.25} justifyContent="flex-end">
                <Box sx={{ bgcolor: 'action.selected', borderRadius: 3, px: 2, py: 1.25, maxWidth: '85%' }}>
                  <Typography sx={{ fontSize: '0.9rem' }}>{m.text}</Typography>
                </Box>
              </Stack>
            ) : (
              <Stack key={m.id} direction="row" spacing={1.25}>
                <Avatar sx={(t) => ({ width: 26, height: 26, backgroundImage: t.brand.gradient.signature })}> </Avatar>
                <Box sx={{ minWidth: 0, flex: 1 }}>
                  <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.7rem', color: 'text.disabled', mb: 0.5 })}>Orchestrator</Typography>
                  {m.steps && m.steps.length > 0 && (
                    <Stack spacing={0.5} sx={{ mb: 1, p: 1, borderRadius: 2, bgcolor: 'action.hover' }}>
                      {m.steps.map((s, i) => (
                        <Stack key={`${s}-${i}`} direction="row" spacing={1} alignItems="center">
                          <Box component="span" sx={{ color: 'secondary.main', fontSize: '0.8rem' }}>→</Box>
                          <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.74rem', color: 'text.secondary' })}>{s}</Typography>
                        </Stack>
                      ))}
                    </Stack>
                  )}
                  {m.text ? <Markdown>{m.text}</Markdown> : null}
                </Box>
              </Stack>
            ),
          )}
          {thinking && (
            <Stack direction="row" spacing={1.25} alignItems="center">
              <Avatar sx={(t) => ({ width: 26, height: 26, backgroundImage: t.brand.gradient.signature })}> </Avatar>
              <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.8rem', color: 'text.disabled' })}>thinking…</Typography>
            </Stack>
          )}
        </Stack>
      </Box>

      <Box sx={{ p: 1.5, borderTop: 1, borderColor: 'divider' }}>
        <Box sx={{ border: 1, borderColor: 'divider', borderRadius: 3, bgcolor: 'background.paper', p: 1.25, '&:focus-within': { borderColor: 'primary.main' } }}>
          <InputBase
            fullWidth
            multiline
            maxRows={5}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send(); } }}
            placeholder="What would you like to change?"
            sx={{ fontSize: '0.9rem', px: 0.5 }}
          />
          <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mt: 0.5 }}>
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.72rem', color: isLive ? 'success.main' : 'text.disabled' })}>{isLive ? '● connected to orchestrator' : '○ offline preview · ⏎ to send'}</Typography>
            <IconButton onClick={send} size="small" aria-label="Send" sx={(t) => ({ color: '#fff', backgroundImage: t.brand.gradient.signature, width: 30, height: 30 })}>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2"><path d="M5 12h14M13 6l6 6-6 6" /></svg>
            </IconButton>
          </Stack>
        </Box>
      </Box>
    </Box>
  );
}
