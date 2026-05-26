package learning

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v5"
)

// Global publisher — set once at boot via SetGlobal so existing call
// sites can emit OutcomeEvents without threading a *Publisher through
// every constructor. This mirrors the outboxhooks.SetRegistry pattern.
// Nil-safe: Publish/PublishInTx are no-ops when the global is unset.
var (
	globalMu sync.RWMutex
	global   *Publisher
)

// SetGlobal installs the process-wide publisher. Safe to call from any
// goroutine; replacement is atomic per call. Pass nil to disable.
func SetGlobal(p *Publisher) {
	globalMu.Lock()
	global = p
	globalMu.Unlock()
}

// GetGlobal returns the installed publisher or nil.
func GetGlobal() *Publisher {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return global
}

// Publish is a convenience wrapper around the global publisher. Call
// sites that don't want to thread *Publisher through their
// constructors use this directly; the call is a no-op until SetGlobal
// is invoked at boot.
func Publish(ctx context.Context, evt OutcomeEvent) {
	if p := GetGlobal(); p != nil {
		p.Publish(ctx, evt)
	}
}

// PublishInTx writes an outcome inside an existing pgx.Tx. Preferred
// inside Postgres-backed services so the outcome commits atomically
// with the business row.
func PublishInTx(ctx context.Context, tx pgx.Tx, evt OutcomeEvent) error {
	if p := GetGlobal(); p != nil {
		return p.PublishInTx(ctx, tx, evt)
	}
	return nil
}
