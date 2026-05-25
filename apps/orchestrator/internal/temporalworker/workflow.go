package temporalworker

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	WorkflowName = "FinisherExecutionWorkflow"

	eventExecutionStarted   = "execution.started.v1"
	eventExecutionCompleted = "execution.completed.v1"
	eventExecutionStopped   = "execution.stopped.v1"
	eventExecutionKilled    = "execution.killed.v1"
	eventExecutionFailed    = "execution.failed.v1"
)

// FinisherExecutionWorkflow is the production durable runner for one
// already-admitted paid execution. Admission and wallet hold currently
// happen in createPaidExecution; the workflow owns start, guard checks,
// the finisher run, terminal FSM transition, settlement, and event
// emission.
func FinisherExecutionWorkflow(ctx workflow.Context, input WorkflowInput) (WorkflowOutput, error) {
	if input.ExecutionID == "" || input.ProjectID == "" {
		return WorkflowOutput{FinalStatus: "failed"}, ErrInvalidArgument
	}

	activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 6 * time.Hour,
		HeartbeatTimeout:    90 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:        time.Second,
			BackoffCoefficient:     2,
			MaximumInterval:        time.Minute,
			MaximumAttempts:        5,
			NonRetryableErrorTypes: nonRetryableErrorTypes(),
		},
	})
	settleCtx, _ := workflow.NewDisconnectedContext(activityCtx)

	seq := 1
	if err := workflow.ExecuteActivity(activityCtx, StartExecutionActivity, StartInput{
		ExecutionID: input.ExecutionID,
	}).Get(activityCtx, nil); err != nil {
		return settleWorkflow(settleCtx, input, "failed", seq, "start_failed", err.Error())
	}
	_ = workflow.ExecuteActivity(activityCtx, EmitExecutionEventActivity, EmitInput{
		ExecutionID: input.ExecutionID,
		EventType:   eventExecutionStarted,
		Sequence:    seq,
		Payload: map[string]any{
			"project_id": input.ProjectID,
		},
	}).Get(activityCtx, nil)
	seq++

	var pg PGOutput
	if err := workflow.ExecuteActivity(activityCtx, ProfitGuardBeforeStepActivity, PGInput{
		ExecutionID: input.ExecutionID,
		Point:       "before_long_verification",
	}).Get(activityCtx, &pg); err == nil {
		switch pg.Action {
		case "stop", "pause":
			return settleWorkflow(settleCtx, input, "stopped", seq, pg.Reason, "")
		case "kill":
			return settleWorkflow(settleCtx, input, "killed", seq, pg.Reason, "")
		}
	}

	var gate GateOutput
	runErr := workflow.ExecuteActivity(activityCtx, RunGateActivity, GateInput{
		ExecutionID: input.ExecutionID,
		ProjectID:   input.ProjectID,
		Gate:        "finisher",
		Iteration:   1,
	}).Get(activityCtx, &gate)
	if runErr != nil {
		return settleWorkflow(settleCtx, input, "failed", seq, "finisher_error", runErr.Error())
	}
	if !gate.Passed {
		return settleWorkflow(settleCtx, input, "failed", seq, "finisher_incomplete", "")
	}
	return settleWorkflow(settleCtx, input, "succeeded", seq, "completed", "")
}

func settleWorkflow(ctx workflow.Context, input WorkflowInput, status string, seq int, reason, detail string) (WorkflowOutput, error) {
	var settlement SettleOutput
	_ = workflow.ExecuteActivity(ctx, SettleExecutionActivity, SettleInput{
		ExecutionID: input.ExecutionID,
		FinalStatus: status,
	}).Get(ctx, &settlement)

	eventType := eventExecutionCompleted
	switch status {
	case "failed":
		eventType = eventExecutionFailed
	case "stopped":
		eventType = eventExecutionStopped
	case "killed":
		eventType = eventExecutionKilled
	}
	_ = workflow.ExecuteActivity(ctx, EmitExecutionEventActivity, EmitInput{
		ExecutionID: input.ExecutionID,
		EventType:   eventType,
		Sequence:    seq,
		Payload: map[string]any{
			"project_id": input.ProjectID,
			"status":     status,
			"reason":     reason,
			"detail":     detail,
		},
	}).Get(ctx, nil)

	return WorkflowOutput{
		FinalStatus:     status,
		SpentUSD:        settlement.SpentUSD,
		CompletionScore: settlement.CompletionScore,
		GrossMarginPct:  settlement.GrossMarginPct,
	}, nil
}
