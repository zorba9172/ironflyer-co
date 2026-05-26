// Audit log sidebar TreeView. Flat list, newest first, capped at 50 rows.
//
// The first row is a synthetic header that surfaces the hash-chain
// verdict — "chain intact" or "chain broken at #N" — fetched from
// /audit/verify on activate + on every refresh(). Enterprise compliance
// is the differentiator here, so chain breakage must be visible without
// digging.

import * as vscode from 'vscode';
import { Api, AuditEntry } from './api';
import { Auth } from './auth';
import { ActiveProject } from './activeProject';

type ChainStatus =
  | { state: 'unknown' }
  | { state: 'intact' }
  | { state: 'broken'; brokenAt: number }
  | { state: 'error'; message: string };

type Node =
  | { kind: 'chainStatus'; status: ChainStatus }
  | { kind: 'entry'; entry: AuditEntry };

export class IronflyerAuditProvider implements vscode.TreeDataProvider<Node> {
  private readonly _onDidChange = new vscode.EventEmitter<Node | undefined>();
  readonly onDidChangeTreeData = this._onDidChange.event;

  private entries: AuditEntry[] = [];
  private chain: ChainStatus = { state: 'unknown' };
  private loading?: Promise<void>;

  constructor(
    private readonly api: Api,
    private readonly auth: Auth,
    private readonly active: ActiveProject,
  ) {
    auth.onDidChange(() => this.refresh());
    active.onDidChange(() => this.refresh());
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
        this.entries = [];
        this.chain = { state: 'unknown' };
        this._onDidChange.fire(undefined);
        return;
      }
      const projectId = this.active.get()?.id;
      try {
        const [entries, verify] = await Promise.all([
          this.api.listAudit({ projectId, limit: 50 }),
          this.api.verifyAudit().catch((err) => ({ ok: false, brokenAt: undefined, _err: err } as any)),
        ]);
        this.entries = entries;
        if ((verify as any)._err) {
          this.chain = { state: 'error', message: ((verify as any)._err as Error).message };
        } else if (verify.ok) {
          this.chain = { state: 'intact' };
        } else {
          this.chain = { state: 'broken', brokenAt: verify.brokenAt ?? -1 };
        }
      } catch (err) {
        this.entries = [];
        this.chain = { state: 'error', message: (err as Error).message };
      }
      this._onDidChange.fire(undefined);
    })().finally(() => {
      this.loading = undefined;
    });
    return this.loading;
  }

  getTreeItem(node: Node): vscode.TreeItem {
    if (node.kind === 'chainStatus') {
      const { label, icon, color, tooltip } = describeChain(node.status);
      const item = new vscode.TreeItem(label);
      item.iconPath = new vscode.ThemeIcon(icon, color ? new vscode.ThemeColor(color) : undefined);
      item.tooltip = tooltip;
      item.contextValue = 'ironflyer.auditChainStatus';
      return item;
    }
    const e = node.entry;
    const item = new vscode.TreeItem(`${e.action} · ${e.outcome}`);
    item.description = humanTime(e.createdAt);
    const tooltip = new vscode.MarkdownString(
      `**${e.action}** · _${e.outcome}_\n\n` +
      (e.summary ? `${e.summary}\n\n` : '') +
      `**createdAt:** ${e.createdAt}\n\n` +
      (e.projectId ? `**project:** \`${e.projectId}\`\n\n` : '') +
      (e.agentRole ? `**agent:** \`${e.agentRole}\`\n\n` : '') +
      `**hash:** \`${e.contentHash}\``,
    );
    item.tooltip = tooltip;
    item.iconPath = new vscode.ThemeIcon(outcomeIcon(e.outcome), outcomeColor(e.outcome));
    item.contextValue = 'ironflyer.auditEntry';
    return item;
  }

  async getChildren(parent?: Node): Promise<Node[]> {
    if (!parent) {
      const token = await this.auth.getToken();
      if (!token) return [];
      return [
        { kind: 'chainStatus', status: this.chain },
        ...this.entries.map((e) => ({ kind: 'entry' as const, entry: e })),
      ];
    }
    return [];
  }
}

function describeChain(status: ChainStatus): {
  label: string;
  icon: string;
  color?: string;
  tooltip: string;
} {
  switch (status.state) {
    case 'intact':
      return {
        label: '✓ chain intact',
        icon: 'pass-filled',
        color: 'charts.green',
        tooltip: 'Audit hash chain verified end-to-end.',
      };
    case 'broken':
      return {
        label: `✗ chain broken at #${status.brokenAt}`,
        icon: 'error',
        color: 'charts.red',
        tooltip: `Audit hash chain breaks at index ${status.brokenAt}. Treat the log as tampered.`,
      };
    case 'error':
      return {
        label: '? chain unknown',
        icon: 'warning',
        color: 'charts.orange',
        tooltip: `Could not verify audit chain: ${status.message}`,
      };
    case 'unknown':
    default:
      return {
        label: '… verifying chain',
        icon: 'sync',
        tooltip: 'Verifying audit hash chain…',
      };
  }
}

function outcomeIcon(outcome: string): string {
  switch (outcome) {
    case 'success': return 'check';
    case 'failure': return 'error';
    case 'blocked': return 'circle-slash';
    default: return 'circle-outline';
  }
}

function outcomeColor(outcome: string): vscode.ThemeColor | undefined {
  switch (outcome) {
    case 'success': return new vscode.ThemeColor('charts.green');
    case 'failure': return new vscode.ThemeColor('charts.red');
    case 'blocked': return new vscode.ThemeColor('charts.orange');
    default: return undefined;
  }
}

function humanTime(iso: string | undefined): string {
  if (!iso) return '';
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return '';
  const diff = Date.now() - t;
  if (diff < 0) return '';
  const s = Math.floor(diff / 1000);
  if (s < 60) return `${s}s ago`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  const d = Math.floor(h / 24);
  return `${d}d ago`;
}
