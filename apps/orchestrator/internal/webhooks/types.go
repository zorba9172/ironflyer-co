// Package webhooks lets a user subscribe an external HTTPS endpoint to the
// rich SSE events emitted by the finisher engine. It signs every payload
// with HMAC-SHA256 (keyed on a per-subscription secret) so receivers can
// verify the call really came from Ironflyer.
//
// Wire shape:
//   POST <subscription.URL>
//   Content-Type: application/json
//   X-Ironflyer-Signature: sha256=<hex(hmac(secret, body))>
//   X-Ironflyer-Event: <event name>
//   X-Ironflyer-Delivery: <uuid — same id retried on each attempt>
//   Idempotency-Key: <delivery id>  // mirror, makes dedupe trivial server-side
//
// Subscribers can opt into a subset of events with the Events list — leaving
// it nil or empty means "send everything". An optional ProjectID narrows the
// fan-out to a single project; empty means "every project I own".
package webhooks

import "time"

// Subscription is a single registered webhook target.
type Subscription struct {
	ID           string    `json:"id"`
	UserID       string    `json:"userId"`
	ProjectID    string    `json:"projectId,omitempty"` // empty = every project owned by UserID
	URL          string    `json:"url"`
	Secret       string    `json:"secret,omitempty"`
	Events       []string  `json:"events,omitempty"` // empty = all
	CreatedAt    time.Time `json:"createdAt"`
	LastSentAt   time.Time `json:"lastSentAt,omitempty"`
	FailureCount int       `json:"failureCount"`
	Disabled     bool      `json:"disabled"`
}

// Matches reports whether evt should be delivered to s. An empty Events list
// is treated as "wildcard" — every event matches.
func (s Subscription) Matches(eventName string) bool {
	if len(s.Events) == 0 {
		return true
	}
	for _, e := range s.Events {
		if e == eventName || e == "*" {
			return true
		}
	}
	return false
}
