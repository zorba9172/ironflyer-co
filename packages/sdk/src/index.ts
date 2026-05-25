/**
 * @ironflyer/sdk — official TypeScript client for the Ironflyer
 * orchestrator GraphQL API.
 *
 * Public surface:
 *
 *   - `Ironflyer` — the class clients construct.
 *   - `IronflyerError` — thrown on any HTTP or WS failure.
 *   - `IronflyerOptions` — constructor options shape.
 *   - Re-exported codegen types (inputs, enums, payload shapes) so
 *     consumers don't have to dig into `@ironflyer/sdk/gql`.
 */

export { Ironflyer, IronflyerError, type IronflyerOptions, type WebSocketImpl } from './client.js';

// Re-export the codegen-generated types so consumers can name them
// without reaching past the package root.
export type {
  // Auth
  User,
  Session,
  SignInInput,
  SignUpInput,

  // Projects
  Project,
  ProjectFile,
  ProjectGraph,
  ProjectGraphNode,
  ProjectGraphEdge,
  CodeSearchHit,
  CreateProjectInput,
  UpdateProjectInput,

  // Gates + finisher
  GateVerdict,
  GateIssue,
  GateStatus,
  RerunGateInput,

  // Patches
  Patch,
  PatchChange,
  PatchChangeOp,
  PatchStatus,
  ProposePatchInput,
  PatchChangeInput,

  // Budget
  Plan,
  Rate,
  BudgetSummary,
  LedgerEntry,
  VaultSnapshot,
  StartCheckoutInput,
  CancelSubscriptionInput,
  StripeCheckoutSession,
  Subscription_Stripe as StripeSubscription,

  // Memory
  MemoryRecord,
  MemoryKind,
  MemoryQueryInput,
  AddMemoryInput,

  // Audit
  AuditEntry,
  AuditOutcome,
  AuditQueryInput,
  AuditVerifyResult,

  // Webhooks
  WebhookSubscription,
  WebhookDeliveryStatus,
  CreateWebhookInput,
  WebhookTestResult,

  // Deploys
  Deploy,
  DeployStatus,
  DeployTarget,
  StartDeployInput,
  DeployEnvVar,
  VercelEnvTarget,

  // Chats
  Chat,
  ChatMessage,
  ChatAttachment,
  ChatAttachmentInput,
  CreateChatInput,
  ForkChatInput,
  ChatInput,
  ChatDelta,
  ChatStartDelta,
  ChatTextDelta,
  ChatThinkingDelta,
  ChatToolUseDelta,
  ChatDoneDelta,
  ChatErrorDelta,

  // Inline completion
  InlineInput,
  InlineDelta,
  InlineStartDelta,
  InlineTextDelta,
  InlineDoneDelta,
  InlineCancelledDelta,
  InlineErrorDelta,

  // Workspaces
  Workspace,
  WorkspaceFile,
  WorkspaceFileContent,
  ExecResult,
  PtyEvent,
  PtyOutput,
  PtyExit,

  // Run events
  RunEvent,
  RunExecutionEvent,
  RunGateEvent,
  RunDoneEvent,
  RunErrorEvent,

  // Cost delta
  CostDelta,

  // Common
  OperationResult,
} from './gql/index.js';
