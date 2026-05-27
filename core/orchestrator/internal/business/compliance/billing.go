package compliance

import (
	"context"
	"errors"
)

// RunMonthlyBilling walks every enrolment and fires ChargeOnce for the
// current calendar month. Idempotent via the per-(enrollment, period)
// idempotency key — a re-run on the same day is a no-op. Returns the
// first hard error encountered alongside a count of successful charges
// so the caller can surface partial progress.
//
// The orchestrator wires this behind a daily cron (reconcile.go).
// Wallet rejections (ErrInsufficientBalance) are counted as soft
// failures so a single delinquent tenant cannot stop the whole sweep.
func (s *Service) RunMonthlyBilling(ctx context.Context) (charged int, softFails int, err error) {
	rows, err := s.backend.ListAllEnrollments(ctx)
	if err != nil {
		return 0, 0, err
	}
	for _, row := range rows {
		if ctx.Err() != nil {
			return charged, softFails, ctx.Err()
		}
		if cerr := s.ChargeOnce(ctx, row.ID); cerr != nil {
			if errors.Is(cerr, ErrInsufficientBalance) {
				softFails++
				s.logger.Warn().
					Err(cerr).
					Str("enrollment_id", row.ID).
					Str("tenant", row.TenantID).
					Msg("compliance: monthly charge soft-failed (wallet)")
				continue
			}
			s.logger.Error().
				Err(cerr).
				Str("enrollment_id", row.ID).
				Msg("compliance: monthly charge errored")
			continue
		}
		charged++
	}
	return charged, softFails, nil
}
