package diagnostics

import (
	"time"

	"github.com/rs/zerolog"
)

// Service is the operator-facing surface around the Ring buffer. The
// HTTP and GraphQL surfaces call through this single seam so the
// dependency wireup stays focused.
type Service struct {
	ring   *Ring
	logger zerolog.Logger
}

// NewService binds a Ring and a logger. Either may be nil — the
// service degrades to no-op behaviour so resolvers can ship without
// the diagnostic plane being enabled.
func NewService(ring *Ring, logger zerolog.Logger) *Service {
	return &Service{ring: ring, logger: logger}
}

// Ring returns the underlying buffer. The HTTP tail endpoint walks
// the snapshot directly; the GraphQL resolver also reaches in for
// recentLogs filtering.
func (s *Service) Ring() *Ring {
	if s == nil {
		return nil
	}
	return s.ring
}

// RecentLogs returns up to `limit` entries newer than `since`, after
// applying minLevel (warn by default). The slice is newest-first.
func (s *Service) RecentLogs(since time.Time, limit int, minLevel string) []Entry {
	if s == nil || s.ring == nil {
		return nil
	}
	if minLevel == "" {
		minLevel = "warn"
	}
	if limit <= 0 {
		limit = 200
	}
	all := s.ring.SnapshotSince(since, 0)
	filtered := FilterByLevel(all, minLevel)
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered
}

// RecentErrors returns the error-class aggregation, capped at `limit`.
func (s *Service) RecentErrors(limit int) []Aggregate {
	if s == nil || s.ring == nil {
		return nil
	}
	if limit <= 0 {
		limit = 50
	}
	all := s.ring.Aggregate()
	if len(all) > limit {
		all = all[:limit]
	}
	return all
}
