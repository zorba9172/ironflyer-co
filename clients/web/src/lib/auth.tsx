"use client";

// AuthProvider — the single React surface for "who is the caller".
//
// State: `{ user, loading, authenticated, signIn, signUp, signOut,
// refresh, tenantID }` exposed through useAuth(). The JWT itself is
// owned by src/lib/apollo.tsx so the Apollo link can read it without
// going through React. After every auth-changing event we reset the
// Apollo store so cached data from the previous identity cannot leak.
//
// Typed operations come from the codegen output
// (src/lib/gql/__generated__.ts).

import { useApolloClient } from "@apollo/client";
import { useRouter } from "next/navigation";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import {
  apolloClient,
  clearToken,
  getToken,
  registerUnauthorizedHandler,
  setToken,
} from "./apollo";
import { extractErrorMessage } from "./errors";
import {
  CurrentUserDocument,
  type CurrentUserQuery,
  useCurrentUserQuery,
  useSignInMutation,
  useSignOutMutation,
  useSignUpMutation,
  type SignInInput,
  type SignUpInput,
} from "./gql/__generated__";

// Re-export so callers can `import { getToken } from "@/lib/auth"`.
export { getToken, setToken, clearToken } from "./apollo";

export type AuthUser = NonNullable<CurrentUserQuery["me"]>;

export interface AuthSession {
  token: string;
  expiresAt?: string | null;
  user: AuthUser;
}

export interface AuthContextValue {
  user: AuthUser | null;
  loading: boolean;
  // True only when a token exists. Use this to decide whether to render
  // the signed-in shell or the auth surface.
  authenticated: boolean;
  // Backend convention: tenantID == orgId when an org exists, else
  // user.id. Matches every per-tenant query (wallet, executions, …).
  tenantID: string | null;
  signIn: (input: SignInInput) => Promise<AuthSession>;
  signUp: (input: SignUpInput) => Promise<AuthSession>;
  signOut: () => Promise<void>;
  refresh: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const router = useRouter();
  const client = useApolloClient();

  const [hydrated, setHydrated] = useState(false);
  const [hasToken, setHasToken] = useState(false);

  // Hydrate after mount so the server and the first client render both
  // start from the same unauthenticated shell. This prevents cockpit
  // nav/marketing nav hydration mismatches when a cookie exists.
  useEffect(() => {
    setHasToken(!!getToken());
    setHydrated(true);
  }, []);

  // Wire 401 → clear local session + bounce to /login. Survives any
  // tab the user has open.
  useEffect(() => {
    registerUnauthorizedHandler(() => {
      clearToken();
      setHasToken(false);
      void apolloClient.clearStore().catch(() => undefined);
      if (typeof window === "undefined") return;
      const path = window.location.pathname;
      if (path.startsWith("/login") || path.startsWith("/signup")) return;
      router.replace("/login");
    });
  }, [router]);

  const { data, loading, refetch } = useCurrentUserQuery({
    skip: !hydrated || !hasToken,
    fetchPolicy: "cache-and-network",
    notifyOnNetworkStatusChange: true,
  });

  const [signInMutation] = useSignInMutation();
  const [signUpMutation] = useSignUpMutation();
  const [signOutMutation] = useSignOutMutation();

  const signIn = useCallback(
    async (input: SignInInput): Promise<AuthSession> => {
      try {
        const res = await signInMutation({ variables: { input } });
        const session = res.data?.signIn;
        if (!session) throw new Error("Sign-in failed: empty response");
      setToken(session.token);
      setHydrated(true);
      setHasToken(true);
        await client.resetStore().catch(() => undefined);
        return session as AuthSession;
      } catch (e) {
        throw new Error(extractErrorMessage(e));
      }
    },
    [signInMutation, client],
  );

  const signUp = useCallback(
    async (input: SignUpInput): Promise<AuthSession> => {
      try {
        const res = await signUpMutation({ variables: { input } });
        const session = res.data?.signUp;
        if (!session) throw new Error("Sign-up failed: empty response");
      setToken(session.token);
      setHydrated(true);
      setHasToken(true);
        await client.resetStore().catch(() => undefined);
        return session as AuthSession;
      } catch (e) {
        throw new Error(extractErrorMessage(e));
      }
    },
    [signUpMutation, client],
  );

  const signOut = useCallback(async (): Promise<void> => {
    try {
      await signOutMutation();
    } catch {
      // Swallow — we still want to clear local state even if the
      // server-side revoke RPC fails (offline, expired token, etc.).
    }
    clearToken();
    setHydrated(true);
    setHasToken(false);
    await client.clearStore().catch(() => undefined);
    if (typeof window !== "undefined") router.replace("/login");
  }, [signOutMutation, client, router]);

  const refresh = useCallback(async (): Promise<void> => {
    if (!getToken()) return;
    await refetch();
  }, [refetch]);

  const user = hydrated && hasToken ? data?.me ?? null : null;
  const tenantID = user?.orgId || user?.id || null;

  const value = useMemo<AuthContextValue>(
    () => ({
      user,
      loading: !hydrated || (hasToken ? loading : false),
      authenticated: hydrated && hasToken,
      tenantID,
      signIn,
      signUp,
      signOut,
      refresh,
    }),
    [user, hydrated, hasToken, loading, tenantID, signIn, signUp, signOut, refresh],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used inside <AuthProvider>");
  return ctx;
}

// RequireAuth — wrap any client-side page that needs a session. While
// the token is hydrating or the `me` query is in-flight we render a
// neutral fallback so we don't flash the marketing surface.

import { Box, CircularProgress } from "@mui/material";

export function RequireAuth({
  children,
  fallback,
}: {
  children: ReactNode;
  fallback?: ReactNode;
}) {
  const { authenticated, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!authenticated && !loading) router.replace("/login");
  }, [authenticated, loading, router]);

  if (!authenticated || loading) {
    return (
      <>
        {fallback ?? (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              minHeight: "60vh",
            }}
          >
            <CircularProgress size={28} />
          </Box>
        )}
      </>
    );
  }
  return <>{children}</>;
}

// Re-exported so RequireAuth and other modules can issue the underlying
// document where a typed hook is not flexible enough.
export { CurrentUserDocument };

// Re-export from src/lib/hooks so existing call sites that import
// `useIsOperator` from "@/lib/auth" keep working without ripple
// refactors. Canonical implementation lives in src/lib/hooks.
export { useIsOperator } from "./hooks/useIsOperator";
