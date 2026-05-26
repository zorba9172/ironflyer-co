// Package securityreport realises 90-day-plan item "customer-visible
// security report": the per-execution surface that turns the
// finisher Security gate's bag of issues into a structured,
// queryable resource the customer dashboard and audit pipeline can
// consume.
//
// severity.go owns the canonical severity vocabulary and the ranking
// helpers Builder.Build relies on. Severity strings are wire values
// — never rename in place; only add new constants.
package securityreport

// Severity is the canonical 5-level scale we project every static or
// LLM-derived finding onto. The strings are lower-case so they read
// cleanly on the wire (GraphQL enums are upcased separately).
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// RankSeverity maps Severity to a numeric weight used for sorting,
// max-aggregation, and the OverallScore formula. Unknown strings
// rank as info (0) so a degraded source can't accidentally inflate
// the worst-severity verdict.
func RankSeverity(s Severity) int {
	switch s {
	case SeverityCritical:
		return 4
	case SeverityHigh:
		return 3
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 1
	case SeverityInfo:
		return 0
	}
	return 0
}

// WorstSeverity returns the highest-ranked severity present in the
// findings slice. An empty slice returns SeverityInfo so the caller
// can render "no issues" without special-casing nil.
func WorstSeverity(findings []Finding) Severity {
	worst := SeverityInfo
	worstRank := RankSeverity(worst)
	for _, f := range findings {
		if r := RankSeverity(f.Severity); r > worstRank {
			worstRank = r
			worst = f.Severity
		}
	}
	return worst
}

// NormaliseSeverity coerces free-form strings from upstream scanners
// (semgrep ERROR / WARNING, gitleaks "high", govulncheck implicit
// critical) into the canonical scale. The finisher's domain.Severity
// vocabulary is one such upstream and is handled by the caller.
func NormaliseSeverity(s string) Severity {
	switch s {
	case "critical", "CRITICAL", "Critical":
		return SeverityCritical
	case "high", "HIGH", "High", "error", "ERROR":
		return SeverityHigh
	case "medium", "MEDIUM", "Medium", "warning", "WARNING", "warn":
		return SeverityMedium
	case "low", "LOW", "Low":
		return SeverityLow
	case "info", "INFO", "Info", "":
		return SeverityInfo
	}
	return SeverityInfo
}
