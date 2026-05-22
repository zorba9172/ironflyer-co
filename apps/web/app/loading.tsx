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
        <div
          style={{
            width: 56,
            height: 56,
            background: '#e5ff00',
            borderRadius: 12,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontFamily: 'var(--font-display), Arial Black, sans-serif',
            fontSize: 36,
            color: '#0d0e0f',
            lineHeight: 1,
          }}
          aria-hidden
        >
          I
        </div>

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
          טוען…
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
