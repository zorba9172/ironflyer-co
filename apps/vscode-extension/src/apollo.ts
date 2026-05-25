// Apollo Client setup for the VSCode extension.
//
// One client per (endpoint, token) snapshot — when either changes we
// dispose the previous client and build a new one so the WS connection
// re-handshakes with the fresh Authorization header. This is the
// simplest way to keep the auth contract aligned with the legacy REST
// client (which read the token on every request).
//
// Split-link layout:
//
//   subscriptions  ─▶ GraphQLWsLink (graphql-ws → ws://…/graphql)
//   queries+muts   ─▶ HttpLink       (http://…/graphql)
//
// Both links carry the same Bearer token; the HTTP link reads it per
// request (via `setContext`-style fetch wrapper), the WS link injects
// it through the `connectionParams` callback on (re)connect.

import {
  ApolloClient,
  ApolloLink,
  HttpLink,
  InMemoryCache,
  split,
} from '@apollo/client/core';
import { GraphQLWsLink } from '@apollo/client/link/subscriptions';
import { setContext } from '@apollo/client/link/context';
import { getMainDefinition } from '@apollo/client/utilities';
import { createClient } from 'graphql-ws';
import { OperationDefinitionNode } from 'graphql';
import WebSocket from 'ws';

export type TokenProvider = () => Promise<string | undefined>;

export interface ApolloFactoryArgs {
  endpoint: string;        // e.g. http://localhost:8080 — no trailing /graphql
  getToken: TokenProvider; // pulled fresh from SecretStorage on every op
}

/**
 * Build a fresh ApolloClient wired against the given orchestrator
 * endpoint. Returns the client plus a disposer that closes the WS link
 * cleanly so the extension host doesn't leak sockets on reload.
 */
export function createApolloClient(args: ApolloFactoryArgs): {
  client: ApolloClient<unknown>;
  dispose: () => void;
} {
  const httpUri = joinUri(args.endpoint, '/graphql');
  const wsUri = toWsUri(httpUri);

  // ------------- HTTP link -------------

  const httpLink = new HttpLink({
    uri: httpUri,
    // Apollo passes the standard `fetch` from globalThis (Node 18+).
    // VSCode's extension host bundles Node 20, so this is always present.
    fetch,
  });

  const httpAuthLink = setContext(async (_op, prev) => {
    const token = await args.getToken();
    const headers: Record<string, string> = {
      ...(prev.headers as Record<string, string> | undefined),
    };
    if (token) headers.Authorization = `Bearer ${token}`;
    return { headers };
  });

  // ------------- WS link -------------

  // The graphql-ws client manages its own reconnect loop with a small
  // jittered backoff — we do NOT add our own. `connectionParams` is a
  // callback so every (re)connect picks up the latest token.
  const wsClient = createClient({
    url: wsUri,
    webSocketImpl: WebSocket as unknown as typeof globalThis.WebSocket,
    connectionParams: async () => {
      const token = await args.getToken();
      return token ? { authorization: `Bearer ${token}` } : {};
    },
    // Keep the socket warm-ish. The orchestrator emits ping frames every
    // 30s so anything between 20-40s is fine.
    keepAlive: 30_000,
    retryAttempts: Infinity,
  });

  const wsLink = new GraphQLWsLink(wsClient);

  // ------------- Split link -------------

  const splitLink = split(
    ({ query }) => {
      const def = getMainDefinition(query) as OperationDefinitionNode;
      return def.kind === 'OperationDefinition' && def.operation === 'subscription';
    },
    wsLink,
    ApolloLink.from([httpAuthLink, httpLink]),
  );

  const client = new ApolloClient({
    link: splitLink,
    cache: new InMemoryCache({
      // The extension is short-lived UI state — we don't want stale
      // cache reads bleeding across sign-outs.
      typePolicies: {
        Query: {
          fields: {
            // Re-fetch on every query — the extension's UI surfaces are
            // few enough that we trade cache-hits for freshness.
            projects: { merge: false },
            patches: { merge: false },
            gates: { merge: false },
            memory: { merge: false },
            audit: { merge: false },
            agentTelemetry: { merge: false },
          },
        },
      },
    }),
    defaultOptions: {
      query:    { fetchPolicy: 'no-cache', errorPolicy: 'all' },
      mutate:   { fetchPolicy: 'no-cache', errorPolicy: 'all' },
      watchQuery:  { fetchPolicy: 'no-cache', errorPolicy: 'all' },
    },
    assumeImmutableResults: true,
  });

  return {
    client,
    dispose: () => {
      try { void wsClient.dispose(); } catch { /* swallow — best-effort */ }
      client.stop();
    },
  };
}

function joinUri(base: string, path: string): string {
  return `${base.replace(/\/+$/, '')}${path}`;
}

function toWsUri(httpUri: string): string {
  return httpUri.replace(/^http(s?):\/\//i, (_m, s) => `ws${s}://`);
}
