// Output-channel logger. Replaces ad-hoc console.log/warn calls so users
// can read what the extension is doing from View → Output → "Ironflyer".
// The channel is created lazily on first write so we don't pollute the
// Output dropdown for users who never hit an error path.

import * as vscode from 'vscode';

let channel: vscode.OutputChannel | undefined;

function ensure(): vscode.OutputChannel {
  if (!channel) channel = vscode.window.createOutputChannel('Ironflyer');
  return channel;
}

function timestamp(): string {
  const d = new Date();
  return d.toISOString().replace('T', ' ').slice(0, 19);
}

function format(level: string, msg: string, err?: unknown): string {
  let out = `[${timestamp()}] ${level} ${msg}`;
  if (err !== undefined) {
    if (err instanceof Error) {
      out += ` :: ${err.name}: ${err.message}`;
      if (err.stack) out += `\n${err.stack}`;
    } else {
      try { out += ` :: ${JSON.stringify(err)}`; }
      catch { out += ` :: ${String(err)}`; }
    }
  }
  return out;
}

export const log = {
  info(msg: string, err?: unknown): void { ensure().appendLine(format('INFO ', msg, err)); },
  warn(msg: string, err?: unknown): void { ensure().appendLine(format('WARN ', msg, err)); },
  error(msg: string, err?: unknown): void { ensure().appendLine(format('ERROR', msg, err)); },
  show(): void { ensure().show(); },
  dispose(): void { channel?.dispose(); channel = undefined; },
};
