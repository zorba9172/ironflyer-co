import { lazy, Suspense } from 'react';
import { useMounted } from './useMounted';

const Inner = lazy(() => import('./CodeEditorInner'));

// Lazy CodeMirror 6 editor (not VS Code) — light, fast, themable.
export function CodeEditor({ value, language, height = '100%', dark = true, readOnly, onChange }: {
  value: string;
  language?: string;
  height?: number | string;
  dark?: boolean;
  readOnly?: boolean;
  onChange?: (v: string) => void;
}) {
  const mounted = useMounted();
  if (!mounted) return <div style={{ height }} />;
  return (
    <Suspense fallback={<div style={{ height }} />}>
      <Inner value={value} language={language} height={height} dark={dark} readOnly={readOnly} onChange={onChange} />
    </Suspense>
  );
}
