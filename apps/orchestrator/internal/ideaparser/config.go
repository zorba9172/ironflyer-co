package ideaparser

import (
	"os"
	"strconv"
	"strings"
)

// Config tunes both parser backends. LoadConfig reads the env once
// at construction time so the wireup helper can decide which backend
// to wire. Every field has a defensible default so a zero-config
// boot still works.
type Config struct {
	// LLMEnabled chooses between LLMParser (true) and RulesParser
	// (false). Defaults to true so production picks the smarter
	// path; flip via IRONFLYER_IDEAPARSER_LLM=false to force the
	// offline fallback.
	LLMEnabled bool

	// MaxOutputTokens caps the LLM response length so a runaway
	// model does not balloon the cheap-tier call into a quality-tier
	// invoice. 600 tokens covers a verbose Idea JSON with margin.
	MaxOutputTokens int

	// Temperature gives the model just enough wiggle room to be
	// useful on ambiguous prompts without turning the picker
	// non-deterministic across reloads. 0.2 is the V22 cheap-call
	// default.
	Temperature float32

	// MinBudgetUSD is the floor the parser refuses to go below
	// regardless of what the model recommends. Below this number
	// the wallet hold is too small to cover even the cheapest
	// blueprint's prior cost.
	MinBudgetUSD float64

	// StopLossMultiplier scales the suggested budget into the
	// stop-loss ceiling. Defaults to 1.5 — the execution gets a
	// 50% safety margin before ProfitGuard hard-kills it.
	StopLossMultiplier float64
}

// DefaultConfig returns the V22 defaults — LLM enabled, 600 token
// cap, 0.2 temperature, $1 floor, 1.5x stop-loss.
func DefaultConfig() Config {
	return Config{
		LLMEnabled:         true,
		MaxOutputTokens:    600,
		Temperature:        0.2,
		MinBudgetUSD:       1.0,
		StopLossMultiplier: 1.5,
	}
}

// LoadConfig is DefaultConfig overlaid with the four
// IRONFLYER_IDEAPARSER_* environment variables. Unrecognised values
// fall back to the default so the parser never refuses to boot.
func LoadConfig() Config {
	cfg := DefaultConfig()
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_IDEAPARSER_LLM")); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			cfg.LLMEnabled = true
		case "0", "false", "no", "off":
			cfg.LLMEnabled = false
		}
	}
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_IDEAPARSER_MAX_TOKENS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxOutputTokens = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_IDEAPARSER_TEMPERATURE")); v != "" {
		if f, err := strconv.ParseFloat(v, 32); err == nil && f >= 0 {
			cfg.Temperature = float32(f)
		}
	}
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_IDEAPARSER_MIN_BUDGET_USD")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			cfg.MinBudgetUSD = f
		}
	}
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_IDEAPARSER_STOP_LOSS_MULT")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 1 {
			cfg.StopLossMultiplier = f
		}
	}
	return cfg
}
