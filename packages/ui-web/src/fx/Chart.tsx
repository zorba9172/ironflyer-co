import { lazy, Suspense } from 'react';
import type { EChartsOption } from 'echarts';
import { useMounted } from './useMounted';

export type { EChartsOption };

const Inner = lazy(() => import('./ChartInner'));

// Lazy echarts chart — the chart lib loads only on the client when mounted.
export function Chart({ option, height = 280 }: { option: EChartsOption; height?: number | string }) {
  const mounted = useMounted();
  if (!mounted) return <div style={{ height }} />;
  return (
    <Suspense fallback={<div style={{ height }} />}>
      <Inner option={option} height={height} />
    </Suspense>
  );
}
