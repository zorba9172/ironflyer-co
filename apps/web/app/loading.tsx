// Global suspense fallback. No spinner — a static lime glyph plus a
// minimal skeleton row to anchor layout while the real content streams in.
// Spinners imply "indefinite waiting"; a static placeholder reads as
// "almost ready" which matches the product promise.
export default function Loading() {
  return (
    <main
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: '#f4f0e8',
        color: '#0d0e0f',
        fontFamily: 'var(--font-body), Inter, sans-serif',
      }}
    >
      <div
        style={{
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          gap: 24,
          width: 'min(420px, 80vw)',
        }}
      >
        {/* Logo glyph */}
        <svg width="64" height="64" viewBox="0 0 64 64" aria-hidden>
          <rect x="4" y="4" width="56" height="56" rx="8" fill="#0d0e0f" />
          <path d="M19 14h13c9 0 15 5 15 13 0 6-3 10-9 12l10 11H35L26 40h-3v10H12V14h7Z" fill="#e5ff00" />
          <path d="M23 23h12c3 0 5 2 5 5s-2 5-5 5H23V23Z" fill="#0d0e0f" />
          <path d="M15 14h10v36H15V14Z" fill="#e5ff00" />
          <path d="M28 18h16v4H28V18Zm0 12h16v4H28v-4Zm0 12h16v4H28v-4Z" fill="#f4f0e8" />
          <path d="M46 24l8 8-8 8v-6h-6v-4h6v-6Z" fill="#f4f0e8" />
        </svg>

        {/* Skeleton rows — keyframes are scoped via <style> so we don't
            need to touch the global stylesheet from this file. */}
        <style>{`@keyframes ironflyer-skeleton {
          0% { background-position: 200% 0; }
          100% { background-position: -200% 0; }
        }`}</style>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12, width: '100%' }}>
          <div style={skeletonStyle(220)} />
          <div style={skeletonStyle(320)} />
          <div style={skeletonStyle(180)} />
        </div>

        <span
          style={{
            color: '#77736b',
            fontSize: '0.875rem',
          }}
        >
          Loading...
        </span>
      </div>
    </main>
  );
}

function skeletonStyle(width: number): React.CSSProperties {
  return {
    height: 14,
    width,
    maxWidth: '100%',
    background: 'linear-gradient(90deg, #e7dfd2 0%, #f4f0e8 50%, #e7dfd2 100%)',
    backgroundSize: '200% 100%',
    borderRadius: 4,
    animation: 'ironflyer-skeleton 1.4s ease-in-out infinite',
  };
}
