// Wraps VSCode workspace configuration so the rest of the extension can
// stay decoupled from the contributes.configuration schema.
import * as vscode from 'vscode';

export type ViewportPreset = 'responsive' | 'mobile' | 'tablet' | 'desktop';

export interface Config {
  orchestratorUrl: string;
  runtimeUrl: string;
  webUrl: string;
  defaultProject: string;
  previewViewport: ViewportPreset;
}

export function readConfig(): Config {
  const cfg = vscode.workspace.getConfiguration('ironflyer');
  // `ironflyer.apiUrl` is the friendlier alias documented in the package
  // manifest; if set it wins so power users can change one knob without
  // touching the legacy `orchestratorUrl` key.
  const apiUrl = trimSlash(cfg.get<string>('apiUrl', ''));
  const orchestratorUrl = trimSlash(cfg.get<string>('orchestratorUrl', 'http://localhost:8080'));
  return {
    orchestratorUrl: apiUrl || orchestratorUrl,
    runtimeUrl: trimSlash(cfg.get<string>('runtimeUrl', 'http://localhost:8090')),
    webUrl: trimSlash(cfg.get<string>('webUrl', 'http://localhost:3000')),
    defaultProject: (cfg.get<string>('defaultProject', '') ?? '').trim(),
    previewViewport: (cfg.get<ViewportPreset>('preview.defaultViewport', 'responsive') ?? 'responsive'),
  };
}

function trimSlash(s: string): string {
  return (s ?? '').replace(/\/+$/, '');
}
