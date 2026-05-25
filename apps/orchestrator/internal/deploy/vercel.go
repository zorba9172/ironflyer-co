package deploy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
)

// Default Vercel cost-attribution rates. These are approximate — real
// Vercel billing is project-level (concurrent builds, function GB-Hr,
// edge invocations) and never per-deployment. The orchestrator surfaces
// a best-effort per-deploy line so dashboards see a number; the
// platform reconciliation job in finance/ trues this up against the
// monthly Vercel invoice.
//
// Defaults map roughly to "$0.36/hour of build time" and "$0.01 per
// production promote"; operators tune via env without rebuilding:
//
//	IRONFLYER_VERCEL_BUILD_RATE_USD_PER_SEC   (default 0.0001)
//	IRONFLYER_VERCEL_PROMOTE_FLAT_USD         (default 0.01)
const (
	defaultVercelBuildRateUSDPerSec = 0.0001
	defaultVercelPromoteFlatUSD     = 0.01
)

// VercelAdapter is the V22 Vercel REST v1 Adapter. It speaks the
// documented public API only (no third-party SDK) so the dependency
// surface stays auditable.
//
// Auth: a per-tenant VERCEL_TOKEN, resolved through SecretResolver
// on every call. The token is never cached past the in-flight call.
//
// Idempotency: every POST sets an `Idempotency-Key` header per the
// V22 deploy lifecycle plan, derived deterministically from the
// deployID + endpoint so retries collapse on the Vercel side.
//
// Cost: Vercel does not return a per-deployment cost number on the
// REST surface; the adapter reports decimal.Zero and the Service
// records nothing extra. The integration agent layers BillingGuard
// /tick reporting on top if/when cost attribution is desired.
type VercelAdapter struct {
	secrets SecretResolver
	client  *http.Client
	base    string // https://api.vercel.com
	log     zerolog.Logger
}

// NewVercelAdapter wires the adapter with the supplied dependencies.
// A nil client falls back to a 30s-timeout http.Client; an empty
// base falls back to https://api.vercel.com.
func NewVercelAdapter(secrets SecretResolver, client *http.Client, base string, log zerolog.Logger) *VercelAdapter {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if base == "" {
		base = "https://api.vercel.com"
	}
	return &VercelAdapter{secrets: secrets, client: client, base: base, log: log}
}

// Name returns the Target key.
func (*VercelAdapter) Name() string { return string(TargetVercel) }

// Plan returns a no-side-effect plan. We don't pre-create the Vercel
// project here — the project must already exist on the Vercel side
// (referenced via metadata["vercel_project_id"]). When that hint is
// missing we fall back to deriving a project name from PlanInput.
func (a *VercelAdapter) Plan(_ context.Context, p PlanInput) (PlanResult, error) {
	projectID := metaString(p.Metadata, "vercel_project_id")
	if projectID == "" {
		projectID = metaString(p.Metadata, "vercel_project_name")
	}
	if projectID == "" {
		projectID = fmt.Sprintf("ironflyer-%s", shortHash(p.ProjectID))
	}
	notes := []string{
		"vercel adapter v1",
		"cost attribution flows through BillingGuard, not the Vercel REST surface",
	}
	if p.Environment == EnvironmentProduction {
		notes = append(notes, "production deploy: approval row required before Promote")
	}
	return PlanResult{
		ProviderProjectID: projectID,
		EstimatedCostUSD:  decimal.Zero,
		Notes:             notes,
	}, nil
}

// BuildPreview calls POST /v13/deployments with target=preview.
// We submit a minimal envelope — the artifact bundle itself is
// uploaded out-of-band by the runtime / preview builder; this call
// triggers the build under the supplied project + git ref.
func (a *VercelAdapter) BuildPreview(ctx context.Context, deployID string, plan PlanResult) (PreviewResult, error) {
	body := map[string]any{
		"name":        plan.ProviderProjectID,
		"target":      "preview",
		"projectName": plan.ProviderProjectID,
		"meta": map[string]string{
			"ironflyer_deploy_id": deployID,
		},
	}
	var resp vercelDeploymentResponse
	if err := a.doJSON(ctx, http.MethodPost,
		"/v13/deployments",
		idempotencyKey(deployID, "preview"),
		body, &resp,
	); err != nil {
		return PreviewResult{}, err
	}
	url := resp.URL
	if url != "" && !strings.HasPrefix(url, "http") {
		url = "https://" + url
	}
	cost := a.fetchBuildCost(ctx, resp.ID)
	return PreviewResult{
		ProviderDeploymentID: resp.ID,
		PreviewURL:           url,
		CostUSD:              cost,
	}, nil
}

// Promote calls POST /v13/deployments/{id}/promote. Vercel returns
// the production URL alias in the project payload; we surface what
// the promote call echoes back.
func (a *VercelAdapter) Promote(ctx context.Context, deployID, providerDeploymentID string) (PromoteResult, error) {
	if providerDeploymentID == "" {
		return PromoteResult{}, fmt.Errorf("%w: missing provider deployment id", ErrInvalidState)
	}
	var resp vercelDeploymentResponse
	if err := a.doJSON(ctx, http.MethodPost,
		fmt.Sprintf("/v13/deployments/%s/promote", providerDeploymentID),
		idempotencyKey(deployID, "promote"),
		map[string]any{},
		&resp,
	); err != nil {
		return PromoteResult{}, err
	}
	url := resp.URL
	if url == "" && len(resp.Alias) > 0 {
		url = resp.Alias[0]
	}
	if url != "" && !strings.HasPrefix(url, "http") {
		url = "https://" + url
	}
	cost := a.fetchBuildCost(ctx, providerDeploymentID).Add(promoteFlatUSD())
	return PromoteResult{
		ProductionURL: url,
		CostUSD:       cost,
	}, nil
}

// fetchBuildCost asks Vercel for the deployment metadata, computes
// build duration from buildingAt → ready, and applies the configured
// per-second rate. Failure is intentionally non-fatal — the deploy
// already happened; missing cost data is a dashboards-only loss and
// must NEVER cause the caller to fail the deploy.
//
// Real Vercel billing is project-level (concurrent builds, function
// GB-Hr, edge invocations), not per-deployment. This helper produces
// an approximate per-deploy line so dashboards see a number; the
// finance reconciliation job trues this up against the monthly Vercel
// invoice.
func (a *VercelAdapter) fetchBuildCost(ctx context.Context, providerDeploymentID string) decimal.Decimal {
	if providerDeploymentID == "" {
		return decimal.Zero
	}
	var resp vercelDeploymentMetadata
	if err := a.doJSON(ctx, http.MethodGet,
		"/v13/deployments/"+providerDeploymentID,
		"", nil, &resp,
	); err != nil {
		a.log.Warn().
			Err(err).
			Str("provider_deployment_id", providerDeploymentID).
			Msg("vercel: cost lookup failed (approximate cost will be zero)")
		return decimal.Zero
	}
	durationSec := buildDurationSeconds(resp.BuildingAt, resp.Ready)
	if durationSec <= 0 {
		return decimal.Zero
	}
	rate := buildRateUSDPerSec()
	if rate <= 0 {
		return decimal.Zero
	}
	cost := decimal.NewFromFloat(rate).Mul(decimal.NewFromFloat(durationSec))
	return cost
}

// vercelDeploymentMetadata is the trimmed GET /v13/deployments/{id}
// payload we read for cost attribution. Vercel returns timestamps in
// milliseconds-since-epoch on this surface.
type vercelDeploymentMetadata struct {
	ID         string `json:"id"`
	BuildingAt int64  `json:"buildingAt"`
	Ready      int64  `json:"ready"`
}

// buildDurationSeconds converts (buildingAt, ready) millisecond
// timestamps into a float seconds value. Returns 0 when either bound
// is missing or the duration is negative.
func buildDurationSeconds(buildingAtMs, readyMs int64) float64 {
	if buildingAtMs <= 0 || readyMs <= 0 || readyMs <= buildingAtMs {
		return 0
	}
	return float64(readyMs-buildingAtMs) / 1000.0
}

// buildRateUSDPerSec resolves the per-second build cost rate. The env
// override lets operators tune Vercel billing reality without a
// rebuild; an unparseable / negative value falls back to the default
// rather than disabling cost attribution entirely.
func buildRateUSDPerSec() float64 {
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_VERCEL_BUILD_RATE_USD_PER_SEC")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			return f
		}
	}
	return defaultVercelBuildRateUSDPerSec
}

// promoteFlatUSD resolves the flat per-promote fee. Same fallback
// semantics as buildRateUSDPerSec.
func promoteFlatUSD() decimal.Decimal {
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_VERCEL_PROMOTE_FLAT_USD")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			return decimal.NewFromFloat(f)
		}
	}
	return decimal.NewFromFloat(defaultVercelPromoteFlatUSD)
}

// Rollback uses Vercel's documented alias-rollback path. We POST to
// /v13/deployments/{id}/rollback with the target version when
// supplied; an empty toVersion lets Vercel revert to the previous
// successful production deploy.
func (a *VercelAdapter) Rollback(ctx context.Context, deployID, providerDeploymentID, toVersion string) (RollbackResult, error) {
	if providerDeploymentID == "" {
		return RollbackResult{}, fmt.Errorf("%w: missing provider deployment id", ErrInvalidState)
	}
	body := map[string]any{}
	if toVersion != "" {
		body["toVersion"] = toVersion
	}
	var resp vercelDeploymentResponse
	if err := a.doJSON(ctx, http.MethodPost,
		fmt.Sprintf("/v13/deployments/%s/rollback", providerDeploymentID),
		idempotencyKey(deployID, "rollback"),
		body, &resp,
	); err != nil {
		return RollbackResult{}, err
	}
	out := toVersion
	if out == "" {
		out = resp.ID
	}
	return RollbackResult{ToVersion: out}, nil
}

// doJSON is the shared request helper. It resolves the per-tenant
// VERCEL_TOKEN on every call, attaches the Idempotency-Key header,
// JSON-marshals the body, and decodes 2xx responses into out.
func (a *VercelAdapter) doJSON(ctx context.Context, method, path, idemKey string, body, out any) error {
	if a.secrets == nil {
		return fmt.Errorf("%w: secret resolver not configured", ErrSecretMissing)
	}
	// Vercel tokens are per-tenant in our model — the SecretResolver
	// implementation reads tenant from the caller's context. We don't
	// have explicit tenant/project args here; the integration agent
	// adapts SecretResolver around context propagation so callers
	// upstream of doJSON pass the relevant ids via ctx values.
	token, err := a.secrets.Resolve(ctx, tenantFromContext(ctx), projectFromContext(ctx), "VERCEL_TOKEN")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSecretMissing, err)
	}
	if len(token) == 0 {
		return ErrSecretMissing
	}

	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("vercel: marshal body: %w", err)
		}
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.base+path, rdr)
	if err != nil {
		return fmt.Errorf("vercel: new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(token)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrProviderFailure, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		buf, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		a.log.Warn().
			Int("status", resp.StatusCode).
			Str("method", method).
			Str("path", path).
			Bytes("body", buf).
			Msg("vercel api error")
		return fmt.Errorf("%w: status=%d body=%s", ErrProviderFailure, resp.StatusCode, string(buf))
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("vercel: decode response: %w", err)
	}
	return nil
}

// vercelDeploymentResponse is the trimmed subset of the v13 deployment
// payload the adapter reads. We deliberately don't model every field
// — only what the Service needs to fill the durable row.
type vercelDeploymentResponse struct {
	ID    string   `json:"id"`
	URL   string   `json:"url"`
	Alias []string `json:"alias"`
}

// idempotencyKey is the deterministic per-(deployID, endpoint) key
// Vercel's Idempotency-Key header consumes. Using sha256 over the
// inputs guarantees stable retries; the prefix keeps the key
// human-readable in audit logs.
func idempotencyKey(deployID, endpoint string) string {
	sum := sha256.Sum256([]byte(deployID + "|" + endpoint))
	return "ironflyer-" + endpoint + "-" + hex.EncodeToString(sum[:8])
}

// shortHash returns the first 8 hex chars of sha256(input) — used to
// build deterministic provider-project names when callers don't pass
// an explicit vercel_project_id hint.
func shortHash(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:4])
}

// metaString safely reads a string from PlanInput.Metadata.
func metaString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// Context keys for tenant / project propagation. The integration
// agent populates these on the per-call ctx before invoking the
// Service so the Vercel adapter can resolve the right secret without
// passing extra arguments through the Adapter contract.
type ctxKey string

const (
	ctxKeyTenant  ctxKey = "ironflyer.deploy.tenant"
	ctxKeyProject ctxKey = "ironflyer.deploy.project"
)

// WithTenant returns a ctx with the tenant id attached. Service
// implementations call this before invoking an Adapter so the
// Vercel SecretResolver call sees the right tenant.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, ctxKeyTenant, tenantID)
}

// WithProject mirrors WithTenant for project id.
func WithProject(ctx context.Context, projectID string) context.Context {
	return context.WithValue(ctx, ctxKeyProject, projectID)
}

func tenantFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyTenant).(string)
	return v
}

func projectFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyProject).(string)
	return v
}
