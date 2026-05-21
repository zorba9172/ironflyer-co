// Status bar item showing the user's plan + month-to-date spend. Polls
// /budget/users/me on a low cadence so it stays fresh without thrashing
// the orchestrator.

import * as vscode from 'vscode';
import { Api } from './api';
import { Auth } from './auth';

const POLL_MS = 60_000;

export class StatusBar implements vscode.Disposable {
  private readonly item: vscode.StatusBarItem;
  private timer: NodeJS.Timeout | undefined;

  constructor(private readonly api: Api, private readonly auth: Auth) {
    this.item = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Right, 100);
    this.item.command = 'ironflyer.showBudget';
    auth.onDidChange(() => this.refresh());
    this.refresh();
    this.timer = setInterval(() => this.refresh(), POLL_MS);
  }

  async refresh(): Promise<void> {
    const token = await this.auth.getToken();
    if (!token) {
      this.item.text = '$(rocket) Ironflyer';
      this.item.tooltip = 'Sign in to Ironflyer';
      this.item.command = 'ironflyer.signIn';
      this.item.show();
      return;
    }
    try {
      const b = await this.api.myBudget();
      this.item.text = `$(rocket) ${b.tier} · $${Number(b.monthSpend).toFixed(2)}/${Number(b.monthCap).toFixed(0)}`;
      this.item.tooltip = `Ironflyer · plan ${b.tier} · spent $${b.monthSpend} of $${b.monthCap}${b.hardStop ? ' (hard stop)' : ''}`;
      this.item.command = 'ironflyer.showBudget';
    } catch {
      this.item.text = '$(rocket) Ironflyer';
      this.item.tooltip = 'Could not read budget — click to retry';
      this.item.command = 'ironflyer.showBudget';
    }
    this.item.show();
  }

  dispose(): void {
    if (this.timer) clearInterval(this.timer);
    this.item.dispose();
  }
}
