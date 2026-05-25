"use client";

// /login — Base44-style split AuthShell hosting SignInForm. Honours
// ?redirect= for routing back to the originating page after a
// successful sign-in. If the user is already signed in we bounce them
// straight to the redirect target so a stale tab doesn't look broken.

import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect } from "react";
import { AuthShell } from "../../src/components/auth/AuthShell";
import { SignInForm } from "../../src/components/auth/SignInForm";
import { useAuth } from "../../src/lib/auth";

function safeRedirect(value: string | null): string {
  // Same-origin path only — never trust a callback URL.
  if (!value) return "/";
  if (!value.startsWith("/") || value.startsWith("//")) return "/";
  return value;
}

export default function LoginPage() {
  return (
    <Suspense fallback={null}>
      <LoginPageInner />
    </Suspense>
  );
}

function LoginPageInner() {
  const router = useRouter();
  const search = useSearchParams();
  const { authenticated, loading } = useAuth();

  // Accept ?redirect= or ?next= — the brief uses next=, the prior
  // codebase uses redirect=. Honour either.
  const redirect = safeRedirect(
    search?.get("redirect") ?? search?.get("next") ?? null,
  );
  const signupHref =
    redirect && redirect !== "/"
      ? `/signup?redirect=${encodeURIComponent(redirect)}`
      : "/signup";

  useEffect(() => {
    if (loading) return;
    if (authenticated) router.replace(redirect);
  }, [authenticated, loading, redirect, router]);

  return (
    <AuthShell
      mode="signin"
      title="Sign in to continue"
      subtitle="Return to your Studio workspace, generated code, live previews and the deploy lane."
      switchHref={signupHref}
      switchPrompt="New here?"
      switchAction="Create an account"
    >
      <SignInForm onSuccess={() => router.replace(redirect)} />
    </AuthShell>
  );
}
