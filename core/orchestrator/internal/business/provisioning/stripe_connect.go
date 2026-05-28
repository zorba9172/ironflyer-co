package provisioning

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/pkg/httpclient"
)

// StripeConnectOpts wires the Stripe Connect connector. SecretKey is
// the Ironflyer *platform* secret (not a connected-account key) — Stripe
// Connect calls use the platform key plus a `Stripe-Account` header
// when acting on behalf of the merchant.
//
// Default decisions:
//   - Account type is Standard. Standard accounts keep dispute /
//     compliance liability with the merchant, which is the right
//     posture for Ironflyer-as-issuer at this tier.
//   - application_fee_amount is computed via RevenuePolicy at the
//     RecordRevenue boundary — the connector never invents its own cut.
//
// We do NOT take a hard dependency on github.com/stripe/stripe-go to
// keep the orchestrator module light. Direct HTTP against
// api.stripe.com mirrors how wallet/stripe.go talks to Checkout —
// same auth header, same idempotency-key contract.
type StripeConnectOpts struct {
	// SecretKey is the Ironflyer platform secret (sk_*). Empty disables
	// the connector — Enabled() returns false and Provision returns a
	// disabled error so dev environments without Stripe still boot.
	SecretKey string

	// WebhookSecret signs Stripe Connect events (application_fee.created,
	// charge.succeeded with on_behalf_of, account.updated). The wallet
	// webhook uses a separate secret because Stripe issues one secret
	// per webhook endpoint and the Connect events live under a distinct
	// endpoint.
	WebhookSecret string

	// ReturnURL / RefreshURL are the Stripe Connect AccountLinks
	// post-onboarding redirects. Both fall back to localhost dev URLs
	// when empty so the connector keeps producing valid AccountLinks
	// in dev boots.
	ReturnURL  string
	RefreshURL string

	// HTTPClient lets callers swap in a custom transport. Nil falls
	// back to the standard 15s-timeout client from pkg/httpclient.
	HTTPClient *http.Client

	// Policies is the live RevenuePolicy lookup. application_fee_amount
	// per charge is `ApplyPolicy(charge.amount, Policies.Get(stripe-connect))`
	// — the connector never hard-codes a percentage.
	Policies PolicyStore
}

// StripeConnect is the Connector implementation for Stripe Standard
// accounts. Satisfies Connector.
type StripeConnect struct {
	opts StripeConnectOpts
	http *http.Client
}

// NewStripeConnect constructs a StripeConnect connector. Empty opts
// produce a disabled connector — Provision / RecordRevenue / Handle
// Webhook all surface ErrConnectorDisabled until SecretKey is set.
func NewStripeConnect(opts StripeConnectOpts) *StripeConnect {
	if opts.HTTPClient == nil {
		opts.HTTPClient = httpclient.Standard(15 * time.Second)
	}
	if opts.ReturnURL == "" {
		opts.ReturnURL = "http://localhost:3000/app/integrations?stripe=connected"
	}
	if opts.RefreshURL == "" {
		opts.RefreshURL = "http://localhost:3000/app/integrations?stripe=refresh"
	}
	return &StripeConnect{opts: opts, http: opts.HTTPClient}
}

// Name implements Connector.
func (s *StripeConnect) Name() string { return KindStripeConnect }

// Label implements Connector.
func (s *StripeConnect) Label() string { return "Stripe payments (Connect Standard)" }

// Enabled implements Connector.
func (s *StripeConnect) Enabled() bool { return s != nil && s.opts.SecretKey != "" }

// Provision implements Connector. Creates a Standard Connect account
// for the merchant and an AccountLinks onboarding URL. We stash the
// (tenant, project) pair in the account's metadata so the webhook can
// route events back without a side table lookup. The returned
// ProvisionedResource carries the Stripe account id (acct_*) as
// ExternalID; the resolver bundles the onboarding URL into its own
// response shape because URLs are time-limited and not durable.
func (s *StripeConnect) Provision(ctx context.Context, tenant, project string, opts ProvisionOptions) (ProvisionedResource, error) {
	if !s.Enabled() {
		return ProvisionedResource{}, ErrConnectorDisabled
	}
	form := url.Values{}
	form.Set("type", "standard")
	form.Set("metadata[ironflyer_tenant]", tenant)
	form.Set("metadata[ironflyer_project]", project)
	for k, v := range opts.Metadata {
		form.Set("metadata["+k+"]", v)
	}
	acctID, err := s.stripePOST(ctx, "/v1/accounts", form, "")
	if err != nil {
		return ProvisionedResource{}, fmt.Errorf("provisioning: stripe connect account: %w", err)
	}
	return ProvisionedResource{
		TenantID:   tenant,
		ProjectID:  project,
		Kind:       KindStripeConnect,
		ExternalID: acctID,
		Status:     StatusPending,
	}, nil
}

// CreateOnboardingLink mints a fresh AccountLinks URL for the
// previously-Provisioned account. Exposed separately from Provision
// because AccountLinks expire after ~5 minutes; the resolver re-mints
// on every visit to the integration page so the URL is always live.
func (s *StripeConnect) CreateOnboardingLink(ctx context.Context, resource ProvisionedResource) (string, error) {
	if !s.Enabled() {
		return "", ErrConnectorDisabled
	}
	form := url.Values{}
	form.Set("account", resource.ExternalID)
	form.Set("type", "account_onboarding")
	form.Set("return_url", s.opts.ReturnURL)
	form.Set("refresh_url", s.opts.RefreshURL)
	urlStr, err := s.stripePOSTField(ctx, "/v1/account_links", form, "", "url")
	if err != nil {
		return "", fmt.Errorf("provisioning: stripe account_links: %w", err)
	}
	return urlStr, nil
}

// CreateTransfer issues a Stripe Transfer of amountUSD from the
// platform balance to the connected account `destinationAcct`
// (acct_*). This is the payout rail the Finisher Guild uses to pay a
// finisher their cut once a task is accepted.
//
// idempotencyKey dedupes retries: Stripe guarantees that two requests
// with the same Idempotency-Key produce one transfer, so a guild
// payout retry (or a Temporal activity replay) never double-pays. The
// caller passes the guild payout id so the key is stable across
// restarts.
//
// Returns the Stripe transfer id (tr_*) on success.
func (s *StripeConnect) CreateTransfer(ctx context.Context, destinationAcct string, amountUSD decimal.Decimal, idempotencyKey string) (string, error) {
	if !s.Enabled() {
		return "", ErrConnectorDisabled
	}
	if destinationAcct == "" {
		return "", errors.New("provisioning: stripe transfer missing destination account")
	}
	if !amountUSD.IsPositive() {
		return "", ErrInvalidAmount
	}
	// Stripe amounts are in the smallest currency unit (cents).
	cents := amountUSD.Mul(decimal.NewFromInt(100)).Round(0).IntPart()
	form := url.Values{}
	form.Set("amount", strconv.FormatInt(cents, 10))
	form.Set("currency", "usd")
	form.Set("destination", destinationAcct)

	// Build the request directly (not via stripePOST) so we can pin a
	// caller-supplied Idempotency-Key instead of a random one — the
	// whole point of a payout transfer is replay safety.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.stripe.com/v1/transfers", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.opts.SecretKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if idempotencyKey == "" {
		idempotencyKey = "ironflyer-payout-" + uuid.NewString()
	}
	req.Header.Set("Idempotency-Key", idempotencyKey)
	resp, err := s.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("stripe transfer %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("parse stripe transfer: %w", err)
	}
	if out.ID == "" {
		return "", errors.New("provisioning: stripe transfer response missing id")
	}
	return out.ID, nil
}

// RecordRevenue implements Connector. Reads application_fees from
// Stripe for the connected account since the last sweep. The cron in
// reconcile.go drives this; the webhook path is the primary, faster
// channel — this method exists so a missed webhook still lands the
// cut on the next sweep.
//
// We page through application_fees with limit=100 and trust the
// idempotency on RecordRevenue to short-circuit duplicates already
// landed via webhook. Stripe's list endpoint orders newest-first; we
// bail on the first row whose id we already recorded — but to keep
// the connector stateless, that check happens at the Service layer
// via ErrDuplicateEvent.
func (s *StripeConnect) RecordRevenue(ctx context.Context, resource ProvisionedResource) ([]RevenueEvent, error) {
	if !s.Enabled() {
		return nil, ErrConnectorDisabled
	}
	if resource.ExternalID == "" {
		return nil, errors.New("provisioning: stripe connect resource missing external id")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.stripe.com/v1/application_fees?limit=100", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.opts.SecretKey)
	// `Stripe-Account` scopes the list to the connected account.
	req.Header.Set("Stripe-Account", resource.ExternalID)
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("stripe %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var page struct {
		Data []struct {
			ID       string `json:"id"`
			Amount   int64  `json:"amount"`
			Currency string `json:"currency"`
			Charge   string `json:"charge"`
			Created  int64  `json:"created"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("parse stripe application_fees: %w", err)
	}
	out := make([]RevenueEvent, 0, len(page.Data))
	for _, fee := range page.Data {
		// application_fee.amount is already the Ironflyer cut (the
		// platform fee, in cents). Gross == amount/0.0X is not
		// reliably reconstructable here — the wallet records the cut
		// directly and leaves Gross as the fee amount divided by the
		// active SharePct as a best-effort estimate so the cockpit
		// can still render "$X gross routed through the rail".
		cut := decimal.NewFromInt(fee.Amount).Div(decimal.NewFromInt(100))
		gross := cut
		if s.opts.Policies != nil {
			if pol, err := s.opts.Policies.Get(KindStripeConnect); err == nil && pol.SharePct.IsPositive() {
				gross = cut.Div(pol.SharePct)
			}
		}
		out = append(out, RevenueEvent{
			ResourceID:      resource.ID,
			OccurredAt:      time.Unix(fee.Created, 0).UTC(),
			GrossAmountUSD:  gross,
			IronflyerCutUSD: cut,
			ExternalRef:     fee.ID,
		})
	}
	return out, nil
}

// HandleWebhook implements Connector. Verifies the Stripe Connect
// webhook signature against opts.WebhookSecret, and extracts the
// application_fee.created event shape into a RevenueEvent. Other
// event types (account.updated, charge.succeeded) return (nil, nil)
// so the route returns 200 OK without writing a revenue row.
func (s *StripeConnect) HandleWebhook(ctx context.Context, rawBody []byte, signatureHeader string) (*RevenueEvent, error) {
	if s.opts.WebhookSecret == "" {
		return nil, errors.New("provisioning: stripe connect webhook secret not configured")
	}
	ev, err := verifyConnectSignature(rawBody, signatureHeader, s.opts.WebhookSecret, 5*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("provisioning: webhook signature: %w", err)
	}
	if ev.Type != "application_fee.created" {
		// Non-revenue events are still useful telemetry (e.g.
		// account.updated for activation), but they don't produce a
		// RevenueEvent and the route should still 200.
		return nil, nil
	}
	obj := ev.Data.Object
	feeID, _ := obj["id"].(string)
	if feeID == "" {
		return nil, errors.New("provisioning: webhook missing application fee id")
	}
	var amountCents int64
	switch v := obj["amount"].(type) {
	case float64:
		amountCents = int64(v)
	case json.Number:
		amountCents, _ = v.Int64()
	}
	if amountCents <= 0 {
		return nil, ErrInvalidAmount
	}
	created := time.Now().UTC()
	if v, ok := obj["created"].(float64); ok {
		created = time.Unix(int64(v), 0).UTC()
	}
	cut := decimal.NewFromInt(amountCents).Div(decimal.NewFromInt(100))
	// Gross is reconstructable from the originating charge for a true
	// 1:1 cut/gross pairing. Without a second round-trip to fetch the
	// charge we approximate gross via the active policy — same fallback
	// the cron uses in RecordRevenue.
	gross := cut
	if s.opts.Policies != nil {
		if pol, err := s.opts.Policies.Get(KindStripeConnect); err == nil && pol.SharePct.IsPositive() {
			gross = cut.Div(pol.SharePct)
		}
	}
	return &RevenueEvent{
		// ResourceID is filled in by wireup — the webhook handler in
		// httpapi/api.go resolves the connected account id (from
		// ev.Account, set by Stripe on Connect events) back to the
		// ProvisionedResource row before persisting.
		OccurredAt:      created,
		GrossAmountUSD:  gross,
		IronflyerCutUSD: cut,
		ExternalRef:     feeID,
	}, nil
}

// Suspend implements Connector. Stripe Standard accounts cannot be
// "suspended" via the API in the same way platform-controlled accounts
// can — the best we can do is reject_account, which permanently flags
// the account. We take the safe path: mark the row suspended locally
// and surface a one-line operator hint in the error. Re-enabling is a
// manual Stripe Dashboard action.
func (s *StripeConnect) Suspend(_ context.Context, _ ProvisionedResource) error {
	if !s.Enabled() {
		return ErrConnectorDisabled
	}
	// Intentionally a no-op against Stripe — local suspension is
	// enforced by the orchestrator (refusing to forward new payment
	// attempts to the rail). Operators close accounts via the Stripe
	// Dashboard rejection flow when offboarding for real.
	return nil
}

// stripePOST is the small Stripe helper — POSTs form, parses {id, ...}
// from the JSON response, returns id. accountID is passed as the
// Stripe-Account header when non-empty (used for on-behalf-of calls).
func (s *StripeConnect) stripePOST(ctx context.Context, path string, form url.Values, accountID string) (string, error) {
	return s.stripePOSTField(ctx, path, form, accountID, "id")
}

// stripePOSTField is stripePOST plus the ability to pluck a different
// top-level field from the response (e.g. "url" for /v1/account_links).
func (s *StripeConnect) stripePOSTField(ctx context.Context, path string, form url.Values, accountID, field string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.stripe.com"+path, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.opts.SecretKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Idempotency-Key", "ironflyer-connect-"+uuid.NewString())
	if accountID != "" {
		req.Header.Set("Stripe-Account", accountID)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("stripe %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var generic map[string]any
	if err := json.Unmarshal(body, &generic); err != nil {
		return "", fmt.Errorf("parse stripe response: %w", err)
	}
	v, ok := generic[field].(string)
	if !ok || v == "" {
		return "", fmt.Errorf("stripe response missing field %q", field)
	}
	return v, nil
}

// stripeConnectEvent is the minimal Event shape we decode for Connect
// webhooks. Account is set by Stripe on Connect deliveries — that's
// how the platform endpoint distinguishes which connected account the
// event belongs to.
type stripeConnectEvent struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Account string `json:"account"`
	Data    struct {
		Object map[string]any `json:"object"`
	} `json:"data"`
}

// verifyConnectSignature mirrors verifyStripeSignature in wallet/stripe.go.
// Stripe Connect webhooks use the same Stripe-Signature header format,
// just with a different signing secret per endpoint.
func verifyConnectSignature(rawBody []byte, header, secret string, tolerance time.Duration) (stripeConnectEvent, error) {
	var ts string
	sigs := make([]string, 0, 2)
	for _, part := range strings.Split(header, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			ts = kv[1]
		case "v1":
			sigs = append(sigs, kv[1])
		}
	}
	if ts == "" || len(sigs) == 0 {
		return stripeConnectEvent{}, errors.New("malformed Stripe-Signature header")
	}
	tsInt, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return stripeConnectEvent{}, fmt.Errorf("bad timestamp: %w", err)
	}
	if tolerance > 0 {
		delta := time.Since(time.Unix(tsInt, 0))
		if delta > tolerance || delta < -tolerance {
			return stripeConnectEvent{}, errors.New("timestamp outside tolerance")
		}
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(rawBody)
	expected := hex.EncodeToString(mac.Sum(nil))
	match := false
	for _, sig := range sigs {
		if hmac.Equal([]byte(sig), []byte(expected)) {
			match = true
			break
		}
	}
	if !match {
		return stripeConnectEvent{}, errors.New("signature mismatch")
	}
	var ev stripeConnectEvent
	if err := json.Unmarshal(rawBody, &ev); err != nil {
		return stripeConnectEvent{}, fmt.Errorf("parse event: %w", err)
	}
	return ev, nil
}

// ConnectedAccountFromWebhook extracts the connected-account id from
// the raw body. Exposed so the HTTP handler can resolve which
// ProvisionedResource the inbound event belongs to BEFORE asking the
// connector to parse the body — keeps the routing logic in one place.
func ConnectedAccountFromWebhook(rawBody []byte) string {
	var peek struct {
		Account string `json:"account"`
	}
	if err := json.Unmarshal(rawBody, &peek); err != nil {
		return ""
	}
	return peek.Account
}

var _ Connector = (*StripeConnect)(nil)
