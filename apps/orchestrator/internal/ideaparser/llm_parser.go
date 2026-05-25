package ideaparser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"ironflyer/apps/orchestrator/internal/blueprints"
	"ironflyer/apps/orchestrator/internal/providers"
)

// LLMParser routes the parse call to the cheap-tier provider chain
// via providers.Router. The model is instructed to pick exactly one
// blueprint from the catalogue and return a structured JSON Idea;
// any failure (network, parse, validation) falls back to RulesParser
// so the studio entrypoint never returns a 500.
type LLMParser struct {
	router   *providers.Router
	registry blueprints.Registry
	cfg      Config
	log      zerolog.Logger
	knownIDs map[string]struct{}
	catalog  string
	fallback *RulesParser
}

// NewLLMParser wires the production parser. router is required —
// without it the studio entrypoint cannot reason about the spec; the
// wireup helper should pick RulesParser instead when no provider is
// available.
func NewLLMParser(router *providers.Router, registry blueprints.Registry, cfg Config, log zerolog.Logger) *LLMParser {
	return &LLMParser{
		router:   router,
		registry: registry,
		cfg:      cfg,
		log:      log,
		knownIDs: catalogIDs(registry),
		catalog:  catalogSummary(registry),
		fallback: NewRulesParser(registry, cfg, log),
	}
}

// systemPromptTemplate is the exact system prompt sent to the model.
// %s is replaced with catalogSummary(registry). Kept in one place so
// the agent-spec report can quote it verbatim.
const systemPromptTemplate = `You are Ironflyer's studio intake. Your only job is to choose ONE blueprint from the catalogue below for a paid AI execution and return a single JSON object describing the plan.

CATALOGUE (pick exactly one blueprint_id from these ids):
%s

INSTRUCTIONS
1. Read the user's idea.
2. Choose the cheapest blueprint that can plausibly satisfy it. Prefer static-landing for marketing pages, nextjs-saas only when the spec mentions auth / billing / subscriptions, expo-react-native only when the spec is explicitly mobile.
3. Write a short title (3-6 words), a 1-2 sentence summary, a one-sentence blueprint_reason, and 3-6 lowercase tags.
4. suggested_budget_usd MUST be a number between 1 and the max_budget_usd value the caller provides. Keep it close to the blueprint's cost_prior_usd; only bid higher when the spec is unusually large.
5. stop_loss_usd MUST be >= suggested_budget_usd and <= max_budget_usd.
6. confidence is your self-rated 0..1 score for the blueprint choice.

OUTPUT
Return ONLY a single JSON object, no markdown, no commentary. Schema:
{
  "title": string,
  "summary": string,
  "blueprint_id": string,         // MUST be one of the catalogue ids above
  "blueprint_reason": string,
  "suggested_budget_usd": number,
  "stop_loss_usd": number,
  "tags": [string],
  "confidence": number,           // 0..1
  "prompt_summary": string        // distilled spec for the finisher
}`

// llmIdea mirrors the JSON schema in systemPromptTemplate. Decoded
// from the model's response, then validated and clamped before
// being projected into a domain Idea.
type llmIdea struct {
	Title              string   `json:"title"`
	Summary            string   `json:"summary"`
	BlueprintID        string   `json:"blueprint_id"`
	BlueprintReason    string   `json:"blueprint_reason"`
	SuggestedBudgetUSD float64  `json:"suggested_budget_usd"`
	StopLossUSD        float64  `json:"stop_loss_usd"`
	Tags               []string `json:"tags"`
	Confidence         float64  `json:"confidence"`
	PromptSummary      string   `json:"prompt_summary"`
}

// Parse runs the LLM picker. On any non-validation failure the
// rules fallback runs and its result is returned; the caller cannot
// distinguish the two paths (by design — the studio entrypoint
// degrades gracefully without surfacing provider outages to the
// user).
func (p *LLMParser) Parse(ctx context.Context, in ParseInput) (Idea, error) {
	if strings.TrimSpace(in.Text) == "" {
		return Idea{}, ErrEmptyText
	}
	if in.MaxBudgetUSD.LessThanOrEqual(decimal.Zero) {
		return Idea{}, ErrNoBudget
	}
	if len(p.knownIDs) == 0 {
		return Idea{}, ErrNoBlueprints
	}

	idea, err := p.parseViaLLM(ctx, in)
	if err == nil {
		return idea, nil
	}
	p.log.Warn().Err(err).Str("tenant_id", in.TenantID).Msg("ideaparser: llm path failed; falling back to rules")
	return p.fallback.Parse(ctx, in)
}

// parseViaLLM does one cheap-tier completion + JSON decode + clamp.
// Any error here aborts the LLM path; Parse() falls back to rules.
func (p *LLMParser) parseViaLLM(ctx context.Context, in ParseInput) (Idea, error) {
	if p.router == nil {
		return Idea{}, errors.New("ideaparser: no provider router wired")
	}

	system := fmt.Sprintf(systemPromptTemplate, p.catalog)
	prompt := fmt.Sprintf("USER IDEA:\n%s\n\nmax_budget_usd = %s\n\nReturn ONLY the JSON object.", strings.TrimSpace(in.Text), in.MaxBudgetUSD.String())

	req := providers.Request{
		System: system,
		Prompt: prompt,
		Capabilities: []providers.Capability{
			providers.CapCheap,
			providers.CapJSON,
			providers.CapFast,
		},
		MaxTokens:   p.cfg.MaxOutputTokens,
		Temperature: p.cfg.Temperature,
		TenantID:    in.TenantID,
	}

	resp, err := p.router.Complete(ctx, req)
	if err != nil {
		return Idea{}, fmt.Errorf("%w: %v", ErrLLMResponse, err)
	}

	raw := extractJSONObject(resp.Text)
	if raw == "" {
		return Idea{}, fmt.Errorf("%w: no JSON object in response", ErrLLMResponse)
	}
	var dec llmIdea
	if err := json.Unmarshal([]byte(raw), &dec); err != nil {
		return Idea{}, fmt.Errorf("%w: %v", ErrLLMResponse, err)
	}

	return p.validateAndClamp(in, dec)
}

// validateAndClamp converts the decoded llmIdea into a domain
// Idea, normalising fields and clamping money against the wallet
// ceiling + config floor. Returns ErrLLMResponse when the model
// chose a blueprint id outside the catalogue (so the caller falls
// back to rules rather than admitting an unknown blueprint).
func (p *LLMParser) validateAndClamp(in ParseInput, raw llmIdea) (Idea, error) {
	bid := strings.TrimSpace(raw.BlueprintID)
	if _, ok := p.knownIDs[bid]; !ok {
		return Idea{}, fmt.Errorf("%w: unknown blueprint_id %q", ErrLLMResponse, bid)
	}

	suggested := decimal.NewFromFloat(raw.SuggestedBudgetUSD)
	minBudget := decimal.NewFromFloat(p.cfg.MinBudgetUSD)
	if suggested.LessThan(minBudget) {
		suggested = minBudget
	}
	if suggested.GreaterThan(in.MaxBudgetUSD) {
		suggested = in.MaxBudgetUSD
	}
	if suggested.LessThanOrEqual(decimal.Zero) {
		return Idea{}, fmt.Errorf("%w: suggested budget non-positive", ErrLLMResponse)
	}

	stop := decimal.NewFromFloat(raw.StopLossUSD)
	mult := suggested.Mul(decimal.NewFromFloat(p.cfg.StopLossMultiplier))
	if stop.LessThan(mult) {
		stop = mult
	}
	if stop.GreaterThan(in.MaxBudgetUSD) {
		stop = in.MaxBudgetUSD
	}
	if stop.LessThan(suggested) {
		stop = suggested
	}

	conf := raw.Confidence
	if conf < 0 {
		conf = 0
	}
	if conf > 1 {
		conf = 1
	}

	title := strings.TrimSpace(raw.Title)
	if title == "" {
		title = deriveTitle(in.Text, bid)
	}
	summary := strings.TrimSpace(raw.Summary)
	if summary == "" {
		summary = deriveSummary(in.Text)
	}
	promptSummary := strings.TrimSpace(raw.PromptSummary)
	if promptSummary == "" {
		promptSummary = summary
	}

	tags := make([]string, 0, len(raw.Tags))
	seen := map[string]struct{}{}
	for _, t := range raw.Tags {
		t = strings.TrimSpace(strings.ToLower(t))
		if t == "" {
			continue
		}
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		tags = append(tags, t)
	}
	if len(tags) == 0 {
		if bp, ok := p.registry.Get(bid); ok && bp.Category != "" {
			tags = []string{bp.Category}
		}
	}

	return Idea{
		Title:           title,
		Summary:         summary,
		BlueprintID:     bid,
		BlueprintReason: strings.TrimSpace(raw.BlueprintReason),
		SuggestedBudget: suggested,
		Tags:            tags,
		StopLossUSD:     stop,
		Confidence:      conf,
		PromptSummary:   promptSummary,
	}, nil
}

// extractJSONObject pulls the first balanced { ... } object out of
// the model's text. Tolerates leading markdown fences ("```json")
// and trailing commentary that some models emit despite the
// "return ONLY JSON" instruction.
func extractJSONObject(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Strip a leading ``` or ```json fence if present.
	if strings.HasPrefix(s, "```") {
		if idx := strings.Index(s, "\n"); idx >= 0 {
			s = s[idx+1:]
		}
		if end := strings.LastIndex(s, "```"); end >= 0 {
			s = s[:end]
		}
		s = strings.TrimSpace(s)
	}
	start := strings.Index(s, "{")
	if start < 0 {
		return ""
	}
	depth := 0
	inStr := false
	escape := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			if escape {
				escape = false
				continue
			}
			switch c {
			case '\\':
				escape = true
			case '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}
