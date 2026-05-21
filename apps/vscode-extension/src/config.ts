// Wraps VSCode workspace configuration so the rest of the extension can
// stay decoupled from the contributes.configuration schema.
import * as vscode from 'vscode';

export interface Config {
  orchestratorUrl: string;
  webUrl: string;
}

export function readConfig(): Config {
  const cfg = vscode.workspace.getConfiguration('ironflyer');
  return {
    orchestratorUrl: trimSlash(cfg.get<string>('orchestratorUrl', 'http://localhost:8080')),
    webUrl: trimSlash(cfg.get<string>('webUrl', 'http://localhost:3000')),
  };
}

function trimSlash(s: string): string {
  return s.replace(/\/+$/, '');
}
