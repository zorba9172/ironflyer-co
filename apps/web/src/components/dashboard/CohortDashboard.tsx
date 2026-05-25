"use client";

// CohortDashboard — owns the query + loading/error handling around
// CohortTable. The table is split out so other surfaces can reuse it.

import { Card } from "@mui/material";
import { useMemo } from "react";
import { useCohortDashboardQuery } from "../../lib/gql/__generated__";
import { ErrorPanel, LoadingPanel } from "../cockpit";
import { CohortTable } from "./CohortTable";

export function CohortDashboard() {
  const sinceMonth = useMemo(() => {
    const d = new Date();
    d.setMonth(d.getMonth() - 6);
    d.setDate(1);
    d.setHours(0, 0, 0, 0);
    return d.toISOString();
  }, []);

  const { data, loading, error, refetch } = useCohortDashboardQuery({
    variables: { sinceMonth },
    fetchPolicy: "cache-and-network",
  });

  if (loading && !data) {
    return (
      <Card sx={{ p: 0 }}>
        <LoadingPanel label="Loading cohorts" minHeight={260} />
      </Card>
    );
  }
  if (error) {
    return <ErrorPanel error={error} title="Cohort dashboard unavailable" onRetry={() => void refetch()} />;
  }
  const cohorts = data?.cohortDashboard.cohorts ?? [];
  return <CohortTable cohorts={cohorts} />;
}
