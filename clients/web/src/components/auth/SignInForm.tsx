"use client";

// SignInForm — email + password. Submits through useAuth().signIn,
// which writes the JWT to the cookie (apollo.tsx setToken) and resets
// the Apollo store so cached data from a prior identity cannot leak.
// On success we call onSuccess(); the page decides where to route
// (typically the `redirect` query param or "/").

import { ArrowForwardRounded } from "@mui/icons-material";
import {
  Box,
  Button,
  CircularProgress,
  Stack,
  TextField,
  Typography,
} from "@mui/material";
import Link from "next/link";
import { useState, type FormEvent } from "react";
import { useAuth } from "../../lib/auth";
import { tokens } from "../../theme";
import { ErrorPanel } from "../cockpit/ErrorPanel";
import { SocialLoginButtons } from "./SocialLoginButtons";

export interface SignInFormProps {
  onSuccess: () => void;
  initialEmail?: string;
  returnTo?: string;
}

export function SignInForm({ onSuccess, initialEmail = "", returnTo = "/" }: SignInFormProps) {
  const { signIn } = useAuth();

  const [email, setEmail] = useState(initialEmail);
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<unknown>(null);

  const emailOk = /^\S+@\S+\.\S+$/.test(email.trim());
  const passwordOk = password.length >= 8;
  const canSubmit = emailOk && passwordOk && !submitting;

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!canSubmit) return;
    setError(null);
    setSubmitting(true);
    try {
      await signIn({ email: email.trim(), password });
      onSuccess();
    } catch (err) {
      setError(err);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Box component="form" onSubmit={handleSubmit}>
      <Stack spacing={2}>
        <SocialLoginButtons returnTo={returnTo} disabled={submitting} />
        <Field
          label="Email"
          type="email"
          autoComplete="email"
          autoFocus
          value={email}
          onChange={setEmail}
          required
        />
        <Field
          label="Password"
          type="password"
          autoComplete="current-password"
          value={password}
          onChange={setPassword}
          required
          helperText={
            password.length > 0 && password.length < 8
              ? "Use at least 8 characters."
              : undefined
          }
        />
        <Stack direction="row" justifyContent="flex-end">
          <Typography
            component={Link}
            href="/login/reset"
            sx={{
              fontSize: 12,
              color: tokens.color.text.secondary,
              textDecoration: "none",
              "&:hover": { color: tokens.color.accent.violet },
            }}
          >
            Forgot password?
          </Typography>
        </Stack>
        {error ? <ErrorPanel error={error} title="Could not sign in" /> : null}
        <Button
          type="submit"
          variant="contained"
          color="primary"
          disabled={!canSubmit}
          endIcon={
            submitting ? (
              <CircularProgress size={14} sx={{ color: tokens.color.text.primary }} />
            ) : (
              <ArrowForwardRounded sx={{ fontSize: 18 }} />
            )
          }
          sx={{ minHeight: 44, fontSize: 14 }}
        >
          {submitting ? "Signing in…" : "Sign in"}
        </Button>
      </Stack>
    </Box>
  );
}

function Field(props: {
  label: string;
  type?: string;
  value: string;
  onChange: (next: string) => void;
  autoComplete?: string;
  autoFocus?: boolean;
  required?: boolean;
  helperText?: string;
}) {
  const { label, type = "text", value, onChange, autoComplete, autoFocus, required, helperText } = props;
  return (
    <TextField
      label={label}
      type={type}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      autoComplete={autoComplete}
      autoFocus={autoFocus}
      required={required}
      fullWidth
      size="medium"
      helperText={helperText}
      slotProps={{
        inputLabel: {
          sx: { color: tokens.color.text.secondary },
        },
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
  );
}
