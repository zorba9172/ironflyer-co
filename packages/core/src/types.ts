import { z } from 'zod';

// Minimal shared domain shapes. Expand alongside the orchestrator schema;
// the SDK remains the source of truth for wire types.

export const FinisherGateState = z.enum([
  'unstarted',
  'in_progress',
  'blocked',
  'closed',
]);
export type FinisherGateState = z.infer<typeof FinisherGateState>;

export const ProjectSummary = z.object({
  id: z.string(),
  name: z.string(),
  ownerId: z.string(),
  isPublic: z.boolean().default(false),
  openGates: z.number().int().nonnegative().default(0),
  updatedAt: z.string(),
});
export type ProjectSummary = z.infer<typeof ProjectSummary>;
