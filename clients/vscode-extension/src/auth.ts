// Auth lifecycle for the Ironflyer extension.
//
// The JWT is stored in VSCode's SecretStorage so it survives reloads and
// is not exposed to settings.json or other extensions. The sign-in flow
// punts the user to the Ironflyer web app — that's the only surface that
// knows how to do email/password, GitHub OAuth, and Stripe linkage. The
// web app then calls vscode://ironflyer.ironflyer/auth?token=… which our
// URI handler routes back to setToken().

import * as vscode from 'vscode';
import { readConfig } from './config';

const SECRET_KEY = 'ironflyer.token';

export class Auth {
  private readonly _onDidChange = new vscode.EventEmitter<string | undefined>();
  readonly onDidChange = this._onDidChange.event;

  constructor(private readonly secrets: vscode.SecretStorage) {}

  async getToken(): Promise<string | undefined> {
    return this.secrets.get(SECRET_KEY);
  }

  async setToken(token: string | undefined): Promise<void> {
    if (token && token.trim()) {
      await this.secrets.store(SECRET_KEY, token.trim());
    } else {
      await this.secrets.delete(SECRET_KEY);
    }
    this._onDidChange.fire(token);
    // Drive `when` clauses in package.json (viewsWelcome etc.).
    await vscode.commands.executeCommand(
      'setContext',
      'ironflyer.signedIn',
      Boolean(token && token.trim()),
    );
  }

  /**
   * Open the web app's sign-in page with a callback to our URI handler.
   * Returns immediately — the actual token arrival is async via the URI
   * handler. Callers should listen to `onDidChange` if they need to react.
   */
  async beginSignIn(): Promise<void> {
    const { webUrl } = readConfig();
    const callback = vscode.Uri.parse(
      `${vscode.env.uriScheme}://ironflyer.ironflyer/auth`,
    );
    const url = `${webUrl}/login?source=vscode&callback=${encodeURIComponent(callback.toString())}`;
    await vscode.env.openExternal(vscode.Uri.parse(url));
    void vscode.window.showInformationMessage(
      'Finish signing in inside your browser — VSCode will pick the session up automatically.',
    );
  }
}
