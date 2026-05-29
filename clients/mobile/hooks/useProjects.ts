// Live project data with graceful sample-data fallback. Screens consume this
// hook instead of importing sampleData directly, so the app shows real
// orchestrator state when reachable and still renders fully offline.

import { useEffect, useState } from 'react';
import { fetchProjects } from '../lib/api';
import { projects as sampleProjects, type Project } from '../lib/sampleData';

export type ProjectsResult = {
  data: Project[];
  loading: boolean;
  // True once a live orchestrator response populated the list; false while
  // showing sample data (offline / error / empty).
  isLive: boolean;
  error?: string;
};

export function useProjects(): ProjectsResult {
  const [data, setData] = useState<Project[]>(sampleProjects);
  const [loading, setLoading] = useState(true);
  const [isLive, setIsLive] = useState(false);
  const [error, setError] = useState<string | undefined>(undefined);

  useEffect(() => {
    const controller = new AbortController();
    let active = true;

    fetchProjects(controller.signal)
      .then((live) => {
        if (!active) return;
        // Empty live result still falls back to sample so the screens are
        // never blank in a fresh/demo environment.
        if (live.length > 0) {
          setData(live);
          setIsLive(true);
          setError(undefined);
        }
      })
      .catch((e: unknown) => {
        if (!active) return;
        setError(e instanceof Error ? e.message : 'failed to load projects');
      })
      .finally(() => {
        if (active) setLoading(false);
      });

    return () => {
      active = false;
      controller.abort();
    };
  }, []);

  return { data, loading, isLive, error };
}

// Single-project view derived from the same fetch, so the detail screen shares
// the live/fallback behavior without a second round-trip shape.
export function useProject(id: string | undefined): {
  project: Project | undefined;
  loading: boolean;
  isLive: boolean;
} {
  const { data, loading, isLive } = useProjects();
  const project = id ? data.find((p) => p.id === id) : undefined;
  return { project, loading, isLive };
}
