package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// openPool opens a pgxpool against POSTGRES_URL (or the --dsn override
// each subcommand wires through its own flag set). The pool is sized
// small — the CLI is short-lived and we want to minimise blast radius
// if the operator runs it against the live primary.
func openPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if dsn == "" {
		dsn = os.Getenv("POSTGRES_URL")
	}
	if dsn == "" {
		return nil, errors.New("missing POSTGRES_URL (or --dsn)")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	cfg.MaxConns = 2
	cfg.MinConns = 0
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	return pool, nil
}

// printJSON writes v to stdout as pretty-printed JSON. The CLI is a
// pipe-into-jq tool; pretty-print by default keeps it readable when
// the operator is just eyeballing the result.
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// withTimeout is the standard 30s context every subcommand uses so a
// stuck DB never wedges the operator's terminal.
func withTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 30*time.Second)
}

// usageError is what the top-level dispatcher prints when a subcommand
// is mis-invoked. Keeps the exit-code semantics consistent — 2 means
// "you typed the wrong thing", 1 means "the operation actually failed".
type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

func fail(err error) {
	fmt.Fprintf(os.Stderr, "ironflyer-ops: %v\n", err)
	var ue *usageError
	if errors.As(err, &ue) {
		os.Exit(2)
	}
	os.Exit(1)
}
