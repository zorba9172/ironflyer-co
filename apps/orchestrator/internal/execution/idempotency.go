package execution

import (
	"context"
	"errors"
)

// V22 Wave 3 / Item 5 — execution-FSM idempotency for Temporal retries
// (WORKFLOWS.md "Idempotency open gaps").
//
// The execution FSM in fsm.go already prevents a transition from
// happening twice — once you're past `running`, CanTransition refuses
// to fire `running` again, and the existing Admit/Start/Succeed/Fail/
// Stop/Kill return ErrIllegalTransition on the second call.
//
// That is correct but inconvenient for Temporal: an at-least-once
// activity retry sees ErrIllegalTransition and has no way to tell it
// apart from a real bug. The IdempotentService surface below replaces
// "second-call → error" with "second-call → no-op nil", letting the
// workflow re-drive the same activity body without special-casing the
// retry boundary.
//
// Semantics:
//
//   - If the current status equals the target terminal status, return
//     nil (the activity already landed).
//   - If the current status is past the target on the FSM happy path
//     (e.g. Start → row is already Succeeded), return nil — we cannot
//     "un-succeed" to satisfy a retried Start, and the workflow already
//     observed the later state through a different activity.
//   - Otherwise, dispatch to the underlying non-idempotent method.
//
// Op-key columns added by migrations/00037 give us a second axis of
// safety: storing the op_key alongside the status transition means a
// retried activity that races AGAINST the first attempt (rather than
// arriving after it) collides on the unique index instead of bypassing
// the FSM.

// IdempotentService is the opt-in surface Temporal activities use. The
// existing Service interface is unchanged so all the existing callers
// (resolvers, finisher engine, settler) keep working without
// modification.
type IdempotentService interface {
	Service

	// AdmitIdempotent is Admit that returns nil when the row is
	// already at or past StatusAdmitted on the FSM happy path. opKey
	// is reserved for future per-call dedupe via admit_op_key — today
	// the FSM read is the source of truth and opKey is recorded only
	// in the audit/event payload.
	AdmitIdempotent(ctx context.Context, id, opKey string) error
	// StartIdempotent is Start that no-ops when the row is at or past
	// StatusRunning.
	StartIdempotent(ctx context.Context, id, opKey string) error
	// SucceedIdempotent is Succeed that no-ops when the row is at or
	// past StatusSucceeded.
	SucceedIdempotent(ctx context.Context, id, opKey string) error
	// FailIdempotent is Fail that no-ops when the row is already in a
	// terminal non-succeeded status — a Fail retry against an
	// already-Stopped/Killed row is treated as "the workflow already
	// observed the closure via a different path".
	FailIdempotent(ctx context.Context, id, reason, opKey string) error
	// StopIdempotent is Stop that no-ops when the row is already at
	// StatusStopped or any other terminal.
	StopIdempotent(ctx context.Context, id, reason, opKey string) error
	// KillIdempotent is Kill that no-ops when the row is already at
	// StatusKilled or any other terminal.
	KillIdempotent(ctx context.Context, id, reason, opKey string) error
}

// statusOrder is the FSM happy-path ordinal used by isAtOrPast. Higher
// number = later in the lifecycle. Terminal statuses share the highest
// rung so any of (succeeded/failed/stopped/killed) is "at or past" any
// of the others — the workflow can't undo a terminal anyway.
func statusOrder(s Status) int {
	switch s {
	case StatusCreated:
		return 0
	case StatusAdmitted:
		return 1
	case StatusRunning:
		return 2
	case StatusPausedForBudget:
		return 2
	case StatusSucceeded, StatusFailed, StatusStopped, StatusKilled:
		return 3
	case StatusRefunded:
		return 4
	}
	return -1
}

// isAtOrPast returns true when `current` is the same as or strictly
// later than `target` on the happy path.
func isAtOrPast(current, target Status) bool {
	return statusOrder(current) >= statusOrder(target)
}

// idempotent is the shared wrapper: read current, short-circuit if
// already at-or-past target, otherwise dispatch. ErrIllegalTransition
// errors from the underlying call are swallowed to nil only when a
// concurrent caller landed the same transition first — verified by
// re-reading the row.
func idempotent(ctx context.Context, svc Service, id string, target Status, doIt func() error) error {
	row, err := svc.Get(ctx, id)
	if err != nil {
		return err
	}
	if isAtOrPast(row.Status, target) {
		return nil
	}
	if err := doIt(); err != nil {
		if errors.Is(err, ErrIllegalTransition) {
			// Re-read; a concurrent retry may have landed the move
			// between our Get and our doIt call.
			if r2, e2 := svc.Get(ctx, id); e2 == nil && isAtOrPast(r2.Status, target) {
				return nil
			}
		}
		return err
	}
	return nil
}

// MemoryIdempotent wraps Memory with the IdempotentService surface.
// Mirror struct for *Postgres is below.
type MemoryIdempotent struct {
	*Memory
}

// IdempotentMemory returns the IdempotentService façade over a Memory
// service. Pointer-to-pointer indirection lets the wrapper extend the
// behaviour without modifying the base struct.
func IdempotentMemory(m *Memory) *MemoryIdempotent { return &MemoryIdempotent{Memory: m} }

func (m *MemoryIdempotent) AdmitIdempotent(ctx context.Context, id, _ string) error {
	return idempotent(ctx, m.Memory, id, StatusAdmitted, func() error { return m.Memory.Admit(ctx, id) })
}
func (m *MemoryIdempotent) StartIdempotent(ctx context.Context, id, _ string) error {
	return idempotent(ctx, m.Memory, id, StatusRunning, func() error { return m.Memory.Start(ctx, id) })
}
func (m *MemoryIdempotent) SucceedIdempotent(ctx context.Context, id, _ string) error {
	return idempotent(ctx, m.Memory, id, StatusSucceeded, func() error { return m.Memory.Succeed(ctx, id) })
}
func (m *MemoryIdempotent) FailIdempotent(ctx context.Context, id, reason, _ string) error {
	return idempotent(ctx, m.Memory, id, StatusFailed, func() error { return m.Memory.Fail(ctx, id, reason) })
}
func (m *MemoryIdempotent) StopIdempotent(ctx context.Context, id, reason, _ string) error {
	return idempotent(ctx, m.Memory, id, StatusStopped, func() error { return m.Memory.Stop(ctx, id, reason) })
}
func (m *MemoryIdempotent) KillIdempotent(ctx context.Context, id, reason, _ string) error {
	return idempotent(ctx, m.Memory, id, StatusKilled, func() error { return m.Memory.Kill(ctx, id, reason) })
}

// PostgresIdempotent wraps Postgres with the IdempotentService surface.
type PostgresIdempotent struct {
	*Postgres
}

// IdempotentPostgres returns the IdempotentService façade over a
// Postgres service. Same pointer-to-pointer story as MemoryIdempotent.
func IdempotentPostgres(p *Postgres) *PostgresIdempotent { return &PostgresIdempotent{Postgres: p} }

func (p *PostgresIdempotent) AdmitIdempotent(ctx context.Context, id, opKey string) error {
	return idempotent(ctx, p.Postgres, id, StatusAdmitted, func() error {
		if err := p.Postgres.Admit(ctx, id); err != nil {
			return err
		}
		return p.stampOpKey(ctx, id, "admit_op_key", opKey)
	})
}
func (p *PostgresIdempotent) StartIdempotent(ctx context.Context, id, opKey string) error {
	return idempotent(ctx, p.Postgres, id, StatusRunning, func() error {
		if err := p.Postgres.Start(ctx, id); err != nil {
			return err
		}
		return p.stampOpKey(ctx, id, "start_op_key", opKey)
	})
}
func (p *PostgresIdempotent) SucceedIdempotent(ctx context.Context, id, opKey string) error {
	return idempotent(ctx, p.Postgres, id, StatusSucceeded, func() error {
		if err := p.Postgres.Succeed(ctx, id); err != nil {
			return err
		}
		return p.stampOpKey(ctx, id, "settle_op_key", opKey)
	})
}
func (p *PostgresIdempotent) FailIdempotent(ctx context.Context, id, reason, opKey string) error {
	return idempotent(ctx, p.Postgres, id, StatusFailed, func() error {
		if err := p.Postgres.Fail(ctx, id, reason); err != nil {
			return err
		}
		return p.stampOpKey(ctx, id, "settle_op_key", opKey)
	})
}
func (p *PostgresIdempotent) StopIdempotent(ctx context.Context, id, reason, opKey string) error {
	return idempotent(ctx, p.Postgres, id, StatusStopped, func() error {
		if err := p.Postgres.Stop(ctx, id, reason); err != nil {
			return err
		}
		return p.stampOpKey(ctx, id, "settle_op_key", opKey)
	})
}
func (p *PostgresIdempotent) KillIdempotent(ctx context.Context, id, reason, opKey string) error {
	return idempotent(ctx, p.Postgres, id, StatusKilled, func() error {
		if err := p.Postgres.Kill(ctx, id, reason); err != nil {
			return err
		}
		return p.stampOpKey(ctx, id, "settle_op_key", opKey)
	})
}

// stampOpKey writes the supplied op_key into one of the per-transition
// columns added by migrations/00037 (admit_op_key, start_op_key,
// settle_op_key). The column name MUST come from a closed set defined
// by this package — never user input — so building it into the SQL
// string is safe.
//
// Empty opKey is a no-op. The UPDATE refuses to overwrite a non-empty
// existing value (COALESCE keeps the prior key) so a retry that
// supplies a different key never silently rewrites history.
func (p *PostgresIdempotent) stampOpKey(ctx context.Context, id, column, opKey string) error {
	if opKey == "" {
		return nil
	}
	switch column {
	case "admit_op_key", "start_op_key", "settle_op_key":
	default:
		return errors.New("execution: unknown op_key column " + column)
	}
	_, err := p.Postgres.pool.Exec(ctx,
		`UPDATE executions SET `+column+` = COALESCE(`+column+`, $2) WHERE id = $1`,
		id, opKey)
	return err
}

// Compile-time guarantees that both wrappers satisfy IdempotentService.
var (
	_ IdempotentService = (*MemoryIdempotent)(nil)
	_ IdempotentService = (*PostgresIdempotent)(nil)
)
