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
  completionsEnabled: boolean;
  completionsDebounceMs: number;
  completionsMaxLines: number;
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
    completionsEnabled: cfg.get<boolean>('completions.enabled', true),
    // Debounce floor at 50ms — anything lower trashes the keystroke
    // loop without giving the orchestrator a chance to deduplicate.
    completionsDebounceMs: Math.max(50, cfg.get<number>('completions.debounceMs', 250)),
    // Clamp the line cap so a misconfiguration can't request a 9k-token
    // multi-page completion.
    completionsMaxLines: Math.max(1, Math.min(40, cfg.get<number>('completions.maxLines', 6))),
  };
}

function trimSlash(s: string): string {
  return (s ?? '').replace(/\/+$/, '');
}
