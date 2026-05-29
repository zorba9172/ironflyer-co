import { useLiveProjectId } from './useLiveProjectId';
import { useStudio } from '../store';

// The live backend project id every Operate surface scopes to: the project the
// session has open, falling back to the tenant's first live project.
export function useOperateProjectId(): string | null {
  const storeProjectId = useStudio((s) => s.liveProjectId);
  const firstProjectId = useLiveProjectId();
  return storeProjectId ?? firstProjectId;
}
