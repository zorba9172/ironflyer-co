package costcascade

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ironflyer/core/orchestrator/internal/ai/providers"
)

// CachedResponse is one replayable completion: the full assembled text plus
// the provider/model/usage of the original billed call. A cache hit replays
// this as a synthetic Delta stream and charges NOTHING — that is the entire
// point of the layer.
type CachedResponse struct {
	Text     string
	Thinking string
	Provider string
	Model    string
	Usage    providers.Usage
	storedAt time.Time
}

// ResponseCache is an exact-hash, bounded-LRU, TTL'd store of completions.
//
// Safety is deliberate: only DETERMINISTIC-intent calls are eligible
// (Temperature == 0, no tools, no attachments). Two byte-identical
// zero-temperature requests are expected to produce an equivalent answer,
// so replaying a recent one is correct — and it is exactly the "hash the
// input; if identical, return the cached answer; no AI call" layer from the
// cost-optimization vision. There is no semantic/fuzzy matching here: a
// single character of prompt drift misses and falls through to a real
// model call, so generated code is never served from a stale near-match.
type ResponseCache struct {
	mu      sync.Mutex
	ll      *list.List               // front = most-recently-used
	entries map[string]*list.Element // key → element holding *cacheNode
	max     int
	ttl     time.Duration
}

type cacheNode struct {
	key string
	val CachedResponse
}

// NewResponseCache builds the cache. maxEntries <= 0 falls back to 512; a
// non-positive ttl falls back to 30 minutes.
func NewResponseCache(maxEntries int, ttl time.Duration) *ResponseCache {
	if maxEntries <= 0 {
		maxEntries = 512
	}
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	return &ResponseCache{
		ll:      list.New(),
		entries: make(map[string]*list.Element, maxEntries),
		max:     maxEntries,
		ttl:     ttl,
	}
}

// Eligible reports whether a request may be cached/served from cache. The
// predicate is conservative on purpose (see the type doc).
func (c *ResponseCache) Eligible(req providers.Request) bool {
	if c == nil {
		return false
	}
	if req.Temperature != 0 {
		return false
	}
	if len(req.Tools) > 0 || len(req.Attachments) > 0 {
		return false
	}
	if strings.TrimSpace(req.Prompt) == "" && strings.TrimSpace(req.System) == "" {
		return false
	}
	return true
}

// key derives the canonical exact-match hash for a request. Every field
// that can change the answer participates; the capability set is sorted so
// tag ordering never splits the cache.
func key(req providers.Request) string {
	caps := make([]string, len(req.Capabilities))
	for i, cp := range req.Capabilities {
		caps[i] = string(cp)
	}
	sort.Strings(caps)
	h := sha256.New()
	write := func(parts ...string) {
		for _, p := range parts {
			h.Write([]byte(p))
			h.Write([]byte{0x1f}) // unit separator — avoids field-boundary collisions
		}
	}
	write("sys", req.System)
	write("prompt", req.Prompt)
	write("ctx", req.ProjectContext)
	write("schema", req.JSONSchema)
	write("caps", strings.Join(caps, ","))
	write("max", strconv.Itoa(req.MaxTokens))
	write("temp", strconv.FormatFloat(float64(req.Temperature), 'f', -1, 32))
	write("think", strconv.FormatBool(req.EnableThinking), strconv.Itoa(req.ThinkingBudget))
	return hex.EncodeToString(h.Sum(nil))
}

// Get returns a fresh cached response for the request, if present and not
// expired. A hit is promoted to MRU; an expired entry is evicted on read.
// ctx is accepted to satisfy the ResponseStore contract (a semantic store
// needs it to embed); the exact-hash store ignores it.
func (c *ResponseCache) Get(_ context.Context, req providers.Request) (CachedResponse, bool) {
	if c == nil {
		return CachedResponse{}, false
	}
	k := key(req)
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.entries[k]
	if !ok {
		return CachedResponse{}, false
	}
	node := el.Value.(*cacheNode)
	if time.Since(node.val.storedAt) > c.ttl {
		c.ll.Remove(el)
		delete(c.entries, k)
		return CachedResponse{}, false
	}
	c.ll.MoveToFront(el)
	return node.val, true
}

// Put stores a completed response under the request's hash, evicting the
// LRU entry when at capacity. The stored timestamp is stamped here so TTL
// is measured from insertion. ctx satisfies the ResponseStore contract and
// is ignored by the exact-hash store.
func (c *ResponseCache) Put(_ context.Context, req providers.Request, resp CachedResponse) {
	if c == nil {
		return
	}
	if strings.TrimSpace(resp.Text) == "" {
		return // never cache an empty completion
	}
	k := key(req)
	resp.storedAt = time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.entries[k]; ok {
		el.Value.(*cacheNode).val = resp
		c.ll.MoveToFront(el)
		return
	}
	el := c.ll.PushFront(&cacheNode{key: k, val: resp})
	c.entries[k] = el
	for c.ll.Len() > c.max {
		oldest := c.ll.Back()
		if oldest == nil {
			break
		}
		c.ll.Remove(oldest)
		delete(c.entries, oldest.Value.(*cacheNode).key)
	}
}
