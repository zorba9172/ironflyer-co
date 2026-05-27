// Package compliance turns the finisher's framework-specific gates
// (PCI / HIPAA / SOC2 / GDPR) into a premium per-project subscription
// SKU.
//
// Design notes (defaults documented here so call sites don't need to
// re-derive them):
//
//   - Tenant scoping: every operation is keyed by (tenant, project,
//     framework). Tenant comes from the resolver via tenantFor(user).
//     Cross-tenant reads return ErrNotFound.
//
//   - Frameworks are static (frameworks.go). Adding one is a code change
//     so prices, gate lists, and the attestation surface stay
//     reviewable.
//
//   - Pricing: per-framework monthly USD price billed by the wallet via
//     Service.RunMonthlyBilling. Idempotency key is
//     "compliance:<tenant>:<project>:<framework>:<YYYY-MM>" so a
//     re-tick during the same calendar month never double-charges.
//
//   - Evaluation: EvaluateAll runs every gate in the framework's gate
//     list against the project workspace, persists ControlResult rows,
//     and publishes a KindGateOutcome OutcomeEvent per result so the
//     Feedback Brain sees compliance signal alongside generic gates.
//
//   - Attestation: ExportAuditBundle signs a JWT
//     ("Ironflyer Verified — project P passed framework F at T") with
//     HS256 using IRONFLYER_ATTESTATION_SECRET. No third-party auditor
//     API is called; the JWT is the evidence handed to an external
//     auditor.
//
//   - OutcomeEvent: emitted on enroll, unenroll, evaluate (per
//     control), and charge so the Feedback Brain can mine adoption,
//     verdict drift, and revenue.
//
//   - Logging: zerolog only.
//
//   - Money: shopspring/decimal everywhere.
package compliance

import (
	"time"

	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/ai/domain"
)

// Framework is a static catalogue entry: a regulatory regime, the gate
// set that evaluates it, and the monthly subscription price billed via
// the wallet.
type Framework struct {
	// Key is the stable identifier used in URLs and idempotency keys.
	// Forever lowercase, hyphenated.
	Key string
	// Label is the operator-facing display name.
	Label string
	// Compliance is the Project.Spec.HasCompliance() string the gates
	// key off (e.g. "pci", "hipaa", "soc2", "gdpr"). Set so EvaluateAll
	// can inject the regime into the spec for a one-shot evaluation
	// even when the project itself has not opted in.
	Compliance string
	// Gates are the finisher GateName constants that evaluate this
	// framework. Order is honoured in ControlResult listings so the
	// audit bundle reads in a deterministic sequence.
	Gates []domain.GateName
	// MonthlyPriceUSD is the subscription price charged once per
	// calendar month per (tenant, project, framework). Pre-discount.
	MonthlyPriceUSD decimal.Decimal
	// EvidenceTemplates names the canonical evidence sections embedded
	// in the AuditBundle README. Currently rendered as headers; future
	// rev fills them with per-control evidence.
	EvidenceTemplates []string
}

// Tier is the dashboard projection of a framework's commercial level.
// Today every framework is "premium"; reserved for future "team" /
// "enterprise" variants without a schema change.
type Tier string

const (
	TierPremium Tier = "premium"
)

// VerdictKind is the closed taxonomy of EnrolledProject.LastVerdict
// values surfaced on the dashboard.
type VerdictKind string

const (
	VerdictPending VerdictKind = "pending"
	VerdictPass    VerdictKind = "pass"
	VerdictFail    VerdictKind = "fail"
)

// ControlStatus is the per-control evaluation result. n/a means the
// gate skipped the control (e.g. no PHI tables on a HIPAA project, so
// the encryption-at-rest control did not apply).
type ControlStatus string

const (
	StatusPass ControlStatus = "pass"
	StatusFail ControlStatus = "fail"
	StatusNA   ControlStatus = "n/a"
)

// EnrolledProject is the per-(tenant, project, framework) subscription
// row. ID is stable so a UI selection survives across evaluations.
type EnrolledProject struct {
	ID              string
	TenantID        string
	ProjectID       string
	FrameworkKey    string
	EnrolledAt      time.Time
	LastEvaluatedAt *time.Time
	LastVerdict     VerdictKind
	NextChargeAt    time.Time
}

// ControlResult is one finisher-gate finding aggregated into the
// compliance projection. Evidence is the operator-facing one-liner;
// the gate's full hint text is what we copy in there.
type ControlResult struct {
	ID           string
	EnrollmentID string
	ControlKey   string
	FrameworkKey string
	Status       ControlStatus
	Severity     domain.Severity
	Evidence     string
	Path         string
	EvaluatedAt  time.Time
}

// AuditBundle is the export payload. DownloadURL is the signed S3 URL
// (or an inline data URL when no S3 client is wired); AttestationJWT
// is the HS256-signed Ironflyer Verified token; Controls is the most
// recent evaluation projection.
type AuditBundle struct {
	ProjectID      string
	FrameworkKey   string
	Framework      Framework
	GeneratedAt    time.Time
	Controls       []ControlResult
	AttestationJWT string
	DownloadURL    string
	// TarGzBytes is the raw bundle payload — included so resolvers that
	// inline-serve the bundle (no S3) can return it directly.
	TarGzBytes []byte
}

// Charge is one monthly subscription debit, recorded so the
// reconciler can prove every active enrolment was billed for the
// month. Period is the YYYY-MM key the idempotency join uses.
type Charge struct {
	ID             string
	EnrollmentID   string
	TenantID       string
	FrameworkKey   string
	Period         string // "2026-05"
	AmountUSD      decimal.Decimal
	ChargedAt      time.Time
	IdempotencyKey string
}
