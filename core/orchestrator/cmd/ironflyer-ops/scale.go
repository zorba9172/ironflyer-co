package main

import (
	"context"
	"flag"
	"os"
	"strconv"
	"strings"

	"ironflyer/core/orchestrator/internal/business/execution"
)

// runScaleSnapshot reports active + queued execution counts plus
// sandbox capacity. SandboxCapacity comes from
// IRONFLYER_SANDBOX_CAPACITY because the runtime quota config is
// owned by the runtime app, not the orchestrator — surfacing the
// number the operator already configured beats inventing a new one.
func runScaleSnapshot(parent context.Context, args []string) error {
	fs := flag.NewFlagSet("scale snapshot", flag.ContinueOnError)
	var dsn string
	fs.StringVar(&dsn, "dsn", "", "Postgres DSN (defaults to POSTGRES_URL)")
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

	exec := execution.NewPostgres(pool)
	active, err := exec.ActiveCount(ctx)
	if err != nil {
		return err
	}
	queued, err := exec.QueuedCount(ctx)
	if err != nil {
		return err
	}

	capacity := 0
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_SANDBOX_CAPACITY")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			capacity = n
		}
	}
	util := 0.0
	if capacity > 0 {
		util = float64(active) / float64(capacity) * 100.0
	}

	return printJSON(map[string]any{
		"activeExecutions":     active,
		"queuedExecutions":     queued,
		"sandboxCapacity":      capacity,
		"workerUtilizationPct": util,
	})
}
