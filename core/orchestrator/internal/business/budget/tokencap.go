package budget

// tokencap.go — defense-in-depth per-call prompt size limit.
//
// Why this exists: even with a generous user-level budget cap and a
// per-workload ProfitGuard floor, a SINGLE prompt that balloons (long
// context window, reasoning trace fed back, accumulated repair
// history) can cost $5–$20 by itself. ProfitGuard reserves on
// estimate but the actual debit lands AFTER the call; an underestimate
// blows past the per-step cap silently.
//
// This guard runs BEFORE the call. It is a hard ceiling — exceeding
// it returns ErrPromptTooLarge, the caller bails, ProfitGuard never
// sees the call.
//
// Tunable via env:
//
//	IRONFLYER_MAX_PROMPT_TOKENS   total (input + output) ceiling per
//	                              call. Default 60_000 (sane bound
//	                              for Sonnet/Haiku; Opus 4.7 allows
//	                              200k context but charging that on a
//	                              single retry is rarely justified).
//	IRONFLYER_MAX_INPUT_TOKENS    input-only ceiling. Default 50_000.
//	IRONFLYER_MAX_OUTPUT_TOKENS   output-only ceiling. Default 16_000.
//
// The defaults are chosen so the typical agent prompt (5–10k
// in + 2–4k out) passes comfortably while a runaway loop (the same
// prompt re-fed with full history) trips the cap on iteration 2 or 3.

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

// ErrPromptTooLarge is returned by CheckPromptCap when the estimated
// token count for a single call exceeds the configured ceiling. The
// caller should surface this to the user with a clear "too large"
// signal rather than silently retrying with a smaller prompt — the
// agent prompt construction logic is the real bug.
var ErrPromptTooLarge = errors.New("prompt too large for single call")

// PromptCap is the resolved ceiling set. Zero on any field means
// "no cap for that axis".
type PromptCap struct {
	MaxTotalTokens  int
	MaxInputTokens  int
	MaxOutputTokens int
}

// DefaultPromptCap resolves the cap from env, falling back to the
// safe defaults documented at the top of this file.
func DefaultPromptCap() PromptCap {
	return PromptCap{
		MaxTotalTokens:  envInt("IRONFLYER_MAX_PROMPT_TOKENS", 60_000),
		MaxInputTokens:  envInt("IRONFLYER_MAX_INPUT_TOKENS", 50_000),
		MaxOutputTokens: envInt("IRONFLYER_MAX_OUTPUT_TOKENS", 16_000),
	}
}

// CheckPromptCap returns nil when the (input, output) estimate is
// under every configured ceiling, ErrPromptTooLarge with a precise
// reason otherwise. Use BEFORE dispatching to a provider:
//
//	if err := cap.CheckPromptCap(estIn, estOut); err != nil {
//	    return err
//	}
func (c PromptCap) CheckPromptCap(estInputTokens, estOutputTokens int) error {
	if c.MaxInputTokens > 0 && estInputTokens > c.MaxInputTokens {
		return fmt.Errorf("%w: input %d > cap %d", ErrPromptTooLarge, estInputTokens, c.MaxInputTokens)
	}
	if c.MaxOutputTokens > 0 && estOutputTokens > c.MaxOutputTokens {
		return fmt.Errorf("%w: output %d > cap %d", ErrPromptTooLarge, estOutputTokens, c.MaxOutputTokens)
	}
	if c.MaxTotalTokens > 0 && (estInputTokens+estOutputTokens) > c.MaxTotalTokens {
		return fmt.Errorf("%w: total %d > cap %d",
			ErrPromptTooLarge, estInputTokens+estOutputTokens, c.MaxTotalTokens)
	}
	return nil
}

func envInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return fallback
	}
	return n
}
