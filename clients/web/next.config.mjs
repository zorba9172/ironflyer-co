import { withSentryConfig } from "@sentry/nextjs";

/** @type {import('next').NextConfig} */
const nextConfig = {
  images: {
    formats: ["image/avif", "image/webp"]
  },

  // Cold-start optimization: deep-link the MUI / icon / chart imports so
  // tree shaking lands one named export per module instead of pulling the
  // whole barrel. Next 15 supports this for any package listed below.
  // optimizeCss inlines critical CSS at build so first paint doesn't wait
  // on a network round-trip for the stylesheet.
  experimental: {
    optimizeCss: true,
    optimizePackageImports: [
      "@mui/material",
      "@mui/icons-material",
      "@mui/lab",
      "@mui/x-charts",
      "@mui/x-data-grid",
      "echarts",
      "echarts-for-react",
      "@xyflow/react",
      "lodash"
    ]
  },

  allowedDevOrigins: ["127.0.0.1", "localhost", "192.168.1.227"],

  // Production same-origin proxy for openvscode-server. In production
  // we want /ide/* to flow through the web origin so cookies, clipboard,
  // popups, and X-Frame-Options are friction-free. This rewrite handles
  // the HTTP plane; a real reverse proxy in front (nginx / Caddy /
  // cloudflared) terminates the WebSocket upgrade for VS Code's
  // extension-host channel, which Next.js dev rewrites do not proxy.
  //
  // In dev, the iframe points directly at http://localhost:3030/ide via
  // NEXT_PUBLIC_OPENVSCODE_URL so WS works without a custom server.
  async rewrites() {
    const openvscodeUpstream = (
      process.env.IRONFLYER_OPENVSCODE_UPSTREAM ?? "http://localhost:3030"
    ).replace(/\/+$/, "");
    return [
      // openvscode-server is started with --server-base-path /ide so
      // every asset URL already begins with /ide; we proxy 1-to-1.
      { source: "/ide", destination: `${openvscodeUpstream}/ide` },
      { source: "/ide/:path*", destination: `${openvscodeUpstream}/ide/:path*` }
    ];
  }
};

// Wrap with Sentry so production builds upload source maps to Sentry.
// When SENTRY_AUTH_TOKEN is unset (local dev / preview without secrets)
// the wrap is a no-op for the upload step — @sentry/nextjs handles the
// missing token gracefully. We still apply the wrap unconditionally so
// the runtime instrumentation hooks (sentry.{client,server,edge}.
// config.ts) wire in via the same plugin.
export default withSentryConfig(nextConfig, {
  silent: true,
  org: process.env.SENTRY_ORG,
  project: process.env.SENTRY_PROJECT,
  authToken: process.env.SENTRY_AUTH_TOKEN,
  widenClientFileUpload: true,
  hideSourceMaps: true,
  disableLogger: true
});
