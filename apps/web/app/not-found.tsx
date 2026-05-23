import Link from 'next/link';
import type { Metadata } from 'next';

export const metadata: Metadata = {
  title: '404 — Page not found | Ironflyer',
  description: 'The page you were looking for does not exist. Return home or check service status.',
};

// Branded 404. English copy, lime CTA back home, single subtle
// illustration drawn in CSS so we don't pay an extra network round-trip
// to render the not-found state.
export default function NotFound() {
  return (
    <main
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: 32,
        background: '#f4f0e8',
        color: '#0d0e0f',
        fontFamily: 'var(--font-body), Inter, sans-serif',
      }}
    >
      <div
        style={{
          maxWidth: 560,
          width: '100%',
          textAlign: 'center',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          gap: 24,
        }}
      >
        {/* Branded route miss illustration. */}
        <div
          aria-hidden
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 12,
            fontFamily: 'var(--font-display), Arial Black, sans-serif',
            fontSize: 96,
            lineHeight: 1,
            letterSpacing: 0,
          }}
        >
          <span>4</span>
          <svg width="84" height="84" viewBox="0 0 64 64" aria-hidden>
            <rect x="4" y="4" width="56" height="56" rx="8" fill="#0d0e0f" />
            <path d="M19 14h13c9 0 15 5 15 13 0 6-3 10-9 12l10 11H35L26 40h-3v10H12V14h7Z" fill="#e5ff00" />
            <path d="M23 23h12c3 0 5 2 5 5s-2 5-5 5H23V23Z" fill="#0d0e0f" />
            <path d="M15 14h10v36H15V14Z" fill="#e5ff00" />
            <path d="M28 18h16v4H28V18Zm0 12h16v4H28v-4Zm0 12h16v4H28v-4Z" fill="#f4f0e8" />
            <path d="M46 24l8 8-8 8v-6h-6v-4h6v-6Z" fill="#f4f0e8" />
          </svg>
          <span>4</span>
        </div>

        <h1
          style={{
            margin: 0,
            fontFamily: 'var(--font-display), Arial Black, sans-serif',
            fontSize: '2rem',
            letterSpacing: 0,
          }}
        >
          This page did not pass the route gate.
        </h1>
        <p style={{ margin: 0, color: '#3a3a36', maxWidth: 440, lineHeight: 1.5 }}>
          The link may be wrong, the page may have moved, or the route was removed.
          Head home or check status if something feels off.
        </p>

        <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', justifyContent: 'center' }}>
          <Link
            href="/"
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              padding: '12px 24px',
              background: '#e5ff00',
              color: '#0d0e0f',
              borderRadius: 8,
              fontWeight: 700,
              textDecoration: 'none',
            }}
          >
            Back home
          </Link>
          <Link
            href="/status"
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              padding: '12px 24px',
              border: '1px solid rgba(13,14,15,0.2)',
              color: '#0d0e0f',
              borderRadius: 8,
              fontWeight: 600,
              textDecoration: 'none',
            }}
          >
            Check service status
          </Link>
        </div>
      </div>
    </main>
  );
}
