// CompletionScorer wraps the generic Service.Score call with the
// feature vector + decoding convention the "will this execution
// finish profitably?" predictor expects. Keeping the wrapper in this
// package (instead of in learning/) lets the Service stay generic and
// the call sites stay clean.

package inference

import (
	"context"
	"errors"
)

// CompletionScorerModelName is the registry key for the completion
// score predictor. Stable across orchestrator restarts so dashboards
// can attribute a score to a specific model version.
const CompletionScorerModelName = "completion-scorer-v1"

// CompletionFeatures is the float-encoded view of an execution that
// the model was trained on. The feature engineering is intentionally
// shallow — anything fancy (token-level features, embeddings of the
// prompt) belongs in a downstream model, not in this one. The order
// here is THE training contract: changing it silently breaks the
// model's predictions, so the field names match the column order in
// the training notebook one-for-one.
type CompletionFeatures struct {
	// PromptLengthLog is log10(1 + len(prompt)). Captures "how big is
	// the ask?" without letting an outlier swamp the linear model.
	PromptLengthLog float32
	// GateFailureRate is the user's historical fail-rate across all
	// gates in the last 30 days, clamped to [0, 1].
	GateFailureRate float32
	// ProviderConfidence is the router's current confidence in the
	// chosen provider for this workload (bandit posterior mean).
	ProviderConfidence float32
	// WalletHeadroomLog is log10(1 + walletBalance - reservation).
	// Negative values mean the user is running on a thin wallet so
	// the scorer can dampen its completion prediction.
	WalletHeadroomLog float32
	// PriorSuccessRate is the user's historical success rate on
	// blueprints of the same kind in the last 90 days.
	PriorSuccessRate float32
	// ComplexityScore is the blueprint's own difficulty signal,
	// normalised to [0, 1].
	ComplexityScore float32
}

// flat returns the features in the exact order the ONNX model was
// trained against. Single source of truth for the column order.
func (f CompletionFeatures) flat() []float32 {
	return []float32{
		f.PromptLengthLog,
		f.GateFailureRate,
		f.ProviderConfidence,
		f.WalletHeadroomLog,
		f.PriorSuccessRate,
		f.ComplexityScore,
	}
}

// CompletionScore wraps the model's raw output. We surface the
// probability AND whether the prediction is available so callers can
// either blend it into their heuristic or fall back when the model is
// not loaded.
type CompletionScore struct {
	// Probability in [0, 1] that the execution finishes successfully
	// AND with positive gross margin. Sigmoid output from the model
	// head — the orchestrator clamps it defensively.
	Probability float32
	// Available is true when the score came from a real model; false
	// when the caller should fall back to its heuristic prior.
	Available bool
}

// PredictCompletion wraps Service.Score for the completion-score
// model. Returns Available=false with no error when the model is
// unavailable so callers can branch on the value alone. Any other
// error is propagated so the caller decides whether to log + degrade
// or hard-fail.
func PredictCompletion(ctx context.Context, svc Service, features CompletionFeatures) (CompletionScore, error) {
	if svc == nil {
		return CompletionScore{}, nil
	}
	out, err := svc.Score(ctx, CompletionScorerModelName, features.flat())
	if errors.Is(err, ErrModelUnavailable) {
		return CompletionScore{}, nil
	}
	if err != nil {
		return CompletionScore{}, err
	}
	if len(out) == 0 {
		return CompletionScore{}, nil
	}
	p := out[0]
	switch {
	case p < 0:
		p = 0
	case p > 1:
		p = 1
	}
	return CompletionScore{Probability: p, Available: true}, nil
}

// CompletionScorerModel describes the artefact for LoadModel. Helper
// keeps main.go's wireup short and self-documenting.
func CompletionScorerModel(path string) Model {
	return Model{
		Name:        CompletionScorerModelName,
		Version:     "v1",
		Path:        path,
		InputShape:  []int64{-1, 6},
		OutputShape: []int64{-1, 1},
	}
}
