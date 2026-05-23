import { ImageResponse } from 'next/og';

// Next.js convention: a single `apple-icon.tsx` becomes the
// `apple-touch-icon` link tag for iOS home-screen installs.
// 180x180 is Apple's recommended size for retina displays.
//
// Same gate mark as app/icon.tsx, scaled up. Static + edge-cacheable.
export const size = { width: 180, height: 180 };
export const contentType = 'image/png';
export const dynamic = 'force-static';

export default function AppleIcon() {
  return new ImageResponse(
    (
      <div
        style={{ width: '100%', height: '100%', display: 'flex', background: '#0d0e0f' }}
      >
        <svg width="180" height="180" viewBox="0 0 64 64">
          <rect x="4" y="4" width="56" height="56" rx="8" fill="#0d0e0f" />
          <path d="M19 14h13c9 0 15 5 15 13 0 6-3 10-9 12l10 11H35L26 40h-3v10H12V14h7Z" fill="#e5ff00" />
          <path d="M23 23h12c3 0 5 2 5 5s-2 5-5 5H23V23Z" fill="#0d0e0f" />
          <path d="M15 14h10v36H15V14Z" fill="#e5ff00" />
          <path d="M28 18h16v4H28V18Zm0 12h16v4H28v-4Zm0 12h16v4H28v-4Z" fill="#f4f0e8" />
          <path d="M46 24l8 8-8 8v-6h-6v-4h6v-6Z" fill="#f4f0e8" />
        </svg>
      </div>
    ),
    { ...size },
  );
}
