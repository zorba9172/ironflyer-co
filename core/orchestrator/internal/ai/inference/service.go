// Package inference's Service contract + the shared NoopService that
// every build (with or without -tags onnx) compiles in. The OnnxService
// constructor lives in onnx.go (built only with -tags onnx) and the
// stub in onnx_disabled.go — both implement the same Service
// interface so callers stay tag-agnostic.

package inference

import (
	"context"
	"errors"
	"sync"

	"github.com/rs/zerolog"
)

// ErrModelUnavailable is the sentinel returned by NoopService and by
// OnnxService when the requested model name is not loaded. Callers MUST
// use errors.Is(err, ErrModelUnavailable) to detect the "no scorer
// today, fall back to the heuristic" case rather than treating it as a
// hard failure. The orchestrator's correctness must not depend on any
// inference model being present — the models are a speed + accuracy
// upgrade, never a load-bearing dependency.
var ErrModelUnavailable = errors.New("inference: model unavailable")

// ErrFeatureShapeMismatch is returned by Score when the provided
// feature vector length does not match the model's declared InputShape.
// Surfaced separately so callers can log it loudly (it indicates a
// caller bug, not an operator misconfiguration).
var ErrFeatureShapeMismatch = errors.New("inference: feature shape mismatch")

// Model is the static description of an ONNX artefact the orchestrator
// can serve. The struct is intentionally minimal — anything fancier
// (tokenizers, label maps, calibration tables) lives in the model-
// specific wrapper (completion_scorer.go etc.) so the registry stays
// generic.
type Model struct {
	// Name is the registry key (e.g. "completion-scorer-v1"). Callers
	// pass this to Score; it must be stable across orchestrator
	// restarts so dashboards can attribute scores to a specific model
	// version.
	Name string

	// Version is informational — surfaced in /healthz and the
	// inference_models metric so operators can see which build is
	// live without exec'ing into the pod.
	Version string

	// Path is the absolute filesystem path to the .onnx artefact.
	// In the Helm chart this is rendered under IRONFLYER_MODELS_DIR
	// (default /var/ironflyer/models) and populated by the
	// inference-models initContainer pulling from S3 / R2.
	Path string

	// InputShape declares the tensor shape Score() expects. Use -1 for
	// dynamic dims (batch). The first dim is conventionally the batch
	// dim; subsequent dims are the per-sample feature shape.
	InputShape []int64

	// OutputShape mirrors InputShape for the predictor head. For
	// classification heads this is typically [-1, numClasses]; for
	// regression heads it's [-1, 1].
	OutputShape []int64
}

// Service is the runtime-callable contract. Concurrent calls to Score
// are safe — implementations serialise on the underlying ORT session
// internally (ORT sessions are not goroutine-safe).
//
// Score takes a flat []float32 features vector. The implementation
// reshapes it according to model.InputShape and returns the model's
// output as a flat []float32 (callers reshape per their head). The
// flat layout keeps the interface tiny while letting callers express
// their own feature engineering on top.
type Service interface {
	// Score runs one inference pass against the named model. Returns
	// ErrModelUnavailable when the model is not loaded; returns
	// ErrFeatureShapeMismatch when len(features) != product of model
	// InputShape (excluding the batch dim).
	Score(ctx context.Context, modelName string, features []float32) ([]float32, error)

	// LoadModel registers a model with the service. Idempotent: calling
	// it twice with the same Name replaces the previous artefact (so
	// operators can hot-swap a new version without restarting the pod).
	// The Noop impl records the registration but returns
	// ErrModelUnavailable from Score so the contract stays honest.
	LoadModel(ctx context.Context, model Model) error

	// Models lists everything currently registered. Used by the
	// /healthz JSON payload and the inference_models Prometheus gauge.
	Models() []Model

	// HealthCheck returns nil when the service is in a serviceable
	// state. The Noop impl always returns nil (no models registered is
	// not an error — it's the default). The Onnx impl returns the
	// last ORT init error if startup never completed.
	HealthCheck(ctx context.Context) error
}

// NoopService is the default implementation compiled into every
// orchestrator build. It records LoadModel calls so /healthz can report
// "model declared but inference disabled — rebuild with -tags onnx"
// instead of silently swallowing them, but Score always returns
// ErrModelUnavailable.
type NoopService struct {
	mu     sync.RWMutex
	logger zerolog.Logger
	models map[string]Model
}

// NewNoopService returns a Service that records model registrations but
// never runs inference. Use this in every build that does NOT have
// IRONFLYER_INFERENCE_ENABLED=true or was not built with -tags onnx.
func NewNoopService(logger zerolog.Logger) *NoopService {
	return &NoopService{
		logger: logger.With().Str("component", "inference.noop").Logger(),
		models: make(map[string]Model),
	}
}

// Score always returns ErrModelUnavailable so callers fall through to
// their heuristic path. Logged at debug level so an operator who
// genuinely cares can grep for the call site without drowning the
// production log.
func (s *NoopService) Score(_ context.Context, modelName string, _ []float32) ([]float32, error) {
	s.logger.Debug().Str("model", modelName).Msg("Score: noop service, returning ErrModelUnavailable")
	return nil, ErrModelUnavailable
}

// LoadModel records the registration and logs at info level so
// operators see "model declared, inference disabled" on startup —
// surfaces a misconfiguration (Helm flipped inference.enabled but the
// binary lacks -tags onnx) without crashing the pod.
func (s *NoopService) LoadModel(_ context.Context, model Model) error {
	s.mu.Lock()
	s.models[model.Name] = model
	s.mu.Unlock()
	s.logger.Info().
		Str("model", model.Name).
		Str("version", model.Version).
		Msg("LoadModel: registered, but inference disabled (NoopService); rebuild with -tags onnx to enable")
	return nil
}

// Models returns a stable snapshot of the registered models. Safe to
// call concurrently with LoadModel.
func (s *NoopService) Models() []Model {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Model, 0, len(s.models))
	for _, m := range s.models {
		out = append(out, m)
	}
	return out
}

// HealthCheck always returns nil — "disabled" is the default state,
// not a failure mode. The /healthz payload separately surfaces whether
// any model is actually live via Models().
func (s *NoopService) HealthCheck(_ context.Context) error { return nil }

// compile-time interface satisfaction.
var _ Service = (*NoopService)(nil)
