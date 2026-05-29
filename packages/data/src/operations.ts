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

export const APPLY_PATCH = /* GraphQL */ `
  mutation ApplyPatch($patchId: ID!) { applyPatch(patchId: $patchId) { id title state lines } }`;

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
    ledger(filter: $filter) { id executionID entryType direction amountUSD provider billable createdAt }
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
