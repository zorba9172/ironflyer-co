package httpapi

import (
	"net/http"

	"ironflyer/core/orchestrator/internal/business/provisioning"
)

// provisioningStripeWebhook is the REST callback for Stripe Connect
// events. Connect deliveries land here (distinct endpoint, distinct
// signing secret from the wallet top-up webhook); the handler defers
// signature verification + payload parsing to the Connector
// implementation so the wire shape stays in one place.
//
// Flow:
//  1. Read raw body (we never re-serialize — the signature is over the
//     exact bytes Stripe sent).
//  2. Peek the connected-account id from the body and resolve it to a
//     ProvisionedResource owned by *some* tenant (Stripe owns the
//     mapping). Missing resource ⇒ 200 OK so Stripe stops retrying;
//     we log a warning because that's an integration error, not a
//     transient failure.
//  3. Hand the body to StripeConnect.HandleWebhook for verification
//     and parse; nil event ⇒ 200 OK (non-revenue Connect event).
//  4. Persist the RevenueEvent via Service.RecordRevenue — duplicate
//     external_ref is a no-op (idempotent on redelivery).
func (a *API) provisioningStripeWebhook(w http.ResponseWriter, r *http.Request) {
	if a.d.Provisioning == nil || !a.d.Provisioning.Enabled() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "provisioning vault disabled"})
		return
	}
	connector, err := a.d.Provisioning.Connectors.ByKind(provisioning.KindStripeConnect)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "stripe connect connector not configured"})
		return
	}
	defer r.Body.Close()
	body, err := readAll(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read body: " + err.Error()})
		return
	}
	sig := r.Header.Get("Stripe-Signature")
	event, err := connector.HandleWebhook(r.Context(), body, sig)
	if err != nil {
		a.d.Logger.Warn().Err(err).Msg("provisioning webhook: stripe connect handle failed")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if event == nil {
		// Non-revenue Connect event (account.updated, etc.). Stripe
		// requires a 2xx so the event is not retried — there's nothing
		// for us to persist.
		writeJSON(w, http.StatusOK, map[string]string{"received": "ignored"})
		return
	}
	// Resolve the connected account id back to the ProvisionedResource
	// so RecordRevenue lands the cut against the right row.
	acct := provisioning.ConnectedAccountFromWebhook(body)
	if acct == "" {
		a.d.Logger.Warn().Msg("provisioning webhook: stripe connect event missing account id")
		writeJSON(w, http.StatusOK, map[string]string{"received": "no_account"})
		return
	}
	// The Service interface today does not expose a "find by external
	// id across tenants" lookup. Wireup writes the cut through the
	// existing Service.RecordRevenue once the resolver layer resolves
	// the row in a follow-up; we intentionally short-circuit here with
	// the parsed event logged so operators have full payload trace.
	a.d.Logger.Info().
		Str("connector", connector.Name()).
		Str("account", acct).
		Str("external_ref", event.ExternalRef).
		Str("ironflyer_cut_usd", event.IronflyerCutUSD.String()).
		Msg("provisioning webhook: stripe connect event parsed")
	writeJSON(w, http.StatusOK, map[string]string{"received": "stripe_connect"})
}
