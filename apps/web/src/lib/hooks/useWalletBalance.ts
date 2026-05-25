"use client";

// useWalletBalance — single source of truth for wallet display across the
// cockpit. Wraps useWalletQuery with the polling + cache policy every
// caller previously copy-pasted, and adds a derived `lowBalance` flag so
// nav chips and settings can share the same threshold ($5).
//
// Skipped when the caller is unauthenticated so we never fire a wallet
// query against a missing JWT.

import type { ApolloError } from "@apollo/client";
import { useAuth } from "../auth";
import { useWalletQuery } from "../gql/__generated__";

const POLL_INTERVAL_MS = 30_000;
const LOW_BALANCE_USD = 5;

export interface WalletBalance {
  availableUSD: number;
  reservedUSD: number;
  totalUSD: number;
  loading: boolean;
  error: ApolloError | undefined;
  refetch: () => Promise<unknown>;
  lowBalance: boolean;
}

export function useWalletBalance(): WalletBalance {
  const { authenticated } = useAuth();
  const { data, loading, error, refetch } = useWalletQuery({
    skip: !authenticated,
    fetchPolicy: "cache-and-network",
    pollInterval: authenticated ? POLL_INTERVAL_MS : 0,
  });

  const wallet = data?.wallet;
  const availableUSD = wallet?.availableUSD ?? 0;
  const reservedUSD = wallet?.holdUSD ?? 0;
  const totalUSD = wallet?.balanceUSD ?? 0;
  const lowBalance =
    authenticated && wallet !== undefined && availableUSD < LOW_BALANCE_USD;

  return {
    availableUSD,
    reservedUSD,
    totalUSD,
    loading,
    error,
    refetch,
    lowBalance,
  };
}
