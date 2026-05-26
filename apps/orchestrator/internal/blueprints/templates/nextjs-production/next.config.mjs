/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  experimental: {
    // App Router is stable on 15.x; this block is reserved for blueprint
    // upgrades (typedRoutes, ppr, etc.).
  },
};

export default nextConfig;
