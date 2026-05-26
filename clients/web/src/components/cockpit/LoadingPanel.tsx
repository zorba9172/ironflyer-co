"use client";

// LoadingPanel — centred spinner with an optional caption. Used inside
// page bodies while the first paint of data is in-flight. Avoid using
// for tiny ranges of UI (use <Skeleton /> directly) — this panel is
// shaped for "the whole tab is waiting".

import { Box, CircularProgress, Stack, Typography, type SxProps, type Theme } from "@mui/material";
import { tokens } from "../../theme";

export interface LoadingPanelProps {
  label?: string;
  // minHeight in px; default 240. Pass "100%" if the parent should
  // own the full height.
  minHeight?: number | string;
  sx?: SxProps<Theme>;
}

export function LoadingPanel({ label, minHeight = 240, sx }: LoadingPanelProps) {
  return (
    <Box
      role="status"
      aria-busy="true"
      sx={{
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        minHeight,
        ...sx,
      }}
    >
      <Stack alignItems="center" spacing={2}>
        <CircularProgress
          size={28}
          thickness={4.5}
          sx={{ color: tokens.color.accent.violet }}
        />
        {label && (
          <Typography
            variant="overline"
            sx={{ color: tokens.color.text.muted, letterSpacing: 1.2 }}
          >
            {label}
          </Typography>
        )}
      </Stack>
    </Box>
  );
}
