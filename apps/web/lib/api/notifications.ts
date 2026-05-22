'use client';

// Notifications + webhooks client. Talks to the orchestrator routes added by
// Agent N: /webhooks (CRUD + test) and /notifications/preferences (GET/PUT).
//
// All endpoints are auth-scoped — the orchestrator filters by the JWT and
// never trusts the userId field in the body, so the client doesn't need to
// thread it through.

import { auth } from '../auth';

const base = '/api/orchestrator';

// NotificationRule mirrors the Go struct one-to-one so JSON round-trips with
// no field-name translation. Keep these aligned when the backend evolves.
export interface NotificationRule {
  userId: string;
  email: string;
  onRunComplete: boolean;
  onGateFailed: boolean;
  onDeployDone: boolean;
  onBudgetWarning: boolean;
  channelEmail: boolean;
  channelWebhook: boolean;
}

// Webhook subscription as returned by the orchestrator. The secret may be
// omitted on responses to avoid round-tripping it through the browser; we
// keep it optional so list + create can share the same type.
export interface Webhook {
  id: string;
  userId: string;
  projectId?: string;
  url: string;
  events?: string[];
  secret?: string;
  createdAt: string;
  lastSentAt?: string;
  failureCount: number;
  disabled: boolean;
}

export interface CreateWebhookInput {
  url: string;
  events?: string[];
  projectId?: string;
  secret?: string;
}

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
  if (!res.ok) throw new Error(`${res.status}: ${await res.text()}`);
  if (res.status === 204) return undefined as unknown as T;
  return res.json() as Promise<T>;
}

export const notificationsApi = {
  /** Load the current user's notification rule. Server synthesises a default
   * row when none exists, so callers can render the form unconditionally. */
  getPreferences: () => jsonFetch<NotificationRule>('/notifications/preferences'),
  /** Persist the user's notification rule. Server forces userId server-side. */
  setPreferences: (rule: NotificationRule) =>
    jsonFetch<NotificationRule>('/notifications/preferences', {
      method: 'PUT',
      body: JSON.stringify(rule),
    }),
};

export const webhooksApi = {
  list: () => jsonFetch<Webhook[]>('/webhooks'),
  create: (input: CreateWebhookInput) =>
    jsonFetch<Webhook>('/webhooks', {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  remove: async (id: string) => {
    const res = await fetch(`${base}/webhooks/${id}`, {
      method: 'DELETE',
      headers: { ...auth.authHeader() },
    });
    if (!res.ok && res.status !== 204) throw new Error(await res.text());
  },
  test: (id: string) =>
    jsonFetch<{ queued: boolean; id: string }>(`/webhooks/${id}/test`, {
      method: 'POST',
    }),
};

/** Stable list of webhook event names the dashboard renders in the
 * "Add webhook" picker. The server treats an empty list as "everything"
 * so this catalogue is purely a UX hint. */
export const WEBHOOK_EVENT_CATALOG: { name: string; label: string }[] = [
  { name: 'run_complete',   label: 'הריצה הסתיימה' },
  { name: 'run_failed',     label: 'הריצה נכשלה' },
  { name: 'gate_passed',    label: 'שער עבר' },
  { name: 'gate_failed',    label: 'שער נכשל' },
  { name: 'patch_applied',  label: 'פאצ\' הוחל' },
  { name: 'patch_rejected', label: 'פאצ\' נדחה' },
  { name: 'webhook_test',   label: 'בדיקת Webhook' },
];
