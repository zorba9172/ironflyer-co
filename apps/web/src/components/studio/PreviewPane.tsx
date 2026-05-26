"use client";

// PreviewPane — sandboxed iframe of the live preview URL once the
// orchestrator's executionSupportBundle reports one. While the
// execution is still building we render LoadingPanel with the current
// gate name + progress (X of N).
//
// The bundle query polls every 5s while status is not terminal; we
// stop polling once a previewURL lands AND status is terminal so the
// component is silent on a finished run.

import { OpenInNewRounded, RefreshRounded } from "@mui/icons-material";
import { Box, IconButton, Stack, Tooltip, Typography } from "@mui/material";
import { useEffect, useMemo } from "react";
import { LoadingPanel } from "../cockpit/LoadingPanel";
import { useExecutionSupportBundleQuery } from "../../lib/gql/__generated__";
import { tokens } from "../../theme";

export interface PreviewPaneProps {
  executionID: string;
  executionStatus: string;
}

const TERMINAL = new Set(["succeeded", "failed", "stopped", "killed", "refunded"]);

export function PreviewPane({ executionID, executionStatus }: PreviewPaneProps) {
  const isTerminal = TERMINAL.has(executionStatus);
  const query = useExecutionSupportBundleQuery({
    variables: { executionID },
    pollInterval: isTerminal ? 0 : 5000,
    fetchPolicy: "cache-and-network",
  });

  useEffect(() => {
    // When the execution flips to terminal we want one final refetch
    // so the previewURL / changedFiles / costReport reflect the
    // committed state.
    if (isTerminal) {
      void query.refetch().catch(() => undefined);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isTerminal]);

  const bundle = query.data?.executionSupportBundle;
  const previewURL = bundle?.previewURL ?? null;

  const currentGate = useMemo(() => {
    if (!bundle) return null;
    const stages = bundle.gateReport.stages;
    const passed = stages.filter((s) => s.status === "passed" || s.status === "pass").length;
    const next = stages.find((s) => s.status !== "passed" && s.status !== "pass");
    return {
      total: stages.length,
      passed,
      next,
      name: next?.name ?? (stages.length > 0 ? stages[stages.length - 1].name : null),
      status: next?.status ?? "complete",
    };
  }, [bundle]);

  if (!previewURL) {
    let label: string;
    if (!currentGate) {
      label = query.loading ? "Bootstrapping support bundle…" : "Waiting for the first preview build";
    } else if (currentGate.total === 0) {
      // Planner hasn't produced any gate stages yet — avoid the nonsensical
      // "gate 1 of 0" rendering.
      label = "Planning… waiting for the first gate to publish";
    } else if (!currentGate.next) {
      // All gates passed but no preview URL has landed yet.
      label = `Finalising preview… ${currentGate.passed} of ${currentGate.total} gates passed`;
    } else {
      label = `Building preview… gate ${currentGate.passed + 1} of ${currentGate.total} (${currentGate.name})`;
    }
    return (
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          height: "100%",
          bgcolor: tokens.color.bg.base,
        }}
      >
        <LoadingPanel label={label} minHeight="100%" />
      </Box>
    );
  }

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        height: "100%",
        bgcolor: tokens.color.bg.inset,
      }}
    >
      <Stack
        direction="row"
        spacing={1}
        sx={{
          alignItems: "center",
          borderBottom: `1px solid ${tokens.color.border.subtle}`,
          bgcolor: tokens.color.bg.surface,
          px: 1.5,
          py: 0.75,
        }}
      >
        <Box
          sx={{
            bgcolor: tokens.color.bg.inset,
            border: `1px solid ${tokens.color.border.subtle}`,
            borderRadius: 0.75,
            color: tokens.color.text.secondary,
            flex: 1,
            fontFamily: tokens.font.mono,
            fontSize: 11.5,
            overflow: "hidden",
            px: 1,
            py: 0.5,
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
        >
          {previewURL}
        </Box>
        <Tooltip title="Refresh preview" arrow>
          <IconButton
            size="small"
            onClick={() => void query.refetch()}
            sx={{ color: tokens.color.text.secondary }}
            aria-label="Refresh"
          >
            <RefreshRounded sx={{ fontSize: 16 }} />
          </IconButton>
        </Tooltip>
        <Tooltip title="Open in new tab" arrow>
          <IconButton
            size="small"
            component="a"
            href={previewURL}
            target="_blank"
            rel="noopener noreferrer"
            sx={{ color: tokens.color.text.secondary }}
            aria-label="Open preview in new tab"
          >
            <OpenInNewRounded sx={{ fontSize: 16 }} />
          </IconButton>
        </Tooltip>
      </Stack>
      <Box sx={{ flex: 1, minHeight: 0, position: "relative" }}>
        <Box
          component="iframe"
          src={previewURL}
          // Sandboxing is mandatory per the agent brief — the preview
          // runs untrusted, AI-generated code. We allow scripts, forms
          // and same-origin so a typical web app boots; we do NOT allow
          // top-navigation or popups.
          sandbox="allow-scripts allow-same-origin allow-forms"
          referrerPolicy="no-referrer"
          title="Live preview"
          sx={{
            border: "none",
            display: "block",
            height: "100%",
            width: "100%",
            bgcolor: tokens.color.bg.alabaster,
          }}
        />
        {query.error && (
          <Typography
            sx={{
              position: "absolute",
              right: 8,
              bottom: 8,
              bgcolor: tokens.color.bg.surface,
              border: `1px solid ${tokens.color.accent.danger}55`,
              borderRadius: 0.75,
              color: tokens.color.accent.danger,
              fontFamily: tokens.font.mono,
              fontSize: 11,
              px: 1,
              py: 0.5,
            }}
          >
            bundle refresh failed
          </Typography>
        )}
      </Box>
    </Box>
  );
}
