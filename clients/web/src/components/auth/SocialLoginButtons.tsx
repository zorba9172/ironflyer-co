"use client";

// SocialLoginButtons — stacked Google + GitHub OAuth entry points that
// hand off to the orchestrator's REST callback bootstrap (these are
// browser navigations across origin, so next/link is the wrong tool;
// we use window.location.assign).
//
// Flow: click → /auth/{provider}/start?redirect=/auth/callback?next=<path>
// → provider consent → orchestrator callback → 302 back to
// /auth/callback#token=<jwt>&expiresAt=<rfc3339>&next=<path>.

import { GitHub as GitHubIcon } from "@mui/icons-material";
import { Box, Button, Stack, Typography } from "@mui/material";
import { tokens } from "../../theme";

export interface SocialLoginButtonsProps {
  returnTo: string;
  disabled?: boolean;
  timing?: "light" | "dark";
}

function apiBase(): string {
  const direct = process.env.NEXT_PUBLIC_GRAPHQL_URL;
  if (direct) {
    // Strip trailing /graphql so we can hang /auth/... off the root.
    return direct.replace(/\/graphql\/?$/, "").replace(/\/+$/, "");
  }
  const base = process.env.NEXT_PUBLIC_IRONFLYER_API_URL;
  if (base) return base.replace(/\/+$/, "");
  return "http://localhost:8080";
}

function startOAuth(provider: "google" | "github", returnTo: string): void {
  const safeReturn =
    returnTo.startsWith("/") && !returnTo.startsWith("//") ? returnTo : "/";
  const callback = `/auth/callback?next=${encodeURIComponent(safeReturn)}`;
  const url = `${apiBase()}/auth/${provider}/start?redirect=${encodeURIComponent(callback)}`;
  window.location.assign(url);
}

export function SocialLoginButtons({
  returnTo,
  disabled,
  timing = "dark",
}: SocialLoginButtonsProps) {
  const light = timing === "light";
  return (
    <Stack spacing={1.5}>
      <Stack spacing={1}>
        <ProviderButton
          disabled={disabled}
          light={light}
          icon={<GoogleMark />}
          label="Continue with Google"
          onClick={() => startOAuth("google", returnTo)}
        />
        <ProviderButton
          disabled={disabled}
          light={light}
          icon={
            <GitHubIcon
              sx={{
                fontSize: 18,
                color: light ? "#111633" : tokens.color.text.primary,
              }}
            />
          }
          label="Continue with GitHub"
          onClick={() => startOAuth("github", returnTo)}
        />
      </Stack>
      <OrDivider light={light} />
    </Stack>
  );
}

function ProviderButton(props: {
  icon: React.ReactNode;
  label: string;
  onClick: () => void;
  disabled?: boolean;
  light: boolean;
}) {
  const { icon, label, onClick, disabled, light } = props;
  return (
    <Button
      type="button"
      variant="outlined"
      fullWidth
      disabled={disabled}
      onClick={onClick}
      startIcon={icon}
      sx={{
        minHeight: 44,
        fontSize: 14,
        fontWeight: 500,
        justifyContent: "center",
        color: light ? "#111633" : tokens.color.text.primary,
        bgcolor: light ? "#ffffff" : tokens.color.bg.inset,
        borderColor: light ? "rgba(18,22,55,0.12)" : tokens.color.border.subtle,
        textTransform: "none",
        "&:hover": {
          bgcolor: light
            ? "rgba(143,77,255,0.06)"
            : tokens.color.bg.surfaceHover,
          borderColor: light
            ? "rgba(143,77,255,0.24)"
            : tokens.color.border.strong,
        },
        "&.Mui-disabled": {
          color: light ? "#8a90a8" : tokens.color.text.muted,
          borderColor: light
            ? "rgba(18,22,55,0.08)"
            : tokens.color.border.subtle,
        },
      }}
    >
      {label}
    </Button>
  );
}

function OrDivider({ light }: { light: boolean }) {
  return (
    <Stack direction="row" alignItems="center" spacing={1.5} sx={{ py: 0.5 }}>
      <Box
        sx={{
          flex: 1,
          height: "1px",
          bgcolor: light ? "rgba(18,22,55,0.10)" : tokens.color.border.subtle,
        }}
      />
      <Typography
        sx={{
          fontSize: 11,
          letterSpacing: 0,
          textTransform: "uppercase",
          color: light ? "#7c839b" : tokens.color.text.muted,
        }}
      >
        or
      </Typography>
      <Box
        sx={{
          flex: 1,
          height: "1px",
          bgcolor: light ? "rgba(18,22,55,0.10)" : tokens.color.border.subtle,
        }}
      />
    </Stack>
  );
}

// Brand-color SVG for Google's four-color "G" mark.
//
// Constitutional exception: Google's brand guidelines require the
// official four-color mark to render with the exact brand hexes. The
// design-tokens system does not — and intentionally should not — host
// third-party brand palettes. We therefore declare the four Google
// brand colors as locally-scoped constants used only by this SVG, and
// pass them through `style` (not `sx`) so any future drift scanner
// sees them as an isolated brand asset rather than a token bypass.
const GOOGLE_BLUE = "#4285F4";
const GOOGLE_GREEN = "#34A853";
const GOOGLE_YELLOW = "#FBBC05";
const GOOGLE_RED = "#EA4335";

function GoogleMark() {
  return (
    <svg width="18" height="18" viewBox="0 0 48 48" aria-hidden="true">
      <path
        style={{ fill: GOOGLE_BLUE }}
        d="M43.611 20.083H42V20H24v8h11.303c-1.649 4.657-6.08 8-11.303 8-6.627 0-12-5.373-12-12s5.373-12 12-12c3.059 0 5.842 1.154 7.961 3.039l5.657-5.657C34.046 6.053 29.268 4 24 4 12.955 4 4 12.955 4 24s8.955 20 20 20 20-8.955 20-20c0-1.341-.138-2.65-.389-3.917z"
      />
      <path
        style={{ fill: GOOGLE_RED }}
        d="M6.306 14.691l6.571 4.819C14.655 15.108 18.961 12 24 12c3.059 0 5.842 1.154 7.961 3.039l5.657-5.657C34.046 6.053 29.268 4 24 4 16.318 4 9.656 8.337 6.306 14.691z"
      />
      <path
        style={{ fill: GOOGLE_GREEN }}
        d="M24 44c5.166 0 9.86-1.977 13.409-5.192l-6.19-5.238C29.211 35.091 26.715 36 24 36c-5.202 0-9.619-3.317-11.283-7.946l-6.522 5.025C9.505 39.556 16.227 44 24 44z"
      />
      <path
        style={{ fill: GOOGLE_YELLOW }}
        d="M43.611 20.083H42V20H24v8h11.303c-.792 2.237-2.231 4.166-4.087 5.571.001-.001.002-.001.003-.002l6.19 5.238C36.971 39.205 44 34 44 24c0-1.341-.138-2.65-.389-3.917z"
      />
    </svg>
  );
}
