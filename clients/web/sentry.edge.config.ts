// Sentry edge init — middleware + edge route handlers. Same DSN
// as the server runtime; sampled lower because the edge is just
// auth + redirects today.
import * as Sentry from "@sentry/nextjs";

const dsn = process.env.SENTRY_DSN_WEB ?? process.env.SENTRY_DSN;

Sentry.init({
  dsn,
  enabled: !!dsn,
  tracesSampleRate: 0.1,
  environment: process.env.IRONFLYER_ENV ?? "development",
});
