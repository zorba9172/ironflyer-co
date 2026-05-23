package agents

import (
	_ "embed"
	"fmt"

	"gopkg.in/yaml.v3"

	"ironflyer/apps/orchestrator/internal/providers"
)

//go:embed agents.yaml
var defaultAgentsYAML []byte

// agentSpec is the YAML row shape — kept private since callers only see
// the parsed Agent values via the Registry.
type agentSpec struct {
	Role           string   `yaml:"role"`
	System         string   `yaml:"system"`
	Capabilities   []string `yaml:"capabilities"`
	EnableThinking bool     `yaml:"enableThinking"`
}

type agentFile struct {
	Agents []agentSpec `yaml:"agents"`
}

// LoadDefaults parses the embedded agents.yaml into Agent values. Replaces
// the hand-written RegisterDefaults so prompts can be tuned by editing YAML
// without recompiling.
func LoadDefaults() ([]Agent, error) {
	return parseAgents(defaultAgentsYAML)
}

// LoadFromBytes parses an arbitrary agents.yaml-shaped payload. Exposed for
// tests and for callers that want to ship operator-managed overrides.
func LoadFromBytes(raw []byte) ([]Agent, error) {
	return parseAgents(raw)
}

func parseAgents(raw []byte) ([]Agent, error) {
	var f agentFile
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("agents yaml: %w", err)
	}
	if len(f.Agents) == 0 {
		return nil, fmt.Errorf("agents yaml: no entries")
	}
	out := make([]Agent, 0, len(f.Agents))
	for i, s := range f.Agents {
		if s.Role == "" {
			return nil, fmt.Errorf("agents yaml[%d]: role required", i)
		}
		caps, err := mapCapabilities(s.Capabilities)
		if err != nil {
			return nil, fmt.Errorf("agents yaml[%s]: %w", s.Role, err)
		}
		out = append(out, Agent{
			Role:           Role(s.Role),
			System:         s.System,
			Capabilities:   caps,
			EnableThinking: s.EnableThinking,
		})
	}
	return out, nil
}

// mapCapabilities translates the YAML string tags into the typed Capability
// constants the provider router consumes. Unknown tags are an error so a
// typo in the YAML doesn't silently disable routing hints.
func mapCapabilities(tags []string) ([]providers.Capability, error) {
	known := map[string]providers.Capability{
		"reasoning": providers.CapReasoning,
		"code":      providers.CapCode,
		"json":      providers.CapJSON,
		"vision":    providers.CapVision,
		"cheap":     providers.CapCheap,
		"fast":      providers.CapFast,
		"private":   providers.CapPrivate,
		"thinking":  providers.CapThinking,
		"tools":     providers.CapTools,
		"cache":     providers.CapCache,
		// speculative: race the top-two matching providers; first to
		// emit a token wins, loser is cancelled. See providers.CapSpeculative.
		"speculative": providers.CapSpeculative,
	}
	out := make([]providers.Capability, 0, len(tags))
	for _, t := range tags {
		c, ok := known[t]
		if !ok {
			return nil, fmt.Errorf("unknown capability %q", t)
		}
		out = append(out, c)
	}
	return out, nil
}
