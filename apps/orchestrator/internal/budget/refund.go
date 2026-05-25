// Refund flow — operator-initiated reimbursement of a Stripe charge.
//
// Policy: refunding a past charge does NOT cancel the user's active
// subscription. A refund is about a specific cash transaction; cancel
// is about future billing intent. Conflating the two is a common
// support footgun and we explicitly avoid it. Operators who want both
// flows must issue the refund AND call CancelSubscription separately.

package budget

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// RefundResult is the structured projection of Stripe's Refund object
// that the orchestrator surfaces back to admin callers. The wire shape
// is intentionally narrow — full refund metadata lives on the Stripe
// dashboard, the orchestrator's view is "did the refund land, for how
// much, what's the status".
type RefundResult struct {
	ID          string
	ChargeID    string
	AmountCents int64
	Currency    string
	Status      string // "succeeded" | "pending" | "failed" | "canceled"
	Reason      string
	CreatedAt   time.Time
}

// stripeRefundReasons is the small set Stripe accepts on its native
// `reason` field. We pass the caller's reason through only when it
// matches; otherwise we keep Stripe's native field empty and store the
// free-form reason on the orchestrator audit row instead.
var stripeRefundReasons = map[string]bool{
	"duplicate":             true,
	"fraudulent":            true,
	"requested_by_customer": true,
}

// Refund calls Stripe /v1/refunds for the supplied chargeID. When
// amountCents is nil → full refund of the remaining refundable balance
// on the charge. Otherwise *amountCents is the partial-refund amount
// in cents; Stripe will reject if it exceeds what's left on the charge.
//
// The reason argument is stored on the orchestrator audit row no
// matter what — Stripe's native `reason` is only set when reason maps
// to one of the three values Stripe documents.
func (s *StripeService) Refund(ctx context.Context, chargeID string, amountCents *int64, reason string) (RefundResult, error) {
	if !s.Enabled() {
		return RefundResult{}, errors.New("stripe disabled")
	}
	if strings.TrimSpace(chargeID) == "" {
		return RefundResult{}, errors.New("chargeID required")
	}
	if amountCents != nil && *amountCents <= 0 {
		return RefundResult{}, errors.New("amountCents must be > 0 when set; pass nil for a full refund")
	}
	form := url.Values{}
	form.Set("charge", chargeID)
	if amountCents != nil {
		form.Set("amount", strconv.FormatInt(*amountCents, 10))
	}
	r := strings.TrimSpace(strings.ToLower(reason))
	if stripeRefundReasons[r] {
		form.Set("reason", r)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.stripe.com/v1/refunds",
		strings.NewReader(form.Encode()))
	if err != nil {
		return RefundResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+s.SecretKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return RefundResult{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return RefundResult{}, fmt.Errorf("stripe refunds %d: %s",
			resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var raw struct {
		ID       string `json:"id"`
		Charge   string `json:"charge"`
		Amount   int64  `json:"amount"`
		Currency string `json:"currency"`
		Status   string `json:"status"`
		Reason   string `json:"reason"`
		Created  int64  `json:"created"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return RefundResult{}, fmt.Errorf("parse stripe refund: %w", err)
	}
	created := time.Now().UTC()
	if raw.Created > 0 {
		created = time.Unix(raw.Created, 0).UTC()
	}
	out := RefundResult{
		ID:          raw.ID,
		ChargeID:    raw.Charge,
		AmountCents: raw.Amount,
		Currency:    strings.ToUpper(raw.Currency),
		Status:      raw.Status,
		Reason:      reason, // keep the caller's free-form reason
		CreatedAt:   created,
	}
	if raw.Charge == "" {
		out.ChargeID = chargeID
	}
	return out, nil
}
