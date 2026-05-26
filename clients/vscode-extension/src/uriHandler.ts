// URI handler for vscode://ironflyer.ironflyer/...
//
// The only path we handle today is /auth?token=<jwt>, posted by the web app
// after a successful sign-in. We deliberately do not accept tokens from any
// other source — if a future flow needs to drop credentials in, route them
// through the orchestrator first and exchange there.
//
// Shape contract:
//   - Path must be exactly /auth — anything else is rejected.
//   - `token` must be a non-empty string between 32 and 4096 bytes
//     (JWTs are typically 200–600 bytes; the upper bound is a defensive
//     cap so a malicious link can't blow up SecretStorage).

import * as vscode from 'vscode';
import { Auth } from './auth';

const TOKEN_MIN_LEN = 32;
const TOKEN_MAX_LEN = 4096;

export class IronflyerUriHandler implements vscode.UriHandler {
  constructor(private readonly auth: Auth) {}

  async handleUri(uri: vscode.Uri): Promise<void> {
    if (uri.path !== '/auth') {
      void vscode.window.showWarningMessage(`Ironflyer: unknown URI path ${uri.path}`);
      return;
    }
    const params = new URLSearchParams(uri.query);
    const token = (params.get('token') ?? '').trim();
    if (!token) {
      void vscode.window.showErrorMessage('Ironflyer sign-in callback missing token.');
      return;
    }
    if (token.length < TOKEN_MIN_LEN || token.length > TOKEN_MAX_LEN) {
      void vscode.window.showErrorMessage('Ironflyer sign-in token rejected: length out of bounds.');
      return;
    }
    // Cheap structural sanity check — JWTs are three base64url segments
    // separated by dots. We don't verify the signature here (the
    // orchestrator does that on the next API call), just shape.
    if (!/^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/.test(token)) {
      void vscode.window.showErrorMessage('Ironflyer sign-in token rejected: malformed.');
      return;
    }
    await this.auth.setToken(token);
    void vscode.window.showInformationMessage('Ironflyer: signed in.');
  }
}
