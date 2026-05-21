/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  devIndicators: false,
  experimental: {
    optimizePackageImports: ['@mui/material', '@mui/icons-material'],
  },
  async rewrites() {
    return [
      {
        source: '/api/orchestrator/:path*',
        destination: `${process.env.ORCHESTRATOR_URL ?? 'http://localhost:8080'}/:path*`,
      },
      {
        source: '/api/runtime/:path*',
        destination: `${process.env.RUNTIME_URL ?? 'http://localhost:8090'}/:path*`,
      },
    ];
  },
};

export default nextConfig;
