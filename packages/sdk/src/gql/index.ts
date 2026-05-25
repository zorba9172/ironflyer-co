import type { GraphQLClient, RequestOptions } from 'graphql-request';
import gql from 'graphql-tag';
export type Maybe<T> = T | null;
export type InputMaybe<T> = T | null;
export type Exact<T extends { [key: string]: unknown }> = { [K in keyof T]: T[K] };
export type MakeOptional<T, K extends keyof T> = Omit<T, K> & { [SubKey in K]?: Maybe<T[SubKey]> };
export type MakeMaybe<T, K extends keyof T> = Omit<T, K> & { [SubKey in K]: Maybe<T[SubKey]> };
export type MakeEmpty<T extends { [key: string]: unknown }, K extends keyof T> = { [_ in K]?: never };
export type Incremental<T> = T | { [P in keyof T]?: P extends ' $fragmentName' | '__typename' ? T[P] : never };
type GraphQLClientRequestHeaders = RequestOptions['requestHeaders'];
/** All built-in and custom scalars, mapped to their actual values */
export type Scalars = {
  ID: { input: string; output: string; }
  String: { input: string; output: string; }
  Boolean: { input: boolean; output: boolean; }
  Int: { input: number; output: number; }
  Float: { input: number; output: number; }
  Bytes: { input: any; output: any; }
  DateTime: { input: string; output: string; }
  Decimal: { input: string; output: string; }
  JSON: { input: unknown; output: unknown; }
};

export type AddIpAllowlistInput = {
  cidr: Scalars['String']['input'];
  note?: InputMaybe<Scalars['String']['input']>;
};

export type AddMemoryInput = {
  body: Scalars['String']['input'];
  gateName?: InputMaybe<Scalars['String']['input']>;
  kind: MemoryKind;
  projectId?: InputMaybe<Scalars['ID']['input']>;
  storyId?: InputMaybe<Scalars['ID']['input']>;
  tags?: InputMaybe<Array<Scalars['String']['input']>>;
  title?: InputMaybe<Scalars['String']['input']>;
};

export type AddSubprojectInput = {
  name: Scalars['String']['input'];
  path: Scalars['String']['input'];
  role?: InputMaybe<Scalars['String']['input']>;
  stack?: InputMaybe<Scalars['JSON']['input']>;
};

export type AddVisualTargetInput = {
  imagePngBase64: Scalars['String']['input'];
  name: Scalars['String']['input'];
  routeHint?: InputMaybe<Scalars['String']['input']>;
  tolerance?: InputMaybe<Scalars['Float']['input']>;
  viewportH?: InputMaybe<Scalars['Int']['input']>;
  viewportW?: InputMaybe<Scalars['Int']['input']>;
};

export type AdminRefundInput = {
  amountCents?: InputMaybe<Scalars['Int']['input']>;
  chargeId: Scalars['ID']['input'];
  reason?: InputMaybe<Scalars['String']['input']>;
  userId: Scalars['ID']['input'];
};

export type Affiliate = {
  __typename?: 'Affiliate';
  code: Scalars['String']['output'];
  createdAt: Scalars['DateTime']['output'];
  id: Scalars['ID']['output'];
  payoutRate: Scalars['Float']['output'];
  userId: Scalars['ID']['output'];
};

export type AffiliatePayout = {
  __typename?: 'AffiliatePayout';
  affiliateId: Scalars['ID']['output'];
  amountUsd: Scalars['Decimal']['output'];
  id: Scalars['ID']['output'];
  note?: Maybe<Scalars['String']['output']>;
  status: Scalars['String']['output'];
  ts: Scalars['DateTime']['output'];
};

export type AffiliateSignupInput = {
  payoutMethod?: InputMaybe<Scalars['String']['input']>;
  paypalEmail?: InputMaybe<Scalars['String']['input']>;
};

export type AffiliateStats = {
  __typename?: 'AffiliateStats';
  affiliate: Affiliate;
  lifetimeEarningsUsd: Scalars['Decimal']['output'];
  paidReferralCount: Scalars['Int']['output'];
  pendingPayoutUsd: Scalars['Decimal']['output'];
  referralCount: Scalars['Int']['output'];
};

export type Agent = {
  __typename?: 'Agent';
  capabilities: Array<Scalars['String']['output']>;
  enableThinking: Scalars['Boolean']['output'];
  role: Scalars['String']['output'];
  system?: Maybe<Scalars['String']['output']>;
};

export type AgentCall = {
  __typename?: 'AgentCall';
  cacheReadTokens: Scalars['Int']['output'];
  cacheWriteTokens: Scalars['Int']['output'];
  capabilities?: Maybe<Array<Scalars['String']['output']>>;
  completionTokens: Scalars['Int']['output'];
  costUsd: Scalars['Decimal']['output'];
  durationMs: Scalars['Int']['output'];
  error?: Maybe<Scalars['String']['output']>;
  id: Scalars['ID']['output'];
  model?: Maybe<Scalars['String']['output']>;
  projectId?: Maybe<Scalars['ID']['output']>;
  promptTokens: Scalars['Int']['output'];
  provider: Scalars['String']['output'];
  role?: Maybe<Scalars['String']['output']>;
  ts: Scalars['DateTime']['output'];
  userId?: Maybe<Scalars['ID']['output']>;
};

export type AppendChatMessageInput = {
  attachments?: InputMaybe<Array<ChatAttachmentInput>>;
  chatId: Scalars['ID']['input'];
  role: Scalars['String']['input'];
  text?: InputMaybe<Scalars['String']['input']>;
  toolUse?: InputMaybe<Scalars['JSON']['input']>;
};

export type AuditEntry = {
  __typename?: 'AuditEntry';
  action: Scalars['String']['output'];
  actor?: Maybe<Scalars['String']['output']>;
  agentRole?: Maybe<Scalars['String']['output']>;
  gateName?: Maybe<Scalars['String']['output']>;
  hash: Scalars['String']['output'];
  id: Scalars['ID']['output'];
  inputHash?: Maybe<Scalars['String']['output']>;
  ok: Scalars['Boolean']['output'];
  outcome: AuditOutcome;
  outputHash?: Maybe<Scalars['String']['output']>;
  payload?: Maybe<Scalars['JSON']['output']>;
  prevHash?: Maybe<Scalars['String']['output']>;
  projectId?: Maybe<Scalars['ID']['output']>;
  resource?: Maybe<Scalars['String']['output']>;
  storyId?: Maybe<Scalars['String']['output']>;
  summary?: Maybe<Scalars['String']['output']>;
  ts: Scalars['DateTime']['output'];
  userId?: Maybe<Scalars['ID']['output']>;
};

export type AuditOutcome =
  | 'BLOCKED'
  | 'FAILURE'
  | 'SKIPPED'
  | 'SUCCESS';

export type AuditQueryInput = {
  action?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  outcome?: InputMaybe<AuditOutcome>;
  projectId?: InputMaybe<Scalars['ID']['input']>;
  since?: InputMaybe<Scalars['DateTime']['input']>;
  until?: InputMaybe<Scalars['DateTime']['input']>;
  userId?: InputMaybe<Scalars['ID']['input']>;
};

export type AuditVerifyResult = {
  __typename?: 'AuditVerifyResult';
  firstBadIndex: Scalars['Int']['output'];
  intact: Scalars['Boolean']['output'];
};

export type BanditCapability = {
  __typename?: 'BanditCapability';
  capability: Scalars['String']['output'];
  total: Scalars['Int']['output'];
  winners: Array<BanditWinner>;
};

export type BanditRanking = {
  __typename?: 'BanditRanking';
  capabilities: Array<BanditCapability>;
  lastConfidence?: Maybe<Scalars['Float']['output']>;
  lookback: Scalars['Int']['output'];
  sampled: Scalars['Int']['output'];
  strategy: Scalars['String']['output'];
};

export type BanditWinner = {
  __typename?: 'BanditWinner';
  avgCostUsd: Scalars['Float']['output'];
  calls: Scalars['Int']['output'];
  errors: Scalars['Int']['output'];
  isLeader: Scalars['Boolean']['output'];
  lastUsed?: Maybe<Scalars['DateTime']['output']>;
  meanReward: Scalars['Float']['output'];
  model?: Maybe<Scalars['String']['output']>;
  provider: Scalars['String']['output'];
  share: Scalars['Float']['output'];
};

export type BillingPortalSession = {
  __typename?: 'BillingPortalSession';
  expiresAt: Scalars['DateTime']['output'];
  url: Scalars['String']['output'];
};

export type BudgetSummary = {
  __typename?: 'BudgetSummary';
  email: Scalars['String']['output'];
  entries: Array<LedgerEntry>;
  spentUsd: Scalars['Decimal']['output'];
  tier: Scalars['String']['output'];
  userId: Scalars['ID']['output'];
};

export type CancelSubscriptionInput = {
  atPeriodEnd: Scalars['Boolean']['input'];
};

export type Chat = {
  __typename?: 'Chat';
  createdAt: Scalars['DateTime']['output'];
  id: Scalars['ID']['output'];
  messageCount: Scalars['Int']['output'];
  parentChatId?: Maybe<Scalars['ID']['output']>;
  projectId: Scalars['ID']['output'];
  title?: Maybe<Scalars['String']['output']>;
  updatedAt: Scalars['DateTime']['output'];
};

export type ChatAttachment = {
  __typename?: 'ChatAttachment';
  base64: Scalars['String']['output'];
  mediaType: Scalars['String']['output'];
};

export type ChatAttachmentInput = {
  base64: Scalars['String']['input'];
  mediaType: Scalars['String']['input'];
};

export type ChatDelta = ChatDoneDelta | ChatErrorDelta | ChatStartDelta | ChatTextDelta | ChatThinkingDelta | ChatToolUseDelta;

export type ChatDoneDelta = {
  __typename?: 'ChatDoneDelta';
  model?: Maybe<Scalars['String']['output']>;
  provider?: Maybe<Scalars['String']['output']>;
  turnId: Scalars['ID']['output'];
  usage?: Maybe<Scalars['JSON']['output']>;
};

export type ChatErrorDelta = {
  __typename?: 'ChatErrorDelta';
  code: Scalars['String']['output'];
  message: Scalars['String']['output'];
  turnId?: Maybe<Scalars['ID']['output']>;
};

export type ChatInput = {
  attachments?: InputMaybe<Array<ChatAttachmentInput>>;
  chatId?: InputMaybe<Scalars['ID']['input']>;
  effort?: InputMaybe<Scalars['String']['input']>;
  prompt: Scalars['String']['input'];
  role?: InputMaybe<Scalars['String']['input']>;
};

export type ChatMessage = {
  __typename?: 'ChatMessage';
  attachments?: Maybe<Array<ChatAttachment>>;
  chatId: Scalars['ID']['output'];
  createdAt: Scalars['DateTime']['output'];
  id: Scalars['ID']['output'];
  model?: Maybe<Scalars['String']['output']>;
  provider?: Maybe<Scalars['String']['output']>;
  role: Scalars['String']['output'];
  text?: Maybe<Scalars['String']['output']>;
  toolUse?: Maybe<Scalars['JSON']['output']>;
  usage?: Maybe<Scalars['JSON']['output']>;
};

export type ChatStartDelta = {
  __typename?: 'ChatStartDelta';
  model: Scalars['String']['output'];
  provider: Scalars['String']['output'];
  turnId: Scalars['ID']['output'];
};

export type ChatTextDelta = {
  __typename?: 'ChatTextDelta';
  text: Scalars['String']['output'];
  turnId: Scalars['ID']['output'];
};

export type ChatThinkingDelta = {
  __typename?: 'ChatThinkingDelta';
  text: Scalars['String']['output'];
  turnId: Scalars['ID']['output'];
};

export type ChatToolUseDelta = {
  __typename?: 'ChatToolUseDelta';
  toolUse: Scalars['JSON']['output'];
  turnId: Scalars['ID']['output'];
};

export type CloneIntoWorkspaceInput = {
  ref?: InputMaybe<Scalars['String']['input']>;
  subdir?: InputMaybe<Scalars['String']['input']>;
  workspaceId: Scalars['ID']['input'];
};

export type CloneIntoWorkspaceResult = {
  __typename?: 'CloneIntoWorkspaceResult';
  ok: Scalars['Boolean']['output'];
  payload?: Maybe<Scalars['JSON']['output']>;
  status: Scalars['Int']['output'];
};

export type CodeSearchHit = {
  __typename?: 'CodeSearchHit';
  endLine: Scalars['Int']['output'];
  path: Scalars['String']['output'];
  score: Scalars['Float']['output'];
  startLine: Scalars['Int']['output'];
  symbols?: Maybe<Array<Scalars['String']['output']>>;
  text: Scalars['String']['output'];
};

export type CollabChatMessage = {
  __typename?: 'CollabChatMessage';
  email?: Maybe<Scalars['String']['output']>;
  id: Scalars['ID']['output'];
  projectId: Scalars['ID']['output'];
  text: Scalars['String']['output'];
  ts: Scalars['DateTime']['output'];
  userId: Scalars['ID']['output'];
};

export type Collaborator = {
  __typename?: 'Collaborator';
  acceptedAt?: Maybe<Scalars['DateTime']['output']>;
  email: Scalars['String']['output'];
  id: Scalars['ID']['output'];
  invitedAt: Scalars['DateTime']['output'];
  projectId: Scalars['ID']['output'];
  role: Scalars['String']['output'];
  userId?: Maybe<Scalars['ID']['output']>;
};

export type ConnectGithubInput = {
  defaultBranch?: InputMaybe<Scalars['String']['input']>;
  fullName?: InputMaybe<Scalars['String']['input']>;
  htmlUrl?: InputMaybe<Scalars['String']['input']>;
  owner?: InputMaybe<Scalars['String']['input']>;
  repo?: InputMaybe<Scalars['String']['input']>;
};

export type CostDelta = {
  __typename?: 'CostDelta';
  agent?: Maybe<Scalars['String']['output']>;
  durationMs?: Maybe<Scalars['Int']['output']>;
  model?: Maybe<Scalars['String']['output']>;
  provider?: Maybe<Scalars['String']['output']>;
  ts: Scalars['DateTime']['output'];
  usdSpent: Scalars['Decimal']['output'];
};

export type CreateChatInput = {
  parentChatId?: InputMaybe<Scalars['ID']['input']>;
  projectId: Scalars['ID']['input'];
  title?: InputMaybe<Scalars['String']['input']>;
};

export type CreateCustomDomainInput = {
  hostname: Scalars['String']['input'];
  projectId: Scalars['ID']['input'];
};

export type CreateProjectInput = {
  description?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['ID']['input']>;
  idea?: InputMaybe<Scalars['String']['input']>;
  name: Scalars['String']['input'];
};

export type CreateShareLinkInput = {
  projectId: Scalars['ID']['input'];
  ttlSeconds?: InputMaybe<Scalars['Int']['input']>;
};

export type CreateStageInput = {
  description?: InputMaybe<Scalars['String']['input']>;
  name: Scalars['String']['input'];
  patchIds: Array<Scalars['ID']['input']>;
  projectId: Scalars['ID']['input'];
};

export type CreateWebhookInput = {
  events: Array<Scalars['String']['input']>;
  secret?: InputMaybe<Scalars['String']['input']>;
  url: Scalars['String']['input'];
};

export type CursorClearedEvent = {
  __typename?: 'CursorClearedEvent';
  path: Scalars['String']['output'];
  userId: Scalars['ID']['output'];
};

export type CursorEvent = CursorClearedEvent | CursorMovedEvent;

export type CursorMovedEvent = {
  __typename?: 'CursorMovedEvent';
  position: CursorPosition;
};

export type CursorPosition = {
  __typename?: 'CursorPosition';
  column: Scalars['Int']['output'];
  line: Scalars['Int']['output'];
  path: Scalars['String']['output'];
  selection?: Maybe<Scalars['JSON']['output']>;
  userId: Scalars['ID']['output'];
};

export type CustomDomain = {
  __typename?: 'CustomDomain';
  createdAt: Scalars['DateTime']['output'];
  hostname: Scalars['String']['output'];
  id: Scalars['ID']['output'];
  projectId: Scalars['ID']['output'];
  status: CustomDomainStatus;
  verifiedAt?: Maybe<Scalars['DateTime']['output']>;
  verifyTxt?: Maybe<Scalars['String']['output']>;
};

export type CustomDomainStatus =
  | 'FAILED'
  | 'PENDING'
  | 'REVOKED'
  | 'VERIFIED';

export type Deploy = {
  __typename?: 'Deploy';
  artifact?: Maybe<Scalars['JSON']['output']>;
  durationMs?: Maybe<Scalars['Int']['output']>;
  finishedAt?: Maybe<Scalars['DateTime']['output']>;
  id: Scalars['ID']['output'];
  log: Array<DeployLogLine>;
  projectId: Scalars['ID']['output'];
  startedAt: Scalars['DateTime']['output'];
  status: DeployStatus;
  target: DeployTarget;
  targetMeta?: Maybe<Scalars['JSON']['output']>;
  url?: Maybe<Scalars['String']['output']>;
};

export type DeployBuildLogLine = {
  __typename?: 'DeployBuildLogLine';
  line: Scalars['String']['output'];
  source: Scalars['String']['output'];
  ts: Scalars['DateTime']['output'];
};

export type DeployEnvVar = {
  key: Scalars['String']['input'];
  target?: InputMaybe<VercelEnvTarget>;
  value: Scalars['String']['input'];
};

export type DeployErrorEvent = {
  __typename?: 'DeployErrorEvent';
  code: Scalars['String']['output'];
  deployId: Scalars['ID']['output'];
  message: Scalars['String']['output'];
};

export type DeployEvent = DeployBuildLogLine | DeployErrorEvent | DeployFinishedEvent | DeployLogEvent | DeployStateEvent;

export type DeployFinishedEvent = {
  __typename?: 'DeployFinishedEvent';
  deployId: Scalars['ID']['output'];
  durationMs?: Maybe<Scalars['Int']['output']>;
  status: DeployStatus;
  url?: Maybe<Scalars['String']['output']>;
};

export type DeployLogEvent = {
  __typename?: 'DeployLogEvent';
  deployId: Scalars['ID']['output'];
  line: DeployLogLine;
};

export type DeployLogLine = {
  __typename?: 'DeployLogLine';
  level: Scalars['String']['output'];
  message: Scalars['String']['output'];
  ts: Scalars['DateTime']['output'];
};

export type DeployStateEvent = {
  __typename?: 'DeployStateEvent';
  deployId: Scalars['ID']['output'];
  status: DeployStatus;
  ts: Scalars['DateTime']['output'];
};

export type DeployStatus =
  | 'BUILDING'
  | 'CANCELLED'
  | 'FAILED'
  | 'PENDING'
  | 'PLANNING'
  | 'SHIPPING'
  | 'SUCCESS'
  | 'TESTING';

export type DeployTarget =
  | 'RUNTIME'
  | 'VERCEL';

export type DunningState =
  | 'GIVING_UP'
  | 'NONE'
  | 'PAUSED'
  | 'RETRY_1'
  | 'RETRY_2';

export type EmailChangeInput = {
  currentPassword: Scalars['String']['input'];
  newEmail: Scalars['String']['input'];
};

export type EnterpriseLeadInput = {
  company?: InputMaybe<Scalars['String']['input']>;
  email: Scalars['String']['input'];
  message?: InputMaybe<Scalars['String']['input']>;
  source?: InputMaybe<Scalars['String']['input']>;
};

export type ExecResult = {
  __typename?: 'ExecResult';
  durMs: Scalars['Int']['output'];
  exitCode: Scalars['Int']['output'];
  stderr: Scalars['String']['output'];
  stdout: Scalars['String']['output'];
  timedOut: Scalars['Boolean']['output'];
};

export type ExportGithubInput = {
  description?: InputMaybe<Scalars['String']['input']>;
  owner: Scalars['String']['input'];
  private?: InputMaybe<Scalars['Boolean']['input']>;
  projectId: Scalars['ID']['input'];
  repo: Scalars['String']['input'];
};

export type FigmaImport = {
  __typename?: 'FigmaImport';
  createdAt: Scalars['DateTime']['output'];
  fileKey: Scalars['String']['output'];
  finishedAt?: Maybe<Scalars['DateTime']['output']>;
  id: Scalars['ID']['output'];
  manifest?: Maybe<Scalars['JSON']['output']>;
  message?: Maybe<Scalars['String']['output']>;
  projectId: Scalars['ID']['output'];
  status: FigmaImportStatusKind;
  workspaceId: Scalars['ID']['output'];
};

export type FigmaImportDoneEvent = {
  __typename?: 'FigmaImportDoneEvent';
  importId: Scalars['ID']['output'];
  manifest?: Maybe<Scalars['JSON']['output']>;
};

export type FigmaImportErrorEvent = {
  __typename?: 'FigmaImportErrorEvent';
  code: Scalars['String']['output'];
  importId: Scalars['ID']['output'];
  message: Scalars['String']['output'];
};

export type FigmaImportEvent = FigmaImportDoneEvent | FigmaImportErrorEvent | FigmaImportProgressEvent | FigmaImportStateEvent;

export type FigmaImportInput = {
  fileKey: Scalars['String']['input'];
  workspaceId: Scalars['ID']['input'];
};

export type FigmaImportProgressEvent = {
  __typename?: 'FigmaImportProgressEvent';
  importId: Scalars['ID']['output'];
  pct?: Maybe<Scalars['Float']['output']>;
  step: Scalars['String']['output'];
};

export type FigmaImportResult = {
  __typename?: 'FigmaImportResult';
  ok: Scalars['Boolean']['output'];
  payload?: Maybe<Scalars['JSON']['output']>;
};

export type FigmaImportStateEvent = {
  __typename?: 'FigmaImportStateEvent';
  importId: Scalars['ID']['output'];
  status: FigmaImportStatusKind;
  ts: Scalars['DateTime']['output'];
};

export type FigmaImportStatusKind =
  | 'FAILED'
  | 'QUEUED'
  | 'RUNNING'
  | 'SUCCESS';

export type ForkChatInput = {
  chatId: Scalars['ID']['input'];
  fromMessageId?: InputMaybe<Scalars['ID']['input']>;
  title?: InputMaybe<Scalars['String']['input']>;
};

export type GateIssue = {
  __typename?: 'GateIssue';
  line?: Maybe<Scalars['Int']['output']>;
  message: Scalars['String']['output'];
  path?: Maybe<Scalars['String']['output']>;
  rule?: Maybe<Scalars['String']['output']>;
  severity?: Maybe<Scalars['String']['output']>;
};

export type GateStatus =
  | 'BLOCKED'
  | 'FAIL'
  | 'PASS'
  | 'PENDING'
  | 'RUNNING'
  | 'SKIPPED'
  | 'WARN';

export type GateVerdict = {
  __typename?: 'GateVerdict';
  durationMs?: Maybe<Scalars['Int']['output']>;
  finishedAt?: Maybe<Scalars['DateTime']['output']>;
  gate: Scalars['String']['output'];
  issues: Array<GateIssue>;
  notes?: Maybe<Scalars['String']['output']>;
  startedAt?: Maybe<Scalars['DateTime']['output']>;
  status: GateStatus;
};

export type GitHubAppInstallation = {
  __typename?: 'GitHubAppInstallation';
  account: Scalars['String']['output'];
  accountAvatarUrl?: Maybe<Scalars['String']['output']>;
  htmlUrl: Scalars['String']['output'];
  id: Scalars['ID']['output'];
  installedAt?: Maybe<Scalars['DateTime']['output']>;
  permissions?: Maybe<Scalars['JSON']['output']>;
  type: Scalars['String']['output'];
};

export type GitHubAuthChallenge = {
  __typename?: 'GitHubAuthChallenge';
  authUrl: Scalars['String']['output'];
  state: Scalars['String']['output'];
};

export type GitHubLoginResult = {
  __typename?: 'GitHubLoginResult';
  login: Scalars['String']['output'];
  token: Scalars['String']['output'];
  user: User;
};

export type GitHubRepo = {
  __typename?: 'GitHubRepo';
  defaultBranch?: Maybe<Scalars['String']['output']>;
  description?: Maybe<Scalars['String']['output']>;
  fullName: Scalars['String']['output'];
  htmlUrl: Scalars['String']['output'];
  id: Scalars['ID']['output'];
  name: Scalars['String']['output'];
  owner: Scalars['String']['output'];
  private: Scalars['Boolean']['output'];
  pushedAt?: Maybe<Scalars['DateTime']['output']>;
};

export type GitHubUserLink = {
  __typename?: 'GitHubUserLink';
  avatarUrl?: Maybe<Scalars['String']['output']>;
  connected: Scalars['Boolean']['output'];
  connectedAt?: Maybe<Scalars['DateTime']['output']>;
  login?: Maybe<Scalars['String']['output']>;
  scopes?: Maybe<Array<Scalars['String']['output']>>;
};

export type GithubLink = {
  __typename?: 'GithubLink';
  defaultBranch?: Maybe<Scalars['String']['output']>;
  fullName: Scalars['String']['output'];
  htmlUrl?: Maybe<Scalars['String']['output']>;
  owner: Scalars['String']['output'];
  repo: Scalars['String']['output'];
};

export type GqlError = {
  __typename?: 'GqlError';
  code: Scalars['String']['output'];
  details?: Maybe<Scalars['JSON']['output']>;
  message: Scalars['String']['output'];
};

export type HeartbeatEvent = {
  __typename?: 'HeartbeatEvent';
  message?: Maybe<Scalars['String']['output']>;
  ts: Scalars['DateTime']['output'];
};

export type IpAllowlistEntry = {
  __typename?: 'IPAllowlistEntry';
  cidr: Scalars['String']['output'];
  createdAt: Scalars['DateTime']['output'];
  id: Scalars['ID']['output'];
  note?: Maybe<Scalars['String']['output']>;
  orgId: Scalars['ID']['output'];
};

export type Import = {
  __typename?: 'Import';
  finishedAt?: Maybe<Scalars['DateTime']['output']>;
  id: Scalars['ID']['output'];
  message?: Maybe<Scalars['String']['output']>;
  projectId: Scalars['ID']['output'];
  ref?: Maybe<Scalars['String']['output']>;
  source: Scalars['String']['output'];
  startedAt: Scalars['DateTime']['output'];
  status: ImportStatus;
};

export type ImportStatus =
  | 'CANCELLED'
  | 'FAILED'
  | 'QUEUED'
  | 'RUNNING'
  | 'SUCCESS';

export type InlineCancelledDelta = {
  __typename?: 'InlineCancelledDelta';
  reason?: Maybe<Scalars['String']['output']>;
  requestId: Scalars['ID']['output'];
};

export type InlineCompletion = {
  __typename?: 'InlineCompletion';
  accepted: Scalars['Boolean']['output'];
  cursor: Scalars['Int']['output'];
  finishedAt?: Maybe<Scalars['DateTime']['output']>;
  model: Scalars['String']['output'];
  provider: Scalars['String']['output'];
  requestId: Scalars['ID']['output'];
  startedAt: Scalars['DateTime']['output'];
  text: Scalars['String']['output'];
};

export type InlineDelta = InlineCancelledDelta | InlineDoneDelta | InlineErrorDelta | InlineStartDelta | InlineTextDelta;

export type InlineDoneDelta = {
  __typename?: 'InlineDoneDelta';
  model?: Maybe<Scalars['String']['output']>;
  provider?: Maybe<Scalars['String']['output']>;
  requestId: Scalars['ID']['output'];
  usage?: Maybe<Scalars['JSON']['output']>;
};

export type InlineErrorDelta = {
  __typename?: 'InlineErrorDelta';
  code: Scalars['String']['output'];
  message: Scalars['String']['output'];
  requestId?: Maybe<Scalars['ID']['output']>;
};

export type InlineInput = {
  cursor?: InputMaybe<Scalars['Int']['input']>;
  effort?: InputMaybe<Scalars['String']['input']>;
  language?: InputMaybe<Scalars['String']['input']>;
  path?: InputMaybe<Scalars['String']['input']>;
  prefix: Scalars['String']['input'];
  projectId?: InputMaybe<Scalars['ID']['input']>;
  requestId?: InputMaybe<Scalars['ID']['input']>;
  suffix?: InputMaybe<Scalars['String']['input']>;
  workspaceId?: InputMaybe<Scalars['ID']['input']>;
};

export type InlineStartDelta = {
  __typename?: 'InlineStartDelta';
  model: Scalars['String']['output'];
  provider: Scalars['String']['output'];
  requestId: Scalars['ID']['output'];
};

export type InlineTextDelta = {
  __typename?: 'InlineTextDelta';
  requestId: Scalars['ID']['output'];
  text: Scalars['String']['output'];
};

export type InviteCollaboratorInput = {
  email: Scalars['String']['input'];
  projectId: Scalars['ID']['input'];
  role?: InputMaybe<Scalars['String']['input']>;
};

export type Invoice = {
  __typename?: 'Invoice';
  amountCents: Scalars['Int']['output'];
  createdAt: Scalars['DateTime']['output'];
  currency: Scalars['String']['output'];
  hostedInvoiceUrl?: Maybe<Scalars['String']['output']>;
  id: Scalars['ID']['output'];
  invoicePdfUrl?: Maybe<Scalars['String']['output']>;
  periodEnd?: Maybe<Scalars['DateTime']['output']>;
  periodStart?: Maybe<Scalars['DateTime']['output']>;
  status: Scalars['String']['output'];
};

export type Lead = {
  __typename?: 'Lead';
  company?: Maybe<Scalars['String']['output']>;
  createdAt: Scalars['DateTime']['output'];
  email: Scalars['String']['output'];
  id: Scalars['ID']['output'];
  message?: Maybe<Scalars['String']['output']>;
  source?: Maybe<Scalars['String']['output']>;
};

export type LedgerEntry = {
  __typename?: 'LedgerEntry';
  agent?: Maybe<Scalars['String']['output']>;
  completionTokens: Scalars['Int']['output'];
  costUsd: Scalars['Decimal']['output'];
  durationMs?: Maybe<Scalars['Int']['output']>;
  id: Scalars['ID']['output'];
  model?: Maybe<Scalars['String']['output']>;
  projectId?: Maybe<Scalars['ID']['output']>;
  promptTokens: Scalars['Int']['output'];
  provider?: Maybe<Scalars['String']['output']>;
  revenueUsd: Scalars['Decimal']['output'];
  ts: Scalars['DateTime']['output'];
  userId: Scalars['ID']['output'];
};

export type MemoryFederationEntry = {
  __typename?: 'MemoryFederationEntry';
  joinedAt: Scalars['DateTime']['output'];
  projectId: Scalars['ID']['output'];
};

export type MemoryKind =
  | 'BUSINESS'
  | 'EXECUTION'
  | 'PROJECT'
  | 'USER';

export type MemoryQueryInput = {
  federated?: InputMaybe<Scalars['Boolean']['input']>;
  gateName?: InputMaybe<Scalars['String']['input']>;
  kind?: InputMaybe<MemoryKind>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  projectId?: InputMaybe<Scalars['ID']['input']>;
  q?: InputMaybe<Scalars['String']['input']>;
  storyId?: InputMaybe<Scalars['ID']['input']>;
  tag?: InputMaybe<Scalars['String']['input']>;
  userId?: InputMaybe<Scalars['ID']['input']>;
};

export type MemoryRecord = {
  __typename?: 'MemoryRecord';
  body: Scalars['String']['output'];
  createdAt: Scalars['DateTime']['output'];
  gateName?: Maybe<Scalars['String']['output']>;
  id: Scalars['ID']['output'];
  kind: MemoryKind;
  projectId?: Maybe<Scalars['ID']['output']>;
  storyId?: Maybe<Scalars['ID']['output']>;
  tags?: Maybe<Array<Scalars['String']['output']>>;
  title?: Maybe<Scalars['String']['output']>;
  userId?: Maybe<Scalars['ID']['output']>;
};

export type MfaEnrollment = {
  __typename?: 'MfaEnrollment';
  qrCodeDataUrl: Scalars['String']['output'];
  recoveryCodes: Array<Scalars['String']['output']>;
  secret: Scalars['String']['output'];
};

export type Mutation = {
  __typename?: 'Mutation';
  _empty?: Maybe<Scalars['String']['output']>;
  acceptInlineCompletion: OperationResult;
  addIPAllowlist: IpAllowlistEntry;
  addMemory: MemoryRecord;
  addSubproject: Subproject;
  addVisualTarget: VisualTarget;
  adminRefund: RefundResult;
  appendChatMessage: ChatMessage;
  applyPatch: Patch;
  applyStage: Scalars['JSON']['output'];
  brainstormRun: Scalars['JSON']['output'];
  bulkDeleteProjects: OperationResult;
  cancelDeploy: OperationResult;
  cancelSubscription: OperationResult;
  captureEnterpriseLead: Lead;
  cloneIntoWorkspace: CloneIntoWorkspaceResult;
  completeGithubAuth: GitHubLoginResult;
  completeMfaSignIn: Session;
  confirmEmailChange: OperationResult;
  confirmMfaEnrollment: OperationResult;
  connectGithub: Project;
  createBillingPortalSession: BillingPortalSession;
  createChat: Chat;
  createCustomDomain: CustomDomain;
  createProject: Project;
  createShareLink: ShareLink;
  createStage: PatchStage;
  createWebhook: WebhookSubscription;
  createWorkspace: Workspace;
  deleteCustomDomain: OperationResult;
  deleteIPAllowlist: OperationResult;
  deleteMemory: OperationResult;
  deleteProject: OperationResult;
  deleteShareLink: OperationResult;
  deleteSubproject: OperationResult;
  deleteVisualTarget: OperationResult;
  deleteWebhook: OperationResult;
  disableMfa: OperationResult;
  disconnectGithub: Project;
  disconnectGithubUser: OperationResult;
  enrollMfa: MfaEnrollment;
  execInWorkspace: ExecResult;
  exportGithub: OperationResult;
  exportZipUrl: Scalars['String']['output'];
  figmaImport: FigmaImportResult;
  forkChat: Chat;
  inviteCollaborator: Collaborator;
  joinMemoryFederation: MemoryFederationEntry;
  leaveMemoryFederation: OperationResult;
  mcpRPC: Scalars['JSON']['output'];
  planDeploy: Scalars['JSON']['output'];
  promptPlan: Scalars['JSON']['output'];
  proposePatch: Patch;
  proposeSymbolPatch: Patch;
  recordAffiliatePayout: AffiliatePayout;
  rejectStage: PatchStage;
  removeCollaborator: OperationResult;
  renameSymbol: Patch;
  requestEmailChange: OperationResult;
  requestPasswordReset: OperationResult;
  rerunGate: GateVerdict;
  resendVerificationEmail: OperationResult;
  resetPassword: Session;
  retryWebhookDelivery: OperationResult;
  revokeAllOtherSessions: OperationResult;
  revokeSession: OperationResult;
  rollbackPatch: Scalars['JSON']['output'];
  runFinisher: Scalars['JSON']['output'];
  sendWorkspacePtyInput: OperationResult;
  setNotificationPreferences: NotificationPreferences;
  setTelemetryPreference: User;
  signIn: Session;
  signOut: OperationResult;
  signUp: Session;
  signupAffiliate: Affiliate;
  startCheckout: StripeCheckoutSession;
  startDeploy: Deploy;
  startFigmaImport: FigmaImport;
  startGithubLink: GitHubAuthChallenge;
  startGithubLogin: GitHubAuthChallenge;
  startImport: Import;
  startWorkspace: Workspace;
  stopWorkspace: OperationResult;
  testWebhook: WebhookTestResult;
  updateProject: Project;
  updateSamlConfig: SamlConfig;
  verifyCustomDomain: CustomDomain;
  verifyEmail: Session;
  visualDiff: VisualDiffResult;
  visualEdit: Patch;
  writeWorkspaceFile: OperationResult;
};


export type MutationAcceptInlineCompletionArgs = {
  requestId: Scalars['ID']['input'];
};


export type MutationAddIpAllowlistArgs = {
  input: AddIpAllowlistInput;
  orgId: Scalars['ID']['input'];
};


export type MutationAddMemoryArgs = {
  input: AddMemoryInput;
};


export type MutationAddSubprojectArgs = {
  id: Scalars['ID']['input'];
  input: AddSubprojectInput;
};


export type MutationAddVisualTargetArgs = {
  id: Scalars['ID']['input'];
  input: AddVisualTargetInput;
};


export type MutationAdminRefundArgs = {
  input: AdminRefundInput;
};


export type MutationAppendChatMessageArgs = {
  input: AppendChatMessageInput;
};


export type MutationApplyPatchArgs = {
  patchId: Scalars['ID']['input'];
};


export type MutationApplyStageArgs = {
  stageId: Scalars['ID']['input'];
};


export type MutationBrainstormRunArgs = {
  goal: Scalars['String']['input'];
  id: Scalars['ID']['input'];
  role?: InputMaybe<Scalars['String']['input']>;
};


export type MutationBulkDeleteProjectsArgs = {
  ids: Array<Scalars['ID']['input']>;
};


export type MutationCancelDeployArgs = {
  id: Scalars['ID']['input'];
};


export type MutationCancelSubscriptionArgs = {
  input: CancelSubscriptionInput;
};


export type MutationCaptureEnterpriseLeadArgs = {
  input: EnterpriseLeadInput;
};


export type MutationCloneIntoWorkspaceArgs = {
  id: Scalars['ID']['input'];
  input: CloneIntoWorkspaceInput;
};


export type MutationCompleteGithubAuthArgs = {
  code: Scalars['String']['input'];
  state: Scalars['String']['input'];
};


export type MutationCompleteMfaSignInArgs = {
  challenge: Scalars['String']['input'];
  code: Scalars['String']['input'];
};


export type MutationConfirmEmailChangeArgs = {
  token: Scalars['String']['input'];
};


export type MutationConfirmMfaEnrollmentArgs = {
  code: Scalars['String']['input'];
};


export type MutationConnectGithubArgs = {
  id: Scalars['ID']['input'];
  input: ConnectGithubInput;
};


export type MutationCreateBillingPortalSessionArgs = {
  returnUrl: Scalars['String']['input'];
};


export type MutationCreateChatArgs = {
  input: CreateChatInput;
};


export type MutationCreateCustomDomainArgs = {
  input: CreateCustomDomainInput;
};


export type MutationCreateProjectArgs = {
  input: CreateProjectInput;
};


export type MutationCreateShareLinkArgs = {
  input: CreateShareLinkInput;
};


export type MutationCreateStageArgs = {
  input: CreateStageInput;
};


export type MutationCreateWebhookArgs = {
  input: CreateWebhookInput;
};


export type MutationCreateWorkspaceArgs = {
  driver?: InputMaybe<Scalars['String']['input']>;
  projectId: Scalars['ID']['input'];
};


export type MutationDeleteCustomDomainArgs = {
  id: Scalars['ID']['input'];
};


export type MutationDeleteIpAllowlistArgs = {
  id: Scalars['ID']['input'];
  orgId: Scalars['ID']['input'];
};


export type MutationDeleteMemoryArgs = {
  id: Scalars['ID']['input'];
};


export type MutationDeleteProjectArgs = {
  id: Scalars['ID']['input'];
};


export type MutationDeleteShareLinkArgs = {
  linkId: Scalars['ID']['input'];
};


export type MutationDeleteSubprojectArgs = {
  id: Scalars['ID']['input'];
  subId: Scalars['ID']['input'];
};


export type MutationDeleteVisualTargetArgs = {
  id: Scalars['ID']['input'];
  targetId: Scalars['ID']['input'];
};


export type MutationDeleteWebhookArgs = {
  id: Scalars['ID']['input'];
};


export type MutationDisableMfaArgs = {
  code: Scalars['String']['input'];
};


export type MutationDisconnectGithubArgs = {
  id: Scalars['ID']['input'];
};


export type MutationExecInWorkspaceArgs = {
  command: Scalars['String']['input'];
  timeoutSec?: InputMaybe<Scalars['Int']['input']>;
  workspaceId: Scalars['ID']['input'];
};


export type MutationExportGithubArgs = {
  input: ExportGithubInput;
};


export type MutationExportZipUrlArgs = {
  projectId: Scalars['ID']['input'];
};


export type MutationFigmaImportArgs = {
  id: Scalars['ID']['input'];
  input: FigmaImportInput;
};


export type MutationForkChatArgs = {
  input: ForkChatInput;
};


export type MutationInviteCollaboratorArgs = {
  input: InviteCollaboratorInput;
};


export type MutationJoinMemoryFederationArgs = {
  projectId: Scalars['ID']['input'];
};


export type MutationLeaveMemoryFederationArgs = {
  projectId: Scalars['ID']['input'];
};


export type MutationMcpRpcArgs = {
  envelope: Scalars['JSON']['input'];
};


export type MutationPlanDeployArgs = {
  projectId: Scalars['ID']['input'];
};


export type MutationPromptPlanArgs = {
  id: Scalars['ID']['input'];
  prompt: Scalars['String']['input'];
};


export type MutationProposePatchArgs = {
  input: ProposePatchInput;
};


export type MutationProposeSymbolPatchArgs = {
  input: SymbolPatchInput;
};


export type MutationRecordAffiliatePayoutArgs = {
  input: RecordAffiliatePayoutInput;
};


export type MutationRejectStageArgs = {
  reason?: InputMaybe<Scalars['String']['input']>;
  stageId: Scalars['ID']['input'];
};


export type MutationRemoveCollaboratorArgs = {
  collaboratorId: Scalars['ID']['input'];
  projectId: Scalars['ID']['input'];
};


export type MutationRenameSymbolArgs = {
  input: RenameSymbolInput;
};


export type MutationRequestEmailChangeArgs = {
  input: EmailChangeInput;
};


export type MutationRequestPasswordResetArgs = {
  email: Scalars['String']['input'];
};


export type MutationRerunGateArgs = {
  input: RerunGateInput;
};


export type MutationResetPasswordArgs = {
  newPassword: Scalars['String']['input'];
  token: Scalars['String']['input'];
};


export type MutationRetryWebhookDeliveryArgs = {
  id: Scalars['ID']['input'];
};


export type MutationRevokeSessionArgs = {
  jti: Scalars['ID']['input'];
};


export type MutationRollbackPatchArgs = {
  patchId: Scalars['ID']['input'];
};


export type MutationRunFinisherArgs = {
  id: Scalars['ID']['input'];
};


export type MutationSendWorkspacePtyInputArgs = {
  data: Scalars['String']['input'];
  workspaceId: Scalars['ID']['input'];
};


export type MutationSetNotificationPreferencesArgs = {
  input: SetNotificationPreferencesInput;
};


export type MutationSetTelemetryPreferenceArgs = {
  input: TelemetryPreferenceInput;
};


export type MutationSignInArgs = {
  input: SignInInput;
};


export type MutationSignUpArgs = {
  input: SignUpInput;
};


export type MutationSignupAffiliateArgs = {
  input: AffiliateSignupInput;
};


export type MutationStartCheckoutArgs = {
  input: StartCheckoutInput;
};


export type MutationStartDeployArgs = {
  input: StartDeployInput;
};


export type MutationStartFigmaImportArgs = {
  input: StartFigmaImportInput;
};


export type MutationStartImportArgs = {
  input: StartImportInput;
};


export type MutationStartWorkspaceArgs = {
  id: Scalars['ID']['input'];
};


export type MutationStopWorkspaceArgs = {
  id: Scalars['ID']['input'];
};


export type MutationTestWebhookArgs = {
  id: Scalars['ID']['input'];
};


export type MutationUpdateProjectArgs = {
  id: Scalars['ID']['input'];
  input: UpdateProjectInput;
};


export type MutationUpdateSamlConfigArgs = {
  input: UpdateSamlConfigInput;
  orgId: Scalars['ID']['input'];
};


export type MutationVerifyCustomDomainArgs = {
  id: Scalars['ID']['input'];
};


export type MutationVerifyEmailArgs = {
  token: Scalars['String']['input'];
};


export type MutationVisualDiffArgs = {
  input: VisualDiffInput;
};


export type MutationVisualEditArgs = {
  id: Scalars['ID']['input'];
  instruction: Scalars['String']['input'];
  path?: InputMaybe<Scalars['String']['input']>;
  screenshot?: InputMaybe<Scalars['String']['input']>;
  screenshotMediaType?: InputMaybe<Scalars['String']['input']>;
  selector: Scalars['String']['input'];
};


export type MutationWriteWorkspaceFileArgs = {
  content: Scalars['String']['input'];
  path: Scalars['String']['input'];
  workspaceId: Scalars['ID']['input'];
};

export type NotificationChannel =
  | 'EMAIL'
  | 'IN_APP'
  | 'SLACK'
  | 'WEBHOOK';

export type NotificationPreferences = {
  __typename?: 'NotificationPreferences';
  rules: Array<NotificationRule>;
  userId: Scalars['ID']['output'];
};

export type NotificationRule = {
  __typename?: 'NotificationRule';
  channels: Array<NotificationChannel>;
  enabled: Scalars['Boolean']['output'];
  event: Scalars['String']['output'];
};

export type NotificationRuleInput = {
  channels: Array<NotificationChannel>;
  enabled: Scalars['Boolean']['input'];
  event: Scalars['String']['input'];
};

export type OperationResult = {
  __typename?: 'OperationResult';
  message?: Maybe<Scalars['String']['output']>;
  ok: Scalars['Boolean']['output'];
};

export type PrReview = {
  __typename?: 'PRReview';
  commentUrl?: Maybe<Scalars['String']['output']>;
  commitSha: Scalars['String']['output'];
  durationMs?: Maybe<Scalars['Int']['output']>;
  finishedAt?: Maybe<Scalars['DateTime']['output']>;
  gateVerdicts: Array<GateVerdict>;
  id: Scalars['ID']['output'];
  installationId: Scalars['ID']['output'];
  prNumber: Scalars['Int']['output'];
  repoFullName: Scalars['String']['output'];
  startedAt: Scalars['DateTime']['output'];
  status: PrReviewStatus;
};

export type PrReviewStatus =
  | 'ERRORED'
  | 'FAILED'
  | 'PASSED'
  | 'QUEUED'
  | 'RUNNING';

export type PageInfo = {
  __typename?: 'PageInfo';
  endCursor?: Maybe<Scalars['String']['output']>;
  hasNextPage: Scalars['Boolean']['output'];
  totalCount?: Maybe<Scalars['Int']['output']>;
};

export type Patch = {
  __typename?: 'Patch';
  appliedAt?: Maybe<Scalars['DateTime']['output']>;
  author?: Maybe<Scalars['String']['output']>;
  changes: Array<PatchChange>;
  conflicts?: Maybe<Array<PatchConflict>>;
  createdAt: Scalars['DateTime']['output'];
  id: Scalars['ID']['output'];
  projectId: Scalars['ID']['output'];
  stage?: Maybe<PatchStage>;
  stageId?: Maybe<Scalars['ID']['output']>;
  status: PatchStatus;
  summary?: Maybe<Scalars['String']['output']>;
  title?: Maybe<Scalars['String']['output']>;
};

export type PatchAnchorOp = {
  __typename?: 'PatchAnchorOp';
  anchor: Scalars['String']['output'];
  path: Scalars['String']['output'];
  replacement: Scalars['String']['output'];
};

export type PatchChange = {
  __typename?: 'PatchChange';
  anchor?: Maybe<Scalars['String']['output']>;
  content?: Maybe<Scalars['String']['output']>;
  op: PatchChangeOp;
  path: Scalars['String']['output'];
  replacement?: Maybe<Scalars['String']['output']>;
  symbol?: Maybe<Scalars['String']['output']>;
};

export type PatchChangeInput = {
  anchor?: InputMaybe<Scalars['String']['input']>;
  content?: InputMaybe<Scalars['String']['input']>;
  op: PatchChangeOp;
  path: Scalars['String']['input'];
  replacement?: InputMaybe<Scalars['String']['input']>;
  symbol?: InputMaybe<Scalars['String']['input']>;
};

export type PatchChangeOp =
  | 'ANCHOR_REPLACE'
  | 'CREATE'
  | 'DELETE'
  | 'INSERT_AFTER'
  | 'INSERT_BEFORE'
  | 'REPLACE'
  | 'SYMBOL_REPLACE';

export type PatchConflict = {
  __typename?: 'PatchConflict';
  base: Scalars['String']['output'];
  markers: Scalars['String']['output'];
  ours: Scalars['String']['output'];
  path: Scalars['String']['output'];
  theirs: Scalars['String']['output'];
};

export type PatchOp = PatchAnchorOp | PatchSymbolOp;

export type PatchStage = {
  __typename?: 'PatchStage';
  createdAt: Scalars['DateTime']['output'];
  description?: Maybe<Scalars['String']['output']>;
  id: Scalars['ID']['output'];
  name: Scalars['String']['output'];
  patchIds: Array<Scalars['ID']['output']>;
  projectId: Scalars['ID']['output'];
  rejectionReason?: Maybe<Scalars['String']['output']>;
  status: PatchStageStatus;
  updatedAt: Scalars['DateTime']['output'];
};

export type PatchStageStatus =
  | 'APPLIED'
  | 'OPEN'
  | 'REJECTED'
  | 'REVIEWED';

export type PatchStatus =
  | 'APPLIED'
  | 'APPROVED'
  | 'CONFLICTED'
  | 'PROPOSED'
  | 'REJECTED'
  | 'ROLLED_BACK';

export type PatchSymbolOp = {
  __typename?: 'PatchSymbolOp';
  path: Scalars['String']['output'];
  replacement: Scalars['String']['output'];
  symbol: Scalars['String']['output'];
};

export type Plan = {
  __typename?: 'Plan';
  costCapUsd: Scalars['Decimal']['output'];
  description?: Maybe<Scalars['String']['output']>;
  features: Array<Scalars['String']['output']>;
  name: Scalars['String']['output'];
  priceUsd: Scalars['Decimal']['output'];
  stripePriceId?: Maybe<Scalars['String']['output']>;
  tier: Scalars['String']['output'];
};

export type Presence = {
  __typename?: 'Presence';
  color?: Maybe<Scalars['String']['output']>;
  email?: Maybe<Scalars['String']['output']>;
  lastSeen?: Maybe<Scalars['DateTime']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  online: Scalars['Boolean']['output'];
  userId: Scalars['ID']['output'];
};

export type PresenceEvent = PresenceJoinedEvent | PresenceLeftEvent | PresenceUpdatedEvent;

export type PresenceJoinedEvent = {
  __typename?: 'PresenceJoinedEvent';
  presence: Presence;
};

export type PresenceLeftEvent = {
  __typename?: 'PresenceLeftEvent';
  ts: Scalars['DateTime']['output'];
  userId: Scalars['ID']['output'];
};

export type PresenceUpdatedEvent = {
  __typename?: 'PresenceUpdatedEvent';
  presence: Presence;
};

export type Project = {
  __typename?: 'Project';
  createdAt: Scalars['DateTime']['output'];
  description?: Maybe<Scalars['String']['output']>;
  files: Array<ProjectFile>;
  gates: Array<GateVerdict>;
  github?: Maybe<GithubLink>;
  id: Scalars['ID']['output'];
  idea?: Maybe<Scalars['String']['output']>;
  isPublic: Scalars['Boolean']['output'];
  name: Scalars['String']['output'];
  ownerId: Scalars['ID']['output'];
  status: Scalars['String']['output'];
  subprojects: Array<Subproject>;
  updatedAt: Scalars['DateTime']['output'];
  visualTargets: Array<VisualTarget>;
};

export type ProjectFile = {
  __typename?: 'ProjectFile';
  content?: Maybe<Scalars['String']['output']>;
  language?: Maybe<Scalars['String']['output']>;
  path: Scalars['String']['output'];
  size?: Maybe<Scalars['Int']['output']>;
  updatedAt?: Maybe<Scalars['DateTime']['output']>;
};

export type ProjectGraph = {
  __typename?: 'ProjectGraph';
  edges: Array<ProjectGraphEdge>;
  nodes: Array<ProjectGraphNode>;
};

export type ProjectGraphEdge = {
  __typename?: 'ProjectGraphEdge';
  from: Scalars['ID']['output'];
  kind: Scalars['String']['output'];
  to: Scalars['ID']['output'];
};

export type ProjectGraphNode = {
  __typename?: 'ProjectGraphNode';
  id: Scalars['ID']['output'];
  language?: Maybe<Scalars['String']['output']>;
  path: Scalars['String']['output'];
  size?: Maybe<Scalars['Int']['output']>;
};

export type ProposePatchInput = {
  author?: InputMaybe<Scalars['String']['input']>;
  changes: Array<PatchChangeInput>;
  projectId: Scalars['ID']['input'];
  summary?: InputMaybe<Scalars['String']['input']>;
  title?: InputMaybe<Scalars['String']['input']>;
};

export type ProviderHealth = {
  __typename?: 'ProviderHealth';
  lastCheckedAt?: Maybe<Scalars['DateTime']['output']>;
  lastError?: Maybe<Scalars['String']['output']>;
  latencyMs?: Maybe<Scalars['Int']['output']>;
  ok: Scalars['Boolean']['output'];
  provider: Scalars['String']['output'];
};

export type PtyEvent = PtyExit | PtyOutput;

export type PtyExit = {
  __typename?: 'PtyExit';
  code: Scalars['Int']['output'];
};

export type PtyOutput = {
  __typename?: 'PtyOutput';
  data: Scalars['String']['output'];
};

export type Query = {
  __typename?: 'Query';
  affiliatePayoutHistory: Array<AffiliatePayout>;
  agentTelemetry: Array<AgentCall>;
  agents: Array<Agent>;
  audit: Array<AuditEntry>;
  auditExportCsvUrl: Scalars['String']['output'];
  auditExportPdfUrl: Scalars['String']['output'];
  banditRanking: BanditRanking;
  chat?: Maybe<Chat>;
  chatMessages: Array<ChatMessage>;
  chats: Array<Chat>;
  collaborators: Array<Collaborator>;
  deploy?: Maybe<Deploy>;
  deploys: Array<Deploy>;
  gate?: Maybe<GateVerdict>;
  gates: Array<GateVerdict>;
  githubAppInstallations: Array<GitHubAppInstallation>;
  githubRepos: Array<GitHubRepo>;
  githubUserLink: GitHubUserLink;
  importStatus?: Maybe<Import>;
  invoices: Array<Invoice>;
  ipAllowlist: Array<IpAllowlistEntry>;
  me?: Maybe<User>;
  memory: Array<MemoryRecord>;
  memoryFederation: Array<MemoryFederationEntry>;
  myAffiliate?: Maybe<AffiliateStats>;
  myBudget: BudgetSummary;
  mySessions: Array<Session>;
  mySubscription?: Maybe<Subscription_Stripe>;
  notificationPreferences: NotificationPreferences;
  patch?: Maybe<Patch>;
  patchSnapshots?: Maybe<Scalars['JSON']['output']>;
  patches: Array<Patch>;
  ping: Scalars['String']['output'];
  plans: Array<Plan>;
  prReview?: Maybe<PrReview>;
  prReviews: Array<PrReview>;
  presence: Array<Presence>;
  project?: Maybe<Project>;
  projectDomains: Array<CustomDomain>;
  projectFiles: Array<ProjectFile>;
  projectGraph: ProjectGraph;
  projectSnapshot: Scalars['JSON']['output'];
  projectSubprojects: Array<Subproject>;
  projectVisualTargets: Array<VisualTarget>;
  projects: Array<Project>;
  providerHealth: Array<ProviderHealth>;
  rates: Array<Rate>;
  reflections: Array<Reflection>;
  samlConfig?: Maybe<SamlConfig>;
  searchProjectCode: Array<CodeSearchHit>;
  shareLinks: Array<ShareLink>;
  sharedSnapshot?: Maybe<SharedSnapshot>;
  stage?: Maybe<PatchStage>;
  stages: Array<PatchStage>;
  status: Array<ServiceStatus>;
  uptime24h: Array<UptimeSeries>;
  vault: VaultSnapshot;
  verifyAudit: AuditVerifyResult;
  version: VersionInfo;
  webhookDeadLetters: Array<WebhookDelivery>;
  webhooks: Array<WebhookSubscription>;
  workspace?: Maybe<Workspace>;
  workspaceFile?: Maybe<WorkspaceFileContent>;
  workspaceFiles: Array<WorkspaceFile>;
  workspaces: Array<Workspace>;
};


export type QueryAffiliatePayoutHistoryArgs = {
  affiliateId?: InputMaybe<Scalars['ID']['input']>;
};


export type QueryAgentTelemetryArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
  modelName?: InputMaybe<Scalars['String']['input']>;
  provider?: InputMaybe<Scalars['String']['input']>;
  role?: InputMaybe<Scalars['String']['input']>;
};


export type QueryAuditArgs = {
  query?: InputMaybe<AuditQueryInput>;
};


export type QueryAuditExportCsvUrlArgs = {
  query?: InputMaybe<AuditQueryInput>;
};


export type QueryAuditExportPdfUrlArgs = {
  query?: InputMaybe<AuditQueryInput>;
};


export type QueryBanditRankingArgs = {
  lookback?: InputMaybe<Scalars['Int']['input']>;
};


export type QueryChatArgs = {
  chatId: Scalars['ID']['input'];
};


export type QueryChatMessagesArgs = {
  chatId: Scalars['ID']['input'];
};


export type QueryChatsArgs = {
  projectId: Scalars['ID']['input'];
};


export type QueryCollaboratorsArgs = {
  projectId: Scalars['ID']['input'];
};


export type QueryDeployArgs = {
  id: Scalars['ID']['input'];
};


export type QueryDeploysArgs = {
  projectId: Scalars['ID']['input'];
};


export type QueryGateArgs = {
  gate: Scalars['String']['input'];
  projectId: Scalars['ID']['input'];
};


export type QueryGatesArgs = {
  projectId: Scalars['ID']['input'];
  sub?: InputMaybe<Scalars['ID']['input']>;
};


export type QueryImportStatusArgs = {
  projectId: Scalars['ID']['input'];
};


export type QueryInvoicesArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
};


export type QueryIpAllowlistArgs = {
  orgId: Scalars['ID']['input'];
};


export type QueryMemoryArgs = {
  query?: InputMaybe<MemoryQueryInput>;
};


export type QueryPatchArgs = {
  id: Scalars['ID']['input'];
};


export type QueryPatchSnapshotsArgs = {
  projectId: Scalars['ID']['input'];
};


export type QueryPatchesArgs = {
  projectId: Scalars['ID']['input'];
};


export type QueryPrReviewArgs = {
  id: Scalars['ID']['input'];
};


export type QueryPrReviewsArgs = {
  installationId?: InputMaybe<Scalars['ID']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
};


export type QueryPresenceArgs = {
  projectId: Scalars['ID']['input'];
};


export type QueryProjectArgs = {
  id: Scalars['ID']['input'];
};


export type QueryProjectDomainsArgs = {
  projectId: Scalars['ID']['input'];
};


export type QueryProjectFilesArgs = {
  id: Scalars['ID']['input'];
};


export type QueryProjectGraphArgs = {
  id: Scalars['ID']['input'];
};


export type QueryProjectSnapshotArgs = {
  id: Scalars['ID']['input'];
};


export type QueryProjectSubprojectsArgs = {
  id: Scalars['ID']['input'];
};


export type QueryProjectVisualTargetsArgs = {
  id: Scalars['ID']['input'];
};


export type QueryReflectionsArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
  projectId?: InputMaybe<Scalars['ID']['input']>;
};


export type QuerySamlConfigArgs = {
  orgId: Scalars['ID']['input'];
};


export type QuerySearchProjectCodeArgs = {
  id: Scalars['ID']['input'];
  k?: InputMaybe<Scalars['Int']['input']>;
  maxKb?: InputMaybe<Scalars['Int']['input']>;
  q: Scalars['String']['input'];
};


export type QueryShareLinksArgs = {
  projectId: Scalars['ID']['input'];
};


export type QuerySharedSnapshotArgs = {
  slug: Scalars['String']['input'];
};


export type QueryStageArgs = {
  id: Scalars['ID']['input'];
};


export type QueryStagesArgs = {
  projectId: Scalars['ID']['input'];
};


export type QueryWebhookDeadLettersArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
};


export type QueryWorkspaceArgs = {
  id: Scalars['ID']['input'];
};


export type QueryWorkspaceFileArgs = {
  path: Scalars['String']['input'];
  workspaceId: Scalars['ID']['input'];
};


export type QueryWorkspaceFilesArgs = {
  path?: InputMaybe<Scalars['String']['input']>;
  workspaceId: Scalars['ID']['input'];
};


export type QueryWorkspacesArgs = {
  projectId: Scalars['ID']['input'];
};

export type Rate = {
  __typename?: 'Rate';
  completionPerMTok: Scalars['Decimal']['output'];
  model: Scalars['String']['output'];
  promptPerMTok: Scalars['Decimal']['output'];
  provider: Scalars['String']['output'];
};

export type RecordAffiliatePayoutInput = {
  affiliateId: Scalars['ID']['input'];
  amountUsd: Scalars['Decimal']['input'];
  note?: InputMaybe<Scalars['String']['input']>;
};

export type Referral = {
  __typename?: 'Referral';
  affiliateId: Scalars['ID']['output'];
  firstPaidAt?: Maybe<Scalars['DateTime']['output']>;
  id: Scalars['ID']['output'];
  referredUserId?: Maybe<Scalars['ID']['output']>;
  signedUpAt?: Maybe<Scalars['DateTime']['output']>;
  totalRevenueUsd: Scalars['Decimal']['output'];
};

export type Reflection = {
  __typename?: 'Reflection';
  body: Scalars['String']['output'];
  createdAt: Scalars['DateTime']['output'];
  gateName?: Maybe<Scalars['String']['output']>;
  id: Scalars['ID']['output'];
  projectId: Scalars['ID']['output'];
  storyId?: Maybe<Scalars['ID']['output']>;
  voteShare?: Maybe<VoteShare>;
};

export type RefundResult = {
  __typename?: 'RefundResult';
  amountCents: Scalars['Int']['output'];
  chargeId: Scalars['ID']['output'];
  createdAt: Scalars['DateTime']['output'];
  currency: Scalars['String']['output'];
  id: Scalars['ID']['output'];
  reason?: Maybe<Scalars['String']['output']>;
  status: Scalars['String']['output'];
};

export type RenameSymbolInput = {
  kind?: InputMaybe<SymbolKind>;
  newName: Scalars['String']['input'];
  oldName: Scalars['String']['input'];
  projectId: Scalars['ID']['input'];
};

export type RerunGateInput = {
  gate: Scalars['String']['input'];
  projectId: Scalars['ID']['input'];
};

export type RunDoneEvent = {
  __typename?: 'RunDoneEvent';
  ok: Scalars['Boolean']['output'];
  summary?: Maybe<Scalars['JSON']['output']>;
  ts: Scalars['DateTime']['output'];
};

export type RunErrorEvent = {
  __typename?: 'RunErrorEvent';
  code: Scalars['String']['output'];
  message: Scalars['String']['output'];
  ts: Scalars['DateTime']['output'];
};

export type RunEvent = RunDoneEvent | RunErrorEvent | RunExecutionEvent | RunGateEvent;

export type RunExecutionEvent = {
  __typename?: 'RunExecutionEvent';
  payload: Scalars['JSON']['output'];
  ts: Scalars['DateTime']['output'];
};

export type RunGateEvent = {
  __typename?: 'RunGateEvent';
  gate: Scalars['String']['output'];
  message?: Maybe<Scalars['String']['output']>;
  status: Scalars['String']['output'];
  ts: Scalars['DateTime']['output'];
};

export type SamlConfig = {
  __typename?: 'SamlConfig';
  acsUrl: Scalars['String']['output'];
  attributeEmail?: Maybe<Scalars['String']['output']>;
  attributeGroups?: Maybe<Scalars['String']['output']>;
  audience?: Maybe<Scalars['String']['output']>;
  certificate?: Maybe<Scalars['String']['output']>;
  defaultRole?: Maybe<Scalars['String']['output']>;
  enabled: Scalars['Boolean']['output'];
  idpEntityId?: Maybe<Scalars['String']['output']>;
  orgId: Scalars['ID']['output'];
  ssoUrl?: Maybe<Scalars['String']['output']>;
  updatedAt?: Maybe<Scalars['DateTime']['output']>;
};

export type ServiceStatus = {
  __typename?: 'ServiceStatus';
  checkedAt: Scalars['DateTime']['output'];
  message?: Maybe<Scalars['String']['output']>;
  ok: Scalars['Boolean']['output'];
  service: Scalars['String']['output'];
};

export type Session = {
  __typename?: 'Session';
  current?: Maybe<Scalars['Boolean']['output']>;
  expiresAt?: Maybe<Scalars['DateTime']['output']>;
  ipAddress?: Maybe<Scalars['String']['output']>;
  jti?: Maybe<Scalars['ID']['output']>;
  lastSeenAt?: Maybe<Scalars['DateTime']['output']>;
  mfaChallenge?: Maybe<Scalars['String']['output']>;
  mfaRequired?: Maybe<Scalars['Boolean']['output']>;
  token: Scalars['String']['output'];
  user: User;
  userAgent?: Maybe<Scalars['String']['output']>;
};

export type SetNotificationPreferencesInput = {
  rules: Array<NotificationRuleInput>;
};

export type ShareLink = {
  __typename?: 'ShareLink';
  createdAt: Scalars['DateTime']['output'];
  expiresAt?: Maybe<Scalars['DateTime']['output']>;
  id: Scalars['ID']['output'];
  projectId: Scalars['ID']['output'];
  slug: Scalars['String']['output'];
  url: Scalars['String']['output'];
  views: Scalars['Int']['output'];
};

export type SharedSnapshot = {
  __typename?: 'SharedSnapshot';
  capturedAt: Scalars['DateTime']['output'];
  expiresAt?: Maybe<Scalars['DateTime']['output']>;
  project: Project;
  slug: Scalars['String']['output'];
};

export type SignInInput = {
  email: Scalars['String']['input'];
  password: Scalars['String']['input'];
};

export type SignUpInput = {
  email: Scalars['String']['input'];
  name?: InputMaybe<Scalars['String']['input']>;
  password: Scalars['String']['input'];
};

export type StartCheckoutInput = {
  cancelUrl?: InputMaybe<Scalars['String']['input']>;
  successUrl?: InputMaybe<Scalars['String']['input']>;
  tier: Scalars['String']['input'];
};

export type StartDeployInput = {
  env?: InputMaybe<Scalars['JSON']['input']>;
  envVars?: InputMaybe<Array<DeployEnvVar>>;
  productionAlias?: InputMaybe<Scalars['String']['input']>;
  projectId: Scalars['ID']['input'];
  ref?: InputMaybe<Scalars['String']['input']>;
  region?: InputMaybe<Scalars['String']['input']>;
  target?: InputMaybe<DeployTarget>;
  vercelTeamId?: InputMaybe<Scalars['String']['input']>;
};

export type StartFigmaImportInput = {
  fileKey: Scalars['String']['input'];
  projectId: Scalars['ID']['input'];
  workspaceId: Scalars['ID']['input'];
};

export type StartImportInput = {
  projectId?: InputMaybe<Scalars['ID']['input']>;
  projectName?: InputMaybe<Scalars['String']['input']>;
  ref?: InputMaybe<Scalars['String']['input']>;
  source: Scalars['String']['input'];
};

export type StripeCheckoutSession = {
  __typename?: 'StripeCheckoutSession';
  sessionId: Scalars['String']['output'];
  url: Scalars['String']['output'];
};

export type Subproject = {
  __typename?: 'Subproject';
  createdAt: Scalars['DateTime']['output'];
  gates: Array<GateVerdict>;
  id: Scalars['ID']['output'];
  name: Scalars['String']['output'];
  path: Scalars['String']['output'];
  role?: Maybe<Scalars['String']['output']>;
  stack?: Maybe<Scalars['JSON']['output']>;
};

export type Subscription = {
  __typename?: 'Subscription';
  _heartbeat: HeartbeatEvent;
  chatStream: ChatDelta;
  collabChat: CollabChatMessage;
  collabCursors: CursorEvent;
  collabPresence: PresenceEvent;
  costStream: CostDelta;
  deployStream: DeployEvent;
  figmaImportStatus: FigmaImportEvent;
  inlineCompletion: InlineDelta;
  runProject: RunEvent;
  workspacePty: PtyEvent;
};


export type SubscriptionChatStreamArgs = {
  input: ChatInput;
  projectId: Scalars['ID']['input'];
};


export type SubscriptionCollabChatArgs = {
  projectId: Scalars['ID']['input'];
};


export type SubscriptionCollabCursorsArgs = {
  projectId: Scalars['ID']['input'];
};


export type SubscriptionCollabPresenceArgs = {
  projectId: Scalars['ID']['input'];
};


export type SubscriptionDeployStreamArgs = {
  deployId: Scalars['ID']['input'];
};


export type SubscriptionFigmaImportStatusArgs = {
  importId: Scalars['ID']['input'];
};


export type SubscriptionInlineCompletionArgs = {
  input: InlineInput;
};


export type SubscriptionRunProjectArgs = {
  projectId: Scalars['ID']['input'];
};


export type SubscriptionWorkspacePtyArgs = {
  workspaceId: Scalars['ID']['input'];
};

export type Subscription_Stripe = {
  __typename?: 'Subscription_Stripe';
  cancelAtPeriodEnd?: Maybe<Scalars['Boolean']['output']>;
  currentPeriodEnd?: Maybe<Scalars['DateTime']['output']>;
  customerId?: Maybe<Scalars['String']['output']>;
  dunningState?: Maybe<DunningState>;
  status?: Maybe<Scalars['String']['output']>;
  stripeCustomerId?: Maybe<Scalars['String']['output']>;
  stripeSubscriptionId?: Maybe<Scalars['String']['output']>;
  subscriptionId?: Maybe<Scalars['String']['output']>;
  tier: Scalars['String']['output'];
  userId: Scalars['ID']['output'];
};

export type SymbolAction =
  | 'DELETE_SYMBOL'
  | 'INSERT_AFTER'
  | 'RENAME'
  | 'REPLACE_BODY';

export type SymbolKind =
  | 'CLASS'
  | 'CONST'
  | 'FUNCTION'
  | 'INTERFACE'
  | 'METHOD'
  | 'STRUCT'
  | 'TYPE'
  | 'VAR';

export type SymbolPatchInput = {
  action: SymbolAction;
  author?: InputMaybe<Scalars['String']['input']>;
  kind: SymbolKind;
  name: Scalars['String']['input'];
  newBody?: InputMaybe<Scalars['String']['input']>;
  newDecl?: InputMaybe<Scalars['String']['input']>;
  newName?: InputMaybe<Scalars['String']['input']>;
  path: Scalars['String']['input'];
  projectId: Scalars['ID']['input'];
  receiver?: InputMaybe<Scalars['String']['input']>;
  summary?: InputMaybe<Scalars['String']['input']>;
  title?: InputMaybe<Scalars['String']['input']>;
};

export type TelemetryPreferenceInput = {
  optOut: Scalars['Boolean']['input'];
};

export type UpdateProjectInput = {
  description?: InputMaybe<Scalars['String']['input']>;
  idea?: InputMaybe<Scalars['String']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
  status?: InputMaybe<Scalars['String']['input']>;
};

export type UpdateSamlConfigInput = {
  attributeEmail?: InputMaybe<Scalars['String']['input']>;
  attributeGroups?: InputMaybe<Scalars['String']['input']>;
  audience?: InputMaybe<Scalars['String']['input']>;
  certificate?: InputMaybe<Scalars['String']['input']>;
  defaultRole?: InputMaybe<Scalars['String']['input']>;
  enabled: Scalars['Boolean']['input'];
  idpEntityId?: InputMaybe<Scalars['String']['input']>;
  ssoUrl?: InputMaybe<Scalars['String']['input']>;
};

export type UptimeSample = {
  __typename?: 'UptimeSample';
  latencyMs?: Maybe<Scalars['Int']['output']>;
  ok: Scalars['Boolean']['output'];
  ts: Scalars['DateTime']['output'];
};

export type UptimeSeries = {
  __typename?: 'UptimeSeries';
  samples: Array<UptimeSample>;
  service: Scalars['String']['output'];
  successRatio: Scalars['Float']['output'];
  windowHours: Scalars['Int']['output'];
};

export type User = {
  __typename?: 'User';
  createdAt: Scalars['DateTime']['output'];
  email: Scalars['String']['output'];
  emailVerifiedAt?: Maybe<Scalars['DateTime']['output']>;
  id: Scalars['ID']['output'];
  mfaEnabled: Scalars['Boolean']['output'];
  name?: Maybe<Scalars['String']['output']>;
  orgId?: Maybe<Scalars['String']['output']>;
  plan?: Maybe<Scalars['String']['output']>;
  telemetryOptOut: Scalars['Boolean']['output'];
};

export type VaultSnapshot = {
  __typename?: 'VaultSnapshot';
  asOf: Scalars['DateTime']['output'];
  entries: Scalars['Int']['output'];
  marginUsd: Scalars['Decimal']['output'];
  providerCostUsd: Scalars['Decimal']['output'];
  revenueUsd: Scalars['Decimal']['output'];
};

export type VercelEnvTarget =
  | 'DEVELOPMENT'
  | 'PREVIEW'
  | 'PRODUCTION';

export type VersionInfo = {
  __typename?: 'VersionInfo';
  buildTime: Scalars['String']['output'];
  commit: Scalars['String']['output'];
  service: Scalars['String']['output'];
  version: Scalars['String']['output'];
};

export type VisualDiffInput = {
  livePngBase64: Scalars['String']['input'];
  projectId: Scalars['ID']['input'];
  targetId: Scalars['ID']['input'];
  tolerance?: InputMaybe<Scalars['Float']['input']>;
};

export type VisualDiffResult = {
  __typename?: 'VisualDiffResult';
  diffPngBase64?: Maybe<Scalars['String']['output']>;
  matched: Scalars['Boolean']['output'];
  message?: Maybe<Scalars['String']['output']>;
  ok: Scalars['Boolean']['output'];
  scorePct?: Maybe<Scalars['Float']['output']>;
};

export type VisualTarget = {
  __typename?: 'VisualTarget';
  id: Scalars['ID']['output'];
  imagePngBase64?: Maybe<Scalars['String']['output']>;
  name: Scalars['String']['output'];
  routeHint?: Maybe<Scalars['String']['output']>;
  tolerance: Scalars['Float']['output'];
  viewportH: Scalars['Int']['output'];
  viewportW: Scalars['Int']['output'];
};

export type VoteShare = {
  __typename?: 'VoteShare';
  con: Scalars['Int']['output'];
  neutral: Scalars['Int']['output'];
  pro: Scalars['Int']['output'];
};

export type WebhookDelivery = {
  __typename?: 'WebhookDelivery';
  attempts: Scalars['Int']['output'];
  createdAt: Scalars['DateTime']['output'];
  deliveredAt?: Maybe<Scalars['DateTime']['output']>;
  event: Scalars['String']['output'];
  id: Scalars['ID']['output'];
  lastError?: Maybe<Scalars['String']['output']>;
  payload?: Maybe<Scalars['JSON']['output']>;
  status: WebhookDeliveryStatus;
  subscriptionId: Scalars['ID']['output'];
};

export type WebhookDeliveryStatus =
  | 'DEAD_LETTER'
  | 'DELIVERED'
  | 'FAILED'
  | 'PENDING'
  | 'RETRYING';

export type WebhookSubscription = {
  __typename?: 'WebhookSubscription';
  createdAt: Scalars['DateTime']['output'];
  enabled: Scalars['Boolean']['output'];
  events: Array<Scalars['String']['output']>;
  id: Scalars['ID']['output'];
  lastDeliveryAt?: Maybe<Scalars['DateTime']['output']>;
  secret?: Maybe<Scalars['String']['output']>;
  url: Scalars['String']['output'];
  userId: Scalars['ID']['output'];
};

export type WebhookTestResult = {
  __typename?: 'WebhookTestResult';
  body?: Maybe<Scalars['String']['output']>;
  ok: Scalars['Boolean']['output'];
  status: Scalars['Int']['output'];
};

export type Workspace = {
  __typename?: 'Workspace';
  createdAt?: Maybe<Scalars['DateTime']['output']>;
  driver: Scalars['String']['output'];
  id: Scalars['ID']['output'];
  projectId?: Maybe<Scalars['ID']['output']>;
  status: Scalars['String']['output'];
  updatedAt?: Maybe<Scalars['DateTime']['output']>;
};

export type WorkspaceFile = {
  __typename?: 'WorkspaceFile';
  isDir: Scalars['Boolean']['output'];
  modifiedAt?: Maybe<Scalars['DateTime']['output']>;
  path: Scalars['String']['output'];
  size: Scalars['Int']['output'];
};

export type WorkspaceFileContent = {
  __typename?: 'WorkspaceFileContent';
  bytes: Scalars['Int']['output'];
  content: Scalars['String']['output'];
  encoding: Scalars['String']['output'];
  path: Scalars['String']['output'];
};

export type UserFieldsFragment = { __typename?: 'User', id: string, email: string, name?: string | null, plan?: string | null, orgId?: string | null, telemetryOptOut: boolean, createdAt: string };

export type SessionFieldsFragment = { __typename?: 'Session', token: string, expiresAt?: string | null, user: { __typename?: 'User', id: string, email: string, name?: string | null, plan?: string | null, orgId?: string | null, telemetryOptOut: boolean, createdAt: string } };

export type GateVerdictFieldsFragment = { __typename?: 'GateVerdict', gate: string, status: GateStatus, startedAt?: string | null, finishedAt?: string | null, durationMs?: number | null, notes?: string | null, issues: Array<{ __typename?: 'GateIssue', path?: string | null, line?: number | null, rule?: string | null, severity?: string | null, message: string }> };

export type ProjectFieldsFragment = { __typename?: 'Project', id: string, name: string, description?: string | null, status: string, ownerId: string, isPublic: boolean, idea?: string | null, createdAt: string, updatedAt: string, github?: { __typename?: 'GithubLink', owner: string, repo: string, fullName: string, defaultBranch?: string | null, htmlUrl?: string | null } | null };

export type PatchFieldsFragment = { __typename?: 'Patch', id: string, projectId: string, title?: string | null, summary?: string | null, author?: string | null, status: PatchStatus, createdAt: string, appliedAt?: string | null, changes: Array<{ __typename?: 'PatchChange', op: PatchChangeOp, path: string, content?: string | null, anchor?: string | null, replacement?: string | null, symbol?: string | null }> };

export type ChatFieldsFragment = { __typename?: 'Chat', id: string, projectId: string, title?: string | null, parentChatId?: string | null, messageCount: number, createdAt: string, updatedAt: string };

export type ChatMessageFieldsFragment = { __typename?: 'ChatMessage', id: string, chatId: string, role: string, text?: string | null, toolUse?: unknown | null, createdAt: string, provider?: string | null, model?: string | null, usage?: unknown | null, attachments?: Array<{ __typename?: 'ChatAttachment', mediaType: string, base64: string }> | null };

export type WorkspaceFieldsFragment = { __typename?: 'Workspace', id: string, projectId?: string | null, driver: string, status: string, createdAt?: string | null, updatedAt?: string | null };

export type DeployFieldsFragment = { __typename?: 'Deploy', id: string, projectId: string, target: DeployTarget, targetMeta?: unknown | null, status: DeployStatus, url?: string | null, startedAt: string, finishedAt?: string | null, durationMs?: number | null, artifact?: unknown | null, log: Array<{ __typename?: 'DeployLogLine', ts: string, level: string, message: string }> };

export type MemoryRecordFieldsFragment = { __typename?: 'MemoryRecord', id: string, kind: MemoryKind, userId?: string | null, projectId?: string | null, storyId?: string | null, gateName?: string | null, title?: string | null, body: string, tags?: Array<string> | null, createdAt: string };

export type AuditEntryFieldsFragment = { __typename?: 'AuditEntry', id: string, ts: string, userId?: string | null, projectId?: string | null, action: string, outcome: AuditOutcome, ok: boolean, actor?: string | null, resource?: string | null, storyId?: string | null, gateName?: string | null, agentRole?: string | null, summary?: string | null, hash: string, prevHash?: string | null, inputHash?: string | null, outputHash?: string | null, payload?: unknown | null };

export type WebhookFieldsFragment = { __typename?: 'WebhookSubscription', id: string, userId: string, url: string, events: Array<string>, secret?: string | null, enabled: boolean, createdAt: string, lastDeliveryAt?: string | null };

export type MeQueryVariables = Exact<{ [key: string]: never; }>;


export type MeQuery = { __typename?: 'Query', me?: { __typename?: 'User', id: string, email: string, name?: string | null, plan?: string | null, orgId?: string | null, telemetryOptOut: boolean, createdAt: string } | null };

export type SignInMutationVariables = Exact<{
  input: SignInInput;
}>;


export type SignInMutation = { __typename?: 'Mutation', signIn: { __typename?: 'Session', token: string, expiresAt?: string | null, user: { __typename?: 'User', id: string, email: string, name?: string | null, plan?: string | null, orgId?: string | null, telemetryOptOut: boolean, createdAt: string } } };

export type SignUpMutationVariables = Exact<{
  input: SignUpInput;
}>;


export type SignUpMutation = { __typename?: 'Mutation', signUp: { __typename?: 'Session', token: string, expiresAt?: string | null, user: { __typename?: 'User', id: string, email: string, name?: string | null, plan?: string | null, orgId?: string | null, telemetryOptOut: boolean, createdAt: string } } };

export type SignOutMutationVariables = Exact<{ [key: string]: never; }>;


export type SignOutMutation = { __typename?: 'Mutation', signOut: { __typename?: 'OperationResult', ok: boolean } };

export type ProjectsQueryVariables = Exact<{ [key: string]: never; }>;


export type ProjectsQuery = { __typename?: 'Query', projects: Array<{ __typename?: 'Project', id: string, name: string, description?: string | null, status: string, ownerId: string, isPublic: boolean, idea?: string | null, createdAt: string, updatedAt: string, github?: { __typename?: 'GithubLink', owner: string, repo: string, fullName: string, defaultBranch?: string | null, htmlUrl?: string | null } | null }> };

export type ProjectQueryVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type ProjectQuery = { __typename?: 'Query', project?: { __typename?: 'Project', id: string, name: string, description?: string | null, status: string, ownerId: string, isPublic: boolean, idea?: string | null, createdAt: string, updatedAt: string, files: Array<{ __typename?: 'ProjectFile', path: string, size?: number | null, language?: string | null, updatedAt?: string | null }>, gates: Array<{ __typename?: 'GateVerdict', gate: string, status: GateStatus, startedAt?: string | null, finishedAt?: string | null, durationMs?: number | null, notes?: string | null, issues: Array<{ __typename?: 'GateIssue', path?: string | null, line?: number | null, rule?: string | null, severity?: string | null, message: string }> }>, github?: { __typename?: 'GithubLink', owner: string, repo: string, fullName: string, defaultBranch?: string | null, htmlUrl?: string | null } | null } | null };

export type ProjectFilesQueryVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type ProjectFilesQuery = { __typename?: 'Query', projectFiles: Array<{ __typename?: 'ProjectFile', path: string, content?: string | null, size?: number | null, language?: string | null, updatedAt?: string | null }> };

export type ProjectGraphQueryVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type ProjectGraphQuery = { __typename?: 'Query', projectGraph: { __typename?: 'ProjectGraph', nodes: Array<{ __typename?: 'ProjectGraphNode', id: string, path: string, language?: string | null, size?: number | null }>, edges: Array<{ __typename?: 'ProjectGraphEdge', from: string, to: string, kind: string }> } };

export type ProjectSnapshotQueryVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type ProjectSnapshotQuery = { __typename?: 'Query', projectSnapshot: unknown };

export type SearchProjectCodeQueryVariables = Exact<{
  id: Scalars['ID']['input'];
  q: Scalars['String']['input'];
  k?: InputMaybe<Scalars['Int']['input']>;
  maxKb?: InputMaybe<Scalars['Int']['input']>;
}>;


export type SearchProjectCodeQuery = { __typename?: 'Query', searchProjectCode: Array<{ __typename?: 'CodeSearchHit', path: string, startLine: number, endLine: number, symbols?: Array<string> | null, score: number, text: string }> };

export type CreateProjectMutationVariables = Exact<{
  input: CreateProjectInput;
}>;


export type CreateProjectMutation = { __typename?: 'Mutation', createProject: { __typename?: 'Project', id: string, name: string, description?: string | null, status: string, ownerId: string, isPublic: boolean, idea?: string | null, createdAt: string, updatedAt: string, github?: { __typename?: 'GithubLink', owner: string, repo: string, fullName: string, defaultBranch?: string | null, htmlUrl?: string | null } | null } };

export type UpdateProjectMutationVariables = Exact<{
  id: Scalars['ID']['input'];
  input: UpdateProjectInput;
}>;


export type UpdateProjectMutation = { __typename?: 'Mutation', updateProject: { __typename?: 'Project', id: string, name: string, description?: string | null, status: string, ownerId: string, isPublic: boolean, idea?: string | null, createdAt: string, updatedAt: string, github?: { __typename?: 'GithubLink', owner: string, repo: string, fullName: string, defaultBranch?: string | null, htmlUrl?: string | null } | null } };

export type DeleteProjectMutationVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type DeleteProjectMutation = { __typename?: 'Mutation', deleteProject: { __typename?: 'OperationResult', ok: boolean } };

export type BulkDeleteProjectsMutationVariables = Exact<{
  ids: Array<Scalars['ID']['input']> | Scalars['ID']['input'];
}>;


export type BulkDeleteProjectsMutation = { __typename?: 'Mutation', bulkDeleteProjects: { __typename?: 'OperationResult', ok: boolean } };

export type GatesQueryVariables = Exact<{
  projectId: Scalars['ID']['input'];
  sub?: InputMaybe<Scalars['ID']['input']>;
}>;


export type GatesQuery = { __typename?: 'Query', gates: Array<{ __typename?: 'GateVerdict', gate: string, status: GateStatus, startedAt?: string | null, finishedAt?: string | null, durationMs?: number | null, notes?: string | null, issues: Array<{ __typename?: 'GateIssue', path?: string | null, line?: number | null, rule?: string | null, severity?: string | null, message: string }> }> };

export type RerunGateMutationVariables = Exact<{
  input: RerunGateInput;
}>;


export type RerunGateMutation = { __typename?: 'Mutation', rerunGate: { __typename?: 'GateVerdict', gate: string, status: GateStatus, startedAt?: string | null, finishedAt?: string | null, durationMs?: number | null, notes?: string | null, issues: Array<{ __typename?: 'GateIssue', path?: string | null, line?: number | null, rule?: string | null, severity?: string | null, message: string }> } };

export type RunFinisherMutationVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type RunFinisherMutation = { __typename?: 'Mutation', runFinisher: unknown };

export type PatchesQueryVariables = Exact<{
  projectId: Scalars['ID']['input'];
}>;


export type PatchesQuery = { __typename?: 'Query', patches: Array<{ __typename?: 'Patch', id: string, projectId: string, title?: string | null, summary?: string | null, author?: string | null, status: PatchStatus, createdAt: string, appliedAt?: string | null, changes: Array<{ __typename?: 'PatchChange', op: PatchChangeOp, path: string, content?: string | null, anchor?: string | null, replacement?: string | null, symbol?: string | null }> }> };

export type ProposePatchMutationVariables = Exact<{
  input: ProposePatchInput;
}>;


export type ProposePatchMutation = { __typename?: 'Mutation', proposePatch: { __typename?: 'Patch', id: string, projectId: string, title?: string | null, summary?: string | null, author?: string | null, status: PatchStatus, createdAt: string, appliedAt?: string | null, changes: Array<{ __typename?: 'PatchChange', op: PatchChangeOp, path: string, content?: string | null, anchor?: string | null, replacement?: string | null, symbol?: string | null }> } };

export type ApplyPatchMutationVariables = Exact<{
  patchId: Scalars['ID']['input'];
}>;


export type ApplyPatchMutation = { __typename?: 'Mutation', applyPatch: { __typename?: 'Patch', id: string, projectId: string, title?: string | null, summary?: string | null, author?: string | null, status: PatchStatus, createdAt: string, appliedAt?: string | null, changes: Array<{ __typename?: 'PatchChange', op: PatchChangeOp, path: string, content?: string | null, anchor?: string | null, replacement?: string | null, symbol?: string | null }> } };

export type RollbackPatchMutationVariables = Exact<{
  patchId: Scalars['ID']['input'];
}>;


export type RollbackPatchMutation = { __typename?: 'Mutation', rollbackPatch: unknown };

export type MyBudgetQueryVariables = Exact<{ [key: string]: never; }>;


export type MyBudgetQuery = { __typename?: 'Query', myBudget: { __typename?: 'BudgetSummary', userId: string, email: string, tier: string, spentUsd: string, entries: Array<{ __typename?: 'LedgerEntry', id: string, provider?: string | null, model?: string | null, promptTokens: number, completionTokens: number, costUsd: string, revenueUsd: string, ts: string, agent?: string | null, durationMs?: number | null }> } };

export type PlansQueryVariables = Exact<{ [key: string]: never; }>;


export type PlansQuery = { __typename?: 'Query', plans: Array<{ __typename?: 'Plan', tier: string, name: string, priceUsd: string, costCapUsd: string, description?: string | null, features: Array<string>, stripePriceId?: string | null }> };

export type RatesQueryVariables = Exact<{ [key: string]: never; }>;


export type RatesQuery = { __typename?: 'Query', rates: Array<{ __typename?: 'Rate', provider: string, model: string, promptPerMTok: string, completionPerMTok: string }> };

export type VaultQueryVariables = Exact<{ [key: string]: never; }>;


export type VaultQuery = { __typename?: 'Query', vault: { __typename?: 'VaultSnapshot', revenueUsd: string, providerCostUsd: string, marginUsd: string, entries: number, asOf: string } };

export type MySubscriptionQueryVariables = Exact<{ [key: string]: never; }>;


export type MySubscriptionQuery = { __typename?: 'Query', mySubscription?: { __typename?: 'Subscription_Stripe', userId: string, tier: string, customerId?: string | null, subscriptionId?: string | null, status?: string | null, currentPeriodEnd?: string | null, cancelAtPeriodEnd?: boolean | null } | null };

export type StartCheckoutMutationVariables = Exact<{
  input: StartCheckoutInput;
}>;


export type StartCheckoutMutation = { __typename?: 'Mutation', startCheckout: { __typename?: 'StripeCheckoutSession', sessionId: string, url: string } };

export type CancelSubscriptionMutationVariables = Exact<{
  input: CancelSubscriptionInput;
}>;


export type CancelSubscriptionMutation = { __typename?: 'Mutation', cancelSubscription: { __typename?: 'OperationResult', ok: boolean } };

export type MemoryQueryVariables = Exact<{
  query?: InputMaybe<MemoryQueryInput>;
}>;


export type MemoryQuery = { __typename?: 'Query', memory: Array<{ __typename?: 'MemoryRecord', id: string, kind: MemoryKind, userId?: string | null, projectId?: string | null, storyId?: string | null, gateName?: string | null, title?: string | null, body: string, tags?: Array<string> | null, createdAt: string }> };

export type AddMemoryMutationVariables = Exact<{
  input: AddMemoryInput;
}>;


export type AddMemoryMutation = { __typename?: 'Mutation', addMemory: { __typename?: 'MemoryRecord', id: string, kind: MemoryKind, userId?: string | null, projectId?: string | null, storyId?: string | null, gateName?: string | null, title?: string | null, body: string, tags?: Array<string> | null, createdAt: string } };

export type DeleteMemoryMutationVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type DeleteMemoryMutation = { __typename?: 'Mutation', deleteMemory: { __typename?: 'OperationResult', ok: boolean } };

export type AuditQueryVariables = Exact<{
  query?: InputMaybe<AuditQueryInput>;
}>;


export type AuditQuery = { __typename?: 'Query', audit: Array<{ __typename?: 'AuditEntry', id: string, ts: string, userId?: string | null, projectId?: string | null, action: string, outcome: AuditOutcome, ok: boolean, actor?: string | null, resource?: string | null, storyId?: string | null, gateName?: string | null, agentRole?: string | null, summary?: string | null, hash: string, prevHash?: string | null, inputHash?: string | null, outputHash?: string | null, payload?: unknown | null }> };

export type VerifyAuditQueryVariables = Exact<{ [key: string]: never; }>;


export type VerifyAuditQuery = { __typename?: 'Query', verifyAudit: { __typename?: 'AuditVerifyResult', intact: boolean, firstBadIndex: number } };

export type WebhooksQueryVariables = Exact<{ [key: string]: never; }>;


export type WebhooksQuery = { __typename?: 'Query', webhooks: Array<{ __typename?: 'WebhookSubscription', id: string, userId: string, url: string, events: Array<string>, secret?: string | null, enabled: boolean, createdAt: string, lastDeliveryAt?: string | null }> };

export type CreateWebhookMutationVariables = Exact<{
  input: CreateWebhookInput;
}>;


export type CreateWebhookMutation = { __typename?: 'Mutation', createWebhook: { __typename?: 'WebhookSubscription', id: string, userId: string, url: string, events: Array<string>, secret?: string | null, enabled: boolean, createdAt: string, lastDeliveryAt?: string | null } };

export type DeleteWebhookMutationVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type DeleteWebhookMutation = { __typename?: 'Mutation', deleteWebhook: { __typename?: 'OperationResult', ok: boolean } };

export type TestWebhookMutationVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type TestWebhookMutation = { __typename?: 'Mutation', testWebhook: { __typename?: 'WebhookTestResult', ok: boolean, status: number, body?: string | null } };

export type DeploysQueryVariables = Exact<{
  projectId: Scalars['ID']['input'];
}>;


export type DeploysQuery = { __typename?: 'Query', deploys: Array<{ __typename?: 'Deploy', id: string, projectId: string, target: DeployTarget, targetMeta?: unknown | null, status: DeployStatus, url?: string | null, startedAt: string, finishedAt?: string | null, durationMs?: number | null, artifact?: unknown | null, log: Array<{ __typename?: 'DeployLogLine', ts: string, level: string, message: string }> }> };

export type StartDeployMutationVariables = Exact<{
  input: StartDeployInput;
}>;


export type StartDeployMutation = { __typename?: 'Mutation', startDeploy: { __typename?: 'Deploy', id: string, projectId: string, target: DeployTarget, targetMeta?: unknown | null, status: DeployStatus, url?: string | null, startedAt: string, finishedAt?: string | null, durationMs?: number | null, artifact?: unknown | null, log: Array<{ __typename?: 'DeployLogLine', ts: string, level: string, message: string }> } };

export type AcceptInlineCompletionMutationVariables = Exact<{
  requestId: Scalars['ID']['input'];
}>;


export type AcceptInlineCompletionMutation = { __typename?: 'Mutation', acceptInlineCompletion: { __typename?: 'OperationResult', ok: boolean } };

export type ChatsQueryVariables = Exact<{
  projectId: Scalars['ID']['input'];
}>;


export type ChatsQuery = { __typename?: 'Query', chats: Array<{ __typename?: 'Chat', id: string, projectId: string, title?: string | null, parentChatId?: string | null, messageCount: number, createdAt: string, updatedAt: string }> };

export type ChatMessagesQueryVariables = Exact<{
  chatId: Scalars['ID']['input'];
}>;


export type ChatMessagesQuery = { __typename?: 'Query', chatMessages: Array<{ __typename?: 'ChatMessage', id: string, chatId: string, role: string, text?: string | null, toolUse?: unknown | null, createdAt: string, provider?: string | null, model?: string | null, usage?: unknown | null, attachments?: Array<{ __typename?: 'ChatAttachment', mediaType: string, base64: string }> | null }> };

export type CreateChatMutationVariables = Exact<{
  input: CreateChatInput;
}>;


export type CreateChatMutation = { __typename?: 'Mutation', createChat: { __typename?: 'Chat', id: string, projectId: string, title?: string | null, parentChatId?: string | null, messageCount: number, createdAt: string, updatedAt: string } };

export type ForkChatMutationVariables = Exact<{
  input: ForkChatInput;
}>;


export type ForkChatMutation = { __typename?: 'Mutation', forkChat: { __typename?: 'Chat', id: string, projectId: string, title?: string | null, parentChatId?: string | null, messageCount: number, createdAt: string, updatedAt: string } };

export type WorkspacesQueryVariables = Exact<{
  projectId: Scalars['ID']['input'];
}>;


export type WorkspacesQuery = { __typename?: 'Query', workspaces: Array<{ __typename?: 'Workspace', id: string, projectId?: string | null, driver: string, status: string, createdAt?: string | null, updatedAt?: string | null }> };

export type WorkspaceQueryVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type WorkspaceQuery = { __typename?: 'Query', workspace?: { __typename?: 'Workspace', id: string, projectId?: string | null, driver: string, status: string, createdAt?: string | null, updatedAt?: string | null } | null };

export type WorkspaceFilesQueryVariables = Exact<{
  workspaceId: Scalars['ID']['input'];
  path?: InputMaybe<Scalars['String']['input']>;
}>;


export type WorkspaceFilesQuery = { __typename?: 'Query', workspaceFiles: Array<{ __typename?: 'WorkspaceFile', path: string, size: number, isDir: boolean, modifiedAt?: string | null }> };

export type WorkspaceFileQueryVariables = Exact<{
  workspaceId: Scalars['ID']['input'];
  path: Scalars['String']['input'];
}>;


export type WorkspaceFileQuery = { __typename?: 'Query', workspaceFile?: { __typename?: 'WorkspaceFileContent', path: string, content: string, bytes: number, encoding: string } | null };

export type CreateWorkspaceMutationVariables = Exact<{
  projectId: Scalars['ID']['input'];
  driver?: InputMaybe<Scalars['String']['input']>;
}>;


export type CreateWorkspaceMutation = { __typename?: 'Mutation', createWorkspace: { __typename?: 'Workspace', id: string, projectId?: string | null, driver: string, status: string, createdAt?: string | null, updatedAt?: string | null } };

export type StartWorkspaceMutationVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type StartWorkspaceMutation = { __typename?: 'Mutation', startWorkspace: { __typename?: 'Workspace', id: string, projectId?: string | null, driver: string, status: string, createdAt?: string | null, updatedAt?: string | null } };

export type StopWorkspaceMutationVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type StopWorkspaceMutation = { __typename?: 'Mutation', stopWorkspace: { __typename?: 'OperationResult', ok: boolean } };

export type WriteWorkspaceFileMutationVariables = Exact<{
  workspaceId: Scalars['ID']['input'];
  path: Scalars['String']['input'];
  content: Scalars['String']['input'];
}>;


export type WriteWorkspaceFileMutation = { __typename?: 'Mutation', writeWorkspaceFile: { __typename?: 'OperationResult', ok: boolean } };

export type ExecInWorkspaceMutationVariables = Exact<{
  workspaceId: Scalars['ID']['input'];
  command: Scalars['String']['input'];
  timeoutSec?: InputMaybe<Scalars['Int']['input']>;
}>;


export type ExecInWorkspaceMutation = { __typename?: 'Mutation', execInWorkspace: { __typename?: 'ExecResult', exitCode: number, stdout: string, stderr: string, durMs: number, timedOut: boolean } };

export type RunProjectSubscriptionVariables = Exact<{
  projectId: Scalars['ID']['input'];
}>;


export type RunProjectSubscription = { __typename?: 'Subscription', runProject: { __typename: 'RunDoneEvent', ts: string, ok: boolean } | { __typename: 'RunErrorEvent', ts: string, code: string, message: string } | { __typename: 'RunExecutionEvent', ts: string, payload: unknown } | { __typename: 'RunGateEvent', ts: string, gate: string, status: string, gateMessage?: string | null } };

export type ChatStreamSubscriptionVariables = Exact<{
  projectId: Scalars['ID']['input'];
  input: ChatInput;
}>;


export type ChatStreamSubscription = { __typename?: 'Subscription', chatStream: { __typename: 'ChatDoneDelta', usage?: unknown | null, doneTurnId: string, doneProvider?: string | null, doneModel?: string | null } | { __typename: 'ChatErrorDelta', code: string, message: string, errorTurnId?: string | null } | { __typename: 'ChatStartDelta', startTurnId: string, startProvider: string, startModel: string } | { __typename: 'ChatTextDelta', text: string, textTurnId: string } | { __typename: 'ChatThinkingDelta', thinkingTurnId: string, thinkingText: string } | { __typename: 'ChatToolUseDelta', toolUse: unknown, toolUseTurnId: string } };

export type InlineCompletionSubscriptionVariables = Exact<{
  input: InlineInput;
}>;


export type InlineCompletionSubscription = { __typename?: 'Subscription', inlineCompletion: { __typename: 'InlineCancelledDelta', reason?: string | null, cancelledRequestId: string } | { __typename: 'InlineDoneDelta', usage?: unknown | null, doneRequestId: string, doneProvider?: string | null, doneModel?: string | null } | { __typename: 'InlineErrorDelta', code: string, message: string, errorRequestId?: string | null } | { __typename: 'InlineStartDelta', startRequestId: string, startProvider: string, startModel: string } | { __typename: 'InlineTextDelta', text: string, textRequestId: string } };

export type DeployStreamSubscriptionVariables = Exact<{
  deployId: Scalars['ID']['input'];
}>;


export type DeployStreamSubscription = { __typename?: 'Subscription', deployStream: { __typename: 'DeployBuildLogLine', source: string, buildTs: string, buildLine: string } | { __typename: 'DeployErrorEvent', code: string, message: string, errorDeployId: string } | { __typename: 'DeployFinishedEvent', url?: string | null, durationMs?: number | null, finishedDeployId: string, finishedStatus: DeployStatus } | { __typename: 'DeployLogEvent', logDeployId: string, logLine: { __typename?: 'DeployLogLine', ts: string, level: string, message: string } } | { __typename: 'DeployStateEvent', deployId: string, status: DeployStatus, ts: string } };

export type WorkspacePtySubscriptionVariables = Exact<{
  workspaceId: Scalars['ID']['input'];
}>;


export type WorkspacePtySubscription = { __typename?: 'Subscription', workspacePty: { __typename: 'PtyExit', code: number } | { __typename: 'PtyOutput', data: string } };

export type CostStreamSubscriptionVariables = Exact<{ [key: string]: never; }>;


export type CostStreamSubscription = { __typename?: 'Subscription', costStream: { __typename?: 'CostDelta', ts: string, usdSpent: string, model?: string | null, provider?: string | null, agent?: string | null, durationMs?: number | null } };

export const UserFieldsFragmentDoc = gql`
    fragment UserFields on User {
  id
  email
  name
  plan
  orgId
  telemetryOptOut
  createdAt
}
    `;
export const SessionFieldsFragmentDoc = gql`
    fragment SessionFields on Session {
  token
  expiresAt
  user {
    ...UserFields
  }
}
    ${UserFieldsFragmentDoc}`;
export const GateVerdictFieldsFragmentDoc = gql`
    fragment GateVerdictFields on GateVerdict {
  gate
  status
  startedAt
  finishedAt
  durationMs
  notes
  issues {
    path
    line
    rule
    severity
    message
  }
}
    `;
export const ProjectFieldsFragmentDoc = gql`
    fragment ProjectFields on Project {
  id
  name
  description
  status
  ownerId
  isPublic
  idea
  createdAt
  updatedAt
  github {
    owner
    repo
    fullName
    defaultBranch
    htmlUrl
  }
}
    `;
export const PatchFieldsFragmentDoc = gql`
    fragment PatchFields on Patch {
  id
  projectId
  title
  summary
  author
  status
  createdAt
  appliedAt
  changes {
    op
    path
    content
    anchor
    replacement
    symbol
  }
}
    `;
export const ChatFieldsFragmentDoc = gql`
    fragment ChatFields on Chat {
  id
  projectId
  title
  parentChatId
  messageCount
  createdAt
  updatedAt
}
    `;
export const ChatMessageFieldsFragmentDoc = gql`
    fragment ChatMessageFields on ChatMessage {
  id
  chatId
  role
  text
  toolUse
  createdAt
  provider
  model
  usage
  attachments {
    mediaType
    base64
  }
}
    `;
export const WorkspaceFieldsFragmentDoc = gql`
    fragment WorkspaceFields on Workspace {
  id
  projectId
  driver
  status
  createdAt
  updatedAt
}
    `;
export const DeployFieldsFragmentDoc = gql`
    fragment DeployFields on Deploy {
  id
  projectId
  target
  targetMeta
  status
  url
  startedAt
  finishedAt
  durationMs
  artifact
  log {
    ts
    level
    message
  }
}
    `;
export const MemoryRecordFieldsFragmentDoc = gql`
    fragment MemoryRecordFields on MemoryRecord {
  id
  kind
  userId
  projectId
  storyId
  gateName
  title
  body
  tags
  createdAt
}
    `;
export const AuditEntryFieldsFragmentDoc = gql`
    fragment AuditEntryFields on AuditEntry {
  id
  ts
  userId
  projectId
  action
  outcome
  ok
  actor
  resource
  storyId
  gateName
  agentRole
  summary
  hash
  prevHash
  inputHash
  outputHash
  payload
}
    `;
export const WebhookFieldsFragmentDoc = gql`
    fragment WebhookFields on WebhookSubscription {
  id
  userId
  url
  events
  secret
  enabled
  createdAt
  lastDeliveryAt
}
    `;
export const MeDocument = gql`
    query Me {
  me {
    ...UserFields
  }
}
    ${UserFieldsFragmentDoc}`;
export const SignInDocument = gql`
    mutation SignIn($input: SignInInput!) {
  signIn(input: $input) {
    ...SessionFields
  }
}
    ${SessionFieldsFragmentDoc}`;
export const SignUpDocument = gql`
    mutation SignUp($input: SignUpInput!) {
  signUp(input: $input) {
    ...SessionFields
  }
}
    ${SessionFieldsFragmentDoc}`;
export const SignOutDocument = gql`
    mutation SignOut {
  signOut {
    ok
  }
}
    `;
export const ProjectsDocument = gql`
    query Projects {
  projects {
    ...ProjectFields
  }
}
    ${ProjectFieldsFragmentDoc}`;
export const ProjectDocument = gql`
    query Project($id: ID!) {
  project(id: $id) {
    ...ProjectFields
    files {
      path
      size
      language
      updatedAt
    }
    gates {
      ...GateVerdictFields
    }
  }
}
    ${ProjectFieldsFragmentDoc}
${GateVerdictFieldsFragmentDoc}`;
export const ProjectFilesDocument = gql`
    query ProjectFiles($id: ID!) {
  projectFiles(id: $id) {
    path
    content
    size
    language
    updatedAt
  }
}
    `;
export const ProjectGraphDocument = gql`
    query ProjectGraph($id: ID!) {
  projectGraph(id: $id) {
    nodes {
      id
      path
      language
      size
    }
    edges {
      from
      to
      kind
    }
  }
}
    `;
export const ProjectSnapshotDocument = gql`
    query ProjectSnapshot($id: ID!) {
  projectSnapshot(id: $id)
}
    `;
export const SearchProjectCodeDocument = gql`
    query SearchProjectCode($id: ID!, $q: String!, $k: Int, $maxKb: Int) {
  searchProjectCode(id: $id, q: $q, k: $k, maxKb: $maxKb) {
    path
    startLine
    endLine
    symbols
    score
    text
  }
}
    `;
export const CreateProjectDocument = gql`
    mutation CreateProject($input: CreateProjectInput!) {
  createProject(input: $input) {
    ...ProjectFields
  }
}
    ${ProjectFieldsFragmentDoc}`;
export const UpdateProjectDocument = gql`
    mutation UpdateProject($id: ID!, $input: UpdateProjectInput!) {
  updateProject(id: $id, input: $input) {
    ...ProjectFields
  }
}
    ${ProjectFieldsFragmentDoc}`;
export const DeleteProjectDocument = gql`
    mutation DeleteProject($id: ID!) {
  deleteProject(id: $id) {
    ok
  }
}
    `;
export const BulkDeleteProjectsDocument = gql`
    mutation BulkDeleteProjects($ids: [ID!]!) {
  bulkDeleteProjects(ids: $ids) {
    ok
  }
}
    `;
export const GatesDocument = gql`
    query Gates($projectId: ID!, $sub: ID) {
  gates(projectId: $projectId, sub: $sub) {
    ...GateVerdictFields
  }
}
    ${GateVerdictFieldsFragmentDoc}`;
export const RerunGateDocument = gql`
    mutation RerunGate($input: RerunGateInput!) {
  rerunGate(input: $input) {
    ...GateVerdictFields
  }
}
    ${GateVerdictFieldsFragmentDoc}`;
export const RunFinisherDocument = gql`
    mutation RunFinisher($id: ID!) {
  runFinisher(id: $id)
}
    `;
export const PatchesDocument = gql`
    query Patches($projectId: ID!) {
  patches(projectId: $projectId) {
    ...PatchFields
  }
}
    ${PatchFieldsFragmentDoc}`;
export const ProposePatchDocument = gql`
    mutation ProposePatch($input: ProposePatchInput!) {
  proposePatch(input: $input) {
    ...PatchFields
  }
}
    ${PatchFieldsFragmentDoc}`;
export const ApplyPatchDocument = gql`
    mutation ApplyPatch($patchId: ID!) {
  applyPatch(patchId: $patchId) {
    ...PatchFields
  }
}
    ${PatchFieldsFragmentDoc}`;
export const RollbackPatchDocument = gql`
    mutation RollbackPatch($patchId: ID!) {
  rollbackPatch(patchId: $patchId)
}
    `;
export const MyBudgetDocument = gql`
    query MyBudget {
  myBudget {
    userId
    email
    tier
    spentUsd
    entries {
      id
      provider
      model
      promptTokens
      completionTokens
      costUsd
      revenueUsd
      ts
      agent
      durationMs
    }
  }
}
    `;
export const PlansDocument = gql`
    query Plans {
  plans {
    tier
    name
    priceUsd
    costCapUsd
    description
    features
    stripePriceId
  }
}
    `;
export const RatesDocument = gql`
    query Rates {
  rates {
    provider
    model
    promptPerMTok
    completionPerMTok
  }
}
    `;
export const VaultDocument = gql`
    query Vault {
  vault {
    revenueUsd
    providerCostUsd
    marginUsd
    entries
    asOf
  }
}
    `;
export const MySubscriptionDocument = gql`
    query MySubscription {
  mySubscription {
    userId
    tier
    customerId
    subscriptionId
    status
    currentPeriodEnd
    cancelAtPeriodEnd
  }
}
    `;
export const StartCheckoutDocument = gql`
    mutation StartCheckout($input: StartCheckoutInput!) {
  startCheckout(input: $input) {
    sessionId
    url
  }
}
    `;
export const CancelSubscriptionDocument = gql`
    mutation CancelSubscription($input: CancelSubscriptionInput!) {
  cancelSubscription(input: $input) {
    ok
  }
}
    `;
export const MemoryDocument = gql`
    query Memory($query: MemoryQueryInput) {
  memory(query: $query) {
    ...MemoryRecordFields
  }
}
    ${MemoryRecordFieldsFragmentDoc}`;
export const AddMemoryDocument = gql`
    mutation AddMemory($input: AddMemoryInput!) {
  addMemory(input: $input) {
    ...MemoryRecordFields
  }
}
    ${MemoryRecordFieldsFragmentDoc}`;
export const DeleteMemoryDocument = gql`
    mutation DeleteMemory($id: ID!) {
  deleteMemory(id: $id) {
    ok
  }
}
    `;
export const AuditDocument = gql`
    query Audit($query: AuditQueryInput) {
  audit(query: $query) {
    ...AuditEntryFields
  }
}
    ${AuditEntryFieldsFragmentDoc}`;
export const VerifyAuditDocument = gql`
    query VerifyAudit {
  verifyAudit {
    intact
    firstBadIndex
  }
}
    `;
export const WebhooksDocument = gql`
    query Webhooks {
  webhooks {
    ...WebhookFields
  }
}
    ${WebhookFieldsFragmentDoc}`;
export const CreateWebhookDocument = gql`
    mutation CreateWebhook($input: CreateWebhookInput!) {
  createWebhook(input: $input) {
    ...WebhookFields
  }
}
    ${WebhookFieldsFragmentDoc}`;
export const DeleteWebhookDocument = gql`
    mutation DeleteWebhook($id: ID!) {
  deleteWebhook(id: $id) {
    ok
  }
}
    `;
export const TestWebhookDocument = gql`
    mutation TestWebhook($id: ID!) {
  testWebhook(id: $id) {
    ok
    status
    body
  }
}
    `;
export const DeploysDocument = gql`
    query Deploys($projectId: ID!) {
  deploys(projectId: $projectId) {
    ...DeployFields
  }
}
    ${DeployFieldsFragmentDoc}`;
export const StartDeployDocument = gql`
    mutation StartDeploy($input: StartDeployInput!) {
  startDeploy(input: $input) {
    ...DeployFields
  }
}
    ${DeployFieldsFragmentDoc}`;
export const AcceptInlineCompletionDocument = gql`
    mutation AcceptInlineCompletion($requestId: ID!) {
  acceptInlineCompletion(requestId: $requestId) {
    ok
  }
}
    `;
export const ChatsDocument = gql`
    query Chats($projectId: ID!) {
  chats(projectId: $projectId) {
    ...ChatFields
  }
}
    ${ChatFieldsFragmentDoc}`;
export const ChatMessagesDocument = gql`
    query ChatMessages($chatId: ID!) {
  chatMessages(chatId: $chatId) {
    ...ChatMessageFields
  }
}
    ${ChatMessageFieldsFragmentDoc}`;
export const CreateChatDocument = gql`
    mutation CreateChat($input: CreateChatInput!) {
  createChat(input: $input) {
    ...ChatFields
  }
}
    ${ChatFieldsFragmentDoc}`;
export const ForkChatDocument = gql`
    mutation ForkChat($input: ForkChatInput!) {
  forkChat(input: $input) {
    ...ChatFields
  }
}
    ${ChatFieldsFragmentDoc}`;
export const WorkspacesDocument = gql`
    query Workspaces($projectId: ID!) {
  workspaces(projectId: $projectId) {
    ...WorkspaceFields
  }
}
    ${WorkspaceFieldsFragmentDoc}`;
export const WorkspaceDocument = gql`
    query Workspace($id: ID!) {
  workspace(id: $id) {
    ...WorkspaceFields
  }
}
    ${WorkspaceFieldsFragmentDoc}`;
export const WorkspaceFilesDocument = gql`
    query WorkspaceFiles($workspaceId: ID!, $path: String) {
  workspaceFiles(workspaceId: $workspaceId, path: $path) {
    path
    size
    isDir
    modifiedAt
  }
}
    `;
export const WorkspaceFileDocument = gql`
    query WorkspaceFile($workspaceId: ID!, $path: String!) {
  workspaceFile(workspaceId: $workspaceId, path: $path) {
    path
    content
    bytes
    encoding
  }
}
    `;
export const CreateWorkspaceDocument = gql`
    mutation CreateWorkspace($projectId: ID!, $driver: String) {
  createWorkspace(projectId: $projectId, driver: $driver) {
    ...WorkspaceFields
  }
}
    ${WorkspaceFieldsFragmentDoc}`;
export const StartWorkspaceDocument = gql`
    mutation StartWorkspace($id: ID!) {
  startWorkspace(id: $id) {
    ...WorkspaceFields
  }
}
    ${WorkspaceFieldsFragmentDoc}`;
export const StopWorkspaceDocument = gql`
    mutation StopWorkspace($id: ID!) {
  stopWorkspace(id: $id) {
    ok
  }
}
    `;
export const WriteWorkspaceFileDocument = gql`
    mutation WriteWorkspaceFile($workspaceId: ID!, $path: String!, $content: String!) {
  writeWorkspaceFile(workspaceId: $workspaceId, path: $path, content: $content) {
    ok
  }
}
    `;
export const ExecInWorkspaceDocument = gql`
    mutation ExecInWorkspace($workspaceId: ID!, $command: String!, $timeoutSec: Int) {
  execInWorkspace(
    workspaceId: $workspaceId
    command: $command
    timeoutSec: $timeoutSec
  ) {
    exitCode
    stdout
    stderr
    durMs
    timedOut
  }
}
    `;
export const RunProjectDocument = gql`
    subscription RunProject($projectId: ID!) {
  runProject(projectId: $projectId) {
    __typename
    ... on RunExecutionEvent {
      ts
      payload
    }
    ... on RunGateEvent {
      ts
      gate
      status
      gateMessage: message
    }
    ... on RunDoneEvent {
      ts
      ok
    }
    ... on RunErrorEvent {
      ts
      code
      message
    }
  }
}
    `;
export const ChatStreamDocument = gql`
    subscription ChatStream($projectId: ID!, $input: ChatInput!) {
  chatStream(projectId: $projectId, input: $input) {
    __typename
    ... on ChatStartDelta {
      startTurnId: turnId
      startProvider: provider
      startModel: model
    }
    ... on ChatTextDelta {
      textTurnId: turnId
      text
    }
    ... on ChatThinkingDelta {
      thinkingTurnId: turnId
      thinkingText: text
    }
    ... on ChatToolUseDelta {
      toolUseTurnId: turnId
      toolUse
    }
    ... on ChatDoneDelta {
      doneTurnId: turnId
      doneProvider: provider
      doneModel: model
      usage
    }
    ... on ChatErrorDelta {
      errorTurnId: turnId
      code
      message
    }
  }
}
    `;
export const InlineCompletionDocument = gql`
    subscription InlineCompletion($input: InlineInput!) {
  inlineCompletion(input: $input) {
    __typename
    ... on InlineStartDelta {
      startRequestId: requestId
      startProvider: provider
      startModel: model
    }
    ... on InlineTextDelta {
      textRequestId: requestId
      text
    }
    ... on InlineDoneDelta {
      doneRequestId: requestId
      doneProvider: provider
      doneModel: model
      usage
    }
    ... on InlineCancelledDelta {
      cancelledRequestId: requestId
      reason
    }
    ... on InlineErrorDelta {
      errorRequestId: requestId
      code
      message
    }
  }
}
    `;
export const DeployStreamDocument = gql`
    subscription DeployStream($deployId: ID!) {
  deployStream(deployId: $deployId) {
    __typename
    ... on DeployStateEvent {
      deployId
      status
      ts
    }
    ... on DeployLogEvent {
      logDeployId: deployId
      logLine: line {
        ts
        level
        message
      }
    }
    ... on DeployFinishedEvent {
      finishedDeployId: deployId
      finishedStatus: status
      url
      durationMs
    }
    ... on DeployErrorEvent {
      errorDeployId: deployId
      code
      message
    }
    ... on DeployBuildLogLine {
      buildTs: ts
      source
      buildLine: line
    }
  }
}
    `;
export const WorkspacePtyDocument = gql`
    subscription WorkspacePty($workspaceId: ID!) {
  workspacePty(workspaceId: $workspaceId) {
    __typename
    ... on PtyOutput {
      data
    }
    ... on PtyExit {
      code
    }
  }
}
    `;
export const CostStreamDocument = gql`
    subscription CostStream {
  costStream {
    ts
    usdSpent
    model
    provider
    agent
    durationMs
  }
}
    `;

export type SdkFunctionWrapper = <T>(action: (requestHeaders?:Record<string, string>) => Promise<T>, operationName: string, operationType?: string, variables?: any) => Promise<T>;


const defaultWrapper: SdkFunctionWrapper = (action, _operationName, _operationType, _variables) => action();

export function getSdk(client: GraphQLClient, withWrapper: SdkFunctionWrapper = defaultWrapper) {
  return {
    Me(variables?: MeQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<MeQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<MeQuery>({ document: MeDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'Me', 'query', variables);
    },
    SignIn(variables: SignInMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<SignInMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<SignInMutation>({ document: SignInDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'SignIn', 'mutation', variables);
    },
    SignUp(variables: SignUpMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<SignUpMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<SignUpMutation>({ document: SignUpDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'SignUp', 'mutation', variables);
    },
    SignOut(variables?: SignOutMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<SignOutMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<SignOutMutation>({ document: SignOutDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'SignOut', 'mutation', variables);
    },
    Projects(variables?: ProjectsQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<ProjectsQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<ProjectsQuery>({ document: ProjectsDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'Projects', 'query', variables);
    },
    Project(variables: ProjectQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<ProjectQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<ProjectQuery>({ document: ProjectDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'Project', 'query', variables);
    },
    ProjectFiles(variables: ProjectFilesQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<ProjectFilesQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<ProjectFilesQuery>({ document: ProjectFilesDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'ProjectFiles', 'query', variables);
    },
    ProjectGraph(variables: ProjectGraphQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<ProjectGraphQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<ProjectGraphQuery>({ document: ProjectGraphDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'ProjectGraph', 'query', variables);
    },
    ProjectSnapshot(variables: ProjectSnapshotQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<ProjectSnapshotQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<ProjectSnapshotQuery>({ document: ProjectSnapshotDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'ProjectSnapshot', 'query', variables);
    },
    SearchProjectCode(variables: SearchProjectCodeQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<SearchProjectCodeQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<SearchProjectCodeQuery>({ document: SearchProjectCodeDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'SearchProjectCode', 'query', variables);
    },
    CreateProject(variables: CreateProjectMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<CreateProjectMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<CreateProjectMutation>({ document: CreateProjectDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'CreateProject', 'mutation', variables);
    },
    UpdateProject(variables: UpdateProjectMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<UpdateProjectMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<UpdateProjectMutation>({ document: UpdateProjectDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'UpdateProject', 'mutation', variables);
    },
    DeleteProject(variables: DeleteProjectMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<DeleteProjectMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<DeleteProjectMutation>({ document: DeleteProjectDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'DeleteProject', 'mutation', variables);
    },
    BulkDeleteProjects(variables: BulkDeleteProjectsMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<BulkDeleteProjectsMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<BulkDeleteProjectsMutation>({ document: BulkDeleteProjectsDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'BulkDeleteProjects', 'mutation', variables);
    },
    Gates(variables: GatesQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<GatesQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<GatesQuery>({ document: GatesDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'Gates', 'query', variables);
    },
    RerunGate(variables: RerunGateMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<RerunGateMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<RerunGateMutation>({ document: RerunGateDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'RerunGate', 'mutation', variables);
    },
    RunFinisher(variables: RunFinisherMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<RunFinisherMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<RunFinisherMutation>({ document: RunFinisherDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'RunFinisher', 'mutation', variables);
    },
    Patches(variables: PatchesQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<PatchesQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<PatchesQuery>({ document: PatchesDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'Patches', 'query', variables);
    },
    ProposePatch(variables: ProposePatchMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<ProposePatchMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<ProposePatchMutation>({ document: ProposePatchDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'ProposePatch', 'mutation', variables);
    },
    ApplyPatch(variables: ApplyPatchMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<ApplyPatchMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<ApplyPatchMutation>({ document: ApplyPatchDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'ApplyPatch', 'mutation', variables);
    },
    RollbackPatch(variables: RollbackPatchMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<RollbackPatchMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<RollbackPatchMutation>({ document: RollbackPatchDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'RollbackPatch', 'mutation', variables);
    },
    MyBudget(variables?: MyBudgetQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<MyBudgetQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<MyBudgetQuery>({ document: MyBudgetDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'MyBudget', 'query', variables);
    },
    Plans(variables?: PlansQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<PlansQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<PlansQuery>({ document: PlansDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'Plans', 'query', variables);
    },
    Rates(variables?: RatesQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<RatesQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<RatesQuery>({ document: RatesDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'Rates', 'query', variables);
    },
    Vault(variables?: VaultQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<VaultQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<VaultQuery>({ document: VaultDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'Vault', 'query', variables);
    },
    MySubscription(variables?: MySubscriptionQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<MySubscriptionQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<MySubscriptionQuery>({ document: MySubscriptionDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'MySubscription', 'query', variables);
    },
    StartCheckout(variables: StartCheckoutMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<StartCheckoutMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<StartCheckoutMutation>({ document: StartCheckoutDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'StartCheckout', 'mutation', variables);
    },
    CancelSubscription(variables: CancelSubscriptionMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<CancelSubscriptionMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<CancelSubscriptionMutation>({ document: CancelSubscriptionDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'CancelSubscription', 'mutation', variables);
    },
    Memory(variables?: MemoryQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<MemoryQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<MemoryQuery>({ document: MemoryDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'Memory', 'query', variables);
    },
    AddMemory(variables: AddMemoryMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<AddMemoryMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<AddMemoryMutation>({ document: AddMemoryDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'AddMemory', 'mutation', variables);
    },
    DeleteMemory(variables: DeleteMemoryMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<DeleteMemoryMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<DeleteMemoryMutation>({ document: DeleteMemoryDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'DeleteMemory', 'mutation', variables);
    },
    Audit(variables?: AuditQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<AuditQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<AuditQuery>({ document: AuditDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'Audit', 'query', variables);
    },
    VerifyAudit(variables?: VerifyAuditQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<VerifyAuditQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<VerifyAuditQuery>({ document: VerifyAuditDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'VerifyAudit', 'query', variables);
    },
    Webhooks(variables?: WebhooksQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<WebhooksQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<WebhooksQuery>({ document: WebhooksDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'Webhooks', 'query', variables);
    },
    CreateWebhook(variables: CreateWebhookMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<CreateWebhookMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<CreateWebhookMutation>({ document: CreateWebhookDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'CreateWebhook', 'mutation', variables);
    },
    DeleteWebhook(variables: DeleteWebhookMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<DeleteWebhookMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<DeleteWebhookMutation>({ document: DeleteWebhookDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'DeleteWebhook', 'mutation', variables);
    },
    TestWebhook(variables: TestWebhookMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<TestWebhookMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<TestWebhookMutation>({ document: TestWebhookDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'TestWebhook', 'mutation', variables);
    },
    Deploys(variables: DeploysQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<DeploysQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<DeploysQuery>({ document: DeploysDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'Deploys', 'query', variables);
    },
    StartDeploy(variables: StartDeployMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<StartDeployMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<StartDeployMutation>({ document: StartDeployDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'StartDeploy', 'mutation', variables);
    },
    AcceptInlineCompletion(variables: AcceptInlineCompletionMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<AcceptInlineCompletionMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<AcceptInlineCompletionMutation>({ document: AcceptInlineCompletionDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'AcceptInlineCompletion', 'mutation', variables);
    },
    Chats(variables: ChatsQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<ChatsQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<ChatsQuery>({ document: ChatsDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'Chats', 'query', variables);
    },
    ChatMessages(variables: ChatMessagesQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<ChatMessagesQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<ChatMessagesQuery>({ document: ChatMessagesDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'ChatMessages', 'query', variables);
    },
    CreateChat(variables: CreateChatMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<CreateChatMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<CreateChatMutation>({ document: CreateChatDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'CreateChat', 'mutation', variables);
    },
    ForkChat(variables: ForkChatMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<ForkChatMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<ForkChatMutation>({ document: ForkChatDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'ForkChat', 'mutation', variables);
    },
    Workspaces(variables: WorkspacesQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<WorkspacesQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<WorkspacesQuery>({ document: WorkspacesDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'Workspaces', 'query', variables);
    },
    Workspace(variables: WorkspaceQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<WorkspaceQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<WorkspaceQuery>({ document: WorkspaceDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'Workspace', 'query', variables);
    },
    WorkspaceFiles(variables: WorkspaceFilesQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<WorkspaceFilesQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<WorkspaceFilesQuery>({ document: WorkspaceFilesDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'WorkspaceFiles', 'query', variables);
    },
    WorkspaceFile(variables: WorkspaceFileQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<WorkspaceFileQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<WorkspaceFileQuery>({ document: WorkspaceFileDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'WorkspaceFile', 'query', variables);
    },
    CreateWorkspace(variables: CreateWorkspaceMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<CreateWorkspaceMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<CreateWorkspaceMutation>({ document: CreateWorkspaceDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'CreateWorkspace', 'mutation', variables);
    },
    StartWorkspace(variables: StartWorkspaceMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<StartWorkspaceMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<StartWorkspaceMutation>({ document: StartWorkspaceDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'StartWorkspace', 'mutation', variables);
    },
    StopWorkspace(variables: StopWorkspaceMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<StopWorkspaceMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<StopWorkspaceMutation>({ document: StopWorkspaceDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'StopWorkspace', 'mutation', variables);
    },
    WriteWorkspaceFile(variables: WriteWorkspaceFileMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<WriteWorkspaceFileMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<WriteWorkspaceFileMutation>({ document: WriteWorkspaceFileDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'WriteWorkspaceFile', 'mutation', variables);
    },
    ExecInWorkspace(variables: ExecInWorkspaceMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<ExecInWorkspaceMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<ExecInWorkspaceMutation>({ document: ExecInWorkspaceDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'ExecInWorkspace', 'mutation', variables);
    },
    RunProject(variables: RunProjectSubscriptionVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<RunProjectSubscription> {
      return withWrapper((wrappedRequestHeaders) => client.request<RunProjectSubscription>({ document: RunProjectDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'RunProject', 'subscription', variables);
    },
    ChatStream(variables: ChatStreamSubscriptionVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<ChatStreamSubscription> {
      return withWrapper((wrappedRequestHeaders) => client.request<ChatStreamSubscription>({ document: ChatStreamDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'ChatStream', 'subscription', variables);
    },
    InlineCompletion(variables: InlineCompletionSubscriptionVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<InlineCompletionSubscription> {
      return withWrapper((wrappedRequestHeaders) => client.request<InlineCompletionSubscription>({ document: InlineCompletionDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'InlineCompletion', 'subscription', variables);
    },
    DeployStream(variables: DeployStreamSubscriptionVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<DeployStreamSubscription> {
      return withWrapper((wrappedRequestHeaders) => client.request<DeployStreamSubscription>({ document: DeployStreamDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'DeployStream', 'subscription', variables);
    },
    WorkspacePty(variables: WorkspacePtySubscriptionVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<WorkspacePtySubscription> {
      return withWrapper((wrappedRequestHeaders) => client.request<WorkspacePtySubscription>({ document: WorkspacePtyDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'WorkspacePty', 'subscription', variables);
    },
    CostStream(variables?: CostStreamSubscriptionVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<CostStreamSubscription> {
      return withWrapper((wrappedRequestHeaders) => client.request<CostStreamSubscription>({ document: CostStreamDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'CostStream', 'subscription', variables);
    }
  };
}
export type Sdk = ReturnType<typeof getSdk>;