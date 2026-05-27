"use client";

// Apollo Client wiring for the Ironflyer orchestrator GraphQL API.
//
//   HTTP   queries + mutations   → /graphql
//   WS     subscriptions         → /graphql (graphql-transport-ws)
//
// Each request carries an Authorization: Bearer <token> header pulled
// from the long-lived cookie ("ironflyer_token"). WS connections pass
// the token through connectionParams AND a ?token= query fallback so
// proxies that strip custom WS headers still authenticate the
// handshake — this mirrors the orchestrator's ?token= acceptance path.
//
// Token lifecycle is owned by AuthProvider (src/lib/auth.tsx).
//
// Environment variables (in order of precedence):
//   NEXT_PUBLIC_GRAPHQL_URL     full http(s) URL to /graphql
//   NEXT_PUBLIC_GRAPHQL_WS_URL  full ws(s) URL to /graphql
//   NEXT_PUBLIC_IRONFLYER_API_URL  api base (we append /graphql)
//   NEXT_PUBLIC_IRONFLYER_WS_URL   ws base  (we append /graphql)
//   fallback: http://localhost:8080/graphql + ws://localhost:8080/graphql

import {
  ApolloClient,
  ApolloLink,
  ApolloProvider as ApolloLibProvider,
  HttpLink,
  InMemoryCache,
  from,
  split,
} from "@apollo/client";
import { setContext } from "@apollo/client/link/context";
import { onError } from "@apollo/client/link/error";
import { createPersistedQueryLink } from "@apollo/client/link/persisted-queries";
import { GraphQLWsLink } from "@apollo/client/link/subscriptions";
import { getMainDefinition } from "@apollo/client/utilities";
import { createClient as createWsClient } from "graphql-ws";
import Cookies from "js-cookie";
import { useMemo, type ReactNode } from "react";

// ----- token storage (module-scoped, mirrored to cookie) -----

const COOKIE_NAME = "ironflyer_token";
const COOKIE_EXPIRES_DAYS = 30;

let cachedToken: string | null = null;
let cacheHydrated = false;
let csrfBootstrap: Promise<void> | null = null;

function hydrate(): void {
  if (cacheHydrated) return;
  cacheHydrated = true;
  if (typeof window === "undefined") return;
  const v = Cookies.get(COOKIE_NAME);
  cachedToken = v ?? null;
}

export function getToken(): string | null {
  hydrate();
  return cachedToken;
}

export function setToken(token: string): void {
  cachedToken = token;
  cacheHydrated = true;
  if (typeof window === "undefined") return;
  Cookies.set(COOKIE_NAME, token, {
    expires: COOKIE_EXPIRES_DAYS,
    sameSite: "lax",
    secure: typeof location !== "undefined" && location.protocol === "https:",
    path: "/",
  });
}

export function clearToken(): void {
  cachedToken = null;
  cacheHydrated = true;
  if (typeof window === "undefined") return;
  Cookies.remove(COOKIE_NAME, { path: "/" });
}

// ----- URL resolution -----

const DEFAULT_HTTP = "http://localhost:8080/graphql";
const DEFAULT_WS = "ws://localhost:8080/graphql";

function trimSlash(s: string): string {
  return s.replace(/\/+$/, "");
}

function httpEndpoint(): string {
  const direct = process.env.NEXT_PUBLIC_GRAPHQL_URL;
  if (direct) return direct;
  const base = process.env.NEXT_PUBLIC_IRONFLYER_API_URL;
  if (base) return trimSlash(base) + "/graphql";
  return DEFAULT_HTTP;
}

function wsEndpoint(): string {
  const direct = process.env.NEXT_PUBLIC_GRAPHQL_WS_URL;
  if (direct) return direct;
  const base = process.env.NEXT_PUBLIC_IRONFLYER_WS_URL;
  if (base) return trimSlash(base) + "/graphql";
  // derive from HTTP endpoint
  const http = httpEndpoint();
  if (http.startsWith("https://")) return "wss://" + http.slice("https://".length);
  if (http.startsWith("http://")) return "ws://" + http.slice("http://".length);
  return DEFAULT_WS;
}

async function sha256(input: string): Promise<string> {
  const bytes = new TextEncoder().encode(input);
  const digest = await crypto.subtle.digest("SHA-256", bytes);
  return [...new Uint8Array(digest)]
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

// ----- links -----

// Surfaced so the auth context can flip cookies + reset cache from one
// place when a session ends.
type UnauthorizedHandler = () => void;
let onUnauthorized: UnauthorizedHandler = () => {
  // Default: in the browser, clear the token and bounce to /login.
  if (typeof window === "undefined") return;
  clearToken();
  // Avoid loop on the auth routes themselves.
  if (!window.location.pathname.startsWith("/login") && !window.location.pathname.startsWith("/signup")) {
    window.location.assign("/login");
  }
};

export function registerUnauthorizedHandler(handler: UnauthorizedHandler): void {
  onUnauthorized = handler;
}

function buildAuthLink(): ApolloLink {
  return setContext(async (_op, prev) => {
    const token = getToken();
    const headers = { ...((prev as { headers?: Record<string, string> }).headers ?? {}) };
    if (token) headers.Authorization = `Bearer ${token}`;
    // CSRF: the orchestrator's gqlhardening.CSRFMiddleware requires
    // double-submit (cookie + header) on every state-changing POST. The
    // server sets a non-HttpOnly cookie named `ironflyer_csrf`; we mirror
    // its value into the configured header on every request. Bearer-
    // auth requests are exempt server-side, but we still send the header
    // because token presence is determined per-request and the SPA may
    // make anonymous POSTs (signIn/signUp) before a token exists.
    if (!token && typeof document !== "undefined" && !Cookies.get("ironflyer_csrf")) {
      csrfBootstrap ??= fetch(httpEndpoint(), {
        method: "GET",
        credentials: "include",
        mode: "cors",
      })
        .catch(() => undefined)
        .then(() => undefined)
        .finally(() => {
          csrfBootstrap = null;
        });
      await csrfBootstrap;
    }
    if (typeof document !== "undefined") {
      const csrf = Cookies.get("ironflyer_csrf");
      if (csrf) headers["X-Ironflyer-CSRF"] = csrf;
    }
    return { headers };
  });
}

function buildErrorLink(): ApolloLink {
  return onError(({ graphQLErrors, networkError }) => {
    const ne = networkError as (Error & { statusCode?: number }) | null;
    if (ne && ne.statusCode === 401) {
      onUnauthorized();
      return;
    }
    if (graphQLErrors) {
      for (const g of graphQLErrors) {
        const code = (g.extensions as { code?: string } | undefined)?.code;
        if (code === "UNAUTHENTICATED" || code === "UNAUTHORIZED") {
          onUnauthorized();
          return;
        }
      }
    }
  });
}

function buildHttpLink(): ApolloLink {
  return new HttpLink({
    uri: httpEndpoint(),
    // include lets the browser send + receive the `ironflyer_csrf`
    // cookie alongside the request. Without it the double-submit
    // header above has nothing on the wire to compare against.
    credentials: "include",
  });
}

function buildWsLink(): ApolloLink | null {
  if (typeof window === "undefined") return null;
  const wsClient = createWsClient({
    url: () => {
      const token = getToken();
      const base = wsEndpoint();
      return token ? `${base}?token=${encodeURIComponent(token)}` : base;
    },
    lazy: true,
    connectionParams: () => {
      const token = getToken();
      return { authorization: token ? `Bearer ${token}` : "" };
    },
    retryAttempts: 5,
  });
  return new GraphQLWsLink(wsClient);
}

function buildLink(): ApolloLink {
  const authLink = buildAuthLink();
  const errorLink = buildErrorLink();
  const persistedLink = createPersistedQueryLink({ sha256 });
  const httpLink = buildHttpLink();
  const wsLink = buildWsLink();
  const httpChain = from([errorLink, authLink, persistedLink, httpLink]);
  if (!wsLink) return httpChain;
  return split(
    ({ query }) => {
      const def = getMainDefinition(query);
      return def.kind === "OperationDefinition" && def.operation === "subscription";
    },
    wsLink,
    httpChain,
  );
}

function buildCache(): InMemoryCache {
  return new InMemoryCache({
    // typePolicies — every entity that recurs across queries gets an
    // explicit keyFields so Apollo can dedupe it instead of treating
    // two query responses as independent objects. Without these
    // mappings the cache fragments and re-renders multiply when the
    // same Execution / Patch / Deploy / Project surfaces in multiple
    // panes (cockpit + studio + dashboard).
    typePolicies: {
      User: { keyFields: ["id"] },
      Wallet: { keyFields: ["tenantID"] },
      Execution: { keyFields: ["id"] },
      Deploy: { keyFields: ["id"] },
      DeployApproval: { keyFields: ["id"] },
      Blueprint: { keyFields: ["id"] },
      BlueprintStats: { keyFields: ["blueprintID"] },
      DashboardBlueprintStats: { keyFields: ["blueprintID"] },
      Project: { keyFields: ["id"] },
      Patch: { keyFields: ["id"] },
      LedgerEntry: { keyFields: ["id"] },
      GateVerdict: { keyFields: ["executionID", "name"] },
      ProjectFile: { keyFields: ["path"] },
      // Notification is normalized by id so a single mutation update
      // (e.g. markNotificationRead) flows through to every list that
      // currently holds the entity — bell + future inbox surfaces.
      Notification: { keyFields: ["id"] },
      Query: {
        fields: {
          // Notifications list merge — incoming items are deduped by
          // id against existing references, new items prepend, and
          // the union is sorted descending by createdAt so the bell
          // and any future paginated inbox surface a stable visual
          // order regardless of whether the data arrived from the
          // initial query, a refetch, or a subscription push.
          //
          // We compare by __ref because once `Notification` has its
          // own keyFields the cache stores list members as references
          // ({ __ref: "Notification:<id>" }) rather than inlined
          // objects. Apollo invokes `read` after `merge`, and any
          // resolver consuming the field receives the deduped,
          // sorted list with full normalized fields read-through.
          notifications: {
            keyArgs: ["unreadOnly"],
            merge(
              existing: ReadonlyArray<{ __ref?: string }> = [],
              incoming: ReadonlyArray<{ __ref?: string }> = [],
              { readField },
            ) {
              const byRef = new Map<string, { __ref?: string }>();
              for (const ref of incoming) {
                const key = ref.__ref ?? JSON.stringify(ref);
                byRef.set(key, ref);
              }
              for (const ref of existing) {
                const key = ref.__ref ?? JSON.stringify(ref);
                if (!byRef.has(key)) byRef.set(key, ref);
              }
              const merged = Array.from(byRef.values());
              merged.sort((a, b) => {
                const aT = (readField("createdAt", a) as string | undefined) ?? "";
                const bT = (readField("createdAt", b) as string | undefined) ?? "";
                if (aT === bT) return 0;
                return aT < bT ? 1 : -1;
              });
              return merged;
            },
          },
        },
      },
    },
  });
}

export function createApolloClient(): ApolloClient<unknown> {
  const devtoolsEnabled =
    typeof window !== "undefined" && process.env.NODE_ENV !== "production";
  return new ApolloClient({
    link: buildLink(),
    cache: buildCache(),
    defaultOptions: {
      // After the first network hit each watchQuery demotes itself to
      // cache-first so remounts (route changes, suspense replays,
      // accordion open/close on dashboards) hit the cache instead of
      // firing a redundant /graphql round trip. Individual hooks can
      // still override per-call via `nextFetchPolicy: "cache-and-network"`
      // when they want continuous freshness.
      watchQuery: {
        fetchPolicy: "cache-and-network",
        nextFetchPolicy: "cache-first",
        errorPolicy: "all",
      },
      query: { fetchPolicy: "network-only", errorPolicy: "all" },
      mutate: { errorPolicy: "all" },
    },
    // Apollo 3.14+ moved this under `devtools.enabled`. Cast to satisfy
    // older type defs without losing the new field at runtime.
    devtools: { enabled: devtoolsEnabled },
  } as unknown as ConstructorParameters<typeof ApolloClient>[0]);
}

// Singleton on the client; fresh client per server render pass.
let _client: ApolloClient<unknown> | null = null;

export function getApolloClient(): ApolloClient<unknown> {
  if (typeof window === "undefined") return createApolloClient();
  if (!_client) _client = createApolloClient();
  return _client;
}

// React wrapper consumed by app/providers.tsx.
export function ApolloProvider({ children }: { children: ReactNode }) {
  const client = useMemo(() => getApolloClient(), []);
  return <ApolloLibProvider client={client}>{children}</ApolloLibProvider>;
}

// Convenience accessor for non-React call sites that need a client.
// The previous module-scoped `apolloClient` constant eagerly created
// an ApolloClient at import time, which on the server runs once per
// build module reload (and during streaming SSR, per request payload).
// Lazy access via `getApolloClient()` is cheap on the client (the
// singleton is cached after the first call) and avoids server work
// entirely when a module imports the symbol just to call .clearStore()
// inside an effect that never runs server-side.
//
// New call sites: prefer `useApolloClient()` from `@apollo/client`
// inside React; reach for `getApolloClient()` only from non-React code.
