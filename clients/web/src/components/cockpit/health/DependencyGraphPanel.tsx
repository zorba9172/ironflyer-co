"use client";

// DependencyGraphPanel — internal package dependency graph.
//
// Default: collapsed summary — count of layers + rules + cycles.
// On click, expands to a full @xyflow/react graph with edges colored
// by allow/deny per the architecture manifest.
//
// Backend gap: the architecture manifest lives at
// `.ironflyer/architecture.json` on disk. A future GraphQL field
// `query { architecture { layers, rules } }` on the dashboards.health
// resolver projects it. Until that lands, this panel renders a
// summary based on the HealthDashboard.DependencyCycles sentinel and
// the optional layers prop. The expand-into-graph path stays lazy
// (the @xyflow/react chunk only loads when expanded).

import { useState } from "react";
import dynamic from "next/dynamic";
import { Box, Stack, Typography } from "@mui/material";
import { ExpandMoreRounded } from "@mui/icons-material";
import { tokens } from "../../../theme";
import { PanelFrame, PanelStubEmpty } from "./PanelFrame";
import type {
  HealthDashboardShape,
  ArchitectureLayer,
  ArchitectureRule,
} from "./types";

const DependencyGraphCanvas = dynamic(
  () => import("./DependencyGraphCanvas").then((m) => m.DependencyGraphCanvas),
  {
    ssr: false,
    loading: () => (
      <Box
        sx={{
          height: 320,
          borderRadius: 1,
          bgcolor: tokens.color.bg.surfaceRaised,
        }}
      />
    ),
  },
);

export interface DependencyGraphPanelProps {
  data: HealthDashboardShape;
  layers?: ArchitectureLayer[];
  rules?: ArchitectureRule[];
}

export function DependencyGraphPanel({ data, layers, rules }: DependencyGraphPanelProps) {
  const [open, setOpen] = useState(false);
  const cyclesWired = data.dependencyCycles !== -1;
  const manifestWired = !!(layers && layers.length > 0);

  return (
    <PanelFrame
      eyebrow="Architecture"
      title="Dependency graph"
      hint={
        cyclesWired
          ? data.dependencyCycles === 0
            ? "Zero import cycles — the architecture manifest is satisfied."
            : `${data.dependencyCycles} cycle${data.dependencyCycles === 1 ? "" : "s"} detected — open the graph to see the deny edges.`
          : undefined
      }
    >
      {!cyclesWired && !manifestWired ? (
        <PanelStubEmpty>
          Dependency report not yet wired. Install dependency-cruiser / madge
          and set{" "}
          <Box
            component="span"
            sx={{ fontFamily: tokens.font.mono, color: tokens.color.text.primary }}
          >
            IRONFLYER_DEPCYCLE_REPORT_PATH
          </Box>
          . The architecture manifest belongs at{" "}
          <Box
            component="span"
            sx={{ fontFamily: tokens.font.mono, color: tokens.color.text.primary }}
          >
            .ironflyer/architecture.json
          </Box>
          {" "}and ships through a future GraphQL{" "}
          <Box
            component="span"
            sx={{ fontFamily: tokens.font.mono, color: tokens.color.text.primary }}
          >
            architecture {"{ layers rules }"}
          </Box>
          {" "}field.
        </PanelStubEmpty>
      ) : (
        <Box>
          <Stack direction="row" spacing={2} sx={{ mb: 1.5 }}>
            <SummaryStat
              label="Layers"
              value={String(layers?.length ?? 0)}
              tone={manifestWired ? "primary" : "muted"}
            />
            <SummaryStat
              label="Rules"
              value={String(rules?.length ?? 0)}
              tone={manifestWired ? "primary" : "muted"}
            />
            <SummaryStat
              label="Cycles"
              value={String(cyclesWired ? data.dependencyCycles : "—")}
              tone={
                !cyclesWired
                  ? "muted"
                  : data.dependencyCycles === 0
                    ? "success"
                    : "danger"
              }
            />
          </Stack>

          <Box
            onClick={() => setOpen((v) => !v)}
            role="button"
            tabIndex={0}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") setOpen((v) => !v);
            }}
            sx={{
              cursor: "pointer",
              borderRadius: 1,
              border: `1px solid ${tokens.color.border.subtle}`,
              px: 1.5,
              py: 1,
              "&:hover": {
                borderColor: tokens.color.border.strong,
                bgcolor: tokens.color.bg.surfaceHover,
              },
            }}
          >
            <Stack direction="row" justifyContent="space-between" alignItems="center">
              <Typography
                sx={{
                  fontSize: 12.5,
                  fontWeight: 700,
                  color: tokens.color.text.secondary,
                }}
              >
                {open ? "Hide graph" : "Expand graph"}
              </Typography>
              <ExpandMoreRounded
                sx={{
                  fontSize: 18,
                  color: tokens.color.text.muted,
                  transform: open ? "rotate(180deg)" : "rotate(0deg)",
                  transition: `transform ${tokens.motion.fast} ${tokens.motion.snap}`,
                }}
              />
            </Stack>
            {open && (
              <Box sx={{ mt: 1.5 }}>
                <DependencyGraphCanvas
                  layers={layers ?? []}
                  rules={rules ?? []}
                />
              </Box>
            )}
          </Box>
        </Box>
      )}
    </PanelFrame>
  );
}

function SummaryStat({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone: "primary" | "muted" | "success" | "danger";
}) {
  const color =
    tone === "success"
      ? tokens.color.brand.mint
      : tone === "danger"
        ? tokens.color.accent.coral
        : tone === "muted"
          ? tokens.color.text.muted
          : tokens.color.text.primary;
  return (
    <Stack spacing={0.25}>
      <Typography
        variant="overline"
        sx={{ color: tokens.color.text.muted, letterSpacing: 1.2, lineHeight: 1 }}
      >
        {label}
      </Typography>
      <Typography
        sx={{
          fontFamily: tokens.font.mono,
          fontSize: 22,
          fontWeight: 700,
          color,
          lineHeight: 1.1,
        }}
      >
        {value}
      </Typography>
    </Stack>
  );
}
