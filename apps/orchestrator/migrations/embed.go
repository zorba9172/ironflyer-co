// Package migrations exposes the versioned SQL files as an embedded FS so
// both cmd/orchestrator (startup-time auto-migrate) and cmd/migrate
// (operator CLI) share the exact same payload. New schema changes land
// here as numbered files following the goose +goose Up / +goose Down
// convention — never inside per-package CREATE TABLE blocks.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

// Dir is the FS sub-directory used by goose. The embed root already maps
// to the files at top level, so callers pass MigrationsDir = "." into
// goose.UpContext after goose.SetBaseFS(FS).
const Dir = "."
