import { useState } from 'react';
import { Box, Button, Divider, Link, Stack, TextField, Typography } from '@mui/material';
import { LuShieldCheck, LuWallet, LuGitPullRequestArrow } from 'react-icons/lu';
import { useAuth } from '@ironflyer/data';
import { AmbientBackdrop } from './home/AmbientBackdrop';
import { NeonLogomark } from './auth/NeonLogomark';

// Shown only when an orchestrator endpoint is configured but there's no session.
// Premium "Neon Intelligence" auth surface, consistent with the locked landing:
// an AmbientBackdrop behind a centered glass card with the neon logomark, the
// sign-in / sign-up form, and the gradient primary CTA.
export function Login() {
  const { signIn, signUp } = useAuth();
  const [mode, setMode] = useState<'in' | 'up'>('in');
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setBusy(true);
    try {
      if (mode === 'in') await signIn(email.trim(), password);
      else await signUp(email.trim(), password, name.trim() || undefined);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Authentication failed.');
    } finally {
      setBusy(false);
    }
  };

  // Shared field chrome: glass fill, calm hairline, neon-blue focus rim.
  const fieldSx = (theme: import('@mui/material').Theme) => ({
    '& .MuiOutlinedInput-root': {
      borderRadius: `${theme.studio.radius.sm}px`,
      backgroundColor: theme.palette.surfaceRaised,
      transition: `border-color ${theme.studio.motion.fast}, box-shadow ${theme.studio.motion.fast}`,
      '& fieldset': { borderColor: theme.palette.divider },
      '&:hover fieldset': { borderColor: theme.palette.text.secondary },
      '&.Mui-focused fieldset': { borderColor: theme.studio.neon.blue, borderWidth: 1 },
      '&.Mui-focused': { boxShadow: `0 0 0 3px ${theme.studio.neon.blue}22` },
    },
    '& .MuiInputLabel-root.Mui-focused': { color: theme.studio.neon.blue },
  });

  return (
    <Box
      sx={(theme) => ({
        position: 'relative',
        minHeight: '100vh',
        display: 'grid',
        placeItems: 'center',
        bgcolor: theme.palette.background.default,
        color: theme.palette.text.primary,
        p: { xs: 2.5, sm: 3 },
        overflow: 'hidden',
      })}
    >
      <AmbientBackdrop />

      <Box
        component="form"
        onSubmit={submit}
        sx={(theme) => ({
          position: 'relative',
          zIndex: 1,
          width: '100%',
          maxWidth: 420,
          p: { xs: 3.5, sm: 4.5 },
          borderRadius: `${theme.studio.radius.xl}px`,
          border: `1px solid ${theme.palette.cardBorder}`,
          backgroundColor: theme.palette.background.paper,
          backdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
          WebkitBackdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
          boxShadow: theme.studio.effect.promptBuilder.glow,
          transition: `background-color ${theme.studio.motion.base}, border-color ${theme.studio.motion.base}`,
        })}
      >
        <Stack alignItems="center" spacing={2} sx={{ mb: 3.5 }}>
          <NeonLogomark size={44} />
          <Stack spacing={0.75} alignItems="center">
            <Typography
              variant="h5"
              sx={{ fontWeight: 800, letterSpacing: '-0.01em', textAlign: 'center' }}
            >
              {mode === 'in' ? 'Sign in to Ironflyer' : 'Create your account'}
            </Typography>
            <Typography
              variant="body2"
              sx={(theme) => ({ color: theme.palette.text.secondary, textAlign: 'center' })}
            >
              Finish what your AI started.
            </Typography>
          </Stack>
        </Stack>

        <Stack spacing={2}>
          {mode === 'up' && (
            <TextField
              label="Name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              fullWidth
              sx={fieldSx}
            />
          )}
          <TextField
            label="Email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            fullWidth
            autoFocus
            required
            sx={fieldSx}
          />
          <TextField
            label="Password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            fullWidth
            required
            sx={fieldSx}
          />

          {error && (
            <Typography
              variant="caption"
              role="alert"
              sx={(theme) => ({
                display: 'block',
                px: 1.5,
                py: 1,
                borderRadius: `${theme.studio.radius.sm}px`,
                color: theme.studio.neon.danger,
                border: `1px solid ${theme.studio.neon.danger}33`,
                backgroundColor: `${theme.studio.neon.danger}14`,
              })}
            >
              {error}
            </Typography>
          )}

          <Button
            type="submit"
            variant="contained"
            color="primary"
            disabled={busy || !email || !password}
            size="large"
            sx={(theme) => ({
              height: `${theme.studio.effect.cta.height}px`,
              borderRadius: `${theme.studio.effect.cta.radius}px`,
              fontSize: '1rem',
              fontWeight: 700,
            })}
          >
            {busy ? 'Working…' : mode === 'in' ? 'Sign in' : 'Create account'}
          </Button>

          <Typography
            variant="body2"
            sx={(theme) => ({ textAlign: 'center', color: theme.palette.text.secondary })}
          >
            {mode === 'in' ? 'No account? ' : 'Have an account? '}
            <Link
              component="button"
              type="button"
              onClick={() => {
                setMode(mode === 'in' ? 'up' : 'in');
                setError('');
              }}
              sx={(theme) => ({
                color: theme.studio.neon.blue,
                fontWeight: 600,
                textDecoration: 'none',
                '&:hover': { textDecoration: 'underline' },
              })}
            >
              {mode === 'in' ? 'Create one' : 'Sign in'}
            </Link>
          </Typography>
        </Stack>

        <Divider sx={(theme) => ({ my: 3, borderColor: theme.palette.divider })} />

        {/* Proof row — the production discipline that sets Ironflyer apart. */}
        <Stack direction="row" justifyContent="space-between" spacing={1}>
          {[
            { icon: <LuShieldCheck size={16} />, label: 'Gated reviews' },
            { icon: <LuWallet size={16} />, label: 'Prepaid wallet' },
            { icon: <LuGitPullRequestArrow size={16} />, label: 'Reviewable patches' },
          ].map((item) => (
            <Stack key={item.label} alignItems="center" spacing={0.75} sx={{ flex: 1, minWidth: 0 }}>
              <Box
                sx={(theme) => ({
                  display: 'grid',
                  placeItems: 'center',
                  width: 34,
                  height: 34,
                  borderRadius: `${theme.studio.radius.sm}px`,
                  color: theme.studio.neon.blue,
                  border: `1px solid ${theme.palette.cardBorder}`,
                  backgroundColor: theme.palette.cardBg,
                })}
              >
                {item.icon}
              </Box>
              <Typography
                variant="caption"
                sx={(theme) => ({
                  color: theme.palette.text.secondary,
                  textAlign: 'center',
                  lineHeight: 1.2,
                })}
              >
                {item.label}
              </Typography>
            </Stack>
          ))}
        </Stack>
      </Box>
    </Box>
  );
}
