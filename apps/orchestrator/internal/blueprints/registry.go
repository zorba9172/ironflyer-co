package blueprints

import (
	"sort"
	"strings"
)

// Registry is the read surface every caller (ProfitGuard, finisher,
// GraphQL resolvers, dashboards) talks to when it needs a blueprint
// by id, the full catalogue, or a ranked recommendation list.
//
// The built-in implementation returns a fixed slice loaded once at
// startup from blueprints_data.go; the embedded templates/ tree is
// the source of truth for file content. Tests / dev tooling that
// want to inject extra blueprints can implement Registry themselves.
type Registry interface {
	// List returns every registered blueprint, in stable (registry-
	// defined) order. Callers that want their own ordering should
	// sort the returned slice.
	List() []Blueprint
	// Get returns the blueprint with the given id, or (zero, false)
	// if no such blueprint is registered. The found flag is the
	// canonical "missing" signal — never compare against zero IDs.
	Get(id string) (Blueprint, bool)
	// Recommend returns the blueprints that best match the supplied
	// tags, filtered to those whose CostPriorUSD is at or below
	// riskCeiling. riskCeiling <= 0 means "no ceiling". Tags are
	// matched case-insensitively against blueprint Category + Name
	// + ID; an empty tags slice returns the catalogue sorted by
	// CostPriorUSD ascending (cheapest first).
	Recommend(tags []string, riskCeiling float64) []Blueprint
}

// builtInRegistry is the package-internal Registry implementation. It
// holds the slice produced by builtInBlueprints() (constructed once
// at package init) and indexes by id for O(1) Get.
type builtInRegistry struct {
	all   []Blueprint
	byID  map[string]Blueprint
}

// NewBuiltInRegistry returns the canonical Registry for V22 — the
// three v1 blueprints embedded in this package. The slice is
// recomputed on every call so a caller holding the previous one is
// not affected by mutations.
func NewBuiltInRegistry() Registry {
	all := builtInBlueprints()
	byID := make(map[string]Blueprint, len(all))
	for _, b := range all {
		byID[b.ID] = b
	}
	return &builtInRegistry{all: all, byID: byID}
}

// List returns a copy of the registered slice so callers cannot
// reorder the package's internal state.
func (r *builtInRegistry) List() []Blueprint {
	out := make([]Blueprint, len(r.all))
	copy(out, r.all)
	return out
}

// Get is O(1) via the id index built in NewBuiltInRegistry.
func (r *builtInRegistry) Get(id string) (Blueprint, bool) {
	b, ok := r.byID[id]
	return b, ok
}

// Recommend filters by riskCeiling, scores against tags, and returns
// the matched blueprints highest-score first (ties broken by the
// cheaper CostPriorUSD).
func (r *builtInRegistry) Recommend(tags []string, riskCeiling float64) []Blueprint {
	// Normalize tags once.
	norm := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(strings.ToLower(t))
		if t != "" {
			norm = append(norm, t)
		}
	}

	type scored struct {
		b     Blueprint
		score int
	}
	candidates := make([]scored, 0, len(r.all))
	for _, b := range r.all {
		if riskCeiling > 0 && b.CostPriorUSD.InexactFloat64() > riskCeiling {
			continue
		}
		s := 0
		hay := strings.ToLower(b.ID + " " + b.Name + " " + b.Category)
		for _, t := range norm {
			if strings.Contains(hay, t) {
				s++
			}
		}
		candidates = append(candidates, scored{b: b, score: s})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].b.CostPriorUSD.LessThan(candidates[j].b.CostPriorUSD)
	})

	out := make([]Blueprint, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, c.b)
	}
	return out
}
