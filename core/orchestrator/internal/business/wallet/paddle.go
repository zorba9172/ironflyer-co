package wallet

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/pkg/httpclient"
)

// PaddleTopperOpts configures the Paddle wallet top-up adapter. Mirrors
// StripeTopperOpts so the registry can construct both providers from
// a single config struct in main.go without special-casing.
type PaddleTopperOpts struct {
	// APIKey is the Paddle Billing bearer token. Empty disables the
	// Topper — CreateCheckoutSession returns "paddle disabled" so
	// dev environments still build.
	APIKey string

	// WebhookSecret is the HMAC key for verifying Paddle-Signature
	// headers on inbound transaction.completed events.
	WebhookSecret string

	// CreditMultiplier is the platform markup, mirroring the Stripe topper:
	// the buyer is CHARGED amountUSD×CreditMultiplier but CREDITED amountUSD
	// (custom_data.amount_usd unchanged), so margin = 1 − 1/CreditMultiplier
	// on every prepaid dollar. Zero or ≤ 1 means no markup.
	CreditMultiplier decimal.Decimal

	// Environment switches the API base. "sandbox" routes to the
	// Paddle sandbox; anything else (including "" and "live") uses
	// production.
	Environment string

	// SuccessURL / CancelURL are the redirect targets Paddle hands
	// back to the browser after the user completes (or abandons) the
	// hosted checkout. Both must be absolute URLs.
	SuccessURL string
	CancelURL  string

	// HTTPClient lets callers swap in a custom transport. Nil falls
	// back to a 15s timeout client matching StripeTopper.
	HTTPClient *http.Client
}

// PaddleTopper turns Paddle Billing transactions into wallet credits.
// Implements Topper. Wallet credits arrive on transaction.completed
// (or transaction.paid for hosted-checkout flows); synchronous
// creation only stages the pending row.
//
// We use the Paddle "non-catalog" transaction shape so we don't have
// to maintain a Paddle price id per top-up tier — the amount is sent
// inline alongside the same SupportedTopUpAmounts gate Stripe uses,
// keeping the two providers symmetrical.
type PaddleTopper struct {
	opts    PaddleTopperOpts
	baseURL string
	service Service
	http    *http.Client
}

// NewPaddleTopper builds the adapter. Returns a non-nil topper even
// when APIKey is empty so the registry can hold it; Enabled() reports
// false in that state.
func NewPaddleTopper(service Service, opts PaddleTopperOpts) *PaddleTopper {
	base := "https://api.paddle.com"
	if strings.EqualFold(strings.TrimSpace(opts.Environment), "sandbox") {
		base = "https://sandbox-api.paddle.com"
	}
	if opts.SuccessURL == "" {
		opts.SuccessURL = "http://localhost:3000/app/wallet?paddle=success"
	}
	if opts.CancelURL == "" {
		opts.CancelURL = "http://localhost:3000/app/wallet?paddle=cancel"
	}
	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = httpclient.Standard(15 * time.Second)
	}
	return &PaddleTopper{opts: opts, baseURL: base, service: service, http: httpClient}
}

// Name implements Topper.
func (p *PaddleTopper) Name() string { return ProviderPaddle }

// Label implements Topper. The "MoR" suffix is intentional — Paddle's
// merchant-of-record status is the reason an operator would prefer
// it (global VAT/sales-tax compliance) so the UI flags it explicitly.
func (p *PaddleTopper) Label() string { return "Card (Paddle · MoR)" }

// Enabled implements Topper.
func (p *PaddleTopper) Enabled() bool { return p != nil && p.opts.APIKey != "" }

// CreateCheckoutSession provisions a one-off Paddle transaction with a
// hosted checkout URL. The amount is sent inline via the non-catalog
// price object so we don't have to maintain price IDs per top-up
// tier. custom_data carries tenant + amount + purpose so the webhook
// can route the credit back without a separate lookup.
func (p *PaddleTopper) CreateCheckoutSession(ctx context.Context, tenant string, amountUSD decimal.Decimal) (CheckoutSession, error) {
	if !p.Enabled() {
		return CheckoutSession{}, errors.New("wallet: paddle disabled")
	}
	if !IsSupportedAmount(amountUSD) {
		return CheckoutSession{}, ErrInvalidAmount
	}
	// Charge amountUSD×CreditMultiplier (the platform markup); credit
	// amountUSD (custom_data.amount_usd below is unchanged, so the webhook
	// and reconcile still grant amountUSD).
	chargeUSD := amountUSD
	if p.opts.CreditMultiplier.GreaterThan(decimal.NewFromInt(1)) {
		chargeUSD = amountUSD.Mul(p.opts.CreditMultiplier)
	}
	cents := chargeUSD.Mul(decimal.NewFromInt(100)).IntPart()

	body := map[string]any{
		"items": []map[string]any{
			{
				"quantity": 1,
				"price": map[string]any{
					"description": fmt.Sprintf("Ironflyer wallet credit — $%s", amountUSD.StringFixed(2)),
					"tax_mode":    "account_setting",
					"unit_price": map[string]any{
						"amount":        strconv.FormatInt(cents, 10),
						"currency_code": "USD",
					},
					"quantity": map[string]any{"minimum": 1, "maximum": 1},
					"product": map[string]any{
						"name":         "Ironflyer wallet top-up",
						"tax_category": "standard",
					},
				},
			},
		},
		"collection_mode": "automatic",
		"custom_data": map[string]string{
			"tenant_id":  tenant,
			"amount_usd": amountUSD.String(),
			"purpose":    "wallet_topup",
			"provider":   ProviderPaddle,
		},
	}
	if p.opts.SuccessURL != "" {
		body["checkout"] = map[string]string{"url": p.opts.SuccessURL}
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return CheckoutSession{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.baseURL+"/transactions", bytes.NewReader(buf))
	if err != nil {
		return CheckoutSession{}, err
	}
	req.Header.Set("Authorization", "Bearer "+p.opts.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.http.Do(req)
	if err != nil {
		return CheckoutSession{}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return CheckoutSession{}, fmt.Errorf("paddle %d: %s",
			resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var out struct {
		Data struct {
			ID       string `json:"id"`
			Checkout struct {
				URL string `json:"url"`
			} `json:"checkout"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return CheckoutSession{}, fmt.Errorf("parse paddle response: %w", err)
	}
	if out.Data.Checkout.URL == "" || out.Data.ID == "" {
		return CheckoutSession{}, errors.New("paddle returned incomplete transaction")
	}
	// Stage the pending row so the webhook has something to flip.
	// Paddle transaction ids are prefixed "txn_" — distinct from
	// Stripe's "cs_" prefix, so we can share the wallet_topups
	// stripe_session_id column without collision.
	if _, err := p.service.CreatePendingTopUp(ctx, tenant, amountUSD, out.Data.ID); err != nil {
		return CheckoutSession{URL: out.Data.Checkout.URL, SessionID: out.Data.ID, Provider: ProviderPaddle}, nil
	}
	return CheckoutSession{URL: out.Data.Checkout.URL, SessionID: out.Data.ID, Provider: ProviderPaddle}, nil
}

// HandleWebhook verifies the Paddle-Signature header and applies the
// credit when the event is transaction.completed (or
// transaction.paid) with custom_data.purpose=wallet_topup. Idempotent
// via the UNIQUE constraint on wallet_topups.stripe_session_id (the
// column is named for legacy reasons but stores any provider's
// transaction id).
func (p *PaddleTopper) HandleWebhook(ctx context.Context, rawBody []byte, signatureHeader string) error {
	if p.opts.WebhookSecret == "" {
		return errors.New("wallet: paddle webhook secret not configured")
	}
	ev, err := verifyPaddleSignature(rawBody, signatureHeader, p.opts.WebhookSecret, 5*time.Minute)
	if err != nil {
		return fmt.Errorf("wallet: webhook signature: %w", err)
	}
	// transaction.paid fires for hosted checkouts; transaction.completed
	// fires for B2B invoice flows. We accept either as a credit signal.
	if ev.EventType != "transaction.completed" && ev.EventType != "transaction.paid" {
		return nil
	}
	cd, _ := ev.Data["custom_data"].(map[string]any)
	// custom_data is required — without it we cannot route to a tenant.
	if cd == nil {
		return nil
	}
	if purpose, _ := cd["purpose"].(string); purpose != "" && purpose != "wallet_topup" {
		return nil
	}
	sessionID, _ := ev.Data["id"].(string)
	if sessionID == "" {
		return errors.New("wallet: paddle webhook missing transaction id")
	}
	tenant, _ := cd["tenant_id"].(string)
	if tenant == "" {
		return errors.New("wallet: paddle webhook missing tenant id")
	}
	amount, err := paddleAmount(ev.Data, cd)
	if err != nil {
		return fmt.Errorf("wallet: paddle webhook amount: %w", err)
	}
	return p.service.TopUp(ctx, tenant, amount, sessionID)
}

// paddleEvent is the minimal Paddle webhook event shape.
type paddleEvent struct {
	EventType string         `json:"event_type"`
	Data      map[string]any `json:"data"`
}

// verifyPaddleSignature checks the `ts=...;h1=...` Paddle-Signature
// header. Signed payload is `<ts>:<body>` HMAC-SHA256'd with the
// webhook secret. Tolerance window matches Stripe to keep the
// replay-protection contract symmetrical between providers.
func verifyPaddleSignature(rawBody []byte, header, secret string, tolerance time.Duration) (paddleEvent, error) {
	var ts, h1 string
	for _, part := range strings.Split(header, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "ts":
			ts = kv[1]
		case "h1":
			h1 = kv[1]
		}
	}
	if ts == "" || h1 == "" {
		return paddleEvent{}, errors.New("malformed Paddle-Signature header")
	}
	tsInt, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return paddleEvent{}, fmt.Errorf("bad timestamp: %w", err)
	}
	if tolerance > 0 {
		delta := time.Since(time.Unix(tsInt, 0))
		if delta > tolerance || delta < -tolerance {
			return paddleEvent{}, errors.New("timestamp outside tolerance")
		}
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts))
	mac.Write([]byte(":"))
	mac.Write(rawBody)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(h1), []byte(expected)) {
		return paddleEvent{}, errors.New("signature mismatch")
	}
	var ev paddleEvent
	if err := json.Unmarshal(rawBody, &ev); err != nil {
		return paddleEvent{}, fmt.Errorf("parse event: %w", err)
	}
	return ev, nil
}

// paddleAmount reconstructs the USD amount from a transaction event.
// We trust custom_data.amount_usd when present (we stamped it
// ourselves at CreateCheckoutSession), and fall back to
// data.details.totals.grand_total (minor units, string) when
// custom_data was scrubbed. USD is the only currency the wallet
// supports today; non-USD payments are rejected with an error so the
// operator notices before the credit lands at the wrong rate.
func paddleAmount(data map[string]any, customData map[string]any) (decimal.Decimal, error) {
	if v, ok := customData["amount_usd"].(string); ok && v != "" {
		d, err := decimal.NewFromString(v)
		if err == nil && d.IsPositive() {
			return d, nil
		}
	}
	details, _ := data["details"].(map[string]any)
	if details != nil {
		if totals, ok := details["totals"].(map[string]any); ok {
			if cc, _ := totals["currency_code"].(string); cc != "" && !strings.EqualFold(cc, "USD") {
				return decimal.Zero, fmt.Errorf("non-USD currency %q", cc)
			}
			if grand, ok := totals["grand_total"].(string); ok && grand != "" {
				cents, err := strconv.ParseInt(grand, 10, 64)
				if err == nil && cents > 0 {
					return decimal.NewFromInt(cents).Div(decimal.NewFromInt(100)), nil
				}
			}
		}
	}
	return decimal.Zero, errors.New("no amount on paddle transaction")
}

// VerifySession queries Paddle's /transactions/{id} endpoint for the
// current state of a transaction. Maps Paddle's status enum onto the
// provider-neutral VerifyStatus:
//
//	status=completed | paid | billed → VerifyPaid
//	status=ready | draft             → VerifyOpen
//	status=canceled                  → VerifyExpired
//	anything else                    → VerifyFailed
func (p *PaddleTopper) VerifySession(ctx context.Context, sessionID string) (VerifyResult, error) {
	if !p.Enabled() {
		return VerifyResult{}, errors.New("wallet: paddle disabled")
	}
	if sessionID == "" {
		return VerifyResult{}, errors.New("wallet: empty transaction id")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		p.baseURL+"/transactions/"+sessionID, nil)
	if err != nil {
		return VerifyResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+p.opts.APIKey)
	resp, err := p.http.Do(req)
	if err != nil {
		return VerifyResult{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return VerifyResult{Status: VerifyFailed}, nil
	}
	if resp.StatusCode/100 != 2 {
		return VerifyResult{}, fmt.Errorf("paddle verify %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out struct {
		Data struct {
			Status     string            `json:"status"`
			CustomData map[string]string `json:"custom_data"`
			Details    struct {
				Totals struct {
					CurrencyCode string `json:"currency_code"`
					GrandTotal   string `json:"grand_total"`
				} `json:"totals"`
			} `json:"details"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return VerifyResult{}, fmt.Errorf("parse paddle transaction: %w", err)
	}
	cd := out.Data.CustomData
	if cd == nil {
		cd = map[string]string{}
	}
	// Translate amount: prefer custom_data.amount_usd (we stamped it
	// at checkout creation), fall back to details.totals.grand_total
	// in minor units.
	var amount decimal.Decimal
	if v, ok := cd["amount_usd"]; ok && v != "" {
		if d, err := decimal.NewFromString(v); err == nil {
			amount = d
		}
	}
	if amount.IsZero() && out.Data.Details.Totals.GrandTotal != "" {
		if cents, err := strconv.ParseInt(out.Data.Details.Totals.GrandTotal, 10, 64); err == nil && cents > 0 {
			if strings.EqualFold(out.Data.Details.Totals.CurrencyCode, "USD") || out.Data.Details.Totals.CurrencyCode == "" {
				amount = decimal.NewFromInt(cents).Div(decimal.NewFromInt(100))
			}
		}
	}
	result := VerifyResult{Amount: amount}
	switch out.Data.Status {
	case "completed", "paid", "billed":
		result.Status = VerifyPaid
	case "ready", "draft":
		result.Status = VerifyOpen
	case "canceled":
		result.Status = VerifyExpired
	default:
		result.Status = VerifyFailed
	}
	return result, nil
}

var _ Topper = (*PaddleTopper)(nil)
