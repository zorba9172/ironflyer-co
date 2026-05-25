// Audit-export wireup — V22 Wave-3 (Agent 35).
//
// Builds the StoreExporter on top of the orchestrator's audit.Store and
// projects operator-tunable knobs (signed-URL base, TTL, max entries)
// out of env so deployment cycles do not require code changes.
package wireup

import (
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog"

	"ironflyer/apps/orchestrator/internal/audit"
	"ironflyer/apps/orchestrator/internal/auditexport"
)

// BuildAuditExporter wraps the audit store with the V22 export surface.
// Returns nil when auditStore is nil so the resolver layer can degrade
// to gqlNotConfigured cleanly.
func BuildAuditExporter(auditStore audit.Store, log zerolog.Logger) auditexport.Exporter {
	if auditStore == nil {
		log.Warn().Msg("auditexport: audit store unwired; exporter disabled")
		return nil
	}
	exp := auditexport.NewStoreExporter(auditStore)
	if max := envInt("IRONFLYER_AUDIT_EXPORT_MAX_ENTRIES"); max > 0 {
		exp.MaxEntries = max
	}
	return exp
}

// BuildAuditExportConfig pulls TTL + base URL knobs out of env. Defaults
// match auditexport.DefaultConfig() so a dev box without env still works.
func BuildAuditExportConfig() auditexport.Config {
	cfg := auditexport.DefaultConfig()
	if base := os.Getenv("IRONFLYER_AUDIT_EXPORT_BASE_URL"); base != "" {
		cfg.SignedURLBase = base
	}
	if ttl := envDuration("IRONFLYER_AUDIT_EXPORT_TTL"); ttl > 0 {
		cfg.SignedURLTTL = ttl
	}
	if max := envInt("IRONFLYER_AUDIT_EXPORT_MAX_ENTRIES"); max > 0 {
		cfg.MaxEntries = max
	}
	return cfg
}

func envInt(key string) int {
	v := os.Getenv(key)
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func envDuration(key string) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return 0
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return 0
	}
	return d
}
