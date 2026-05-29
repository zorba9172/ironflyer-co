import { lazy, Suspense } from 'react';
import { useMounted } from './useMounted';

const Inner = lazy(() => import('./LivePreviewInner'));

export type LivePreviewTemplate = 'vite-react-ts' | 'static';

// Lazy in-browser preview (Sandpack runs a bundler in a web worker). Renders
// the generated project's output; the source stays in the Code tab. `onError`
// fires with the bundler's message when a build breaks (null once it recovers)
// so the host can offer a one-click "fix it" action.
export function LivePreview({ files, template = 'vite-react-ts', dark = true, onError }: {
  files: Record<string, string>;
  template?: LivePreviewTemplate;
  dark?: boolean;
  onError?: (message: string | null) => void;
}) {
  const mounted = useMounted();
  if (!mounted) return <div style={{ height: '100%' }} />;
  return (
    <Suspense fallback={<div style={{ height: '100%' }} />}>
      <Inner files={files} template={template} dark={dark} onError={onError} />
    </Suspense>
  );
}
