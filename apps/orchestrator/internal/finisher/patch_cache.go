// Package finisher — story-level patch cache. Mirrors planCache but
// remembers the actual code patch the Coder produced for a (story,
// specHash) pair. On a retry / repair cycle, the engine first asks the
// cache "do you already have an accepted patch for this story?" — if
// yes, it tries to apply that patch directly through the normal
// lifecycle (Propose → Apply). The patch engine's anchor validator is
// the source of truth for whether the cached patch still fits the
// current file tree; on rejection the cache hit is discarded and the
// loop falls through to a fresh Coder call.
//
// This is a multiplier on the recovery loop: when the orchestrator has
// to repeat the same story (because Lint failed, then Test failed, then
// Security failed), every successful retry currently re-prompts a 50-
// 200K-token Coder + Critic + Reviewer chain. With the cache, the
// second iteration short-circuits to a single Propose+Apply.

package finisher

import (
	"sync"
	"time"

	"ironflyer/apps/orchestrator/internal/patch"
)

type patchCacheEntry struct {
	SpecHash string
	Patch    patch.Patch
	StoredAt time.Time
}

type patchCache struct {
	mu      sync.Mutex
	entries map[string]patchCacheEntry // keyed by "<projectID>:<storyID>"
	max     int
}

func newPatchCache() *patchCache {
	return &patchCache{entries: map[string]patchCacheEntry{}, max: 256}
}

func patchCacheKey(projectID, storyID string) string {
	return projectID + ":" + storyID
}

// get returns the cached patch when (projectID, storyID, specHash) all
// match. A cache miss returns the zero value.
func (c *patchCache) get(projectID, storyID, specHash string) (patch.Patch, bool) {
	if c == nil {
		return patch.Patch{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[patchCacheKey(projectID, storyID)]
	if !ok || e.SpecHash != specHash {
		return patch.Patch{}, false
	}
	return e.Patch, true
}

// put stores a successful patch for later replay. The patch is cloned
// at the FileChange level so later mutations by the caller don't bleed
// into the cache entry.
func (c *patchCache) put(projectID, storyID, specHash string, p patch.Patch) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= c.max {
		// O(N) LRU-by-timestamp eviction. N is bounded by `max`, so fine.
		var oldestKey string
		var oldestTime time.Time
		for k, v := range c.entries {
			if oldestKey == "" || v.StoredAt.Before(oldestTime) {
				oldestKey, oldestTime = k, v.StoredAt
			}
		}
		delete(c.entries, oldestKey)
	}
	clone := p
	if len(p.Changes) > 0 {
		clone.Changes = make([]patch.FileChange, len(p.Changes))
		copy(clone.Changes, p.Changes)
	}
	c.entries[patchCacheKey(projectID, storyID)] = patchCacheEntry{
		SpecHash: specHash,
		Patch:    clone,
		StoredAt: time.Now().UTC(),
	}
}
