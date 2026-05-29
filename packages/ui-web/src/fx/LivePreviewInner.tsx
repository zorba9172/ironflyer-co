import { useEffect } from 'react';
import { SandpackProvider, SandpackPreview, SandpackLayout, useSandpack } from '@codesandbox/sandpack-react';

type Template = 'vite-react-ts' | 'static';

// Bridges the in-browser bundler's compile/runtime messages to a plain
// callback, so the host surface can offer a "fix it" action when a build
// breaks. Lives inside SandpackProvider so it can use the Sandpack context.
function ErrorBridge({ onError }: { onError?: (message: string | null) => void }) {
  const { listen } = useSandpack();
  useEffect(() => {
    if (!onError) return;
    return listen((msg) => {
      const m = msg as unknown as { type: string; action?: string; message?: string; title?: string; compilatonError?: boolean };
      if (m.type === 'action' && m.action === 'show-error') {
        onError(m.message || m.title || 'The preview failed to build.');
      } else if (m.type === 'success' || (m.type === 'done' && !m.compilatonError)) {
        onError(null);
      }
    });
  }, [listen, onError]);
  return null;
}

// Runs the generated project in an in-browser bundler (web worker) and shows
// only the rendered output — the editable source lives in the Code tab.
export default function LivePreviewInner({ files, template, dark, onError }: {
  files: Record<string, string>;
  template: Template;
  dark: boolean;
  onError?: (message: string | null) => void;
}) {
  // Pull declared dependencies out of package.json (if present) so libraries
  // the app imports (react-router, etc.) resolve. The file itself is dropped
  // so it can't override the template's entry/scripts.
  let dependencies: Record<string, string> | undefined;
  const pkgRaw = files['/package.json'];
  if (pkgRaw) {
    try {
      const p = JSON.parse(pkgRaw) as { dependencies?: Record<string, string> };
      if (p.dependencies && Object.keys(p.dependencies).length > 0) dependencies = p.dependencies;
    } catch {
      /* malformed package.json — fall back to template defaults */
    }
  }
  const sandpackFiles = { ...files };
  delete sandpackFiles['/package.json'];

  return (
    <SandpackProvider
      template={template}
      theme={dark ? 'dark' : 'light'}
      files={sandpackFiles}
      customSetup={dependencies ? { dependencies } : undefined}
      options={{ recompileMode: 'delayed', recompileDelay: 400 }}
      style={{ height: '100%' }}
    >
      <ErrorBridge onError={onError} />
      <SandpackLayout style={{ height: '100%', border: 'none', borderRadius: 0 }}>
        <SandpackPreview showNavigator showRefreshButton showOpenInCodeSandbox={false} style={{ height: '100%' }} />
      </SandpackLayout>
    </SandpackProvider>
  );
}
