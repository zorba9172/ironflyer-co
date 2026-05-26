// Live Preview webview view. Lives in the Ironflyer activity bar; renders
// the runtime's per-workspace preview URL inside a sandboxed iframe so the
// user can see their app evolve while the agent works.
//
// Lifecycle:
//   - The view is resolved by VSCode once on activity-bar reveal.
//   - State changes (active project pinned, sign-in, refresh) call
//     `setProject()` which re-queries the runtime for a fresh preview URL.
//   - The webview owns viewport switching (responsive / mobile / tablet /
//     desktop) and a refresh button that bumps the iframe's `src` with a
//     cache-busting query param.
//   - If no workspace exists yet for the project we render a friendly empty
//     state with a "Run Finisher" button.

import * as vscode from 'vscode';
import { Api, ApiError, Project, Workspace } from './api';
import { Auth } from './auth';
import { readConfig, ViewportPreset } from './config';
import { log } from './logger';

type State =
  | { kind: 'signed-out' }
  | { kind: 'no-project' }
  | { kind: 'loading'; project: Project }
  | { kind: 'ready'; project: Project; previewUrl: string; workspace: Workspace }
  | { kind: 'empty'; project: Project; reason: string }
  | { kind: 'error'; project: Project; message: string };

interface InboundMessage {
  type:
    | 'refresh'
    | 'openExternal'
    | 'runFinisher'
    | 'createWorkspace'
    | 'signIn'
    | 'pickProject'
    | 'viewportChanged';
  viewport?: ViewportPreset;
}

export class PreviewView implements vscode.WebviewViewProvider, vscode.Disposable {
  static readonly viewId = 'ironflyer.preview';

  private view: vscode.WebviewView | undefined;
  private state: State = { kind: 'signed-out' };
  private project: Project | undefined;
  private viewport: ViewportPreset;
  private readonly disposables: vscode.Disposable[] = [];

  constructor(
    private readonly api: Api,
    private readonly auth: Auth,
    private readonly extensionUri: vscode.Uri,
  ) {
    this.viewport = readConfig().previewViewport;
    this.disposables.push(this.auth.onDidChange(() => void this.refresh()));
  }

  resolveWebviewView(view: vscode.WebviewView): void {
    this.view = view;
    view.webview.options = {
      enableScripts: true,
      localResourceRoots: [vscode.Uri.joinPath(this.extensionUri, 'media')],
    };
    view.webview.onDidReceiveMessage(
      (msg: InboundMessage) => void this.handle(msg),
      undefined,
      this.disposables,
    );
    view.onDidDispose(() => { this.view = undefined; }, undefined, this.disposables);
    void this.refresh();
  }

  /**
   * Called by the host whenever the active project changes. We re-resolve
   * the workspace immediately so the iframe reflects what the user pinned.
   */
  async setProject(project: Project | undefined): Promise<void> {
    this.project = project;
    await this.refresh();
  }

  /** Re-render the webview with the latest state from the runtime. */
  async refresh(): Promise<void> {
    if (!this.view) return;
    const token = await this.auth.getToken();
    if (!token) {
      this.render({ kind: 'signed-out' });
      return;
    }
    if (!this.project) {
      this.render({ kind: 'no-project' });
      return;
    }
    this.render({ kind: 'loading', project: this.project });
    try {
      const ws = await this.api.findWorkspaceForProject(this.project.id);
      if (!ws) {
        this.render({
          kind: 'empty',
          project: this.project,
          reason: 'No workspace yet for this project. Run the Finisher to provision one.',
        });
        return;
      }
      if (!ws.previewUrl) {
        this.render({
          kind: 'empty',
          project: this.project,
          reason: `Workspace ${ws.id} has no live preview URL yet — the sandbox is still booting.`,
        });
        return;
      }
      this.render({ kind: 'ready', project: this.project, previewUrl: ws.previewUrl, workspace: ws });
    } catch (err) {
      log.warn('preview refresh failed', err);
      const message = err instanceof ApiError
        ? `${err.status} — ${err.message}`
        : (err as Error).message;
      this.render({ kind: 'error', project: this.project, message });
    }
  }

  dispose(): void {
    while (this.disposables.length) this.disposables.pop()?.dispose();
  }

  private render(state: State): void {
    this.state = state;
    if (!this.view) return;
    this.view.webview.html = renderHtml(this.view.webview, state, this.viewport);
    this.updateBadge(state);
  }

  private updateBadge(state: State): void {
    if (!this.view) return;
    switch (state.kind) {
      case 'loading':
        this.view.description = 'loading…';
        break;
      case 'ready':
        this.view.description = state.project.name;
        break;
      case 'empty':
        this.view.description = 'no preview';
        break;
      case 'error':
        this.view.description = 'error';
        break;
      default:
        this.view.description = undefined;
    }
  }

  private async handle(msg: InboundMessage): Promise<void> {
    switch (msg.type) {
      case 'refresh':
        await this.refresh();
        return;
      case 'openExternal':
        if (this.state.kind === 'ready') {
          void vscode.env.openExternal(vscode.Uri.parse(this.state.previewUrl));
        }
        return;
      case 'runFinisher':
        await vscode.commands.executeCommand('ironflyer.runFinisher');
        return;
      case 'createWorkspace':
        if (!this.project) return;
        try {
          await vscode.window.withProgress(
            { location: { viewId: PreviewView.viewId }, title: 'Provisioning workspace…' },
            () => this.api.createWorkspace(this.project!.id),
          );
        } catch (err) {
          log.warn('createWorkspace failed', err);
          void vscode.window.showErrorMessage(`Ironflyer: ${(err as Error).message}`);
        }
        await this.refresh();
        return;
      case 'signIn':
        await vscode.commands.executeCommand('ironflyer.signIn');
        return;
      case 'pickProject':
        await vscode.commands.executeCommand('ironflyer.setActiveProject');
        return;
      case 'viewportChanged':
        if (msg.viewport) {
          this.viewport = msg.viewport;
          // re-render preserves state but updates viewport class
          this.render(this.state);
        }
        return;
    }
  }
}

function renderHtml(
  webview: vscode.Webview,
  state: State,
  viewport: ViewportPreset,
): string {
  const nonce = randomNonce();
  // CSP: scripts only with our nonce; iframes are allowed from http(s); style
  // inline is acceptable because the runtime preview URL is trusted.
  const csp = [
    `default-src 'none'`,
    `style-src ${webview.cspSource} 'unsafe-inline'`,
    `script-src 'nonce-${nonce}'`,
    `img-src ${webview.cspSource} https: data:`,
    `frame-src https: http: data:`,
  ].join('; ');

  return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8" />
<meta http-equiv="Content-Security-Policy" content="${csp}" />
<title>Ironflyer Live Preview</title>
<style>
  :root { color-scheme: var(--vscode-color-scheme, light dark); }
  html, body {
    height: 100%; margin: 0; padding: 0;
    background: var(--vscode-editor-background);
    color: var(--vscode-foreground);
    font-family: var(--vscode-font-family);
    font-size: 13px;
    overflow: hidden;
  }
  .toolbar {
    display: flex; align-items: center; gap: 6px;
    padding: 6px 8px;
    border-bottom: 1px solid var(--vscode-panel-border);
    background: var(--vscode-sideBar-background);
  }
  .toolbar select, .toolbar button {
    background: var(--vscode-button-secondaryBackground, transparent);
    color: var(--vscode-button-secondaryForeground, var(--vscode-foreground));
    border: 1px solid var(--vscode-input-border, var(--vscode-panel-border));
    border-radius: 4px;
    padding: 3px 8px;
    font: inherit;
    cursor: pointer;
  }
  .toolbar button:hover { background: var(--vscode-button-secondaryHoverBackground, var(--vscode-list-hoverBackground)); }
  .toolbar .spacer { flex: 1; }
  .toolbar .label { opacity: 0.75; font-size: 11px; text-transform: uppercase; letter-spacing: 0.04em; }
  .stage {
    height: calc(100% - 38px); display: flex; align-items: stretch; justify-content: center;
    background: var(--vscode-editorWidget-background);
    overflow: auto;
  }
  .frame-shell {
    width: 100%; height: 100%; background: white;
    box-shadow: 0 1px 8px rgba(0,0,0,0.25);
    border: 0; transition: width 0.18s ease, height 0.18s ease;
  }
  .stage.mobile  .frame-shell { width: 390px;  height: 844px; max-height: 100%; margin: 8px auto; border-radius: 18px; }
  .stage.tablet  .frame-shell { width: 820px;  height: 100%;  max-width: 100%; margin: 8px auto; border-radius: 10px; }
  .stage.desktop .frame-shell { width: 1280px; height: 100%;  max-width: 100%; margin: 8px auto; }
  iframe { width: 100%; height: 100%; border: 0; background: white; }
  .empty {
    display: flex; flex-direction: column; align-items: center; justify-content: center;
    height: 100%; gap: 12px; text-align: center; padding: 24px;
  }
  .empty h2 { margin: 0; font-size: 14px; font-weight: 600; }
  .empty p  { margin: 0; opacity: 0.75; max-width: 32em; line-height: 1.45; }
  .empty button {
    background: var(--vscode-button-background); color: var(--vscode-button-foreground);
    border: 0; border-radius: 4px; padding: 6px 14px; cursor: pointer; font: inherit;
  }
  .empty button:hover { background: var(--vscode-button-hoverBackground); }
  .dot {
    display: inline-block; width: 8px; height: 8px; border-radius: 50%;
    background: var(--vscode-charts-green, #4ade80); margin-right: 6px;
  }
  .dot.warn  { background: var(--vscode-charts-orange, #f59e0b); }
  .dot.error { background: var(--vscode-charts-red,    #ef4444); }
  .dot.idle  { background: var(--vscode-foreground); opacity: 0.4; }
</style>
</head>
<body>
${renderBody(state, viewport)}
<script nonce="${nonce}">
  const vscode = acquireVsCodeApi();
  const send = (msg) => vscode.postMessage(msg);
  const $ = (sel) => document.querySelector(sel);

  const refresh = $('#refresh');         if (refresh) refresh.onclick = () => send({ type: 'refresh' });
  const openExt = $('#open-external');   if (openExt) openExt.onclick = () => send({ type: 'openExternal' });
  const runFin  = $('#run-finisher');    if (runFin)  runFin.onclick  = () => send({ type: 'runFinisher' });
  const create  = $('#create-workspace');if (create)  create.onclick  = () => send({ type: 'createWorkspace' });
  const signIn  = $('#sign-in');         if (signIn)  signIn.onclick  = () => send({ type: 'signIn' });
  const pick    = $('#pick-project');    if (pick)    pick.onclick    = () => send({ type: 'pickProject' });

  const vp = $('#viewport');
  if (vp) vp.onchange = () => {
    const stage = $('.stage');
    if (stage) {
      stage.classList.remove('mobile','tablet','desktop','responsive');
      stage.classList.add(vp.value);
    }
    send({ type: 'viewportChanged', viewport: vp.value });
  };
</script>
</body>
</html>`;
}

function renderBody(state: State, viewport: ViewportPreset): string {
  switch (state.kind) {
    case 'signed-out':
      return `<div class="empty">
  <h2>Sign in to Ironflyer</h2>
  <p>The Live Preview shows your project's sandboxed workspace in real time.</p>
  <button id="sign-in">Sign In</button>
</div>`;
    case 'no-project':
      return `<div class="empty">
  <h2>Pin a project</h2>
  <p>Tell Ironflyer which project this window belongs to and we'll point the preview at its workspace.</p>
  <button id="pick-project">Pick a project</button>
</div>`;
    case 'loading':
      return `<div class="empty">
  <h2><span class="dot idle"></span> Loading preview…</h2>
  <p>Asking the runtime for the workspace tied to <strong>${escapeHtml(state.project.name)}</strong>.</p>
</div>`;
    case 'empty':
      return `<div class="empty">
  <h2><span class="dot warn"></span> No live preview yet</h2>
  <p>${escapeHtml(state.reason)}</p>
  <div style="display:flex;gap:8px">
    <button id="run-finisher">Run Finisher</button>
    <button id="create-workspace">Provision Workspace</button>
    <button id="refresh">Retry</button>
  </div>
</div>`;
    case 'error':
      return `<div class="empty">
  <h2><span class="dot error"></span> Couldn't load the preview</h2>
  <p>${escapeHtml(state.message)}</p>
  <button id="refresh">Retry</button>
</div>`;
    case 'ready': {
      const cacheBust = `${state.previewUrl}${state.previewUrl.includes('?') ? '&' : '?'}_t=${Date.now()}`;
      return `<div class="toolbar">
  <span class="label"><span class="dot"></span>${escapeHtml(state.project.name)}</span>
  <span class="spacer"></span>
  <select id="viewport" title="Viewport">
    <option value="responsive" ${viewport === 'responsive' ? 'selected' : ''}>Responsive</option>
    <option value="mobile"     ${viewport === 'mobile'     ? 'selected' : ''}>Mobile (390×844)</option>
    <option value="tablet"     ${viewport === 'tablet'     ? 'selected' : ''}>Tablet (820×—)</option>
    <option value="desktop"    ${viewport === 'desktop'    ? 'selected' : ''}>Desktop (1280×—)</option>
  </select>
  <button id="refresh" title="Refresh">⟳ Refresh</button>
  <button id="open-external" title="Open in browser">↗ Browser</button>
</div>
<div class="stage ${escapeHtml(viewport)}">
  <iframe class="frame-shell"
    src="${escapeHtml(cacheBust)}"
    sandbox="allow-scripts allow-forms allow-same-origin allow-popups allow-modals"
    referrerpolicy="no-referrer"
    title="Ironflyer Live Preview"></iframe>
</div>`;
    }
  }
}

function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  } as Record<string, string>)[c]);
}

function randomNonce(): string {
  return [...Array(32)].map(() => Math.floor(Math.random() * 16).toString(16)).join('');
}
