import { ImageResponse } from 'next/og';

// 1200x630 social card: disciplined brand mark, product promise, and proof
// line. Keep it flat and legible; no ornamental background effects.
export const alt = 'Ironflyer — ship software that survives review';
export const size = { width: 1200, height: 630 };
export const contentType = 'image/png';

export default function OpenGraphImage() {
  const headlineFont = 'Arial Black, Arial, sans-serif';
  const bodyFont = 'Arial, sans-serif';

  return new ImageResponse(
    (
      <div
        style={{
          width: '100%',
          height: '100%',
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'space-between',
          padding: 72,
          background: '#f4f0e8',
          color: '#0d0e0f',
          fontFamily: headlineFont,
        }}
      >
        {/* Top row: wordmark */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 18,
          }}
        >
          <svg width="72" height="72" viewBox="0 0 64 64">
            <rect x="4" y="4" width="56" height="56" rx="8" fill="#0d0e0f" />
            <path d="M19 14h13c9 0 15 5 15 13 0 6-3 10-9 12l10 11H35L26 40h-3v10H12V14h7Z" fill="#e5ff00" />
            <path d="M23 23h12c3 0 5 2 5 5s-2 5-5 5H23V23Z" fill="#0d0e0f" />
            <path d="M15 14h10v36H15V14Z" fill="#e5ff00" />
            <path d="M28 18h16v4H28V18Zm0 12h16v4H28v-4Zm0 12h16v4H28v-4Z" fill="#f4f0e8" />
            <path d="M46 24l8 8-8 8v-6h-6v-4h6v-6Z" fill="#f4f0e8" />
          </svg>
          <div
            style={{
              fontSize: 36,
              fontWeight: 900,
              letterSpacing: 0,
            }}
          >
            Ironflyer
          </div>
        </div>

        {/* Headline */}
        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            gap: 24,
          }}
        >
          <div
            style={{
              fontSize: 88,
              fontWeight: 900,
              lineHeight: 1.05,
              letterSpacing: 0,
              maxWidth: 980,
              fontFamily: headlineFont,
            }}
          >
            The #1 AI Completion Engine.
          </div>
          <div
            style={{
              fontSize: 32,
              fontWeight: 400,
              color: '#3a3a36',
              maxWidth: 880,
              fontFamily: bodyFont,
            }}
          >
            From AI-generated starts to proved, deploy-ready software.
          </div>
        </div>

        {/* Footer: lime accent + url */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            fontSize: 24,
            fontWeight: 600,
            color: '#0d0e0f',
            fontFamily: bodyFont,
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <div
              style={{
                width: 14,
                height: 14,
                background: '#e5ff00',
                borderRadius: 2,
              }}
            />
            ironflyer.dev
          </div>
          <div style={{ color: '#5a564d' }}>gates · patches · runtime · ledger</div>
        </div>
      </div>
    ),
    size,
  );
}
