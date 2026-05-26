"use client";

// SignUpForm — email + password + (optional) display name. The
// orchestrator's SignUpInput accepts name but not orgName today, so
// we expose name as "What should we call you?" and skip org creation
// at sign-up (operator console handles orgs). After useAuth().signUp,
// the JWT is already set + the Apollo store has been reset; we hand
// control back to the page via onSuccess() which decides redirect.

import { ArrowForwardRounded } from "@mui/icons-material";
import {
  Box,
  Button,
  CircularProgress,
  Stack,
  TextField,
  Typography,
} from "@mui/material";
import { useState, type FormEvent } from "react";
import { useAuth } from "../../lib/auth";
import { tokens } from "../../theme";
import { ErrorPanel } from "../cockpit/ErrorPanel";
import { SocialLoginButtons } from "./SocialLoginButtons";

export interface SignUpFormProps {
  onSuccess: () => void;
  initialEmail?: string;
  returnTo?: string;
}

export function SignUpForm({ onSuccess, initialEmail = "", returnTo = "/" }: SignUpFormProps) {
  const { signUp } = useAuth();

  const [name, setName] = useState("");
  const [email, setEmail] = useState(initialEmail);
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<unknown>(null);

  const emailOk = /^\S+@\S+\.\S+$/.test(email.trim());
  const passwordOk = password.length >= 8;
  const confirmOk = confirm === password && confirm.length > 0;
  const canSubmit = emailOk && passwordOk && confirmOk && !submitting;

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!canSubmit) return;
    setError(null);
    setSubmitting(true);
    try {
      await signUp({
        email: email.trim(),
        password,
        name: name.trim() ? name.trim() : null,
      });
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
          label="What should we call you? (optional)"
          autoComplete="name"
          value={name}
          onChange={setName}
        />
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
          autoComplete="new-password"
          value={password}
          onChange={setPassword}
          required
          helperText={
            password.length > 0 && password.length < 8
              ? "At least 8 characters."
              : "Use at least 8 characters."
          }
        />
        <Field
          label="Confirm password"
          type="password"
          autoComplete="new-password"
          value={confirm}
          onChange={setConfirm}
          required
          helperText={
            confirm.length > 0 && confirm !== password
              ? "Passwords do not match."
              : undefined
          }
        />
        <Typography sx={{ fontSize: 11.5, color: tokens.color.text.muted, lineHeight: 1.5 }}>
          By creating an account you agree to use IronFlyer for lawful builds
          and keep your workspace data connected to the product flow.
        </Typography>
        {error ? <ErrorPanel error={error} title="Could not create your account" /> : null}
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
          {submitting ? "Creating account…" : "Create account"}
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
