package deploy

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestMemoryDomainServiceReserveManagedSubdomain(t *testing.T) {
	svc := NewMemoryDomainService(
		map[string]DomainProvider{"ironflyer": NewStaticDomainProvider("ironflyer", "ironflyer.test", "edge.ironflyer.test")},
		nil,
		"ironflyer",
		"manual",
	)
	got, err := svc.ReserveSubdomain(context.Background(), ReserveSubdomainInput{
		TenantID: "tenant-1", ProjectID: "project-1", Subdomain: "My App!", Primary: true,
	})
	if err != nil {
		t.Fatalf("ReserveSubdomain: %v", err)
	}
	if got.Hostname != "my-app.ironflyer.test" {
		t.Fatalf("hostname = %q", got.Hostname)
	}
	if got.Status != DomainStatusLive || got.CertificateStatus != CertificateStatusActive {
		t.Fatalf("unexpected status: %+v", got)
	}
	if !got.Primary {
		t.Fatalf("managed subdomain should be primary")
	}
}

func TestMemoryDomainServiceConnectDomainAndSetPrimary(t *testing.T) {
	svc := NewMemoryDomainService(
		map[string]DomainProvider{"ironflyer": NewStaticDomainProvider("ironflyer", "ironflyer.test", "edge.ironflyer.test")},
		nil,
		"ironflyer",
		"manual",
	)
	ctx := context.Background()
	managed, err := svc.ReserveSubdomain(ctx, ReserveSubdomainInput{
		TenantID: "tenant-1", ProjectID: "project-1", Subdomain: "app", Primary: true,
	})
	if err != nil {
		t.Fatalf("ReserveSubdomain: %v", err)
	}
	custom, err := svc.ConnectDomain(ctx, ConnectDomainInput{
		TenantID: "tenant-1", ProjectID: "project-1", Hostname: "www.example.com", Primary: false,
	})
	if err != nil {
		t.Fatalf("ConnectDomain: %v", err)
	}
	if custom.Status != DomainStatusPendingDNS {
		t.Fatalf("custom status = %q", custom.Status)
	}
	if len(custom.DNSRecords) == 0 {
		t.Fatalf("expected DNS records")
	}
	primary, err := svc.SetPrimaryDomain(ctx, "tenant-1", custom.ID)
	if err != nil {
		t.Fatalf("SetPrimaryDomain: %v", err)
	}
	if !primary.Primary {
		t.Fatalf("custom domain should be primary")
	}
	rows, err := svc.ListProjectDomains(ctx, "tenant-1", "project-1")
	if err != nil {
		t.Fatalf("ListProjectDomains: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(rows))
	}
	if rows[0].ID != custom.ID || !rows[0].Primary {
		t.Fatalf("primary domain should sort first: %+v", rows)
	}
	refreshed, err := svc.CheckDomain(ctx, "tenant-1", managed.ID)
	if err != nil {
		t.Fatalf("CheckDomain: %v", err)
	}
	if refreshed.Status != DomainStatusLive {
		t.Fatalf("managed check should stay live: %+v", refreshed)
	}
}

func TestNoopRegistrarAvailabilityIsManual(t *testing.T) {
	got, err := NewNoopRegistrar("manual").Availability(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("Availability: %v", err)
	}
	if got.CanPurchase || got.Available {
		t.Fatalf("manual registrar should not purchase: %+v", got)
	}
	if len(got.Requirements) == 0 {
		t.Fatalf("expected requirements")
	}
}

func TestMemoryDomainServicePurchaseDisabledByDefault(t *testing.T) {
	svc := NewMemoryDomainService(
		map[string]DomainProvider{"ironflyer": NewStaticDomainProvider("ironflyer", "ironflyer.test", "edge.ironflyer.test")},
		map[string]Registrar{"test": &fakeRegistrar{price: decimal.NewFromInt(12), canPurchase: true}},
		"ironflyer",
		"test",
	)
	_, err := svc.PurchaseDomain(context.Background(), PurchaseDomainInput{
		TenantID: "tenant-1", ProjectID: "project-1", Domain: "example.com",
		Registrar: "test", Years: 1, ExpectedPriceUSD: decimal.NewFromInt(12),
	})
	if err == nil {
		t.Fatal("expected disabled purchase to fail")
	}
	if !errors.Is(err, ErrInvalidState) {
		t.Fatalf("expected ErrInvalidState, got %v", err)
	}
}

func TestMemoryDomainServicePurchaseChecksServerSidePrice(t *testing.T) {
	registrar := &fakeRegistrar{price: decimal.NewFromInt(12), canPurchase: true}
	svc := NewMemoryDomainService(
		map[string]DomainProvider{"ironflyer": NewStaticDomainProvider("ironflyer", "ironflyer.test", "edge.ironflyer.test")},
		map[string]Registrar{"test": registrar},
		"ironflyer",
		"test",
		WithDomainPurchasePolicy(DomainPurchasePolicy{
			Enabled:           true,
			MaxPriceUSD:       decimal.NewFromInt(20),
			PriceTolerancePct: decimal.NewFromInt(10),
		}),
	)
	got, err := svc.PurchaseDomain(context.Background(), PurchaseDomainInput{
		TenantID: "tenant-1", ProjectID: "project-1", Domain: "example.com",
		Registrar: "test", Years: 1, ExpectedPriceUSD: decimal.NewFromInt(12),
		Primary: true,
	})
	if err != nil {
		t.Fatalf("PurchaseDomain: %v", err)
	}
	if got.Kind != DomainKindRegistered || got.Registrar != "test" {
		t.Fatalf("unexpected registered domain: %+v", got)
	}
	if registrar.purchases != 1 {
		t.Fatalf("expected one registrar purchase, got %d", registrar.purchases)
	}
	if got.Metadata["purchase_price_usd"] != "12.00" {
		t.Fatalf("missing purchase metadata: %+v", got.Metadata)
	}
}

func TestMemoryDomainServicePurchaseBlocksPriceDrift(t *testing.T) {
	registrar := &fakeRegistrar{price: decimal.NewFromInt(30), canPurchase: true}
	svc := NewMemoryDomainService(
		map[string]DomainProvider{"ironflyer": NewStaticDomainProvider("ironflyer", "ironflyer.test", "edge.ironflyer.test")},
		map[string]Registrar{"test": registrar},
		"ironflyer",
		"test",
		WithDomainPurchasePolicy(DomainPurchasePolicy{
			Enabled:           true,
			MaxPriceUSD:       decimal.NewFromInt(50),
			PriceTolerancePct: decimal.NewFromInt(10),
		}),
	)
	_, err := svc.PurchaseDomain(context.Background(), PurchaseDomainInput{
		TenantID: "tenant-1", ProjectID: "project-1", Domain: "example.com",
		Registrar: "test", Years: 1, ExpectedPriceUSD: decimal.NewFromInt(20),
	})
	if err == nil {
		t.Fatal("expected price drift to fail")
	}
	if registrar.purchases != 0 {
		t.Fatalf("registrar purchase should not be called after price drift, got %d", registrar.purchases)
	}
}

type fakeRegistrar struct {
	price       decimal.Decimal
	canPurchase bool
	purchases   int
}

func (r *fakeRegistrar) Name() string { return "test" }

func (r *fakeRegistrar) Availability(_ context.Context, domain string) (DomainAvailability, error) {
	return DomainAvailability{
		Domain:      normalizeHostname(domain),
		Available:   r.canPurchase,
		Registrar:   r.Name(),
		PriceUSD:    r.price,
		Currency:    "USD",
		CanPurchase: r.canPurchase,
		Reason:      "test quote",
		CheckedAt:   testTime(),
	}, nil
}

func (r *fakeRegistrar) Purchase(_ context.Context, in PurchaseDomainInput) (DomainAvailability, error) {
	r.purchases++
	return DomainAvailability{
		Domain:      normalizeHostname(in.Domain),
		Available:   false,
		Registrar:   r.Name(),
		PriceUSD:    r.price,
		Currency:    "USD",
		CanPurchase: true,
		Reason:      "registered",
		CheckedAt:   testTime(),
	}, nil
}

func testTime() time.Time {
	return time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
}
