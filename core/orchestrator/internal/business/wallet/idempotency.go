package wallet

import (
	"context"

	"github.com/shopspring/decimal"
)

// V22 Wave 3 / Item 5 — wallet operation idempotency for Temporal
// retries (WORKFLOWS.md "Idempotency open gaps").
//
// Temporal gives at-least-once activity execution. Every wallet
// mutation a workflow drives MUST be safe to retry without
// double-charging or double-releasing. The pre-V22 method signatures
// (Hold / Release / Debit / TopUp on Service) had no operation key, so
// a retried activity could land a second hold while the first was
// still being committed.
//
// The IdempotentService interface below is the opt-in surface
// activities use. Implementations:
//
//   - Look up the op_key in wallet_operations (migrations/00037).
//   - If the prior outcome was "succeeded", return nil — the activity
//     already landed, retry is a no-op.
//   - If the prior outcome was "failed", return the recorded error so
//     the workflow surfaces the same failure mode on every retry
//     instead of "first failed, retry succeeded" inversions.
//   - Otherwise, run the mutation AND insert the wallet_operations row
//     in the same transaction; a PK collision on op_key proves a
//     concurrent retry won and the loser reads back the winner's row.
//
// The Service interface is intentionally unchanged — existing callers
// (the resolver layer, the Stripe webhook, the legacy finisher) keep
// their non-idempotent contract and a single helper here threads them
// to the new methods via a generated key.
type IdempotentService interface {
	Service

	// HoldWithKey is Hold gated by op_key. Returns nil on first
	// success AND on subsequent retries with the same key.
	HoldWithKey(ctx context.Context, tenant string, amount decimal.Decimal, opKey string) error
	// ReleaseWithKey is Release gated by op_key.
	ReleaseWithKey(ctx context.Context, tenant string, amount decimal.Decimal, opKey string) error
	// DebitWithKey is Debit gated by op_key.
	DebitWithKey(ctx context.Context, tenant string, amount decimal.Decimal, opKey string) error
	// TopUpWithKey is TopUp gated by op_key. The Stripe session id is
	// already a natural dedupe key for webhook calls — opKey is the
	// orchestrator's own dedupe handle on top of it (so a workflow
	// that retries the TopUp activity itself doesn't double-credit).
	TopUpWithKey(ctx context.Context, tenant string, amount decimal.Decimal, stripeSessionID, opKey string) error
}

// OpType is the closed enum stored in wallet_operations.op_type. Wire
// values match the CHECK constraint in 00037_idempotency_keys.sql.
type OpType string

const (
	OpHold    OpType = "hold"
	OpRelease OpType = "release"
	OpDebit   OpType = "debit"
	OpTopUp   OpType = "topup"
	OpRefund  OpType = "refund"
)
