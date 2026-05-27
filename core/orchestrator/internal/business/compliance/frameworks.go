package compliance

import (
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/ai/domain"
)

// catalogue is the static list of supported frameworks. Order matters
// for the operator dashboard — PCI lands first because it carries the
// hardest economic gravity (every fintech needs it before a merchant
// account).
var catalogue = []Framework{
	{
		Key:        "pci-dss-4",
		Label:      "PCI-DSS v4",
		Compliance: "pci",
		Gates:      []domain.GateName{domain.GateCompliancePCI},
		// $499/mo — premium tier, the highest-risk gate.
		MonthlyPriceUSD: decimal.NewFromInt(499),
		EvidenceTemplates: []string{
			"PAN exposure scan",
			"Webhook signature enforcement",
			"TLS on payment routes",
			"Log scrubbing",
		},
	},
	{
		Key:        "hipaa",
		Label:      "HIPAA Security Rule",
		Compliance: "hipaa",
		Gates:      []domain.GateName{domain.GateComplianceHIPAA},
		// $399/mo — health vertical, regulated but smaller surface than
		// PCI dispute window.
		MonthlyPriceUSD: decimal.NewFromInt(399),
		EvidenceTemplates: []string{
			"Access control + RBAC",
			"PHI encryption at rest",
			"Audit logging",
			"Transmission security",
			"BAA artefact",
		},
	},
	{
		Key:        "soc2-type-ii",
		Label:      "SOC 2 Type II",
		Compliance: "soc2",
		Gates:      []domain.GateName{domain.GateComplianceSOC2},
		// $299/mo — broad B2B SaaS demand, lower per-finding severity.
		MonthlyPriceUSD: decimal.NewFromInt(299),
		EvidenceTemplates: []string{
			"Risk assessment (CC3.4)",
			"Logical access (CC6.1)",
			"Encryption in transit / at rest (CC6.6 / CC6.7)",
			"Monitoring (CC7.2)",
			"Audit logging (CC7.3)",
			"Change management (CC8.1)",
		},
	},
	{
		Key:        "gdpr",
		Label:      "GDPR (EU)",
		Compliance: "gdpr",
		Gates:      []domain.GateName{domain.GateComplianceGDPR},
		// $199/mo — entry-tier, easiest to satisfy with the gate set.
		MonthlyPriceUSD: decimal.NewFromInt(199),
		EvidenceTemplates: []string{
			"Cookie consent (Art. 7)",
			"Privacy policy (Art. 12)",
			"Data export (Art. 20)",
			"Account deletion (Art. 17)",
			"PII-free analytics (Art. 5)",
		},
	},
}

// Frameworks returns the static catalogue. Callers MUST NOT mutate the
// returned slice — each Framework is a value, but the slice header is
// shared. We return a fresh slice header so range-with-write loops
// don't accidentally clobber the next caller.
func Frameworks() []Framework {
	out := make([]Framework, len(catalogue))
	copy(out, catalogue)
	return out
}

// LookupFramework returns the framework with the given key, or
// ErrUnknownFramework. Lookup is case-sensitive — keys are
// hyphen-lower by spec.
func LookupFramework(key string) (Framework, error) {
	for _, f := range catalogue {
		if f.Key == key {
			return f, nil
		}
	}
	return Framework{}, ErrUnknownFramework
}
