"use client";

// /login/reset — password-reset placeholder. The orchestrator exposes a
// `requestPasswordReset` mutation but no GraphQL document is wired into
// codegen yet, so we render a clear "coming soon" panel inside the
// AuthShell instead of a fake form. Visitors are pointed at support
// (the orchestrator owner) and back at sign-in so they are never
// trapped.

import { ArrowBackRounded, EmailOutlined } from "@mui/icons-material";
import { Box, Button, Stack, Typography } from "@mui/material";
import Link from "next/link";
import { AuthShell } from "../../../src/components/auth/AuthShell";
import { tokens } from "../../../src/theme";

export default function PasswordResetPage() {
  return (
    <AuthShell
      mode="signin"
      title="Reset your password"
      subtitle="Self-service reset is on the way. In the meantime, our team can verify your account and re-issue access by email."
      switchHref="/login"
      switchPrompt="Remembered your password?"
      switchAction="Back to sign in"
    >
      <Stack spacing={2.5}>
        <Box
          sx={{
            p: 2,
            borderRadius: `${tokens.radius.sm}px`,
            border: `1px solid ${tokens.color.border.subtle}`,
            bgcolor: tokens.color.bg.inset,
          }}
        >
          <Stack direction="row" alignItems="center" spacing={1.2}>
            <EmailOutlined sx={{ color: tokens.color.accent.violet, fontSize: 20 }} />
            <Box>
              <Typography sx={{ fontSize: 13, fontWeight: 700, color: tokens.color.text.primary }}>
                Email support
              </Typography>
              <Typography sx={{ fontSize: 12.5, color: tokens.color.text.secondary, mt: 0.25 }}>
                Reach us at <strong>support@ironflyer.app</strong> with the
                account address. We will verify the identity behind the
                wallet and re-issue access.
              </Typography>
            </Box>
          </Stack>
        </Box>
        <Button
          component={Link}
          href="/login"
          variant="contained"
          color="primary"
          startIcon={<ArrowBackRounded sx={{ fontSize: 18 }} />}
          sx={{ minHeight: 44 }}
        >
          Back to sign in
        </Button>
      </Stack>
    </AuthShell>
  );
}
