"use client";

// ExecutionFlow — React Flow DAG for /execution/[id].
//
// Replaces the linear PhaseStepper with a real workflow diagram:
//
//   Queue → Plan → Patch → Build → Verify → Ship
//                                     │
//                                     ├── profit_gate
//                                     ├── correctness_gate
//                                     └── security_gate
//
// Each node is a self-contained MUI card with status color, count
// chip, and a rich tooltip explaining where we are, what completed,
// and what's missing to close the phase. The Verify branch fans out
// to the actual gate verdicts from the support bundle so a blocked
// gate is one hover away.
//
// Pan/zoom, MiniMap, and the Controls bar are kept on so the same
// component scales to larger workflows once Phase events grow beyond
// the canonical 6.
//
// The chart chunk + @xyflow/react CSS load lazily from
// next/dynamic at the call site — this file should never end up in
// the cold studio bundle.

import { useMemo, useEffect, useState } from "react";
import {
  Background,
  BackgroundVariant,
  Controls,
  MarkerType,
  MiniMap,
  ReactFlow,
  ReactFlowProvider,
  type Edge,
  type Node,
  type NodeProps,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import {
  Box,
  IconButton,
  Stack,
  Tooltip,
  Typography,
} from "@mui/material";
import {
  ExpandLessRounded,
  ExpandMoreRounded,
  ReportProblemRounded,
} from "@mui/icons-material";
import { tokens } from "../../theme";
import { formatMoney } from "../../lib/format";
import { relativeTime } from "../../lib/relativeTime";

// ----- Public surface ----------------------------------------------

export type PhaseStatus = "pending" | "active" | "done" | "failed";
export type GateVerdict = "pass" | "fail" | "blocked" | "skip" | "pending";

export interface ExecutionFlowPhase {
  key: string;
  label: string;
  status: PhaseStatus;
  // Latest event of this phase, used as the "what just happened" line.
  lastEvent?: { type: string; at: string; summary?: string } | null;
  // How many events this phase has logged so far.
  eventCount?: number;
  // Optional cost attributed to this phase.
  costUSD?: number;
  // The "what is missing to close" hint computed by the page.
  missing?: string;
}

export interface ExecutionFlowGate {
  name: string;
  status: GateVerdict;
  issuesCount?: number;
  // One-line verdict explanation pulled from ProfitGuard decisions
  // or the gate's own report.
  rationale?: string;
}

export interface ExecutionFlowProps {
  phases: ExecutionFlowPhase[];
  gates?: ExecutionFlowGate[];
  // Status of the whole execution, used to color the canvas border.
  status: string;
  // Total elapsed time on the run — shown in the canvas header chip.
  elapsedLabel?: string;
  // Optional click handler: when the operator clicks a phase node the
  // page can scroll the event timeline to that phase's first event.
  onPhaseClick?: (phaseKey: string) => void;
}

// ----- Color & status helpers --------------------------------------

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

function gateColor(s: GateVerdict): string {
  switch (s) {
    case "pass":
      return tokens.color.accent.success;
    case "fail":
      return tokens.color.accent.danger;
    case "blocked":
      return tokens.color.brand.amber;
    case "skip":
      return tokens.color.text.muted;
    default:
      return tokens.color.accent.violet;
  }
}

function statusGlyph(s: PhaseStatus): string {
  switch (s) {
    case "done":
      return "✓";
    case "active":
      return "●";
    case "failed":
      return "!";
    default:
      return "·";
  }
}

// ----- Tooltip body -------------------------------------------------

// PhaseTooltip — the rich hover content. Sections render only when
// the underlying data is present so the tooltip stays compact for
// phases with little signal.
function PhaseTooltip({
  phase,
  gates,
}: {
  phase: ExecutionFlowPhase;
  gates: ExecutionFlowGate[];
}) {
  const color = phaseColor(phase.status);
  const verifyBound = phase.key === "verify" && gates.length > 0;

  return (
    <Box sx={{ minWidth: 240, maxWidth: 340 }}>
      <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 0.5 }}>
        <Box
          sx={{
            width: 8,
            height: 8,
            borderRadius: 999,
            bgcolor: color,
          }}
        />
        <Typography
          sx={{
            color: tokens.color.text.primary,
            fontSize: 13,
            fontWeight: 800,
          }}
        >
          {phase.label}
        </Typography>
        <Typography
          sx={{
            color,
            fontFamily: tokens.font.mono,
            fontSize: 10.5,
            letterSpacing: 0.6,
            textTransform: "uppercase",
          }}
        >
          {phase.status}
        </Typography>
      </Stack>

      {phase.lastEvent && (
        <Stack spacing={0.25} sx={{ mb: 0.75 }}>
          <Typography
            variant="overline"
            sx={{ color: tokens.color.text.muted, letterSpacing: 1, fontSize: 10 }}
          >
            Last activity
          </Typography>
          <Typography
            sx={{
              color: tokens.color.text.secondary,
              fontFamily: tokens.font.mono,
              fontSize: 11.5,
            }}
          >
            {phase.lastEvent.type}{" "}
            <Box component="span" sx={{ color: tokens.color.text.muted }}>
              · {relativeTime(phase.lastEvent.at)}
            </Box>
          </Typography>
          {phase.lastEvent.summary && (
            <Typography
              sx={{ color: tokens.color.text.secondary, fontSize: 12 }}
            >
              {phase.lastEvent.summary}
            </Typography>
          )}
        </Stack>
      )}

      <Stack direction="row" spacing={1.5} sx={{ mb: 0.5 }}>
        {typeof phase.eventCount === "number" && (
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
            }}
          >
            {phase.eventCount} event{phase.eventCount === 1 ? "" : "s"}
          </Typography>
        )}
        {typeof phase.costUSD === "number" && phase.costUSD > 0 && (
          <Typography
            sx={{
              color: tokens.color.accent.coral,
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
            }}
          >
            {formatMoney(phase.costUSD)}
          </Typography>
        )}
      </Stack>

      {phase.missing && phase.status !== "done" && (
        <Box
          sx={{
            mt: 0.75,
            p: 0.75,
            borderRadius: 0.75,
            bgcolor: `${tokens.color.brand.amber}1a`,
            border: `1px solid ${tokens.color.brand.amber}55`,
          }}
        >
          <Typography
            variant="overline"
            sx={{
              color: tokens.color.brand.amber,
              letterSpacing: 1,
              fontSize: 10,
            }}
          >
            What's missing
          </Typography>
          <Typography
            sx={{ color: tokens.color.text.secondary, fontSize: 12 }}
          >
            {phase.missing}
          </Typography>
        </Box>
      )}

      {verifyBound && (
        <Box sx={{ mt: 0.75 }}>
          <Typography
            variant="overline"
            sx={{ color: tokens.color.text.muted, letterSpacing: 1, fontSize: 10 }}
          >
            Gates
          </Typography>
          <Stack spacing={0.4} sx={{ mt: 0.25 }}>
            {gates.map((g) => (
              <Stack
                key={g.name}
                direction="row"
                alignItems="center"
                spacing={0.75}
              >
                <Box
                  sx={{
                    width: 6,
                    height: 6,
                    borderRadius: 999,
                    bgcolor: gateColor(g.status),
                  }}
                />
                <Typography
                  sx={{
                    color: tokens.color.text.secondary,
                    fontFamily: tokens.font.mono,
                    fontSize: 11,
                    flex: 1,
                  }}
                >
                  {g.name}
                </Typography>
                <Typography
                  sx={{
                    color: gateColor(g.status),
                    fontFamily: tokens.font.mono,
                    fontSize: 10.5,
                    textTransform: "uppercase",
                  }}
                >
                  {g.status}
                </Typography>
              </Stack>
            ))}
          </Stack>
        </Box>
      )}
    </Box>
  );
}

function GateTooltip({ gate }: { gate: ExecutionFlowGate }) {
  const color = gateColor(gate.status);
  return (
    <Box sx={{ minWidth: 220, maxWidth: 320 }}>
      <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 0.5 }}>
        <Box sx={{ width: 8, height: 8, borderRadius: 999, bgcolor: color }} />
        <Typography
          sx={{ color: tokens.color.text.primary, fontSize: 13, fontWeight: 800 }}
        >
          {gate.name}
        </Typography>
        <Typography
          sx={{
            color,
            fontFamily: tokens.font.mono,
            fontSize: 10.5,
            letterSpacing: 0.6,
            textTransform: "uppercase",
          }}
        >
          {gate.status}
        </Typography>
      </Stack>
      {typeof gate.issuesCount === "number" && (
        <Typography
          sx={{
            color: tokens.color.text.muted,
            fontFamily: tokens.font.mono,
            fontSize: 11,
            mb: gate.rationale ? 0.5 : 0,
          }}
        >
          {gate.issuesCount} issue{gate.issuesCount === 1 ? "" : "s"} recorded
        </Typography>
      )}
      {gate.rationale && (
        <Typography sx={{ color: tokens.color.text.secondary, fontSize: 12 }}>
          {gate.rationale}
        </Typography>
      )}
    </Box>
  );
}

// ----- Custom nodes -------------------------------------------------

type PhaseNodeData = {
  phase: ExecutionFlowPhase;
  index: number;
  gates: ExecutionFlowGate[];
  onClick?: (key: string) => void;
};

type GateNodeData = {
  gate: ExecutionFlowGate;
};

function PhaseNode({ data }: NodeProps) {
  const { phase, index, gates, onClick } = data as PhaseNodeData;
  const color = phaseColor(phase.status);
  const isActive = phase.status === "active";

  return (
    <Tooltip
      arrow
      placement="bottom"
      enterDelay={120}
      title={<PhaseTooltip phase={phase} gates={gates} />}
      slotProps={{
        tooltip: {
          sx: {
            bgcolor: tokens.color.bg.surface,
            border: `1px solid ${tokens.color.border.strong}`,
            color: tokens.color.text.primary,
            p: 1.25,
            maxWidth: "none",
          },
        },
        arrow: { sx: { color: tokens.color.bg.surface } },
      }}
    >
      <Box
        onClick={() => onClick?.(phase.key)}
        sx={{
          width: 168,
          borderRadius: 1.5,
          bgcolor: tokens.color.bg.surfaceRaised,
          border: `1px solid ${color}66`,
          boxShadow: isActive
            ? `0 0 0 3px ${color}1f, 0 8px 24px ${tokens.color.bg.inset}`
            : `0 4px 14px ${tokens.color.bg.inset}`,
          cursor: onClick ? "pointer" : "default",
          transition: "transform 160ms ease, box-shadow 160ms ease",
          "&:hover": {
            transform: "translateY(-1px)",
            borderColor: `${color}aa`,
          },
          // Active pulse — same rhythm as the old stepper.
          animation: isActive
            ? "ironflyerPhasePulse 1.6s ease-in-out infinite"
            : "none",
          "@keyframes ironflyerPhasePulse": {
            "0%, 100%": { boxShadow: `0 0 0 0 ${color}00` },
            "50%": { boxShadow: `0 0 0 6px ${color}33` },
          },
        }}
      >
        <Stack
          direction="row"
          alignItems="center"
          spacing={1}
          sx={{
            px: 1.25,
            py: 0.75,
            borderBottom: `1px solid ${tokens.color.border.subtle}`,
          }}
        >
          <Box
            sx={{
              width: 22,
              height: 22,
              borderRadius: 999,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              bgcolor: phase.status === "done" ? color : `${color}1c`,
              color:
                phase.status === "done"
                  ? tokens.color.text.inverse
                  : color,
              fontFamily: tokens.font.mono,
              fontSize: 11,
              fontWeight: 800,
            }}
          >
            {phase.status === "pending" ? index + 1 : statusGlyph(phase.status)}
          </Box>
          <Typography
            sx={{
              color:
                phase.status === "pending"
                  ? tokens.color.text.muted
                  : tokens.color.text.primary,
              fontWeight: 800,
              fontSize: 12.5,
              flex: 1,
              minWidth: 0,
              whiteSpace: "nowrap",
              overflow: "hidden",
              textOverflow: "ellipsis",
            }}
          >
            {phase.label}
          </Typography>
        </Stack>
        <Stack sx={{ px: 1.25, py: 0.75 }} spacing={0.25}>
          <Typography
            sx={{
              color,
              fontFamily: tokens.font.mono,
              fontSize: 10,
              letterSpacing: 0.6,
              textTransform: "uppercase",
            }}
          >
            {phase.status}
          </Typography>
          <Stack
            direction="row"
            spacing={1}
            alignItems="baseline"
            sx={{ minHeight: 14 }}
          >
            {typeof phase.eventCount === "number" && (
              <Typography
                sx={{
                  color: tokens.color.text.muted,
                  fontFamily: tokens.font.mono,
                  fontSize: 10.5,
                }}
              >
                {phase.eventCount} ev
              </Typography>
            )}
            {typeof phase.costUSD === "number" && phase.costUSD > 0 && (
              <Typography
                sx={{
                  color: tokens.color.accent.coral,
                  fontFamily: tokens.font.mono,
                  fontSize: 10.5,
                }}
              >
                {formatMoney(phase.costUSD)}
              </Typography>
            )}
          </Stack>
        </Stack>
      </Box>
    </Tooltip>
  );
}

function GateNode({ data }: NodeProps) {
  const { gate } = data as GateNodeData;
  const color = gateColor(gate.status);
  return (
    <Tooltip
      arrow
      placement="right"
      enterDelay={120}
      title={<GateTooltip gate={gate} />}
      slotProps={{
        tooltip: {
          sx: {
            bgcolor: tokens.color.bg.surface,
            border: `1px solid ${tokens.color.border.strong}`,
            color: tokens.color.text.primary,
            p: 1.25,
            maxWidth: "none",
          },
        },
        arrow: { sx: { color: tokens.color.bg.surface } },
      }}
    >
      <Stack
        direction="row"
        alignItems="center"
        spacing={0.75}
        sx={{
          px: 1,
          py: 0.5,
          width: 156,
          borderRadius: 999,
          bgcolor: tokens.color.bg.surfaceRaised,
          border: `1px solid ${color}77`,
          color: tokens.color.text.primary,
          boxShadow: `0 2px 10px ${tokens.color.bg.inset}`,
        }}
      >
        <Box sx={{ width: 7, height: 7, borderRadius: 999, bgcolor: color }} />
        <Typography
          sx={{
            color: tokens.color.text.secondary,
            fontFamily: tokens.font.mono,
            fontSize: 11,
            flex: 1,
            minWidth: 0,
            whiteSpace: "nowrap",
            overflow: "hidden",
            textOverflow: "ellipsis",
          }}
        >
          {gate.name}
        </Typography>
        <Typography
          sx={{
            color,
            fontFamily: tokens.font.mono,
            fontSize: 10,
            textTransform: "uppercase",
          }}
        >
          {gate.status}
        </Typography>
      </Stack>
    </Tooltip>
  );
}

const nodeTypes = { phase: PhaseNode, gate: GateNode };

// ----- Unclosed end-to-end summary ---------------------------------

interface UnclosedItem {
  kind: "phase" | "gate";
  label: string;
  detail?: string;
  severity: "active" | "pending" | "blocked" | "failed";
}

interface UnclosedSummary {
  items: UnclosedItem[];
  total: number;
}

// buildUnclosedSummary — distills the workflow + gate state into the
// "what is still open" tray. Per the Visualization-First Contract
// the operator must be able to read open work without expanding the
// timeline; this list is the canonical answer.
function buildUnclosedSummary(
  phases: ExecutionFlowPhase[],
  gates: ExecutionFlowGate[],
): UnclosedSummary {
  const items: UnclosedItem[] = [];

  // Phases that have not finished succeed-side become rows. We skip
  // `pending` rows for downstream phases when an earlier phase is
  // active or failed — the operator already knows the chain is
  // blocked upstream and the noise hurts readability.
  let upstreamBlocked = false;
  for (const p of phases) {
    if (p.status === "done") continue;
    if (p.status === "failed") {
      items.push({
        kind: "phase",
        label: `${p.label} failed`,
        detail: p.missing ?? p.lastEvent?.type,
        severity: "failed",
      });
      upstreamBlocked = true;
      continue;
    }
    if (p.status === "active") {
      items.push({
        kind: "phase",
        label: `${p.label} in progress`,
        detail: p.missing ?? p.lastEvent?.type,
        severity: "active",
      });
      upstreamBlocked = true;
      continue;
    }
    // pending
    if (!upstreamBlocked) {
      items.push({
        kind: "phase",
        label: `${p.label} not started`,
        detail: p.missing,
        severity: "pending",
      });
    }
  }

  for (const g of gates) {
    if (g.status === "pass" || g.status === "skip") continue;
    items.push({
      kind: "gate",
      label: `Gate ${g.name}`,
      detail:
        g.rationale ??
        (typeof g.issuesCount === "number" && g.issuesCount > 0
          ? `${g.issuesCount} issue${g.issuesCount === 1 ? "" : "s"}`
          : g.status),
      severity:
        g.status === "fail"
          ? "failed"
          : g.status === "blocked"
            ? "blocked"
            : "pending",
    });
  }

  return { items, total: items.length };
}

// ----- Layout -------------------------------------------------------

// PHASE_GAP — horizontal step between phase nodes. Generous enough
// that the edge labels (when we add them later) don't collide with
// node borders.
const PHASE_GAP = 220;
const PHASE_Y = 80;
const GATE_X_OFFSET = 8; // anchor gates under Verify's right edge
const GATE_Y_START = 200;
const GATE_GAP = 60;

function buildGraph(
  phases: ExecutionFlowPhase[],
  gates: ExecutionFlowGate[],
  onPhaseClick?: (key: string) => void,
): { nodes: Node[]; edges: Edge[] } {
  const nodes: Node[] = phases.map((p, i) => ({
    id: `phase:${p.key}`,
    type: "phase",
    position: { x: i * PHASE_GAP, y: PHASE_Y },
    data: { phase: p, index: i, gates, onClick: onPhaseClick } as PhaseNodeData,
    draggable: false,
    selectable: false,
  }));

  const edges: Edge[] = [];
  for (let i = 0; i < phases.length - 1; i++) {
    const a = phases[i];
    const b = phases[i + 1];
    const isLive =
      (a.status === "done" && (b.status === "active" || b.status === "done")) ||
      (a.status === "active" && b.status === "pending");
    const segColor =
      a.status === "failed" || b.status === "failed"
        ? tokens.color.accent.danger
        : a.status === "done"
          ? tokens.color.accent.success
          : tokens.color.border.strong;
    edges.push({
      id: `edge:${a.key}->${b.key}`,
      source: `phase:${a.key}`,
      target: `phase:${b.key}`,
      animated: isLive && b.status === "active",
      style: { stroke: segColor, strokeWidth: 1.6 },
      markerEnd: {
        type: MarkerType.ArrowClosed,
        color: segColor,
        width: 14,
        height: 14,
      },
    });
  }

  // Fan out the Verify phase to its gate sub-nodes.
  const verifyIdx = phases.findIndex((p) => p.key === "verify");
  if (verifyIdx >= 0 && gates.length > 0) {
    const baseX = verifyIdx * PHASE_GAP + GATE_X_OFFSET;
    gates.forEach((g, i) => {
      const id = `gate:${g.name}`;
      nodes.push({
        id,
        type: "gate",
        position: { x: baseX, y: GATE_Y_START + i * GATE_GAP },
        data: { gate: g } as GateNodeData,
        draggable: false,
        selectable: false,
      });
      const c = gateColor(g.status);
      edges.push({
        id: `edge:verify->${g.name}`,
        source: `phase:verify`,
        target: id,
        style: {
          stroke: c,
          strokeWidth: 1.2,
          strokeDasharray: g.status === "pending" ? "4 4" : undefined,
        },
        markerEnd: {
          type: MarkerType.ArrowClosed,
          color: c,
          width: 10,
          height: 10,
        },
      });
    });
  }

  return { nodes, edges };
}

// ----- Theme override -----------------------------------------------

// React Flow ships a default light-ish theme. We override its CSS
// vars so the canvas, controls and minimap blend into the dark
// cockpit. Scoped to .ironflyer-rf so we never affect other RFs.
function useRFThemeStyle() {
  useEffect(() => {
    const id = "ironflyer-rf-theme";
    if (document.getElementById(id)) return;
    const style = document.createElement("style");
    style.id = id;
    style.textContent = `
.ironflyer-rf .react-flow__attribution { display: none; }
.ironflyer-rf .react-flow__controls {
  background: ${tokens.color.bg.surface};
  border: 1px solid ${tokens.color.border.subtle};
  border-radius: 6px;
  box-shadow: 0 4px 14px ${tokens.color.bg.inset};
}
.ironflyer-rf .react-flow__controls-button {
  background: ${tokens.color.bg.surface};
  border-bottom: 1px solid ${tokens.color.border.subtle};
  color: ${tokens.color.text.secondary};
  fill: ${tokens.color.text.secondary};
}
.ironflyer-rf .react-flow__controls-button:hover {
  background: ${tokens.color.bg.surfaceHover};
}
.ironflyer-rf .react-flow__minimap {
  background: ${tokens.color.bg.inset};
  border: 1px solid ${tokens.color.border.subtle};
  border-radius: 6px;
}
.ironflyer-rf .react-flow__background {
  background: ${tokens.color.bg.base};
}
`;
    document.head.appendChild(style);
  }, []);
}

// ----- Main component ----------------------------------------------

export function ExecutionFlow({
  phases,
  gates = [],
  status,
  elapsedLabel,
  onPhaseClick,
}: ExecutionFlowProps) {
  useRFThemeStyle();
  const { nodes, edges } = useMemo(
    () => buildGraph(phases, gates, onPhaseClick),
    [phases, gates, onPhaseClick],
  );

  // "What is not closed end-to-end" — collapsible info-graph that
  // mirrors the Visualization-First Contract: the operator must read
  // open work in under 10 seconds without expanding the timeline.
  // The aggregate counts unclosed phases + unresolved gates so a
  // dead-flat "running" status still names what is blocking.
  const unclosed = useMemo(() => buildUnclosedSummary(phases, gates), [phases, gates]);
  const [openUnclosed, setOpenUnclosed] = useState(false);

  // Compute the bounding height so the canvas grows when gates are
  // present without leaving a sea of empty pixels when they aren't.
  const canvasH = gates.length > 0 ? 360 : 220;

  return (
    <Box
      className="ironflyer-rf"
      sx={{
        position: "relative",
        height: canvasH,
        bgcolor: tokens.color.bg.base,
        borderRadius: 1.5,
        border: `1px solid ${tokens.color.border.subtle}`,
        overflow: "hidden",
      }}
    >
      <Stack
        direction="row"
        alignItems="center"
        spacing={1}
        sx={{
          position: "absolute",
          top: 8,
          left: 12,
          right: 12,
          zIndex: 5,
          pointerEvents: "none",
        }}
      >
        <Typography
          variant="overline"
          sx={{
            color: tokens.color.text.muted,
            letterSpacing: 1.2,
            fontSize: 10.5,
          }}
        >
          Workflow
        </Typography>
        <Typography
          sx={{
            color: tokens.color.text.muted,
            fontFamily: tokens.font.mono,
            fontSize: 10.5,
          }}
        >
          · {status}
        </Typography>
        <Box sx={{ flex: 1 }} />
        {unclosed.total > 0 && (
          <Stack
            direction="row"
            alignItems="center"
            spacing={0.5}
            sx={{ pointerEvents: "auto" }}
          >
            <ReportProblemRounded
              sx={{
                fontSize: 13,
                color: tokens.color.brand.amber,
              }}
            />
            <Typography
              sx={{
                color: tokens.color.brand.amber,
                fontFamily: tokens.font.mono,
                fontSize: 10.5,
                fontWeight: 700,
              }}
            >
              {unclosed.total} unclosed
            </Typography>
            <IconButton
              size="small"
              aria-label={openUnclosed ? "Collapse open work" : "Expand open work"}
              onClick={() => setOpenUnclosed((v) => !v)}
              sx={{
                p: 0.25,
                color: tokens.color.text.muted,
                "&:hover": { color: tokens.color.text.primary },
              }}
            >
              {openUnclosed ? (
                <ExpandLessRounded sx={{ fontSize: 14 }} />
              ) : (
                <ExpandMoreRounded sx={{ fontSize: 14 }} />
              )}
            </IconButton>
          </Stack>
        )}
        {elapsedLabel && (
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
            }}
          >
            total {elapsedLabel}
          </Typography>
        )}
      </Stack>

      {/* Expanded "what's not closed end-to-end" tray — floats over
          the canvas without resizing it so a glance stays cheap. */}
      {openUnclosed && unclosed.total > 0 && (
        <Box
          sx={{
            position: "absolute",
            top: 30,
            right: 12,
            zIndex: 6,
            maxWidth: 320,
            bgcolor: tokens.color.bg.surface,
            border: `1px solid ${tokens.color.border.strong}`,
            borderRadius: 1,
            boxShadow: `0 12px 32px ${tokens.color.bg.inset}`,
            p: 1.25,
          }}
        >
          <Typography
            variant="overline"
            sx={{
              color: tokens.color.brand.amber,
              letterSpacing: 1.2,
              fontSize: 10,
            }}
          >
            Open end-to-end
          </Typography>
          <Stack spacing={0.6} sx={{ mt: 0.5 }}>
            {unclosed.items.map((it) => (
              <Stack
                key={`${it.kind}:${it.label}`}
                direction="row"
                spacing={0.75}
                alignItems="baseline"
              >
                <Box
                  sx={{
                    width: 6,
                    height: 6,
                    mt: 0.6,
                    borderRadius: 999,
                    bgcolor:
                      it.severity === "failed"
                        ? tokens.color.accent.danger
                        : it.severity === "blocked"
                          ? tokens.color.brand.amber
                          : tokens.color.text.muted,
                    flexShrink: 0,
                  }}
                />
                <Typography
                  sx={{
                    color: tokens.color.text.secondary,
                    fontFamily: tokens.font.mono,
                    fontSize: 11,
                    flex: 1,
                  }}
                >
                  <Box
                    component="span"
                    sx={{ color: tokens.color.text.primary, fontWeight: 700 }}
                  >
                    {it.label}
                  </Box>
                  {it.detail && (
                    <>
                      {" "}
                      <Box component="span" sx={{ color: tokens.color.text.muted }}>
                        · {it.detail}
                      </Box>
                    </>
                  )}
                </Typography>
              </Stack>
            ))}
          </Stack>
        </Box>
      )}

      <ReactFlowProvider>
        <ReactFlow
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          fitView
          fitViewOptions={{ padding: 0.18, includeHiddenNodes: false }}
          minZoom={0.4}
          maxZoom={1.5}
          panOnScroll
          panOnDrag
          zoomOnScroll={false}
          zoomOnPinch
          nodesDraggable={false}
          nodesConnectable={false}
          elementsSelectable={false}
          proOptions={{ hideAttribution: true }}
        >
          <Background
            variant={BackgroundVariant.Dots}
            gap={16}
            size={1}
            color={tokens.color.border.subtle}
          />
          <Controls
            position="bottom-right"
            showInteractive={false}
            showFitView
          />
          <MiniMap
            position="bottom-left"
            pannable
            zoomable
            maskColor={`${tokens.color.bg.inset}cc`}
            nodeColor={(n) => {
              const data = n.data as PhaseNodeData | GateNodeData | undefined;
              if (!data) return tokens.color.text.muted;
              if ("phase" in data) return phaseColor(data.phase.status);
              if ("gate" in data) return gateColor(data.gate.status);
              return tokens.color.text.muted;
            }}
          />
        </ReactFlow>
      </ReactFlowProvider>
    </Box>
  );
}
