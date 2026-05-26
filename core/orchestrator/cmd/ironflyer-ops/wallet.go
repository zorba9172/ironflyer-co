package main

import (
	"context"
	"flag"

	"ironflyer/core/orchestrator/internal/business/wallet"
)

// runWalletShow prints the wallet balance, hold, and lifetime
// counters for a tenant. We talk to wallet.PostgresService directly so
// the operator sees exactly the row the API would render — no
// shimming, no caching.
func runWalletShow(parent context.Context, args []string) error {
	fs := flag.NewFlagSet("wallet show", flag.ContinueOnError)
	var (
		dsn    string
		tenant string
	)
	fs.StringVar(&dsn, "dsn", "", "Postgres DSN (defaults to POSTGRES_URL)")
	fs.StringVar(&tenant, "tenant", "", "tenant id (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if tenant == "" {
		return &usageError{msg: "wallet show: --tenant is required"}
	}

	ctx, cancel := withTimeout(parent)
	defer cancel()

	pool, err := openPool(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()

	svc := wallet.NewPostgresService(pool)
	w, err := svc.Get(ctx, tenant)
	if err != nil {
		return err
	}
	return printJSON(map[string]any{
		"tenantId":         w.TenantID,
		"balanceUsd":       w.BalanceUSD,
		"holdUsd":          w.HoldUSD,
		"availableUsd":     w.AvailableUSD(),
		"lifetimeTopUpUsd": w.LifetimeTopUpUSD,
		"lifetimeSpendUsd": w.LifetimeSpendUSD,
		"updatedAt":        w.UpdatedAt,
		"createdAt":        w.CreatedAt,
	})
}
