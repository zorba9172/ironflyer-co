// Thin wrapper around @sentry/node so the extension can capture
// unhandled errors without scattering SDK calls across every command.
// Init is gated on the ironflyer.sentryDsn workspace setting — empty
// keeps Sentry off entirely, so we never collect telemetry from a user
// who hasn't opted in.
import * as Sentry from '@sentry/node';
import * as vscode from 'vscode';

let initialised = false;

export function initSentry(): boolean {
  if (initialised) return true;
  const cfg = vscode.workspace.getConfiguration('ironflyer');
  const dsn = (cfg.get<string>('sentryDsn') ?? '').trim();
  if (!dsn) return false;
  Sentry.init({
    dsn,
    environment: process.env.IRONFLYER_ENV || 'production',
    release: process.env.IRONFLYER_VERSION,
    // VSCode is an editor — perf tracing would be noise. We only want
    // exception capture from the extension host.
    tracesSampleRate: 0,
  });
  initialised = true;
  return true;
}

export function captureException(err: unknown, context?: Record<string, unknown>): void {
  if (!initialised) return;
  try {
    if (context) {
      Sentry.withScope((scope) => {
        scope.setExtras(context);
        Sentry.captureException(err);
      });
    } else {
      Sentry.captureException(err);
    }
  } catch {
    // Best-effort — never let an instrumentation failure surface in the editor.
  }
}

export async function flushSentry(): Promise<void> {
  if (!initialised) return;
  try {
    await Sentry.flush(2000);
  } catch {
    /* ignore */
  }
}
