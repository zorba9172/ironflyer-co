package main

import (
	"context"
	"flag"
	"time"

	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/operations/deploy"
)

// runApprovalsPending lists every open deploy_approvals row, scoped
// to a single tenant when --tenant is supplied. We talk to deploy
// through its Postgres service so the CLI shares the FSM + projection
// logic with the API path — no risk of seeing a divergent shape.
func runApprovalsPending(parent context.Context, args []string) error {
	fs := flag.NewFlagSet("approvals pending", flag.ContinueOnError)
	var (
		dsn    string
		tenant string
	)
	fs.StringVar(&dsn, "dsn", "", "Postgres DSN (defaults to POSTGRES_URL)")
	fs.StringVar(&tenant, "tenant", "", "tenant id filter (optional — empty enumerates all)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, cancel := withTimeout(parent)
	defer cancel()

	pool, err := openPool(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()

	// We intentionally hand a noop adapter map + nil ProfitGuard checker
	// to the deploy service — we only call read-only methods, so the
	// write-path deps are never exercised. zerolog.Nop keeps the CLI
	// silent unless we explicitly print.
	svc := deploy.NewPostgresService(pool, deploy.DefaultConfig(), zerolog.Nop(), nil, nil)

	var rows []deploy.Approval
	if tenant != "" {
		rows, err = svc.PendingApprovals(ctx, tenant)
		if err != nil {
			return err
		}
	} else {
		tenants, err := svc.TenantsWithPendingApprovals(ctx)
		if err != nil {
			return err
		}
		for _, t := range tenants {
			batch, err := svc.PendingApprovals(ctx, t)
			if err != nil {
				return err
			}
			rows = append(rows, batch...)
		}
	}
	return printJSON(map[string]any{
		"tenant":   tenant,
		"count":    len(rows),
		"pending":  rows,
		"reportAt": time.Now().UTC(),
	})
}
