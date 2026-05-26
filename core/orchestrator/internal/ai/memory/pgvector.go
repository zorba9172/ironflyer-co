// Package memory — Postgres + pgvector backed Store.
//
// PgVectorStore is the production alternative to SurrealStore for
// operators who already run Aurora Postgres (Round 11 Pulumi provisioned
// it) and don't want to stand up SurrealDB just for the four memory
// dimensions. It implements the same memory.Store contract as
// MemoryStore and SurrealStore — meaning federation, HTTP handlers, MCP
// tools, and resolvers see no difference. Selection happens at boot via
// MEMORY_BACKEND=pgvector.
//
// Storage layout (see migrations/00017_pgvector_memory.sql):
//
//   table memory_records
//     id          TEXT PRIMARY KEY
//     user_id     TEXT NOT NULL              -- owner clamp (mandatory)
//     project_id  TEXT                       -- scoping
//     kind        TEXT NOT NULL              -- project|execution|user|business
//     story_id    TEXT
//     gate_name   TEXT
//     title       TEXT
//     body        TEXT NOT NULL
//     tags        JSONB                      -- []string
//     confidence  DOUBLE PRECISION
//     embedding   vector(1024)               -- bge-m3 default; NULLable
//     metadata    JSONB                      -- reserved for future use
//     created_at  TIMESTAMPTZ NOT NULL
//     updated_at  TIMESTAMPTZ NOT NULL
//
// Vector search uses the cosine distance operator `<=>` combined with
// the HNSW index installed by the migration. The owner clamp lives in
// the WHERE clause (`user_id = $1`) so even a misconfigured caller can't
// surface another tenant's rows.
//
// Smart fallback: when the embedder is offline (no HF key, ONNX not
// built) the store still accepts Record (embedding column NULL) and
// Query/Search return zero results from the semantic path with a clear
// log line. This keeps the orchestrator bootable when memory is not on
// the critical path.

package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/ai/embeddings"
	"ironflyer/core/orchestrator/internal/operations/metrics"
)

// PgVectorStore persists memory.Record rows in Postgres with pgvector
// embeddings. Safe for concurrent use — pgxpool handles connection
// serialisation.
type PgVectorStore struct {
	pool     *pgxpool.Pool
	logger   zerolog.Logger
	embedder embeddings.Embedder
}

// NewPgVectorStore wraps an already-open pgxpool. The embedder is
// optional — when nil OR a NoopEmbedder, the store still works for
// Record / GetByID / Delete / non-semantic Query, just without vector
// re-ranking.
func NewPgVectorStore(pool *pgxpool.Pool, embedder embeddings.Embedder, logger zerolog.Logger) *PgVectorStore {
	return &PgVectorStore{pool: pool, logger: logger, embedder: embedder}
}

// embeddingDim is the column dim declared by migrations/00017. Operators
// who switch HF_EMBEDDINGS_MODEL to a model with a different output dim
// must ALTER the column type before turning pgvector on.
const embeddingDim = 1024

// Record persists r. ID + CreatedAt are filled when zero (mirrors the
// MemoryStore contract). When an embedder is configured the (title +
// body) text is encoded synchronously so the resulting vector is
// available for the next Search immediately; embed errors are logged but
// don't fail the write (smart fallback).
func (p *PgVectorStore) Record(ctx context.Context, r Record) (Record, error) {
	if p == nil || p.pool == nil {
		return Record{}, errors.New("pgvector: pool not configured")
	}
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}

	var tagsJSON []byte
	if len(r.Tags) > 0 {
		b, err := json.Marshal(r.Tags)
		if err != nil {
			return Record{}, fmt.Errorf("pgvector: marshal tags: %w", err)
		}
		tagsJSON = b
	}

	// Embed synchronously so the vector is present for the next Search.
	// Failures degrade to a NULL embedding column — Search will simply
	// skip this row in the cosine ranking until it's re-embedded.
	var vec any // pgvector.Vector or nil
	if emb := p.embedder; emb != nil {
		text := r.Title
		if r.Body != "" {
			if text != "" {
				text += "\n\n"
			}
			text += r.Body
		}
		if text != "" {
			if v, err := emb.Embed(ctx, text); err == nil && len(v) > 0 {
				if len(v) != embeddingDim {
					p.logger.Warn().
						Int("got", len(v)).
						Int("want", embeddingDim).
						Msg("pgvector: embedder returned unexpected dim; storing NULL")
				} else {
					vec = pgvector.NewVector(v)
				}
			} else if err != nil && !errors.Is(err, embeddings.ErrDisabled) {
				p.logger.Warn().Err(err).Msg("pgvector: embed failed; storing NULL embedding")
			}
		}
	}

	op := "add"
	defer observeOp(p, op, time.Now())
	_, err := p.pool.Exec(ctx, `
		INSERT INTO memory_records
		  (id, user_id, project_id, kind, story_id, gate_name,
		   title, body, tags, confidence, embedding, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$12)
		ON CONFLICT (id) DO UPDATE SET
		  user_id    = EXCLUDED.user_id,
		  project_id = EXCLUDED.project_id,
		  kind       = EXCLUDED.kind,
		  story_id   = EXCLUDED.story_id,
		  gate_name  = EXCLUDED.gate_name,
		  title      = EXCLUDED.title,
		  body       = EXCLUDED.body,
		  tags       = EXCLUDED.tags,
		  confidence = EXCLUDED.confidence,
		  embedding  = COALESCE(EXCLUDED.embedding, memory_records.embedding),
		  updated_at = EXCLUDED.updated_at
	`,
		r.ID,
		r.UserID,
		nullableText(r.ProjectID),
		string(r.Kind),
		nullableText(r.StoryID),
		nullableText(r.GateName),
		nullableText(r.Title),
		r.Body,
		nullableJSON(tagsJSON),
		r.Confidence,
		vec,
		r.CreatedAt,
	)
	if err != nil {
		return Record{}, fmt.Errorf("pgvector: insert: %w", err)
	}
	return r, nil
}

// Query returns records newest-first matching every set field on q.
// Mirrors MemoryStore.Query semantics: an empty query (no Kind +
// ProjectID + UserID) returns nil.
//
// When Substring is set AND an embedder is wired AND the encode
// succeeds, results are ordered by cosine distance to the query vector
// (HNSW-backed). Otherwise we order by created_at DESC and apply a
// case-insensitive substring filter inside the SQL.
//
// Federation is handled exactly as in surreal.go: a second query against
// the federation set with the SAME user_id clamp, results annotated with
// the source project id.
func (p *PgVectorStore) Query(ctx context.Context, q Query) ([]Record, error) {
	if p == nil || p.pool == nil {
		return nil, errors.New("pgvector: pool not configured")
	}
	if q.Kind == "" && q.ProjectID == "" && q.UserID == "" {
		return nil, nil
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}

	op := "list"
	if q.Substring != "" {
		op = "search"
	}
	defer observeOp(p, op, time.Now())

	var queryVec *pgvector.Vector
	if q.Substring != "" && p.embedder != nil {
		if v, err := p.embedder.Embed(ctx, q.Substring); err == nil && len(v) == embeddingDim {
			pv := pgvector.NewVector(v)
			queryVec = &pv
		} else if err != nil && !errors.Is(err, embeddings.ErrDisabled) {
			p.logger.Warn().Err(err).Msg("pgvector: query embed failed; falling back to substring filter")
		} else if err == nil && len(v) != embeddingDim {
			p.logger.Warn().
				Int("got", len(v)).
				Int("want", embeddingDim).
				Msg("pgvector: query embed dim mismatch; falling back to substring filter")
		}
	}

	out, err := p.runQuery(ctx, q, limit, queryVec, q.ProjectID, nil)
	if err != nil {
		return nil, err
	}

	// Federated pass — same-user records from any of FederatedProjectIDs
	// (excluding the local ProjectID). Owner clamp is unconditional via
	// the q.UserID filter inside runQuery.
	if q.IncludeFederated && q.UserID != "" && len(q.FederatedProjectIDs) > 0 {
		fedCap := q.FederatedLimit
		if fedCap <= 0 {
			fedCap = 5
		}
		fedIDs := make([]string, 0, len(q.FederatedProjectIDs))
		for _, pid := range q.FederatedProjectIDs {
			if pid == "" || pid == q.ProjectID {
				continue
			}
			fedIDs = append(fedIDs, pid)
		}
		if len(fedIDs) > 0 {
			fedOut, ferr := p.runQuery(ctx, q, fedCap, queryVec, "", fedIDs)
			if ferr != nil {
				p.logger.Warn().Err(ferr).Msg("pgvector: federation query failed")
			} else {
				fedOut = AnnotateFederatedRecords(fedOut, q.ProjectID)
				out = append(out, fedOut...)
			}
		}
	}

	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// runQuery is the shared SELECT path. When scopeProjectID is non-empty
// the SQL pins project_id to that single value (the local scan). When
// fedProjectIDs is non-empty the SQL pins project_id to ANY(fedProjectIDs)
// (the federated scan). The two are mutually exclusive in practice.
func (p *PgVectorStore) runQuery(
	ctx context.Context,
	q Query,
	limit int,
	queryVec *pgvector.Vector,
	scopeProjectID string,
	fedProjectIDs []string,
) ([]Record, error) {
	args := make([]any, 0, 8)
	where := make([]string, 0, 8)

	add := func(clauseFmt string, val any) {
		args = append(args, val)
		where = append(where, fmt.Sprintf(clauseFmt, len(args)))
	}

	if q.UserID != "" {
		add("user_id = $%d", q.UserID)
	}
	if q.Kind != "" {
		add("kind = $%d", string(q.Kind))
	}
	if scopeProjectID != "" {
		add("project_id = $%d", scopeProjectID)
	}
	if len(fedProjectIDs) > 0 {
		add("project_id = ANY($%d)", fedProjectIDs)
	}
	if q.StoryID != "" {
		add("story_id = $%d", q.StoryID)
	}
	if q.GateName != "" {
		add("gate_name = $%d", q.GateName)
	}
	if q.Tag != "" {
		// tags is JSONB array of strings; `?` checks string membership.
		add("tags ? $%d", q.Tag)
	}
	if q.Substring != "" && queryVec == nil {
		// Plain substring filter (LOWER on both sides). The semantic
		// path skips this clause so the HNSW ordering isn't pre-pruned.
		args = append(args, "%"+strings.ToLower(q.Substring)+"%")
		idx := len(args)
		where = append(where, fmt.Sprintf("(LOWER(title) LIKE $%d OR LOWER(body) LIKE $%d)", idx, idx))
	}

	sql := `SELECT id, user_id, project_id, kind, story_id, gate_name,
	               title, body, tags, confidence, created_at
	        FROM memory_records`
	if len(where) > 0 {
		sql += " WHERE " + strings.Join(where, " AND ")
	}
	if queryVec != nil {
		args = append(args, *queryVec)
		sql += fmt.Sprintf(" ORDER BY embedding <=> $%d NULLS LAST", len(args))
	} else {
		sql += " ORDER BY created_at DESC"
	}
	args = append(args, limit)
	sql += fmt.Sprintf(" LIMIT $%d", len(args))

	rows, err := p.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("pgvector: select: %w", err)
	}
	defer rows.Close()
	return scanRecords(rows)
}

// GetByID returns the record with the supplied id or ErrNotFound. The
// HTTP layer uses this to verify ownership before Delete.
func (p *PgVectorStore) GetByID(ctx context.Context, id string) (Record, error) {
	if p == nil || p.pool == nil {
		return Record{}, errors.New("pgvector: pool not configured")
	}
	defer observeOp(p, "get", time.Now())
	rows, err := p.pool.Query(ctx, `
		SELECT id, user_id, project_id, kind, story_id, gate_name,
		       title, body, tags, confidence, created_at
		FROM memory_records WHERE id = $1 LIMIT 1`, id)
	if err != nil {
		return Record{}, fmt.Errorf("pgvector: get: %w", err)
	}
	defer rows.Close()
	recs, err := scanRecords(rows)
	if err != nil {
		return Record{}, err
	}
	if len(recs) == 0 {
		return Record{}, ErrNotFound
	}
	return recs[0], nil
}

// Delete removes the row by id. Idempotent — deleting a missing id
// returns nil so the HTTP DELETE can map to 204. Ownership is NOT
// checked here; callers must GetByID + auth.EnsureOwnerString first.
func (p *PgVectorStore) Delete(ctx context.Context, id string) error {
	if p == nil || p.pool == nil {
		return errors.New("pgvector: pool not configured")
	}
	defer observeOp(p, "delete", time.Now())
	_, err := p.pool.Exec(ctx, `DELETE FROM memory_records WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("pgvector: delete: %w", err)
	}
	return nil
}

// ---- helpers -------------------------------------------------------------

func scanRecords(rows pgx.Rows) ([]Record, error) {
	out := []Record{}
	for rows.Next() {
		var (
			r           Record
			projectID   *string
			storyID     *string
			gateName    *string
			title       *string
			tagsJSON    []byte
			createdAt   time.Time
		)
		if err := rows.Scan(
			&r.ID,
			&r.UserID,
			&projectID,
			&r.Kind,
			&storyID,
			&gateName,
			&title,
			&r.Body,
			&tagsJSON,
			&r.Confidence,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("pgvector: scan: %w", err)
		}
		if projectID != nil {
			r.ProjectID = *projectID
		}
		if storyID != nil {
			r.StoryID = *storyID
		}
		if gateName != nil {
			r.GateName = *gateName
		}
		if title != nil {
			r.Title = *title
		}
		if len(tagsJSON) > 0 {
			_ = json.Unmarshal(tagsJSON, &r.Tags)
		}
		r.CreatedAt = createdAt
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pgvector: rows: %w", err)
	}
	return out, nil
}

func nullableText(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

// observeOp is the metrics hook. Call-sites read as
// `defer observeOp(p, op, time.Now())`: start a timer, record on return.
func observeOp(_ *PgVectorStore, op string, start time.Time) {
	metrics.ObserveMemoryOp("pgvector", op, time.Since(start))
}

var _ Store = (*PgVectorStore)(nil)
