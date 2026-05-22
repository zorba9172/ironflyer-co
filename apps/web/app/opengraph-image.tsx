import { ImageResponse } from 'next/og';
import { readFile } from 'node:fs/promises';
import path from 'node:path';

// 1200x630 social card. Hebrew-first headline, alabaster background,
// subtle radial gradient, lime accent block carrying the wordmark.
//
// Hebrew typography: ships with Noto Sans Hebrew Bold (variable) — a
// permissive OFL font from Google — under apps/web/app/_fonts/.
// Vercel's build packs that file into the OG function payload because it
// sits inside the app/ tree, so cold-start fetches are local I/O.
//
// Production install procedure (when the TTF is missing from the repo):
//   1. Download the file from
//      https://fonts.google.com/noto/specimen/Noto+Sans+Hebrew
//   2. Place it at apps/web/app/_fonts/NotoSansHebrew-Bold.ttf
//   3. Redeploy. If missing, this route falls back to system Arial which
//      renders Hebrew via the platform's fallback font matching — usable
//      but visually inconsistent across browsers/OSes.
export const alt = 'Ironflyer — האפליקציות הטובות בעולם נסגרות בעצמן';
export const size = { width: 1200, height: 630 };
export const contentType = 'image/png';

async function loadHebrewFont(): Promise<ArrayBuffer | null> {
  try {
    const filePath = path.join(
      process.cwd(),
      'app',
      '_fonts',
      'NotoSansHebrew-Bold.ttf',
    );
    const buf = await readFile(filePath);
    return buf.buffer.slice(
      buf.byteOffset,
      buf.byteOffset + buf.byteLength,
    ) as ArrayBuffer;
  } catch {
    return null;
  }
}

export default async function OpenGraphImage() {
  const hebrew = await loadHebrewFont();
  const headlineFont = hebrew
    ? 'NotoSansHebrew'
    : 'Arial Black, Arial, sans-serif';
  const bodyFont = hebrew ? 'NotoSansHebrew' : 'Arial, sans-serif';

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
          background:
            'radial-gradient(circle at 20% 20%, #f7f3ea 0%, #e7dfd2 70%, #d9cfbe 100%)',
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
          <div
            style={{
              width: 64,
              height: 64,
              background: '#e5ff00',
              borderRadius: 12,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: '#0d0e0f',
              fontSize: 48,
              fontWeight: 900,
              lineHeight: 1,
            }}
          >
            I
          </div>
          <div
            style={{
              fontSize: 36,
              fontWeight: 900,
              letterSpacing: -0.5,
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
            direction: 'rtl',
          }}
        >
          <div
            style={{
              fontSize: 88,
              fontWeight: 900,
              lineHeight: 1.05,
              letterSpacing: -1.5,
              maxWidth: 980,
              fontFamily: headlineFont,
            }}
          >
            האפליקציות הטובות בעולם נסגרות בעצמן.
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
            AI Product Finisher — Spec, UX, Code, Tests, Security, Deploy.
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
          <div style={{ color: '#5a564d' }}>finish, don&apos;t fake</div>
        </div>
      </div>
    ),
    {
      ...size,
      fonts: hebrew
        ? [
            {
              name: 'NotoSansHebrew',
              data: hebrew,
              weight: 700,
              style: 'normal',
            },
          ]
        : undefined,
    },
  );
}
