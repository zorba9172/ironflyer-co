"use client";

// DeployActionBar — state-aware action buttons rendered on
// /deploy/[id]. The state machine maps deploy.status (× whether an
// approval is needed × whether the caller can approve) onto the
// available actions:
//
//   preview_ready & no approval needed → [Promote to production]
//   preview_ready & approval required  → [Request approval]
//   awaiting_approval                  → [Approve] [Reject]   (decider)
//                                         (informational chip otherwise)
//   approved                           → [Promote to production]
//   live  | promoting                  → [Rollback]
//   rolled_back | cancelled | failed   → (no buttons)
//
// `cancelDeploy` is wired live now (typed mutation from
// operations/deploy.graphql). Cancels a deploy still in
// planned/building/preview_ready/awaiting_approval states; resolvers
// reject the call once the deploy promotes to live.

import { Box, Button, CircularProgress, Stack, Typography } from "@mui/material";
import { useState } from "react";
import { extractErrorMessage } from "../../lib/errors";
import {
  useCancelDeployMutation,
  usePromoteDeployMutation,
  useRequestDeployApprovalMutation,
  useRollbackDeployMutation,
  type DeployCoreFragment,
} from "../../lib/gql/__generated__";
import { tokens } from "../../theme";

export interface DeployActionBarProps {
  deploy: DeployCoreFragment;
  // True when the caller belongs to the platform-operator role and is
  // allowed to decide pending approvals. We do not run a GraphQL check
  // here — the resolver enforces it; the chip is hidden when false to
  // avoid an obviously-non-functional control.
  canApprove: boolean;
  // True when the project's promote policy requires a decided
  // approval before production. When unknown, default true (safer).
  requiresApproval?: boolean;
  onChanged?: () => void;
}

export function DeployActionBar({
  deploy,
  canApprove,
  requiresApproval = true,
  onChanged,
}: DeployActionBarProps) {
  const [error, setError] = useState<string | null>(null);

  const [requestApproval, { loading: requesting }] = useRequestDeployApprovalMutation();
  const [promote, { loading: promoting }] = usePromoteDeployMutation();
  const [rollback, { loading: rollingBack }] = useRollbackDeployMutation();
  const [cancel, { loading: cancelling }] = useCancelDeployMutation();

  const status = deploy.status;
  const id = deploy.id;

  const withGuard = async (fn: () => Promise<unknown>) => {
    setError(null);
    try {
      await fn();
      onChanged?.();
    } catch (e) {
      setError(extractErrorMessage(e));
    }
  };

  const actions: React.ReactNode[] = [];

  if (status === "preview_ready" && requiresApproval) {
    actions.push(
      <Button
        key="request"
        variant="contained"
        color="primary"
        disabled={requesting}
        startIcon={requesting ? <CircularProgress size={14} thickness={6} /> : null}
        onClick={() =>
          void withGuard(() =>
            requestApproval({
              variables: { deployID: id, expiresInMinutes: 30 },
              refetchQueries: ["Deploy", "PendingDeployApprovals"],
            }),
          )
        }
      >
        Request approval
      </Button>,
    );
  }

  if ((status === "preview_ready" && !requiresApproval) || status === "approved") {
    actions.push(
      <Button
        key="promote"
        variant="contained"
        color="primary"
        disabled={promoting}
        startIcon={promoting ? <CircularProgress size={14} thickness={6} /> : null}
        onClick={() =>
          void withGuard(() =>
            promote({ variables: { deployID: id }, refetchQueries: ["Deploy"] }),
          )
        }
      >
        Promote to production
      </Button>,
    );
  }

  if (status === "awaiting_approval") {
    if (canApprove) {
      // Decision happens in the Approvals tab; mirror a hint here.
      actions.push(
        <Typography
          key="decide-hint"
          sx={{ color: tokens.color.text.muted, fontSize: 13 }}
        >
          Use the Approvals tab to approve or reject the pending request.
        </Typography>,
      );
    } else {
      actions.push(
        <Typography
          key="awaiting"
          sx={{ color: tokens.color.text.muted, fontSize: 13 }}
        >
          Awaiting platform-operator decision.
        </Typography>,
      );
    }
  }

  if (status === "live" || status === "promoting") {
    actions.push(
      <Button
        key="rollback"
        variant="outlined"
        disabled={rollingBack}
        startIcon={rollingBack ? <CircularProgress size={14} thickness={6} /> : null}
        onClick={() =>
          void withGuard(() =>
            rollback({
              variables: { deployID: id, reason: "Operator rollback" },
              refetchQueries: ["Deploy"],
            }),
          )
        }
        sx={{
          borderColor: tokens.color.accent.warning,
          color: tokens.color.accent.warning,
          "&:hover": {
            borderColor: tokens.color.accent.warning,
            bgcolor: `${tokens.color.accent.warning}14`,
          },
        }}
      >
        Rollback
      </Button>,
    );
  }

  // Cancel — live cancel for any non-terminal state up through
  // awaiting_approval. The resolver itself enforces the "no cancel
  // after promote" rule.
  if (
    status === "planned" ||
    status === "building" ||
    status === "preview_ready" ||
    status === "awaiting_approval"
  ) {
    actions.push(
      <Button
        key="cancel"
        variant="outlined"
        disabled={cancelling}
        startIcon={cancelling ? <CircularProgress size={14} thickness={6} /> : null}
        onClick={() =>
          void withGuard(() =>
            cancel({
              variables: { deployID: id, reason: "Operator cancel" },
              refetchQueries: ["Deploy", "PendingDeployApprovals"],
            }),
          )
        }
        sx={{
          borderColor: tokens.color.border.strong,
          color: tokens.color.text.secondary,
          "&:hover": {
            borderColor: tokens.color.accent.danger,
            color: tokens.color.accent.danger,
            bgcolor: `${tokens.color.accent.danger}10`,
          },
        }}
      >
        Cancel
      </Button>,
    );
  }

  if (actions.length === 0) {
    actions.push(
      <Typography
        key="terminal"
        sx={{ color: tokens.color.text.muted, fontSize: 13 }}
      >
        Deploy is in a terminal state — no actions available.
      </Typography>,
    );
  }

  return (
    <Box>
      <Stack
        direction="row"
        spacing={1.25}
        flexWrap="wrap"
        sx={{ rowGap: 1 }}
        alignItems="center"
      >
        {actions}
      </Stack>
      {error && (
        <Typography sx={{ mt: 1, color: tokens.color.accent.danger, fontSize: 12.5 }}>
          {error}
        </Typography>
      )}
    </Box>
  );
}
