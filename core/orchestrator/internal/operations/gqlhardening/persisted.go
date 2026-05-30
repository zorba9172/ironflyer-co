package gqlhardening

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// openRegistration reports whether the IRONFLYER_PERSISTED_OPEN_REGISTRATION
// env var lets non-operator callers register persisted queries on first
// touch (auto-APQ). Useful when no build-time seed is shipped and the
// SPA is the source of truth for the allow-list.
func openRegistration() bool {
	v := strings.TrimSpace(os.Getenv("IRONFLYER_PERSISTED_OPEN_REGISTRATION"))
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// Store is the persisted-query allowlist. In dev mode the in-memory
// store is fine; production wires a Postgres-backed store against the
// persisted_queries table (migration 00036).
type Store interface {
	Get(ctx context.Context, hash string) (query string, found bool, err error)
	Register(ctx context.Context, hash, query, opName, byTenant string) error
	Touch(ctx context.Context, hash string) error
	// Count returns the number of registered (active) persisted queries
	// in the store. Used by the startup hardening banner so an operator
	// can see at a glance how locked-down the APQ surface is. Best
	// effort: implementations may return 0, nil when a count is
	// expensive or unavailable.
	Count(ctx context.Context) (int, error)
}

// OperatorCheck reports whether the request's principal is an operator
// — operators bypass the production allowlist so they can run ad-hoc
// queries through Sandbox / the CLI without pre-registering every
// shape. The wiring agent supplies the closure.
type OperatorCheck func(ctx context.Context) bool

// HashQuery returns the lowercase hex sha256 of `query`. Apollo's APQ
// protocol uses the same digest, which lets existing clients reuse
// their hashing code unchanged.
func HashQuery(query string) string {
	sum := sha256.Sum256([]byte(query))
	return hex.EncodeToString(sum[:])
}

// MemoryStore is an in-process Store keyed by hash. Useful in dev,
// tests, and as the fallback when Postgres isn't wired in.
type MemoryStore struct {
	mu      sync.RWMutex
	entries map[string]memoryEntry
}

type memoryEntry struct {
	query    string
	opName   string
	tenant   string
	created  time.Time
	lastUsed time.Time
	useCount int64
}

// NewMemoryStore builds an empty in-process persisted-query store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{entries: map[string]memoryEntry{}}
}

func (m *MemoryStore) Get(_ context.Context, hash string) (string, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.entries[hash]
	return e.query, ok, nil
}

func (m *MemoryStore) Register(_ context.Context, hash, query, opName, byTenant string) error {
	if hash == "" || query == "" {
		return errors.New("gqlhardening: register requires non-empty hash + query")
	}
	if HashQuery(query) != hash {
		return errors.New("gqlhardening: hash does not match query")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[hash] = memoryEntry{
		query:   query,
		opName:  opName,
		tenant:  byTenant,
		created: time.Now(),
	}
	return nil
}

func (m *MemoryStore) Count(_ context.Context) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entries), nil
}

func (m *MemoryStore) Touch(_ context.Context, hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[hash]
	if !ok {
		return nil
	}
	e.lastUsed = time.Now()
	e.useCount++
	m.entries[hash] = e
	return nil
}

// PostgresStore is the production Store backed by pgxpool against the
// persisted_queries table (migration 00036). Touch is best-effort —
// rate-limit telemetry, not part of the hot path.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore wires the persisted-query store to an existing
// pgxpool. The wiring agent owns pool lifecycle.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (p *PostgresStore) Get(ctx context.Context, hash string) (string, bool, error) {
	if p == nil || p.pool == nil {
		return "", false, errors.New("gqlhardening: nil postgres pool")
	}
	var query string
	err := p.pool.QueryRow(ctx,
		`SELECT query FROM persisted_queries WHERE hash = $1 AND active = true`,
		hash,
	).Scan(&query)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return query, true, nil
}

func (p *PostgresStore) Register(ctx context.Context, hash, query, opName, byTenant string) error {
	if p == nil || p.pool == nil {
		return errors.New("gqlhardening: nil postgres pool")
	}
	if HashQuery(query) != hash {
		return errors.New("gqlhardening: hash does not match query")
	}
	var tenantArg any = byTenant
	if byTenant == "" {
		tenantArg = nil
	}
	var opArg any = opName
	if opName == "" {
		opArg = nil
	}
	_, err := p.pool.Exec(ctx,
		`INSERT INTO persisted_queries (hash, query, operation_name, registered_by_tenant_id)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (hash) DO UPDATE
		   SET active = true,
		       operation_name = COALESCE(persisted_queries.operation_name, EXCLUDED.operation_name)`,
		hash, query, opArg, tenantArg,
	)
	return err
}

func (p *PostgresStore) Count(ctx context.Context) (int, error) {
	if p == nil || p.pool == nil {
		return 0, nil
	}
	var n int
	err := p.pool.QueryRow(ctx,
		`SELECT count(*) FROM persisted_queries WHERE active = true`,
	).Scan(&n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (p *PostgresStore) Touch(ctx context.Context, hash string) error {
	if p == nil || p.pool == nil {
		return nil
	}
	_, _ = p.pool.Exec(ctx,
		`UPDATE persisted_queries SET last_used_at = now(), use_count = use_count + 1 WHERE hash = $1`,
		hash,
	)
	return nil
}

// PersistedQueriesMiddleware honours the Apollo APQ extension on
// POST /graphql. The semantics, distilled from the Apollo spec:
//
//   - Body has both `query` and `extensions.persistedQuery.sha256Hash`
//     → verify the hash, register the query, run the query.
//   - Body has only the hash → look the query up; if missing, return
//     PERSISTED_QUERY_NOT_FOUND so the client falls back to sending the
//     full query on the next request.
//   - Body has only the query → in prodMode, reject unless the
//     principal is an operator. In dev, pass through.
//
// The middleware rewrites the request body before forwarding so the
// downstream gqlgen handler sees a fully-resolved `query` field.
func PersistedQueriesMiddleware(store Store, prodMode bool, isOperator OperatorCheck) func(http.Handler) http.Handler {
	const maxGraphQLJSONBody = 2 << 20
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || store == nil {
				next.ServeHTTP(w, r)
				return
			}
			body, err := io.ReadAll(io.LimitReader(r.Body, maxGraphQLJSONBody+1))
			_ = r.Body.Close()
			if err != nil {
				writePersistedError(w, http.StatusBadRequest, "INVALID_BODY", "failed to read request body")
				return
			}
			if len(body) > maxGraphQLJSONBody {
				writePersistedError(w, http.StatusRequestEntityTooLarge, "BODY_TOO_LARGE", "graphql request body is too large")
				return
			}
			var raw map[string]any
			if err := json.Unmarshal(body, &raw); err != nil || raw == nil {
				// Not a JSON GraphQL request — let the handler fail the
				// body-parse with its own native error.
				r.Body = io.NopCloser(bytes.NewReader(body))
				r.ContentLength = int64(len(body))
				next.ServeHTTP(w, r)
				return
			}
			query, _ := raw["query"].(string)
			opName, _ := raw["operationName"].(string)
			hash, hasHash := extractHash(raw)

			switch {
			case hasHash && query != "":
				// Hash + query: verify + register, then continue.
				if HashQuery(query) != hash {
					writePersistedError(w, http.StatusBadRequest, "HASH_MISMATCH", "persisted query hash does not match query")
					return
				}
				if prodMode && !operatorAllowed(r.Context(), isOperator) && !openRegistration() {
					// In strict prodMode the registration path is
					// operator-only — applications should pre-register
					// allowed shapes during build/deploy. Set
					// IRONFLYER_PERSISTED_OPEN_REGISTRATION=true to
					// allow first-touch registration from any caller
					// (auth-required mutations like signIn need this).
					writePersistedError(w, http.StatusForbidden, "PERSISTED_QUERY_REGISTER_FORBIDDEN", "registering persisted queries requires operator role in production")
					return
				}
				if err := store.Register(r.Context(), hash, query, opName, tenantFromBody(raw)); err != nil {
					writePersistedError(w, http.StatusServiceUnavailable, "PERSISTED_QUERY_REGISTER_FAILED", "persisted query registration failed")
					return
				}
				persistedHits.WithLabelValues("register").Inc()
				_ = store.Touch(r.Context(), hash)
			case hasHash:
				resolved, found, err := store.Get(r.Context(), hash)
				if err != nil {
					writePersistedError(w, http.StatusServiceUnavailable, "PERSISTED_QUERY_LOOKUP_FAILED", "persisted query lookup failed")
					return
				}
				if !found {
					persistedHits.WithLabelValues("miss").Inc()
					writePersistedError(w, http.StatusOK, "PERSISTED_QUERY_NOT_FOUND", "PersistedQueryNotFound")
					return
				}
				raw["query"] = resolved
				persistedHits.WithLabelValues("hit").Inc()
				_ = store.Touch(r.Context(), hash)
			case query != "":
				// Plain query, no hash.
				if prodMode && !operatorAllowed(r.Context(), isOperator) {
					writePersistedError(w, http.StatusForbidden, "PERSISTED_QUERY_REQUIRED", "production accepts persisted queries only")
					return
				}
				// Dev mode: pass through unchanged.
			default:
				// Neither query nor hash — let downstream error.
			}

			newBody, err := json.Marshal(raw)
			if err != nil {
				writePersistedError(w, http.StatusInternalServerError, "INTERNAL", "failed to rewrite request body")
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(newBody))
			r.ContentLength = int64(len(newBody))
			next.ServeHTTP(w, r)
		})
	}
}

func extractHash(raw map[string]any) (string, bool) {
	ext, ok := raw["extensions"].(map[string]any)
	if !ok {
		return "", false
	}
	pq, ok := ext["persistedQuery"].(map[string]any)
	if !ok {
		return "", false
	}
	h, _ := pq["sha256Hash"].(string)
	return h, h != ""
}

func tenantFromBody(raw map[string]any) string {
	// We don't trust the body to declare the tenant — the wiring agent
	// stamps an X-Tenant header from the auth context in front of us,
	// but here we keep the value empty so the DB row carries NULL
	// rather than a forged ID.
	_ = raw
	return ""
}

func operatorAllowed(ctx context.Context, isOperator OperatorCheck) bool {
	if isOperator == nil {
		return false
	}
	return isOperator(ctx)
}

func writePersistedError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	payload := map[string]any{
		"errors": []map[string]any{{
			"message": message,
			"extensions": map[string]any{
				"code": code,
			},
		}},
	}
	_ = json.NewEncoder(w).Encode(payload)
}

// statusError is a placeholder used by callers that want a typed
// error from the middleware. Currently unused externally but kept so
// future callers can match on it.
type statusError struct {
	status int
	code   string
	msg    string
}

func (e *statusError) Error() string { return fmt.Sprintf("%s: %s", e.code, e.msg) }
