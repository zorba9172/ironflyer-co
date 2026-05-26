package deploy

import (
	"context"
	"fmt"

	"github.com/shopspring/decimal"
)

type DomainServiceOption func(*MemoryDomainService)

func WithDomainPurchasePolicy(policy DomainPurchasePolicy) DomainServiceOption {
	return func(s *MemoryDomainService) {
		s.purchasePolicy = normalizeDomainPurchasePolicy(policy)
	}
}

func normalizeDomainPurchasePolicy(policy DomainPurchasePolicy) DomainPurchasePolicy {
	if policy.MaxPriceUSD.LessThanOrEqual(decimal.Zero) {
		policy.MaxPriceUSD = decimal.NewFromInt(75)
	}
	if policy.PriceTolerancePct.LessThan(decimal.Zero) {
		policy.PriceTolerancePct = decimal.Zero
	}
	return policy
}

func (s *MemoryDomainService) validateDomainPurchase(ctx context.Context, registrar Registrar, in PurchaseDomainInput) (DomainAvailability, error) {
	policy := normalizeDomainPurchasePolicy(s.purchasePolicy)
	domain := normalizeHostname(in.Domain)
	if err := validateHostname(domain); err != nil {
		return DomainAvailability{}, err
	}
	if !policy.Enabled {
		return DomainAvailability{}, fmt.Errorf("%w: domain purchase is disabled by operator policy", ErrInvalidState)
	}
	if in.TenantID == "" || in.ProjectID == "" {
		return DomainAvailability{}, fmt.Errorf("%w: tenant_id and project_id required", ErrInvalidState)
	}
	if in.Years <= 0 || in.Years > 10 {
		return DomainAvailability{}, fmt.Errorf("%w: domain purchase years must be between 1 and 10", ErrInvalidState)
	}
	if in.ExpectedPriceUSD.LessThanOrEqual(decimal.Zero) {
		return DomainAvailability{}, fmt.Errorf("%w: expected domain price is required", ErrInvalidState)
	}
	if policy.RequireRegistrantContact && len(in.Contact) == 0 {
		return DomainAvailability{}, fmt.Errorf("%w: registrant contact is required", ErrInvalidState)
	}

	avail, err := registrar.Availability(ctx, domain)
	if err != nil {
		return DomainAvailability{}, err
	}
	if !avail.CanPurchase {
		return DomainAvailability{}, fmt.Errorf("%w: domain cannot be purchased: %s", ErrInvalidState, firstNonEmpty(avail.Reason, "registrar declined purchase"))
	}
	if avail.PriceUSD.LessThanOrEqual(decimal.Zero) {
		return DomainAvailability{}, fmt.Errorf("%w: registrar returned no positive price", ErrInvalidState)
	}
	if avail.PriceUSD.GreaterThan(policy.MaxPriceUSD) {
		return DomainAvailability{}, fmt.Errorf("%w: domain price %s exceeds max %s USD", ErrInvalidState, avail.PriceUSD.StringFixedBank(2), policy.MaxPriceUSD.StringFixedBank(2))
	}

	tolerance := decimal.NewFromInt(1).Add(policy.PriceTolerancePct.Div(decimal.NewFromInt(100)))
	priceCeiling := in.ExpectedPriceUSD.Mul(tolerance)
	if avail.PriceUSD.GreaterThan(priceCeiling) {
		return DomainAvailability{}, fmt.Errorf("%w: domain price changed from expected %s to %s USD", ErrInvalidState, in.ExpectedPriceUSD.StringFixedBank(2), avail.PriceUSD.StringFixedBank(2))
	}
	return avail, nil
}

func domainPurchaseMetadata(in map[string]any, registrar Registrar, avail DomainAvailability) map[string]any {
	return mergeDomainMetadata(in, map[string]any{
		"domain_purchase":          "submitted",
		"registrar":                registrar.Name(),
		"purchase_price_usd":       avail.PriceUSD.StringFixedBank(2),
		"purchase_currency":        firstNonEmpty(avail.Currency, "USD"),
		"availability_checked_at":  avail.CheckedAt.Format(timeRFC3339NanoUTC),
		"registrar_purchase_quote": firstNonEmpty(avail.Reason, "available"),
	})
}

const timeRFC3339NanoUTC = "2006-01-02T15:04:05.999999999Z07:00"
