// Package bus is the multi-pod event fan-out substrate. GraphQL
// subscriptions, SSE streams, and other live feeds run in-process today
// — when the orchestrator scales horizontally on Kubernetes a subscriber
// on pod A would not see events emitted by a resolver on pod B unless
// every emit also crosses a shared message bus.
//
// This package supplies that bus in two flavours:
//
//   - MemoryBus — single-pod, no external dependency. Used in dev and
//     in CI where REDIS_URL is unset. Publish is a direct map-of-chan
//     fan-out; Subscribe returns a buffered channel.
//
//   - RedisBus — wraps the existing redisbus.Client and uses Redis
//     pub/sub for cross-pod delivery. Plain pub/sub (not Streams) because
//     subscribers cannot reconnect mid-stream — they re-subscribe at the
//     subscription layer, not at the bus layer.
//
// On top of either backend, the Multiplexer (multi.go) maintains an
// in-process subscriber map so same-pod delivery never round-trips
// through Redis, and so a pod ignores its own messages when they come
// back across the wire (the 8-byte pod-id prefix marks each payload).
//
// Topic convention: `<kind>:<key>` where kind is the first colon segment
// (collab.presence, collab.cursors, collab.chat, cost, finisher.run,
// deploy, figma, inline). Topics with the prefix `local:` are kept on
// this pod only — used by short-lived flows where publisher and
// subscriber are guaranteed to share a pod (e.g. inline completion).
package bus

import (
	"context"
	"errors"
)

// Bus is the publish/subscribe contract every backend implements. Both
// Publish and Subscribe are nil-safe at the multiplexer layer — callers
// never branch on backend availability.
//
// Publish writes the payload to every active subscriber of `topic`.
// Delivery is best-effort: a slow subscriber drops messages rather than
// back-pressuring the producer.
//
// Subscribe attaches a new subscriber and returns the receive channel
// plus an idempotent cancel function. Callers MUST invoke the cancel
// when done; it tears down the underlying Redis subscription if this
// was the last live subscriber on the topic.
type Bus interface {
	Publish(ctx context.Context, topic string, payload []byte) error
	Subscribe(ctx context.Context, topic string) (<-chan []byte, func(), error)
	Close() error
}

// SubBuffer is the per-subscriber channel capacity. Sized to absorb a
// brief burst of cursor/chat events (10/sec for a few seconds) without
// dropping; beyond that the oldest entry is evicted so a stuck consumer
// can't pin memory.
const SubBuffer = 256

// ErrClosed is returned by Publish/Subscribe on a closed Bus.
var ErrClosed = errors.New("bus: closed")

// LocalPrefix marks a topic as same-pod only. The Multiplexer recognises
// the prefix and skips the cross-pod hop entirely — useful for flows
// where the publisher and subscriber are the same request handler
// (e.g. inline completion).
const LocalPrefix = "local:"

// isLocal returns true when the topic should never cross pods.
func isLocal(topic string) bool {
	return len(topic) >= len(LocalPrefix) && topic[:len(LocalPrefix)] == LocalPrefix
}
