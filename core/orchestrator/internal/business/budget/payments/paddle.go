package payments

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

	"ironflyer/core/orchestrator/internal/pkg/httpclient"
)

// PaddleService is a thin REST client against Paddle Billing (v2 API).
// No paddle-go dep — only two endpoints are used: POST /transactions
// to mint a hosted checkout URL, plus webhook signature verification.
//
// Env contract:
//
//	PADDLE_API_KEY        — bearer token, required to enable the service
//	PADDLE_WEBHOOK_SECRET — HMAC key for verifying Paddle-Signature
//	PADDLE_ENV            — "live" | "sandbox" (default "live")
//	PADDLE_PRICE_PRO/TEAM/ENTERPRISE — Paddle price IDs (pri_...)
type PaddleService struct {
	APIKey        string
	WebhookSecret string
	BaseURL       string
	Prices        map[PlanTier]string
	SuccessURL    string
	CancelURL     string
	HTTPClient    *http.Client
}

// PaddleOpts is the constructor input.
type PaddleOpts struct {
	APIKey        string
	WebhookSecret string
	Environment   string // "live" | "sandbox"
	Prices        map[PlanTier]string
	SuccessURL    string
	CancelURL     string
}

// NewPaddleService builds a Paddle Billing client. Returns a non-nil
// service even when the API key is unset so the registry can hold it;
// Enabled() reports false in that state.
func NewPaddleService(o PaddleOpts) *PaddleService {
	base := "https://api.paddle.com"
	if strings.EqualFold(strings.TrimSpace(o.Environment), "sandbox") {
		base = "https://sandbox-api.paddle.com"
	}
	if o.Prices == nil {
		o.Prices = map[PlanTier]string{}
	}
	return &PaddleService{
		APIKey:        o.APIKey,
		WebhookSecret: o.WebhookSecret,
		BaseURL:       base,
		Prices:        o.Prices,
		SuccessURL:    o.SuccessURL,
		CancelURL:     o.CancelURL,
		HTTPClient:    httpclient.Standard(15 * time.Second),
	}
}

// Name implements Provider.
func (p *PaddleService) Name() string { return "paddle" }

// Enabled implements Provider.
func (p *PaddleService) Enabled() bool { return p != nil && p.APIKey != "" }

// CreateCheckoutSession opens a Paddle hosted checkout via the
// Transactions API. We stamp user_id + tier into custom_data so the
// webhook can map back without a separate Paddle-customer lookup.
func (p *PaddleService) CreateCheckoutSession(ctx context.Context, userID, email string, tier PlanTier) (string, error) {
	if !p.Enabled() {
		return "", errors.New("paddle disabled")
	}
	price, ok := p.Prices[tier]
	if !ok || price == "" {
		return "", fmt.Errorf("no Paddle price configured for tier %q", tier)
	}

	body := map[string]any{
		"items": []map[string]any{
			{"price_id": price, "quantity": 1},
		},
		"collection_mode": "automatic",
		"custom_data": map[string]string{
			"user_id": userID,
			"tier":    string(tier),
		},
	}
	if email != "" {
		body["customer"] = map[string]string{"email": email}
	}
	if p.SuccessURL != "" || p.CancelURL != "" {
		ck := map[string]string{}
		if p.SuccessURL != "" {
			ck["url"] = p.SuccessURL
		}
		body["checkout"] = ck
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.BaseURL+"/transactions", bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("paddle %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
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
		return "", fmt.Errorf("parse paddle response: %w", err)
	}
	if out.Data.Checkout.URL == "" {
		return "", errors.New("paddle returned no checkout URL")
	}
	return out.Data.Checkout.URL, nil
}

// VerifyWebhook validates a Paddle-Signature header of the form
// `ts=...;h1=...`. The signed payload is `<ts>:<body>` hashed with
// HMAC-SHA256 under PADDLE_WEBHOOK_SECRET.
func (p *PaddleService) VerifyWebhook(payload []byte, sigHeader string) (Event, error) {
	if p.WebhookSecret == "" {
		return Event{}, errors.New("paddle webhook secret not configured")
	}
	var ts, h1 string
	for _, part := range strings.Split(sigHeader, ";") {
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
		return Event{}, errors.New("malformed Paddle-Signature header")
	}
	if _, err := strconv.ParseInt(ts, 10, 64); err != nil {
		return Event{}, fmt.Errorf("bad timestamp: %w", err)
	}
	mac := hmac.New(sha256.New, []byte(p.WebhookSecret))
	mac.Write([]byte(ts))
	mac.Write([]byte(":"))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(h1), []byte(expected)) {
		return Event{}, errors.New("signature mismatch")
	}

	var raw struct {
		EventType string         `json:"event_type"`
		Data      map[string]any `json:"data"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return Event{}, fmt.Errorf("parse event: %w", err)
	}
	ev := Event{
		Provider: "paddle",
		Type:     raw.EventType,
		Raw:      raw.Data,
	}
	if raw.Data != nil {
		if cd, ok := raw.Data["custom_data"].(map[string]any); ok {
			if uid, _ := cd["user_id"].(string); uid != "" {
				ev.UserID = uid
			}
			if tier, _ := cd["tier"].(string); tier != "" {
				ev.Tier = PlanTier(tier)
			}
		}
	}
	return ev, nil
}

var _ Provider = (*PaddleService)(nil)
