package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"

	"ironflyer/apps/orchestrator/internal/domain"
)

// SurrealStore persists projects in SurrealDB as documents in the `project`
// table. The whole project graph (files, gates, events) is stored as a nested
// `data` object so the table stays schemaless to additions.
type SurrealStore struct {
	mu  sync.Mutex
	db  *surrealdb.DB
	ctx context.Context
}

type SurrealOpts struct {
	URL       string
	Namespace string
	Database  string
	User      string
	Pass      string
}

func ConnectSurreal(ctx context.Context, opts SurrealOpts) (*surrealdb.DB, error) {
	if opts.URL == "" {
		return nil, errors.New("SurrealDB URL empty")
	}
	db, err := surrealdb.FromEndpointURLString(ctx, opts.URL)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	if _, err := db.SignIn(ctx, surrealdb.Auth{
		Username: opts.User, Password: opts.Pass,
	}); err != nil {
		return nil, fmt.Errorf("signin: %w", err)
	}
	if err := db.Use(ctx, opts.Namespace, opts.Database); err != nil {
		return nil, fmt.Errorf("use: %w", err)
	}
	return db, nil
}

// surrealSchema is intentionally permissive — project documents may grow new
// fields per release and we don't want migrations gating feature work.
const surrealSchema = `
DEFINE TABLE IF NOT EXISTS project SCHEMALESS;
DEFINE FIELD IF NOT EXISTS name        ON TABLE project TYPE string;
DEFINE FIELD IF NOT EXISTS description ON TABLE project TYPE option<string>;
DEFINE FIELD IF NOT EXISTS status      ON TABLE project TYPE string;
DEFINE FIELD IF NOT EXISTS owner_id    ON TABLE project TYPE option<string>;
DEFINE FIELD IF NOT EXISTS data        ON TABLE project TYPE object;
DEFINE FIELD IF NOT EXISTS created_at  ON TABLE project TYPE datetime;
DEFINE FIELD IF NOT EXISTS updated_at  ON TABLE project TYPE datetime;
DEFINE INDEX IF NOT EXISTS project_status ON TABLE project COLUMNS status;
`

func BootstrapSurreal(ctx context.Context, db *surrealdb.DB) error {
	res, err := surrealdb.Query[any](ctx, db, surrealSchema, nil)
	if err != nil {
		return fmt.Errorf("schema query: %w", err)
	}
	if res != nil {
		for _, r := range *res {
			if r.Status != "OK" {
				return fmt.Errorf("schema statement failed: %s", r.Status)
			}
		}
	}
	return nil
}

func NewSurrealStore(ctx context.Context, db *surrealdb.DB) *SurrealStore {
	return &SurrealStore{db: db, ctx: ctx}
}

// projectRow is the read shape. We rebuild domain.Project from the nested
// `data` field. SurrealDB returns `id` as a Thing — we don't need it on read
// because the data already carries the project's own ID.
type projectRow struct {
	Data domain.Project `json:"data"`
}

// buildContent constructs the CREATE/UPDATE payload. Top-level fields mirror
// the most-queried bits; `data` carries everything else. We deliberately do
// NOT include an `id` key — `type::record('project', $id)` sets the record ID.
func buildContent(p domain.Project) map[string]any {
	doc := map[string]any{
		"name":        p.Name,
		"description": p.Description,
		"status":      p.Status,
		"data":        p,
		"created_at":  models.CustomDateTime{Time: p.CreatedAt},
		"updated_at":  models.CustomDateTime{Time: p.UpdatedAt},
	}
	if p.OwnerID != "" {
		doc["owner_id"] = p.OwnerID
	}
	// Surface artifacts at the top level too so they can be queried/indexed
	// without unpacking the nested `data` blob. The nested copy under `data`
	// remains authoritative on read — we never read this field back.
	if len(p.Artifacts) > 0 {
		doc["artifacts"] = p.Artifacts
	}
	return doc
}

func (s *SurrealStore) List() []domain.Project {
	rows, err := surrealdb.Query[[]projectRow](s.ctx, s.db,
		"SELECT data FROM project", nil)
	if err != nil || rows == nil || len(*rows) == 0 {
		return nil
	}
	first := (*rows)[0]
	out := make([]domain.Project, 0, len(first.Result))
	for _, r := range first.Result {
		out = append(out, r.Data)
	}
	return out
}

func (s *SurrealStore) Get(id string) (domain.Project, error) {
	rows, err := surrealdb.Query[[]projectRow](s.ctx, s.db,
		"SELECT data FROM type::record('project', $id)",
		map[string]any{"id": id})
	if err != nil {
		return domain.Project{}, err
	}
	if rows == nil || len(*rows) == 0 || len((*rows)[0].Result) == 0 {
		return domain.Project{}, ErrNotFound
	}
	return (*rows)[0].Result[0].Data, nil
}

func (s *SurrealStore) Create(p domain.Project) (domain.Project, error) {
	if p.ID == "" {
		return domain.Project{}, errors.New("project id required")
	}
	if _, err := s.Get(p.ID); err == nil {
		return domain.Project{}, errors.New("project already exists")
	}
	if p.Gates == nil {
		p.Gates = emptyGates(time.Now().UTC())
	}
	res, err := surrealdb.Query[any](s.ctx, s.db,
		"CREATE type::record('project', $id) CONTENT $doc",
		map[string]any{"id": p.ID, "doc": buildContent(p)})
	if err != nil {
		return domain.Project{}, fmt.Errorf("surreal create: %w", err)
	}
	if res != nil {
		for _, r := range *res {
			if r.Status != "OK" {
				return domain.Project{}, fmt.Errorf("surreal create status: %s", r.Status)
			}
		}
	}
	return p, nil
}

func (s *SurrealStore) Update(id string, fn func(*domain.Project)) (domain.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.Get(id)
	if err != nil {
		return domain.Project{}, err
	}
	fn(&p)
	p.UpdatedAt = time.Now().UTC()
	res, err := surrealdb.Query[any](s.ctx, s.db,
		"UPDATE type::record('project', $id) CONTENT $doc",
		map[string]any{"id": id, "doc": buildContent(p)})
	if err != nil {
		return domain.Project{}, fmt.Errorf("surreal update: %w", err)
	}
	if res != nil {
		for _, r := range *res {
			if r.Status != "OK" {
				return domain.Project{}, fmt.Errorf("surreal update status: %s", r.Status)
			}
		}
	}
	return p, nil
}

// Delete removes a project document from SurrealDB.
func (s *SurrealStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := surrealdb.Query[any](s.ctx, s.db,
		"DELETE type::record('project', $id)",
		map[string]any{"id": id})
	if err != nil {
		return fmt.Errorf("surreal delete: %w", err)
	}
	if res != nil {
		for _, r := range *res {
			if r.Status != "OK" {
				return fmt.Errorf("surreal delete status: %s", r.Status)
			}
		}
	}
	return nil
}

// Seed inserts the demo project if it doesn't exist. Returns the error
// rather than swallowing it.
func (s *SurrealStore) Seed() error {
	if _, err := s.Get("demo"); err == nil {
		return nil
	}
	now := time.Now().UTC()
	_, err := s.Create(domain.Project{
		ID:          "demo",
		Name:        "Demo SaaS Workspace",
		Description: "Prompt-to-product execution workspace.",
		Status:      "ready",
		Spec: domain.ProductSpec{
			Idea: "A lightweight invoicing tool for freelancers.",
			Stack: domain.StackDecision{
				Frontend: "Next.js + MUI",
				Backend:  "Go stdlib",
				Storage:  "Postgres",
				Auth:     "JWT",
			},
		},
		Files: []domain.FileNode{
			{Path: "apps/web/app/page.tsx", Type: "file"},
			{Path: "apps/api/main.go", Type: "file"},
			{Path: "README.md", Type: "file"},
		},
		CreatedAt: now, UpdatedAt: now,
	})
	return err
}

var _ Store = (*SurrealStore)(nil)
