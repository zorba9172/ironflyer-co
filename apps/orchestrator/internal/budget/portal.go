// Customer Portal — Stripe-hosted self-service billing.
//
// Stripe runs the entire "change card, change plan, view invoices,
// cancel subscription" UI for us. The orchestrator only vends a short-
// lived session URL; the user clicks through, Stripe handles the rest,
// and any subscription mutations propagate back via the same webhook
// path Checkout already uses.
//
// This means we never build a "change card" form — Stripe owns the
// card vaulting + 3-D Secure + receipt page. Replacing the portal with
// custom UI would also mean PCI scope compliance work; not worth it.

package budget

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PortalSession is the projection of Stripe's BillingPortalSession we
// surface to GraphQL clients. Stripe's session.url is short-lived
// (default ~5 min before redirect, then anchored to the user's
// authentication state inside the portal), so the SPA must redirect
// promptly.
type PortalSession struct {
	URL       string
	ExpiresAt time.Time
}

// CreatePortalSession opens a Stripe Customer Portal session for the
// supplied customer id. returnURL is where the portal sends the user
// after they click "Return to Ironflyer". Both arguments are required —
// the orchestrator never lets callers omit them so an unbound portal
// session can't redirect users to an attacker-controlled URL.
//
// The owner check happens one layer up (resolver) — this method blindly
// trusts customerID. Callers MUST verify customerID matches the
// authenticated user's Stripe customer id before invoking.
func (s *StripeService) CreatePortalSession(ctx context.Context, customerID, returnURL string) (PortalSession, error) {
	if !s.Enabled() {
		return PortalSession{}, errors.New("stripe disabled")
	}
	if strings.TrimSpace(customerID) == "" {
		return PortalSession{}, errors.New("customerID required")
	}
	if strings.TrimSpace(returnURL) == "" {
		return PortalSession{}, errors.New("returnURL required")
	}
	form := url.Values{}
	form.Set("customer", customerID)
	form.Set("return_url", returnURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.stripe.com/v1/billing_portal/sessions",
		strings.NewReader(form.Encode()))
	if err != nil {
		return PortalSession{}, err
	}
	req.Header.Set("Authorization", "Bearer "+s.SecretKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return PortalSession{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return PortalSession{}, fmt.Errorf("stripe portal %d: %s",
			resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out struct {
		URL     string `json:"url"`
		Created int64  `json:"created"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return PortalSession{}, fmt.Errorf("parse stripe portal: %w", err)
	}
	if out.URL == "" {
		return PortalSession{}, errors.New("stripe returned no portal URL")
	}
	// Stripe doesn't return an explicit expiry. The portal session is
	// short-lived (Stripe's docs say "a few minutes"); we surface a
	// conservative 5-minute window so the SPA can decide whether to
	// re-mint before redirecting.
	created := time.Now().UTC()
	if out.Created > 0 {
		created = time.Unix(out.Created, 0).UTC()
	}
	return PortalSession{
		URL:       out.URL,
		ExpiresAt: created.Add(5 * time.Minute),
	}, nil
}
