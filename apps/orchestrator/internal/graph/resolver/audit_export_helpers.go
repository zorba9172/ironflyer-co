// Helpers used by audit_export.resolver.go. Kept in a non-resolver
// file so gqlgen does not strip them on regenerate.
package resolver

import (
	"context"
	"strings"

	"ironflyer/apps/orchestrator/internal/auditexport"
	"ironflyer/apps/orchestrator/internal/auth"
)

func parseFormat(s *string) auditexport.Format {
	if s == nil {
		return auditexport.FormatCSV
	}
	switch strings.ToLower(strings.TrimSpace(*s)) {
	case "jsonl", "ndjson":
		return auditexport.FormatJSONL
	case "csv", "":
		return auditexport.FormatCSV
	}
	return auditexport.FormatCSV
}

func derefBool(p *bool) bool {
	return p != nil && *p
}

type countingWriter struct {
	bytes int
	lines int
}

func (c *countingWriter) Write(p []byte) (int, error) {
	c.bytes += len(p)
	for _, b := range p {
		if b == '\n' {
			c.lines++
		}
	}
	return len(p), nil
}

// callerIsPlatformOperator gates the platform-operator-only branches
// of the audit export resolver (notably the TenantWildcard cross-
// tenant dump). The trust boundary lives in package auth: the call
// here is a single line so every gate that asks "is this an
// operator?" gets the same answer. The legacy Plan="operator"
// shortcut still resolves true via auth.IsPlatformOperatorContext's
// upstream operator.IsOperator caller path — see migration 00038
// for the transition plan.
func callerIsPlatformOperator(ctx context.Context) bool {
	return auth.IsPlatformOperatorContext(ctx)
}
