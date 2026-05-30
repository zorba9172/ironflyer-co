package appsec

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	Version               string              `json:"version,omitempty"`
	ExcludePaths          []string            `json:"excludePaths,omitempty"`
	DisabledScanners      []string            `json:"disabledScanners,omitempty"`
	EnableNetworkScanners bool                `json:"enableNetworkScanners,omitempty"`
	SeverityOverrides     map[string]Severity `json:"severityOverrides,omitempty"`
	Waivers               []Waiver            `json:"waivers,omitempty"`
}

type Waiver struct {
	RuleID    string    `json:"ruleId"`
	Path      string    `json:"path,omitempty"`
	Reason    string    `json:"reason,omitempty"`
	ExpiresAt time.Time `json:"expiresAt,omitempty"`
}

func ResolveConfig(target Target) Config {
	if target.Config != nil {
		return normaliseConfig(*target.Config)
	}
	for _, f := range target.Files {
		path := strings.ToLower(cleanPath(f.Path))
		if path != ".ironflyer/appsec.json" && path != "appsec.json" {
			continue
		}
		var cfg Config
		if err := json.Unmarshal([]byte(f.Content), &cfg); err == nil {
			return normaliseProjectConfig(cfg)
		}
	}
	return Config{}
}

func normaliseProjectConfig(cfg Config) Config {
	cfg = normaliseConfig(cfg)
	// Project-local files are generated and tenant-editable, so they may
	// tune non-security metadata only. Do not let an app waive, exclude,
	// disable, or downgrade its own scan findings.
	cfg.ExcludePaths = nil
	cfg.DisabledScanners = nil
	cfg.SeverityOverrides = nil
	cfg.Waivers = nil
	return cfg
}

func normaliseConfig(cfg Config) Config {
	for i := range cfg.ExcludePaths {
		cfg.ExcludePaths[i] = cleanPath(cfg.ExcludePaths[i])
	}
	for i := range cfg.DisabledScanners {
		cfg.DisabledScanners[i] = strings.ToLower(strings.TrimSpace(cfg.DisabledScanners[i]))
	}
	if cfg.SeverityOverrides != nil {
		next := map[string]Severity{}
		for rule, sev := range cfg.SeverityOverrides {
			next[strings.ToLower(strings.TrimSpace(rule))] = normaliseSeverity(sev)
		}
		cfg.SeverityOverrides = next
	}
	for i := range cfg.Waivers {
		cfg.Waivers[i].RuleID = strings.ToLower(strings.TrimSpace(cfg.Waivers[i].RuleID))
		cfg.Waivers[i].Path = cleanPath(cfg.Waivers[i].Path)
	}
	return cfg
}

func (cfg Config) ScannerDisabled(id string) bool {
	id = strings.ToLower(strings.TrimSpace(id))
	for _, disabled := range cfg.DisabledScanners {
		if disabled == id {
			return true
		}
	}
	return false
}

func (cfg Config) Apply(findings []Finding, now time.Time) []Finding {
	out := make([]Finding, 0, len(findings))
	for _, f := range findings {
		if cfg.excludesPath(f.Path) || cfg.waives(f, now) {
			continue
		}
		if sev, ok := cfg.SeverityOverrides[strings.ToLower(f.RuleID)]; ok {
			f.Severity = sev
		}
		out = append(out, f)
	}
	return out
}

func (cfg Config) excludesPath(path string) bool {
	path = cleanPath(path)
	for _, pattern := range cfg.ExcludePaths {
		if pathMatches(pattern, path) {
			return true
		}
	}
	return false
}

func (cfg Config) waives(f Finding, now time.Time) bool {
	ruleID := strings.ToLower(strings.TrimSpace(f.RuleID))
	for _, w := range cfg.Waivers {
		if w.RuleID != "" && w.RuleID != ruleID {
			continue
		}
		if w.Path != "" && !pathMatches(w.Path, f.Path) {
			continue
		}
		if !w.ExpiresAt.IsZero() && now.After(w.ExpiresAt) {
			continue
		}
		return true
	}
	return false
}

func pathMatches(pattern, path string) bool {
	pattern = cleanPath(pattern)
	path = cleanPath(path)
	if pattern == "" {
		return false
	}
	if pattern == path {
		return true
	}
	if strings.HasSuffix(pattern, "/") && strings.HasPrefix(path, pattern) {
		return true
	}
	if ok, _ := filepath.Match(pattern, path); ok {
		return true
	}
	if strings.Contains(pattern, "*") {
		if ok, _ := filepath.Match(strings.ReplaceAll(pattern, "**/", ""), path); ok {
			return true
		}
	}
	return strings.HasPrefix(path, strings.TrimSuffix(pattern, "/")+"/")
}

func normaliseSeverity(sev Severity) Severity {
	switch strings.ToLower(strings.TrimSpace(string(sev))) {
	case "critical":
		return SeverityCritical
	case "high":
		return SeverityHigh
	case "medium", "warning", "warn":
		return SeverityMedium
	case "low":
		return SeverityLow
	default:
		return SeverityInfo
	}
}
