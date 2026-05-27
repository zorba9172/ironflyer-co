"use client";

// /deploy — list view + approval queue tabs.
//
// Tabs: [My deploys] [Approval queue] (queue tab only renders when
// the caller is an operator). The list filter chips (All / Pending
// approval / Promoted / Failed) drive a client-side filter over the
// 50 most recent deploys; the deploys query has no server-side
// status filter today.

import { Box, Stack, Tab, Tabs } from "@mui/material";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useMemo } from "react";
import {
  EmptyState,
  ErrorPanel,
  LoadingPanel,
  PageHeader,
} from "../../src/components/cockpit";
import {
  ApprovalsList,
  DeploysTable,
  type DeployRow,
} from "../../src/components/deploy";
import {
  FilterChips,
  type FilterChipOption,
} from "../../src/components/executions";
import { RequireAuth, useAuth } from "../../src/lib/auth";
import {
  useDeploysQuery,
  usePendingDeployApprovalsQuery,
} from "../../src/lib/gql/__generated__";
import { tokens } from "../../src/theme";

type DeployFilter = "all" | "pending_approval" | "promoted" | "failed";

const DEPLOY_FILTERS: FilterChipOption<DeployFilter>[] = [
  { value: "all", label: "All" },
  { value: "pending_approval", label: "Pending approval" },
  { value: "promoted", label: "Promoted" },
  { value: "failed", label: "Failed" },
];

const PENDING_STATES = new Set([
  "preview_ready",
  "awaiting_approval",
  "approved",
]);
const PROMOTED_STATES = new Set(["promoting", "live"]);
const FAILED_STATES = new Set(["failed", "rolled_back", "cancelled"]);

function isOperator(plan?: string | null): boolean {
  if (!plan) return false;
  const p = plan.toLowerCase();
  return p === "operator" || p === "admin" || p === "owner";
}

function tabFromString(v: string | null): "mine" | "queue" {
  return v === "queue" ? "queue" : "mine";
}

function filterFromString(v: string | null): DeployFilter {
  switch (v) {
    case "pending_approval":
    case "promoted":
    case "failed":
      return v;
    default:
      return "all";
  }
}

export default function DeployListPage() {
  return (
    <RequireAuth>
      <Suspense fallback={null}>
        <DeployListInner />
      </Suspense>
    </RequireAuth>
  );
}

function DeployListInner() {
  const router = useRouter();
  const search = useSearchParams();
  const { user } = useAuth();
  const operator = isOperator(user?.plan);

  const requestedTab = tabFromString(search.get("tab"));
  const tab = operator ? requestedTab : "mine";
  const filter = filterFromString(search.get("status"));

  const updateParams = (mut: (p: URLSearchParams) => void) => {
    const params = new URLSearchParams(search?.toString());
    mut(params);
    const qs = params.toString();
    router.replace(qs ? `/deploy?${qs}` : "/deploy");
  };

  const setTab = (next: "mine" | "queue") => {
    updateParams((p) => {
      if (next === "mine") p.delete("tab");
      else p.set("tab", "queue");
    });
  };
  const setFilter = (next: DeployFilter) => {
    updateParams((p) => {
      if (next === "all") p.delete("status");
      else p.set("status", next);
    });
  };

  useEffect(() => {
    if (!operator && requestedTab === "queue") {
      updateParams((p) => p.delete("tab"));
    }
    // updateParams closes over search/router; this effect only needs to
    // normalize the invalid deep-link once when role/search changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [operator, requestedTab]);

  const deploysQ = useDeploysQuery({
    variables: { limit: 50, offset: 0 },
    fetchPolicy: "cache-and-network",
    notifyOnNetworkStatusChange: true,
    skip: tab !== "mine",
  });
  const queueQ = usePendingDeployApprovalsQuery({
    fetchPolicy: "cache-and-network",
    notifyOnNetworkStatusChange: true,
    skip: tab !== "queue" || !operator,
  });

  const rows = useMemo<DeployRow[]>(() => {
    const list = deploysQ.data?.deploys ?? [];
    if (filter === "all") return list;
    return list.filter((d) => {
      const s = d.status.toLowerCase();
      if (filter === "pending_approval") return PENDING_STATES.has(s);
      if (filter === "promoted") return PROMOTED_STATES.has(s);
      if (filter === "failed") return FAILED_STATES.has(s);
      return true;
    });
  }, [deploysQ.data, filter]);

  return (
    <Box>
      <PageHeader
        title="Deploys"
        description="Every plan → preview → approval → promote / rollback transition. Production promotes carry a ProfitGuard verdict and (when policy requires) a decided approval."
      />
      {operator && (
        <Tabs
          value={tab}
          onChange={(_, v) => setTab(v as "mine" | "queue")}
          sx={{
            mb: 2,
            borderBottom: `1px solid ${tokens.color.border.subtle}`,
            "& .MuiTab-root": {
              textTransform: "none",
              fontWeight: 700,
              fontSize: 13.5,
              minHeight: 40,
            },
            "& .MuiTabs-indicator": {
              backgroundColor: tokens.color.accent.violet,
            },
          }}
        >
          <Tab value="mine" label="My deploys" />
          <Tab value="queue" label="Approval queue" />
        </Tabs>
      )}
      {tab === "mine" ? (
        <Stack spacing={2}>
          <FilterChips<DeployFilter>
            options={DEPLOY_FILTERS}
            value={filter}
            onChange={setFilter}
          />
          {deploysQ.loading && !deploysQ.data ? (
            <LoadingPanel label="Loading deploys" />
          ) : deploysQ.error ? (
            <ErrorPanel
              error={deploysQ.error}
              title="Deploys unavailable"
              onRetry={() => void deploysQ.refetch()}
            />
          ) : (deploysQ.data?.deploys ?? []).length === 0 ? (
            <EmptyState
              title="No deploys yet"
              body="Deploys land here once an execution promotes its first artifact. Plan → preview → approval → promote happens automatically when ProfitGuard clears the run."
              cta={{ label: "See executions", href: "/executions" }}
            />
          ) : (
            <DeploysTable rows={rows} />
          )}
        </Stack>
      ) : (
        <Box>
          {queueQ.loading && !queueQ.data ? (
            <LoadingPanel label="Loading approval queue" />
          ) : queueQ.error ? (
            <ErrorPanel
              error={queueQ.error}
              title="Approval queue unavailable"
              onRetry={() => void queueQ.refetch()}
            />
          ) : (
            <ApprovalsList
              approvals={queueQ.data?.pendingDeployApprovals ?? []}
              canDecide={operator}
              linkToDeploy
            />
          )}
        </Box>
      )}
    </Box>
  );
}
