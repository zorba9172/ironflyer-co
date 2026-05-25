"use client";

// /login/reset — self-service password reset. Without a token we send
// a reset email; with ?token= we accept the new password and establish
// the returned session.

import { ArrowBackRounded, ArrowForwardRounded, EmailOutlined } from "@mui/icons-material";
import { Box, Button, Stack, Typography } from "@mui/material";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useMemo, useState, type FormEvent } from "react";
import { AuthShell } from "../../../src/components/auth/AuthShell";
import { ErrorPanel } from "../../../src/components/cockpit/ErrorPanel";
import { setToken } from "../../../src/lib/auth";
import {
  useRequestPasswordResetMutation,
  useResetPasswordMutation,
} from "../../../src/lib/gql/__generated__";
import { tokens } from "../../../src/theme";
import { TextField, CircularProgress } from "@mui/material";

export default function PasswordResetPage() {
  return (
    <Suspense fallback={null}>
      <PasswordResetInner />
    </Suspense>
  );
}

function PasswordResetInner() {
  const router = useRouter();
  const search = useSearchParams();
  const token = search?.get("token")?.trim() ?? "";
  const mode = token ? "set" : "request";

  return (
    <AuthShell
      mode="signin"
      title="Reset your password"
      subtitle={
        mode === "set"
          ? "Choose a new password and return directly to your Studio workspace."
          : "Enter your account email and we will send a secure reset link."
      }
      switchHref="/login"
      switchPrompt="Remembered your password?"
      switchAction="Back to sign in"
    >
      {mode === "set" ? (
        <SetPasswordForm
          token={token}
          onDone={() => router.replace("/studio")}
        />
      ) : (
        <RequestResetForm />
      )}
    </AuthShell>
  );
}

function RequestResetForm() {
  const [email, setEmail] = useState("");
  const [sent, setSent] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [requestReset, requestState] = useRequestPasswordResetMutation();
  const emailOk = /^\S+@\S+\.\S+$/.test(email.trim());

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!emailOk || requestState.loading) return;
    setError(null);
    try {
      await requestReset({ variables: { email: email.trim() } });
      setSent(true);
    } catch (err) {
      setError(err);
    }
  };

  if (sent) {
    return (
      <Stack spacing={2.5}>
        <Notice
          title="Check your email"
          body="If this account exists, a secure reset link is on the way. Keep this tab open or return to sign in."
        />
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
    );
  }

  return (
    <Box component="form" onSubmit={submit}>
      <Stack spacing={2}>
        <TextField
          label="Email"
          type="email"
          autoComplete="email"
          autoFocus
          required
          fullWidth
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          slotProps={{
            inputLabel: { sx: { color: tokens.color.text.secondary } },
            input: {
              sx: {
                bgcolor: tokens.color.bg.inset,
                "& fieldset": { borderColor: tokens.color.border.subtle },
                "&:hover fieldset": { borderColor: tokens.color.border.strong },
                "&.Mui-focused fieldset": { borderColor: `${tokens.color.border.accent} !important` },
              },
            },
          }}
        />
        {error ? <ErrorPanel error={error} title="Could not send reset link" /> : null}
        <Button
          type="submit"
          variant="contained"
          color="primary"
          disabled={!emailOk || requestState.loading}
          endIcon={
            requestState.loading ? (
              <CircularProgress size={14} sx={{ color: tokens.color.text.primary }} />
            ) : (
              <ArrowForwardRounded sx={{ fontSize: 18 }} />
            )
          }
          sx={{ minHeight: 44 }}
        >
          {requestState.loading ? "Sending…" : "Send reset link"}
        </Button>
      </Stack>
    </Box>
  );
}

function SetPasswordForm({ token, onDone }: { token: string; onDone: () => void }) {
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState<unknown>(null);
  const [resetPassword, resetState] = useResetPasswordMutation();
  const passwordOk = password.length >= 8;
  const confirmOk = confirm === password && confirm.length > 0;
  const canSubmit = passwordOk && confirmOk && !resetState.loading;

  const helper = useMemo(() => {
    if (password.length > 0 && !passwordOk) return "Use at least 8 characters.";
    if (confirm.length > 0 && !confirmOk) return "Passwords do not match.";
    return "Use at least 8 characters.";
  }, [confirm.length, confirmOk, password.length, passwordOk]);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!canSubmit) return;
    setError(null);
    try {
      const result = await resetPassword({
        variables: { token, newPassword: password },
      });
      const session = result.data?.resetPassword;
      if (!session?.token) throw new Error("Reset completed without a session token.");
      setToken(session.token);
      onDone();
    } catch (err) {
      setError(err);
    }
  };

  return (
    <Box component="form" onSubmit={submit}>
      <Stack spacing={2}>
        <TextField
          label="New password"
          type="password"
          autoComplete="new-password"
          required
          fullWidth
          value={password}
          helperText={helper}
          onChange={(event) => setPassword(event.target.value)}
          slotProps={{
            inputLabel: { sx: { color: tokens.color.text.secondary } },
            input: {
              sx: {
                bgcolor: tokens.color.bg.inset,
                "& fieldset": { borderColor: tokens.color.border.subtle },
                "&:hover fieldset": { borderColor: tokens.color.border.strong },
                "&.Mui-focused fieldset": { borderColor: `${tokens.color.border.accent} !important` },
              },
            },
          }}
        />
        <TextField
          label="Confirm password"
          type="password"
          autoComplete="new-password"
          required
          fullWidth
          value={confirm}
          onChange={(event) => setConfirm(event.target.value)}
          slotProps={{
            inputLabel: { sx: { color: tokens.color.text.secondary } },
            input: {
              sx: {
                bgcolor: tokens.color.bg.inset,
                "& fieldset": { borderColor: tokens.color.border.subtle },
                "&:hover fieldset": { borderColor: tokens.color.border.strong },
                "&.Mui-focused fieldset": { borderColor: `${tokens.color.border.accent} !important` },
              },
            },
          }}
        />
        {error ? <ErrorPanel error={error} title="Could not reset password" /> : null}
        <Button
          type="submit"
          variant="contained"
          color="primary"
          disabled={!canSubmit}
          endIcon={
            resetState.loading ? (
              <CircularProgress size={14} sx={{ color: tokens.color.text.primary }} />
            ) : (
              <ArrowForwardRounded sx={{ fontSize: 18 }} />
            )
          }
          sx={{ minHeight: 44 }}
        >
          {resetState.loading ? "Updating…" : "Update password"}
        </Button>
      </Stack>
    </Box>
  );
}

function Notice({ title, body }: { title: string; body: string }) {
  return (
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
            {title}
          </Typography>
          <Typography sx={{ fontSize: 12.5, color: tokens.color.text.secondary, mt: 0.25 }}>
            {body}
          </Typography>
        </Box>
      </Stack>
    </Box>
  );
}
