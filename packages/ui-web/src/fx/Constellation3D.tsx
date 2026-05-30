import { lazy, Suspense } from 'react';
import { useMounted } from './useMounted';
import type { Constellation3DNode, Constellation3DLink } from './Constellation3DInner';

export type { Constellation3DNode, Constellation3DLink };

const Inner = lazy(() => import('./Constellation3DInner'));

export type Constellation3DProps = {
  nodes: Constellation3DNode[];
  links: Constellation3DLink[];
  colors: string[];
  height?: number;
  rotate?: boolean;
};

// Lazy, client-only 3D node constellation bound to a graph. three.js loads on
// mount behind Suspense — never in the cold bundle. Pass a palette so each
// surface keeps its own color discipline.
export function Constellation3D({ nodes, links, colors, height = 320, rotate = true }: Constellation3DProps) {
  const mounted = useMounted();
  if (!mounted) return <div style={{ height }} aria-hidden="true" />;
  return (
    <Suspense fallback={<div style={{ height }} aria-hidden="true" />}>
      <Inner nodes={nodes} links={links} colors={colors} height={height} rotate={rotate} />
    </Suspense>
  );
}
