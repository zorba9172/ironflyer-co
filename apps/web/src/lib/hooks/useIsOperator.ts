"use client";

// useIsOperator — boolean predicate over `user.plan`. The same predicate
// is re-exported from `src/lib/auth` so existing call sites that import
// `useIsOperator` from there keep working without ripple refactors.

import { useAuth } from "../auth";

const OPERATOR_PLANS = new Set(["operator", "admin", "owner"]);

export function useIsOperator(): boolean {
  const { user } = useAuth();
  const plan = user?.plan;
  if (!plan) return false;
  return OPERATOR_PLANS.has(plan.toLowerCase());
}
