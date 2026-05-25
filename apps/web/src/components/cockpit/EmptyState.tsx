"use client";

// EmptyState — first-run / no-results panel. Tight composition: title,
// body, single CTA. Keep copy specific — "No paid executions yet" beats
// "Nothing here".

import { Box, Button, Stack, Typography, type SxProps, type Theme } from "@mui/material";
import Link from "next/link";
import type { ReactNode } from "react";
import { tokens } from "../../theme";

export interface EmptyStateCta {
  label: string;
  href?: string;
  onClick?: () => void;
}

export interface EmptyStateProps {
  title: ReactNode;
  body?: ReactNode;
  icon?: ReactNode;
  cta?: EmptyStateCta;
  sx?: SxProps<Theme>;
}

export function EmptyState({ title, body, icon, cta, sx }: EmptyStateProps) {
  return (
    <Box
      sx={{
        border: `1px dashed ${tokens.color.border.subtle}`,
        borderRadius: 1,
        py: { xs: 6, md: 8 },
        px: 3,
        textAlign: "center",
        bgcolor: tokens.color.bg.inset,
        ...sx,
      }}
    >
      <Stack alignItems="center" spacing={2}>
        {icon && (
          <Box sx={{ color: tokens.color.text.muted, fontSize: 32 }}>{icon}</Box>
        )}
        <Typography
          sx={{
            fontWeight: 800,
            fontSize: { xs: 18, md: 22 },
            color: tokens.color.text.primary,
            letterSpacing: -0.3,
          }}
        >
          {title}
        </Typography>
        {body && (
          <Typography
            sx={{ maxWidth: 480, color: tokens.color.text.secondary, fontSize: 14 }}
          >
            {body}
          </Typography>
        )}
        {cta && (
          <Box sx={{ pt: 1 }}>
            {cta.href ? (
              <Button
                component={Link}
                href={cta.href}
                variant="contained"
                color="primary"
                size="medium"
              >
                {cta.label}
              </Button>
            ) : (
              <Button onClick={cta.onClick} variant="contained" color="primary" size="medium">
                {cta.label}
              </Button>
            )}
          </Box>
        )}
      </Stack>
    </Box>
  );
}
