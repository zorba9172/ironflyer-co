// Shared types for the Studio split-view (Agent 48, extended by Agent 58).
//
// A "studio message" is the unit rendered inside ChatPanel. It can be
// authored by the human ("user"), produced by the model ("assistant"),
// emitted by the orchestrator's execution feed ("system"), surfaced as
// a recoverable error ("error"), condensed as a single cost tick chip
// ("costtick"), or — new in A58 — one of four agent-reasoning variants
// that render as live, in-progress bubbles:
//
//   agent_progress — a stage just started ("Starting code review")
//   agent_action   — the stage is doing work ("Calling claude-sonnet…")
//   agent_result   — an action finished; success/failure with summary
//   refinement_ack — orchestrator confirmed it picked up a studio.refine
//
// The buffer is persisted to localStorage keyed by executionID so a page
// refresh never loses chat history.

// StudioTabKey controls the right-hand workspace mode. `workbench` is
// the locked VS Code Cloud target view — three concurrent columns
// (Prompt | Code | Preview) with the Studio Assistant strip below.
// The other modes are full-bleed alternatives for when the operator
// needs a single surface to dominate the workbench.
export type StudioTabKey =
  | "workbench"
  | "preview"
  | "dashboard"
  | "files"
  | "code"
  | "patches";

// Variant is the discriminator for MessageBubble. We keep the legacy
// field name `role` for backward compatibility with DashboardPane and
// chatStorage validation; semantically it is the "variant" union.
export type StudioMessageRole =
  | "user"
  | "assistant"
  | "system"
  | "error"
  | "costtick"
  | "agent_progress"
  | "agent_action"
  | "agent_result"
  | "refinement_ack";

export interface StudioAttachment {
  id: string;
  name: string;
  type: string;
  size: number;
  kind: "image" | "document" | "code" | "data";
  previewUrl?: string;
}

export interface StudioMessage {
  // Stable id — falls back to a UUID-ish random when the source event
  // does not provide one. Used as the React key in MessageList.
  id: string;
  role: StudioMessageRole;
  // The visible body. For assistant/system this is the human-readable
  // summary; for costtick this is the amount string; for agent_* this
  // is the short verb ("Calling claude-sonnet for code").
  body: string;
  // ISO timestamp; we render relativeTime() next to the bubble.
  createdAt: string;
  // Optional "thought for X" pre-amble — assistant only.
  thinking?: string;
  // Free-form metadata so the bubble can decide presentation. For
  // costtick we carry { amountUSD, ledgerEntryID? }. For system we
  // carry { eventType } so the dashboard can cross-reference.
  meta?: Record<string, unknown>;
  attachments?: StudioAttachment[];
  // ── Agent reasoning fields (A58) ───────────────────────────────────
  // Lifecycle stage the agent is operating on: spec | ux | arch | code
  // | test | security | deploy | studio.
  stage?: string;
  // The agent role inside the stage: planner | coder | reviewer | …
  agentRole?: string;
  // Action verb for agent_action: model_call | patch_propose |
  // patch_apply | repair_lookup | repair_apply | refinement_consumed |
  // still_working.
  action?: string;
  // True while the action is in flight; flips false when the matching
  // agent_result lands.
  inProgress?: boolean;
  // For agent_result: did the action succeed?
  success?: boolean;
  // Longer body of an agent_result (collapsible in the bubble).
  summary?: string;
  // Materialised cost in USD (mirrors costtick.meta.amountUSD).
  costUSD?: number;
  // Alias for `thinking` to match the A55 spec; reserved for future
  // streaming token deltas.
  thoughts?: string;
  // Duration in ms for agent.stage.completed.v1.
  durationMs?: number;
}

// The execution event vocabulary the studio reacts to. The
// orchestrator's source-of-truth lives in
// core/orchestrator/internal/execution/events.go — keep the strings in
// sync if new events are added.
export const EVENT_TYPES = {
  GateVerdict: "gate.verdict.v1",
  PatchApplied: "patch.applied.v1",
  RecoveryRecipeHit: "recovery.recipe_hit.v1",
  ProfitGuardStop: "profitguard_stop",
  ExecutionSettled: "execution.settled.v1",
  CostAdded: "cost_added",
  Started: "started",
  Admitted: "admitted",
  ScoreUpdated: "score.updated.v1",
  StudioRefine: "studio.refine.v1",
  AgentStageStarted: "agent.stage.started.v1",
  AgentStageAction: "agent.stage.action.v1",
  AgentStageResult: "agent.stage.result.v1",
  AgentStageCompleted: "agent.stage.completed.v1",
  StudioRefineConsumed: "studio.refine.consumed.v1",
} as const;

// Convenience aliases — match the A55 wire constant names exactly so
// downstream wiring can `import { EVENT_AGENT_STAGE_STARTED } from
// "./types"` without going through the EVENT_TYPES object.
export const EVENT_AGENT_STAGE_STARTED = EVENT_TYPES.AgentStageStarted;
export const EVENT_AGENT_STAGE_ACTION = EVENT_TYPES.AgentStageAction;
export const EVENT_AGENT_STAGE_RESULT = EVENT_TYPES.AgentStageResult;
export const EVENT_AGENT_STAGE_COMPLETED = EVENT_TYPES.AgentStageCompleted;
export const EVENT_STUDIO_REFINE_CONSUMED = EVENT_TYPES.StudioRefineConsumed;
