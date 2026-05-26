// Package migrate wraps goose so the orchestrator (and the standalone
// `migrate` CLI) share one entry point. Migrations live in
// core/orchestrator/migrations/ as numbered SQL files (goose format).
// Goose tracks applied versions in `goose_db_version`, so every entry
// point is idempotent.
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

// Up applies every pending migration. Safe to call repeatedly; goose
// stores applied versions in `goose_db_version`.
func Up(ctx context.Context, db *sql.DB, files fs.FS) error {
	return run(ctx, db, files, func(d *sql.DB, dir string) error {
		return goose.UpContext(ctx, d, dir)
	})
}

// Down rolls back the most recent migration. Production rollbacks
// should be exceptional; prefer roll-forward fixes.
func Down(ctx context.Context, db *sql.DB, files fs.FS) error {
	return run(ctx, db, files, func(d *sql.DB, dir string) error {
		return goose.DownContext(ctx, d, dir)
	})
}

// Status prints the current migration state to goose's logger.
func Status(ctx context.Context, db *sql.DB, files fs.FS) error {
	return run(ctx, db, files, func(d *sql.DB, dir string) error {
		return goose.StatusContext(ctx, d, dir)
	})
}

// Version returns the current applied version number.
func Version(ctx context.Context, db *sql.DB, files fs.FS) (int64, error) {
	if err := configure(files); err != nil {
		return 0, err
	}
	return goose.GetDBVersionContext(ctx, db)
}

// FromPool wraps a pgxpool.Pool in a database/sql.DB suitable for goose.
// The returned *sql.DB owns its own connections (separate from pgx's
// pool) and should be Close()'d by the caller. Returns nil if pool is
// nil so callers can skip migrations cleanly when running on memory
// backends.
func FromPool(pool *pgxpool.Pool) (*sql.DB, error) {
	if pool == nil {
		return nil, nil
	}
	cfg := pool.Config()
	if cfg == nil || cfg.ConnConfig == nil {
		return nil, errors.New("pgxpool: missing connection config")
	}
	// pgx's stdlib driver accepts the same DSN.
	return sql.Open("pgx", cfg.ConnString())
}

// RunPool is a convenience for the orchestrator: open a sql.DB from the
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
