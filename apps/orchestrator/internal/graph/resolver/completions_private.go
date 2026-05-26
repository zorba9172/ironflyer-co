package resolver

// completions_private.go — env switch for routing inline completions
// to a private (self-hosted / on-prem) provider when one is wired.
//
// Why a separate file: gqlgen aggressively moves helpers out of
// auto-generated resolver files into a "/* harms way */" comment
// block on the next regenerate. Helpers that live in their own file
// survive every codegen pass untouched.

import (
	"os"
	"strings"
)

// inlineCompletionsPrivate reports whether the inline-completion
// subscription resolver should add CapPrivate to the bandit request,
// preferring a self-hosted provider over the public Haiku/Flash/4o-mini
// pool. Default OFF — turning the knob ON in self-hosted prod cuts
// per-keystroke cost AND keeps source code on infrastructure the
// operator controls.
func inlineCompletionsPrivate() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("IRONFLYER_INLINE_COMPLETIONS_PRIVATE")))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}
