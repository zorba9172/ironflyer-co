import { createContext, useContext, useEffect, useState, type ReactNode } from 'react';
import { useRequest } from './provider';
import { ME, SIGN_IN, SIGN_UP, SIGN_OUT } from './operations';

const TOKEN_KEY = 'if-token';

export interface AuthUser {
  id: string;
  email: string;
  name?: string | null;
  plan?: string | null;
}

interface AuthContextValue {
  /** an endpoint is configured (we can talk to the orchestrator) */
  online: boolean;
  /** initial session check finished */
  ready: boolean;
  user: AuthUser | null;
  signIn: (email: string, password: string) => Promise<void>;
  signUp: (email: string, password: string, name?: string) => Promise<void>;
  signOut: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const request = useRequest();
  const [user, setUser] = useState<AuthUser | null>(null);
  const [ready, setReady] = useState(false);

  // Restore the session from a stored token when online.
  useEffect(() => {
    let alive = true;
    if (!request) {
      setReady(true);
      return;
    }
    const token = typeof localStorage !== 'undefined' ? localStorage.getItem(TOKEN_KEY) : null;
    if (!token) {
      setReady(true);
      return;
    }
    request<{ me: AuthUser | null }>('Me', ME)
      .then((d) => alive && setUser(d.me))
      .catch(() => localStorage.removeItem(TOKEN_KEY))
      .finally(() => alive && setReady(true));
    return () => {
      alive = false;
    };
  }, [request]);

  const signIn = async (email: string, password: string) => {
    if (!request) throw new Error('No orchestrator endpoint configured.');
    const d = await request<{ signIn: { token: string; user: AuthUser } }>('SignIn', SIGN_IN, { input: { email, password } });
    localStorage.setItem(TOKEN_KEY, d.signIn.token);
    setUser(d.signIn.user);
  };

  const signUp = async (email: string, password: string, name?: string) => {
    if (!request) throw new Error('No orchestrator endpoint configured.');
    const d = await request<{ signUp: { token: string; user: AuthUser } }>('SignUp', SIGN_UP, { input: { email, password, name } });
    localStorage.setItem(TOKEN_KEY, d.signUp.token);
    setUser(d.signUp.user);
  };

  const signOut = async () => {
    try {
      if (request) await request('SignOut', SIGN_OUT);
    } catch {
      /* best effort */
    }
    localStorage.removeItem(TOKEN_KEY);
    setUser(null);
  };

  return <AuthContext.Provider value={{ online: !!request, ready, user, signIn, signUp, signOut }}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
}
