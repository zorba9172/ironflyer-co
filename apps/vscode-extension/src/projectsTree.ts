// Sidebar TreeView showing the signed-in user's Ironflyer projects.
//
// We use the orchestrator's GET /projects, which is already auth-scoped to
// the caller, so there's nothing to filter client-side. Children of a
// project show a snapshot of file count + Finisher gates state, fetched
// lazily on expand so the initial render stays snappy.

import * as vscode from 'vscode';
import { Api, Project } from './api';
import { Auth } from './auth';

export class ProjectsTree implements vscode.TreeDataProvider<Node> {
  private readonly _onDidChange = new vscode.EventEmitter<Node | undefined>();
  readonly onDidChangeTreeData = this._onDidChange.event;

  constructor(private readonly api: Api, private readonly auth: Auth) {
    auth.onDidChange(() => this.refresh());
  }

  refresh(node?: Node): void {
    this._onDidChange.fire(node);
  }

  getTreeItem(node: Node): vscode.TreeItem {
    if (node.kind === 'project') {
      const item = new vscode.TreeItem(node.project.name, vscode.TreeItemCollapsibleState.Collapsed);
      item.id = node.project.id;
      item.description = node.project.description ?? node.project.spec?.idea ?? '';
      item.tooltip = node.project.description ?? '';
      item.contextValue = 'ironflyer.project';
      item.iconPath = new vscode.ThemeIcon('rocket');
      item.command = {
        command: 'ironflyer.openChat',
        title: 'Open chat',
        arguments: [node.project.id],
      };
      return item;
    }
    const item = new vscode.TreeItem(node.label);
    item.description = node.value;
    item.iconPath = node.icon ? new vscode.ThemeIcon(node.icon) : undefined;
    return item;
  }

  async getChildren(parent?: Node): Promise<Node[]> {
    if (!parent) {
      const token = await this.auth.getToken();
      if (!token) return [];
      try {
        const projects = await this.api.listProjects();
        return projects.map((p) => ({ kind: 'project' as const, project: p }));
      } catch (err) {
        void vscode.window.showErrorMessage(`Ironflyer: failed to load projects — ${(err as Error).message}`);
        return [];
      }
    }
    if (parent.kind === 'project') {
      const p = parent.project;
      return [
        { kind: 'meta', label: 'Files', value: String(p.files?.length ?? '?'), icon: 'files' },
        { kind: 'meta', label: 'ID', value: p.id, icon: 'symbol-key' },
        { kind: 'meta', label: 'Visibility', value: p.isPublic ? 'public' : 'private', icon: 'eye' },
      ];
    }
    return [];
  }
}

type Node =
  | { kind: 'project'; project: Project }
  | { kind: 'meta'; label: string; value: string; icon?: string };
