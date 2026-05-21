// Package leads persists enterprise sales-qualifying form submissions so
// they survive process restarts and can be reviewed without trawling logs.
//
// Two backends:
//   - MemoryStore — dev / test default; no infra.
//   - PostgresStore — production; uses the orchestrator's shared pgxpool.
package leads

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Lead is the persisted form submission. Fields are intentionally loose
// (free text) to capture whatever the lead writes; downstream CRM ingest can
// re-shape later.
type Lead struct {
	ID        string    `json:"id"`
	Name      string    `json:"name,omitempty"`
	Email     string    `json:"email"`
	Company   string    `json:"company"`
	TeamSize  string    `json:"teamSize,omitempty"`
	UseCase   string    `json:"useCase,omitempty"`
	Budget    string    `json:"budget,omitempty"`
	Timeline  string    `json:"timeline,omitempty"`
	Source    string    `json:"source,omitempty"`
	UserAgent string    `json:"userAgent,omitempty"`
	IP        string    `json:"ip,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// Store is the persistence contract — memory + postgres both implement it.
type Store interface {
	Create(ctx context.Context, l Lead) (Lead, error)
	List(ctx context.Context, limit int) ([]Lead, error)
}

// MemoryStore is the dev/test implementation.
type MemoryStore struct {
	mu    sync.RWMutex
	items []Lead
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{} }

func (s *MemoryStore) Create(_ context.Context, l Lead) (Lead, error) {
	if l.ID == "" {
		l.ID = "lead_" + uuid.NewString()
	}
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now().UTC()
	}
	s.mu.Lock()
	s.items = append(s.items, l)
	s.mu.Unlock()
	return l, nil
}

func (s *MemoryStore) List(_ context.Context, limit int) ([]Lead, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.items) {
		limit = len(s.items)
	}
	out := make([]Lead, 0, limit)
	// Return newest first.
	for i := len(s.items) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, s.items[i])
	}
	return out, nil
}

// PostgresStore writes to the `enterprise_leads` table.
type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// BootstrapPostgres creates the table when missing. Safe to call on every
// orchestrator boot.
func BootstrapPostgres(ctx context.Context, pool *pgxpool.Pool) error {
	const ddl = `
CREATE TABLE IF NOT EXISTS enterprise_leads (
  id          text PRIMARY KEY,
  name        text NOT NULL DEFAULT '',
  email       text NOT NULL,
  company     text NOT NULL,
  team_size   text NOT NULL DEFAULT '',
  use_case    text NOT NULL DEFAULT '',
  budget      text NOT NULL DEFAULT '',
  timeline    text NOT NULL DEFAULT '',
  source      text NOT NULL DEFAULT '',
  user_agent  text NOT NULL DEFAULT '',
  ip          text NOT NULL DEFAULT '',
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS enterprise_leads_created_at_idx ON enterprise_leads (created_at DESC);
CREATE INDEX IF NOT EXISTS enterprise_leads_email_idx ON enterprise_leads (lower(email));
`
	_, err := pool.Exec(ctx, ddl)
	return err
}

func (s *PostgresStore) Create(ctx context.Context, l Lead) (Lead, error) {
	if l.ID == "" {
		l.ID = "lead_" + uuid.NewString()
	}
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now().UTC()
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO enterprise_leads
  (id, name, email, company, team_size, use_case, budget, timeline, source, user_agent, ip, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
`,
		l.ID, l.Name, l.Email, l.Company, l.TeamSize, l.UseCase,
		l.Budget, l.Timeline, l.Source, l.UserAgent, l.IP, l.CreatedAt,
	)
	if err != nil {
		return Lead{}, err
	}
	return l, nil
}

func (s *PostgresStore) List(ctx context.Context, limit int) ([]Lead, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
SELECT id, name, email, company, team_size, use_case, budget, timeline,
       source, user_agent, ip, created_at
FROM enterprise_leads
ORDER BY created_at DESC
LIMIT $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Lead
	for rows.Next() {
		var l Lead
		if err := rows.Scan(&l.ID, &l.Name, &l.Email, &l.Company, &l.TeamSize,
			&l.UseCase, &l.Budget, &l.Timeline, &l.Source, &l.UserAgent, &l.IP,
			&l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

var _ Store = (*MemoryStore)(nil)
var _ Store = (*PostgresStore)(nil)
