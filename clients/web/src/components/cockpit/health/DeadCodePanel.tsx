"use client";

// DeadCodePanel — headline count of unused exports / files / deps,
// plus a collapsed list of the top 10 offenders.
//
// Backend: HealthDashboard.DeadcodeCount is the project-wide tally
// projected from knip / ts-prune / unparam reports. The per-file
// offender list is a follow-up — the persisted report has it; the
// GraphQL resolver does not yet project it. Until then we render the
// headline number and a stub list when no offenders are available.

import { useState } from "react";
import {
  Box,
  Collapse,
  Stack,
  Typography,
} from "@mui/material";
import { ExpandMoreRounded } from "@mui/icons-material";
import { tokens } from "../../../theme";
import { PanelFrame, PanelStubEmpty } from "./PanelFrame";
import type { HealthDashboardShape } from "./types";

export interface DeadCodeOffender {
  path: string;
  symbol: string;
  // "export" | "file" | "dep"
  kind: string;
}

export interface DeadCodePanelProps {
  data: HealthDashboardShape;
  offenders?: DeadCodeOffender[];
}

export function DeadCodePanel({ data, offenders }: DeadCodePanelProps) {
  const [open, setOpen] = useState(false);
  const wired = data.deadcodeCount !== -1;
  const top = (offenders ?? []).slice(0, 10);

  return (
    <PanelFrame
      eyebrow="Dead code"
      title="Unused exports & files"
      hint={
        wired
          ? "Sourced from knip / ts-prune / unparam — click to expand the offender list."
          : undefined
      }
    >
      {!wired ? (
        <PanelStubEmpty>
          Dead-code report not yet wired. Install knip (TS) + unparam (Go) and
          set{" "}
          <Box
            component="span"
            sx={{ fontFamily: tokens.font.mono, color: tokens.color.text.primary }}
          >
            IRONFLYER_DEADCODE_REPORT_PATH
          </Box>
          {" "}so the Anti-Bloat gate can publish unused-export counts.
        </PanelStubEmpty>
      ) : (
        <Box>
          <Stack direction="row" alignItems="baseline" spacing={1.25}>
            <Typography
              sx={{
                fontFamily: tokens.font.mono,
                fontSize: 36,
                fontWeight: 800,
                color:
                  data.deadcodeCount === 0
                    ? tokens.color.brand.mint
                    : tokens.color.text.primary,
                lineHeight: 1,
                letterSpacing: -0.5,
              }}
            >
              {data.deadcodeCount}
            </Typography>
            <Typography sx={{ fontSize: 13, color: tokens.color.text.secondary }}>
              unused symbols
            </Typography>
          </Stack>

          <Box
            onClick={() => setOpen((v) => !v)}
            role="button"
            tabIndex={0}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") setOpen((v) => !v);
            }}
            sx={{
              mt: 2,
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
                sx={{ fontSize: 12.5, fontWeight: 700, color: tokens.color.text.secondary }}
              >
                Top offenders ({top.length})
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
            <Collapse in={open} unmountOnExit>
              <Stack spacing={0.5} sx={{ mt: 1.25 }}>
                {top.length === 0 ? (
                  <Typography sx={{ fontSize: 12, color: tokens.color.text.muted }}>
                    Offender list not yet projected by the resolver — the report
                    has the data; the GraphQL field is the follow-up.
                  </Typography>
                ) : (
                  top.map((o) => (
                    <Stack
                      key={`${o.path}:${o.symbol}`}
                      direction="row"
                      spacing={1}
                      alignItems="baseline"
                      sx={{
                        fontFamily: tokens.font.mono,
                        fontSize: 11.5,
                        color: tokens.color.text.muted,
                      }}
                    >
                      <Box
                        component="span"
                        sx={{
                          minWidth: 38,
                          color: tokens.color.accent.coral,
                          textTransform: "uppercase",
                          letterSpacing: 0.5,
                          fontSize: 10,
                        }}
                      >
                        {o.kind}
                      </Box>
                      <Box component="span" sx={{ color: tokens.color.text.secondary }}>
                        {o.path}
                      </Box>
                      <Box component="span" sx={{ color: tokens.color.text.primary }}>
                        {o.symbol}
                      </Box>
                    </Stack>
                  ))
                )}
              </Stack>
            </Collapse>
          </Box>
        </Box>
      )}
    </PanelFrame>
  );
}
