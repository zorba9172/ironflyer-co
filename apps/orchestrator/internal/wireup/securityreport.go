// Security-report wireup — V22 Wave-3 (Agent 35), updated by Agent 39.
//
// Adapts the orchestrator's execution.Service into all three
// securityreport sources:
//
//   - ExecutionSource: tenant + status (live since wave 3).
//   - FindingSource:   security-finding events emitted by the
//                      finisher Security gate hook (Agent 39).
//   - PolicySource:    tenant policy — DefaultPolicy() for V1
//                      until per-tenant overrides land (Agent 39).
//
// The Builder remains nil-tolerant: callers that lose any single
// source still receive a clean degraded report rather than a 5xx.
package wireup

import (
	"context"

	"github.com/rs/zerolog"

	"ironflyer/apps/orchestrator/internal/execution"
	"ironflyer/apps/orchestrator/internal/securityreport"
)

// BuildSecurityReportBuilder constructs a StandardBuilder against the
// real execution service. Returns nil when execSvc is missing so the
// resolver can degrade to gqlNotConfigured.
func BuildSecurityReportBuilder(execSvc execution.Service, log zerolog.Logger) securityreport.Builder {
	if execSvc == nil {
		log.Warn().Msg("securityreport: execution service unwired; builder disabled")
		return nil
	}
	return securityreport.NewStandardBuilder(
		&secrExecutionAdapter{exec: execSvc},
		&secrFindingAdapter{exec: execSvc}, // real findings via execution_events
		&secrPolicyAdapter{},               // DefaultPolicy() until tenant store lands
	)
}

// secrExecutionAdapter projects execution.Get into the meta the
// security report needs.
type secrExecutionAdapter struct {
	exec execution.Service
}

func (a *secrExecutionAdapter) GetExecution(ctx context.Context, id string) (securityreport.ExecutionMeta, error) {
	e, err := a.exec.Get(ctx, id)
	if err != nil {
		return securityreport.ExecutionMeta{}, err
	}
	return securityreport.ExecutionMeta{
		ID:         e.ID,
		TenantID:   e.TenantID,
		Status:     string(e.Status),
		GateStatus: "", // No canonical V22 source for gate aggregate status yet.
	}, nil
}
