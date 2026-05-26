/** @type {import('next').NextConfig} */
const nextConfig = {
  images: {
    formats: ["image/avif", "image/webp"]
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

export default nextConfig;
