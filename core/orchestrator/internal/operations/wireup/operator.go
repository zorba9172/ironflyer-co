// Operator wireup — V22 Wave-3 (Agent 36).
//
// Stitches abuse / audit / deploy / execution / wallet into the
// operator.OperatorService. Every dep may be nil — operator methods
// re-check and return ErrNotConfigured per the package contract.
package wireup

import (
	"ironflyer/core/orchestrator/internal/operations/abuse"
	"ironflyer/core/orchestrator/internal/operations/audit"
	"ironflyer/core/orchestrator/internal/operations/deploy"
	"ironflyer/core/orchestrator/internal/business/execution"
	"ironflyer/core/orchestrator/internal/operations/operator"
	"ironflyer/core/orchestrator/internal/business/wallet"
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
