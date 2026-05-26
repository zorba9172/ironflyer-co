// Serves the read-only "left" (current project file) and "right" (proposed
// patch content) sides of the diff editor over the ironflyer:// scheme.
//
// Caching strategy: project files are fetched in bulk on first request per
// project; patches are fetched per project too. Both caches are short-lived
// — `invalidate()` is called when patches refresh or apply.

import * as vscode from 'vscode';
import { Api, Patch, ProjectFile } from './api';
import { parsePatchUri } from './patchUri';

export class PatchDiffProvider implements vscode.TextDocumentContentProvider {
  private readonly _onDidChange = new vscode.EventEmitter<vscode.Uri>();
  readonly onDidChange = this._onDidChange.event;

  private readonly fileCache = new Map<string, Promise<ProjectFile[]>>();
  private readonly patchCache = new Map<string, Promise<Patch[]>>();

  constructor(private readonly api: Api) {}

  async provideTextDocumentContent(uri: vscode.Uri): Promise<string> {
    const parsed = parsePatchUri(uri.toString());
    if (!parsed) return '';
    if (parsed.side === 'current') {
      const files = await this.filesFor(parsed.id);
      const f = files.find((x) => x.path === parsed.path);
      return f?.content ?? '';
    }
    // proposed: parsed.id is patchId; we need to find the patch in some
    // project's list. Since the tree always opens the diff with the
    // project's id known, we cache patches keyed by projectId once they
    // are loaded — and fall back to scanning all loaded caches.
    for (const promise of this.patchCache.values()) {
      const list = await promise;
      const p = list.find((x) => x.id === parsed.id);
      if (!p) continue;
      const ch = p.changes.find((c) => c.path === parsed.path);
      return ch?.content ?? '';
    }
    return '';
  }

  primePatchesFor(projectId: string): Promise<Patch[]> {
    let p = this.patchCache.get(projectId);
    if (!p) {
      p = this.api.listPatches(projectId).catch((e) => {
        this.patchCache.delete(projectId);
        throw e;
      });
      this.patchCache.set(projectId, p);
    }
    return p;
  }

  private filesFor(projectId: string): Promise<ProjectFile[]> {
    let p = this.fileCache.get(projectId);
    if (!p) {
      p = this.api.listFiles(projectId).catch((e) => {
        this.fileCache.delete(projectId);
        throw e;
      });
      this.fileCache.set(projectId, p);
    }
    return p;
  }

  invalidate(projectId?: string): void {
    if (projectId) {
      this.fileCache.delete(projectId);
      this.patchCache.delete(projectId);
    } else {
      this.fileCache.clear();
      this.patchCache.clear();
    }
  }
}
