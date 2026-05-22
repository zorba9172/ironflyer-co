// Surfaces "Ask Ironflyer to fix" as a CodeAction on any diagnostic in
// the editor. Clicking it routes to `ironflyer.fixDiagnostic`, which
// formats a prompt with the diagnostic + code snippet and sends it to
// the active project's chat.

import * as vscode from 'vscode';

export class IronflyerCodeActions implements vscode.CodeActionProvider {
  static readonly metadata: vscode.CodeActionProviderMetadata = {
    providedCodeActionKinds: [vscode.CodeActionKind.QuickFix],
  };

  provideCodeActions(
    document: vscode.TextDocument,
    _range: vscode.Range,
    ctx: vscode.CodeActionContext,
  ): vscode.CodeAction[] {
    const diagnostics = ctx.diagnostics;
    if (diagnostics.length === 0) return [];
    return diagnostics.map((d) => {
      const action = new vscode.CodeAction(
        `Ask Ironflyer to fix: ${ellipsize(d.message, 60)}`,
        vscode.CodeActionKind.QuickFix,
      );
      action.command = {
        command: 'ironflyer.fixDiagnostic',
        title: 'Ask Ironflyer to fix',
        arguments: [{
          uri: document.uri.toString(),
          languageId: document.languageId,
          range: serializeRange(d.range),
          diagnostic: {
            message: d.message,
            severity: serializeSeverity(d.severity),
            source: d.source,
            code: typeof d.code === 'object' ? String(d.code.value) : (d.code === undefined ? undefined : String(d.code)),
          },
        }],
      };
      action.diagnostics = [d];
      action.isPreferred = false;
      return action;
    });
  }
}

export function serializeRange(r: vscode.Range): { startLine: number; endLine: number } {
  return { startLine: r.start.line + 1, endLine: r.end.line + 1 };
}

export function serializeSeverity(s: vscode.DiagnosticSeverity): 'error' | 'warning' | 'info' | 'hint' {
  switch (s) {
    case vscode.DiagnosticSeverity.Error:       return 'error';
    case vscode.DiagnosticSeverity.Warning:     return 'warning';
    case vscode.DiagnosticSeverity.Information: return 'info';
    case vscode.DiagnosticSeverity.Hint:        return 'hint';
    default: return 'info';
  }
}

function ellipsize(s: string, max: number): string {
  return s.length > max ? s.slice(0, max - 1) + '…' : s;
}
