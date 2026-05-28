import { useEffect, useRef, useState } from 'react';
import { Avatar, Box, IconButton, InputBase, Stack, Typography } from '@mui/material';

interface Msg { id: string; from: 'user' | 'agent'; text: string; steps?: string[] }

const demoSeed: Msg[] = [
  { id: 'm1', from: 'user', text: 'Import the Northwind checkout from Lovable and tell me what is missing.' },
  {
    id: 'm2', from: 'agent',
    text: 'Imported and ran it in a sandbox. Six finisher gates mapped — two are open and one is blocked. Start with Money: the Stripe webhook is unverified.',
    steps: ['Read entities/Order', 'Read routes/checkout', 'Scanned secrets', 'Mapped gates'],
  },
];

export function ChatPanel({ initialPrompt }: { initialPrompt?: string }) {
  const [messages, setMessages] = useState<Msg[]>(() =>
    initialPrompt ? [{ id: 'p0', from: 'user', text: initialPrompt }] : demoSeed,
  );
  const [draft, setDraft] = useState('');
  const [thinking, setThinking] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);

  // When the session started from a composer prompt, simulate the first read.
  useEffect(() => {
    if (!initialPrompt) return;
    setThinking(true);
    const t = setTimeout(() => {
      setMessages((m) => [...m, {
        id: 'a0', from: 'agent',
        text: 'Reading your project and mapping the six finisher gates. Switch to the Dashboard to watch them resolve — I will start with whatever is blocking the next deploy.',
        steps: ['Cloned into sandbox', 'Detected stack', 'Mapped gates', 'Scored completion'],
      }]);
      setThinking(false);
    }, 1100);
    return () => clearTimeout(t);
  }, [initialPrompt]);

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: 'smooth' });
  }, [messages, thinking]);

  const send = () => {
    const text = draft.trim();
    if (!text) return;
    setMessages((m) => [...m, { id: `u${m.length}`, from: 'user', text }]);
    setDraft('');
    setThinking(true);
    setTimeout(() => {
      setMessages((m) => [...m, { id: `a${m.length}`, from: 'agent', text: 'On it — proposing a patch against your workspace. Review the diff before it applies.' }]);
      setThinking(false);
    }, 900);
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
            <Typography sx={{ fontSize: '0.78rem', color: 'text.disabled' }}>Discuss · ⏎ to send</Typography>
            <IconButton onClick={send} size="small" aria-label="Send" sx={(t) => ({ color: '#fff', backgroundImage: t.brand.gradient.signature, width: 30, height: 30 })}>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2"><path d="M5 12h14M13 6l6 6-6 6" /></svg>
            </IconButton>
          </Stack>
        </Box>
      </Box>
    </Box>
  );
}
