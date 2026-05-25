package ideaparser

import "errors"

// ErrEmptyText is returned when ParseInput.Text is empty or
// whitespace-only. Resolvers map this to a typed GraphQL error
// (extension {"code":"INVALID_INPUT"}) so the form can highlight
// the prompt field.
var ErrEmptyText = errors.New("ideaparser: empty text")

// ErrNoBudget is returned when ParseInput.MaxBudgetUSD is <= 0.
// The studio resolver should never reach the parser without
// fetching the wallet first; this error guards against bypass.
var ErrNoBudget = errors.New("ideaparser: no budget")

// ErrNoBlueprints is returned when the registry returned an empty
// list. Should be impossible in production (the built-in registry
// always carries the 8 V22 blueprints) but defensible for tests
// that wire a custom registry.
var ErrNoBlueprints = errors.New("ideaparser: no blueprints registered")

// ErrLLMResponse is returned when the LLM either errored out or
// returned a payload we could not parse. The LLM parser converts
// this into a fallback-to-rules path; callers only see it when
// the rules fallback itself fails.
var ErrLLMResponse = errors.New("ideaparser: llm response invalid")
