// migrate is the operator-side CLI for applying / rolling back the
// orchestrator's schema migrations independently of the orchestrator
// pod. Designed to run from a Helm pre-install / pre-upgrade Job so the
// database is at the right schema version BEFORE the orchestrator pod
// starts answering traffic.
//
// Usage:
//
//	migrate up        # apply every pending migration
//	migrate down      # roll back the most recent migration
//	migrate status    # print the applied / pending list
//	migrate version   # print the currently-applied version
//
// Reads POSTGRES_URL from the environment (same env var the orchestrator
// uses). Exits non-zero on failure so Helm marks the Job failed and
// blocks the install.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"ironflyer/core/orchestrator/internal/operations/migrate"
	"ironflyer/core/orchestrator/migrations"
)

const usage = `migrate — orchestrator schema migrator

Usage:
  migrate up       Apply every pending migration
  migrate down     Roll back the most recent migration
  migrate status   Print the migration ledger
  migrate version  Print the currently-applied version

Environment:
  POSTGRES_URL     PostgreSQL DSN (required)
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmd := strings.ToLower(strings.TrimSpace(os.Args[1]))

	dsn := strings.TrimSpace(os.Getenv("POSTGRES_URL"))
	if dsn == "" {
		fail(errors.New("POSTGRES_URL is required"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fail(fmt.Errorf("open postgres: %w", err))
	}
	defer db.Close()

	pingCtx, pingCancel := context.WithTimeout(ctx, 10*time.Second)
	defer pingCancel()
	if err := db.PingContext(pingCtx); err != nil {
		fail(fmt.Errorf("ping postgres: %w", err))
	}

	switch cmd {
	case "up":
		if err := migrate.Up(ctx, db, migrations.FS); err != nil {
			fail(fmt.Errorf("migrate up: %w", err))
		}
		fmt.Println("migrations applied")
	case "down":
		if err := migrate.Down(ctx, db, migrations.FS); err != nil {
			fail(fmt.Errorf("migrate down: %w", err))
		}
		fmt.Println("most recent migration rolled back")
	case "status":
		if err := migrate.Status(ctx, db, migrations.FS); err != nil {
			fail(fmt.Errorf("migrate status: %w", err))
		}
	case "version":
		v, err := migrate.Version(ctx, db, migrations.FS)
		if err != nil {
			fail(fmt.Errorf("migrate version: %w", err))
		}
		fmt.Printf("current version: %d\n", v)
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
	os.Exit(1)
}
