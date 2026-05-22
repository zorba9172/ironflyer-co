'use client';

// Additional billing-related helpers that are not in the legacy `api`
// object. The orchestrator may expose a per-user vault endpoint
// (revenue / providerCost / margin). If it doesn't, callers can treat
// the rejection as "vault disabled" and render a graceful empty state.
//
// Endpoints assumed (best-effort, with graceful fallback):
//   GET  /budget/vault/me     -> per-user VaultSnapshot
//   POST /budget/checkout     -> already in api.startCheckout
//   POST /account/delete      -> account self-delete
//   POST /projects/bulk-delete -> deletes all owned projects

import { auth } from '../auth';
import { VaultSnapshot } from '../api';

const base = '/api/orchestrator';

async function jsonFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${base}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...auth.authHeader(),
      ...(init?.headers ?? {}),
    },
    cache: 'no-store',
  });
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}: ${await res.text()}`);
  return res.json() as Promise<T>;
}

export const billingApi = {
  /** Per-user vault snapshot (revenue, providerCost, refunds, margin). */
  myVault: () => jsonFetch<VaultSnapshot>('/budget/vault/me'),
  /** Account-level vault as a fallback when /vault/me isn't wired yet. */
  workspaceVault: () => jsonFetch<VaultSnapshot>('/budget/vault'),
};

export const accountApi = {
  /** Self-service account deletion. Server should require recent auth. */
  deleteAccount: async () => {
    const res = await fetch(`${base}/account/delete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...auth.authHeader() },
    });
    if (!res.ok && res.status !== 204) throw new Error(await res.text());
  },
  /** Bulk-delete every project this user owns. */
  deleteAllProjects: async () => {
    const res = await fetch(`${base}/projects/bulk-delete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...auth.authHeader() },
    });
    if (!res.ok && res.status !== 204) throw new Error(await res.text());
  },
};
