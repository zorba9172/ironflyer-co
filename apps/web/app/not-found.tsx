import Link from 'next/link';
import type { Metadata } from 'next';

export const metadata: Metadata = {
  title: '404 — הדף לא נמצא | Ironflyer',
  description: 'הדף שחיפשתם נעלם או לא קיים. חזרו לעמוד הבית.',
};

// Branded 404. Hebrew-first copy, lime CTA back home, single subtle
// illustration drawn in CSS so we don't pay an extra network round-trip
// to render the not-found state.
export default function NotFound() {
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
          gap: 24,
        }}
      >
        {/* Subtle illustration — 404 made from a lime block and a glyph */}
        <div
          aria-hidden
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 12,
            fontFamily: 'var(--font-display), Arial Black, sans-serif',
            fontSize: 96,
            lineHeight: 1,
            letterSpacing: -3,
          }}
        >
          <span>4</span>
          <span
            style={{
              display: 'inline-flex',
              width: 84,
              height: 84,
              background: '#e5ff00',
              borderRadius: 12,
              alignItems: 'center',
              justifyContent: 'center',
              color: '#0d0e0f',
            }}
          >
            0
          </span>
          <span>4</span>
        </div>

        <h1
          style={{
            margin: 0,
            fontFamily: 'var(--font-display), Arial Black, sans-serif',
            fontSize: '2rem',
            letterSpacing: -0.5,
          }}
        >
          הדף הזה כבר נסגר.
        </h1>
        <p style={{ margin: 0, color: '#3a3a36', maxWidth: 440, lineHeight: 1.5 }}>
          כנראה שהקישור שגוי או שהדף הוסר. בדקו את הכתובת או חזרו לעמוד הבית כדי
          להתחיל מחדש.
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
            חזרה לעמוד הבית
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
            בדיקת סטטוס שירותים
          </Link>
        </div>
      </div>
    </main>
  );
}
