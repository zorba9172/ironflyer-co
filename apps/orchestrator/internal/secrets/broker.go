// Package secrets is the V22 secret broker. It is the only component
// in the orchestrator allowed to unwrap secret material. Every other
// caller — deploy adapter, workspace mounter, provider router, model
// context builder — holds a SecretRef (public, AI-safe) and obtains a
// short-lived Capability via Release(). The Capability is then
// presented back via Resolve() to fetch the raw value at the moment
// of use, under audit, with a TTL enforced by the broker itself.
//
// Architectural contract (see docs/ARCHITECTURE_POLICY_SECURITY.md
// "Secret Handling"):
//
//   - Postgres stores references, version, metadata, last-used. NEVER
//     the value.
//   - Model context may carry secret names only.
//   - Capability NEVER carries the raw value.
//   - Every Release writes a row to secret_releases with the calling
//     policy decision ID.
//   - Resolve verifies Capability is not expired, scrub-registers the
//     resolved material in the Redactor so downstream log/trace/prompt
//     flow strips it, and emits an audit record.
//   - Rotate increments version and (in real backend impls) triggers
//     upstream rotation; here it bumps the version + RotatedAt so the
//     rest of the system can observe rotation events.
package secrets

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"ironflyer/apps/orchestrator/internal/audit"
)

// Broker is the public contract every consumer codes against. Callers
// MUST pass the policy decision ID from their upstream PEP into
// Release; the broker refuses to release material without one.
type Broker interface {
	// Lookup returns the public reference (name, backend, release_class,
	// version) — NEVER the value. Safe to expose to AI as
	// "you may request this secret".
	Lookup(ctx context.Context, tenantID, projectID, name string) (SecretRef, error)

	// Release returns a Capability that downstream code can present
	// to Resolve. Requires policyDecisionID from a PEP allow.
	Release(ctx context.Context, ref SecretRef, releaseTo string, policyDecisionID string, expiresIn time.Duration, scope ReleaseScope) (Capability, error)

	// Resolve materialises the value from the backend using a
	// Capability. The returned []byte is owned by the caller and MUST
	// be zeroed after use; the broker has already snapshotted a hash
	// into the audit row before returning.
	Resolve(ctx context.Context, cap Capability) ([]byte, error)

	// Rotate increments version + delegates to backend rotation.
	Rotate(ctx context.Context, refID string) (SecretRef, error)

	// Bind associates a secret with a deploy provider's native secret
	// API (the preferred path — provider-native secret references over
	// raw mounts). Today this writes the binding into ref.Metadata so
	// downstream deploy code can render template variables; the actual
	// provider-side write is performed by Agent 21 (deploy adapter).
	Bind(ctx context.Context, ref SecretRef, providerName, providerSecretKey string) error
}

// brokerImpl is the production Broker. Construction goes through
// New() so callers can mix backends without touching the impl.
type brokerImpl struct {
	store      Store
	backends   map[Backend]BackendImpl
	defaultBE  Backend
	redactor   *Redactor
	audit      audit.Store
	logger     zerolog.Logger
	clock      func() time.Time
	mu         sync.RWMutex
	// capabilities indexes live capabilities by ID. The broker keeps
	// only metadata + the decoded reference; the value is never cached.
	capabilities map[string]Capability
}

// Option customises broker construction. Stays a free function list so
// the integration agent can wire optional dependencies (audit, redactor,
// clock) without changing the New() signature later.
type Option func(*brokerImpl)

// WithAudit attaches an audit.Store. Every Release, Resolve, Rotate,
// and Bind writes a row through it.
func WithAudit(a audit.Store) Option {
	return func(b *brokerImpl) { b.audit = a }
}

// WithRedactor attaches the process-wide Redactor. The broker
// AddSecret-registers every resolved value so logs/traces/prompts
// strip it; RemoveSecret-unregisters on capability expiry.
func WithRedactor(r *Redactor) Option {
	return func(b *brokerImpl) { b.redactor = r }
}

// WithLogger lets the integration agent inject the orchestrator's
// zerolog instance. The broker logs only metadata.
func WithLogger(l zerolog.Logger) Option {
	return func(b *brokerImpl) { b.logger = l }
}

// WithClock allows deterministic time injection in tests / replays.
// Production wiring uses time.Now.
func WithClock(fn func() time.Time) Option {
	return func(b *brokerImpl) { b.clock = fn }
}

// New constructs a Broker. The cfg drives default backend selection;
// store is the persistence layer (Postgres or in-memory). bes is the
// list of available backend implementations — typically env + memory
// for dev, env + aws_secrets + vault for production.
func New(cfg Config, store Store, bes []BackendImpl, opts ...Option) Broker {
	b := &brokerImpl{
		store:        store,
		backends:     make(map[Backend]BackendImpl, len(bes)),
		defaultBE:    cfg.DefaultBackend,
		redactor:     NewRedactor(),
		logger:       zerolog.Nop(),
		clock:        func() time.Time { return time.Now().UTC() },
		capabilities: make(map[string]Capability),
	}
	for _, be := range bes {
		if be == nil {
			continue
		}
		b.backends[be.Name()] = be
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Lookup is the AI-safe public reference fetch.
func (b *brokerImpl) Lookup(ctx context.Context, tenantID, projectID, name string) (SecretRef, error) {
	if tenantID == "" || name == "" {
		return SecretRef{}, fmt.Errorf("secrets: lookup requires tenant + name")
	}
	return b.store.LookupRef(ctx, tenantID, projectID, name)
}

// Release issues a Capability after validating the PDP allow and
// writing the audit row + secret_releases row. The Capability never
// contains the value; Resolve loads it on demand.
func (b *brokerImpl) Release(ctx context.Context, ref SecretRef, releaseTo string, policyDecisionID string, expiresIn time.Duration, scope ReleaseScope) (Capability, error) {
	if policyDecisionID == "" {
		return Capability{}, ErrPolicyDecisionRequired
	}
	if !validReleaseTo(releaseTo) {
		return Capability{}, ErrInvalidReleaseTo
	}
	if !validReleaseClass(ref.ReleaseClass) {
		return Capability{}, ErrInvalidReleaseClass
	}
	if expiresIn <= 0 {
		expiresIn = 5 * time.Minute
	}

	be, err := b.resolveBackend(ref.Backend)
	if err != nil {
		return Capability{}, err
	}

	// Load once at release-time so the broker can compute the
	// redaction proof and (for runtime mounts) pre-register the value
	// with the Redactor — this is what prevents the value from leaking
	// into a log line that fires between Release and Resolve.
	value, err := be.Load(ctx, ref)
	if err != nil {
		// Backend not configured is still an audit-worthy event: the
		// caller asked for material that the operator has not wired up.
		b.recordAudit(ctx, audit.Entry{
			Action:    audit.ActionSecretWritten,
			Outcome:   audit.OutcomeFailure,
			ProjectID: ref.ProjectID,
			Summary:   fmt.Sprintf("release blocked: %s/%s", ref.TenantID, ref.Name),
			Attrs: map[string]any{
				"secret_name":      ref.Name,
				"secret_ref_id":    ref.ID,
				"release_class":    string(ref.ReleaseClass),
				"backend":          string(ref.Backend),
				"policy_decision":  policyDecisionID,
				"error":            err.Error(),
			},
		})
		return Capability{}, err
	}

	now := b.clock()
	cap := Capability{
		ID:               "cap_" + uuid.NewString(),
		SecretRefID:      ref.ID,
		Name:             ref.Name,
		ReleaseClass:     ref.ReleaseClass,
		ReleasedTo:       releaseTo,
		ExpiresAt:        now.Add(expiresIn),
		PolicyDecisionID: policyDecisionID,
		Scope:            scope,
		RedactionProof:   Proof(value),
	}

	rel := ReleaseRecord{
		SecretRefID:      ref.ID,
		TenantID:         ref.TenantID,
		ExecutionID:      scope.ExecutionID,
		WorkspaceID:      scope.WorkspaceID,
		PolicyDecisionID: policyDecisionID,
		ReleasedTo:       releaseTo,
		ReleasedAt:       now,
		ExpiresAt:        cap.ExpiresAt,
		RedactionProof:   cap.RedactionProof,
	}
	if _, err := b.store.RecordRelease(ctx, rel); err != nil {
		// Zero the buffer we briefly held so we don't leave the value
		// in heap after a failed write.
		zero(value)
		return Capability{}, fmt.Errorf("secrets: record release: %w", err)
	}

	// Register with the redactor so any downstream log/trace/prompt
	// flow that incorporates the bytes will scrub them.
	if b.redactor != nil {
		b.redactor.AddSecret(ref.Name, value)
	}

	b.mu.Lock()
	b.capabilities[cap.ID] = cap
	b.mu.Unlock()

	// Schedule expiry cleanup: drop the redactor entry + the cached
	// capability after ExpiresAt elapses. The goroutine is lightweight
	// (one timer per active capability) and avoids holding values past
	// the broker's own TTL contract.
	go b.expireAfter(cap.ID, ref.Name, expiresIn)

	zero(value)

	b.recordAudit(ctx, audit.Entry{
		Action:    audit.ActionSecretWritten,
		Outcome:   audit.OutcomeSuccess,
		ProjectID: ref.ProjectID,
		Summary:   fmt.Sprintf("released %s to %s", ref.Name, releaseTo),
		Attrs: map[string]any{
			"secret_name":      ref.Name,
			"secret_ref_id":    ref.ID,
			"release_class":    string(ref.ReleaseClass),
			"capability_id":    cap.ID,
			"released_to":      releaseTo,
			"policy_decision":  policyDecisionID,
			"expires_at":       cap.ExpiresAt.Format(time.RFC3339),
			"redaction_proof":  cap.RedactionProof,
			"execution_id":     scope.ExecutionID,
			"workspace_id":     scope.WorkspaceID,
			"deploy_target":    scope.DeployTarget,
		},
	})

	b.logger.Info().
		Str("secret_name", ref.Name).
		Str("capability_id", cap.ID).
		Str("released_to", releaseTo).
		Str("policy_decision", policyDecisionID).
		Time("expires_at", cap.ExpiresAt).
		Msg("secret released")

	return cap, nil
}

// Resolve verifies + loads. The broker re-loads from the backend on
// every call so it never holds raw material between calls; this also
// catches mid-flight rotations.
func (b *brokerImpl) Resolve(ctx context.Context, cap Capability) ([]byte, error) {
	if cap.ID == "" || cap.SecretRefID == "" {
		return nil, ErrCapabilityInvalid
	}
	now := b.clock()
	if cap.Expired(now) {
		b.recordAudit(ctx, audit.Entry{
			Action:  audit.ActionSecretWritten,
			Outcome: audit.OutcomeBlocked,
			Summary: fmt.Sprintf("resolve blocked: capability %s expired", cap.ID),
			Attrs: map[string]any{
				"capability_id":   cap.ID,
				"secret_name":     cap.Name,
				"secret_ref_id":   cap.SecretRefID,
				"policy_decision": cap.PolicyDecisionID,
			},
		})
		return nil, ErrCapabilityExpired
	}

	b.mu.RLock()
	known, ok := b.capabilities[cap.ID]
	b.mu.RUnlock()
	if !ok {
		// We don't have a record of this capability — it may have
		// expired and been GC'd, or it was forged. Either way, refuse.
		return nil, ErrCapabilityInvalid
	}
	if known.Expired(now) {
		return nil, ErrCapabilityExpired
	}

	ref, err := b.store.GetRef(ctx, cap.SecretRefID)
	if err != nil {
		return nil, err
	}
	be, err := b.resolveBackend(ref.Backend)
	if err != nil {
		return nil, err
	}
	value, err := be.Load(ctx, ref)
	if err != nil {
		return nil, err
	}

	if b.redactor != nil {
		b.redactor.AddSecret(ref.Name, value)
	}

	b.recordAudit(ctx, audit.Entry{
		Action:    audit.ActionSecretWritten,
		Outcome:   audit.OutcomeSuccess,
		ProjectID: ref.ProjectID,
		Summary:   fmt.Sprintf("resolved %s via %s", ref.Name, cap.ReleasedTo),
		Attrs: map[string]any{
			"secret_name":     ref.Name,
			"secret_ref_id":   ref.ID,
			"capability_id":   cap.ID,
			"released_to":     cap.ReleasedTo,
			"policy_decision": cap.PolicyDecisionID,
			"redaction_proof": Proof(value),
			"resolve_event":   true,
		},
	})

	return value, nil
}

// Rotate bumps version + stamps RotatedAt. Backend-side rotation is
// deferred to the real AWS / Vault implementations.
func (b *brokerImpl) Rotate(ctx context.Context, refID string) (SecretRef, error) {
	if refID == "" {
		return SecretRef{}, ErrSecretNotFound
	}
	ref, err := b.store.UpdateRef(ctx, refID, func(r *SecretRef) {
		r.Version++
		now := b.clock()
		r.RotatedAt = &now
	})
	if err != nil {
		return SecretRef{}, err
	}

	// Drop the redactor entry — the bytes from the previous version
	// are no longer in play and shouldn't keep matching.
	if b.redactor != nil {
		b.redactor.RemoveSecret(ref.Name)
	}

	b.recordAudit(ctx, audit.Entry{
		Action:    audit.ActionSecretWritten,
		Outcome:   audit.OutcomeSuccess,
		ProjectID: ref.ProjectID,
		Summary:   fmt.Sprintf("rotated %s to version %d", ref.Name, ref.Version),
		Attrs: map[string]any{
			"secret_name":   ref.Name,
			"secret_ref_id": ref.ID,
			"version":       ref.Version,
			"rotate_event":  true,
		},
	})

	return ref, nil
}

// Bind annotates a SecretRef with a provider-native binding (e.g.
// Vercel env-var name, Cloudflare KV key). The deploy adapter reads
// this metadata to render provider-native secret references instead
// of embedding values into generated deploy files.
func (b *brokerImpl) Bind(ctx context.Context, ref SecretRef, providerName, providerSecretKey string) error {
	providerName = strings.TrimSpace(providerName)
	providerSecretKey = strings.TrimSpace(providerSecretKey)
	if providerName == "" || providerSecretKey == "" {
		return fmt.Errorf("secrets: bind requires provider name + key")
	}
	_, err := b.store.UpdateRef(ctx, ref.ID, func(r *SecretRef) {
		if r.Metadata == nil {
			r.Metadata = map[string]any{}
		}
		bindings, _ := r.Metadata["provider_bindings"].(map[string]any)
		if bindings == nil {
			bindings = map[string]any{}
		}
		bindings[providerName] = providerSecretKey
		r.Metadata["provider_bindings"] = bindings
	})
	if err != nil {
		return err
	}
	b.recordAudit(ctx, audit.Entry{
		Action:    audit.ActionSecretWritten,
		Outcome:   audit.OutcomeSuccess,
		ProjectID: ref.ProjectID,
		Summary:   fmt.Sprintf("bound %s to %s", ref.Name, providerName),
		Attrs: map[string]any{
			"secret_name":         ref.Name,
			"secret_ref_id":       ref.ID,
			"provider_name":       providerName,
			"provider_secret_key": providerSecretKey,
			"bind_event":          true,
		},
	})
	return nil
}

// resolveBackend looks up the backend impl, falling back to the
// configured default for legacy SecretRefs that don't pin one.
func (b *brokerImpl) resolveBackend(name Backend) (BackendImpl, error) {
	if name == "" {
		name = b.defaultBE
	}
	be, ok := b.backends[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownBackend, name)
	}
	return be, nil
}

// expireAfter drops the capability + redactor entry once its TTL
// elapses. Runs in its own goroutine; the cost is one timer per active
// capability, which is trivial at expected scale.
func (b *brokerImpl) expireAfter(capID, secretName string, after time.Duration) {
	t := time.NewTimer(after)
	defer t.Stop()
	<-t.C
	b.mu.Lock()
	delete(b.capabilities, capID)
	b.mu.Unlock()
	if b.redactor != nil {
		b.redactor.RemoveSecret(secretName)
	}
}

// recordAudit best-effort writes an audit row. The broker MUST NOT
// fail a release because the audit store is degraded; the failure is
// surfaced via the logger so ops can react.
func (b *brokerImpl) recordAudit(ctx context.Context, e audit.Entry) {
	if b.audit == nil {
		return
	}
	if _, err := b.audit.Record(ctx, e); err != nil {
		b.logger.Error().Err(err).Str("action", string(e.Action)).Msg("secret audit write failed")
	}
}

// zero best-effort wipes a byte slice. Go's runtime is allowed to
// move heap memory around, so this is not a guarantee — but every
// little bit reduces the window an attacker has against a post-mortem
// dump of the orchestrator pod.
func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// Sentinel guard: keep the errors import used even if every reference
// is via fmt.Errorf %w (defensive — the linter sometimes nudges
// otherwise valid code on imports).
var _ = errors.New
