// Package redisbus wraps the go-redis client with the few helpers the
// orchestrator actually needs to scale horizontally: a distributed lock
// keyed by project ID so two pods can't run the finisher on the same
// project concurrently, and a fixed-window rate limiter so the
// per-user / per-IP buckets become a single source of truth across
// pods.
//
// Every helper is nil-safe: when *Client is nil the orchestrator is
// running single-pod and the in-process implementations remain the
// source of truth. That lets main.go opt-in to Redis behind a flag
// without forcing every code path to branch on availability.
package redisbus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps go-redis with the helpers the orchestrator actually
// needs. Nil-safe: every helper degrades cleanly when Client is nil
// (callers use the in-process fallback in that case).
type Client struct {
	*redis.Client
}

// New connects to Redis at addr (host:port) with the given password
// and DB index. Connection establishment is lazy in go-redis — callers
// should follow up with Ping() to fail fast at startup.
func New(addr, password string, db int) (*Client, error) {
	if addr == "" {
		return nil, errors.New("redisbus: empty address")
	}
	rc := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &Client{Client: rc}, nil
}

// Ping verifies the Redis connection is alive. Returns nil on a
// nil client so the opt-in path in main.go can call it unconditionally
// during smoke checks.
func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.Client == nil {
		return nil
	}
	return c.Client.Ping(ctx).Err()
}

// unlockScript only deletes the lock key when the value still matches
// the token we set — prevents a slow holder from accidentally releasing
// a newer holder's lock after its own TTL expired.
var unlockScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
else
    return 0
end
`)

// Lock acquires a distributed lock keyed by `key` for `ttl`. Returns
// the unlock function (idempotent) and a bool indicating whether
// the lock was acquired. Returns (no-op unlock, true) when c is nil
// — the in-process orchestrator effectively single-pod, so the
// mutex Engine already owns is sufficient.
func (c *Client) Lock(ctx context.Context, key string, ttl time.Duration) (unlock func(), acquired bool, err error) {
	noop := func() {}
	if c == nil || c.Client == nil {
		return noop, true, nil
	}
	token, err := randomToken()
	if err != nil {
		return noop, false, err
	}
	ok, err := c.Client.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return noop, false, err
	}
	if !ok {
		return noop, false, nil
	}
	var released bool
	unlock = func() {
		if released {
			return
		}
		released = true
		// Use a background context so a cancelled caller still releases
		// the lock — relying on TTL alone would block other pods for
		// up to `ttl` after a panic/cancel.
		relCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = unlockScript.Run(relCtx, c.Client, []string{key}, token).Result()
	}
	return unlock, true, nil
}

// AllowRate enforces a fixed-window rate limit. Returns (allowed,
// remaining, error). When c is nil, allowed=true with remaining=-1
// so the caller's in-process limiter takes over.
func (c *Client) AllowRate(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, remaining int, err error) {
	if c == nil || c.Client == nil {
		return true, -1, nil
	}
	if limit <= 0 {
		return true, -1, nil
	}
	pipe := c.Client.TxPipeline()
	incr := pipe.Incr(ctx, key)
	// EXPIRE ... NX only sets the TTL when the key has no TTL set yet,
	// which is exactly the semantics of a fixed window: the first
	// request in the window starts the clock.
	pipe.ExpireNX(ctx, key, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, 0, err
	}
	count := int(incr.Val())
	if count > limit {
		return false, 0, nil
	}
	return true, limit - count, nil
}

// Publish broadcasts a JSON-marshallable payload to all subscribers
// of the given channel name. Returns the number of receivers that
// got the message (Redis PUBLISH return value). When c is nil, the
// call is a no-op returning (0, nil) so callers in single-pod mode
// don't need to gate every publish site.
func (c *Client) Publish(ctx context.Context, channel string, payload any) (int64, error) {
	if c == nil || c.Client == nil {
		return 0, nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	return c.Client.Publish(ctx, channel, data).Result()
}

// Subscribe opens a long-lived subscription to the given channel.
// Returns a receive-only channel of raw JSON-encoded payload strings.
// The returned cancel func releases the underlying Redis subscriber
// and closes the channel; it is safe to call multiple times. When c
// is nil, Subscribe returns a closed channel and a no-op cancel so
// callers can fall through to their in-process subscription path.
func (c *Client) Subscribe(ctx context.Context, channel string) (<-chan string, func(), error) {
	if c == nil || c.Client == nil {
		ch := make(chan string)
		close(ch)
		return ch, func() {}, nil
	}
	pubsub := c.Client.Subscribe(ctx, channel)
	// Wait for the subscription to be established so the caller can
	// rely on subsequent publishes actually being delivered.
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return nil, nil, err
	}
	out := make(chan string, 64)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer close(out)
		src := pubsub.Channel()
		for msg := range src {
			select {
			case out <- msg.Payload:
			default:
				// Drop on slow consumer — SSE will reconnect and
				// resume from the project's persisted event log.
			}
		}
	}()
	var cancelled bool
	cancel := func() {
		if cancelled {
			return
		}
		cancelled = true
		_ = pubsub.Close()
		<-done
	}
	return out, cancel, nil
}

func randomToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
