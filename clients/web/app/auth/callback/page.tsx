"use client";

// /auth/callback — terminal page for the OAuth round-trip. The
// orchestrator finishes the provider exchange and 302s back here with
// the JWT in the URL fragment (`#token=...&expiresAt=...`); fragments
// never reach the server, so the token cannot leak through proxy
// access logs or referrer headers.
//
// On mount we parse the fragment, write the token via setToken so
// Apollo's link picks it up, reset the store so any cached identity
// from a prior session is dropped, then bounce to the safe `next`
// target (default "/").

import { useApolloClient } from "@apollo/client";
import { Box, Button, CircularProgress, Stack, Typography } from "@mui/material";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";
import { setToken } from "../../../src/lib/apollo";
import { tokens } from "../../../src/theme";

function safeNext(value: string | null): string {
  if (!value) return "/";
  if (!value.startsWith("/") || value.startsWith("//")) return "/";
  return value;
}

function parseFragment(hash: string): Record<string, string> {
  const out: Record<string, string> = {};
  const raw = hash.startsWith("#") ? hash.slice(1) : hash;
  if (!raw) return out;
  for (const pair of raw.split("&")) {
    if (!pair) continue;
    const idx = pair.indexOf("=");
    if (idx === -1) {
      out[decodeURIComponent(pair)] = "";
      continue;
    }
    const k = decodeURIComponent(pair.slice(0, idx));
    const v = decodeURIComponent(pair.slice(idx + 1));
    out[k] = v;
  }
  return out;
}

export default function AuthCallbackPage() {
  return (
    <Suspense fallback={<CallbackChrome label="Completing sign-in…" />}>
      <AuthCallbackInner />
    </Suspense>
  );
}

function AuthCallbackInner() {
  const router = useRouter();
  const search = useSearchParams();
  const client = useApolloClient();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (typeof window === "undefined") return;
    const frag = parseFragment(window.location.hash);
    const token = frag.token;
    if (!token) {
      setError("Sign-in did not complete: no token returned.");
      return;
    }

    try {
      setToken(token);
    } catch {
      setError("Could not persist your session. Please try again.");
      return;
    }

    const next = safeNext(frag.next ?? search?.get("next") ?? null);

    void client
      .resetStore()
      .catch(() => undefined)
      .finally(() => {
        if (typeof window !== "undefined") {
          window.history.replaceState(null, "", window.location.pathname);
        }
        router.replace(next);
      });
  }, [client, router, search]);

  if (error) return <CallbackError message={error} />;
  return <CallbackChrome label="Completing sign-in…" />;
}

function CallbackChrome({ label }: { label: string }) {
  return (
    <Box
      sx={{
        minHeight: "100vh",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        bgcolor: tokens.color.bg.base,
        color: tokens.color.text.primary,
        px: 3,
      }}
    >
      <Stack spacing={2} alignItems="center">
        <CircularProgress size={28} sx={{ color: tokens.color.accent.violet }} />
        <Typography sx={{ fontSize: 14, color: tokens.color.text.secondary }}>
          {label}
        </Typography>
      </Stack>
    </Box>
  );
}

function CallbackError({ message }: { message: string }) {
  return (
    <Box
      sx={{
        minHeight: "100vh",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        bgcolor: tokens.color.bg.base,
        color: tokens.color.text.primary,
        px: 3,
      }}
    >
      <Stack spacing={2.5} alignItems="center" sx={{ maxWidth: 360, textAlign: "center" }}>
        <Typography sx={{ fontSize: 18, fontWeight: 600 }}>
          Sign-in could not complete
        </Typography>
        <Typography sx={{ fontSize: 13, color: tokens.color.text.secondary }}>
          {message}
        </Typography>
        <Button
          component={Link}
          href="/login"
          variant="contained"
          color="primary"
          sx={{ minHeight: 40, fontSize: 13 }}
        >
          Back to sign-in
        </Button>
      </Stack>
    </Box>
  );
}
