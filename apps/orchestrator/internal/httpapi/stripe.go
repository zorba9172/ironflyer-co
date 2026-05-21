package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"ironflyer/apps/orchestrator/internal/auth"
	"ironflyer/apps/orchestrator/internal/budget"
)

// startCheckout creates a Stripe Checkout Session for the authenticated user
// and returns the redirect URL. The browser then sends the user to Stripe;
// when the subscription is paid Stripe calls back via /budget/webhook.
func (a *API) startCheckout(w http.ResponseWriter, r *http.Request) {
	if a.d.Stripe == nil || !a.d.Stripe.Enabled() {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("stripe disabled"))
		return
	}
	u, ok := auth.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errJSON("unauthenticated"))
		return
	}
	var body struct {
		Tier string `json:"tier"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON"))
		return
	}
	if body.Tier == "" {
		writeJSON(w, http.StatusBadRequest, errJSON("tier required"))
		return
	}
	url, err := a.d.Stripe.CreateCheckoutSession(r.Context(), u.ID, u.Email, budget.PlanTier(body.Tier))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

// stripeWebhook verifies the Stripe-Signature header against the raw body,
// then dispatches to Billing.ApplyStripeEvent. We always 200 once the
// signature checks out so Stripe stops retrying — handler errors are logged.
func (a *API) stripeWebhook(w http.ResponseWriter, r *http.Request) {
	if a.d.Stripe == nil || !a.d.Stripe.Enabled() {
		http.Error(w, "stripe disabled", http.StatusServiceUnavailable)
		return
	}
	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	ev, err := a.d.Stripe.VerifyWebhook(payload, r.Header.Get("Stripe-Signature"), 5*time.Minute)
	if err != nil {
		a.d.Logger.Warn().Err(err).Msg("stripe webhook verification failed")
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}
	if err := a.d.Billing.ApplyStripeEvent(r.Context(), ev, a.d.Auth); err != nil {
		a.d.Logger.Error().Err(err).Str("type", ev.Type).Str("id", ev.ID).
			Msg("stripe event handling failed")
	}
	w.WriteHeader(http.StatusOK)
}
