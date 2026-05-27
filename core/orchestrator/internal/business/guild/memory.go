package guild

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// MemoryService is the in-process Service used by dev boots and as a
// clean substrate for resolver wiring before Postgres is provisioned.
// A single sync.Mutex guards every map — guild traffic is per-tenant
// and low-volume relative to wallet/ledger, so the contention surface
// is negligible and per-shard locks would only add bug area.
type MemoryService struct {
	mu        sync.Mutex
	profiles  map[string]FinisherProfile // id -> profile
	byUser    map[string]string          // userID -> profile id
	tasks     map[string]GuildTask
	bidsByID  map[string]Bid
	bidsByTsk map[string][]string // taskID -> []bidID, insertion order
	templates map[string]Template // id -> template
	bySlug    map[string]string   // slug -> template id
	installs  map[string]Install
	payouts   map[string]Payout
	opKeys    map[string]OpOutcome
}

// NewMemoryService returns a zeroed in-process store.
func NewMemoryService() *MemoryService {
	return &MemoryService{
		profiles:  map[string]FinisherProfile{},
		byUser:    map[string]string{},
		tasks:     map[string]GuildTask{},
		bidsByID:  map[string]Bid{},
		bidsByTsk: map[string][]string{},
		templates: map[string]Template{},
		bySlug:    map[string]string{},
		installs:  map[string]Install{},
		payouts:   map[string]Payout{},
		opKeys:    map[string]OpOutcome{},
	}
}

// --- finisher profiles -------------------------------------------------

// UpsertFinisherProfile creates or updates the caller's profile. The
// (UserID -> id) index guarantees one profile per user; subsequent calls
// from the same user mutate the existing row.
func (s *MemoryService) UpsertFinisherProfile(_ context.Context, p FinisherProfile) (FinisherProfile, error) {
	if strings.TrimSpace(p.UserID) == "" || strings.TrimSpace(p.DisplayName) == "" {
		return FinisherProfile{}, ErrInvalidAmount
	}
	if p.HourlyRateUSD.IsNegative() {
		return FinisherProfile{}, ErrInvalidAmount
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existingID, ok := s.byUser[p.UserID]; ok {
		ex := s.profiles[existingID]
		ex.DisplayName = p.DisplayName
		ex.Skills = append([]string(nil), p.Skills...)
		ex.HourlyRateUSD = p.HourlyRateUSD
		s.profiles[existingID] = ex
		return ex, nil
	}
	p.ID = uuid.NewString()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	if p.Rating.IsZero() {
		p.Rating = decimal.Zero
	}
	s.profiles[p.ID] = p
	s.byUser[p.UserID] = p.ID
	return p, nil
}

// GetFinisherProfile returns the profile by id or ErrNotFound.
func (s *MemoryService) GetFinisherProfile(_ context.Context, id string) (FinisherProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.profiles[id]
	if !ok {
		return FinisherProfile{}, ErrNotFound
	}
	return p, nil
}

// GetFinisherProfileByUser returns the profile owned by userID. The
// resolver uses this for myFinisherProfile.
func (s *MemoryService) GetFinisherProfileByUser(_ context.Context, userID string) (FinisherProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.byUser[userID]
	if !ok {
		return FinisherProfile{}, ErrNotFound
	}
	return s.profiles[id], nil
}

// --- tasks -------------------------------------------------------------

// CreateTask inserts a new task with a generated id + open status.
func (s *MemoryService) CreateTask(_ context.Context, t GuildTask) (GuildTask, error) {
	if t.PriceUSDFloor.IsNegative() || t.PriceUSDFloor.IsZero() {
		return GuildTask{}, ErrInvalidAmount
	}
	if strings.TrimSpace(t.ProjectID) == "" || strings.TrimSpace(t.TenantID) == "" {
		return GuildTask{}, ErrInvalidAmount
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}
	if t.Status == "" {
		t.Status = TaskStatusOpen
	}
	s.tasks[t.ID] = t
	return t, nil
}

// GetTask returns the task or ErrNotFound.
func (s *MemoryService) GetTask(_ context.Context, id string) (GuildTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return GuildTask{}, ErrNotFound
	}
	return t, nil
}

// ListTasks returns tasks newest-first, optionally filtered by status
// and tenant.
func (s *MemoryService) ListTasks(_ context.Context, filter TaskFilter) ([]GuildTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]GuildTask, 0, len(s.tasks))
	for _, t := range s.tasks {
		if filter.Status != "" && t.Status != filter.Status {
			continue
		}
		if filter.TenantID != "" && t.TenantID != filter.TenantID {
			continue
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out, nil
}

// UpdateTaskStatus mutates status (+ optional assigned finisher) and
// stamps AcceptedAt on the accepted transition.
func (s *MemoryService) UpdateTaskStatus(_ context.Context, taskID, status string, assignedTo *string) (GuildTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[taskID]
	if !ok {
		return GuildTask{}, ErrNotFound
	}
	t.Status = status
	if assignedTo != nil {
		t.AssignedTo = assignedTo
	}
	if status == TaskStatusAccepted && t.AcceptedAt == nil {
		now := time.Now().UTC()
		t.AcceptedAt = &now
	}
	s.tasks[taskID] = t
	return t, nil
}

// --- bids --------------------------------------------------------------

// PlaceBid inserts a new bid. PriceUSD must be <= task.PriceUSDFloor —
// the resolver runs that check before calling; we re-assert here so a
// direct service caller cannot bypass it.
func (s *MemoryService) PlaceBid(_ context.Context, b Bid) (Bid, error) {
	if b.PriceUSD.IsZero() || b.PriceUSD.IsNegative() {
		return Bid{}, ErrInvalidAmount
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[b.TaskID]
	if !ok {
		return Bid{}, ErrNotFound
	}
	if task.Status != TaskStatusOpen && task.Status != TaskStatusBidding {
		return Bid{}, ErrTaskClosed
	}
	if b.PriceUSD.GreaterThan(task.PriceUSDFloor) {
		return Bid{}, ErrBidTooHigh
	}
	if b.ID == "" {
		b.ID = uuid.NewString()
	}
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now().UTC()
	}
	if b.Status == "" {
		b.Status = BidStatusOpen
	}
	s.bidsByID[b.ID] = b
	s.bidsByTsk[b.TaskID] = append(s.bidsByTsk[b.TaskID], b.ID)
	// First bid flips the task to bidding so the UI can distinguish
	// "open + waiting" from "open + has offers".
	if task.Status == TaskStatusOpen {
		task.Status = TaskStatusBidding
		s.tasks[b.TaskID] = task
	}
	return b, nil
}

// ListBids returns bids for a task, newest-first.
func (s *MemoryService) ListBids(_ context.Context, taskID string) ([]Bid, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := s.bidsByTsk[taskID]
	out := make([]Bid, 0, len(ids))
	for _, id := range ids {
		out = append(out, s.bidsByID[id])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// GetBid returns the bid by id.
func (s *MemoryService) GetBid(_ context.Context, id string) (Bid, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.bidsByID[id]
	if !ok {
		return Bid{}, ErrNotFound
	}
	return b, nil
}

// UpdateBidStatus mutates a bid's status. Caller is responsible for
// FSM correctness; the memory backend trusts it.
func (s *MemoryService) UpdateBidStatus(_ context.Context, bidID, status string) (Bid, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.bidsByID[bidID]
	if !ok {
		return Bid{}, ErrNotFound
	}
	b.Status = status
	s.bidsByID[bidID] = b
	return b, nil
}

// CountBidsForTask returns the number of bids on a task. Used by the
// GraphQL bidCount field so the resolver does not have to call ListBids
// just for a count.
func (s *MemoryService) CountBidsForTask(_ context.Context, taskID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.bidsByTsk[taskID]), nil
}

// --- templates ---------------------------------------------------------

// UpsertTemplate inserts or updates by slug. Slug is the natural
// uniqueness key (kebab-case URL fragment).
func (s *MemoryService) UpsertTemplate(_ context.Context, t Template) (Template, error) {
	if strings.TrimSpace(t.Slug) == "" || strings.TrimSpace(t.Name) == "" {
		return Template{}, ErrInvalidAmount
	}
	if t.PriceUSD.IsNegative() {
		return Template{}, ErrInvalidAmount
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existingID, ok := s.bySlug[t.Slug]; ok {
		ex := s.templates[existingID]
		if ex.AuthorUserID != t.AuthorUserID {
			return Template{}, ErrForbidden
		}
		ex.Name = t.Name
		ex.Description = t.Description
		ex.PriceUSD = t.PriceUSD
		ex.GatesPassed = append([]string(nil), t.GatesPassed...)
		s.templates[existingID] = ex
		return ex, nil
	}
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}
	s.templates[t.ID] = t
	s.bySlug[t.Slug] = t.ID
	return t, nil
}

// GetTemplateBySlug returns the template by its public slug.
func (s *MemoryService) GetTemplateBySlug(_ context.Context, slug string) (Template, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.bySlug[slug]
	if !ok {
		return Template{}, ErrNotFound
	}
	return s.templates[id], nil
}

// ListTemplates returns templates newest-first, optionally filtered to
// verified-only (the production default for non-author callers).
func (s *MemoryService) ListTemplates(_ context.Context, verifiedOnly bool) ([]Template, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Template, 0, len(s.templates))
	for _, t := range s.templates {
		if verifiedOnly && !t.Verified {
			continue
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// IncrementTemplateInstallCount is called by RecordInstall after the
// wallet debit lands so install_count tracks succeeded installs only.
func (s *MemoryService) IncrementTemplateInstallCount(_ context.Context, templateID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.templates[templateID]
	if !ok {
		return ErrNotFound
	}
	t.InstallCount++
	s.templates[templateID] = t
	return nil
}

// --- installs / payouts ------------------------------------------------

// RecordInstall stores the rev-share row. Caller is responsible for
// the split arithmetic (see templates.go::computeInstallSplit).
func (s *MemoryService) RecordInstall(_ context.Context, i Install) (Install, error) {
	if i.AmountUSD.IsNegative() {
		return Install{}, ErrInvalidAmount
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if i.ID == "" {
		i.ID = uuid.NewString()
	}
	if i.InstalledAt.IsZero() {
		i.InstalledAt = time.Now().UTC()
	}
	s.installs[i.ID] = i
	return i, nil
}

// RecordPayout stores the cash-out row in pending state. The real
// Stripe transfer is a TODO — see payouts.go::payOutFinisher.
func (s *MemoryService) RecordPayout(_ context.Context, p Payout) (Payout, error) {
	if p.AmountUSD.IsNegative() {
		return Payout{}, ErrInvalidAmount
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	if p.Status == "" {
		p.Status = "pending"
	}
	s.payouts[p.ID] = p
	return p, nil
}

// --- idempotency -------------------------------------------------------

// RecallOp returns the prior outcome of an opKey-keyed mutation if one
// landed. Mirrors wallet.MemoryService's recallOp/rememberOp pair.
func (s *MemoryService) RecallOp(_ context.Context, opKey string) (OpOutcome, bool, error) {
	if opKey == "" {
		return OpOutcome{}, false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.opKeys[opKey]
	return v, ok, nil
}

// RecordOp records the outcome of an opKey-keyed mutation.
func (s *MemoryService) RecordOp(_ context.Context, opKey, _ string, _ decimal.Decimal, status, errorCode string) error {
	if opKey == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opKeys[opKey] = OpOutcome{Status: status, ErrorCode: errorCode}
	return nil
}

// --- reconciliation ---------------------------------------------------

// ListStaleOpenBids returns bids older than olderThanSec that are
// still open — the cron uses this to mark abandoned offers as
// withdrawn.
func (s *MemoryService) ListStaleOpenBids(_ context.Context, olderThanSec int) ([]Bid, error) {
	cutoff := time.Now().UTC().Add(-time.Duration(olderThanSec) * time.Second)
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Bid{}
	for _, b := range s.bidsByID {
		if b.Status != BidStatusOpen {
			continue
		}
		if b.CreatedAt.Before(cutoff) {
			out = append(out, b)
		}
	}
	return out, nil
}

// ListAbandonedTasks returns open / bidding tasks past their SLA so the
// cron can expire them and release the requestor's hold.
func (s *MemoryService) ListAbandonedTasks(_ context.Context, _ int) ([]GuildTask, error) {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []GuildTask{}
	for _, t := range s.tasks {
		if t.Status != TaskStatusOpen && t.Status != TaskStatusBidding {
			continue
		}
		if t.SLAHours <= 0 {
			continue
		}
		deadline := t.CreatedAt.Add(time.Duration(t.SLAHours) * time.Hour)
		if now.After(deadline) {
			out = append(out, t)
		}
	}
	return out, nil
}
