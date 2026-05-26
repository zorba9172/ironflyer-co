package deploy

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

type DomainKind string

const (
	DomainKindManaged    DomainKind = "managed_subdomain"
	DomainKindConnected  DomainKind = "connected_domain"
	DomainKindRegistered DomainKind = "registered_domain"
)

func (k DomainKind) String() string { return string(k) }

type DomainStatus string

const (
	DomainStatusPendingDNS DomainStatus = "pending_dns"
	DomainStatusVerifying  DomainStatus = "verifying"
	DomainStatusLive       DomainStatus = "live"
	DomainStatusFailed     DomainStatus = "failed"
	DomainStatusRemoved    DomainStatus = "removed"
)

func (s DomainStatus) String() string { return string(s) }

type CertificateStatus string

const (
	CertificateStatusPending CertificateStatus = "pending"
	CertificateStatusActive  CertificateStatus = "active"
	CertificateStatusFailed  CertificateStatus = "failed"
)

func (s CertificateStatus) String() string { return string(s) }

type DNSRecord struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Value string `json:"value"`
	TTL   int    `json:"ttl,omitempty"`
}

type Domain struct {
	ID                 string
	TenantID           string
	ProjectID          string
	DeployID           string
	Hostname           string
	Kind               DomainKind
	Status             DomainStatus
	Provider           string
	Registrar          string
	Primary            bool
	DNSRecords         []DNSRecord
	VerificationStatus string
	CertificateStatus  CertificateStatus
	Instructions       string
	Metadata           map[string]any
	CreatedAt          time.Time
	UpdatedAt          time.Time
	VerifiedAt         *time.Time
	LiveAt             *time.Time
}

type ReserveSubdomainInput struct {
	TenantID  string
	ProjectID string
	DeployID  string
	Subdomain string
	Provider  string
	Primary   bool
	Metadata  map[string]any
}

type ConnectDomainInput struct {
	TenantID  string
	ProjectID string
	DeployID  string
	Hostname  string
	Provider  string
	Primary   bool
	Metadata  map[string]any
}

type PurchaseDomainInput struct {
	TenantID         string
	ProjectID        string
	DeployID         string
	Domain           string
	Provider         string
	Registrar        string
	Years            int
	AutoRenew        bool
	ExpectedPriceUSD decimal.Decimal
	Contact          map[string]string
	Primary          bool
	Metadata         map[string]any
}

type DomainPurchasePolicy struct {
	Enabled                  bool
	MaxPriceUSD              decimal.Decimal
	PriceTolerancePct        decimal.Decimal
	RequireRegistrantContact bool
}

func DefaultDomainPurchasePolicy() DomainPurchasePolicy {
	return DomainPurchasePolicy{
		Enabled:           false,
		MaxPriceUSD:       decimal.NewFromInt(75),
		PriceTolerancePct: decimal.NewFromInt(10),
	}
}

type DomainAvailability struct {
	Domain       string
	Available    bool
	Registrar    string
	PriceUSD     decimal.Decimal
	Currency     string
	Premium      bool
	CanPurchase  bool
	Reason       string
	CheckedAt    time.Time
	Requirements []string
}

type DomainProvisionRequest struct {
	TenantID  string
	ProjectID string
	DeployID  string
	Hostname  string
	Kind      DomainKind
	Primary   bool
	Metadata  map[string]any
}

type DomainProvisionResult struct {
	Provider              string
	DNSRecords            []DNSRecord
	Status                DomainStatus
	VerificationStatus    string
	CertificateStatus     CertificateStatus
	Instructions          string
	ProviderDomainID      string
	ProviderCertificateID string
}

type DomainCheckResult struct {
	Status             DomainStatus
	VerificationStatus string
	CertificateStatus  CertificateStatus
	DNSRecords         []DNSRecord
	Instructions       string
	Metadata           map[string]any
}

type DomainProvider interface {
	Name() string
	ManagedBaseDomain() string
	Provision(ctx context.Context, req DomainProvisionRequest) (DomainProvisionResult, error)
	Check(ctx context.Context, d Domain) (DomainCheckResult, error)
}

type Registrar interface {
	Name() string
	Availability(ctx context.Context, domain string) (DomainAvailability, error)
	Purchase(ctx context.Context, in PurchaseDomainInput) (DomainAvailability, error)
}

type DomainService interface {
	ReserveSubdomain(ctx context.Context, in ReserveSubdomainInput) (Domain, error)
	ConnectDomain(ctx context.Context, in ConnectDomainInput) (Domain, error)
	PurchaseDomain(ctx context.Context, in PurchaseDomainInput) (Domain, error)
	CheckDomain(ctx context.Context, tenantID, id string) (Domain, error)
	SetPrimaryDomain(ctx context.Context, tenantID, id string) (Domain, error)
	ListProjectDomains(ctx context.Context, tenantID, projectID string) ([]Domain, error)
	DomainAvailability(ctx context.Context, registrar, domain string) (DomainAvailability, error)
}
