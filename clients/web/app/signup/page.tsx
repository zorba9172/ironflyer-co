"use client";

// /signup — Base44-style split AuthShell hosting SignUpForm. On success
// we route to "/?welcome=1" so the home page can show a welcome banner
// and auto-launch any pending idea the visitor saved before bouncing
// into the sign-up flow. If the user is already signed in, we bounce
// them to the redirect target so a stale tab doesn't trap them here.

import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect } from "react";
import { AuthShell } from "../../src/components/auth/AuthShell";
import { SignUpForm } from "../../src/components/auth/SignUpForm";
import { useAuth } from "../../src/lib/auth";

function safeRedirect(value: string | null): string {
  if (!value) return "/?welcome=1";
  if (!value.startsWith("/") || value.startsWith("//")) return "/?welcome=1";
  // Preserve welcome banner if redirect points at the home page.
  if (value === "/" || value === "") return "/?welcome=1";
  return value;
}

export default function SignUpPage() {
  return (
    <Suspense fallback={null}>
      <SignUpPageInner />
    </Suspense>
  );
}

function SignUpPageInner() {
  const router = useRouter();
  const search = useSearchParams();
  const { authenticated, loading } = useAuth();

  // Accept ?redirect= / ?next= / ?returnTo= — keeps the home composer's
  // "continue building" flow working end to end.
  const redirect = safeRedirect(
    search?.get("redirect") ?? search?.get("next") ?? search?.get("returnTo") ?? null,
  );
  const loginHref =
    redirect && redirect !== "/?welcome=1"
      ? `/login?redirect=${encodeURIComponent(redirect)}`
      : "/login";

  useEffect(() => {
    if (loading) return;
    if (authenticated) router.replace(redirect);
  }, [authenticated, loading, redirect, router]);

  return (
    <AuthShell
      mode="signup"
      title="Create your account"
      subtitle="Start a workspace, generate your first preview and keep every project connected to Studio."
      switchHref={loginHref}
      switchPrompt="Already have an account?"
      switchAction="Sign in"
    >
      <SignUpForm onSuccess={() => router.replace(redirect)} />
    </AuthShell>
  );
}
