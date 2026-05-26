"use client";

// RunControls — state-aware lifecycle action bar for the studio.
//
// State matrix:
//   created / admitted           → status pill only (no actions yet)
//   running / scoring (alive)    → [Stop] · [Refund disabled] · live pill
//   paused_for_budget            → [Top up & resume] (deep-links to
//                                  /wallet#topup with a return URL)
//   succeeded                    → [Run again] · [Refund]
//   failed / stopped / killed    → [Run again] · [Refund]
//   refunded                     → [Run again] · refunded pill
//
// Compromises documented inline:
//   • Pause / resume mutations do NOT exist in the orchestrator
//     schema yet (no pauseExecution / resumeExecution). For v1 we
//     therefore hide a true Pause button and only ship Stop. The
//     "Top up & resume" affordance for the paused_for_budget bucket
//     is best-effort — when the wallet refills the orchestrator
//     resumes the execution automatically on its own scheduler.
//   • Hot key Cmd+. opens the Stop confirmation dialog (same chord
//     Replit and Bolt use to halt agents).
//
// Stop and Refund modals are owned by src/components/executions/. We
// reuse them rather than duplicating.

import {
  AccountBalanceWalletOutlined,
  ReplayRounded,
  StopRounded,
  UndoOutlined,
} from "@mui/icons-material";
import {
  Box,
  Button,
  CircularProgress,
  Stack,
  Tooltip,
} from "@mui/material";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { RefundExecutionDialog } from "../executions/RefundExecutionDialog";
import { StopExecutionDialog } from "../executions/StopExecutionDialog";
import {
  useCreatePaidExecutionMutation,
  type ExecutionCoreFragment,
} from "../../lib/gql/__generated__";
import { extractErrorMessage, normalizeError } from "../../lib/errors";
import { pushToast } from "../../lib/stores/uiStore";
import { tokens } from "../../theme";

const TERMINAL = new Set(["succeeded", "failed", "stopped", "killed", "refunded"]);
const PRE_RUN = new Set(["created", "admitted"]);

// Heuristic — the FSM does not surface a dedicated `paused_for_budget`
// status today, but the metadata.reason field on a halted-but-not-
// terminal execution sometimes carries one. We treat budget pause as a
// future status and detect it defensively so the button works the
// moment the FSM ships the value.
function isPausedForBudget(execution: ExecutionCoreFragment): boolean {
  if (execution.status === "paused_for_budget") return true;
  if (execution.status !== "paused") return false;
  const meta = (execution.metadata ?? {}) as Record<string, unknown>;
  const reason = String(meta.pauseReason ?? meta.reason ?? "");
  return reason.includes("budget") || reason.includes("funds");
}

export interface RunControlsProps {
  execution: ExecutionCoreFragment;
  projectID: string;
}

export function RunControls({ execution, projectID }: RunControlsProps) {
  const router = useRouter();
  const isTerminal = TERMINAL.has(execution.status);
  const isPreRun = PRE_RUN.has(execution.status);
  const pausedForBudget = isPausedForBudget(execution);

  const [createExec, { loading: starting }] = useCreatePaidExecutionMutation();
  const [stopOpen, setStopOpen] = useState(false);
  const [refundOpen, setRefundOpen] = useState(false);

  // Cmd/Ctrl + . → Stop confirmation. Only when we have something to
  // stop (alive execution) and the user is not typing in an input.
  useEffect(() => {
    if (isTerminal || isPreRun) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key !== "." || !(e.metaKey || e.ctrlKey)) return;
      const tgt = e.target as HTMLElement | null;
      const tag = tgt?.tagName?.toLowerCase();
      if (tag === "input" || tag === "textarea" || tgt?.isContentEditable) return;
      e.preventDefault();
      setStopOpen(true);
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [isTerminal, isPreRun]);

  const onRunAgain = useCallback(async () => {
    if (!isTerminal || starting) return;
    try {
      const res = await createExec({
        variables: {
          input: {
            projectID,
            budgetUSD: execution.budgetUSD,
            stopLossUSD: execution.stopLossUSD ?? execution.budgetUSD,
            promptSummary: execution.promptSummary,
            metadata: { source: "studio.run_again", previousExecutionID: execution.id },
          },
        },
      });
      const id = res.data?.createPaidExecution?.id;
      if (id) {
        pushToast({ message: "New execution queued.", severity: "success" });
        router.refresh();
      }
    } catch (e) {
      const n = normalizeError(e);
      if (
        n.code === "INSUFFICIENT_FUNDS" ||
        n.code === "PAYMENT_REQUIRED" ||
        n.status === 402
      ) {
        pushToast({
          message: "Wallet too low — top up to run again.",
          severity: "warning",
          href: "/wallet",
          actionLabel: "Top up",
        });
        return;
      }
      pushToast({ message: extractErrorMessage(e), severity: "error" });
    }
  }, [createExec, execution, isTerminal, projectID, router, starting]);

  const unusedReserve = Math.max(
    0,
    execution.reservedUSD - execution.spentUSD,
  );

  return (
    <>
      <Stack direction="row" spacing={0.5} sx={{ alignItems: "center" }}>
        {/* Pre-run: status pill only */}
        {isPreRun ? <LivePill label={execution.status} tone="muted" /> : null}

        {/* Paused for budget: top-up affordance */}
        {pausedForBudget ? (
          <Tooltip title="Top up wallet to resume this run" arrow>
            <Button
              size="small"
              onClick={() => router.push(`/wallet?return=/p/${encodeURIComponent(projectID)}`)}
              startIcon={<AccountBalanceWalletOutlined sx={{ fontSize: 16 }} />}
              sx={accentButtonSx}
            >
              Top up &amp; resume
            </Button>
          </Tooltip>
        ) : null}

        {/* Alive: Stop + (refund disabled placeholder) + live pill */}
        {!isTerminal && !isPreRun && !pausedForBudget ? (
          <>
            <Tooltip title="Stop this execution (⌘.)" arrow>
              <span>
                <Button
                  size="small"
                  onClick={() => setStopOpen(true)}
                  startIcon={<StopRounded sx={{ fontSize: 16 }} />}
                  sx={dangerButtonSx}
                >
                  Stop
                </Button>
              </span>
            </Tooltip>
            <Tooltip title="Refund unlocks after the execution terminates" arrow>
              <span>
                <Button
                  size="small"
                  disabled
                  startIcon={<UndoOutlined sx={{ fontSize: 16 }} />}
                  sx={ghostButtonSx}
                >
                  Refund
                </Button>
              </span>
            </Tooltip>
            <LivePill label="live" tone="lime" pulse />
          </>
        ) : null}

        {/* Terminal: Run again + Refund */}
        {isTerminal ? (
          <>
            <Tooltip title="Run again with the same prompt" arrow>
              <span>
                <Button
                  size="small"
                  onClick={onRunAgain}
                  disabled={starting}
                  startIcon={
                    starting ? (
                      <CircularProgress
                        size={12}
                        thickness={6}
                        sx={{ color: tokens.color.text.inverse }}
                      />
                    ) : (
                      <ReplayRounded sx={{ fontSize: 16 }} />
                    )
                  }
                  sx={accentButtonSx}
                >
                  Run again
                </Button>
              </span>
            </Tooltip>
            <Tooltip
              title={
                unusedReserve > 0
                  ? `Refund up to ${unusedReserve.toFixed(2)} unused USD`
                  : "Issue a wallet credit-back"
              }
              arrow
            >
              <span>
                <Button
                  size="small"
                  onClick={() => setRefundOpen(true)}
                  startIcon={<UndoOutlined sx={{ fontSize: 16 }} />}
                  sx={ghostButtonSx}
                >
                  Refund
                </Button>
              </span>
            </Tooltip>
            <LivePill label={execution.status} tone="muted" />
          </>
        ) : null}
      </Stack>

      <StopExecutionDialog
        open={stopOpen}
        executionID={execution.id}
        onClose={() => setStopOpen(false)}
        onStopped={() =>
          pushToast({
            message: "Stop requested. The agent will halt at the next safe boundary.",
            severity: "info",
          })
        }
      />

      <RefundExecutionDialog
        open={refundOpen}
        executionID={execution.id}
        unusedReserveUSD={unusedReserve}
        onClose={() => setRefundOpen(false)}
        onRefunded={() => pushToast({ message: "Refund issued.", severity: "success" })}
      />
    </>
  );
}

// LivePill — tiny status indicator used at both ends of the matrix.
function LivePill({
  label,
  tone,
  pulse,
}: {
  label: string;
  tone: "lime" | "muted";
  pulse?: boolean;
}) {
  const color = tone === "lime" ? tokens.color.accent.success : tokens.color.text.secondary;
  return (
    <Box
      sx={{
        alignItems: "center",
        bgcolor: tokens.color.bg.inset,
        border: `1px solid ${tone === "lime" ? `${tokens.color.accent.success}33` : tokens.color.border.subtle}`,
        borderRadius: 0.75,
        color,
        display: "inline-flex",
        fontFamily: tokens.font.mono,
        fontSize: 10.5,
        fontWeight: 800,
        gap: 0.5,
        height: 28,
        letterSpacing: 0.6,
        px: 1,
        textTransform: "uppercase",
      }}
    >
      <Box
        sx={{
          bgcolor: color,
          borderRadius: "50%",
          height: 6,
          width: 6,
          ...(pulse
            ? {
                animation: "ifPulse 1.4s ease-in-out infinite",
                "@keyframes ifPulse": {
                  "0%, 100%": { opacity: 1 },
                  "50%": { opacity: 0.3 },
                },
              }
            : null),
        }}
      />
      {label}
    </Box>
  );
}

// Shared sx — tight 28px-tall buttons in three flavours. Accent uses
// the locked coral → magenta → purple gradient (per the constitution:
// "Do not revive the old lime-first identity").
const accentButtonSx = {
  background: `linear-gradient(100deg, ${tokens.color.accent.coral}, ${tokens.color.brand.magenta} 52%, ${tokens.color.accent.purple})`,
  color: tokens.color.text.primary,
  fontFamily: tokens.font.family,
  fontSize: 12,
  fontWeight: 700,
  height: 28,
  px: 1.25,
  textTransform: "none",
  "&:hover": {
    background: `linear-gradient(100deg, ${tokens.color.accent.coral}, ${tokens.color.brand.magenta} 52%, ${tokens.color.accent.purple})`,
    filter: "brightness(1.06)",
  },
  "&.Mui-disabled": {
    background: tokens.color.bg.surfaceHover,
    color: tokens.color.text.muted,
  },
} as const;

const dangerButtonSx = {
  bgcolor: "transparent",
  border: `1px solid ${tokens.color.accent.danger}66`,
  color: tokens.color.accent.danger,
  fontFamily: tokens.font.family,
  fontSize: 12,
  fontWeight: 700,
  height: 28,
  px: 1.25,
  textTransform: "none",
  "&:hover": {
    bgcolor: `${tokens.color.accent.danger}14`,
    borderColor: tokens.color.accent.danger,
  },
  "&.Mui-disabled": {
    borderColor: tokens.color.border.subtle,
    color: tokens.color.text.muted,
  },
} as const;

const ghostButtonSx = {
  bgcolor: "transparent",
  border: `1px solid ${tokens.color.border.subtle}`,
  color: tokens.color.text.secondary,
  fontFamily: tokens.font.family,
  fontSize: 12,
  fontWeight: 700,
  height: 28,
  px: 1.25,
  textTransform: "none",
  "&:hover": {
    bgcolor: tokens.color.bg.surfaceHover,
    borderColor: tokens.color.border.strong,
    color: tokens.color.text.primary,
  },
  "&.Mui-disabled": {
    borderColor: tokens.color.border.subtle,
    color: tokens.color.text.muted,
  },
} as const;
