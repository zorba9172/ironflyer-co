// Package memory: SurrealDB-backed Store.
//
// SurrealStore persists memory records in SurrealDB so the four memory
// dimensions survive orchestrator restarts. The schema below is
// SCHEMALESS so we can add fields to Record without a migration.
//
// Storage layout:
//
//   table  memory_record (SCHEMALESS)
//   top-level fields (indexed for filtering / ordering):
//     kind, projectId, userId, storyId, gateName,
//     title, body, tags, confidence, createdAt
//   indexes:
//     memory_byKindProject  on (kind, projectId)
//     memory_byUser         on (userId)
//
// The record's own ID is the SurrealDB record id under the
// `memory_record:` prefix; we strip that prefix on read so callers see
// the same uuid string the MemoryStore would have produced.

package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"

	"ironflyer/core/orchestrator/internal/ai/embeddings"
)

// DefaultVectorDim is the embedding dimension assumed when none is
// configured. Tracks embeddings.DefaultModel ("BAAI/bge-m3", 1024-dim).
const DefaultVectorDim = 1024

// SurrealStore is the persistent backend for memory.Record.
// Safe for concurrent use — SurrealDB's connection handles serialisation.
//
// When an embedder is attached via WithVectorSearch, the store persists a
// per-record embedding inside SurrealDB and answers Substring queries with a
// native HNSW K-nearest-neighbour search (SurrealQL `<|K|>`). This makes
// semantic memory durable and cross-pod — unlike the in-process VectorStore
// wrapper whose vectors live in a sync.Map that dies with the process.
type SurrealStore struct {
	db       *surrealdb.DB
	embedder embeddings.Embedder
	vecDim   int
}

// NewSurrealStore wraps an already-connected *surrealdb.DB. Call
// BootstrapSurreal once at startup if you want the indexes installed.
func NewSurrealStore(db *surrealdb.DB) *SurrealStore {
	return &SurrealStore{db: db}
}

// WithVectorSearch enables native HNSW semantic search. embedder produces the
// per-record vectors; dim is the embedding dimension the HNSW index is defined
// for (must match the embedder's output). A zero/negative dim falls back to
// DefaultVectorDim. Nil embedder is a no-op (the store stays document-only).
func (s *SurrealStore) WithVectorSearch(embedder embeddings.Embedder, dim int) *SurrealStore {
	if dim <= 0 {
		dim = DefaultVectorDim
	}
	s.embedder = embedder
	s.vecDim = dim
	return s
}

// vectorEnabled reports whether native semantic search is wired.
func (s *SurrealStore) vectorEnabled() bool { return s.embedder != nil && s.vecDim > 0 }

// surrealMemorySchema is permissive — we add new fields to Record
// independently of operator-side migrations.
const surrealMemorySchema = `
DEFINE TABLE IF NOT EXISTS memory_record SCHEMALESS;
DEFINE FIELD IF NOT EXISTS kind       ON TABLE memory_record TYPE string;
DEFINE FIELD IF NOT EXISTS projectId  ON TABLE memory_record TYPE option<string>;
DEFINE FIELD IF NOT EXISTS userId     ON TABLE memory_record TYPE option<string>;
DEFINE FIELD IF NOT EXISTS storyId    ON TABLE memory_record TYPE option<string>;
DEFINE FIELD IF NOT EXISTS gateName   ON TABLE memory_record TYPE option<string>;
DEFINE FIELD IF NOT EXISTS title      ON TABLE memory_record TYPE option<string>;
DEFINE FIELD IF NOT EXISTS body       ON TABLE memory_record TYPE option<string>;
DEFINE FIELD IF NOT EXISTS tags       ON TABLE memory_record TYPE option<array<string>>;
DEFINE FIELD IF NOT EXISTS confidence ON TABLE memory_record TYPE option<number>;
DEFINE FIELD IF NOT EXISTS createdAt  ON TABLE memory_record TYPE datetime;
DEFINE FIELD IF NOT EXISTS embedding  ON TABLE memory_record TYPE option<array<float>>;
DEFINE INDEX IF NOT EXISTS memory_byKindProject ON TABLE memory_record COLUMNS kind, projectId;
DEFINE INDEX IF NOT EXISTS memory_byUser        ON TABLE memory_record COLUMNS userId;
DEFINE INDEX IF NOT EXISTS memory_byCreatedAt   ON TABLE memory_record COLUMNS createdAt;
`

// BootstrapSurrealVectorIndex defines the HNSW index on memory_record.embedding
// for native semantic search. Separate from the base schema because the index
// requires a concrete DIMENSION that must match the embedder. Idempotent
// (IF NOT EXISTS). COSINE distance + F32 matches the bge-m3 default; callers
// pass the embedder's true dimension.
func BootstrapSurrealVectorIndex(ctx context.Context, db *surrealdb.DB, dim int) error {
	if dim <= 0 {
		dim = DefaultVectorDim
	}
	stmt := fmt.Sprintf(
		"DEFINE INDEX IF NOT EXISTS memory_vec ON TABLE memory_record FIELDS embedding HNSW DIMENSION %d DIST COSINE TYPE F32;",
		dim)
	res, err := surrealdb.Query[any](ctx, db, stmt, nil)
	if err != nil {
		return fmt.Errorf("memory surreal vector index: %w", err)
	}
	if res != nil {
		for _, r := range *res {
			if r.Status != "OK" {
				return fmt.Errorf("memory surreal vector index status: %s", r.Status)
			}
		}
	}
	return nil
}

// BootstrapSurreal installs the memory_record table + indexes. Idempotent;
// safe to call on every boot.
func BootstrapSurreal(ctx context.Context, db *surrealdb.DB) error {
	res, err := surrealdb.Query[any](ctx, db, surrealMemorySchema, nil)
	if err != nil {
		return fmt.Errorf("memory surreal schema query: %w", err)
	}
	if res != nil {
		for _, r := range *res {
			if r.Status != "OK" {
				return fmt.Errorf("memory surreal schema statement failed: %s", r.Status)
			}
		}
	}
	return nil
}

// memoryRow is the read shape. It mirrors Record but uses
// models.CustomDateTime for the time field so the server returns it as
// a real datetime instead of a string. The id field is a Thing
// (table:identifier) so we extract the identifier portion in toRecord.
type memoryRow struct {
	ID         *models.RecordID      `json:"id,omitempty"`
	Kind       Kind                  `json:"kind"`
	ProjectID  string                `json:"projectId,omitempty"`
	UserID     string                `json:"userId,omitempty"`
	StoryID    string                `json:"storyId,omitempty"`
	GateName   string                `json:"gateName,omitempty"`
	Title      string                `json:"title,omitempty"`
	Body       string                `json:"body,omitempty"`
	Tags       []string              `json:"tags,omitempty"`
	Confidence float64               `json:"confidence,omitempty"`
	CreatedAt  models.CustomDateTime `json:"createdAt"`
}

func (r memoryRow) toRecord() Record {
	id := ""
	if r.ID != nil {
		// SurrealDB record IDs serialise as `table:id`; we already know
		// the table, so we keep just the identifier portion. The
		// underlying ID can be any CBOR value (string, uuid, etc.); we
		// stringify whatever is there.
		id = fmt.Sprintf("%v", r.ID.ID)
		// Trim wrapping quotes / brackets surrealdb sometimes emits.
		id = strings.Trim(id, "\"⟨⟩")
	}
	return Record{
		ID:         id,
		Kind:       r.Kind,
		ProjectID:  r.ProjectID,
		UserID:     r.UserID,
		StoryID:    r.StoryID,
		GateName:   r.GateName,
		Title:      r.Title,
		Body:       r.Body,
		Tags:       r.Tags,
		Confidence: r.Confidence,
		CreatedAt:  r.CreatedAt.Time,
	}
}

func toMemoryRowContent(r Record) map[string]any {
	doc := map[string]any{
		"kind":      string(r.Kind),
		"title":     r.Title,
		"body":      r.Body,
		"createdAt": models.CustomDateTime{Time: r.CreatedAt},
	}
	if r.ProjectID != "" {
		doc["projectId"] = r.ProjectID
	}
	if r.UserID != "" {
		doc["userId"] = r.UserID
	}
	if r.StoryID != "" {
		doc["storyId"] = r.StoryID
	}
	if r.GateName != "" {
		doc["gateName"] = r.GateName
	}
	if len(r.Tags) > 0 {
		doc["tags"] = r.Tags
	}
	if r.Confidence != 0 {
		doc["confidence"] = r.Confidence
	}
	return doc
}

// Record persists r. ID + CreatedAt are filled when zero (mirrors
// MemoryStore.Record). The stored record is returned so callers can
// attach the assigned id.
func (s *SurrealStore) Record(ctx context.Context, r Record) (Record, error) {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	// We use CREATE via SurrealQL with type::record so the record id is
	// our uuid string. The generic surrealdb.Create helper requires the
	// generic param to deserialise — keeping it via Query avoids issues
	// with empty struct{} type-params under CBOR.
	res, err := surrealdb.Query[any](ctx, s.db,
		"CREATE type::record('memory_record', $id) CONTENT $doc",
		map[string]any{"id": r.ID, "doc": toMemoryRowContent(r)})
	if err != nil {
		return Record{}, fmt.Errorf("memory surreal create: %w", err)
	}
	if res != nil {
		for _, qr := range *res {
			if qr.Status != "OK" {
				return Record{}, fmt.Errorf("memory surreal create status: %s", qr.Status)
			}
		}
	}
	// Persist the embedding out-of-band so the write stays off the
	// embedder's latency budget, but lands durably in SurrealDB (unlike the
	// in-process VectorStore cache). A failed embed leaves the record
	// searchable by substring — semantic search just skips it.
	if s.vectorEnabled() {
		if text := embedText(r); text != "" {
			id := r.ID
			go func() {
				vec, err := s.embedder.Embed(context.Background(), text)
				if err != nil || len(vec) == 0 {
					return
				}
				_, _ = surrealdb.Query[any](context.Background(), s.db,
					"UPDATE type::record('memory_record', $id) SET embedding = $vec",
					map[string]any{"id": id, "vec": vec})
			}()
		}
	}
	return r, nil
}

// embedText is the canonical "what to embed" for a record: title then body.
func embedText(r Record) string {
	text := r.Title
	if r.Body != "" {
		if text != "" {
			text += "\n\n"
		}
		text += r.Body
	}
	return text
}

// Query returns records newest-first that match every set field on q.
// An empty Query returns nothing — callers must scope by at least Kind,
// ProjectID, or UserID, mirroring the MemoryStore contract.
func (s *SurrealStore) Query(ctx context.Context, q Query) ([]Record, error) {
	if q.Kind == "" && q.ProjectID == "" && q.UserID == "" {
		return nil, nil
	}
	// Native semantic path: embed the substring and run an HNSW KNN search
	// in SurrealDB. Any failure (embedder down, KNN unsupported, no embedded
	// matches yet) falls through to the substring path below, so the store
	// never goes blind on a transient embedder/index hiccup.
	if s.vectorEnabled() && q.Substring != "" {
		if qvec, err := s.embedder.Embed(ctx, q.Substring); err == nil && len(qvec) > 0 {
			if recs, err := s.semanticQuery(ctx, q, qvec); err == nil && len(recs) > 0 {
				return recs, nil
			}
		}
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}

	vars := map[string]any{}
	var where []string
	if q.Kind != "" {
		where = append(where, "kind = $kind")
		vars["kind"] = string(q.Kind)
	}
	if q.ProjectID != "" {
		where = append(where, "projectId = $projectId")
		vars["projectId"] = q.ProjectID
	}
	if q.UserID != "" {
		where = append(where, "userId = $userId")
		vars["userId"] = q.UserID
	}
	if q.StoryID != "" {
		where = append(where, "storyId = $storyId")
		vars["storyId"] = q.StoryID
	}
	if q.GateName != "" {
		where = append(where, "gateName = $gateName")
		vars["gateName"] = q.GateName
	}
	if q.Tag != "" {
		where = append(where, "tags CONTAINS $tag")
		vars["tag"] = q.Tag
	}
	if q.Substring != "" {
		where = append(where,
			"(string::lowercase(title) CONTAINS $needle OR string::lowercase(body) CONTAINS $needle)")
		vars["needle"] = strings.ToLower(q.Substring)
	}

	sql := "SELECT * FROM memory_record"
	if len(where) > 0 {
		sql += " WHERE " + strings.Join(where, " AND ")
	}
	sql += " ORDER BY createdAt DESC LIMIT " + fmt.Sprintf("%d", limit)

	res, err := surrealdb.Query[[]memoryRow](ctx, s.db, sql, vars)
	if err != nil {
		return nil, fmt.Errorf("memory surreal query: %w", err)
	}
	out := make([]Record, 0, limit)
	if res != nil && len(*res) > 0 {
		for _, row := range (*res)[0].Result {
			out = append(out, row.toRecord())
		}
	}

	// Federated pass — same-user records from any of q.FederatedProjectIDs
	// (excluding the local q.ProjectID). Owner isolation is enforced via
	// the userId clause; without a non-empty UserID we refuse to widen.
	if q.IncludeFederated && q.UserID != "" && len(q.FederatedProjectIDs) > 0 {
		fedCap := q.FederatedLimit
		if fedCap <= 0 {
			fedCap = 5
		}
		// Strip the local project id from the federation set.
		fedIDs := make([]string, 0, len(q.FederatedProjectIDs))
		for _, pid := range q.FederatedProjectIDs {
			if pid == "" || pid == q.ProjectID {
				continue
			}
			fedIDs = append(fedIDs, pid)
		}
		if len(fedIDs) > 0 {
			fedVars := map[string]any{
				"userId":   q.UserID,
				"fedProjs": fedIDs,
			}
			fedWhere := []string{
				"userId = $userId",
				"projectId IN $fedProjs",
			}
			if q.Kind != "" {
				fedWhere = append(fedWhere, "kind = $kind")
				fedVars["kind"] = string(q.Kind)
			}
			if q.StoryID != "" {
				fedWhere = append(fedWhere, "storyId = $storyId")
				fedVars["storyId"] = q.StoryID
			}
			if q.GateName != "" {
				fedWhere = append(fedWhere, "gateName = $gateName")
				fedVars["gateName"] = q.GateName
			}
			if q.Tag != "" {
				fedWhere = append(fedWhere, "tags CONTAINS $tag")
				fedVars["tag"] = q.Tag
			}
			if q.Substring != "" {
				fedWhere = append(fedWhere,
					"(string::lowercase(title) CONTAINS $needle OR string::lowercase(body) CONTAINS $needle)")
				fedVars["needle"] = strings.ToLower(q.Substring)
			}
			fedSQL := "SELECT * FROM memory_record WHERE " +
				strings.Join(fedWhere, " AND ") +
				" ORDER BY createdAt DESC LIMIT " + fmt.Sprintf("%d", fedCap)
			fedRes, err := surrealdb.Query[[]memoryRow](ctx, s.db, fedSQL, fedVars)
			if err == nil && fedRes != nil && len(*fedRes) > 0 {
				fedRows := (*fedRes)[0].Result
				fedRecords := make([]Record, 0, len(fedRows))
				for _, row := range fedRows {
					fedRecords = append(fedRecords, row.toRecord())
				}
				fedRecords = AnnotateFederatedRecords(fedRecords, q.ProjectID)
				out = append(out, fedRecords...)
			}
		}
	}

	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// semanticQuery runs a native HNSW K-nearest-neighbour search over
// memory_record.embedding, scoped by the same filters as Query, ordered by
// vector distance (closest first). qvec is the query embedding. Records
// without a stored embedding simply don't match the KNN predicate.
func (s *SurrealStore) semanticQuery(ctx context.Context, q Query, qvec []float32) ([]Record, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}
	vars := map[string]any{"qvec": qvec}
	var where []string
	if q.Kind != "" {
		where = append(where, "kind = $kind")
		vars["kind"] = string(q.Kind)
	}
	if q.ProjectID != "" {
		where = append(where, "projectId = $projectId")
		vars["projectId"] = q.ProjectID
	}
	if q.UserID != "" {
		where = append(where, "userId = $userId")
		vars["userId"] = q.UserID
	}
	if q.StoryID != "" {
		where = append(where, "storyId = $storyId")
		vars["storyId"] = q.StoryID
	}
	if q.GateName != "" {
		where = append(where, "gateName = $gateName")
		vars["gateName"] = q.GateName
	}
	if q.Tag != "" {
		where = append(where, "tags CONTAINS $tag")
		vars["tag"] = q.Tag
	}
	// KNN predicate: SurrealDB 2.x HNSW form is `<|K,EF|>` — K results among
	// EF dynamic candidates (a larger EF trades latency for recall). Scope
	// predicates are placed BEFORE the operator (supported in 2.x). K and EF
	// are int literals we control (page size + candidate pool), so formatting
	// them into the statement is safe; the vector rides as a parameter.
	ef := limit * 8
	if ef < 64 {
		ef = 64
	}
	where = append(where, fmt.Sprintf("embedding <|%d,%d|> $qvec", limit, ef))
	// vector::distance::knn() returns the distance the KNN operator already
	// computed (no recomputation) so we can order by closeness.
	sql := "SELECT * FROM memory_record WHERE " + strings.Join(where, " AND ") +
		" ORDER BY vector::distance::knn() LIMIT " + fmt.Sprintf("%d", limit)

	res, err := surrealdb.Query[[]memoryRow](ctx, s.db, sql, vars)
	if err != nil {
		return nil, fmt.Errorf("memory surreal knn: %w", err)
	}
	out := make([]Record, 0, limit)
	if res != nil && len(*res) > 0 {
		for _, row := range (*res)[0].Result {
			out = append(out, row.toRecord())
		}
	}
	return out, nil
}

// GetByID returns the record with the supplied id, or ErrNotFound when
// nothing matches. Used by the HTTP layer to verify ownership before
// Delete (see TENANT ISOLATION INVARIANT on memory.Store).
func (s *SurrealStore) GetByID(ctx context.Context, id string) (Record, error) {
	res, err := surrealdb.Query[[]memoryRow](ctx, s.db,
		"SELECT * FROM type::record('memory_record', $id) LIMIT 1",
		map[string]any{"id": id})
	if err != nil {
		return Record{}, fmt.Errorf("memory surreal get: %w", err)
	}
	if res == nil || len(*res) == 0 || len((*res)[0].Result) == 0 {
		return Record{}, ErrNotFound
	}
	return (*res)[0].Result[0].toRecord(), nil
}

// Delete removes a record by id. Idempotent — deleting an unknown id
// returns nil so the HTTP layer can map DELETE to 204 unconditionally.
//
// TENANT ISOLATION: ownership is NOT enforced here. Callers must
// GetByID first and verify the resulting record's owner with
// auth.EnsureOwnerString.
func (s *SurrealStore) Delete(ctx context.Context, id string) error {
	res, err := surrealdb.Query[any](ctx, s.db,
		"DELETE type::record('memory_record', $id)",
		map[string]any{"id": id})
	if err != nil {
		return fmt.Errorf("memory surreal delete: %w", err)
	}
	if res != nil {
		for _, qr := range *res {
			if qr.Status != "OK" {
				return fmt.Errorf("memory surreal delete status: %s", qr.Status)
			}
		}
	}
	return nil
}

var _ Store = (*SurrealStore)(nil)

// RawQuery is an escape hatch for callers (currently the memorygraph
// package) that need to issue ad-hoc SurrealQL against the same
// connection without copying the schema-aware shape helpers above.
// It returns one map per row across every statement's first result
// page so the caller does not need to import surreal-internal types.
//
// This is intentionally narrow: no streaming, no per-statement
// segmentation, no type coercion. Callers MUST scope by tenantId in
// the query body — the helper does not enforce isolation.
func (s *SurrealStore) RawQuery(ctx context.Context, surql string, vars map[string]any) ([]map[string]any, error) {
	res, err := surrealdb.Query[[]map[string]any](ctx, s.db, surql, vars)
	if err != nil {
		return nil, fmt.Errorf("memory surreal raw query: %w", err)
	}
	if res == nil {
		return nil, nil
	}
	var out []map[string]any
	for _, qr := range *res {
		if qr.Status != "OK" {
			return nil, fmt.Errorf("memory surreal raw query status: %s", qr.Status)
		}
		out = append(out, qr.Result...)
	}
	return out, nil
}
