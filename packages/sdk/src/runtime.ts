import { Transport, type TransportConfig } from './http.js';
import type {
  ExecRequest, ExecResult, RuntimeFileEntry, Workspace,
} from './types.js';

// RuntimeClient wraps the Ironflyer workspace runtime HTTP API: workspace
// lifecycle, file I/O, command exec, and the PTY WebSocket URL (callers
// open the socket themselves since SDKs shouldn't pull in a WS library).
export class RuntimeClient {
  private t: Transport;

  constructor(cfg: TransportConfig) {
    this.t = new Transport(cfg);
  }

  health() {
    return this.t.json<{ ok: boolean; service: string; driver: string; authMode: string }>(
      '/health');
  }

  // ---------- Workspaces -----------------------------------------------------

  list() { return this.t.json<Workspace[]>('/workspaces'); }
  get(id: string) { return this.t.json<Workspace>(`/workspaces/${id}`); }

  create(body: { userId?: string; projectId?: string } = {}) {
    return this.t.json<Workspace>('/workspaces', {
      method: 'POST', body: JSON.stringify(body),
    });
  }

  async destroy(id: string): Promise<void> {
    const res = await this.t.raw(`/workspaces/${id}`, { method: 'DELETE' });
    if (!res.ok && res.status !== 404) {
      throw new Error(`destroy ${id}: ${res.status} ${await res.text()}`);
    }
  }

  // ---------- Files ----------------------------------------------------------

  listFiles(id: string) {
    return this.t.json<RuntimeFileEntry[]>(`/workspaces/${id}/files`);
  }

  async readFile(id: string, path: string): Promise<string> {
    const res = await this.t.raw(`/workspaces/${id}/files/${encodeURI(path)}`);
    if (!res.ok) throw new Error(`${res.status}: ${await res.text()}`);
    return res.text();
  }

  async writeFile(id: string, path: string, data: string | Uint8Array): Promise<void> {
    const body: BodyInit = typeof data === 'string'
      ? data
      // Slice into a fresh ArrayBuffer so TS doesn't trip on SharedArrayBuffer
      // unions across DOM lib versions; runtime cost is one shallow copy.
      : new Blob([data.buffer.slice(data.byteOffset, data.byteOffset + data.byteLength) as ArrayBuffer]);
    const res = await this.t.raw(`/workspaces/${id}/files/${encodeURI(path)}`, {
      method: 'PUT', body,
    });
    if (!res.ok) throw new Error(`${res.status}: ${await res.text()}`);
  }

  // ---------- Exec / git -----------------------------------------------------

  exec(id: string, body: ExecRequest) {
    return this.t.json<ExecResult>(`/workspaces/${id}/exec`, {
      method: 'POST', body: JSON.stringify(body),
    });
  }

  gitClone(id: string, body: { cloneUrl: string; token?: string; ref?: string; subdir?: string }) {
    return this.t.json<{ status: string }>(`/workspaces/${id}/git-clone`, {
      method: 'POST', body: JSON.stringify(body),
    });
  }

  // ---------- Terminal -------------------------------------------------------

  /**
   * terminalURL returns the WebSocket URL for a workspace PTY. EventSource /
   * fetch can't drive a duplex socket so callers wire it through a WS client
   * of their choice (browser WebSocket, ws, etc.). Replace the http(s) scheme
   * with ws(s) ourselves so callers don't have to.
   */
  terminalURL(id: string): string {
    const u = `${this.t.baseUrl}/workspaces/${id}/terminal`;
    return u.replace(/^http/, 'ws');
  }
}
