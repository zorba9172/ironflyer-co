"use client";

// useIDEWriteBack — bridges the openvscode container's on-disk state
// back into projectFiles so Monaco + the finisher loop see edits the
// operator made inside the cloud IDE.
//
// Flow:
//   1. Monaco's projectFiles → /api/ide/sync POST (handled in
//      IDEFramePane mount effect) primes the IDE workspace.
//   2. This hook then polls /api/ide/sync (GET) on a fixed cadence,
//      hashes each returned file, and diffs against the last set we
//      pushed.
//   3. If anything diverged (add, modify, delete), we call the
//      writeProjectFiles GraphQL mutation with the full new set and
//      refetch ProjectFiles so the Studio's Monaco view updates.
//
// Why a "full set" push rather than diff patches: the orchestrator's
// projectFiles is a single Files slice on domain.Project, so the
// resolver already takes a replacement set. Diffing client-side adds
// complexity without a server-side win.
//
// AI-generated changes still flow through patch.Engine.Propose — this
// path exists only for human operator edits made directly inside the
// embedded VS Code, which would otherwise be invisible to the rest of
// the platform.

import { useCallback, useEffect, useRef, useState } from "react";
import {
  ProjectFilesDocument,
  useWriteProjectFilesMutation,
} from "../../lib/gql/__generated__";

const POLL_INTERVAL_MS = 4000;
const INITIAL_POLL_DELAY_MS = 1500;

export type WriteBackStatus =
  | "idle"
  | "watching"
  | "pushing"
  | "synced"
  | "failed";

export interface WriteBackState {
  status: WriteBackStatus;
  lastPushedAt: number | null;
  lastError: string | null;
  pushedFileCount: number;
}

interface IDEFile {
  path: string;
  content: string;
}

interface SeedFile {
  path: string;
  content: string | null;
}

interface IDESyncGetResponse {
  folder: string;
  files: Array<{ path: string; content: string; size: number; updatedAt: string }>;
}

export interface UseIDEWriteBackArgs {
  projectID: string;
  // Wait until the iframe has loaded AND the initial Monaco→IDE push
  // finished. Otherwise we'd push the empty pre-seed state straight
  // back over projectFiles.
  enabled: boolean;
  // The current Monaco snapshot used to prime the "last pushed"
  // fingerprint so we don't mistake the existing state for an edit.
  seedFiles: SeedFile[] | null;
}

export function useIDEWriteBack({
  projectID,
  enabled,
  seedFiles,
}: UseIDEWriteBackArgs): WriteBackState {
  const [state, setState] = useState<WriteBackState>({
    status: "idle",
    lastPushedAt: null,
    lastError: null,
    pushedFileCount: 0,
  });

  // The fingerprint is path → sha256(content). We only push when the
  // IDE's set diverges from this; on every successful push we
  // overwrite it. Stored in a ref so the polling effect doesn't
  // re-fire on every fingerprint change.
  const fingerprintRef = useRef<Map<string, string> | null>(null);
  const aliveRef = useRef(true);
  const [writeProjectFiles] = useWriteProjectFilesMutation();

  // Seed the fingerprint when projectFiles first arrive so the very
  // first poll doesn't see "everything is new" and trigger a no-op
  // round-trip push.
  useEffect(() => {
    if (!seedFiles || fingerprintRef.current) return;
    let cancelled = false;
    (async () => {
      const next = new Map<string, string>();
      for (const f of seedFiles) {
        if (!f.path || f.content == null) continue;
        next.set(f.path, await sha256(f.content));
      }
      if (cancelled) return;
      fingerprintRef.current = next;
    })();
    return () => {
      cancelled = true;
    };
  }, [seedFiles]);

  const pushBack = useCallback(
    async (files: IDEFile[]) => {
      setState((s) => ({ ...s, status: "pushing", lastError: null }));
      try {
        await writeProjectFiles({
          variables: {
            id: projectID,
            files: files.map((f) => ({ path: f.path, content: f.content })),
          },
          refetchQueries: [
            { query: ProjectFilesDocument, variables: { id: projectID } },
          ],
          awaitRefetchQueries: false,
        });
        if (!aliveRef.current) return;
        setState({
          status: "synced",
          lastPushedAt: Date.now(),
          lastError: null,
          pushedFileCount: files.length,
        });
      } catch (e) {
        if (!aliveRef.current) return;
        setState((s) => ({
          ...s,
          status: "failed",
          lastError: e instanceof Error ? e.message : "writeback failed",
        }));
      }
    },
    [projectID, writeProjectFiles],
  );

  useEffect(() => {
    aliveRef.current = true;
    if (!enabled || !projectID) {
      setState((s) => ({ ...s, status: "idle" }));
      return () => {
        aliveRef.current = false;
      };
    }
    let timer: ReturnType<typeof setTimeout> | null = null;

    const tick = async () => {
      if (!aliveRef.current) return;
      try {
        const res = await fetch(
          `/api/ide/sync?projectID=${encodeURIComponent(projectID)}`,
          { cache: "no-store" },
        );
        if (!res.ok) {
          // Sync route returns 401 if the cookie expired — surface
          // that as a watching state so we don't spam errors while
          // the user is reauthenticating in another tab.
          if (res.status === 401 && aliveRef.current) {
            setState((s) => ({ ...s, status: "watching" }));
          }
          schedule();
          return;
        }
        const body = (await res.json()) as IDESyncGetResponse;
        const ideFiles = (body.files ?? []).filter(
          (f) => typeof f.path === "string" && typeof f.content === "string",
        );
        const observed = new Map<string, string>();
        for (const f of ideFiles) {
          observed.set(f.path, await sha256(f.content));
        }
        const fingerprint = fingerprintRef.current ?? new Map<string, string>();
        if (!fingerprintsEqual(observed, fingerprint)) {
          await pushBack(ideFiles);
          fingerprintRef.current = observed;
        } else if (aliveRef.current) {
          setState((s) =>
            s.status === "watching" || s.status === "synced"
              ? s
              : { ...s, status: "watching" },
          );
        }
      } catch (e) {
        if (aliveRef.current) {
          setState((s) => ({
            ...s,
            status: "failed",
            lastError: e instanceof Error ? e.message : "poll failed",
          }));
        }
      }
      schedule();
    };

    const schedule = () => {
      if (!aliveRef.current) return;
      timer = setTimeout(tick, POLL_INTERVAL_MS);
    };

    setState((s) => ({ ...s, status: "watching" }));
    timer = setTimeout(tick, INITIAL_POLL_DELAY_MS);

    return () => {
      aliveRef.current = false;
      if (timer) clearTimeout(timer);
    };
  }, [enabled, projectID, pushBack]);

  return state;
}

function fingerprintsEqual(
  a: Map<string, string>,
  b: Map<string, string>,
): boolean {
  if (a.size !== b.size) return false;
  for (const [k, v] of a) {
    if (b.get(k) !== v) return false;
  }
  return true;
}

async function sha256(text: string): Promise<string> {
  if (typeof crypto === "undefined" || !crypto.subtle) {
    // Fallback for environments without WebCrypto. Length + first
    // chars is a weak fingerprint but enough to detect drastic
    // changes; production browsers all expose subtle.
    return `len:${text.length}:${text.slice(0, 32)}`;
  }
  const data = new TextEncoder().encode(text);
  const digest = await crypto.subtle.digest("SHA-256", data);
  const bytes = new Uint8Array(digest);
  let out = "";
  for (const b of bytes) {
    out += b.toString(16).padStart(2, "0");
  }
  return out;
}
