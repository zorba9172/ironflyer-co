"use client";

// ApprovalsList — list of deploy approvals (pending queue on /deploy
// + per-deploy history on /deploy/[id]). Optionally exposes
// approve/reject inline buttons when canDecide is set.

import {
  Box,
  Button,
  Card,
  CircularProgress,
  Stack,
  Typography,
} from "@mui/material";
import Link from "next/link";
import { useState } from "react";
import { extractErrorMessage } from "../../lib/errors";
import {
  useDecideDeployApprovalMutation,
  type DeployApprovalCoreFragment,
} from "../../lib/gql/__generated__";
import { formatMoney } from "../../lib/format";
import { relativeTime } from "../../lib/relativeTime";
import { tokens } from "../../theme";
import { StatusBadge } from "../cockpit";

export interface ApprovalsListProps {
  approvals: DeployApprovalCoreFragment[];
  // When true, render Approve/Reject buttons inline for pending
  // approvals. The /deploy approval queue uses this; the per-deploy
  // history list does not.
  canDecide?: boolean;
  // Whether each row should link to its deploy page (only useful on
  // the approval queue, not on the per-deploy history view).
  linkToDeploy?: boolean;
}

export function ApprovalsList({
  approvals,
  canDecide = false,
  linkToDeploy = false,
}: ApprovalsListProps) {
  if (approvals.length === 0) {
    return (
      <Box
        sx={{
          border: `1px dashed ${tokens.color.border.subtle}`,
          borderRadius: 1,
          p: 4,
          textAlign: "center",
          color: tokens.color.text.muted,
          fontSize: 13.5,
        }}
      >
        No deploy approvals in this view.
      </Box>
    );
  }
  return (
    <Stack spacing={1}>
      {approvals.map((a) => (
        <ApprovalCard
          key={a.id}
          approval={a}
          canDecide={canDecide}
          linkToDeploy={linkToDeploy}
        />
      ))}
    </Stack>
  );
}

function ApprovalCard({
  approval,
  canDecide,
  linkToDeploy,
}: {
  approval: DeployApprovalCoreFragment;
  canDecide: boolean;
  linkToDeploy: boolean;
}) {
  const [decide, { loading }] = useDecideDeployApprovalMutation();
  const [error, setError] = useState<string | null>(null);

  const isPending = approval.status === "pending";

  const handleDecide = async (approve: boolean) => {
    setError(null);
    try {
      await decide({
        variables: {
          approvalID: approval.id,
          approve,
          note: approve ? "Approved" : "Rejected",
        },
        refetchQueries: ["PendingDeployApprovals", "Deploy"],
      });
    } catch (e) {
      setError(extractErrorMessage(e));
    }
  };

  return (
    <Card sx={{ p: 2 }}>
      <Stack
        direction={{ xs: "column", md: "row" }}
        spacing={1.5}
        justifyContent="space-between"
        alignItems={{ md: "center" }}
      >
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Stack direction="row" spacing={1.25} alignItems="center" flexWrap="wrap">
            <StatusBadge status={approval.status} />
            {linkToDeploy ? (
              <Typography
                component={Link}
                href={`/deploy/${approval.deployID}`}
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 13,
                  color: tokens.color.accent.violet,
                  textDecoration: "none",
                  "&:hover": { textDecoration: "underline" },
                }}
              >
                deploy {shortId(approval.deployID)}
              </Typography>
            ) : (
              <Typography sx={{ fontFamily: tokens.font.mono, fontSize: 13, color: tokens.color.text.secondary }}>
                approval {shortId(approval.id)}
              </Typography>
            )}
            <Typography
              sx={{ color: tokens.color.text.muted, fontFamily: tokens.font.mono, fontSize: 11.5 }}
            >
              requested {relativeTime(approval.requestedAt)} · expires {relativeTime(approval.expiresAt)}
            </Typography>
          </Stack>
          <Stack
            direction="row"
            spacing={2}
            sx={{ mt: 1 }}
            flexWrap="wrap"
            divider={<Box sx={{ width: 1, bgcolor: tokens.color.border.subtle }} />}
          >
            <Typography sx={{ fontFamily: tokens.font.mono, fontSize: 12, color: tokens.color.text.secondary }}>
              cost impact {formatMoney(approval.costImpactUSD)}
            </Typography>
            <Typography sx={{ fontFamily: tokens.font.mono, fontSize: 12, color: tokens.color.text.muted }}>
              diff {shortHash(approval.diffHash)}
            </Typography>
            <Typography sx={{ fontFamily: tokens.font.mono, fontSize: 12, color: tokens.color.text.muted }}>
              artifact {shortHash(approval.artifactHash)}
            </Typography>
          </Stack>
          {approval.decisionNote && (
            <Typography sx={{ mt: 0.75, color: tokens.color.text.secondary, fontSize: 13 }}>
              note: {approval.decisionNote}
            </Typography>
          )}
          {error && (
            <Typography sx={{ mt: 0.75, color: tokens.color.accent.danger, fontSize: 12.5 }}>
              {error}
            </Typography>
          )}
        </Box>
        {canDecide && isPending && (
          <Stack direction="row" spacing={1} alignItems="center">
            <Button
              size="small"
              variant="outlined"
              disabled={loading}
              onClick={() => void handleDecide(false)}
              sx={{
                borderColor: tokens.color.accent.danger,
                color: tokens.color.accent.danger,
                fontFamily: tokens.font.mono,
                "&:hover": {
                  borderColor: tokens.color.accent.danger,
                  bgcolor: `${tokens.color.accent.danger}14`,
                },
              }}
            >
              Reject
            </Button>
            <Button
              size="small"
              variant="contained"
              color="primary"
              disabled={loading}
              onClick={() => void handleDecide(true)}
              startIcon={
                loading ? (
                  <CircularProgress size={12} thickness={6} sx={{ color: tokens.color.text.inverse }} />
                ) : null
              }
            >
              Approve
            </Button>
          </Stack>
        )}
      </Stack>
    </Card>
  );
}

function shortId(id: string): string {
  if (id.length <= 12) return id;
  return `${id.slice(0, 8)}…`;
}
function shortHash(hash: string): string {
  if (!hash) return "—";
  if (hash.length <= 12) return hash;
  return hash.slice(0, 10);
}
