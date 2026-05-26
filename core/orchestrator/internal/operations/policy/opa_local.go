package policy

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/open-policy-agent/opa/rego"
	"github.com/rs/zerolog"
)

// LocalPDP evaluates Rego bundles in-process using the OPA Go SDK.
// A single PreparedEvalQuery is built at construction time and reused
// for every Decide call so per-request cost is just input
// marshalling + topdown evaluation.
type LocalPDP struct {
	bundles map[string]string
	version string
	log     zerolog.Logger
	auditor *Auditor

	mu      sync.RWMutex
	prepped rego.PreparedEvalQuery
}

// Compile-time interface assertion.
var _ PDP = (*LocalPDP)(nil)

// NewLocalPDP builds the in-process PDP from the supplied bundle set.
// The bundles map is (module name -> rego source); LoadBundles() in
// bundle.go is the usual producer.
//
// The constructor compiles and prepares the query `data.ironflyer.decision`
// once. A bundle that fails to parse is a hard error: returning a half-
// configured PDP would silently fall back to default-deny without the
// operator noticing.
func NewLocalPDP(bundles map[string]string, version string, auditor *Auditor, log zerolog.Logger) (*LocalPDP, error) {
	if len(bundles) == 0 {
		return nil, fmt.Errorf("policy: NewLocalPDP requires at least one bundle")
	}
	p := &LocalPDP{
		bundles: bundles,
		version: version,
		log:     log.With().Str("subsystem", "policy.local").Logger(),
		auditor: auditor,
	}
	if err := p.prepare(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *LocalPDP) prepare() error {
	opts := []func(*rego.Rego){
		rego.Query("data.ironflyer.decision"),
	}
	for name, src := range p.bundles {
		opts = append(opts, rego.Module(name+".rego", src))
	}
	r := rego.New(opts...)
	pq, err := r.PrepareForEval(context.Background())
	if err != nil {
		return fmt.Errorf("policy: prepare rego: %w", err)
	}
	p.mu.Lock()
	p.prepped = pq
	p.mu.Unlock()
	return nil
}

// BundleVersion implements PDP.
func (p *LocalPDP) BundleVersion() string { return p.version }

// Decide implements PDP. Every return path produces a Decision with a
// non-empty DecisionID — even errors — so the audit chain can pin the
// call.
func (p *LocalPDP) Decide(ctx context.Context, req DecisionRequest) (Decision, error) {
	decisionID := "pdec_" + uuid.NewString()
	input := buildInput(req)

	p.mu.RLock()
	pq := p.prepped
	p.mu.RUnlock()

	rs, err := pq.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		dec := Decision{
			DecisionID:          decisionID,
			Effect:              EffectDeny,
			Risk:                RiskHigh,
			Reason:              "pdp_eval_error",
			PolicyBundleVersion: p.version,
		}
		p.emitAudit(ctx, req, dec, err)
		return dec, fmt.Errorf("policy: eval %s: %w", req.Action, err)
	}

	dec := parseDecision(rs, decisionID, p.version)
	p.emitAudit(ctx, req, dec, nil)
	return dec, nil
}

func (p *LocalPDP) emitAudit(ctx context.Context, req DecisionRequest, dec Decision, evalErr error) {
	if p.auditor == nil {
		return
	}
	if err := p.auditor.Record(ctx, req, dec, evalErr); err != nil {
		// Audit failure must never fail the PEP path. We log loud and
		// keep going; the operator dashboard surfaces audit health.
		p.log.Error().Err(err).Str("decision_id", dec.DecisionID).Msg("audit write failed")
	}
}

// parseDecision converts a Rego ResultSet into a Decision. The bundle
// contract is data.ironflyer.decision = {effect, risk, reason,
// ttl_seconds, obligations}. Anything that fails to parse becomes a
// deny so a malformed bundle cannot accidentally allow.
func parseDecision(rs rego.ResultSet, decisionID, version string) Decision {
	dec := Decision{
		DecisionID:          decisionID,
		Effect:              EffectDeny,
		Risk:                RiskHigh,
		Reason:              "pdp_no_result",
		PolicyBundleVersion: version,
	}
	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		return dec
	}
	raw, ok := rs[0].Expressions[0].Value.(map[string]any)
	if !ok {
		return dec
	}
	if v, ok := raw["effect"].(string); ok {
		switch Effect(v) {
		case EffectAllow:
			dec.Effect = EffectAllow
		case EffectDeny:
			dec.Effect = EffectDeny
		}
	}
	if v, ok := raw["risk"].(string); ok && v != "" {
		dec.Risk = v
	}
	if v, ok := raw["reason"].(string); ok && v != "" {
		dec.Reason = v
	}
	// Rego numbers come back as json.Number for many call paths; the
	// rego SDK normalizes to float64 in EvalInput-fed inputs. Accept
	// both.
	switch v := raw["ttl_seconds"].(type) {
	case float64:
		dec.TTLSeconds = int(v)
	case int:
		dec.TTLSeconds = v
	case int64:
		dec.TTLSeconds = int(v)
	}
	if obls, ok := raw["obligations"].([]any); ok {
		for _, o := range obls {
			om, ok := o.(map[string]any)
			if !ok {
				continue
			}
			ob := Obligation{}
			if k, ok := om["kind"].(string); ok {
				ob.Kind = k
			}
			if p, ok := om["params"].(map[string]any); ok {
				ob.Params = p
			}
			if ob.Kind != "" {
				dec.Obligations = append(dec.Obligations, ob)
			}
		}
	}
	return dec
}
