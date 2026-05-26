package execution

import (
	"context"

	"ironflyer/core/orchestrator/internal/ai/learning"
)

// LearningExecutionReader adapts an execution.Service into the
// learning.ExecutionReader interface. The closure-score calculator
// reads the trimmed view rather than the full Execution struct so the
// learning package never has to import business/execution (which would
// create an import cycle with this package's learning.Publish calls).
type LearningExecutionReader struct {
	svc Service
}

// NewLearningExecutionReader returns the adapter. svc MAY be nil —
// GetExecutionView then returns an empty view + nil error so the
// closure resolver still renders.
func NewLearningExecutionReader(svc Service) *LearningExecutionReader {
	return &LearningExecutionReader{svc: svc}
}

// GetExecutionView satisfies learning.ExecutionReader.
func (r *LearningExecutionReader) GetExecutionView(ctx context.Context, id string) (learning.ExecutionView, error) {
	if r == nil || r.svc == nil {
		return learning.ExecutionView{ID: id}, nil
	}
	exec, err := r.svc.Get(ctx, id)
	if err != nil {
		return learning.ExecutionView{}, err
	}
	return learning.ExecutionView{
		ID:              exec.ID,
		CompletionScore: exec.CompletionScore,
		RevenueUSD:      exec.RevenueUSD,
		SpentUSD:        exec.SpentUSD,
		ReservedUSD:     exec.ReservedUSD,
	}, nil
}
