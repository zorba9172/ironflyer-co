"use client";

// /execution/[id] — live cockpit for a single paid run.
//
// Layout (top → bottom):
//   1. Sticky header strip — breadcrumb / short id / status badge /
//      live elapsed clock / action buttons (Open Workspace, Stop,
//      Refund, Security report).
//   2. Phase stepper — Queue → Plan → Patch → Build → Verify → Ship.
//      Status of each phase is derived from execution.status +
//      streamed events; the active phase pulses.
//   3. Three-column body:
//        • Left — Cost & limits (CostBreakdown + MoneyChip stats).
//        • Center — Live event timeline (filter chips + search +
//          expand-for-payload, auto-scroll-to-newest).
//        • Right — Gate verdicts (from ProfitGuard decisions) +
//          mini ledger feed + Support bundle quick-open.
//   4. Sticky bottom toolbar — copy execution id, copy curl, V22
//      docs link.
//
// Live updates: we subscribe via useExecutionFeedSubscription when
// the status is not terminal; the execution row itself polls every
// 4s so balance/spend numbers update even when no event arrives.
// Both feeds switch off the moment the status becomes terminal.

import {
  AccountBalanceWalletOutlined,
  ArrowBackRounded,
  ContentCopyRounded,
  LaunchRounded,
  OpenInNewRounded,
  PauseRounded,
  PlayArrowRounded,
  ShieldOutlined,
  StopRounded,
  TerminalRounded,
} from "@mui/icons-material";
import {
  Box,
  Button,
  Card,
  Chip,
  CircularProgress,
  IconButton,
  Stack,
  Tooltip,
  Typography,
} from "@mui/material";
import Link from "next/link";
import {
  use,
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import {
  EmptyState,
  ErrorPanel,
  LoadingPanel,
  MoneyChip,
  PageHeader,
  StatusBadge,
} from "../../../src/components/cockpit";
import {
  CostBreakdown,
  LiveEventTimeline,
  RefundExecutionDialog,
  StopExecutionDialog,
  SupportBundlePanel,
  type LiveEvent,
} from "../../../src/components/executions";
import { RequireAuth, useAuth } from "../../../src/lib/auth";
import {
  useExecutionFeedSubscription,
  useExecutionLedgerQuery,
  useExecutionQuery,
  useProfitGuardDecisionsQuery,
  type ExecutionCoreFragment,
} from "../../../src/lib/gql/__generated__";
import { formatDateTime, formatMoney } from "../../../src/lib/format";
import { relativeTime } from "../../../src/lib/relativeTime";
import { tokens } from "../../../src/theme";

// ─── status helpers ───────────────────────────────────────────────

const RUNNING_STATES = new Set([
  "created",
  "queued",
  "admitted",
  "running",
  "scoring",
  "paused",
]);
const TERMINAL_STATES = new Set([
  "succeeded",
  "success",
  "failed",
  "killed",
  "stopped",
  "refunded",
  "done",
]);

function isOperator(plan?: string | null): boolean {
  if (!plan) return false;
  const p = plan.toLowerCase();
  return p === "operator" || p === "admin" || p === "owner";
}

function shortId(id: string): string {
  if (id.length <= 12) return id;
  return `${id.slice(0, 8)}…`;
}

// ─── phase stepper ────────────────────────────────────────────────

type PhaseKey = "queue" | "plan" | "patch" | "build" | "verify" | "ship";
type PhaseStatus = "pending" | "active" | "done" | "failed";

interface PhaseSpec {
  key: PhaseKey;
  label: string;
}

const PHASES: PhaseSpec[] = [
  { key: "queue", label: "Queue" },
  { key: "plan", label: "Plan" },
  { key: "patch", label: "Patch" },
  { key: "build", label: "Build" },
  { key: "verify", label: "Verify" },
  { key: "ship", label: "Ship" },
];

// derivePhases — best-effort mapping from status + event types to a
// six-phase stepper. The orchestrator does not currently emit a
// canonical phase field on every event, so this routes based on
// keywords. When the schema grows a phase column we should swap to
// that.
//
// TODO(exec-wire): switch to a canonical execution.phase field once
// the orchestrator surfaces it through GraphQL — keyword matching is
// inherently lossy.
function derivePhases(
  status: string,
  events: LiveEvent[],
): Record<PhaseKey, PhaseStatus> {
  const lower = status.toLowerCase();
  const map: Record<PhaseKey, PhaseStatus> = {
    queue: "pending",
    plan: "pending",
    patch: "pending",
    build: "pending",
    verify: "pending",
    ship: "pending",
  };

  // queue is always "done" the moment we have any execution row
  map.queue = "done";

  // walk events to mark phases as done in order
  for (const e of events) {
    const t = e.eventType.toLowerCase();
    if (t.includes("plan")) map.plan = "done";
    if (t.includes("patch") || t.includes("diff") || t.includes("file"))
      map.patch = "done";
    if (t.includes("build") || t.includes("compile")) map.build = "done";
    if (t.includes("verify") || t.includes("gate") || t.includes("verdict"))
      map.verify = "done";
    if (t.includes("deploy") || t.includes("ship") || t.includes("promote"))
      map.ship = "done";
  }

  // current state pin
  const isRunning = RUNNING_STATES.has(lower);
  const isTerminal = TERMINAL_STATES.has(lower);

  if (lower === "failed" || lower === "killed") {
    // bind the failure to the most recent in-progress phase
    const order: PhaseKey[] = ["plan", "patch", "build", "verify", "ship"];
    let pinned = false;
    for (const k of order) {
      if (map[k] === "pending" && !pinned) {
        map[k] = "failed";
        pinned = true;
      }
    }
    if (!pinned) map.ship = "failed";
    return map;
  }

  if (isTerminal) {
    // every phase up to ship is done
    for (const p of PHASES) map[p.key] = "done";
    return map;
  }

  // running — first pending phase becomes "active"
  if (isRunning) {
    const order: PhaseKey[] = ["plan", "patch", "build", "verify", "ship"];
    for (const k of order) {
      if (map[k] === "pending") {
        map[k] = "active";
        break;
      }
    }
  }

  return map;
}

function phaseColor(s: PhaseStatus): string {
  switch (s) {
    case "done":
      return tokens.color.accent.success;
    case "active":
      return tokens.color.accent.violet;
    case "failed":
      return tokens.color.accent.danger;
    default:
      return tokens.color.text.muted;
  }
}

// ─── live elapsed clock ───────────────────────────────────────────

function useElapsed(startedAt: string | null, active: boolean): string {
  const [, tick] = useState(0);
  useEffect(() => {
    if (!active) return;
    const i = setInterval(() => tick((n) => n + 1), 1000);
    return () => clearInterval(i);
  }, [active]);
  if (!startedAt) return "—";
  const ms = Date.now() - new Date(startedAt).getTime();
  if (!Number.isFinite(ms) || ms < 0) return "—";
  const sec = Math.floor(ms / 1000);
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  const s = sec % 60;
  if (h > 0) return `${h}h ${m.toString().padStart(2, "0")}m ${s.toString().padStart(2, "0")}s`;
  if (m > 0) return `${m}m ${s.toString().padStart(2, "0")}s`;
  return `${s}s`;
}

// ─── page ─────────────────────────────────────────────────────────

export default function ExecutionDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  return (
    <RequireAuth>
      <ExecutionCockpit id={id} />
    </RequireAuth>
  );
}

function ExecutionCockpit({ id }: { id: string }) {
  const { user } = useAuth();
  const operator = isOperator(user?.plan);

  const [stopOpen, setStopOpen] = useState(false);
  const [refundOpen, setRefundOpen] = useState(false);
  const [costOpen, setCostOpen] = useState(true);
  const [rightOpen, setRightOpen] = useState(true);

  const execQ = useExecutionQuery({
    variables: { id },
    fetchPolicy: "cache-and-network",
    pollInterval: 4000,
  });

  const e = execQ.data?.execution;
  const statusLower = (e?.status ?? "").toLowerCase();
  const terminal = TERMINAL_STATES.has(statusLower);
  const running = RUNNING_STATES.has(statusLower);

  // Stop polling once we're terminal. Apollo lets us toggle interval
  // via startPolling / stopPolling on the query observer.
  useEffect(() => {
    if (terminal) execQ.stopPolling();
    else if (running) execQ.startPolling(4000);
    // execQ is stable — only the state flags matter.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [terminal, running]);

  // ── live event stream ─────────────────────────────────────────
  const feed = useExecutionFeedSubscription({
    variables: { id },
    skip: terminal,
  });

  const [events, setEvents] = useState<LiveEvent[]>([]);
  useEffect(() => {
    const ev = feed.data?.executionFeed;
    if (!ev) return;
    setEvents((prev) => {
      const next: LiveEvent = {
        id: `${ev.createdAt}-${ev.eventType}-${prev.length}`,
        eventType: ev.eventType,
        createdAt: ev.createdAt,
        payload: ev.payload,
      };
      // Avoid duplicate appends if the same payload re-emits within
      // the same tick (subscription deduping is a server concern).
      if (prev.length > 0) {
        const last = prev[prev.length - 1];
        if (
          last.eventType === next.eventType &&
          last.createdAt === next.createdAt
        ) {
          return prev;
        }
      }
      return [...prev, next];
    });
  }, [feed.data]);

  // ── early returns ─────────────────────────────────────────────
  if (execQ.loading && !e) return <LoadingPanel label="Loading execution" />;
  if (execQ.error) {
    return (
      <ErrorPanel
        error={execQ.error}
        title="Execution unavailable"
        onRetry={() => void execQ.refetch()}
      />
    );
  }
  if (!e) {
    return (
      <Box>
        <PageHeader
          title="Execution not found"
          breadcrumbs={[
            { label: "Executions", href: "/executions" },
            { label: id },
          ]}
        />
        <EmptyState
          title="This execution does not exist or you do not have access."
          cta={{ label: "Back to executions", href: "/executions" }}
        />
      </Box>
    );
  }

  return (
    <Box>
      <HeaderStrip
        execution={e}
        operator={operator}
        running={running}
        terminal={terminal}
        feedConnected={!terminal && !feed.error}
        onStop={() => setStopOpen(true)}
        onRefund={() => setRefundOpen(true)}
      />

      <PhaseStepper status={e.status} events={events} createdAt={e.createdAt} />

      <Box
        sx={{
          mt: 3,
          display: "grid",
          gap: 2,
          gridTemplateColumns: {
            xs: "minmax(0, 1fr)",
            md: `${costOpen ? "280px" : "44px"} minmax(0, 1fr) ${rightOpen ? "320px" : "44px"}`,
          },
          alignItems: "start",
        }}
      >
        <SidePanel
          title="Cost & limits"
          collapsed={!costOpen}
          onToggle={() => setCostOpen((v) => !v)}
        >
          <CostPanel execution={e} />
        </SidePanel>

        <CenterPanel execution={e} events={events} feedError={feed.error} />

        <SidePanel
          title="Verdicts"
          collapsed={!rightOpen}
          onToggle={() => setRightOpen((v) => !v)}
          align="end"
        >
          <RightPanel executionID={e.id} />
        </SidePanel>
      </Box>

      <BottomToolbar executionID={e.id} />

      <StopExecutionDialog
        open={stopOpen}
        executionID={e.id}
        onClose={() => setStopOpen(false)}
        onStopped={() => void execQ.refetch()}
      />
      <RefundExecutionDialog
        open={refundOpen}
        executionID={e.id}
        unusedReserveUSD={Math.max(0, e.reservedUSD - e.spentUSD)}
        onClose={() => setRefundOpen(false)}
        onRefunded={() => void execQ.refetch()}
      />
    </Box>
  );
}

// ─── header strip ─────────────────────────────────────────────────

function HeaderStrip({
  execution: e,
  operator,
  running,
  terminal,
  feedConnected,
  onStop,
  onRefund,
}: {
  execution: ExecutionCoreFragment;
  operator: boolean;
  running: boolean;
  terminal: boolean;
  feedConnected: boolean;
  onStop: () => void;
  onRefund: () => void;
}) {
  const elapsed = useElapsed(e.startedAt ?? e.createdAt, !terminal);

  return (
    <Box
      sx={{
        position: "sticky",
        top: 70,
        zIndex: 4,
        mx: { xs: -2, md: -4 },
        px: { xs: 2, md: 4 },
        py: 1.5,
        mb: 2,
        bgcolor: `${tokens.color.bg.surface}d9`,
        backdropFilter: "blur(14px) saturate(140%)",
        borderBottom: `1px solid ${tokens.color.border.subtle}`,
      }}
    >
      <PageHeader
        sx={{ mb: 0 }}
        eyebrow={`Execution · ${shortId(e.id)}`}
        title={e.promptSummary || `Execution ${shortId(e.id)}`}
        breadcrumbs={[
          { label: "Executions", href: "/executions" },
          {
            label: e.projectID ? "Project" : "Project —",
            href: e.projectID ? `/p/${e.projectID}` : undefined,
          },
          { label: shortId(e.id) },
        ]}
        actions={
          <Stack direction="row" spacing={1} sx={{ flexWrap: "wrap", rowGap: 1 }}>
            <Button
              component={Link}
              href="/executions"
              size="small"
              variant="outlined"
              startIcon={<ArrowBackRounded sx={{ fontSize: 16 }} />}
              sx={{
                borderColor: tokens.color.border.strong,
                color: tokens.color.text.primary,
              }}
            >
              Back
            </Button>
            {e.projectID && (
              <Button
                component={Link}
                href={`/p/${e.projectID}`}
                size="small"
                variant="contained"
                color="primary"
                startIcon={<TerminalRounded sx={{ fontSize: 16 }} />}
              >
                Open workspace
              </Button>
            )}
            <Button
              component={Link}
              href={`/execution/${e.id}/security`}
              size="small"
              variant="outlined"
              startIcon={<ShieldOutlined sx={{ fontSize: 16 }} />}
              sx={{
                borderColor: tokens.color.border.strong,
                color: tokens.color.text.primary,
              }}
            >
              Security
            </Button>
            {running && (
              <Button
                size="small"
                variant="outlined"
                onClick={onStop}
                startIcon={<StopRounded sx={{ fontSize: 16 }} />}
                sx={{
                  borderColor: `${tokens.color.accent.danger}66`,
                  color: tokens.color.accent.danger,
                  "&:hover": {
                    borderColor: tokens.color.accent.danger,
                    bgcolor: `${tokens.color.accent.danger}14`,
                  },
                }}
              >
                Stop run
              </Button>
            )}
            {operator && terminal && (
              <Button
                size="small"
                variant="outlined"
                onClick={onRefund}
                startIcon={<AccountBalanceWalletOutlined sx={{ fontSize: 16 }} />}
                sx={{
                  borderColor: `${tokens.color.accent.warning}66`,
                  color: tokens.color.accent.warning,
                  "&:hover": {
                    borderColor: tokens.color.accent.warning,
                    bgcolor: `${tokens.color.accent.warning}14`,
                  },
                }}
              >
                Refund
              </Button>
            )}
          </Stack>
        }
      />
      <Stack
        direction="row"
        spacing={1.25}
        alignItems="center"
        sx={{ mt: 1.5, flexWrap: "wrap", rowGap: 1 }}
      >
        <StatusBadge status={e.status} />
        {!terminal && (
          <Chip
            size="small"
            icon={
              feedConnected ? (
                <PlayArrowRounded sx={{ fontSize: 14, color: `${tokens.color.accent.success} !important` }} />
              ) : (
                <PauseRounded sx={{ fontSize: 14, color: `${tokens.color.accent.warning} !important` }} />
              )
            }
            label={feedConnected ? `LIVE · ${elapsed}` : `RECONNECTING · ${elapsed}`}
            sx={{
              bgcolor: feedConnected
                ? `${tokens.color.accent.success}1c`
                : `${tokens.color.accent.warning}1c`,
              color: feedConnected
                ? tokens.color.accent.success
                : tokens.color.accent.warning,
              border: `1px solid ${
                feedConnected
                  ? `${tokens.color.accent.success}55`
                  : `${tokens.color.accent.warning}55`
              }`,
              fontFamily: tokens.font.mono,
              fontWeight: 700,
              fontSize: 10.5,
              letterSpacing: 0.8,
              height: 22,
              borderRadius: 0.75,
              animation: feedConnected
                ? "ironflyerLivePulse 2.4s ease-in-out infinite"
                : "none",
              "@keyframes ironflyerLivePulse": {
                "0%, 100%": { boxShadow: `0 0 0 0 ${tokens.color.accent.success}00` },
                "50%": { boxShadow: `0 0 0 4px ${tokens.color.accent.success}33` },
              },
            }}
          />
        )}
        {terminal && e.endedAt && (
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 11.5,
            }}
          >
            ended {relativeTime(e.endedAt)} · {formatDateTime(e.endedAt)}
          </Typography>
        )}
        {e.failureReason && (
          <Typography sx={{ color: tokens.color.accent.danger, fontSize: 13 }}>
            {e.failureReason}
          </Typography>
        )}
      </Stack>
    </Box>
  );
}

// ─── phase stepper ────────────────────────────────────────────────

function PhaseStepper({
  status,
  events,
  createdAt,
}: {
  status: string;
  events: LiveEvent[];
  createdAt: string;
}) {
  const phases = useMemo(() => derivePhases(status, events), [status, events]);
  const elapsed = useElapsed(createdAt, true);

  return (
    <Card sx={{ p: 2, mb: 1 }}>
      <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1.25 }}>
        <Typography
          variant="overline"
          sx={{ color: tokens.color.text.muted, letterSpacing: 1.2 }}
        >
          Phases
        </Typography>
        <Box sx={{ flex: 1 }} />
        <Typography
          sx={{
            color: tokens.color.text.muted,
            fontFamily: tokens.font.mono,
            fontSize: 11,
          }}
        >
          total {elapsed}
        </Typography>
      </Stack>
      <Stack
        direction="row"
        alignItems="center"
        spacing={0}
        sx={{ overflowX: "auto", pb: 0.5 }}
      >
        {PHASES.map((p, i) => {
          const s = phases[p.key];
          const color = phaseColor(s);
          return (
            <Stack
              key={p.key}
              direction="row"
              alignItems="center"
              spacing={1.25}
              sx={{ flex: 1, minWidth: 90 }}
            >
              <Box
                sx={{
                  position: "relative",
                  width: 28,
                  height: 28,
                  borderRadius: "50%",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  bgcolor: s === "done" ? color : `${color}1c`,
                  border: `1px solid ${color}66`,
                  color: s === "done" ? tokens.color.text.inverse : color,
                  fontFamily: tokens.font.mono,
                  fontWeight: 800,
                  fontSize: 12,
                  flexShrink: 0,
                  animation:
                    s === "active"
                      ? "ironflyerPhasePulse 1.6s ease-in-out infinite"
                      : "none",
                  "@keyframes ironflyerPhasePulse": {
                    "0%, 100%": { boxShadow: `0 0 0 0 ${color}00` },
                    "50%": { boxShadow: `0 0 0 6px ${color}33` },
                  },
                }}
              >
                {i + 1}
              </Box>
              <Stack sx={{ minWidth: 0 }}>
                <Typography
                  sx={{
                    fontSize: 12.5,
                    fontWeight: 700,
                    color:
                      s === "pending"
                        ? tokens.color.text.muted
                        : tokens.color.text.primary,
                  }}
                >
                  {p.label}
                </Typography>
                <Typography
                  sx={{
                    fontFamily: tokens.font.mono,
                    fontSize: 10.5,
                    color,
                    letterSpacing: 0.5,
                    textTransform: "uppercase",
                  }}
                >
                  {s}
                </Typography>
              </Stack>
              {i < PHASES.length - 1 && (
                <Box
                  sx={{
                    flex: 1,
                    height: 1,
                    bgcolor:
                      s === "done" ? color : tokens.color.border.subtle,
                    opacity: 0.7,
                    minWidth: 12,
                  }}
                />
              )}
            </Stack>
          );
        })}
      </Stack>
    </Card>
  );
}

// ─── side panel shell ─────────────────────────────────────────────

function SidePanel({
  title,
  collapsed,
  onToggle,
  align,
  children,
}: {
  title: string;
  collapsed: boolean;
  onToggle: () => void;
  align?: "start" | "end";
  children: ReactNode;
}) {
  if (collapsed) {
    return (
      <Card
        sx={{
          p: 0.5,
          display: { xs: "none", md: "flex" },
          flexDirection: "column",
          alignItems: "center",
          minHeight: 200,
        }}
      >
        <IconButton size="small" onClick={onToggle} sx={{ color: tokens.color.text.secondary }}>
          <OpenInNewRounded sx={{ fontSize: 16 }} />
        </IconButton>
        <Typography
          sx={{
            mt: 1,
            color: tokens.color.text.muted,
            fontSize: 10.5,
            letterSpacing: 0.8,
            textTransform: "uppercase",
            writingMode: "vertical-rl",
            transform: align === "end" ? "rotate(180deg)" : "none",
          }}
        >
          {title}
        </Typography>
      </Card>
    );
  }
  return (
    <Card sx={{ p: 2 }}>
      <Stack
        direction="row"
        alignItems="center"
        spacing={1}
        sx={{ mb: 1.5 }}
      >
        <Typography
          variant="overline"
          sx={{ color: tokens.color.text.muted, letterSpacing: 1.2 }}
        >
          {title}
        </Typography>
        <Box sx={{ flex: 1 }} />
        <IconButton
          size="small"
          onClick={onToggle}
          aria-label={`Collapse ${title}`}
          sx={{ color: tokens.color.text.secondary }}
        >
          <OpenInNewRounded sx={{ fontSize: 14, transform: "rotate(180deg)" }} />
        </IconButton>
      </Stack>
      {children}
    </Card>
  );
}

// ─── cost panel ───────────────────────────────────────────────────

function CostPanel({ execution: e }: { execution: ExecutionCoreFragment }) {
  const spendPct =
    e.budgetUSD > 0 ? Math.min(100, (e.spentUSD / e.budgetUSD) * 100) : 0;
  const reservedPct =
    e.budgetUSD > 0
      ? Math.min(100, ((e.reservedUSD + e.spentUSD) / e.budgetUSD) * 100)
      : 0;
  return (
    <Stack spacing={2}>
      <Stack spacing={1}>
        <Stack direction="row" justifyContent="space-between" alignItems="baseline">
          <Typography sx={{ fontSize: 12.5, color: tokens.color.text.secondary }}>
            Spend vs budget
          </Typography>
          <Typography
            sx={{
              fontFamily: tokens.font.mono,
              fontSize: 11.5,
              color: tokens.color.text.muted,
            }}
          >
            {spendPct.toFixed(0)}%
          </Typography>
        </Stack>
        <Box
          sx={{
            position: "relative",
            height: 10,
            borderRadius: 1,
            bgcolor: tokens.color.bg.inset,
            border: `1px solid ${tokens.color.border.subtle}`,
            overflow: "hidden",
          }}
        >
          <Box
            sx={{
              position: "absolute",
              left: 0,
              top: 0,
              bottom: 0,
              width: `${reservedPct}%`,
              bgcolor: `${tokens.color.accent.violet}55`,
            }}
          />
          <Box
            sx={{
              position: "absolute",
              left: 0,
              top: 0,
              bottom: 0,
              width: `${spendPct}%`,
              bgcolor: tokens.color.accent.coral,
            }}
          />
        </Box>
        <Stack direction="row" spacing={0.75} sx={{ flexWrap: "wrap", rowGap: 1 }}>
          <MoneyChip amountUSD={e.spentUSD} color="negative" />
          <MoneyChip amountUSD={e.reservedUSD} color="accent" />
          <MoneyChip amountUSD={e.budgetUSD} color="neutral" />
        </Stack>
        <Typography
          sx={{ color: tokens.color.text.muted, fontFamily: tokens.font.mono, fontSize: 11 }}
        >
          spent · reserved · budget
        </Typography>
      </Stack>

      <Box>
        <Typography
          variant="overline"
          sx={{ color: tokens.color.text.muted, letterSpacing: 1.2 }}
        >
          Cost breakdown
        </Typography>
        <Box sx={{ mt: 1 }}>
          <CostBreakdown
            providerCostUSD={e.providerCostUSD}
            sandboxCostUSD={e.sandboxCostUSD}
            storageCostUSD={e.storageCostUSD}
            deploymentCostUSD={e.deploymentCostUSD}
          />
        </Box>
      </Box>

      <Box>
        <Typography
          variant="overline"
          sx={{ color: tokens.color.text.muted, letterSpacing: 1.2 }}
        >
          Wallet rollup
        </Typography>
        <Stack direction="row" spacing={0.75} sx={{ mt: 1, flexWrap: "wrap", rowGap: 1 }}>
          <MoneyChip amountUSD={e.revenueUSD} color="positive" />
          <MoneyChip amountUSD={e.providerCostUSD} color="negative" />
          <MoneyChip amountUSD={e.refundedUSD} color="warning" />
        </Stack>
        <Typography
          sx={{ mt: 1, color: tokens.color.text.muted, fontFamily: tokens.font.mono, fontSize: 11 }}
        >
          revenue · provider · refunded
        </Typography>
      </Box>
    </Stack>
  );
}

// ─── center panel (event feed) ────────────────────────────────────

function CenterPanel({
  execution: e,
  events,
  feedError,
}: {
  execution: ExecutionCoreFragment;
  events: LiveEvent[];
  feedError: unknown;
}) {
  const elapsed = useElapsed(e.createdAt, !e.startedAt);
  const waiting = events.length === 0 && !e.startedAt;
  return (
    <Card
      sx={{
        p: 2,
        minHeight: 560,
        display: "flex",
        flexDirection: "column",
        gap: 1.25,
      }}
    >
      <Stack direction="row" alignItems="center" spacing={1}>
        <Typography
          variant="overline"
          sx={{ color: tokens.color.text.muted, letterSpacing: 1.2 }}
        >
          Event timeline
        </Typography>
        <Box sx={{ flex: 1 }} />
        <Typography
          sx={{
            color: tokens.color.text.muted,
            fontFamily: tokens.font.mono,
            fontSize: 11,
          }}
        >
          {events.length} events
        </Typography>
      </Stack>
      {waiting ? (
        <Box
          sx={{
            flex: 1,
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            justifyContent: "center",
            gap: 1.5,
            border: `1px dashed ${tokens.color.border.subtle}`,
            borderRadius: 1,
            bgcolor: tokens.color.bg.inset,
            minHeight: 480,
            p: 4,
            textAlign: "center",
          }}
        >
          <CircularProgress
            size={32}
            sx={{ color: tokens.color.accent.violet }}
          />
          <Typography sx={{ fontWeight: 700, fontSize: 15 }}>
            Waiting for the runtime to claim this execution…
          </Typography>
          <Typography sx={{ color: tokens.color.text.muted, fontSize: 12.5 }}>
            queued {elapsed} ago · admission gate runs before any provider call
          </Typography>
          {feedError ? (
            <Typography sx={{ color: tokens.color.accent.warning, fontSize: 12 }}>
              Live feed unavailable — polling for updates instead.
            </Typography>
          ) : null}
        </Box>
      ) : (
        <LiveEventTimeline events={events} height={520} />
      )}
    </Card>
  );
}

// ─── right panel (verdicts + ledger + bundle) ─────────────────────

function RightPanel({ executionID }: { executionID: string }) {
  return (
    <Stack spacing={2}>
      <VerdictStack executionID={executionID} />
      <LedgerStrip executionID={executionID} />
      <Box>
        <Typography
          variant="overline"
          sx={{ color: tokens.color.text.muted, letterSpacing: 1.2 }}
        >
          Support bundle
        </Typography>
        <Box sx={{ mt: 1 }}>
          <SupportBundlePanel executionID={executionID} />
        </Box>
      </Box>
    </Stack>
  );
}

function VerdictStack({ executionID }: { executionID: string }) {
  const { data, loading, error } = useProfitGuardDecisionsQuery({
    variables: { executionID, limit: 20 },
    fetchPolicy: "cache-and-network",
    pollInterval: 8000,
  });
  const decisions = data?.profitGuardDecisions ?? [];

  if (loading && decisions.length === 0) {
    return (
      <Box sx={{ py: 1, color: tokens.color.text.muted, fontSize: 12.5 }}>
        Loading verdicts…
      </Box>
    );
  }
  if (error) {
    return (
      <Typography sx={{ color: tokens.color.accent.danger, fontSize: 12.5 }}>
        Could not load verdicts.
      </Typography>
    );
  }
  if (decisions.length === 0) {
    return (
      <Box
        sx={{
          border: `1px dashed ${tokens.color.border.subtle}`,
          borderRadius: 1,
          p: 1.5,
          color: tokens.color.text.muted,
          fontSize: 12.5,
        }}
      >
        No verdicts yet. ProfitGuard logs every enforcement point as it fires.
      </Box>
    );
  }
  return (
    <Stack spacing={1}>
      {decisions.slice(0, 6).map((d) => (
        <Box
          key={d.id}
          sx={{
            border: `1px solid ${tokens.color.border.subtle}`,
            borderRadius: 1,
            p: 1.25,
            bgcolor: tokens.color.bg.inset,
          }}
        >
          <Stack direction="row" alignItems="center" spacing={1}>
            <StatusBadge status={d.decision} tone={verdictTone(d.decision)} />
            <Typography
              sx={{
                fontFamily: tokens.font.mono,
                fontSize: 11,
                color: tokens.color.text.muted,
                ml: "auto",
                whiteSpace: "nowrap",
              }}
            >
              {relativeTime(d.createdAt)}
            </Typography>
          </Stack>
          <Typography
            sx={{
              mt: 0.5,
              fontSize: 12.5,
              color: tokens.color.text.primary,
              fontWeight: 600,
            }}
          >
            {d.enforcementPoint}
          </Typography>
          <Typography
            sx={{
              mt: 0.25,
              fontSize: 12,
              color: tokens.color.text.secondary,
              overflow: "hidden",
              display: "-webkit-box",
              WebkitLineClamp: 2,
              WebkitBoxOrient: "vertical",
            }}
          >
            {d.reason}
          </Typography>
        </Box>
      ))}
    </Stack>
  );
}

function LedgerStrip({ executionID }: { executionID: string }) {
  const { data, loading } = useExecutionLedgerQuery({
    variables: { executionID, limit: 8, offset: 0 },
    fetchPolicy: "cache-and-network",
    pollInterval: 6000,
  });
  const entries = data?.executionLedger ?? [];
  return (
    <Box>
      <Typography
        variant="overline"
        sx={{ color: tokens.color.text.muted, letterSpacing: 1.2 }}
      >
        Ledger feed
      </Typography>
      {loading && entries.length === 0 ? (
        <Typography sx={{ mt: 1, color: tokens.color.text.muted, fontSize: 12.5 }}>
          Loading entries…
        </Typography>
      ) : entries.length === 0 ? (
        <Typography sx={{ mt: 1, color: tokens.color.text.muted, fontSize: 12.5 }}>
          No ledger entries yet.
        </Typography>
      ) : (
        <Stack spacing={0.5} sx={{ mt: 1 }}>
          {entries.slice(0, 8).map((entry) => {
            const isIn = entry.direction.toLowerCase() === "credit";
            return (
              <Stack
                key={entry.id}
                direction="row"
                spacing={1}
                alignItems="center"
                sx={{
                  px: 1,
                  py: 0.5,
                  bgcolor: tokens.color.bg.inset,
                  border: `1px solid ${tokens.color.border.subtle}`,
                  borderRadius: 0.75,
                }}
              >
                <Typography
                  sx={{
                    fontFamily: tokens.font.mono,
                    fontSize: 10.5,
                    color: tokens.color.text.muted,
                    whiteSpace: "nowrap",
                  }}
                >
                  {relativeTime(entry.createdAt)}
                </Typography>
                <Typography
                  sx={{
                    flex: 1,
                    fontFamily: tokens.font.mono,
                    fontSize: 11.5,
                    color: tokens.color.text.secondary,
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                  }}
                >
                  {entry.entryType}
                </Typography>
                <Typography
                  sx={{
                    fontFamily: tokens.font.mono,
                    fontWeight: 700,
                    fontSize: 11.5,
                    color: isIn
                      ? tokens.color.accent.success
                      : tokens.color.accent.coral,
                  }}
                >
                  {isIn ? "+" : "−"}
                  {formatMoney(Math.abs(entry.amountUSD))}
                </Typography>
              </Stack>
            );
          })}
        </Stack>
      )}
    </Box>
  );
}

function verdictTone(
  decision: string,
): "success" | "warning" | "danger" | "info" | "neutral" {
  const s = decision.toLowerCase();
  if (s === "proceed" || s === "admit" || s === "allow") return "success";
  if (s === "degrade" || s === "downgrade" || s === "pause") return "warning";
  if (s === "kill_branch" || s === "block" || s === "reject") return "danger";
  return "info";
}

// ─── bottom toolbar ───────────────────────────────────────────────

function BottomToolbar({ executionID }: { executionID: string }) {
  const [copied, setCopied] = useState<string | null>(null);

  const copy = useCallback(async (label: string, value: string) => {
    if (typeof navigator === "undefined" || !navigator.clipboard) return;
    try {
      await navigator.clipboard.writeText(value);
      setCopied(label);
      setTimeout(() => setCopied(null), 1400);
    } catch {
      // clipboard refused — silently no-op
    }
  }, []);

  const curl = useMemo(() => {
    // Best-effort GraphQL re-run snippet. The orchestrator endpoint
    // varies per environment; this is a copyable template.
    const body = JSON.stringify({
      query:
        "query Execution($id: ID!) { execution(id: $id) { id status spentUSD budgetUSD } }",
      variables: { id: executionID },
    });
    return `curl -sS https://YOUR_ORCHESTRATOR/graphql \\
  -H 'authorization: Bearer YOUR_TOKEN' \\
  -H 'content-type: application/json' \\
  --data '${body}'`;
  }, [executionID]);

  return (
    <Box
      sx={{
        position: "sticky",
        bottom: 0,
        zIndex: 3,
        mt: 3,
        mx: { xs: -2, md: -4 },
        px: { xs: 2, md: 4 },
        py: 1,
        bgcolor: `${tokens.color.bg.surface}d9`,
        backdropFilter: "blur(14px) saturate(140%)",
        borderTop: `1px solid ${tokens.color.border.subtle}`,
      }}
    >
      <Stack
        direction="row"
        spacing={1}
        alignItems="center"
        sx={{ flexWrap: "wrap", rowGap: 1 }}
      >
        <Tooltip title="Copy execution id" arrow>
          <Button
            size="small"
            variant="text"
            startIcon={<ContentCopyRounded sx={{ fontSize: 14 }} />}
            onClick={() => copy("id", executionID)}
            sx={{
              color: tokens.color.text.secondary,
              fontFamily: tokens.font.mono,
              fontSize: 11.5,
            }}
          >
            {copied === "id" ? "copied" : shortId(executionID)}
          </Button>
        </Tooltip>
        <Tooltip title="Copy curl to re-run via GraphQL" arrow>
          <Button
            size="small"
            variant="text"
            startIcon={<TerminalRounded sx={{ fontSize: 14 }} />}
            onClick={() => copy("curl", curl)}
            sx={{
              color: tokens.color.text.secondary,
              fontFamily: tokens.font.mono,
              fontSize: 11.5,
            }}
          >
            {copied === "curl" ? "copied curl" : "copy curl"}
          </Button>
        </Tooltip>
        <Box sx={{ flex: 1 }} />
        <Button
          component={Link}
          href="https://github.com/ironflyer/ironflyer/blob/main/docs/V22_PLAN.md"
          target="_blank"
          rel="noopener noreferrer"
          size="small"
          variant="text"
          endIcon={<LaunchRounded sx={{ fontSize: 14 }} />}
          sx={{
            color: tokens.color.text.secondary,
            fontFamily: tokens.font.mono,
            fontSize: 11.5,
          }}
        >
          V22 plan
        </Button>
      </Stack>
    </Box>
  );
}
