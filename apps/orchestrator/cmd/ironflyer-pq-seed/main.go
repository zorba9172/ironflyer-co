// ironflyer-pq-seed scans the repo's .graphql / .gql files, extracts
// every query / mutation / subscription operation, computes the
// sha256 each one hashes to under the persisted-query allowlist, and
// inserts the rows into the persisted_queries table (migration 00036).
//
// The seeder exists because production mode of the GraphQL hardening
// middleware rejects any operation that is not in the allowlist
// (operator principals bypass). New client builds therefore have to
// register their query shapes before the deploy ships, and this CLI
// is the canonical way to do that without round-tripping every query
// through the live API.
//
// Usage:
//
//	ironflyer-pq-seed \
//	  -dsn $POSTGRES_URL \
//	  -scan apps/web,packages/sdk,apps/vscode-extension \
//	  -tenant <uuid> \
//	  -dry-run
//
// Idempotent: re-running the seeder is safe — INSERT ... ON CONFLICT
// DO NOTHING leaves existing rows untouched.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vektah/gqlparser/v2/ast"
	gqlparser "github.com/vektah/gqlparser/v2/parser"
)

func main() {
	var (
		dsn      string
		scanDirs string
		tenantID string
		dryRun   bool
	)
	flag.StringVar(&dsn, "dsn", os.Getenv("POSTGRES_URL"), "Postgres DSN (defaults to POSTGRES_URL)")
	flag.StringVar(&scanDirs, "scan", "apps/web,packages/sdk,apps/vscode-extension", "comma-separated dirs to scan for .graphql/.gql files")
	flag.StringVar(&tenantID, "tenant", "", "registered_by_tenant_id UUID (optional)")
	flag.BoolVar(&dryRun, "dry-run", false, "print extracted queries without inserting")
	flag.Parse()

	if dsn == "" && !dryRun {
		fatal(errors.New("missing -dsn (or POSTGRES_URL env)"))
	}
	dirs := splitDirs(scanDirs)
	if len(dirs) == 0 {
		fatal(errors.New("no -scan directories provided"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	queries, scanned, parseFailures := collectQueries(dirs)
	if len(queries) == 0 {
		fmt.Fprintf(os.Stderr, "pq-seed: scanned %d files, no operations found (%d parse failures)\n", scanned, parseFailures)
		return
	}

	if dryRun {
		printDryRun(queries)
		fmt.Fprintf(os.Stderr, "pq-seed: dry-run — scanned=%d found=%d parseFailures=%d\n",
			scanned, len(queries), parseFailures)
		return
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fatal(fmt.Errorf("connect: %w", err))
	}
	defer pool.Close()

	inserted, skipped, err := insertQueries(ctx, pool, queries, tenantID)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("pq-seed: scanned=%d found=%d inserted=%d skipped=%d parseFailures=%d\n",
		scanned, len(queries), inserted, skipped, parseFailures)
}

// extracted is one operation pulled out of one .graphql file.
type extracted struct {
	Hash     string
	Query    string
	Op       string // operationName ("" for anonymous)
	SourceFn string // filename for log lines
}

// collectQueries walks every supplied directory, parses each .graphql
// / .gql file with gqlparser, and returns the canonicalised operation
// bodies + their sha256s. Files that fail to parse are skipped with a
// warning — many `.graphql` files in the repo are schema-only and
// gqlparser's query parser rejects them; that is expected.
func collectQueries(dirs []string) ([]extracted, int, int) {
	var out []extracted
	scanned := 0
	parseFailures := 0
	for _, dir := range dirs {
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				// Missing directory or unreadable file — keep walking
				// rather than failing the whole seed run.
				return nil
			}
			if d.IsDir() {
				if shouldSkipDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".graphql" && ext != ".gql" {
				return nil
			}
			scanned++
			body, rerr := os.ReadFile(path)
			if rerr != nil {
				fmt.Fprintf(os.Stderr, "pq-seed: warn: read %s: %v\n", path, rerr)
				return nil
			}
			ops, perr := parseOperations(path, string(body))
			if perr != nil {
				parseFailures++
				fmt.Fprintf(os.Stderr, "pq-seed: warn: parse %s: %v (likely schema-only file)\n", path, perr)
				return nil
			}
			for _, op := range ops {
				out = append(out, extracted{
					Hash:     hashQuery(op.body),
					Query:    op.body,
					Op:       op.name,
					SourceFn: path,
				})
			}
			return nil
		})
	}
	return out, scanned, parseFailures
}

type parsedOp struct {
	name string
	body string
}

// parseOperations runs gqlparser over the file body and returns one
// parsedOp per OperationDefinition. The body slice we hand back is the
// exact substring of the source file that contains the operation —
// using positions rather than the formatter keeps the sha256 stable
// against the bytes the client already sees, which is what the Apollo
// APQ contract requires.
func parseOperations(path, body string) ([]parsedOp, error) {
	doc, err := gqlparser.ParseQuery(&ast.Source{Name: path, Input: body})
	if err != nil {
		return nil, err
	}
	var out []parsedOp
	for _, op := range doc.Operations {
		text := operationText(body, op)
		if strings.TrimSpace(text) == "" {
			continue
		}
		out = append(out, parsedOp{name: op.Name, body: text})
	}
	return out, nil
}

// operationText extracts the raw source slice for one operation. If
// the AST position metadata is missing we fall back to the whole
// document so the operation still seeds — the sha256 will be of the
// full file in that edge case.
func operationText(body string, op *ast.OperationDefinition) string {
	if op == nil || op.Position == nil || op.Position.Src == nil {
		return strings.TrimSpace(body)
	}
	start := op.Position.Start
	end := op.Position.End
	if start < 0 || end <= start || end > len(body) {
		return strings.TrimSpace(body)
	}
	return strings.TrimSpace(body[start:end])
}

func hashQuery(q string) string {
	sum := sha256.Sum256([]byte(q))
	return hex.EncodeToString(sum[:])
}

// insertQueries pushes every extracted op into persisted_queries with
// INSERT ... ON CONFLICT DO NOTHING. The conflict count surfaces as
// "skipped" so re-runs produce a clean diff on the operator's screen.
func insertQueries(ctx context.Context, pool *pgxpool.Pool, queries []extracted, tenantID string) (int, int, error) {
	inserted := 0
	skipped := 0
	var tenantArg any = tenantID
	if strings.TrimSpace(tenantID) == "" {
		tenantArg = nil
	}
	for _, q := range queries {
		var opArg any = q.Op
		if q.Op == "" {
			opArg = nil
		}
		ct, err := pool.Exec(ctx,
			`INSERT INTO persisted_queries (hash, query, operation_name, registered_by_tenant_id)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (hash) DO NOTHING`,
			q.Hash, q.Query, opArg, tenantArg,
		)
		if err != nil {
			return inserted, skipped, fmt.Errorf("insert %s (%s): %w", q.Hash, q.SourceFn, err)
		}
		if ct.RowsAffected() == 0 {
			skipped++
		} else {
			inserted++
		}
	}
	return inserted, skipped, nil
}

func printDryRun(queries []extracted) {
	for _, q := range queries {
		name := q.Op
		if name == "" {
			name = "<anonymous>"
		}
		fmt.Printf("# %s — %s — %s\n%s\n\n", q.Hash, name, q.SourceFn, q.Query)
	}
}

// shouldSkipDir filters out directories that frequently contain
// generated or vendored GraphQL fragments we should not try to seed.
func shouldSkipDir(name string) bool {
	switch name {
	case "node_modules", ".next", "dist", "build", ".git", "generated":
		return true
	}
	return false
}

func splitDirs(csv string) []string {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "pq-seed: %v\n", err)
	os.Exit(1)
}
