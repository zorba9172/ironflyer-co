import type { MetadataRoute } from 'next';

// PWA manifest expressed as a Next.js metadata route. Next will emit
// /manifest.webmanifest at build time from this function — no static
// public-folder copy to drift from the design tokens.
//
// The icon entry references /icon and /apple-icon (Next.js generates
// those URLs from app/icon.tsx + app/apple-icon.tsx) so the favicon,
// PWA icon, and iOS home-screen icon stay in sync.
export default function manifest(): MetadataRoute.Manifest {
  return {
    name: 'Ironflyer — AI Product Finisher',
    short_name: 'Ironflyer',
    description:
      'Build AI apps that finish themselves. Spec, UX, Code, Tests, Security, Deploy — all gated.',
    start_url: '/',
    scope: '/',
    display: 'standalone',
    display_override: ['window-controls-overlay', 'standalone'],
    orientation: 'portrait',
    background_color: '#f4f0e8',
    theme_color: '#e5ff00',
    lang: 'en-US',
    dir: 'ltr',
    categories: ['productivity', 'developer', 'business'],
    icons: [
      {
        src: '/icon',
        sizes: '32x32',
        type: 'image/png',
        purpose: 'any',
      },
      {
        src: '/icon',
        sizes: '192x192',
        type: 'image/png',
        purpose: 'any',
      },
      {
        src: '/icon',
        sizes: '512x512',
        type: 'image/png',
        purpose: 'any',
      },
      {
        src: '/icon',
        sizes: 'any',
        type: 'image/png',
        purpose: 'maskable',
      },
      {
        src: '/apple-icon',
        sizes: '180x180',
        type: 'image/png',
        purpose: 'any',
      },
    ],
    shortcuts: [
      {
        name: 'Dashboard',
        short_name: 'Dashboard',
        description: 'Open the Ironflyer dashboard',
        url: '/app',
        icons: [{ src: '/icon', sizes: '192x192', type: 'image/png' }],
      },
      {
        name: 'Templates',
        short_name: 'Templates',
        description: 'Browse the template gallery',
        url: '/templates',
        icons: [{ src: '/icon', sizes: '192x192', type: 'image/png' }],
      },
      {
        name: 'Settings',
        short_name: 'Settings',
        description: 'Account and workspace settings',
        url: '/app?panel=settings',
        icons: [{ src: '/icon', sizes: '192x192', type: 'image/png' }],
      },
    ],
    screenshots: [
      {
        src: '/screenshots/dashboard-wide.png',
        sizes: '1280x720',
        type: 'image/png',
        form_factor: 'wide',
      },
      {
        src: '/screenshots/dashboard-narrow.png',
        sizes: '720x1280',
        type: 'image/png',
        form_factor: 'narrow',
      },
    ],
  };
}
