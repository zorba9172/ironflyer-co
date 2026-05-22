import { ImageResponse } from 'next/og';

// Next.js convention: a single `apple-icon.tsx` becomes the
// `apple-touch-icon` link tag for iOS home-screen installs.
// 180x180 is Apple's recommended size for retina displays.
//
// Same monogram as app/icon.tsx, scaled up: lime square with the "I"
// mark on an alabaster ground. Static + edge-cacheable.
export const size = { width: 180, height: 180 };
export const contentType = 'image/png';
export const dynamic = 'force-static';

export default function AppleIcon() {
  return new ImageResponse(
    (
      <div
        style={{
          width: '100%',
          height: '100%',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          background: '#f4f0e8',
          borderRadius: 36,
        }}
      >
        <div
          style={{
            width: 132,
            height: 132,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            background: '#e5ff00',
            color: '#0d0e0f',
            fontSize: 112,
            fontWeight: 900,
            fontFamily: 'Arial Black, Arial, sans-serif',
            lineHeight: 1,
            borderRadius: 24,
          }}
        >
          I
        </div>
      </div>
    ),
    { ...size },
  );
}
