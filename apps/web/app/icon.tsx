import { ImageResponse } from 'next/og';

// Next.js favicon convention. Rendered at request time via the OG Image
// runtime so we never ship a binary PNG to the repo. The result is a
// 32x32 PNG: the compact Ironflyer gate mark.
export const size = { width: 32, height: 32 };
export const contentType = 'image/png';
// Cached at the edge: the icon is deterministic.
export const dynamic = 'force-static';

export default function Icon() {
  return new ImageResponse(
    (
      <div
        style={{ width: '100%', height: '100%', display: 'flex', background: '#0d0e0f' }}
      >
        <svg width="32" height="32" viewBox="0 0 64 64">
          <rect x="4" y="4" width="56" height="56" rx="8" fill="#0d0e0f" />
          <path d="M19 14h13c9 0 15 5 15 13 0 6-3 10-9 12l10 11H35L26 40h-3v10H12V14h7Z" fill="#e5ff00" />
          <path d="M23 23h12c3 0 5 2 5 5s-2 5-5 5H23V23Z" fill="#0d0e0f" />
          <path d="M15 14h10v36H15V14Z" fill="#e5ff00" />
          <path d="M28 18h16v4H28V18Zm0 12h16v4H28v-4Zm0 12h16v4H28v-4Z" fill="#f4f0e8" />
          <path d="M46 24l8 8-8 8v-6h-6v-4h6v-6Z" fill="#f4f0e8" />
        </svg>
      </div>
    ),
    size,
  );
}
