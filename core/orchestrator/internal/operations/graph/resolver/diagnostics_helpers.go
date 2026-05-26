package resolver

import "ironflyer/core/orchestrator/internal/operations/graph/model"

// toJSONFields converts a free-form map into the model JSON scalar
// shape. Kept out of diagnostics.resolver.go so gqlgen's resolver
// regeneration doesn't comment it out as "unknown code".
func toJSONFields(in map[string]any) model.JSON {
	if in == nil {
		return model.JSON{}
	}
	out := make(model.JSON, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
