package policy

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/customer/auth"
)

// PEP is the Policy Enforcement Point — the thin wrapper every caller
// uses to consult the PDP. Callers never speak Rego or even
// DecisionRequest directly; they call PEP.Allow / PEP.MustAllow with
// the action they're about to perform plus the Resource it targets.
//
// The PEP is intentionally NOT package-global. Construct one in main,
// pass it into every subsystem that needs to ask "may I do X?".
type PEP struct {
	pdp PDP
	log zerolog.Logger
	cfg Config
}

// NewPEP builds the PEP using the supplied config + auditor. It picks
// the PDP implementation per cfg.Mode:
//
//	local    -> LocalPDP (in-process OPA, embedded bundles by default)
//	remote   -> RemotePDP (OPA sidecar over HTTP)
//	disabled -> DisabledPDP (only if cfg.AllowDisabledMode == true)
//
// On any failure the constructor returns an error so the orchestrator
// refuses to boot rather than running with the policy plane silently
// open.
func NewPEP(cfg Config, auditor *Auditor, log zerolog.Logger) (*PEP, error) {
	plog := log.With().Str("subsystem", "policy.pep").Logger()
	switch cfg.Mode {
	case ModeDisabled:
		if !cfg.AllowDisabledMode {
			return nil, fmt.Errorf("policy: mode=disabled requires IRONFLYER_OPA_ALLOW_DISABLED=1")
		}
		plog.Warn().Msg("policy plane disabled — all decisions will allow; never run in production")
		return &PEP{pdp: NewDisabledPDP(), log: plog, cfg: cfg}, nil
	case ModeRemote:
		// Remote PDPs own their own bundle versioning; we still load
		// embedded bundles to compute a local hash for audit, but the
		// remote authority is the OPA sidecar.
		_, version, err := LoadBundles(cfg)
		if err != nil {
			return nil, err
		}
		pdp, err := NewRemotePDP(cfg.RemoteURL, version, auditor, log)
		if err != nil {
			return nil, err
		}
		return &PEP{pdp: pdp, log: plog, cfg: cfg}, nil
	case ModeLocal, "":
		bundles, version, err := LoadBundles(cfg)
		if err != nil {
			return nil, err
		}
		pdp, err := NewLocalPDP(bundles, version, auditor, log)
		if err != nil {
			return nil, err
		}
		return &PEP{pdp: pdp, log: plog, cfg: cfg}, nil
	default:
		return nil, fmt.Errorf("policy: unknown mode %q", cfg.Mode)
	}
}

// BundleVersion exposes the active PDP's bundle hash for /policy/version
// surfaces and audit pinning.
func (p *PEP) BundleVersion() string { return p.pdp.BundleVersion() }

// PDP returns the underlying decision point. Tests and operator tools
// can use this; production callers should stick to Allow/MustAllow.
func (p *PEP) PDP() PDP { return p.pdp }

// Option is the builder for a DecisionRequest. PEP callers pass these
// into Allow/MustAllow so the call site reads as a single fluent line.
type Option func(*DecisionRequest)

// WithPrincipal pins the principal explicitly. Most callers should
// rely on the auth-context derivation; this exists for system actors
// (Temporal workers, cron jobs) that don't have a request-scoped user.
func WithPrincipal(p Principal) Option {
	return func(req *DecisionRequest) { req.Principal = p }
}

// WithDelegation captures the AI/agent acting under the principal.
func WithDelegation(d Delegation) Option {
	return func(req *DecisionRequest) { req.Delegation = d }
}

// WithContextKV adds a single key/value to the context bag.
// Convenience for one-off attributes; for several keys prefer
// WithContext.
func WithContextKV(k string, v any) Option {
	return func(req *DecisionRequest) {
		if req.Context == nil {
			req.Context = map[string]any{}
		}
		req.Context[k] = v
	}
}

// WithContext merges the supplied map into the request context. Keys
// in the map win over anything set earlier in the option chain.
func WithContext(m map[string]any) Option {
	return func(req *DecisionRequest) {
		if req.Context == nil {
			req.Context = map[string]any{}
		}
		for k, v := range m {
			req.Context[k] = v
		}
	}
}

// WithProfitGuardDecisionID stamps the ProfitGuard decision into the
// context so deploy_approval.rego can require it. ProfitGuard decides
// "should we pay for this?"; policy decides "is it allowed?".
func WithProfitGuardDecisionID(id string) Option {
	return WithContextKV("profitguard_decision_id", id)
}

// WithGateState attaches the finisher-gate verdict map (security/test/
// deploy => pass|fail|waived) so deploy bundles can require pass.
func WithGateState(gates map[string]string) Option {
	cp := make(map[string]any, len(gates))
	for k, v := range gates {
		cp[k] = v
	}
	return WithContextKV("gate_state", cp)
}

// Allow evaluates the action against the PDP and returns the Decision.
// Callers can inspect Obligations and choose how to honour them; if
// they just want a hard yes/no, MustAllow is the simpler path.
//
// Allow does NOT return ErrPolicyDeny on deny — it returns the
// Decision and a nil error. Use the Effect to branch.
func (p *PEP) Allow(ctx context.Context, action string, resource Resource, opts ...Option) (Decision, error) {
	req := p.buildRequest(ctx, action, resource, opts...)
	dec, err := p.pdp.Decide(ctx, req)
	if err != nil {
		// On eval error the PDP already returned a deny; surface both.
		p.log.Warn().Err(err).
			Str("action", action).
			Str("decision_id", dec.DecisionID).
			Msg("policy evaluation error")
		return dec, err
	}
	if dec.Effect == EffectDeny {
		p.log.Info().
			Str("action", action).
			Str("decision_id", dec.DecisionID).
			Str("reason", dec.Reason).
			Str("risk", dec.Risk).
			Msg("policy deny")
	}
	return dec, nil
}

// MustAllow is the strict variant: it returns ErrPolicyDeny (wrapped
// in *DenyError) when the PDP denies. Use this at the call site that
// owns the side effect (provider dispatch, runtime exec, deploy
// start, secret release).
func (p *PEP) MustAllow(ctx context.Context, action string, resource Resource, opts ...Option) (Decision, error) {
	dec, err := p.Allow(ctx, action, resource, opts...)
	if err != nil {
		return dec, err
	}
	if dec.Effect != EffectAllow {
		return dec, &DenyError{Decision: dec}
	}
	return dec, nil
}

// buildRequest assembles a DecisionRequest from the auth-context
// principal plus the option overrides. The PEP defaults principal /
// delegation from ctx so callers can usually pass just (action,
// resource) and the right identity is attached automatically.
func (p *PEP) buildRequest(ctx context.Context, action string, resource Resource, opts ...Option) DecisionRequest {
	req := DecisionRequest{
		Action:   action,
		Resource: resource,
		Context:  map[string]any{},
	}
	// Default principal from auth context.
	if u, ok := auth.FromContext(ctx); ok {
		req.Principal = Principal{
			Kind:     "user",
			UserID:   u.ID,
			TenantID: u.OrgID,
			MFA:      u.MfaEnabled,
		}
		// Resource tenant defaults to principal tenant unless caller
		// pinned it explicitly via WithContext / direct field set.
		if req.Resource.TenantID == "" {
			req.Resource.TenantID = u.OrgID
		}
	} else {
		// No authenticated user — treat as anonymous. Rego bundles
		// will deny anything sensitive.
		req.Principal = Principal{Kind: "anonymous"}
	}
	for _, opt := range opts {
		opt(&req)
	}
	return req
}
