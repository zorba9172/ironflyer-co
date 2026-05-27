package compliance

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/ai/finisher"
	"ironflyer/core/orchestrator/internal/ai/learning"
)

// Backend is the persistence contract. MemoryBackend (memory.go) and
// PostgresBackend (postgres.go) both implement it. Service is the
// public surface every resolver and cron uses; Backend stays internal.
type Backend interface {
	// Enroll inserts a new enrolment row. Returns ErrAlreadyEnrolled
	// when a row already exists for (tenant, project, framework); the
	// caller decides whether to return the existing row.
	Enroll(ctx context.Context, row EnrolledProject) error

	// GetEnrollment by id (no tenant filter — caller MUST check
	// EnrolledProject.TenantID against the actor before serving). The
	// resolver layer enforces tenant scoping; Backend exposes the raw
	// row so the reconciler can sweep every tenant.
	GetEnrollment(ctx context.Context, id string) (EnrolledProject, error)

	// GetEnrollmentByTuple resolves (tenant, project, framework) →
	// enrolment row. Returns ErrNotFound when absent.
	GetEnrollmentByTuple(ctx context.Context, tenant, projectID, framework string) (EnrolledProject, error)

	// ListEnrollments returns every enrolment for a tenant, optionally
	// scoped to one project. Newest first.
	ListEnrollments(ctx context.Context, tenant, projectID string) ([]EnrolledProject, error)

	// ListAllEnrollments returns every enrolment across every tenant
	// for the reconciler.
	ListAllEnrollments(ctx context.Context) ([]EnrolledProject, error)

	// MarkEvaluated updates LastEvaluatedAt + LastVerdict.
	MarkEvaluated(ctx context.Context, id string, at time.Time, verdict VerdictKind) error

	// DeleteEnrollment removes an enrolment. Idempotent — missing rows
	// return nil.
	DeleteEnrollment(ctx context.Context, id string) error

	// SaveResults atomically replaces the result set for one
	// enrolment with the supplied slice. Used so the dashboard always
	// shows the most recent verdict per control without union-merge
	// semantics.
	SaveResults(ctx context.Context, enrollmentID string, results []ControlResult) error

	// ListResults returns the persisted results for an enrolment,
	// ordered by Severity then ControlKey.
	ListResults(ctx context.Context, enrollmentID string) ([]ControlResult, error)

	// RecordCharge persists a Charge row keyed by IdempotencyKey.
	// Returns ErrAlreadyEnrolled when the row already exists so the
	// monthly reconciler can short-circuit. (We intentionally re-use
	// the sentinel — both errors mean "this state already exists".)
	RecordCharge(ctx context.Context, charge Charge) error

	// HasCharge reports whether a Charge with the given idempotency
	// key already landed. Cheap so the reconciler can pre-flight.
	HasCharge(ctx context.Context, idempotencyKey string) (bool, error)
}

// ProjectLoader is the seam compliance uses to read a project. We
// don't import operations/store here to keep the dependency arrow
// pointing outward.
type ProjectLoader interface {
	Get(id string) (domain.Project, error)
}

// WalletDebiter is the surface compliance uses to charge the monthly
// subscription. wallet.Service already satisfies this — we narrow it
// so the package only sees the operation it needs.
type WalletDebiter interface {
	Hold(ctx context.Context, tenant string, amount decimal.Decimal) error
	Debit(ctx context.Context, tenant string, amount decimal.Decimal) error
}

// Service is the public surface. Construct with NewService; the
// orchestrator wires it next to wallet.Service. Every operation is
// owner-checked at the resolver layer; Service itself trusts the
// supplied tenant string.
type Service struct {
	backend           Backend
	projects          ProjectLoader
	wallet            WalletDebiter
	attestationSecret string
	logger            zerolog.Logger
	// Now is overridable for testability of the reconciler (we never
	// add tests, but we keep the seam so it stays inspectable).
	Now func() time.Time
}

// NewService builds a Service. attestationSecret carries the HS256
// signing key (env IRONFLYER_ATTESTATION_SECRET); when empty,
// ExportAuditBundle returns ErrAttestationDisabled and the dashboard
// renders a "verify by setting IRONFLYER_ATTESTATION_SECRET" empty
// state. Wallet may be nil for dev boots; billing then falls back to
// a logged no-op.
func NewService(backend Backend, projects ProjectLoader, wallet WalletDebiter, attestationSecret string, logger zerolog.Logger) *Service {
	return &Service{
		backend:           backend,
		projects:          projects,
		wallet:            wallet,
		attestationSecret: attestationSecret,
		logger:            logger,
		Now:               func() time.Time { return time.Now().UTC() },
	}
}

// Frameworks proxies frameworks.go so callers depend on a single
// surface.
func (s *Service) Frameworks() []Framework { return Frameworks() }

// Enroll creates the (tenant, project, framework) enrolment row. The
// first charge fires at the next monthly reconciler tick — callers who
// want immediate billing should call ChargeOnce after Enroll. Emits an
// OutcomeEvent with the framework key tagged.
func (s *Service) Enroll(ctx context.Context, tenant, projectID, frameworkKey string) (EnrolledProject, error) {
	f, err := LookupFramework(frameworkKey)
	if err != nil {
		return EnrolledProject{}, err
	}
	now := s.Now()
	row := EnrolledProject{
		ID:           uuid.NewString(),
		TenantID:     tenant,
		ProjectID:    projectID,
		FrameworkKey: f.Key,
		EnrolledAt:   now,
		LastVerdict:  VerdictPending,
		NextChargeAt: nextMonth(now),
	}
	if err := s.backend.Enroll(ctx, row); err != nil {
		if errors.Is(err, ErrAlreadyEnrolled) {
			existing, gerr := s.backend.GetEnrollmentByTuple(ctx, tenant, projectID, f.Key)
			if gerr != nil {
				return EnrolledProject{}, gerr
			}
			return existing, nil
		}
		return EnrolledProject{}, err
	}
	s.publish(ctx, tenant, "compliance_enrolled", projectID, f.Key, VerdictPending, true)
	s.logger.Info().
		Str("tenant", tenant).
		Str("project_id", projectID).
		Str("framework", f.Key).
		Str("enrollment_id", row.ID).
		Msg("compliance enrolled")
	return row, nil
}

// Unenroll deletes the enrolment row. Past charges remain in the
// charges table so the audit trail is preserved.
func (s *Service) Unenroll(ctx context.Context, tenant, enrollmentID string) error {
	row, err := s.backend.GetEnrollment(ctx, enrollmentID)
	if err != nil {
		return err
	}
	if row.TenantID != tenant {
		return ErrNotFound
	}
	if err := s.backend.DeleteEnrollment(ctx, enrollmentID); err != nil {
		return err
	}
	s.publish(ctx, tenant, "compliance_unenrolled", row.ProjectID, row.FrameworkKey, VerdictPending, true)
	s.logger.Info().
		Str("tenant", tenant).
		Str("project_id", row.ProjectID).
		Str("framework", row.FrameworkKey).
		Msg("compliance unenrolled")
	return nil
}

// ListEnrolled returns the tenant's active enrolments, optionally
// scoped to one project.
func (s *Service) ListEnrolled(ctx context.Context, tenant, projectID string) ([]EnrolledProject, error) {
	return s.backend.ListEnrollments(ctx, tenant, projectID)
}

// EvaluateAll runs every gate in the framework against the project
// workspace and persists the result projection. Returns the new
// ControlResult slice ordered by severity (critical first). Emits one
// OutcomeEvent per control and one summary "compliance_evaluated"
// event keyed by verdict.
func (s *Service) EvaluateAll(ctx context.Context, tenant, projectID, frameworkKey string) ([]ControlResult, error) {
	f, err := LookupFramework(frameworkKey)
	if err != nil {
		return nil, err
	}
	row, err := s.backend.GetEnrollmentByTuple(ctx, tenant, projectID, f.Key)
	if err != nil {
		return nil, err
	}
	proj, err := s.projects.Get(projectID)
	if err != nil {
		return nil, fmt.Errorf("compliance: load project %s: %w", projectID, err)
	}
	// Inject the framework's compliance regime so gates that key off
	// Project.Spec.HasCompliance fire even when the spec did not
	// declare it. This is the differentiator: the SKU buys evaluation
	// regardless of upstream spec hygiene.
	if !proj.Spec.HasCompliance(f.Compliance) {
		proj.Spec.Compliance = append([]string{f.Compliance}, proj.Spec.Compliance...)
	}

	env := &finisher.GateEnv{Project: &proj}
	now := s.Now()
	var results []ControlResult
	verdict := VerdictPass
	for _, gateName := range f.Gates {
		gate := lookupGate(gateName)
		if gate == nil {
			continue
		}
		issues := gate.Check(ctx, env)
		if len(issues) == 0 {
			results = append(results, ControlResult{
				ID:           uuid.NewString(),
				EnrollmentID: row.ID,
				ControlKey:   string(gateName),
				FrameworkKey: f.Key,
				Status:       StatusPass,
				Severity:     domain.SeverityInfo,
				Evidence:     "no findings",
				EvaluatedAt:  now,
			})
			continue
		}
		for _, iss := range issues {
			status := StatusFail
			if iss.Severity == domain.SeverityInfo {
				status = StatusNA
			}
			if status == StatusFail {
				verdict = VerdictFail
			}
			results = append(results, ControlResult{
				ID:           uuid.NewString(),
				EnrollmentID: row.ID,
				ControlKey:   controlKey(gateName, iss.Message),
				FrameworkKey: f.Key,
				Status:       status,
				Severity:     iss.Severity,
				Evidence:     iss.Message + " — " + iss.Hint,
				Path:         iss.Path,
				EvaluatedAt:  now,
			})
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		return severityRank(results[i].Severity) < severityRank(results[j].Severity)
	})
	if err := s.backend.SaveResults(ctx, row.ID, results); err != nil {
		return nil, err
	}
	if err := s.backend.MarkEvaluated(ctx, row.ID, now, verdict); err != nil {
		return nil, err
	}
	s.publish(ctx, tenant, "compliance_evaluated", projectID, f.Key, verdict, verdict == VerdictPass)
	return results, nil
}

// ListResults returns the most recent persisted results for an
// enrolment. Used by the dashboard between evaluations.
func (s *Service) ListResults(ctx context.Context, tenant, projectID, frameworkKey string) ([]ControlResult, error) {
	row, err := s.backend.GetEnrollmentByTuple(ctx, tenant, projectID, frameworkKey)
	if err != nil {
		return nil, err
	}
	return s.backend.ListResults(ctx, row.ID)
}

// ExportAuditBundle re-evaluates the framework, packages the results
// + signed attestation into a tar.gz, and returns the bundle for the
// resolver to serve. Returns ErrAttestationDisabled when the secret
// is unset.
func (s *Service) ExportAuditBundle(ctx context.Context, tenant, projectID, frameworkKey string) (AuditBundle, error) {
	if strings.TrimSpace(s.attestationSecret) == "" {
		return AuditBundle{}, ErrAttestationDisabled
	}
	f, err := LookupFramework(frameworkKey)
	if err != nil {
		return AuditBundle{}, err
	}
	results, err := s.EvaluateAll(ctx, tenant, projectID, frameworkKey)
	if err != nil {
		return AuditBundle{}, err
	}
	verdict := VerdictPass
	for _, r := range results {
		if r.Status == StatusFail {
			verdict = VerdictFail
			break
		}
	}
	now := s.Now()
	jwt, err := signAttestation(s.attestationSecret, attestationClaims{
		Issuer:    "ironflyer",
		Subject:   projectID,
		Audience:  "external-auditor",
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(90 * 24 * time.Hour).Unix(),
		Framework: f.Key,
		Verdict:   string(verdict),
		Tenant:    tenant,
	})
	if err != nil {
		return AuditBundle{}, err
	}
	bundle := AuditBundle{
		ProjectID:      projectID,
		FrameworkKey:   f.Key,
		Framework:      f,
		GeneratedAt:    now,
		Controls:       results,
		AttestationJWT: jwt,
	}
	tarBytes, err := buildAuditTarGz(bundle)
	if err != nil {
		return AuditBundle{}, err
	}
	bundle.TarGzBytes = tarBytes
	// No S3 client here — resolver inlines as a data URL so the bundle
	// is downloadable from the dashboard without extra infra. When the
	// orchestrator later wires an s3client, a wrapper can replace
	// DownloadURL with a signed object URL.
	bundle.DownloadURL = inlineDataURL(tarBytes)
	return bundle, nil
}

// ChargeOnce runs the wallet debit for a single enrolment + period.
// Idempotent on (enrollmentID, period): a re-tick during the same
// month returns nil without charging. Used by RunMonthlyBilling and
// available standalone so the resolver can fire an immediate first
// charge on Enroll if desired.
func (s *Service) ChargeOnce(ctx context.Context, enrollmentID string) error {
	row, err := s.backend.GetEnrollment(ctx, enrollmentID)
	if err != nil {
		return err
	}
	f, err := LookupFramework(row.FrameworkKey)
	if err != nil {
		return err
	}
	period := monthKey(s.Now())
	idem := fmt.Sprintf("compliance:%s:%s:%s:%s", row.TenantID, row.ProjectID, f.Key, period)
	already, err := s.backend.HasCharge(ctx, idem)
	if err != nil {
		return err
	}
	if already {
		return nil
	}
	if s.wallet == nil {
		s.logger.Warn().
			Str("enrollment_id", row.ID).
			Str("period", period).
			Msg("compliance: wallet not wired — skipping charge")
		return nil
	}
	if err := s.wallet.Hold(ctx, row.TenantID, f.MonthlyPriceUSD); err != nil {
		s.publish(ctx, row.TenantID, "compliance_charge_failed", row.ProjectID, f.Key, row.LastVerdict, false)
		return fmt.Errorf("%w: %v", ErrInsufficientBalance, err)
	}
	if err := s.wallet.Debit(ctx, row.TenantID, f.MonthlyPriceUSD); err != nil {
		s.publish(ctx, row.TenantID, "compliance_charge_failed", row.ProjectID, f.Key, row.LastVerdict, false)
		return err
	}
	charge := Charge{
		ID:             uuid.NewString(),
		EnrollmentID:   row.ID,
		TenantID:       row.TenantID,
		FrameworkKey:   f.Key,
		Period:         period,
		AmountUSD:      f.MonthlyPriceUSD,
		ChargedAt:      s.Now(),
		IdempotencyKey: idem,
	}
	if err := s.backend.RecordCharge(ctx, charge); err != nil && !errors.Is(err, ErrAlreadyEnrolled) {
		// The wallet already debited; the row miss-record is logged but
		// not retried. Future tick will see HasCharge=true via the
		// idem-keyed unique constraint and short-circuit.
		s.logger.Error().Err(err).
			Str("idem_key", idem).
			Msg("compliance: charge recorded only in wallet (DB persist failed)")
	}
	s.publish(ctx, row.TenantID, "compliance_charged", row.ProjectID, f.Key, row.LastVerdict, true)
	return nil
}

// publish emits an OutcomeEvent so the Feedback Brain sees every
// compliance state transition.
func (s *Service) publish(ctx context.Context, tenant, kindHint, projectID, framework string, verdict VerdictKind, success bool) {
	learning.Publish(ctx, learning.OutcomeEvent{
		TenantID: tenant,
		Kind:     learning.KindGateOutcome,
		Attributes: map[string]any{
			"compliance_event": kindHint,
			"project_id":       projectID,
			"framework":        framework,
			"verdict":          string(verdict),
		},
		Success: learning.BoolPtr(success),
		Tags: map[string]string{
			"framework": framework,
		},
	})
}

// nextMonth returns the first UTC day of the next calendar month at
// midnight. Used as the EnrolledProject.NextChargeAt default.
func nextMonth(now time.Time) time.Time {
	y, m, _ := now.UTC().Date()
	return time.Date(y, m+1, 1, 0, 0, 0, 0, time.UTC)
}

// monthKey is the "YYYY-MM" string used as the idempotency partition
// for a calendar month.
func monthKey(t time.Time) string {
	return t.UTC().Format("2006-01")
}

// severityRank maps domain.Severity to a sort ordering with the
// highest-severity findings first.
func severityRank(s domain.Severity) int {
	switch s {
	case domain.SeverityCritical:
		return 0
	case domain.SeverityError:
		return 1
	case domain.SeverityWarning:
		return 2
	case domain.SeverityInfo:
		return 3
	}
	return 4
}

// controlKey derives a stable control identifier from the gate name +
// the first colon-prefix of the issue message (e.g. "PCI 3.4" out of
// "PCI 3.4: candidate PAN found …"). Falls back to the gate name
// alone when no prefix exists.
func controlKey(gate domain.GateName, message string) string {
	idx := strings.Index(message, ":")
	if idx <= 0 || idx > 40 {
		return string(gate)
	}
	return string(gate) + "/" + strings.TrimSpace(message[:idx])
}

// lookupGate returns the finisher.Gate matching the supplied name, or
// nil when the name is unknown.
func lookupGate(name domain.GateName) finisher.Gate {
	for _, g := range finisher.DefaultGates() {
		if g.Name() == name {
			return g
		}
	}
	return nil
}
