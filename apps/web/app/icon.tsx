import { ImageResponse } from 'next/og';
import { readFile } from 'node:fs/promises';
import path from 'node:path';

// Next.js favicon convention. Rendered at request time via the OG Image
// runtime so we never ship a binary PNG to the repo. The result is a
// 32x32 PNG: lime square on alabaster, monogram "I" in display weight.
//
// Hebrew-capable font (Noto Sans Hebrew Bold variable, ~110KB) loaded from
// apps/web/app/_fonts/ at build time. The "I" glyph itself is Latin, but
// the same font is shared with opengraph-image.tsx so the loader cost is
// paid once and the favicon stays consistent with the social card.
//
// Production install: the TTF ships in the repo under apps/web/app/_fonts/.
// On Vercel's edge runtime the file is bundled into the function payload
// automatically because it lives under the app/ directory tree.
export const size = { width: 32, height: 32 };
export const contentType = 'image/png';
// Cached at the edge: the icon is deterministic.
export const dynamic = 'force-static';

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
    // Font missing — fall back to system Arial Black via fontFamily below.
    return null;
  }
}

export default async function Icon() {
  const hebrew = await loadHebrewFont();
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
          borderRadius: 6,
        }}
      >
        <div
          style={{
            width: 24,
            height: 24,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            background: '#e5ff00',
            color: '#0d0e0f',
            fontSize: 20,
            fontWeight: 900,
            fontFamily: hebrew ? 'NotoSansHebrew' : 'Arial Black, Arial, sans-serif',
            lineHeight: 1,
            borderRadius: 4,
          }}
        >
          I
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
