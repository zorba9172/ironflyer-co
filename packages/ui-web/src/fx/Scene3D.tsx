import { lazy, Suspense } from 'react';
import { useMounted } from './useMounted';

const Inner = lazy(() => import('./Scene3DInner'));

// Lazy three.js accent — client-only, never in the cold bundle.
export function Scene3D({ height = 320 }: { height?: number }) {
  const mounted = useMounted();
  if (!mounted) return <div style={{ height }} aria-hidden="true" />;
  return (
    <Suspense fallback={<div style={{ height }} aria-hidden="true" />}>
      <Inner height={height} />
    </Suspense>
  );
}
