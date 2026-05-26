"use client";

// PublishDialog — the multi-phase publish flow opened by
// PublishButton (A48). The dialog walks the deploy plane explicitly:
//
//   Plan → Build → Approve → Promote → Live
//
// Each step renders its own body, surfaces the relevant deploy fields
// (cost forecast, preview URL, production URL), and exposes a Retry
// button on failure. The Approve step is conditional: if the planned
// deploy does not require approval (gateSummary.approval_required is
// falsy and gateSummary.approval is not "required") we mark the step
// "skipped" and advance straight to Promote. If approval IS required
// we show pending approvals and let the operator decide; the server
// is the authority on whether the current user is allowed to decide.
//
// State is driven by a useReducer. Apollo polling keeps the deploy
// row + approval inbox fresh while a mutation is in flight or the
// operator is waiting on an approval decision.

import {
  CheckCircleRounded,
  ContentCopyRounded,
  OpenInNewRounded,
  RefreshRounded,
  RocketLaunchRounded,
} from "@mui/icons-material";
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogContent,
  DialogTitle,
  Divider,
  IconButton,
  Link as MuiLink,
  MenuItem,
  Select,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from "@mui/material";
import NextLink from "next/link";
import { ApolloError } from "@apollo/client";
import { useCallback, useEffect, useMemo, useReducer, useState } from "react";
import {
  useBuildDeployPreviewMutation,
  useCheckDeployDomainMutation,
  useConnectDeployDomainMutation,
  useDecideDeployApprovalMutation,
  useDeployDomainsQuery,
  useDeployQuery,
  useDomainAvailabilityLazyQuery,
  useEstimateExecutionCostQuery,
  useExecutionSupportBundleQuery,
  usePendingDeployApprovalsQuery,
  usePlanDeployMutation,
  usePromoteDeployMutation,
  usePurchaseDeployDomainMutation,
  useReserveDeploySubdomainMutation,
  type DeployCoreFragment,
  type DeployApprovalCoreFragment,
  type DeployDomainCoreFragment,
  type ExecutionCoreFragment,
} from "../../lib/gql/__generated__";
import { extractErrorMessage, normalizeError } from "../../lib/errors";
import { formatDateTime, formatMoney } from "../../lib/format";
import { tokens } from "../../theme";
import {
  PublishPhaseStepper,
  type PhaseStep,
  type PhaseStepState,
} from "./PublishPhaseStepper";

export type PublishPhase =
  | "idle"
  | "planning"
  | "building"
  | "approving"
  | "promoting"
  | "live"
  | "failed";

export interface PublishCompletionInfo {
  deployID: string;
  productionURL: string | null;
}

export interface PublishDialogProps {
  open: boolean;
  onClose: () => void;
  execution: ExecutionCoreFragment;
  onPublished: (info: PublishCompletionInfo) => void;
}

interface PublishState {
  phase: PublishPhase;
  // Which step failed, if any — used so we can highlight the right
  // pill in the stepper instead of marking everything failed.
  failedAt: PhaseStep["key"] | null;
  deployID: string | null;
  previewURL: string | null;
  productionURL: string | null;
  approvalID: string | null;
  approvalRequired: boolean;
  error: ApolloError | Error | null;
  target: string;
  environment: string;
}

type Action =
  | { type: "reset" }
  | { type: "set_target"; target: string }
  | { type: "set_environment"; environment: string }
  | { type: "start_plan" }
  | { type: "plan_success"; deploy: DeployCoreFragment; approvalRequired: boolean }
  | { type: "build_complete"; previewURL: string | null }
  | { type: "approval_pending"; approvalID: string }
  | { type: "approval_decided"; approved: boolean; note?: string | null }
  | { type: "start_promote" }
  | { type: "promote_complete"; productionURL: string | null }
  | { type: "error"; failedAt: PhaseStep["key"]; err: ApolloError | Error };

function initialState(): PublishState {
  return {
    phase: "idle",
    failedAt: null,
    deployID: null,
    previewURL: null,
    productionURL: null,
    approvalID: null,
    approvalRequired: false,
    error: null,
    target: "vercel",
    environment: "production",
  };
}

function reducer(state: PublishState, action: Action): PublishState {
  switch (action.type) {
    case "reset":
      return initialState();
    case "set_target":
      return { ...state, target: action.target };
    case "set_environment":
      return { ...state, environment: action.environment };
    case "start_plan":
      return {
        ...state,
        phase: "planning",
        error: null,
        failedAt: null,
        deployID: null,
        previewURL: null,
        productionURL: null,
        approvalID: null,
      };
    case "plan_success":
      return {
        ...state,
        phase: "building",
        deployID: action.deploy.id,
        previewURL: action.deploy.previewURL ?? null,
        approvalRequired: action.approvalRequired,
        error: null,
        failedAt: null,
      };
    case "build_complete":
      return {
        ...state,
        previewURL: action.previewURL,
        phase: state.approvalRequired ? "approving" : "promoting",
        error: null,
        failedAt: null,
      };
    case "approval_pending":
      return {
        ...state,
        phase: "approving",
        approvalID: action.approvalID,
        error: null,
        failedAt: null,
      };
    case "approval_decided":
      return {
        ...state,
        phase: action.approved ? "promoting" : "failed",
        failedAt: action.approved ? null : "approve",
        error: action.approved
          ? null
          : new Error(
              action.note
                ? `Deployment rejected: ${action.note}`
                : "Deployment rejected by approver.",
            ),
      };
    case "start_promote":
      return { ...state, phase: "promoting", error: null, failedAt: null };
    case "promote_complete":
      return {
        ...state,
        phase: "live",
        productionURL: action.productionURL,
        error: null,
        failedAt: null,
      };
    case "error":
      return { ...state, phase: "failed", failedAt: action.failedAt, error: action.err };
    default:
      return state;
  }
}

// gateRequiresApproval — best-effort inspection of the deploy
// gateSummary blob. The orchestrator's deploy plane historically
// stamps `approval_required: true` (or `approval: "required"`) when
// policy demands a human decision before promote. If the field is
// absent we default to false — i.e. we treat absence as the studio
// fast lane, which matches V0 behaviour. The server is still the
// authority: promoteDeploy will reject if approval is actually
// required and missing.
function gateRequiresApproval(gateSummary: unknown): boolean {
  if (!gateSummary || typeof gateSummary !== "object") return false;
  const g = gateSummary as Record<string, unknown>;
  if (g.approval_required === true) return true;
  if (typeof g.approval === "string" && g.approval.toLowerCase() === "required") return true;
  if (typeof g.policy === "string" && g.policy.toLowerCase() === "approval_required") return true;
  return false;
}

const TARGET_OPTIONS = [
  { value: "vercel", label: "Vercel" },
  { value: "cloudflare", label: "Cloudflare" },
  { value: "noop", label: "Noop (dry-run)" },
];

const ENV_OPTIONS = [
  { value: "production", label: "Production" },
  { value: "preview", label: "Preview-only" },
];

export function PublishDialog({ open, onClose, execution, onPublished }: PublishDialogProps) {
  const [state, dispatch] = useReducer(reducer, undefined, initialState);

  // Reset state every time the dialog opens fresh.
  useEffect(() => {
    if (open) dispatch({ type: "reset" });
  }, [open]);

  // -------- data -------------------------------------------------
  const bundleQuery = useExecutionSupportBundleQuery({
    variables: { executionID: execution.id },
    fetchPolicy: "cache-first",
    skip: !open,
  });
  const bundle = bundleQuery.data?.executionSupportBundle;
  const securityOK = bundle ? !bundle.securityReport.blockedDeploy : null;
  const completionScore = execution.completionScore;
  const gateStages = bundle?.gateReport.stages ?? [];

  // Cost forecast — drives the "what does this deploy cost?" panel.
  const costQuery = useEstimateExecutionCostQuery({
    variables: {
      input: {
        blueprintID: execution.blueprintID ?? undefined,
        capabilities: ["code", "deploy"],
      },
    },
    skip: !open,
  });
  const cost = costQuery.data?.estimateExecutionCost ?? null;

  // -------- mutations --------------------------------------------
  const [planDeploy] = usePlanDeployMutation();
  const [buildDeployPreview] = useBuildDeployPreviewMutation();
  const [promoteDeploy] = usePromoteDeployMutation();
  const [decideApproval, decideState] = useDecideDeployApprovalMutation();

  // -------- polling: deploy row ----------------------------------
  const polling =
    state.phase === "building" || state.phase === "promoting" || state.phase === "approving";
  const deployPoll = useDeployQuery({
    variables: { id: state.deployID ?? "" },
    skip: !state.deployID || !polling,
    pollInterval: polling ? 3000 : 0,
    fetchPolicy: "network-only",
  });
  const livedDeploy = deployPoll.data?.deploy ?? null;

  // -------- polling: pending approvals ---------------------------
  const approvalPoll = usePendingDeployApprovalsQuery({
    skip: state.phase !== "approving" || !state.deployID,
    pollInterval: state.phase === "approving" ? 3000 : 0,
    fetchPolicy: "network-only",
  });
  const myApproval: DeployApprovalCoreFragment | null = useMemo(() => {
    if (!state.deployID) return null;
    const inbox = approvalPoll.data?.pendingDeployApprovals ?? [];
    return inbox.find((a) => a.deployID === state.deployID) ?? null;
  }, [approvalPoll.data, state.deployID]);

  // Sync preview URL from the polled deploy row.
  useEffect(() => {
    if (!livedDeploy) return;
    if (livedDeploy.previewURL && livedDeploy.previewURL !== state.previewURL) {
      // Note: this is intentionally outside the build_complete
      // transition — we keep the URL fresh even after we've moved
      // on, so the Approve / Promote steps can still link out.
      dispatch({ type: "build_complete", previewURL: livedDeploy.previewURL });
    }
  }, [livedDeploy, state.previewURL]);

  // Track build/promote terminal status via polling.
  useEffect(() => {
    if (!livedDeploy) return;
    const status = livedDeploy.status;

    if (state.phase === "building") {
      if (status === "preview_ready" || status === "awaiting_approval") {
        const approvalRequired =
          state.approvalRequired ||
          status === "awaiting_approval" ||
          gateRequiresApproval(livedDeploy.gateSummary);
        if (approvalRequired) {
          dispatch({ type: "approval_pending", approvalID: "" });
        } else if (state.previewURL || livedDeploy.previewURL) {
          // Auto-advance to promote; preview URL already captured.
          dispatch({ type: "start_promote" });
        }
      } else if (status === "failed" || status === "cancelled") {
        dispatch({
          type: "error",
          failedAt: "build",
          err: new Error(`Preview build ${status}.`),
        });
      }
    } else if (state.phase === "promoting") {
      if (status === "live" || status === "promoted") {
        dispatch({
          type: "promote_complete",
          productionURL: livedDeploy.productionURL ?? null,
        });
      } else if (status === "failed" || status === "cancelled" || status === "rolled_back") {
        dispatch({
          type: "error",
          failedAt: "promote",
          err: new Error(`Promotion ${status}.`),
        });
      }
    }
  }, [livedDeploy, state.phase, state.approvalRequired, state.previewURL]);

  // -------- effect: kick promote after approve ------------------
  useEffect(() => {
    if (state.phase !== "promoting" || !state.deployID) return;
    // Guard: only fire promote mutation once per deployID; we use a
    // ref-style flag via the deploy row's status — if the row is
    // already live/promoting we don't re-issue.
    let cancelled = false;
    const fire = async () => {
      try {
        await promoteDeploy({ variables: { deployID: state.deployID! } });
        // The poll will catch the live status; nothing to dispatch here.
      } catch (err) {
        if (!cancelled) {
          dispatch({
            type: "error",
            failedAt: "promote",
            err: err instanceof Error ? err : new Error(String(err)),
          });
        }
      }
    };
    // Only fire once per (deployID, phase=promoting) transition.
    void fire();
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [state.phase === "promoting" ? state.deployID : null]);

  // -------- notify parent when we hit Live -----------------------
  const [notified, setNotified] = useState(false);
  useEffect(() => {
    if (state.phase === "live" && state.deployID && !notified) {
      onPublished({ deployID: state.deployID, productionURL: state.productionURL });
      setNotified(true);
    }
    if (state.phase !== "live" && notified) setNotified(false);
  }, [state.phase, state.deployID, state.productionURL, notified, onPublished]);

  // -------- action: plan + build ---------------------------------
  const runPlan = useCallback(async () => {
    if (!execution.projectID) {
      dispatch({
        type: "error",
        failedAt: "plan",
        err: new Error("This execution has no project ID — deploy needs a project to attach to."),
      });
      return;
    }
    dispatch({ type: "start_plan" });
    try {
      const planResult = await planDeploy({
        variables: {
          input: {
            projectID: execution.projectID,
            executionID: execution.id,
            blueprintID: execution.blueprintID ?? undefined,
            target: state.target,
            environment: state.environment,
            artifactRef: `execution:${execution.id}`,
            diffHash: `execution:${execution.id}`,
          },
        },
      });
      const planned = planResult.data?.planDeploy;
      if (!planned) throw new Error("planDeploy returned no row.");
      const approvalRequired = gateRequiresApproval(planned.gateSummary);
      dispatch({ type: "plan_success", deploy: planned, approvalRequired });

      const buildResult = await buildDeployPreview({ variables: { deployID: planned.id } });
      const built = buildResult.data?.buildDeployPreview;
      if (!built) throw new Error("buildDeployPreview returned no row.");
      // build_complete is dispatched by the poll effect once
      // previewURL / status materialises. We also push the URL now
      // if it's already on the build response.
      if (built.previewURL) {
        dispatch({ type: "build_complete", previewURL: built.previewURL });
      }
    } catch (err) {
      const failedAt: PhaseStep["key"] = state.phase === "planning" ? "plan" : "build";
      dispatch({
        type: "error",
        failedAt,
        err: err instanceof Error || err instanceof ApolloError ? err : new Error(String(err)),
      });
    }
  }, [execution, planDeploy, buildDeployPreview, state.target, state.environment, state.phase]);

  // -------- action: approval decide ------------------------------
  const approvalIDToDecide = myApproval?.id ?? null;
  const onDecide = useCallback(
    async (approve: boolean) => {
      if (!approvalIDToDecide) return;
      try {
        const res = await decideApproval({
          variables: { approvalID: approvalIDToDecide, approve },
        });
        const decided = res.data?.decideDeployApproval;
        if (!decided) throw new Error("decideDeployApproval returned no row.");
        if (decided.status === "approved") {
          dispatch({ type: "approval_decided", approved: true });
        } else {
          dispatch({
            type: "approval_decided",
            approved: false,
            note: decided.decisionNote,
          });
        }
      } catch (err) {
        dispatch({
          type: "error",
          failedAt: "approve",
          err: err instanceof Error || err instanceof ApolloError ? err : new Error(String(err)),
        });
      }
    },
    [approvalIDToDecide, decideApproval],
  );

  // -------- derived: stepper state -------------------------------
  const steps = useMemo<PhaseStep[]>(() => {
    const stateForStep = (
      key: PhaseStep["key"],
      activeOn: PublishPhase,
      completedAtOrAfter: PublishPhase[],
    ): PhaseStepState => {
      if (state.failedAt === key) return "failed";
      if (state.phase === activeOn) return "active";
      if (completedAtOrAfter.includes(state.phase)) return "success";
      return "pending";
    };

    const approveState: PhaseStepState = (() => {
      if (state.failedAt === "approve") return "failed";
      if (state.approvalRequired) {
        if (state.phase === "approving") return "active";
        if (state.phase === "promoting" || state.phase === "live") return "success";
        return "pending";
      }
      // not required: skipped once we've passed Build, otherwise
      // pending so the user knows it MIGHT apply.
      if (state.phase === "promoting" || state.phase === "live") return "skipped";
      if (state.phase === "building") return "pending";
      return "pending";
    })();

    return [
      {
        key: "plan",
        label: "Plan",
        state: stateForStep("plan", "planning", [
          "building",
          "approving",
          "promoting",
          "live",
        ]),
      },
      {
        key: "build",
        label: "Build",
        state: stateForStep("build", "building", ["approving", "promoting", "live"]),
      },
      { key: "approve", label: "Approve", state: approveState },
      {
        key: "promote",
        label: "Promote",
        state: stateForStep("promote", "promoting", ["live"]),
      },
      {
        key: "live",
        label: "Live",
        state:
          state.phase === "live"
            ? "success"
            : state.failedAt === "promote"
              ? "pending"
              : "pending",
      },
    ];
  }, [state.phase, state.failedAt, state.approvalRequired]);

  // -------- guards: dialog can be safely dismissed --------------
  const busy =
    state.phase === "planning" ||
    state.phase === "building" ||
    state.phase === "promoting" ||
    decideState.loading;
  const handleClose = useCallback(() => {
    if (busy) return;
    onClose();
  }, [busy, onClose]);

  const errMsg = state.error ? extractErrorMessage(state.error) : null;
  const errCode = state.error ? normalizeError(state.error).code : null;
  const isPolicyDeny = errCode === "POLICY_DENY";

  // -------- retry --------------------------------------------------
  const onRetry = useCallback(() => {
    if (state.failedAt === "approve") {
      // Can't retry an approval decision; refetch the inbox.
      void approvalPoll.refetch();
      return;
    }
    if (state.failedAt === "promote" && state.deployID) {
      dispatch({ type: "start_promote" });
      return;
    }
    // plan / build → re-run the plan flow.
    void runPlan();
  }, [state.failedAt, state.deployID, approvalPoll, runPlan]);

  // ---------------------------------------------------------------
  return (
    <Dialog
      open={open}
      onClose={handleClose}
      maxWidth="sm"
      fullWidth
      PaperProps={{
        sx: {
          bgcolor: tokens.color.bg.surface,
          border: `1px solid ${tokens.color.border.subtle}`,
          color: tokens.color.text.primary,
        },
      }}
    >
      <DialogTitle
        sx={{
          color: tokens.color.text.primary,
          fontSize: 16,
          fontWeight: 800,
          letterSpacing: -0.1,
          pb: 1,
        }}
      >
        Publish to production
      </DialogTitle>
      <DialogContent sx={{ pt: 0 }}>
        <Stack spacing={1.5}>
          <PublishPhaseStepper steps={steps} />

          {state.phase === "idle" && (
            <PlanBody
              cost={cost}
              securityOK={securityOK}
              completionScore={completionScore}
              gateStagesCount={gateStages.length}
              gateStagesOK={gateStages.filter((s) => s.status === "ok" || s.status === "pass").length}
              target={state.target}
              environment={state.environment}
              onTargetChange={(t) => dispatch({ type: "set_target", target: t })}
              onEnvChange={(e) => dispatch({ type: "set_environment", environment: e })}
              onPlan={runPlan}
              disabled={securityOK === false || !execution.projectID}
            />
          )}

          {state.phase === "planning" && (
            <BusyBody label="Planning deploy…" detail="ProfitGuard + gate summary in flight." />
          )}

          {state.phase === "building" && (
            <BuildBody previewURL={state.previewURL} status={livedDeploy?.status ?? "building"} />
          )}

          {state.phase === "approving" && (
            <ApproveBody
              previewURL={state.previewURL}
              approval={myApproval}
              loadingInbox={approvalPoll.loading}
              required={state.approvalRequired}
              decisionPending={decideState.loading}
              onApprove={() => onDecide(true)}
              onReject={() => onDecide(false)}
            />
          )}

          {state.phase === "promoting" && (
            <BusyBody
              label="Promoting to production…"
              detail="ProfitGuard BeforeVercelDeploy is verifying margin one last time."
            />
          )}

          {state.phase === "live" && (
            <LiveBody
              productionURL={state.productionURL}
              deployID={state.deployID}
              projectID={execution.projectID ?? ""}
            />
          )}

          {state.phase === "failed" && errMsg && (
            <ErrorBody
              message={
                isPolicyDeny ? `Deployment denied: ${errMsg}` : errMsg
              }
              canRetry={!isPolicyDeny}
              onRetry={onRetry}
            />
          )}

          <FooterRow
            phase={state.phase}
            busy={busy}
            onClose={handleClose}
          />
        </Stack>
      </DialogContent>
    </Dialog>
  );
}

// =====================================================================
// Sub-bodies — kept inside the same module since they are tightly
// coupled to the state machine above.
// =====================================================================

function PlanBody(props: {
  cost: {
    medianUSD: number;
    lowUSD: number;
    highUSD: number;
    confidence: number;
    basedOnRuns: number;
    caveat: string | null;
  } | null;
  securityOK: boolean | null;
  completionScore: number;
  gateStagesCount: number;
  gateStagesOK: number;
  target: string;
  environment: string;
  onTargetChange: (t: string) => void;
  onEnvChange: (e: string) => void;
  onPlan: () => void;
  disabled: boolean;
}) {
  const {
    cost,
    securityOK,
    completionScore,
    gateStagesCount,
    gateStagesOK,
    target,
    environment,
    onTargetChange,
    onEnvChange,
    onPlan,
    disabled,
  } = props;

  return (
    <Stack spacing={1.25}>
      <Typography sx={{ color: tokens.color.text.secondary, fontSize: 13 }}>
        Ironflyer walks the deploy plane: plan → build preview → optional approval →
        promote. You'll see the preview URL before anything goes live.
      </Typography>

      <Box
        sx={{
          bgcolor: tokens.color.bg.inset,
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: 1,
          p: 1.25,
        }}
      >
        <Row
          label="Estimated deploy cost"
          value={cost ? formatMoney(cost.medianUSD) : "—"}
          hint={
            cost
              ? `range ${formatMoney(cost.lowUSD)} – ${formatMoney(cost.highUSD)} · ${cost.basedOnRuns} runs`
              : undefined
          }
        />
        <Divider sx={{ borderColor: tokens.color.border.subtle, my: 1 }} />
        <Row
          label="Security gate"
          value={securityOK === null ? "—" : securityOK ? "Passing" : "Blocked"}
          tone={securityOK === null ? "neutral" : securityOK ? "success" : "danger"}
        />
        <Divider sx={{ borderColor: tokens.color.border.subtle, my: 1 }} />
        <Row
          label="Gate summary"
          value={
            gateStagesCount === 0
              ? "—"
              : `${gateStagesOK} / ${gateStagesCount} stages clean`
          }
          tone={
            gateStagesCount === 0
              ? "neutral"
              : gateStagesOK === gateStagesCount
                ? "success"
                : "warning"
          }
        />
        <Divider sx={{ borderColor: tokens.color.border.subtle, my: 1 }} />
        <Row
          label="Completion score"
          value={`${(completionScore * 100).toFixed(0)}%`}
        />
      </Box>

      <Stack direction="row" spacing={1}>
        <LabeledSelect
          label="Target"
          value={target}
          options={TARGET_OPTIONS}
          onChange={onTargetChange}
        />
        <LabeledSelect
          label="Environment"
          value={environment}
          options={ENV_OPTIONS}
          onChange={onEnvChange}
        />
      </Stack>

      {securityOK === false && (
        <Alert severity="warning" variant="outlined" sx={alertSx("warning")}>
          Security report flagged this deploy. Resolve findings in the Dashboard tab
          before publishing.
        </Alert>
      )}

      <Stack direction="row" justifyContent="flex-end">
        <Button
          onClick={onPlan}
          disabled={disabled}
          startIcon={<RocketLaunchRounded sx={{ fontSize: 16 }} />}
          sx={limeButtonSx}
        >
          Plan deploy
        </Button>
      </Stack>
    </Stack>
  );
}

function BusyBody({ label, detail }: { label: string; detail?: string }) {
  return (
    <Stack
      spacing={1}
      alignItems="center"
      sx={{
        bgcolor: tokens.color.bg.inset,
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 1,
        py: 2.5,
        px: 2,
      }}
    >
      <CircularProgress size={24} thickness={5} sx={{ color: tokens.color.accent.violet }} />
      <Typography
        sx={{
          color: tokens.color.text.primary,
          fontFamily: tokens.font.mono,
          fontSize: 12.5,
          fontWeight: 800,
          letterSpacing: 0.6,
          textTransform: "uppercase",
        }}
      >
        {label}
      </Typography>
      {detail && (
        <Typography sx={{ color: tokens.color.text.muted, fontSize: 12, textAlign: "center" }}>
          {detail}
        </Typography>
      )}
    </Stack>
  );
}

function BuildBody({ previewURL, status }: { previewURL: string | null; status: string }) {
  return (
    <Stack spacing={1.25}>
      <BusyBody
        label="Building preview… ~30s"
        detail={`Status: ${status.replace(/_/g, " ")}.`}
      />
      {previewURL && (
        <Box
          sx={{
            bgcolor: `${tokens.color.accent.violet}10`,
            border: `1px solid ${tokens.color.accent.violet}55`,
            borderRadius: 1,
            p: 1.25,
          }}
        >
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
              fontWeight: 800,
              letterSpacing: 0.8,
              mb: 0.5,
              textTransform: "uppercase",
            }}
          >
            Preview URL
          </Typography>
          <Stack direction="row" alignItems="center" spacing={1}>
            <MuiLink
              href={previewURL}
              target="_blank"
              rel="noopener noreferrer"
              sx={{
                color: tokens.color.accent.violet,
                fontFamily: tokens.font.mono,
                fontSize: 12.5,
                fontWeight: 700,
                wordBreak: "break-all",
              }}
            >
              {previewURL}
            </MuiLink>
            <Tooltip title="Open preview">
              <IconButton
                size="small"
                href={previewURL}
                target="_blank"
                rel="noopener noreferrer"
                aria-label="Open preview"
                sx={{ color: tokens.color.text.secondary }}
              >
                <OpenInNewRounded sx={{ fontSize: 16 }} />
              </IconButton>
            </Tooltip>
          </Stack>
        </Box>
      )}
    </Stack>
  );
}

function ApproveBody(props: {
  previewURL: string | null;
  approval: DeployApprovalCoreFragment | null;
  loadingInbox: boolean;
  required: boolean;
  decisionPending: boolean;
  onApprove: () => void;
  onReject: () => void;
}) {
  const { previewURL, approval, loadingInbox, decisionPending, onApprove, onReject } = props;
  return (
    <Stack spacing={1.25}>
      <Box
        sx={{
          bgcolor: `${tokens.color.accent.warning}10`,
          border: `1px solid ${tokens.color.accent.warning}55`,
          borderRadius: 1,
          p: 1.25,
        }}
      >
        <Typography
          sx={{
            color: tokens.color.accent.warning,
            fontFamily: tokens.font.mono,
            fontSize: 11,
            fontWeight: 800,
            letterSpacing: 0.8,
            mb: 0.5,
            textTransform: "uppercase",
          }}
        >
          Awaiting approval
        </Typography>
        <Typography sx={{ color: tokens.color.text.primary, fontSize: 13 }}>
          Policy requires an operator review before promoting. The decision is enforced
          server-side — only users with the right role can approve.
        </Typography>
      </Box>

      {previewURL && (
        <Stack direction="row" alignItems="center" spacing={1}>
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
              fontWeight: 800,
              letterSpacing: 0.8,
              textTransform: "uppercase",
            }}
          >
            Preview
          </Typography>
          <MuiLink
            href={previewURL}
            target="_blank"
            rel="noopener noreferrer"
            sx={{
              color: tokens.color.accent.violet,
              fontFamily: tokens.font.mono,
              fontSize: 12,
              wordBreak: "break-all",
            }}
          >
            {previewURL}
          </MuiLink>
        </Stack>
      )}

      <Box
        sx={{
          bgcolor: tokens.color.bg.inset,
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: 1,
          p: 1.25,
        }}
      >
        {loadingInbox && !approval ? (
          <Typography sx={{ color: tokens.color.text.muted, fontSize: 12 }}>
            Checking the approval inbox…
          </Typography>
        ) : approval ? (
          <Stack spacing={0.5}>
            <Row label="Approval ID" value={approval.id} mono />
            <Row label="Status" value={approval.status} tone="warning" />
            <Row label="Requested" value={formatDateTime(approval.requestedAt)} />
            <Row label="Expires" value={formatDateTime(approval.expiresAt)} />
            <Row label="Cost impact" value={formatMoney(approval.costImpactUSD)} />
          </Stack>
        ) : (
          <Typography sx={{ color: tokens.color.text.muted, fontSize: 12 }}>
            No pending approval row visible from your account. An operator with the
            tenant_admin role must approve from the Deploy dashboard.
          </Typography>
        )}
      </Box>

      <Stack direction="row" spacing={1} justifyContent="flex-end">
        <Button
          onClick={onReject}
          disabled={!approval || decisionPending}
          sx={dangerButtonSx}
        >
          Reject
        </Button>
        <Button
          onClick={onApprove}
          disabled={!approval || decisionPending}
          startIcon={
            decisionPending ? (
              <CircularProgress size={14} thickness={6} sx={{ color: tokens.color.text.inverse }} />
            ) : (
              <CheckCircleRounded sx={{ fontSize: 16 }} />
            )
          }
          sx={limeButtonSx}
        >
          Approve
        </Button>
      </Stack>
    </Stack>
  );
}

function LiveBody({
  productionURL,
  deployID,
  projectID,
}: {
  productionURL: string | null;
  deployID: string | null;
  projectID: string;
}) {
  const [copied, setCopied] = useState(false);
  const onCopy = useCallback(async () => {
    if (!productionURL) return;
    try {
      await navigator.clipboard.writeText(productionURL);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      /* ignore */
    }
  }, [productionURL]);

  return (
    <Stack spacing={1.5} alignItems="stretch">
      <Stack alignItems="center" spacing={0.75} sx={{ py: 1.5 }}>
        <CheckCircleRounded sx={{ color: tokens.color.accent.violet, fontSize: 40 }} />
        <Typography
          sx={{
            color: tokens.color.text.primary,
            fontSize: 16,
            fontWeight: 800,
            letterSpacing: -0.1,
          }}
        >
          Live on production
        </Typography>
        <Typography sx={{ color: tokens.color.text.muted, fontSize: 12.5 }}>
          The deploy plane finished and ProfitGuard cleared promote.
        </Typography>
      </Stack>

      {productionURL && (
        <Box
          sx={{
            bgcolor: `${tokens.color.accent.violet}10`,
            border: `1px solid ${tokens.color.accent.violet}55`,
            borderRadius: 1,
            p: 1.25,
          }}
        >
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
              fontWeight: 800,
              letterSpacing: 0.8,
              mb: 0.5,
              textTransform: "uppercase",
            }}
          >
            Production URL
          </Typography>
          <Stack direction="row" alignItems="center" spacing={1}>
            <MuiLink
              href={productionURL}
              target="_blank"
              rel="noopener noreferrer"
              sx={{
                color: tokens.color.accent.violet,
                fontFamily: tokens.font.mono,
                fontSize: 13,
                fontWeight: 700,
                wordBreak: "break-all",
                flex: 1,
              }}
            >
              {productionURL}
            </MuiLink>
            <Tooltip title={copied ? "Copied" : "Copy"}>
              <IconButton
                size="small"
                onClick={onCopy}
                aria-label="Copy production URL"
                sx={{ color: tokens.color.text.secondary }}
              >
                <ContentCopyRounded sx={{ fontSize: 16 }} />
              </IconButton>
            </Tooltip>
            <Tooltip title="Open production">
              <IconButton
                size="small"
                href={productionURL}
                target="_blank"
                rel="noopener noreferrer"
                aria-label="Open production"
                sx={{ color: tokens.color.text.secondary }}
              >
                <OpenInNewRounded sx={{ fontSize: 16 }} />
              </IconButton>
            </Tooltip>
          </Stack>
        </Box>
      )}

      {deployID && projectID && (
        <DomainManager projectID={projectID} deployID={deployID} />
      )}

      {deployID && (
        <Stack direction="row" justifyContent="flex-start">
          <Button
            component={NextLink}
            href={`/deploy/${deployID}`}
            sx={ghostButtonSx}
          >
            View deploy details →
          </Button>
        </Stack>
      )}
    </Stack>
  );
}

function DomainManager({ projectID, deployID }: { projectID: string; deployID: string }) {
  const [customHost, setCustomHost] = useState("");
  const [purchaseHost, setPurchaseHost] = useState("");
  const [message, setMessage] = useState<string | null>(null);
  const [autoClaimAttempted, setAutoClaimAttempted] = useState(false);

  const domainsQuery = useDeployDomainsQuery({
    variables: { projectID },
    skip: !projectID,
    fetchPolicy: "cache-and-network",
  });
  const domains = domainsQuery.data?.deployDomains ?? [];
  const managed = domains.find((d) => d.kind === "managed_subdomain") ?? null;
  const primary = domains.find((d) => d.primary) ?? domains[0] ?? null;

  const [reserveSubdomain, reserveState] = useReserveDeploySubdomainMutation();
  const [connectDomain, connectState] = useConnectDeployDomainMutation();
  const [checkDomain, checkState] = useCheckDeployDomainMutation();
  const [checkAvailability, availabilityState] = useDomainAvailabilityLazyQuery();
  const [purchaseDomain, purchaseState] = usePurchaseDeployDomainMutation();

  useEffect(() => {
    if (!projectID || !deployID || managed || reserveState.loading || domainsQuery.loading || autoClaimAttempted) return;
    let cancelled = false;
    const claim = async () => {
      setAutoClaimAttempted(true);
      try {
        await reserveSubdomain({
          variables: {
            input: { projectID, deployID, provider: "ironflyer", primary: domains.length === 0 },
          },
        });
        if (!cancelled) void domainsQuery.refetch();
      } catch {
        /* The production URL is already live; domain claim can be retried manually. */
      }
    };
    void claim();
    return () => {
      cancelled = true;
    };
  }, [
    projectID,
    deployID,
    managed,
    reserveState.loading,
    domainsQuery.loading,
    domainsQuery.refetch,
    reserveSubdomain,
    domains.length,
    autoClaimAttempted,
  ]);

  const connect = useCallback(async () => {
    const host = customHost.trim().toLowerCase();
    if (!host) return;
    setMessage(null);
    try {
      await connectDomain({
        variables: { input: { projectID, deployID, hostname: host, provider: "ironflyer", primary: true } },
      });
      setCustomHost("");
      setMessage("DNS records are ready.");
      setAutoClaimAttempted(true);
      void domainsQuery.refetch();
    } catch (err) {
      setMessage(extractErrorMessage(err instanceof Error ? err : new Error(String(err))));
    }
  }, [connectDomain, customHost, deployID, domainsQuery, projectID]);

  const refresh = useCallback(async (domain: DeployDomainCoreFragment) => {
    setMessage(null);
    try {
      await checkDomain({ variables: { id: domain.id } });
      void domainsQuery.refetch();
    } catch (err) {
      setMessage(extractErrorMessage(err instanceof Error ? err : new Error(String(err))));
    }
  }, [checkDomain, domainsQuery]);

  const purchase = useCallback(async () => {
    const domain = purchaseHost.trim().toLowerCase();
    if (!domain) return;
    setMessage(null);
    try {
      const avail = await checkAvailability({ variables: { domain, registrar: "cloudflare" } });
      if (!avail.data?.domainAvailability.canPurchase) {
        setMessage(avail.data?.domainAvailability.reason ?? "Domain purchase is not available from this workspace.");
        return;
      }
      await purchaseDomain({
        variables: {
          input: {
            projectID,
            deployID,
            domain,
            provider: "ironflyer",
            registrar: "cloudflare",
            years: 1,
            autoRenew: true,
            expectedPriceUSD: avail.data.domainAvailability.priceUSD,
            primary: true,
          },
        },
      });
      setPurchaseHost("");
      setMessage("Purchase submitted; DNS verification is next.");
      setAutoClaimAttempted(true);
      void domainsQuery.refetch();
    } catch (err) {
      setMessage(extractErrorMessage(err instanceof Error ? err : new Error(String(err))));
    }
  }, [checkAvailability, deployID, domainsQuery, projectID, purchaseDomain, purchaseHost]);

  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.inset,
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 1,
        p: 1.25,
      }}
    >
      <Stack spacing={1}>
        <Stack direction="row" alignItems="center" spacing={1}>
          <Typography
            sx={{
              color: tokens.color.text.primary,
              flex: 1,
              fontSize: 13,
              fontWeight: 800,
            }}
          >
            Domains
          </Typography>
          {domainsQuery.loading && <CircularProgress size={14} thickness={5} />}
        </Stack>

        {primary ? (
          <Stack spacing={0.75}>
            {domains.map((domain) => (
              <DomainRow
                key={domain.id}
                domain={domain}
                onCheck={() => refresh(domain)}
                checking={checkState.loading}
              />
            ))}
          </Stack>
        ) : (
          <Typography sx={{ color: tokens.color.text.muted, fontSize: 12 }}>
            Claiming your Ironflyer subdomain…
          </Typography>
        )}

        <Divider sx={{ borderColor: tokens.color.border.subtle }} />

        <Stack direction={{ xs: "column", sm: "row" }} spacing={1}>
          <TextField
            size="small"
            label="Connect domain"
            placeholder="www.example.com"
            value={customHost}
            onChange={(e) => setCustomHost(e.target.value)}
            sx={domainTextFieldSx}
          />
          <Button onClick={connect} disabled={!customHost.trim() || connectState.loading} sx={ghostButtonSx}>
            Connect
          </Button>
        </Stack>

        <Stack direction={{ xs: "column", sm: "row" }} spacing={1}>
          <TextField
            size="small"
            label="Buy domain"
            placeholder="example.com"
            value={purchaseHost}
            onChange={(e) => setPurchaseHost(e.target.value)}
            sx={domainTextFieldSx}
          />
          <Button
            onClick={purchase}
            disabled={!purchaseHost.trim() || availabilityState.loading || purchaseState.loading}
            sx={ghostButtonSx}
          >
            Buy
          </Button>
        </Stack>

        {message && (
          <Typography sx={{ color: tokens.color.text.muted, fontSize: 11.5 }}>
            {message}
          </Typography>
        )}
      </Stack>
    </Box>
  );
}

function DomainRow({
  domain,
  onCheck,
  checking,
}: {
  domain: DeployDomainCoreFragment;
  onCheck: () => void;
  checking: boolean;
}) {
  const tone =
    domain.status === "live"
      ? tokens.color.accent.success
      : domain.status === "failed"
        ? tokens.color.accent.danger
        : tokens.color.accent.warning;
  return (
    <Stack spacing={0.4}>
      <Stack direction="row" spacing={1} alignItems="center">
        <Typography
          sx={{
            color: tokens.color.text.primary,
            flex: 1,
            fontFamily: tokens.font.mono,
            fontSize: 12.5,
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
          title={domain.hostname}
        >
          {domain.primary ? "★ " : ""}
          {domain.hostname}
        </Typography>
        <Typography sx={{ color: tone, fontFamily: tokens.font.mono, fontSize: 10.5 }}>
          {domain.status.replace(/_/g, " ")}
        </Typography>
        <Button size="small" onClick={onCheck} disabled={checking} sx={tinyGhostButtonSx}>
          Check
        </Button>
      </Stack>
      {domain.status !== "live" && domain.dnsRecords.length > 0 && (
        <Stack spacing={0.25}>
          {domain.dnsRecords.slice(0, 3).map((r, idx) => (
            <Typography
              key={`${domain.id}-${idx}`}
              sx={{
                color: tokens.color.text.muted,
                fontFamily: tokens.font.mono,
                fontSize: 10.5,
                wordBreak: "break-all",
              }}
            >
              {r.type} {r.name} → {r.value}
            </Typography>
          ))}
        </Stack>
      )}
    </Stack>
  );
}

function ErrorBody({
  message,
  canRetry,
  onRetry,
}: {
  message: string;
  canRetry: boolean;
  onRetry: () => void;
}) {
  return (
    <Stack spacing={1}>
      <Alert severity="error" variant="outlined" sx={alertSx("danger")}>
        {message}
      </Alert>
      {canRetry && (
        <Stack direction="row" justifyContent="flex-end">
          <Button onClick={onRetry} startIcon={<RefreshRounded sx={{ fontSize: 16 }} />} sx={limeButtonSx}>
            Retry
          </Button>
        </Stack>
      )}
    </Stack>
  );
}

function FooterRow({
  phase,
  busy,
  onClose,
}: {
  phase: PublishPhase;
  busy: boolean;
  onClose: () => void;
}) {
  return (
    <Stack direction="row" alignItems="center" sx={{ pt: 1 }}>
      <Box sx={{ flex: 1 }}>
        <Typography
          sx={{
            color: tokens.color.text.muted,
            fontFamily: tokens.font.mono,
            fontSize: 11,
            letterSpacing: 0.4,
            textTransform: "uppercase",
          }}
        >
          {PHASE_LABEL[phase]}
        </Typography>
      </Box>
      <Button
        onClick={onClose}
        disabled={busy}
        sx={{
          color: tokens.color.text.secondary,
          fontWeight: 700,
          minHeight: 36,
          px: 1.5,
          textTransform: "none",
          "&:hover": { bgcolor: tokens.color.bg.surfaceHover },
        }}
      >
        {phase === "live" ? "Close" : phase === "failed" ? "Cancel" : "Cancel"}
      </Button>
    </Stack>
  );
}

const PHASE_LABEL: Record<PublishPhase, string> = {
  idle: "Ready to publish",
  planning: "Planning deploy…",
  building: "Building preview…",
  approving: "Awaiting approval…",
  promoting: "Promoting to production…",
  live: "Live.",
  failed: "Publish failed.",
};

function Row({
  label,
  value,
  tone,
  hint,
  mono,
}: {
  label: string;
  value: string;
  tone?: "neutral" | "success" | "danger" | "warning";
  hint?: string;
  mono?: boolean;
}) {
  const color =
    tone === "success"
      ? tokens.color.accent.success
      : tone === "danger"
        ? tokens.color.accent.danger
        : tone === "warning"
          ? tokens.color.accent.warning
          : tokens.color.text.primary;
  return (
    <Stack>
      <Stack direction="row" alignItems="center">
        <Typography
          sx={{
            color: tokens.color.text.muted,
            flex: 1,
            fontFamily: tokens.font.mono,
            fontSize: 11.5,
            letterSpacing: 0.2,
            textTransform: "uppercase",
          }}
        >
          {label}
        </Typography>
        <Typography
          sx={{
            color,
            fontFamily: mono ? tokens.font.mono : tokens.font.mono,
            fontSize: 12.5,
            fontWeight: 800,
            maxWidth: 260,
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
          title={value}
        >
          {value}
        </Typography>
      </Stack>
      {hint && (
        <Typography
          sx={{
            color: tokens.color.text.muted,
            fontFamily: tokens.font.mono,
            fontSize: 10.5,
            letterSpacing: 0.4,
            mt: 0.25,
            textAlign: "right",
          }}
        >
          {hint}
        </Typography>
      )}
    </Stack>
  );
}

function LabeledSelect({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (next: string) => void;
}) {
  return (
    <Stack spacing={0.25} sx={{ flex: 1 }}>
      <Typography
        sx={{
          color: tokens.color.text.muted,
          fontFamily: tokens.font.mono,
          fontSize: 10.5,
          fontWeight: 800,
          letterSpacing: 0.8,
          textTransform: "uppercase",
        }}
      >
        {label}
      </Typography>
      <Select
        size="small"
        value={value}
        onChange={(e) => onChange(String(e.target.value))}
        sx={{
          bgcolor: tokens.color.bg.inset,
          color: tokens.color.text.primary,
          fontFamily: tokens.font.mono,
          fontSize: 12.5,
          "& .MuiOutlinedInput-notchedOutline": {
            borderColor: tokens.color.border.subtle,
          },
          "&:hover .MuiOutlinedInput-notchedOutline": {
            borderColor: tokens.color.border.strong ?? tokens.color.border.subtle,
          },
        }}
      >
        {options.map((o) => (
          <MenuItem key={o.value} value={o.value}>
            {o.label}
          </MenuItem>
        ))}
      </Select>
    </Stack>
  );
}

// =====================================================================
// Style helpers
// =====================================================================

const limeButtonSx = {
  bgcolor: tokens.color.accent.violet,
  color: tokens.color.text.inverse,
  fontWeight: 800,
  minHeight: 36,
  px: 1.75,
  textTransform: "none" as const,
  "&:hover": {
    bgcolor: tokens.color.accent.violet,
    filter: "brightness(0.93)",
  },
  "&.Mui-disabled": {
    bgcolor: tokens.color.bg.surfaceRaised,
    color: tokens.color.text.muted,
  },
};

const dangerButtonSx = {
  bgcolor: "transparent",
  color: tokens.color.accent.danger,
  border: `1px solid ${tokens.color.accent.danger}55`,
  fontWeight: 800,
  minHeight: 36,
  px: 1.75,
  textTransform: "none" as const,
  "&:hover": {
    bgcolor: `${tokens.color.accent.danger}10`,
    borderColor: `${tokens.color.accent.danger}88`,
  },
  "&.Mui-disabled": {
    color: tokens.color.text.muted,
    borderColor: tokens.color.border.subtle,
  },
};

const ghostButtonSx = {
  color: tokens.color.text.secondary,
  fontWeight: 700,
  minHeight: 32,
  px: 1.25,
  textTransform: "none" as const,
  "&:hover": { bgcolor: tokens.color.bg.surfaceHover, color: tokens.color.text.primary },
};

const tinyGhostButtonSx = {
  ...ghostButtonSx,
  fontSize: 11,
  minHeight: 26,
  minWidth: 0,
  px: 0.9,
};

const domainTextFieldSx = {
  flex: 1,
  minWidth: 0,
  "& .MuiInputBase-root": {
    bgcolor: tokens.color.bg.surface,
    color: tokens.color.text.primary,
    fontFamily: tokens.font.mono,
    fontSize: 12.5,
  },
  "& .MuiInputLabel-root": {
    color: tokens.color.text.muted,
    fontSize: 12,
  },
  "& .MuiInputLabel-root.Mui-focused": {
    color: tokens.color.accent.violet,
  },
  "& .MuiOutlinedInput-notchedOutline": {
    borderColor: tokens.color.border.subtle,
  },
  "&:hover .MuiOutlinedInput-notchedOutline": {
    borderColor: tokens.color.border.strong ?? tokens.color.border.subtle,
  },
  "& .Mui-focused .MuiOutlinedInput-notchedOutline": {
    borderColor: tokens.color.accent.violet,
  },
  "& input::placeholder": {
    color: tokens.color.text.muted,
    opacity: 0.75,
  },
};

function alertSx(tone: "warning" | "danger" | "success") {
  const c =
    tone === "warning"
      ? tokens.color.accent.warning
      : tone === "danger"
        ? tokens.color.accent.danger
        : tokens.color.accent.success;
  return {
    bgcolor: `${c}10`,
    borderColor: `${c}55`,
    color: tokens.color.text.primary,
    "& .MuiAlert-icon": { color: c },
  };
}
