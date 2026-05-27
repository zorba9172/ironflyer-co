// Extension entrypoint. Wires the long-lived services together and registers
// commands, the URI handler, the TreeViews, the Live Preview webview, and
// the status bar.
//
// Everything that owns state — Auth, Api, ProjectsTree, StatusBar — gets
// pushed onto `context.subscriptions` so VSCode can tear them down cleanly
// on deactivate.

import * as vscode from 'vscode';
import { Auth } from './auth';
import { Api, ApiError, GateName, GateState, MemoryRecord } from './api';
import { IronflyerUriHandler } from './uriHandler';
import { ProjectsTree } from './projectsTree';
import { StatusBar } from './statusBar';
import { PatchDiffProvider } from './diffProvider';
import { PatchesTree } from './patchesTree';
import { buildPatchUri, patchTabTitle } from './patchUri';
import { GatesTree } from './gatesTree';
import { IronflyerMemoryProvider } from './memoryTree';
import { IronflyerAuditProvider } from './auditTree';
import { IronflyerTelemetryProvider } from './telemetryTree';
import { ProjectStream } from './projectStream';
import { throttleTrailing } from './throttle';
import { ActiveProject } from './activeProject';
import { IronflyerCodeActions } from './codeActions';
import { buildFixPrompt, DiagnosticSeverity } from './fixPrompt';
import { log } from './logger';
import { PreviewView } from './previewView';
import { RunOutput } from './runOutput';
import { readConfig } from './config';
import { initSentry, captureException, flushSentry } from './sentry';
import {
  IronflyerInlineCompletionProvider,
  InlineCompletionsStatusBar,
} from './inlineCompletions';

// Heavy modules that aren't needed for activation are imported lazily
// inside the command handler that needs them. Keeps cold-start parse
// cost off the critical path.
type ChatPanelMod = typeof import('./chatPanel');
type GraphViewMod = typeof import('./graphView');
type MemoryTreeMod = typeof import('./memoryTree');

export function activate(context: vscode.ExtensionContext): void {
  // Sentry — fail-soft. Returns false when ironflyer.sentryDsn is empty,
  // so users who haven't opted in never ship telemetry from their editor.
  if (initSentry()) {
    log.info('Sentry exception reporting enabled');
  }
  context.subscriptions.push({ dispose: () => void flushSentry() });
  const auth = new Auth(context.secrets);
  const api = new Api(auth);
  const tree = new ProjectsTree(api, auth);
  const diffProvider = new PatchDiffProvider(api);
  const patchesTree = new PatchesTree(api, auth, diffProvider);
  const gatesTree = new GatesTree(api, auth);
  const projectStream = new ProjectStream(api, (msg, err) => log.warn(msg, err));
  const activeProject = new ActiveProject(context.workspaceState, api, auth);
  const status = new StatusBar(api, auth, activeProject);
  const inlineProvider = new IronflyerInlineCompletionProvider(api, auth);
  const inlineStatus = new InlineCompletionsStatusBar();
  const previewView = new PreviewView(api, auth, context.extensionUri);
  const runOutput = new RunOutput();
  const memoryTree = new IronflyerMemoryProvider(api, auth, activeProject);
  const auditTree = new IronflyerAuditProvider(api, auth, activeProject);
  const telemetryTree = new IronflyerTelemetryProvider(api, auth);

  // Coalesce bursts of lifecycle events into a single refresh per project.
  // Without this the trees thrash when the orchestrator emits a stream
  // of "running" events at 50ms intervals during a Finisher pass.
  const refreshProject = throttleTrailing((projectId: string) => {
    patchesTree.refreshProject(projectId);
    gatesTree.refreshProject(projectId);
    if (activeProject.get()?.id === projectId) void previewView.refresh();
  }, 300);

  // Push initial signedIn context so viewsWelcome resolves correctly.
  void (async () => {
    const t = await auth.getToken();
    void vscode.commands.executeCommand('setContext', 'ironflyer.signedIn', Boolean(t));
  })();

  // Auto-pin defaultProject from settings if nothing pinned yet.
  void (async () => {
    if (activeProject.get()) return;
    const { defaultProject } = readConfig();
    if (!defaultProject) return;
    if (!(await auth.getToken())) return;
    try {
      const p = await api.getProject(defaultProject);
      await activeProject.set({ id: p.id, name: p.name });
      void previewView.setProject(p);
    } catch (err) {
      log.warn(`defaultProject ${defaultProject} not loadable`, err);
    }
  })();

  // Sync preview view with the pinned project whenever it changes.
  activeProject.onDidChange(async (ref) => {
    if (!ref) {
      await previewView.setProject(undefined);
      return;
    }
    try {
      const p = await api.getProject(ref.id);
      await previewView.setProject(p);
    } catch (err) {
      log.warn('active project changed but getProject failed', err);
    }
  });

  // Subscribe to the pinned project's lifecycle stream so we can surface
  // run events globally (Run output channel + patch toasts), even when no
  // chat panel is open.
  let globalStreamSub: { dispose(): void } | undefined;
  const wireGlobalStream = async () => {
    globalStreamSub?.dispose();
    globalStreamSub = undefined;
    const ref = activeProject.get();
    if (!ref || !(await auth.getToken())) return;
    globalStreamSub = projectStream.subscribe(ref.id, (evt) => {
      runOutput.handle(ref.id, ref.name, evt, {
        onPatchProposed: (patchId) => offerPatchReview(api, auth, patchesTree, tree, diffProvider, patchId, ref.id),
      });
      refreshProject(ref.id);
    });
  };
  activeProject.onDidChange(() => void wireGlobalStream());
  auth.onDidChange(() => void wireGlobalStream());
  void wireGlobalStream();

  context.subscriptions.push(
    vscode.window.registerUriHandler(new IronflyerUriHandler(auth)),
    vscode.window.registerTreeDataProvider('ironflyer.projects', tree),
    vscode.window.registerTreeDataProvider('ironflyer.patches', patchesTree),
    vscode.window.registerTreeDataProvider('ironflyer.gates', gatesTree),
    vscode.window.registerTreeDataProvider('ironflyer.memory', memoryTree),
    vscode.window.registerTreeDataProvider('ironflyer.audit', auditTree),
    vscode.window.registerTreeDataProvider('ironflyer.telemetry', telemetryTree),
    vscode.window.registerWebviewViewProvider(PreviewView.viewId, previewView, {
      webviewOptions: { retainContextWhenHidden: true },
    }),
    vscode.workspace.registerTextDocumentContentProvider('ironflyer', diffProvider),
    vscode.languages.registerCodeActionsProvider(
      { scheme: 'file' },
      new IronflyerCodeActions(),
      IronflyerCodeActions.metadata,
    ),
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (e.affectsConfiguration('ironflyer.preview')
       || e.affectsConfiguration('ironflyer.runtimeUrl')) {
        void previewView.refresh();
      }
    }),
    {
      dispose: () => {
        projectStream.disposeAll();
        refreshProject.cancel();
        globalStreamSub?.dispose();
        runOutput.dispose();
        previewView.dispose();
        log.dispose();
      },
    },
    status,
    inlineStatus,

    // Cursor-style ghost-text. Registered for all languages — the
    // orchestrator picks the right provider via the capability-tagged
    // bandit, and the provider itself bails when the user disables
    // `ironflyer.completions.enabled` so the registration stays static.
    vscode.languages.registerInlineCompletionItemProvider(
      { pattern: '**' },
      inlineProvider,
    ),

    vscode.commands.registerCommand('ironflyer.toggleInlineCompletions', async () => {
      const cfg = vscode.workspace.getConfiguration('ironflyer');
      const current = cfg.get<boolean>('completions.enabled', true);
      const target = !current;
      // Workspace wins when an explicit workspace value already exists;
      // otherwise we flip the Global setting so a fresh window inherits
      // the user's preference.
      const inspect = cfg.inspect<boolean>('completions.enabled');
      const scope = inspect?.workspaceValue !== undefined
        ? vscode.ConfigurationTarget.Workspace
        : vscode.ConfigurationTarget.Global;
      await cfg.update('completions.enabled', target, scope);
      inlineStatus.refresh();
      void vscode.window.showInformationMessage(
        `Ironflyer inline AI is now ${target ? 'on' : 'off'}.`,
      );
    }),

    vscode.commands.registerCommand('ironflyer.inlineCompletionAccepted', () => {
      void api.inlineCompletionAccept();
    }),

    vscode.commands.registerCommand('ironflyer.signIn', () => auth.beginSignIn()),

    vscode.commands.registerCommand('ironflyer.signOut', async () => {
      await auth.setToken(undefined);
      void vscode.window.showInformationMessage('Ironflyer: signed out.');
    }),

    vscode.commands.registerCommand('ironflyer.refresh', () => {
      tree.refresh();
      patchesTree.refresh();
      gatesTree.refresh();
      memoryTree.refresh();
      auditTree.refresh();
      telemetryTree.refresh();
      void previewView.refresh();
    }),

    vscode.commands.registerCommand('ironflyer.openMemoryRecord', async (record: MemoryRecord) => {
      if (!record) return;
      const { openMemoryRecord } = await import('./memoryTree') as MemoryTreeMod;
      await openMemoryRecord(record);
    }),

    vscode.commands.registerCommand('ironflyer.openDependencyGraph', async () => {
      const { GraphView } = await import('./graphView') as GraphViewMod;
      GraphView.reveal(api, activeProject);
    }),

    vscode.commands.registerCommand(
      'ironflyer.showPatchDiff',
      async (arg: { projectId: string; patchId: string; path: string; op: string }) => {
        if (!arg?.patchId || !arg?.path || !arg?.projectId) return;
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
      const patchId = await resolvePatchId(api, auth, arg);
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
        void previewView.refresh();
        void vscode.window.showInformationMessage(`Patch ${patchId} applied.`);
      } catch (err) {
        await handleError(err, auth, () => api.applyPatch(patchId));
      }
    }),

    vscode.commands.registerCommand('ironflyer.rejectPatch', async (arg: unknown) => {
      const patchId = await resolvePatchId(api, auth, arg);
      if (!patchId) return;
      try {
        await withProgress(`Rejecting ${patchId}`, () => api.rejectPatch(patchId));
        patchesTree.refresh();
        void vscode.window.showInformationMessage(`Patch ${patchId} rejected.`);
      } catch (err) {
        await handleError(err, auth);
      }
    }),

    vscode.commands.registerCommand('ironflyer.openChat', async (arg: unknown) => {
      const project = await resolveProject(api, auth, arg);
      if (!project) return;
      const { ChatPanel } = await import('./chatPanel') as ChatPanelMod;
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
        await activeProject.set({ id: project.id, name: project.name });
        const { ChatPanel } = await import('./chatPanel') as ChatPanelMod;
        ChatPanel.reveal(api, context, project, {
          stream: projectStream,
          onProjectEvent: (projectId) => refreshProject(projectId),
        });
      } catch (err) {
        await handleError(err, auth);
      }
    }),

    vscode.commands.registerCommand('ironflyer.runFinisher', async (arg: unknown) => {
      const project = await resolveProject(api, auth, arg);
      if (!project) return;
      runOutput.show();
      try {
        await withProgress(`Running Finisher on ${project.name}`, async () => {
          await api.runFinisher(project.id);
        });
      } catch (err) {
        await handleError(err, auth, () => api.runFinisher(project.id));
        return;
      }
      tree.refresh();
      patchesTree.refresh();
      gatesTree.refresh();
      void previewView.refresh();
    }),

    vscode.commands.registerCommand(
      'ironflyer.rerunGate',
      async (arg: { gate: GateState; projectId: string } | unknown) => {
        const node = arg as { gate?: GateState; projectId?: string } | undefined;
        if (!node?.gate?.name || !node?.projectId) {
          void vscode.window.showWarningMessage('Right-click a gate to re-run it.');
          return;
        }
        const gate = node.gate.name as GateName;
        runOutput.show();
        try {
          await withProgress(`Re-running ${gate} gate`, () =>
            api.runFinisher(node.projectId!, gate),
          );
          gatesTree.refreshProject(node.projectId!);
        } catch (err) {
          await handleError(err, auth);
        }
      },
    ),

    vscode.commands.registerCommand('ironflyer.openPreview', async (arg: unknown) => {
      const project = await resolveProject(api, auth, arg);
      if (!project) return;
      // Make sure the pin matches what we're previewing.
      await activeProject.set({ id: project.id, name: project.name });
      await vscode.commands.executeCommand('workbench.view.extension.ironflyer');
      // The view itself reads the active project on its next refresh().
      await previewView.setProject(project);
    }),

    vscode.commands.registerCommand('ironflyer.refreshPreview', () => previewView.refresh()),

    vscode.commands.registerCommand('ironflyer.openPreviewInBrowser', async () => {
      const ref = activeProject.get();
      if (!ref) {
        void vscode.window.showWarningMessage('Pin a project first.');
        return;
      }
      try {
        const ws = await api.findWorkspaceForProject(ref.id);
        if (!ws?.previewUrl) {
          void vscode.window.showInformationMessage(
            'No preview URL yet — run the Finisher to provision a workspace.',
          );
          return;
        }
        void vscode.env.openExternal(vscode.Uri.parse(ws.previewUrl));
      } catch (err) {
        await handleError(err, auth);
      }
    }),

    vscode.commands.registerCommand('ironflyer.openProjectInBrowser', async (arg: unknown) => {
      const project = await resolveProject(api, auth, arg);
      if (!project) return;
      const { webUrl } = readConfig();
      void vscode.env.openExternal(vscode.Uri.parse(`${webUrl}/projects/${project.id}`));
    }),

    vscode.commands.registerCommand('ironflyer.setActiveProject', async () => {
      const ref = await activeProject.pick();
      if (ref) void vscode.window.showInformationMessage(`Ironflyer: pinned to ${ref.name}`);
    }),

    vscode.commands.registerCommand('ironflyer.quickActions', async () => {
      const ref = activeProject.get();
      type Item = vscode.QuickPickItem & { run: () => Thenable<unknown> | Promise<unknown> | void };
      const items: Item[] = [
        {
          label: '$(play) Run Finisher',
          description: ref ? `on ${ref.name}` : 'pick a project',
          run: () => vscode.commands.executeCommand('ironflyer.runFinisher'),
        },
        {
          label: '$(preview) Open Live Preview',
          run: () => vscode.commands.executeCommand('ironflyer.openPreview'),
        },
        {
          label: '$(comment-discussion) Open Chat',
          run: () => vscode.commands.executeCommand('ironflyer.openChat'),
        },
        {
          label: '$(git-pull-request) Show Patches',
          run: () => vscode.commands.executeCommand('workbench.view.extension.ironflyer'),
        },
        {
          label: '$(checklist) Show Gates',
          run: () => vscode.commands.executeCommand('workbench.view.extension.ironflyer'),
        },
        {
          label: '$(credit-card) Show Budget',
          run: () => vscode.commands.executeCommand('ironflyer.showBudget'),
        },
        {
          label: '$(pin) Pin Active Project',
          run: () => vscode.commands.executeCommand('ironflyer.setActiveProject'),
        },
        {
          label: '$(output) Show Run Output',
          run: () => vscode.commands.executeCommand('ironflyer.showRunOutput'),
        },
        {
          label: '$(notebook) Show Logs',
          run: () => vscode.commands.executeCommand('ironflyer.showLogs'),
        },
      ];
      const pick = await vscode.window.showQuickPick(items, {
        placeHolder: 'Ironflyer · pick an action',
      });
      if (pick) await pick.run();
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
        const { ChatPanel } = await import('./chatPanel') as ChatPanelMod;
        const panel = ChatPanel.reveal(api, context, project, {
          stream: projectStream,
          onProjectEvent: (projectId) => refreshProject(projectId),
        });
        panel.submitFromHost(prompt, 'coder', 'economy');
      },
    ),

    vscode.commands.registerCommand('ironflyer.showLogs', () => log.show()),
    vscode.commands.registerCommand('ironflyer.showRunOutput', () => runOutput.show()),

    vscode.commands.registerCommand('ironflyer.showBudget', async () => {
      try {
        const b = await api.myBudget();
        void vscode.window.showInformationMessage(
          `Ironflyer · ${b.tier} · $${b.monthSpend} of $${b.monthCap}${b.hardStop ? ' (hard stop)' : ''}`,
        );
      } catch (err) {
        await handleError(err, auth, () => api.myBudget());
      }
    }),
  );
}

export function deactivate(): void {}

/**
 * Toast that surfaces a freshly proposed patch. The lifecycle stream is the
 * authoritative source; we de-dupe by patchId in RunOutput so the toast
 * fires once per patch even if the orchestrator emits the event twice.
 */
function offerPatchReview(
  api: Api,
  auth: Auth,
  patchesTree: PatchesTree,
  tree: ProjectsTree,
  diff: PatchDiffProvider,
  patchId: string,
  projectId: string,
): void {
  void (async () => {
    // Fetch the patch metadata so the toast carries a real title.
    let title = patchId;
    try {
      const all = await api.listPatches(projectId);
      const p = all.find((x) => x.id === patchId);
      if (p?.title) title = p.title;
    } catch { /* fall through with raw id */ }

    const pick = await vscode.window.showInformationMessage(
      `Ironflyer · new patch proposed: ${title}`,
      'Review',
      'Apply',
      'Dismiss',
    );
    if (pick === 'Review') {
      try {
        await diff.primePatchesFor(projectId);
        const list = await api.listPatches(projectId);
        const p = list.find((x) => x.id === patchId);
        const change = p?.changes[0];
        if (change) {
          await vscode.commands.executeCommand('ironflyer.showPatchDiff', {
            projectId, patchId, path: change.path, op: change.op,
          });
        } else {
          await vscode.commands.executeCommand('workbench.view.extension.ironflyer');
        }
      } catch (err) {
        await handleError(err, auth);
      }
      return;
    }
    if (pick === 'Apply') {
      try {
        await vscode.window.withProgress(
          { location: vscode.ProgressLocation.Notification, title: `Applying ${patchId}` },
          () => api.applyPatch(patchId),
        );
        patchesTree.refresh();
        tree.refresh();
        void vscode.window.showInformationMessage(`Patch ${patchId} applied.`);
      } catch (err) {
        await handleError(err, auth);
      }
    }
  })();
}

async function resolvePatchId(api: Api, auth: Auth, arg: unknown): Promise<string | undefined> {
  if (typeof arg === 'string') return arg;
  if (arg && typeof arg === 'object') {
    const a = arg as any;
    if (a.patchId) return String(a.patchId);
    if (a.kind === 'patch' && a.patch?.id) return String(a.patch.id);
  }
  let projects;
  try {
    projects = await api.listProjects();
  } catch (err) {
    await handleError(err, auth);
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
    await handleError(err, auth);
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

async function resolveProject(api: Api, auth: Auth, arg: unknown) {
  if (typeof arg === 'string') {
    try {
      return await api.getProject(arg);
    } catch (err) {
      await handleError(err, auth);
      return undefined;
    }
  }
  if (arg && typeof arg === 'object') {
    const a = arg as any;
    if (a.project?.id) {
      try {
        return await api.getProject(String(a.project.id));
      } catch (err) {
        await handleError(err, auth);
        return undefined;
      }
    }
  }
  let projects;
  try {
    projects = await api.listProjects();
  } catch (err) {
    await handleError(err, auth);
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

/**
 * One funnel for every API failure. 401 → prompt re-sign-in. 5xx → toast
 * with Retry + View Logs. Everything else → simple error toast. The
 * optional `retry` closure lets the caller hook into the Retry button.
 */
async function handleError(
  err: unknown,
  auth: Auth,
  retry?: () => Promise<unknown>,
): Promise<void> {
  log.error('command failed', err);
  // Report to Sentry alongside the existing error toast. 401s are user
  // state, not a defect — skip those so the dashboard doesn't fill with
  // session-expiry noise. Everything else (5xx, unexpected client-side
  // throws) is worth investigating.
  if (!(err instanceof ApiError && err.status === 401)) {
    captureException(err, err instanceof ApiError ? { status: err.status } : undefined);
  }
  if (err instanceof ApiError && err.status === 401) {
    const pick = await vscode.window.showWarningMessage(
      'Ironflyer session expired — sign in again.',
      'Sign In',
    );
    await auth.setToken(undefined);
    if (pick === 'Sign In') await auth.beginSignIn();
    return;
  }
  if (err instanceof ApiError && err.status >= 500) {
    const actions = retry ? ['Retry', 'View Logs'] : ['View Logs'];
    const pick = await vscode.window.showErrorMessage(
      `Ironflyer · server error (${err.status}): ${err.message}`,
      ...actions,
    );
    if (pick === 'View Logs') log.show();
    if (pick === 'Retry' && retry) {
      try { await retry(); } catch (again) { await handleError(again, auth); }
    }
    return;
  }
  const message = err instanceof Error ? err.message : String(err);
  void vscode.window.showErrorMessage(`Ironflyer: ${message}`);
}
