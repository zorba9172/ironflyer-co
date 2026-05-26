package policy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/pkg/httpclient"
)

// RemotePDP talks to an OPA sidecar / cluster service over HTTP.
// Configured via IRONFLYER_OPA_REMOTE_URL pointing at an instance that
// exposes data.ironflyer.decision (POST /v1/data/ironflyer/decision).
//
// This implementation deliberately ships a fixed 5s timeout and a
// shared http.Client; per-call overrides go through ctx.
type RemotePDP struct {
	endpoint string
	client   *http.Client
	version  string
	log      zerolog.Logger
	auditor  *Auditor
}

var _ PDP = (*RemotePDP)(nil)

// NewRemotePDP wires a sidecar PDP. baseURL is the OPA root (e.g.
// http://localhost:8181); the constructor appends the canonical decision
// path. version is supplied by the caller because the remote OPA owns
// its own bundle lifecycle; pass the bundle hash here for audit pinning.
func NewRemotePDP(baseURL, version string, auditor *Auditor, log zerolog.Logger) (*RemotePDP, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, fmt.Errorf("policy: RemotePDP requires a base URL")
	}
	return &RemotePDP{
		endpoint: strings.TrimRight(baseURL, "/") + "/v1/data/ironflyer/decision",
		client:   httpclient.Standard(5 * time.Second),
		version:  version,
		log:      log.With().Str("subsystem", "policy.remote").Logger(),
		auditor:  auditor,
	}, nil
}

// BundleVersion implements PDP.
func (p *RemotePDP) BundleVersion() string { return p.version }

// Decide implements PDP.
func (p *RemotePDP) Decide(ctx context.Context, req DecisionRequest) (Decision, error) {
	decisionID := "pdec_" + uuid.NewString()
	payload := map[string]any{"input": buildInput(req)}
	body, err := json.Marshal(payload)
	if err != nil {
		dec := p.denyDecision(decisionID, "pdp_marshal_error")
		p.emitAudit(ctx, req, dec, err)
		return dec, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		dec := p.denyDecision(decisionID, "pdp_request_error")
		p.emitAudit(ctx, req, dec, err)
		return dec, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		dec := p.denyDecision(decisionID, "pdp_unreachable")
		p.emitAudit(ctx, req, dec, err)
		return dec, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		dec := p.denyDecision(decisionID, fmt.Sprintf("pdp_http_%d", resp.StatusCode))
		p.emitAudit(ctx, req, dec, fmt.Errorf("opa status %d", resp.StatusCode))
		return dec, fmt.Errorf("policy: opa http %d", resp.StatusCode)
	}

	var envelope struct {
		Result map[string]any `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		dec := p.denyDecision(decisionID, "pdp_decode_error")
		p.emitAudit(ctx, req, dec, err)
		return dec, err
	}

	dec := decisionFromMap(envelope.Result, decisionID, p.version)
	p.emitAudit(ctx, req, dec, nil)
	return dec, nil
}

func (p *RemotePDP) denyDecision(id, reason string) Decision {
	return Decision{
		DecisionID:          id,
		Effect:              EffectDeny,
		Risk:                RiskHigh,
		Reason:              reason,
		PolicyBundleVersion: p.version,
	}
}

func (p *RemotePDP) emitAudit(ctx context.Context, req DecisionRequest, dec Decision, evalErr error) {
	if p.auditor == nil {
		return
	}
	if err := p.auditor.Record(ctx, req, dec, evalErr); err != nil {
		p.log.Error().Err(err).Str("decision_id", dec.DecisionID).Msg("audit write failed")
	}
}

// decisionFromMap mirrors parseDecision but takes a plain map (the
// remote PDP returns JSON, not a rego.ResultSet).
func decisionFromMap(raw map[string]any, decisionID, version string) Decision {
	dec := Decision{
		DecisionID:          decisionID,
		Effect:              EffectDeny,
		Risk:                RiskHigh,
		Reason:              "pdp_no_result",
		PolicyBundleVersion: version,
	}
	if raw == nil {
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
	switch v := raw["ttl_seconds"].(type) {
	case float64:
		dec.TTLSeconds = int(v)
	case int:
		dec.TTLSeconds = v
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
