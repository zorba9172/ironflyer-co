// Sentry server init — Node runtime (SSR, route handlers, server
// actions). DSN flows through SENTRY_DSN_WEB so it stays out of the
// client bundle. Empty DSN short-circuits init.
import * as Sentry from "@sentry/nextjs";

const dsn = process.env.SENTRY_DSN_WEB ?? process.env.SENTRY_DSN;

Sentry.init({
  dsn,
  enabled: !!dsn,
  tracesSampleRate: 0.1,
  environment: process.env.IRONFLYER_ENV ?? "development",
});
