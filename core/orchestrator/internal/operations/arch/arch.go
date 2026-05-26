// Package arch reads the Ironflyer Architecture Manifest
// (`.ironflyer/architecture.json`) and enforces layering rules on import
// paths. It is the structural backbone of the Anti-Bloat Engine
// (docs/ANTI_BLOAT_ENGINE.md, playbook §8.9): every patch's affected
// packages are validated against the manifest before apply.
//
// The manifest declares:
//
//   - layers: the canonical layer names (domain, business, ai,
//     operations, customer, suppliers, interface, …).
//   - rules:  per-pair allow/deny entries. A wildcard "*" in `to`
//     denies (or allows) every other layer.
//   - cycles: "deny" today; future "warn" is intentionally distinct.
//   - owners: human-readable ownership map, used by dashboards and
//     repair hints but not by Validate itself.
//
// This package does no IO beyond reading the manifest file once. The
// `dep_graph` / `arch_boundary` gates call Validate with the package
// path and import path extracted from a patch's affected paths.
package arch

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Manifest is the parsed `.ironflyer/architecture.json`.
type Manifest struct {
	Layers []string            `json:"layers"`
	Rules  []Rule              `json:"rules"`
	Cycles string              `json:"cycles"`
	Owners map[string][]string `json:"owners"`
}

// Rule is one from→to allow/deny edge. `To == "*"` matches every layer
// other than `From` and is used to set a default (typically deny).
type Rule struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Allow bool   `json:"allow"`
}

// Load reads a manifest from disk. An empty path defaults to
// `.ironflyer/architecture.json` relative to the current working
// directory. The returned Manifest is usable when err == nil; on a
// missing file the caller gets a zero Manifest + a wrapped fs error so
// startup can decide whether to fail-loud or warn-degrade.
func Load(path string) (Manifest, error) {
	if strings.TrimSpace(path) == "" {
		path = ".ironflyer/architecture.json"
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("arch: read manifest %q: %w", path, err)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Manifest{}, fmt.Errorf("arch: parse manifest %q: %w", path, err)
	}
	if err := m.validateShape(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// LoadFromBytes parses an in-memory manifest. Used by callers (e.g.
// tests, embedded fallbacks) that already hold the JSON bytes.
func LoadFromBytes(raw []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Manifest{}, fmt.Errorf("arch: parse manifest: %w", err)
	}
	if err := m.validateShape(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// validateShape sanity-checks a freshly-parsed Manifest. It does NOT
// run any import-graph analysis — that happens in Validate.
func (m Manifest) validateShape() error {
	if len(m.Layers) == 0 {
		return fmt.Errorf("arch: manifest has no layers")
	}
	known := make(map[string]struct{}, len(m.Layers))
	for _, l := range m.Layers {
		known[l] = struct{}{}
	}
	for i, r := range m.Rules {
		if r.From == "" {
			return fmt.Errorf("arch: rules[%d].from empty", i)
		}
		if _, ok := known[r.From]; !ok {
			return fmt.Errorf("arch: rules[%d].from %q not declared in layers", i, r.From)
		}
		if r.To != "*" {
			if _, ok := known[r.To]; !ok {
				return fmt.Errorf("arch: rules[%d].to %q not declared in layers", i, r.To)
			}
		}
	}
	return nil
}

// LayerOf returns the manifest layer name that owns importPath, or "".
// importPath is a Go import path (e.g.
// "ironflyer/core/orchestrator/internal/business/ledger"); the helper
// extracts the layer segment that sits between `internal/` and the
// next slash. Paths that don't match the convention return "".
func (m Manifest) LayerOf(importPath string) string {
	const marker = "/internal/"
	i := strings.Index(importPath, marker)
	if i < 0 {
		return ""
	}
	rest := importPath[i+len(marker):]
	j := strings.Index(rest, "/")
	if j < 0 {
		j = len(rest)
	}
	candidate := rest[:j]
	for _, l := range m.Layers {
		if l == candidate {
			return l
		}
	}
	return ""
}

// Validate reports whether a package at pkgPath may import importPath
// under the manifest's rules. Both arguments are Go import paths. A
// nil error means the edge is allowed (or one of the endpoints lies
// outside the manifest's surface, e.g. stdlib / third-party). A non-nil
// error names the rule that blocks the edge.
//
// The rule lookup order:
//
//  1. Both endpoints map to layers; same-layer edges are always allowed.
//  2. An explicit { from, to } rule wins, allow or deny.
//  3. A wildcard { from, to: "*" } rule provides the default for that
//     layer.
//  4. With no matching rule the edge is allowed (open by default —
//     operators add rules to tighten).
func (m Manifest) Validate(pkgPath, importPath string) error {
	from := m.LayerOf(pkgPath)
	to := m.LayerOf(importPath)
	if from == "" || to == "" {
		// One side is outside the manifest — e.g. stdlib or a third
		// party module. Treat as allowed; the manifest only governs
		// edges among declared layers.
		return nil
	}
	if from == to {
		return nil
	}
	// Explicit rule first.
	for _, r := range m.Rules {
		if r.From == from && r.To == to {
			if r.Allow {
				return nil
			}
			return fmt.Errorf("arch: layering violation: %s → %s denied by manifest rule (%s → %s)", pkgPath, importPath, from, to)
		}
	}
	// Wildcard default for the From layer.
	for _, r := range m.Rules {
		if r.From == from && r.To == "*" {
			if r.Allow {
				return nil
			}
			return fmt.Errorf("arch: layering violation: %s → %s denied by wildcard rule (%s → *)", pkgPath, importPath, from)
		}
	}
	return nil
}

// OwnersFor returns the manifest "owners" map entries whose paths
// overlap pkgPath. The paths in the manifest are layer-relative
// (e.g. "business/wallet"); a match is reported when the pkgPath
// contains the entry as a substring. Useful for dashboard
// human-readable annotations.
func (m Manifest) OwnersFor(pkgPath string) []string {
	out := make([]string, 0)
	for name, paths := range m.Owners {
		for _, p := range paths {
			if strings.Contains(pkgPath, "/"+p) || strings.HasSuffix(pkgPath, "/"+p) {
				out = append(out, name)
				break
			}
		}
	}
	return out
}
