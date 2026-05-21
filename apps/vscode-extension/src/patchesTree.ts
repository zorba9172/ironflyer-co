// Patches sidebar TreeView. Three levels:
//
//   Project (only those with ≥1 patch)
//     └── Patch (status badge in the description)
//           └── Change (op + path)
//
// We snapshot the patch list per project on first expand and again on
// `refresh()`. The PatchDiffProvider warms up the same cache so opening
// a diff is instant.

import * as vscode from 'vscode';
import { Api, Patch, PatchChange, Project } from './api';
import { Auth } from './auth';
import { PatchDiffProvider } from './diffProvider';
import { shortId } from './patchUri';

type Node =
  | { kind: 'project'; project: Project }
  | { kind: 'patch'; projectId: string; patch: Patch }
  | { kind: 'change'; projectId: string; patch: Patch; change: PatchChange };

export class PatchesTree implements vscode.TreeDataProvider<Node> {
  private readonly _onDidChange = new vscode.EventEmitter<Node | undefined>();
  readonly onDidChangeTreeData = this._onDidChange.event;

  constructor(
    private readonly api: Api,
    private readonly auth: Auth,
    private readonly diff: PatchDiffProvider,
  ) {
    auth.onDidChange(() => this.refresh());
  }

  refresh(node?: Node): void {
    this.diff.invalidate();
    this._onDidChange.fire(node);
  }

  getTreeItem(node: Node): vscode.TreeItem {
    if (node.kind === 'project') {
      const item = new vscode.TreeItem(node.project.name, vscode.TreeItemCollapsibleState.Collapsed);
      item.iconPath = new vscode.ThemeIcon('repo');
      item.contextValue = 'ironflyer.patchProject';
      return item;
    }
    if (node.kind === 'patch') {
      const item = new vscode.TreeItem(
        node.patch.title || shortId(node.patch.id),
        vscode.TreeItemCollapsibleState.Collapsed,
      );
      item.description = `${node.patch.status} · ${node.patch.changes.length} file${node.patch.changes.length === 1 ? '' : 's'}`;
      item.tooltip = node.patch.summary ?? '';
      item.iconPath = new vscode.ThemeIcon(statusIcon(node.patch.status), statusColor(node.patch.status));
      item.contextValue = `ironflyer.patch.${node.patch.status}`;
      return item;
    }
    const item = new vscode.TreeItem(node.change.path);
    item.description = node.change.op;
    item.iconPath = new vscode.ThemeIcon(opIcon(node.change.op));
    item.contextValue = 'ironflyer.patchChange';
    item.command = {
      command: 'ironflyer.showPatchDiff',
      title: 'Show diff',
      arguments: [{ projectId: node.projectId, patchId: node.patch.id, path: node.change.path, op: node.change.op }],
    };
    return item;
  }

  async getChildren(parent?: Node): Promise<Node[]> {
    if (!parent) {
      const token = await this.auth.getToken();
      if (!token) return [];
      try {
        const projects = await this.api.listProjects();
        // Filter to projects that actually have patches. Doing this lazily
        // would feel snappier but the user with 1 project + 0 patches would
        // see a stale "expand-to-find-nothing" — better to be honest up front.
        const withPatches: Node[] = [];
        for (const p of projects) {
          const list = await this.diff.primePatchesFor(p.id);
          if (list.length > 0) withPatches.push({ kind: 'project', project: p });
        }
        return withPatches;
      } catch (err) {
        void vscode.window.showErrorMessage(`Ironflyer: failed to load patches — ${(err as Error).message}`);
        return [];
      }
    }
    if (parent.kind === 'project') {
      const list = await this.diff.primePatchesFor(parent.project.id);
      return list.map((p) => ({ kind: 'patch' as const, projectId: parent.project.id, patch: p }));
    }
    if (parent.kind === 'patch') {
      return parent.patch.changes.map((c) => ({
        kind: 'change' as const,
        projectId: parent.projectId,
        patch: parent.patch,
        change: c,
      }));
    }
    return [];
  }
}

function statusIcon(s: string): string {
  switch (s) {
    case 'applied': return 'check';
    case 'validated': return 'pass';
    case 'proposed': return 'circle-outline';
    case 'rejected': return 'error';
    case 'rolled-back': return 'history';
    default: return 'circle-outline';
  }
}

function statusColor(s: string): vscode.ThemeColor | undefined {
  switch (s) {
    case 'applied': return new vscode.ThemeColor('charts.green');
    case 'rejected': return new vscode.ThemeColor('charts.red');
    case 'rolled-back': return new vscode.ThemeColor('charts.orange');
    default: return undefined;
  }
}

function opIcon(op: string): string {
  switch (op) {
    case 'create': return 'diff-added';
    case 'update': return 'diff-modified';
    case 'delete': return 'diff-removed';
    default: return 'file';
  }
}
