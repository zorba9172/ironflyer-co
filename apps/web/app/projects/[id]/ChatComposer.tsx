'use client';

import { useEffect, useRef, useState } from 'react';
import {
  Add, ArrowUpward, AutoAwesome, Bolt, Chat as ChatIcon, Mic, MicOff, Stop,
} from '@mui/icons-material';
import {
  Box, IconButton, Menu, MenuItem, Stack, TextField, Tooltip, Typography,
} from '@mui/material';
import { tokens } from '../../../lib/theme';

export type ComposerMode = 'build' | 'chat' | 'plan';

const MODE_LABEL: Record<ComposerMode, string> = {
  build: 'Build',
  chat: 'Chat',
  plan: 'Plan',
};

const MODE_DESCRIPTION: Record<ComposerMode, string> = {
  build: 'Stream code edits + run the finisher gates on commit.',
  chat: 'Conversation only — no file mutations or gate runs.',
  plan: 'Multi-model bake-off; agent debates the goal before acting.',
};

interface Props {
  value: string;
  onChange: (v: string) => void;
  onSend: (mode: ComposerMode) => void;
  onAbort?: () => void;
  streaming: boolean;
  attachments?: string[];
  onRemoveAttachment?: (path: string) => void;
  onAddAttachment?: () => void;
  disabled?: boolean;
  placeholder?: string;
}

// ChatComposer is the Lovable-style input row: a mode pill (Build/Chat/Plan)
// selects how Send dispatches, an attach button surfaces context files, and
// a microphone toggles Web Speech transcription when available.
export function ChatComposer({
  value, onChange, onSend, onAbort, streaming,
  attachments = [], onRemoveAttachment, onAddAttachment,
  disabled, placeholder,
}: Props) {
  const [mode, setMode] = useState<ComposerMode>('build');
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const [listening, setListening] = useState(false);
  const recognitionRef = useRef<SpeechRecognitionLike | null>(null);

  const canSubmit = value.trim().length > 0 && !disabled && !streaming;

  function submit() {
    if (!canSubmit) return;
    onSend(mode);
  }

  function toggleListening() {
    if (typeof window === 'undefined') return;
    const W = window as unknown as { SpeechRecognition?: SpeechRecognitionCtor; webkitSpeechRecognition?: SpeechRecognitionCtor };
    const Ctor = W.SpeechRecognition ?? W.webkitSpeechRecognition;
    if (!Ctor) return; // Browser doesn't support — handled by `available` check.
    if (listening) {
      recognitionRef.current?.stop();
      setListening(false);
      return;
    }
    const rec = new Ctor();
    rec.lang = navigator.language || 'en-US';
    rec.interimResults = true;
    rec.continuous = true;
    const seed = value;
    rec.onresult = (ev) => {
      let transcript = '';
      for (let i = 0; i < ev.results.length; i++) {
        transcript += ev.results[i][0].transcript;
      }
      onChange((seed ? seed + ' ' : '') + transcript.trim());
    };
    rec.onend = () => setListening(false);
    rec.onerror = () => setListening(false);
    rec.start();
    recognitionRef.current = rec;
    setListening(true);
  }

  useEffect(() => () => recognitionRef.current?.stop(), []);

  const voiceAvailable = typeof window !== 'undefined' &&
    !!((window as unknown as { SpeechRecognition?: unknown; webkitSpeechRecognition?: unknown })
       .SpeechRecognition ||
       (window as unknown as { webkitSpeechRecognition?: unknown }).webkitSpeechRecognition);

  return (
    <Box sx={{ p: 1.4 }}>
      {/* Attachment pills */}
      {attachments.length > 0 && (
        <Stack direction="row" flexWrap="wrap" spacing={0.6} sx={{ mb: 0.8 }}>
          {attachments.map((path) => (
            <Box key={path} sx={{
              display: 'inline-flex', alignItems: 'center', gap: 0.5,
              bgcolor: tokens.color.bg.inset, color: tokens.color.text.primary,
              fontFamily: tokens.font.mono, fontSize: 11,
              borderRadius: 1, px: 0.8, py: 0.3,
            }}>
              <span>{path}</span>
              {onRemoveAttachment && (
                <IconButton size="small" onClick={() => onRemoveAttachment(path)}
                            sx={{ ml: 0.3, p: 0.1, color: tokens.color.text.muted }}>
                  ✕
                </IconButton>
              )}
            </Box>
          ))}
        </Stack>
      )}

      <Box sx={{
        borderRadius: 2.2, bgcolor: tokens.color.bg.inset,
        border: `1px solid ${tokens.color.border.subtle}`,
        '&:focus-within': { borderColor: tokens.color.accent.lime },
        transition: `border-color ${tokens.motion.base} ${tokens.motion.curve}`,
      }}>
        <TextField
          fullWidth multiline minRows={2} maxRows={10}
          placeholder={placeholder ?? 'Ask Ironflyer to build, change, or explain…'}
          value={value} onChange={(e) => onChange(e.target.value)} disabled={disabled}
          variant="standard"
          InputProps={{ disableUnderline: true,
            sx: { px: 1.4, pt: 1, fontSize: 14 } }}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
              e.preventDefault(); submit();
            }
          }}
        />

        <Stack direction="row" alignItems="center" sx={{ px: 0.6, pb: 0.6 }}>
          <Tooltip title="Attach a workspace file as context">
            <IconButton size="small" onClick={onAddAttachment} disabled={!onAddAttachment}
                        sx={{ color: tokens.color.text.muted }}>
              <Add fontSize="small" />
            </IconButton>
          </Tooltip>

          <Tooltip title={voiceAvailable ? (listening ? 'Stop dictation' : 'Dictate') : 'Voice input unavailable in this browser'}>
            <span>
              <IconButton size="small" onClick={toggleListening} disabled={!voiceAvailable}
                          sx={{ color: listening ? tokens.color.accent.lime : tokens.color.text.muted }}>
                {listening ? <MicOff fontSize="small" /> : <Mic fontSize="small" />}
              </IconButton>
            </span>
          </Tooltip>

          <Box sx={{ flex: 1 }} />

          {/* Mode pill — Build / Chat / Plan */}
          <Box
            onClick={(e) => setMenuAnchor(e.currentTarget)}
            sx={{
              cursor: 'pointer', display: 'inline-flex', alignItems: 'center', gap: 0.6,
              px: 1, py: 0.5, borderRadius: 1.2,
              bgcolor: tokens.color.bg.surface, color: tokens.color.text.primary,
              fontSize: 12, fontWeight: 700,
              border: `1px solid ${tokens.color.border.subtle}`,
              '&:hover': { borderColor: tokens.color.accent.lime },
            }}>
            {iconForMode(mode)}
            <Typography variant="caption" sx={{ fontWeight: 700 }}>{MODE_LABEL[mode]}</Typography>
            <Typography variant="caption" sx={{ color: tokens.color.text.muted }}>▾</Typography>
          </Box>
          <Menu open={!!menuAnchor} anchorEl={menuAnchor} onClose={() => setMenuAnchor(null)}>
            {(['build', 'chat', 'plan'] as ComposerMode[]).map((m) => (
              <MenuItem key={m} selected={mode === m} onClick={() => { setMode(m); setMenuAnchor(null); }}
                        sx={{ alignItems: 'flex-start', maxWidth: 320 }}>
                <Stack direction="row" spacing={1.2}>
                  <Box sx={{ mt: 0.4 }}>{iconForMode(m)}</Box>
                  <Stack spacing={0.2}>
                    <Typography variant="body2" fontWeight={700}>{MODE_LABEL[m]}</Typography>
                    <Typography variant="caption" color="text.secondary">{MODE_DESCRIPTION[m]}</Typography>
                  </Stack>
                </Stack>
              </MenuItem>
            ))}
          </Menu>

          <Box sx={{ width: 6 }} />

          {streaming ? (
            <Tooltip title="Stop">
              <IconButton onClick={onAbort} size="small"
                          sx={{ bgcolor: tokens.color.bg.surface, color: tokens.color.text.primary,
                                '&:hover': { bgcolor: tokens.color.bg.surfaceHover } }}>
                <Stop fontSize="small" />
              </IconButton>
            </Tooltip>
          ) : (
            <Tooltip title="Send (⌘↵)">
              <span>
                <IconButton onClick={submit} disabled={!canSubmit} size="small"
                            sx={{
                              bgcolor: canSubmit ? tokens.color.accent.lime : tokens.color.bg.surface,
                              color: canSubmit ? '#0d0e0f' : tokens.color.text.muted,
                              '&:hover': { bgcolor: canSubmit ? '#c7df00' : tokens.color.bg.surface },
                              '&.Mui-disabled': { color: tokens.color.text.muted, bgcolor: tokens.color.bg.surface },
                            }}>
                  <ArrowUpward fontSize="small" />
                </IconButton>
              </span>
            </Tooltip>
          )}
        </Stack>
      </Box>
    </Box>
  );
}

function iconForMode(m: ComposerMode) {
  switch (m) {
    case 'build': return <Bolt fontSize="small" />;
    case 'chat':  return <ChatIcon fontSize="small" />;
    case 'plan':  return <AutoAwesome fontSize="small" />;
  }
}

// Minimal SpeechRecognition typings — the standard lib doesn't ship these
// in the DOM types yet, and we only need a tiny slice.
interface SpeechRecognitionLike {
  lang: string;
  continuous: boolean;
  interimResults: boolean;
  onresult: (ev: { results: ArrayLike<ArrayLike<{ transcript: string }>> }) => void;
  onend: () => void;
  onerror: (ev: unknown) => void;
  start: () => void;
  stop: () => void;
}
type SpeechRecognitionCtor = new () => SpeechRecognitionLike;
