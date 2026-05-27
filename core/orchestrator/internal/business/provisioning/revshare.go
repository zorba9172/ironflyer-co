package provisioning

import "github.com/shopspring/decimal"

// ApplyPolicy computes the Ironflyer cut for a single gross-amount
// event under the given RevenuePolicy. The result is always >= 0 and
// always >= MinFeeUSD on positive gross amounts — the floor exists
// because the issuing party (us) pays a fixed cost per call against
// the underlying rail (Stripe processing fee, domain renewal,
// Resend API request), so a 0.005% cut on a $1 charge would put
// the rail underwater on its own infra.
//
// Returns (cut, ok). ok is false when gross is non-positive; the
// caller should treat that as ErrInvalidAmount rather than recording
// a phantom zero-revenue event.
func ApplyPolicy(gross decimal.Decimal, policy RevenuePolicy) (decimal.Decimal, bool) {
	if !gross.IsPositive() {
		return decimal.Zero, false
	}
	cut := gross.Mul(policy.SharePct)
	if policy.MinFeeUSD.IsPositive() && cut.LessThan(policy.MinFeeUSD) {
		cut = policy.MinFeeUSD
	}
	// Never charge more than the gross — a misconfigured policy with
	// SharePct > 1.0 plus a high floor would otherwise hand the
	// merchant a negative settlement. Defensive cap, never expected
	// to fire under sane policies.
	if cut.GreaterThan(gross) {
		cut = gross
	}
	return cut, true
}

// DefaultPolicies returns the seed RevenuePolicy set the in-memory
// PolicyStore boots with. Production operators override these via the
// `revenue_policies` Postgres table — these defaults are intentionally
// conservative (1.5% Stripe, $1 floor) so a forgotten migration in a
// staging env still produces sensible cut numbers.
func DefaultPolicies() map[string]RevenuePolicy {
	return map[string]RevenuePolicy{
		KindStripeConnect: {
			Kind:           KindStripeConnect,
			SharePct:       decimal.NewFromFloat(0.015),
			MinFeeUSD:      decimal.NewFromFloat(0.05),
			BillingCadence: CadencePerTransaction,
		},
		KindCloudflareDomain: {
			Kind:           KindCloudflareDomain,
			SharePct:       decimal.NewFromFloat(0.20),
			MinFeeUSD:      decimal.NewFromFloat(1.00),
			BillingCadence: CadenceMonthlyAggregate,
		},
		KindResendEmail: {
			Kind:           KindResendEmail,
			SharePct:       decimal.NewFromFloat(0.10),
			MinFeeUSD:      decimal.NewFromFloat(0.50),
			BillingCadence: CadenceMonthlyAggregate,
		},
		KindHosting: {
			Kind:           KindHosting,
			SharePct:       decimal.NewFromFloat(0.15),
			MinFeeUSD:      decimal.NewFromFloat(1.00),
			BillingCadence: CadenceMonthlyAggregate,
		},
	}
}

// MemoryPolicyStore is the default PolicyStore — backed by an
// in-process map seeded from DefaultPolicies. Sufficient for dev and
// for staging environments that haven't run the policy migration yet.
type MemoryPolicyStore struct {
	policies map[string]RevenuePolicy
}

// NewMemoryPolicyStore returns a store seeded with DefaultPolicies. To
// override, callers may mutate the returned policies map under their
// own synchronisation — the store is read-only after construction so
// resolvers can read without a lock.
func NewMemoryPolicyStore() *MemoryPolicyStore {
	return &MemoryPolicyStore{policies: DefaultPolicies()}
}

// Get implements PolicyStore.
func (s *MemoryPolicyStore) Get(kind string) (RevenuePolicy, error) {
	if s == nil {
		return RevenuePolicy{}, ErrPolicyMissing
	}
	p, ok := s.policies[kind]
	if !ok {
		return RevenuePolicy{}, ErrPolicyMissing
	}
	return p, nil
}
