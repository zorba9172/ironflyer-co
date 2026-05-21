// URI handler for vscode://ironflyer.ironflyer/...
//
// The only path we handle today is /auth?token=<jwt>, posted by the web app
// after a successful sign-in. We deliberately do not accept tokens from any
// other source — if a future flow needs to drop credentials in, route them
// through the orchestrator first and exchange there.

import * as vscode from 'vscode';
import { Auth } from './auth';

export class IronflyerUriHandler implements vscode.UriHandler {
  constructor(private readonly auth: Auth) {}

  async handleUri(uri: vscode.Uri): Promise<void> {
    if (uri.path !== '/auth') {
      void vscode.window.showWarningMessage(`Ironflyer: unknown URI path ${uri.path}`);
      return;
    }
    const params = new URLSearchParams(uri.query);
    const token = params.get('token');
    if (!token) {
      void vscode.window.showErrorMessage('Ironflyer sign-in callback missing token.');
      return;
    }
    await this.auth.setToken(token);
    void vscode.window.showInformationMessage('Ironflyer: signed in.');
  }
}
