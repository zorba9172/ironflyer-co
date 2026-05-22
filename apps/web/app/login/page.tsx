'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import {
  Box, Button, Divider, Stack, Tab, Tabs, TextField, Typography,
} from '@mui/material';
import { GitHub, Google, Lock } from '@mui/icons-material';
import { useAuth } from '../auth-context';
import { auth as authStore } from '../../lib/auth';
import { tokens } from '../../lib/theme';
import { PromptBox } from '../prompt-box';
import { githubLoginStartURL } from '../../lib/github';

// Allow VSCode and (later) Cursor / Windsurf URI schemes to round-trip the
// JWT back into a desktop client. Anything else is silently ignored — we
// don't want this page to be a token-exfiltration redirect.
const ALLOWED_CALLBACK_SCHEMES = ['vscode', 'vscode-insiders', 'cursor', 'windsurf'];
const CALLBACK_STORAGE_KEY = 'ironflyer.desktopCallback';

function readVscodeCallback(): string | null {
  if (typeof window === 'undefined') return null;
  const params = new URLSearchParams(window.location.search);
  let cb: string | null = null;
  if (params.get('source') === 'vscode') {
    cb = params.get('callback');
  }
  // GitHub OAuth round-trips through a fixed redirect that drops query
  // params — so we persist the callback before kicking GitHub off and
  // rehydrate it here. We never trust the stored value if its scheme is
  // not on the allowlist.
  if (!cb) cb = window.sessionStorage.getItem(CALLBACK_STORAGE_KEY);
  if (!cb) return null;
  try {
    const u = new URL(cb);
    const scheme = u.protocol.replace(/:$/, '');
    if (!ALLOWED_CALLBACK_SCHEMES.includes(scheme)) return null;
    return cb;
  } catch {
    return null;
  }
}

function persistDesktopCallback(): void {
  if (typeof window === 'undefined') return;
  const params = new URLSearchParams(window.location.search);
  if (params.get('source') === 'vscode') {
    const cb = params.get('callback');
    if (cb) window.sessionStorage.setItem(CALLBACK_STORAGE_KEY, cb);
  }
}

function redirectToCallback(callback: string, token: string): void {
  const sep = callback.includes('?') ? '&' : '?';
  window.location.href = `${callback}${sep}token=${encodeURIComponent(token)}`;
}

export default function LoginPage() {
  const { user, loading, login, signup } = useAuth();
  const [mode, setMode] = useState<'login' | 'signup'>('login');
  const [email, setEmail] = useState('demo@ironflyer.dev');
  const [password, setPassword] = useState('demo1234');
  const [name, setName] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [pendingIdea, setPendingIdea] = useState('Ask Ironflyer to build');

  useEffect(() => {
    const pending = window.localStorage.getItem('ironflyer.pendingIdea');
    if (pending) setPendingIdea(pending);
    persistDesktopCallback();
  }, []);

  useEffect(() => {
    if (loading || !user || typeof window === 'undefined') return;
    const callback = readVscodeCallback();
    const token = authStore.token();
    if (callback && token) {
      redirectToCallback(callback, token);
      return;
    }
    window.location.href = '/app';
  }, [user, loading]);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true); setError(null);
    try {
      if (mode === 'login') await login(email, password);
      else await signup(email, password, name);
      const callback = readVscodeCallback();
      const token = authStore.token();
      if (callback && token) {
        redirectToCallback(callback, token);
      } else {
        window.location.href = '/app';
      }
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Box sx={{
      minHeight: '100vh',
      display: 'grid',
      gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' },
      bgcolor: tokens.color.bg.alabaster,
      color: tokens.color.text.inverse,
    }}>
      <Box sx={{
        display: 'flex',
        alignItems: { xs: 'flex-start', md: 'center' },
        justifyContent: 'center',
        p: { xs: 3, md: 6 },
        pt: { xs: 8, md: 6 },
        pb: { xs: 6, md: 6 },
      }}>
        <Box sx={{
          width: '100%',
          maxWidth: 420,
          bgcolor: '#fffcf3',
          border: '1px solid rgba(13,14,15,0.06)',
          borderRadius: 4,
          p: { xs: 3.6, md: 4.8 },
          boxShadow: '0 24px 64px rgba(13,14,15,0.08)',
        }}>
          <Link href="/" style={{ color: 'inherit', textDecoration: 'none' }}>
            <Stack direction="row" alignItems="center" spacing={1.5} sx={{ mb: 5 }}>
              <Box sx={{ width: 34, height: 34, borderRadius: 1, bgcolor: tokens.color.accent.lime }} />
              <Typography variant="h5" sx={{ fontFamily: tokens.font.display, fontWeight: 400, letterSpacing: 0, textTransform: 'uppercase' }}>Ironflyer</Typography>
            </Stack>
          </Link>

          <Typography variant="h3" sx={{ mb: 3, letterSpacing: 0, textTransform: 'uppercase' }}>
            {mode === 'login' ? 'Log in' : 'Create account'}
          </Typography>

          <Stack spacing={1.2} sx={{ mb: 2.5 }}>
            <Button variant="outlined" fullWidth startIcon={<Google />} sx={socialButtonSx}>Continue with Google</Button>
            <Button
              variant="outlined" fullWidth startIcon={<GitHub />} sx={socialButtonSx}
              href={githubLoginStartURL}
            >
              Continue with GitHub
            </Button>
          </Stack>

          <Stack direction="row" spacing={2} alignItems="center" sx={{ my: 2.5 }}>
            <Divider sx={{ flex: 1 }} />
            <Typography variant="caption" color="text.secondary">Or</Typography>
            <Divider sx={{ flex: 1 }} />
          </Stack>

          <Tabs value={mode} onChange={(_, value) => setMode(value)} sx={{ mb: 2 }}>
            <Tab label="Sign in" value="login" />
            <Tab label="Sign up" value="signup" />
          </Tabs>

          <form onSubmit={submit}>
            <Stack spacing={1.6}>
              {mode === 'signup' && (
                <TextField label="Name" value={name} onChange={(event) => setName(event.target.value)} InputLabelProps={{ shrink: true }} />
              )}
              <TextField label="Email" type="email" value={email} onChange={(event) => setEmail(event.target.value)} required InputLabelProps={{ shrink: true }} />
              <TextField label="Password" type="password" value={password} onChange={(event) => setPassword(event.target.value)} required InputLabelProps={{ shrink: true }} />
              {error && <Typography variant="body2" color="error">{error}</Typography>}
              <Button type="submit" variant="contained" disabled={busy} fullWidth sx={{ py: 1.25 }}>
                {busy ? 'Working...' : mode === 'login' ? 'Continue' : 'Create workspace'}
              </Button>
            </Stack>
          </form>

          <Typography variant="body2" sx={{ mt: 2.5, color: '#555' }}>
            {mode === 'login' ? "Don't have an account? " : 'Already have an account? '}
            <Button onClick={() => setMode(mode === 'login' ? 'signup' : 'login')} sx={{ p: 0, minWidth: 0, color: '#111', textDecoration: 'underline' }}>
              {mode === 'login' ? 'Create your account' : 'Log in'}
            </Button>
          </Typography>

          {mode === 'login' && (
            <Typography variant="caption" sx={{ display: 'block', mt: 2, color: '#666' }}>
              Dev demo: <b>demo@ironflyer.dev</b> / <b>demo1234</b>
            </Typography>
          )}

          <Stack direction="row" spacing={1} alignItems="center" sx={{ mt: 3, color: '#666' }}>
            <Lock fontSize="small" />
            <Typography variant="caption">SSO available on Team and Enterprise plans</Typography>
          </Stack>
        </Box>
      </Box>

      <Box sx={{
        display: { xs: 'none', md: 'flex' },
        flexDirection: 'column',
        alignItems: 'flex-start',
        justifyContent: 'center',
        p: 8,
        gap: 4,
        position: 'relative',
        background: 'linear-gradient(140deg, #0d0e0f 0%, #15161a 60%, #1d1d1a 100%)',
        color: tokens.color.bg.alabaster,
        overflow: 'hidden',
      }}>
        <Box sx={{
          position: 'absolute',
          inset: 'auto -160px -200px auto',
          width: 520, height: 520,
          background: 'radial-gradient(circle, rgba(229,255,0,0.22), transparent 60%)',
          filter: 'blur(20px)',
          pointerEvents: 'none',
        }} />
        <Box sx={{ position: 'relative', maxWidth: 480 }}>
          <Typography variant="overline" sx={{ color: tokens.color.accent.lime, fontWeight: 900, letterSpacing: '0.14em' }}>
            Welcome back to Ironflyer
          </Typography>
          <Typography sx={{
            mt: 1.2,
            fontFamily: tokens.font.display,
            fontWeight: 400,
            fontSize: 42,
            lineHeight: 0.95,
            letterSpacing: '-0.01em',
            color: tokens.color.bg.alabaster,
          }}>
            Pick up where the finisher loop left off.
          </Typography>
          <Typography sx={{ mt: 2, color: '#cfc7b8', fontWeight: 500, fontSize: 16, lineHeight: 1.5 }}>
            Sign in to open your workspaces, gate runs, and budget ledger. Your draft prompt is preserved below — submit it after you log in.
          </Typography>
        </Box>
        <Box sx={{ width: '100%', maxWidth: 520, position: 'relative' }}>
          <PromptBox
            value={pendingIdea}
            onChange={setPendingIdea}
            size="preview"
            cta="Build"
            placeholder="Ask Ironflyer to build"
          />
        </Box>
      </Box>
    </Box>
  );
}

const socialButtonSx = {
  color: '#111',
  borderColor: 'rgba(17,17,17,0.25)',
  borderRadius: 1,
  py: 1,
  '&:hover': { borderColor: '#111', bgcolor: 'rgba(17,17,17,0.04)' },
};
