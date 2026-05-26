// Package audit: SurrealDB-backed Store.
//
// SurrealStore persists the audit hash chain in SurrealDB so the log
// survives orchestrator restarts. The chain head (lastHash) is cached
// in-process to avoid a round-trip per write; on boot we read the most
// recent entry to seed it, so the chain continues across restarts —
// which is the entire point of moving off the in-process ring buffer.
//
// Storage layout:
//
//   table  audit_entry (SCHEMALESS)
//   top-level fields (indexed): userId, projectId, action, outcome,
//                               createdAt
//   payload: the canonical JSON of the Entry, byte-identical to what
//            hashEntry would marshal. Keeping the bytes verbatim is the
//            simplest way to keep Verify() honest across the
//            memory ↔ surreal boundary.

package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// SurrealStore is the persistent backend for audit.Entry. Implementations
// MUST preserve insertion order and refuse mutations to already-stored
// entries — both invariants are honoured here: rows are immutable once
// CREATEd, and we order by createdAt ASC for chain verification.
type SurrealStore struct {
	db       *surrealdb.DB
	mu       sync.Mutex
	lastHash string
}

// surrealAuditSchema is permissive. The payload column holds the
// canonical JSON; the top-level columns are denormalised projections so
// the query filters can hit indexes.
const surrealAuditSchema = `
DEFINE TABLE IF NOT EXISTS audit_entry SCHEMALESS;
DEFINE FIELD IF NOT EXISTS userId    ON TABLE audit_entry TYPE option<string>;
DEFINE FIELD IF NOT EXISTS projectId ON TABLE audit_entry TYPE option<string>;
DEFINE FIELD IF NOT EXISTS action    ON TABLE audit_entry TYPE string;
DEFINE FIELD IF NOT EXISTS outcome   ON TABLE audit_entry TYPE option<string>;
DEFINE FIELD IF NOT EXISTS createdAt ON TABLE audit_entry TYPE datetime;
DEFINE FIELD IF NOT EXISTS payload   ON TABLE audit_entry TYPE string;
DEFINE INDEX IF NOT EXISTS audit_byUser      ON TABLE audit_entry COLUMNS userId;
DEFINE INDEX IF NOT EXISTS audit_byProject   ON TABLE audit_entry COLUMNS projectId;
DEFINE INDEX IF NOT EXISTS audit_byAction    ON TABLE audit_entry COLUMNS action;
DEFINE INDEX IF NOT EXISTS audit_byCreatedAt ON TABLE audit_entry COLUMNS createdAt;
`

// NewSurrealStore installs the audit_entry schema (idempotent), seeds
// the in-process lastHash by reading the most recent entry, and returns
// a ready-to-use Store. The "BIG win" the package doc mentions is that
// after a crash + restart, new entries chain onto the persisted head
// rather than starting a fresh chain.
func NewSurrealStore(ctx context.Context, db *surrealdb.DB) (*SurrealStore, error) {
	if err := bootstrapSurrealAudit(ctx, db); err != nil {
		return nil, err
	}
	s := &SurrealStore{db: db}
	if err := s.loadLastHash(ctx); err != nil {
		return nil, fmt.Errorf("audit surreal head: %w", err)
	}
	return s, nil
}

func bootstrapSurrealAudit(ctx context.Context, db *surrealdb.DB) error {
	res, err := surrealdb.Query[any](ctx, db, surrealAuditSchema, nil)
	if err != nil {
		return fmt.Errorf("audit surreal schema query: %w", err)
	}
	if res != nil {
		for _, r := range *res {
			if r.Status != "OK" {
				return fmt.Errorf("audit surreal schema statement failed: %s", r.Status)
			}
		}
	}
	return nil
}

// loadLastHash reads the most recent entry's contentHash so the next
// Record() call chains onto it instead of starting a new chain after a
// process restart.
func (s *SurrealStore) loadLastHash(ctx context.Context) error {
	res, err := surrealdb.Query[[]auditRow](ctx, s.db,
		"SELECT payload FROM audit_entry ORDER BY createdAt DESC LIMIT 1", nil)
	if err != nil {
		return err
	}
	if res == nil || len(*res) == 0 || len((*res)[0].Result) == 0 {
		s.lastHash = ""
		return nil
	}
	e, err := (*res)[0].Result[0].decode()
	if err != nil {
		return err
	}
	s.lastHash = e.ContentHash
	return nil
}

// auditRow is the on-disk shape. payload holds the canonical JSON of
// the Entry. We project the indexed columns up so SurrealDB can use
// them in WHERE clauses; the authoritative read comes from payload.
type auditRow struct {
	Payload string `json:"payload"`
}

func (r auditRow) decode() (Entry, error) {
	var e Entry
	if r.Payload == "" {
		return e, nil
	}
	if err := json.Unmarshal([]byte(r.Payload), &e); err != nil {
		return e, fmt.Errorf("audit payload decode: %w", err)
	}
	return e, nil
}

func toAuditContent(e Entry) (map[string]any, error) {
	payload, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("audit payload encode: %w", err)
	}
	doc := map[string]any{
		"action":    string(e.Action),
		"createdAt": models.CustomDateTime{Time: e.CreatedAt},
		"payload":   string(payload),
	}
	if e.UserID != "" {
		doc["userId"] = e.UserID
	}
	if e.ProjectID != "" {
		doc["projectId"] = e.ProjectID
	}
	if e.Outcome != "" {
		doc["outcome"] = string(e.Outcome)
	}
	return doc, nil
}

// Record appends an entry. ID + CreatedAt + PrevHash + ContentHash are
// filled here so the chain stays internally consistent. The mutex
// serialises writes so concurrent Recorders can't interleave PrevHash.
func (s *SurrealStore) Record(ctx context.Context, e Entry) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e.ID = uuid.NewString()
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	e.PrevHash = s.lastHash
	e.ContentHash = hashEntry(e)

	doc, err := toAuditContent(e)
	if err != nil {
		return Entry{}, err
	}
	res, err := surrealdb.Query[any](ctx, s.db,
		"CREATE type::record('audit_entry', $id) CONTENT $doc",
		map[string]any{"id": e.ID, "doc": doc})
	if err != nil {
		return Entry{}, fmt.Errorf("audit surreal create: %w", err)
	}
	if res != nil {
		for _, qr := range *res {
			if qr.Status != "OK" {
				return Entry{}, fmt.Errorf("audit surreal create status: %s", qr.Status)
			}
		}
	}
	s.lastHash = e.ContentHash
	return e, nil
}

// Query returns entries newest-first that match every set field.
// Limit 0 defaults to 100 (matches the MemoryStore contract).
func (s *SurrealStore) Query(ctx context.Context, q Query) ([]Entry, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 100
	}

	vars := map[string]any{}
	var where []string
	if q.UserID != "" {
		where = append(where, "userId = $userId")
		vars["userId"] = q.UserID
	}
	if q.ProjectID != "" {
		where = append(where, "projectId = $projectId")
		vars["projectId"] = q.ProjectID
	}
	if q.Action != "" {
		where = append(where, "action = $action")
		vars["action"] = string(q.Action)
	}
	if q.Outcome != "" {
		where = append(where, "outcome = $outcome")
		vars["outcome"] = string(q.Outcome)
	}
	if !q.Since.IsZero() {
		where = append(where, "createdAt >= $since")
		vars["since"] = models.CustomDateTime{Time: q.Since}
	}
	if !q.Until.IsZero() {
		where = append(where, "createdAt <= $until")
		vars["until"] = models.CustomDateTime{Time: q.Until}
	}

	var sqlBuf strings.Builder
	sqlBuf.Grow(96)
	sqlBuf.WriteString("SELECT payload FROM audit_entry")
	if len(where) > 0 {
		sqlBuf.WriteString(" WHERE ")
		sqlBuf.WriteString(strings.Join(where, " AND "))
	}
	sqlBuf.WriteString(" ORDER BY createdAt DESC LIMIT ")
	sqlBuf.WriteString(strconv.Itoa(limit))
	sql := sqlBuf.String()

	res, err := surrealdb.Query[[]auditRow](ctx, s.db, sql, vars)
	if err != nil {
		return nil, fmt.Errorf("audit surreal query: %w", err)
	}
	if res == nil || len(*res) == 0 {
		return nil, nil
	}
	rows := (*res)[0].Result
	out := make([]Entry, 0, len(rows))
	for _, row := range rows {
		e, err := row.decode()
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// Verify walks the audit_entry table in insertion order (createdAt ASC)
// and recomputes the hash chain. Returns the index of the first
// inconsistency or -1 if the log is intact.
func (s *SurrealStore) Verify(ctx context.Context) (int, error) {
	res, err := surrealdb.Query[[]auditRow](ctx, s.db,
		"SELECT payload FROM audit_entry ORDER BY createdAt ASC", nil)
	if err != nil {
		return 0, fmt.Errorf("audit surreal verify: %w", err)
	}
	if res == nil || len(*res) == 0 {
		return -1, nil
	}
	rows := (*res)[0].Result
	prev := ""
	for i, row := range rows {
		e, err := row.decode()
		if err != nil {
			return i, err
		}
		if e.PrevHash != prev {
			return i, nil
		}
		check := e
		check.ContentHash = ""
		if hashEntry(check) != e.ContentHash {
			return i, nil
		}
		prev = e.ContentHash
	}
	return -1, nil
}

var _ Store = (*SurrealStore)(nil)
