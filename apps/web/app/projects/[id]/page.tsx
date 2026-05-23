'use client';

// Project workspace — the moment of magic. Three-pane layout:
//   LEFT (320px):  Files tree + Gates timeline + Patches list (collapsible)
//   CENTER:        Tabs — Chat / Editor / Preview / Terminal
//   RIGHT (360px): Run panel with live SSE-fed activity timeline
//
// On first mount we:
//   - resolve the project,
//   - try to find an existing workspace for it (or create one lazily),
//   - subscribe to the orchestrator's run-stream,
//   - load the file tree and patches list,
//   - if a ?initialPrompt= query is present, kick off the first run.

import { use, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import {
  Box, Chip, IconButton, Stack, Tab, Tabs, Typography,
} from '@mui/material';
import {
  Chat as ChatIcon, Code as CodeIcon, KeyboardBackspace, Terminal as TerminalIcon,
  Visibility,
} from '@mui/icons-material';

import { api, Project } from '../../../lib/api';
import { runtime, Workspace as WS, FileEntry } from '../../../lib/runtime';
import { patches as patchApi, Patch } from '../../../lib/api/patches';
import { RunEvent, subscribeRunStream } from '../../../lib/api/orchestrator-stream';
import { tokens } from '../../../lib/theme';
import { RequireAuth } from '../../auth-context';

import { WorkspaceSidebar } from '../../../components/workspace/WorkspaceSidebar';
import { RunPanel } from '../../../components/workspace/RunPanel';
import { PreviewPane } from '../../../components/workspace/PreviewPane';
import { EditorPane } from '../../../components/workspace/EditorPane';
import { ChatPane } from '../../../components/workspace/ChatPane';
import { PatchDrawer } from '../../../components/workspace/PatchDrawer';
import { Terminal } from './Terminal';

// GATE_ORDER mirrors finisher.DefaultGates() on the orchestrator. Keep in
// sync when the gate list changes — the backend treats the gate keys as
// authoritative, the UI just renders them in this order with friendlier
// labels.
const GATE_ORDER: { key: string; label: string }[] = [
  { key: 'spec', label: 'Spec' },
  { key: 'ux', label: 'UX' },
  { key: 'arch', label: 'Architecture' },
  { key: 'code', label: 'Code' },
  { key: 'lint', label: 'Lint' },
  { key: 'test', label: 'Tests' },
  { key: 'security', label: 'Security' },
  { key: 'budget', label: 'Budget' },
  { key: 'deploy', label: 'Deploy' },
];

type CenterTab = 'chat' | 'editor' | 'preview' | 'terminal';

export default function ProjectPage({ params }: { params: Promise<{ id: string }> }) {
  return (
    <RequireAuth>
      <ProjectWorkspace params={params} />
    </RequireAuth>
  );
}

function ProjectWorkspace({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const searchParams = useSearchParams();
  const initialPrompt = searchParams.get('initialPrompt');

  const [project, setProject] = useState<Project | null>(null);
  const [projectError, setProjectError] = useState<string | null>(null);

  const [workspace, setWorkspace] = useState<WS | null>(null);
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [filesLoading, setFilesLoading] = useState(false);
  const [filesError, setFilesError] = useState<string | null>(null);

  const [patches, setPatches] = useState<Patch[]>([]);
  const [patchesLoading, setPatchesLoading] = useState(false);
  const [patchesError, setPatchesError] = useState<string | null>(null);
  const [activePatch, setActivePatch] = useState<Patch | null>(null);
  const [applyingPatch, setApplyingPatch] = useState(false);
  const [rollingBack, setRollingBack] = useState(false);

  const [runEvents, setRunEvents] = useState<RunEvent[]>([]);
  const [streamHealthy, setStreamHealthy] = useState(true);
  const [running, setRunning] = useState(false);
  const [hasRunOnce, setHasRunOnce] = useState(false);

  const [tab, setTab] = useState<CenterTab>('chat');
  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const initialPromptFired = useRef(false);

  // ---- bootstrap --------------------------------------------------------

  const loadProject = useCallback(async () => {
    try {
      const p = await api.getProject(id);
      setProject(p);
      setProjectError(null);
    } catch (e) {
      setProjectError(String((e as Error)?.message ?? e));
    }
  }, [id]);

  const loadPatches = useCallback(async () => {
    setPatchesLoading(true);
    setPatchesError(null);
    try {
      setPatches(await patchApi.list(id));
    } catch (e) {
      setPatchesError(String((e as Error)?.message ?? e));
    } finally {
      setPatchesLoading(false);
    }
  }, [id]);

  const loadFiles = useCallback(async (ws: WS) => {
    setFilesLoading(true);
    setFilesError(null);
    try {
      setFiles(await runtime.listFiles(ws.id));
    } catch (e) {
      setFilesError(String((e as Error)?.message ?? e));
    } finally {
      setFilesLoading(false);
    }
  }, []);

  // Find or create a workspace bound to this project. We never destroy on
  // unmount — the user expects the sandbox to persist between visits.
  const ensureWorkspace = useCallback(async () => {
    try {
      const all = await runtime.list();
      const existing = all.find((w) => w.projectId === id);
      if (existing) {
        setWorkspace(existing);
        await loadFiles(existing);
        return existing;
      }
    } catch {
      // listing might fail in stricter runtimes — fall through to create.
    }
    try {
      const ws = await runtime.create({ userId: 'demo', projectId: id });
      setWorkspace(ws);
      await loadFiles(ws);
      return ws;
    } catch (e) {
      setFilesError(String((e as Error)?.message ?? e));
      return null;
    }
  }, [id, loadFiles]);

  useEffect(() => {
    void loadProject();
    void loadPatches();
    void ensureWorkspace();
  }, [loadProject, loadPatches, ensureWorkspace]);

  // ---- live event stream -----------------------------------------------

  useEffect(() => {
    const handle = subscribeRunStream(id, {
      onEvent: (e) => {
        setRunEvents((prev) => {
          // de-dupe on id; cap at 400 to keep memory predictable.
          if (prev.some((p) => p.id === e.id)) return prev;
          const next = [...prev, e];
          return next.length > 400 ? next.slice(next.length - 400) : next;
        });
        // run_complete / run_failed both clear the running flag.
        if (e.kind === 'run_complete' || e.kind === 'run_failed') {
          setRunning(false);
          void loadProject();
          void loadPatches();
        }
        if (e.kind === 'run_started') setRunning(true);
        if (e.kind === 'patch_proposed' || e.kind === 'patch_applied') void loadPatches();
      },
      onOpen: () => setStreamHealthy(true),
      onError: () => setStreamHealthy(false),
    });
    return () => handle.close();
  }, [id, loadProject, loadPatches]);

  // After a successful run, default to Preview so the user sees the live app.
  useEffect(() => {
    if (!hasRunOnce) return;
    const completed = runEvents.some((e) => e.kind === 'run_complete' || e.status === 'done');
    if (completed) setTab('preview');
  }, [hasRunOnce, runEvents]);

  // ---- actions ----------------------------------------------------------

  const runFinisher = useCallback(async () => {
    setRunning(true);
    setHasRunOnce(true);
    try {
      await api.runFinisher(id);
    } catch (e) {
      setProjectError(String((e as Error)?.message ?? e));
      setRunning(false);
    } finally {
      void loadProject();
    }
  }, [id, loadProject]);

  // ?initialPrompt= → kick a run automatically on first mount, exactly once.
  useEffect(() => {
    if (!initialPrompt || initialPromptFired.current || !project) return;
    initialPromptFired.current = true;
    setTab('chat');
    void runFinisher();
  }, [initialPrompt, project, runFinisher]);

  const onSelectPatch = useCallback(async (patchId: string) => {
    const p = patches.find((x) => x.id === patchId);
    if (p) setActivePatch(p);
  }, [patches]);

  const onApplyPatch = useCallback(async () => {
    if (!activePatch) return;
    setApplyingPatch(true);
    try {
      const updated = await patchApi.apply(id, activePatch.id);
      setActivePatch(updated);
      await loadPatches();
      await loadProject();
      if (workspace) void loadFiles(workspace);
    } catch (e) {
      setActivePatch((p) => p ? { ...p, status: 'rejected' } : p);
      setProjectError(String((e as Error)?.message ?? e));
    } finally {
      setApplyingPatch(false);
    }
  }, [activePatch, id, loadFiles, loadPatches, loadProject, workspace]);

  const onRollbackPatch = useCallback(async () => {
    if (!activePatch) return;
    setRollingBack(true);
    try {
      await patchApi.rollback(activePatch.id);
      await loadPatches();
      await loadProject();
      if (workspace) void loadFiles(workspace);
      // Closing the drawer mirrors the implicit "operation done" affordance —
      // the patch is no longer applied, so its details panel is misleading.
      setActivePatch(null);
    } catch (e) {
      setProjectError(String((e as Error)?.message ?? e));
    } finally {
      setRollingBack(false);
    }
  }, [activePatch, loadFiles, loadPatches, loadProject, workspace]);

  const onSelectFile = useCallback((path: string) => {
    setSelectedFile(path);
    setTab('editor');
  }, []);

  const onRepair = useCallback(async () => {
    // Repair currently maps to a fresh run — the gate engine re-attempts the
    // failed gate first. When Agent A ships a dedicated /repair endpoint we
    // can swap it in here without touching the UI.
    await runFinisher();
  }, [runFinisher]);

  // ---- render -----------------------------------------------------------

  if (!project) {
    return (
      <Box sx={{ p: 4, minHeight: '100vh', bgcolor: tokens.color.bg.alabaster }}>
        <Typography variant="body2" color="text.secondary">
          {projectError ? `Loading error: ${projectError}` : 'Loading the workspace...'}
        </Typography>
      </Box>
    );
  }

  return (
    <Box dir="ltr" sx={{
      minHeight: '100vh',
      bgcolor: tokens.color.bg.alabaster,
      color: tokens.color.text.inverse,
      overflow: 'hidden',
    }}>
      <ProjectHeader p={project} running={running} />

      <Box sx={{
        display: 'grid',
        gridTemplateColumns: { xs: '1fr', lg: '320px minmax(0, 1fr) 360px' },
        gap: 1.2,
        p: { xs: 1, lg: 1.4 },
        height: { xs: 'auto', lg: 'calc(100vh - 58px)' },
        minHeight: { xs: 'calc(100vh - 58px)', lg: 'auto' },
      }}>
        <Box sx={{ minHeight: 0, display: { xs: 'none', lg: 'block' } }}>
          <WorkspaceSidebar
            project={project}
            workspace={workspace}
            files={files}
            filesLoading={filesLoading}
            filesError={filesError}
            selectedFile={selectedFile}
            onSelectFile={onSelectFile}
            gateOrder={GATE_ORDER}
            patches={patches}
            patchesLoading={patchesLoading}
            patchesError={patchesError}
            onSelectPatch={onSelectPatch}
            onRetryFiles={() => workspace && loadFiles(workspace)}
            onRetryPatches={loadPatches}
          />
        </Box>

        <Box sx={{
          minHeight: 0,
          borderRadius: '14px',
          border: '1px solid rgba(17,17,17,0.12)',
          bgcolor: tokens.color.bg.surface,
          overflow: 'hidden',
          display: 'flex',
          flexDirection: 'column',
        }}>
          <Tabs
            value={tab}
            onChange={(_, v) => setTab(v as CenterTab)}
            sx={{
              px: 1.4,
              minHeight: 46,
              borderBottom: `1px solid ${tokens.color.border.subtle}`,
              '& .MuiTab-root': {
                minHeight: 46, fontSize: 13, fontWeight: 700,
                color: tokens.color.text.muted,
              },
              '& .Mui-selected': { color: tokens.color.text.inverse },
              '& .MuiTabs-indicator': { bgcolor: tokens.color.accent.lime, height: 2.5 },
            }}
          >
            <Tab icon={<ChatIcon fontSize="small" />} iconPosition="start" value="chat"     label="Chat" />
            <Tab icon={<CodeIcon fontSize="small" />} iconPosition="start" value="editor"   label="Editor" />
            <Tab icon={<Visibility fontSize="small" />} iconPosition="start" value="preview" label="Preview" />
            <Tab icon={<TerminalIcon fontSize="small" />} iconPosition="start" value="terminal" label="Terminal" />
          </Tabs>

          <Box sx={{ flex: 1, minHeight: 0, p: 1.4 }}>
            {tab === 'chat'     && <ChatPane projectId={id} />}
            {tab === 'editor'   && <EditorPane workspace={workspace} selectedFile={selectedFile} />}
            {tab === 'preview'  && (
              <PreviewPane
                workspace={workspace}
                refreshKey={runEvents.length}
                projectId={id}
                onPatchProposed={(p) => {
                  setPatches((curr) => [p, ...curr.filter((c) => c.id !== p.id)]);
                  setActivePatch(p);
                }}
              />
            )}
            {tab === 'terminal' && (
              <Box sx={{ height: '100%', minHeight: 320 }}>
                <Terminal workspaceId={workspace?.id ?? null} />
              </Box>
            )}
          </Box>
        </Box>

        <Box sx={{ minHeight: 0, display: { xs: 'none', lg: 'block' } }}>
          <RunPanel
            events={runEvents}
            running={running}
            streamHealthy={streamHealthy}
            onRun={runFinisher}
            onRepair={onRepair}
          />
        </Box>
      </Box>

      <PatchDrawer
        open={Boolean(activePatch)}
        patch={activePatch}
        applying={applyingPatch}
        rollingBack={rollingBack}
        onClose={() => setActivePatch(null)}
        onApply={onApplyPatch}
        onRollback={onRollbackPatch}
        previousFiles={Object.fromEntries(
          (project?.files ?? [])
            .filter((f) => typeof f.content === 'string')
            .map((f) => [f.path, f.content as string]),
        )}
      />
    </Box>
  );
}

function ProjectHeader({ p, running }: { p: Project; running: boolean }) {
  const passed = useMemo(
    () => Object.values(p.gates).filter((g) => g.status === 'passed').length,
    [p.gates],
  );
  const total = Object.keys(p.gates).length || GATE_ORDER.length;
  return (
    <Box sx={{
      px: 1.6, py: 0.8, minHeight: 58,
      borderBottom: '1px solid rgba(17,17,17,0.12)',
      display: 'flex', alignItems: 'center', justifyContent: 'space-between',
      gap: 2,
      bgcolor: 'rgba(248,244,236,0.94)',
      color: tokens.color.text.inverse,
    }}>
      <Stack direction="row" alignItems="center" spacing={1.4} sx={{ minWidth: 0 }}>
        <IconButton size="small" href="/app" sx={{ color: '#4a453e' }}>
          <KeyboardBackspace fontSize="small" />
        </IconButton>
        <Box sx={{ minWidth: 0 }}>
          <Typography variant="subtitle1" sx={{ fontWeight: 900 }} noWrap>{p.name}</Typography>
          <Typography variant="caption" sx={{ color: '#686158' }} noWrap>
            {p.spec.idea || p.description}
          </Typography>
        </Box>
      </Stack>
      <Stack direction="row" alignItems="center" spacing={1}>
        <Chip
          label={`Gates ${passed}/${total}`}
          size="small"
          sx={{
            bgcolor: '#fffaf1', color: tokens.color.text.inverse,
            border: '1px solid rgba(17,17,17,0.12)', fontWeight: 800,
          }}
        />
        {running && (
          <Chip
            label="Running"
            size="small"
            sx={{
              bgcolor: tokens.color.accent.lime,
              color: tokens.color.text.inverse,
              fontWeight: 800,
            }}
          />
        )}
      </Stack>
    </Box>
  );
}
