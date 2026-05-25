import { expect, test, type Page, type Route } from "@playwright/test";

const now = "2026-05-25T09:00:00.000Z";
const userID = "user-e2e";
const projectID = "project-e2e";
const executionID = "exec-e2e";
const workspaceID = "workspace-e2e";

const promptText =
  "Build a client operations portal with projects, invoices, approvals, role-based access and activity dashboard.";

const project = {
  __typename: "Project",
  id: projectID,
  name: "Client operations portal",
  description: "Projects, invoices, approvals and team activity in one responsive portal.",
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
    "data:text/html,%3Chtml%3E%3Cbody%20style%3D%22font-family%3AInter%3Bbackground%3A%23080918%3Bcolor%3Awhite%3B%22%3E%3Ch1%3EClient%20operations%20portal%3C%2Fh1%3E%3Cp%3EProjects%2C%20invoices%20and%20approvals%20are%20live.%3C%2Fp%3E%3C%2Fbody%3E%3C%2Fhtml%3E",
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
    id: "patch-e2e-1",
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
  id: "deploy-e2e",
  tenantID: userID,
  projectID,
  executionID,
  blueprintID: "client-portal",
  target: "vercel",
  environment: "production",
  status: "live",
  providerDeploymentID: "vercel-e2e",
  previewURL: supportBundle.previewURL,
  productionURL: "https://client-ops.ironflyer.test",
  diffHash: "diff-e2e-1234567890",
  artifactHash: "artifact-e2e-1234567890",
  gateSummary: { completion: "passed", security: "passed", margin: "86%" },
  costUSD: 1.9,
  createdAt: now,
  previewReadyAt: now,
  promotedAt: now,
  rolledBackAt: null,
};

const topUps = [
  {
    __typename: "WalletTopUp",
    id: "topup-e2e-1",
    amountUSD: 100,
    status: "completed",
    createdAt: now,
    completedAt: now,
  },
];

const ledgerEntries = [
  {
    __typename: "WalletLedgerEntry",
    id: "ledger-e2e-1",
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
    id: "ledger-e2e-2",
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

const profitGuardDecisions = [
  {
    __typename: "ProfitGuardDecision",
    id: "pg-e2e-1",
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
];

function operationNameFromBody(body: unknown): string {
  if (!body || typeof body !== "object") return "unknown";
  const candidate = body as { operationName?: unknown; query?: unknown };
  if (typeof candidate.operationName === "string" && candidate.operationName) {
    return candidate.operationName;
  }
  if (typeof candidate.query === "string") {
    const match = candidate.query.match(/\b(query|mutation|subscription)\s+([A-Za-z0-9_]+)/);
    if (match?.[2]) return match[2];
  }
  return "unknown";
}

async function fulfill(route: Route, payload: unknown) {
  await route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify(payload),
  });
}

async function mockGraphQL(page: Page) {
  const calls: string[] = [];
  const describeInputs: unknown[] = [];
  const unknownOperations: string[] = [];

  await page.route("**/graphql", async (route) => {
    const body = route.request().postDataJSON() as unknown;
    const op = operationNameFromBody(body);
    calls.push(op);

    if (op === "CurrentUser") {
      await fulfill(route, {
        data: {
          me: {
            __typename: "User",
            id: userID,
            email: "founder@ironflyer.test",
            name: "E2E Founder",
            plan: "team",
            orgId: null,
            telemetryOptOut: false,
            emailVerifiedAt: now,
            createdAt: now,
          },
        },
      });
      return;
    }

    if (op === "Projects") {
      await fulfill(route, { data: { projects: [project] } });
      return;
    }

    if (op === "Project") {
      await fulfill(route, { data: { project } });
      return;
    }

    if (op === "DescribeIdea") {
      const variables = (body as { variables?: { input?: unknown } }).variables;
      describeInputs.push(variables?.input ?? null);
      await fulfill(route, {
        data: {
          describeIdea: {
            __typename: "StudioBootstrap",
            project,
            execution: {
              __typename: "Execution",
              id: executionID,
              status: "running",
              budgetUSD: execution.budgetUSD,
              reservedUSD: execution.reservedUSD,
              spentUSD: 0,
              stopLossUSD: execution.stopLossUSD,
              promptSummary: execution.promptSummary,
              createdAt: execution.createdAt,
              admittedAt: execution.admittedAt,
              startedAt: execution.startedAt,
            },
            idea: {
              __typename: "ParsedIdea",
              title: "Client operations portal",
              summary: "A bilingual client operations portal with billing and approvals.",
              blueprintID: "client-portal",
              blueprintReason: "The request maps to a portal workflow with CRUD, roles and dashboards.",
              suggestedBudgetUSD: 18,
              tags: ["portal", "billing", "approvals"],
              stopLossUSD: 25,
              confidence: 0.92,
            },
            costEstimate: {
              __typename: "CostEstimate",
              lowUSD: 4,
              medianUSD: 8,
              highUSD: 14,
              p95USD: 22,
              confidence: 0.86,
              basedOnRuns: 42,
              caveat: null,
            },
          },
        },
      });
      return;
    }

    if (op === "Executions") {
      await fulfill(route, { data: { executions: [execution] } });
      return;
    }

    if (op === "Execution") {
      await fulfill(route, { data: { execution } });
      return;
    }

    if (op === "ExecutionSupportBundle") {
      await fulfill(route, { data: { executionSupportBundle: supportBundle } });
      return;
    }

    if (op === "Deploys") {
      await fulfill(route, { data: { deploys: [deploy] } });
      return;
    }

    if (op === "Deploy") {
      await fulfill(route, { data: { deploy } });
      return;
    }

    if (op === "PendingDeployApprovals") {
      await fulfill(route, { data: { pendingDeployApprovals: [] } });
      return;
    }

    if (op === "Wallet") {
      await fulfill(route, {
        data: {
          wallet: {
            __typename: "Wallet",
            tenantID: userID,
            balanceUSD: 250,
            holdUSD: 0,
            availableUSD: 250,
            lifetimeTopUpUSD: 500,
            lifetimeSpendUSD: 250,
            updatedAt: now,
          },
        },
      });
      return;
    }

    if (op === "ProjectFiles") {
      await fulfill(route, { data: { projectFiles: files } });
      return;
    }

    if (op === "WalletTopUps") {
      await fulfill(route, { data: { walletTopUps: topUps } });
      return;
    }

    if (op === "ExecutionLedger") {
      await fulfill(route, { data: { executionLedger: ledgerEntries } });
      return;
    }

    if (op === "ProfitGuardDecisions") {
      await fulfill(route, { data: { profitGuardDecisions } });
      return;
    }

    if (op === "WalletCreateTopUp") {
      await fulfill(route, {
        data: {
          walletCreateTopUp: {
            __typename: "CheckoutSession",
            url: "https://checkout.stripe.test/session/e2e",
            sessionID: "stripe-e2e",
          },
        },
      });
      return;
    }

    if (op === "StudioCodePanePatches" || op === "StudioPatches" || op === "Patches") {
      await fulfill(route, { data: { patches } });
      return;
    }

    if (op === "RefineIdea") {
      await fulfill(route, {
        data: {
          refineIdea: {
            __typename: "StudioBootstrap",
            project: { __typename: "Project", id: projectID, name: project.name },
            execution: {
              __typename: "Execution",
              id: executionID,
              status: "succeeded",
              spentUSD: execution.spentUSD,
              reservedUSD: 0,
            },
            idea: {
              __typename: "ParsedIdea",
              title: "Client operations portal",
              summary: "Refined portal workflow.",
              blueprintID: "client-portal",
              blueprintReason: "Matches client operations.",
              confidence: 0.94,
            },
            costEstimate: {
              __typename: "CostEstimate",
              medianUSD: 8,
              lowUSD: 4,
              highUSD: 14,
              confidence: 0.88,
            },
          },
        },
      });
      return;
    }

    unknownOperations.push(op);
    await fulfill(route, { errors: [{ message: `Unhandled operation in E2E mock: ${op}` }] });
  });

  return { calls, describeInputs, unknownOperations };
}

async function expectNoHorizontalOverflow(page: Page) {
  const metrics = await page.evaluate(() => ({
    clientWidth: document.documentElement.clientWidth,
    scrollWidth: document.documentElement.scrollWidth,
    bodyScrollWidth: document.body.scrollWidth,
  }));
  expect(metrics.scrollWidth, JSON.stringify(metrics)).toBeLessThanOrEqual(
    metrics.clientWidth + 1,
  );
  expect(metrics.bodyScrollWidth, JSON.stringify(metrics)).toBeLessThanOrEqual(
    metrics.clientWidth + 1,
  );
}

async function expectVisibleText(page: Page, text: string | RegExp) {
  await expect
    .poll(async () => {
      return page
        .locator("body *")
        .evaluateAll((elements, source) => {
          const payload = source as { kind: "string" | "regex"; value: string };
          const isMatch = (value: string) =>
            payload.kind === "string"
              ? value.includes(payload.value)
              : new RegExp(payload.value).test(value);
          return elements.some((element) => {
            const textContent = element.textContent ?? "";
            if (!isMatch(textContent)) return false;
            const style = window.getComputedStyle(element);
            const rect = element.getBoundingClientRect();
            return (
              style.display !== "none" &&
              style.visibility !== "hidden" &&
              Number(style.opacity) !== 0 &&
              rect.width > 0 &&
              rect.height > 0
            );
          });
        },
        typeof text === "string"
          ? { kind: "string", value: text }
          : { kind: "regex", value: text.source },
      );
    })
    .toBe(true);
}

test.describe("Studio app creation journey", () => {
  test.beforeEach(async ({ context, page }) => {
    await context.addCookies([
      {
        name: "ironflyer_token",
        value: "e2e-token",
        domain: "127.0.0.1",
        path: "/",
        sameSite: "Lax",
      },
    ]);
    await page.addInitScript(() => window.localStorage.clear());
  });

  test("creates an app from a prompt and opens the live studio execution", async ({
    page,
    isMobile,
  }) => {
    const gql = await mockGraphQL(page);

    await page.goto("/studio");
    await expect(page.getByText(/AI Prompt/i)).toBeVisible();
    await expectNoHorizontalOverflow(page);

    await page.getByRole("textbox", { name: /New project prompt/i }).fill(promptText);
    await page.getByRole("button", { name: /Generate/i }).click();

    await expect(page).toHaveURL(/\/p\/project-e2e/);
    await expect
      .poll(() => page.url())
      .toContain("executionID=exec-e2e");
    await expectVisibleText(page, "Client operations portal");
    await expect(page.getByText(/SCORE/i).first()).toBeVisible();
    await expectNoHorizontalOverflow(page);

    expect(gql.describeInputs).toEqual([
      expect.objectContaining({
        text: promptText,
        startImmediately: true,
      }),
    ]);

    await page
      .getByRole("textbox", { name: /Send a message to Ironflyer/i })
      .fill("Add an invoice export flow and mention approvals.");
    await page.getByRole("button", { name: /^Send$/i }).click();
    await expectVisibleText(page, "Refinement queued");
    expect(gql.calls).toContain("RefineIdea");
    await expectNoHorizontalOverflow(page);

    await page.getByRole("tab", { name: /Dashboard/i }).click();
    await expect(page.getByText(/Execution dashboard/i)).toBeVisible();
    await expect(page.getByText(/Gross margin/i).first()).toBeVisible();
    await expectNoHorizontalOverflow(page);

    if (!isMobile) {
      await page.getByRole("tab", { name: /^Code$/i }).click();
      await expect(page.getByText("Dashboard.tsx", { exact: true })).toBeVisible();
      await expectNoHorizontalOverflow(page);

      await page.getByRole("tab", { name: /Patches/i }).click();
      await expect(page.getByText(/Create client portal dashboard/i)).toBeVisible();
      await expectNoHorizontalOverflow(page);
    }

    expect(gql.calls).toContain("DescribeIdea");
    expect(gql.calls).toContain("Execution");
    expect(gql.calls).toContain("ExecutionSupportBundle");
    expect(gql.unknownOperations).toEqual([]);
  });

  test("walks the core post-build surfaces without breaking the app shell", async ({
    page,
    isMobile,
  }) => {
    const gql = await mockGraphQL(page);

    await page.goto(`/p/${projectID}?tab=preview&executionID=${executionID}`);
    await expectVisibleText(page, "Client operations portal");
    await expect(page.getByRole("tab", { name: /Preview/i })).toBeVisible();
    await expectNoHorizontalOverflow(page);

    await page.getByRole("tab", { name: /Dashboard/i }).click();
    await expect(page.getByText(/Execution dashboard/i)).toBeVisible();
    await expectNoHorizontalOverflow(page);

    if (isMobile) {
      await page.getByRole("tab", { name: /^Code$/i }).click();
      await expectVisibleText(page, "Dashboard.tsx");
      await expectNoHorizontalOverflow(page);
    }

    await page.goto(`/execution/${executionID}`);
    await expect(page.getByRole("heading", { name: /Client operations portal/i })).toBeVisible();
    await expect(page.getByText(/Gross margin/i).first()).toBeVisible();
    await expectNoHorizontalOverflow(page);

    await page.getByRole("tab", { name: /Cost/i }).click();
    await expect(page.getByText(/Cost breakdown/i)).toBeVisible();
    await expect(page.getByText(/Wallet rollup/i)).toBeVisible();
    await expectNoHorizontalOverflow(page);

    await page.getByRole("tab", { name: /Ledger/i }).click();
    await expect(page.getByText(/provider_cost/i)).toBeVisible();
    await expect(page.getByText(/sandbox_cost/i)).toBeVisible();
    await expectNoHorizontalOverflow(page);

    await page.getByRole("tab", { name: /ProfitGuard/i }).click();
    await expect(page.getByText(/Budget, margin and risk are inside policy/i)).toBeVisible();
    await expectNoHorizontalOverflow(page);

    await page.getByRole("tab", { name: /Bundle/i }).click();
    await expect(page.getByText(/Gate report/i)).toBeVisible();
    await expect(page.getByText(/Security report/i)).toBeVisible();
    await expectNoHorizontalOverflow(page);

    await page.goto("/wallet");
    await expect(page.getByRole("heading", { name: /^Wallet$/i })).toBeVisible();
    await expect(page.getByText("$250.00").first()).toBeVisible();
    await expect(page.getByText(/Recent top-ups/i)).toBeVisible();
    await expect(page.getByText(/Recent activity/i)).toBeVisible();
    await expectNoHorizontalOverflow(page);

    await page.goto("/deploy");
    await expect(page.getByRole("heading", { name: /^Deploys$/i })).toBeVisible();
    await expect(
      page.getByRole("row", { name: /deploy-e2e project-e2e vercel production/i }),
    ).toBeVisible();
    await expectNoHorizontalOverflow(page);

    await page.goto(`/deploy/${deploy.id}`);
    await expect(page.getByRole("heading", { name: /vercel/i })).toBeVisible();
    await expect(page.getByText(/Open production/i)).toBeVisible();
    await expectNoHorizontalOverflow(page);

    expect(gql.calls).toEqual(
      expect.arrayContaining([
        "Execution",
        "ExecutionLedger",
        "ProfitGuardDecisions",
        "ExecutionSupportBundle",
        "Wallet",
        "WalletTopUps",
        "Deploys",
        "Deploy",
      ]),
    );
    expect(gql.unknownOperations).toEqual([]);
  });
});
