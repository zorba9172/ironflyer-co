import { createRequire } from "node:module";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";

const require = createRequire(path.resolve("apps/web/package.json"));
const { chromium } = require("@playwright/test");

const BASE_URL = process.env.SCREENSHOT_BASE_URL || "http://127.0.0.1:3004";
const OUT_ROOT =
  process.env.SCREENSHOT_OUT ||
  path.resolve("design-handoff-screenshots", "ironflyer-app-2026-05-26");
const ONLY_FAILED = process.env.SCREENSHOT_ONLY_FAILED === "1";
const NAV_TIMEOUT_MS = Number(process.env.SCREENSHOT_NAV_TIMEOUT_MS || 60000);

const now = "2026-05-26T09:00:00.000Z";
const userID = "user-design";
const projectID = "demo";
const executionID = "demo";
const deployID = "demo";
const workspaceID = "workspace-design";
const promptText =
  "Build a client operations portal with projects, invoices, approvals, role-based access and activity dashboard.";

const routes = [
  { slug: "00-home", path: "/" },
  { slug: "01-product", path: "/product" },
  { slug: "02-solutions", path: "/solutions" },
  { slug: "03-resources", path: "/resources" },
  { slug: "04-enterprise", path: "/enterprise" },
  { slug: "05-login", path: "/login" },
  { slug: "06-signup", path: "/signup" },
  { slug: "07-login-reset", path: "/login/reset" },
  { slug: "08-dashboard", path: "/dashboard" },
  { slug: "09-projects", path: "/projects" },
  { slug: "10-templates", path: "/templates" },
  { slug: "11-pricing", path: "/pricing" },
  { slug: "12-studio", path: "/studio" },
  { slug: "13-studio-demo", path: "/studio/demo" },
  { slug: "14-project-workbench", path: "/p/demo?executionID=demo" },
  { slug: "15-executions", path: "/executions" },
  { slug: "16-execution-index", path: "/execution" },
  { slug: "17-execution-detail", path: "/execution/demo" },
  { slug: "18-execution-security", path: "/execution/demo/security" },
  { slug: "19-deploy", path: "/deploy" },
  { slug: "20-deploy-demo", path: "/deploy/demo" },
  { slug: "21-wallet", path: "/wallet" },
  { slug: "22-wallet-topup", path: "/wallet/topup" },
  { slug: "23-settings", path: "/settings" },
  { slug: "24-operator", path: "/operator" },
];

const viewports = [
  { name: "desktop-1440", width: 1440, height: 1100 },
  { name: "mobile-390", width: 390, height: 844 },
];

const project = {
  __typename: "Project",
  id: projectID,
  name: "Client operations portal",
  description:
    "Projects, invoices, approvals and team activity in one responsive portal.",
  status: "active",
  ownerId: userID,
  isPublic: false,
  idea: promptText,
  createdAt: now,
  updatedAt: now,
};

const execution = {
  __typename: "Execution",
  id: executionID,
  tenantID: userID,
  projectID,
  blueprintID: "client-portal",
  workspaceID,
  status: "succeeded",
  budgetUSD: 18,
  reservedUSD: 0,
  spentUSD: 4.2,
  refundedUSD: 0,
  revenueUSD: 18,
  providerCostUSD: 1.1,
  sandboxCostUSD: 0.7,
  storageCostUSD: 0.1,
  deploymentCostUSD: 0,
  completionScore: 0.94,
  grossMarginPct: 0.86,
  expectedCompletionDelta: 0.06,
  riskScore: 0.08,
  stopLossUSD: 25,
  promptSummary: "Client operations portal",
  failureReason: null,
  metadata: {},
  createdAt: now,
  admittedAt: now,
  startedAt: now,
  endedAt: now,
};

const supportBundle = {
  __typename: "SupportBundle",
  executionID,
  tenantID: userID,
  status: "succeeded",
  previewURL:
    "data:text/html,%3Chtml%3E%3Cbody%20style%3D%22font-family%3AInter%3Bbackground%3A%23080918%3Bcolor%3Awhite%3Bpadding%3A32px%3B%22%3E%3Ch1%3EClient%20operations%20portal%3C%2Fh1%3E%3Cp%3EProjects%2C%20invoices%20and%20approvals%20are%20live.%3C%2Fp%3E%3C%2Fbody%3E%3C%2Fhtml%3E",
  productionURL: null,
  changedFiles: ["src/app/Dashboard.tsx", "src/app/api/invoices.ts"],
  patchCount: 2,
  gateReport: {
    __typename: "GateReport",
    completionScore: 0.94,
    stages: [
      { __typename: "GateStage", name: "Plan", status: "passed", issuesCount: 0 },
      { __typename: "GateStage", name: "Build", status: "passed", issuesCount: 0 },
      { __typename: "GateStage", name: "Review", status: "passed", issuesCount: 0 },
      { __typename: "GateStage", name: "Deploy", status: "passed", issuesCount: 0 },
    ],
  },
  securityReport: {
    __typename: "SupportSecurityReport",
    passRate: 0.98,
    blockedDeploy: false,
    findings: [],
  },
  costReport: {
    __typename: "CostReport",
    revenueUSD: 18,
    providerCostUSD: 1.1,
    sandboxCostUSD: 0.7,
    storageCostUSD: 0.1,
    deploymentCostUSD: 0,
    grossMarginPct: 0.86,
  },
  nextBestAction: {
    __typename: "NextAction",
    kind: "review",
    title: "Review generated preview",
    reason: "The build passed all gates and is ready for product review.",
    cta: "Open preview",
  },
  generatedAt: now,
};

const files = [
  {
    __typename: "ProjectFile",
    path: "src/app/Dashboard.tsx",
    content:
      "export function Dashboard() {\n  return <main>Client operations portal</main>;\n}\n",
    size: 78,
    language: "tsx",
    updatedAt: now,
  },
  {
    __typename: "ProjectFile",
    path: "src/app/api/invoices.ts",
    content: "export const invoices = [];\n",
    size: 27,
    language: "ts",
    updatedAt: now,
  },
];

const patches = [
  {
    __typename: "Patch",
    id: "patch-design-1",
    projectId: projectID,
    title: "Create client portal dashboard",
    summary: "Adds dashboard cards, project table and approval statuses.",
    author: "IronFlyer",
    status: "APPLIED",
    createdAt: now,
    appliedAt: now,
    changes: [
      {
        __typename: "PatchChange",
        op: "UPSERT",
        path: "src/app/Dashboard.tsx",
        anchor: null,
        replacement: null,
        symbol: "Dashboard",
        content: files[0].content,
      },
    ],
  },
];

const deploy = {
  __typename: "Deploy",
  id: deployID,
  tenantID: userID,
  projectID,
  executionID,
  blueprintID: "client-portal",
  target: "vercel",
  environment: "production",
  status: "live",
  providerDeploymentID: "vercel-design",
  previewURL: supportBundle.previewURL,
  productionURL: "https://client-ops.ironflyer.test",
  diffHash: "diff-design-1234567890",
  artifactHash: "artifact-design-1234567890",
  gateSummary: { completion: "passed", security: "passed", margin: "86%" },
  costUSD: 1.9,
  createdAt: now,
  previewReadyAt: now,
  promotedAt: now,
  rolledBackAt: null,
};

const ledgerEntries = [
  {
    __typename: "WalletLedgerEntry",
    id: "ledger-design-1",
    tenantID: userID,
    executionID,
    entryType: "provider_cost",
    direction: "debit",
    amountUSD: 1.1,
    provider: "openai",
    billable: true,
    marginRelevant: true,
    metadata: { model: "gpt-5" },
    createdAt: now,
  },
  {
    __typename: "WalletLedgerEntry",
    id: "ledger-design-2",
    tenantID: userID,
    executionID,
    entryType: "sandbox_cost",
    direction: "debit",
    amountUSD: 0.7,
    provider: "runtime",
    billable: true,
    marginRelevant: true,
    metadata: {},
    createdAt: now,
  },
];

const wallet = {
  __typename: "Wallet",
  tenantID: userID,
  balanceUSD: 250,
  holdUSD: 0,
  availableUSD: 250,
  lifetimeTopUpUSD: 500,
  lifetimeSpendUSD: 250,
  updatedAt: now,
};

const securityReport = {
  __typename: "ExecutionSecurityReport",
  executionID,
  tenantID: userID,
  status: "passed",
  overallScore: 98,
  secretsFound: 0,
  outdatedDeps: 1,
  owaspCoverage: 0.96,
  blockedDeploy: false,
  generatedAt: now,
  findings: [
    {
      __typename: "SecurityFinding",
      id: "finding-design-1",
      severity: "low",
      ruleID: "deps.pin",
      category: "dependency",
      path: "package.json",
      line: 12,
      summary: "Pin one transitive package before production.",
      remediation: "Run the dependency update gate.",
      detectedAt: now,
    },
  ],
};

function operationNameFromBody(body) {
  if (!body || typeof body !== "object") return "unknown";
  if (typeof body.operationName === "string" && body.operationName) return body.operationName;
  if (typeof body.query === "string") {
    const match = body.query.match(/\b(query|mutation|subscription)\s+([A-Za-z0-9_]+)/);
    if (match?.[2]) return match[2];
  }
  return "unknown";
}

async function fulfill(route, payload) {
  await route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify(payload),
  });
}

async function installMocks(page, unknownOperations) {
  await page.route("**/api/ide/sync**", async (route) => {
    if (route.request().method() === "GET") {
      await fulfill(route, { folder: "/home/workspace/projects/demo", files });
      return;
    }
    await fulfill(route, {
      folder: "/home/workspace/projects/demo",
      written: files.length,
      skipped: 0,
      preserved: 0,
      removed: 0,
    });
  });

  await page.route("**/graphql", async (route) => {
    const body = route.request().postDataJSON();
    const op = operationNameFromBody(body);
    const data = responseForOperation(op, body?.variables ?? {});
    if (data) {
      await fulfill(route, { data });
      return;
    }
    unknownOperations.push(op);
    await fulfill(route, { data: fallbackForOperation(op) });
  });
}

function responseForOperation(op, variables) {
  switch (op) {
    case "CurrentUser":
      return {
        me: {
          __typename: "User",
          id: userID,
          email: "founder@ironflyer.test",
          name: "Design Founder",
          plan: "team",
          orgId: null,
          telemetryOptOut: false,
          emailVerifiedAt: now,
          createdAt: now,
        },
      };
    case "Projects":
      return { projects: [project] };
    case "Project":
      return { project: { ...project, id: variables.id || projectID } };
    case "ProjectExecutions":
      return { projectExecutions: [execution] };
    case "Executions":
      return { executions: [execution] };
    case "Execution":
      return { execution: { ...execution, id: variables.id || executionID } };
    case "ExecutionSupportBundle":
      return { executionSupportBundle: supportBundle };
    case "ProjectFiles":
      return { projectFiles: files };
    case "StudioCodePanePatches":
    case "StudioPatches":
    case "Patches":
      return { patches };
    case "Wallet":
      return { wallet };
    case "WalletTopUps":
      return {
        walletTopUps: [
          {
            __typename: "WalletTopUp",
            id: "topup-design-1",
            amountUSD: 100,
            status: "completed",
            createdAt: now,
            completedAt: now,
          },
        ],
      };
    case "ExecutionLedger":
      return { executionLedger: ledgerEntries };
    case "LedgerRollup":
      return {
        ledgerRollup: {
          __typename: "LedgerRollup",
          revenueUSD: 18,
          providerCostUSD: 1.1,
          sandboxCostUSD: 0.7,
          storageCostUSD: 0.1,
          deploymentCostUSD: 0,
          premiumReasoningCostUSD: 0.2,
          refundsUSD: 0,
          platformMarginUSD: 15.9,
          grossMarginPct: 0.86,
        },
      };
    case "ProfitGuardDecisions":
      return {
        profitGuardDecisions: [
          {
            __typename: "ProfitGuardDecision",
            id: "pg-design-1",
            executionID,
            enforcementPoint: "admit",
            decision: "proceed",
            reason: "Budget, margin and risk are inside policy.",
            spentUSD: 0,
            reservedUSD: 18,
            estimatedStepCostUSD: 8,
            expectedCompletionDelta: 0.64,
            expectedMarginPct: 0.86,
            riskScore: 0.08,
            recommendedProvider: "openai",
            createdAt: now,
          },
        ],
      };
    case "ExecutionSecurityReport":
      return { executionSecurityReport: securityReport };
    case "Deploys":
      return { deploys: [deploy] };
    case "Deploy":
      return { deploy: { ...deploy, id: variables.id || deployID } };
    case "PendingDeployApprovals":
    case "OperatorPendingApprovals":
      return { [op === "PendingDeployApprovals" ? "pendingDeployApprovals" : "operatorPendingApprovals"]: [] };
    case "ProfitDashboard":
      return {
        profitDashboard: {
          __typename: "ProfitDashboard",
          windowStart: "2026-05-25T00:00:00.000Z",
          windowEnd: now,
          revenueUSD: 18420,
          providerCostUSD: 2860,
          sandboxCostUSD: 1210,
          otherCostUSD: 390,
          grossProfitUSD: 13960,
          grossMarginPct: 0.76,
          activeExecutions: 18,
          blockedExecutions: 2,
          refundCount: 1,
          topUpRate: 0.64,
        },
      };
    case "ScaleDashboard":
      return {
        scaleDashboard: {
          __typename: "ScaleDashboard",
          activeExecutions: 18,
          queuedExecutions: 4,
          queueWaitSec: 42,
          sandboxCapacity: 64,
          workerUtilizationPct: 0.68,
          scaleHealth: "healthy",
        },
      };
    case "CohortDashboard":
      return {
        cohortDashboard: {
          __typename: "CohortDashboard",
          cohorts: [
            {
              __typename: "CohortRow",
              month: "2026-05",
              newPayingUsers: 42,
              secondExecutionUsers: 29,
              day7RepeatUsers: 24,
              day30RepeatUsers: 18,
              avgSpendUSD: 84,
              grossMarginPct: 0.74,
              completionRate: 0.92,
              refundRate: 0.02,
              supportTicketsPerExec: 0.08,
            },
          ],
        },
      };
    case "BlueprintDashboard":
      return {
        blueprintDashboard: {
          __typename: "BlueprintDashboard",
          blueprints: [
            {
              __typename: "BlueprintDashboardRow",
              blueprintID: "client-portal",
              executions: 128,
              avgRevenueUSD: 18,
              avgCostUSD: 3.4,
              grossMarginPct: 0.81,
              previewSuccess: 0.96,
              refunds: 2,
              repairCount: 7,
              avgCompletionScore: 0.93,
            },
          ],
        },
      };
    case "OperatorAbuseScore":
      return {
        operatorAbuseScore: {
          __typename: "AbuseScore",
          tenantID: userID,
          userID,
          score: 0.08,
          tier: "low",
        },
      };
    case "OperatorScaleSnapshot":
      return {
        operatorScaleSnapshot: {
          __typename: "ScaleSnapshot",
          activeExecutions: 18,
          queuedExecutions: 4,
          sandboxCapacity: 64,
          workerUtilizationPct: 0.68,
        },
      };
    case "OperatorWalletSnapshot":
      return {
        operatorWalletSnapshot: {
          __typename: "OperatorWalletSnapshot",
          tenantID: userID,
          balanceUSD: 250,
          holdUSD: 0,
          lifetimeTopUpUSD: 500,
          lifetimeSpendUSD: 250,
        },
      };
    case "OperatorAuditCursor":
      return {
        operatorAuditCursor: [
          {
            __typename: "AuditEvent",
            id: "audit-design-1",
            timestamp: now,
            action: "workspace.snapshot",
            outcome: "allowed",
            hash: "sha256:design",
          },
        ],
      };
    case "Blueprints":
      return {
        blueprints: [
          {
            __typename: "Blueprint",
            id: "client-portal",
            name: "Client Portal",
            description: "Authenticated portal with dashboard and approvals.",
            category: "operations",
            costPriorUSD: 8,
            expectedTimeToPreviewSec: 180,
            supportedGates: ["security", "cost", "deploy"],
            fileCount: 12,
          },
        ],
      };
    case "BlueprintRanking":
      return {
        blueprintRanking: [
          {
            __typename: "BlueprintStats",
            blueprintID: "client-portal",
            executions: 128,
            previewSuccess: 0.96,
            refunds: 2,
            repairCount: 7,
            avgRevenueUSD: 18,
            avgCostUSD: 3.4,
            grossMarginPct: 0.81,
            avgCompletionScore: 0.93,
            avgTimeToPreviewSec: 178,
          },
        ],
      };
    case "RequestPasswordReset":
      return { requestPasswordReset: true };
    case "SignOut":
      return { signOut: true };
    default:
      return null;
  }
}

function fallbackForOperation(op) {
  const key = op && op !== "unknown" ? op[0].toLowerCase() + op.slice(1) : "ok";
  return { [key]: null };
}

function slugifyPath(routePath) {
  return routePath
    .replace(/^\//, "")
    .replace(/\//g, "__")
    .replace(/[^a-zA-Z0-9._-]+/g, "-") || "home";
}

async function waitForSettled(page) {
  await page.waitForLoadState("domcontentloaded").catch(() => undefined);
  await page.waitForLoadState("networkidle", { timeout: 2500 }).catch(() => undefined);
  await page.waitForTimeout(900);
}

async function captureOne(browser, viewport, routeInfo) {
  const context = await browser.newContext({
    viewport: { width: viewport.width, height: viewport.height },
    deviceScaleFactor: 1,
    reducedMotion: "reduce",
  });
  await context.addCookies([
    {
      name: "ironflyer_token",
      value: "design-token",
      domain: "127.0.0.1",
      path: "/",
      sameSite: "Lax",
    },
  ]);
  const page = await context.newPage();
  const unknownOperations = [];
  const consoleErrors = [];
  page.on("console", (message) => {
    if (message.type() === "error") consoleErrors.push(message.text());
  });
  await page.addInitScript(() => {
    window.localStorage.clear();
    window.sessionStorage.clear();
  });
  await installMocks(page, unknownOperations);

  const url = `${BASE_URL}${routeInfo.path}`;
  const file = `${routeInfo.slug}-${slugifyPath(routeInfo.path)}.png`;
  const dir = path.join(OUT_ROOT, viewport.name);
  const screenshotPath = path.join(dir, file);
  let status = "ok";
  let finalUrl = url;
  let title = "";
  let error = null;
  try {
    try {
      await page.goto(url, { waitUntil: "domcontentloaded", timeout: NAV_TIMEOUT_MS });
    } catch {
      await page.goto(url, { waitUntil: "commit", timeout: 15000 });
    }
    await waitForSettled(page);
    finalUrl = page.url();
    title = await page.title().catch(() => "");
    await page.screenshot({
      path: screenshotPath,
      fullPage: true,
      animations: "disabled",
    });
  } catch (e) {
    status = "error";
    error = e instanceof Error ? e.message : String(e);
    await page.screenshot({
      path: screenshotPath,
      fullPage: true,
      animations: "disabled",
    }).catch(() => undefined);
  } finally {
    await context.close();
  }

  return {
    status,
    route: routeInfo.path,
    slug: routeInfo.slug,
    viewport: viewport.name,
    width: viewport.width,
    height: viewport.height,
    file: `${viewport.name}/${file}`,
    url,
    finalUrl,
    title,
    unknownOperations: [...new Set(unknownOperations)],
    consoleErrors: consoleErrors.slice(0, 8),
    error,
  };
}

function htmlIndex(results) {
  const cards = results
    .map(
      (item) => `
      <article>
        <h2>${item.viewport} · ${item.slug}</h2>
        <p><code>${item.route}</code> · ${item.status}</p>
        <a href="${item.file}"><img src="${item.file}" alt="${item.slug}" loading="lazy"></a>
      </article>`,
    )
    .join("\n");
  return `<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <title>IronFlyer screenshots 2026-05-26</title>
  <style>
    body { margin: 0; background: #050612; color: #f7f4ff; font-family: Inter, system-ui, sans-serif; }
    header { padding: 24px; border-bottom: 1px solid rgba(178,133,255,.16); }
    main { display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: 18px; padding: 24px; }
    article { background: #0c0d20; border: 1px solid rgba(178,133,255,.16); border-radius: 8px; overflow: hidden; }
    h1 { margin: 0 0 8px; font-size: 22px; }
    h2 { margin: 12px 12px 4px; font-size: 13px; }
    p { margin: 0 12px 12px; color: #b9b2d3; font-size: 12px; }
    img { display: block; width: 100%; background: #080918; border-top: 1px solid rgba(178,133,255,.16); }
    code { color: #b56cff; }
  </style>
</head>
<body>
  <header>
    <h1>IronFlyer design screenshot handoff · 2026-05-26</h1>
    <p>${results.length} captures · desktop 1440 and mobile 390 · mocked authenticated product data</p>
  </header>
  <main>${cards}</main>
</body>
</html>`;
}

async function main() {
  for (const viewport of viewports) {
    await mkdir(path.join(OUT_ROOT, viewport.name), { recursive: true });
  }

  let previousManifest = null;
  if (ONLY_FAILED) {
    previousManifest = JSON.parse(
      await readFile(path.join(OUT_ROOT, "manifest.json"), "utf8"),
    );
  }

  const pairs = ONLY_FAILED
    ? previousManifest.errors.map((item) => ({
        viewport: viewports.find((v) => v.name === item.viewport),
        routeInfo: routes.find((r) => r.path === item.route),
      })).filter((item) => item.viewport && item.routeInfo)
    : viewports.flatMap((viewport) =>
        routes.map((routeInfo) => ({ viewport, routeInfo })),
      );

  const browser = await chromium.launch({ headless: true });
  const results = [];
  try {
    for (const { viewport, routeInfo } of pairs) {
      process.stdout.write(`capture ${viewport.name} ${routeInfo.path}\n`);
      results.push(await captureOne(browser, viewport, routeInfo));
    }
  } finally {
    await browser.close();
  }

  const mergedResults = previousManifest
    ? previousManifest.results.map((old) => {
        const next = results.find(
          (item) => item.viewport === old.viewport && item.route === old.route,
        );
        return next ?? old;
      })
    : results;

  const manifest = {
    capturedAt: new Date().toISOString(),
    baseUrl: BASE_URL,
    output: OUT_ROOT,
    viewports,
    routes,
    results: mergedResults,
    errors: mergedResults.filter((r) => r.status !== "ok"),
    unknownOperations: [...new Set(mergedResults.flatMap((r) => r.unknownOperations))],
  };
  await writeFile(path.join(OUT_ROOT, "manifest.json"), JSON.stringify(manifest, null, 2));
  await writeFile(path.join(OUT_ROOT, "index.html"), htmlIndex(mergedResults));

  const failed = manifest.errors.length;
  const unknown = manifest.unknownOperations.length;
  process.stdout.write(
    `done: ${results.length} screenshots, ${failed} failed, ${unknown} unknown GraphQL ops\n${OUT_ROOT}\n`,
  );
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
