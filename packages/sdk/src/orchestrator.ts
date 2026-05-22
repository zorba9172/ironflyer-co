import { Transport, type TransportConfig } from './http.js';
import type {
  Agent, AuthResponse, AuthUser, BrainstormOutcome, ChatDelta, GateState,
  LedgerEntry, Plan, Project, Rate, RunReport, UserBudget, VaultSnapshot,
} from './types.js';

// OrchestratorClient wraps every public endpoint of the Ironflyer orchestrator
// HTTP API. Construct with new OrchestratorClient({...}) or use the top-level
// ironflyer() factory to share a token provider across orchestrator + runtime.
export class OrchestratorClient {
  private t: Transport;

  constructor(cfg: TransportConfig) {
    this.t = new Transport(cfg);
  }

  // ---------- Auth -----------------------------------------------------------

  health() {
    return this.t.json<{ ok: boolean; service: string; version: string }>('/health');
  }

  signup(body: { email: string; password: string; name?: string }) {
    return this.t.json<AuthResponse>('/auth/signup', {
      method: 'POST', body: JSON.stringify(body),
    });
  }

  login(body: { email: string; password: string }) {
    return this.t.json<AuthResponse>('/auth/login', {
      method: 'POST', body: JSON.stringify(body),
    });
  }

  me() { return this.t.json<AuthUser>('/auth/me'); }

  // ---------- Projects -------------------------------------------------------

  listProjects() { return this.t.json<Project[]>('/projects'); }
  getProject(id: string) { return this.t.json<Project>(`/projects/${id}`); }

  createProject(body: { id?: string; name: string; description?: string; idea?: string }) {
    return this.t.json<Project>('/projects', { method: 'POST', body: JSON.stringify(body) });
  }

  listGates(id: string) { return this.t.json<GateState[]>(`/projects/${id}/gates`); }

  runFinisher(id: string) {
    return this.t.json<RunReport>(`/projects/${id}/run`, { method: 'POST' });
  }

  brainstorm(id: string, body: { goal: string; role?: string }) {
    return this.t.json<BrainstormOutcome>(`/projects/${id}/brainstorm`, {
      method: 'POST', body: JSON.stringify(body),
    });
  }

  // ---------- Budget ---------------------------------------------------------

  // ---------- Agents ---------------------------------------------------------

  /** Full agent catalogue — role, system prompt, capability tags. */
  listAgents() { return this.t.json<Agent[]>('/agents'); }

  listPlans() { return this.t.json<Plan[]>('/budget/plans'); }
  listRates() { return this.t.json<Rate[]>('/budget/rates'); }
  vault() { return this.t.json<VaultSnapshot>('/budget/vault'); }
  myBudget() { return this.t.json<UserBudget>('/budget/users/me'); }

  startCheckout(tier: string) {
    return this.t.json<{ url: string }>('/budget/checkout', {
      method: 'POST', body: JSON.stringify({ tier }),
    });
  }

  // ---------- GitHub integration --------------------------------------------

  githubMe() {
    return this.t.json<{ connected: boolean; login?: string; scopes?: string[] }>(
      '/integrations/github/me');
  }

  githubRepos() {
    return this.t.json<Array<{
      fullName: string; defaultBranch: string; htmlUrl: string; private: boolean;
    }>>('/integrations/github/repos');
  }

  connectProjectToRepo(id: string, body: {
    owner?: string; repo?: string; fullName?: string;
    defaultBranch?: string; htmlUrl?: string;
  }) {
    return this.t.json<Project>(`/projects/${id}/connect-github`, {
      method: 'POST', body: JSON.stringify(body),
    });
  }

  cloneIntoWorkspace(id: string, body: { workspaceId: string; ref?: string; subdir?: string }) {
    return this.t.json<{ status: string }>(`/projects/${id}/clone-into-workspace`, {
      method: 'POST', body: JSON.stringify(body),
    });
  }

  // ---------- Streaming chat -------------------------------------------------

  /**
   * Returns the URL of the project execution SSE stream with the bearer
   * token appended as ?token=… so EventSource (which cannot set headers)
   * still authenticates.
   */
  streamURL(id: string) {
    return this.t.appendTokenParam(`${this.t.baseUrl}/projects/${id}/stream`);
  }

  /**
   * streamChat opens a POST SSE stream against /chat. Browsers do not allow
   * EventSource for POST so we use fetch + ReadableStream and parse SSE.
   * Pass an AbortSignal to cancel mid-stream.
   */
  async streamChat(
    projectId: string,
    body: { prompt: string; role?: string; effort?: 'lite' | 'economy' | 'power' },
    onDelta: (d: ChatDelta) => void,
    signal?: AbortSignal,
  ): Promise<void> {
    const headers = {
      'Content-Type': 'application/json',
      ...(await this.t.authHeader()),
    };
    const f = (globalThis as { fetch?: typeof fetch }).fetch ?? fetch;
    const res = await f(`${this.t.baseUrl}/projects/${projectId}/chat`, {
      method: 'POST', headers, body: JSON.stringify(body), signal,
    });
    if (!res.ok || !res.body) {
      onDelta({ kind: 'error', error: `${res.status}: ${await res.text()}` });
      return;
    }
    const reader = res.body.getReader();
    const dec = new TextDecoder();
    let buf = '';
    for (;;) {
      const { value, done } = await reader.read();
      if (done) return;
      buf += dec.decode(value, { stream: true });
      let idx: number;
      while ((idx = buf.indexOf('\n\n')) >= 0) {
        const block = buf.slice(0, idx);
        buf = buf.slice(idx + 2);
        let event = 'message';
        let data = '';
        for (const line of block.split('\n')) {
          if (line.startsWith('event:')) event = line.slice(6).trim();
          else if (line.startsWith('data:')) data += line.slice(5).trim();
        }
        if (!data) continue;
        try {
          const parsed = JSON.parse(data) as Record<string, unknown>;
          switch (event) {
            case 'turn': onDelta({ kind: 'turn', id: String(parsed.id), role: String(parsed.role) }); break;
            case 'start': onDelta({ kind: 'start', provider: String(parsed.provider), model: String(parsed.model), turn: String(parsed.turn) }); break;
            case 'text': onDelta({ kind: 'text', text: String(parsed.text), turn: String(parsed.turn) }); break;
            case 'thinking': onDelta({ kind: 'thinking', text: String(parsed.text), turn: String(parsed.turn) }); break;
            case 'tool_use': onDelta({ kind: 'tool_use', data: parsed }); break;
            case 'done': onDelta({ kind: 'done', turn: String(parsed.turn), provider: String(parsed.provider), model: String(parsed.model), usage: parsed.usage }); break;
            case 'error': onDelta({ kind: 'error', error: String(parsed.error) }); break;
          }
        } catch {
          // Malformed SSE event — swallow so a single bad packet doesn't kill the stream.
        }
      }
    }
  }

  // ---------- Lower-level helpers callers occasionally need ------------------

  listLedger() { return this.t.json<LedgerEntry[]>('/budget/users/me'); }
}
