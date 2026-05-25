// runtime.ts — tiny fetch client for the apps/runtime workspace HTTP API.
//
// The orchestrator never proxies workspace file traffic — the browser
// hits the runtime pod directly. That keeps the GraphQL layer small and
// lets large file payloads stream without GraphQL deserialization.
//
// Convention used by the studio: workspaceID == executionID. The
// runtime accepts an orchestrator-supplied workspace id at create time;
// every paid execution uses its execution id as the workspace id so the
// two stay 1:1. If that ever changes, swap in bundle.workspaceID from
// GraphQL — this client only needs the id string.
//
// All calls use browser-native fetch (no axios) and forward the auth
// token as a Bearer header. Reads cap content at 256KB and tag binary
// files so the viewer can render a placeholder instead of crashing the
// page on a 10MB tarball.

import { getToken } from "./apollo";

const RUNTIME_BASE =
  process.env.NEXT_PUBLIC_RUNTIME_URL || "http://localhost:8090";

// Cap browser-side file reads at 256KB. The runtime itself does not
// enforce this — it streams whatever the driver returns — so we apply
// the cap client-side by slicing the response.
const MAX_READ_BYTES = 256 * 1024;

export interface FileEntry {
  path: string; // relative to workspace root
  kind: "file" | "dir";
  size?: number;
  modifiedAt?: string;
}

export interface FileContent {
  path: string;
  content: string; // utf-8 text, OR base64 when isBinary is true
  isBinary?: boolean;
  truncated?: boolean;
  size: number; // raw byte count of the file on disk
}

// Raw shape the runtime returns from GET /workspaces/{id}/files.
// Source of truth: apps/runtime/internal/sandbox/manager.go FileEntry.
interface RuntimeFileEntry {
  path: string;
  size: number;
  isDir: boolean;
}

class RuntimeError extends Error {
  readonly status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
    this.name = "RuntimeError";
  }
}

function authHeaders(token?: string): Record<string, string> {
  const t = token ?? getToken();
  if (!t) return {};
  return { Authorization: `Bearer ${t}` };
}

async function runtimeFetch(
  path: string,
  init: RequestInit,
  token?: string,
): Promise<Response> {
  const url = `${RUNTIME_BASE}${path}`;
  const res = await fetch(url, {
    ...init,
    headers: {
      ...(init.headers ?? {}),
      ...authHeaders(token),
    },
  });
  return res;
}

// listWorkspaceFiles fetches the recursive file listing for a workspace.
// The runtime's GET /workspaces/{id}/files always returns the full tree
// (the driver walks the workspace root in one pass). We accept opts so
// the call site can pre-filter by subpath for future incremental loads,
// but for now we always return the full list and let the tree component
// prune client-side.
export async function listWorkspaceFiles(
  workspaceID: string,
  opts?: { path?: string; recursive?: boolean },
  token?: string,
): Promise<FileEntry[]> {
  if (!workspaceID) return [];
  const res = await runtimeFetch(
    `/workspaces/${encodeURIComponent(workspaceID)}/files`,
    { method: "GET" },
    token,
  );
  if (!res.ok) {
    if (res.status === 404) return [];
    const txt = await safeText(res);
    throw new RuntimeError(res.status, `listFiles failed: ${txt || res.statusText}`);
  }
  const raw = (await res.json()) as RuntimeFileEntry[] | null;
  if (!Array.isArray(raw)) return [];

  let entries: FileEntry[] = raw.map((e) => ({
    path: e.path,
    kind: e.isDir ? "dir" : "file",
    size: e.size,
  }));

  // Optional client-side subpath filter. The runtime doesn't honor the
  // `path` query argument today (the handler ignores it) so we trim
  // here. Same outcome, just paid in CPU instead of bandwidth.
  const sub = opts?.path?.replace(/^\/+|\/+$/g, "");
  if (sub) {
    const prefix = `${sub}/`;
    entries = entries.filter(
      (e) => e.path === sub || e.path.startsWith(prefix),
    );
  }
  return entries;
}

// readWorkspaceFile pulls a single file's contents. The runtime returns
// raw bytes with content-type application/octet-stream; we detect
// binary by null-byte sniff and base64-encode when needed. UTF-8 files
// are decoded on the spot. Truncation is applied client-side at 256KB.
export async function readWorkspaceFile(
  workspaceID: string,
  path: string,
  token?: string,
): Promise<FileContent> {
  if (!workspaceID) {
    throw new RuntimeError(400, "missing workspaceID");
  }
  // The runtime route is `/workspaces/{id}/files/*` so each segment
  // gets URL-encoded individually to preserve slashes.
  const safePath = path
    .split("/")
    .filter(Boolean)
    .map((seg) => encodeURIComponent(seg))
    .join("/");
  const res = await runtimeFetch(
    `/workspaces/${encodeURIComponent(workspaceID)}/files/${safePath}`,
    { method: "GET" },
    token,
  );
  if (!res.ok) {
    const txt = await safeText(res);
    throw new RuntimeError(res.status, `readFile failed: ${txt || res.statusText}`);
  }
  const buf = new Uint8Array(await res.arrayBuffer());
  const size = buf.byteLength;

  // Detect binary by scanning the first 8KB for null bytes — same
  // heuristic git + ripgrep use. Cheap and correct enough for the
  // studio viewer.
  const sniff = buf.subarray(0, Math.min(buf.byteLength, 8 * 1024));
  let isBinary = false;
  for (let i = 0; i < sniff.length; i++) {
    if (sniff[i] === 0) {
      isBinary = true;
      break;
    }
  }

  if (isBinary) {
    // Return base64 of the first 256KB so the viewer has *something*
    // to show but never tries to render megabytes of garbage.
    const slice = buf.subarray(0, Math.min(buf.byteLength, MAX_READ_BYTES));
    return {
      path,
      content: uint8ToBase64(slice),
      isBinary: true,
      truncated: buf.byteLength > MAX_READ_BYTES,
      size,
    };
  }

  // Text path. Slice first then decode — TextDecoder over a sliced
  // view skips allocating a giant intermediate string.
  const truncated = buf.byteLength > MAX_READ_BYTES;
  const slice = truncated ? buf.subarray(0, MAX_READ_BYTES) : buf;
  // fatal:false tolerates invalid UTF-8 in source files that snuck in
  // (e.g. an editor saved Latin-1). The replacement chars are
  // preferable to a thrown error.
  const text = new TextDecoder("utf-8", { fatal: false }).decode(slice);
  return {
    path,
    content: text,
    truncated,
    size,
  };
}

async function safeText(res: Response): Promise<string> {
  try {
    const t = await res.text();
    return t.slice(0, 500);
  } catch {
    return "";
  }
}

// uint8ToBase64 — chunked btoa so we don't blow the call-stack on a
// 256KB binary payload. 32KB chunks keep us comfortably within every
// browser's argument-list limit.
function uint8ToBase64(buf: Uint8Array): string {
  const chunk = 0x8000;
  let s = "";
  for (let i = 0; i < buf.length; i += chunk) {
    s += String.fromCharCode.apply(
      null,
      Array.from(buf.subarray(i, i + chunk)),
    );
  }
  if (typeof btoa === "function") return btoa(s);
  // Node / SSR fallback. The studio is client-only, but the import
  // resolver still type-checks this file in the server bundle.
  return Buffer.from(s, "binary").toString("base64");
}
