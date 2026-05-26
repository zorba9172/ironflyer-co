// Idea parser wireup — V22 studio entrypoint.
//
// BuildIdeaParser chooses between the LLM-backed parser and the
// keyword-rules fallback based on the env-driven Config and whether
// a provider router is wired. main.go assigns the result onto
// resolver.Resolver.IdeaParser (added by the integration agent).
package wireup

import (
	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/business/blueprints"
	"ironflyer/core/orchestrator/internal/ai/ideaparser"
	"ironflyer/core/orchestrator/internal/ai/providers"
)

// BuildIdeaParser returns the configured ideaparser.Parser. When
// IRONFLYER_IDEAPARSER_LLM is true (the default) AND a non-nil
// router is supplied, the LLM-backed parser is wired with the
// rules parser as its internal fallback. Otherwise the rules
// parser is returned directly so the studio entrypoint always
// works on dev / air-gapped boots.
func BuildIdeaParser(router *providers.Router, registry blueprints.Registry, log zerolog.Logger) ideaparser.Parser {
	cfg := ideaparser.LoadConfig()
	if cfg.LLMEnabled && router != nil {
		log.Info().
			Bool("llm", true).
			Int("max_output_tokens", cfg.MaxOutputTokens).
			Msg("ideaparser: llm backend wired")
		return ideaparser.NewLLMParser(router, registry, cfg, log)
	}
	log.Info().
		Bool("llm", false).
		Msg("ideaparser: rules backend wired (no router or LLM disabled)")
	return ideaparser.NewRulesParser(registry, cfg, log)
}
