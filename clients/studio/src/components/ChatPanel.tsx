import { useEffect, useRef, useState } from 'react';
import { Avatar, Box, IconButton, InputBase, Stack, Typography } from '@mui/material';
import { useChatStream } from '@ironflyer/data';
import { useStudio } from '../store';

interface Msg { id: string; from: 'user' | 'agent'; text: string; steps?: string[] }

function offlineReply(prompt: string): string {
  const q = prompt.length > 80 ? `${prompt.slice(0, 80)}…` : prompt;
  return `“${q}” — that's exactly what the finisher is for. I'm in offline preview right now, so I'm not wired to the orchestrator. Bring it up locally and set VITE_GRAPHQL_ENDPOINT, and I'll do it for real: read the code, propose reviewable patches, and close the gates. Until then the Map, Security, and Dashboard show sample data.`;
}

export function ChatPanel({ initialPrompt }: { initialPrompt?: string }) {
  const projectId = useStudio((s) => s.current.id);
  const { isLive, send: streamSend } = useChatStream();
  const [messages, setMessages] = useState<Msg[]>(() =>
    initialPrompt ? [{ id: 'p0', from: 'user', text: initialPrompt }] : [],
  );
  const [draft, setDraft] = useState('');
  const [thinking, setThinking] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const startedRef = useRef(false);

  // Stream a reply from the orchestrator when connected; honest offline note otherwise.
  const respond = async (prompt: string) => {
    setThinking(true);
    if (isLive) {
      const id = `a${Date.now()}`;
      setMessages((m) => [...m, { id, from: 'agent', text: '' }]);
      try {
        await streamSend(projectId, prompt, (delta) =>
          setMessages((m) => m.map((x) => (x.id === id ? { ...x, text: x.text + delta } : x))),
        );
      } catch {
        setMessages((m) => m.map((x) => (x.id === id && !x.text ? { ...x, text: 'Lost the connection to the orchestrator. Try again.' } : x)));
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
  useEffect(() => {
    if (initialPrompt && !startedRef.current) {
      startedRef.current = true;
      void respond(initialPrompt);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialPrompt]);

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
                <Box sx={{ minWidth: 0 }}>
                  <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.7rem', color: 'text.disabled', mb: 0.5 })}>Orchestrator</Typography>
                  <Typography sx={{ fontSize: '0.9rem', lineHeight: 1.55 }}>{m.text}</Typography>
                  {m.steps && (
                    <Stack spacing={0.5} sx={{ mt: 1.25 }}>
                      {m.steps.map((s) => (
                        <Stack key={s} direction="row" spacing={1} alignItems="center">
                          <Box component="span" sx={{ color: 'success.main', fontSize: '0.8rem' }}>✓</Box>
                          <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.78rem', color: 'text.secondary' })}>{s}</Typography>
                        </Stack>
                      ))}
                    </Stack>
                  )}
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
