// Webview panel that hosts the per-project chat. The webview owns DOM/UI;
// the extension owns auth, the SSE connection, and the project context.
// Messages flow over `postMessage` in both directions.

import * as vscode from 'vscode';
import { Api, Project, SSEEvent } from './api';
import { ProjectStream } from './projectStream';

interface InboundMessage {
  type: 'prompt';
  text: string;
  role?: string;
  effort?: string;
}

interface OutboundMessage {
  type: string;
  [k: string]: any;
}

export interface ChatPanelDeps {
  stream: ProjectStream;
  onProjectEvent(projectId: string, event: SSEEvent): void;
}

export class ChatPanel {
  private static readonly panels = new Map<string, ChatPanel>();

  static reveal(
    api: Api,
    ctx: vscode.ExtensionContext,
    project: Project,
    deps: ChatPanelDeps,
  ): ChatPanel {
    const existing = ChatPanel.panels.get(project.id);
    if (existing) {
      existing.panel.reveal();
      return existing;
    }
    const panel = vscode.window.createWebviewPanel(
      'ironflyer.chat',
      `Ironflyer · ${project.name}`,
      vscode.ViewColumn.Beside,
      {
        enableScripts: true,
        retainContextWhenHidden: true,
        localResourceRoots: [vscode.Uri.joinPath(ctx.extensionUri, 'media')],
      },
    );
    return new ChatPanel(panel, api, ctx, project, deps);
  }

  /**
   * Host-driven submission — used by the "Ask Ironflyer to fix" code
   * action. Renders the user turn in the webview for transparency, then
   * fires the same chat pipeline as if the user had typed it.
   */
  submitFromHost(text: string, role?: string, effort?: string): void {
    if (!text.trim()) return;
    this.post({ type: 'user-message', text });
    void this.handle({ type: 'prompt', text, role, effort });
  }

  private readonly disposables: vscode.Disposable[] = [];
  private abort: AbortController | undefined;
  private streamSub: { dispose(): void } | undefined;

  private constructor(
    private readonly panel: vscode.WebviewPanel,
    private readonly api: Api,
    ctx: vscode.ExtensionContext,
    private readonly project: Project,
    private readonly deps: ChatPanelDeps,
  ) {
    ChatPanel.panels.set(project.id, this);
    panel.webview.html = renderHtml(panel.webview, ctx.extensionUri, project);
    panel.onDidDispose(() => this.dispose(), null, this.disposables);
    panel.webview.onDidReceiveMessage(
      (msg: InboundMessage) => this.handle(msg),
      null,
      this.disposables,
    );
    this.streamSub = this.deps.stream.subscribe(project.id, (evt) => {
      this.deps.onProjectEvent(project.id, evt);
      // Surface execution-stream events to the chat log so the user sees
      // gate progress without staring at the trees.
      this.post({ type: 'lifecycle', data: evt.data });
    });
  }

  private async handle(msg: InboundMessage): Promise<void> {
    if (msg.type !== 'prompt') return;
    if (!msg.text || !msg.text.trim()) return;
    this.abort?.abort();
    this.abort = new AbortController();
    this.post({ type: 'turn-start' });
    try {
      const stream = this.api.chat(
        this.project.id,
        { prompt: msg.text, role: msg.role, effort: msg.effort },
        this.abort.signal,
      );
      for await (const evt of stream) {
        this.post({ type: 'sse', event: evt.event, data: evt.data });
        if (evt.event === 'done' || evt.event === 'error') break;
      }
    } catch (err) {
      if ((err as Error).name === 'AbortError') return;
      this.post({ type: 'error', message: (err as Error).message });
    } finally {
      this.post({ type: 'turn-end' });
    }
  }

  private post(msg: OutboundMessage): void {
    void this.panel.webview.postMessage(msg);
  }

  private dispose(): void {
    ChatPanel.panels.delete(this.project.id);
    this.abort?.abort();
    this.streamSub?.dispose();
    this.streamSub = undefined;
    while (this.disposables.length) this.disposables.pop()?.dispose();
    this.panel.dispose();
  }
}

function renderHtml(webview: vscode.Webview, root: vscode.Uri, project: Project): string {
  const nonce = randomNonce();
  const css = webview.asWebviewUri(vscode.Uri.joinPath(root, 'media', 'chat.css'));
  const js = webview.asWebviewUri(vscode.Uri.joinPath(root, 'media', 'chat.js'));
  const csp = [
    `default-src 'none'`,
    `style-src ${webview.cspSource} 'unsafe-inline'`,
    `script-src 'nonce-${nonce}'`,
    `img-src ${webview.cspSource} https: data:`,
  ].join('; ');
  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta http-equiv="Content-Security-Policy" content="${csp}" />
  <link rel="stylesheet" href="${css}" />
  <title>Ironflyer · ${escapeHtml(project.name)}</title>
</head>
<body>
  <header>
    <div class="brand"><span class="dot"></span><strong>Ironflyer</strong> · ${escapeHtml(project.name)}</div>
    <div class="meta">${escapeHtml(project.description ?? project.spec?.idea ?? '')}</div>
  </header>
  <main id="log" aria-live="polite"></main>
  <form id="composer" autocomplete="off">
    <div class="row">
      <select id="role" title="Agent role">
        <option value="planner" selected>planner</option>
        <option value="architect">architect</option>
        <option value="coder">coder</option>
        <option value="reviewer">reviewer</option>
        <option value="tester">tester</option>
        <option value="security">security</option>
      </select>
      <select id="effort" title="Effort">
        <option value="lite">Lite</option>
        <option value="economy" selected>Economy</option>
        <option value="power">Power</option>
      </select>
    </div>
    <textarea id="prompt" rows="3" placeholder="Ask Ironflyer to finish something…"></textarea>
    <div class="row">
      <button type="submit" id="send">Send</button>
      <button type="button" id="cancel" hidden>Cancel</button>
    </div>
  </form>
  <script nonce="${nonce}" src="${js}"></script>
</body>
</html>`;
}

function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  } as Record<string, string>)[c]);
}

function randomNonce(): string {
  return [...Array(32)].map(() => Math.floor(Math.random() * 16).toString(16)).join('');
}
