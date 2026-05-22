// Dedicated "Ironflyer Run" output channel.
//
// The Finisher emits a fast-paced stream of events (gate_started,
// gate_passed, gate_failed, patch_proposed, run_complete, etc.). We tail
// the project's /stream into this channel so the user has a permanent
// transcript even after toasts disappear. Toast notifications are emitted
// only for the few events that need a human decision.

import * as vscode from 'vscode';
import { SSEEvent } from './api';

export interface RunEventOpts {
  onPatchProposed?: (patchId: string, projectId: string) => void;
}

export class RunOutput implements vscode.Disposable {
  private channel: vscode.OutputChannel | undefined;
  private readonly seenPatches = new Set<string>();

  private ensure(): vscode.OutputChannel {
    if (!this.channel) this.channel = vscode.window.createOutputChannel('Ironflyer Run');
    return this.channel;
  }

  show(): void { this.ensure().show(true); }

  /**
   * Surface a lifecycle event in the Run channel + optional toasts.
   * `projectName` is only used for the toast text — the channel always
   * prefixes lines with the projectId for unambiguous grep.
   */
  handle(projectId: string, projectName: string, evt: SSEEvent, opts: RunEventOpts = {}): void {
    const ch = this.ensure();
    const data = safeStringify(evt.data);
    ch.appendLine(`[${timestamp()}] ${projectId} ${evt.event} ${data}`);

    const name = projectName || projectId;
    const payload = evt.data as Record<string, unknown> | undefined;

    switch (evt.event) {
      case 'gate_started':
        // Quiet — too noisy for a toast.
        return;
      case 'gate_passed':
        return;
      case 'gate_failed': {
        const gate = (payload?.gate as string) ?? 'a gate';
        const reason = (payload?.reason as string) ?? 'see Run output for details';
        void vscode.window
          .showWarningMessage(
            `Ironflyer · ${name} · ${gate} failed: ${reason}`,
            'View Run Output',
            'Open Gates',
          )
          .then((pick) => {
            if (pick === 'View Run Output') ch.show(true);
            if (pick === 'Open Gates') {
              void vscode.commands.executeCommand('workbench.view.extension.ironflyer');
            }
          });
        return;
      }
      case 'run_complete': {
        const status = (payload?.status as string) ?? 'complete';
        const msg = `Ironflyer · ${name} · run ${status}.`;
        if (status === 'success' || status === 'passed') {
          void vscode.window.showInformationMessage(msg);
        } else {
          void vscode.window.showWarningMessage(msg, 'View Run Output').then((p) => {
            if (p === 'View Run Output') ch.show(true);
          });
        }
        return;
      }
      case 'patch_proposed': {
        const patchId = String(payload?.patchId ?? payload?.id ?? '');
        if (!patchId || this.seenPatches.has(patchId)) return;
        this.seenPatches.add(patchId);
        opts.onPatchProposed?.(patchId, projectId);
        return;
      }
    }
  }

  dispose(): void {
    this.channel?.dispose();
    this.channel = undefined;
    this.seenPatches.clear();
  }
}

function timestamp(): string {
  return new Date().toISOString().replace('T', ' ').slice(0, 19);
}

function safeStringify(d: unknown): string {
  if (d === undefined || d === null) return '';
  try { return JSON.stringify(d); } catch { return String(d); }
}
