package policy

import (
	"os"
	"strings"
)

// Mode selects which PDP implementation backs the PEP.
type Mode string

const (
	// ModeLocal evaluates Rego bundles in-process via OPA's Go SDK.
	// Default when policy is enabled.
	ModeLocal Mode = "local"
	// ModeRemote calls an OPA sidecar over HTTP.
	ModeRemote Mode = "remote"
	// ModeDisabled installs a stub PDP that always allows. Used only
	// in greenfield development; production refuses to boot in this
	// mode unless IRONFLYER_OPA_ALLOW_DISABLED=1 is set.
	ModeDisabled Mode = "disabled"
)

// Config captures every environment-tunable knob the policy plane
// exposes. Loaded once at orchestrator startup; never mutated at
// runtime so a malicious request cannot flip the default-deny switch.
type Config struct {
	// Mode is "local" | "remote" | "disabled". Defaults to "local".
	Mode Mode
	// RemoteURL is the base URL of an OPA sidecar
	// (e.g. http://localhost:8181). Only consulted when Mode==remote.
	RemoteURL string
	// BundleDir, when set, loads .rego files from disk instead of the
	// embedded bundles. Operators use this to roll out an updated
	// bundle without rebuilding the orchestrator.
	BundleDir string
	// DefaultDeny is the safety switch: when the PDP cannot evaluate
	// (bundle load failure, OPA unreachable) the PEP returns deny
	// instead of allow. Defaults to true. NEVER disable in production.
	DefaultDeny bool
	// AllowDisabledMode is the production safety latch. Even if Mode
	// is "disabled", the constructor refuses to return a stub PDP
	// unless this flag is true.
	AllowDisabledMode bool
}

// LoadConfig reads IRONFLYER_OPA_* and returns the Config. Unknown
// modes fall back to ModeLocal with DefaultDeny on so a typo cannot
// accidentally open the policy plane.
func LoadConfig() Config {
	cfg := Config{
		Mode:              ModeLocal,
		DefaultDeny:       true,
		AllowDisabledMode: envTruthy("IRONFLYER_OPA_ALLOW_DISABLED"),
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("IRONFLYER_OPA_MODE"))) {
	case "remote":
		cfg.Mode = ModeRemote
	case "disabled":
		cfg.Mode = ModeDisabled
	case "", "local":
		cfg.Mode = ModeLocal
	}
	cfg.RemoteURL = strings.TrimSpace(os.Getenv("IRONFLYER_OPA_REMOTE_URL"))
	cfg.BundleDir = strings.TrimSpace(os.Getenv("IRONFLYER_OPA_BUNDLE_DIR"))
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_OPA_DEFAULT_DENY")); v != "" {
		// Explicit override; otherwise stay default-deny=true.
		cfg.DefaultDeny = envTruthy("IRONFLYER_OPA_DEFAULT_DENY")
	}
	return cfg
}

func envTruthy(k string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(k))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
