package deploy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
)

type StaticDomainProvider struct {
	name       string
	baseDomain string
	target     string
}

func NewStaticDomainProvider(name, baseDomain, target string) StaticDomainProvider {
	if strings.TrimSpace(name) == "" {
		name = "ironflyer"
	}
	if strings.TrimSpace(baseDomain) == "" {
		baseDomain = "ironflyer.app"
	}
	if strings.TrimSpace(target) == "" {
		target = "edge.ironflyer.app"
	}
	return StaticDomainProvider{name: name, baseDomain: normalizeHostname(baseDomain), target: normalizeHostname(target)}
}

func (p StaticDomainProvider) Name() string { return p.name }

func (p StaticDomainProvider) ManagedBaseDomain() string { return p.baseDomain }

func (p StaticDomainProvider) Provision(_ context.Context, req DomainProvisionRequest) (DomainProvisionResult, error) {
	if err := validateHostname(req.Hostname); err != nil {
		return DomainProvisionResult{}, err
	}
	if req.Kind == DomainKindManaged && !sameApex(req.Hostname, p.baseDomain) {
		return DomainProvisionResult{}, fmt.Errorf("%w: managed hostname must live under %s", ErrInvalidState, p.baseDomain)
	}
	status := DomainStatusPendingDNS
	verify := "dns_pending"
	cert := CertificateStatusPending
	if req.Kind == DomainKindManaged {
		status = DomainStatusLive
		verify = "verified"
		cert = CertificateStatusActive
	}
	return DomainProvisionResult{
		Provider:           p.Name(),
		DNSRecords:         sortedDNSRecords(dnsTargetFor(req.Hostname, p.target)),
		Status:             status,
		VerificationStatus: verify,
		CertificateStatus:  cert,
		Instructions:       "Add the DNS records exactly as shown. Ironflyer will verify ownership and issue TLS automatically.",
	}, nil
}

func (p StaticDomainProvider) Check(_ context.Context, d Domain) (DomainCheckResult, error) {
	if d.Kind == DomainKindManaged {
		return DomainCheckResult{
			Status:             DomainStatusLive,
			VerificationStatus: "verified",
			CertificateStatus:  CertificateStatusActive,
			DNSRecords:         d.DNSRecords,
			Instructions:       "Managed Ironflyer subdomain is live.",
		}, nil
	}
	status := DomainStatusVerifying
	verify := "dns_pending"
	cert := CertificateStatusPending
	if strings.EqualFold(fmt.Sprint(d.Metadata["dns_verified"]), "true") {
		status = DomainStatusLive
		verify = "verified"
		cert = CertificateStatusActive
	}
	return DomainCheckResult{
		Status:             status,
		VerificationStatus: verify,
		CertificateStatus:  cert,
		DNSRecords:         d.DNSRecords,
		Instructions:       "Waiting for DNS propagation. Keep the TXT ownership record until the domain is live.",
	}, nil
}

type NoopRegistrar struct {
	name string
}

func NewNoopRegistrar(name string) NoopRegistrar {
	if strings.TrimSpace(name) == "" {
		name = "manual"
	}
	return NoopRegistrar{name: name}
}

func (r NoopRegistrar) Name() string { return r.name }

func (r NoopRegistrar) Availability(_ context.Context, domain string) (DomainAvailability, error) {
	domain = normalizeHostname(domain)
	if err := validateHostname(domain); err != nil {
		return DomainAvailability{}, err
	}
	return DomainAvailability{
		Domain:       domain,
		Available:    false,
		Registrar:    r.name,
		CanPurchase:  false,
		Reason:       "registrar API is not configured; connect an existing domain or configure Cloudflare/Vercel registrar credentials",
		CheckedAt:    time.Now().UTC(),
		Requirements: []string{"registrar_api_token", "billing_profile", "registrant_contact"},
	}, nil
}

func (r NoopRegistrar) Purchase(ctx context.Context, in PurchaseDomainInput) (DomainAvailability, error) {
	avail, err := r.Availability(ctx, in.Domain)
	if err != nil {
		return DomainAvailability{}, err
	}
	avail.Reason = "purchase is disabled for the manual registrar"
	return avail, ErrProviderFailure
}

type CloudflareRegistrar struct {
	secrets   SecretResolver
	client    *http.Client
	accountID string
	base      string
	log       zerolog.Logger
}

func NewCloudflareRegistrar(secrets SecretResolver, client *http.Client, accountID, base string, log zerolog.Logger) *CloudflareRegistrar {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if strings.TrimSpace(base) == "" {
		base = "https://api.cloudflare.com/client/v4"
	}
	return &CloudflareRegistrar{
		secrets:   secrets,
		client:    client,
		accountID: strings.TrimSpace(accountID),
		base:      strings.TrimRight(base, "/"),
		log:       log,
	}
}

func (*CloudflareRegistrar) Name() string { return "cloudflare" }

func (r *CloudflareRegistrar) Availability(ctx context.Context, domain string) (DomainAvailability, error) {
	domain = normalizeHostname(domain)
	if err := validateHostname(domain); err != nil {
		return DomainAvailability{}, err
	}
	if r == nil || r.secrets == nil || r.accountID == "" {
		return DomainAvailability{}, ErrSecretMissing
	}
	var resp struct {
		Result []struct {
			DomainName               string  `json:"domain_name"`
			Available                bool    `json:"available"`
			CanRegisterViaAPI        bool    `json:"can_register_via_api"`
			Premium                  bool    `json:"premium"`
			Price                    float64 `json:"price"`
			Currency                 string  `json:"currency"`
			UnsupportedReason        string  `json:"unsupported_reason"`
			AvailabilityCheckMessage string  `json:"availability_check_message"`
		} `json:"result"`
	}
	if err := r.doJSON(ctx, http.MethodPost, "/accounts/"+r.accountID+"/registrar/domain-check", map[string]any{
		"domains": []string{domain},
	}, &resp); err != nil {
		return DomainAvailability{}, err
	}
	out := DomainAvailability{Domain: domain, Registrar: r.Name(), CheckedAt: time.Now().UTC()}
	if len(resp.Result) == 0 {
		out.Reason = "no registrar result returned"
		return out, nil
	}
	got := resp.Result[0]
	out.Domain = firstNonEmpty(got.DomainName, domain)
	out.Available = got.Available
	out.CanPurchase = got.Available && got.CanRegisterViaAPI
	out.Premium = got.Premium
	out.PriceUSD = decimal.NewFromFloat(got.Price)
	out.Currency = firstNonEmpty(got.Currency, "USD")
	out.Reason = firstNonEmpty(got.UnsupportedReason, got.AvailabilityCheckMessage)
	return out, nil
}

func (r *CloudflareRegistrar) Purchase(ctx context.Context, in PurchaseDomainInput) (DomainAvailability, error) {
	domain := normalizeHostname(in.Domain)
	if err := validateHostname(domain); err != nil {
		return DomainAvailability{}, err
	}
	if r == nil || r.secrets == nil || r.accountID == "" {
		return DomainAvailability{}, ErrSecretMissing
	}
	body := map[string]any{
		"domain_name": domain,
		"auto_renew":  in.AutoRenew,
	}
	if len(in.Contact) > 0 {
		body["registrant_contact"] = in.Contact
	}
	var resp struct {
		Result map[string]any `json:"result"`
	}
	if err := r.doJSON(ctx, http.MethodPost, "/accounts/"+r.accountID+"/registrar/registrations", body, &resp); err != nil {
		return DomainAvailability{}, err
	}
	return DomainAvailability{
		Domain:      domain,
		Available:   false,
		Registrar:   r.Name(),
		CanPurchase: true,
		PriceUSD:    in.ExpectedPriceUSD,
		Currency:    "USD",
		Reason:      "registration submitted",
		CheckedAt:   time.Now().UTC(),
	}, nil
}

func (r *CloudflareRegistrar) doJSON(ctx context.Context, method, path string, body, out any) error {
	token, err := r.secrets.Resolve(ctx, tenantFromContext(ctx), projectFromContext(ctx), "CLOUDFLARE_API_TOKEN")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSecretMissing, err)
	}
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, r.base+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(token)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrProviderFailure, err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		r.log.Warn().Int("status", res.StatusCode).Str("path", path).Bytes("body", raw).Msg("cloudflare registrar api error")
		return fmt.Errorf("%w: status=%d body=%s", ErrProviderFailure, res.StatusCode, string(raw))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("cloudflare registrar: decode: %w", err)
	}
	return nil
}
