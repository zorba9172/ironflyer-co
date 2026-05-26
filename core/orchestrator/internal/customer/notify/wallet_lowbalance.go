package notify

import (
	"context"
	"strings"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/business/wallet"
)

// DefaultLowBalanceThresholdCents is the per-user wallet alert threshold
// when IRONFLYER_WALLET_LOW_BALANCE_THRESHOLD_CENTS is unset. $5.00
// matches the V22 wallet floor used as the operator-friendly "top-up
// reminder" line — high enough that the user sees the warning before
// any paid execution refuses with 402, low enough that the alert is
// not noisy on healthy accounts.
const DefaultLowBalanceThresholdCents = 500

// userEmailLookup is the narrow surface the watcher uses to recover the
// recipient address for a tenant. The orchestrator's auth.Service +
// notify.PrefsStore both satisfy it via small adapters in main.go.
type userEmailLookup func(ctx context.Context, tenant string) (string, string)

// LowBalanceWalletService is a wallet.Service decorator that dispatches
// KindLowBalance when a Debit crosses the configured threshold from
// above. Wrapping (instead of mutating the wallet package) keeps the
// wallet contract pristine — wallet.Service stays a pure money primitive
// and the notification side-effect lives in the customer layer where it
// belongs.
type LowBalanceWalletService struct {
	inner        wallet.Service
	dispatcher   *Dispatcher
	lookup       userEmailLookup
	thresholdUSD decimal.Decimal
	currency     string
	logger       zerolog.Logger
}

// NewLowBalanceWalletService wraps inner. thresholdCents is converted
// once to a decimal so the per-Debit hot path stays allocation-free.
// currency is the ISO label stamped on the dispatched payload; the
// V22 wallet is USD-only so this defaults to "USD" when empty.
func NewLowBalanceWalletService(inner wallet.Service, dispatcher *Dispatcher, lookup userEmailLookup, thresholdCents int, currency string, logger zerolog.Logger) *LowBalanceWalletService {
	if thresholdCents <= 0 {
		thresholdCents = DefaultLowBalanceThresholdCents
	}
	cur := strings.ToUpper(strings.TrimSpace(currency))
	if cur == "" {
		cur = "USD"
	}
	return &LowBalanceWalletService{
		inner:        inner,
		dispatcher:   dispatcher,
		lookup:       lookup,
		thresholdUSD: decimal.NewFromInt(int64(thresholdCents)).Div(decimal.NewFromInt(100)),
		currency:     cur,
		logger:       logger,
	}
}

// thresholdCents materializes the watcher's threshold as cents for the
// dispatched payload — the wallet stores decimal USD, the payload
// carries integer cents to match the receipt template's contract.
func (s *LowBalanceWalletService) thresholdCents() int {
	return int(s.thresholdUSD.Mul(decimal.NewFromInt(100)).IntPart())
}

func (s *LowBalanceWalletService) Get(ctx context.Context, tenant string) (wallet.Wallet, error) {
	return s.inner.Get(ctx, tenant)
}

func (s *LowBalanceWalletService) TopUp(ctx context.Context, tenant string, amount decimal.Decimal, stripeSessionID string) error {
	return s.inner.TopUp(ctx, tenant, amount, stripeSessionID)
}

func (s *LowBalanceWalletService) Hold(ctx context.Context, tenant string, amount decimal.Decimal) error {
	return s.inner.Hold(ctx, tenant, amount)
}

func (s *LowBalanceWalletService) Release(ctx context.Context, tenant string, amount decimal.Decimal) error {
	return s.inner.Release(ctx, tenant, amount)
}

// Debit instruments the underlying Debit so a balance that crosses the
// low-balance line emits exactly one alert. We sample the wallet before
// the debit so we can detect the crossing edge (pre >= threshold,
// post < threshold). Same-day idempotency on the payload makes
// oscillating balances safe.
func (s *LowBalanceWalletService) Debit(ctx context.Context, tenant string, amount decimal.Decimal) error {
	pre, _ := s.inner.Get(ctx, tenant)
	if err := s.inner.Debit(ctx, tenant, amount); err != nil {
		return err
	}
	s.maybeAlert(ctx, tenant, pre.BalanceUSD)
	return nil
}

// LifetimeStats / ListTopUps / CreatePendingTopUp pass through verbatim.
func (s *LowBalanceWalletService) LifetimeStats(ctx context.Context, tenant string) (wallet.LifetimeStats, error) {
	return s.inner.LifetimeStats(ctx, tenant)
}

func (s *LowBalanceWalletService) ListTopUps(ctx context.Context, tenant string, limit int) ([]wallet.TopUp, error) {
	return s.inner.ListTopUps(ctx, tenant, limit)
}

func (s *LowBalanceWalletService) CreatePendingTopUp(ctx context.Context, tenant string, amount decimal.Decimal, stripeSessionID string) (wallet.TopUp, error) {
	return s.inner.CreatePendingTopUp(ctx, tenant, amount, stripeSessionID)
}

// maybeAlert dispatches when the post-debit balance has just crossed
// below the threshold. Dispatched payload's same-day idempotency key
// (yyyymmdd) means a user that drains, tops up, and drains again on
// the same day still only sees one alert.
func (s *LowBalanceWalletService) maybeAlert(ctx context.Context, tenant string, preBalance decimal.Decimal) {
	if s == nil || s.dispatcher == nil {
		return
	}
	post, err := s.inner.Get(ctx, tenant)
	if err != nil {
		return
	}
	if post.BalanceUSD.GreaterThanOrEqual(s.thresholdUSD) {
		return
	}
	if preBalance.LessThan(s.thresholdUSD) {
		return
	}
	userID, email := "", ""
	if s.lookup != nil {
		userID, email = s.lookup(ctx, tenant)
	}
	if userID == "" {
		userID = tenant
	}
	balanceCents := int(post.BalanceUSD.Mul(decimal.NewFromInt(100)).IntPart())
	if balanceCents < 0 {
		balanceCents = 0
	}
	if err := s.dispatcher.Dispatch(ctx, userID, email, KindLowBalance, LowBalancePayload{
		Currency:       s.currency,
		BalanceCents:   balanceCents,
		ThresholdCents: s.thresholdCents(),
	}); err != nil {
		s.logger.Warn().Err(err).Str("tenant", tenant).Msg("notify: low-balance dispatch failed")
	}
}
