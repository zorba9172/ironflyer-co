// HallucinationDetector wraps the generic Service.Score call with the
// feature vector the "is this LLM output likely hallucinated?"
// classifier expects. The model is a small distilled BERT head trained
// on (output, retrieved_context) pairs labelled by grounded vs.
// fabricated — keeps Ironflyer's "no fake-ship" guarantee enforceable
// without a round-trip to a third-party fact-check API.

package inference

import (
	"context"
	"errors"
)

// HallucinationModelName is the registry key for the classifier.
const HallucinationModelName = "hallucination-v1"

// HallucinationFeatures is the float-encoded view of one LLM response
// + its retrieval grounding. As with CompletionFeatures, this order
// IS the training contract.
type HallucinationFeatures struct {
	// OutputLengthLog is log10(1 + len(output)). Very long outputs
	// with very little grounding are the canonical hallucination
	// shape.
	OutputLengthLog float32
	// RetrievalOverlap is the fraction of output n-grams (n=3) that
	// appear verbatim in the retrieved context window. Computed by
	// the caller (cheap; pure Go).
	RetrievalOverlap float32
	// CitationDensity is citations-per-1000-chars in the output. A
	// thoughtful answer that names its sources usually wins here.
	CitationDensity float32
	// HedgingDensity is hedging-phrases-per-1000-chars ("I think",
	// "possibly", "may be"). Inverted in the model's head — too
	// little hedging in a long answer is suspicious.
	HedgingDensity float32
	// ProviderConfidence is the same signal the completion scorer
	// uses — re-shared so the classifier learns to discount low-
	// confidence outputs from a poorly-tuned provider.
	ProviderConfidence float32
}

func (f HallucinationFeatures) flat() []float32 {
	return []float32{
		f.OutputLengthLog,
		f.RetrievalOverlap,
		f.CitationDensity,
		f.HedgingDensity,
		f.ProviderConfidence,
	}
}

// HallucinationVerdict is the classifier's decoded answer. The
// orchestrator uses Likelihood to gate the verifier's "let this
// answer through" decision; Available is false when the model isn't
// loaded so the caller can fall back to "let it through with a
// could-not-check note".
type HallucinationVerdict struct {
	// Likelihood in [0, 1] that the output is hallucinated relative
	// to its retrieval context. Higher = more suspicious.
	Likelihood float32
	// Available mirrors CompletionScore.Available.
	Available bool
}

// DetectHallucination runs the classifier. Same contract as
// PredictCompletion: Available=false with nil error when the model
// isn't loaded.
func DetectHallucination(ctx context.Context, svc Service, features HallucinationFeatures) (HallucinationVerdict, error) {
	if svc == nil {
		return HallucinationVerdict{}, nil
	}
	out, err := svc.Score(ctx, HallucinationModelName, features.flat())
	if errors.Is(err, ErrModelUnavailable) {
		return HallucinationVerdict{}, nil
	}
	if err != nil {
		return HallucinationVerdict{}, err
	}
	if len(out) == 0 {
		return HallucinationVerdict{}, nil
	}
	p := out[0]
	switch {
	case p < 0:
		p = 0
	case p > 1:
		p = 1
	}
	return HallucinationVerdict{Likelihood: p, Available: true}, nil
}

// HallucinationModel builds the LoadModel descriptor.
func HallucinationModel(path string) Model {
	return Model{
		Name:        HallucinationModelName,
		Version:     "v1",
		Path:        path,
		InputShape:  []int64{-1, 5},
		OutputShape: []int64{-1, 1},
	}
}
