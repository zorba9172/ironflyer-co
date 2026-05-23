// Telemetry sidebar TreeView. Flat list of the last 30 agent calls,
// newest first. Each row surfaces provider/model, cost and duration —
// the dials operators need to spot expensive or slow agents at a glance.

import * as vscode from 'vscode';
import { AgentCall, Api } from './api';
import { Auth } from './auth';

type Node = { kind: 'call'; call: AgentCall };

export class IronflyerTelemetryProvider implements vscode.TreeDataProvider<Node> {
  private readonly _onDidChange = new vscode.EventEmitter<Node | undefined>();
  readonly onDidChangeTreeData = this._onDidChange.event;

  private calls: AgentCall[] = [];
  private loading?: Promise<void>;

  constructor(private readonly api: Api, private readonly auth: Auth) {
    auth.onDidChange(() => this.refresh());
    void this.reload();
  }

  refresh(): void {
    void this.reload();
  }

  private async reload(): Promise<void> {
    if (this.loading) return this.loading;
    this.loading = (async () => {
      const token = await this.auth.getToken();
      if (!token) {
        this.calls = [];
        this._onDidChange.fire(undefined);
        return;
      }
      try {
        this.calls = await this.api.listAgentTelemetry(30);
      } catch {
        this.calls = [];
      }
      this._onDidChange.fire(undefined);
    })().finally(() => {
      this.loading = undefined;
    });
    return this.loading;
  }

  getTreeItem(node: Node): vscode.TreeItem {
    const c = node.call;
    const item = new vscode.TreeItem(`${c.provider}/${c.model}`);
    item.description = `${formatCost(c.costUSD)} · ${c.durationMs}ms`;
    item.tooltip = new vscode.MarkdownString('```json\n' + JSON.stringify(c, null, 2) + '\n```');
    item.iconPath = new vscode.ThemeIcon(
      c.error ? 'error' : 'pulse',
      c.error ? new vscode.ThemeColor('charts.red') : undefined,
    );
    item.contextValue = 'ironflyer.agentCall';
    return item;
  }

  async getChildren(parent?: Node): Promise<Node[]> {
    if (!parent) {
      const token = await this.auth.getToken();
      if (!token) return [];
      return this.calls.map((c) => ({ kind: 'call' as const, call: c }));
    }
    return [];
  }
}

function formatCost(n: number): string {
  if (!isFinite(n)) return '$?';
  if (n === 0) return '$0';
  if (n < 0.01) return `$${n.toFixed(4)}`;
  if (n < 1) return `$${n.toFixed(3)}`;
  return `$${n.toFixed(2)}`;
}
