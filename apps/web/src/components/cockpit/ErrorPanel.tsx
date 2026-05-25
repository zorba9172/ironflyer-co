"use client";

// ErrorPanel — surfaces a normalised GraphQL / network error to the
// user. Backed by lib/errors.ts so we get a consistent shape regardless
// of where the error originated (Apollo, network, runtime throw).
//
// Use for non-blocking inline errors (a failing query inside a panel).
// Hard 401s are intercepted by the Apollo error link and routed
// through AuthProvider → /login redirect.

import {
  ErrorOutlineRounded,
  RefreshRounded,
} from "@mui/icons-material";
import { Box, Button, Stack, Typography, type SxProps, type Theme } from "@mui/material";
import { normalizeError } from "../../lib/errors";
import { tokens } from "../../theme";

export interface ErrorPanelProps {
  error: unknown;
  title?: string;
  onRetry?: () => void;
  sx?: SxProps<Theme>;
}

export function ErrorPanel({ error, title = "Something went wrong", onRetry, sx }: ErrorPanelProps) {
  const n = normalizeError(error);
  return (
    <Box
      role="alert"
      sx={{
        border: `1px solid ${tokens.color.accent.danger}66`,
        bgcolor: `${tokens.color.accent.danger}10`,
        borderRadius: 1,
        p: 2.5,
        ...sx,
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
              fontFamily: n.isNetwork ? tokens.font.family : undefined,
            }}
          >
            {n.message}
          </Typography>
          {n.code && (
            <Typography
              sx={{
                mt: 0.5,
                color: tokens.color.text.muted,
                fontFamily: tokens.font.mono,
                fontSize: 11.5,
              }}
            >
              code: {n.code}
            </Typography>
          )}
        </Box>
        {onRetry && (
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
        )}
      </Stack>
    </Box>
  );
}
