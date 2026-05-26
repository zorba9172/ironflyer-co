// Package profitguardctx propagates the active execution id (and the
// tenant that owns it) through context. Provider routers, runtime
// clients, and the finisher loop look up the id to decide whether
// ProfitGuard should run for this call path, and whether per-token
// cost attribution should land in the ledger / on the execution row.
//
// Calls that do not carry an execution id are unmetered internal
// calls — back-compat for V21 → V22 transition; ProfitGuard and
// cost attribution are skipped.
package profitguardctx

import "context"

type ctxKey struct{ name string }

var (
	execKey   = ctxKey{"execID"}
	tenantKey = ctxKey{"tenantID"}
)

// WithExecution attaches executionID + tenantID to ctx so downstream
// hooks can find the active execution and route per-token cost to
// the correct tenant ledger. Empty executionIDs are ignored (returns
// ctx unchanged) so callers that conditionally have an id don't have
// to branch. tenantID is allowed to be empty — the cost-attribution
// path treats that as "skip ledger write but still attribute on the
// execution row".
func WithExecution(ctx context.Context, executionID, tenantID string) context.Context {
	if executionID == "" {
		return ctx
	}
	ctx = context.WithValue(ctx, execKey, executionID)
	if tenantID != "" {
		ctx = context.WithValue(ctx, tenantKey, tenantID)
	}
	return ctx
}

// ExecutionID returns the execution id from ctx, or ("", false) if
// none was attached. Callers that should skip ProfitGuard when there
// is no execution use the boolean to short-circuit.
func ExecutionID(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	v, ok := ctx.Value(execKey).(string)
	if !ok || v == "" {
		return "", false
	}
	return v, true
}

// TenantID returns the tenant id from ctx, or ("", false) if none
// was attached. The per-token cost attribution path uses this to
// avoid writing ledger entries with a zero tenant.
func TenantID(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	v, ok := ctx.Value(tenantKey).(string)
	if !ok || v == "" {
		return "", false
	}
	return v, true
}
