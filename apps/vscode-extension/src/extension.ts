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
import { PatchDiffProvider } from './diffProvider';
import { PatchesTree } from './patchesTree';
import { buildPatchUri, patchTabTitle } from './patchUri';
import { GatesTree } from './gatesTree';
import { ProjectStream } from './projectStream';
import { throttleTrailing } from './throttle';
import { ActiveProject } from './activeProject';
import { IronflyerCodeActions } from './codeActions';
import { buildFixPrompt, DiagnosticSeverity } from './fixPrompt';
import { log } from './logger';

export function activate(context: vscode.ExtensionContext): void {
  const auth = new Auth(context.secrets);
  const api = new Api(auth);
  const tree = new ProjectsTree(api, auth);
  const diffProvider = new PatchDiffProvider(api);
  const patchesTree = new PatchesTree(api, auth, diffProvider);
  const gatesTree = new GatesTree(api, auth);
  const projectStream = new ProjectStream(api, (msg, err) => log.warn(msg, err));
  const status = new StatusBar(api, auth);
  const activeProject = new ActiveProject(context.workspaceState, api, auth);

  // Coalesce bursts of lifecycle events into a single refresh per project.
  // Without this the trees thrash when the orchestrator emits a stream
  // of "running" events at 50ms intervals during a Finisher pass.
  const refreshProject = throttleTrailing((projectId: string) => {
    patchesTree.refreshProject(projectId);
    gatesTree.refreshProject(projectId);
  }, 300);

  // Push initial signedIn context so viewsWelcome resolves correctly.
  void auth.getToken().then((t) =>
    vscode.commands.executeCommand('setContext', 'ironflyer.signedIn', Boolean(t)),
  );

  context.subscriptions.push(
    vscode.window.registerUriHandler(new IronflyerUriHandler(auth)),
    vscode.window.registerTreeDataProvider('ironflyer.projects', tree),
    vscode.window.registerTreeDataProvider('ironflyer.patches', patchesTree),
    vscode.window.registerTreeDataProvider('ironflyer.gates', gatesTree),
    vscode.workspace.registerTextDocumentContentProvider('ironflyer', diffProvider),
    vscode.languages.registerCodeActionsProvider(
      { scheme: 'file' },
      new IronflyerCodeActions(),
      IronflyerCodeActions.metadata,
    ),
    { dispose: () => { projectStream.disposeAll(); refreshProject.cancel(); log.dispose(); } },
    status,

    vscode.commands.registerCommand('ironflyer.signIn', () => auth.beginSignIn()),

    vscode.commands.registerCommand('ironflyer.signOut', async () => {
      await auth.setToken(undefined);
      void vscode.window.showInformationMessage('Ironflyer: signed out.');
    }),

    vscode.commands.registerCommand('ironflyer.refresh', () => {
      tree.refresh();
      patchesTree.refresh();
      gatesTree.refresh();
    }),

    vscode.commands.registerCommand(
      'ironflyer.showPatchDiff',
      async (arg: { projectId: string; patchId: string; path: string; op: string }) => {
        if (!arg?.patchId || !arg?.path || !arg?.projectId) return;
        // Make sure the diff provider has fetched the patch list for this
        // project; otherwise the "proposed" side would return empty.
        await diffProvider.primePatchesFor(arg.projectId);
        const left = vscode.Uri.parse(buildPatchUri('current', arg.projectId, arg.path));
        const right = vscode.Uri.parse(buildPatchUri('proposed', arg.patchId, arg.path));
        await vscode.commands.executeCommand(
          'vscode.diff',
          left,
          right,
          patchTabTitle(arg.patchId, arg.path),
          { preview: true },
        );
      },
    ),

    vscode.commands.registerCommand('ironflyer.applyPatch', async (arg: unknown) => {
      const patchId = await resolvePatchId(api, arg);
      if (!patchId) return;
      const choice = await vscode.window.showWarningMessage(
        `Apply patch ${patchId}?`,
        { modal: true },
        'Apply',
      );
      if (choice !== 'Apply') return;
      try {
        await withProgress(`Applying ${patchId}`, () => api.applyPatch(patchId));
        patchesTree.refresh();
        tree.refresh();
        void vscode.window.showInformationMessage(`Patch ${patchId} applied.`);
      } catch (err) {
        await handleError(err, auth);
      }
    }),

    vscode.commands.registerCommand('ironflyer.openChat', async (arg: unknown) => {
      const project = await resolveProject(api, arg);
      if (!project) return;
      ChatPanel.reveal(api, context, project, {
        stream: projectStream,
        onProjectEvent: (projectId) => refreshProject(projectId),
      });
    }),

    vscode.commands.registerCommand('ironflyer.newProject', async () => {
      if (!(await auth.getToken())) {
        const sign = await vscode.window.showWarningMessage(
          'Sign in to Ironflyer first.',
          'Sign In',
        );
        if (sign) await auth.beginSignIn();
        return;
      }
      const name = await vscode.window.showInputBox({
        prompt: 'Project name',
        placeHolder: 'e.g. "ledger-rewrite" or "mobile-onboarding"',
        validateInput: (v) => (v.trim() ? undefined : 'Name is required'),
      });
      if (!name) return;
      const idea = await vscode.window.showInputBox({
        prompt: 'One-line idea — what does it do?',
        placeHolder: 'A self-serve audit report for SOC-2 evidence.',
      });
      try {
        const project = await withProgress(`Creating ${name}`, () =>
          api.createProject({ name: name.trim(), idea: idea?.trim() }),
        );
        tree.refresh();
        ChatPanel.reveal(api, context, project, {
          stream: projectStream,
          onProjectEvent: (projectId) => refreshProject(projectId),
        });
      } catch (err) {
        await handleError(err, auth);
      }
    }),

    vscode.commands.registerCommand('ironflyer.runFinisher', async (arg: unknown) => {
      const project = await resolveProject(api, arg);
      if (!project) return;
      await withProgress(`Running Finisher on ${project.name}`, async () => {
        await api.runFinisher(project.id);
      });
      tree.refresh();
      patchesTree.refresh();
      gatesTree.refresh();
    }),

    vscode.commands.registerCommand('ironflyer.openProjectInBrowser', async (arg: unknown) => {
      const project = await resolveProject(api, arg);
      if (!project) return;
      const cfg = vscode.workspace.getConfiguration('ironflyer');
      const webUrl = (cfg.get<string>('webUrl', 'http://localhost:3000') ?? '').replace(/\/+$/, '');
      void vscode.env.openExternal(vscode.Uri.parse(`${webUrl}/projects/${project.id}`));
    }),

    vscode.commands.registerCommand('ironflyer.setActiveProject', async () => {
      const ref = await activeProject.pick();
      if (ref) void vscode.window.showInformationMessage(`Ironflyer: pinned to ${ref.name}`);
    }),

    vscode.commands.registerCommand(
      'ironflyer.fixDiagnostic',
      async (payload: {
        uri: string;
        languageId: string;
        range: { startLine: number; endLine: number };
        diagnostic: { message: string; severity: DiagnosticSeverity; source?: string; code?: string };
      }) => {
        if (!payload?.uri) return;
        const ref = await activeProject.resolve();
        if (!ref) return;
        let project;
        try {
          project = await api.getProject(ref.id);
        } catch (err) {
          await handleError(err, auth);
          return;
        }
        const doc = await vscode.workspace.openTextDocument(vscode.Uri.parse(payload.uri));
        const startLineIdx = Math.max(0, payload.range.startLine - 1);
        const endLineIdx = Math.min(doc.lineCount - 1, payload.range.endLine - 1);
        const snippet = doc.getText(
          new vscode.Range(startLineIdx, 0, endLineIdx, doc.lineAt(endLineIdx).range.end.character),
        );
        const filePath = vscode.workspace.asRelativePath(doc.uri, false);
        const prompt = buildFixPrompt({
          filePath,
          language: payload.languageId,
          startLine: payload.range.startLine,
          endLine: payload.range.endLine,
          message: payload.diagnostic.message,
          severity: payload.diagnostic.severity,
          source: payload.diagnostic.source,
          code: payload.diagnostic.code,
          snippet,
        });
        const panel = ChatPanel.reveal(api, context, project, {
          stream: projectStream,
          onProjectEvent: (projectId) => refreshProject(projectId),
        });
        panel.submitFromHost(prompt, 'coder', 'economy');
      },
    ),

    vscode.commands.registerCommand('ironflyer.showLogs', () => log.show()),

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

async function resolvePatchId(api: Api, arg: unknown): Promise<string | undefined> {
  if (typeof arg === 'string') return arg;
  if (arg && typeof arg === 'object') {
    const a = arg as any;
    if (a.patchId) return String(a.patchId);
    // PatchesTree node of kind "patch".
    if (a.kind === 'patch' && a.patch?.id) return String(a.patch.id);
  }
  // Command-palette path: pick a project, then a patch.
  let projects;
  try {
    projects = await api.listProjects();
  } catch (err) {
    void vscode.window.showErrorMessage(`Ironflyer: ${(err as Error).message}`);
    return undefined;
  }
  if (projects.length === 0) return undefined;
  const projectPick = await vscode.window.showQuickPick(
    projects.map((p) => ({ label: p.name, description: p.id, id: p.id })),
    { placeHolder: 'Select a project' },
  );
  if (!projectPick) return undefined;
  let patches;
  try {
    patches = await api.listPatches(projectPick.id);
  } catch (err) {
    void vscode.window.showErrorMessage(`Ironflyer: ${(err as Error).message}`);
    return undefined;
  }
  if (patches.length === 0) {
    void vscode.window.showInformationMessage('No patches for that project.');
    return undefined;
  }
  const pick = await vscode.window.showQuickPick(
    patches.map((p) => ({
      label: p.title || p.id,
      description: p.status,
      detail: `${p.changes.length} file change${p.changes.length === 1 ? '' : 's'}`,
      id: p.id,
    })),
    { placeHolder: 'Select a patch' },
  );
  return pick?.id;
}

async function resolveProject(api: Api, arg: unknown) {
  // String id (command.arguments on TreeItem.command).
  if (typeof arg === 'string') {
    try {
      return await api.getProject(arg);
    } catch (err) {
      void vscode.window.showErrorMessage(`Ironflyer: ${(err as Error).message}`);
      return undefined;
    }
  }
  // Tree element passed by `view/item/context` menus — any of our nodes
  // that own a project will set `.project`.
  if (arg && typeof arg === 'object') {
    const a = arg as any;
    if (a.project?.id) {
      try {
        return await api.getProject(String(a.project.id));
      } catch (err) {
        void vscode.window.showErrorMessage(`Ironflyer: ${(err as Error).message}`);
        return undefined;
      }
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
  log.error('command failed', err);
  if (err instanceof ApiError && err.status === 401) {
    void vscode.window.showWarningMessage('Ironflyer session expired — sign in again.');
    await auth.setToken(undefined);
    return;
  }
  void vscode.window.showErrorMessage(`Ironflyer: ${(err as Error).message}`);
}
