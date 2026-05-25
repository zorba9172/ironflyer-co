package main

import (
	"context"
	"errors"
	"flag"

	"ironflyer/apps/orchestrator/internal/abuse"
)

// runAbuseScore prints the (score, tier) pair for the supplied
// (tenant, user). We construct an abuse Engine from the Postgres
// store + LoadConfig so the result matches what the live orchestrator
// would compute — the in-process score cache is a Local-only
// optimisation, the underlying score read goes to Postgres.
func runAbuseScore(parent context.Context, args []string) error {
	fs := flag.NewFlagSet("abuse score", flag.ContinueOnError)
	var (
		dsn    string
		tenant string
		user   string
	)
	fs.StringVar(&dsn, "dsn", "", "Postgres DSN (defaults to POSTGRES_URL)")
	fs.StringVar(&tenant, "tenant", "", "tenant id (required)")
	fs.StringVar(&user, "user", "", "user id (optional — empty resolves the tenant-wide score)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if tenant == "" {
		return &usageError{msg: "abuse score: --tenant is required"}
	}

	ctx, cancel := withTimeout(parent)
	defer cancel()

	pool, err := openPool(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()

	store := abuse.NewPostgresStore(pool)
	if store == nil {
		return errors.New("abuse: failed to build postgres store")
	}
	engine := abuse.NewEngine(abuse.LoadConfig(), store)
	score, tier, err := engine.Score(ctx, tenant, user)
	if err != nil {
		return err
	}
	return printJSON(map[string]any{
		"tenant": tenant,
		"user":   user,
		"score":  score,
		"tier":   string(tier),
	})
}
