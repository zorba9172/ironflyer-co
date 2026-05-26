// Sentry client init — runs in the browser. DSN flows through
// NEXT_PUBLIC_SENTRY_DSN so it ships with the bundle. Empty DSN
// short-circuits init (enabled flag) so local dev stays quiet.
import * as Sentry from "@sentry/nextjs";

const dsn = process.env.NEXT_PUBLIC_SENTRY_DSN;

Sentry.init({
  dsn,
  enabled: !!dsn,
  tracesSampleRate: 0.1,
  replaysSessionSampleRate: 0,
  replaysOnErrorSampleRate: 0,
  environment: process.env.NEXT_PUBLIC_IRONFLYER_ENV ?? "development",
});
