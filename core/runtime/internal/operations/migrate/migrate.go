// Package migrate wraps goose so the runtime service brings its schema
// up to date on every boot. Migrations live in core/runtime/migrations/
// as numbered SQL files (goose format). Goose tracks applied versions in
// `goose_db_version`, so every entry point is idempotent.
//
// This mirrors core/orchestrator/internal/migrate so the two services
// behave identically: same dialect, same embedded-FS contract, same
// nil-safe RunPool helper that no-ops when running on a memory store.
package migrate

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // pgx stdlib driver for database/sql
	"github.com/pressly/goose/v3"
)

const (
	// MigrationsDir is the path inside the embedded FS that holds the
	// SQL files. The migrations package embeds *.sql at the root of its
	// FS, so callers pass "." after goose.SetBaseFS.
	MigrationsDir = "."

	dialect = "postgres"
)

// Up applies every pending migration. Safe to call repeatedly.
func Up(ctx context.Context, db *sql.DB, files fs.FS) error {
	return run(ctx, db, files, func(d *sql.DB, dir string) error {
		return goose.UpContext(ctx, d, dir)
	})
}

// FromPool wraps a pgxpool.Pool in a database/sql.DB suitable for goose.
func FromPool(pool *pgxpool.Pool) (*sql.DB, error) {
	if pool == nil {
		return nil, nil
	}
	cfg := pool.Config()
	if cfg == nil || cfg.ConnConfig == nil {
		return nil, errors.New("pgxpool: missing connection config")
	}
	return sql.Open("pgx", cfg.ConnString())
}

// RunPool is the convenience used by main.go: open a sql.DB from the
// pgx pool, apply Up, close. Skips silently when pool is nil so memory
// backends keep working.
func RunPool(ctx context.Context, pool *pgxpool.Pool, files fs.FS) error {
	if pool == nil {
		return nil
	}
	db, err := FromPool(pool)
	if err != nil {
		return err
	}
	if db == nil {
		return nil
	}
	defer db.Close()
	return Up(ctx, db, files)
}

func configure(files fs.FS) error {
	goose.SetBaseFS(files)
	return goose.SetDialect(dialect)
}

func run(ctx context.Context, db *sql.DB, files fs.FS, fn func(*sql.DB, string) error) error {
	if err := configure(files); err != nil {
		return err
	}
	return fn(db, MigrationsDir)
}
