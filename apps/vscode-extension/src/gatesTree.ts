// Gates sidebar TreeView. Three levels:
//
//   Project
//     └── Gate (status icon + age in description)
//           └── Issue (severity icon + message)
//
// Patches Tree filters to projects-with-patches; gates are universal —
// every project always has 8 gates, even if all "pending". So we show
// every project unconditionally and let the gate node's description
// summarize state ("5 / 8 passed").

import * as vscode from 'vscode';
import { Api, GateState, Issue, Project } from './api';
import { Auth } from './auth';
import { gateColorId, gateIcon, issueColorId, issueIcon, summarizeGates } from './gateIcons';

type Node =
  | { kind: 'project'; project: Project; gates: GateState[] }
  | { kind: 'gate'; projectId: string; gate: GateState }
  | { kind: 'issue'; gate: GateState; issue: Issue };

export class GatesTree implements vscode.TreeDataProvider<Node> {
  private readonly _onDidChange = new vscode.EventEmitter<Node | undefined>();
  readonly onDidChangeTreeData = this._onDidChange.event;

  private cache = new Map<string, Promise<GateState[]>>();

  constructor(private readonly api: Api, private readonly auth: Auth) {
    auth.onDidChange(() => this.refresh());
  }

  refresh(node?: Node): void {
    this.cache.clear();
    this._onDidChange.fire(node);
  }

  getTreeItem(node: Node): vscode.TreeItem {
    if (node.kind === 'project') {
      const item = new vscode.TreeItem(
        node.project.name,
        vscode.TreeItemCollapsibleState.Collapsed,
      );
      item.description = summarizeGates(node.gates);
      item.iconPath = new vscode.ThemeIcon('rocket');
      item.contextValue = 'ironflyer.gateProject';
      item.command = {
        command: 'ironflyer.runFinisher',
        title: 'Run Finisher',
        arguments: [node.project.id],
      };
      return item;
    }
    if (node.kind === 'gate') {
      const issueCount = node.gate.issues?.length ?? 0;
      const collapsible = issueCount > 0
        ? vscode.TreeItemCollapsibleState.Collapsed
        : vscode.TreeItemCollapsibleState.None;
      const item = new vscode.TreeItem(node.gate.name, collapsible);
      const parts: string[] = [node.gate.status];
      if (issueCount > 0) parts.push(`${issueCount} issue${issueCount === 1 ? '' : 's'}`);
      item.description = parts.join(' · ');
      const color = gateColorId(node.gate.status);
      item.iconPath = new vscode.ThemeIcon(
        gateIcon(node.gate.status),
        color ? new vscode.ThemeColor(color) : undefined,
      );
      item.tooltip = `${node.gate.name} — last updated ${node.gate.updatedAt}`;
      item.contextValue = `ironflyer.gate.${node.gate.status}`;
      return item;
    }
    const item = new vscode.TreeItem(node.issue.message);
    item.description = node.issue.path ?? '';
    item.tooltip = node.issue.hint ?? node.issue.message;
    const color = issueColorId(node.issue.severity);
    item.iconPath = new vscode.ThemeIcon(
      issueIcon(node.issue.severity),
      color ? new vscode.ThemeColor(color) : undefined,
    );
    item.contextValue = 'ironflyer.gateIssue';
    return item;
  }

  async getChildren(parent?: Node): Promise<Node[]> {
    if (!parent) {
      const token = await this.auth.getToken();
      if (!token) return [];
      try {
        const projects = await this.api.listProjects();
        const out: Node[] = [];
        for (const p of projects) {
          const gates = await this.gatesFor(p.id);
          out.push({ kind: 'project', project: p, gates });
        }
        return out;
      } catch (err) {
        void vscode.window.showErrorMessage(`Ironflyer: failed to load gates — ${(err as Error).message}`);
        return [];
      }
    }
    if (parent.kind === 'project') {
      return parent.gates.map((g) => ({ kind: 'gate' as const, projectId: parent.project.id, gate: g }));
    }
    if (parent.kind === 'gate') {
      return (parent.gate.issues ?? []).map((i) => ({ kind: 'issue' as const, gate: parent.gate, issue: i }));
    }
    return [];
  }

  private gatesFor(projectId: string): Promise<GateState[]> {
    let p = this.cache.get(projectId);
    if (!p) {
      p = this.api.listGates(projectId).catch((e) => {
        this.cache.delete(projectId);
        throw e;
      });
      this.cache.set(projectId, p);
    }
    return p;
  }
}
