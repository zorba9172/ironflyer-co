// Package schema embeds the ClickHouse DDL applied at orchestrator
// startup. The files are split numerically so dependencies (raw →
// fact → rollup) are applied in order.
//
// Each file is a sequence of CREATE TABLE / CREATE MATERIALIZED VIEW
// IF NOT EXISTS statements — safe to re-run on every boot.
package schema

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed *.sql
var FS embed.FS

// Statement is one DDL statement extracted from an embedded SQL file.
type Statement struct {
	File string
	Body string
}

// Load reads every embedded *.sql file in lexical order and splits
// each into individual statements on ";\n" boundaries. CLickHouse's
// Go driver cannot execute a multi-statement batch in a single Exec
// so the caller iterates and applies one statement at a time.
func Load() ([]Statement, error) {
	entries, err := fs.ReadDir(FS, ".")
	if err != nil {
		return nil, fmt.Errorf("clickhouse schema: read dir: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, ".sql") {
			names = append(names, n)
		}
	}
	sort.Strings(names)

	out := make([]Statement, 0, 64)
	for _, n := range names {
		body, err := fs.ReadFile(FS, n)
		if err != nil {
			return nil, fmt.Errorf("clickhouse schema: read %s: %w", n, err)
		}
		for _, stmt := range splitStatements(string(body)) {
			out = append(out, Statement{File: n, Body: stmt})
		}
	}
	return out, nil
}

// splitStatements parses a SQL file into individual statements. It
// strips full-line "--" comments and splits on top-level ";". Inline
// comments after code on the same line are preserved (ClickHouse
// accepts them).
func splitStatements(src string) []string {
	out := make([]string, 0, 16)
	var b strings.Builder
	for _, line := range strings.Split(src, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "--") || trim == "" {
			// Keep blank lines and full-line comments out of the
			// statement body so the resulting string is compact.
			b.WriteByte('\n')
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
		if strings.HasSuffix(trim, ";") {
			stmt := strings.TrimSpace(b.String())
			stmt = strings.TrimSuffix(stmt, ";")
			stmt = strings.TrimSpace(stmt)
			if stmt != "" {
				out = append(out, stmt)
			}
			b.Reset()
		}
	}
	if leftover := strings.TrimSpace(b.String()); leftover != "" {
		leftover = strings.TrimSuffix(leftover, ";")
		leftover = strings.TrimSpace(leftover)
		if leftover != "" {
			out = append(out, leftover)
		}
	}
	return out
}
