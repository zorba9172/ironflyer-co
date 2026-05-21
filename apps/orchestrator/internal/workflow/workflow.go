// Package workflow defines the Temporal Workflow + Activities for the
// Ironflyer finisher loop. The workflow orchestrates gate checks in parallel
// where safe, dispatches repair agents on failure, and is durable across
// crashes/redeploys.
//
// Run a Temporal dev server with: `temporal server start-dev`.
// Then start the orchestrator with IRONFLYER_EXECUTOR=temporal.
package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"ironflyer/apps/orchestrator/internal/domain"
)

const TaskQueueDefault = "ironflyer-finisher"

const (
	WorkflowFinisher = "FinisherWorkflow"
	ActivityCheckGate = "CheckGate"
	ActivityRunAgent  = "RunAgent"
	ActivityRecordEvent = "RecordEvent"
)

type FinisherInput struct {
	ProjectID     string
	MaxIterations int
}

type FinisherOutput struct {
	ProjectID  string             `json:"projectId"`
	Iterations int                `json:"iterations"`
	Gates      []domain.GateState `json:"gates"`
	Completed  bool               `json:"completed"`
}

// FinisherWorkflow is the durable orchestration of the gate-driven finisher
// loop. Gates that don't depend on each other run in parallel via workflow.Go.
func FinisherWorkflow(ctx workflow.Context, in FinisherInput) (FinisherOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("FinisherWorkflow starting", "projectId", in.ProjectID)

	if in.MaxIterations == 0 {
		in.MaxIterations = 4
	}

	out := FinisherOutput{ProjectID: in.ProjectID}

	activityOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		HeartbeatTimeout:    30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    1 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOpts)

	gateOrder := []domain.GateName{
		domain.GateSpec, domain.GateUX, domain.GateArch,
		domain.GateCode, domain.GateTest, domain.GateSecurity, domain.GateDeploy,
	}

	for iter := 0; iter < in.MaxIterations; iter++ {
		out.Iterations = iter + 1
		logger.Info("iteration", "n", out.Iterations)

		// Phase 1: run all gate checks in parallel. They are read-only.
		gateResults := make(map[domain.GateName]CheckGateResult, len(gateOrder))
		futures := make(map[domain.GateName]workflow.Future, len(gateOrder))
		for _, g := range gateOrder {
			gate := g
			futures[gate] = workflow.ExecuteActivity(ctx, ActivityCheckGate, CheckGateInput{
				ProjectID: in.ProjectID, Gate: gate,
			})
		}
		for gate, f := range futures {
			var r CheckGateResult
			if err := f.Get(ctx, &r); err != nil {
				logger.Error("gate check failed", "gate", gate, "err", err)
				gateResults[gate] = CheckGateResult{Gate: gate, Status: domain.GateStatusBlocked}
				continue
			}
			gateResults[gate] = r
		}

		// Phase 2: collect failing gates that need repair, ordered.
		allPassed := true
		var toRepair []CheckGateResult
		for _, g := range gateOrder {
			r := gateResults[g]
			if r.Status != domain.GateStatusPassed {
				allPassed = false
				toRepair = append(toRepair, r)
			}
		}

		// Phase 3: dispatch repair agents serially (later: in parallel where
		// safe). Each agent run is its own Activity with retry policy.
		for _, r := range toRepair {
			_ = workflow.ExecuteActivity(ctx, ActivityRunAgent, RunAgentInput{
				ProjectID: in.ProjectID,
				Role:      r.RepairAgent,
				Gate:      r.Gate,
				Issues:    r.Issues,
			}).Get(ctx, nil)
		}

		if allPassed {
			out.Completed = true
			break
		}
	}

	// Snapshot final gate state by re-checking once.
	for _, g := range gateOrder {
		var r CheckGateResult
		err := workflow.ExecuteActivity(ctx, ActivityCheckGate, CheckGateInput{
			ProjectID: in.ProjectID, Gate: g,
		}).Get(ctx, &r)
		if err != nil {
			continue
		}
		out.Gates = append(out.Gates, domain.GateState{
			Name:     g,
			Status:   r.Status,
			Issues:   r.Issues,
			UpdatedAt: workflow.Now(ctx),
		})
	}

	return out, nil
}

type CheckGateInput struct {
	ProjectID string          `json:"projectId"`
	Gate      domain.GateName `json:"gate"`
}

type CheckGateResult struct {
	Gate         domain.GateName    `json:"gate"`
	Status       domain.GateStatus  `json:"status"`
	Issues       []domain.Issue     `json:"issues,omitempty"`
	RepairAgent  string             `json:"repairAgent"`
}

type RunAgentInput struct {
	ProjectID string          `json:"projectId"`
	Role      string          `json:"role"`
	Gate      domain.GateName `json:"gate"`
	Issues    []domain.Issue  `json:"issues"`
}

type RunAgentResult struct {
	Output   string  `json:"output"`
	Provider string  `json:"provider"`
	Tokens   int     `json:"tokens"`
	CostUSD  float64 `json:"costUSD"`
}
