'use client';

import { useEffect } from 'react';

// Top-level client error boundary. Hebrew-first copy, lime "Reset" CTA
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
      dir="rtl"
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
        <div
          aria-hidden
          style={{
            width: 72,
            height: 72,
            background: '#e5ff00',
            borderRadius: 16,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontFamily: 'var(--font-display), Arial Black, sans-serif',
            fontSize: 48,
            color: '#0d0e0f',
            lineHeight: 1,
          }}
        >
          !
        </div>
        <h1
          style={{
            margin: 0,
            fontFamily: 'var(--font-display), Arial Black, sans-serif',
            fontSize: '1.875rem',
            letterSpacing: -0.5,
          }}
        >
          משהו השתבש בצד שלנו. עברנו לבדוק את זה.
        </h1>
        <p style={{ margin: 0, color: '#3a3a36', lineHeight: 1.5 }}>
          השגיאה נרשמה אצלנו. אפשר לנסות לטעון את הדף מחדש, או לבדוק אם השירות
          זמין.
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
            נסה שוב
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
            דווח על תקלה
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
            סטטוס שירותים
          </a>
        </div>
      </div>
    </main>
  );
}
