package ideaparser

import (
	"fmt"
	"sort"
	"strings"

	"ironflyer/apps/orchestrator/internal/blueprints"
)

// catalogSummary returns the blueprint catalogue rendered as a
// compact, model-friendly markdown block. The LLM picker consumes
// this as part of its system prompt; the rules parser uses
// catalogIDs to validate the keyword table against the registry at
// startup.
//
// Format (one row per blueprint, separated by blank lines):
//
//	- id: nextjs-mvp
//	  name: Next.js 15 MVP
//	  category: webapp
//	  cost_prior_usd: 0.85
//	  supported_gates: scaffold,build,typecheck
//	  description: <one paragraph>
func catalogSummary(reg blueprints.Registry) string {
	bps := reg.List()
	sort.SliceStable(bps, func(i, j int) bool { return bps[i].ID < bps[j].ID })
	var b strings.Builder
	for i, bp := range bps {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "- id: %s\n", bp.ID)
		fmt.Fprintf(&b, "  name: %s\n", bp.Name)
		fmt.Fprintf(&b, "  category: %s\n", bp.Category)
		fmt.Fprintf(&b, "  cost_prior_usd: %s\n", bp.CostPriorUSD.String())
		fmt.Fprintf(&b, "  supported_gates: %s\n", strings.Join(bp.SupportedGates, ","))
		fmt.Fprintf(&b, "  description: %s\n", strings.TrimSpace(bp.Description))
	}
	return b.String()
}

// catalogIDs returns the set of blueprint ids known to the registry,
// for O(1) membership checks. Used by both backends to clamp model
// hallucinations and rules-table typos to a valid id.
func catalogIDs(reg blueprints.Registry) map[string]struct{} {
	bps := reg.List()
	out := make(map[string]struct{}, len(bps))
	for _, bp := range bps {
		out[bp.ID] = struct{}{}
	}
	return out
}
