import { useState } from 'react';
import { Box, Button, Stack, TextField, Typography } from '@mui/material';
import { useAuth } from '@ironflyer/data';
import { LogoMark } from '../components/LogoMark';

// Shown only when an orchestrator endpoint is configured but there's no session.
export function Login() {
  const { signIn } = useAuth();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setBusy(true);
    try {
      await signIn(email.trim(), password);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Sign in failed.');
    } finally {
      setBusy(false);
    }
  };

  return (
    <Box sx={{ minHeight: '100vh', display: 'grid', placeItems: 'center', bgcolor: 'background.default', p: 3 }}>
      <Box sx={(t) => ({ position: 'fixed', inset: '-30% 0 auto 0', height: 480, background: `radial-gradient(50% 50% at 50% 0%, ${t.brand.accent.primary}24, transparent 70%)`, pointerEvents: 'none' })} />
      <Box component="form" onSubmit={submit} sx={{ position: 'relative', width: '100%', maxWidth: 380, border: 1, borderColor: 'divider', borderRadius: 4, bgcolor: 'background.paper', p: 4 }}>
        <Stack alignItems="center" spacing={1.5} sx={{ mb: 3 }}>
          <LogoMark size={36} />
          <Typography variant="h5" sx={{ fontSize: '1.4rem' }}>Sign in to Ironflyer</Typography>
          <Typography sx={{ color: 'text.secondary', fontSize: '0.88rem', textAlign: 'center' }}>Finish what your AI started.</Typography>
        </Stack>

        <Stack spacing={2}>
          <TextField label="Email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} fullWidth size="small" autoFocus required />
          <TextField label="Password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} fullWidth size="small" required />
          {error && <Typography sx={{ color: 'error.main', fontSize: '0.82rem' }}>{error}</Typography>}
          <Button type="submit" variant="contained" disabled={busy || !email || !password} size="large">
            {busy ? 'Signing in…' : 'Sign in'}
          </Button>
        </Stack>
      </Box>
    </Box>
  );
}
