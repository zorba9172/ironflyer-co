// Centralized query keys so web + native cache identically.
export const queryKeys = {
  projects: () => ['projects'] as const,
  project: (id: string) => ['projects', id] as const,
  gates: (projectId: string) => ['projects', projectId, 'gates'] as const,
  wallet: () => ['wallet'] as const,
} as const;
