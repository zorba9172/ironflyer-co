// Dependency Graph webview. Renders the orchestrator's projectgraph.Graph
// as a Mermaid `graph LR` diagram. The webview pulls Mermaid from the
// jsDelivr CDN at runtime — bundling the library would balloon the
// extension VSIX with no real upside for what is a peek-only view.
//
// The panel owns its own message protocol: the webview fires `refresh`
// when the toolbar refresh button is clicked, and the host posts
// `setGraph` whenever new data lands.

import * as vscode from 'vscode';
import { Api, ProjectGraph } from './api';
import { ActiveProject } from './activeProject';

export class GraphView {
  static readonly viewType = 'ironflyer.dependencyGraph';
  private static current: GraphView | undefined;

  private readonly panel: vscode.WebviewPanel;
  private readonly disposables: vscode.Disposable[] = [];
  private disposed = false;

  static reveal(api: Api, active: ActiveProject): GraphView | undefined {
    const ref = active.get();
    if (!ref) {
      void vscode.window.showWarningMessage('Pin an Ironflyer project to view its dependency graph.');
      return undefined;
    }
    if (GraphView.current && !GraphView.current.disposed) {
      GraphView.current.panel.reveal(vscode.ViewColumn.Active);
      void GraphView.current.reload();
      return GraphView.current;
    }
    const panel = vscode.window.createWebviewPanel(
      GraphView.viewType,
      `Ironflyer · ${ref.name} · Dependency Graph`,
      vscode.ViewColumn.Active,
      { enableScripts: true, retainContextWhenHidden: true },
    );
    const view = new GraphView(panel, api, active);
    GraphView.current = view;
    return view;
  }

  private constructor(
    panel: vscode.WebviewPanel,
    private readonly api: Api,
    private readonly active: ActiveProject,
  ) {
    this.panel = panel;
    this.panel.webview.html = this.renderHtml();
    this.disposables.push(
      this.panel.onDidDispose(() => this.dispose()),
      this.panel.webview.onDidReceiveMessage((msg) => {
        if (msg?.type === 'refresh') void this.reload();
      }),
      this.active.onDidChange(() => void this.reload()),
    );
    void this.reload();
  }

  private async reload(): Promise<void> {
    const ref = this.active.get();
    if (!ref) {
      void this.panel.webview.postMessage({
        type: 'setGraph',
        diagram: '%% no project pinned',
        title: 'No project pinned',
      });
      return;
    }
    this.panel.title = `Ironflyer · ${ref.name} · Dependency Graph`;
    try {
      const graph = await this.api.projectGraph(ref.id);
      const diagram = toMermaid(graph);
      void this.panel.webview.postMessage({ type: 'setGraph', diagram, title: ref.name });
    } catch (err) {
      void this.panel.webview.postMessage({
        type: 'setGraph',
        diagram: `%% failed to load graph: ${(err as Error).message}`,
        title: ref.name,
        error: (err as Error).message,
      });
    }
  }

  dispose(): void {
    if (this.disposed) return;
    this.disposed = true;
    if (GraphView.current === this) GraphView.current = undefined;
    this.disposables.forEach((d) => d.dispose());
    this.panel.dispose();
  }

  private renderHtml(): string {
    const csp = [
      "default-src 'none'",
      // Mermaid script — single external dependency, pinned to a minor.
      "script-src https://cdn.jsdelivr.net 'unsafe-inline'",
      "style-src 'unsafe-inline'",
      "font-src https://cdn.jsdelivr.net data:",
      "img-src data:",
    ].join('; ');
    return `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta http-equiv="Content-Security-Policy" content="${csp}" />
    <title>Ironflyer · Dependency Graph</title>
    <style>
      body {
        margin: 0;
        font-family: var(--vscode-font-family);
        color: var(--vscode-foreground);
        background: var(--vscode-editor-background);
      }
      header {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 8px 14px;
        border-bottom: 1px solid var(--vscode-panel-border);
        position: sticky;
        top: 0;
        background: var(--vscode-editor-background);
      }
      header h1 {
        font-size: 13px;
        font-weight: 600;
        margin: 0;
        flex: 1;
      }
      button {
        background: var(--vscode-button-background);
        color: var(--vscode-button-foreground);
        border: none;
        padding: 4px 10px;
        cursor: pointer;
        border-radius: 4px;
        font-size: 12px;
      }
      button:hover { background: var(--vscode-button-hoverBackground); }
      main { padding: 16px; overflow: auto; }
      .error {
        color: var(--vscode-errorForeground);
        font-family: var(--vscode-editor-font-family);
        white-space: pre-wrap;
        padding: 12px;
        background: var(--vscode-textBlockQuote-background);
        border-left: 3px solid var(--vscode-errorForeground);
      }
      .empty {
        color: var(--vscode-descriptionForeground);
        font-style: italic;
        padding: 12px;
      }
      pre.mermaid { background: transparent; }
    </style>
  </head>
  <body>
    <header>
      <h1 id="title">Ironflyer · Dependency Graph</h1>
      <button id="refresh" title="Refresh">↻ Refresh</button>
    </header>
    <main id="main">
      <div class="empty">Loading dependency graph…</div>
    </main>
    <script type="module">
      import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.esm.min.mjs';
      mermaid.initialize({
        startOnLoad: false,
        theme: 'dark',
        securityLevel: 'strict',
      });
      const vscode = acquireVsCodeApi();
      const main = document.getElementById('main');
      const titleEl = document.getElementById('title');
      const btn = document.getElementById('refresh');
      btn.addEventListener('click', () => {
        main.innerHTML = '<div class="empty">Refreshing…</div>';
        vscode.postMessage({ type: 'refresh' });
      });
      let renderSeq = 0;
      window.addEventListener('message', async (event) => {
        const msg = event.data;
        if (!msg || msg.type !== 'setGraph') return;
        if (msg.title) titleEl.textContent = 'Ironflyer · ' + msg.title + ' · Dependency Graph';
        if (msg.error) {
          main.innerHTML = '';
          const div = document.createElement('div');
          div.className = 'error';
          div.textContent = msg.error;
          main.appendChild(div);
          return;
        }
        const seq = ++renderSeq;
        try {
          const { svg } = await mermaid.render('graph-' + seq, msg.diagram);
          if (seq !== renderSeq) return;
          main.innerHTML = svg;
        } catch (err) {
          if (seq !== renderSeq) return;
          main.innerHTML = '';
          const div = document.createElement('div');
          div.className = 'error';
          div.textContent = 'Failed to render diagram: ' + (err && err.message ? err.message : String(err));
          main.appendChild(div);
        }
      });
    </script>
  </body>
</html>`;
  }
}

/**
 * Convert a project graph to a Mermaid `graph LR` source string. Each
 * file becomes a node with a sanitized id; each edge becomes an arrow.
 * Empty graphs return a comment so Mermaid still parses successfully.
 */
export function toMermaid(graph: ProjectGraph): string {
  if (!graph || (graph.nodes.length === 0 && graph.edges.length === 0)) {
    return 'graph LR\n  empty[No dependencies detected yet]';
  }
  const ids = new Map<string, string>();
  const idFor = (path: string): string => {
    let id = ids.get(path);
    if (id) return id;
    id = `n${ids.size}`;
    ids.set(path, id);
    return id;
  };
  const lines: string[] = ['graph LR'];
  for (const node of graph.nodes) {
    const id = idFor(node.path);
    lines.push(`  ${id}["${escapeMermaidLabel(node.path)}"]`);
  }
  for (const edge of graph.edges) {
    // Ensure both endpoints have ids even if the node list omitted one.
    const from = idFor(edge.from);
    const to = idFor(edge.to);
    lines.push(`  ${from} --> ${to}`);
  }
  return lines.join('\n');
}

function escapeMermaidLabel(s: string): string {
  return s.replace(/"/g, '\\"');
}
