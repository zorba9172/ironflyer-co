// Package finisher — N-best voting wrapper for high-stakes agent calls.
// Real multi-agent coordination requires more than a single LLM
// verdict — when Architect picks a stack or Critic decides whether a
// patch ships, we'd rather race N copies of the same role and take
// the majority answer than trust one roll of the dice.
//
// The wrapper is opt-in: callers wrap a normal agents.Run() with
// RunVoted() when the decision matters more than the latency.

package finisher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"sync"

	"ironflyer/apps/orchestrator/internal/agents"
)

// VoteOpts configures one vote.
type VoteOpts struct {
	// N is the number of parallel runs; default 3 when N <= 0.
	N int
	// VoteKey extracts the canonical "what this vote is FOR" string
	// from a Result. Two results that share a VoteKey count as the
	// same vote. Default: identity over res.Output trimmed.
	VoteKey func(agents.Result) string
	// Confidence threshold — we require >= Confidence fraction of
	// voters to agree before returning a winner. Below this, return
	// (Result{}, false) so the caller falls back to the original
	// single-shot Run.
	Confidence float64 // default 0.5
}

// RunVoted dispatches `n` copies of task in parallel and returns
// the winning Result. Failed runs (errors) count as a "no vote" and
// don't influence the result.
//
// Return contract:
//   - (winner, true,  nil) — majority winner found.
//   - (zero,   false, nil) — all runs failed OR no bucket met Confidence.
//     The caller should fall back to a single-shot Run.
//   - (zero,   false, err) — ctx was cancelled while goroutines ran.
//
// Callers that need the winning share for telemetry can use
// RunVotedShare; this entry point stays narrow so the typical caller
// doesn't have to thread a value it doesn't care about.
func RunVoted(ctx context.Context, r *agents.Registry, task agents.Task, opts VoteOpts) (agents.Result, bool, error) {
	res, ok, _, err := RunVotedShare(ctx, r, task, opts)
	return res, ok, err
}

// RunVotedShare is RunVoted plus the winning bucket's share of
// successful voters (winner_count / total_successes). Returned share is
// zero when ok is false. Cancellation behaviour matches RunVoted.
func RunVotedShare(ctx context.Context, r *agents.Registry, task agents.Task, opts VoteOpts) (agents.Result, bool, float64, error) {
	n := opts.N
	if n <= 0 {
		n = 3
	}
	conf := opts.Confidence
	if conf <= 0 {
		conf = 0.5
	}
	keyFn := opts.VoteKey
	if keyFn == nil {
		keyFn = defaultVoteKey
	}

	type slot struct {
		res agents.Result
		ok  bool
	}
	results := make([]slot, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			res, err := r.Run(ctx, task)
			if err != nil {
				return
			}
			results[i] = slot{res: res, ok: true}
		}()
	}

	// Wait for either all goroutines or ctx cancellation. We still let
	// the goroutines finish in the cancellation case — they will observe
	// ctx.Done() through r.Run and unwind quickly.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-ctx.Done():
		// Drain stragglers so we don't leak goroutines, but report the
		// cancellation back to the caller.
		<-done
		return agents.Result{}, false, 0, ctx.Err()
	case <-done:
	}

	// Bucket results by canonical vote key.
	type bucket struct {
		key   string
		first agents.Result
		count int
	}
	byKey := map[string]*bucket{}
	order := []string{}
	successes := 0
	for _, s := range results {
		if !s.ok {
			continue
		}
		successes++
		k := keyFn(s.res)
		b, ok := byKey[k]
		if !ok {
			b = &bucket{key: k, first: s.res}
			byKey[k] = b
			order = append(order, k)
		}
		b.count++
	}

	if successes == 0 {
		return agents.Result{}, false, 0, nil
	}

	// Pick the bucket with the most votes. Ties broken by insertion
	// order — keeps the result deterministic when N=2 splits down the
	// middle (the earliest-keyed bucket wins the tie before the
	// Confidence gate runs).
	sort.SliceStable(order, func(i, j int) bool {
		return byKey[order[i]].count > byKey[order[j]].count
	})
	winner := byKey[order[0]]

	share := float64(winner.count) / float64(successes)
	if share < conf {
		return agents.Result{}, false, 0, nil
	}
	return winner.first, true, share, nil
}

// defaultVoteKey produces a canonical key from a Result. When the
// output parses as JSON we marshal the decoded value back out so
// whitespace + key-order differences collapse to the same vote.
// Otherwise we lowercase + collapse whitespace and hash so two
// near-identical free-form answers vote together.
func defaultVoteKey(res agents.Result) string {
	out := strings.TrimSpace(res.Output)
	if out == "" {
		return ""
	}

	// Try to find a JSON object/array inside the output. Models often
	// wrap JSON in prose; we accept either pure JSON or the first
	// balanced object/array we can decode.
	if candidate, ok := extractJSON(out); ok {
		var parsed any
		if err := json.Unmarshal([]byte(candidate), &parsed); err == nil {
			normalised, err := json.Marshal(canonicalise(parsed))
			if err == nil {
				sum := sha256.Sum256(normalised)
				return "json:" + hex.EncodeToString(sum[:])
			}
		}
	}

	// Free-form fallback: lowercase, collapse runs of whitespace, hash.
	lower := strings.ToLower(out)
	var b strings.Builder
	b.Grow(len(lower))
	prevSpace := false
	for _, r := range lower {
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(b.String())))
	return "text:" + hex.EncodeToString(sum[:])
}

// canonicalise sorts every map key recursively so map[string]any
// values with different key orders produce the same Marshal output.
func canonicalise(v any) any {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		// We can't change the type to "ordered map" — json.Marshal
		// already sorts map[string]any keys alphabetically, so the
		// recursion below is enough.
		out := make(map[string]any, len(t))
		for _, k := range keys {
			out[k] = canonicalise(t[k])
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = canonicalise(e)
		}
		return out
	default:
		return v
	}
}

// extractJSON returns the first balanced JSON object or array
// substring inside s. Used so a Critic answer like "Verdict: { ... }"
// still parses for the vote key. Returns ("", false) if no
// candidate is found.
func extractJSON(s string) (string, bool) {
	start := -1
	var open, close byte
	for i := 0; i < len(s); i++ {
		if s[i] == '{' {
			start = i
			open, close = '{', '}'
			break
		}
		if s[i] == '[' {
			start = i
			open, close = '[', ']'
			break
		}
	}
	if start < 0 {
		return "", false
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return s[start : i+1], true
			}
		}
	}
	return "", false
}

// formatFloat renders f with two decimal places. Used in event
// messages so dashboard JSON parsing stays predictable.
func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', 2, 64)
}
