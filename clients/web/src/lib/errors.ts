// Normalisation helpers for Apollo / GraphQL errors.
//
// The orchestrator returns its own typed error envelope (GqlError) for
// resolver-level failures and standard GraphQLError objects for
// validation / parse failures. ApolloError wraps both, plus the network
// error. extractErrorMessage picks the first usable string for the UI;
// extractError returns a structured shape for ErrorPanel.

import { ApolloError, type ServerError, type ServerParseError } from "@apollo/client";
import type { GraphQLFormattedError } from "graphql";

export interface NormalizedError {
  // user-presentable headline; always non-empty.
  message: string;
  // backend error code (NOT_FOUND, PAYMENT_REQUIRED, FORBIDDEN, …) when
  // the orchestrator attaches one via extensions.code.
  code: string | null;
  // raw graphQLErrors[].extensions if any; useful for top_up_url etc.
  extensions: Record<string, unknown> | null;
  // True when the only failure is the network — distinguishes "server
  // returned 5xx" from "validation failed".
  isNetwork: boolean;
  // HTTP status code from a fetch failure when known.
  status: number | null;
}

export function normalizeError(err: unknown): NormalizedError {
  if (err instanceof ApolloError) {
    const g = err.graphQLErrors?.[0];
    if (g) return fromGraphQLError(g);
    const ne = err.networkError;
    if (ne) return fromNetworkError(ne);
    return {
      message: err.message || "Something went wrong.",
      code: null,
      extensions: null,
      isNetwork: false,
      status: null,
    };
  }
  if (err instanceof Error) {
    return {
      message: err.message || "Something went wrong.",
      code: null,
      extensions: null,
      isNetwork: false,
      status: null,
    };
  }
  return {
    message: "Something went wrong.",
    code: null,
    extensions: null,
    isNetwork: false,
    status: null,
  };
}

export function extractErrorMessage(err: unknown): string {
  return normalizeError(err).message;
}

// is401 — used by the Apollo error link + auth context to detect a
// session that needs to be cleared.
export function is401(err: unknown): boolean {
  const n = normalizeError(err);
  if (n.status === 401) return true;
  if (n.code === "UNAUTHENTICATED" || n.code === "UNAUTHORIZED") return true;
  return false;
}

function fromGraphQLError(g: GraphQLFormattedError): NormalizedError {
  const ext = (g.extensions ?? null) as Record<string, unknown> | null;
  const code = typeof ext?.code === "string" ? (ext.code as string) : null;
  return {
    message: g.message || "Request failed.",
    code,
    extensions: ext,
    isNetwork: false,
    status: typeof ext?.status === "number" ? (ext.status as number) : null,
  };
}

function fromNetworkError(ne: Error | ServerError | ServerParseError): NormalizedError {
  const status = "statusCode" in ne && typeof ne.statusCode === "number" ? ne.statusCode : null;
  return {
    message:
      status === 401
        ? "Your session expired. Please sign in again."
        : ne.message || "Network error — check your connection.",
    code: status === 401 ? "UNAUTHENTICATED" : null,
    extensions: null,
    isNetwork: true,
    status,
  };
}
