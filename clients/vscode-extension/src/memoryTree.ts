// Memory sidebar TreeView. Two levels:
//
//   Kind (project / execution / user / business)
//     └── Record (title · tag list · body in tooltip + markdown preview)
//
// The memory engine is the compounding-asset layer of the AI Completion
// Infrastructure blueprint — every kind has its own bucket and we hit
// /memory once per kind, scoped to the pinned project where applicable.
// User memory is intentionally project-agnostic.

import * as vscode from 'vscode';
import { Api, MemoryKind, MemoryRecord } from './api';
import { Auth } from './auth';
import { ActiveProject } from './activeProject';

type Node =
  | { kind: 'group'; memoryKind: MemoryKind; records: MemoryRecord[] }
  | { kind: 'record'; record: MemoryRecord };

const KINDS: { kind: MemoryKind; label: string; icon: string }[] = [
  { kind: 'project',   label: 'Project',   icon: 'rocket' },
  { kind: 'execution', label: 'Execution', icon: 'play-circle' },
  { kind: 'user',      label: 'User',      icon: 'person' },
  { kind: 'business',  label: 'Business',  icon: 'briefcase' },
];

export class IronflyerMemoryProvider implements vscode.TreeDataProvider<Node> {
  private readonly _onDidChange = new vscode.EventEmitter<Node | undefined>();
  readonly onDidChangeTreeData = this._onDidChange.event;

  private cache = new Map<MemoryKind, Promise<MemoryRecord[]>>();

  constructor(
    private readonly api: Api,
    private readonly auth: Auth,
    private readonly active: ActiveProject,
  ) {
    auth.onDidChange(() => this.refresh());
    active.onDidChange(() => this.refresh());
  }

  refresh(): void {
    this.cache.clear();
    this._onDidChange.fire(undefined);
  }

  getTreeItem(node: Node): vscode.TreeItem {
    if (node.kind === 'group') {
      const meta = KINDS.find((k) => k.kind === node.memoryKind)!;
      const item = new vscode.TreeItem(meta.label, vscode.TreeItemCollapsibleState.Collapsed);
      item.description = `${node.records.length} record${node.records.length === 1 ? '' : 's'}`;
      item.iconPath = new vscode.ThemeIcon(meta.icon);
      item.contextValue = `ironflyer.memoryGroup.${node.memoryKind}`;
      return item;
    }
    const r = node.record;
    const item = new vscode.TreeItem(r.title || '(untitled)');
    item.description = (r.tags ?? []).join(', ');
    item.tooltip = new vscode.MarkdownString(
      `**${r.title || '(untitled)'}**\n\n` +
      (r.body ? `${r.body}\n\n` : '') +
      `_${r.kind}${r.projectId ? ` · ${r.projectId}` : ''}${r.createdAt ? ` · ${r.createdAt}` : ''}_`,
    );
    item.iconPath = new vscode.ThemeIcon('note');
    item.contextValue = 'ironflyer.memoryRecord';
    item.command = {
      command: 'ironflyer.openMemoryRecord',
      title: 'Open memory record',
      arguments: [r],
    };
    return item;
  }

  async getChildren(parent?: Node): Promise<Node[]> {
    if (!parent) {
      const token = await this.auth.getToken();
      if (!token) return [];
      const projectId = this.active.get()?.id;
      const groups = await Promise.all(
        KINDS.map(async (k) => {
          const records = await this.recordsFor(k.kind, projectId);
          return { kind: 'group' as const, memoryKind: k.kind, records };
        }),
      );
      return groups;
    }
    if (parent.kind === 'group') {
      return parent.records.map((r) => ({ kind: 'record' as const, record: r }));
    }
    return [];
  }

  private recordsFor(kind: MemoryKind, projectId?: string): Promise<MemoryRecord[]> {
    let p = this.cache.get(kind);
    if (!p) {
      // User memory is project-agnostic; everything else is scoped by the
      // pinned project. Without a pinned project, project / execution /
      // business reads degrade to "kind only" (orchestrator may return 400
      // — swallow it so the tree stays usable).
      const opts: { kind: MemoryKind; projectId?: string; limit?: number } = { kind, limit: 50 };
      if (projectId && kind !== 'user') opts.projectId = projectId;
      p = this.api.listMemory(opts).catch(() => {
        this.cache.delete(kind);
        return [] as MemoryRecord[];
      });
      this.cache.set(kind, p);
    }
    return p;
  }
}

/**
 * Opens a memory record in a temporary markdown preview. We use a virtual
 * untitled document so the record stays read-only — memory writes go
 * through the API, not file edits.
 */
export async function openMemoryRecord(record: MemoryRecord): Promise<void> {
  const md = [
    `# ${record.title || '(untitled)'}`,
    '',
    `**kind:** \`${record.kind}\``,
    record.projectId ? `**project:** \`${record.projectId}\`` : '',
    record.userId ? `**user:** \`${record.userId}\`` : '',
    record.gateName ? `**gate:** \`${record.gateName}\`` : '',
    record.tags && record.tags.length > 0 ? `**tags:** ${record.tags.map((t) => `\`${t}\``).join(' ')}` : '',
    record.confidence !== undefined ? `**confidence:** ${record.confidence}` : '',
    `**createdAt:** ${record.createdAt}`,
    '',
    '---',
    '',
    record.body || '_(empty)_',
  ].filter(Boolean).join('\n');
  const doc = await vscode.workspace.openTextDocument({ content: md, language: 'markdown' });
  await vscode.commands.executeCommand('markdown.showPreview', doc.uri);
}
