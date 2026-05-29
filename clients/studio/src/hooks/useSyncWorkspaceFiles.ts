import { useEffect, useRef } from 'react';
import { writeWorkspaceFile } from '@ironflyer/data';
import { useStudio } from '../store';

// Streams the chat-generated files into the runtime workspace so generated code
// shows up in the editor live as it is produced. The chat upserts files into
// the store with a bumped `rev` on every change (see ChatPanel), so we push
// only the files whose rev advanced since we last sent them — incremental, not
// a full re-push on every token. Best-effort: a failed write is retried on the
// next rev bump and never blocks the others.
export function useSyncWorkspaceFiles(projectId: string | undefined, ready: boolean) {
  const files = useStudio((s) => s.generatedFiles);
  // path -> last rev successfully sent, scoped to the active projectId.
  const sent = useRef<{ projectId?: string; revs: Map<string, number> }>({
    projectId: undefined,
    revs: new Map(),
  });
  const inFlight = useRef(false);

  useEffect(() => {
    if (!ready || !projectId || files.length === 0 || inFlight.current) {
      return;
    }
    // Reset the per-path ledger when the workspace changes.
    if (sent.current.projectId !== projectId) {
      sent.current = { projectId, revs: new Map() };
    }
    const pending = files.filter((f) => (sent.current.revs.get(f.path) ?? -1) < f.rev);
    if (pending.length === 0) {
      return;
    }

    inFlight.current = true;
    let cancelled = false;
    (async () => {
      for (const f of pending) {
        if (cancelled) {
          break;
        }
        try {
          await writeWorkspaceFile(projectId, f.path, f.content);
          sent.current.revs.set(f.path, f.rev);
        } catch {
          // leave this path unmarked so it retries on the next rev bump
        }
      }
      inFlight.current = false;
    })();
    return () => {
      cancelled = true;
      inFlight.current = false;
    };
  }, [ready, projectId, files]);
}
