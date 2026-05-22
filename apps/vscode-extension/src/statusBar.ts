// Status bar — three items, left-to-right:
//
//   1. Project + last-gate badge   ($(rocket) myproj · pass)
//   2. Budget                      ($20 · $4.12/40)
//   3. Run quick action            ($(play) Run)
//
// The middle item is the legacy budget pill (clicks → showBudget). The
// project pill clicks → quickActions (a single command palette listing
// every Ironflyer surface). The run pill clicks → runFinisher and is only
// visible once a project is pinned. Everything polls on a low cadence and
// reacts immediately to auth + activeProject changes.

import * as vscode from 'vscode';
import { Api, GateState } from './api';
import { Auth } from './auth';
import { ActiveProject } from './activeProject';
import { gateColorId, summarizeGates } from './gateIcons';

const POLL_MS = 60_000;
const GATE_POLL_MS = 20_000;

export class StatusBar implements vscode.Disposable {
  private readonly projectItem: vscode.StatusBarItem;
  private readonly budgetItem: vscode.StatusBarItem;
  private readonly runItem: vscode.StatusBarItem;

  private budgetTimer: NodeJS.Timeout | undefined;
  private gateTimer: NodeJS.Timeout | undefined;
  private readonly disposables: vscode.Disposable[] = [];

  constructor(
    private readonly api: Api,
    private readonly auth: Auth,
    private readonly activeProject: ActiveProject,
  ) {
    // Right-aligned, in priority order so the project pill stays leftmost.
    this.projectItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Right, 102);
    this.budgetItem  = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Right, 101);
    this.runItem     = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Right, 100);

    this.projectItem.command = 'ironflyer.quickActions';
    this.budgetItem.command  = 'ironflyer.showBudget';
    this.runItem.command     = 'ironflyer.runFinisher';
    this.runItem.text        = '$(play) Run';
    this.runItem.tooltip     = 'Ironflyer · run Finisher on the pinned project';

    this.disposables.push(
      auth.onDidChange(() => this.refreshAll()),
      activeProject.onDidChange(() => this.refreshGates()),
    );

    this.refreshAll();
    this.budgetTimer = setInterval(() => this.refreshBudget(), POLL_MS);
    this.gateTimer   = setInterval(() => this.refreshGates(),  GATE_POLL_MS);
  }

  private refreshAll(): void {
    void this.refreshBudget();
    void this.refreshGates();
  }

  private async refreshBudget(): Promise<void> {
    const token = await this.auth.getToken();
    if (!token) {
      this.budgetItem.text = '$(rocket) Ironflyer';
      this.budgetItem.tooltip = 'Sign in to Ironflyer';
      this.budgetItem.command = 'ironflyer.signIn';
      this.budgetItem.show();
      return;
    }
    try {
      const b = await this.api.myBudget();
      this.budgetItem.text = `$(credit-card) ${b.tier} · $${Number(b.monthSpend).toFixed(2)}/${Number(b.monthCap).toFixed(0)}`;
      this.budgetItem.tooltip = `Ironflyer · plan ${b.tier} · spent $${b.monthSpend} of $${b.monthCap}${b.hardStop ? ' (hard stop)' : ''}`;
      this.budgetItem.command = 'ironflyer.showBudget';
    } catch {
      this.budgetItem.text = '$(credit-card) —';
      this.budgetItem.tooltip = 'Could not read budget — click to retry';
      this.budgetItem.command = 'ironflyer.showBudget';
    }
    this.budgetItem.show();
  }

  private async refreshGates(): Promise<void> {
    const token = await this.auth.getToken();
    if (!token) {
      this.projectItem.hide();
      this.runItem.hide();
      return;
    }
    const ref = this.activeProject.get();
    if (!ref) {
      this.projectItem.text = '$(rocket) Pin project';
      this.projectItem.tooltip = 'No active Ironflyer project — click to pick one';
      this.projectItem.command = 'ironflyer.setActiveProject';
      this.projectItem.backgroundColor = undefined;
      this.projectItem.show();
      this.runItem.hide();
      return;
    }
    this.runItem.show();
    this.projectItem.text = `$(rocket) ${ref.name}`;
    this.projectItem.tooltip = `Ironflyer · ${ref.name} — click for actions`;
    try {
      const gates = await this.api.listGates(ref.id);
      const badge = renderGateBadge(gates);
      this.projectItem.text = `$(rocket) ${ref.name} · ${badge.text}`;
      this.projectItem.tooltip = new vscode.MarkdownString(
        `**${ref.name}**\n\n${summarizeGates(gates)}\n\n_Click for quick actions._`,
      );
      this.projectItem.backgroundColor = badge.background;
    } catch {
      // Auth + transport errors are absorbed; the pill still works as a
      // command-launcher even when /gates is down.
      this.projectItem.backgroundColor = undefined;
    }
    this.projectItem.show();
  }

  dispose(): void {
    if (this.budgetTimer) clearInterval(this.budgetTimer);
    if (this.gateTimer)   clearInterval(this.gateTimer);
    while (this.disposables.length) this.disposables.pop()?.dispose();
    this.projectItem.dispose();
    this.budgetItem.dispose();
    this.runItem.dispose();
  }
}

function renderGateBadge(gates: GateState[]): { text: string; background?: vscode.ThemeColor } {
  if (gates.length === 0) return { text: 'no gates' };
  const counts: Record<string, number> = {};
  for (const g of gates) counts[g.status] = (counts[g.status] ?? 0) + 1;
  if (counts.failed) {
    return {
      text: `$(error) ${counts.failed} failed`,
      background: new vscode.ThemeColor('statusBarItem.errorBackground'),
    };
  }
  if (counts.running) {
    return { text: `$(loading~spin) ${counts.running} running` };
  }
  if (counts.blocked) {
    return {
      text: `$(circle-slash) ${counts.blocked} blocked`,
      background: new vscode.ThemeColor('statusBarItem.warningBackground'),
    };
  }
  const passed = counts.passed ?? 0;
  if (passed === gates.length) return { text: `$(pass-filled) all passed` };
  // Use the colour of the dominant status as a hint.
  const dominant = Object.entries(counts).sort((a, b) => b[1] - a[1])[0]?.[0] as
    | 'passed' | 'failed' | 'running' | 'blocked' | 'pending' | 'repaired' | undefined;
  if (dominant) {
    const tone = gateColorId(dominant);
    if (tone) {
      // We don't have a way to colour a status-bar foreground reliably, so
      // we just emit a short summary — colour is reserved for failed/blocked.
    }
  }
  return { text: `${passed}/${gates.length} passed` };
}
