package ideaparser

import (
	"context"
	"strings"
	"unicode"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/business/blueprints"
)

// RulesParser is the keyword-only offline fallback. It is always
// safe to call — it requires no provider, no network, and no
// external state. Production deployments wire RulesParser as the
// LLMParser's failure backstop so the studio entrypoint stays
// functional even when every model is unreachable.
type RulesParser struct {
	cfg      Config
	registry blueprints.Registry
	knownIDs map[string]struct{}
	log      zerolog.Logger
}

// NewRulesParser constructs the offline keyword-rules parser.
// registry is required so the parser can validate the rule table
// against the live catalogue at runtime (a typo in the table or a
// retired blueprint id falls back to the default rather than
// breaking the studio entrypoint).
func NewRulesParser(registry blueprints.Registry, cfg Config, log zerolog.Logger) *RulesParser {
	return &RulesParser{
		cfg:      cfg,
		registry: registry,
		knownIDs: catalogIDs(registry),
		log:      log,
	}
}

// rule is one (keyword-set → blueprint, baseline-budget) mapping.
// Order matters: rules earlier in the slice take priority on a hit
// so the SaaS-leaning keywords beat the generic "web app" rule.
type rule struct {
	keywords    []string
	blueprintID string
	budgetUSD   float64
	tag         string
}

// ruleTable is the V22 mapping. Mirrors the agent-spec table; edit
// here and the LLM parser's fallback path picks up the change for
// free.
var ruleTable = []rule{
	// Static / marketing sites — cheapest. Match first so "static
	// landing page" doesn't get pulled into nextjs-production by the
	// "page" / "site" overlap with the default branch.
	{[]string{"static", "landing", "marketing"}, "static-landing", 2.0, "static"},
	// SaaS bundles — auth + billing keywords win over generic
	// "web app" because they're a stricter superset of features.
	{[]string{"saas", "subscription", "stripe", "auth", "billing"}, "nextjs-saas", 5.0, "saas"},
	// Vue tagged before generic React/Next so vue-spa hits when
	// the user is explicit.
	{[]string{"vue"}, "vue-spa", 3.0, "spa"},
	// Mobile.
	{[]string{"mobile", "ios", "android", "expo", "react native"}, "expo-react-native", 4.0, "mobile"},
	// Bot.
	{[]string{"discord", "bot", "chatbot"}, "discord-bot-py", 2.0, "bot"},
	// API-only — pure backend, no UI. python-flask-api is cheaper
	// than go-http-api in the V22 priors so it wins on a generic
	// hit; explicit "go" or "golang" pushes go-http-api.
	{[]string{"go", "golang"}, "go-http-api", 3.0, "api"},
	{[]string{"python", "flask", "fastapi"}, "python-flask-api", 3.0, "api"},
	{[]string{"api", "backend", "rest", "graphql server", "microservice"}, "python-flask-api", 3.0, "api"},
	// Generic webapp — last so the more specific rules win.
	{[]string{"next", "react", "web app", "webapp", "dashboard", "crm", "tool"}, "nextjs-production", 4.0, "webapp"},
}

// Parse picks the first matching rule (in declaration order) and
// constructs an Idea around it. When no rule fires the parser
// falls back to static-landing with the configured min-budget —
// the cheapest possible blueprint so the studio always returns
// something runnable.
func (p *RulesParser) Parse(ctx context.Context, in ParseInput) (Idea, error) {
	if strings.TrimSpace(in.Text) == "" {
		return Idea{}, ErrEmptyText
	}
	if in.MaxBudgetUSD.LessThanOrEqual(decimal.Zero) {
		return Idea{}, ErrNoBudget
	}
	if len(p.knownIDs) == 0 {
		return Idea{}, ErrNoBlueprints
	}

	text := strings.ToLower(in.Text)

	pick := rule{
		keywords:    nil,
		blueprintID: "static-landing",
		budgetUSD:   p.cfg.MinBudgetUSD,
		tag:         "static",
	}
	confidence := 0.35 // default branch — low confidence
	matchedKeyword := ""

	for _, r := range ruleTable {
		if _, ok := p.knownIDs[r.blueprintID]; !ok {
			continue // rule references a blueprint that isn't registered
		}
		for _, kw := range r.keywords {
			if containsWord(text, kw) {
				pick = r
				confidence = 0.65
				matchedKeyword = kw
				break
			}
		}
		if matchedKeyword != "" {
			break
		}
	}

	// Clamp the budget to the wallet ceiling and the configured min.
	suggested := decimal.NewFromFloat(pick.budgetUSD)
	minBudget := decimal.NewFromFloat(p.cfg.MinBudgetUSD)
	if suggested.LessThan(minBudget) {
		suggested = minBudget
	}
	if suggested.GreaterThan(in.MaxBudgetUSD) {
		suggested = in.MaxBudgetUSD
	}
	stopLoss := suggested.Mul(decimal.NewFromFloat(p.cfg.StopLossMultiplier))
	if stopLoss.GreaterThan(in.MaxBudgetUSD) {
		stopLoss = in.MaxBudgetUSD
	}
	if stopLoss.LessThan(suggested) {
		stopLoss = suggested
	}

	title := deriveTitle(in.Text, pick.blueprintID)
	summary := deriveSummary(in.Text)
	reason := "keyword-rules: " + pick.blueprintID
	if matchedKeyword != "" {
		reason = "keyword '" + matchedKeyword + "' → " + pick.blueprintID
	}

	tags := []string{pick.tag}
	if c, ok := p.registry.Get(pick.blueprintID); ok && c.Category != "" && c.Category != pick.tag {
		tags = append(tags, c.Category)
	}

	return Idea{
		Title:           title,
		Summary:         summary,
		BlueprintID:     pick.blueprintID,
		BlueprintReason: reason,
		SuggestedBudget: suggested,
		Tags:            tags,
		StopLossUSD:     stopLoss,
		Confidence:      confidence,
		PromptSummary:   summary,
	}, nil
}

// containsWord returns true when needle appears in haystack as a
// whole-word match. Used so "go" doesn't match "google" and "api"
// doesn't match "rapidly".
func containsWord(haystack, needle string) bool {
	idx := 0
	for {
		off := strings.Index(haystack[idx:], needle)
		if off < 0 {
			return false
		}
		start := idx + off
		end := start + len(needle)
		leftOK := start == 0 || !isWordRune(rune(haystack[start-1]))
		rightOK := end == len(haystack) || !isWordRune(rune(haystack[end]))
		if leftOK && rightOK {
			return true
		}
		idx = end
	}
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// deriveTitle returns a short project label from the user's text.
// Strategy: take the first 6 words (clipped, no punctuation) — the
// LLM path overrides this with a proper title; the rules path's
// goal is "good enough that the project list isn't a wall of
// 'Untitled'".
func deriveTitle(text, blueprintID string) string {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return strings.Title(strings.ReplaceAll(blueprintID, "-", " "))
	}
	fields := strings.Fields(clean)
	if len(fields) > 6 {
		fields = fields[:6]
	}
	joined := strings.Join(fields, " ")
	joined = strings.TrimRight(joined, ".,!?:;")
	if joined == "" {
		return strings.Title(strings.ReplaceAll(blueprintID, "-", " "))
	}
	// First letter capital so the project list renders cleanly.
	runes := []rune(joined)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// deriveSummary returns the user's text trimmed to the first
// sentence (period / newline) or 240 chars, whichever is shorter.
func deriveSummary(text string) string {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return ""
	}
	// First sentence boundary.
	if idx := strings.IndexAny(clean, ".!?\n"); idx > 0 {
		clean = clean[:idx+1]
	}
	if len(clean) > 240 {
		clean = clean[:237] + "..."
	}
	return clean
}
