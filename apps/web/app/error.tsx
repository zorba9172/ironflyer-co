'use client';

import { useEffect } from 'react';

// Top-level client error boundary. English copy, lime "Reset" CTA
// that re-mounts the failing tree via reset(), and a secondary link to
// our public status page where users can confirm whether the issue is
// on our side.
export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    // Report to the browser console with the request digest so support
    // can correlate it to a server-side trace. We deliberately avoid an
    // automatic POST — the user gets a "Report" button instead.
    // eslint-disable-next-line no-console
    console.error('[ironflyer] unhandled error', error);
  }, [error]);

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
          gap: 20,
        }}
      >
        <svg width="76" height="76" viewBox="0 0 64 64" aria-hidden>
          <rect x="4" y="4" width="56" height="56" rx="8" fill="#0d0e0f" />
          <path d="M19 14h13c9 0 15 5 15 13 0 6-3 10-9 12l10 11H35L26 40h-3v10H12V14h7Z" fill="#e5ff00" />
          <path d="M23 23h12c3 0 5 2 5 5s-2 5-5 5H23V23Z" fill="#0d0e0f" />
          <path d="M15 14h10v36H15V14Z" fill="#e5ff00" />
          <path d="M28 18h16v4H28V18Zm0 12h16v4H28v-4Zm0 12h16v4H28v-4Z" fill="#f4f0e8" />
          <path d="M46 24l8 8-8 8v-6h-6v-4h6v-6Z" fill="#f4f0e8" />
        </svg>
        <h1
          style={{
            margin: 0,
            fontFamily: 'var(--font-display), Arial Black, sans-serif',
            fontSize: '1.875rem',
            letterSpacing: 0,
          }}
        >
          Something failed inside the loop.
        </h1>
        <p style={{ margin: 0, color: '#3a3a36', lineHeight: 1.5 }}>
          The error was logged. Try again, or check service status if the problem keeps happening.
        </p>
        {error.digest && (
          <code
            style={{
              fontFamily: 'ui-monospace, SFMono-Regular, monospace',
              fontSize: '0.75rem',
              color: '#77736b',
              background: '#e7dfd2',
              padding: '4px 10px',
              borderRadius: 6,
            }}
          >
            digest: {error.digest}
          </code>
        )}
        <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', justifyContent: 'center' }}>
          <button
            type="button"
            onClick={() => reset()}
            style={{
              padding: '12px 24px',
              background: '#e5ff00',
              color: '#0d0e0f',
              border: 'none',
              borderRadius: 8,
              fontWeight: 700,
              cursor: 'pointer',
            }}
          >
            Try again
          </button>
          <a
            href="https://github.com/ironflyer/ironflyer/issues/new"
            target="_blank"
            rel="noreferrer"
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
            Report issue
          </a>
          <a
            href="/status"
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              padding: '12px 24px',
              color: '#0d0e0f',
              borderRadius: 8,
              fontWeight: 600,
              textDecoration: 'none',
            }}
          >
            Service status
          </a>
        </div>
      </div>
    </main>
  );
}
