package deploy

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type MemoryDomainService struct {
	providers        map[string]DomainProvider
	registrars       map[string]Registrar
	defaultProvider  string
	defaultRegistrar string
	purchasePolicy   DomainPurchasePolicy
	// pg is the optional BeforeDomainPurchase guard. nil means
	// "ProfitGuard not wired" — PurchaseDomain proceeds permissively.
	// Wire via WithDomainProfitGuard at construction time.
	pg ProfitGuardChecker

	mu      sync.RWMutex
	domains map[string]*Domain
}

func NewMemoryDomainService(providers map[string]DomainProvider, registrars map[string]Registrar, defaultProvider, defaultRegistrar string, opts ...DomainServiceOption) *MemoryDomainService {
	if providers == nil {
		providers = map[string]DomainProvider{}
	}
	if registrars == nil {
		registrars = map[string]Registrar{}
	}
	if defaultProvider == "" {
		defaultProvider = "ironflyer"
	}
	if defaultRegistrar == "" {
		defaultRegistrar = "manual"
	}
	if _, ok := providers[defaultProvider]; !ok {
		providers[defaultProvider] = NewStaticDomainProvider(defaultProvider, "ironflyer.app", "edge.ironflyer.app")
	}
	if _, ok := registrars[defaultRegistrar]; !ok {
		registrars[defaultRegistrar] = NewNoopRegistrar(defaultRegistrar)
	}
	svc := &MemoryDomainService{
		providers:        providers,
		registrars:       registrars,
		defaultProvider:  defaultProvider,
		defaultRegistrar: defaultRegistrar,
		purchasePolicy:   DefaultDomainPurchasePolicy(),
		domains:          map[string]*Domain{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

func (s *MemoryDomainService) ReserveSubdomain(ctx context.Context, in ReserveSubdomainInput) (Domain, error) {
	if in.TenantID == "" || in.ProjectID == "" {
		return Domain{}, fmt.Errorf("%w: tenant_id and project_id required", ErrInvalidState)
	}
	provider, name, err := s.provider(in.Provider)
	if err != nil {
		return Domain{}, err
	}
	sub := normalizeSubdomain(in.Subdomain, in.ProjectID)
	host := managedHostname(sub, provider.ManagedBaseDomain())
	prov, err := provider.Provision(ctx, DomainProvisionRequest{
		TenantID: in.TenantID, ProjectID: in.ProjectID, DeployID: in.DeployID,
		Hostname: host, Kind: DomainKindManaged, Primary: in.Primary, Metadata: in.Metadata,
	})
	if err != nil {
		return Domain{}, err
	}
	return s.insert(Domain{
		ID: uuid.NewString(), TenantID: in.TenantID, ProjectID: in.ProjectID, DeployID: in.DeployID,
		Hostname: host, Kind: DomainKindManaged, Status: prov.Status, Provider: firstNonEmpty(prov.Provider, name),
		Primary: in.Primary, DNSRecords: sortedDNSRecords(prov.DNSRecords),
		VerificationStatus: prov.VerificationStatus, CertificateStatus: prov.CertificateStatus,
		Instructions: prov.Instructions, Metadata: mergeDomainMetadata(in.Metadata, providerMeta(prov)),
	})
}

func (s *MemoryDomainService) ConnectDomain(ctx context.Context, in ConnectDomainInput) (Domain, error) {
	if in.TenantID == "" || in.ProjectID == "" {
		return Domain{}, fmt.Errorf("%w: tenant_id and project_id required", ErrInvalidState)
	}
	host := normalizeHostname(in.Hostname)
	if err := validateHostname(host); err != nil {
		return Domain{}, err
	}
	provider, name, err := s.provider(in.Provider)
	if err != nil {
		return Domain{}, err
	}
	prov, err := provider.Provision(ctx, DomainProvisionRequest{
		TenantID: in.TenantID, ProjectID: in.ProjectID, DeployID: in.DeployID,
		Hostname: host, Kind: DomainKindConnected, Primary: in.Primary, Metadata: in.Metadata,
	})
	if err != nil {
		return Domain{}, err
	}
	return s.insert(Domain{
		ID: uuid.NewString(), TenantID: in.TenantID, ProjectID: in.ProjectID, DeployID: in.DeployID,
		Hostname: host, Kind: DomainKindConnected, Status: prov.Status, Provider: firstNonEmpty(prov.Provider, name),
		Primary: in.Primary, DNSRecords: sortedDNSRecords(prov.DNSRecords),
		VerificationStatus: prov.VerificationStatus, CertificateStatus: prov.CertificateStatus,
		Instructions: prov.Instructions, Metadata: mergeDomainMetadata(in.Metadata, providerMeta(prov)),
	})
}

func (s *MemoryDomainService) PurchaseDomain(ctx context.Context, in PurchaseDomainInput) (Domain, error) {
	if in.Registrar == "" {
		in.Registrar = s.defaultRegistrar
	}
	registrar, ok := s.registrars[in.Registrar]
	if !ok {
		return Domain{}, fmt.Errorf("%w: registrar %s", ErrUnknownTarget, in.Registrar)
	}
	purchaseCtx := WithTenant(WithProject(ctx, in.ProjectID), in.TenantID)
	avail, err := s.validateDomainPurchase(purchaseCtx, registrar, in)
	if err != nil {
		return Domain{}, err
	}
	// BeforeDomainPurchase ProfitGuard hook — refuse the registrar
	// call when margin has collapsed (V22 deferred site, now closed).
	// avail.PriceUSD has already been clamped by validateDomainPurchase
	// against DomainPurchasePolicy.MaxPriceUSD ($75 default); this gate
	// adds a tenant-level margin check before any wire money is spent.
	if err := GuardDomainPurchase(purchaseCtx, s.pg, map[string]any{
		"tenant_id":         in.TenantID,
		"project_id":        in.ProjectID,
		"domain":            normalizeHostname(in.Domain),
		"registrar":         registrar.Name(),
		"price_usd":         avail.PriceUSD.StringFixedBank(2),
		"max_price_usd":     s.purchasePolicy.MaxPriceUSD.StringFixedBank(2),
		"enforcement_point": "before_domain_purchase",
	}); err != nil {
		return Domain{}, err
	}
	if _, err := registrar.Purchase(purchaseCtx, in); err != nil {
		return Domain{}, err
	}
	d, err := s.ConnectDomain(ctx, ConnectDomainInput{
		TenantID: in.TenantID, ProjectID: in.ProjectID, DeployID: in.DeployID,
		Hostname: in.Domain, Provider: in.Provider, Primary: in.Primary,
		Metadata: domainPurchaseMetadata(in.Metadata, registrar, avail),
	})
	if err != nil {
		return Domain{}, err
	}
	d.Kind = DomainKindRegistered
	d.Registrar = registrar.Name()
	s.mu.Lock()
	if row := s.domains[d.ID]; row != nil {
		row.Kind = d.Kind
		row.Registrar = d.Registrar
		d = cloneDomain(row)
	}
	s.mu.Unlock()
	return d, nil
}

func (s *MemoryDomainService) CheckDomain(ctx context.Context, tenantID, id string) (Domain, error) {
	s.mu.RLock()
	row, ok := s.domains[id]
	if !ok || row.TenantID != tenantID {
		s.mu.RUnlock()
		return Domain{}, ErrNotFound
	}
	d := cloneDomain(row)
	s.mu.RUnlock()
	provider, _, err := s.provider(d.Provider)
	if err != nil {
		return Domain{}, err
	}
	check, err := provider.Check(ctx, d)
	if err != nil {
		return Domain{}, err
	}
	now := time.Now().UTC()
	s.mu.Lock()
	row = s.domains[id]
	if row == nil || row.TenantID != tenantID {
		s.mu.Unlock()
		return Domain{}, ErrNotFound
	}
	row.Status = check.Status
	row.VerificationStatus = check.VerificationStatus
	row.CertificateStatus = check.CertificateStatus
	if len(check.DNSRecords) > 0 {
		row.DNSRecords = sortedDNSRecords(check.DNSRecords)
	}
	if check.Instructions != "" {
		row.Instructions = check.Instructions
	}
	row.Metadata = mergeDomainMetadata(row.Metadata, check.Metadata)
	row.UpdatedAt = now
	if check.VerificationStatus == "verified" && row.VerifiedAt == nil {
		t := now
		row.VerifiedAt = &t
	}
	if check.Status == DomainStatusLive && row.LiveAt == nil {
		t := now
		row.LiveAt = &t
	}
	out := cloneDomain(row)
	s.mu.Unlock()
	return out, nil
}

func (s *MemoryDomainService) SetPrimaryDomain(_ context.Context, tenantID, id string) (Domain, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.domains[id]
	if !ok || row.TenantID != tenantID {
		return Domain{}, ErrNotFound
	}
	for _, d := range s.domains {
		if d.TenantID == row.TenantID && d.ProjectID == row.ProjectID {
			d.Primary = false
			d.UpdatedAt = time.Now().UTC()
		}
	}
	row.Primary = true
	row.UpdatedAt = time.Now().UTC()
	return cloneDomain(row), nil
}

func (s *MemoryDomainService) ListProjectDomains(_ context.Context, tenantID, projectID string) ([]Domain, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Domain, 0)
	for _, d := range s.domains {
		if d.TenantID == tenantID && d.ProjectID == projectID && d.Status != DomainStatusRemoved {
			out = append(out, cloneDomain(d))
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Primary != out[j].Primary {
			return out[i].Primary
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func (s *MemoryDomainService) DomainAvailability(ctx context.Context, registrarName, domain string) (DomainAvailability, error) {
	if registrarName == "" {
		registrarName = s.defaultRegistrar
	}
	registrar, ok := s.registrars[registrarName]
	if !ok {
		return DomainAvailability{}, fmt.Errorf("%w: registrar %s", ErrUnknownTarget, registrarName)
	}
	return registrar.Availability(ctx, domain)
}

func (s *MemoryDomainService) insert(d Domain) (Domain, error) {
	now := time.Now().UTC()
	d.Hostname = normalizeHostname(d.Hostname)
	if err := validateHostname(d.Hostname); err != nil {
		return Domain{}, err
	}
	d.CreatedAt = now
	d.UpdatedAt = now
	if d.Status == "" {
		d.Status = DomainStatusPendingDNS
	}
	if d.CertificateStatus == "" {
		d.CertificateStatus = CertificateStatusPending
	}
	if d.Metadata == nil {
		d.Metadata = map[string]any{}
	}
	if d.Status == DomainStatusLive {
		t := now
		d.LiveAt = &t
		d.VerifiedAt = &t
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.domains {
		if existing.TenantID == d.TenantID && strings.EqualFold(existing.Hostname, d.Hostname) && existing.Status != DomainStatusRemoved {
			return Domain{}, fmt.Errorf("%w: domain already connected", ErrInvalidState)
		}
	}
	if d.Primary {
		for _, existing := range s.domains {
			if existing.TenantID == d.TenantID && existing.ProjectID == d.ProjectID {
				existing.Primary = false
			}
		}
	}
	s.domains[d.ID] = &d
	return cloneDomain(&d), nil
}

func (s *MemoryDomainService) provider(name string) (DomainProvider, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = s.defaultProvider
	}
	provider, ok := s.providers[name]
	if !ok {
		return nil, name, fmt.Errorf("%w: domain provider %s", ErrUnknownTarget, name)
	}
	return provider, name, nil
}

func cloneDomain(d *Domain) Domain {
	if d == nil {
		return Domain{}
	}
	out := *d
	out.DNSRecords = append([]DNSRecord(nil), d.DNSRecords...)
	out.Metadata = copyMapAny(d.Metadata)
	return out
}

func providerMeta(prov DomainProvisionResult) map[string]any {
	out := map[string]any{}
	if prov.ProviderDomainID != "" {
		out["provider_domain_id"] = prov.ProviderDomainID
	}
	if prov.ProviderCertificateID != "" {
		out["provider_certificate_id"] = prov.ProviderCertificateID
	}
	return out
}
