package budget

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

	"github.com/shopspring/decimal"
)

// StripeService talks to Stripe directly via REST + form encoding.
// We intentionally avoid stripe-go to keep the dependency surface small;
// only two endpoints are used (checkout.sessions create + webhook verify).
type StripeService struct {
	SecretKey     string
	WebhookSecret string
	Prices        map[PlanTier]string
	SuccessURL    string
	CancelURL     string
	HTTPClient    *http.Client
}

type StripeOpts struct {
	SecretKey     string
	WebhookSecret string
	Prices        map[PlanTier]string
	SuccessURL    string
	CancelURL     string
}

func NewStripeService(o StripeOpts) *StripeService {
	if o.SuccessURL == "" {
		o.SuccessURL = "http://localhost:3000/app/settings?stripe=success"
	}
	if o.CancelURL == "" {
		o.CancelURL = "http://localhost:3000/pricing?stripe=cancel"
	}
	if o.Prices == nil {
		o.Prices = map[PlanTier]string{}
	}
	return &StripeService{
		SecretKey: o.SecretKey, WebhookSecret: o.WebhookSecret,
		Prices: o.Prices, SuccessURL: o.SuccessURL, CancelURL: o.CancelURL,
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// Enabled is true iff we have a secret key. Webhook secret is required only
// for the webhook handler, checked separately in VerifyWebhook.
func (s *StripeService) Enabled() bool { return s != nil && s.SecretKey != "" }

// CreateCheckoutSession returns the URL the user should be redirected to.
// We attach user_id + tier in metadata so the webhook can map back without
// a separate customer table lookup.
func (s *StripeService) CreateCheckoutSession(ctx context.Context, userID, email string, tier PlanTier) (string, error) {
	if !s.Enabled() {
		return "", errors.New("stripe disabled")
	}
	price, ok := s.Prices[tier]
	if !ok || price == "" {
		return "", fmt.Errorf("no Stripe price configured for tier %q", tier)
	}
	form := url.Values{}
	form.Set("mode", "subscription")
	form.Set("line_items[0][price]", price)
	form.Set("line_items[0][quantity]", "1")
	form.Set("success_url", s.SuccessURL)
	form.Set("cancel_url", s.CancelURL)
	form.Set("client_reference_id", userID)
	form.Set("metadata[user_id]", userID)
	form.Set("metadata[tier]", string(tier))
	if email != "" {
		form.Set("customer_email", email)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.stripe.com/v1/checkout/sessions",
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.SecretKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("stripe %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out struct {
		URL string `json:"url"`
		ID  string `json:"id"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("parse stripe response: %w", err)
	}
	if out.URL == "" {
		return "", errors.New("stripe returned no checkout URL")
	}
	return out.URL, nil
}

// StripeEvent is the subset of a Stripe Event we care about.
type StripeEvent struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		Object map[string]any `json:"object"`
	} `json:"data"`
}

// VerifyWebhook validates the Stripe-Signature header against the raw body.
// Signature scheme: header is `t=TS,v1=SIG[,v1=SIG]`. signed payload is
// `TS.body`; HMAC-SHA256 with the webhook secret must equal some v1.
func (s *StripeService) VerifyWebhook(payload []byte, sigHeader string, tolerance time.Duration) (StripeEvent, error) {
	if s.WebhookSecret == "" {
		return StripeEvent{}, errors.New("stripe webhook secret not configured")
	}
	var ts string
	var sigs []string
	for _, part := range strings.Split(sigHeader, ",") {
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
		return StripeEvent{}, errors.New("malformed Stripe-Signature header")
	}
	tsInt, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return StripeEvent{}, fmt.Errorf("bad timestamp: %w", err)
	}
	if tolerance > 0 {
		if delta := time.Since(time.Unix(tsInt, 0)); delta > tolerance || delta < -tolerance {
			return StripeEvent{}, errors.New("timestamp outside tolerance")
		}
	}
	mac := hmac.New(sha256.New, []byte(s.WebhookSecret))
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	match := false
	for _, sig := range sigs {
		if hmac.Equal([]byte(sig), []byte(expected)) {
			match = true
			break
		}
	}
	if !match {
		return StripeEvent{}, errors.New("signature mismatch")
	}
	var ev StripeEvent
	if err := json.Unmarshal(payload, &ev); err != nil {
		return StripeEvent{}, fmt.Errorf("parse event: %w", err)
	}
	return ev, nil
}

// PlanSetter persists a user's plan tier. Implemented by auth.Service so we
// don't take a direct dependency on the auth package here.
type PlanSetter interface {
	SetPlan(ctx context.Context, userID, plan string) error
}

// ApplyStripeEvent updates billing + plan state in response to a verified
// Stripe webhook. Returns an error only for malformed payloads; unknown event
// types are ignored (so adding new webhook subscriptions never fails).
func (b *Billing) ApplyStripeEvent(ctx context.Context, ev StripeEvent, ps PlanSetter) error {
	obj := ev.Data.Object
	switch ev.Type {
	case "checkout.session.completed":
		uid := stripeUserID(obj)
		if uid == "" {
			return errors.New("checkout.session.completed: missing user_id")
		}
		tier := stripeTier(obj)
		if tier == "" {
			return errors.New("checkout.session.completed: missing tier")
		}
		if ps != nil {
			if err := ps.SetPlan(ctx, uid, string(tier)); err != nil {
				return fmt.Errorf("persist plan: %w", err)
			}
		}
		b.AssignPlan(ctx, uid, tier)
		return nil

	case "invoice.payment_succeeded":
		// Recurring renewal — record revenue. AssignPlan already credited the
		// first month at checkout, so we only credit *additional* invoices
		// (billing_reason ≠ subscription_create).
		reason, _ := obj["billing_reason"].(string)
		if reason == "subscription_create" {
			return nil
		}
		cents, _ := stripeNum(obj, "amount_paid")
		if cents <= 0 {
			return nil
		}
		uid := stripeUserID(obj)
		_, err := b.Vault.Record(ctx, VaultEntry{
			Kind: VaultRevenue, UserID: uid,
			Amount: decimal.NewFromInt(cents).Div(decimal.NewFromInt(100)),
			Note:   "stripe invoice " + stripeString(obj, "id"),
		})
		return err

	case "customer.subscription.deleted":
		uid := stripeUserID(obj)
		if uid == "" {
			return nil
		}
		if ps != nil {
			_ = ps.SetPlan(ctx, uid, string(TierFree))
		}
		b.mu.Lock()
		b.userPlan[uid] = TierFree
		b.mu.Unlock()
		return nil

	case "charge.refunded":
		cents, _ := stripeNum(obj, "amount_refunded")
		if cents <= 0 {
			return nil
		}
		_, err := b.Vault.Record(ctx, VaultEntry{
			Kind: VaultRefund, UserID: stripeUserID(obj),
			Amount: decimal.NewFromInt(cents).Div(decimal.NewFromInt(100)),
			Note:   "stripe refund " + stripeString(obj, "id"),
		})
		return err
	}
	return nil
}

// stripeUserID pulls the user id Stripe relayed back to us, preferring the
// dedicated client_reference_id and falling back to our metadata bag.
func stripeUserID(obj map[string]any) string {
	if v, ok := obj["client_reference_id"].(string); ok && v != "" {
		return v
	}
	if m, ok := obj["metadata"].(map[string]any); ok {
		if v, ok := m["user_id"].(string); ok {
			return v
		}
	}
	return ""
}

func stripeTier(obj map[string]any) PlanTier {
	if m, ok := obj["metadata"].(map[string]any); ok {
		if v, ok := m["tier"].(string); ok && v != "" {
			return PlanTier(v)
		}
	}
	return ""
}

func stripeNum(obj map[string]any, key string) (int64, bool) {
	switch v := obj[key].(type) {
	case float64:
		return int64(v), true
	case int64:
		return v, true
	case int:
		return int64(v), true
	case json.Number:
		i, err := v.Int64()
		return i, err == nil
	}
	return 0, false
}

func stripeString(obj map[string]any, key string) string {
	if v, ok := obj[key].(string); ok {
		return v
	}
	return ""
}
