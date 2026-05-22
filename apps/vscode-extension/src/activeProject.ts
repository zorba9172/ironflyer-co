// Active-project state — stored in workspaceState so each VSCode window
// can pin to a different Ironflyer project. Used by the code-action
// provider ("Ask Ironflyer to fix") and the status bar item.
//
// Storage shape: { id: string; name: string } | undefined.

import * as vscode from 'vscode';
import { Api, Project } from './api';
import { Auth } from './auth';

const KEY = 'ironflyer.activeProject';

export interface ActiveProjectRef {
  id: string;
  name: string;
}

export class ActiveProject {
  private readonly _onDidChange = new vscode.EventEmitter<ActiveProjectRef | undefined>();
  readonly onDidChange = this._onDidChange.event;

  constructor(
    private readonly state: vscode.Memento,
    private readonly api: Api,
    private readonly auth: Auth,
  ) {
    auth.onDidChange((t) => {
      if (!t) void this.set(undefined);
    });
  }

  get(): ActiveProjectRef | undefined {
    return this.state.get<ActiveProjectRef>(KEY);
  }

  async set(ref: ActiveProjectRef | undefined): Promise<void> {
    await this.state.update(KEY, ref);
    await vscode.commands.executeCommand('setContext', 'ironflyer.hasActiveProject', Boolean(ref));
    this._onDidChange.fire(ref);
  }

  /**
   * Prompts the user with a quick-pick of their projects. If only one
   * exists it auto-selects. Returns the chosen ref (also persisted).
   */
  async pick(): Promise<ActiveProjectRef | undefined> {
    if (!(await this.auth.getToken())) {
      void vscode.window.showWarningMessage('Sign in to Ironflyer first.');
      return undefined;
    }
    let projects: Project[];
    try {
      projects = await this.api.listProjects();
    } catch (err) {
      void vscode.window.showErrorMessage(`Ironflyer: ${(err as Error).message}`);
      return undefined;
    }
    if (projects.length === 0) {
      void vscode.window.showInformationMessage('No Ironflyer projects yet.');
      return undefined;
    }
    if (projects.length === 1) {
      const ref = { id: projects[0].id, name: projects[0].name };
      await this.set(ref);
      return ref;
    }
    const pick = await vscode.window.showQuickPick(
      projects.map((p) => ({
        label: p.name,
        description: p.id,
        detail: p.description ?? p.spec?.idea,
        project: p,
      })),
      { placeHolder: 'Pin an Ironflyer project to this window' },
    );
    if (!pick) return undefined;
    const ref = { id: pick.project.id, name: pick.project.name };
    await this.set(ref);
    return ref;
  }

  /**
   * Returns the current active project, prompting once if there isn't one
   * yet. Returns undefined if the user cancels.
   */
  async resolve(): Promise<ActiveProjectRef | undefined> {
    const current = this.get();
    if (current) return current;
    return this.pick();
  }
}
