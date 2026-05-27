// IntentClassifier wraps the generic Service.Score call with the
// label set the "what kind of work is the user asking for?" model
// emits. Used by the IntentRouter upstream of the Coder so that an
// expensive Sonnet/Opus call is only spent on intents that actually
// need it.

package inference

import (
	"context"
	"errors"
)

// IntentModelName is the registry key for the intent classifier.
const IntentModelName = "intent-classifier-v1"

// IntentLabel enumerates the model head's output classes. Order
// matches the training notebook's label encoder — do not reorder
// without retraining the model.
type IntentLabel int

const (
	IntentUnknown IntentLabel = iota
	IntentBuild
	IntentRefactor
	IntentDebug
	IntentDeploy
	IntentExplain
	IntentTest // we still emit this label even though we don't write tests — the model needs to recognise the user asking for tests so we can refuse politely
)

// String renders an IntentLabel for logs + dashboards.
func (l IntentLabel) String() string {
	switch l {
	case IntentBuild:
		return "build"
	case IntentRefactor:
		return "refactor"
	case IntentDebug:
		return "debug"
	case IntentDeploy:
		return "deploy"
	case IntentExplain:
		return "explain"
	case IntentTest:
		return "test"
	default:
		return "unknown"
	}
}

// IntentFeatures is the float-encoded view of the user's prompt + the
// project context. Real production deployments will replace this with
// a small sentence-encoder embedding (a 384-dim vector) — the current
// shape is the lexical-features baseline so the wiring is exercised
// before the embedding upgrade lands.
type IntentFeatures struct {
	// Embedding is the encoded prompt. Length must match the model's
	// per-sample input dim (384 for the bge-small encoder).
	Embedding []float32
}

// IntentPrediction is the decoded answer with calibrated confidence.
type IntentPrediction struct {
	// Label is the argmax over the model's softmax head.
	Label IntentLabel
	// Confidence is the probability mass on the chosen Label.
	Confidence float32
	// Available mirrors CompletionScore.Available.
	Available bool
}

// ClassifyIntent runs the classifier. Same contract as
// PredictCompletion: Available=false with nil error when the model
// isn't loaded so the caller falls back to the lexical router.
func ClassifyIntent(ctx context.Context, svc Service, features IntentFeatures) (IntentPrediction, error) {
	if svc == nil || len(features.Embedding) == 0 {
		return IntentPrediction{}, nil
	}
	out, err := svc.Score(ctx, IntentModelName, features.Embedding)
	if errors.Is(err, ErrModelUnavailable) {
		return IntentPrediction{}, nil
	}
	if err != nil {
		return IntentPrediction{}, err
	}
	if len(out) == 0 {
		return IntentPrediction{}, nil
	}
	argmax := 0
	best := out[0]
	for i := 1; i < len(out); i++ {
		if out[i] > best {
			best = out[i]
			argmax = i
		}
	}
	if best < 0 {
		best = 0
	}
	if best > 1 {
		best = 1
	}
	return IntentPrediction{
		Label:      IntentLabel(argmax),
		Confidence: best,
		Available:  true,
	}, nil
}

// IntentClassifierModel builds the LoadModel descriptor. Per-sample
// shape is 384 (the bge-small-en-v1.5 hidden size used elsewhere in
// the stack). Output shape is 7 — one logit per IntentLabel.
func IntentClassifierModel(path string) Model {
	return Model{
		Name:        IntentModelName,
		Version:     "v1",
		Path:        path,
		InputShape:  []int64{-1, 384},
		OutputShape: []int64{-1, 7},
	}
}
