package guild

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// PostgresService is the production-grade backend. SQL strings are
// inline-literal (mirrors wallet/ledger/blueprints) so the schema
// shape stays visible at the call site. Migrations live in
// migrations/000XX_guild.sql — the table set is finisher_profiles,
// guild_tasks, guild_bids, templates, template_installs,
// guild_payouts, and guild_operations (idempotency).
type PostgresService struct {
	pool *pgxpool.Pool
}

// NewPostgresService wires the service to an existing pgxpool.
func NewPostgresService(pool *pgxpool.Pool) *PostgresService {
	return &PostgresService{pool: pool}
}

// --- finisher profiles -------------------------------------------------

// UpsertFinisherProfile inserts or updates by user_id. ON CONFLICT on
// the UNIQUE(user_id) index closes the race between two concurrent
// upserts from the same user.
func (s *PostgresService) UpsertFinisherProfile(ctx context.Context, p FinisherProfile) (FinisherProfile, error) {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	row := s.pool.QueryRow(ctx, `
        INSERT INTO finisher_profiles
            (id, user_id, display_name, skills, hourly_rate_usd, rating, verified, created_at)
        VALUES ($1, $2, $3, $4, $5::numeric, $6::numeric, $7, $8)
        ON CONFLICT (user_id) DO UPDATE
        SET display_name    = EXCLUDED.display_name,
            skills          = EXCLUDED.skills,
            hourly_rate_usd = EXCLUDED.hourly_rate_usd
        RETURNING id, user_id, display_name, skills, hourly_rate_usd::text,
                  completed_task_count, rating::text, verified, created_at`,
		p.ID, p.UserID, p.DisplayName, p.Skills, p.HourlyRateUSD.String(),
		p.Rating.String(), p.Verified, p.CreatedAt)
	return scanProfile(row)
}

// GetFinisherProfile by id.
func (s *PostgresService) GetFinisherProfile(ctx context.Context, id string) (FinisherProfile, error) {
	row := s.pool.QueryRow(ctx, `
        SELECT id, user_id, display_name, skills, hourly_rate_usd::text,
               completed_task_count, rating::text, verified, created_at
        FROM finisher_profiles WHERE id = $1`, id)
	return scanProfile(row)
}

// GetFinisherProfileByUser by user_id.
func (s *PostgresService) GetFinisherProfileByUser(ctx context.Context, userID string) (FinisherProfile, error) {
	row := s.pool.QueryRow(ctx, `
        SELECT id, user_id, display_name, skills, hourly_rate_usd::text,
               completed_task_count, rating::text, verified, created_at
        FROM finisher_profiles WHERE user_id = $1`, userID)
	return scanProfile(row)
}

// --- tasks -------------------------------------------------------------

// CreateTask inserts a new task in 'open' status.
func (s *PostgresService) CreateTask(ctx context.Context, t GuildTask) (GuildTask, error) {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}
	if t.Status == "" {
		t.Status = TaskStatusOpen
	}
	row := s.pool.QueryRow(ctx, `
        INSERT INTO guild_tasks
            (id, project_id, tenant_id, gate_failure_id, title, description,
             price_usd_floor, sla_hours, status, created_at)
        VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7::numeric, $8, $9, $10)
        RETURNING id, project_id, tenant_id, COALESCE(gate_failure_id, ''),
                  title, description, price_usd_floor::text, sla_hours,
                  status, assigned_to, created_at, accepted_at`,
		t.ID, t.ProjectID, t.TenantID, t.GateFailureID, t.Title, t.Description,
		t.PriceUSDFloor.String(), t.SLAHours, t.Status, t.CreatedAt)
	return scanTask(row)
}

// GetTask by id.
func (s *PostgresService) GetTask(ctx context.Context, id string) (GuildTask, error) {
	row := s.pool.QueryRow(ctx, `
        SELECT id, project_id, tenant_id, COALESCE(gate_failure_id, ''),
               title, description, price_usd_floor::text, sla_hours,
               status, assigned_to, created_at, accepted_at
        FROM guild_tasks WHERE id = $1`, id)
	return scanTask(row)
}

// ListTasks with optional status / tenant filters.
func (s *PostgresService) ListTasks(ctx context.Context, filter TaskFilter) ([]GuildTask, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
        SELECT id, project_id, tenant_id, COALESCE(gate_failure_id, ''),
               title, description, price_usd_floor::text, sla_hours,
               status, assigned_to, created_at, accepted_at
        FROM guild_tasks
        WHERE ($1 = '' OR status    = $1)
          AND ($2 = '' OR tenant_id = $2)
        ORDER BY created_at DESC
        LIMIT $3`, filter.Status, filter.TenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("guild: list tasks: %w", err)
	}
	defer rows.Close()
	out := []GuildTask{}
	for rows.Next() {
		t, err := scanTaskRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// UpdateTaskStatus.
func (s *PostgresService) UpdateTaskStatus(ctx context.Context, taskID, status string, assignedTo *string) (GuildTask, error) {
	row := s.pool.QueryRow(ctx, `
        UPDATE guild_tasks
           SET status      = $2,
               assigned_to = COALESCE($3, assigned_to),
               accepted_at = CASE WHEN $2 = 'accepted' AND accepted_at IS NULL
                                  THEN now() ELSE accepted_at END
         WHERE id = $1
         RETURNING id, project_id, tenant_id, COALESCE(gate_failure_id, ''),
                   title, description, price_usd_floor::text, sla_hours,
                   status, assigned_to, created_at, accepted_at`,
		taskID, status, assignedTo)
	return scanTask(row)
}

// --- bids --------------------------------------------------------------

// PlaceBid inserts. The CHECK constraint in 000XX_guild.sql enforces
// price_usd > 0 — invalid bids surface as a Postgres error.
func (s *PostgresService) PlaceBid(ctx context.Context, b Bid) (Bid, error) {
	if b.ID == "" {
		b.ID = uuid.NewString()
	}
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now().UTC()
	}
	if b.Status == "" {
		b.Status = BidStatusOpen
	}
	// Lock the task row so the floor / status check is consistent with
	// the bid insert under concurrent PlaceBid calls.
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Bid{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status string
	var floor string
	if err := tx.QueryRow(ctx, `
        SELECT status, price_usd_floor::text
        FROM guild_tasks WHERE id = $1 FOR UPDATE`, b.TaskID).Scan(&status, &floor); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Bid{}, ErrNotFound
		}
		return Bid{}, err
	}
	if status != TaskStatusOpen && status != TaskStatusBidding {
		return Bid{}, ErrTaskClosed
	}
	floorDec, _ := decimal.NewFromString(floor)
	if b.PriceUSD.GreaterThan(floorDec) {
		return Bid{}, ErrBidTooHigh
	}
	row := tx.QueryRow(ctx, `
        INSERT INTO guild_bids
            (id, task_id, finisher_id, price_usd, estimated_hours, note, status, created_at)
        VALUES ($1, $2, $3, $4::numeric, $5, $6, $7, $8)
        RETURNING id, task_id, finisher_id, price_usd::text, estimated_hours, note, status, created_at`,
		b.ID, b.TaskID, b.FinisherID, b.PriceUSD.String(), b.EstimatedHours,
		b.Note, b.Status, b.CreatedAt)
	out, err := scanBid(row)
	if err != nil {
		return Bid{}, err
	}
	// Flip task to bidding on first bid.
	if _, err := tx.Exec(ctx, `
        UPDATE guild_tasks SET status = 'bidding'
         WHERE id = $1 AND status = 'open'`, b.TaskID); err != nil {
		return Bid{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Bid{}, err
	}
	return out, nil
}

// ListBids for a task.
func (s *PostgresService) ListBids(ctx context.Context, taskID string) ([]Bid, error) {
	rows, err := s.pool.Query(ctx, `
        SELECT id, task_id, finisher_id, price_usd::text, estimated_hours, note, status, created_at
        FROM guild_bids WHERE task_id = $1 ORDER BY created_at DESC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("guild: list bids: %w", err)
	}
	defer rows.Close()
	out := []Bid{}
	for rows.Next() {
		b, err := scanBidRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// GetBid by id.
func (s *PostgresService) GetBid(ctx context.Context, id string) (Bid, error) {
	row := s.pool.QueryRow(ctx, `
        SELECT id, task_id, finisher_id, price_usd::text, estimated_hours, note, status, created_at
        FROM guild_bids WHERE id = $1`, id)
	return scanBid(row)
}

// UpdateBidStatus.
func (s *PostgresService) UpdateBidStatus(ctx context.Context, bidID, status string) (Bid, error) {
	row := s.pool.QueryRow(ctx, `
        UPDATE guild_bids SET status = $2 WHERE id = $1
        RETURNING id, task_id, finisher_id, price_usd::text, estimated_hours, note, status, created_at`,
		bidID, status)
	return scanBid(row)
}

// CountBidsForTask.
func (s *PostgresService) CountBidsForTask(ctx context.Context, taskID string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM guild_bids WHERE task_id = $1`, taskID).Scan(&n)
	return n, err
}

// --- templates ---------------------------------------------------------

// UpsertTemplate inserts or updates by slug. ON CONFLICT on
// UNIQUE(slug) closes the race.
func (s *PostgresService) UpsertTemplate(ctx context.Context, t Template) (Template, error) {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}
	row := s.pool.QueryRow(ctx, `
        INSERT INTO templates
            (id, author_user_id, slug, name, description, price_usd, gates_passed, verified, created_at)
        VALUES ($1, $2, $3, $4, $5, $6::numeric, $7, $8, $9)
        ON CONFLICT (slug) DO UPDATE
        SET name         = EXCLUDED.name,
            description  = EXCLUDED.description,
            price_usd    = EXCLUDED.price_usd,
            gates_passed = EXCLUDED.gates_passed
        WHERE templates.author_user_id = EXCLUDED.author_user_id
        RETURNING id, author_user_id, slug, name, description,
                  price_usd::text, gates_passed, install_count, verified, created_at`,
		t.ID, t.AuthorUserID, t.Slug, t.Name, t.Description,
		t.PriceUSD.String(), t.GatesPassed, t.Verified, t.CreatedAt)
	return scanTemplate(row)
}

// GetTemplateBySlug.
func (s *PostgresService) GetTemplateBySlug(ctx context.Context, slug string) (Template, error) {
	row := s.pool.QueryRow(ctx, `
        SELECT id, author_user_id, slug, name, description,
               price_usd::text, gates_passed, install_count, verified, created_at
        FROM templates WHERE slug = $1`, slug)
	return scanTemplate(row)
}

// ListTemplates.
func (s *PostgresService) ListTemplates(ctx context.Context, verifiedOnly bool) ([]Template, error) {
	rows, err := s.pool.Query(ctx, `
        SELECT id, author_user_id, slug, name, description,
               price_usd::text, gates_passed, install_count, verified, created_at
        FROM templates
        WHERE NOT $1 OR verified = TRUE
        ORDER BY created_at DESC`, verifiedOnly)
	if err != nil {
		return nil, fmt.Errorf("guild: list templates: %w", err)
	}
	defer rows.Close()
	out := []Template{}
	for rows.Next() {
		t, err := scanTemplateRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// IncrementTemplateInstallCount.
func (s *PostgresService) IncrementTemplateInstallCount(ctx context.Context, templateID string) error {
	_, err := s.pool.Exec(ctx, `
        UPDATE templates SET install_count = install_count + 1 WHERE id = $1`, templateID)
	return err
}

// --- installs / payouts ------------------------------------------------

// RecordInstall.
func (s *PostgresService) RecordInstall(ctx context.Context, i Install) (Install, error) {
	if i.ID == "" {
		i.ID = uuid.NewString()
	}
	if i.InstalledAt.IsZero() {
		i.InstalledAt = time.Now().UTC()
	}
	_, err := s.pool.Exec(ctx, `
        INSERT INTO template_installs
            (id, template_id, project_id, tenant_id, amount_usd,
             author_payout_usd, platform_cut_usd, installed_at)
        VALUES ($1, $2, $3, $4, $5::numeric, $6::numeric, $7::numeric, $8)`,
		i.ID, i.TemplateID, i.ProjectID, i.TenantID,
		i.AmountUSD.String(), i.AuthorPayoutUSD.String(), i.PlatformCutUSD.String(),
		i.InstalledAt)
	if err != nil {
		return Install{}, err
	}
	return i, nil
}

// RecordPayout.
func (s *PostgresService) RecordPayout(ctx context.Context, p Payout) (Payout, error) {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	if p.Status == "" {
		p.Status = "pending"
	}
	_, err := s.pool.Exec(ctx, `
        INSERT INTO guild_payouts
            (id, task_id, finisher_id, amount_usd, finisher_cut_usd, platform_cut_usd, status, created_at)
        VALUES ($1, $2, $3, $4::numeric, $5::numeric, $6::numeric, $7, $8)`,
		p.ID, p.TaskID, p.FinisherID,
		p.AmountUSD.String(), p.FinisherCutUSD.String(), p.PlatformCutUSD.String(),
		p.Status, p.CreatedAt)
	if err != nil {
		return Payout{}, err
	}
	return p, nil
}

// --- idempotency -------------------------------------------------------

// RecallOp returns a prior outcome by op_key or (zero, false) when no
// row is present.
func (s *PostgresService) RecallOp(ctx context.Context, opKey string) (OpOutcome, bool, error) {
	if opKey == "" {
		return OpOutcome{}, false, nil
	}
	var status, code string
	err := s.pool.QueryRow(ctx,
		`SELECT status, COALESCE(error_code, '') FROM guild_operations WHERE op_key = $1`,
		opKey).Scan(&status, &code)
	if errors.Is(err, pgx.ErrNoRows) {
		return OpOutcome{}, false, nil
	}
	if err != nil {
		return OpOutcome{}, false, err
	}
	return OpOutcome{Status: status, ErrorCode: code}, true, nil
}

// RecordOp.
func (s *PostgresService) RecordOp(ctx context.Context, opKey, opType string, amount decimal.Decimal, status, errorCode string) error {
	if opKey == "" {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
        INSERT INTO guild_operations(op_key, op_type, amount_usd, status, error_code)
        VALUES ($1, $2, $3::numeric, $4, NULLIF($5, ''))
        ON CONFLICT (op_key) DO NOTHING`,
		opKey, opType, amount.String(), status, errorCode)
	return err
}

// --- reconciliation ---------------------------------------------------

// ListStaleOpenBids.
func (s *PostgresService) ListStaleOpenBids(ctx context.Context, olderThanSec int) ([]Bid, error) {
	cutoff := time.Now().UTC().Add(-time.Duration(olderThanSec) * time.Second)
	rows, err := s.pool.Query(ctx, `
        SELECT id, task_id, finisher_id, price_usd::text, estimated_hours, note, status, created_at
        FROM guild_bids
        WHERE status = 'open' AND created_at < $1
        ORDER BY created_at ASC
        LIMIT 1000`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Bid{}
	for rows.Next() {
		b, err := scanBidRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ListAbandonedTasks returns tasks whose deadline (created_at +
// sla_hours) is in the past and that are still open / bidding.
func (s *PostgresService) ListAbandonedTasks(ctx context.Context, _ int) ([]GuildTask, error) {
	rows, err := s.pool.Query(ctx, `
        SELECT id, project_id, tenant_id, COALESCE(gate_failure_id, ''),
               title, description, price_usd_floor::text, sla_hours,
               status, assigned_to, created_at, accepted_at
        FROM guild_tasks
        WHERE status IN ('open','bidding')
          AND sla_hours > 0
          AND created_at + (sla_hours * INTERVAL '1 hour') < now()
        LIMIT 1000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GuildTask{}
	for rows.Next() {
		t, err := scanTaskRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// --- scanners ----------------------------------------------------------

func scanProfile(row pgx.Row) (FinisherProfile, error) {
	var p FinisherProfile
	var rate, rating string
	if err := row.Scan(&p.ID, &p.UserID, &p.DisplayName, &p.Skills,
		&rate, &p.CompletedTaskCount, &rating, &p.Verified, &p.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FinisherProfile{}, ErrNotFound
		}
		return FinisherProfile{}, err
	}
	p.HourlyRateUSD, _ = decimal.NewFromString(rate)
	p.Rating, _ = decimal.NewFromString(rating)
	return p, nil
}

func scanTask(row pgx.Row) (GuildTask, error) {
	var t GuildTask
	var floor string
	var assignedTo *string
	var acceptedAt *time.Time
	if err := row.Scan(&t.ID, &t.ProjectID, &t.TenantID, &t.GateFailureID,
		&t.Title, &t.Description, &floor, &t.SLAHours,
		&t.Status, &assignedTo, &t.CreatedAt, &acceptedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return GuildTask{}, ErrNotFound
		}
		return GuildTask{}, err
	}
	t.PriceUSDFloor, _ = decimal.NewFromString(floor)
	t.AssignedTo = assignedTo
	t.AcceptedAt = acceptedAt
	return t, nil
}

// scanTaskRow is the rows.Scan variant — pgx.Rows does not implement
// pgx.Row directly because Next() owns the cursor; both signatures
// share the same scan body.
func scanTaskRow(rows pgx.Rows) (GuildTask, error) {
	var t GuildTask
	var floor string
	var assignedTo *string
	var acceptedAt *time.Time
	if err := rows.Scan(&t.ID, &t.ProjectID, &t.TenantID, &t.GateFailureID,
		&t.Title, &t.Description, &floor, &t.SLAHours,
		&t.Status, &assignedTo, &t.CreatedAt, &acceptedAt); err != nil {
		return GuildTask{}, err
	}
	t.PriceUSDFloor, _ = decimal.NewFromString(floor)
	t.AssignedTo = assignedTo
	t.AcceptedAt = acceptedAt
	return t, nil
}

func scanBid(row pgx.Row) (Bid, error) {
	var b Bid
	var price string
	if err := row.Scan(&b.ID, &b.TaskID, &b.FinisherID, &price,
		&b.EstimatedHours, &b.Note, &b.Status, &b.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Bid{}, ErrNotFound
		}
		return Bid{}, err
	}
	b.PriceUSD, _ = decimal.NewFromString(price)
	return b, nil
}

func scanBidRow(rows pgx.Rows) (Bid, error) {
	var b Bid
	var price string
	if err := rows.Scan(&b.ID, &b.TaskID, &b.FinisherID, &price,
		&b.EstimatedHours, &b.Note, &b.Status, &b.CreatedAt); err != nil {
		return Bid{}, err
	}
	b.PriceUSD, _ = decimal.NewFromString(price)
	return b, nil
}

func scanTemplate(row pgx.Row) (Template, error) {
	var t Template
	var price string
	if err := row.Scan(&t.ID, &t.AuthorUserID, &t.Slug, &t.Name, &t.Description,
		&price, &t.GatesPassed, &t.InstallCount, &t.Verified, &t.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Template{}, ErrNotFound
		}
		return Template{}, err
	}
	t.PriceUSD, _ = decimal.NewFromString(price)
	return t, nil
}

func scanTemplateRow(rows pgx.Rows) (Template, error) {
	var t Template
	var price string
	if err := rows.Scan(&t.ID, &t.AuthorUserID, &t.Slug, &t.Name, &t.Description,
		&price, &t.GatesPassed, &t.InstallCount, &t.Verified, &t.CreatedAt); err != nil {
		return Template{}, err
	}
	t.PriceUSD, _ = decimal.NewFromString(price)
	return t, nil
}
