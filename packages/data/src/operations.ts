// Real orchestrator operations (subset), matching packages/sdk/src/operations.graphql
// and the GraphQL schema. Used by the data hooks when an endpoint is configured.

export const ME = /* GraphQL */ `query Me { me { id email name plan } }`;

export const SIGN_IN = /* GraphQL */ `
  mutation SignIn($input: SignInInput!) {
    signIn(input: $input) { token expiresAt user { id email name plan } }
  }`;

export const SIGN_UP = /* GraphQL */ `
  mutation SignUp($input: SignUpInput!) {
    signUp(input: $input) { token expiresAt user { id email name plan } }
  }`;

export const SIGN_OUT = /* GraphQL */ `mutation SignOut { signOut { ok } }`;

export const PROJECTS = /* GraphQL */ `
  query Projects { projects { id name description status idea updatedAt createdAt } }`;

export const CREATE_PROJECT = /* GraphQL */ `
  mutation CreateProject($input: CreateProjectInput!) {
    createProject(input: $input) { id name status updatedAt }
  }`;

export const WRITE_PROJECT_FILES = /* GraphQL */ `
  mutation WriteProjectFiles($id: ID!, $files: [WriteProjectFileInput!]!) {
    writeProjectFiles(id: $id, files: $files) { path size language }
  }`;

export const DELETE_PROJECT = /* GraphQL */ `
  mutation DeleteProject($id: ID!) { deleteProject(id: $id) { ok } }`;

export const PROJECT_SNAPSHOT = /* GraphQL */ `
  query ProjectSnapshot($id: ID!) { projectSnapshot(id: $id) }`;

export const PROJECT_FILES = /* GraphQL */ `
  query ProjectFiles($id: ID!) { projectFiles(id: $id) { path size language content } }`;

export const GATES = /* GraphQL */ `
  query Gates($projectId: ID!) {
    gates(projectId: $projectId) {
      gate status durationMs notes
      issues { severity message path line }
    }
  }`;

export const RUN_FINISHER = /* GraphQL */ `mutation RunFinisher($id: ID!) { runFinisher(id: $id) }`;

// Re-run a single gate against the current tree without the full finisher loop.
export const RERUN_GATE = /* GraphQL */ `
  mutation RerunGate($input: RerunGateInput!) {
    rerunGate(input: $input) {
      gate status durationMs notes
      issues { severity message path line }
    }
  }`;

export const RUN_PROJECT_SUB = /* GraphQL */ `
  subscription RunProject($projectId: ID!) {
    runProject(projectId: $projectId) {
      __typename
      ... on RunExecutionEvent { ts payload }
      ... on RunGateEvent { ts gate status message }
      ... on RunDoneEvent { ts ok }
      ... on RunErrorEvent { ts code message }
    }
  }`;

export const CREATE_PAID_EXECUTION = /* GraphQL */ `
  mutation CreatePaidExecution($input: CreatePaidExecutionInput!) {
    createPaidExecution(input: $input) { ... on Execution { id status } }
  }`;

export type PatchStatus = 'PROPOSED' | 'APPROVED' | 'APPLIED' | 'REJECTED' | 'ROLLED_BACK' | 'CONFLICTED';
export type PatchStageStatus = 'OPEN' | 'REVIEWED' | 'APPLIED' | 'REJECTED';
export type PatchChangeOp = 'CREATE' | 'REPLACE' | 'DELETE' | 'INSERT_BEFORE' | 'INSERT_AFTER' | 'ANCHOR_REPLACE' | 'SYMBOL_REPLACE';

export interface PatchChange {
  op: PatchChangeOp;
  path: string;
  content?: string | null;
  anchor?: string | null;
  replacement?: string | null;
  symbol?: string | null;
}

export interface PatchConflict {
  path: string;
  base: string;
  ours: string;
  theirs: string;
  markers: string;
}

export interface PatchStage {
  id: string;
  projectId: string;
  name: string;
  description?: string | null;
  patchIds: string[];
  status: PatchStageStatus;
  rejectionReason?: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface Patch {
  id: string;
  projectId: string;
  title?: string | null;
  summary?: string | null;
  author?: string | null;
  status: PatchStatus;
  createdAt: string;
  appliedAt?: string | null;
  changes: PatchChange[];
  stageId?: string | null;
  stage?: PatchStage | null;
  conflicts?: PatchConflict[] | null;
}

const PATCH_FIELDS = /* GraphQL */ `
  id projectId title summary author status createdAt appliedAt stageId
  changes { op path content anchor replacement symbol }
  stage { id projectId name description patchIds status rejectionReason createdAt updatedAt }
  conflicts { path base ours theirs markers }`;

export const PATCHES = /* GraphQL */ `
  query Patches($projectId: ID!) {
    patches(projectId: $projectId) { ${PATCH_FIELDS} }
  }`;

export const PROPOSE_PATCH = /* GraphQL */ `
  mutation ProposePatch($input: ProposePatchInput!) {
    proposePatch(input: $input) { ${PATCH_FIELDS} }
  }`;

export const APPLY_PATCH = /* GraphQL */ `
  mutation ApplyPatch($patchId: ID!) {
    applyPatch(patchId: $patchId) { ${PATCH_FIELDS} }
  }`;

export const ROLLBACK_PATCH = /* GraphQL */ `
  mutation RollbackPatch($patchId: ID!) { rollbackPatch(patchId: $patchId) }`;

// --- Economics: wallet + ledger rollup (Dashboard / Usage real meters) ---
export const WALLET = /* GraphQL */ `
  query Wallet { wallet { balanceUSD holdUSD availableUSD lifetimeTopUpUSD lifetimeSpendUSD updatedAt } }`;

export const LEDGER_ROLLUP = /* GraphQL */ `
  query LedgerRollup($since: DateTime!, $until: DateTime!) {
    ledgerRollup(since: $since, until: $until) {
      revenueUSD providerCostUSD sandboxCostUSD storageCostUSD deploymentCostUSD
      premiumReasoningCostUSD refundsUSD platformMarginUSD grossMarginPct
    }
  }`;

export const LEDGER = /* GraphQL */ `
  query Ledger($filter: LedgerFilter) {
    ledger(filter: $filter) { id executionID entryType direction amountUSD provider metadata billable createdAt }
  }`;

// --- Cost forecast: "what would this cost?" before any wallet hold (H2
// plan-gate). Backed by internal/forecast via the estimateExecutionCost
// resolver; powers the PreflightDialog's real estimate + the CostHUD
// trajectory. All inputs optional — an empty input yields the tenant/global
// baseline estimate.
export const ESTIMATE_EXECUTION_COST = /* GraphQL */ `
  query EstimateExecutionCost($input: EstimateInput!) {
    estimateExecutionCost(input: $input) {
      lowUSD medianUSD highUSD p95USD confidence basedOnRuns caveat
    }
  }`;

// --- Executions for a project (latest run drives the Security report) ---
export const PROJECT_EXECUTIONS = /* GraphQL */ `
  query ProjectExecutions($projectId: ID!, $limit: Int = 20) {
    projectExecutions(projectId: $projectId, limit: $limit) {
      id status budgetUSD spentUSD reservedUSD refundedUSD revenueUSD
      providerCostUSD sandboxCostUSD storageCostUSD deploymentCostUSD
      completionScore grossMarginPct riskScore createdAt startedAt promptSummary
    }
  }`;

// --- AppSec: real scanner report for an execution ---
export const EXECUTION_SECURITY_REPORT = /* GraphQL */ `
  query ExecutionSecurityReport($executionID: ID!) {
    executionSecurityReport(executionID: $executionID) {
      status overallScore secretsFound outdatedDeps blockedDeploy owaspCoverage generatedAt
      findings { id severity ruleID category path line summary remediation detectedAt }
    }
  }`;

// --- Code Health (real code-quality metrics) ---
export const HEALTH_DASHBOARD = /* GraphQL */ `
  query HealthDashboard {
    healthDashboard {
      reuseRate dedupRate deadCodeCount dependencyCycles locPerCapability atlasCapabilityCount
      complexityHistogram { range count }
    }
  }`;

// --- Project trajectory / risk (Performance + Dashboard signal) ---
export const SENTINEL_FORECAST = /* GraphQL */ `
  query SentinelForecast($projectId: ID!) {
    sentinelForecast(projectId: $projectId) {
      level spentUSD hardCapUSD burnRatePerHourUSD extrapolatedTotalUSD
      remainingHeadroomUSD etaCompletionAt
    }
  }`;

// --- Operate › Domains (real deploy-domains plane already in the orchestrator) ---
const DOMAIN_FIELDS = /* GraphQL */ `
  id hostname kind status provider registrar primary verificationStatus
  certificateStatus instructions createdAt verifiedAt liveAt
  dnsRecords { type name value ttl }`;

export const DEPLOY_DOMAINS = /* GraphQL */ `
  query DeployDomains($projectID: ID!) {
    deployDomains(projectID: $projectID) { ${DOMAIN_FIELDS} }
  }`;

export const CONNECT_DEPLOY_DOMAIN = /* GraphQL */ `
  mutation ConnectDeployDomain($input: ConnectDeployDomainInput!) {
    connectDeployDomain(input: $input) { ${DOMAIN_FIELDS} }
  }`;

export const CHECK_DEPLOY_DOMAIN = /* GraphQL */ `
  mutation CheckDeployDomain($id: ID!) {
    checkDeployDomain(id: $id) { ${DOMAIN_FIELDS} }
  }`;

export const SET_PRIMARY_DEPLOY_DOMAIN = /* GraphQL */ `
  mutation SetPrimaryDeployDomain($id: ID!) {
    setPrimaryDeployDomain(id: $id) { ${DOMAIN_FIELDS} }
  }`;

// --- Operate › Data ---
export const APP_DATA_SCHEMA = /* GraphQL */ `
  query AppDataSchema($projectID: ID!) {
    appDataSchema(projectID: $projectID) {
      name rowCount columns { name type nullable primaryKey references }
    }
  }`;
export const APP_TABLE_ROWS = /* GraphQL */ `
  query AppTableRows($projectID: ID!, $table: String!, $limit: Int = 25) {
    appTableRows(projectID: $projectID, table: $table, limit: $limit) { table columns rows total }
  }`;

// --- Test coverage capability (Quality › Coverage). Toggle is per-project;
// the report is the CoverageGate's latest run (empty until it runs). ---
const COVERAGE_FIELDS = /* GraphQL */ `
  projectID enabled overallPct minPct tool generatedAt
  files { path linePct uncovered }`;
export const COVERAGE_REPORT = /* GraphQL */ `
  query CoverageReport($projectID: ID!) {
    coverageReport(projectID: $projectID) { ${COVERAGE_FIELDS} }
  }`;
export const SET_COVERAGE_ENABLED = /* GraphQL */ `
  mutation SetCoverageEnabled($projectID: ID!, $enabled: Boolean!, $minPct: Float) {
    setCoverageEnabled(projectID: $projectID, enabled: $enabled, minPct: $minPct) { ${COVERAGE_FIELDS} }
  }`;

// --- Operate › Users ---
const APP_END_USER_FIELDS = /* GraphQL */ `id email name role status provider lastSeenAt createdAt`;
export const APP_END_USERS = /* GraphQL */ `
  query AppEndUsers($projectID: ID!, $limit: Int = 100, $offset: Int = 0) {
    appEndUsers(projectID: $projectID, limit: $limit, offset: $offset) { ${APP_END_USER_FIELDS} }
  }`;
export const APP_USER_STATS = /* GraphQL */ `
  query AppUserStats($projectID: ID!) {
    appUserStats(projectID: $projectID) { total active7d newThisWeek suspended byRole { role count } }
  }`;
export const SET_APP_USER_ROLE = /* GraphQL */ `
  mutation SetAppUserRole($projectID: ID!, $userID: ID!, $role: String!) {
    setAppUserRole(projectID: $projectID, userID: $userID, role: $role) { ${APP_END_USER_FIELDS} }
  }`;
export const SET_APP_USER_SUSPENDED = /* GraphQL */ `
  mutation SetAppUserSuspended($projectID: ID!, $userID: ID!, $suspended: Boolean!) {
    setAppUserSuspended(projectID: $projectID, userID: $userID, suspended: $suspended) { ${APP_END_USER_FIELDS} }
  }`;

// --- Operate › Analytics ---
export const APP_ANALYTICS = /* GraphQL */ `
  query AppAnalytics($projectID: ID!, $days: Int = 30) {
    appAnalytics(projectID: $projectID, days: $days) {
      rangeDays visitors pageViews sessions bounceRatePct avgSessionSeconds visitorsDeltaPct
      series { ts visitors pageViews sessions }
      topPages { path views avgSeconds }
      topReferrers { source visitors }
      events { name count conversionPct }
    }
  }`;

// --- Operate › Automations ---
const AUTOMATION_FIELDS = /* GraphQL */ `id name triggerKind triggerConfig action enabled lastRunAt lastStatus runs createdAt updatedAt`;
export const AUTOMATIONS = /* GraphQL */ `
  query Automations($projectID: ID!) { automations(projectID: $projectID) { ${AUTOMATION_FIELDS} } }`;
export const CREATE_AUTOMATION = /* GraphQL */ `
  mutation CreateAutomation($input: CreateAutomationInput!) { createAutomation(input: $input) { ${AUTOMATION_FIELDS} } }`;
export const SET_AUTOMATION_ENABLED = /* GraphQL */ `
  mutation SetAutomationEnabled($id: ID!, $enabled: Boolean!) { setAutomationEnabled(id: $id, enabled: $enabled) { ${AUTOMATION_FIELDS} } }`;
export const RUN_AUTOMATION = /* GraphQL */ `
  mutation RunAutomation($id: ID!) { runAutomation(id: $id) { ${AUTOMATION_FIELDS} } }`;
export const DELETE_AUTOMATION = /* GraphQL */ `
  mutation DeleteAutomation($id: ID!) { deleteAutomation(id: $id) { ok } }`;

// --- Operate › API ---
const APP_API_KEY_FIELDS = /* GraphQL */ `id name prefix scopes lastUsedAt createdAt revoked`;
export const APP_API_KEYS = /* GraphQL */ `
  query AppApiKeys($projectID: ID!) { appApiKeys(projectID: $projectID) { ${APP_API_KEY_FIELDS} } }`;
export const APP_ENDPOINTS = /* GraphQL */ `
  query AppEndpoints($projectID: ID!) { appEndpoints(projectID: $projectID) { method path description auth } }`;
export const APP_WEBHOOKS = /* GraphQL */ `
  query AppWebhooks($projectID: ID!) { appWebhooks(projectID: $projectID) { id url events enabled createdAt } }`;
export const CREATE_APP_API_KEY = /* GraphQL */ `
  mutation CreateAppApiKey($input: CreateAppApiKeyInput!) {
    createAppApiKey(input: $input) { secret key { ${APP_API_KEY_FIELDS} } }
  }`;
export const REVOKE_APP_API_KEY = /* GraphQL */ `
  mutation RevokeAppApiKey($id: ID!) { revokeAppApiKey(id: $id) { ${APP_API_KEY_FIELDS} } }`;
export const CREATE_APP_WEBHOOK = /* GraphQL */ `
  mutation CreateAppWebhook($input: CreateAppWebhookInput!) {
    createAppWebhook(input: $input) { id url events enabled createdAt }
  }`;
export const SET_APP_WEBHOOK_ENABLED = /* GraphQL */ `
  mutation SetAppWebhookEnabled($id: ID!, $enabled: Boolean!) {
    setAppWebhookEnabled(id: $id, enabled: $enabled) { id url events enabled createdAt }
  }`;
export const DELETE_APP_WEBHOOK = /* GraphQL */ `
  mutation DeleteAppWebhook($id: ID!) { deleteAppWebhook(id: $id) { ok } }`;

// --- Operate › Marketing (SEO) ---
const APP_SEO_FIELDS = /* GraphQL */ `projectID title description keywords ogImageURL twitterHandle canonicalURL robots sitemapEnabled updatedAt`;
export const APP_SEO_SETTINGS = /* GraphQL */ `
  query AppSeoSettings($projectID: ID!) { appSeoSettings(projectID: $projectID) { ${APP_SEO_FIELDS} } }`;
export const APP_SEO_AUDIT = /* GraphQL */ `
  query AppSeoAudit($projectID: ID!) { appSeoAudit(projectID: $projectID) { score checks { key label passed detail } } }`;
export const UPDATE_APP_SEO_SETTINGS = /* GraphQL */ `
  mutation UpdateAppSeoSettings($projectID: ID!, $input: UpdateAppSeoSettingsInput!) {
    updateAppSeoSettings(projectID: $projectID, input: $input) { ${APP_SEO_FIELDS} }
  }`;

// --- Operate › Settings ---
const APP_SETTINGS_FIELDS = /* GraphQL */ `projectID displayName visibility region supportEmail updatedAt envVars { key valuePreview secret updatedAt }`;
export const APP_SETTINGS = /* GraphQL */ `
  query AppSettings($projectID: ID!) { appSettings(projectID: $projectID) { ${APP_SETTINGS_FIELDS} } }`;
export const UPDATE_APP_SETTINGS = /* GraphQL */ `
  mutation UpdateAppSettings($projectID: ID!, $input: UpdateAppSettingsInput!) {
    updateAppSettings(projectID: $projectID, input: $input) { ${APP_SETTINGS_FIELDS} }
  }`;
export const SET_APP_ENV_VAR = /* GraphQL */ `
  mutation SetAppEnvVar($projectID: ID!, $key: String!, $value: String!, $secret: Boolean = false) {
    setAppEnvVar(projectID: $projectID, key: $key, value: $value, secret: $secret) { ${APP_SETTINGS_FIELDS} }
  }`;
export const DELETE_APP_ENV_VAR = /* GraphQL */ `
  mutation DeleteAppEnvVar($projectID: ID!, $key: String!) {
    deleteAppEnvVar(projectID: $projectID, key: $key) { ${APP_SETTINGS_FIELDS} }
  }`;

// --- Agents + crews (operator-defined agent layer) ---------------------
// The built-in finisher roster (read-only).
export const AGENTS = /* GraphQL */ `
  query Agents { agents { role system capabilities enableThinking } }`;

const CUSTOM_AGENT_FIELDS = /* GraphQL */ `
  id name role description instructions baseRole gateId
  skills tools responsibilities guardrails knowledge
  model autonomy canDelegate handoffTo
  schedule { mode every at weekday trigger enabled }
  updatedAt`;

const CREW_FIELDS = /* GraphQL */ `
  id name goal process memberIds managerId
  schedule { mode every at weekday trigger enabled }
  updatedAt`;

export const CUSTOM_AGENTS = /* GraphQL */ `
  query CustomAgents { customAgents { ${CUSTOM_AGENT_FIELDS} } }`;

export const CREWS = /* GraphQL */ `
  query Crews { crews { ${CREW_FIELDS} } }`;

export const SAVE_CUSTOM_AGENT = /* GraphQL */ `
  mutation SaveCustomAgent($input: SaveCustomAgentInput!) {
    saveCustomAgent(input: $input) { ${CUSTOM_AGENT_FIELDS} }
  }`;

export const DELETE_CUSTOM_AGENT = /* GraphQL */ `
  mutation DeleteCustomAgent($id: ID!) { deleteCustomAgent(id: $id) }`;

export const SAVE_CREW = /* GraphQL */ `
  mutation SaveCrew($input: SaveCrewInput!) {
    saveCrew(input: $input) { ${CREW_FIELDS} }
  }`;

export const DELETE_CREW = /* GraphQL */ `
  mutation DeleteCrew($id: ID!) { deleteCrew(id: $id) }`;

export const RUN_CREW = /* GraphQL */ `
  mutation RunCrew($id: ID!, $projectId: ID!) {
    runCrew(id: $id, projectId: $projectId) {
      crewId process totalCostUsd
      members { agentId name role output tokens costUsd error }
    }
  }`;

// --- Figma import ------------------------------------------------------
// Decompose a Figma file into design tokens + component inventory + rendered
// frames (backed by the orchestrator's Figma REST extractor).
export const IMPORT_FIGMA = /* GraphQL */ `
  mutation ImportFigma($fileKey: String!) {
    importFigma(fileKey: $fileKey) {
      fileKey name
      colors { hex alpha }
      typography { fontFamily fontSize fontWeight lineHeight }
      spacing radii
      components { id name type layoutMode width height children }
      frames { id name width height imageUrl }
    }
  }`;
