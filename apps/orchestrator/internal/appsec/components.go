package appsec

import (
	"encoding/json"
	"strings"
)

func parseGoModComponents(path, body string) []Component {
	var out []Component
	inRequireBlock := false
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "require (") {
			inRequireBlock = true
			continue
		}
		if inRequireBlock && line == ")" {
			inRequireBlock = false
			continue
		}
		if !inRequireBlock {
			if !strings.HasPrefix(line, "require ") {
				continue
			}
			line = strings.TrimSpace(strings.TrimPrefix(line, "require "))
		}
		line = strings.Split(line, "//")[0]
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		out = append(out, Component{
			Ecosystem: "go",
			Name:      fields[0],
			Version:   fields[1],
			Path:      cleanPath(path),
			Dev:       strings.Contains(raw, "// indirect"),
		})
	}
	return out
}

func parsePackageLockComponents(path, body string) []Component {
	var doc struct {
		LockfileVersion int `json:"lockfileVersion"`
		Packages        map[string]struct {
			Version    string `json:"version"`
			Dev        bool   `json:"dev,omitempty"`
			Deprecated string `json:"deprecated,omitempty"`
		} `json:"packages"`
		Dependencies map[string]struct {
			Version    string `json:"version"`
			Dev        bool   `json:"dev,omitempty"`
			Deprecated string `json:"deprecated,omitempty"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []Component
	for pkgPath, pkg := range doc.Packages {
		if !strings.HasPrefix(pkgPath, "node_modules/") || pkg.Version == "" {
			continue
		}
		name := strings.TrimPrefix(pkgPath, "node_modules/")
		key := name + "@" + pkg.Version
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Component{
			Ecosystem:  "npm",
			Name:       name,
			Version:    pkg.Version,
			Path:       cleanPath(path),
			Dev:        pkg.Dev,
			Deprecated: strings.TrimSpace(pkg.Deprecated),
		})
	}
	for name, dep := range doc.Dependencies {
		if dep.Version == "" {
			continue
		}
		key := name + "@" + dep.Version
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Component{
			Ecosystem:  "npm",
			Name:       name,
			Version:    dep.Version,
			Path:       cleanPath(path),
			Dev:        dep.Dev,
			Deprecated: strings.TrimSpace(dep.Deprecated),
		})
	}
	return out
}
