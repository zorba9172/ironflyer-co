"use client";

// PublishButton (A48) — the top-right CTA in the studio header.
//
// V0 was a single all-or-nothing dialog that fired plan → build →
// promote in lockstep, surfaced no preview URL until live, and lost
// approvals entirely. This rewrite is a thin button + status badge:
// the actual flow lives in <PublishDialog>, which walks the deploy
// plane step by step (plan → build → optional approve → promote →
// live), shows the preview URL the moment buildDeployPreview returns
// it, and surfaces ProfitGuard / policy denials as actionable errors
// with a Retry button.
//
// Button states:
//   disabled    — execution not succeeded OR security gate blocked
//   armed       — lime, says "Publish"
//   in flight   — lime + spinner, says "Publishing…"
//   published   — lime + checkmark, says "Published" (for an
//                 execution that already has a successful production
//                 deploy on file)
//
// The success Snackbar fires when the dialog reaches Live and the
// user closes it; it carries the production URL so the user can
// keep verifying after the dialog disappears.

import {
  CheckRounded,
  OpenInNewRounded,
  RocketLaunchRounded,
} from "@mui/icons-material";
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  IconButton,
  Snackbar,
  Tooltip,
} from "@mui/material";
import { useCallback, useMemo, useState } from "react";
import {
  useDeploysQuery,
  useExecutionSupportBundleQuery,
  type ExecutionCoreFragment,
} from "../../lib/gql/__generated__";
import { tokens } from "../../theme";
import {
  PublishDialog,
  type PublishCompletionInfo,
} from "./PublishDialog";

export interface PublishButtonProps {
  execution: ExecutionCoreFragment;
}

type SnackbarState = {
  open: boolean;
  productionURL: string | null;
};

export function PublishButton({ execution }: PublishButtonProps) {
  const [open, setOpen] = useState(false);
  const [inFlight, setInFlight] = useState(false);
  const [snackbar, setSnackbar] = useState<SnackbarState>({
    open: false,
    productionURL: null,
  });

  // We pull the support bundle to know if the security gate blocked
  // deploy. The bundle is also what feeds the dialog's plan-stage
  // panel, so by the time the user clicks Publish it's usually warm.
  const bundleQuery = useExecutionSupportBundleQuery({
    variables: { executionID: execution.id },
    fetchPolicy: "cache-first",
    skip: execution.status !== "succeeded",
  });
  const bundle = bundleQuery.data?.executionSupportBundle;
  const blockedBySecurity = bundle?.securityReport.blockedDeploy === true;

  // "Already published" detection — surface a different label when a
  // production deploy for this execution already landed. We look at
  // the tenant's recent deploys instead of adding a new bespoke
  // operation, per the no-new-GraphQL constraint.
  const deploysQuery = useDeploysQuery({
    variables: { limit: 50, offset: 0 },
    fetchPolicy: "cache-and-network",
  });
  const publishedDeploy = useMemo(() => {
    const rows = deploysQuery.data?.deploys ?? [];
    return (
      rows.find(
        (d) =>
          d.executionID === execution.id &&
          d.environment === "production" &&
          (d.status === "live" || d.status === "promoted"),
      ) ?? null
    );
  }, [deploysQuery.data, execution.id]);

  const disabled =
    execution.status !== "succeeded" || blockedBySecurity || inFlight;

  const tooltip = (() => {
    if (execution.status !== "succeeded") {
      return "Publish unlocks once the execution succeeds.";
    }
    if (blockedBySecurity) {
      return "Security gate blocked this build — resolve findings in Dashboard first.";
    }
    if (publishedDeploy) {
      return "Already live in production. Click to publish a new deploy.";
    }
    return "Publish to production. ProfitGuard runs the deploy plane.";
  })();

  const onClick = useCallback(() => {
    setOpen(true);
  }, []);

  const onClose = useCallback(() => {
    setOpen(false);
    setInFlight(false);
  }, []);

  const onPublished = useCallback((info: PublishCompletionInfo) => {
    // Publish hit Live — show the snackbar now so even if the user
    // dismisses the dialog the URL is still front-and-centre.
    setSnackbar({ open: true, productionURL: info.productionURL });
    setInFlight(false);
    // Refresh the deploys list so the button flips to "Published".
    void deploysQuery.refetch();
  }, [deploysQuery]);

  // ----------------------------------------------------------------
  const label = (() => {
    if (inFlight) return "Publishing…";
    if (publishedDeploy && !open) return "Published";
    if (execution.status !== "succeeded") return "Publish (awaiting success)";
    return "Publish";
  })();

  const startIcon = inFlight ? (
    <CircularProgress
      size={14}
      thickness={6}
      sx={{ color: tokens.color.text.inverse }}
    />
  ) : publishedDeploy && !open ? (
    <CheckRounded sx={{ fontSize: 16 }} />
  ) : (
    <RocketLaunchRounded sx={{ fontSize: 16 }} />
  );

  // The button is "armed" (lime) when it's a real CTA. When already
  // published we still render lime + check to keep the success
  // signal visible, but tooltip + dialog still allow republish.
  const armed = execution.status === "succeeded" && !blockedBySecurity;

  return (
    <>
      <Tooltip title={tooltip} arrow>
        {/* Tooltip needs a real DOM node; span wraps disabled buttons. */}
        <Box component="span" sx={{ display: "inline-flex" }}>
          <Button
            size="small"
            variant="contained"
            startIcon={startIcon}
            disabled={disabled}
            onClick={onClick}
            sx={{
              bgcolor: armed
                ? tokens.color.accent.violet
                : tokens.color.bg.surfaceRaised,
              color: armed
                ? tokens.color.text.inverse
                : tokens.color.text.muted,
              fontFamily: tokens.font.family,
              fontWeight: 800,
              letterSpacing: 0.2,
              minHeight: 32,
              px: 1.5,
              textTransform: "none",
              "&:hover": {
                bgcolor: armed
                  ? tokens.color.accent.violet
                  : tokens.color.bg.surfaceRaised,
                filter: armed ? "brightness(0.93)" : "none",
              },
              "&.Mui-disabled": {
                bgcolor: tokens.color.bg.surfaceRaised,
                color: tokens.color.text.muted,
              },
            }}
          >
            {label}
          </Button>
        </Box>
      </Tooltip>

      <PublishDialog
        open={open}
        onClose={onClose}
        execution={execution}
        onPublished={(info) => {
          setInFlight(false);
          onPublished(info);
        }}
      />

      <Snackbar
        open={snackbar.open}
        autoHideDuration={8000}
        onClose={() => setSnackbar({ open: false, productionURL: null })}
        anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
      >
        <Alert
          severity="success"
          variant="outlined"
          icon={<CheckRounded sx={{ color: tokens.color.accent.violet }} />}
          onClose={() => setSnackbar({ open: false, productionURL: null })}
          sx={{
            bgcolor: tokens.color.bg.surface,
            borderColor: `${tokens.color.accent.violet}66`,
            color: tokens.color.text.primary,
            "& .MuiAlert-action": { alignItems: "center" },
          }}
          action={
            snackbar.productionURL ? (
              <IconButton
                size="small"
                href={snackbar.productionURL}
                target="_blank"
                rel="noopener noreferrer"
                aria-label="Open production URL"
                sx={{ color: tokens.color.accent.violet }}
              >
                <OpenInNewRounded sx={{ fontSize: 18 }} />
              </IconButton>
            ) : undefined
          }
        >
          {snackbar.productionURL
            ? `Published. Production URL: ${snackbar.productionURL}`
            : "Published."}
        </Alert>
      </Snackbar>
    </>
  );
}
