package resolver

import "strings"

// normalizePlatformList trims, lowercases, dedupes, and orders the
// requested platform list. An empty input is treated as "all three"
// per the schema contract documented in assets.graphql.
//
// Kept in a non-resolver file so `go run github.com/99designs/gqlgen
// generate` does not move the helper into the resolver's
// "out-of-harm's-way" trailing comment block.
func normalizePlatformList(in []string) []string {
	all := []string{"android", "ios", "expo"}
	if len(in) == 0 {
		return all
	}
	seen := map[string]struct{}{}
	out := []string{}
	for _, p := range in {
		k := strings.ToLower(strings.TrimSpace(p))
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	if len(out) == 0 {
		return all
	}
	return out
}
