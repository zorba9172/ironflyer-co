"use client";

// /wallet — prepaid credit balance + top-up + history. Two columns on
// desktop, single column on mobile. The wallet is the V22 law-1
// contract; this is the only surface that lets the user load credits.

import { Box, Stack } from "@mui/material";
import {
  EmptyState,
  ErrorPanel,
  LoadingPanel,
  PageHeader,
} from "../../src/components/cockpit";
import { LedgerPreview } from "../../src/components/wallet/LedgerPreview";
import { TopUpCard } from "../../src/components/wallet/TopUpCard";
import { TopUpHistory } from "../../src/components/wallet/TopUpHistory";
import { WalletBalanceCard } from "../../src/components/wallet/WalletBalanceCard";
import { RequireAuth } from "../../src/lib/auth";
// We keep the raw useWalletQuery here (not the shared useWalletBalance
// hook) because the wallet detail page needs the full record shape —
// lifetimeTopUpUSD, lifetimeSpendUSD, updatedAt — that the hook
// deliberately doesn't expose. The hook is the cockpit-chip / dashboard
// display contract; this page is the source of truth surface.
import { useWalletQuery } from "../../src/lib/gql/__generated__";

export default function WalletPage() {
  return (
    <RequireAuth>
      <WalletView />
    </RequireAuth>
  );
}

function WalletView() {
  const walletQuery = useWalletQuery({
    fetchPolicy: "cache-and-network",
    pollInterval: 15000,
  });

  const wallet = walletQuery.data?.wallet;

  return (
    <Box>
      <PageHeader
        title="Wallet"
        description="Prepaid credit balance. Every paid execution holds against your available balance before it starts, then debits the ledger as cost lands."
      />

      {walletQuery.loading && !wallet ? (
        <LoadingPanel label="Loading wallet" />
      ) : walletQuery.error && !wallet ? (
        <ErrorPanel
          error={walletQuery.error}
          title="Could not load wallet"
          onRetry={() => void walletQuery.refetch()}
        />
      ) : !wallet ? (
        <EmptyState
          title="Wallet not provisioned"
          body="The orchestrator has not opened a wallet for this tenant yet. Reload once you have completed signup or contact support."
        />
      ) : (
        <Box
          sx={{
            display: "grid",
            gap: 2.5,
            gridTemplateColumns: { xs: "1fr", lg: "3fr 2fr" },
          }}
        >
          <Stack spacing={2.5}>
            <WalletBalanceCard
              availableUSD={wallet.availableUSD}
              holdUSD={wallet.holdUSD}
              lifetimeTopUpUSD={wallet.lifetimeTopUpUSD}
              lifetimeSpendUSD={wallet.lifetimeSpendUSD}
              updatedAt={wallet.updatedAt}
            />
            <TopUpCard />
          </Stack>
          <Stack spacing={2.5}>
            <TopUpHistory />
            <LedgerPreview />
          </Stack>
        </Box>
      )}
    </Box>
  );
}
