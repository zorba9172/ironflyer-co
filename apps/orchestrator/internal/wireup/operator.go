// Operator wireup — V22 Wave-3 (Agent 36).
//
// Stitches abuse / audit / deploy / execution / wallet into the
// operator.OperatorService. Every dep may be nil — operator methods
// re-check and return ErrNotConfigured per the package contract.
package wireup

import (
	"ironflyer/apps/orchestrator/internal/abuse"
	"ironflyer/apps/orchestrator/internal/audit"
	"ironflyer/apps/orchestrator/internal/deploy"
	"ironflyer/apps/orchestrator/internal/execution"
	"ironflyer/apps/orchestrator/internal/operator"
	"ironflyer/apps/orchestrator/internal/wallet"
)

// BuildOperator constructs the OperatorService from its 5 source
// dependencies. SandboxCapacity defaults to zero — resolvers
// will then report 0% utilization (operator dashboards understand it
// as "capacity not declared").
func BuildOperator(
	deploySvc deploy.Service,
	abuseEngine abuse.Engine,
	execSvc execution.Service,
	walletSvc wallet.Service,
	auditStore audit.Store,
	sandboxCapacity int,
) operator.OperatorService {
	return operator.New(operator.Deps{
		Deploy:          deploySvc,
		Abuse:           abuseEngine,
		Execution:       execSvc,
		Wallet:          walletSvc,
		Audit:           auditStore,
		SandboxCapacity: sandboxCapacity,
	})
}
