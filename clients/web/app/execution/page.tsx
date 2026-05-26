"use client";

// /execution — entry point to the cockpit shell. The cockpit is
// per-execution at /execution/[id]; this page picks the caller's most
// recent execution and routes to it. When there is no execution yet we
// render the cockpit chrome with an empty-state pointing back to the
// composer so the surface still reads as a cockpit, not a 404.

import { Box } from "@mui/material";
import { useRouter } from "next/navigation";
import { useEffect } from "react";
import {
  EmptyState,
  ErrorPanel,
  LoadingPanel,
  PageHeader,
} from "../../src/components/cockpit";
import { RequireAuth } from "../../src/lib/auth";
import { useExecutionsQuery } from "../../src/lib/gql/__generated__";

export default function ExecutionEntryPage() {
  return (
    <RequireAuth>
      <ExecutionEntryInner />
    </RequireAuth>
  );
}

function ExecutionEntryInner() {
  const router = useRouter();
  const { data, loading, error, refetch } = useExecutionsQuery({
    variables: { limit: 1, offset: 0 },
    fetchPolicy: "cache-and-network",
  });

  const latest = data?.executions?.[0];

  useEffect(() => {
    if (!latest) return;
    router.replace(`/execution/${latest.id}`);
  }, [latest, router]);

  if (loading && !data) {
    return <LoadingPanel label="Loading cockpit" />;
  }
  if (error) {
    return (
      <ErrorPanel
        error={error}
        title="Cockpit unavailable"
        onRetry={() => void refetch()}
      />
    );
  }
  if (latest) {
    // Brief render while the router.replace lands.
    return <LoadingPanel label="Opening latest execution" />;
  }

  return (
    <Box>
      <PageHeader
        title="Execution cockpit"
        description="Every paid finisher run lands here with live cost, gate verdicts, and ledger feed. Start a build to open your first cockpit."
        breadcrumbs={[{ label: "Executions", href: "/executions" }, { label: "Cockpit" }]}
      />
      <EmptyState
        title="No executions yet"
        body="Describe an idea from the home composer and Ironflyer will admit your first paid execution after a ProfitGuard verdict. The cockpit opens automatically the moment it starts."
        cta={{ label: "Start a build", href: "/" }}
      />
    </Box>
  );
}
