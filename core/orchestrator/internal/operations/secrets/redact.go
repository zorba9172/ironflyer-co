package secrets

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"sync"
)

// Redactor is the single, deterministic scrubber used everywhere
// AI-visible bytes flow out of the orchestrator: model prompts before
// they hit a provider, log lines before they hit zerolog, OTel attrs
// before they hit the exporter, and audit attrs before they hit the
// store.
//
// Two complementary mechanisms:
//
//  1. Exact-value replacement. The broker calls AddSecret(name, value)
//     every time it loads material so the redactor "knows" the byte
//     sequence and can replace it with [redacted:NAME] anywhere it
//     appears downstream.
//
//  2. Pattern replacement. Optional regex patterns (e.g. AWS access
//     key prefixes, Stripe sk_ tokens) catch secrets that the broker
//     never minted but that leaked in from upstream systems.
//
// The Redactor is safe for concurrent use. AddSecret/RemoveSecret are
// O(1) but Scrub is O(N * known-secrets); for the orchestrator's
// workload (a few dozen secrets per tenant, KB-scale log lines) this
// is comfortably fast.
type Redactor struct {
	mu       sync.RWMutex
	values   map[string][]byte // name -> raw value
	names    map[string]string // hash(name) -> name (for collision-safe iteration)
	patterns []*regexp.Regexp
}

// NewRedactor returns a ready-to-use Redactor with no secrets and no
// patterns registered.
func NewRedactor() *Redactor {
	return &Redactor{
		values: make(map[string][]byte),
		names:  make(map[string]string),
	}
}

// AddPattern registers a regexp whose matches will be replaced with
// "[redacted:pattern]". Useful for credentials that flow in from
// upstream (e.g. AWS_*_ID prefixes, Stripe live keys) and which the
// broker did not personally release.
func (r *Redactor) AddPattern(p *regexp.Regexp) {
	if p == nil {
		return
	}
	r.mu.Lock()
	r.patterns = append(r.patterns, p)
	r.mu.Unlock()
}

// AddSecret registers a raw value to be scrubbed wherever it appears.
// The Redactor takes a copy of value so callers may zero their own
// buffer afterwards. Re-registering the same name overwrites the
// previous value (useful on rotation).
func (r *Redactor) AddSecret(name string, value []byte) {
	if name == "" || len(value) == 0 {
		return
	}
	cp := make([]byte, len(value))
	copy(cp, value)
	r.mu.Lock()
	r.values[name] = cp
	r.names[name] = name
	r.mu.Unlock()
}

// RemoveSecret drops a previously registered value. The broker calls
// this when a Capability expires so the in-process redactor stops
// holding material it no longer has any reason to keep.
func (r *Redactor) RemoveSecret(name string) {
	r.mu.Lock()
	if v, ok := r.values[name]; ok {
		// Best-effort zeroisation; Go's runtime may have moved the
		// bytes by now, but where it hasn't, we clear them.
		for i := range v {
			v[i] = 0
		}
		delete(r.values, name)
		delete(r.names, name)
	}
	r.mu.Unlock()
}

// Scrub returns input with every known secret value replaced by
// "[redacted:NAME]" and every pattern match replaced by
// "[redacted:pattern]". Stable iteration order makes the output
// deterministic for hashing and audit.
func (r *Redactor) Scrub(input string) string {
	return string(r.ScrubBytes([]byte(input)))
}

// ScrubBytes is the zero-copy-friendly variant used by middleware
// that already has a byte buffer in hand (HTTP error renderers, span
// exporters, log encoders).
func (r *Redactor) ScrubBytes(input []byte) []byte {
	if len(input) == 0 {
		return input
	}
	r.mu.RLock()
	// Snapshot the maps + patterns so we can release the lock before
	// doing the (potentially slow) regex/bytes work. Sorting names by
	// descending value length avoids partial-overlap bugs where a
	// shorter secret hides inside a longer one.
	names := make([]string, 0, len(r.values))
	for n := range r.values {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		return len(r.values[names[i]]) > len(r.values[names[j]])
	})
	values := make(map[string][]byte, len(names))
	for _, n := range names {
		values[n] = append([]byte(nil), r.values[n]...)
	}
	patterns := append([]*regexp.Regexp(nil), r.patterns...)
	r.mu.RUnlock()

	out := input
	for _, n := range names {
		v := values[n]
		if len(v) == 0 {
			continue
		}
		replacement := []byte(fmt.Sprintf("[redacted:%s]", n))
		out = bytes.ReplaceAll(out, v, replacement)
	}
	for _, p := range patterns {
		out = p.ReplaceAll(out, []byte("[redacted:pattern]"))
	}
	return out
}

// Proof returns the canonical redaction-proof string the broker writes
// into Capability.RedactionProof and into secret_releases.redaction_proof.
// Centralising the format here keeps the audit chain consistent.
func Proof(value []byte) string {
	if len(value) == 0 {
		return "sha256:redacted"
	}
	sum := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(sum[:])
}
