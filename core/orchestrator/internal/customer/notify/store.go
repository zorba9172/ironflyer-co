package notify

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a notification or outbox row does not
// exist or is not owned by the requested user.
var ErrNotFound = errors.New("notify: not found")

// NotificationStore persists durable in-app notification rows for the
// bell + notifications query/subscription. Every read is scoped by
// userID — multi-tenancy invariant.
type NotificationStore interface {
	Create(ctx context.Context, n Notification) error
	List(ctx context.Context, userID string, unreadOnly bool, limit int) ([]Notification, error)
	UnreadCount(ctx context.Context, userID string) (int, error)
	MarkRead(ctx context.Context, userID, id string) (Notification, error)
	MarkAllRead(ctx context.Context, userID string) (int, error)
}

// OutboxStore persists pending notification deliveries. The Worker
// drains it; the Dispatcher fills it.
type OutboxStore interface {
	Enqueue(ctx context.Context, item OutboxItem, idempotencyKey string) (string, error)
	Claim(ctx context.Context, batch int) ([]OutboxItem, error)
	MarkEmailSent(ctx context.Context, id string) error
	MarkInAppSent(ctx context.Context, id string) error
	MarkDelivered(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id string, reason string, backoff time.Duration) error
	MarkDeadLettered(ctx context.Context, id string, reason string) error
}

// MemoryNotificationStore is a process-local store used in dev and when
// no Postgres pool is wired. Concurrent-safe.
type MemoryNotificationStore struct {
	mu   sync.Mutex
	rows []Notification
}

// NewMemoryNotificationStore returns an empty in-memory store.
func NewMemoryNotificationStore() *MemoryNotificationStore {
	return &MemoryNotificationStore{}
}

// Create inserts a new notification row.
func (m *MemoryNotificationStore) Create(_ context.Context, n Notification) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n.ID == "" {
		n.ID = uuid.NewString()
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}
	m.rows = append(m.rows, n)
	return nil
}

// List returns the user's notifications newest-first, optionally
// filtered to unread.
func (m *MemoryNotificationStore) List(_ context.Context, userID string, unreadOnly bool, limit int) ([]Notification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Notification, 0, len(m.rows))
	for _, n := range m.rows {
		if n.UserID != userID {
			continue
		}
		if unreadOnly && n.ReadAt != nil {
			continue
		}
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// UnreadCount returns the number of unread rows for the user.
func (m *MemoryNotificationStore) UnreadCount(_ context.Context, userID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, r := range m.rows {
		if r.UserID == userID && r.ReadAt == nil {
			n++
		}
	}
	return n, nil
}

// MarkRead flips a single notification to read and returns the updated
// row. Returns ErrNotFound when the row does not exist or is not owned
// by the user.
func (m *MemoryNotificationStore) MarkRead(_ context.Context, userID, id string) (Notification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.rows {
		if m.rows[i].ID != id || m.rows[i].UserID != userID {
			continue
		}
		if m.rows[i].ReadAt == nil {
			t := time.Now().UTC()
			m.rows[i].ReadAt = &t
		}
		return m.rows[i], nil
	}
	return Notification{}, ErrNotFound
}

// MarkAllRead flips every unread row for the user to read and returns
// the count of rows that transitioned.
func (m *MemoryNotificationStore) MarkAllRead(_ context.Context, userID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	now := time.Now().UTC()
	for i := range m.rows {
		if m.rows[i].UserID != userID || m.rows[i].ReadAt != nil {
			continue
		}
		t := now
		m.rows[i].ReadAt = &t
		n++
	}
	return n, nil
}

// MemoryOutboxStore is the in-memory OutboxStore used in dev. Claim
// returns rows whose NextAttemptAt is due; the Worker drives state.
type MemoryOutboxStore struct {
	mu   sync.Mutex
	rows map[string]*OutboxItem
	idem map[string]string // idempotency key → outbox id
}

// NewMemoryOutboxStore returns an empty in-memory outbox.
func NewMemoryOutboxStore() *MemoryOutboxStore {
	return &MemoryOutboxStore{
		rows: make(map[string]*OutboxItem),
		idem: make(map[string]string),
	}
}

// Enqueue persists an OutboxItem and binds the idempotency key. A key
// collision returns the pre-existing outbox id without inserting a new
// row.
func (m *MemoryOutboxStore) Enqueue(_ context.Context, item OutboxItem, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.idem[key]; ok {
		return existing, nil
	}
	if item.ID == "" {
		item.ID = uuid.NewString()
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	}
	if item.NextAttemptAt.IsZero() {
		item.NextAttemptAt = item.CreatedAt
	}
	cp := item
	m.rows[item.ID] = &cp
	m.idem[key] = item.ID
	return item.ID, nil
}

// Claim returns up to batch due items in due-order. Memory mode is
// single-process so we don't need a lock token — the caller owns the
// claimed rows until MarkDelivered / MarkFailed / MarkDeadLettered.
func (m *MemoryOutboxStore) Claim(_ context.Context, batch int) ([]OutboxItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	candidates := make([]*OutboxItem, 0, len(m.rows))
	for _, r := range m.rows {
		if r.DeliveredAt != nil || r.DeadLetteredAt != nil {
			continue
		}
		if r.NextAttemptAt.After(now) {
			continue
		}
		candidates = append(candidates, r)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].NextAttemptAt.Before(candidates[j].NextAttemptAt) })
	if batch > 0 && len(candidates) > batch {
		candidates = candidates[:batch]
	}
	out := make([]OutboxItem, 0, len(candidates))
	for _, r := range candidates {
		// Park the next attempt so the next Claim doesn't re-pick the
		// same row while the worker is still delivering.
		r.NextAttemptAt = now.Add(60 * time.Second)
		out = append(out, *r)
	}
	return out, nil
}

// MarkEmailSent stamps email_sent_at on the row.
func (m *MemoryOutboxStore) MarkEmailSent(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rows[id]
	if !ok {
		return ErrNotFound
	}
	t := time.Now().UTC()
	r.EmailSentAt = &t
	return nil
}

// MarkInAppSent stamps inapp_sent_at on the row.
func (m *MemoryOutboxStore) MarkInAppSent(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rows[id]
	if !ok {
		return ErrNotFound
	}
	t := time.Now().UTC()
	r.InAppSentAt = &t
	return nil
}

// MarkDelivered stamps delivered_at on the row.
func (m *MemoryOutboxStore) MarkDelivered(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rows[id]
	if !ok {
		return ErrNotFound
	}
	t := time.Now().UTC()
	r.DeliveredAt = &t
	return nil
}

// MarkFailed bumps the attempts counter and schedules the next retry.
func (m *MemoryOutboxStore) MarkFailed(_ context.Context, id string, reason string, backoff time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rows[id]
	if !ok {
		return ErrNotFound
	}
	r.Attempts++
	r.LastError = reason
	r.NextAttemptAt = time.Now().UTC().Add(backoff)
	return nil
}

// MarkDeadLettered stamps dead_lettered_at and the final reason.
func (m *MemoryOutboxStore) MarkDeadLettered(_ context.Context, id string, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rows[id]
	if !ok {
		return ErrNotFound
	}
	t := time.Now().UTC()
	r.DeadLetteredAt = &t
	r.LastError = reason
	return nil
}

// PostgresNotificationStore persists rows in the notifications table.
type PostgresNotificationStore struct {
	pool *pgxpool.Pool
}

// NewPostgresNotificationStore constructs a store bound to pool.
func NewPostgresNotificationStore(pool *pgxpool.Pool) *PostgresNotificationStore {
	return &PostgresNotificationStore{pool: pool}
}

// Create inserts a new notification row.
func (p *PostgresNotificationStore) Create(ctx context.Context, n Notification) error {
	if n.ID == "" {
		n.ID = uuid.NewString()
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}
	_, err := p.pool.Exec(ctx, `
		INSERT INTO notifications (id, user_id, kind, title, body, link, severity, read_at, created_at)
		VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8,$9)
	`, n.ID, n.UserID, n.Kind, n.Title, n.Body, n.Link, n.Severity, n.ReadAt, n.CreatedAt)
	return err
}

// List returns the user's notifications newest-first.
func (p *PostgresNotificationStore) List(ctx context.Context, userID string, unreadOnly bool, limit int) ([]Notification, error) {
	if limit <= 0 {
		limit = 100
	}
	q := `SELECT id, user_id, kind, title, body, COALESCE(link,''), severity, read_at, created_at
	      FROM notifications WHERE user_id = $1`
	if unreadOnly {
		q += ` AND read_at IS NULL`
	}
	q += ` ORDER BY created_at DESC LIMIT $2`
	rows, err := p.pool.Query(ctx, q, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Notification, 0)
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Kind, &n.Title, &n.Body, &n.Link, &n.Severity, &n.ReadAt, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// UnreadCount returns the count of unread rows.
func (p *PostgresNotificationStore) UnreadCount(ctx context.Context, userID string) (int, error) {
	var n int
	err := p.pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE user_id=$1 AND read_at IS NULL`, userID).Scan(&n)
	return n, err
}

// MarkRead flips a single row and returns the updated record.
func (p *PostgresNotificationStore) MarkRead(ctx context.Context, userID, id string) (Notification, error) {
	row := p.pool.QueryRow(ctx, `
		UPDATE notifications SET read_at = COALESCE(read_at, now())
		WHERE id = $1 AND user_id = $2
		RETURNING id, user_id, kind, title, body, COALESCE(link,''), severity, read_at, created_at
	`, id, userID)
	var n Notification
	if err := row.Scan(&n.ID, &n.UserID, &n.Kind, &n.Title, &n.Body, &n.Link, &n.Severity, &n.ReadAt, &n.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Notification{}, ErrNotFound
		}
		return Notification{}, err
	}
	return n, nil
}

// MarkAllRead flips every unread row for the user.
func (p *PostgresNotificationStore) MarkAllRead(ctx context.Context, userID string) (int, error) {
	tag, err := p.pool.Exec(ctx, `UPDATE notifications SET read_at = now() WHERE user_id = $1 AND read_at IS NULL`, userID)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// PostgresOutboxStore persists rows in notification_outbox +
// notification_idempotency.
type PostgresOutboxStore struct {
	pool *pgxpool.Pool
}

// NewPostgresOutboxStore constructs a store bound to pool.
func NewPostgresOutboxStore(pool *pgxpool.Pool) *PostgresOutboxStore {
	return &PostgresOutboxStore{pool: pool}
}

// Enqueue performs an idempotent insert: a key collision returns the
// pre-existing outbox id and skips the insert. Wrapped in a tx so the
// outbox row + idempotency row land atomically.
func (p *PostgresOutboxStore) Enqueue(ctx context.Context, item OutboxItem, key string) (string, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var existing string
	err = tx.QueryRow(ctx, `SELECT outbox_id FROM notification_idempotency WHERE key=$1`, key).Scan(&existing)
	if err == nil && existing != "" {
		return existing, tx.Commit(ctx)
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}

	if item.ID == "" {
		item.ID = uuid.NewString()
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	}
	if item.NextAttemptAt.IsZero() {
		item.NextAttemptAt = item.CreatedAt
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO notification_outbox
		  (id, user_id, kind, payload, email_target, inapp_target, attempts, next_attempt_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,0,$7,$8)
	`, item.ID, item.UserID, string(item.Kind), item.Payload, item.EmailTarget, item.InAppTarget, item.NextAttemptAt, item.CreatedAt); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO notification_idempotency (key, outbox_id) VALUES ($1, $2)
		ON CONFLICT (key) DO NOTHING
	`, key, item.ID); err != nil {
		return "", err
	}
	return item.ID, tx.Commit(ctx)
}

// Claim picks up to batch due rows using SELECT ... FOR UPDATE SKIP
// LOCKED so multiple workers can run safely. The transaction is held
// open through the UPDATE so claimed rows are stamped with a new
// next_attempt_at before release.
func (p *PostgresOutboxStore) Claim(ctx context.Context, batch int) ([]OutboxItem, error) {
	if batch <= 0 {
		batch = 16
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		SELECT id, user_id, kind, payload, email_target, inapp_target, attempts,
		       next_attempt_at, COALESCE(last_error,''),
		       delivered_at, dead_lettered_at, email_sent_at, inapp_sent_at, created_at
		FROM notification_outbox
		WHERE delivered_at IS NULL
		  AND dead_lettered_at IS NULL
		  AND next_attempt_at <= now()
		ORDER BY next_attempt_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`, batch)
	if err != nil {
		return nil, err
	}
	out := make([]OutboxItem, 0, batch)
	ids := make([]string, 0, batch)
	for rows.Next() {
		var it OutboxItem
		var kind string
		if err := rows.Scan(&it.ID, &it.UserID, &kind, &it.Payload, &it.EmailTarget, &it.InAppTarget, &it.Attempts,
			&it.NextAttemptAt, &it.LastError, &it.DeliveredAt, &it.DeadLetteredAt, &it.EmailSentAt, &it.InAppSentAt, &it.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		it.Kind = Kind(kind)
		out = append(out, it)
		ids = append(ids, it.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return out, tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE notification_outbox
		SET next_attempt_at = now() + interval '60 seconds'
		WHERE id = ANY($1)
	`, ids); err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}

// MarkEmailSent stamps email_sent_at.
func (p *PostgresOutboxStore) MarkEmailSent(ctx context.Context, id string) error {
	_, err := p.pool.Exec(ctx, `UPDATE notification_outbox SET email_sent_at = now() WHERE id = $1`, id)
	return err
}

// MarkInAppSent stamps inapp_sent_at.
func (p *PostgresOutboxStore) MarkInAppSent(ctx context.Context, id string) error {
	_, err := p.pool.Exec(ctx, `UPDATE notification_outbox SET inapp_sent_at = now() WHERE id = $1`, id)
	return err
}

// MarkDelivered stamps delivered_at.
func (p *PostgresOutboxStore) MarkDelivered(ctx context.Context, id string) error {
	_, err := p.pool.Exec(ctx, `UPDATE notification_outbox SET delivered_at = now() WHERE id = $1`, id)
	return err
}

// MarkFailed bumps attempts + schedules a retry.
func (p *PostgresOutboxStore) MarkFailed(ctx context.Context, id string, reason string, backoff time.Duration) error {
	_, err := p.pool.Exec(ctx, `
		UPDATE notification_outbox
		SET attempts = attempts + 1,
		    last_error = $2,
		    next_attempt_at = now() + ($3 || ' milliseconds')::interval
		WHERE id = $1
	`, id, reason, backoff.Milliseconds())
	return err
}

// MarkDeadLettered stamps dead_lettered_at and the final reason.
func (p *PostgresOutboxStore) MarkDeadLettered(ctx context.Context, id string, reason string) error {
	_, err := p.pool.Exec(ctx, `
		UPDATE notification_outbox
		SET dead_lettered_at = now(), last_error = $2
		WHERE id = $1
	`, id, reason)
	return err
}
