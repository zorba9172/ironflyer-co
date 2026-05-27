package wallet

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

// SupportedTopUpAmounts is the closed set of top-up tiers Ironflyer
// sells. Keeping the menu fixed (vs. an arbitrary-amount field) lets us
// pre-price Stripe Tax and keeps the dashboard CTA short. Any amount
// the user might want above $500 routes through sales.
var SupportedTopUpAmounts = []decimal.Decimal{
	decimal.NewFromInt(10),
	decimal.NewFromInt(25),
	decimal.NewFromInt(50),
	decimal.NewFromInt(100),
	decimal.NewFromInt(250),
	decimal.NewFromInt(500),
}

// IsSupportedAmount returns true when amount is one of the fixed tiers.
// Equality is by decimal value, not text representation, so "100" and
// "100.000000" both match.
func IsSupportedAmount(amount decimal.Decimal) bool {
	for _, v := range SupportedTopUpAmounts {
		if v.Equal(amount) {
			return true
		}
	}
	return false
}

// StripeTopperOpts wires the Stripe checkout machinery to a backing
// Service. The orchestrator constructs one StripeTopper at startup;
// resolvers borrow the same instance for every checkout request.
type StripeTopperOpts struct {
	// SecretKey is the Stripe API secret used for the /v1/checkout/
	// sessions call. Empty disables the Topper — CreateCheckoutSession
	// returns an "stripe disabled" error so dev environments still
	// build.
	SecretKey string

	// WebhookSecret signs the inbound checkout.session.completed
	// events. Empty disables webhook verification (HandleWebhook
	// returns an error).
	WebhookSecret string

	// SuccessURL / CancelURL are the redirect targets Stripe hands
	// back to the browser. Both must be absolute URLs.
	SuccessURL string
	CancelURL  string

	// HTTPClient lets callers swap in a custom transport (test fakes,
	// hardened TLS configs). Nil falls back to a 15s timeout client.
	HTTPClient *http.Client
}

// TopperOpts is retained as an alias for backwards compatibility with
// callers that pre-date the multi-provider refactor (cmd/orchestrator,
// historical fixtures). New code should use StripeTopperOpts.
type TopperOpts = StripeTopperOpts

// StripeTopper turns Stripe Checkout sessions into wallet credits. It
// is the only thing in the package that talks to Stripe. Satisfies
// Topper.
type StripeTopper struct {
	opts    StripeTopperOpts
	service Service
	http    *http.Client
}

// NewStripeTopper constructs a Topper backed by the given Service.
func NewStripeTopper(service Service, opts StripeTopperOpts) *StripeTopper {
	if opts.SuccessURL == "" {
		opts.SuccessURL = "http://localhost:3000/app/wallet?stripe=success"
	}
	if opts.CancelURL == "" {
		opts.CancelURL = "http://localhost:3000/app/wallet?stripe=cancel"
	}
	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = httpclient.Standard(15 * time.Second)
	}
	return &StripeTopper{opts: opts, service: service, http: httpClient}
}

// NewTopper is retained as an alias for NewStripeTopper so existing
// wireup keeps compiling through the multi-provider refactor.
func NewTopper(service Service, opts StripeTopperOpts) *StripeTopper {
	return NewStripeTopper(service, opts)
}

// Name implements Topper.
func (t *StripeTopper) Name() string { return ProviderStripe }

// Label implements Topper.
func (t *StripeTopper) Label() string { return "Card (Stripe)" }

// Enabled is true iff a Stripe secret key is configured. Resolvers
// check this before exposing the walletCreateTopUp mutation so dev
// environments fail loud and clear.
func (t *StripeTopper) Enabled() bool { return t != nil && t.opts.SecretKey != "" }

// CreateCheckoutSession provisions a one-shot Stripe Checkout for the
// given tenant + amount. We use mode=payment (not subscription)
// because wallet top-ups are discrete prepaid credits, not a
// recurring entitlement. The tenant id and amount land in metadata so
// the webhook can route the credit back without a customer-table
// lookup. A wallet_topups row is staged in 'pending' state before we
// return; the webhook flips it to 'succeeded' on payment.
//
// We pass a fresh Idempotency-Key per call so a transient HTTP
// retry never opens a duplicate Stripe session for the same intent
// (Stripe folds repeated calls with the same key into one session
// for 24h).
func (t *StripeTopper) CreateCheckoutSession(ctx context.Context, tenant string, amountUSD decimal.Decimal) (CheckoutSession, error) {
	if !t.Enabled() {
		return CheckoutSession{}, errors.New("wallet: stripe disabled")
	}
	if !IsSupportedAmount(amountUSD) {
		return CheckoutSession{}, ErrInvalidAmount
	}
	form := url.Values{}
	form.Set("mode", "payment")
	form.Set("success_url", t.opts.SuccessURL)
	form.Set("cancel_url", t.opts.CancelURL)
	form.Set("client_reference_id", tenant)
	form.Set("metadata[tenant_id]", tenant)
	form.Set("metadata[amount_usd]", amountUSD.String())
	form.Set("metadata[purpose]", "wallet_topup")
	form.Set("metadata[provider]", ProviderStripe)
	// price_data lets us send the amount inline instead of pre-creating
	// a Price object per tier. unit_amount is in cents.
	cents := amountUSD.Mul(decimal.NewFromInt(100)).IntPart()
	form.Set("line_items[0][quantity]", "1")
	form.Set("line_items[0][price_data][currency]", "usd")
	form.Set("line_items[0][price_data][unit_amount]", strconv.FormatInt(cents, 10))
	form.Set("line_items[0][price_data][product_data][name]",
		fmt.Sprintf("Ironflyer wallet top-up — $%s", amountUSD.StringFixed(2)))
	// Tax collection: same flags as the subscription path so VAT/GST
	// behave consistently for B2B buyers.
	form.Set("automatic_tax[enabled]", "true")
	form.Set("tax_id_collection[enabled]", "true")
	form.Set("customer_creation", "if_required")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.stripe.com/v1/checkout/sessions",
		strings.NewReader(form.Encode()))
	if err != nil {
		return CheckoutSession{}, err
	}
	req.Header.Set("Authorization", "Bearer "+t.opts.SecretKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Stripe officially supports Idempotency-Key on all POST endpoints.
	// We use a per-call UUID; a transient retry of THIS exact call
	// (same request object) would reuse the key and not double-create.
	req.Header.Set("Idempotency-Key", "wallet-topup-"+tenant+"-"+uuid.NewString())
	resp, err := t.http.Do(req)
	if err != nil {
		return CheckoutSession{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return CheckoutSession{}, fmt.Errorf("stripe %d: %s",
			resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out struct {
		URL string `json:"url"`
		ID  string `json:"id"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return CheckoutSession{}, fmt.Errorf("parse stripe response: %w", err)
	}
	if out.URL == "" || out.ID == "" {
		return CheckoutSession{}, errors.New("stripe returned incomplete session")
	}
	// Stage the pending row so the webhook has something to flip.
	if _, err := t.service.CreatePendingTopUp(ctx, tenant, amountUSD, out.ID); err != nil {
		// Don't fail the user-visible request — the webhook will
		// recover via the synthetic-row fallback in TopUp.
		// Surfacing the error here would leave the user with a paid
		// Stripe session and no way to retry.
		return CheckoutSession{URL: out.URL, SessionID: out.ID, Provider: ProviderStripe}, nil
	}
	return CheckoutSession{URL: out.URL, SessionID: out.ID, Provider: ProviderStripe}, nil
}

// HandleWebhook verifies the Stripe-Signature header against the raw
// payload, decodes a checkout.session.completed event, and credits the
// wallet. The verify + crediting flow is idempotent — Stripe retries
// the same event ID on transient failures, and the UNIQUE constraint
// on wallet_topups.stripe_session_id deduplicates.
//
// signatureHeader is the value of the Stripe-Signature HTTP header.
// rawBody is the unmodified request body Stripe signed.
func (t *StripeTopper) HandleWebhook(ctx context.Context, rawBody []byte, signatureHeader string) error {
	if t.opts.WebhookSecret == "" {
		return errors.New("wallet: stripe webhook secret not configured")
	}
	ev, err := verifyStripeSignature(rawBody, signatureHeader, t.opts.WebhookSecret, 5*time.Minute)
	if err != nil {
		return fmt.Errorf("wallet: webhook signature: %w", err)
	}
	if ev.Type != "checkout.session.completed" {
		// Non-checkout events are ignored so adding new event
		// subscriptions to Stripe never fails our endpoint.
		return nil
	}
	obj := ev.Data.Object
	// Only credit sessions tagged purpose=wallet_topup. Other purposes
	// (subscription checkouts, etc.) flow through internal/budget.
	if meta, ok := obj["metadata"].(map[string]any); ok {
		if purpose, _ := meta["purpose"].(string); purpose != "" && purpose != "wallet_topup" {
			return nil
		}
	}
	sessionID, _ := obj["id"].(string)
	if sessionID == "" {
		return errors.New("wallet: webhook missing session id")
	}
	tenant := stripeTenant(obj)
	if tenant == "" {
		return errors.New("wallet: webhook missing tenant id")
	}
	amount, err := stripeAmount(obj)
	if err != nil {
		return fmt.Errorf("wallet: webhook amount: %w", err)
	}
	return t.service.TopUp(ctx, tenant, amount, sessionID)
}

// stripeEvent is the minimal Event shape we decode. Mirrors the
// internal/budget StripeEvent but lives here so wallet stays
// dependency-isolated from the legacy budget package.
type stripeEvent struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		Object map[string]any `json:"object"`
	} `json:"data"`
}

// verifyStripeSignature validates the Stripe-Signature header against
// rawBody using webhookSecret. Header format: `t=TS,v1=SIG[,v1=SIG]`.
// The signed payload is `TS.body`; HMAC-SHA256 with the secret must
// equal at least one of the v1 entries.
func verifyStripeSignature(rawBody []byte, header, secret string, tolerance time.Duration) (stripeEvent, error) {
	var ts string
	// Stripe currently rotates at most a handful of v1 entries per header.
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
		return stripeEvent{}, errors.New("malformed Stripe-Signature header")
	}
	tsInt, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return stripeEvent{}, fmt.Errorf("bad timestamp: %w", err)
	}
	if tolerance > 0 {
		delta := time.Since(time.Unix(tsInt, 0))
		if delta > tolerance || delta < -tolerance {
			return stripeEvent{}, errors.New("timestamp outside tolerance")
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
		return stripeEvent{}, errors.New("signature mismatch")
	}
	var ev stripeEvent
	if err := json.Unmarshal(rawBody, &ev); err != nil {
		return stripeEvent{}, fmt.Errorf("parse event: %w", err)
	}
	return ev, nil
}

// stripeTenant pulls the tenant id from the Checkout session object,
// preferring client_reference_id and falling back to metadata.tenant_id.
func stripeTenant(obj map[string]any) string {
	if v, ok := obj["client_reference_id"].(string); ok && v != "" {
		return v
	}
	if m, ok := obj["metadata"].(map[string]any); ok {
		if v, ok := m["tenant_id"].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// stripeAmount reconstructs the USD amount from the session. We trust
// metadata.amount_usd when present (we stamped it ourselves at
// CreateCheckoutSession), and fall back to amount_total (cents) when
// metadata is absent. amount_total is canonical on the Stripe side so
// the fallback is safe even if the metadata was scrubbed.
func stripeAmount(obj map[string]any) (decimal.Decimal, error) {
	if m, ok := obj["metadata"].(map[string]any); ok {
		if v, ok := m["amount_usd"].(string); ok && v != "" {
			d, err := decimal.NewFromString(v)
			if err == nil && d.IsPositive() {
				return d, nil
			}
		}
	}
	if v, ok := obj["amount_total"]; ok {
		switch n := v.(type) {
		case float64:
			return decimal.NewFromInt(int64(n)).Div(decimal.NewFromInt(100)), nil
		case json.Number:
			i, err := n.Int64()
			if err != nil {
				return decimal.Zero, err
			}
			return decimal.NewFromInt(i).Div(decimal.NewFromInt(100)), nil
		}
	}
	return decimal.Zero, errors.New("no amount on session")
}

// VerifySession queries Stripe's /v1/checkout/sessions/{id} endpoint
// for the current state of a session. Maps Stripe's status +
// payment_status pair onto the provider-neutral VerifyStatus enum:
//
//	status=complete + payment_status=paid               → VerifyPaid
//	status=complete + payment_status=no_payment_required → VerifyPaid (zero-cost)
//	status=expired                                       → VerifyExpired
//	status=open                                          → VerifyOpen
//	anything else                                        → VerifyFailed
func (t *StripeTopper) VerifySession(ctx context.Context, sessionID string) (VerifyResult, error) {
	if !t.Enabled() {
		return VerifyResult{}, errors.New("wallet: stripe disabled")
	}
	if sessionID == "" {
		return VerifyResult{}, errors.New("wallet: empty session id")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.stripe.com/v1/checkout/sessions/"+sessionID, nil)
	if err != nil {
		return VerifyResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+t.opts.SecretKey)
	resp, err := t.http.Do(req)
	if err != nil {
		return VerifyResult{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return VerifyResult{Status: VerifyFailed}, nil
	}
	if resp.StatusCode/100 != 2 {
		return VerifyResult{}, fmt.Errorf("stripe verify %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out struct {
		Status        string `json:"status"`
		PaymentStatus string `json:"payment_status"`
		AmountTotal   int64  `json:"amount_total"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return VerifyResult{}, fmt.Errorf("parse stripe session: %w", err)
	}
	result := VerifyResult{Amount: decimal.NewFromInt(out.AmountTotal).Div(decimal.NewFromInt(100))}
	switch out.Status {
	case "complete":
		if out.PaymentStatus == "paid" || out.PaymentStatus == "no_payment_required" {
			result.Status = VerifyPaid
		} else {
			result.Status = VerifyFailed
		}
	case "open":
		result.Status = VerifyOpen
	case "expired":
		result.Status = VerifyExpired
	default:
		result.Status = VerifyFailed
	}
	return result, nil
}

var _ Topper = (*StripeTopper)(nil)
