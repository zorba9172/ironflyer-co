'use client';

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import { Box, Stack, Typography } from '@mui/material';
import { auth, AuthUser } from '../lib/auth';
import { tokens } from '../lib/theme';
import { IronflyerMark } from '../components/brand/IronflyerLogo';

interface AuthCtx {
  user: AuthUser | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<void>;
  signup: (email: string, password: string, name?: string) => Promise<void>;
  logout: () => void;
}

const Ctx = createContext<AuthCtx>({
  user: null, loading: true,
  login: async () => {}, signup: async () => {}, logout: () => {},
});

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(() => auth.user());
  const [loading, setLoading] = useState(() => !auth.user());

  // Hydrate order: (1) cache → (2) absorb GitHub OAuth token from URL hash
  // if present → (3) verify with /auth/me.
  useEffect(() => {
    const cached = auth.user();
    if (cached) {
      setUser(cached);
      setLoading(false);
    }
    (async () => {
      try {
        const fromHash = await auth.adoptTokenFromHash();
        if (fromHash) {
          setUser(fromHash);
          setLoading(false);
          return;
        }
        const u = await auth.me();
        setUser(u ?? cached);
      } catch {
        setUser(cached);
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  const login = useCallback(async (email: string, password: string) => {
    setUser(await auth.login(email, password));
  }, []);
  const signup = useCallback(async (email: string, password: string, name?: string) => {
    setUser(await auth.signup(email, password, name));
  }, []);
  const logout = useCallback(() => {
    auth.clear();
    setUser(null);
    if (typeof window !== 'undefined') window.location.href = '/login';
  }, []);

  const value = useMemo(() => ({ user, loading, login, signup, logout }),
    [user, loading, login, signup, logout]);

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useAuth() { return useContext(Ctx); }

// Guard wraps a route that requires a logged-in user. While loading we render
// nothing; if unauthenticated, redirect to /login.
export function RequireAuth({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth();
  useEffect(() => {
    if (!loading && !user && typeof window !== 'undefined') {
      window.location.href = '/login';
    }
  }, [user, loading]);
  if (loading || !user) return <AuthLoading />;
  return <>{children}</>;
}

function AuthLoading() {
  return (
    <Box sx={{ minHeight: '100vh', display: 'grid', placeItems: 'center', bgcolor: tokens.color.bg.base }}>
      <Stack spacing={1.5} alignItems="center">
        <IronflyerMark size={42} tone="dark" />
        <Typography variant="body2" color="text.secondary">Opening Ironflyer workspace...</Typography>
      </Stack>
    </Box>
  );
}
