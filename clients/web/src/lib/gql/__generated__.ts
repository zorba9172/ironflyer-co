import { gql } from '@apollo/client';
import * as Apollo from '@apollo/client';
export type Maybe<T> = T | null;
export type InputMaybe<T> = Maybe<T>;
export type Exact<T extends { [key: string]: unknown }> = { [K in keyof T]: T[K] };
export type MakeOptional<T, K extends keyof T> = Omit<T, K> & { [SubKey in K]?: Maybe<T[SubKey]> };
export type MakeMaybe<T, K extends keyof T> = Omit<T, K> & { [SubKey in K]: Maybe<T[SubKey]> };
export type MakeEmpty<T extends { [key: string]: unknown }, K extends keyof T> = { [_ in K]?: never };
export type Incremental<T> = T | { [P in keyof T]?: P extends ' $fragmentName' | '__typename' ? T[P] : never };
const defaultOptions = {} as const;
/** All built-in and custom scalars, mapped to their actual values */
export type Scalars = {
  ID: { input: string; output: string; }
  String: { input: string; output: string; }
  Boolean: { input: boolean; output: boolean; }
  Int: { input: number; output: number; }
  Float: { input: number; output: number; }
  Bytes: { input: string; output: string; }
  DateTime: { input: string; output: string; }
  Decimal: { input: string; output: string; }
  JSON: { input: unknown; output: unknown; }
};

export type AbuseScoreResult = {
  __typename?: 'AbuseScoreResult';
  score: Scalars['Int']['output'];
  tenantID: Scalars['ID']['output'];
  tier: Scalars['String']['output'];
  userID: Scalars['ID']['output'];
};

export type Agent = {
  __typename?: 'Agent';
  capabilities: Array<Scalars['String']['output']>;
  enableThinking: Scalars['Boolean']['output'];
  role: Scalars['String']['output'];
  system: Maybe<Scalars['String']['output']>;
};

export type AgentCall = {
  __typename?: 'AgentCall';
  cacheReadTokens: Scalars['Int']['output'];
  cacheWriteTokens: Scalars['Int']['output'];
  capabilities: Maybe<Array<Scalars['String']['output']>>;
  completionTokens: Scalars['Int']['output'];
  costUsd: Scalars['Decimal']['output'];
  durationMs: Scalars['Int']['output'];
  error: Maybe<Scalars['String']['output']>;
  id: Scalars['ID']['output'];
  model: Maybe<Scalars['String']['output']>;
  projectId: Maybe<Scalars['ID']['output']>;
  promptTokens: Scalars['Int']['output'];
  provider: Scalars['String']['output'];
  role: Maybe<Scalars['String']['output']>;
  ts: Scalars['DateTime']['output'];
  userId: Maybe<Scalars['ID']['output']>;
};

export type AppetizeApp = {
  __typename?: 'AppetizeApp';
  createdAt: Scalars['DateTime']['output'];
  embedUrl: Scalars['String']['output'];
  platform: Scalars['String']['output'];
  publicKey: Scalars['String']['output'];
};

export type AppetizeUploadInput = {
  buildId: Scalars['String']['input'];
  deviceProfile?: InputMaybe<Scalars['String']['input']>;
  projectId: Scalars['String']['input'];
};

export type ArchRule = {
  __typename?: 'ArchRule';
  allow: Scalars['Boolean']['output'];
  from: Scalars['String']['output'];
  to: Scalars['String']['output'];
};

export type Architecture = {
  __typename?: 'Architecture';
  cycles: Scalars['String']['output'];
  layers: Array<Scalars['String']['output']>;
  rules: Array<ArchRule>;
};

export type AuditChainProof = {
  __typename?: 'AuditChainProof';
  brokenLinks: Array<BrokenAuditLink>;
  endHash: Scalars['String']['output'];
  entryCount: Scalars['Int']['output'];
  startHash: Scalars['String']['output'];
  verified: Scalars['Boolean']['output'];
  windowEnd: Scalars['DateTime']['output'];
  windowStart: Scalars['DateTime']['output'];
};

export type AuditEntry = {
  __typename?: 'AuditEntry';
  action: Scalars['String']['output'];
  actor: Maybe<Scalars['String']['output']>;
  agentRole: Maybe<Scalars['String']['output']>;
  gateName: Maybe<Scalars['String']['output']>;
  hash: Scalars['String']['output'];
  id: Scalars['ID']['output'];
  inputHash: Maybe<Scalars['String']['output']>;
  ok: Scalars['Boolean']['output'];
  outcome: AuditOutcome;
  outputHash: Maybe<Scalars['String']['output']>;
  payload: Maybe<Scalars['JSON']['output']>;
  prevHash: Maybe<Scalars['String']['output']>;
  projectId: Maybe<Scalars['ID']['output']>;
  resource: Maybe<Scalars['String']['output']>;
  storyId: Maybe<Scalars['String']['output']>;
  summary: Maybe<Scalars['String']['output']>;
  ts: Scalars['DateTime']['output'];
  userId: Maybe<Scalars['ID']['output']>;
};

export type AuditExportFilter = {
  eventTypes?: InputMaybe<Array<Scalars['String']['input']>>;
  format?: InputMaybe<Scalars['String']['input']>;
  includeAttrs?: InputMaybe<Scalars['Boolean']['input']>;
  since: Scalars['DateTime']['input'];
  tenantId: Scalars['ID']['input'];
  until: Scalars['DateTime']['input'];
};

export type AuditExportPreview = {
  __typename?: 'AuditExportPreview';
  estimatedBytes: Scalars['Int']['output'];
  estimatedEntryCount: Scalars['Int']['output'];
  expiresAt: Maybe<Scalars['DateTime']['output']>;
  format: Scalars['String']['output'];
  signedURL: Maybe<Scalars['String']['output']>;
};

export enum AuditOutcome {
  Blocked = 'BLOCKED',
  Failure = 'FAILURE',
  Skipped = 'SKIPPED',
  Success = 'SUCCESS'
}

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
  lastConfidence: Maybe<Scalars['Float']['output']>;
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
  lastUsed: Maybe<Scalars['DateTime']['output']>;
  meanReward: Scalars['Float']['output'];
  model: Maybe<Scalars['String']['output']>;
  provider: Scalars['String']['output'];
  share: Scalars['Float']['output'];
};

export type Blueprint = {
  __typename?: 'Blueprint';
  category: Scalars['String']['output'];
  costPriorUSD: Scalars['Float']['output'];
  description: Scalars['String']['output'];
  expectedTimeToPreviewSec: Scalars['Int']['output'];
  fileCount: Scalars['Int']['output'];
  id: Scalars['String']['output'];
  name: Scalars['String']['output'];
  supportedGates: Array<Scalars['String']['output']>;
};

export type BlueprintDashboard = {
  __typename?: 'BlueprintDashboard';
  blueprints: Array<DashboardBlueprintStats>;
};

export type BlueprintStats = {
  __typename?: 'BlueprintStats';
  avgCompletionScore: Scalars['Float']['output'];
  avgCostUSD: Scalars['Float']['output'];
  avgRevenueUSD: Scalars['Float']['output'];
  avgTimeToPreviewSec: Scalars['Float']['output'];
  blueprintID: Scalars['String']['output'];
  executions: Scalars['Int']['output'];
  grossMarginPct: Scalars['Float']['output'];
  previewSuccess: Scalars['Int']['output'];
  refunds: Scalars['Int']['output'];
  repairCount: Scalars['Int']['output'];
};

export type BlueprintSuccessRate = {
  __typename?: 'BlueprintSuccessRate';
  avgMargin: Scalars['Float']['output'];
  blueprintID: Scalars['String']['output'];
  blueprintName: Scalars['String']['output'];
  successRate: Scalars['Float']['output'];
};

export type BrokenAuditLink = {
  __typename?: 'BrokenAuditLink';
  actualPrevHash: Scalars['String']['output'];
  atEntryID: Scalars['ID']['output'];
  expectedPrevHash: Scalars['String']['output'];
};

export type BudgetSummary = {
  __typename?: 'BudgetSummary';
  email: Scalars['String']['output'];
  entries: Array<LedgerEntry>;
  spentUsd: Scalars['Decimal']['output'];
  tier: Scalars['String']['output'];
  userId: Scalars['ID']['output'];
};

export type ChannelPref = {
  __typename?: 'ChannelPref';
  email: Scalars['Boolean']['output'];
  inApp: Scalars['Boolean']['output'];
};

export type ChannelPrefInput = {
  email: Scalars['Boolean']['input'];
  inApp: Scalars['Boolean']['input'];
};

export type Cohort = {
  __typename?: 'Cohort';
  avgSpendUSD: Scalars['Float']['output'];
  completionRate: Scalars['Float']['output'];
  day7RepeatUsers: Scalars['Int']['output'];
  day30RepeatUsers: Scalars['Int']['output'];
  grossMarginPct: Scalars['Float']['output'];
  month: Scalars['DateTime']['output'];
  newPayingUsers: Scalars['Int']['output'];
  refundRate: Scalars['Float']['output'];
  secondExecutionUsers: Scalars['Int']['output'];
  supportTicketsPerExec: Scalars['Float']['output'];
};

export type CohortDashboard = {
  __typename?: 'CohortDashboard';
  cohorts: Array<Cohort>;
};

export type ComplexityBucket = {
  __typename?: 'ComplexityBucket';
  count: Scalars['Int']['output'];
  range: Scalars['String']['output'];
};

export type ConnectDeployDomainInput = {
  deployID?: InputMaybe<Scalars['ID']['input']>;
  hostname: Scalars['String']['input'];
  metadata?: InputMaybe<Scalars['JSON']['input']>;
  primary?: InputMaybe<Scalars['Boolean']['input']>;
  projectID: Scalars['ID']['input'];
  provider?: InputMaybe<Scalars['String']['input']>;
};

export type CostDelta = {
  __typename?: 'CostDelta';
  agent: Maybe<Scalars['String']['output']>;
  durationMs: Maybe<Scalars['Int']['output']>;
  model: Maybe<Scalars['String']['output']>;
  provider: Maybe<Scalars['String']['output']>;
  ts: Scalars['DateTime']['output'];
  usdSpent: Scalars['Decimal']['output'];
};

export type CostEstimate = {
  __typename?: 'CostEstimate';
  basedOnRuns: Scalars['Int']['output'];
  breakdown: Scalars['JSON']['output'];
  caveat: Maybe<Scalars['String']['output']>;
  confidence: Scalars['Float']['output'];
  highUSD: Scalars['Float']['output'];
  lowUSD: Scalars['Float']['output'];
  medianUSD: Scalars['Float']['output'];
  p95USD: Scalars['Float']['output'];
};

export type CostReport = {
  __typename?: 'CostReport';
  deploymentCostUSD: Scalars['Float']['output'];
  grossMarginPct: Scalars['Float']['output'];
  providerCostUSD: Scalars['Float']['output'];
  revenueUSD: Scalars['Float']['output'];
  sandboxCostUSD: Scalars['Float']['output'];
  storageCostUSD: Scalars['Float']['output'];
};

export type CreatePaidExecutionInput = {
  blueprintID?: InputMaybe<Scalars['String']['input']>;
  budgetUSD: Scalars['Float']['input'];
  metadata?: InputMaybe<Scalars['JSON']['input']>;
  projectID?: InputMaybe<Scalars['ID']['input']>;
  promptSummary?: InputMaybe<Scalars['String']['input']>;
  stopLossUSD?: InputMaybe<Scalars['Float']['input']>;
};

export type CreateProjectInput = {
  description?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['ID']['input']>;
  idea?: InputMaybe<Scalars['String']['input']>;
  name: Scalars['String']['input'];
};

export type CreateStageInput = {
  description?: InputMaybe<Scalars['String']['input']>;
  name: Scalars['String']['input'];
  patchIds: Array<Scalars['ID']['input']>;
  projectId: Scalars['ID']['input'];
};

export type DnsRecord = {
  __typename?: 'DNSRecord';
  name: Scalars['String']['output'];
  ttl: Maybe<Scalars['Int']['output']>;
  type: Scalars['String']['output'];
  value: Scalars['String']['output'];
};

export type DashboardBlueprintStats = {
  __typename?: 'DashboardBlueprintStats';
  avgCompletionScore: Scalars['Float']['output'];
  avgCostUSD: Scalars['Float']['output'];
  avgRevenueUSD: Scalars['Float']['output'];
  blueprintID: Scalars['String']['output'];
  executions: Scalars['Int']['output'];
  grossMarginPct: Scalars['Float']['output'];
  previewSuccess: Scalars['Int']['output'];
  refunds: Scalars['Int']['output'];
  repairCount: Scalars['Int']['output'];
};

export type Deploy = {
  __typename?: 'Deploy';
  artifactHash: Maybe<Scalars['String']['output']>;
  blueprintID: Maybe<Scalars['String']['output']>;
  costUSD: Scalars['Float']['output'];
  createdAt: Scalars['DateTime']['output'];
  diffHash: Maybe<Scalars['String']['output']>;
  environment: Scalars['String']['output'];
  executionID: Maybe<Scalars['ID']['output']>;
  gateSummary: Scalars['JSON']['output'];
  id: Scalars['ID']['output'];
  previewReadyAt: Maybe<Scalars['DateTime']['output']>;
  previewURL: Maybe<Scalars['String']['output']>;
  productionURL: Maybe<Scalars['String']['output']>;
  projectID: Scalars['ID']['output'];
  promotedAt: Maybe<Scalars['DateTime']['output']>;
  providerDeploymentID: Maybe<Scalars['String']['output']>;
  rolledBackAt: Maybe<Scalars['DateTime']['output']>;
  status: Scalars['String']['output'];
  target: Scalars['String']['output'];
  tenantID: Scalars['ID']['output'];
};

export type DeployApproval = {
  __typename?: 'DeployApproval';
  artifactHash: Scalars['String']['output'];
  costImpactUSD: Scalars['Float']['output'];
  decidedAt: Maybe<Scalars['DateTime']['output']>;
  decisionNote: Maybe<Scalars['String']['output']>;
  deployID: Scalars['ID']['output'];
  diffHash: Scalars['String']['output'];
  expiresAt: Scalars['DateTime']['output'];
  gateSummary: Scalars['JSON']['output'];
  id: Scalars['ID']['output'];
  requestedAt: Scalars['DateTime']['output'];
  status: Scalars['String']['output'];
  tenantID: Scalars['ID']['output'];
};

export type DeployDomain = {
  __typename?: 'DeployDomain';
  certificateStatus: Scalars['String']['output'];
  createdAt: Scalars['DateTime']['output'];
  deployID: Maybe<Scalars['ID']['output']>;
  dnsRecords: Array<DnsRecord>;
  hostname: Scalars['String']['output'];
  id: Scalars['ID']['output'];
  instructions: Scalars['String']['output'];
  kind: Scalars['String']['output'];
  liveAt: Maybe<Scalars['DateTime']['output']>;
  metadata: Scalars['JSON']['output'];
  primary: Scalars['Boolean']['output'];
  projectID: Scalars['ID']['output'];
  provider: Scalars['String']['output'];
  registrar: Maybe<Scalars['String']['output']>;
  status: Scalars['String']['output'];
  tenantID: Scalars['ID']['output'];
  updatedAt: Scalars['DateTime']['output'];
  verificationStatus: Scalars['String']['output'];
  verifiedAt: Maybe<Scalars['DateTime']['output']>;
};

export type DeployEvent = {
  __typename?: 'DeployEvent';
  createdAt: Scalars['DateTime']['output'];
  deployID: Scalars['ID']['output'];
  eventType: Scalars['String']['output'];
  payload: Scalars['JSON']['output'];
};

export type DescribeIdeaInput = {
  blueprintIDOverride?: InputMaybe<Scalars['String']['input']>;
  budgetUSDOverride?: InputMaybe<Scalars['Float']['input']>;
  startImmediately?: InputMaybe<Scalars['Boolean']['input']>;
  text: Scalars['String']['input'];
};

export type DeviceCloudDevice = {
  __typename?: 'DeviceCloudDevice';
  id: Scalars['String']['output'];
  manufacturer: Maybe<Scalars['String']['output']>;
  model: Scalars['String']['output'];
  osVersion: Scalars['String']['output'];
  platform: Scalars['String']['output'];
  provider: Scalars['String']['output'];
  real: Scalars['Boolean']['output'];
};

export type DeviceCloudSession = {
  __typename?: 'DeviceCloudSession';
  appUrl: Maybe<Scalars['String']['output']>;
  billableMinutesUsed: Scalars['Float']['output'];
  deviceId: Scalars['String']['output'];
  expiresAt: Scalars['DateTime']['output'];
  id: Scalars['String']['output'];
  provider: Scalars['String']['output'];
  sessionUrl: Maybe<Scalars['String']['output']>;
  startedAt: Scalars['DateTime']['output'];
  status: Scalars['String']['output'];
};

export type DeviceCloudStartInput = {
  appUrl: Scalars['String']['input'];
  deviceId: Scalars['String']['input'];
  projectId: Scalars['String']['input'];
  provider: Scalars['String']['input'];
  sessionLengthMinutes?: InputMaybe<Scalars['Int']['input']>;
  workspaceId: Scalars['String']['input'];
};

export type DirDup = {
  __typename?: 'DirDup';
  directory: Scalars['String']['output'];
  dupPct: Scalars['Float']['output'];
  files: Scalars['Int']['output'];
};

export type DomainAvailability = {
  __typename?: 'DomainAvailability';
  available: Scalars['Boolean']['output'];
  canPurchase: Scalars['Boolean']['output'];
  checkedAt: Scalars['DateTime']['output'];
  currency: Scalars['String']['output'];
  domain: Scalars['String']['output'];
  premium: Scalars['Boolean']['output'];
  priceUSD: Scalars['Float']['output'];
  reason: Maybe<Scalars['String']['output']>;
  registrar: Scalars['String']['output'];
  requirements: Array<Scalars['String']['output']>;
};

export type EmailChangeInput = {
  currentPassword: Scalars['String']['input'];
  newEmail: Scalars['String']['input'];
};

export type ErrorAggregate = {
  __typename?: 'ErrorAggregate';
  class: Scalars['String']['output'];
  count: Scalars['Int']['output'];
  lastLevel: Scalars['String']['output'];
  lastMessage: Scalars['String']['output'];
  lastSeen: Scalars['DateTime']['output'];
  sampleRequestID: Maybe<Scalars['String']['output']>;
};

export type EstimateInput = {
  blueprintID?: InputMaybe<Scalars['String']['input']>;
  capabilities?: InputMaybe<Array<Scalars['String']['input']>>;
  estimatedDurationSec?: InputMaybe<Scalars['Int']['input']>;
  promptSummary?: InputMaybe<Scalars['String']['input']>;
};

export type Execution = {
  __typename?: 'Execution';
  admittedAt: Maybe<Scalars['DateTime']['output']>;
  blueprintID: Maybe<Scalars['String']['output']>;
  budgetUSD: Scalars['Float']['output'];
  completionScore: Scalars['Float']['output'];
  createdAt: Scalars['DateTime']['output'];
  deploymentCostUSD: Scalars['Float']['output'];
  endedAt: Maybe<Scalars['DateTime']['output']>;
  expectedCompletionDelta: Maybe<Scalars['Float']['output']>;
  failureReason: Maybe<Scalars['String']['output']>;
  grossMarginPct: Maybe<Scalars['Float']['output']>;
  id: Scalars['ID']['output'];
  metadata: Scalars['JSON']['output'];
  projectID: Maybe<Scalars['ID']['output']>;
  promptSummary: Maybe<Scalars['String']['output']>;
  providerCostUSD: Scalars['Float']['output'];
  refundedUSD: Scalars['Float']['output'];
  reservedUSD: Scalars['Float']['output'];
  revenueUSD: Scalars['Float']['output'];
  riskScore: Maybe<Scalars['Float']['output']>;
  sandboxCostUSD: Scalars['Float']['output'];
  spentUSD: Scalars['Float']['output'];
  startedAt: Maybe<Scalars['DateTime']['output']>;
  status: Scalars['String']['output'];
  stopLossUSD: Maybe<Scalars['Float']['output']>;
  storageCostUSD: Scalars['Float']['output'];
  tenantID: Scalars['ID']['output'];
  workspaceID: Maybe<Scalars['String']['output']>;
};

export type ExecutionEvent = {
  __typename?: 'ExecutionEvent';
  createdAt: Scalars['DateTime']['output'];
  eventType: Scalars['String']['output'];
  executionID: Scalars['ID']['output'];
  payload: Scalars['JSON']['output'];
};

export type GateFailureRate = {
  __typename?: 'GateFailureRate';
  failureRate: Scalars['Float']['output'];
  gate: Scalars['String']['output'];
  sampleSize: Scalars['Int']['output'];
};

export type GateIssue = {
  __typename?: 'GateIssue';
  line: Maybe<Scalars['Int']['output']>;
  message: Scalars['String']['output'];
  path: Maybe<Scalars['String']['output']>;
  rule: Maybe<Scalars['String']['output']>;
  severity: Maybe<Scalars['String']['output']>;
};

export type GateReport = {
  __typename?: 'GateReport';
  completionScore: Scalars['Float']['output'];
  stages: Array<GateStage>;
};

export type GateStage = {
  __typename?: 'GateStage';
  issuesCount: Scalars['Int']['output'];
  name: Scalars['String']['output'];
  status: Scalars['String']['output'];
};

export enum GateStatus {
  Blocked = 'BLOCKED',
  Fail = 'FAIL',
  Pass = 'PASS',
  Pending = 'PENDING',
  Running = 'RUNNING',
  Skipped = 'SKIPPED',
  Warn = 'WARN'
}

export type GateVerdict = {
  __typename?: 'GateVerdict';
  durationMs: Maybe<Scalars['Int']['output']>;
  finishedAt: Maybe<Scalars['DateTime']['output']>;
  gate: Scalars['String']['output'];
  issues: Array<GateIssue>;
  notes: Maybe<Scalars['String']['output']>;
  startedAt: Maybe<Scalars['DateTime']['output']>;
  status: GateStatus;
};

export type GenerateMobileAssetEntry = {
  __typename?: 'GenerateMobileAssetEntry';
  height: Scalars['Int']['output'];
  path: Scalars['String']['output'];
  purpose: Scalars['String']['output'];
  sizeBytes: Scalars['Int']['output'];
  width: Scalars['Int']['output'];
};

export type GenerateMobileAssetsInput = {
  backgroundColor: Scalars['String']['input'];
  logoPngBase64: Scalars['String']['input'];
  platforms: Array<Scalars['String']['input']>;
  projectId: Scalars['String']['input'];
  splashForegroundColor?: InputMaybe<Scalars['String']['input']>;
};

export type GenerateMobileAssetsResult = {
  __typename?: 'GenerateMobileAssetsResult';
  entries: Array<GenerateMobileAssetEntry>;
  filesCount: Scalars['Int']['output'];
  generatedAt: Scalars['DateTime']['output'];
  totalBytes: Scalars['Int']['output'];
};

export type GqlError = {
  __typename?: 'GqlError';
  code: Scalars['String']['output'];
  details: Maybe<Scalars['JSON']['output']>;
  message: Scalars['String']['output'];
};

export type HealthMetrics = {
  __typename?: 'HealthMetrics';
  architecture: Architecture;
  atlasCapabilityCount: Scalars['Int']['output'];
  bundleByRoute: Array<RouteBundle>;
  complexityHistogram: Array<ComplexityBucket>;
  deadCodeCount: Scalars['Int']['output'];
  dedupRate: Scalars['Float']['output'];
  dependencyCycles: Scalars['Int']['output'];
  duplicationByDir: Array<DirDup>;
  lastIndexedAt: Maybe<Scalars['DateTime']['output']>;
  locPerCapability: Scalars['Float']['output'];
  reuseRate: Scalars['Float']['output'];
};

export type HeartbeatEvent = {
  __typename?: 'HeartbeatEvent';
  message: Maybe<Scalars['String']['output']>;
  ts: Scalars['DateTime']['output'];
};

export type InlineCancelledDelta = {
  __typename?: 'InlineCancelledDelta';
  reason: Maybe<Scalars['String']['output']>;
  requestId: Scalars['ID']['output'];
};

export type InlineCompletion = {
  __typename?: 'InlineCompletion';
  accepted: Scalars['Boolean']['output'];
  cursor: Scalars['Int']['output'];
  finishedAt: Maybe<Scalars['DateTime']['output']>;
  model: Scalars['String']['output'];
  provider: Scalars['String']['output'];
  requestId: Scalars['ID']['output'];
  startedAt: Scalars['DateTime']['output'];
  text: Scalars['String']['output'];
};

export type InlineDelta = InlineCancelledDelta | InlineDoneDelta | InlineErrorDelta | InlineStartDelta | InlineTextDelta;

export type InlineDoneDelta = {
  __typename?: 'InlineDoneDelta';
  model: Maybe<Scalars['String']['output']>;
  provider: Maybe<Scalars['String']['output']>;
  requestId: Scalars['ID']['output'];
  usage: Maybe<Scalars['JSON']['output']>;
};

export type InlineErrorDelta = {
  __typename?: 'InlineErrorDelta';
  code: Scalars['String']['output'];
  message: Scalars['String']['output'];
  requestId: Maybe<Scalars['ID']['output']>;
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

export type LearningDashboard = {
  __typename?: 'LearningDashboard';
  averageCompletionScore: Scalars['Float']['output'];
  averageMarginPctLast7d: Scalars['Float']['output'];
  banditConfidence: Scalars['Float']['output'];
  blueprintSuccessRates: Array<BlueprintSuccessRate>;
  gateFailureRates: Array<GateFailureRate>;
  lastIndexedAt: Maybe<Scalars['DateTime']['output']>;
  outcomeEventsAllTime: Scalars['Int']['output'];
  outcomeEventsToday: Scalars['Int']['output'];
  repairRecipeHitsLast7d: Scalars['Int']['output'];
  reuseRateLast7d: Scalars['Float']['output'];
  weaknesses: Array<Weakness>;
};

export type LedgerEntry = {
  __typename?: 'LedgerEntry';
  agent: Maybe<Scalars['String']['output']>;
  completionTokens: Scalars['Int']['output'];
  costUsd: Scalars['Decimal']['output'];
  durationMs: Maybe<Scalars['Int']['output']>;
  id: Scalars['ID']['output'];
  model: Maybe<Scalars['String']['output']>;
  projectId: Maybe<Scalars['ID']['output']>;
  promptTokens: Scalars['Int']['output'];
  provider: Maybe<Scalars['String']['output']>;
  revenueUsd: Scalars['Decimal']['output'];
  ts: Scalars['DateTime']['output'];
  userId: Scalars['ID']['output'];
};

export type LedgerFilter = {
  executionID?: InputMaybe<Scalars['ID']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
  since?: InputMaybe<Scalars['DateTime']['input']>;
  types?: InputMaybe<Array<Scalars['String']['input']>>;
  until?: InputMaybe<Scalars['DateTime']['input']>;
};

export type LedgerRollup = {
  __typename?: 'LedgerRollup';
  deploymentCostUSD: Scalars['Float']['output'];
  grossMarginPct: Scalars['Float']['output'];
  platformMarginUSD: Scalars['Float']['output'];
  premiumReasoningCostUSD: Scalars['Float']['output'];
  providerCostUSD: Scalars['Float']['output'];
  refundsUSD: Scalars['Float']['output'];
  revenueUSD: Scalars['Float']['output'];
  sandboxCostUSD: Scalars['Float']['output'];
  storageCostUSD: Scalars['Float']['output'];
};

export type LogEntry = {
  __typename?: 'LogEntry';
  executionID: Maybe<Scalars['String']['output']>;
  fields: Scalars['JSON']['output'];
  level: Scalars['String']['output'];
  message: Scalars['String']['output'];
  requestID: Maybe<Scalars['String']['output']>;
  tenantID: Maybe<Scalars['String']['output']>;
  time: Scalars['DateTime']['output'];
};

export type McpRunningServer = {
  __typename?: 'MCPRunningServer';
  id: Scalars['String']['output'];
  serverId: Scalars['String']['output'];
  startedAt: Scalars['DateTime']['output'];
};

export type McpServerSpec = {
  __typename?: 'MCPServerSpec';
  capabilities: Array<Scalars['String']['output']>;
  category: Scalars['String']['output'];
  description: Scalars['String']['output'];
  envKeys: Array<Scalars['String']['output']>;
  iconUrl: Maybe<Scalars['String']['output']>;
  id: Scalars['String']['output'];
  name: Scalars['String']['output'];
  requiresSecret: Scalars['Boolean']['output'];
  vendor: Scalars['String']['output'];
};

export type MobileBuild = {
  __typename?: 'MobileBuild';
  appBuildVersion: Maybe<Scalars['String']['output']>;
  appVersion: Maybe<Scalars['String']['output']>;
  artifactSizeBytes: Maybe<Scalars['Int']['output']>;
  artifactUrl: Maybe<Scalars['String']['output']>;
  channel: Maybe<Scalars['String']['output']>;
  completedAt: Maybe<Scalars['DateTime']['output']>;
  createdAt: Scalars['DateTime']['output'];
  distribution: Maybe<Scalars['String']['output']>;
  errorMessage: Maybe<Scalars['String']['output']>;
  id: Scalars['ID']['output'];
  initiator: Maybe<Scalars['String']['output']>;
  logUrl: Maybe<Scalars['String']['output']>;
  platform: Scalars['String']['output'];
  profile: Scalars['String']['output'];
  projectId: Scalars['String']['output'];
  sdkVersion: Maybe<Scalars['String']['output']>;
  status: Scalars['String']['output'];
  updatedAt: Scalars['DateTime']['output'];
};

export type MobilePublishUpdateInput = {
  branch?: InputMaybe<Scalars['String']['input']>;
  channel: Scalars['String']['input'];
  manifestExtra?: InputMaybe<Scalars['JSON']['input']>;
  message?: InputMaybe<Scalars['String']['input']>;
  projectId: Scalars['String']['input'];
  runtimeVersion: Scalars['String']['input'];
};

export type MobileSubmission = {
  __typename?: 'MobileSubmission';
  archiveUrl: Maybe<Scalars['String']['output']>;
  buildId: Maybe<Scalars['String']['output']>;
  completedAt: Maybe<Scalars['DateTime']['output']>;
  createdAt: Scalars['DateTime']['output'];
  errorMessage: Maybe<Scalars['String']['output']>;
  id: Scalars['ID']['output'];
  logUrl: Maybe<Scalars['String']['output']>;
  platform: Scalars['String']['output'];
  projectId: Scalars['String']['output'];
  status: Scalars['String']['output'];
  target: Scalars['String']['output'];
  updatedAt: Scalars['DateTime']['output'];
};

export type MobileSubmitInput = {
  androidReleaseStatus?: InputMaybe<Scalars['String']['input']>;
  androidServiceAccountKey?: InputMaybe<Scalars['String']['input']>;
  androidTrack?: InputMaybe<Scalars['String']['input']>;
  buildId: Scalars['String']['input'];
  iosAppleId?: InputMaybe<Scalars['String']['input']>;
  iosAppleTeamId?: InputMaybe<Scalars['String']['input']>;
  iosAscAppId?: InputMaybe<Scalars['String']['input']>;
  iosCompanyName?: InputMaybe<Scalars['String']['input']>;
  iosSku?: InputMaybe<Scalars['String']['input']>;
  platform: Scalars['String']['input'];
  projectId: Scalars['String']['input'];
};

export type MobileTriggerBuildInput = {
  channel?: InputMaybe<Scalars['String']['input']>;
  message?: InputMaybe<Scalars['String']['input']>;
  platform: Scalars['String']['input'];
  profile?: InputMaybe<Scalars['String']['input']>;
  projectId: Scalars['String']['input'];
};

export type MobileUpdate = {
  __typename?: 'MobileUpdate';
  branch: Scalars['String']['output'];
  channel: Scalars['String']['output'];
  createdAt: Scalars['DateTime']['output'];
  groupId: Maybe<Scalars['String']['output']>;
  id: Scalars['ID']['output'];
  manifestUrl: Maybe<Scalars['String']['output']>;
  message: Maybe<Scalars['String']['output']>;
  platform: Maybe<Scalars['String']['output']>;
  runtimeVersion: Scalars['String']['output'];
};

export type Mutation = {
  __typename?: 'Mutation';
  _empty: Maybe<Scalars['String']['output']>;
  acceptInlineCompletion: OperationResult;
  appetizeDeleteApp: Scalars['Boolean']['output'];
  appetizeUploadBuild: AppetizeApp;
  applyPatch: Patch;
  applyStage: Scalars['JSON']['output'];
  buildDeployPreview: Deploy;
  bulkDeleteProjects: OperationResult;
  cancelDeploy: Deploy;
  checkDeployDomain: DeployDomain;
  confirmEmailChange: OperationResult;
  connectDeployDomain: DeployDomain;
  createPaidExecution: Execution;
  createProject: Project;
  createStage: PatchStage;
  decideDeployApproval: DeployApproval;
  deleteProject: OperationResult;
  describeIdea: StudioBootstrap;
  deviceCloudEndSession: Scalars['Boolean']['output'];
  deviceCloudStartSession: DeviceCloudSession;
  generateMobileAssets: GenerateMobileAssetsResult;
  markAllNotificationsRead: Scalars['Int']['output'];
  markNotificationRead: Notification;
  mcpDisable: Scalars['Boolean']['output'];
  mcpEnable: McpRunningServer;
  mobileCancelBuild: Scalars['Boolean']['output'];
  mobilePublishUpdate: MobileUpdate;
  mobileSubmitToStore: MobileSubmission;
  mobileTriggerBuild: MobileBuild;
  planDeploy: Deploy;
  promoteDeploy: Deploy;
  promptPlan: Scalars['JSON']['output'];
  proposePatch: Patch;
  proposeSymbolPatch: Patch;
  purchaseDeployDomain: DeployDomain;
  refineIdea: StudioBootstrap;
  refundExecution: Execution;
  rejectStage: PatchStage;
  renameSymbol: Patch;
  requestDeployApproval: DeployApproval;
  requestEmailChange: OperationResult;
  requestPasswordReset: OperationResult;
  rerunGate: GateVerdict;
  resendVerificationEmail: OperationResult;
  reserveDeploySubdomain: DeployDomain;
  resetPassword: Session;
  revokeAllOtherSessions: OperationResult;
  revokeSession: OperationResult;
  rollbackDeploy: Deploy;
  rollbackPatch: Scalars['JSON']['output'];
  runFinisher: Scalars['JSON']['output'];
  setPrimaryDeployDomain: DeployDomain;
  setTelemetryPreference: User;
  signIn: Session;
  signOut: OperationResult;
  signUp: Session;
  startCheckout: StripeCheckoutSession;
  stopExecution: Execution;
  updateNotificationPreferences: NotificationPreferences;
  updateProject: Project;
  verifyEmail: Session;
  walletCreateTopUp: WalletCheckoutSession;
  writeProjectFiles: Array<ProjectFile>;
};


export type MutationAcceptInlineCompletionArgs = {
  requestId: Scalars['ID']['input'];
};


export type MutationAppetizeDeleteAppArgs = {
  publicKey: Scalars['String']['input'];
};


export type MutationAppetizeUploadBuildArgs = {
  input: AppetizeUploadInput;
};


export type MutationApplyPatchArgs = {
  patchId: Scalars['ID']['input'];
};


export type MutationApplyStageArgs = {
  stageId: Scalars['ID']['input'];
};


export type MutationBuildDeployPreviewArgs = {
  deployID: Scalars['ID']['input'];
};


export type MutationBulkDeleteProjectsArgs = {
  ids: Array<Scalars['ID']['input']>;
};


export type MutationCancelDeployArgs = {
  deployID: Scalars['ID']['input'];
  reason: Scalars['String']['input'];
};


export type MutationCheckDeployDomainArgs = {
  id: Scalars['ID']['input'];
};


export type MutationConfirmEmailChangeArgs = {
  token: Scalars['String']['input'];
};


export type MutationConnectDeployDomainArgs = {
  input: ConnectDeployDomainInput;
};


export type MutationCreatePaidExecutionArgs = {
  input: CreatePaidExecutionInput;
};


export type MutationCreateProjectArgs = {
  input: CreateProjectInput;
};


export type MutationCreateStageArgs = {
  input: CreateStageInput;
};


export type MutationDecideDeployApprovalArgs = {
  approvalID: Scalars['ID']['input'];
  approve: Scalars['Boolean']['input'];
  note?: InputMaybe<Scalars['String']['input']>;
};


export type MutationDeleteProjectArgs = {
  id: Scalars['ID']['input'];
};


export type MutationDescribeIdeaArgs = {
  input: DescribeIdeaInput;
};


export type MutationDeviceCloudEndSessionArgs = {
  provider: Scalars['String']['input'];
  sessionId: Scalars['String']['input'];
};


export type MutationDeviceCloudStartSessionArgs = {
  input: DeviceCloudStartInput;
};


export type MutationGenerateMobileAssetsArgs = {
  input: GenerateMobileAssetsInput;
};


export type MutationMarkNotificationReadArgs = {
  id: Scalars['ID']['input'];
};


export type MutationMcpDisableArgs = {
  projectId: Scalars['String']['input'];
  serverId: Scalars['String']['input'];
};


export type MutationMcpEnableArgs = {
  projectId: Scalars['String']['input'];
  serverId: Scalars['String']['input'];
};


export type MutationMobileCancelBuildArgs = {
  buildId: Scalars['String']['input'];
};


export type MutationMobilePublishUpdateArgs = {
  input: MobilePublishUpdateInput;
};


export type MutationMobileSubmitToStoreArgs = {
  input: MobileSubmitInput;
};


export type MutationMobileTriggerBuildArgs = {
  input: MobileTriggerBuildInput;
};


export type MutationPlanDeployArgs = {
  input: PlanDeployInput;
};


export type MutationPromoteDeployArgs = {
  deployID: Scalars['ID']['input'];
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


export type MutationPurchaseDeployDomainArgs = {
  input: PurchaseDeployDomainInput;
};


export type MutationRefineIdeaArgs = {
  executionID: Scalars['ID']['input'];
  message: Scalars['String']['input'];
};


export type MutationRefundExecutionArgs = {
  amountUSD?: InputMaybe<Scalars['Float']['input']>;
  id: Scalars['ID']['input'];
  reason?: InputMaybe<Scalars['String']['input']>;
};


export type MutationRejectStageArgs = {
  reason?: InputMaybe<Scalars['String']['input']>;
  stageId: Scalars['ID']['input'];
};


export type MutationRenameSymbolArgs = {
  input: RenameSymbolInput;
};


export type MutationRequestDeployApprovalArgs = {
  deployID: Scalars['ID']['input'];
  expiresInMinutes?: InputMaybe<Scalars['Int']['input']>;
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


export type MutationReserveDeploySubdomainArgs = {
  input: ReserveDeploySubdomainInput;
};


export type MutationResetPasswordArgs = {
  newPassword: Scalars['String']['input'];
  token: Scalars['String']['input'];
};


export type MutationRevokeSessionArgs = {
  jti: Scalars['ID']['input'];
};


export type MutationRollbackDeployArgs = {
  deployID: Scalars['ID']['input'];
  reason: Scalars['String']['input'];
};


export type MutationRollbackPatchArgs = {
  patchId: Scalars['ID']['input'];
};


export type MutationRunFinisherArgs = {
  id: Scalars['ID']['input'];
};


export type MutationSetPrimaryDeployDomainArgs = {
  id: Scalars['ID']['input'];
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


export type MutationStartCheckoutArgs = {
  input: StartCheckoutInput;
};


export type MutationStopExecutionArgs = {
  id: Scalars['ID']['input'];
  reason: Scalars['String']['input'];
};


export type MutationUpdateNotificationPreferencesArgs = {
  input: NotificationPreferencesInput;
};


export type MutationUpdateProjectArgs = {
  id: Scalars['ID']['input'];
  input: UpdateProjectInput;
};


export type MutationVerifyEmailArgs = {
  token: Scalars['String']['input'];
};


export type MutationWalletCreateTopUpArgs = {
  amountUSD: Scalars['Float']['input'];
  provider?: InputMaybe<Scalars['String']['input']>;
};


export type MutationWriteProjectFilesArgs = {
  files: Array<WriteProjectFileInput>;
  id: Scalars['ID']['input'];
};

export type NextAction = {
  __typename?: 'NextAction';
  cta: Maybe<Scalars['String']['output']>;
  kind: Scalars['String']['output'];
  reason: Scalars['String']['output'];
  title: Scalars['String']['output'];
};

export type Notification = {
  __typename?: 'Notification';
  body: Scalars['String']['output'];
  createdAt: Scalars['DateTime']['output'];
  id: Scalars['ID']['output'];
  kind: Scalars['String']['output'];
  link: Maybe<Scalars['String']['output']>;
  readAt: Maybe<Scalars['DateTime']['output']>;
  severity: Scalars['String']['output'];
  title: Scalars['String']['output'];
};

export type NotificationPreferences = {
  __typename?: 'NotificationPreferences';
  onBudgetWarning: ChannelPref;
  onDeployDone: ChannelPref;
  onGateFailed: ChannelPref;
  onReceipt: ChannelPref;
  onRunComplete: ChannelPref;
  pauseAll: Scalars['Boolean']['output'];
  userId: Scalars['ID']['output'];
  weeklyDigest: Scalars['Boolean']['output'];
};

export type NotificationPreferencesInput = {
  onBudgetWarning?: InputMaybe<ChannelPrefInput>;
  onDeployDone?: InputMaybe<ChannelPrefInput>;
  onGateFailed?: InputMaybe<ChannelPrefInput>;
  onReceipt?: InputMaybe<ChannelPrefInput>;
  onRunComplete?: InputMaybe<ChannelPrefInput>;
  pauseAll?: InputMaybe<Scalars['Boolean']['input']>;
  weeklyDigest?: InputMaybe<Scalars['Boolean']['input']>;
};

export type OperationResult = {
  __typename?: 'OperationResult';
  message: Maybe<Scalars['String']['output']>;
  ok: Scalars['Boolean']['output'];
};

export type OperatorAuditEntry = {
  __typename?: 'OperatorAuditEntry';
  action: Scalars['String']['output'];
  hash: Scalars['String']['output'];
  id: Scalars['ID']['output'];
  outcome: Scalars['String']['output'];
  timestamp: Scalars['DateTime']['output'];
};

export type OperatorScaleSnapshot = {
  __typename?: 'OperatorScaleSnapshot';
  activeExecutions: Scalars['Int']['output'];
  queuedExecutions: Scalars['Int']['output'];
  sandboxCapacity: Scalars['Int']['output'];
  workerUtilizationPct: Scalars['Float']['output'];
};

export type OperatorWalletSnapshot = {
  __typename?: 'OperatorWalletSnapshot';
  balanceUSD: Scalars['Float']['output'];
  holdUSD: Scalars['Float']['output'];
  lifetimeSpendUSD: Scalars['Float']['output'];
  lifetimeTopUpUSD: Scalars['Float']['output'];
  tenantID: Scalars['ID']['output'];
};

export type PageInfo = {
  __typename?: 'PageInfo';
  endCursor: Maybe<Scalars['String']['output']>;
  hasNextPage: Scalars['Boolean']['output'];
  totalCount: Maybe<Scalars['Int']['output']>;
};

export type ParsedIdea = {
  __typename?: 'ParsedIdea';
  blueprintID: Scalars['String']['output'];
  blueprintReason: Scalars['String']['output'];
  confidence: Scalars['Float']['output'];
  stopLossUSD: Scalars['Float']['output'];
  suggestedBudgetUSD: Scalars['Float']['output'];
  summary: Scalars['String']['output'];
  tags: Array<Scalars['String']['output']>;
  title: Scalars['String']['output'];
};

export type Patch = {
  __typename?: 'Patch';
  appliedAt: Maybe<Scalars['DateTime']['output']>;
  author: Maybe<Scalars['String']['output']>;
  changes: Array<PatchChange>;
  conflicts: Maybe<Array<PatchConflict>>;
  createdAt: Scalars['DateTime']['output'];
  id: Scalars['ID']['output'];
  projectId: Scalars['ID']['output'];
  stage: Maybe<PatchStage>;
  stageId: Maybe<Scalars['ID']['output']>;
  status: PatchStatus;
  summary: Maybe<Scalars['String']['output']>;
  title: Maybe<Scalars['String']['output']>;
};

export type PatchAnchorOp = {
  __typename?: 'PatchAnchorOp';
  anchor: Scalars['String']['output'];
  path: Scalars['String']['output'];
  replacement: Scalars['String']['output'];
};

export type PatchChange = {
  __typename?: 'PatchChange';
  anchor: Maybe<Scalars['String']['output']>;
  content: Maybe<Scalars['String']['output']>;
  op: PatchChangeOp;
  path: Scalars['String']['output'];
  replacement: Maybe<Scalars['String']['output']>;
  symbol: Maybe<Scalars['String']['output']>;
};

export type PatchChangeInput = {
  anchor?: InputMaybe<Scalars['String']['input']>;
  content?: InputMaybe<Scalars['String']['input']>;
  op: PatchChangeOp;
  path: Scalars['String']['input'];
  replacement?: InputMaybe<Scalars['String']['input']>;
  symbol?: InputMaybe<Scalars['String']['input']>;
};

export enum PatchChangeOp {
  AnchorReplace = 'ANCHOR_REPLACE',
  Create = 'CREATE',
  Delete = 'DELETE',
  InsertAfter = 'INSERT_AFTER',
  InsertBefore = 'INSERT_BEFORE',
  Replace = 'REPLACE',
  SymbolReplace = 'SYMBOL_REPLACE'
}

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
  description: Maybe<Scalars['String']['output']>;
  id: Scalars['ID']['output'];
  name: Scalars['String']['output'];
  patchIds: Array<Scalars['ID']['output']>;
  projectId: Scalars['ID']['output'];
  rejectionReason: Maybe<Scalars['String']['output']>;
  status: PatchStageStatus;
  updatedAt: Scalars['DateTime']['output'];
};

export enum PatchStageStatus {
  Applied = 'APPLIED',
  Open = 'OPEN',
  Rejected = 'REJECTED',
  Reviewed = 'REVIEWED'
}

export enum PatchStatus {
  Applied = 'APPLIED',
  Approved = 'APPROVED',
  Conflicted = 'CONFLICTED',
  Proposed = 'PROPOSED',
  Rejected = 'REJECTED',
  RolledBack = 'ROLLED_BACK'
}

export type PatchSymbolOp = {
  __typename?: 'PatchSymbolOp';
  path: Scalars['String']['output'];
  replacement: Scalars['String']['output'];
  symbol: Scalars['String']['output'];
};

export type Plan = {
  __typename?: 'Plan';
  costCapUsd: Scalars['Decimal']['output'];
  description: Maybe<Scalars['String']['output']>;
  features: Array<Scalars['String']['output']>;
  name: Scalars['String']['output'];
  priceUsd: Scalars['Decimal']['output'];
  stripePriceId: Maybe<Scalars['String']['output']>;
  tier: Scalars['String']['output'];
};

export type PlanDeployInput = {
  artifactRef: Scalars['String']['input'];
  blueprintID?: InputMaybe<Scalars['String']['input']>;
  diffHash: Scalars['String']['input'];
  environment: Scalars['String']['input'];
  executionID?: InputMaybe<Scalars['ID']['input']>;
  metadata?: InputMaybe<Scalars['JSON']['input']>;
  projectID: Scalars['ID']['input'];
  target: Scalars['String']['input'];
};

export type ProfitDashboard = {
  __typename?: 'ProfitDashboard';
  activeExecutions: Scalars['Int']['output'];
  blockedExecutions: Scalars['Int']['output'];
  grossMarginPct: Scalars['Float']['output'];
  grossProfitUSD: Scalars['Float']['output'];
  otherCostUSD: Scalars['Float']['output'];
  providerCostUSD: Scalars['Float']['output'];
  refundCount: Scalars['Int']['output'];
  revenueUSD: Scalars['Float']['output'];
  sandboxCostUSD: Scalars['Float']['output'];
  topUpRate: Scalars['Float']['output'];
  windowEnd: Scalars['DateTime']['output'];
  windowStart: Scalars['DateTime']['output'];
};

export type ProfitGuardDecision = {
  __typename?: 'ProfitGuardDecision';
  createdAt: Scalars['DateTime']['output'];
  decision: Scalars['String']['output'];
  enforcementPoint: Scalars['String']['output'];
  estimatedStepCostUSD: Scalars['Float']['output'];
  executionID: Scalars['ID']['output'];
  expectedCompletionDelta: Scalars['Float']['output'];
  expectedMarginPct: Maybe<Scalars['Float']['output']>;
  id: Scalars['ID']['output'];
  reason: Scalars['String']['output'];
  recommendedProvider: Maybe<Scalars['String']['output']>;
  reservedUSD: Scalars['Float']['output'];
  riskScore: Maybe<Scalars['Float']['output']>;
  spentUSD: Scalars['Float']['output'];
};

export type Project = {
  __typename?: 'Project';
  createdAt: Scalars['DateTime']['output'];
  description: Maybe<Scalars['String']['output']>;
  files: Array<ProjectFile>;
  gates: Array<GateVerdict>;
  id: Scalars['ID']['output'];
  idea: Maybe<Scalars['String']['output']>;
  isPublic: Scalars['Boolean']['output'];
  name: Scalars['String']['output'];
  ownerId: Scalars['ID']['output'];
  status: Scalars['String']['output'];
  updatedAt: Scalars['DateTime']['output'];
};

export type ProjectFile = {
  __typename?: 'ProjectFile';
  content: Maybe<Scalars['String']['output']>;
  language: Maybe<Scalars['String']['output']>;
  path: Scalars['String']['output'];
  size: Maybe<Scalars['Int']['output']>;
  updatedAt: Maybe<Scalars['DateTime']['output']>;
};

export type ProposePatchInput = {
  author?: InputMaybe<Scalars['String']['input']>;
  changes: Array<PatchChangeInput>;
  projectId: Scalars['ID']['input'];
  summary?: InputMaybe<Scalars['String']['input']>;
  title?: InputMaybe<Scalars['String']['input']>;
};

export type PurchaseDeployDomainInput = {
  autoRenew?: InputMaybe<Scalars['Boolean']['input']>;
  contact?: InputMaybe<Scalars['JSON']['input']>;
  deployID?: InputMaybe<Scalars['ID']['input']>;
  domain: Scalars['String']['input'];
  expectedPriceUSD?: InputMaybe<Scalars['Float']['input']>;
  metadata?: InputMaybe<Scalars['JSON']['input']>;
  primary?: InputMaybe<Scalars['Boolean']['input']>;
  projectID: Scalars['ID']['input'];
  provider?: InputMaybe<Scalars['String']['input']>;
  registrar?: InputMaybe<Scalars['String']['input']>;
  years?: InputMaybe<Scalars['Int']['input']>;
};

export type Query = {
  __typename?: 'Query';
  agentTelemetry: Array<AgentCall>;
  agents: Array<Agent>;
  audit: Array<AuditEntry>;
  auditChainProof: AuditChainProof;
  auditExportCsvUrl: Scalars['String']['output'];
  auditExportPdfUrl: Scalars['String']['output'];
  auditExportPreview: AuditExportPreview;
  banditRanking: BanditRanking;
  blueprint: Maybe<Blueprint>;
  blueprintDashboard: BlueprintDashboard;
  blueprintRanking: Array<BlueprintStats>;
  blueprintStats: Maybe<BlueprintStats>;
  blueprints: Array<Blueprint>;
  cohortDashboard: CohortDashboard;
  deploy: Maybe<Deploy>;
  deployDomains: Array<DeployDomain>;
  deploys: Array<Deploy>;
  deviceCloudDevices: Array<DeviceCloudDevice>;
  domainAvailability: DomainAvailability;
  estimateExecutionCost: CostEstimate;
  execution: Maybe<Execution>;
  executionLedger: Array<WalletLedgerEntry>;
  executionSecurityReport: SecurityReport;
  executionSupportBundle: SupportBundle;
  executions: Array<Execution>;
  gate: Maybe<GateVerdict>;
  gates: Array<GateVerdict>;
  healthDashboard: HealthMetrics;
  learningDashboard: LearningDashboard;
  ledger: Array<WalletLedgerEntry>;
  ledgerRollup: LedgerRollup;
  mcpCatalog: Array<McpServerSpec>;
  mcpEnabled: Array<McpRunningServer>;
  me: Maybe<User>;
  mobileBuild: Maybe<MobileBuild>;
  mobileBuilds: Array<MobileBuild>;
  mobileSubmissions: Array<MobileSubmission>;
  myBudget: BudgetSummary;
  mySessions: Array<Session>;
  notificationPreferences: NotificationPreferences;
  notifications: Array<Notification>;
  operatorAbuseScore: AbuseScoreResult;
  operatorAuditCursor: Array<OperatorAuditEntry>;
  operatorPendingApprovals: Array<DeployApproval>;
  operatorScaleSnapshot: OperatorScaleSnapshot;
  operatorWalletSnapshot: OperatorWalletSnapshot;
  patch: Maybe<Patch>;
  patchSnapshots: Maybe<Scalars['JSON']['output']>;
  patches: Array<Patch>;
  pendingDeployApprovals: Array<DeployApproval>;
  ping: Scalars['String']['output'];
  plans: Array<Plan>;
  profitDashboard: ProfitDashboard;
  profitGuardDecisions: Array<ProfitGuardDecision>;
  project: Maybe<Project>;
  projectExecutions: Array<Execution>;
  projectFiles: Array<ProjectFile>;
  projects: Array<Project>;
  rates: Array<Rate>;
  recentErrors: Array<ErrorAggregate>;
  recentLogs: Array<LogEntry>;
  scaleDashboard: ScaleDashboard;
  stage: Maybe<PatchStage>;
  stages: Array<PatchStage>;
  tenantProfitToday: LedgerRollup;
  unreadNotificationCount: Scalars['Int']['output'];
  vault: VaultSnapshot;
  verifyAudit: AuditVerifyResult;
  version: VersionInfo;
  wallet: Wallet;
  walletAvailableProviders: Array<WalletProvider>;
  walletTopUps: Array<WalletTopUp>;
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


export type QueryAuditChainProofArgs = {
  since: Scalars['DateTime']['input'];
  until: Scalars['DateTime']['input'];
};


export type QueryAuditExportCsvUrlArgs = {
  query?: InputMaybe<AuditQueryInput>;
};


export type QueryAuditExportPdfUrlArgs = {
  query?: InputMaybe<AuditQueryInput>;
};


export type QueryAuditExportPreviewArgs = {
  filter: AuditExportFilter;
};


export type QueryBanditRankingArgs = {
  lookback?: InputMaybe<Scalars['Int']['input']>;
};


export type QueryBlueprintArgs = {
  id: Scalars['String']['input'];
};


export type QueryBlueprintRankingArgs = {
  byMetric?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
};


export type QueryBlueprintStatsArgs = {
  id: Scalars['String']['input'];
};


export type QueryCohortDashboardArgs = {
  sinceMonth: Scalars['DateTime']['input'];
};


export type QueryDeployArgs = {
  id: Scalars['ID']['input'];
};


export type QueryDeployDomainsArgs = {
  projectID: Scalars['ID']['input'];
};


export type QueryDeploysArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
};


export type QueryDeviceCloudDevicesArgs = {
  platform: Scalars['String']['input'];
};


export type QueryDomainAvailabilityArgs = {
  domain: Scalars['String']['input'];
  registrar?: InputMaybe<Scalars['String']['input']>;
};


export type QueryEstimateExecutionCostArgs = {
  input: EstimateInput;
};


export type QueryExecutionArgs = {
  id: Scalars['ID']['input'];
};


export type QueryExecutionLedgerArgs = {
  executionID: Scalars['ID']['input'];
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
};


export type QueryExecutionSecurityReportArgs = {
  executionID: Scalars['ID']['input'];
};


export type QueryExecutionSupportBundleArgs = {
  executionID: Scalars['ID']['input'];
};


export type QueryExecutionsArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
};


export type QueryGateArgs = {
  gate: Scalars['String']['input'];
  projectId: Scalars['ID']['input'];
};


export type QueryGatesArgs = {
  projectId: Scalars['ID']['input'];
  sub?: InputMaybe<Scalars['ID']['input']>;
};


export type QueryLedgerArgs = {
  filter?: InputMaybe<LedgerFilter>;
};


export type QueryLedgerRollupArgs = {
  since: Scalars['DateTime']['input'];
  until: Scalars['DateTime']['input'];
};


export type QueryMcpEnabledArgs = {
  projectId: Scalars['String']['input'];
};


export type QueryMobileBuildArgs = {
  buildId: Scalars['String']['input'];
};


export type QueryMobileBuildsArgs = {
  projectId: Scalars['String']['input'];
};


export type QueryMobileSubmissionsArgs = {
  projectId: Scalars['String']['input'];
};


export type QueryNotificationsArgs = {
  unreadOnly?: InputMaybe<Scalars['Boolean']['input']>;
};


export type QueryOperatorAbuseScoreArgs = {
  tenantID: Scalars['ID']['input'];
  userID: Scalars['ID']['input'];
};


export type QueryOperatorAuditCursorArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
  since: Scalars['DateTime']['input'];
};


export type QueryOperatorPendingApprovalsArgs = {
  tenantID?: InputMaybe<Scalars['ID']['input']>;
};


export type QueryOperatorWalletSnapshotArgs = {
  tenantID: Scalars['ID']['input'];
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


export type QueryProfitDashboardArgs = {
  since: Scalars['DateTime']['input'];
  until: Scalars['DateTime']['input'];
};


export type QueryProfitGuardDecisionsArgs = {
  executionID?: InputMaybe<Scalars['ID']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
};


export type QueryProjectArgs = {
  id: Scalars['ID']['input'];
};


export type QueryProjectExecutionsArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
  projectId: Scalars['ID']['input'];
};


export type QueryProjectFilesArgs = {
  id: Scalars['ID']['input'];
};


export type QueryProjectsArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
};


export type QueryRecentErrorsArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
};


export type QueryRecentLogsArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
  minLevel?: InputMaybe<Scalars['String']['input']>;
  since?: InputMaybe<Scalars['DateTime']['input']>;
};


export type QueryStageArgs = {
  id: Scalars['ID']['input'];
};


export type QueryStagesArgs = {
  projectId: Scalars['ID']['input'];
};


export type QueryWalletTopUpsArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
};

export type Rate = {
  __typename?: 'Rate';
  completionPerMTok: Scalars['Decimal']['output'];
  model: Scalars['String']['output'];
  promptPerMTok: Scalars['Decimal']['output'];
  provider: Scalars['String']['output'];
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

export type ReserveDeploySubdomainInput = {
  deployID?: InputMaybe<Scalars['ID']['input']>;
  metadata?: InputMaybe<Scalars['JSON']['input']>;
  primary?: InputMaybe<Scalars['Boolean']['input']>;
  projectID: Scalars['ID']['input'];
  provider?: InputMaybe<Scalars['String']['input']>;
  subdomain?: InputMaybe<Scalars['String']['input']>;
};

export type RouteBundle = {
  __typename?: 'RouteBundle';
  firstLoadKB: Scalars['Float']['output'];
  route: Scalars['String']['output'];
  totalKB: Scalars['Float']['output'];
};

export type RunDoneEvent = {
  __typename?: 'RunDoneEvent';
  ok: Scalars['Boolean']['output'];
  summary: Maybe<Scalars['JSON']['output']>;
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
  message: Maybe<Scalars['String']['output']>;
  status: Scalars['String']['output'];
  ts: Scalars['DateTime']['output'];
};

export type ScaleDashboard = {
  __typename?: 'ScaleDashboard';
  activeExecutions: Scalars['Int']['output'];
  queueWaitSec: Scalars['Float']['output'];
  queuedExecutions: Scalars['Int']['output'];
  sandboxCapacity: Scalars['Int']['output'];
  scaleHealth: Scalars['Float']['output'];
  workerUtilizationPct: Scalars['Float']['output'];
};

export type SecurityReport = {
  __typename?: 'SecurityReport';
  blockedDeploy: Scalars['Boolean']['output'];
  executionID: Scalars['ID']['output'];
  findings: Array<SecurityReportFinding>;
  generatedAt: Scalars['DateTime']['output'];
  outdatedDeps: Scalars['Int']['output'];
  overallScore: Scalars['Float']['output'];
  owaspCoverage: Scalars['JSON']['output'];
  secretsFound: Scalars['Int']['output'];
  status: Scalars['String']['output'];
  tenantID: Scalars['ID']['output'];
};

export type SecurityReportFinding = {
  __typename?: 'SecurityReportFinding';
  category: Scalars['String']['output'];
  detectedAt: Scalars['DateTime']['output'];
  id: Scalars['ID']['output'];
  line: Scalars['Int']['output'];
  path: Scalars['String']['output'];
  remediation: Scalars['String']['output'];
  ruleID: Scalars['String']['output'];
  severity: Scalars['String']['output'];
  summary: Scalars['String']['output'];
};

export type Session = {
  __typename?: 'Session';
  current: Maybe<Scalars['Boolean']['output']>;
  expiresAt: Maybe<Scalars['DateTime']['output']>;
  ipAddress: Maybe<Scalars['String']['output']>;
  jti: Maybe<Scalars['ID']['output']>;
  lastSeenAt: Maybe<Scalars['DateTime']['output']>;
  token: Scalars['String']['output'];
  user: User;
  userAgent: Maybe<Scalars['String']['output']>;
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

export type StripeCheckoutSession = {
  __typename?: 'StripeCheckoutSession';
  sessionId: Scalars['String']['output'];
  url: Scalars['String']['output'];
};

export type StudioBootstrap = {
  __typename?: 'StudioBootstrap';
  costEstimate: CostEstimate;
  execution: Execution;
  idea: ParsedIdea;
  project: Project;
};

export type Subscription = {
  __typename?: 'Subscription';
  _heartbeat: HeartbeatEvent;
  costStream: CostDelta;
  deployFeed: DeployEvent;
  executionFeed: ExecutionEvent;
  inlineCompletion: InlineDelta;
  mobileBuildStatus: MobileBuild;
  notificationStream: Notification;
  runProject: RunEvent;
};


export type SubscriptionDeployFeedArgs = {
  id: Scalars['ID']['input'];
};


export type SubscriptionExecutionFeedArgs = {
  id: Scalars['ID']['input'];
};


export type SubscriptionInlineCompletionArgs = {
  input: InlineInput;
};


export type SubscriptionMobileBuildStatusArgs = {
  buildId: Scalars['String']['input'];
};


export type SubscriptionRunProjectArgs = {
  projectId: Scalars['ID']['input'];
};

export type SupportBundle = {
  __typename?: 'SupportBundle';
  changedFiles: Array<Scalars['String']['output']>;
  costReport: CostReport;
  executionID: Scalars['ID']['output'];
  gateReport: GateReport;
  generatedAt: Scalars['DateTime']['output'];
  nextBestAction: NextAction;
  patchCount: Scalars['Int']['output'];
  previewURL: Maybe<Scalars['String']['output']>;
  productionURL: Maybe<Scalars['String']['output']>;
  securityReport: SupportSecurityReport;
  status: Scalars['String']['output'];
  tenantID: Scalars['ID']['output'];
};

export type SupportSecurityFinding = {
  __typename?: 'SupportSecurityFinding';
  line: Scalars['Int']['output'];
  path: Scalars['String']['output'];
  ruleID: Scalars['String']['output'];
  severity: Scalars['String']['output'];
  summary: Scalars['String']['output'];
};

export type SupportSecurityReport = {
  __typename?: 'SupportSecurityReport';
  blockedDeploy: Scalars['Boolean']['output'];
  findings: Array<SupportSecurityFinding>;
  passRate: Scalars['Float']['output'];
};

export enum SymbolAction {
  DeleteSymbol = 'DELETE_SYMBOL',
  InsertAfter = 'INSERT_AFTER',
  Rename = 'RENAME',
  ReplaceBody = 'REPLACE_BODY'
}

export enum SymbolKind {
  Class = 'CLASS',
  Const = 'CONST',
  Function = 'FUNCTION',
  Interface = 'INTERFACE',
  Method = 'METHOD',
  Struct = 'STRUCT',
  Type = 'TYPE',
  Var = 'VAR'
}

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

export type User = {
  __typename?: 'User';
  createdAt: Scalars['DateTime']['output'];
  email: Scalars['String']['output'];
  emailVerifiedAt: Maybe<Scalars['DateTime']['output']>;
  id: Scalars['ID']['output'];
  name: Maybe<Scalars['String']['output']>;
  orgId: Maybe<Scalars['String']['output']>;
  plan: Maybe<Scalars['String']['output']>;
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

export type VersionInfo = {
  __typename?: 'VersionInfo';
  buildTime: Scalars['String']['output'];
  commit: Scalars['String']['output'];
  service: Scalars['String']['output'];
  version: Scalars['String']['output'];
};

export type Wallet = {
  __typename?: 'Wallet';
  availableUSD: Scalars['Float']['output'];
  balanceUSD: Scalars['Float']['output'];
  holdUSD: Scalars['Float']['output'];
  lifetimeSpendUSD: Scalars['Float']['output'];
  lifetimeTopUpUSD: Scalars['Float']['output'];
  tenantID: Scalars['ID']['output'];
  updatedAt: Scalars['DateTime']['output'];
};

export type WalletCheckoutSession = {
  __typename?: 'WalletCheckoutSession';
  provider: Scalars['String']['output'];
  sessionID: Scalars['String']['output'];
  url: Scalars['String']['output'];
};

export type WalletLedgerEntry = {
  __typename?: 'WalletLedgerEntry';
  amountUSD: Scalars['Float']['output'];
  billable: Scalars['Boolean']['output'];
  createdAt: Scalars['DateTime']['output'];
  direction: Scalars['String']['output'];
  entryType: Scalars['String']['output'];
  executionID: Maybe<Scalars['ID']['output']>;
  id: Scalars['ID']['output'];
  marginRelevant: Scalars['Boolean']['output'];
  metadata: Scalars['JSON']['output'];
  provider: Maybe<Scalars['String']['output']>;
  tenantID: Scalars['ID']['output'];
};

export type WalletProvider = {
  __typename?: 'WalletProvider';
  isPrimary: Scalars['Boolean']['output'];
  label: Scalars['String']['output'];
  name: Scalars['String']['output'];
};

export type WalletTopUp = {
  __typename?: 'WalletTopUp';
  amountUSD: Scalars['Float']['output'];
  completedAt: Maybe<Scalars['DateTime']['output']>;
  createdAt: Scalars['DateTime']['output'];
  id: Scalars['ID']['output'];
  provider: Scalars['String']['output'];
  status: Scalars['String']['output'];
};

export type Weakness = {
  __typename?: 'Weakness';
  description: Scalars['String']['output'];
  dimension: Scalars['String']['output'];
  severity: Scalars['String']['output'];
  suggestedAction: Scalars['String']['output'];
};

export type WriteProjectFileInput = {
  content: Scalars['String']['input'];
  path: Scalars['String']['input'];
};

export type LearningDashboardQueryVariables = Exact<{ [key: string]: never; }>;


export type LearningDashboardQuery = { __typename?: 'Query', learningDashboard: { __typename?: 'LearningDashboard', outcomeEventsToday: number, outcomeEventsAllTime: number, reuseRateLast7d: number, repairRecipeHitsLast7d: number, banditConfidence: number, averageCompletionScore: number, averageMarginPctLast7d: number, lastIndexedAt: string | null, gateFailureRates: Array<{ __typename?: 'GateFailureRate', gate: string, failureRate: number, sampleSize: number }>, blueprintSuccessRates: Array<{ __typename?: 'BlueprintSuccessRate', blueprintID: string, blueprintName: string, successRate: number, avgMargin: number }>, weaknesses: Array<{ __typename?: 'Weakness', dimension: string, description: string, severity: string, suggestedAction: string }> } };

export type BanditRankingQueryVariables = Exact<{
  lookback?: InputMaybe<Scalars['Int']['input']>;
}>;


export type BanditRankingQuery = { __typename?: 'Query', banditRanking: { __typename?: 'BanditRanking', lookback: number, strategy: string, capabilities: Array<{ __typename?: 'BanditCapability', capability: string, total: number, winners: Array<{ __typename?: 'BanditWinner', provider: string, model: string | null, share: number, meanReward: number, isLeader: boolean }> }> } };

export type CurrentUserQueryVariables = Exact<{ [key: string]: never; }>;


export type CurrentUserQuery = { __typename?: 'Query', me: { __typename?: 'User', id: string, email: string, name: string | null, plan: string | null, orgId: string | null, telemetryOptOut: boolean, emailVerifiedAt: string | null, createdAt: string } | null };

export type SignInMutationVariables = Exact<{
  input: SignInInput;
}>;


export type SignInMutation = { __typename?: 'Mutation', signIn: { __typename?: 'Session', token: string, expiresAt: string | null, user: { __typename?: 'User', id: string, email: string, name: string | null, plan: string | null, orgId: string | null, telemetryOptOut: boolean, emailVerifiedAt: string | null, createdAt: string } } };

export type SignUpMutationVariables = Exact<{
  input: SignUpInput;
}>;


export type SignUpMutation = { __typename?: 'Mutation', signUp: { __typename?: 'Session', token: string, expiresAt: string | null, user: { __typename?: 'User', id: string, email: string, name: string | null, plan: string | null, orgId: string | null, telemetryOptOut: boolean, emailVerifiedAt: string | null, createdAt: string } } };

export type SignOutMutationVariables = Exact<{ [key: string]: never; }>;


export type SignOutMutation = { __typename?: 'Mutation', signOut: { __typename?: 'OperationResult', ok: boolean, message: string | null } };

export type RequestPasswordResetMutationVariables = Exact<{
  email: Scalars['String']['input'];
}>;


export type RequestPasswordResetMutation = { __typename?: 'Mutation', requestPasswordReset: { __typename?: 'OperationResult', ok: boolean, message: string | null } };

export type ResetPasswordMutationVariables = Exact<{
  token: Scalars['String']['input'];
  newPassword: Scalars['String']['input'];
}>;


export type ResetPasswordMutation = { __typename?: 'Mutation', resetPassword: { __typename?: 'Session', token: string, expiresAt: string | null, user: { __typename?: 'User', id: string, email: string, name: string | null, plan: string | null, orgId: string | null, telemetryOptOut: boolean, emailVerifiedAt: string | null, createdAt: string } } };

export type BlueprintsQueryVariables = Exact<{ [key: string]: never; }>;


export type BlueprintsQuery = { __typename?: 'Query', blueprints: Array<{ __typename?: 'Blueprint', id: string, name: string, description: string, category: string, costPriorUSD: number, expectedTimeToPreviewSec: number, supportedGates: Array<string>, fileCount: number }> };

export type BlueprintQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type BlueprintQuery = { __typename?: 'Query', blueprint: { __typename?: 'Blueprint', id: string, name: string, description: string, category: string, costPriorUSD: number, expectedTimeToPreviewSec: number, supportedGates: Array<string>, fileCount: number } | null };

export type BlueprintStatsQueryVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type BlueprintStatsQuery = { __typename?: 'Query', blueprintStats: { __typename?: 'BlueprintStats', blueprintID: string, executions: number, previewSuccess: number, refunds: number, repairCount: number, avgRevenueUSD: number, avgCostUSD: number, grossMarginPct: number, avgCompletionScore: number, avgTimeToPreviewSec: number } | null };

export type BlueprintRankingQueryVariables = Exact<{
  byMetric?: InputMaybe<Scalars['String']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
}>;


export type BlueprintRankingQuery = { __typename?: 'Query', blueprintRanking: Array<{ __typename?: 'BlueprintStats', blueprintID: string, executions: number, previewSuccess: number, refunds: number, repairCount: number, avgRevenueUSD: number, avgCostUSD: number, grossMarginPct: number, avgCompletionScore: number, avgTimeToPreviewSec: number }> };

export type LedgerEntryRowFragment = { __typename?: 'LedgerEntry', id: string, projectId: string | null, provider: string | null, model: string | null, promptTokens: number, completionTokens: number, costUsd: string, revenueUsd: string, ts: string, agent: string | null, durationMs: number | null };

export type PlansQueryVariables = Exact<{ [key: string]: never; }>;


export type PlansQuery = { __typename?: 'Query', plans: Array<{ __typename?: 'Plan', tier: string, name: string, priceUsd: string, costCapUsd: string, description: string | null, features: Array<string>, stripePriceId: string | null }> };

export type RatesQueryVariables = Exact<{ [key: string]: never; }>;


export type RatesQuery = { __typename?: 'Query', rates: Array<{ __typename?: 'Rate', provider: string, model: string, promptPerMTok: string, completionPerMTok: string }> };

export type VaultQueryVariables = Exact<{ [key: string]: never; }>;


export type VaultQuery = { __typename?: 'Query', vault: { __typename?: 'VaultSnapshot', revenueUsd: string, providerCostUsd: string, marginUsd: string, entries: number, asOf: string } };

export type MyBudgetQueryVariables = Exact<{ [key: string]: never; }>;


export type MyBudgetQuery = { __typename?: 'Query', myBudget: { __typename?: 'BudgetSummary', userId: string, email: string, tier: string, spentUsd: string, entries: Array<{ __typename?: 'LedgerEntry', id: string, projectId: string | null, provider: string | null, model: string | null, promptTokens: number, completionTokens: number, costUsd: string, revenueUsd: string, ts: string, agent: string | null, durationMs: number | null }> } };

export type StartCheckoutMutationVariables = Exact<{
  input: StartCheckoutInput;
}>;


export type StartCheckoutMutation = { __typename?: 'Mutation', startCheckout: { __typename?: 'StripeCheckoutSession', sessionId: string, url: string } };

export type ProfitDashboardQueryVariables = Exact<{
  since: Scalars['DateTime']['input'];
  until: Scalars['DateTime']['input'];
}>;


export type ProfitDashboardQuery = { __typename?: 'Query', profitDashboard: { __typename?: 'ProfitDashboard', windowStart: string, windowEnd: string, revenueUSD: number, providerCostUSD: number, sandboxCostUSD: number, otherCostUSD: number, grossProfitUSD: number, grossMarginPct: number, activeExecutions: number, blockedExecutions: number, refundCount: number, topUpRate: number } };

export type ScaleDashboardQueryVariables = Exact<{ [key: string]: never; }>;


export type ScaleDashboardQuery = { __typename?: 'Query', scaleDashboard: { __typename?: 'ScaleDashboard', activeExecutions: number, queuedExecutions: number, queueWaitSec: number, sandboxCapacity: number, workerUtilizationPct: number, scaleHealth: number } };

export type CohortDashboardQueryVariables = Exact<{
  sinceMonth: Scalars['DateTime']['input'];
}>;


export type CohortDashboardQuery = { __typename?: 'Query', cohortDashboard: { __typename?: 'CohortDashboard', cohorts: Array<{ __typename?: 'Cohort', month: string, newPayingUsers: number, secondExecutionUsers: number, day7RepeatUsers: number, day30RepeatUsers: number, avgSpendUSD: number, grossMarginPct: number, completionRate: number, refundRate: number, supportTicketsPerExec: number }> } };

export type BlueprintDashboardQueryVariables = Exact<{ [key: string]: never; }>;


export type BlueprintDashboardQuery = { __typename?: 'Query', blueprintDashboard: { __typename?: 'BlueprintDashboard', blueprints: Array<{ __typename?: 'DashboardBlueprintStats', blueprintID: string, executions: number, avgRevenueUSD: number, avgCostUSD: number, grossMarginPct: number, previewSuccess: number, refunds: number, repairCount: number, avgCompletionScore: number }> } };

export type DeployCoreFragment = { __typename?: 'Deploy', id: string, tenantID: string, projectID: string, executionID: string | null, blueprintID: string | null, target: string, environment: string, status: string, providerDeploymentID: string | null, previewURL: string | null, productionURL: string | null, diffHash: string | null, artifactHash: string | null, gateSummary: unknown, costUSD: number, createdAt: string, previewReadyAt: string | null, promotedAt: string | null, rolledBackAt: string | null };

export type DeployApprovalCoreFragment = { __typename?: 'DeployApproval', id: string, deployID: string, tenantID: string, status: string, diffHash: string, artifactHash: string, gateSummary: unknown, costImpactUSD: number, expiresAt: string, decisionNote: string | null, requestedAt: string, decidedAt: string | null };

export type DeployDomainCoreFragment = { __typename?: 'DeployDomain', id: string, tenantID: string, projectID: string, deployID: string | null, hostname: string, kind: string, status: string, provider: string, registrar: string | null, primary: boolean, verificationStatus: string, certificateStatus: string, instructions: string, metadata: unknown, createdAt: string, updatedAt: string, verifiedAt: string | null, liveAt: string | null, dnsRecords: Array<{ __typename?: 'DNSRecord', type: string, name: string, value: string, ttl: number | null }> };

export type DeploysQueryVariables = Exact<{
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
}>;


export type DeploysQuery = { __typename?: 'Query', deploys: Array<{ __typename?: 'Deploy', id: string, tenantID: string, projectID: string, executionID: string | null, blueprintID: string | null, target: string, environment: string, status: string, providerDeploymentID: string | null, previewURL: string | null, productionURL: string | null, diffHash: string | null, artifactHash: string | null, gateSummary: unknown, costUSD: number, createdAt: string, previewReadyAt: string | null, promotedAt: string | null, rolledBackAt: string | null }> };

export type DeployQueryVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type DeployQuery = { __typename?: 'Query', deploy: { __typename?: 'Deploy', id: string, tenantID: string, projectID: string, executionID: string | null, blueprintID: string | null, target: string, environment: string, status: string, providerDeploymentID: string | null, previewURL: string | null, productionURL: string | null, diffHash: string | null, artifactHash: string | null, gateSummary: unknown, costUSD: number, createdAt: string, previewReadyAt: string | null, promotedAt: string | null, rolledBackAt: string | null } | null };

export type PendingDeployApprovalsQueryVariables = Exact<{ [key: string]: never; }>;


export type PendingDeployApprovalsQuery = { __typename?: 'Query', pendingDeployApprovals: Array<{ __typename?: 'DeployApproval', id: string, deployID: string, tenantID: string, status: string, diffHash: string, artifactHash: string, gateSummary: unknown, costImpactUSD: number, expiresAt: string, decisionNote: string | null, requestedAt: string, decidedAt: string | null }> };

export type DeployDomainsQueryVariables = Exact<{
  projectID: Scalars['ID']['input'];
}>;


export type DeployDomainsQuery = { __typename?: 'Query', deployDomains: Array<{ __typename?: 'DeployDomain', id: string, tenantID: string, projectID: string, deployID: string | null, hostname: string, kind: string, status: string, provider: string, registrar: string | null, primary: boolean, verificationStatus: string, certificateStatus: string, instructions: string, metadata: unknown, createdAt: string, updatedAt: string, verifiedAt: string | null, liveAt: string | null, dnsRecords: Array<{ __typename?: 'DNSRecord', type: string, name: string, value: string, ttl: number | null }> }> };

export type DomainAvailabilityQueryVariables = Exact<{
  domain: Scalars['String']['input'];
  registrar?: InputMaybe<Scalars['String']['input']>;
}>;


export type DomainAvailabilityQuery = { __typename?: 'Query', domainAvailability: { __typename?: 'DomainAvailability', domain: string, available: boolean, registrar: string, priceUSD: number, currency: string, premium: boolean, canPurchase: boolean, reason: string | null, checkedAt: string, requirements: Array<string> } };

export type PlanDeployMutationVariables = Exact<{
  input: PlanDeployInput;
}>;


export type PlanDeployMutation = { __typename?: 'Mutation', planDeploy: { __typename?: 'Deploy', id: string, tenantID: string, projectID: string, executionID: string | null, blueprintID: string | null, target: string, environment: string, status: string, providerDeploymentID: string | null, previewURL: string | null, productionURL: string | null, diffHash: string | null, artifactHash: string | null, gateSummary: unknown, costUSD: number, createdAt: string, previewReadyAt: string | null, promotedAt: string | null, rolledBackAt: string | null } };

export type CancelDeployMutationVariables = Exact<{
  deployID: Scalars['ID']['input'];
  reason: Scalars['String']['input'];
}>;


export type CancelDeployMutation = { __typename?: 'Mutation', cancelDeploy: { __typename?: 'Deploy', id: string, tenantID: string, projectID: string, executionID: string | null, blueprintID: string | null, target: string, environment: string, status: string, providerDeploymentID: string | null, previewURL: string | null, productionURL: string | null, diffHash: string | null, artifactHash: string | null, gateSummary: unknown, costUSD: number, createdAt: string, previewReadyAt: string | null, promotedAt: string | null, rolledBackAt: string | null } };

export type BuildDeployPreviewMutationVariables = Exact<{
  deployID: Scalars['ID']['input'];
}>;


export type BuildDeployPreviewMutation = { __typename?: 'Mutation', buildDeployPreview: { __typename?: 'Deploy', id: string, tenantID: string, projectID: string, executionID: string | null, blueprintID: string | null, target: string, environment: string, status: string, providerDeploymentID: string | null, previewURL: string | null, productionURL: string | null, diffHash: string | null, artifactHash: string | null, gateSummary: unknown, costUSD: number, createdAt: string, previewReadyAt: string | null, promotedAt: string | null, rolledBackAt: string | null } };

export type RequestDeployApprovalMutationVariables = Exact<{
  deployID: Scalars['ID']['input'];
  expiresInMinutes?: InputMaybe<Scalars['Int']['input']>;
}>;


export type RequestDeployApprovalMutation = { __typename?: 'Mutation', requestDeployApproval: { __typename?: 'DeployApproval', id: string, deployID: string, tenantID: string, status: string, diffHash: string, artifactHash: string, gateSummary: unknown, costImpactUSD: number, expiresAt: string, decisionNote: string | null, requestedAt: string, decidedAt: string | null } };

export type DecideDeployApprovalMutationVariables = Exact<{
  approvalID: Scalars['ID']['input'];
  approve: Scalars['Boolean']['input'];
  note?: InputMaybe<Scalars['String']['input']>;
}>;


export type DecideDeployApprovalMutation = { __typename?: 'Mutation', decideDeployApproval: { __typename?: 'DeployApproval', id: string, deployID: string, tenantID: string, status: string, diffHash: string, artifactHash: string, gateSummary: unknown, costImpactUSD: number, expiresAt: string, decisionNote: string | null, requestedAt: string, decidedAt: string | null } };

export type PromoteDeployMutationVariables = Exact<{
  deployID: Scalars['ID']['input'];
}>;


export type PromoteDeployMutation = { __typename?: 'Mutation', promoteDeploy: { __typename?: 'Deploy', id: string, tenantID: string, projectID: string, executionID: string | null, blueprintID: string | null, target: string, environment: string, status: string, providerDeploymentID: string | null, previewURL: string | null, productionURL: string | null, diffHash: string | null, artifactHash: string | null, gateSummary: unknown, costUSD: number, createdAt: string, previewReadyAt: string | null, promotedAt: string | null, rolledBackAt: string | null } };

export type RollbackDeployMutationVariables = Exact<{
  deployID: Scalars['ID']['input'];
  reason: Scalars['String']['input'];
}>;


export type RollbackDeployMutation = { __typename?: 'Mutation', rollbackDeploy: { __typename?: 'Deploy', id: string, tenantID: string, projectID: string, executionID: string | null, blueprintID: string | null, target: string, environment: string, status: string, providerDeploymentID: string | null, previewURL: string | null, productionURL: string | null, diffHash: string | null, artifactHash: string | null, gateSummary: unknown, costUSD: number, createdAt: string, previewReadyAt: string | null, promotedAt: string | null, rolledBackAt: string | null } };

export type ReserveDeploySubdomainMutationVariables = Exact<{
  input: ReserveDeploySubdomainInput;
}>;


export type ReserveDeploySubdomainMutation = { __typename?: 'Mutation', reserveDeploySubdomain: { __typename?: 'DeployDomain', id: string, tenantID: string, projectID: string, deployID: string | null, hostname: string, kind: string, status: string, provider: string, registrar: string | null, primary: boolean, verificationStatus: string, certificateStatus: string, instructions: string, metadata: unknown, createdAt: string, updatedAt: string, verifiedAt: string | null, liveAt: string | null, dnsRecords: Array<{ __typename?: 'DNSRecord', type: string, name: string, value: string, ttl: number | null }> } };

export type ConnectDeployDomainMutationVariables = Exact<{
  input: ConnectDeployDomainInput;
}>;


export type ConnectDeployDomainMutation = { __typename?: 'Mutation', connectDeployDomain: { __typename?: 'DeployDomain', id: string, tenantID: string, projectID: string, deployID: string | null, hostname: string, kind: string, status: string, provider: string, registrar: string | null, primary: boolean, verificationStatus: string, certificateStatus: string, instructions: string, metadata: unknown, createdAt: string, updatedAt: string, verifiedAt: string | null, liveAt: string | null, dnsRecords: Array<{ __typename?: 'DNSRecord', type: string, name: string, value: string, ttl: number | null }> } };

export type CheckDeployDomainMutationVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type CheckDeployDomainMutation = { __typename?: 'Mutation', checkDeployDomain: { __typename?: 'DeployDomain', id: string, tenantID: string, projectID: string, deployID: string | null, hostname: string, kind: string, status: string, provider: string, registrar: string | null, primary: boolean, verificationStatus: string, certificateStatus: string, instructions: string, metadata: unknown, createdAt: string, updatedAt: string, verifiedAt: string | null, liveAt: string | null, dnsRecords: Array<{ __typename?: 'DNSRecord', type: string, name: string, value: string, ttl: number | null }> } };

export type SetPrimaryDeployDomainMutationVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type SetPrimaryDeployDomainMutation = { __typename?: 'Mutation', setPrimaryDeployDomain: { __typename?: 'DeployDomain', id: string, tenantID: string, projectID: string, deployID: string | null, hostname: string, kind: string, status: string, provider: string, registrar: string | null, primary: boolean, verificationStatus: string, certificateStatus: string, instructions: string, metadata: unknown, createdAt: string, updatedAt: string, verifiedAt: string | null, liveAt: string | null, dnsRecords: Array<{ __typename?: 'DNSRecord', type: string, name: string, value: string, ttl: number | null }> } };

export type PurchaseDeployDomainMutationVariables = Exact<{
  input: PurchaseDeployDomainInput;
}>;


export type PurchaseDeployDomainMutation = { __typename?: 'Mutation', purchaseDeployDomain: { __typename?: 'DeployDomain', id: string, tenantID: string, projectID: string, deployID: string | null, hostname: string, kind: string, status: string, provider: string, registrar: string | null, primary: boolean, verificationStatus: string, certificateStatus: string, instructions: string, metadata: unknown, createdAt: string, updatedAt: string, verifiedAt: string | null, liveAt: string | null, dnsRecords: Array<{ __typename?: 'DNSRecord', type: string, name: string, value: string, ttl: number | null }> } };

export type DeployFeedSubscriptionVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type DeployFeedSubscription = { __typename?: 'Subscription', deployFeed: { __typename?: 'DeployEvent', deployID: string, eventType: string, payload: unknown, createdAt: string } };

export type ExecutionCoreFragment = { __typename?: 'Execution', id: string, tenantID: string, projectID: string | null, blueprintID: string | null, workspaceID: string | null, status: string, budgetUSD: number, reservedUSD: number, spentUSD: number, refundedUSD: number, revenueUSD: number, providerCostUSD: number, sandboxCostUSD: number, storageCostUSD: number, deploymentCostUSD: number, completionScore: number, grossMarginPct: number | null, expectedCompletionDelta: number | null, riskScore: number | null, stopLossUSD: number | null, promptSummary: string | null, failureReason: string | null, metadata: unknown, createdAt: string, admittedAt: string | null, startedAt: string | null, endedAt: string | null };

export type ExecutionsQueryVariables = Exact<{
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
}>;


export type ExecutionsQuery = { __typename?: 'Query', executions: Array<{ __typename?: 'Execution', id: string, tenantID: string, projectID: string | null, blueprintID: string | null, workspaceID: string | null, status: string, budgetUSD: number, reservedUSD: number, spentUSD: number, refundedUSD: number, revenueUSD: number, providerCostUSD: number, sandboxCostUSD: number, storageCostUSD: number, deploymentCostUSD: number, completionScore: number, grossMarginPct: number | null, expectedCompletionDelta: number | null, riskScore: number | null, stopLossUSD: number | null, promptSummary: string | null, failureReason: string | null, metadata: unknown, createdAt: string, admittedAt: string | null, startedAt: string | null, endedAt: string | null }> };

export type ProjectExecutionsQueryVariables = Exact<{
  projectId: Scalars['ID']['input'];
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
}>;


export type ProjectExecutionsQuery = { __typename?: 'Query', projectExecutions: Array<{ __typename?: 'Execution', id: string, tenantID: string, projectID: string | null, blueprintID: string | null, workspaceID: string | null, status: string, budgetUSD: number, reservedUSD: number, spentUSD: number, refundedUSD: number, revenueUSD: number, providerCostUSD: number, sandboxCostUSD: number, storageCostUSD: number, deploymentCostUSD: number, completionScore: number, grossMarginPct: number | null, expectedCompletionDelta: number | null, riskScore: number | null, stopLossUSD: number | null, promptSummary: string | null, failureReason: string | null, metadata: unknown, createdAt: string, admittedAt: string | null, startedAt: string | null, endedAt: string | null }> };

export type ExecutionQueryVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type ExecutionQuery = { __typename?: 'Query', execution: { __typename?: 'Execution', id: string, tenantID: string, projectID: string | null, blueprintID: string | null, workspaceID: string | null, status: string, budgetUSD: number, reservedUSD: number, spentUSD: number, refundedUSD: number, revenueUSD: number, providerCostUSD: number, sandboxCostUSD: number, storageCostUSD: number, deploymentCostUSD: number, completionScore: number, grossMarginPct: number | null, expectedCompletionDelta: number | null, riskScore: number | null, stopLossUSD: number | null, promptSummary: string | null, failureReason: string | null, metadata: unknown, createdAt: string, admittedAt: string | null, startedAt: string | null, endedAt: string | null } | null };

export type CreatePaidExecutionMutationVariables = Exact<{
  input: CreatePaidExecutionInput;
}>;


export type CreatePaidExecutionMutation = { __typename?: 'Mutation', createPaidExecution: { __typename?: 'Execution', id: string, tenantID: string, projectID: string | null, blueprintID: string | null, workspaceID: string | null, status: string, budgetUSD: number, reservedUSD: number, spentUSD: number, refundedUSD: number, revenueUSD: number, providerCostUSD: number, sandboxCostUSD: number, storageCostUSD: number, deploymentCostUSD: number, completionScore: number, grossMarginPct: number | null, expectedCompletionDelta: number | null, riskScore: number | null, stopLossUSD: number | null, promptSummary: string | null, failureReason: string | null, metadata: unknown, createdAt: string, admittedAt: string | null, startedAt: string | null, endedAt: string | null } };

export type StopExecutionMutationVariables = Exact<{
  id: Scalars['ID']['input'];
  reason: Scalars['String']['input'];
}>;


export type StopExecutionMutation = { __typename?: 'Mutation', stopExecution: { __typename?: 'Execution', id: string, tenantID: string, projectID: string | null, blueprintID: string | null, workspaceID: string | null, status: string, budgetUSD: number, reservedUSD: number, spentUSD: number, refundedUSD: number, revenueUSD: number, providerCostUSD: number, sandboxCostUSD: number, storageCostUSD: number, deploymentCostUSD: number, completionScore: number, grossMarginPct: number | null, expectedCompletionDelta: number | null, riskScore: number | null, stopLossUSD: number | null, promptSummary: string | null, failureReason: string | null, metadata: unknown, createdAt: string, admittedAt: string | null, startedAt: string | null, endedAt: string | null } };

export type RefundExecutionMutationVariables = Exact<{
  id: Scalars['ID']['input'];
  amountUSD?: InputMaybe<Scalars['Float']['input']>;
  reason?: InputMaybe<Scalars['String']['input']>;
}>;


export type RefundExecutionMutation = { __typename?: 'Mutation', refundExecution: { __typename?: 'Execution', id: string, tenantID: string, projectID: string | null, blueprintID: string | null, workspaceID: string | null, status: string, budgetUSD: number, reservedUSD: number, spentUSD: number, refundedUSD: number, revenueUSD: number, providerCostUSD: number, sandboxCostUSD: number, storageCostUSD: number, deploymentCostUSD: number, completionScore: number, grossMarginPct: number | null, expectedCompletionDelta: number | null, riskScore: number | null, stopLossUSD: number | null, promptSummary: string | null, failureReason: string | null, metadata: unknown, createdAt: string, admittedAt: string | null, startedAt: string | null, endedAt: string | null } };

export type ExecutionFeedSubscriptionVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type ExecutionFeedSubscription = { __typename?: 'Subscription', executionFeed: { __typename?: 'ExecutionEvent', executionID: string, eventType: string, payload: unknown, createdAt: string } };

export type EstimateExecutionCostQueryVariables = Exact<{
  input: EstimateInput;
}>;


export type EstimateExecutionCostQuery = { __typename?: 'Query', estimateExecutionCost: { __typename?: 'CostEstimate', lowUSD: number, medianUSD: number, highUSD: number, p95USD: number, breakdown: unknown, confidence: number, basedOnRuns: number, caveat: string | null } };

export type GateVerdictCoreFragment = { __typename?: 'GateVerdict', gate: string, status: GateStatus, startedAt: string | null, finishedAt: string | null, durationMs: number | null, notes: string | null, issues: Array<{ __typename?: 'GateIssue', path: string | null, line: number | null, rule: string | null, severity: string | null, message: string }> };

export type ProjectGatesQueryVariables = Exact<{
  projectId: Scalars['ID']['input'];
  sub?: InputMaybe<Scalars['ID']['input']>;
}>;


export type ProjectGatesQuery = { __typename?: 'Query', gates: Array<{ __typename?: 'GateVerdict', gate: string, status: GateStatus, startedAt: string | null, finishedAt: string | null, durationMs: number | null, notes: string | null, issues: Array<{ __typename?: 'GateIssue', path: string | null, line: number | null, rule: string | null, severity: string | null, message: string }> }> };

export type GateQueryVariables = Exact<{
  projectId: Scalars['ID']['input'];
  gate: Scalars['String']['input'];
}>;


export type GateQuery = { __typename?: 'Query', gate: { __typename?: 'GateVerdict', gate: string, status: GateStatus, startedAt: string | null, finishedAt: string | null, durationMs: number | null, notes: string | null, issues: Array<{ __typename?: 'GateIssue', path: string | null, line: number | null, rule: string | null, severity: string | null, message: string }> } | null };

export type RerunGateMutationVariables = Exact<{
  input: RerunGateInput;
}>;


export type RerunGateMutation = { __typename?: 'Mutation', rerunGate: { __typename?: 'GateVerdict', gate: string, status: GateStatus, startedAt: string | null, finishedAt: string | null, durationMs: number | null, notes: string | null, issues: Array<{ __typename?: 'GateIssue', path: string | null, line: number | null, rule: string | null, severity: string | null, message: string }> } };

export type LedgerEntryCoreFragment = { __typename?: 'WalletLedgerEntry', id: string, tenantID: string, executionID: string | null, entryType: string, direction: string, amountUSD: number, provider: string | null, billable: boolean, marginRelevant: boolean, metadata: unknown, createdAt: string };

export type ExecutionLedgerQueryVariables = Exact<{
  executionID: Scalars['ID']['input'];
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
}>;


export type ExecutionLedgerQuery = { __typename?: 'Query', executionLedger: Array<{ __typename?: 'WalletLedgerEntry', id: string, tenantID: string, executionID: string | null, entryType: string, direction: string, amountUSD: number, provider: string | null, billable: boolean, marginRelevant: boolean, metadata: unknown, createdAt: string }> };

export type LedgerRollupQueryVariables = Exact<{
  since: Scalars['DateTime']['input'];
  until: Scalars['DateTime']['input'];
}>;


export type LedgerRollupQuery = { __typename?: 'Query', ledgerRollup: { __typename?: 'LedgerRollup', revenueUSD: number, providerCostUSD: number, sandboxCostUSD: number, storageCostUSD: number, deploymentCostUSD: number, premiumReasoningCostUSD: number, refundsUSD: number, platformMarginUSD: number, grossMarginPct: number } };

export type TenantProfitTodayQueryVariables = Exact<{ [key: string]: never; }>;


export type TenantProfitTodayQuery = { __typename?: 'Query', tenantProfitToday: { __typename?: 'LedgerRollup', revenueUSD: number, providerCostUSD: number, sandboxCostUSD: number, storageCostUSD: number, deploymentCostUSD: number, premiumReasoningCostUSD: number, refundsUSD: number, platformMarginUSD: number, grossMarginPct: number } };

export type NotificationsQueryVariables = Exact<{
  unreadOnly?: InputMaybe<Scalars['Boolean']['input']>;
}>;


export type NotificationsQuery = { __typename?: 'Query', unreadNotificationCount: number, notifications: Array<{ __typename?: 'Notification', id: string, kind: string, title: string, body: string, link: string | null, severity: string, readAt: string | null, createdAt: string }> };

export type NotificationPreferencesQueryVariables = Exact<{ [key: string]: never; }>;


export type NotificationPreferencesQuery = { __typename?: 'Query', notificationPreferences: { __typename?: 'NotificationPreferences', userId: string, pauseAll: boolean, weeklyDigest: boolean, onRunComplete: { __typename?: 'ChannelPref', email: boolean, inApp: boolean }, onGateFailed: { __typename?: 'ChannelPref', email: boolean, inApp: boolean }, onDeployDone: { __typename?: 'ChannelPref', email: boolean, inApp: boolean }, onBudgetWarning: { __typename?: 'ChannelPref', email: boolean, inApp: boolean }, onReceipt: { __typename?: 'ChannelPref', email: boolean, inApp: boolean } } };

export type MarkNotificationReadMutationVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type MarkNotificationReadMutation = { __typename?: 'Mutation', markNotificationRead: { __typename?: 'Notification', id: string, readAt: string | null } };

export type MarkAllNotificationsReadMutationVariables = Exact<{ [key: string]: never; }>;


export type MarkAllNotificationsReadMutation = { __typename?: 'Mutation', markAllNotificationsRead: number };

export type UpdateNotificationPreferencesMutationVariables = Exact<{
  input: NotificationPreferencesInput;
}>;


export type UpdateNotificationPreferencesMutation = { __typename?: 'Mutation', updateNotificationPreferences: { __typename?: 'NotificationPreferences', userId: string, pauseAll: boolean, weeklyDigest: boolean, onRunComplete: { __typename?: 'ChannelPref', email: boolean, inApp: boolean }, onGateFailed: { __typename?: 'ChannelPref', email: boolean, inApp: boolean }, onDeployDone: { __typename?: 'ChannelPref', email: boolean, inApp: boolean }, onBudgetWarning: { __typename?: 'ChannelPref', email: boolean, inApp: boolean }, onReceipt: { __typename?: 'ChannelPref', email: boolean, inApp: boolean } } };

export type NotificationStreamSubscriptionVariables = Exact<{ [key: string]: never; }>;


export type NotificationStreamSubscription = { __typename?: 'Subscription', notificationStream: { __typename?: 'Notification', id: string, kind: string, title: string, body: string, link: string | null, severity: string, readAt: string | null, createdAt: string } };

export type OperatorPendingApprovalsQueryVariables = Exact<{
  tenantID?: InputMaybe<Scalars['ID']['input']>;
}>;


export type OperatorPendingApprovalsQuery = { __typename?: 'Query', operatorPendingApprovals: Array<{ __typename?: 'DeployApproval', id: string, deployID: string, tenantID: string, status: string, diffHash: string, artifactHash: string, gateSummary: unknown, costImpactUSD: number, expiresAt: string, decisionNote: string | null, requestedAt: string, decidedAt: string | null }> };

export type OperatorAbuseScoreQueryVariables = Exact<{
  tenantID: Scalars['ID']['input'];
  userID: Scalars['ID']['input'];
}>;


export type OperatorAbuseScoreQuery = { __typename?: 'Query', operatorAbuseScore: { __typename?: 'AbuseScoreResult', tenantID: string, userID: string, score: number, tier: string } };

export type OperatorScaleSnapshotQueryVariables = Exact<{ [key: string]: never; }>;


export type OperatorScaleSnapshotQuery = { __typename?: 'Query', operatorScaleSnapshot: { __typename?: 'OperatorScaleSnapshot', activeExecutions: number, queuedExecutions: number, sandboxCapacity: number, workerUtilizationPct: number } };

export type OperatorWalletSnapshotQueryVariables = Exact<{
  tenantID: Scalars['ID']['input'];
}>;


export type OperatorWalletSnapshotQuery = { __typename?: 'Query', operatorWalletSnapshot: { __typename?: 'OperatorWalletSnapshot', tenantID: string, balanceUSD: number, holdUSD: number, lifetimeTopUpUSD: number, lifetimeSpendUSD: number } };

export type OperatorAuditCursorQueryVariables = Exact<{
  since: Scalars['DateTime']['input'];
  limit?: InputMaybe<Scalars['Int']['input']>;
}>;


export type OperatorAuditCursorQuery = { __typename?: 'Query', operatorAuditCursor: Array<{ __typename?: 'OperatorAuditEntry', id: string, timestamp: string, action: string, outcome: string, hash: string }> };

export type PatchChangeCoreFragment = { __typename?: 'PatchChange', op: PatchChangeOp, path: string, anchor: string | null, replacement: string | null, symbol: string | null, content: string | null };

export type PatchCoreFragment = { __typename?: 'Patch', id: string, projectId: string, title: string | null, summary: string | null, author: string | null, status: PatchStatus, createdAt: string, appliedAt: string | null, changes: Array<{ __typename?: 'PatchChange', op: PatchChangeOp, path: string, anchor: string | null, replacement: string | null, symbol: string | null, content: string | null }> };

export type PatchesQueryVariables = Exact<{
  projectId: Scalars['ID']['input'];
}>;


export type PatchesQuery = { __typename?: 'Query', patches: Array<{ __typename?: 'Patch', id: string, projectId: string, title: string | null, summary: string | null, author: string | null, status: PatchStatus, createdAt: string, appliedAt: string | null, changes: Array<{ __typename?: 'PatchChange', op: PatchChangeOp, path: string, anchor: string | null, replacement: string | null, symbol: string | null, content: string | null }> }> };

export type ApplyPatchMutationVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type ApplyPatchMutation = { __typename?: 'Mutation', applyPatch: { __typename?: 'Patch', id: string, status: PatchStatus, appliedAt: string | null } };

export type RollbackPatchMutationVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type RollbackPatchMutation = { __typename?: 'Mutation', rollbackPatch: unknown };

export type ProfitGuardDecisionsQueryVariables = Exact<{
  executionID?: InputMaybe<Scalars['ID']['input']>;
  limit?: InputMaybe<Scalars['Int']['input']>;
}>;


export type ProfitGuardDecisionsQuery = { __typename?: 'Query', profitGuardDecisions: Array<{ __typename?: 'ProfitGuardDecision', id: string, executionID: string, enforcementPoint: string, decision: string, reason: string, spentUSD: number, reservedUSD: number, estimatedStepCostUSD: number, expectedCompletionDelta: number, expectedMarginPct: number | null, riskScore: number | null, recommendedProvider: string | null, createdAt: string }> };

export type ProjectCoreFragment = { __typename?: 'Project', id: string, name: string, description: string | null, status: string, ownerId: string, isPublic: boolean, idea: string | null, createdAt: string, updatedAt: string };

export type ProjectsQueryVariables = Exact<{
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
}>;


export type ProjectsQuery = { __typename?: 'Query', projects: Array<{ __typename?: 'Project', id: string, name: string, description: string | null, status: string, ownerId: string, isPublic: boolean, idea: string | null, createdAt: string, updatedAt: string }> };

export type DashboardProjectsQueryVariables = Exact<{ [key: string]: never; }>;


export type DashboardProjectsQuery = { __typename?: 'Query', projects: Array<{ __typename?: 'Project', id: string, name: string, description: string | null, status: string, ownerId: string, isPublic: boolean, idea: string | null, createdAt: string, updatedAt: string, files: Array<{ __typename?: 'ProjectFile', path: string }> }> };

export type ProjectQueryVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type ProjectQuery = { __typename?: 'Query', project: { __typename?: 'Project', id: string, name: string, description: string | null, status: string, ownerId: string, isPublic: boolean, idea: string | null, createdAt: string, updatedAt: string } | null };

export type ProjectFilesQueryVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type ProjectFilesQuery = { __typename?: 'Query', projectFiles: Array<{ __typename?: 'ProjectFile', path: string, content: string | null, size: number | null, language: string | null, updatedAt: string | null }> };

export type CreateProjectMutationVariables = Exact<{
  input: CreateProjectInput;
}>;


export type CreateProjectMutation = { __typename?: 'Mutation', createProject: { __typename?: 'Project', id: string, name: string, description: string | null, status: string, ownerId: string, isPublic: boolean, idea: string | null, createdAt: string, updatedAt: string } };

export type UpdateProjectMutationVariables = Exact<{
  id: Scalars['ID']['input'];
  input: UpdateProjectInput;
}>;


export type UpdateProjectMutation = { __typename?: 'Mutation', updateProject: { __typename?: 'Project', id: string, name: string, description: string | null, status: string, ownerId: string, isPublic: boolean, idea: string | null, createdAt: string, updatedAt: string } };

export type DeleteProjectMutationVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type DeleteProjectMutation = { __typename?: 'Mutation', deleteProject: { __typename?: 'OperationResult', ok: boolean, message: string | null } };

export type BulkDeleteProjectsMutationVariables = Exact<{
  ids: Array<Scalars['ID']['input']> | Scalars['ID']['input'];
}>;


export type BulkDeleteProjectsMutation = { __typename?: 'Mutation', bulkDeleteProjects: { __typename?: 'OperationResult', ok: boolean, message: string | null } };

export type RunFinisherMutationVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type RunFinisherMutation = { __typename?: 'Mutation', runFinisher: unknown };

export type PromptPlanMutationVariables = Exact<{
  id: Scalars['ID']['input'];
  prompt: Scalars['String']['input'];
}>;


export type PromptPlanMutation = { __typename?: 'Mutation', promptPlan: unknown };

export type WriteProjectFilesMutationVariables = Exact<{
  id: Scalars['ID']['input'];
  files: Array<WriteProjectFileInput> | WriteProjectFileInput;
}>;


export type WriteProjectFilesMutation = { __typename?: 'Mutation', writeProjectFiles: Array<{ __typename?: 'ProjectFile', path: string, content: string | null, size: number | null, language: string | null, updatedAt: string | null }> };

export type ExecutionSecurityReportQueryVariables = Exact<{
  executionID: Scalars['ID']['input'];
}>;


export type ExecutionSecurityReportQuery = { __typename?: 'Query', executionSecurityReport: { __typename?: 'SecurityReport', executionID: string, tenantID: string, status: string, overallScore: number, secretsFound: number, outdatedDeps: number, owaspCoverage: unknown, blockedDeploy: boolean, generatedAt: string, findings: Array<{ __typename?: 'SecurityReportFinding', id: string, severity: string, ruleID: string, category: string, path: string, line: number, summary: string, remediation: string, detectedAt: string }> } };

export type DescribeIdeaMutationVariables = Exact<{
  input: DescribeIdeaInput;
}>;


export type DescribeIdeaMutation = { __typename?: 'Mutation', describeIdea: { __typename?: 'StudioBootstrap', project: { __typename?: 'Project', id: string, name: string, idea: string | null, description: string | null, status: string, ownerId: string, isPublic: boolean, createdAt: string, updatedAt: string }, execution: { __typename?: 'Execution', id: string, status: string, budgetUSD: number, reservedUSD: number, spentUSD: number, stopLossUSD: number | null, promptSummary: string | null, createdAt: string, admittedAt: string | null, startedAt: string | null }, idea: { __typename?: 'ParsedIdea', title: string, summary: string, blueprintID: string, blueprintReason: string, suggestedBudgetUSD: number, tags: Array<string>, stopLossUSD: number, confidence: number }, costEstimate: { __typename?: 'CostEstimate', lowUSD: number, medianUSD: number, highUSD: number, p95USD: number, confidence: number, basedOnRuns: number, caveat: string | null } } };

export type RefineIdeaMutationVariables = Exact<{
  executionID: Scalars['ID']['input'];
  message: Scalars['String']['input'];
}>;


export type RefineIdeaMutation = { __typename?: 'Mutation', refineIdea: { __typename?: 'StudioBootstrap', project: { __typename?: 'Project', id: string, name: string }, execution: { __typename?: 'Execution', id: string, status: string, spentUSD: number, reservedUSD: number }, idea: { __typename?: 'ParsedIdea', title: string, summary: string, blueprintID: string, blueprintReason: string, confidence: number }, costEstimate: { __typename?: 'CostEstimate', medianUSD: number, lowUSD: number, highUSD: number, confidence: number } } };

export type WalletQueryVariables = Exact<{ [key: string]: never; }>;


export type WalletQuery = { __typename?: 'Query', wallet: { __typename?: 'Wallet', tenantID: string, balanceUSD: number, holdUSD: number, availableUSD: number, lifetimeTopUpUSD: number, lifetimeSpendUSD: number, updatedAt: string } };

export type WalletTopUpsQueryVariables = Exact<{
  limit?: InputMaybe<Scalars['Int']['input']>;
}>;


export type WalletTopUpsQuery = { __typename?: 'Query', walletTopUps: Array<{ __typename?: 'WalletTopUp', id: string, provider: string, amountUSD: number, status: string, createdAt: string, completedAt: string | null }> };

export type WalletAvailableProvidersQueryVariables = Exact<{ [key: string]: never; }>;


export type WalletAvailableProvidersQuery = { __typename?: 'Query', walletAvailableProviders: Array<{ __typename?: 'WalletProvider', name: string, label: string, isPrimary: boolean }> };

export type WalletCreateTopUpMutationVariables = Exact<{
  amountUSD: Scalars['Float']['input'];
  provider?: InputMaybe<Scalars['String']['input']>;
}>;


export type WalletCreateTopUpMutation = { __typename?: 'Mutation', walletCreateTopUp: { __typename?: 'WalletCheckoutSession', url: string, sessionID: string, provider: string } };

export type ExecutionSupportBundleQueryVariables = Exact<{
  executionID: Scalars['ID']['input'];
}>;


export type ExecutionSupportBundleQuery = { __typename?: 'Query', executionSupportBundle: { __typename?: 'SupportBundle', executionID: string, tenantID: string, status: string, previewURL: string | null, productionURL: string | null, changedFiles: Array<string>, patchCount: number, generatedAt: string, gateReport: { __typename?: 'GateReport', completionScore: number, stages: Array<{ __typename?: 'GateStage', name: string, status: string, issuesCount: number }> }, securityReport: { __typename?: 'SupportSecurityReport', passRate: number, blockedDeploy: boolean, findings: Array<{ __typename?: 'SupportSecurityFinding', severity: string, ruleID: string, path: string, line: number, summary: string }> }, costReport: { __typename?: 'CostReport', revenueUSD: number, providerCostUSD: number, sandboxCostUSD: number, storageCostUSD: number, deploymentCostUSD: number, grossMarginPct: number }, nextBestAction: { __typename?: 'NextAction', kind: string, title: string, reason: string, cta: string | null } } };

export const LedgerEntryRowFragmentDoc = gql`
    fragment LedgerEntryRow on LedgerEntry {
  id
  projectId
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
    `;
export const DeployCoreFragmentDoc = gql`
    fragment DeployCore on Deploy {
  id
  tenantID
  projectID
  executionID
  blueprintID
  target
  environment
  status
  providerDeploymentID
  previewURL
  productionURL
  diffHash
  artifactHash
  gateSummary
  costUSD
  createdAt
  previewReadyAt
  promotedAt
  rolledBackAt
}
    `;
export const DeployApprovalCoreFragmentDoc = gql`
    fragment DeployApprovalCore on DeployApproval {
  id
  deployID
  tenantID
  status
  diffHash
  artifactHash
  gateSummary
  costImpactUSD
  expiresAt
  decisionNote
  requestedAt
  decidedAt
}
    `;
export const DeployDomainCoreFragmentDoc = gql`
    fragment DeployDomainCore on DeployDomain {
  id
  tenantID
  projectID
  deployID
  hostname
  kind
  status
  provider
  registrar
  primary
  verificationStatus
  certificateStatus
  instructions
  metadata
  dnsRecords {
    type
    name
    value
    ttl
  }
  createdAt
  updatedAt
  verifiedAt
  liveAt
}
    `;
export const ExecutionCoreFragmentDoc = gql`
    fragment ExecutionCore on Execution {
  id
  tenantID
  projectID
  blueprintID
  workspaceID
  status
  budgetUSD
  reservedUSD
  spentUSD
  refundedUSD
  revenueUSD
  providerCostUSD
  sandboxCostUSD
  storageCostUSD
  deploymentCostUSD
  completionScore
  grossMarginPct
  expectedCompletionDelta
  riskScore
  stopLossUSD
  promptSummary
  failureReason
  metadata
  createdAt
  admittedAt
  startedAt
  endedAt
}
    `;
export const GateVerdictCoreFragmentDoc = gql`
    fragment GateVerdictCore on GateVerdict {
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
export const LedgerEntryCoreFragmentDoc = gql`
    fragment LedgerEntryCore on WalletLedgerEntry {
  id
  tenantID
  executionID
  entryType
  direction
  amountUSD
  provider
  billable
  marginRelevant
  metadata
  createdAt
}
    `;
export const PatchChangeCoreFragmentDoc = gql`
    fragment PatchChangeCore on PatchChange {
  op
  path
  anchor
  replacement
  symbol
  content
}
    `;
export const PatchCoreFragmentDoc = gql`
    fragment PatchCore on Patch {
  id
  projectId
  title
  summary
  author
  status
  createdAt
  appliedAt
  changes {
    ...PatchChangeCore
  }
}
    ${PatchChangeCoreFragmentDoc}`;
export const ProjectCoreFragmentDoc = gql`
    fragment ProjectCore on Project {
  id
  name
  description
  status
  ownerId
  isPublic
  idea
  createdAt
  updatedAt
}
    `;
export const LearningDashboardDocument = gql`
    query LearningDashboard {
  learningDashboard {
    outcomeEventsToday
    outcomeEventsAllTime
    reuseRateLast7d
    repairRecipeHitsLast7d
    banditConfidence
    averageCompletionScore
    averageMarginPctLast7d
    gateFailureRates {
      gate
      failureRate
      sampleSize
    }
    blueprintSuccessRates {
      blueprintID
      blueprintName
      successRate
      avgMargin
    }
    weaknesses {
      dimension
      description
      severity
      suggestedAction
    }
    lastIndexedAt
  }
}
    `;

/**
 * __useLearningDashboardQuery__
 *
 * To run a query within a React component, call `useLearningDashboardQuery` and pass it any options that fit your needs.
 * When your component renders, `useLearningDashboardQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useLearningDashboardQuery({
 *   variables: {
 *   },
 * });
 */
export function useLearningDashboardQuery(baseOptions?: Apollo.QueryHookOptions<LearningDashboardQuery, LearningDashboardQueryVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<LearningDashboardQuery, LearningDashboardQueryVariables>(LearningDashboardDocument, options);
      }
export function useLearningDashboardLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<LearningDashboardQuery, LearningDashboardQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<LearningDashboardQuery, LearningDashboardQueryVariables>(LearningDashboardDocument, options);
        }
// @ts-ignore
export function useLearningDashboardSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<LearningDashboardQuery, LearningDashboardQueryVariables>): Apollo.UseSuspenseQueryResult<LearningDashboardQuery, LearningDashboardQueryVariables>;
export function useLearningDashboardSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<LearningDashboardQuery, LearningDashboardQueryVariables>): Apollo.UseSuspenseQueryResult<LearningDashboardQuery | undefined, LearningDashboardQueryVariables>;
export function useLearningDashboardSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<LearningDashboardQuery, LearningDashboardQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<LearningDashboardQuery, LearningDashboardQueryVariables>(LearningDashboardDocument, options);
        }
export type LearningDashboardQueryHookResult = ReturnType<typeof useLearningDashboardQuery>;
export type LearningDashboardLazyQueryHookResult = ReturnType<typeof useLearningDashboardLazyQuery>;
export type LearningDashboardSuspenseQueryHookResult = ReturnType<typeof useLearningDashboardSuspenseQuery>;
export type LearningDashboardQueryResult = Apollo.QueryResult<LearningDashboardQuery, LearningDashboardQueryVariables>;
export const BanditRankingDocument = gql`
    query BanditRanking($lookback: Int) {
  banditRanking(lookback: $lookback) {
    lookback
    strategy
    capabilities {
      capability
      total
      winners {
        provider
        model
        share
        meanReward
        isLeader
      }
    }
  }
}
    `;

/**
 * __useBanditRankingQuery__
 *
 * To run a query within a React component, call `useBanditRankingQuery` and pass it any options that fit your needs.
 * When your component renders, `useBanditRankingQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useBanditRankingQuery({
 *   variables: {
 *      lookback: // value for 'lookback'
 *   },
 * });
 */
export function useBanditRankingQuery(baseOptions?: Apollo.QueryHookOptions<BanditRankingQuery, BanditRankingQueryVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<BanditRankingQuery, BanditRankingQueryVariables>(BanditRankingDocument, options);
      }
export function useBanditRankingLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<BanditRankingQuery, BanditRankingQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<BanditRankingQuery, BanditRankingQueryVariables>(BanditRankingDocument, options);
        }
// @ts-ignore
export function useBanditRankingSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<BanditRankingQuery, BanditRankingQueryVariables>): Apollo.UseSuspenseQueryResult<BanditRankingQuery, BanditRankingQueryVariables>;
export function useBanditRankingSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<BanditRankingQuery, BanditRankingQueryVariables>): Apollo.UseSuspenseQueryResult<BanditRankingQuery | undefined, BanditRankingQueryVariables>;
export function useBanditRankingSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<BanditRankingQuery, BanditRankingQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<BanditRankingQuery, BanditRankingQueryVariables>(BanditRankingDocument, options);
        }
export type BanditRankingQueryHookResult = ReturnType<typeof useBanditRankingQuery>;
export type BanditRankingLazyQueryHookResult = ReturnType<typeof useBanditRankingLazyQuery>;
export type BanditRankingSuspenseQueryHookResult = ReturnType<typeof useBanditRankingSuspenseQuery>;
export type BanditRankingQueryResult = Apollo.QueryResult<BanditRankingQuery, BanditRankingQueryVariables>;
export const CurrentUserDocument = gql`
    query CurrentUser {
  me {
    id
    email
    name
    plan
    orgId
    telemetryOptOut
    emailVerifiedAt
    createdAt
  }
}
    `;

/**
 * __useCurrentUserQuery__
 *
 * To run a query within a React component, call `useCurrentUserQuery` and pass it any options that fit your needs.
 * When your component renders, `useCurrentUserQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useCurrentUserQuery({
 *   variables: {
 *   },
 * });
 */
export function useCurrentUserQuery(baseOptions?: Apollo.QueryHookOptions<CurrentUserQuery, CurrentUserQueryVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<CurrentUserQuery, CurrentUserQueryVariables>(CurrentUserDocument, options);
      }
export function useCurrentUserLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<CurrentUserQuery, CurrentUserQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<CurrentUserQuery, CurrentUserQueryVariables>(CurrentUserDocument, options);
        }
// @ts-ignore
export function useCurrentUserSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<CurrentUserQuery, CurrentUserQueryVariables>): Apollo.UseSuspenseQueryResult<CurrentUserQuery, CurrentUserQueryVariables>;
export function useCurrentUserSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<CurrentUserQuery, CurrentUserQueryVariables>): Apollo.UseSuspenseQueryResult<CurrentUserQuery | undefined, CurrentUserQueryVariables>;
export function useCurrentUserSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<CurrentUserQuery, CurrentUserQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<CurrentUserQuery, CurrentUserQueryVariables>(CurrentUserDocument, options);
        }
export type CurrentUserQueryHookResult = ReturnType<typeof useCurrentUserQuery>;
export type CurrentUserLazyQueryHookResult = ReturnType<typeof useCurrentUserLazyQuery>;
export type CurrentUserSuspenseQueryHookResult = ReturnType<typeof useCurrentUserSuspenseQuery>;
export type CurrentUserQueryResult = Apollo.QueryResult<CurrentUserQuery, CurrentUserQueryVariables>;
export const SignInDocument = gql`
    mutation SignIn($input: SignInInput!) {
  signIn(input: $input) {
    token
    expiresAt
    user {
      id
      email
      name
      plan
      orgId
      telemetryOptOut
      emailVerifiedAt
      createdAt
    }
  }
}
    `;
export type SignInMutationFn = Apollo.MutationFunction<SignInMutation, SignInMutationVariables>;

/**
 * __useSignInMutation__
 *
 * To run a mutation, you first call `useSignInMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useSignInMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [signInMutation, { data, loading, error }] = useSignInMutation({
 *   variables: {
 *      input: // value for 'input'
 *   },
 * });
 */
export function useSignInMutation(baseOptions?: Apollo.MutationHookOptions<SignInMutation, SignInMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<SignInMutation, SignInMutationVariables>(SignInDocument, options);
      }
export type SignInMutationHookResult = ReturnType<typeof useSignInMutation>;
export type SignInMutationResult = Apollo.MutationResult<SignInMutation>;
export type SignInMutationOptions = Apollo.BaseMutationOptions<SignInMutation, SignInMutationVariables>;
export const SignUpDocument = gql`
    mutation SignUp($input: SignUpInput!) {
  signUp(input: $input) {
    token
    expiresAt
    user {
      id
      email
      name
      plan
      orgId
      telemetryOptOut
      emailVerifiedAt
      createdAt
    }
  }
}
    `;
export type SignUpMutationFn = Apollo.MutationFunction<SignUpMutation, SignUpMutationVariables>;

/**
 * __useSignUpMutation__
 *
 * To run a mutation, you first call `useSignUpMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useSignUpMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [signUpMutation, { data, loading, error }] = useSignUpMutation({
 *   variables: {
 *      input: // value for 'input'
 *   },
 * });
 */
export function useSignUpMutation(baseOptions?: Apollo.MutationHookOptions<SignUpMutation, SignUpMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<SignUpMutation, SignUpMutationVariables>(SignUpDocument, options);
      }
export type SignUpMutationHookResult = ReturnType<typeof useSignUpMutation>;
export type SignUpMutationResult = Apollo.MutationResult<SignUpMutation>;
export type SignUpMutationOptions = Apollo.BaseMutationOptions<SignUpMutation, SignUpMutationVariables>;
export const SignOutDocument = gql`
    mutation SignOut {
  signOut {
    ok
    message
  }
}
    `;
export type SignOutMutationFn = Apollo.MutationFunction<SignOutMutation, SignOutMutationVariables>;

/**
 * __useSignOutMutation__
 *
 * To run a mutation, you first call `useSignOutMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useSignOutMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [signOutMutation, { data, loading, error }] = useSignOutMutation({
 *   variables: {
 *   },
 * });
 */
export function useSignOutMutation(baseOptions?: Apollo.MutationHookOptions<SignOutMutation, SignOutMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<SignOutMutation, SignOutMutationVariables>(SignOutDocument, options);
      }
export type SignOutMutationHookResult = ReturnType<typeof useSignOutMutation>;
export type SignOutMutationResult = Apollo.MutationResult<SignOutMutation>;
export type SignOutMutationOptions = Apollo.BaseMutationOptions<SignOutMutation, SignOutMutationVariables>;
export const RequestPasswordResetDocument = gql`
    mutation RequestPasswordReset($email: String!) {
  requestPasswordReset(email: $email) {
    ok
    message
  }
}
    `;
export type RequestPasswordResetMutationFn = Apollo.MutationFunction<RequestPasswordResetMutation, RequestPasswordResetMutationVariables>;

/**
 * __useRequestPasswordResetMutation__
 *
 * To run a mutation, you first call `useRequestPasswordResetMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useRequestPasswordResetMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [requestPasswordResetMutation, { data, loading, error }] = useRequestPasswordResetMutation({
 *   variables: {
 *      email: // value for 'email'
 *   },
 * });
 */
export function useRequestPasswordResetMutation(baseOptions?: Apollo.MutationHookOptions<RequestPasswordResetMutation, RequestPasswordResetMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<RequestPasswordResetMutation, RequestPasswordResetMutationVariables>(RequestPasswordResetDocument, options);
      }
export type RequestPasswordResetMutationHookResult = ReturnType<typeof useRequestPasswordResetMutation>;
export type RequestPasswordResetMutationResult = Apollo.MutationResult<RequestPasswordResetMutation>;
export type RequestPasswordResetMutationOptions = Apollo.BaseMutationOptions<RequestPasswordResetMutation, RequestPasswordResetMutationVariables>;
export const ResetPasswordDocument = gql`
    mutation ResetPassword($token: String!, $newPassword: String!) {
  resetPassword(token: $token, newPassword: $newPassword) {
    token
    expiresAt
    user {
      id
      email
      name
      plan
      orgId
      telemetryOptOut
      emailVerifiedAt
      createdAt
    }
  }
}
    `;
export type ResetPasswordMutationFn = Apollo.MutationFunction<ResetPasswordMutation, ResetPasswordMutationVariables>;

/**
 * __useResetPasswordMutation__
 *
 * To run a mutation, you first call `useResetPasswordMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useResetPasswordMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [resetPasswordMutation, { data, loading, error }] = useResetPasswordMutation({
 *   variables: {
 *      token: // value for 'token'
 *      newPassword: // value for 'newPassword'
 *   },
 * });
 */
export function useResetPasswordMutation(baseOptions?: Apollo.MutationHookOptions<ResetPasswordMutation, ResetPasswordMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<ResetPasswordMutation, ResetPasswordMutationVariables>(ResetPasswordDocument, options);
      }
export type ResetPasswordMutationHookResult = ReturnType<typeof useResetPasswordMutation>;
export type ResetPasswordMutationResult = Apollo.MutationResult<ResetPasswordMutation>;
export type ResetPasswordMutationOptions = Apollo.BaseMutationOptions<ResetPasswordMutation, ResetPasswordMutationVariables>;
export const BlueprintsDocument = gql`
    query Blueprints {
  blueprints {
    id
    name
    description
    category
    costPriorUSD
    expectedTimeToPreviewSec
    supportedGates
    fileCount
  }
}
    `;

/**
 * __useBlueprintsQuery__
 *
 * To run a query within a React component, call `useBlueprintsQuery` and pass it any options that fit your needs.
 * When your component renders, `useBlueprintsQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useBlueprintsQuery({
 *   variables: {
 *   },
 * });
 */
export function useBlueprintsQuery(baseOptions?: Apollo.QueryHookOptions<BlueprintsQuery, BlueprintsQueryVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<BlueprintsQuery, BlueprintsQueryVariables>(BlueprintsDocument, options);
      }
export function useBlueprintsLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<BlueprintsQuery, BlueprintsQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<BlueprintsQuery, BlueprintsQueryVariables>(BlueprintsDocument, options);
        }
// @ts-ignore
export function useBlueprintsSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<BlueprintsQuery, BlueprintsQueryVariables>): Apollo.UseSuspenseQueryResult<BlueprintsQuery, BlueprintsQueryVariables>;
export function useBlueprintsSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<BlueprintsQuery, BlueprintsQueryVariables>): Apollo.UseSuspenseQueryResult<BlueprintsQuery | undefined, BlueprintsQueryVariables>;
export function useBlueprintsSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<BlueprintsQuery, BlueprintsQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<BlueprintsQuery, BlueprintsQueryVariables>(BlueprintsDocument, options);
        }
export type BlueprintsQueryHookResult = ReturnType<typeof useBlueprintsQuery>;
export type BlueprintsLazyQueryHookResult = ReturnType<typeof useBlueprintsLazyQuery>;
export type BlueprintsSuspenseQueryHookResult = ReturnType<typeof useBlueprintsSuspenseQuery>;
export type BlueprintsQueryResult = Apollo.QueryResult<BlueprintsQuery, BlueprintsQueryVariables>;
export const BlueprintDocument = gql`
    query Blueprint($id: String!) {
  blueprint(id: $id) {
    id
    name
    description
    category
    costPriorUSD
    expectedTimeToPreviewSec
    supportedGates
    fileCount
  }
}
    `;

/**
 * __useBlueprintQuery__
 *
 * To run a query within a React component, call `useBlueprintQuery` and pass it any options that fit your needs.
 * When your component renders, `useBlueprintQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useBlueprintQuery({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function useBlueprintQuery(baseOptions: Apollo.QueryHookOptions<BlueprintQuery, BlueprintQueryVariables> & ({ variables: BlueprintQueryVariables; skip?: boolean; } | { skip: boolean; }) ) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<BlueprintQuery, BlueprintQueryVariables>(BlueprintDocument, options);
      }
export function useBlueprintLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<BlueprintQuery, BlueprintQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<BlueprintQuery, BlueprintQueryVariables>(BlueprintDocument, options);
        }
// @ts-ignore
export function useBlueprintSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<BlueprintQuery, BlueprintQueryVariables>): Apollo.UseSuspenseQueryResult<BlueprintQuery, BlueprintQueryVariables>;
export function useBlueprintSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<BlueprintQuery, BlueprintQueryVariables>): Apollo.UseSuspenseQueryResult<BlueprintQuery | undefined, BlueprintQueryVariables>;
export function useBlueprintSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<BlueprintQuery, BlueprintQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<BlueprintQuery, BlueprintQueryVariables>(BlueprintDocument, options);
        }
export type BlueprintQueryHookResult = ReturnType<typeof useBlueprintQuery>;
export type BlueprintLazyQueryHookResult = ReturnType<typeof useBlueprintLazyQuery>;
export type BlueprintSuspenseQueryHookResult = ReturnType<typeof useBlueprintSuspenseQuery>;
export type BlueprintQueryResult = Apollo.QueryResult<BlueprintQuery, BlueprintQueryVariables>;
export const BlueprintStatsDocument = gql`
    query BlueprintStats($id: String!) {
  blueprintStats(id: $id) {
    blueprintID
    executions
    previewSuccess
    refunds
    repairCount
    avgRevenueUSD
    avgCostUSD
    grossMarginPct
    avgCompletionScore
    avgTimeToPreviewSec
  }
}
    `;

/**
 * __useBlueprintStatsQuery__
 *
 * To run a query within a React component, call `useBlueprintStatsQuery` and pass it any options that fit your needs.
 * When your component renders, `useBlueprintStatsQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useBlueprintStatsQuery({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function useBlueprintStatsQuery(baseOptions: Apollo.QueryHookOptions<BlueprintStatsQuery, BlueprintStatsQueryVariables> & ({ variables: BlueprintStatsQueryVariables; skip?: boolean; } | { skip: boolean; }) ) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<BlueprintStatsQuery, BlueprintStatsQueryVariables>(BlueprintStatsDocument, options);
      }
export function useBlueprintStatsLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<BlueprintStatsQuery, BlueprintStatsQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<BlueprintStatsQuery, BlueprintStatsQueryVariables>(BlueprintStatsDocument, options);
        }
// @ts-ignore
export function useBlueprintStatsSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<BlueprintStatsQuery, BlueprintStatsQueryVariables>): Apollo.UseSuspenseQueryResult<BlueprintStatsQuery, BlueprintStatsQueryVariables>;
export function useBlueprintStatsSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<BlueprintStatsQuery, BlueprintStatsQueryVariables>): Apollo.UseSuspenseQueryResult<BlueprintStatsQuery | undefined, BlueprintStatsQueryVariables>;
export function useBlueprintStatsSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<BlueprintStatsQuery, BlueprintStatsQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<BlueprintStatsQuery, BlueprintStatsQueryVariables>(BlueprintStatsDocument, options);
        }
export type BlueprintStatsQueryHookResult = ReturnType<typeof useBlueprintStatsQuery>;
export type BlueprintStatsLazyQueryHookResult = ReturnType<typeof useBlueprintStatsLazyQuery>;
export type BlueprintStatsSuspenseQueryHookResult = ReturnType<typeof useBlueprintStatsSuspenseQuery>;
export type BlueprintStatsQueryResult = Apollo.QueryResult<BlueprintStatsQuery, BlueprintStatsQueryVariables>;
export const BlueprintRankingDocument = gql`
    query BlueprintRanking($byMetric: String, $limit: Int) {
  blueprintRanking(byMetric: $byMetric, limit: $limit) {
    blueprintID
    executions
    previewSuccess
    refunds
    repairCount
    avgRevenueUSD
    avgCostUSD
    grossMarginPct
    avgCompletionScore
    avgTimeToPreviewSec
  }
}
    `;

/**
 * __useBlueprintRankingQuery__
 *
 * To run a query within a React component, call `useBlueprintRankingQuery` and pass it any options that fit your needs.
 * When your component renders, `useBlueprintRankingQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useBlueprintRankingQuery({
 *   variables: {
 *      byMetric: // value for 'byMetric'
 *      limit: // value for 'limit'
 *   },
 * });
 */
export function useBlueprintRankingQuery(baseOptions?: Apollo.QueryHookOptions<BlueprintRankingQuery, BlueprintRankingQueryVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<BlueprintRankingQuery, BlueprintRankingQueryVariables>(BlueprintRankingDocument, options);
      }
export function useBlueprintRankingLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<BlueprintRankingQuery, BlueprintRankingQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<BlueprintRankingQuery, BlueprintRankingQueryVariables>(BlueprintRankingDocument, options);
        }
// @ts-ignore
export function useBlueprintRankingSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<BlueprintRankingQuery, BlueprintRankingQueryVariables>): Apollo.UseSuspenseQueryResult<BlueprintRankingQuery, BlueprintRankingQueryVariables>;
export function useBlueprintRankingSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<BlueprintRankingQuery, BlueprintRankingQueryVariables>): Apollo.UseSuspenseQueryResult<BlueprintRankingQuery | undefined, BlueprintRankingQueryVariables>;
export function useBlueprintRankingSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<BlueprintRankingQuery, BlueprintRankingQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<BlueprintRankingQuery, BlueprintRankingQueryVariables>(BlueprintRankingDocument, options);
        }
export type BlueprintRankingQueryHookResult = ReturnType<typeof useBlueprintRankingQuery>;
export type BlueprintRankingLazyQueryHookResult = ReturnType<typeof useBlueprintRankingLazyQuery>;
export type BlueprintRankingSuspenseQueryHookResult = ReturnType<typeof useBlueprintRankingSuspenseQuery>;
export type BlueprintRankingQueryResult = Apollo.QueryResult<BlueprintRankingQuery, BlueprintRankingQueryVariables>;
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

/**
 * __usePlansQuery__
 *
 * To run a query within a React component, call `usePlansQuery` and pass it any options that fit your needs.
 * When your component renders, `usePlansQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = usePlansQuery({
 *   variables: {
 *   },
 * });
 */
export function usePlansQuery(baseOptions?: Apollo.QueryHookOptions<PlansQuery, PlansQueryVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<PlansQuery, PlansQueryVariables>(PlansDocument, options);
      }
export function usePlansLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<PlansQuery, PlansQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<PlansQuery, PlansQueryVariables>(PlansDocument, options);
        }
// @ts-ignore
export function usePlansSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<PlansQuery, PlansQueryVariables>): Apollo.UseSuspenseQueryResult<PlansQuery, PlansQueryVariables>;
export function usePlansSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<PlansQuery, PlansQueryVariables>): Apollo.UseSuspenseQueryResult<PlansQuery | undefined, PlansQueryVariables>;
export function usePlansSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<PlansQuery, PlansQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<PlansQuery, PlansQueryVariables>(PlansDocument, options);
        }
export type PlansQueryHookResult = ReturnType<typeof usePlansQuery>;
export type PlansLazyQueryHookResult = ReturnType<typeof usePlansLazyQuery>;
export type PlansSuspenseQueryHookResult = ReturnType<typeof usePlansSuspenseQuery>;
export type PlansQueryResult = Apollo.QueryResult<PlansQuery, PlansQueryVariables>;
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

/**
 * __useRatesQuery__
 *
 * To run a query within a React component, call `useRatesQuery` and pass it any options that fit your needs.
 * When your component renders, `useRatesQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useRatesQuery({
 *   variables: {
 *   },
 * });
 */
export function useRatesQuery(baseOptions?: Apollo.QueryHookOptions<RatesQuery, RatesQueryVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<RatesQuery, RatesQueryVariables>(RatesDocument, options);
      }
export function useRatesLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<RatesQuery, RatesQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<RatesQuery, RatesQueryVariables>(RatesDocument, options);
        }
// @ts-ignore
export function useRatesSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<RatesQuery, RatesQueryVariables>): Apollo.UseSuspenseQueryResult<RatesQuery, RatesQueryVariables>;
export function useRatesSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<RatesQuery, RatesQueryVariables>): Apollo.UseSuspenseQueryResult<RatesQuery | undefined, RatesQueryVariables>;
export function useRatesSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<RatesQuery, RatesQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<RatesQuery, RatesQueryVariables>(RatesDocument, options);
        }
export type RatesQueryHookResult = ReturnType<typeof useRatesQuery>;
export type RatesLazyQueryHookResult = ReturnType<typeof useRatesLazyQuery>;
export type RatesSuspenseQueryHookResult = ReturnType<typeof useRatesSuspenseQuery>;
export type RatesQueryResult = Apollo.QueryResult<RatesQuery, RatesQueryVariables>;
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

/**
 * __useVaultQuery__
 *
 * To run a query within a React component, call `useVaultQuery` and pass it any options that fit your needs.
 * When your component renders, `useVaultQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useVaultQuery({
 *   variables: {
 *   },
 * });
 */
export function useVaultQuery(baseOptions?: Apollo.QueryHookOptions<VaultQuery, VaultQueryVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<VaultQuery, VaultQueryVariables>(VaultDocument, options);
      }
export function useVaultLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<VaultQuery, VaultQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<VaultQuery, VaultQueryVariables>(VaultDocument, options);
        }
// @ts-ignore
export function useVaultSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<VaultQuery, VaultQueryVariables>): Apollo.UseSuspenseQueryResult<VaultQuery, VaultQueryVariables>;
export function useVaultSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<VaultQuery, VaultQueryVariables>): Apollo.UseSuspenseQueryResult<VaultQuery | undefined, VaultQueryVariables>;
export function useVaultSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<VaultQuery, VaultQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<VaultQuery, VaultQueryVariables>(VaultDocument, options);
        }
export type VaultQueryHookResult = ReturnType<typeof useVaultQuery>;
export type VaultLazyQueryHookResult = ReturnType<typeof useVaultLazyQuery>;
export type VaultSuspenseQueryHookResult = ReturnType<typeof useVaultSuspenseQuery>;
export type VaultQueryResult = Apollo.QueryResult<VaultQuery, VaultQueryVariables>;
export const MyBudgetDocument = gql`
    query MyBudget {
  myBudget {
    userId
    email
    tier
    spentUsd
    entries {
      ...LedgerEntryRow
    }
  }
}
    ${LedgerEntryRowFragmentDoc}`;

/**
 * __useMyBudgetQuery__
 *
 * To run a query within a React component, call `useMyBudgetQuery` and pass it any options that fit your needs.
 * When your component renders, `useMyBudgetQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useMyBudgetQuery({
 *   variables: {
 *   },
 * });
 */
export function useMyBudgetQuery(baseOptions?: Apollo.QueryHookOptions<MyBudgetQuery, MyBudgetQueryVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<MyBudgetQuery, MyBudgetQueryVariables>(MyBudgetDocument, options);
      }
export function useMyBudgetLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<MyBudgetQuery, MyBudgetQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<MyBudgetQuery, MyBudgetQueryVariables>(MyBudgetDocument, options);
        }
// @ts-ignore
export function useMyBudgetSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<MyBudgetQuery, MyBudgetQueryVariables>): Apollo.UseSuspenseQueryResult<MyBudgetQuery, MyBudgetQueryVariables>;
export function useMyBudgetSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<MyBudgetQuery, MyBudgetQueryVariables>): Apollo.UseSuspenseQueryResult<MyBudgetQuery | undefined, MyBudgetQueryVariables>;
export function useMyBudgetSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<MyBudgetQuery, MyBudgetQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<MyBudgetQuery, MyBudgetQueryVariables>(MyBudgetDocument, options);
        }
export type MyBudgetQueryHookResult = ReturnType<typeof useMyBudgetQuery>;
export type MyBudgetLazyQueryHookResult = ReturnType<typeof useMyBudgetLazyQuery>;
export type MyBudgetSuspenseQueryHookResult = ReturnType<typeof useMyBudgetSuspenseQuery>;
export type MyBudgetQueryResult = Apollo.QueryResult<MyBudgetQuery, MyBudgetQueryVariables>;
export const StartCheckoutDocument = gql`
    mutation StartCheckout($input: StartCheckoutInput!) {
  startCheckout(input: $input) {
    sessionId
    url
  }
}
    `;
export type StartCheckoutMutationFn = Apollo.MutationFunction<StartCheckoutMutation, StartCheckoutMutationVariables>;

/**
 * __useStartCheckoutMutation__
 *
 * To run a mutation, you first call `useStartCheckoutMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useStartCheckoutMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [startCheckoutMutation, { data, loading, error }] = useStartCheckoutMutation({
 *   variables: {
 *      input: // value for 'input'
 *   },
 * });
 */
export function useStartCheckoutMutation(baseOptions?: Apollo.MutationHookOptions<StartCheckoutMutation, StartCheckoutMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<StartCheckoutMutation, StartCheckoutMutationVariables>(StartCheckoutDocument, options);
      }
export type StartCheckoutMutationHookResult = ReturnType<typeof useStartCheckoutMutation>;
export type StartCheckoutMutationResult = Apollo.MutationResult<StartCheckoutMutation>;
export type StartCheckoutMutationOptions = Apollo.BaseMutationOptions<StartCheckoutMutation, StartCheckoutMutationVariables>;
export const ProfitDashboardDocument = gql`
    query ProfitDashboard($since: DateTime!, $until: DateTime!) {
  profitDashboard(since: $since, until: $until) {
    windowStart
    windowEnd
    revenueUSD
    providerCostUSD
    sandboxCostUSD
    otherCostUSD
    grossProfitUSD
    grossMarginPct
    activeExecutions
    blockedExecutions
    refundCount
    topUpRate
  }
}
    `;

/**
 * __useProfitDashboardQuery__
 *
 * To run a query within a React component, call `useProfitDashboardQuery` and pass it any options that fit your needs.
 * When your component renders, `useProfitDashboardQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useProfitDashboardQuery({
 *   variables: {
 *      since: // value for 'since'
 *      until: // value for 'until'
 *   },
 * });
 */
export function useProfitDashboardQuery(baseOptions: Apollo.QueryHookOptions<ProfitDashboardQuery, ProfitDashboardQueryVariables> & ({ variables: ProfitDashboardQueryVariables; skip?: boolean; } | { skip: boolean; }) ) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<ProfitDashboardQuery, ProfitDashboardQueryVariables>(ProfitDashboardDocument, options);
      }
export function useProfitDashboardLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<ProfitDashboardQuery, ProfitDashboardQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<ProfitDashboardQuery, ProfitDashboardQueryVariables>(ProfitDashboardDocument, options);
        }
// @ts-ignore
export function useProfitDashboardSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<ProfitDashboardQuery, ProfitDashboardQueryVariables>): Apollo.UseSuspenseQueryResult<ProfitDashboardQuery, ProfitDashboardQueryVariables>;
export function useProfitDashboardSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ProfitDashboardQuery, ProfitDashboardQueryVariables>): Apollo.UseSuspenseQueryResult<ProfitDashboardQuery | undefined, ProfitDashboardQueryVariables>;
export function useProfitDashboardSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ProfitDashboardQuery, ProfitDashboardQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<ProfitDashboardQuery, ProfitDashboardQueryVariables>(ProfitDashboardDocument, options);
        }
export type ProfitDashboardQueryHookResult = ReturnType<typeof useProfitDashboardQuery>;
export type ProfitDashboardLazyQueryHookResult = ReturnType<typeof useProfitDashboardLazyQuery>;
export type ProfitDashboardSuspenseQueryHookResult = ReturnType<typeof useProfitDashboardSuspenseQuery>;
export type ProfitDashboardQueryResult = Apollo.QueryResult<ProfitDashboardQuery, ProfitDashboardQueryVariables>;
export const ScaleDashboardDocument = gql`
    query ScaleDashboard {
  scaleDashboard {
    activeExecutions
    queuedExecutions
    queueWaitSec
    sandboxCapacity
    workerUtilizationPct
    scaleHealth
  }
}
    `;

/**
 * __useScaleDashboardQuery__
 *
 * To run a query within a React component, call `useScaleDashboardQuery` and pass it any options that fit your needs.
 * When your component renders, `useScaleDashboardQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useScaleDashboardQuery({
 *   variables: {
 *   },
 * });
 */
export function useScaleDashboardQuery(baseOptions?: Apollo.QueryHookOptions<ScaleDashboardQuery, ScaleDashboardQueryVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<ScaleDashboardQuery, ScaleDashboardQueryVariables>(ScaleDashboardDocument, options);
      }
export function useScaleDashboardLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<ScaleDashboardQuery, ScaleDashboardQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<ScaleDashboardQuery, ScaleDashboardQueryVariables>(ScaleDashboardDocument, options);
        }
// @ts-ignore
export function useScaleDashboardSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<ScaleDashboardQuery, ScaleDashboardQueryVariables>): Apollo.UseSuspenseQueryResult<ScaleDashboardQuery, ScaleDashboardQueryVariables>;
export function useScaleDashboardSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ScaleDashboardQuery, ScaleDashboardQueryVariables>): Apollo.UseSuspenseQueryResult<ScaleDashboardQuery | undefined, ScaleDashboardQueryVariables>;
export function useScaleDashboardSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ScaleDashboardQuery, ScaleDashboardQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<ScaleDashboardQuery, ScaleDashboardQueryVariables>(ScaleDashboardDocument, options);
        }
export type ScaleDashboardQueryHookResult = ReturnType<typeof useScaleDashboardQuery>;
export type ScaleDashboardLazyQueryHookResult = ReturnType<typeof useScaleDashboardLazyQuery>;
export type ScaleDashboardSuspenseQueryHookResult = ReturnType<typeof useScaleDashboardSuspenseQuery>;
export type ScaleDashboardQueryResult = Apollo.QueryResult<ScaleDashboardQuery, ScaleDashboardQueryVariables>;
export const CohortDashboardDocument = gql`
    query CohortDashboard($sinceMonth: DateTime!) {
  cohortDashboard(sinceMonth: $sinceMonth) {
    cohorts {
      month
      newPayingUsers
      secondExecutionUsers
      day7RepeatUsers
      day30RepeatUsers
      avgSpendUSD
      grossMarginPct
      completionRate
      refundRate
      supportTicketsPerExec
    }
  }
}
    `;

/**
 * __useCohortDashboardQuery__
 *
 * To run a query within a React component, call `useCohortDashboardQuery` and pass it any options that fit your needs.
 * When your component renders, `useCohortDashboardQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useCohortDashboardQuery({
 *   variables: {
 *      sinceMonth: // value for 'sinceMonth'
 *   },
 * });
 */
export function useCohortDashboardQuery(baseOptions: Apollo.QueryHookOptions<CohortDashboardQuery, CohortDashboardQueryVariables> & ({ variables: CohortDashboardQueryVariables; skip?: boolean; } | { skip: boolean; }) ) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<CohortDashboardQuery, CohortDashboardQueryVariables>(CohortDashboardDocument, options);
      }
export function useCohortDashboardLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<CohortDashboardQuery, CohortDashboardQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<CohortDashboardQuery, CohortDashboardQueryVariables>(CohortDashboardDocument, options);
        }
// @ts-ignore
export function useCohortDashboardSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<CohortDashboardQuery, CohortDashboardQueryVariables>): Apollo.UseSuspenseQueryResult<CohortDashboardQuery, CohortDashboardQueryVariables>;
export function useCohortDashboardSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<CohortDashboardQuery, CohortDashboardQueryVariables>): Apollo.UseSuspenseQueryResult<CohortDashboardQuery | undefined, CohortDashboardQueryVariables>;
export function useCohortDashboardSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<CohortDashboardQuery, CohortDashboardQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<CohortDashboardQuery, CohortDashboardQueryVariables>(CohortDashboardDocument, options);
        }
export type CohortDashboardQueryHookResult = ReturnType<typeof useCohortDashboardQuery>;
export type CohortDashboardLazyQueryHookResult = ReturnType<typeof useCohortDashboardLazyQuery>;
export type CohortDashboardSuspenseQueryHookResult = ReturnType<typeof useCohortDashboardSuspenseQuery>;
export type CohortDashboardQueryResult = Apollo.QueryResult<CohortDashboardQuery, CohortDashboardQueryVariables>;
export const BlueprintDashboardDocument = gql`
    query BlueprintDashboard {
  blueprintDashboard {
    blueprints {
      blueprintID
      executions
      avgRevenueUSD
      avgCostUSD
      grossMarginPct
      previewSuccess
      refunds
      repairCount
      avgCompletionScore
    }
  }
}
    `;

/**
 * __useBlueprintDashboardQuery__
 *
 * To run a query within a React component, call `useBlueprintDashboardQuery` and pass it any options that fit your needs.
 * When your component renders, `useBlueprintDashboardQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useBlueprintDashboardQuery({
 *   variables: {
 *   },
 * });
 */
export function useBlueprintDashboardQuery(baseOptions?: Apollo.QueryHookOptions<BlueprintDashboardQuery, BlueprintDashboardQueryVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<BlueprintDashboardQuery, BlueprintDashboardQueryVariables>(BlueprintDashboardDocument, options);
      }
export function useBlueprintDashboardLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<BlueprintDashboardQuery, BlueprintDashboardQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<BlueprintDashboardQuery, BlueprintDashboardQueryVariables>(BlueprintDashboardDocument, options);
        }
// @ts-ignore
export function useBlueprintDashboardSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<BlueprintDashboardQuery, BlueprintDashboardQueryVariables>): Apollo.UseSuspenseQueryResult<BlueprintDashboardQuery, BlueprintDashboardQueryVariables>;
export function useBlueprintDashboardSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<BlueprintDashboardQuery, BlueprintDashboardQueryVariables>): Apollo.UseSuspenseQueryResult<BlueprintDashboardQuery | undefined, BlueprintDashboardQueryVariables>;
export function useBlueprintDashboardSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<BlueprintDashboardQuery, BlueprintDashboardQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<BlueprintDashboardQuery, BlueprintDashboardQueryVariables>(BlueprintDashboardDocument, options);
        }
export type BlueprintDashboardQueryHookResult = ReturnType<typeof useBlueprintDashboardQuery>;
export type BlueprintDashboardLazyQueryHookResult = ReturnType<typeof useBlueprintDashboardLazyQuery>;
export type BlueprintDashboardSuspenseQueryHookResult = ReturnType<typeof useBlueprintDashboardSuspenseQuery>;
export type BlueprintDashboardQueryResult = Apollo.QueryResult<BlueprintDashboardQuery, BlueprintDashboardQueryVariables>;
export const DeploysDocument = gql`
    query Deploys($limit: Int, $offset: Int) {
  deploys(limit: $limit, offset: $offset) {
    ...DeployCore
  }
}
    ${DeployCoreFragmentDoc}`;

/**
 * __useDeploysQuery__
 *
 * To run a query within a React component, call `useDeploysQuery` and pass it any options that fit your needs.
 * When your component renders, `useDeploysQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useDeploysQuery({
 *   variables: {
 *      limit: // value for 'limit'
 *      offset: // value for 'offset'
 *   },
 * });
 */
export function useDeploysQuery(baseOptions?: Apollo.QueryHookOptions<DeploysQuery, DeploysQueryVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<DeploysQuery, DeploysQueryVariables>(DeploysDocument, options);
      }
export function useDeploysLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<DeploysQuery, DeploysQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<DeploysQuery, DeploysQueryVariables>(DeploysDocument, options);
        }
// @ts-ignore
export function useDeploysSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<DeploysQuery, DeploysQueryVariables>): Apollo.UseSuspenseQueryResult<DeploysQuery, DeploysQueryVariables>;
export function useDeploysSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<DeploysQuery, DeploysQueryVariables>): Apollo.UseSuspenseQueryResult<DeploysQuery | undefined, DeploysQueryVariables>;
export function useDeploysSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<DeploysQuery, DeploysQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<DeploysQuery, DeploysQueryVariables>(DeploysDocument, options);
        }
export type DeploysQueryHookResult = ReturnType<typeof useDeploysQuery>;
export type DeploysLazyQueryHookResult = ReturnType<typeof useDeploysLazyQuery>;
export type DeploysSuspenseQueryHookResult = ReturnType<typeof useDeploysSuspenseQuery>;
export type DeploysQueryResult = Apollo.QueryResult<DeploysQuery, DeploysQueryVariables>;
export const DeployDocument = gql`
    query Deploy($id: ID!) {
  deploy(id: $id) {
    ...DeployCore
  }
}
    ${DeployCoreFragmentDoc}`;

/**
 * __useDeployQuery__
 *
 * To run a query within a React component, call `useDeployQuery` and pass it any options that fit your needs.
 * When your component renders, `useDeployQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useDeployQuery({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function useDeployQuery(baseOptions: Apollo.QueryHookOptions<DeployQuery, DeployQueryVariables> & ({ variables: DeployQueryVariables; skip?: boolean; } | { skip: boolean; }) ) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<DeployQuery, DeployQueryVariables>(DeployDocument, options);
      }
export function useDeployLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<DeployQuery, DeployQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<DeployQuery, DeployQueryVariables>(DeployDocument, options);
        }
// @ts-ignore
export function useDeploySuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<DeployQuery, DeployQueryVariables>): Apollo.UseSuspenseQueryResult<DeployQuery, DeployQueryVariables>;
export function useDeploySuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<DeployQuery, DeployQueryVariables>): Apollo.UseSuspenseQueryResult<DeployQuery | undefined, DeployQueryVariables>;
export function useDeploySuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<DeployQuery, DeployQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<DeployQuery, DeployQueryVariables>(DeployDocument, options);
        }
export type DeployQueryHookResult = ReturnType<typeof useDeployQuery>;
export type DeployLazyQueryHookResult = ReturnType<typeof useDeployLazyQuery>;
export type DeploySuspenseQueryHookResult = ReturnType<typeof useDeploySuspenseQuery>;
export type DeployQueryResult = Apollo.QueryResult<DeployQuery, DeployQueryVariables>;
export const PendingDeployApprovalsDocument = gql`
    query PendingDeployApprovals {
  pendingDeployApprovals {
    ...DeployApprovalCore
  }
}
    ${DeployApprovalCoreFragmentDoc}`;

/**
 * __usePendingDeployApprovalsQuery__
 *
 * To run a query within a React component, call `usePendingDeployApprovalsQuery` and pass it any options that fit your needs.
 * When your component renders, `usePendingDeployApprovalsQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = usePendingDeployApprovalsQuery({
 *   variables: {
 *   },
 * });
 */
export function usePendingDeployApprovalsQuery(baseOptions?: Apollo.QueryHookOptions<PendingDeployApprovalsQuery, PendingDeployApprovalsQueryVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<PendingDeployApprovalsQuery, PendingDeployApprovalsQueryVariables>(PendingDeployApprovalsDocument, options);
      }
export function usePendingDeployApprovalsLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<PendingDeployApprovalsQuery, PendingDeployApprovalsQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<PendingDeployApprovalsQuery, PendingDeployApprovalsQueryVariables>(PendingDeployApprovalsDocument, options);
        }
// @ts-ignore
export function usePendingDeployApprovalsSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<PendingDeployApprovalsQuery, PendingDeployApprovalsQueryVariables>): Apollo.UseSuspenseQueryResult<PendingDeployApprovalsQuery, PendingDeployApprovalsQueryVariables>;
export function usePendingDeployApprovalsSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<PendingDeployApprovalsQuery, PendingDeployApprovalsQueryVariables>): Apollo.UseSuspenseQueryResult<PendingDeployApprovalsQuery | undefined, PendingDeployApprovalsQueryVariables>;
export function usePendingDeployApprovalsSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<PendingDeployApprovalsQuery, PendingDeployApprovalsQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<PendingDeployApprovalsQuery, PendingDeployApprovalsQueryVariables>(PendingDeployApprovalsDocument, options);
        }
export type PendingDeployApprovalsQueryHookResult = ReturnType<typeof usePendingDeployApprovalsQuery>;
export type PendingDeployApprovalsLazyQueryHookResult = ReturnType<typeof usePendingDeployApprovalsLazyQuery>;
export type PendingDeployApprovalsSuspenseQueryHookResult = ReturnType<typeof usePendingDeployApprovalsSuspenseQuery>;
export type PendingDeployApprovalsQueryResult = Apollo.QueryResult<PendingDeployApprovalsQuery, PendingDeployApprovalsQueryVariables>;
export const DeployDomainsDocument = gql`
    query DeployDomains($projectID: ID!) {
  deployDomains(projectID: $projectID) {
    ...DeployDomainCore
  }
}
    ${DeployDomainCoreFragmentDoc}`;

/**
 * __useDeployDomainsQuery__
 *
 * To run a query within a React component, call `useDeployDomainsQuery` and pass it any options that fit your needs.
 * When your component renders, `useDeployDomainsQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useDeployDomainsQuery({
 *   variables: {
 *      projectID: // value for 'projectID'
 *   },
 * });
 */
export function useDeployDomainsQuery(baseOptions: Apollo.QueryHookOptions<DeployDomainsQuery, DeployDomainsQueryVariables> & ({ variables: DeployDomainsQueryVariables; skip?: boolean; } | { skip: boolean; }) ) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<DeployDomainsQuery, DeployDomainsQueryVariables>(DeployDomainsDocument, options);
      }
export function useDeployDomainsLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<DeployDomainsQuery, DeployDomainsQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<DeployDomainsQuery, DeployDomainsQueryVariables>(DeployDomainsDocument, options);
        }
// @ts-ignore
export function useDeployDomainsSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<DeployDomainsQuery, DeployDomainsQueryVariables>): Apollo.UseSuspenseQueryResult<DeployDomainsQuery, DeployDomainsQueryVariables>;
export function useDeployDomainsSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<DeployDomainsQuery, DeployDomainsQueryVariables>): Apollo.UseSuspenseQueryResult<DeployDomainsQuery | undefined, DeployDomainsQueryVariables>;
export function useDeployDomainsSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<DeployDomainsQuery, DeployDomainsQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<DeployDomainsQuery, DeployDomainsQueryVariables>(DeployDomainsDocument, options);
        }
export type DeployDomainsQueryHookResult = ReturnType<typeof useDeployDomainsQuery>;
export type DeployDomainsLazyQueryHookResult = ReturnType<typeof useDeployDomainsLazyQuery>;
export type DeployDomainsSuspenseQueryHookResult = ReturnType<typeof useDeployDomainsSuspenseQuery>;
export type DeployDomainsQueryResult = Apollo.QueryResult<DeployDomainsQuery, DeployDomainsQueryVariables>;
export const DomainAvailabilityDocument = gql`
    query DomainAvailability($domain: String!, $registrar: String) {
  domainAvailability(domain: $domain, registrar: $registrar) {
    domain
    available
    registrar
    priceUSD
    currency
    premium
    canPurchase
    reason
    checkedAt
    requirements
  }
}
    `;

/**
 * __useDomainAvailabilityQuery__
 *
 * To run a query within a React component, call `useDomainAvailabilityQuery` and pass it any options that fit your needs.
 * When your component renders, `useDomainAvailabilityQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useDomainAvailabilityQuery({
 *   variables: {
 *      domain: // value for 'domain'
 *      registrar: // value for 'registrar'
 *   },
 * });
 */
export function useDomainAvailabilityQuery(baseOptions: Apollo.QueryHookOptions<DomainAvailabilityQuery, DomainAvailabilityQueryVariables> & ({ variables: DomainAvailabilityQueryVariables; skip?: boolean; } | { skip: boolean; }) ) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<DomainAvailabilityQuery, DomainAvailabilityQueryVariables>(DomainAvailabilityDocument, options);
      }
export function useDomainAvailabilityLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<DomainAvailabilityQuery, DomainAvailabilityQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<DomainAvailabilityQuery, DomainAvailabilityQueryVariables>(DomainAvailabilityDocument, options);
        }
// @ts-ignore
export function useDomainAvailabilitySuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<DomainAvailabilityQuery, DomainAvailabilityQueryVariables>): Apollo.UseSuspenseQueryResult<DomainAvailabilityQuery, DomainAvailabilityQueryVariables>;
export function useDomainAvailabilitySuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<DomainAvailabilityQuery, DomainAvailabilityQueryVariables>): Apollo.UseSuspenseQueryResult<DomainAvailabilityQuery | undefined, DomainAvailabilityQueryVariables>;
export function useDomainAvailabilitySuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<DomainAvailabilityQuery, DomainAvailabilityQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<DomainAvailabilityQuery, DomainAvailabilityQueryVariables>(DomainAvailabilityDocument, options);
        }
export type DomainAvailabilityQueryHookResult = ReturnType<typeof useDomainAvailabilityQuery>;
export type DomainAvailabilityLazyQueryHookResult = ReturnType<typeof useDomainAvailabilityLazyQuery>;
export type DomainAvailabilitySuspenseQueryHookResult = ReturnType<typeof useDomainAvailabilitySuspenseQuery>;
export type DomainAvailabilityQueryResult = Apollo.QueryResult<DomainAvailabilityQuery, DomainAvailabilityQueryVariables>;
export const PlanDeployDocument = gql`
    mutation PlanDeploy($input: PlanDeployInput!) {
  planDeploy(input: $input) {
    ...DeployCore
  }
}
    ${DeployCoreFragmentDoc}`;
export type PlanDeployMutationFn = Apollo.MutationFunction<PlanDeployMutation, PlanDeployMutationVariables>;

/**
 * __usePlanDeployMutation__
 *
 * To run a mutation, you first call `usePlanDeployMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `usePlanDeployMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [planDeployMutation, { data, loading, error }] = usePlanDeployMutation({
 *   variables: {
 *      input: // value for 'input'
 *   },
 * });
 */
export function usePlanDeployMutation(baseOptions?: Apollo.MutationHookOptions<PlanDeployMutation, PlanDeployMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<PlanDeployMutation, PlanDeployMutationVariables>(PlanDeployDocument, options);
      }
export type PlanDeployMutationHookResult = ReturnType<typeof usePlanDeployMutation>;
export type PlanDeployMutationResult = Apollo.MutationResult<PlanDeployMutation>;
export type PlanDeployMutationOptions = Apollo.BaseMutationOptions<PlanDeployMutation, PlanDeployMutationVariables>;
export const CancelDeployDocument = gql`
    mutation CancelDeploy($deployID: ID!, $reason: String!) {
  cancelDeploy(deployID: $deployID, reason: $reason) {
    ...DeployCore
  }
}
    ${DeployCoreFragmentDoc}`;
export type CancelDeployMutationFn = Apollo.MutationFunction<CancelDeployMutation, CancelDeployMutationVariables>;

/**
 * __useCancelDeployMutation__
 *
 * To run a mutation, you first call `useCancelDeployMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useCancelDeployMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [cancelDeployMutation, { data, loading, error }] = useCancelDeployMutation({
 *   variables: {
 *      deployID: // value for 'deployID'
 *      reason: // value for 'reason'
 *   },
 * });
 */
export function useCancelDeployMutation(baseOptions?: Apollo.MutationHookOptions<CancelDeployMutation, CancelDeployMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<CancelDeployMutation, CancelDeployMutationVariables>(CancelDeployDocument, options);
      }
export type CancelDeployMutationHookResult = ReturnType<typeof useCancelDeployMutation>;
export type CancelDeployMutationResult = Apollo.MutationResult<CancelDeployMutation>;
export type CancelDeployMutationOptions = Apollo.BaseMutationOptions<CancelDeployMutation, CancelDeployMutationVariables>;
export const BuildDeployPreviewDocument = gql`
    mutation BuildDeployPreview($deployID: ID!) {
  buildDeployPreview(deployID: $deployID) {
    ...DeployCore
  }
}
    ${DeployCoreFragmentDoc}`;
export type BuildDeployPreviewMutationFn = Apollo.MutationFunction<BuildDeployPreviewMutation, BuildDeployPreviewMutationVariables>;

/**
 * __useBuildDeployPreviewMutation__
 *
 * To run a mutation, you first call `useBuildDeployPreviewMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useBuildDeployPreviewMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [buildDeployPreviewMutation, { data, loading, error }] = useBuildDeployPreviewMutation({
 *   variables: {
 *      deployID: // value for 'deployID'
 *   },
 * });
 */
export function useBuildDeployPreviewMutation(baseOptions?: Apollo.MutationHookOptions<BuildDeployPreviewMutation, BuildDeployPreviewMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<BuildDeployPreviewMutation, BuildDeployPreviewMutationVariables>(BuildDeployPreviewDocument, options);
      }
export type BuildDeployPreviewMutationHookResult = ReturnType<typeof useBuildDeployPreviewMutation>;
export type BuildDeployPreviewMutationResult = Apollo.MutationResult<BuildDeployPreviewMutation>;
export type BuildDeployPreviewMutationOptions = Apollo.BaseMutationOptions<BuildDeployPreviewMutation, BuildDeployPreviewMutationVariables>;
export const RequestDeployApprovalDocument = gql`
    mutation RequestDeployApproval($deployID: ID!, $expiresInMinutes: Int) {
  requestDeployApproval(deployID: $deployID, expiresInMinutes: $expiresInMinutes) {
    ...DeployApprovalCore
  }
}
    ${DeployApprovalCoreFragmentDoc}`;
export type RequestDeployApprovalMutationFn = Apollo.MutationFunction<RequestDeployApprovalMutation, RequestDeployApprovalMutationVariables>;

/**
 * __useRequestDeployApprovalMutation__
 *
 * To run a mutation, you first call `useRequestDeployApprovalMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useRequestDeployApprovalMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [requestDeployApprovalMutation, { data, loading, error }] = useRequestDeployApprovalMutation({
 *   variables: {
 *      deployID: // value for 'deployID'
 *      expiresInMinutes: // value for 'expiresInMinutes'
 *   },
 * });
 */
export function useRequestDeployApprovalMutation(baseOptions?: Apollo.MutationHookOptions<RequestDeployApprovalMutation, RequestDeployApprovalMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<RequestDeployApprovalMutation, RequestDeployApprovalMutationVariables>(RequestDeployApprovalDocument, options);
      }
export type RequestDeployApprovalMutationHookResult = ReturnType<typeof useRequestDeployApprovalMutation>;
export type RequestDeployApprovalMutationResult = Apollo.MutationResult<RequestDeployApprovalMutation>;
export type RequestDeployApprovalMutationOptions = Apollo.BaseMutationOptions<RequestDeployApprovalMutation, RequestDeployApprovalMutationVariables>;
export const DecideDeployApprovalDocument = gql`
    mutation DecideDeployApproval($approvalID: ID!, $approve: Boolean!, $note: String) {
  decideDeployApproval(approvalID: $approvalID, approve: $approve, note: $note) {
    ...DeployApprovalCore
  }
}
    ${DeployApprovalCoreFragmentDoc}`;
export type DecideDeployApprovalMutationFn = Apollo.MutationFunction<DecideDeployApprovalMutation, DecideDeployApprovalMutationVariables>;

/**
 * __useDecideDeployApprovalMutation__
 *
 * To run a mutation, you first call `useDecideDeployApprovalMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useDecideDeployApprovalMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [decideDeployApprovalMutation, { data, loading, error }] = useDecideDeployApprovalMutation({
 *   variables: {
 *      approvalID: // value for 'approvalID'
 *      approve: // value for 'approve'
 *      note: // value for 'note'
 *   },
 * });
 */
export function useDecideDeployApprovalMutation(baseOptions?: Apollo.MutationHookOptions<DecideDeployApprovalMutation, DecideDeployApprovalMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<DecideDeployApprovalMutation, DecideDeployApprovalMutationVariables>(DecideDeployApprovalDocument, options);
      }
export type DecideDeployApprovalMutationHookResult = ReturnType<typeof useDecideDeployApprovalMutation>;
export type DecideDeployApprovalMutationResult = Apollo.MutationResult<DecideDeployApprovalMutation>;
export type DecideDeployApprovalMutationOptions = Apollo.BaseMutationOptions<DecideDeployApprovalMutation, DecideDeployApprovalMutationVariables>;
export const PromoteDeployDocument = gql`
    mutation PromoteDeploy($deployID: ID!) {
  promoteDeploy(deployID: $deployID) {
    ...DeployCore
  }
}
    ${DeployCoreFragmentDoc}`;
export type PromoteDeployMutationFn = Apollo.MutationFunction<PromoteDeployMutation, PromoteDeployMutationVariables>;

/**
 * __usePromoteDeployMutation__
 *
 * To run a mutation, you first call `usePromoteDeployMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `usePromoteDeployMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [promoteDeployMutation, { data, loading, error }] = usePromoteDeployMutation({
 *   variables: {
 *      deployID: // value for 'deployID'
 *   },
 * });
 */
export function usePromoteDeployMutation(baseOptions?: Apollo.MutationHookOptions<PromoteDeployMutation, PromoteDeployMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<PromoteDeployMutation, PromoteDeployMutationVariables>(PromoteDeployDocument, options);
      }
export type PromoteDeployMutationHookResult = ReturnType<typeof usePromoteDeployMutation>;
export type PromoteDeployMutationResult = Apollo.MutationResult<PromoteDeployMutation>;
export type PromoteDeployMutationOptions = Apollo.BaseMutationOptions<PromoteDeployMutation, PromoteDeployMutationVariables>;
export const RollbackDeployDocument = gql`
    mutation RollbackDeploy($deployID: ID!, $reason: String!) {
  rollbackDeploy(deployID: $deployID, reason: $reason) {
    ...DeployCore
  }
}
    ${DeployCoreFragmentDoc}`;
export type RollbackDeployMutationFn = Apollo.MutationFunction<RollbackDeployMutation, RollbackDeployMutationVariables>;

/**
 * __useRollbackDeployMutation__
 *
 * To run a mutation, you first call `useRollbackDeployMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useRollbackDeployMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [rollbackDeployMutation, { data, loading, error }] = useRollbackDeployMutation({
 *   variables: {
 *      deployID: // value for 'deployID'
 *      reason: // value for 'reason'
 *   },
 * });
 */
export function useRollbackDeployMutation(baseOptions?: Apollo.MutationHookOptions<RollbackDeployMutation, RollbackDeployMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<RollbackDeployMutation, RollbackDeployMutationVariables>(RollbackDeployDocument, options);
      }
export type RollbackDeployMutationHookResult = ReturnType<typeof useRollbackDeployMutation>;
export type RollbackDeployMutationResult = Apollo.MutationResult<RollbackDeployMutation>;
export type RollbackDeployMutationOptions = Apollo.BaseMutationOptions<RollbackDeployMutation, RollbackDeployMutationVariables>;
export const ReserveDeploySubdomainDocument = gql`
    mutation ReserveDeploySubdomain($input: ReserveDeploySubdomainInput!) {
  reserveDeploySubdomain(input: $input) {
    ...DeployDomainCore
  }
}
    ${DeployDomainCoreFragmentDoc}`;
export type ReserveDeploySubdomainMutationFn = Apollo.MutationFunction<ReserveDeploySubdomainMutation, ReserveDeploySubdomainMutationVariables>;

/**
 * __useReserveDeploySubdomainMutation__
 *
 * To run a mutation, you first call `useReserveDeploySubdomainMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useReserveDeploySubdomainMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [reserveDeploySubdomainMutation, { data, loading, error }] = useReserveDeploySubdomainMutation({
 *   variables: {
 *      input: // value for 'input'
 *   },
 * });
 */
export function useReserveDeploySubdomainMutation(baseOptions?: Apollo.MutationHookOptions<ReserveDeploySubdomainMutation, ReserveDeploySubdomainMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<ReserveDeploySubdomainMutation, ReserveDeploySubdomainMutationVariables>(ReserveDeploySubdomainDocument, options);
      }
export type ReserveDeploySubdomainMutationHookResult = ReturnType<typeof useReserveDeploySubdomainMutation>;
export type ReserveDeploySubdomainMutationResult = Apollo.MutationResult<ReserveDeploySubdomainMutation>;
export type ReserveDeploySubdomainMutationOptions = Apollo.BaseMutationOptions<ReserveDeploySubdomainMutation, ReserveDeploySubdomainMutationVariables>;
export const ConnectDeployDomainDocument = gql`
    mutation ConnectDeployDomain($input: ConnectDeployDomainInput!) {
  connectDeployDomain(input: $input) {
    ...DeployDomainCore
  }
}
    ${DeployDomainCoreFragmentDoc}`;
export type ConnectDeployDomainMutationFn = Apollo.MutationFunction<ConnectDeployDomainMutation, ConnectDeployDomainMutationVariables>;

/**
 * __useConnectDeployDomainMutation__
 *
 * To run a mutation, you first call `useConnectDeployDomainMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useConnectDeployDomainMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [connectDeployDomainMutation, { data, loading, error }] = useConnectDeployDomainMutation({
 *   variables: {
 *      input: // value for 'input'
 *   },
 * });
 */
export function useConnectDeployDomainMutation(baseOptions?: Apollo.MutationHookOptions<ConnectDeployDomainMutation, ConnectDeployDomainMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<ConnectDeployDomainMutation, ConnectDeployDomainMutationVariables>(ConnectDeployDomainDocument, options);
      }
export type ConnectDeployDomainMutationHookResult = ReturnType<typeof useConnectDeployDomainMutation>;
export type ConnectDeployDomainMutationResult = Apollo.MutationResult<ConnectDeployDomainMutation>;
export type ConnectDeployDomainMutationOptions = Apollo.BaseMutationOptions<ConnectDeployDomainMutation, ConnectDeployDomainMutationVariables>;
export const CheckDeployDomainDocument = gql`
    mutation CheckDeployDomain($id: ID!) {
  checkDeployDomain(id: $id) {
    ...DeployDomainCore
  }
}
    ${DeployDomainCoreFragmentDoc}`;
export type CheckDeployDomainMutationFn = Apollo.MutationFunction<CheckDeployDomainMutation, CheckDeployDomainMutationVariables>;

/**
 * __useCheckDeployDomainMutation__
 *
 * To run a mutation, you first call `useCheckDeployDomainMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useCheckDeployDomainMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [checkDeployDomainMutation, { data, loading, error }] = useCheckDeployDomainMutation({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function useCheckDeployDomainMutation(baseOptions?: Apollo.MutationHookOptions<CheckDeployDomainMutation, CheckDeployDomainMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<CheckDeployDomainMutation, CheckDeployDomainMutationVariables>(CheckDeployDomainDocument, options);
      }
export type CheckDeployDomainMutationHookResult = ReturnType<typeof useCheckDeployDomainMutation>;
export type CheckDeployDomainMutationResult = Apollo.MutationResult<CheckDeployDomainMutation>;
export type CheckDeployDomainMutationOptions = Apollo.BaseMutationOptions<CheckDeployDomainMutation, CheckDeployDomainMutationVariables>;
export const SetPrimaryDeployDomainDocument = gql`
    mutation SetPrimaryDeployDomain($id: ID!) {
  setPrimaryDeployDomain(id: $id) {
    ...DeployDomainCore
  }
}
    ${DeployDomainCoreFragmentDoc}`;
export type SetPrimaryDeployDomainMutationFn = Apollo.MutationFunction<SetPrimaryDeployDomainMutation, SetPrimaryDeployDomainMutationVariables>;

/**
 * __useSetPrimaryDeployDomainMutation__
 *
 * To run a mutation, you first call `useSetPrimaryDeployDomainMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useSetPrimaryDeployDomainMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [setPrimaryDeployDomainMutation, { data, loading, error }] = useSetPrimaryDeployDomainMutation({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function useSetPrimaryDeployDomainMutation(baseOptions?: Apollo.MutationHookOptions<SetPrimaryDeployDomainMutation, SetPrimaryDeployDomainMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<SetPrimaryDeployDomainMutation, SetPrimaryDeployDomainMutationVariables>(SetPrimaryDeployDomainDocument, options);
      }
export type SetPrimaryDeployDomainMutationHookResult = ReturnType<typeof useSetPrimaryDeployDomainMutation>;
export type SetPrimaryDeployDomainMutationResult = Apollo.MutationResult<SetPrimaryDeployDomainMutation>;
export type SetPrimaryDeployDomainMutationOptions = Apollo.BaseMutationOptions<SetPrimaryDeployDomainMutation, SetPrimaryDeployDomainMutationVariables>;
export const PurchaseDeployDomainDocument = gql`
    mutation PurchaseDeployDomain($input: PurchaseDeployDomainInput!) {
  purchaseDeployDomain(input: $input) {
    ...DeployDomainCore
  }
}
    ${DeployDomainCoreFragmentDoc}`;
export type PurchaseDeployDomainMutationFn = Apollo.MutationFunction<PurchaseDeployDomainMutation, PurchaseDeployDomainMutationVariables>;

/**
 * __usePurchaseDeployDomainMutation__
 *
 * To run a mutation, you first call `usePurchaseDeployDomainMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `usePurchaseDeployDomainMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [purchaseDeployDomainMutation, { data, loading, error }] = usePurchaseDeployDomainMutation({
 *   variables: {
 *      input: // value for 'input'
 *   },
 * });
 */
export function usePurchaseDeployDomainMutation(baseOptions?: Apollo.MutationHookOptions<PurchaseDeployDomainMutation, PurchaseDeployDomainMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<PurchaseDeployDomainMutation, PurchaseDeployDomainMutationVariables>(PurchaseDeployDomainDocument, options);
      }
export type PurchaseDeployDomainMutationHookResult = ReturnType<typeof usePurchaseDeployDomainMutation>;
export type PurchaseDeployDomainMutationResult = Apollo.MutationResult<PurchaseDeployDomainMutation>;
export type PurchaseDeployDomainMutationOptions = Apollo.BaseMutationOptions<PurchaseDeployDomainMutation, PurchaseDeployDomainMutationVariables>;
export const DeployFeedDocument = gql`
    subscription DeployFeed($id: ID!) {
  deployFeed(id: $id) {
    deployID
    eventType
    payload
    createdAt
  }
}
    `;

/**
 * __useDeployFeedSubscription__
 *
 * To run a query within a React component, call `useDeployFeedSubscription` and pass it any options that fit your needs.
 * When your component renders, `useDeployFeedSubscription` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the subscription, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useDeployFeedSubscription({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function useDeployFeedSubscription(baseOptions: Apollo.SubscriptionHookOptions<DeployFeedSubscription, DeployFeedSubscriptionVariables> & ({ variables: DeployFeedSubscriptionVariables; skip?: boolean; } | { skip: boolean; }) ) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useSubscription<DeployFeedSubscription, DeployFeedSubscriptionVariables>(DeployFeedDocument, options);
      }
export type DeployFeedSubscriptionHookResult = ReturnType<typeof useDeployFeedSubscription>;
export type DeployFeedSubscriptionResult = Apollo.SubscriptionResult<DeployFeedSubscription>;
export const ExecutionsDocument = gql`
    query Executions($limit: Int, $offset: Int) {
  executions(limit: $limit, offset: $offset) {
    ...ExecutionCore
  }
}
    ${ExecutionCoreFragmentDoc}`;

/**
 * __useExecutionsQuery__
 *
 * To run a query within a React component, call `useExecutionsQuery` and pass it any options that fit your needs.
 * When your component renders, `useExecutionsQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useExecutionsQuery({
 *   variables: {
 *      limit: // value for 'limit'
 *      offset: // value for 'offset'
 *   },
 * });
 */
export function useExecutionsQuery(baseOptions?: Apollo.QueryHookOptions<ExecutionsQuery, ExecutionsQueryVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<ExecutionsQuery, ExecutionsQueryVariables>(ExecutionsDocument, options);
      }
export function useExecutionsLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<ExecutionsQuery, ExecutionsQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<ExecutionsQuery, ExecutionsQueryVariables>(ExecutionsDocument, options);
        }
// @ts-ignore
export function useExecutionsSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<ExecutionsQuery, ExecutionsQueryVariables>): Apollo.UseSuspenseQueryResult<ExecutionsQuery, ExecutionsQueryVariables>;
export function useExecutionsSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ExecutionsQuery, ExecutionsQueryVariables>): Apollo.UseSuspenseQueryResult<ExecutionsQuery | undefined, ExecutionsQueryVariables>;
export function useExecutionsSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ExecutionsQuery, ExecutionsQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<ExecutionsQuery, ExecutionsQueryVariables>(ExecutionsDocument, options);
        }
export type ExecutionsQueryHookResult = ReturnType<typeof useExecutionsQuery>;
export type ExecutionsLazyQueryHookResult = ReturnType<typeof useExecutionsLazyQuery>;
export type ExecutionsSuspenseQueryHookResult = ReturnType<typeof useExecutionsSuspenseQuery>;
export type ExecutionsQueryResult = Apollo.QueryResult<ExecutionsQuery, ExecutionsQueryVariables>;
export const ProjectExecutionsDocument = gql`
    query ProjectExecutions($projectId: ID!, $limit: Int, $offset: Int) {
  projectExecutions(projectId: $projectId, limit: $limit, offset: $offset) {
    ...ExecutionCore
  }
}
    ${ExecutionCoreFragmentDoc}`;

/**
 * __useProjectExecutionsQuery__
 *
 * To run a query within a React component, call `useProjectExecutionsQuery` and pass it any options that fit your needs.
 * When your component renders, `useProjectExecutionsQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useProjectExecutionsQuery({
 *   variables: {
 *      projectId: // value for 'projectId'
 *      limit: // value for 'limit'
 *      offset: // value for 'offset'
 *   },
 * });
 */
export function useProjectExecutionsQuery(baseOptions: Apollo.QueryHookOptions<ProjectExecutionsQuery, ProjectExecutionsQueryVariables> & ({ variables: ProjectExecutionsQueryVariables; skip?: boolean; } | { skip: boolean; }) ) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<ProjectExecutionsQuery, ProjectExecutionsQueryVariables>(ProjectExecutionsDocument, options);
      }
export function useProjectExecutionsLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<ProjectExecutionsQuery, ProjectExecutionsQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<ProjectExecutionsQuery, ProjectExecutionsQueryVariables>(ProjectExecutionsDocument, options);
        }
// @ts-ignore
export function useProjectExecutionsSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<ProjectExecutionsQuery, ProjectExecutionsQueryVariables>): Apollo.UseSuspenseQueryResult<ProjectExecutionsQuery, ProjectExecutionsQueryVariables>;
export function useProjectExecutionsSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ProjectExecutionsQuery, ProjectExecutionsQueryVariables>): Apollo.UseSuspenseQueryResult<ProjectExecutionsQuery | undefined, ProjectExecutionsQueryVariables>;
export function useProjectExecutionsSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ProjectExecutionsQuery, ProjectExecutionsQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<ProjectExecutionsQuery, ProjectExecutionsQueryVariables>(ProjectExecutionsDocument, options);
        }
export type ProjectExecutionsQueryHookResult = ReturnType<typeof useProjectExecutionsQuery>;
export type ProjectExecutionsLazyQueryHookResult = ReturnType<typeof useProjectExecutionsLazyQuery>;
export type ProjectExecutionsSuspenseQueryHookResult = ReturnType<typeof useProjectExecutionsSuspenseQuery>;
export type ProjectExecutionsQueryResult = Apollo.QueryResult<ProjectExecutionsQuery, ProjectExecutionsQueryVariables>;
export const ExecutionDocument = gql`
    query Execution($id: ID!) {
  execution(id: $id) {
    ...ExecutionCore
  }
}
    ${ExecutionCoreFragmentDoc}`;

/**
 * __useExecutionQuery__
 *
 * To run a query within a React component, call `useExecutionQuery` and pass it any options that fit your needs.
 * When your component renders, `useExecutionQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useExecutionQuery({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function useExecutionQuery(baseOptions: Apollo.QueryHookOptions<ExecutionQuery, ExecutionQueryVariables> & ({ variables: ExecutionQueryVariables; skip?: boolean; } | { skip: boolean; }) ) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<ExecutionQuery, ExecutionQueryVariables>(ExecutionDocument, options);
      }
export function useExecutionLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<ExecutionQuery, ExecutionQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<ExecutionQuery, ExecutionQueryVariables>(ExecutionDocument, options);
        }
// @ts-ignore
export function useExecutionSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<ExecutionQuery, ExecutionQueryVariables>): Apollo.UseSuspenseQueryResult<ExecutionQuery, ExecutionQueryVariables>;
export function useExecutionSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ExecutionQuery, ExecutionQueryVariables>): Apollo.UseSuspenseQueryResult<ExecutionQuery | undefined, ExecutionQueryVariables>;
export function useExecutionSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ExecutionQuery, ExecutionQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<ExecutionQuery, ExecutionQueryVariables>(ExecutionDocument, options);
        }
export type ExecutionQueryHookResult = ReturnType<typeof useExecutionQuery>;
export type ExecutionLazyQueryHookResult = ReturnType<typeof useExecutionLazyQuery>;
export type ExecutionSuspenseQueryHookResult = ReturnType<typeof useExecutionSuspenseQuery>;
export type ExecutionQueryResult = Apollo.QueryResult<ExecutionQuery, ExecutionQueryVariables>;
export const CreatePaidExecutionDocument = gql`
    mutation CreatePaidExecution($input: CreatePaidExecutionInput!) {
  createPaidExecution(input: $input) {
    ...ExecutionCore
  }
}
    ${ExecutionCoreFragmentDoc}`;
export type CreatePaidExecutionMutationFn = Apollo.MutationFunction<CreatePaidExecutionMutation, CreatePaidExecutionMutationVariables>;

/**
 * __useCreatePaidExecutionMutation__
 *
 * To run a mutation, you first call `useCreatePaidExecutionMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useCreatePaidExecutionMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [createPaidExecutionMutation, { data, loading, error }] = useCreatePaidExecutionMutation({
 *   variables: {
 *      input: // value for 'input'
 *   },
 * });
 */
export function useCreatePaidExecutionMutation(baseOptions?: Apollo.MutationHookOptions<CreatePaidExecutionMutation, CreatePaidExecutionMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<CreatePaidExecutionMutation, CreatePaidExecutionMutationVariables>(CreatePaidExecutionDocument, options);
      }
export type CreatePaidExecutionMutationHookResult = ReturnType<typeof useCreatePaidExecutionMutation>;
export type CreatePaidExecutionMutationResult = Apollo.MutationResult<CreatePaidExecutionMutation>;
export type CreatePaidExecutionMutationOptions = Apollo.BaseMutationOptions<CreatePaidExecutionMutation, CreatePaidExecutionMutationVariables>;
export const StopExecutionDocument = gql`
    mutation StopExecution($id: ID!, $reason: String!) {
  stopExecution(id: $id, reason: $reason) {
    ...ExecutionCore
  }
}
    ${ExecutionCoreFragmentDoc}`;
export type StopExecutionMutationFn = Apollo.MutationFunction<StopExecutionMutation, StopExecutionMutationVariables>;

/**
 * __useStopExecutionMutation__
 *
 * To run a mutation, you first call `useStopExecutionMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useStopExecutionMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [stopExecutionMutation, { data, loading, error }] = useStopExecutionMutation({
 *   variables: {
 *      id: // value for 'id'
 *      reason: // value for 'reason'
 *   },
 * });
 */
export function useStopExecutionMutation(baseOptions?: Apollo.MutationHookOptions<StopExecutionMutation, StopExecutionMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<StopExecutionMutation, StopExecutionMutationVariables>(StopExecutionDocument, options);
      }
export type StopExecutionMutationHookResult = ReturnType<typeof useStopExecutionMutation>;
export type StopExecutionMutationResult = Apollo.MutationResult<StopExecutionMutation>;
export type StopExecutionMutationOptions = Apollo.BaseMutationOptions<StopExecutionMutation, StopExecutionMutationVariables>;
export const RefundExecutionDocument = gql`
    mutation RefundExecution($id: ID!, $amountUSD: Float, $reason: String) {
  refundExecution(id: $id, amountUSD: $amountUSD, reason: $reason) {
    ...ExecutionCore
  }
}
    ${ExecutionCoreFragmentDoc}`;
export type RefundExecutionMutationFn = Apollo.MutationFunction<RefundExecutionMutation, RefundExecutionMutationVariables>;

/**
 * __useRefundExecutionMutation__
 *
 * To run a mutation, you first call `useRefundExecutionMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useRefundExecutionMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [refundExecutionMutation, { data, loading, error }] = useRefundExecutionMutation({
 *   variables: {
 *      id: // value for 'id'
 *      amountUSD: // value for 'amountUSD'
 *      reason: // value for 'reason'
 *   },
 * });
 */
export function useRefundExecutionMutation(baseOptions?: Apollo.MutationHookOptions<RefundExecutionMutation, RefundExecutionMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<RefundExecutionMutation, RefundExecutionMutationVariables>(RefundExecutionDocument, options);
      }
export type RefundExecutionMutationHookResult = ReturnType<typeof useRefundExecutionMutation>;
export type RefundExecutionMutationResult = Apollo.MutationResult<RefundExecutionMutation>;
export type RefundExecutionMutationOptions = Apollo.BaseMutationOptions<RefundExecutionMutation, RefundExecutionMutationVariables>;
export const ExecutionFeedDocument = gql`
    subscription ExecutionFeed($id: ID!) {
  executionFeed(id: $id) {
    executionID
    eventType
    payload
    createdAt
  }
}
    `;

/**
 * __useExecutionFeedSubscription__
 *
 * To run a query within a React component, call `useExecutionFeedSubscription` and pass it any options that fit your needs.
 * When your component renders, `useExecutionFeedSubscription` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the subscription, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useExecutionFeedSubscription({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function useExecutionFeedSubscription(baseOptions: Apollo.SubscriptionHookOptions<ExecutionFeedSubscription, ExecutionFeedSubscriptionVariables> & ({ variables: ExecutionFeedSubscriptionVariables; skip?: boolean; } | { skip: boolean; }) ) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useSubscription<ExecutionFeedSubscription, ExecutionFeedSubscriptionVariables>(ExecutionFeedDocument, options);
      }
export type ExecutionFeedSubscriptionHookResult = ReturnType<typeof useExecutionFeedSubscription>;
export type ExecutionFeedSubscriptionResult = Apollo.SubscriptionResult<ExecutionFeedSubscription>;
export const EstimateExecutionCostDocument = gql`
    query EstimateExecutionCost($input: EstimateInput!) {
  estimateExecutionCost(input: $input) {
    lowUSD
    medianUSD
    highUSD
    p95USD
    breakdown
    confidence
    basedOnRuns
    caveat
  }
}
    `;

/**
 * __useEstimateExecutionCostQuery__
 *
 * To run a query within a React component, call `useEstimateExecutionCostQuery` and pass it any options that fit your needs.
 * When your component renders, `useEstimateExecutionCostQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useEstimateExecutionCostQuery({
 *   variables: {
 *      input: // value for 'input'
 *   },
 * });
 */
export function useEstimateExecutionCostQuery(baseOptions: Apollo.QueryHookOptions<EstimateExecutionCostQuery, EstimateExecutionCostQueryVariables> & ({ variables: EstimateExecutionCostQueryVariables; skip?: boolean; } | { skip: boolean; }) ) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<EstimateExecutionCostQuery, EstimateExecutionCostQueryVariables>(EstimateExecutionCostDocument, options);
      }
export function useEstimateExecutionCostLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<EstimateExecutionCostQuery, EstimateExecutionCostQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<EstimateExecutionCostQuery, EstimateExecutionCostQueryVariables>(EstimateExecutionCostDocument, options);
        }
// @ts-ignore
export function useEstimateExecutionCostSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<EstimateExecutionCostQuery, EstimateExecutionCostQueryVariables>): Apollo.UseSuspenseQueryResult<EstimateExecutionCostQuery, EstimateExecutionCostQueryVariables>;
export function useEstimateExecutionCostSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<EstimateExecutionCostQuery, EstimateExecutionCostQueryVariables>): Apollo.UseSuspenseQueryResult<EstimateExecutionCostQuery | undefined, EstimateExecutionCostQueryVariables>;
export function useEstimateExecutionCostSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<EstimateExecutionCostQuery, EstimateExecutionCostQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<EstimateExecutionCostQuery, EstimateExecutionCostQueryVariables>(EstimateExecutionCostDocument, options);
        }
export type EstimateExecutionCostQueryHookResult = ReturnType<typeof useEstimateExecutionCostQuery>;
export type EstimateExecutionCostLazyQueryHookResult = ReturnType<typeof useEstimateExecutionCostLazyQuery>;
export type EstimateExecutionCostSuspenseQueryHookResult = ReturnType<typeof useEstimateExecutionCostSuspenseQuery>;
export type EstimateExecutionCostQueryResult = Apollo.QueryResult<EstimateExecutionCostQuery, EstimateExecutionCostQueryVariables>;
export const ProjectGatesDocument = gql`
    query ProjectGates($projectId: ID!, $sub: ID) {
  gates(projectId: $projectId, sub: $sub) {
    ...GateVerdictCore
  }
}
    ${GateVerdictCoreFragmentDoc}`;

/**
 * __useProjectGatesQuery__
 *
 * To run a query within a React component, call `useProjectGatesQuery` and pass it any options that fit your needs.
 * When your component renders, `useProjectGatesQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useProjectGatesQuery({
 *   variables: {
 *      projectId: // value for 'projectId'
 *      sub: // value for 'sub'
 *   },
 * });
 */
export function useProjectGatesQuery(baseOptions: Apollo.QueryHookOptions<ProjectGatesQuery, ProjectGatesQueryVariables> & ({ variables: ProjectGatesQueryVariables; skip?: boolean; } | { skip: boolean; }) ) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<ProjectGatesQuery, ProjectGatesQueryVariables>(ProjectGatesDocument, options);
      }
export function useProjectGatesLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<ProjectGatesQuery, ProjectGatesQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<ProjectGatesQuery, ProjectGatesQueryVariables>(ProjectGatesDocument, options);
        }
// @ts-ignore
export function useProjectGatesSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<ProjectGatesQuery, ProjectGatesQueryVariables>): Apollo.UseSuspenseQueryResult<ProjectGatesQuery, ProjectGatesQueryVariables>;
export function useProjectGatesSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ProjectGatesQuery, ProjectGatesQueryVariables>): Apollo.UseSuspenseQueryResult<ProjectGatesQuery | undefined, ProjectGatesQueryVariables>;
export function useProjectGatesSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ProjectGatesQuery, ProjectGatesQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<ProjectGatesQuery, ProjectGatesQueryVariables>(ProjectGatesDocument, options);
        }
export type ProjectGatesQueryHookResult = ReturnType<typeof useProjectGatesQuery>;
export type ProjectGatesLazyQueryHookResult = ReturnType<typeof useProjectGatesLazyQuery>;
export type ProjectGatesSuspenseQueryHookResult = ReturnType<typeof useProjectGatesSuspenseQuery>;
export type ProjectGatesQueryResult = Apollo.QueryResult<ProjectGatesQuery, ProjectGatesQueryVariables>;
export const GateDocument = gql`
    query Gate($projectId: ID!, $gate: String!) {
  gate(projectId: $projectId, gate: $gate) {
    ...GateVerdictCore
  }
}
    ${GateVerdictCoreFragmentDoc}`;

/**
 * __useGateQuery__
 *
 * To run a query within a React component, call `useGateQuery` and pass it any options that fit your needs.
 * When your component renders, `useGateQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useGateQuery({
 *   variables: {
 *      projectId: // value for 'projectId'
 *      gate: // value for 'gate'
 *   },
 * });
 */
export function useGateQuery(baseOptions: Apollo.QueryHookOptions<GateQuery, GateQueryVariables> & ({ variables: GateQueryVariables; skip?: boolean; } | { skip: boolean; }) ) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<GateQuery, GateQueryVariables>(GateDocument, options);
      }
export function useGateLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<GateQuery, GateQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<GateQuery, GateQueryVariables>(GateDocument, options);
        }
// @ts-ignore
export function useGateSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<GateQuery, GateQueryVariables>): Apollo.UseSuspenseQueryResult<GateQuery, GateQueryVariables>;
export function useGateSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<GateQuery, GateQueryVariables>): Apollo.UseSuspenseQueryResult<GateQuery | undefined, GateQueryVariables>;
export function useGateSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<GateQuery, GateQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<GateQuery, GateQueryVariables>(GateDocument, options);
        }
export type GateQueryHookResult = ReturnType<typeof useGateQuery>;
export type GateLazyQueryHookResult = ReturnType<typeof useGateLazyQuery>;
export type GateSuspenseQueryHookResult = ReturnType<typeof useGateSuspenseQuery>;
export type GateQueryResult = Apollo.QueryResult<GateQuery, GateQueryVariables>;
export const RerunGateDocument = gql`
    mutation RerunGate($input: RerunGateInput!) {
  rerunGate(input: $input) {
    ...GateVerdictCore
  }
}
    ${GateVerdictCoreFragmentDoc}`;
export type RerunGateMutationFn = Apollo.MutationFunction<RerunGateMutation, RerunGateMutationVariables>;

/**
 * __useRerunGateMutation__
 *
 * To run a mutation, you first call `useRerunGateMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useRerunGateMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [rerunGateMutation, { data, loading, error }] = useRerunGateMutation({
 *   variables: {
 *      input: // value for 'input'
 *   },
 * });
 */
export function useRerunGateMutation(baseOptions?: Apollo.MutationHookOptions<RerunGateMutation, RerunGateMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<RerunGateMutation, RerunGateMutationVariables>(RerunGateDocument, options);
      }
export type RerunGateMutationHookResult = ReturnType<typeof useRerunGateMutation>;
export type RerunGateMutationResult = Apollo.MutationResult<RerunGateMutation>;
export type RerunGateMutationOptions = Apollo.BaseMutationOptions<RerunGateMutation, RerunGateMutationVariables>;
export const ExecutionLedgerDocument = gql`
    query ExecutionLedger($executionID: ID!, $limit: Int, $offset: Int) {
  executionLedger(executionID: $executionID, limit: $limit, offset: $offset) {
    ...LedgerEntryCore
  }
}
    ${LedgerEntryCoreFragmentDoc}`;

/**
 * __useExecutionLedgerQuery__
 *
 * To run a query within a React component, call `useExecutionLedgerQuery` and pass it any options that fit your needs.
 * When your component renders, `useExecutionLedgerQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useExecutionLedgerQuery({
 *   variables: {
 *      executionID: // value for 'executionID'
 *      limit: // value for 'limit'
 *      offset: // value for 'offset'
 *   },
 * });
 */
export function useExecutionLedgerQuery(baseOptions: Apollo.QueryHookOptions<ExecutionLedgerQuery, ExecutionLedgerQueryVariables> & ({ variables: ExecutionLedgerQueryVariables; skip?: boolean; } | { skip: boolean; }) ) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<ExecutionLedgerQuery, ExecutionLedgerQueryVariables>(ExecutionLedgerDocument, options);
      }
export function useExecutionLedgerLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<ExecutionLedgerQuery, ExecutionLedgerQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<ExecutionLedgerQuery, ExecutionLedgerQueryVariables>(ExecutionLedgerDocument, options);
        }
// @ts-ignore
export function useExecutionLedgerSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<ExecutionLedgerQuery, ExecutionLedgerQueryVariables>): Apollo.UseSuspenseQueryResult<ExecutionLedgerQuery, ExecutionLedgerQueryVariables>;
export function useExecutionLedgerSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ExecutionLedgerQuery, ExecutionLedgerQueryVariables>): Apollo.UseSuspenseQueryResult<ExecutionLedgerQuery | undefined, ExecutionLedgerQueryVariables>;
export function useExecutionLedgerSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ExecutionLedgerQuery, ExecutionLedgerQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<ExecutionLedgerQuery, ExecutionLedgerQueryVariables>(ExecutionLedgerDocument, options);
        }
export type ExecutionLedgerQueryHookResult = ReturnType<typeof useExecutionLedgerQuery>;
export type ExecutionLedgerLazyQueryHookResult = ReturnType<typeof useExecutionLedgerLazyQuery>;
export type ExecutionLedgerSuspenseQueryHookResult = ReturnType<typeof useExecutionLedgerSuspenseQuery>;
export type ExecutionLedgerQueryResult = Apollo.QueryResult<ExecutionLedgerQuery, ExecutionLedgerQueryVariables>;
export const LedgerRollupDocument = gql`
    query LedgerRollup($since: DateTime!, $until: DateTime!) {
  ledgerRollup(since: $since, until: $until) {
    revenueUSD
    providerCostUSD
    sandboxCostUSD
    storageCostUSD
    deploymentCostUSD
    premiumReasoningCostUSD
    refundsUSD
    platformMarginUSD
    grossMarginPct
  }
}
    `;

/**
 * __useLedgerRollupQuery__
 *
 * To run a query within a React component, call `useLedgerRollupQuery` and pass it any options that fit your needs.
 * When your component renders, `useLedgerRollupQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useLedgerRollupQuery({
 *   variables: {
 *      since: // value for 'since'
 *      until: // value for 'until'
 *   },
 * });
 */
export function useLedgerRollupQuery(baseOptions: Apollo.QueryHookOptions<LedgerRollupQuery, LedgerRollupQueryVariables> & ({ variables: LedgerRollupQueryVariables; skip?: boolean; } | { skip: boolean; }) ) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<LedgerRollupQuery, LedgerRollupQueryVariables>(LedgerRollupDocument, options);
      }
export function useLedgerRollupLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<LedgerRollupQuery, LedgerRollupQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<LedgerRollupQuery, LedgerRollupQueryVariables>(LedgerRollupDocument, options);
        }
// @ts-ignore
export function useLedgerRollupSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<LedgerRollupQuery, LedgerRollupQueryVariables>): Apollo.UseSuspenseQueryResult<LedgerRollupQuery, LedgerRollupQueryVariables>;
export function useLedgerRollupSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<LedgerRollupQuery, LedgerRollupQueryVariables>): Apollo.UseSuspenseQueryResult<LedgerRollupQuery | undefined, LedgerRollupQueryVariables>;
export function useLedgerRollupSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<LedgerRollupQuery, LedgerRollupQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<LedgerRollupQuery, LedgerRollupQueryVariables>(LedgerRollupDocument, options);
        }
export type LedgerRollupQueryHookResult = ReturnType<typeof useLedgerRollupQuery>;
export type LedgerRollupLazyQueryHookResult = ReturnType<typeof useLedgerRollupLazyQuery>;
export type LedgerRollupSuspenseQueryHookResult = ReturnType<typeof useLedgerRollupSuspenseQuery>;
export type LedgerRollupQueryResult = Apollo.QueryResult<LedgerRollupQuery, LedgerRollupQueryVariables>;
export const TenantProfitTodayDocument = gql`
    query TenantProfitToday {
  tenantProfitToday {
    revenueUSD
    providerCostUSD
    sandboxCostUSD
    storageCostUSD
    deploymentCostUSD
    premiumReasoningCostUSD
    refundsUSD
    platformMarginUSD
    grossMarginPct
  }
}
    `;

/**
 * __useTenantProfitTodayQuery__
 *
 * To run a query within a React component, call `useTenantProfitTodayQuery` and pass it any options that fit your needs.
 * When your component renders, `useTenantProfitTodayQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useTenantProfitTodayQuery({
 *   variables: {
 *   },
 * });
 */
export function useTenantProfitTodayQuery(baseOptions?: Apollo.QueryHookOptions<TenantProfitTodayQuery, TenantProfitTodayQueryVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<TenantProfitTodayQuery, TenantProfitTodayQueryVariables>(TenantProfitTodayDocument, options);
      }
export function useTenantProfitTodayLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<TenantProfitTodayQuery, TenantProfitTodayQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<TenantProfitTodayQuery, TenantProfitTodayQueryVariables>(TenantProfitTodayDocument, options);
        }
// @ts-ignore
export function useTenantProfitTodaySuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<TenantProfitTodayQuery, TenantProfitTodayQueryVariables>): Apollo.UseSuspenseQueryResult<TenantProfitTodayQuery, TenantProfitTodayQueryVariables>;
export function useTenantProfitTodaySuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<TenantProfitTodayQuery, TenantProfitTodayQueryVariables>): Apollo.UseSuspenseQueryResult<TenantProfitTodayQuery | undefined, TenantProfitTodayQueryVariables>;
export function useTenantProfitTodaySuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<TenantProfitTodayQuery, TenantProfitTodayQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<TenantProfitTodayQuery, TenantProfitTodayQueryVariables>(TenantProfitTodayDocument, options);
        }
export type TenantProfitTodayQueryHookResult = ReturnType<typeof useTenantProfitTodayQuery>;
export type TenantProfitTodayLazyQueryHookResult = ReturnType<typeof useTenantProfitTodayLazyQuery>;
export type TenantProfitTodaySuspenseQueryHookResult = ReturnType<typeof useTenantProfitTodaySuspenseQuery>;
export type TenantProfitTodayQueryResult = Apollo.QueryResult<TenantProfitTodayQuery, TenantProfitTodayQueryVariables>;
export const NotificationsDocument = gql`
    query Notifications($unreadOnly: Boolean) {
  notifications(unreadOnly: $unreadOnly) {
    id
    kind
    title
    body
    link
    severity
    readAt
    createdAt
  }
  unreadNotificationCount
}
    `;

/**
 * __useNotificationsQuery__
 *
 * To run a query within a React component, call `useNotificationsQuery` and pass it any options that fit your needs.
 * When your component renders, `useNotificationsQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useNotificationsQuery({
 *   variables: {
 *      unreadOnly: // value for 'unreadOnly'
 *   },
 * });
 */
export function useNotificationsQuery(baseOptions?: Apollo.QueryHookOptions<NotificationsQuery, NotificationsQueryVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<NotificationsQuery, NotificationsQueryVariables>(NotificationsDocument, options);
      }
export function useNotificationsLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<NotificationsQuery, NotificationsQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<NotificationsQuery, NotificationsQueryVariables>(NotificationsDocument, options);
        }
// @ts-ignore
export function useNotificationsSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<NotificationsQuery, NotificationsQueryVariables>): Apollo.UseSuspenseQueryResult<NotificationsQuery, NotificationsQueryVariables>;
export function useNotificationsSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<NotificationsQuery, NotificationsQueryVariables>): Apollo.UseSuspenseQueryResult<NotificationsQuery | undefined, NotificationsQueryVariables>;
export function useNotificationsSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<NotificationsQuery, NotificationsQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<NotificationsQuery, NotificationsQueryVariables>(NotificationsDocument, options);
        }
export type NotificationsQueryHookResult = ReturnType<typeof useNotificationsQuery>;
export type NotificationsLazyQueryHookResult = ReturnType<typeof useNotificationsLazyQuery>;
export type NotificationsSuspenseQueryHookResult = ReturnType<typeof useNotificationsSuspenseQuery>;
export type NotificationsQueryResult = Apollo.QueryResult<NotificationsQuery, NotificationsQueryVariables>;
export const NotificationPreferencesDocument = gql`
    query NotificationPreferences {
  notificationPreferences {
    userId
    pauseAll
    weeklyDigest
    onRunComplete {
      email
      inApp
    }
    onGateFailed {
      email
      inApp
    }
    onDeployDone {
      email
      inApp
    }
    onBudgetWarning {
      email
      inApp
    }
    onReceipt {
      email
      inApp
    }
  }
}
    `;

/**
 * __useNotificationPreferencesQuery__
 *
 * To run a query within a React component, call `useNotificationPreferencesQuery` and pass it any options that fit your needs.
 * When your component renders, `useNotificationPreferencesQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useNotificationPreferencesQuery({
 *   variables: {
 *   },
 * });
 */
export function useNotificationPreferencesQuery(baseOptions?: Apollo.QueryHookOptions<NotificationPreferencesQuery, NotificationPreferencesQueryVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<NotificationPreferencesQuery, NotificationPreferencesQueryVariables>(NotificationPreferencesDocument, options);
      }
export function useNotificationPreferencesLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<NotificationPreferencesQuery, NotificationPreferencesQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<NotificationPreferencesQuery, NotificationPreferencesQueryVariables>(NotificationPreferencesDocument, options);
        }
// @ts-ignore
export function useNotificationPreferencesSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<NotificationPreferencesQuery, NotificationPreferencesQueryVariables>): Apollo.UseSuspenseQueryResult<NotificationPreferencesQuery, NotificationPreferencesQueryVariables>;
export function useNotificationPreferencesSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<NotificationPreferencesQuery, NotificationPreferencesQueryVariables>): Apollo.UseSuspenseQueryResult<NotificationPreferencesQuery | undefined, NotificationPreferencesQueryVariables>;
export function useNotificationPreferencesSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<NotificationPreferencesQuery, NotificationPreferencesQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<NotificationPreferencesQuery, NotificationPreferencesQueryVariables>(NotificationPreferencesDocument, options);
        }
export type NotificationPreferencesQueryHookResult = ReturnType<typeof useNotificationPreferencesQuery>;
export type NotificationPreferencesLazyQueryHookResult = ReturnType<typeof useNotificationPreferencesLazyQuery>;
export type NotificationPreferencesSuspenseQueryHookResult = ReturnType<typeof useNotificationPreferencesSuspenseQuery>;
export type NotificationPreferencesQueryResult = Apollo.QueryResult<NotificationPreferencesQuery, NotificationPreferencesQueryVariables>;
export const MarkNotificationReadDocument = gql`
    mutation MarkNotificationRead($id: ID!) {
  markNotificationRead(id: $id) {
    id
    readAt
  }
}
    `;
export type MarkNotificationReadMutationFn = Apollo.MutationFunction<MarkNotificationReadMutation, MarkNotificationReadMutationVariables>;

/**
 * __useMarkNotificationReadMutation__
 *
 * To run a mutation, you first call `useMarkNotificationReadMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useMarkNotificationReadMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [markNotificationReadMutation, { data, loading, error }] = useMarkNotificationReadMutation({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function useMarkNotificationReadMutation(baseOptions?: Apollo.MutationHookOptions<MarkNotificationReadMutation, MarkNotificationReadMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<MarkNotificationReadMutation, MarkNotificationReadMutationVariables>(MarkNotificationReadDocument, options);
      }
export type MarkNotificationReadMutationHookResult = ReturnType<typeof useMarkNotificationReadMutation>;
export type MarkNotificationReadMutationResult = Apollo.MutationResult<MarkNotificationReadMutation>;
export type MarkNotificationReadMutationOptions = Apollo.BaseMutationOptions<MarkNotificationReadMutation, MarkNotificationReadMutationVariables>;
export const MarkAllNotificationsReadDocument = gql`
    mutation MarkAllNotificationsRead {
  markAllNotificationsRead
}
    `;
export type MarkAllNotificationsReadMutationFn = Apollo.MutationFunction<MarkAllNotificationsReadMutation, MarkAllNotificationsReadMutationVariables>;

/**
 * __useMarkAllNotificationsReadMutation__
 *
 * To run a mutation, you first call `useMarkAllNotificationsReadMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useMarkAllNotificationsReadMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [markAllNotificationsReadMutation, { data, loading, error }] = useMarkAllNotificationsReadMutation({
 *   variables: {
 *   },
 * });
 */
export function useMarkAllNotificationsReadMutation(baseOptions?: Apollo.MutationHookOptions<MarkAllNotificationsReadMutation, MarkAllNotificationsReadMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<MarkAllNotificationsReadMutation, MarkAllNotificationsReadMutationVariables>(MarkAllNotificationsReadDocument, options);
      }
export type MarkAllNotificationsReadMutationHookResult = ReturnType<typeof useMarkAllNotificationsReadMutation>;
export type MarkAllNotificationsReadMutationResult = Apollo.MutationResult<MarkAllNotificationsReadMutation>;
export type MarkAllNotificationsReadMutationOptions = Apollo.BaseMutationOptions<MarkAllNotificationsReadMutation, MarkAllNotificationsReadMutationVariables>;
export const UpdateNotificationPreferencesDocument = gql`
    mutation UpdateNotificationPreferences($input: NotificationPreferencesInput!) {
  updateNotificationPreferences(input: $input) {
    userId
    pauseAll
    weeklyDigest
    onRunComplete {
      email
      inApp
    }
    onGateFailed {
      email
      inApp
    }
    onDeployDone {
      email
      inApp
    }
    onBudgetWarning {
      email
      inApp
    }
    onReceipt {
      email
      inApp
    }
  }
}
    `;
export type UpdateNotificationPreferencesMutationFn = Apollo.MutationFunction<UpdateNotificationPreferencesMutation, UpdateNotificationPreferencesMutationVariables>;

/**
 * __useUpdateNotificationPreferencesMutation__
 *
 * To run a mutation, you first call `useUpdateNotificationPreferencesMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useUpdateNotificationPreferencesMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [updateNotificationPreferencesMutation, { data, loading, error }] = useUpdateNotificationPreferencesMutation({
 *   variables: {
 *      input: // value for 'input'
 *   },
 * });
 */
export function useUpdateNotificationPreferencesMutation(baseOptions?: Apollo.MutationHookOptions<UpdateNotificationPreferencesMutation, UpdateNotificationPreferencesMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<UpdateNotificationPreferencesMutation, UpdateNotificationPreferencesMutationVariables>(UpdateNotificationPreferencesDocument, options);
      }
export type UpdateNotificationPreferencesMutationHookResult = ReturnType<typeof useUpdateNotificationPreferencesMutation>;
export type UpdateNotificationPreferencesMutationResult = Apollo.MutationResult<UpdateNotificationPreferencesMutation>;
export type UpdateNotificationPreferencesMutationOptions = Apollo.BaseMutationOptions<UpdateNotificationPreferencesMutation, UpdateNotificationPreferencesMutationVariables>;
export const NotificationStreamDocument = gql`
    subscription NotificationStream {
  notificationStream {
    id
    kind
    title
    body
    link
    severity
    readAt
    createdAt
  }
}
    `;

/**
 * __useNotificationStreamSubscription__
 *
 * To run a query within a React component, call `useNotificationStreamSubscription` and pass it any options that fit your needs.
 * When your component renders, `useNotificationStreamSubscription` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the subscription, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useNotificationStreamSubscription({
 *   variables: {
 *   },
 * });
 */
export function useNotificationStreamSubscription(baseOptions?: Apollo.SubscriptionHookOptions<NotificationStreamSubscription, NotificationStreamSubscriptionVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useSubscription<NotificationStreamSubscription, NotificationStreamSubscriptionVariables>(NotificationStreamDocument, options);
      }
export type NotificationStreamSubscriptionHookResult = ReturnType<typeof useNotificationStreamSubscription>;
export type NotificationStreamSubscriptionResult = Apollo.SubscriptionResult<NotificationStreamSubscription>;
export const OperatorPendingApprovalsDocument = gql`
    query OperatorPendingApprovals($tenantID: ID) {
  operatorPendingApprovals(tenantID: $tenantID) {
    ...DeployApprovalCore
  }
}
    ${DeployApprovalCoreFragmentDoc}`;

/**
 * __useOperatorPendingApprovalsQuery__
 *
 * To run a query within a React component, call `useOperatorPendingApprovalsQuery` and pass it any options that fit your needs.
 * When your component renders, `useOperatorPendingApprovalsQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useOperatorPendingApprovalsQuery({
 *   variables: {
 *      tenantID: // value for 'tenantID'
 *   },
 * });
 */
export function useOperatorPendingApprovalsQuery(baseOptions?: Apollo.QueryHookOptions<OperatorPendingApprovalsQuery, OperatorPendingApprovalsQueryVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<OperatorPendingApprovalsQuery, OperatorPendingApprovalsQueryVariables>(OperatorPendingApprovalsDocument, options);
      }
export function useOperatorPendingApprovalsLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<OperatorPendingApprovalsQuery, OperatorPendingApprovalsQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<OperatorPendingApprovalsQuery, OperatorPendingApprovalsQueryVariables>(OperatorPendingApprovalsDocument, options);
        }
// @ts-ignore
export function useOperatorPendingApprovalsSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<OperatorPendingApprovalsQuery, OperatorPendingApprovalsQueryVariables>): Apollo.UseSuspenseQueryResult<OperatorPendingApprovalsQuery, OperatorPendingApprovalsQueryVariables>;
export function useOperatorPendingApprovalsSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<OperatorPendingApprovalsQuery, OperatorPendingApprovalsQueryVariables>): Apollo.UseSuspenseQueryResult<OperatorPendingApprovalsQuery | undefined, OperatorPendingApprovalsQueryVariables>;
export function useOperatorPendingApprovalsSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<OperatorPendingApprovalsQuery, OperatorPendingApprovalsQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<OperatorPendingApprovalsQuery, OperatorPendingApprovalsQueryVariables>(OperatorPendingApprovalsDocument, options);
        }
export type OperatorPendingApprovalsQueryHookResult = ReturnType<typeof useOperatorPendingApprovalsQuery>;
export type OperatorPendingApprovalsLazyQueryHookResult = ReturnType<typeof useOperatorPendingApprovalsLazyQuery>;
export type OperatorPendingApprovalsSuspenseQueryHookResult = ReturnType<typeof useOperatorPendingApprovalsSuspenseQuery>;
export type OperatorPendingApprovalsQueryResult = Apollo.QueryResult<OperatorPendingApprovalsQuery, OperatorPendingApprovalsQueryVariables>;
export const OperatorAbuseScoreDocument = gql`
    query OperatorAbuseScore($tenantID: ID!, $userID: ID!) {
  operatorAbuseScore(tenantID: $tenantID, userID: $userID) {
    tenantID
    userID
    score
    tier
  }
}
    `;

/**
 * __useOperatorAbuseScoreQuery__
 *
 * To run a query within a React component, call `useOperatorAbuseScoreQuery` and pass it any options that fit your needs.
 * When your component renders, `useOperatorAbuseScoreQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useOperatorAbuseScoreQuery({
 *   variables: {
 *      tenantID: // value for 'tenantID'
 *      userID: // value for 'userID'
 *   },
 * });
 */
export function useOperatorAbuseScoreQuery(baseOptions: Apollo.QueryHookOptions<OperatorAbuseScoreQuery, OperatorAbuseScoreQueryVariables> & ({ variables: OperatorAbuseScoreQueryVariables; skip?: boolean; } | { skip: boolean; }) ) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<OperatorAbuseScoreQuery, OperatorAbuseScoreQueryVariables>(OperatorAbuseScoreDocument, options);
      }
export function useOperatorAbuseScoreLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<OperatorAbuseScoreQuery, OperatorAbuseScoreQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<OperatorAbuseScoreQuery, OperatorAbuseScoreQueryVariables>(OperatorAbuseScoreDocument, options);
        }
// @ts-ignore
export function useOperatorAbuseScoreSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<OperatorAbuseScoreQuery, OperatorAbuseScoreQueryVariables>): Apollo.UseSuspenseQueryResult<OperatorAbuseScoreQuery, OperatorAbuseScoreQueryVariables>;
export function useOperatorAbuseScoreSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<OperatorAbuseScoreQuery, OperatorAbuseScoreQueryVariables>): Apollo.UseSuspenseQueryResult<OperatorAbuseScoreQuery | undefined, OperatorAbuseScoreQueryVariables>;
export function useOperatorAbuseScoreSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<OperatorAbuseScoreQuery, OperatorAbuseScoreQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<OperatorAbuseScoreQuery, OperatorAbuseScoreQueryVariables>(OperatorAbuseScoreDocument, options);
        }
export type OperatorAbuseScoreQueryHookResult = ReturnType<typeof useOperatorAbuseScoreQuery>;
export type OperatorAbuseScoreLazyQueryHookResult = ReturnType<typeof useOperatorAbuseScoreLazyQuery>;
export type OperatorAbuseScoreSuspenseQueryHookResult = ReturnType<typeof useOperatorAbuseScoreSuspenseQuery>;
export type OperatorAbuseScoreQueryResult = Apollo.QueryResult<OperatorAbuseScoreQuery, OperatorAbuseScoreQueryVariables>;
export const OperatorScaleSnapshotDocument = gql`
    query OperatorScaleSnapshot {
  operatorScaleSnapshot {
    activeExecutions
    queuedExecutions
    sandboxCapacity
    workerUtilizationPct
  }
}
    `;

/**
 * __useOperatorScaleSnapshotQuery__
 *
 * To run a query within a React component, call `useOperatorScaleSnapshotQuery` and pass it any options that fit your needs.
 * When your component renders, `useOperatorScaleSnapshotQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useOperatorScaleSnapshotQuery({
 *   variables: {
 *   },
 * });
 */
export function useOperatorScaleSnapshotQuery(baseOptions?: Apollo.QueryHookOptions<OperatorScaleSnapshotQuery, OperatorScaleSnapshotQueryVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<OperatorScaleSnapshotQuery, OperatorScaleSnapshotQueryVariables>(OperatorScaleSnapshotDocument, options);
      }
export function useOperatorScaleSnapshotLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<OperatorScaleSnapshotQuery, OperatorScaleSnapshotQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<OperatorScaleSnapshotQuery, OperatorScaleSnapshotQueryVariables>(OperatorScaleSnapshotDocument, options);
        }
// @ts-ignore
export function useOperatorScaleSnapshotSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<OperatorScaleSnapshotQuery, OperatorScaleSnapshotQueryVariables>): Apollo.UseSuspenseQueryResult<OperatorScaleSnapshotQuery, OperatorScaleSnapshotQueryVariables>;
export function useOperatorScaleSnapshotSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<OperatorScaleSnapshotQuery, OperatorScaleSnapshotQueryVariables>): Apollo.UseSuspenseQueryResult<OperatorScaleSnapshotQuery | undefined, OperatorScaleSnapshotQueryVariables>;
export function useOperatorScaleSnapshotSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<OperatorScaleSnapshotQuery, OperatorScaleSnapshotQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<OperatorScaleSnapshotQuery, OperatorScaleSnapshotQueryVariables>(OperatorScaleSnapshotDocument, options);
        }
export type OperatorScaleSnapshotQueryHookResult = ReturnType<typeof useOperatorScaleSnapshotQuery>;
export type OperatorScaleSnapshotLazyQueryHookResult = ReturnType<typeof useOperatorScaleSnapshotLazyQuery>;
export type OperatorScaleSnapshotSuspenseQueryHookResult = ReturnType<typeof useOperatorScaleSnapshotSuspenseQuery>;
export type OperatorScaleSnapshotQueryResult = Apollo.QueryResult<OperatorScaleSnapshotQuery, OperatorScaleSnapshotQueryVariables>;
export const OperatorWalletSnapshotDocument = gql`
    query OperatorWalletSnapshot($tenantID: ID!) {
  operatorWalletSnapshot(tenantID: $tenantID) {
    tenantID
    balanceUSD
    holdUSD
    lifetimeTopUpUSD
    lifetimeSpendUSD
  }
}
    `;

/**
 * __useOperatorWalletSnapshotQuery__
 *
 * To run a query within a React component, call `useOperatorWalletSnapshotQuery` and pass it any options that fit your needs.
 * When your component renders, `useOperatorWalletSnapshotQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useOperatorWalletSnapshotQuery({
 *   variables: {
 *      tenantID: // value for 'tenantID'
 *   },
 * });
 */
export function useOperatorWalletSnapshotQuery(baseOptions: Apollo.QueryHookOptions<OperatorWalletSnapshotQuery, OperatorWalletSnapshotQueryVariables> & ({ variables: OperatorWalletSnapshotQueryVariables; skip?: boolean; } | { skip: boolean; }) ) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<OperatorWalletSnapshotQuery, OperatorWalletSnapshotQueryVariables>(OperatorWalletSnapshotDocument, options);
      }
export function useOperatorWalletSnapshotLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<OperatorWalletSnapshotQuery, OperatorWalletSnapshotQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<OperatorWalletSnapshotQuery, OperatorWalletSnapshotQueryVariables>(OperatorWalletSnapshotDocument, options);
        }
// @ts-ignore
export function useOperatorWalletSnapshotSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<OperatorWalletSnapshotQuery, OperatorWalletSnapshotQueryVariables>): Apollo.UseSuspenseQueryResult<OperatorWalletSnapshotQuery, OperatorWalletSnapshotQueryVariables>;
export function useOperatorWalletSnapshotSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<OperatorWalletSnapshotQuery, OperatorWalletSnapshotQueryVariables>): Apollo.UseSuspenseQueryResult<OperatorWalletSnapshotQuery | undefined, OperatorWalletSnapshotQueryVariables>;
export function useOperatorWalletSnapshotSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<OperatorWalletSnapshotQuery, OperatorWalletSnapshotQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<OperatorWalletSnapshotQuery, OperatorWalletSnapshotQueryVariables>(OperatorWalletSnapshotDocument, options);
        }
export type OperatorWalletSnapshotQueryHookResult = ReturnType<typeof useOperatorWalletSnapshotQuery>;
export type OperatorWalletSnapshotLazyQueryHookResult = ReturnType<typeof useOperatorWalletSnapshotLazyQuery>;
export type OperatorWalletSnapshotSuspenseQueryHookResult = ReturnType<typeof useOperatorWalletSnapshotSuspenseQuery>;
export type OperatorWalletSnapshotQueryResult = Apollo.QueryResult<OperatorWalletSnapshotQuery, OperatorWalletSnapshotQueryVariables>;
export const OperatorAuditCursorDocument = gql`
    query OperatorAuditCursor($since: DateTime!, $limit: Int) {
  operatorAuditCursor(since: $since, limit: $limit) {
    id
    timestamp
    action
    outcome
    hash
  }
}
    `;

/**
 * __useOperatorAuditCursorQuery__
 *
 * To run a query within a React component, call `useOperatorAuditCursorQuery` and pass it any options that fit your needs.
 * When your component renders, `useOperatorAuditCursorQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useOperatorAuditCursorQuery({
 *   variables: {
 *      since: // value for 'since'
 *      limit: // value for 'limit'
 *   },
 * });
 */
export function useOperatorAuditCursorQuery(baseOptions: Apollo.QueryHookOptions<OperatorAuditCursorQuery, OperatorAuditCursorQueryVariables> & ({ variables: OperatorAuditCursorQueryVariables; skip?: boolean; } | { skip: boolean; }) ) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<OperatorAuditCursorQuery, OperatorAuditCursorQueryVariables>(OperatorAuditCursorDocument, options);
      }
export function useOperatorAuditCursorLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<OperatorAuditCursorQuery, OperatorAuditCursorQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<OperatorAuditCursorQuery, OperatorAuditCursorQueryVariables>(OperatorAuditCursorDocument, options);
        }
// @ts-ignore
export function useOperatorAuditCursorSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<OperatorAuditCursorQuery, OperatorAuditCursorQueryVariables>): Apollo.UseSuspenseQueryResult<OperatorAuditCursorQuery, OperatorAuditCursorQueryVariables>;
export function useOperatorAuditCursorSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<OperatorAuditCursorQuery, OperatorAuditCursorQueryVariables>): Apollo.UseSuspenseQueryResult<OperatorAuditCursorQuery | undefined, OperatorAuditCursorQueryVariables>;
export function useOperatorAuditCursorSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<OperatorAuditCursorQuery, OperatorAuditCursorQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<OperatorAuditCursorQuery, OperatorAuditCursorQueryVariables>(OperatorAuditCursorDocument, options);
        }
export type OperatorAuditCursorQueryHookResult = ReturnType<typeof useOperatorAuditCursorQuery>;
export type OperatorAuditCursorLazyQueryHookResult = ReturnType<typeof useOperatorAuditCursorLazyQuery>;
export type OperatorAuditCursorSuspenseQueryHookResult = ReturnType<typeof useOperatorAuditCursorSuspenseQuery>;
export type OperatorAuditCursorQueryResult = Apollo.QueryResult<OperatorAuditCursorQuery, OperatorAuditCursorQueryVariables>;
export const PatchesDocument = gql`
    query Patches($projectId: ID!) {
  patches(projectId: $projectId) {
    ...PatchCore
  }
}
    ${PatchCoreFragmentDoc}`;

/**
 * __usePatchesQuery__
 *
 * To run a query within a React component, call `usePatchesQuery` and pass it any options that fit your needs.
 * When your component renders, `usePatchesQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = usePatchesQuery({
 *   variables: {
 *      projectId: // value for 'projectId'
 *   },
 * });
 */
export function usePatchesQuery(baseOptions: Apollo.QueryHookOptions<PatchesQuery, PatchesQueryVariables> & ({ variables: PatchesQueryVariables; skip?: boolean; } | { skip: boolean; }) ) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<PatchesQuery, PatchesQueryVariables>(PatchesDocument, options);
      }
export function usePatchesLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<PatchesQuery, PatchesQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<PatchesQuery, PatchesQueryVariables>(PatchesDocument, options);
        }
// @ts-ignore
export function usePatchesSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<PatchesQuery, PatchesQueryVariables>): Apollo.UseSuspenseQueryResult<PatchesQuery, PatchesQueryVariables>;
export function usePatchesSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<PatchesQuery, PatchesQueryVariables>): Apollo.UseSuspenseQueryResult<PatchesQuery | undefined, PatchesQueryVariables>;
export function usePatchesSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<PatchesQuery, PatchesQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<PatchesQuery, PatchesQueryVariables>(PatchesDocument, options);
        }
export type PatchesQueryHookResult = ReturnType<typeof usePatchesQuery>;
export type PatchesLazyQueryHookResult = ReturnType<typeof usePatchesLazyQuery>;
export type PatchesSuspenseQueryHookResult = ReturnType<typeof usePatchesSuspenseQuery>;
export type PatchesQueryResult = Apollo.QueryResult<PatchesQuery, PatchesQueryVariables>;
export const ApplyPatchDocument = gql`
    mutation ApplyPatch($id: ID!) {
  applyPatch(patchId: $id) {
    id
    status
    appliedAt
  }
}
    `;
export type ApplyPatchMutationFn = Apollo.MutationFunction<ApplyPatchMutation, ApplyPatchMutationVariables>;

/**
 * __useApplyPatchMutation__
 *
 * To run a mutation, you first call `useApplyPatchMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useApplyPatchMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [applyPatchMutation, { data, loading, error }] = useApplyPatchMutation({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function useApplyPatchMutation(baseOptions?: Apollo.MutationHookOptions<ApplyPatchMutation, ApplyPatchMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<ApplyPatchMutation, ApplyPatchMutationVariables>(ApplyPatchDocument, options);
      }
export type ApplyPatchMutationHookResult = ReturnType<typeof useApplyPatchMutation>;
export type ApplyPatchMutationResult = Apollo.MutationResult<ApplyPatchMutation>;
export type ApplyPatchMutationOptions = Apollo.BaseMutationOptions<ApplyPatchMutation, ApplyPatchMutationVariables>;
export const RollbackPatchDocument = gql`
    mutation RollbackPatch($id: ID!) {
  rollbackPatch(patchId: $id)
}
    `;
export type RollbackPatchMutationFn = Apollo.MutationFunction<RollbackPatchMutation, RollbackPatchMutationVariables>;

/**
 * __useRollbackPatchMutation__
 *
 * To run a mutation, you first call `useRollbackPatchMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useRollbackPatchMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [rollbackPatchMutation, { data, loading, error }] = useRollbackPatchMutation({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function useRollbackPatchMutation(baseOptions?: Apollo.MutationHookOptions<RollbackPatchMutation, RollbackPatchMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<RollbackPatchMutation, RollbackPatchMutationVariables>(RollbackPatchDocument, options);
      }
export type RollbackPatchMutationHookResult = ReturnType<typeof useRollbackPatchMutation>;
export type RollbackPatchMutationResult = Apollo.MutationResult<RollbackPatchMutation>;
export type RollbackPatchMutationOptions = Apollo.BaseMutationOptions<RollbackPatchMutation, RollbackPatchMutationVariables>;
export const ProfitGuardDecisionsDocument = gql`
    query ProfitGuardDecisions($executionID: ID, $limit: Int) {
  profitGuardDecisions(executionID: $executionID, limit: $limit) {
    id
    executionID
    enforcementPoint
    decision
    reason
    spentUSD
    reservedUSD
    estimatedStepCostUSD
    expectedCompletionDelta
    expectedMarginPct
    riskScore
    recommendedProvider
    createdAt
  }
}
    `;

/**
 * __useProfitGuardDecisionsQuery__
 *
 * To run a query within a React component, call `useProfitGuardDecisionsQuery` and pass it any options that fit your needs.
 * When your component renders, `useProfitGuardDecisionsQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useProfitGuardDecisionsQuery({
 *   variables: {
 *      executionID: // value for 'executionID'
 *      limit: // value for 'limit'
 *   },
 * });
 */
export function useProfitGuardDecisionsQuery(baseOptions?: Apollo.QueryHookOptions<ProfitGuardDecisionsQuery, ProfitGuardDecisionsQueryVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<ProfitGuardDecisionsQuery, ProfitGuardDecisionsQueryVariables>(ProfitGuardDecisionsDocument, options);
      }
export function useProfitGuardDecisionsLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<ProfitGuardDecisionsQuery, ProfitGuardDecisionsQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<ProfitGuardDecisionsQuery, ProfitGuardDecisionsQueryVariables>(ProfitGuardDecisionsDocument, options);
        }
// @ts-ignore
export function useProfitGuardDecisionsSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<ProfitGuardDecisionsQuery, ProfitGuardDecisionsQueryVariables>): Apollo.UseSuspenseQueryResult<ProfitGuardDecisionsQuery, ProfitGuardDecisionsQueryVariables>;
export function useProfitGuardDecisionsSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ProfitGuardDecisionsQuery, ProfitGuardDecisionsQueryVariables>): Apollo.UseSuspenseQueryResult<ProfitGuardDecisionsQuery | undefined, ProfitGuardDecisionsQueryVariables>;
export function useProfitGuardDecisionsSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ProfitGuardDecisionsQuery, ProfitGuardDecisionsQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<ProfitGuardDecisionsQuery, ProfitGuardDecisionsQueryVariables>(ProfitGuardDecisionsDocument, options);
        }
export type ProfitGuardDecisionsQueryHookResult = ReturnType<typeof useProfitGuardDecisionsQuery>;
export type ProfitGuardDecisionsLazyQueryHookResult = ReturnType<typeof useProfitGuardDecisionsLazyQuery>;
export type ProfitGuardDecisionsSuspenseQueryHookResult = ReturnType<typeof useProfitGuardDecisionsSuspenseQuery>;
export type ProfitGuardDecisionsQueryResult = Apollo.QueryResult<ProfitGuardDecisionsQuery, ProfitGuardDecisionsQueryVariables>;
export const ProjectsDocument = gql`
    query Projects($limit: Int, $offset: Int) {
  projects(limit: $limit, offset: $offset) {
    ...ProjectCore
  }
}
    ${ProjectCoreFragmentDoc}`;

/**
 * __useProjectsQuery__
 *
 * To run a query within a React component, call `useProjectsQuery` and pass it any options that fit your needs.
 * When your component renders, `useProjectsQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useProjectsQuery({
 *   variables: {
 *      limit: // value for 'limit'
 *      offset: // value for 'offset'
 *   },
 * });
 */
export function useProjectsQuery(baseOptions?: Apollo.QueryHookOptions<ProjectsQuery, ProjectsQueryVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<ProjectsQuery, ProjectsQueryVariables>(ProjectsDocument, options);
      }
export function useProjectsLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<ProjectsQuery, ProjectsQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<ProjectsQuery, ProjectsQueryVariables>(ProjectsDocument, options);
        }
// @ts-ignore
export function useProjectsSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<ProjectsQuery, ProjectsQueryVariables>): Apollo.UseSuspenseQueryResult<ProjectsQuery, ProjectsQueryVariables>;
export function useProjectsSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ProjectsQuery, ProjectsQueryVariables>): Apollo.UseSuspenseQueryResult<ProjectsQuery | undefined, ProjectsQueryVariables>;
export function useProjectsSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ProjectsQuery, ProjectsQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<ProjectsQuery, ProjectsQueryVariables>(ProjectsDocument, options);
        }
export type ProjectsQueryHookResult = ReturnType<typeof useProjectsQuery>;
export type ProjectsLazyQueryHookResult = ReturnType<typeof useProjectsLazyQuery>;
export type ProjectsSuspenseQueryHookResult = ReturnType<typeof useProjectsSuspenseQuery>;
export type ProjectsQueryResult = Apollo.QueryResult<ProjectsQuery, ProjectsQueryVariables>;
export const DashboardProjectsDocument = gql`
    query DashboardProjects {
  projects {
    ...ProjectCore
    files {
      path
    }
  }
}
    ${ProjectCoreFragmentDoc}`;

/**
 * __useDashboardProjectsQuery__
 *
 * To run a query within a React component, call `useDashboardProjectsQuery` and pass it any options that fit your needs.
 * When your component renders, `useDashboardProjectsQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useDashboardProjectsQuery({
 *   variables: {
 *   },
 * });
 */
export function useDashboardProjectsQuery(baseOptions?: Apollo.QueryHookOptions<DashboardProjectsQuery, DashboardProjectsQueryVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<DashboardProjectsQuery, DashboardProjectsQueryVariables>(DashboardProjectsDocument, options);
      }
export function useDashboardProjectsLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<DashboardProjectsQuery, DashboardProjectsQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<DashboardProjectsQuery, DashboardProjectsQueryVariables>(DashboardProjectsDocument, options);
        }
// @ts-ignore
export function useDashboardProjectsSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<DashboardProjectsQuery, DashboardProjectsQueryVariables>): Apollo.UseSuspenseQueryResult<DashboardProjectsQuery, DashboardProjectsQueryVariables>;
export function useDashboardProjectsSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<DashboardProjectsQuery, DashboardProjectsQueryVariables>): Apollo.UseSuspenseQueryResult<DashboardProjectsQuery | undefined, DashboardProjectsQueryVariables>;
export function useDashboardProjectsSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<DashboardProjectsQuery, DashboardProjectsQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<DashboardProjectsQuery, DashboardProjectsQueryVariables>(DashboardProjectsDocument, options);
        }
export type DashboardProjectsQueryHookResult = ReturnType<typeof useDashboardProjectsQuery>;
export type DashboardProjectsLazyQueryHookResult = ReturnType<typeof useDashboardProjectsLazyQuery>;
export type DashboardProjectsSuspenseQueryHookResult = ReturnType<typeof useDashboardProjectsSuspenseQuery>;
export type DashboardProjectsQueryResult = Apollo.QueryResult<DashboardProjectsQuery, DashboardProjectsQueryVariables>;
export const ProjectDocument = gql`
    query Project($id: ID!) {
  project(id: $id) {
    ...ProjectCore
  }
}
    ${ProjectCoreFragmentDoc}`;

/**
 * __useProjectQuery__
 *
 * To run a query within a React component, call `useProjectQuery` and pass it any options that fit your needs.
 * When your component renders, `useProjectQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useProjectQuery({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function useProjectQuery(baseOptions: Apollo.QueryHookOptions<ProjectQuery, ProjectQueryVariables> & ({ variables: ProjectQueryVariables; skip?: boolean; } | { skip: boolean; }) ) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<ProjectQuery, ProjectQueryVariables>(ProjectDocument, options);
      }
export function useProjectLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<ProjectQuery, ProjectQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<ProjectQuery, ProjectQueryVariables>(ProjectDocument, options);
        }
// @ts-ignore
export function useProjectSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<ProjectQuery, ProjectQueryVariables>): Apollo.UseSuspenseQueryResult<ProjectQuery, ProjectQueryVariables>;
export function useProjectSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ProjectQuery, ProjectQueryVariables>): Apollo.UseSuspenseQueryResult<ProjectQuery | undefined, ProjectQueryVariables>;
export function useProjectSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ProjectQuery, ProjectQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<ProjectQuery, ProjectQueryVariables>(ProjectDocument, options);
        }
export type ProjectQueryHookResult = ReturnType<typeof useProjectQuery>;
export type ProjectLazyQueryHookResult = ReturnType<typeof useProjectLazyQuery>;
export type ProjectSuspenseQueryHookResult = ReturnType<typeof useProjectSuspenseQuery>;
export type ProjectQueryResult = Apollo.QueryResult<ProjectQuery, ProjectQueryVariables>;
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

/**
 * __useProjectFilesQuery__
 *
 * To run a query within a React component, call `useProjectFilesQuery` and pass it any options that fit your needs.
 * When your component renders, `useProjectFilesQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useProjectFilesQuery({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function useProjectFilesQuery(baseOptions: Apollo.QueryHookOptions<ProjectFilesQuery, ProjectFilesQueryVariables> & ({ variables: ProjectFilesQueryVariables; skip?: boolean; } | { skip: boolean; }) ) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<ProjectFilesQuery, ProjectFilesQueryVariables>(ProjectFilesDocument, options);
      }
export function useProjectFilesLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<ProjectFilesQuery, ProjectFilesQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<ProjectFilesQuery, ProjectFilesQueryVariables>(ProjectFilesDocument, options);
        }
// @ts-ignore
export function useProjectFilesSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<ProjectFilesQuery, ProjectFilesQueryVariables>): Apollo.UseSuspenseQueryResult<ProjectFilesQuery, ProjectFilesQueryVariables>;
export function useProjectFilesSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ProjectFilesQuery, ProjectFilesQueryVariables>): Apollo.UseSuspenseQueryResult<ProjectFilesQuery | undefined, ProjectFilesQueryVariables>;
export function useProjectFilesSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ProjectFilesQuery, ProjectFilesQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<ProjectFilesQuery, ProjectFilesQueryVariables>(ProjectFilesDocument, options);
        }
export type ProjectFilesQueryHookResult = ReturnType<typeof useProjectFilesQuery>;
export type ProjectFilesLazyQueryHookResult = ReturnType<typeof useProjectFilesLazyQuery>;
export type ProjectFilesSuspenseQueryHookResult = ReturnType<typeof useProjectFilesSuspenseQuery>;
export type ProjectFilesQueryResult = Apollo.QueryResult<ProjectFilesQuery, ProjectFilesQueryVariables>;
export const CreateProjectDocument = gql`
    mutation CreateProject($input: CreateProjectInput!) {
  createProject(input: $input) {
    ...ProjectCore
  }
}
    ${ProjectCoreFragmentDoc}`;
export type CreateProjectMutationFn = Apollo.MutationFunction<CreateProjectMutation, CreateProjectMutationVariables>;

/**
 * __useCreateProjectMutation__
 *
 * To run a mutation, you first call `useCreateProjectMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useCreateProjectMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [createProjectMutation, { data, loading, error }] = useCreateProjectMutation({
 *   variables: {
 *      input: // value for 'input'
 *   },
 * });
 */
export function useCreateProjectMutation(baseOptions?: Apollo.MutationHookOptions<CreateProjectMutation, CreateProjectMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<CreateProjectMutation, CreateProjectMutationVariables>(CreateProjectDocument, options);
      }
export type CreateProjectMutationHookResult = ReturnType<typeof useCreateProjectMutation>;
export type CreateProjectMutationResult = Apollo.MutationResult<CreateProjectMutation>;
export type CreateProjectMutationOptions = Apollo.BaseMutationOptions<CreateProjectMutation, CreateProjectMutationVariables>;
export const UpdateProjectDocument = gql`
    mutation UpdateProject($id: ID!, $input: UpdateProjectInput!) {
  updateProject(id: $id, input: $input) {
    ...ProjectCore
  }
}
    ${ProjectCoreFragmentDoc}`;
export type UpdateProjectMutationFn = Apollo.MutationFunction<UpdateProjectMutation, UpdateProjectMutationVariables>;

/**
 * __useUpdateProjectMutation__
 *
 * To run a mutation, you first call `useUpdateProjectMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useUpdateProjectMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [updateProjectMutation, { data, loading, error }] = useUpdateProjectMutation({
 *   variables: {
 *      id: // value for 'id'
 *      input: // value for 'input'
 *   },
 * });
 */
export function useUpdateProjectMutation(baseOptions?: Apollo.MutationHookOptions<UpdateProjectMutation, UpdateProjectMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<UpdateProjectMutation, UpdateProjectMutationVariables>(UpdateProjectDocument, options);
      }
export type UpdateProjectMutationHookResult = ReturnType<typeof useUpdateProjectMutation>;
export type UpdateProjectMutationResult = Apollo.MutationResult<UpdateProjectMutation>;
export type UpdateProjectMutationOptions = Apollo.BaseMutationOptions<UpdateProjectMutation, UpdateProjectMutationVariables>;
export const DeleteProjectDocument = gql`
    mutation DeleteProject($id: ID!) {
  deleteProject(id: $id) {
    ok
    message
  }
}
    `;
export type DeleteProjectMutationFn = Apollo.MutationFunction<DeleteProjectMutation, DeleteProjectMutationVariables>;

/**
 * __useDeleteProjectMutation__
 *
 * To run a mutation, you first call `useDeleteProjectMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useDeleteProjectMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [deleteProjectMutation, { data, loading, error }] = useDeleteProjectMutation({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function useDeleteProjectMutation(baseOptions?: Apollo.MutationHookOptions<DeleteProjectMutation, DeleteProjectMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<DeleteProjectMutation, DeleteProjectMutationVariables>(DeleteProjectDocument, options);
      }
export type DeleteProjectMutationHookResult = ReturnType<typeof useDeleteProjectMutation>;
export type DeleteProjectMutationResult = Apollo.MutationResult<DeleteProjectMutation>;
export type DeleteProjectMutationOptions = Apollo.BaseMutationOptions<DeleteProjectMutation, DeleteProjectMutationVariables>;
export const BulkDeleteProjectsDocument = gql`
    mutation BulkDeleteProjects($ids: [ID!]!) {
  bulkDeleteProjects(ids: $ids) {
    ok
    message
  }
}
    `;
export type BulkDeleteProjectsMutationFn = Apollo.MutationFunction<BulkDeleteProjectsMutation, BulkDeleteProjectsMutationVariables>;

/**
 * __useBulkDeleteProjectsMutation__
 *
 * To run a mutation, you first call `useBulkDeleteProjectsMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useBulkDeleteProjectsMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [bulkDeleteProjectsMutation, { data, loading, error }] = useBulkDeleteProjectsMutation({
 *   variables: {
 *      ids: // value for 'ids'
 *   },
 * });
 */
export function useBulkDeleteProjectsMutation(baseOptions?: Apollo.MutationHookOptions<BulkDeleteProjectsMutation, BulkDeleteProjectsMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<BulkDeleteProjectsMutation, BulkDeleteProjectsMutationVariables>(BulkDeleteProjectsDocument, options);
      }
export type BulkDeleteProjectsMutationHookResult = ReturnType<typeof useBulkDeleteProjectsMutation>;
export type BulkDeleteProjectsMutationResult = Apollo.MutationResult<BulkDeleteProjectsMutation>;
export type BulkDeleteProjectsMutationOptions = Apollo.BaseMutationOptions<BulkDeleteProjectsMutation, BulkDeleteProjectsMutationVariables>;
export const RunFinisherDocument = gql`
    mutation RunFinisher($id: ID!) {
  runFinisher(id: $id)
}
    `;
export type RunFinisherMutationFn = Apollo.MutationFunction<RunFinisherMutation, RunFinisherMutationVariables>;

/**
 * __useRunFinisherMutation__
 *
 * To run a mutation, you first call `useRunFinisherMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useRunFinisherMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [runFinisherMutation, { data, loading, error }] = useRunFinisherMutation({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function useRunFinisherMutation(baseOptions?: Apollo.MutationHookOptions<RunFinisherMutation, RunFinisherMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<RunFinisherMutation, RunFinisherMutationVariables>(RunFinisherDocument, options);
      }
export type RunFinisherMutationHookResult = ReturnType<typeof useRunFinisherMutation>;
export type RunFinisherMutationResult = Apollo.MutationResult<RunFinisherMutation>;
export type RunFinisherMutationOptions = Apollo.BaseMutationOptions<RunFinisherMutation, RunFinisherMutationVariables>;
export const PromptPlanDocument = gql`
    mutation PromptPlan($id: ID!, $prompt: String!) {
  promptPlan(id: $id, prompt: $prompt)
}
    `;
export type PromptPlanMutationFn = Apollo.MutationFunction<PromptPlanMutation, PromptPlanMutationVariables>;

/**
 * __usePromptPlanMutation__
 *
 * To run a mutation, you first call `usePromptPlanMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `usePromptPlanMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [promptPlanMutation, { data, loading, error }] = usePromptPlanMutation({
 *   variables: {
 *      id: // value for 'id'
 *      prompt: // value for 'prompt'
 *   },
 * });
 */
export function usePromptPlanMutation(baseOptions?: Apollo.MutationHookOptions<PromptPlanMutation, PromptPlanMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<PromptPlanMutation, PromptPlanMutationVariables>(PromptPlanDocument, options);
      }
export type PromptPlanMutationHookResult = ReturnType<typeof usePromptPlanMutation>;
export type PromptPlanMutationResult = Apollo.MutationResult<PromptPlanMutation>;
export type PromptPlanMutationOptions = Apollo.BaseMutationOptions<PromptPlanMutation, PromptPlanMutationVariables>;
export const WriteProjectFilesDocument = gql`
    mutation WriteProjectFiles($id: ID!, $files: [WriteProjectFileInput!]!) {
  writeProjectFiles(id: $id, files: $files) {
    path
    content
    size
    language
    updatedAt
  }
}
    `;
export type WriteProjectFilesMutationFn = Apollo.MutationFunction<WriteProjectFilesMutation, WriteProjectFilesMutationVariables>;

/**
 * __useWriteProjectFilesMutation__
 *
 * To run a mutation, you first call `useWriteProjectFilesMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useWriteProjectFilesMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [writeProjectFilesMutation, { data, loading, error }] = useWriteProjectFilesMutation({
 *   variables: {
 *      id: // value for 'id'
 *      files: // value for 'files'
 *   },
 * });
 */
export function useWriteProjectFilesMutation(baseOptions?: Apollo.MutationHookOptions<WriteProjectFilesMutation, WriteProjectFilesMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<WriteProjectFilesMutation, WriteProjectFilesMutationVariables>(WriteProjectFilesDocument, options);
      }
export type WriteProjectFilesMutationHookResult = ReturnType<typeof useWriteProjectFilesMutation>;
export type WriteProjectFilesMutationResult = Apollo.MutationResult<WriteProjectFilesMutation>;
export type WriteProjectFilesMutationOptions = Apollo.BaseMutationOptions<WriteProjectFilesMutation, WriteProjectFilesMutationVariables>;
export const ExecutionSecurityReportDocument = gql`
    query ExecutionSecurityReport($executionID: ID!) {
  executionSecurityReport(executionID: $executionID) {
    executionID
    tenantID
    status
    overallScore
    secretsFound
    outdatedDeps
    owaspCoverage
    blockedDeploy
    generatedAt
    findings {
      id
      severity
      ruleID
      category
      path
      line
      summary
      remediation
      detectedAt
    }
  }
}
    `;

/**
 * __useExecutionSecurityReportQuery__
 *
 * To run a query within a React component, call `useExecutionSecurityReportQuery` and pass it any options that fit your needs.
 * When your component renders, `useExecutionSecurityReportQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useExecutionSecurityReportQuery({
 *   variables: {
 *      executionID: // value for 'executionID'
 *   },
 * });
 */
export function useExecutionSecurityReportQuery(baseOptions: Apollo.QueryHookOptions<ExecutionSecurityReportQuery, ExecutionSecurityReportQueryVariables> & ({ variables: ExecutionSecurityReportQueryVariables; skip?: boolean; } | { skip: boolean; }) ) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<ExecutionSecurityReportQuery, ExecutionSecurityReportQueryVariables>(ExecutionSecurityReportDocument, options);
      }
export function useExecutionSecurityReportLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<ExecutionSecurityReportQuery, ExecutionSecurityReportQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<ExecutionSecurityReportQuery, ExecutionSecurityReportQueryVariables>(ExecutionSecurityReportDocument, options);
        }
// @ts-ignore
export function useExecutionSecurityReportSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<ExecutionSecurityReportQuery, ExecutionSecurityReportQueryVariables>): Apollo.UseSuspenseQueryResult<ExecutionSecurityReportQuery, ExecutionSecurityReportQueryVariables>;
export function useExecutionSecurityReportSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ExecutionSecurityReportQuery, ExecutionSecurityReportQueryVariables>): Apollo.UseSuspenseQueryResult<ExecutionSecurityReportQuery | undefined, ExecutionSecurityReportQueryVariables>;
export function useExecutionSecurityReportSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ExecutionSecurityReportQuery, ExecutionSecurityReportQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<ExecutionSecurityReportQuery, ExecutionSecurityReportQueryVariables>(ExecutionSecurityReportDocument, options);
        }
export type ExecutionSecurityReportQueryHookResult = ReturnType<typeof useExecutionSecurityReportQuery>;
export type ExecutionSecurityReportLazyQueryHookResult = ReturnType<typeof useExecutionSecurityReportLazyQuery>;
export type ExecutionSecurityReportSuspenseQueryHookResult = ReturnType<typeof useExecutionSecurityReportSuspenseQuery>;
export type ExecutionSecurityReportQueryResult = Apollo.QueryResult<ExecutionSecurityReportQuery, ExecutionSecurityReportQueryVariables>;
export const DescribeIdeaDocument = gql`
    mutation DescribeIdea($input: DescribeIdeaInput!) {
  describeIdea(input: $input) {
    project {
      id
      name
      idea
      description
      status
      ownerId
      isPublic
      createdAt
      updatedAt
    }
    execution {
      id
      status
      budgetUSD
      reservedUSD
      spentUSD
      stopLossUSD
      promptSummary
      createdAt
      admittedAt
      startedAt
    }
    idea {
      title
      summary
      blueprintID
      blueprintReason
      suggestedBudgetUSD
      tags
      stopLossUSD
      confidence
    }
    costEstimate {
      lowUSD
      medianUSD
      highUSD
      p95USD
      confidence
      basedOnRuns
      caveat
    }
  }
}
    `;
export type DescribeIdeaMutationFn = Apollo.MutationFunction<DescribeIdeaMutation, DescribeIdeaMutationVariables>;

/**
 * __useDescribeIdeaMutation__
 *
 * To run a mutation, you first call `useDescribeIdeaMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useDescribeIdeaMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [describeIdeaMutation, { data, loading, error }] = useDescribeIdeaMutation({
 *   variables: {
 *      input: // value for 'input'
 *   },
 * });
 */
export function useDescribeIdeaMutation(baseOptions?: Apollo.MutationHookOptions<DescribeIdeaMutation, DescribeIdeaMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<DescribeIdeaMutation, DescribeIdeaMutationVariables>(DescribeIdeaDocument, options);
      }
export type DescribeIdeaMutationHookResult = ReturnType<typeof useDescribeIdeaMutation>;
export type DescribeIdeaMutationResult = Apollo.MutationResult<DescribeIdeaMutation>;
export type DescribeIdeaMutationOptions = Apollo.BaseMutationOptions<DescribeIdeaMutation, DescribeIdeaMutationVariables>;
export const RefineIdeaDocument = gql`
    mutation RefineIdea($executionID: ID!, $message: String!) {
  refineIdea(executionID: $executionID, message: $message) {
    project {
      id
      name
    }
    execution {
      id
      status
      spentUSD
      reservedUSD
    }
    idea {
      title
      summary
      blueprintID
      blueprintReason
      confidence
    }
    costEstimate {
      medianUSD
      lowUSD
      highUSD
      confidence
    }
  }
}
    `;
export type RefineIdeaMutationFn = Apollo.MutationFunction<RefineIdeaMutation, RefineIdeaMutationVariables>;

/**
 * __useRefineIdeaMutation__
 *
 * To run a mutation, you first call `useRefineIdeaMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useRefineIdeaMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [refineIdeaMutation, { data, loading, error }] = useRefineIdeaMutation({
 *   variables: {
 *      executionID: // value for 'executionID'
 *      message: // value for 'message'
 *   },
 * });
 */
export function useRefineIdeaMutation(baseOptions?: Apollo.MutationHookOptions<RefineIdeaMutation, RefineIdeaMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<RefineIdeaMutation, RefineIdeaMutationVariables>(RefineIdeaDocument, options);
      }
export type RefineIdeaMutationHookResult = ReturnType<typeof useRefineIdeaMutation>;
export type RefineIdeaMutationResult = Apollo.MutationResult<RefineIdeaMutation>;
export type RefineIdeaMutationOptions = Apollo.BaseMutationOptions<RefineIdeaMutation, RefineIdeaMutationVariables>;
export const WalletDocument = gql`
    query Wallet {
  wallet {
    tenantID
    balanceUSD
    holdUSD
    availableUSD
    lifetimeTopUpUSD
    lifetimeSpendUSD
    updatedAt
  }
}
    `;

/**
 * __useWalletQuery__
 *
 * To run a query within a React component, call `useWalletQuery` and pass it any options that fit your needs.
 * When your component renders, `useWalletQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useWalletQuery({
 *   variables: {
 *   },
 * });
 */
export function useWalletQuery(baseOptions?: Apollo.QueryHookOptions<WalletQuery, WalletQueryVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<WalletQuery, WalletQueryVariables>(WalletDocument, options);
      }
export function useWalletLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<WalletQuery, WalletQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<WalletQuery, WalletQueryVariables>(WalletDocument, options);
        }
// @ts-ignore
export function useWalletSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<WalletQuery, WalletQueryVariables>): Apollo.UseSuspenseQueryResult<WalletQuery, WalletQueryVariables>;
export function useWalletSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<WalletQuery, WalletQueryVariables>): Apollo.UseSuspenseQueryResult<WalletQuery | undefined, WalletQueryVariables>;
export function useWalletSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<WalletQuery, WalletQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<WalletQuery, WalletQueryVariables>(WalletDocument, options);
        }
export type WalletQueryHookResult = ReturnType<typeof useWalletQuery>;
export type WalletLazyQueryHookResult = ReturnType<typeof useWalletLazyQuery>;
export type WalletSuspenseQueryHookResult = ReturnType<typeof useWalletSuspenseQuery>;
export type WalletQueryResult = Apollo.QueryResult<WalletQuery, WalletQueryVariables>;
export const WalletTopUpsDocument = gql`
    query WalletTopUps($limit: Int) {
  walletTopUps(limit: $limit) {
    id
    provider
    amountUSD
    status
    createdAt
    completedAt
  }
}
    `;

/**
 * __useWalletTopUpsQuery__
 *
 * To run a query within a React component, call `useWalletTopUpsQuery` and pass it any options that fit your needs.
 * When your component renders, `useWalletTopUpsQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useWalletTopUpsQuery({
 *   variables: {
 *      limit: // value for 'limit'
 *   },
 * });
 */
export function useWalletTopUpsQuery(baseOptions?: Apollo.QueryHookOptions<WalletTopUpsQuery, WalletTopUpsQueryVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<WalletTopUpsQuery, WalletTopUpsQueryVariables>(WalletTopUpsDocument, options);
      }
export function useWalletTopUpsLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<WalletTopUpsQuery, WalletTopUpsQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<WalletTopUpsQuery, WalletTopUpsQueryVariables>(WalletTopUpsDocument, options);
        }
// @ts-ignore
export function useWalletTopUpsSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<WalletTopUpsQuery, WalletTopUpsQueryVariables>): Apollo.UseSuspenseQueryResult<WalletTopUpsQuery, WalletTopUpsQueryVariables>;
export function useWalletTopUpsSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<WalletTopUpsQuery, WalletTopUpsQueryVariables>): Apollo.UseSuspenseQueryResult<WalletTopUpsQuery | undefined, WalletTopUpsQueryVariables>;
export function useWalletTopUpsSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<WalletTopUpsQuery, WalletTopUpsQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<WalletTopUpsQuery, WalletTopUpsQueryVariables>(WalletTopUpsDocument, options);
        }
export type WalletTopUpsQueryHookResult = ReturnType<typeof useWalletTopUpsQuery>;
export type WalletTopUpsLazyQueryHookResult = ReturnType<typeof useWalletTopUpsLazyQuery>;
export type WalletTopUpsSuspenseQueryHookResult = ReturnType<typeof useWalletTopUpsSuspenseQuery>;
export type WalletTopUpsQueryResult = Apollo.QueryResult<WalletTopUpsQuery, WalletTopUpsQueryVariables>;
export const WalletAvailableProvidersDocument = gql`
    query WalletAvailableProviders {
  walletAvailableProviders {
    name
    label
    isPrimary
  }
}
    `;

/**
 * __useWalletAvailableProvidersQuery__
 *
 * To run a query within a React component, call `useWalletAvailableProvidersQuery` and pass it any options that fit your needs.
 * When your component renders, `useWalletAvailableProvidersQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useWalletAvailableProvidersQuery({
 *   variables: {
 *   },
 * });
 */
export function useWalletAvailableProvidersQuery(baseOptions?: Apollo.QueryHookOptions<WalletAvailableProvidersQuery, WalletAvailableProvidersQueryVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<WalletAvailableProvidersQuery, WalletAvailableProvidersQueryVariables>(WalletAvailableProvidersDocument, options);
      }
export function useWalletAvailableProvidersLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<WalletAvailableProvidersQuery, WalletAvailableProvidersQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<WalletAvailableProvidersQuery, WalletAvailableProvidersQueryVariables>(WalletAvailableProvidersDocument, options);
        }
// @ts-ignore
export function useWalletAvailableProvidersSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<WalletAvailableProvidersQuery, WalletAvailableProvidersQueryVariables>): Apollo.UseSuspenseQueryResult<WalletAvailableProvidersQuery, WalletAvailableProvidersQueryVariables>;
export function useWalletAvailableProvidersSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<WalletAvailableProvidersQuery, WalletAvailableProvidersQueryVariables>): Apollo.UseSuspenseQueryResult<WalletAvailableProvidersQuery | undefined, WalletAvailableProvidersQueryVariables>;
export function useWalletAvailableProvidersSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<WalletAvailableProvidersQuery, WalletAvailableProvidersQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<WalletAvailableProvidersQuery, WalletAvailableProvidersQueryVariables>(WalletAvailableProvidersDocument, options);
        }
export type WalletAvailableProvidersQueryHookResult = ReturnType<typeof useWalletAvailableProvidersQuery>;
export type WalletAvailableProvidersLazyQueryHookResult = ReturnType<typeof useWalletAvailableProvidersLazyQuery>;
export type WalletAvailableProvidersSuspenseQueryHookResult = ReturnType<typeof useWalletAvailableProvidersSuspenseQuery>;
export type WalletAvailableProvidersQueryResult = Apollo.QueryResult<WalletAvailableProvidersQuery, WalletAvailableProvidersQueryVariables>;
export const WalletCreateTopUpDocument = gql`
    mutation WalletCreateTopUp($amountUSD: Float!, $provider: String) {
  walletCreateTopUp(amountUSD: $amountUSD, provider: $provider) {
    url
    sessionID
    provider
  }
}
    `;
export type WalletCreateTopUpMutationFn = Apollo.MutationFunction<WalletCreateTopUpMutation, WalletCreateTopUpMutationVariables>;

/**
 * __useWalletCreateTopUpMutation__
 *
 * To run a mutation, you first call `useWalletCreateTopUpMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useWalletCreateTopUpMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [walletCreateTopUpMutation, { data, loading, error }] = useWalletCreateTopUpMutation({
 *   variables: {
 *      amountUSD: // value for 'amountUSD'
 *      provider: // value for 'provider'
 *   },
 * });
 */
export function useWalletCreateTopUpMutation(baseOptions?: Apollo.MutationHookOptions<WalletCreateTopUpMutation, WalletCreateTopUpMutationVariables>) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useMutation<WalletCreateTopUpMutation, WalletCreateTopUpMutationVariables>(WalletCreateTopUpDocument, options);
      }
export type WalletCreateTopUpMutationHookResult = ReturnType<typeof useWalletCreateTopUpMutation>;
export type WalletCreateTopUpMutationResult = Apollo.MutationResult<WalletCreateTopUpMutation>;
export type WalletCreateTopUpMutationOptions = Apollo.BaseMutationOptions<WalletCreateTopUpMutation, WalletCreateTopUpMutationVariables>;
export const ExecutionSupportBundleDocument = gql`
    query ExecutionSupportBundle($executionID: ID!) {
  executionSupportBundle(executionID: $executionID) {
    executionID
    tenantID
    status
    previewURL
    productionURL
    changedFiles
    patchCount
    gateReport {
      completionScore
      stages {
        name
        status
        issuesCount
      }
    }
    securityReport {
      passRate
      blockedDeploy
      findings {
        severity
        ruleID
        path
        line
        summary
      }
    }
    costReport {
      revenueUSD
      providerCostUSD
      sandboxCostUSD
      storageCostUSD
      deploymentCostUSD
      grossMarginPct
    }
    nextBestAction {
      kind
      title
      reason
      cta
    }
    generatedAt
  }
}
    `;

/**
 * __useExecutionSupportBundleQuery__
 *
 * To run a query within a React component, call `useExecutionSupportBundleQuery` and pass it any options that fit your needs.
 * When your component renders, `useExecutionSupportBundleQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useExecutionSupportBundleQuery({
 *   variables: {
 *      executionID: // value for 'executionID'
 *   },
 * });
 */
export function useExecutionSupportBundleQuery(baseOptions: Apollo.QueryHookOptions<ExecutionSupportBundleQuery, ExecutionSupportBundleQueryVariables> & ({ variables: ExecutionSupportBundleQueryVariables; skip?: boolean; } | { skip: boolean; }) ) {
        const options = {...defaultOptions, ...baseOptions}
        return Apollo.useQuery<ExecutionSupportBundleQuery, ExecutionSupportBundleQueryVariables>(ExecutionSupportBundleDocument, options);
      }
export function useExecutionSupportBundleLazyQuery(baseOptions?: Apollo.LazyQueryHookOptions<ExecutionSupportBundleQuery, ExecutionSupportBundleQueryVariables>) {
          const options = {...defaultOptions, ...baseOptions}
          return Apollo.useLazyQuery<ExecutionSupportBundleQuery, ExecutionSupportBundleQueryVariables>(ExecutionSupportBundleDocument, options);
        }
// @ts-ignore
export function useExecutionSupportBundleSuspenseQuery(baseOptions?: Apollo.SuspenseQueryHookOptions<ExecutionSupportBundleQuery, ExecutionSupportBundleQueryVariables>): Apollo.UseSuspenseQueryResult<ExecutionSupportBundleQuery, ExecutionSupportBundleQueryVariables>;
export function useExecutionSupportBundleSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ExecutionSupportBundleQuery, ExecutionSupportBundleQueryVariables>): Apollo.UseSuspenseQueryResult<ExecutionSupportBundleQuery | undefined, ExecutionSupportBundleQueryVariables>;
export function useExecutionSupportBundleSuspenseQuery(baseOptions?: Apollo.SkipToken | Apollo.SuspenseQueryHookOptions<ExecutionSupportBundleQuery, ExecutionSupportBundleQueryVariables>) {
          const options = baseOptions === Apollo.skipToken ? baseOptions : {...defaultOptions, ...baseOptions}
          return Apollo.useSuspenseQuery<ExecutionSupportBundleQuery, ExecutionSupportBundleQueryVariables>(ExecutionSupportBundleDocument, options);
        }
export type ExecutionSupportBundleQueryHookResult = ReturnType<typeof useExecutionSupportBundleQuery>;
export type ExecutionSupportBundleLazyQueryHookResult = ReturnType<typeof useExecutionSupportBundleLazyQuery>;
export type ExecutionSupportBundleSuspenseQueryHookResult = ReturnType<typeof useExecutionSupportBundleSuspenseQuery>;
export type ExecutionSupportBundleQueryResult = Apollo.QueryResult<ExecutionSupportBundleQuery, ExecutionSupportBundleQueryVariables>;