"use client";

// StudioErrorPanel — local mirror of cockpit/ErrorPanel for the studio
// shell. The brief asks us to avoid hard-importing cockpit/ErrorPanel
// inside the workbench so the studio surface can ship even if cockpit
// errors out. Visual language is identical (token-driven) so the user
// never sees a drift.

import { ErrorOutlineRounded, RefreshRounded } from "@mui/icons-material";
import { Box, Button, Stack, Typography } from "@mui/material";
import { tokens } from "../../theme";

export interface StudioErrorPanelProps {
  title?: string;
  message: string;
  onRetry?: () => void;
}

export function StudioErrorPanel({
  title = "Something went wrong",
  message,
  onRetry,
}: StudioErrorPanelProps) {
  return (
    <Box
      role="alert"
      sx={{
        border: `1px solid ${tokens.color.accent.danger}66`,
        bgcolor: `${tokens.color.accent.danger}10`,
        borderRadius: 1,
        m: 2,
        p: 2.5,
      }}
    >
      <Stack direction="row" spacing={1.5} alignItems="flex-start">
        <ErrorOutlineRounded sx={{ color: tokens.color.accent.danger, mt: 0.25 }} />
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography
            sx={{
              fontWeight: 700,
              color: tokens.color.text.primary,
              fontSize: 14,
            }}
          >
            {title}
          </Typography>
          <Typography
            sx={{
              mt: 0.5,
              color: tokens.color.text.secondary,
              fontSize: 13.5,
            }}
          >
            {message}
          </Typography>
        </Box>
        {onRetry ? (
          <Button
            size="small"
            variant="outlined"
            startIcon={<RefreshRounded fontSize="small" />}
            onClick={onRetry}
            sx={{
              color: tokens.color.text.primary,
              borderColor: tokens.color.border.strong,
              "&:hover": {
                borderColor: tokens.color.accent.violet,
                bgcolor: "transparent",
              },
            }}
          >
            Retry
          </Button>
        ) : null}
      </Stack>
    </Box>
  );
}
