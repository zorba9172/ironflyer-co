import { useEffect, useState } from 'react';
import { useRequest, operations } from '@ironflyer/data';

// Resolves the first real project id from the orchestrator when connected,
// else null (offline / not signed in). Used so chat + dashboard target a real
// project instead of the mock fixture id.
export function useLiveProjectId(): string | null {
  const request = useRequest();
  const [id, setId] = useState<string | null>(null);
  useEffect(() => {
    if (!request) return;
    request<{ projects: { id: string }[] }>('Projects', operations.PROJECTS)
      .then((d) => setId(d.projects?.[0]?.id ?? null))
      .catch(() => setId(null));
  }, [request]);
  return id;
}
