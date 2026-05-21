// Tiny fetch wrapper shared by both clients. We keep the surface minimal so
// the SDK works in Node 20+, Bun, Deno, and browsers without extra polyfills.

export type TokenProvider = () => string | null | undefined | Promise<string | null | undefined>;

export interface TransportConfig {
  /** Absolute base URL (no trailing slash). */
  baseUrl: string;
  /** Returns the current bearer token; called per request so callers can rotate. */
  getToken?: TokenProvider;
  /** Optional fetch override for testing or non-browser environments. */
  fetch?: typeof fetch;
  /** Optional default headers merged into every request. */
  headers?: Record<string, string>;
}

export class IronflyerError extends Error {
  constructor(public status: number, public bodyText: string, message?: string) {
    super(message ?? `Ironflyer ${status}: ${bodyText.slice(0, 200)}`);
    this.name = 'IronflyerError';
  }
}

export class Transport {
  constructor(private cfg: TransportConfig) {
    this.cfg.baseUrl = cfg.baseUrl.replace(/\/+$/, '');
  }

  get baseUrl(): string { return this.cfg.baseUrl; }

  async authHeader(): Promise<Record<string, string>> {
    if (!this.cfg.getToken) return {};
    const tok = await this.cfg.getToken();
    return tok ? { Authorization: `Bearer ${tok}` } : {};
  }

  /** appendTokenParam decorates URLs for EventSource (which can't set headers). */
  async appendTokenParam(url: string): Promise<string> {
    if (!this.cfg.getToken) return url;
    const tok = await this.cfg.getToken();
    if (!tok) return url;
    return url + (url.includes('?') ? '&' : '?') + 'token=' + encodeURIComponent(tok);
  }

  async json<T>(path: string, init?: RequestInit): Promise<T> {
    const auth = await this.authHeader();
    const f = this.cfg.fetch ?? fetch;
    const res = await f(`${this.cfg.baseUrl}${path}`, {
      ...init,
      headers: {
        'Content-Type': 'application/json',
        ...this.cfg.headers,
        ...auth,
        ...(init?.headers ?? {}),
      },
      cache: 'no-store',
    });
    const text = await res.text();
    if (!res.ok) {
      throw new IronflyerError(res.status, text);
    }
    if (!text) return undefined as T;
    try {
      return JSON.parse(text) as T;
    } catch {
      throw new IronflyerError(res.status, text, `invalid JSON response from ${path}`);
    }
  }

  /** Raw fetch — used for non-JSON endpoints (file blobs, streams). */
  async raw(path: string, init?: RequestInit): Promise<Response> {
    const auth = await this.authHeader();
    const f = this.cfg.fetch ?? fetch;
    return f(`${this.cfg.baseUrl}${path}`, {
      ...init,
      headers: {
        ...this.cfg.headers,
        ...auth,
        ...(init?.headers ?? {}),
      },
      cache: 'no-store',
    });
  }
}
