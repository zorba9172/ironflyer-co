"use client";

// /executions — flat list of the caller's executions with status
// chip filters and a text filter. The executions query has no
// server-side status filter so we filter client-side on the 50 most
// recent rows; the URL state stays canonical so deep-links land on
// the right filter.

import { SearchRounded } from "@mui/icons-material";
import { Box, InputAdornment, Stack, TextField } from "@mui/material";
import { useRouter, useSearchParams } from "next/navigation";
import { useMemo, useState } from "react";
import {
  EmptyState,
  ErrorPanel,
  LoadingPanel,
  PageHeader,
} from "../../src/components/cockpit";
import {
  ExecutionsTable,
  FilterChips,
  type FilterChipOption,
} from "../../src/components/executions";
import { useExecutionsQuery } from "../../src/lib/gql/__generated__";
import { RequireAuth } from "../../src/lib/auth";
import { tokens } from "../../src/theme";

type StatusFilter = "all" | "running" | "succeeded" | "failed" | "stopped";

const STATUS_OPTIONS: FilterChipOption<StatusFilter>[] = [
  { value: "all", label: "All" },
  { value: "running", label: "Running" },
  { value: "succeeded", label: "Succeeded" },
  { value: "failed", label: "Failed" },
  { value: "stopped", label: "Stopped" },
];

const RUNNING_STATES = new Set(["created", "admitted", "running", "scoring"]);
const SUCCEEDED_STATES = new Set(["succeeded", "success"]);
const FAILED_STATES = new Set(["failed", "killed"]);
const STOPPED_STATES = new Set(["stopped", "refunded"]);

function statusFromString(v: string | null): StatusFilter {
  switch (v) {
    case "running":
    case "succeeded":
    case "failed":
    case "stopped":
      return v;
    default:
      return "all";
  }
}

export default function ExecutionsListPage() {
  return (
    <RequireAuth>
      <ExecutionsInner />
    </RequireAuth>
  );
}

function ExecutionsInner() {
  const router = useRouter();
  const search = useSearchParams();
  const status = statusFromString(search.get("status"));
  const [query, setQuery] = useState("");

  const { data, loading, error, refetch } = useExecutionsQuery({
    variables: { limit: 50, offset: 0 },
    fetchPolicy: "cache-and-network",
    notifyOnNetworkStatusChange: true,
  });

  const setStatus = (next: StatusFilter) => {
    const params = new URLSearchParams(search?.toString());
    if (next === "all") {
      params.delete("status");
    } else {
      params.set("status", next);
    }
    const qs = params.toString();
    router.replace(qs ? `/executions?${qs}` : "/executions");
  };

  const rows = useMemo(() => {
    const list = data?.executions ?? [];
    return list.filter((e) => {
      if (status !== "all") {
        const s = e.status.toLowerCase();
        if (status === "running" && !RUNNING_STATES.has(s)) return false;
        if (status === "succeeded" && !SUCCEEDED_STATES.has(s)) return false;
        if (status === "failed" && !FAILED_STATES.has(s)) return false;
        if (status === "stopped" && !STOPPED_STATES.has(s)) return false;
      }
      if (query.trim()) {
        const q = query.trim().toLowerCase();
        const hay = [e.id, e.blueprintID ?? "", e.promptSummary ?? "", e.status]
          .join(" ")
          .toLowerCase();
        if (!hay.includes(q)) return false;
      }
      return true;
    });
  }, [data, status, query]);

  return (
    <Box>
      <PageHeader
        title="Executions"
        description="Every paid finisher run, newest first. Click into one to see the cost ledger, gate verdicts, and the customer wow-loop bundle."
      />
      <Stack
        direction={{ xs: "column", md: "row" }}
        spacing={1.5}
        sx={{ mb: 2 }}
        alignItems={{ md: "center" }}
        justifyContent="space-between"
      >
        <FilterChips<StatusFilter>
          options={STATUS_OPTIONS}
          value={status}
          onChange={setStatus}
        />
        <TextField
          size="small"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Filter by id, blueprint, or prompt"
          InputProps={{
            startAdornment: (
              <InputAdornment position="start">
                <SearchRounded sx={{ fontSize: 18, color: tokens.color.text.muted }} />
              </InputAdornment>
            ),
          }}
          sx={{
            minWidth: { xs: 0, md: 300 },
            width: { xs: "100%", md: "auto" },
            "& .MuiOutlinedInput-root": {
              bgcolor: tokens.color.bg.surface,
              fontFamily: tokens.font.mono,
              fontSize: 13,
            },
          }}
        />
      </Stack>
      {loading && !data ? (
        <LoadingPanel label="Loading executions" />
      ) : error ? (
        <ErrorPanel error={error} title="Executions unavailable" onRetry={() => void refetch()} />
      ) : (data?.executions ?? []).length === 0 ? (
        <EmptyState
          title="No paid executions yet"
          body="Every paid finisher run lands here. Describe an idea from the home composer and Ironflyer will admit your first execution after a ProfitGuard verdict."
          cta={{ label: "Start a build", href: "/" }}
        />
      ) : (
        <ExecutionsTable rows={rows} />
      )}
    </Box>
  );
}
