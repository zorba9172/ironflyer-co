package costcascade

import (
	"context"
	"errors"
	"strings"

	"ironflyer/core/orchestrator/internal/ai/providers"
)

// RuleResult is what a matched rule returns. Either it supplies a
// deterministic Answer to replay as a zero-cost stream, or it Refuses the
// call (with RefuseErr) before any model is touched. A rule must never
// fabricate generated code — rules are for the file-existence /
// validation / refusal class of work the vision says should never reach a
// model.
type RuleResult struct {
	Answer    string
	Refuse    bool
	RefuseErr error
}

// Rule is one deterministic short-circuit. Match returns (result, true)
// when it handles the request, (zero, false) to pass it down the cascade.
// Match MUST be pure and fast — no network, no model, no I/O.
type Rule struct {
	Name  string
	Match func(ctx context.Context, req providers.Request) (RuleResult, bool)
}

// RuleSet is an ordered list of rules; the first match wins. Order matters
// — register stricter rules before broader ones.
type RuleSet struct {
	rules []Rule
}

// NewRuleSet builds a rule set seeded with the safe built-in rules. Pass
// extra rules to append them after the built-ins.
func NewRuleSet(extra ...Rule) *RuleSet {
	rs := &RuleSet{}
	rs.rules = append(rs.rules, builtinRules()...)
	rs.rules = append(rs.rules, extra...)
	return rs
}

// Add appends a rule to the end of the set. Returns the set for chaining.
func (rs *RuleSet) Add(r Rule) *RuleSet {
	if rs == nil {
		return rs
	}
	rs.rules = append(rs.rules, r)
	return rs
}

// Match runs the rules in order and returns the first hit.
func (rs *RuleSet) Match(ctx context.Context, req providers.Request) (RuleResult, bool) {
	if rs == nil {
		return RuleResult{}, false
	}
	for _, r := range rs.rules {
		if r.Match == nil {
			continue
		}
		if res, ok := r.Match(ctx, req); ok {
			return res, true
		}
	}
	return RuleResult{}, false
}

// ErrEmptyPrompt is returned by the empty-prompt rule. Refusing here costs
// the caller nothing — an empty prompt can only ever burn tokens producing
// noise.
var ErrEmptyPrompt = errors.New("costcascade: empty prompt refused before model call")

// builtinRules are the deterministic short-circuits safe for every
// deployment. Intentionally tiny — the heavy reuse/validation rules belong
// to the gates and the anti-bloat engine; this set only catches the calls
// that are unambiguously wasteful.
func builtinRules() []Rule {
	return []Rule{
		{
			Name: "empty_prompt",
			Match: func(_ context.Context, req providers.Request) (RuleResult, bool) {
				if strings.TrimSpace(req.Prompt) == "" && strings.TrimSpace(req.System) == "" {
					return RuleResult{Refuse: true, RefuseErr: ErrEmptyPrompt}, true
				}
				return RuleResult{}, false
			},
		},
	}
}
