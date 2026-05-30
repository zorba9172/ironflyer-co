import { lazy, Suspense } from 'react';
import { useMounted } from './useMounted';
import type { Bar3DDatum } from './Bars3DInner';

export type { Bar3DDatum };

const Inner = lazy(() => import('./Bars3DInner'));

export type Bars3DProps = {
  data: Bar3DDatum[];
  /** ordered palette; each bar cycles through it unless the datum sets a color */
  colors: string[];
  height?: number;
  /** value mapped to the tallest bar; defaults to the data max */
  max?: number;
  rotate?: boolean;
};

// Lazy, client-only 3D bar field bound to a data series. three.js never lands
// in the cold bundle — it loads on mount behind Suspense. Colors are passed in
// so each surface keeps its own palette discipline (studio neon, brand cobalt…).
export function Bars3D({ data, colors, height = 320, max, rotate = true }: Bars3DProps) {
  const mounted = useMounted();
  if (!mounted) return <div style={{ height }} aria-hidden="true" />;
  return (
    <Suspense fallback={<div style={{ height }} aria-hidden="true" />}>
      <Inner data={data} colors={colors} height={height} max={max} rotate={rotate} />
    </Suspense>
  );
}
