// Extension entrypoint. Wires the long-lived services together and registers
// commands, the URI handler, the TreeView, and the status bar.
//
// Everything that owns state — Auth, Api, ProjectsTree, StatusBar — gets
// pushed onto `context.subscriptions` so VSCode can tear them down cleanly
// on deactivate.

import * as vscode from 'vscode';
import { Auth } from './auth';
import { Api, ApiError } from './api';
import { IronflyerUriHandler } from './uriHandler';
import { ProjectsTree } from './projectsTree';
import { StatusBar } from './statusBar';
import { ChatPanel } from './chatPanel';

export function activate(context: vscode.ExtensionContext): void {
  const auth = new Auth(context.secrets);
  const api = new Api(auth);
  const tree = new ProjectsTree(api, auth);
  const status = new StatusBar(api, auth);

  // Push initial signedIn context so viewsWelcome resolves correctly.
  void auth.getToken().then((t) =>
    vscode.commands.executeCommand('setContext', 'ironflyer.signedIn', Boolean(t)),
  );

  context.subscriptions.push(
    vscode.window.registerUriHandler(new IronflyerUriHandler(auth)),
    vscode.window.registerTreeDataProvider('ironflyer.projects', tree),
    status,

    vscode.commands.registerCommand('ironflyer.signIn', () => auth.beginSignIn()),

    vscode.commands.registerCommand('ironflyer.signOut', async () => {
      await auth.setToken(undefined);
      void vscode.window.showInformationMessage('Ironflyer: signed out.');
    }),

    vscode.commands.registerCommand('ironflyer.refresh', () => tree.refresh()),

    vscode.commands.registerCommand('ironflyer.openChat', async (arg: unknown) => {
      const project = await resolveProject(api, arg);
      if (!project) return;
      ChatPanel.reveal(api, context, project);
    }),

    vscode.commands.registerCommand('ironflyer.runFinisher', async (arg: unknown) => {
      const project = await resolveProject(api, arg);
      if (!project) return;
      await withProgress(`Running Finisher on ${project.name}`, async () => {
        await api.runFinisher(project.id);
      });
      tree.refresh();
    }),

    vscode.commands.registerCommand('ironflyer.openProjectInBrowser', async (arg: unknown) => {
      const project = await resolveProject(api, arg);
      if (!project) return;
      const cfg = vscode.workspace.getConfiguration('ironflyer');
      const webUrl = (cfg.get<string>('webUrl', 'http://localhost:3000') ?? '').replace(/\/+$/, '');
      void vscode.env.openExternal(vscode.Uri.parse(`${webUrl}/projects/${project.id}`));
    }),

    vscode.commands.registerCommand('ironflyer.showBudget', async () => {
      try {
        const b = await api.myBudget();
        void vscode.window.showInformationMessage(
          `Ironflyer · ${b.tier} · $${b.monthSpend} of $${b.monthCap}${b.hardStop ? ' (hard stop)' : ''}`,
        );
      } catch (err) {
        await handleError(err, auth);
      }
    }),
  );
}

export function deactivate(): void {}

async function resolveProject(api: Api, arg: unknown) {
  if (typeof arg === 'string') {
    try {
      return await api.getProject(arg);
    } catch (err) {
      void vscode.window.showErrorMessage(`Ironflyer: ${(err as Error).message}`);
      return undefined;
    }
  }
  // Quick-pick when invoked from the command palette.
  let projects;
  try {
    projects = await api.listProjects();
  } catch (err) {
    void vscode.window.showErrorMessage(`Ironflyer: ${(err as Error).message}`);
    return undefined;
  }
  if (projects.length === 0) {
    void vscode.window.showInformationMessage('No Ironflyer projects yet.');
    return undefined;
  }
  const pick = await vscode.window.showQuickPick(
    projects.map((p) => ({
      label: p.name,
      description: p.id,
      detail: p.description ?? p.spec?.idea,
      project: p,
    })),
    { placeHolder: 'Select an Ironflyer project' },
  );
  return pick?.project;
}

async function withProgress<T>(title: string, fn: () => Promise<T>): Promise<T> {
  return vscode.window.withProgress(
    { location: vscode.ProgressLocation.Notification, title, cancellable: false },
    fn,
  );
}

async function handleError(err: unknown, auth: Auth): Promise<void> {
  if (err instanceof ApiError && err.status === 401) {
    void vscode.window.showWarningMessage('Ironflyer session expired — sign in again.');
    await auth.setToken(undefined);
    return;
  }
  void vscode.window.showErrorMessage(`Ironflyer: ${(err as Error).message}`);
}
