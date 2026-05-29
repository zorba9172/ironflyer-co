import { useCallback, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useRequest, operations } from '@ironflyer/data';
import { useStudio } from '../store';

interface SaveResult { ok: boolean; projectId?: string; error?: string }

// Persists the session's generated files to the backend via writeProjectFiles
// (the operator write-back path). Creates a real project first if this session
// has never been saved. Returns the project id so callers can route/refresh.
export function useSaveProject() {
  const request = useRequest();
  const qc = useQueryClient();
  const [saving, setSaving] = useState(false);

  const save = useCallback(async (): Promise<SaveResult> => {
    const { liveProjectId, generatedFiles, current, setLiveProjectId, markSaved } = useStudio.getState();
    if (!request) return { ok: false, error: 'offline: connect the orchestrator to save' };
    if (generatedFiles.length === 0) return { ok: false, error: 'nothing to save yet' };

    setSaving(true);
    try {
      let projectId = liveProjectId;
      if (!projectId) {
        const created = await request<{ createProject: { id: string } }>('CreateProject', operations.CREATE_PROJECT, {
          input: { name: current.name || 'Untitled project', idea: useStudio.getState().constitution || undefined },
        });
        projectId = created.createProject.id;
        setLiveProjectId(projectId);
      }
      await request<{ writeProjectFiles: unknown[] }>('WriteProjectFiles', operations.WRITE_PROJECT_FILES, {
        id: projectId,
        files: generatedFiles.map((f) => ({ path: f.path, content: f.content })),
      });
      markSaved();
      // Invalidate so the projects list + this project's files re-fetch.
      void qc.invalidateQueries({ queryKey: ['projects'] });
      void qc.invalidateQueries({ queryKey: ['code-files', projectId] });
      return { ok: true, projectId };
    } catch (e) {
      return { ok: false, error: e instanceof Error ? e.message : 'save failed' };
    } finally {
      setSaving(false);
    }
  }, [request, qc]);

  return { save, saving };
}
