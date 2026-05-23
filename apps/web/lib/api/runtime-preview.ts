// Runtime preview helpers — talk to the runtime's /workspaces/{id}/ports
// endpoint and mint signed preview tokens for the iframe.
//
// Wire format (canonical, owned by the runtime):
//   GET  /workspaces/{id}/ports           -> Array<DetectedPort>
//   POST /workspaces/{id}/preview-token   -> { url, path, token, expiresAt }
//
// DetectedPort native shape:
//   { port, source, firstSeen, lastSeen, previewPath, allowed }
//
// We expose a normalised `PortMapping` to the UI so call sites stay simple.

import { Workspace } from '../runtime';

const base = '/api/runtime';

interface RuntimeDetectedPort {
  port: number;
  source: string;
  firstSeen: string;
  lastSeen: string;
  previewPath: string;
  allowed: boolean;
  healthy?: boolean;
  latencyMs?: number;
}

interface RuntimePreviewToken {
  url: string;
  path: string;
  token: string;
  expiresAt: string;
}

export interface PortMapping {
  port: number;
  scheme: 'http' | 'https';
  ready: boolean;
  url?: string;
  source?: string;
  label?: string;
  // Runtime liveness probe results (runtime polls each forwarded port on
  // GET /ports). `healthy` undefined means the runtime did not probe (older
  // runtime, or port not allowed). `latencyMs` is only populated on a
  // successful probe.
  healthy?: boolean;
  latencyMs?: number;
}

export interface PortsResponse {
  ports: PortMapping[];
  previewToken?: string;
}

function labelForPort(port: number): string {
  switch (port) {
    case 3000:
      return 'Next.js / Node';
    case 5173:
    case 4173:
      return 'Vite';
    case 4321:
      return 'Astro';
    case 8080:
    case 8000:
      return 'Server';
    case 4000:
      return 'GraphQL / Phoenix';
    default:
      return `Port ${port}`;
  }
}

function normalisePort(p: RuntimeDetectedPort): PortMapping {
  return {
    port: p.port,
    scheme: 'http',
    ready: p.allowed,
    source: p.source,
    label: labelForPort(p.port),
    healthy: p.healthy,
    latencyMs: p.latencyMs,
  };
}

export async function getWorkspacePorts(workspaceId: string): Promise<PortsResponse> {
  const res = await fetch(`${base}/workspaces/${workspaceId}/ports`, { cache: 'no-store' });
  if (!res.ok) {
    if (res.status === 404) return { ports: [] };
    throw new Error(`${res.status}: ${await res.text()}`);
  }
  const raw = (await res.json()) as RuntimeDetectedPort[] | { ports?: RuntimeDetectedPort[] };
  const list: RuntimeDetectedPort[] = Array.isArray(raw) ? raw : raw.ports ?? [];
  return { ports: list.map(normalisePort) };
}

/** Mint a signed, short-lived token for `port` so iframes work without auth headers. */
export async function mintPreviewToken(
  workspaceId: string,
  port: number,
): Promise<RuntimePreviewToken | null> {
  const res = await fetch(`${base}/workspaces/${workspaceId}/preview-token`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ port }),
    cache: 'no-store',
  });
  if (!res.ok) return null;
  return (await res.json()) as RuntimePreviewToken;
}

export interface RuntimeShareLink {
  url: string;
  path: string;
  token: string;
  expiresAt: string;
  ttlHours: number;
}

/**
 * Mint a long-lived share link the owner can hand to teammates. The token
 * is bound to (workspace, port) and expires after `ttlHours` (default 7
 * days, capped at 30 by the runtime).
 */
export async function mintShareLink(
  workspaceId: string,
  port: number,
  ttlHours?: number,
): Promise<RuntimeShareLink | null> {
  const res = await fetch(`${base}/workspaces/${workspaceId}/share-link`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ port, ttlHours }),
    cache: 'no-store',
  });
  if (!res.ok) return null;
  const raw = (await res.json()) as RuntimeShareLink;
  // Runtime returns the path part of the URL. Resolve to an absolute URL
  // the user can paste into Slack: the proxy is the web app's own origin
  // (Next rewrite forwards /api/runtime/preview/* to the runtime).
  if (typeof window !== 'undefined' && raw.url.startsWith('/')) {
    return { ...raw, url: window.location.origin + raw.url };
  }
  return raw;
}

// resolvePreviewURL picks the best preview URL for a workspace:
//   1. explicit `chosen.url` from a future runtime that supplies absolute URLs,
//   2. construct `/api/runtime/preview/{workspaceID}/{port}/` (Next rewrite),
//   3. fall back to `workspace.previewUrl` (legacy single-port shape).
// Returns null when no preview is available yet.
export function resolvePreviewURL(
  workspace: Workspace | null,
  ports: PortMapping[],
  token: string | undefined,
  selectedPort?: number,
): string | null {
  if (!workspace) return null;
  const ready = ports.filter((p) => p.ready);
  const explicit = typeof selectedPort === 'number'
    ? ready.find((p) => p.port === selectedPort)
    : undefined;
  const chosen: PortMapping | undefined = explicit ?? ready[0];
  if (chosen?.url) {
    return token ? appendToken(chosen.url, token) : chosen.url;
  }
  if (chosen) {
    const u = `${base}/preview/${workspace.id}/${chosen.port}/`;
    return token ? appendToken(u, token) : u;
  }
  return workspace.previewUrl ?? null;
}

function appendToken(url: string, token: string): string {
  const sep = url.includes('?') ? '&' : '?';
  return `${url}${sep}t=${encodeURIComponent(token)}`;
}
