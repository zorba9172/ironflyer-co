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
//
// Stripe Tax is enabled here: `automatic_tax[enabled]=true` makes Stripe
// compute VAT / GST / sales-tax based on the customer's billing address;
// `customer_update[address]=auto` + `customer_update[name]=auto` let
// Stripe persist the captured address back onto the Customer so future
// invoices have the right tax basis; `tax_id_collection[enabled]=true`
// surfaces a VAT/GST input on the Checkout page so EU/UK B2B buyers
// can supply their tax id. The operator MUST also enable Stripe Tax in
// the Stripe dashboard for these flags to take effect — see
// docs/BILLING.md for the dashboard-side checklist.
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
	// Stripe Tax — automatic tax calculation + B2B VAT capture.
	form.Set("automatic_tax[enabled]", "true")
	form.Set("customer_update[address]", "auto")
	form.Set("customer_update[name]", "auto")
	form.Set("tax_id_collection[enabled]", "true")

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

// SubscriptionMeta is the read shape the GraphQL mySubscription
// resolver projects into model.SubscriptionStripe. Zero-valued fields
// mean "Stripe didn't return one" — the resolver renders them as
// typed nulls.
type SubscriptionMeta struct {
	CustomerID         string
	SubscriptionID     string
	SubscriptionItemID string
	Status             string
	CurrentPeriodEnd   time.Time
	CancelAtPeriodEnd  bool
	PlanTier           PlanTier
}

// SubscriptionStatus fetches the current subscription metadata for a
// given Stripe customer. customerID may be empty: when so, returns
// (zero, nil) so dev environments without Stripe linkage answer
// gracefully. When Stripe is disabled we also return zero+nil rather
// than erroring so the dashboard renders a clean "no subscription"
// state.
func (s *StripeService) SubscriptionStatus(ctx context.Context, customerID string) (SubscriptionMeta, error) {
	if !s.Enabled() || strings.TrimSpace(customerID) == "" {
		return SubscriptionMeta{CustomerID: customerID}, nil
	}
	// List the customer's subscriptions; we surface the first active
	// (or trialing/past_due) one — a customer has at most one in
	// practice on the Ironflyer ladder.
	form := url.Values{}
	form.Set("customer", customerID)
	form.Set("status", "all")
	form.Set("limit", "5")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.stripe.com/v1/subscriptions?"+form.Encode(), nil)
	if err != nil {
		return SubscriptionMeta{CustomerID: customerID}, err
	}
	req.Header.Set("Authorization", "Bearer "+s.SecretKey)
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return SubscriptionMeta{CustomerID: customerID}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return SubscriptionMeta{CustomerID: customerID},
			fmt.Errorf("stripe %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return SubscriptionMeta{CustomerID: customerID}, fmt.Errorf("parse stripe response: %w", err)
	}
	meta := SubscriptionMeta{CustomerID: customerID}
	// Prefer active/trialing/past_due over canceled.
	pick := -1
	rank := func(state string) int {
		switch state {
		case "active":
			return 4
		case "trialing":
			return 3
		case "past_due":
			return 2
		case "unpaid":
			return 1
		}
		return 0
	}
	for i, sub := range out.Data {
		st, _ := sub["status"].(string)
		if pick < 0 || rank(st) > rank(stripeString(out.Data[pick], "status")) {
			pick = i
		}
	}
	if pick < 0 {
		return meta, nil
	}
	sub := out.Data[pick]
	meta.SubscriptionID = stripeString(sub, "id")
	meta.Status = stripeString(sub, "status")
	meta.CancelAtPeriodEnd, _ = sub["cancel_at_period_end"].(bool)
	if ts, ok := stripeNum(sub, "current_period_end"); ok && ts > 0 {
		meta.CurrentPeriodEnd = time.Unix(ts, 0).UTC()
	}
	meta.PlanTier = stripeTier(sub)
	if item := stripeMeteredItem(sub); item != "" {
		meta.SubscriptionItemID = item
	}
	return meta, nil
}

// CancelSubscription cancels a Stripe subscription. When atPeriodEnd is
// true we update the subscription to set cancel_at_period_end=true so
// the user keeps service through the paid window; otherwise we cancel
// immediately. Looks up the customer's current subscription via
// SubscriptionStatus.
func (s *StripeService) CancelSubscription(ctx context.Context, customerID string, atPeriodEnd bool) error {
	if !s.Enabled() {
		return errors.New("stripe disabled")
	}
	if strings.TrimSpace(customerID) == "" {
		return errors.New("customerID required")
	}
	meta, err := s.SubscriptionStatus(ctx, customerID)
	if err != nil {
		return err
	}
	if meta.SubscriptionID == "" {
		return errors.New("no active subscription for customer")
	}
	if atPeriodEnd {
		form := url.Values{}
		form.Set("cancel_at_period_end", "true")
		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			"https://api.stripe.com/v1/subscriptions/"+meta.SubscriptionID,
			strings.NewReader(form.Encode()))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+s.SecretKey)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := s.HTTPClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode/100 != 2 {
			return fmt.Errorf("stripe %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		return nil
	}
	// Immediate cancel — DELETE on the subscription resource.
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		"https://api.stripe.com/v1/subscriptions/"+meta.SubscriptionID, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.SecretKey)
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("stripe %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// CustomerProfile is the address + identity the orchestrator stamps onto
// a Stripe Customer at creation time so Stripe Tax has a tax basis.
// Every field is optional; missing values are NOT silently substituted
// — instead we annotate the customer's metadata so finance can see the
// user opted not to provide. (Stripe Tax can still operate from the
// address Checkout collects at payment time.)
type CustomerProfile struct {
	Name       string
	Email      string
	Line1      string
	Line2      string
	City       string
	State      string
	PostalCode string
	Country    string // ISO-3166 alpha-2; "" leaves Stripe to infer
}

// EnsureCustomer returns the Stripe Customer ID for uid, creating one
// when none exists. Used by the Customer Portal mutation — Stripe
// requires a customer id even for users who never went through
// Checkout (e.g. enterprise customers seeded out-of-band). When the
// caller passes a non-empty CustomerProfile, the fields land on the
// new Customer so Stripe Tax has an address basis from day one;
// otherwise the customer is created with a `metadata[address_status]=
// user opted not to provide` marker so the finance audit trail is
// honest about why the basis is empty.
func (s *StripeService) EnsureCustomer(ctx context.Context, uid string, profile CustomerProfile) (string, error) {
	if !s.Enabled() {
		return "", errors.New("stripe disabled")
	}
	if strings.TrimSpace(uid) == "" {
		return "", errors.New("uid required")
	}
	// Reuse the existing customer when search finds one.
	if cid, _ := s.FindCustomerByUserID(ctx, uid); cid != "" {
		return cid, nil
	}
	form := url.Values{}
	form.Set("metadata[user_id]", uid)
	if profile.Email != "" {
		form.Set("email", profile.Email)
	}
	if profile.Name != "" {
		form.Set("name", profile.Name)
	}
	hasAddress := profile.Line1 != "" || profile.City != "" || profile.PostalCode != "" || profile.Country != ""
	if hasAddress {
		if profile.Line1 != "" {
			form.Set("address[line1]", profile.Line1)
		}
		if profile.Line2 != "" {
			form.Set("address[line2]", profile.Line2)
		}
		if profile.City != "" {
			form.Set("address[city]", profile.City)
		}
		if profile.State != "" {
			form.Set("address[state]", profile.State)
		}
		if profile.PostalCode != "" {
			form.Set("address[postal_code]", profile.PostalCode)
		}
		if profile.Country != "" {
			form.Set("address[country]", profile.Country)
		}
		form.Set("metadata[address_status]", "provided")
	} else {
		// Honest marker for finance auditors — Stripe Tax will fall
		// back to the address Checkout collects at payment time.
		form.Set("metadata[address_status]", "user opted not to provide")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.stripe.com/v1/customers",
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
		return "", fmt.Errorf("stripe customers %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("parse stripe customer: %w", err)
	}
	if out.ID == "" {
		return "", errors.New("stripe returned no customer id")
	}
	return out.ID, nil
}

// FindCustomerByUserID looks up a Stripe customer that was tagged with
// metadata.user_id == uid. Returns "" with no error when no match. The
// orchestrator stamps user_id into checkout.session metadata at
// Checkout time; Stripe propagates that onto the resulting Customer.
func (s *StripeService) FindCustomerByUserID(ctx context.Context, uid string) (string, error) {
	if !s.Enabled() || strings.TrimSpace(uid) == "" {
		return "", nil
	}
	// Stripe's Search API is the right call here; if the account doesn't
	// have search enabled we fall back to listing customers.
	q := url.Values{}
	q.Set("query", "metadata['user_id']:'"+uid+"'")
	q.Set("limit", "1")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.stripe.com/v1/customers/search?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.SecretKey)
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		// Search endpoint may be disabled on legacy accounts; treat as
		// "no match" so the resolver still answers.
		return "", nil
	}
	var out struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", nil
	}
	if len(out.Data) == 0 {
		return "", nil
	}
	return stripeString(out.Data[0], "id"), nil
}

// Invoice is the projection of Stripe's Invoice the dashboard renders.
// Amounts stay in cents (Stripe-native) so we don't lose precision
// converting through float; the hosted + PDF URLs come straight from
// Stripe so users can download the official document without us
// re-rendering anything.
type Invoice struct {
	ID               string
	AmountCents      int64
	Currency         string
	Status           string
	PeriodStart      time.Time
	PeriodEnd        time.Time
	HostedInvoiceURL string
	InvoicePDFURL    string
	CreatedAt        time.Time
}

// ListInvoices fetches the customer's recent invoices from Stripe. The
// hosted_invoice_url and invoice_pdf fields are exactly what Stripe
// emits — operators don't need to render anything orchestrator-side.
// When Stripe is disabled or customerID is empty we return an empty
// slice with no error so dev environments render a clean "no invoices
// yet" state.
func (s *StripeService) ListInvoices(ctx context.Context, customerID string, limit int) ([]Invoice, error) {
	if !s.Enabled() || strings.TrimSpace(customerID) == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 12
	}
	q := url.Values{}
	q.Set("customer", customerID)
	q.Set("limit", strconv.Itoa(limit))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.stripe.com/v1/invoices?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.SecretKey)
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("stripe invoices %d: %s",
			resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse stripe invoices: %w", err)
	}
	invoices := make([]Invoice, 0, len(out.Data))
	for _, row := range out.Data {
		inv := Invoice{
			ID:               stripeString(row, "id"),
			Currency:         strings.ToUpper(stripeString(row, "currency")),
			Status:           stripeString(row, "status"),
			HostedInvoiceURL: stripeString(row, "hosted_invoice_url"),
			InvoicePDFURL:    stripeString(row, "invoice_pdf"),
		}
		if amt, ok := stripeNum(row, "amount_due"); ok {
			inv.AmountCents = amt
		}
		if ts, ok := stripeNum(row, "period_start"); ok && ts > 0 {
			inv.PeriodStart = time.Unix(ts, 0).UTC()
		}
		if ts, ok := stripeNum(row, "period_end"); ok && ts > 0 {
			inv.PeriodEnd = time.Unix(ts, 0).UTC()
		}
		if ts, ok := stripeNum(row, "created"); ok && ts > 0 {
			inv.CreatedAt = time.Unix(ts, 0).UTC()
		}
		invoices = append(invoices, inv)
	}
	return invoices, nil
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
		uid := stripeUserID(obj)
		// Always clear the payment-failed flag — if a card update succeeded
		// and the next invoice paid, the user should be unblocked
		// regardless of billing reason.
		if uid != "" && b.PayFlags != nil {
			_ = b.PayFlags.Clear(ctx, uid)
		}
		if reason == "subscription_create" {
			return nil
		}
		cents, _ := stripeNum(obj, "amount_paid")
		if cents <= 0 {
			return nil
		}
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

	case "invoice.created":
		// Stripe just generated the next period's invoice (which includes
		// any metered usage we reported). Return nil and let the caller's
		// audit hook record the event. We don't touch the vault yet —
		// invoice.payment_succeeded above is the cash-receipt signal.
		return nil

	case "invoice.payment_failed":
		// User's card declined. Flip the payment flag so the metered
		// reporter stops accumulating new usage records the user can't
		// pay for; the UI will block further paid calls until the user
		// updates their payment method (invoice.payment_succeeded clears it).
		uid := stripeUserID(obj)
		if uid != "" && b.PayFlags != nil {
			_ = b.PayFlags.Block(ctx, uid,
				"invoice payment failed: "+stripeString(obj, "id"))
		}
		return nil

	case "customer.subscription.updated":
		// Subscription items changed (user upgraded/downgraded, or the
		// metered SubscriptionItem ID rotated). Refresh the cached plan
		// tier from metadata and pin the metered SubscriptionItem so the
		// reporter has somewhere to POST usage records.
		uid := stripeUserID(obj)
		if uid == "" {
			return nil
		}
		if tier := stripeTier(obj); tier != "" {
			if ps != nil {
				_ = ps.SetPlan(ctx, uid, string(tier))
			}
			b.AssignPlan(ctx, uid, tier)
		}
		if b.SubItems != nil {
			if item := stripeMeteredItem(obj); item != "" {
				_ = b.SubItems.Set(ctx, uid, item)
			}
		}
		return nil
	}
	return nil
}

// stripeMeteredItem digs into a subscription event payload and returns the
// metered SubscriptionItem ID. Stripe nests items under data.object.items.data;
// we pick the first item whose price.recurring.usage_type == "metered".
func stripeMeteredItem(obj map[string]any) string {
	items, ok := obj["items"].(map[string]any)
	if !ok {
		return ""
	}
	data, ok := items["data"].([]any)
	if !ok {
		return ""
	}
	for _, raw := range data {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		price, _ := row["price"].(map[string]any)
		if price == nil {
			continue
		}
		rec, _ := price["recurring"].(map[string]any)
		if rec == nil {
			continue
		}
		if usage, _ := rec["usage_type"].(string); usage == "metered" {
			if id, _ := row["id"].(string); id != "" {
				return id
			}
		}
	}
	return ""
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
