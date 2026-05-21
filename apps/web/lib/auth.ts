'use client';

// Web auth client. Stores the JWT in localStorage (good enough for dev;
// production should move to an httpOnly cookie issued by Next API routes).
//
// The orchestrator's middleware accepts the token either via
//   Authorization: Bearer <jwt>
// or via a ?token=<jwt> query string (needed for EventSource which can't
// set headers).

export interface AuthUser {
  id: string;
  email: string;
  name?: string;
  plan?: string;
  createdAt?: string;
}

const TOKEN_KEY = 'ironflyer.token';
const USER_KEY = 'ironflyer.user';

const base = '/api/orchestrator';

function safeGetItem(k: string): string | null {
  if (typeof window === 'undefined') return null;
  try { return window.localStorage.getItem(k); } catch { return null; }
}
function safeSetItem(k: string, v: string) {
  if (typeof window === 'undefined') return;
  try { window.localStorage.setItem(k, v); } catch {}
}
function safeRemoveItem(k: string) {
  if (typeof window === 'undefined') return;
  try { window.localStorage.removeItem(k); } catch {}
}

export const auth = {
  token(): string | null { return safeGetItem(TOKEN_KEY); },
  user(): AuthUser | null {
    const raw = safeGetItem(USER_KEY);
    if (!raw) return null;
    try { return JSON.parse(raw) as AuthUser; } catch { return null; }
  },
  isAuthenticated(): boolean { return !!safeGetItem(TOKEN_KEY); },
  setSession(token: string, user: AuthUser) {
    safeSetItem(TOKEN_KEY, token);
    safeSetItem(USER_KEY, JSON.stringify(user));
  },
  clear() {
    safeRemoveItem(TOKEN_KEY);
    safeRemoveItem(USER_KEY);
  },

  async login(email: string, password: string): Promise<AuthUser> {
    const res = await fetch(`${base}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password }),
    });
    if (!res.ok) throw new Error(await prettifyError(res));
    const { token, user } = await res.json();
    this.setSession(token, user);
    return user;
  },

  async signup(email: string, password: string, name?: string): Promise<AuthUser> {
    const res = await fetch(`${base}/auth/signup`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password, name }),
    });
    if (!res.ok) throw new Error(await prettifyError(res));
    const { token, user } = await res.json();
    this.setSession(token, user);
    return user;
  },

  async me(): Promise<AuthUser | null> {
    const tok = this.token();
    if (!tok) return null;
    const res = await fetch(`${base}/auth/me`, {
      headers: { Authorization: `Bearer ${tok}` },
      cache: 'no-store',
    });
    if (res.status === 401) {
      this.clear();
      return null;
    }
    if (!res.ok) return null;
    const u = await res.json();
    safeSetItem(USER_KEY, JSON.stringify(u));
    return u as AuthUser;
  },

  // authHeader is the helper used by all API clients to attach the Bearer
  // header to outgoing requests.
  authHeader(): Record<string, string> {
    const t = safeGetItem(TOKEN_KEY);
    return t ? { Authorization: `Bearer ${t}` } : {};
  },

  // appendTokenParam decorates URLs for SSE EventSource (which cannot set
  // headers) by appending ?token=<jwt>.
  appendTokenParam(url: string): string {
    const t = safeGetItem(TOKEN_KEY);
    if (!t) return url;
    return url + (url.includes('?') ? '&' : '?') + 'token=' + encodeURIComponent(t);
  },

  /**
   * adoptTokenFromHash inspects window.location.hash for `token=<jwt>` that
   * the orchestrator places after a GitHub OAuth login. When found, the token
   * is stored, /auth/me is called to hydrate the user object, the hash is
   * scrubbed from the URL, and the freshly-loaded user is returned. Returns
   * null when nothing in the hash matches.
   */
  async adoptTokenFromHash(): Promise<AuthUser | null> {
    if (typeof window === 'undefined') return null;
    const raw = window.location.hash.startsWith('#')
      ? window.location.hash.slice(1)
      : window.location.hash;
    if (!raw) return null;
    const params = new URLSearchParams(raw);
    const token = params.get('token');
    if (!token) return null;
    safeSetItem(TOKEN_KEY, token);
    // Strip the hash before /auth/me so reloads don't replay it.
    window.history.replaceState(null, '', window.location.pathname + window.location.search);
    const u = await this.me();
    return u;
  },
};

async function prettifyError(res: Response): Promise<string> {
  try {
    const data = await res.json();
    return data.error || `${res.status} ${res.statusText}`;
  } catch {
    return `${res.status} ${res.statusText}`;
  }
}
