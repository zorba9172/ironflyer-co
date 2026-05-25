package workspaces

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Registry coordinates ownership of a workspace across runtime pods.
// Every entry is a TTL-bounded Redis key: when a pod owns a workspace
// it heartbeats the key every HeartbeatInterval and the key expires
// after KeyTTL if the pod dies. New requests landing on a different
// pod look up the active owner and proxy to it; if no owner exists,
// the receiving pod takes ownership.
//
// Nil-safe: if New returns nil (Redis URL empty), every method behaves
// as if the calling pod always owns every workspace. That keeps dev /
// single-pod deployments working without conditionals at call sites.
type Registry struct {
	rdb         *redis.Client
	podIP       string
	keyTTL      time.Duration
	heartbeat   time.Duration

	mu    sync.Mutex
	owned map[string]struct{}
}

// Config controls Registry behaviour.
type Config struct {
	RedisURL          string
	PodIP             string
	KeyTTL            time.Duration
	HeartbeatInterval time.Duration
}

// New connects to Redis and returns a Registry. When RedisURL is empty
// the returned Registry is non-nil but every method short-circuits to
// "we own everything" (single-pod mode). That contract lets the HTTP
// layer call Registry methods unconditionally.
func New(cfg Config) (*Registry, error) {
	r := &Registry{
		podIP:     cfg.PodIP,
		keyTTL:    cfg.KeyTTL,
		heartbeat: cfg.HeartbeatInterval,
		owned:     make(map[string]struct{}),
	}
	if r.keyTTL <= 0 {
		r.keyTTL = 90 * time.Second
	}
	if r.heartbeat <= 0 {
		r.heartbeat = 30 * time.Second
	}
	if cfg.RedisURL == "" {
		return r, nil
	}
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, err
	}
	r.rdb = redis.NewClient(opt)
	return r, nil
}

// Close releases the underlying Redis client.
func (r *Registry) Close() error {
	if r == nil || r.rdb == nil {
		return nil
	}
	return r.rdb.Close()
}

// Ping verifies connectivity (no-op when single-pod).
func (r *Registry) Ping(ctx context.Context) error {
	if r == nil || r.rdb == nil {
		return nil
	}
	return r.rdb.Ping(ctx).Err()
}

func key(id string) string { return "runtime:workspace:" + id }

// Claim writes the active-pod key for `id` with our pod IP. The key TTL
// keeps the entry from outliving the pod. Returns true when the local
// pod now owns the workspace.
func (r *Registry) Claim(ctx context.Context, id string) (bool, error) {
	if r == nil {
		return true, nil
	}
	r.mu.Lock()
	r.owned[id] = struct{}{}
	r.mu.Unlock()
	if r.rdb == nil {
		return true, nil
	}
	if err := r.rdb.Set(ctx, key(id), r.podIP, r.keyTTL).Err(); err != nil {
		return false, err
	}
	return true, nil
}

// Lookup returns the pod IP that currently owns `id`, or "" if none.
// When Redis isn't wired we always return our own IP — single-pod
// installs route everything to themselves.
func (r *Registry) Lookup(ctx context.Context, id string) (string, error) {
	if r == nil {
		return "", nil
	}
	if r.rdb == nil {
		return r.podIP, nil
	}
	v, err := r.rdb.Get(ctx, key(id)).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return v, nil
}

// OwnedHere reports whether the local pod is currently tracking this
// workspace as its own. Used by graceful shutdown to know what to hand
// off.
func (r *Registry) OwnedHere(id string) bool {
	if r == nil {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.owned[id]
	return ok
}

// PodIP returns the local pod IP — handy for the /locator endpoint
// and proxy logic.
func (r *Registry) PodIP() string {
	if r == nil {
		return ""
	}
	return r.podIP
}

// Release clears ownership of `id`. Called from graceful shutdown and
// from Destroy. Non-fatal on any error: the key will TTL out anyway.
func (r *Registry) Release(ctx context.Context, id string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	delete(r.owned, id)
	r.mu.Unlock()
	if r.rdb == nil {
		return
	}
	// Only delete if we still own it (script avoids stomping a new owner).
	_, _ = releaseScript.Run(ctx, r.rdb, []string{key(id)}, r.podIP).Result()
}

// Heartbeat starts the background goroutine that refreshes every owned
// key on r.heartbeat cadence. Returns immediately. Stops when ctx is
// cancelled.
func (r *Registry) Heartbeat(ctx context.Context) {
	if r == nil || r.rdb == nil {
		return
	}
	go func() {
		tick := time.NewTicker(r.heartbeat)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				r.refreshOwned(ctx)
			}
		}
	}()
}

func (r *Registry) refreshOwned(ctx context.Context) {
	r.mu.Lock()
	ids := make([]string, 0, len(r.owned))
	for id := range r.owned {
		ids = append(ids, id)
	}
	r.mu.Unlock()
	for _, id := range ids {
		_, _ = refreshScript.Run(ctx, r.rdb, []string{key(id)}, r.podIP, int(r.keyTTL.Seconds())).Result()
	}
}

// OwnedIDs returns a snapshot of the workspaces this pod is currently
// the active owner of. Used by shutdown handoff.
func (r *Registry) OwnedIDs() []string {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.owned))
	for id := range r.owned {
		out = append(out, id)
	}
	return out
}

// releaseScript only deletes the key when the stored value still matches
// our pod IP — same pattern as redisbus.Lock unlock.
var releaseScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
else
    return 0
end
`)

// refreshScript bumps the TTL only when we still own the key; otherwise
// returns 0 so we don't accidentally extend another pod's ownership.
var refreshScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("expire", KEYS[1], tonumber(ARGV[2]))
else
    return 0
end
`)
