// Minimal Sentry envelope sender — no SDK dependency.
//
// We dropped `@sentry/node` because v10 unconditionally pulls in
// hundreds of KB of OpenTelemetry / Prisma / MongoDB / Fastify
// instrumentations that a VSCode extension has no business carrying.
// The SDK does many things we don't need (perf tracing, breadcrumbs,
// auto-instrumentation, native crash reporting); we only ever called
// `captureException` and `flush`.
//
// This module speaks the Sentry envelope protocol directly:
// https://develop.sentry.dev/sdk/envelopes/
//
// Init is gated on the ironflyer.sentryDsn workspace setting — empty
// keeps Sentry off entirely, so we never collect telemetry from a user
// who hasn't opted in. Errors are POSTed best-effort; failures are
// swallowed so instrumentation can never surface in the editor.

import * as vscode from 'vscode';

interface DsnComponents {
  envelopeUrl: string;
  publicKey: string;
}

let endpoint: DsnComponents | undefined;
let environment = 'production';
let release: string | undefined;
const inflight = new Set<Promise<unknown>>();

export function initSentry(): boolean {
  if (endpoint) return true;
  const cfg = vscode.workspace.getConfiguration('ironflyer');
  const dsn = (cfg.get<string>('sentryDsn') ?? '').trim();
  if (!dsn) return false;
  const parsed = parseDsn(dsn);
  if (!parsed) return false;
  endpoint = parsed;
  environment = process.env.IRONFLYER_ENV || 'production';
  release = process.env.IRONFLYER_VERSION;
  return true;
}

export function captureException(err: unknown, context?: Record<string, unknown>): void {
  if (!endpoint) return;
  try {
    const event = buildEvent(err, context);
    const promise = sendEnvelope(endpoint, event).catch(() => undefined);
    inflight.add(promise);
    void promise.finally(() => inflight.delete(promise));
  } catch {
    // Best-effort — never let an instrumentation failure surface in the editor.
  }
}

export async function flushSentry(timeoutMs = 2000): Promise<void> {
  if (!endpoint || inflight.size === 0) return;
  await Promise.race([
    Promise.all(Array.from(inflight)),
    new Promise<void>((resolve) => setTimeout(resolve, timeoutMs)),
  ]);
}

// ---------------- internals ----------------

function parseDsn(dsn: string): DsnComponents | undefined {
  // https://<publicKey>@o<org>.ingest.sentry.io/<projectId>
  try {
    const u = new URL(dsn);
    const publicKey = u.username;
    const projectId = u.pathname.replace(/^\/+/, '');
    if (!publicKey || !projectId) return undefined;
    const envelopeUrl = `${u.protocol}//${u.host}/api/${projectId}/envelope/`;
    return { envelopeUrl, publicKey };
  } catch {
    return undefined;
  }
}

function buildEvent(err: unknown, context?: Record<string, unknown>): Record<string, unknown> {
  const isError = err instanceof Error;
  const message = isError ? `${err.name}: ${err.message}` : String(err);
  const stack = isError && err.stack ? parseStack(err.stack) : undefined;
  return {
    event_id: randomEventId(),
    timestamp: Date.now() / 1000,
    platform: 'node',
    level: 'error',
    environment,
    release,
    server_name: 'vscode-extension',
    exception: {
      values: [{
        type: isError ? err.name : 'Error',
        value: isError ? err.message : message,
        stacktrace: stack ? { frames: stack } : undefined,
      }],
    },
    extra: context,
    tags: { runtime: 'vscode-extension' },
  };
}

function parseStack(stack: string): Array<Record<string, unknown>> {
  // Best-effort V8 stack frame parser. Only used when a real Error is
  // captured; bad parses fall back to a single synthetic frame.
  const lines = stack.split('\n').slice(1, 30);
  const frames: Array<Record<string, unknown>> = [];
  for (const line of lines) {
    const m = line.match(/at (?:(.+) \()?(.+?):(\d+):(\d+)\)?$/);
    if (!m) continue;
    frames.push({
      function: m[1] ?? '<anonymous>',
      filename: m[2],
      lineno: Number(m[3]),
      colno: Number(m[4]),
      in_app: !m[2].includes('node_modules'),
    });
  }
  // Sentry expects frames innermost-last.
  return frames.reverse();
}

async function sendEnvelope(dsn: DsnComponents, event: Record<string, unknown>): Promise<void> {
  const eventId = event.event_id as string;
  const envelopeHeader = JSON.stringify({
    event_id: eventId,
    sent_at: new Date().toISOString(),
    dsn: dsn.envelopeUrl,
  });
  const itemHeader = JSON.stringify({ type: 'event' });
  const itemPayload = JSON.stringify(event);
  const body = `${envelopeHeader}\n${itemHeader}\n${itemPayload}\n`;

  const auth = [
    'Sentry sentry_version=7',
    `sentry_client=ironflyer-vscode/1.0`,
    `sentry_key=${dsn.publicKey}`,
  ].join(', ');

  await fetch(dsn.envelopeUrl, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/x-sentry-envelope',
      'X-Sentry-Auth': auth,
    },
    body,
  });
}

function randomEventId(): string {
  // 32 hex chars, no dashes (Sentry event_id format).
  const bytes = new Uint8Array(16);
  for (let i = 0; i < 16; i++) bytes[i] = Math.floor(Math.random() * 256);
  return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
}
