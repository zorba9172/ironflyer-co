package auth

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// MemoryUserStore is the dev-mode implementation. Seeds a `demo@ironflyer.dev`
// user with password `demo1234` so the existing demo flows keep working.
type MemoryUserStore struct {
	mu       sync.RWMutex
	byID     map[string]User
	byEmail  map[string]string // email → id
	hashes   map[string]string // id → hash
}

func NewMemoryUserStore() *MemoryUserStore {
	s := &MemoryUserStore{
		byID:    make(map[string]User),
		byEmail: make(map[string]string),
		hashes:  make(map[string]string),
	}
	s.seedDemo()
	return s
}

func (s *MemoryUserStore) seedDemo() {
	hash, _ := bcrypt.GenerateFromPassword([]byte("demo1234"), bcrypt.DefaultCost)
	u := User{
		ID: "demo", Email: "demo@ironflyer.dev", Name: "Demo User",
		Plan: "pro", CreatedAt: time.Now().UTC(),
	}
	s.byID[u.ID] = u
	s.byEmail[u.Email] = u.ID
	s.hashes[u.ID] = string(hash)
}

func (s *MemoryUserStore) Create(_ context.Context, email, name, hash string) (User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byEmail[email]; ok {
		return User{}, ErrUserExists
	}
	u := User{
		ID: uuid.NewString(), Email: email, Name: name,
		Plan: "free", CreatedAt: time.Now().UTC(),
	}
	s.byID[u.ID] = u
	s.byEmail[email] = u.ID
	s.hashes[u.ID] = hash
	return u, nil
}

func (s *MemoryUserStore) GetByEmail(_ context.Context, email string) (User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.byEmail[email]
	if !ok {
		return User{}, "", ErrUserNotFound
	}
	return s.byID[id], s.hashes[id], nil
}

func (s *MemoryUserStore) GetByID(_ context.Context, id string) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.byID[id]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return u, nil
}

// GetByIDs batch-loads users by id under a single read lock. Missing ids
// are omitted from the result map; the caller decides whether that's a
// 404 or simply "not in this batch".
func (s *MemoryUserStore) GetByIDs(_ context.Context, ids []string) (map[string]User, error) {
	out := make(map[string]User, len(ids))
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, id := range ids {
		if u, ok := s.byID[id]; ok {
			out[id] = u
		}
	}
	return out, nil
}

func (s *MemoryUserStore) SetPlan(_ context.Context, id, plan string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.byID[id]
	if !ok {
		return ErrUserNotFound
	}
	u.Plan = plan
	s.byID[id] = u
	return nil
}

// SetTelemetryOptOut mirrors the Postgres implementation: flips the
// per-user opt-out boolean in-place. Returns ErrUserNotFound for an
// unknown id so callers can route 404 vs 500.
func (s *MemoryUserStore) SetTelemetryOptOut(_ context.Context, id string, opt bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.byID[id]
	if !ok {
		return ErrUserNotFound
	}
	u.TelemetryOptOut = opt
	s.byID[id] = u
	return nil
}

// SetRoles overwrites the in-memory user's role set so dev seeding
// (e.g. promoting demo@ironflyer.dev to platform_operator) can match
// the Postgres path. Roles are normalised through normaliseRoleSet
// so a stray "Platform_Operator " behaves identically to its
// canonical "platform_operator" form. Implements auth.RoleSetter.
func (s *MemoryUserStore) SetRoles(_ context.Context, id string, roles []string) error {
	clean := normaliseRoleSet(roles)
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.byID[id]
	if !ok {
		return ErrUserNotFound
	}
	u.Roles = clean
	s.byID[id] = u
	return nil
}

// Delete removes a user record and any auxiliary lookups. Idempotent on
// missing IDs: returns ErrUserNotFound so the caller can choose its
// behaviour.
func (s *MemoryUserStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.byID[id]
	if !ok {
		return ErrUserNotFound
	}
	delete(s.byID, id)
	delete(s.byEmail, u.Email)
	delete(s.hashes, id)
	return nil
}

var _ UserStore = (*MemoryUserStore)(nil)
