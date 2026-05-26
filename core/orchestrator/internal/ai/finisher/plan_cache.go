// Package finisher — plan cache. The Planner / Architect / UXer pipeline
// is expensive: it spends extended-thinking tokens on every Run() even
// when the project's spec hasn't materially changed. The plan cache keys
// those artifacts by a content hash of the inputs that actually drive the
// plan (name + description + idea + stories) so a follow-up Run can reuse
// the prior plan instead of repaying for it.
//
// The cache is in-memory (orchestrator process lifetime) and per-project,
// bounded to avoid runaway growth. On a recovery cycle this can save 3+
// expensive LLM calls per failed gate sweep — the wins compound when the
// repair loop needs several iterations to satisfy the gates.
package finisher

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"ironflyer/core/orchestrator/internal/ai/domain"
)

// planCacheEntry pins one set of artifacts to the spec hash they were
// derived from. We store the artifacts as their canonical raw-JSON form
// so the loop can apply them via the same SetArtifact path that a fresh
// Planner / Architect / UXer call would have used.
type planCacheEntry struct {
	SpecHash       string
	PlanJSON       json.RawMessage
	StackJSON      json.RawMessage
	ScreenMapJSON  json.RawMessage
	DesignTokens   json.RawMessage
	UserStories    []domain.UserStory
	DataModel      []domain.EntityDef
	Stack          domain.StackDecision
	StoredAt       time.Time
}

type planCache struct {
	mu      sync.Mutex
	entries map[string]planCacheEntry // keyed by projectID
	max     int
}

func newPlanCache() *planCache {
	return &planCache{entries: map[string]planCacheEntry{}, max: 64}
}

// hashSpec captures the load-bearing inputs that distinguish one Plan from
// another. We deliberately exclude generated files and gate state so that
// re-running a project with the same idea + stories produces a cache hit.
func hashSpec(p *domain.Project) string {
	if p == nil {
		return ""
	}
	h := sha256.New()
	h.Write([]byte(strings.TrimSpace(p.Name)))
	h.Write([]byte{0})
	h.Write([]byte(strings.TrimSpace(p.Description)))
	h.Write([]byte{0})
	h.Write([]byte(strings.TrimSpace(p.Spec.Idea)))
	h.Write([]byte{0})
	// Stories are ordered, so iterate in declared order to keep the hash
	// stable across runs.
	for _, s := range p.Spec.UserStories {
		h.Write([]byte(s.ID))
		h.Write([]byte{1})
		h.Write([]byte(s.As))
		h.Write([]byte{1})
		h.Write([]byte(s.IWant))
		h.Write([]byte{1})
		h.Write([]byte(s.SoThat))
		for _, a := range s.Acceptance {
			h.Write([]byte{2})
			h.Write([]byte(a))
		}
		h.Write([]byte{3})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (c *planCache) get(projectID, specHash string) (planCacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[projectID]
	if !ok {
		return planCacheEntry{}, false
	}
	if entry.SpecHash != specHash {
		return planCacheEntry{}, false
	}
	return entry, true
}

func (c *planCache) put(projectID string, entry planCacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= c.max {
		// Evict the oldest entry. O(N) is fine — N is tiny.
		var oldestKey string
		var oldestTime time.Time
		for k, v := range c.entries {
			if oldestKey == "" || v.StoredAt.Before(oldestTime) {
				oldestKey, oldestTime = k, v.StoredAt
			}
		}
		delete(c.entries, oldestKey)
	}
	entry.StoredAt = time.Now().UTC()
	c.entries[projectID] = entry
}

// captureFromProject builds a cache entry from a project's current
// artifacts. Called after a successful pipeline run so the next Run on
// the same spec can short-circuit. Returns false when the project does
// not yet have enough artifacts to be worth caching.
func captureFromProject(p *domain.Project) (planCacheEntry, bool) {
	if p == nil {
		return planCacheEntry{}, false
	}
	plan, _ := p.GetArtifact(domain.ArtifactPlan)
	stack, _ := p.GetArtifact(domain.ArtifactStack)
	sm, _ := p.GetArtifact(domain.ArtifactScreenMap)
	dt, _ := p.GetArtifact(domain.ArtifactDesignTokens)
	if len(plan) == 0 || len(stack) == 0 || len(sm) == 0 || len(dt) == 0 {
		return planCacheEntry{}, false
	}
	return planCacheEntry{
		SpecHash:      hashSpec(p),
		PlanJSON:      append(json.RawMessage(nil), plan...),
		StackJSON:     append(json.RawMessage(nil), stack...),
		ScreenMapJSON: append(json.RawMessage(nil), sm...),
		DesignTokens:  append(json.RawMessage(nil), dt...),
		UserStories:   append([]domain.UserStory(nil), p.Spec.UserStories...),
		DataModel:     append([]domain.EntityDef(nil), p.Spec.DataModel...),
		Stack:         p.Spec.Stack,
	}, true
}
