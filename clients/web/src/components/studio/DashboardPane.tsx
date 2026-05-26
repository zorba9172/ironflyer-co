"use client";

// DashboardPane — execution-level KPI grid: cost vs budget, gates
// passed, completion score, gross margin. Below the grid we render a
// terse event timeline pulled from the chat buffer (system + costtick
// entries) and a profit summary card.

import {
  AccountTreeRounded,
  AutorenewRounded,
  BoltRounded,
  CheckCircleOutlineRounded,
  ChatBubbleOutlineRounded,
  ChevronLeftRounded,
  ChevronRightRounded,
  DifferenceRounded,
  Inventory2Rounded,
  WarningAmberRounded,
} from "@mui/icons-material";
import {
  Box,
  Button,
  Card,
  Divider,
  LinearProgress,
  Stack,
  Typography,
} from "@mui/material";
import { useMemo, useState, type ReactNode } from "react";
import { CostBreakdownBar } from "../charts/CostBreakdownBar";
import { LoadingPanel } from "../cockpit/LoadingPanel";
import { MetricCard } from "../cockpit/MetricCard";
import { MoneyChip } from "../cockpit/MoneyChip";
import { StatusBadge } from "../cockpit/StatusBadge";
import { relativeTime } from "../../lib/relativeTime";
import {
  useRerunGateMutation,
  useRunFinisherMutation,
  useExecutionSupportBundleQuery,
  useProjectFilesQuery,
  type ExecutionCoreFragment,
} from "../../lib/gql/__generated__";
import { extractErrorMessage } from "../../lib/errors";
import {
  buildProjectIntelligence,
  type ComponentSignal,
  type TechSlice,
} from "../../lib/projectIntelligence";
import { pushToast } from "../../lib/stores/uiStore";
import { tokens } from "../../theme";
import type { StudioMessage } from "./types";

export interface DashboardPaneProps {
  projectID: string;
  execution: ExecutionCoreFragment;
  messages: StudioMessage[];
  leftRailOpen?: boolean;
  chatOpen?: boolean;
  dockOpen?: boolean;
  onToggleLeftRail?: () => void;
  onToggleChat?: () => void;
  onToggleDock?: () => void;
  onRequestAreaClose?: (message: string) => Promise<void>;
}

const TERMINAL = new Set(["succeeded", "failed", "stopped", "killed", "refunded"]);


type AreaHealth = "working" | "open" | "closed";

interface LiveArea {
  key: string;
  label: string;
  status: AreaHealth;
  openIssues: number;
  active: boolean;
  gateName?: string;
  hint?: string;
}

export function DashboardPane({
  projectID,
  execution,
  messages,
  leftRailOpen,
  chatOpen,
  dockOpen,
  onToggleLeftRail,
  onToggleChat,
  onToggleDock,
  onRequestAreaClose,
}: DashboardPaneProps) {
  const isTerminal = TERMINAL.has(execution.status);
  const [rerunGate] = useRerunGateMutation();
  const [runFinisher, runFinisherState] = useRunFinisherMutation();
  const [activeAreaAction, setActiveAreaAction] = useState<string | null>(null);
  const query = useExecutionSupportBundleQuery({
    variables: { executionID: execution.id },
    pollInterval: isTerminal ? 0 : 5000,
    fetchPolicy: "cache-and-network",
  });
  const projectFilesQuery = useProjectFilesQuery({
    variables: { id: projectID },
    skip: !projectID,
    fetchPolicy: "cache-and-network",
  });
  const bundle = query.data?.executionSupportBundle;
  const intelligence = useMemo(
    () => buildProjectIntelligence(projectFilesQuery.data?.projectFiles ?? []),
    [projectFilesQuery.data],
  );

  const gates = bundle?.gateReport.stages ?? [];
  const passedCount = gates.filter(
    (g) => g.status === "passed" || g.status === "pass",
  ).length;
  const totalGates = gates.length;
  const completion = (bundle?.gateReport.completionScore ?? execution.completionScore) || 0;
  const margin =
    bundle?.costReport.grossMarginPct ?? execution.grossMarginPct ?? null;

  const budget = execution.budgetUSD;
  const spent = execution.spentUSD;

  // Timeline = system + costtick entries, newest last.
  const timeline = useMemo(
    () =>
      messages.filter(
        (m) => m.role === "system" || m.role === "costtick",
      ),
    [messages],
  );

  const liveAreas = useMemo(() => buildLiveAreas(gates, messages), [gates, messages]);
  const activeArea = liveAreas.find((a) => a.active) ?? null;
  const openAreaCount = liveAreas.filter((a) => a.status === "open").length;
  const closedAreaCount = liveAreas.filter((a) => a.status === "closed").length;

  const handleRunFinisher = async () => {
    if (!projectID || runFinisherState.loading) return;
    try {
      await runFinisher({ variables: { id: projectID } });
      pushToast({
        message: "Finisher loop restarted for this project.",
        severity: "success",
      });
      await query.refetch();
    } catch (e) {
      pushToast({ message: extractErrorMessage(e), severity: "error" });
    }
  };

  const handleAreaClose = async (area: LiveArea) => {
    if (!projectID || activeAreaAction) return;
    setActiveAreaAction(area.key);
    try {
      if (area.gateName) {
        await rerunGate({
          variables: {
            input: {
              projectId: projectID,
              gate: area.gateName,
            },
          },
        });
        pushToast({
          message: `Gate rerun started: ${area.label}`,
          severity: "success",
        });
      } else if (onRequestAreaClose) {
        await onRequestAreaClose(
          `Close area ${area.label}. Resolve all open issues, rerun checks, and report what is still not closed.`,
        );
        pushToast({
          message: `Close request sent for ${area.label}.`,
          severity: "info",
        });
      } else {
        pushToast({
          message: `No direct action is available for ${area.label}.`,
          severity: "warning",
        });
      }
      await query.refetch();
    } catch (e) {
      pushToast({ message: extractErrorMessage(e), severity: "error" });
    } finally {
      setActiveAreaAction(null);
    }
  };

  if (!bundle && query.loading) {
    return <LoadingPanel label="Loading support bundle…" minHeight="100%" />;
  }

  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.base,
        height: "100%",
        overflowY: "auto",
        p: 2.5,
      }}
    >
      <Stack spacing={2.5}>
        <Stack direction="row" alignItems="center" spacing={1.5}>
          <Typography
            sx={{
              color: tokens.color.text.primary,
              fontSize: 18,
              fontWeight: 800,
              letterSpacing: -0.2,
            }}
          >
            Execution dashboard
          </Typography>
          <StatusBadge status={execution.status} />
          <Box sx={{ flex: 1 }} />
          <Stack
            direction="row"
            spacing={0.75}
            sx={{
              alignItems: "center",
              display: { xs: "none", md: "flex" },
            }}
          >
            {onToggleLeftRail ? (
              <DashboardToggleButton
                active={leftRailOpen !== false}
                icon={
                  leftRailOpen === false ? (
                    <ChevronRightRounded sx={{ fontSize: 16 }} />
                  ) : (
                    <ChevronLeftRounded sx={{ fontSize: 16 }} />
                  )
                }
                label={leftRailOpen === false ? "Expand sidebar" : "Collapse sidebar"}
                onClick={onToggleLeftRail}
              />
            ) : null}
            {onToggleChat ? (
              <DashboardToggleButton
                active={chatOpen !== false}
                icon={<ChatBubbleOutlineRounded sx={{ fontSize: 16 }} />}
                label={chatOpen === false ? "Open chat" : "Hide chat"}
                onClick={onToggleChat}
              />
            ) : null}
            {onToggleDock ? (
              <DashboardToggleButton
                active={dockOpen === true}
                icon={<DifferenceRounded sx={{ fontSize: 16 }} />}
                label={dockOpen ? "Hide dock" : "Open dock"}
                onClick={onToggleDock}
              />
            ) : null}
          </Stack>
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
              letterSpacing: 0.6,
              textTransform: "uppercase",
            }}
          >
            id · {execution.id.slice(0, 8)}
          </Typography>
        </Stack>

        <Card sx={{ p: 2 }}>
          <CostBreakdownBar
            budgetUSD={budget}
            providerCostUSD={bundle?.costReport.providerCostUSD ?? execution.providerCostUSD}
            sandboxCostUSD={bundle?.costReport.sandboxCostUSD ?? execution.sandboxCostUSD}
            storageCostUSD={bundle?.costReport.storageCostUSD ?? execution.storageCostUSD}
            deploymentCostUSD={bundle?.costReport.deploymentCostUSD ?? execution.deploymentCostUSD}
            spentUSD={spent}
          />
        </Card>

        <Box
          sx={{
            display: "grid",
            gap: 1.5,
            gridTemplateColumns: {
              xs: "1fr",
              sm: "repeat(3, 1fr)",
            },
          }}
        >
          <MetricCard
            label="Gates"
            value={
              totalGates > 0
                ? `${passedCount} / ${totalGates}`
                : "—"
            }
            hint="Profit + correctness + security"
            accent="sky"
          />
          <MetricCard
            label="Completion"
            value={`${(completion * 100).toFixed(0)}%`}
            hint="Finisher score"
            accent={completion >= 0.9 ? "lime" : "yellow"}
          />
          <MetricCard
            label="Gross margin"
            value={margin === null ? "—" : `${(margin * 100).toFixed(1)}%`}
            hint="revenue − provider cost"
            accent={margin !== null && margin >= 0.2 ? "lime" : "coral"}
          />
        </Box>

        <ProjectIntelligenceCard
          intelligence={intelligence}
          loading={projectFilesQuery.loading && !projectFilesQuery.data}
        />

        <Card sx={{ p: 2 }}>
          <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1.5 }}>
            <AccountTreeRounded sx={{ color: tokens.color.accent.violet, fontSize: 18 }} />
            <Typography
              variant="overline"
              sx={{ color: tokens.color.text.secondary, letterSpacing: 0.7 }}
            >
              Workflow + Finisher Loop Live
            </Typography>
            <Box sx={{ flex: 1 }} />
            <Button
              size="small"
              onClick={handleRunFinisher}
              disabled={runFinisherState.loading || !projectID}
              startIcon={<AutorenewRounded sx={{ fontSize: 15 }} />}
              sx={{
                border: `1px solid ${tokens.color.border.subtle}`,
                bgcolor: tokens.color.bg.surfaceRaised,
                color: tokens.color.text.secondary,
                fontSize: 11,
                fontWeight: 800,
                minHeight: 28,
                px: 1,
                textTransform: "none",
                "&:hover": {
                  bgcolor: tokens.color.bg.surfaceHover,
                  borderColor: tokens.color.border.accent,
                  color: tokens.color.text.primary,
                },
              }}
            >
              Run finisher
            </Button>
          </Stack>

          <Box
            sx={{
              display: "grid",
              gap: 1,
              gridTemplateColumns: { xs: "1fr", md: "repeat(3, 1fr)" },
              mb: 1.5,
            }}
          >
            <MiniHealthCard
              label="Active now"
              value={activeArea?.label ?? "Waiting"}
              tone={activeArea ? "accent" : "muted"}
            />
            <MiniHealthCard
              label="Open areas"
              value={String(openAreaCount)}
              tone={openAreaCount > 0 ? "danger" : "accent"}
            />
            <MiniHealthCard
              label="Closed areas"
              value={String(closedAreaCount)}
              tone="accent"
            />
          </Box>

          {activeAreaAction ? (
            <LinearProgress
              sx={{
                mb: 1.25,
                bgcolor: `${tokens.color.accent.violet}1f`,
                "& .MuiLinearProgress-bar": { backgroundColor: tokens.color.accent.violet },
              }}
            />
          ) : null}

          {liveAreas.length === 0 ? (
            <Typography sx={{ color: tokens.color.text.muted, fontSize: 13 }}>
              Waiting for the first workflow event.
            </Typography>
          ) : (
            <Stack spacing={0.7}>
              {liveAreas.map((area) => {
                const busy = activeAreaAction === area.key;
                return (
                  <Stack
                    key={area.key}
                    direction="row"
                    spacing={1}
                    alignItems="center"
                    sx={{
                      border: `1px solid ${tokens.color.border.subtle}`,
                      bgcolor: tokens.color.bg.inset,
                      borderRadius: 1,
                      px: 1,
                      py: 0.8,
                    }}
                  >
                    <AreaIcon status={area.status} active={area.active} />
                    <Box sx={{ flex: 1, minWidth: 0 }}>
                      <Typography
                        sx={{
                          color: tokens.color.text.primary,
                          fontFamily: tokens.font.mono,
                          fontSize: 12,
                          fontWeight: 700,
                          overflow: "hidden",
                          textOverflow: "ellipsis",
                          whiteSpace: "nowrap",
                        }}
                      >
                        {area.label}
                      </Typography>
                      <Stack direction="row" spacing={0.8} alignItems="center" sx={{ mt: 0.2 }}>
                        <StatusBadge status={area.status} uppercase={false} />
                        <Typography
                          sx={{
                            color: tokens.color.text.muted,
                            fontFamily: tokens.font.mono,
                            fontSize: 10.5,
                            letterSpacing: 0.3,
                          }}
                        >
                          {area.openIssues} open
                        </Typography>
                        {area.hint ? (
                          <Typography
                            sx={{
                              color: tokens.color.text.muted,
                              fontSize: 10.5,
                              overflow: "hidden",
                              textOverflow: "ellipsis",
                              whiteSpace: "nowrap",
                            }}
                          >
                            {area.hint}
                          </Typography>
                        ) : null}
                      </Stack>
                    </Box>
                    <Button
                      size="small"
                      onClick={() => void handleAreaClose(area)}
                      disabled={busy || activeAreaAction !== null || !projectID}
                      startIcon={<BoltRounded sx={{ fontSize: 14 }} />}
                      sx={{
                        border: `1px solid ${tokens.color.border.subtle}`,
                        bgcolor: tokens.color.bg.surfaceRaised,
                        color: tokens.color.text.secondary,
                        fontSize: 11,
                        fontWeight: 800,
                        minHeight: 27,
                        px: 1,
                        textTransform: "none",
                        whiteSpace: "nowrap",
                        "&:hover": {
                          bgcolor: tokens.color.bg.surfaceHover,
                          borderColor: tokens.color.border.accent,
                          color: tokens.color.text.primary,
                        },
                      }}
                    >
                      {area.status === "closed" ? "Re-check" : "Close area"}
                    </Button>
                  </Stack>
                );
              })}
            </Stack>
          )}
        </Card>

        <Card sx={{ p: 2 }}>
          <Stack
            direction="row"
            alignItems="center"
            justifyContent="space-between"
            sx={{ mb: 1.5 }}
          >
            <Typography
              variant="overline"
              sx={{ color: tokens.color.text.secondary }}
            >
              Gate Report
            </Typography>
            <Typography
              sx={{
                color: tokens.color.text.muted,
                fontFamily: tokens.font.mono,
                fontSize: 11,
              }}
            >
              {totalGates} stages
            </Typography>
          </Stack>
          {totalGates === 0 ? (
            <Typography sx={{ color: tokens.color.text.muted, fontSize: 13 }}>
              No gate verdicts yet. The first verdict will land as soon as the
              orchestrator emits one.
            </Typography>
          ) : (
            <Stack divider={<Divider sx={{ borderColor: tokens.color.border.subtle }} />}>
              {gates.map((g) => (
                <Stack
                  key={`${g.name}_${g.status}`}
                  direction="row"
                  alignItems="center"
                  spacing={1}
                  sx={{ py: 0.75 }}
                >
                  <Typography
                    sx={{
                      color: tokens.color.text.primary,
                      flex: 1,
                      fontFamily: tokens.font.mono,
                      fontSize: 12.5,
                    }}
                  >
                    {g.name}
                  </Typography>
                  <Typography
                    sx={{
                      color: tokens.color.text.muted,
                      fontFamily: tokens.font.mono,
                      fontSize: 11,
                    }}
                  >
                    {g.issuesCount} issue{g.issuesCount === 1 ? "" : "s"}
                  </Typography>
                  <StatusBadge status={g.status} />
                </Stack>
              ))}
            </Stack>
          )}
        </Card>

        <Box
          sx={{
            display: "grid",
            gap: 1.5,
            gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
          }}
        >
          <Card sx={{ p: 2 }}>
            <Typography
              variant="overline"
              sx={{ color: tokens.color.text.secondary, mb: 1, display: "block" }}
            >
              Profit summary
            </Typography>
            <Stack spacing={1.25}>
              <Row label="Revenue" value={bundle?.costReport.revenueUSD ?? execution.revenueUSD} positive />
              <Row label="Provider cost" value={-(bundle?.costReport.providerCostUSD ?? execution.providerCostUSD)} negative />
              <Row label="Sandbox cost" value={-(bundle?.costReport.sandboxCostUSD ?? execution.sandboxCostUSD)} negative />
              <Row label="Storage cost" value={-(bundle?.costReport.storageCostUSD ?? execution.storageCostUSD)} negative />
              <Row label="Deployment cost" value={-(bundle?.costReport.deploymentCostUSD ?? execution.deploymentCostUSD)} negative />
              <Divider sx={{ borderColor: tokens.color.border.subtle }} />
              <Stack direction="row" alignItems="center">
                <Typography sx={{ color: tokens.color.text.primary, flex: 1, fontWeight: 700 }}>
                  Margin
                </Typography>
                <Typography
                  sx={{
                    color:
                      margin !== null && margin >= 0
                        ? tokens.color.accent.success
                        : tokens.color.accent.danger,
                    fontFamily: tokens.font.mono,
                    fontWeight: 800,
                  }}
                >
                  {margin === null ? "—" : `${(margin * 100).toFixed(1)}%`}
                </Typography>
              </Stack>
            </Stack>
          </Card>

          <Card sx={{ p: 2 }}>
            <Typography
              variant="overline"
              sx={{ color: tokens.color.text.secondary, mb: 1, display: "block" }}
            >
              Security
            </Typography>
            {bundle ? (
              <Stack spacing={1}>
                <Stack direction="row" alignItems="center" spacing={1}>
                  <Typography sx={{ color: tokens.color.text.muted, flex: 1, fontSize: 12 }}>
                    Pass rate
                  </Typography>
                  <Typography
                    sx={{
                      color: tokens.color.text.primary,
                      fontFamily: tokens.font.mono,
                      fontWeight: 800,
                    }}
                  >
                    {(bundle.securityReport.passRate * 100).toFixed(0)}%
                  </Typography>
                </Stack>
                <Stack direction="row" alignItems="center" spacing={1}>
                  <Typography sx={{ color: tokens.color.text.muted, flex: 1, fontSize: 12 }}>
                    Blocked deploy
                  </Typography>
                  <StatusBadge
                    status={bundle.securityReport.blockedDeploy ? "fail" : "pass"}
                  />
                </Stack>
                <Stack direction="row" alignItems="center" spacing={1}>
                  <Typography sx={{ color: tokens.color.text.muted, flex: 1, fontSize: 12 }}>
                    Findings
                  </Typography>
                  <Typography
                    sx={{
                      color: tokens.color.text.primary,
                      fontFamily: tokens.font.mono,
                      fontWeight: 800,
                    }}
                  >
                    {bundle.securityReport.findings.length}
                  </Typography>
                </Stack>
              </Stack>
            ) : (
              <Typography sx={{ color: tokens.color.text.muted, fontSize: 13 }}>
                Security report not generated yet.
              </Typography>
            )}
          </Card>
        </Box>

        <Card sx={{ p: 2 }}>
          <Typography
            variant="overline"
            sx={{ color: tokens.color.text.secondary, mb: 1.5, display: "block" }}
          >
            Timeline
          </Typography>
          {timeline.length === 0 ? (
            <Typography sx={{ color: tokens.color.text.muted, fontSize: 13 }}>
              No execution events yet.
            </Typography>
          ) : (
            <Stack spacing={0.6}>
              {timeline.map((m) => (
                <Stack
                  key={`tl_${m.id}`}
                  direction="row"
                  spacing={1.5}
                  sx={{ alignItems: "baseline" }}
                >
                  <Typography
                    sx={{
                      color: tokens.color.text.muted,
                      fontFamily: tokens.font.mono,
                      fontSize: 10.5,
                      letterSpacing: 0.2,
                      minWidth: 78,
                    }}
                  >
                    {relativeTime(m.createdAt)}
                  </Typography>
                  <Typography
                    sx={{
                      color:
                        m.role === "costtick"
                          ? tokens.color.accent.violet
                          : tokens.color.text.secondary,
                      flex: 1,
                      fontSize: 12.5,
                      lineHeight: 1.5,
                    }}
                  >
                    {m.body}
                  </Typography>
                </Stack>
              ))}
            </Stack>
          )}
        </Card>
      </Stack>
    </Box>
  );
}

function ProjectIntelligenceCard({
  intelligence,
  loading,
}: {
  intelligence: ReturnType<typeof buildProjectIntelligence>;
  loading: boolean;
}) {
  const topLanguages = intelligence.languages.slice(0, 4);
  const components = intelligence.components.slice(0, 7);
  return (
    <Card sx={{ p: 2 }}>
      <Stack
        direction={{ xs: "column", md: "row" }}
        spacing={1.5}
        sx={{ alignItems: { xs: "stretch", md: "center" } }}
      >
        <Stack spacing={0.35} sx={{ minWidth: { md: 190 } }}>
          <Stack direction="row" spacing={1} alignItems="center">
            <Inventory2Rounded sx={{ color: tokens.color.accent.sky, fontSize: 18 }} />
            <Typography
              variant="overline"
              sx={{ color: tokens.color.text.secondary, lineHeight: 1.1 }}
            >
              Project intelligence
            </Typography>
          </Stack>
          <Typography
            sx={{
              color: tokens.color.text.primary,
              fontSize: 15,
              fontWeight: 900,
              lineHeight: 1.25,
            }}
          >
            {loading ? "Reading project" : intelligence.primaryStack}
          </Typography>
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
            }}
          >
            {intelligence.totalFiles} files · {formatBytes(intelligence.codeBytes)}
          </Typography>
        </Stack>

        <Box sx={{ flex: 1, minWidth: 0 }}>
          {topLanguages.length === 0 ? (
            <Typography sx={{ color: tokens.color.text.muted, fontSize: 12.5 }}>
              No source profile yet.
            </Typography>
          ) : (
            <Stack spacing={0.8}>
              <Stack direction="row" spacing={0.4} sx={{ height: 8, overflow: "hidden", borderRadius: 1 }}>
                {topLanguages.map((lang) => (
                  <Box
                    key={lang.key}
                    sx={{
                      bgcolor: lang.color,
                      flexBasis: `${Math.max(lang.percent, 4)}%`,
                      minWidth: 10,
                    }}
                  />
                ))}
              </Stack>
              <Stack direction="row" spacing={0.7} sx={{ flexWrap: "wrap", rowGap: 0.7 }}>
                {topLanguages.map((lang) => (
                  <TechPill key={lang.key} lang={lang} />
                ))}
              </Stack>
            </Stack>
          )}
        </Box>

        <Stack
          direction="row"
          spacing={0.6}
          sx={{
            flexWrap: "wrap",
            justifyContent: { xs: "flex-start", md: "flex-end" },
            maxWidth: { md: 360 },
            rowGap: 0.6,
          }}
        >
          {components.length === 0 ? (
            <Typography sx={{ color: tokens.color.text.muted, fontSize: 12.5 }}>
              Components will appear after files land.
            </Typography>
          ) : (
            components.map((component) => (
              <ComponentPill key={component.key} component={component} />
            ))
          )}
        </Stack>
      </Stack>
      <Typography
        sx={{
          color: tokens.color.text.muted,
          fontSize: 12,
          lineHeight: 1.45,
          mt: 1.25,
        }}
      >
        {intelligence.insight}
      </Typography>
    </Card>
  );
}

function TechPill({ lang }: { lang: TechSlice }) {
  return (
    <Stack
      direction="row"
      spacing={0.55}
      alignItems="center"
      sx={{
        border: `1px solid ${tokens.color.border.subtle}`,
        bgcolor: tokens.color.bg.inset,
        borderRadius: 1,
        minHeight: 26,
        px: 0.75,
      }}
    >
      <BadgeIcon label={lang.icon} color={lang.color} />
      <Typography sx={{ color: tokens.color.text.primary, fontSize: 11.5, fontWeight: 800 }}>
        {lang.label}
      </Typography>
      <Typography sx={{ color: tokens.color.text.muted, fontFamily: tokens.font.mono, fontSize: 10.5 }}>
        {lang.percent}%
      </Typography>
    </Stack>
  );
}

function ComponentPill({ component }: { component: ComponentSignal }) {
  const color = componentToneColor(component.tone);
  return (
    <Stack
      direction="row"
      spacing={0.5}
      alignItems="center"
      sx={{
        border: `1px solid ${tokens.color.border.subtle}`,
        bgcolor: tokens.color.bg.surfaceRaised,
        borderRadius: 1,
        minHeight: 25,
        px: 0.7,
      }}
    >
      <BadgeIcon label={component.icon} color={color} />
      <Typography sx={{ color: tokens.color.text.secondary, fontSize: 11.2, fontWeight: 800 }}>
        {component.label}
      </Typography>
    </Stack>
  );
}

function BadgeIcon({ label, color }: { label: string; color: string }) {
  return (
    <Box
      aria-hidden
      sx={{
        alignItems: "center",
        bgcolor: `${color}22`,
        border: `1px solid ${color}66`,
        borderRadius: 0.75,
        color,
        display: "inline-flex",
        fontFamily: tokens.font.mono,
        fontSize: 9,
        fontWeight: 900,
        height: 18,
        justifyContent: "center",
        lineHeight: 1,
        minWidth: 18,
        px: 0.35,
      }}
    >
      {label}
    </Box>
  );
}

function componentToneColor(tone: ComponentSignal["tone"]): string {
  switch (tone) {
    case "data":
      return tokens.color.accent.sky;
    case "deploy":
      return tokens.color.accent.lime;
    case "security":
      return tokens.color.accent.yellow;
    case "primary":
      return tokens.color.accent.violet;
    default:
      return tokens.color.text.muted;
  }
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes}B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)}KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)}MB`;
}

function buildLiveAreas(
  gates: Array<{ name: string; status: string; issuesCount: number }>,
  messages: StudioMessage[],
): LiveArea[] {
  const byKey = new Map<string, LiveArea>();

  for (const gate of gates) {
    const key = normalizeAreaKey(gate.name);
    const status = gateStatusToHealth(gate.status, gate.issuesCount);
    byKey.set(key, {
      key,
      label: gate.name,
      status,
      openIssues: Math.max(0, gate.issuesCount),
      active: status === "working",
      gateName: gate.name,
      hint: "gate",
    });
  }

  for (const m of messages) {
    if (
      (m.role !== "agent_progress" && m.role !== "agent_action" && m.role !== "agent_result") ||
      !m.stage
    ) {
      continue;
    }
    const key = normalizeAreaKey(m.stage);
    const prev = byKey.get(key);
    const stageLabel = prettifyStage(m.stage);
    const stageOpenIssues = prev?.openIssues ?? 0;

    let status: AreaHealth = prev?.status ?? "open";
    if (m.inProgress) {
      status = "working";
    } else if (m.role === "agent_result" && m.success === true && stageOpenIssues === 0) {
      status = "closed";
    } else if (m.role === "agent_result" && m.success === false) {
      status = "open";
    }

    byKey.set(key, {
      key,
      label: prev?.label ?? stageLabel,
      status,
      openIssues: stageOpenIssues,
      active: m.inProgress === true,
      gateName: prev?.gateName,
      hint: m.role === "agent_action" ? "finisher" : prev?.hint,
    });
  }

  return Array.from(byKey.values()).sort((a, b) => {
    const scoreA = areaPriority(a);
    const scoreB = areaPriority(b);
    if (scoreA !== scoreB) return scoreA - scoreB;
    return a.label.localeCompare(b.label);
  });
}

function normalizeAreaKey(raw: string): string {
  return raw.trim().toLowerCase().replace(/[^a-z0-9]+/g, "_");
}

function prettifyStage(stage: string): string {
  const raw = stage.trim();
  if (!raw) return "area";
  return raw
    .replace(/[._-]+/g, " ")
    .replace(/\s+/g, " ")
    .replace(/^\w/, (c) => c.toUpperCase());
}

function gateStatusToHealth(status: string, issuesCount: number): AreaHealth {
  const s = status.toLowerCase();
  if (s === "running" || s === "pending") return "working";
  if (s === "pass" || s === "passed" || s === "ok") {
    return issuesCount > 0 ? "open" : "closed";
  }
  if (s === "fail" || s === "failed" || s === "blocked" || s === "warn") {
    return "open";
  }
  return issuesCount > 0 ? "open" : "working";
}

function areaPriority(area: LiveArea): number {
  if (area.active) return 0;
  if (area.status === "working") return 1;
  if (area.status === "open") return 2;
  return 3;
}

function MiniHealthCard({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone: "accent" | "danger" | "muted";
}) {
  const fg =
    tone === "accent"
      ? tokens.color.accent.success
      : tone === "danger"
        ? tokens.color.accent.danger
        : tokens.color.text.secondary;
  const bg =
    tone === "accent"
      ? `${tokens.color.accent.success}1a`
      : tone === "danger"
        ? `${tokens.color.accent.danger}1a`
        : tokens.color.bg.inset;

  return (
    <Box
      sx={{
        border: `1px solid ${tokens.color.border.subtle}`,
        bgcolor: bg,
        borderRadius: 1,
        px: 1,
        py: 0.85,
      }}
    >
      <Typography
        sx={{
          color: tokens.color.text.muted,
          fontFamily: tokens.font.mono,
          fontSize: 10,
          letterSpacing: 0.4,
          textTransform: "uppercase",
        }}
      >
        {label}
      </Typography>
      <Typography
        sx={{
          color: fg,
          fontSize: 12.5,
          fontWeight: 800,
          lineHeight: 1.3,
          mt: 0.25,
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
        }}
      >
        {value}
      </Typography>
    </Box>
  );
}

function AreaIcon({
  status,
  active,
}: {
  status: AreaHealth;
  active: boolean;
}) {
  if (active || status === "working") {
    return <AutorenewRounded sx={{ color: tokens.color.accent.violet, fontSize: 17 }} />;
  }
  if (status === "closed") {
    return <CheckCircleOutlineRounded sx={{ color: tokens.color.accent.success, fontSize: 17 }} />;
  }
  return <WarningAmberRounded sx={{ color: tokens.color.accent.warning, fontSize: 17 }} />;
}

function DashboardToggleButton({
  active,
  icon,
  label,
  onClick,
}: {
  active: boolean;
  icon: ReactNode;
  label: string;
  onClick: () => void;
}) {
  return (
    <Button
      size="small"
      onClick={onClick}
      startIcon={icon}
      sx={{
        border: `1px solid ${active ? tokens.color.border.strong : tokens.color.border.subtle}`,
        bgcolor: active ? `${tokens.color.accent.purple}1f` : tokens.color.bg.surfaceRaised,
        color: active ? tokens.color.text.primary : tokens.color.text.secondary,
        fontSize: 11.5,
        fontWeight: 800,
        minHeight: 30,
        px: 1,
        whiteSpace: "nowrap",
        "& .MuiButton-startIcon": { mr: 0.5 },
        "&:hover": {
          bgcolor: tokens.color.bg.surfaceHover,
          borderColor: tokens.color.border.accent,
          color: tokens.color.text.primary,
        },
      }}
    >
      {label}
    </Button>
  );
}

function Row({
  label,
  value,
  positive,
  negative,
}: {
  label: string;
  value: number;
  positive?: boolean;
  negative?: boolean;
}) {
  return (
    <Stack direction="row" alignItems="center">
      <Typography sx={{ color: tokens.color.text.muted, flex: 1, fontSize: 12 }}>
        {label}
      </Typography>
      <MoneyChip
        amountUSD={value}
        color={positive ? "positive" : negative ? "negative" : "neutral"}
      />
    </Stack>
  );
}
