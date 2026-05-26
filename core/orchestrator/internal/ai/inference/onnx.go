//go:build onnx

// OnnxService — real local-inference implementation using the same
// yalue/onnxruntime_go binding the embeddings package already pulls in.
// Compiled only with `go build -tags onnx ./...`; the default build
// gets the stub in onnx_disabled.go.
//
// BUILD: requires CGO and the ONNX Runtime C++ shared library on the
// host. Install via apt on Linux x64 or download the release artefact
// from https://github.com/microsoft/onnxruntime/releases. Point
// ONNXRUNTIME_SHARED_LIBRARY at the absolute path of libonnxruntime.so
// before the orchestrator starts. Same env contract as the embeddings
// ONNX backend so a single ORT install powers both surfaces.
//
// MODELS: artefacts live under IRONFLYER_MODELS_DIR (default
// /var/ironflyer/models in the Helm chart). The
// infra/helm/ironflyer/templates/inference-models.yaml initContainer
// pulls them from S3/R2 at deploy time and mounts the directory into
// the orchestrator pod. The directory layout is flat — one .onnx file
// per registered model, named after Model.Name.

package inference

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/rs/zerolog"
	ort "github.com/yalue/onnxruntime_go"
)

// ortInitOnce mirrors embeddings.initORT — the C runtime is a
// process-wide singleton so we only call InitializeEnvironment once
// across all packages. Calling it a second time panics inside the C
// binding. Both packages funnel through their own sync.Once because
// the order in which they're constructed is not guaranteed.
var (
	ortInitOnce sync.Once
	ortInitErr  error
)

func initORT() error {
	ortInitOnce.Do(func() {
		if lib := os.Getenv("ONNXRUNTIME_SHARED_LIBRARY"); lib != "" {
			ort.SetSharedLibraryPath(lib)
		}
		ortInitErr = ort.InitializeEnvironment()
	})
	return ortInitErr
}

// OnnxService loads and serves arbitrary small predictors via the ONNX
// Runtime. Safe for concurrent use: the per-model session is guarded
// by its own mutex because ORT sessions are not goroutine-safe; the
// registry map is guarded separately so a hot LoadModel doesn't block
// concurrent Score calls against a different model.
type OnnxService struct {
	logger    zerolog.Logger
	modelsDir string

	mu       sync.RWMutex
	registry map[string]*onnxModel
}

// onnxModel pairs an ort session with the Model metadata. The session
// mutex serialises Run calls; the metadata is read-only after load.
type onnxModel struct {
	meta Model
	mu   sync.Mutex
	sess *ort.DynamicAdvancedSession
}

// NewOnnxService initialises the ORT runtime and returns a ready Service.
// modelsDir is the base path the LoadModel resolves relative paths
// against; pass an empty string when every Model.Path is absolute.
//
// Initialisation does NOT load any models — that's the operator's
// explicit responsibility via LoadModel (driven from main.go's wireup,
// gated by IRONFLYER_INFERENCE_ENABLED + per-model values in Helm).
// Returning a Service rather than a concrete type so a future failure
// in initORT can swap in a NoopService transparently.
func NewOnnxService(modelsDir string, logger zerolog.Logger) Service {
	logger = logger.With().Str("component", "inference.onnx").Logger()
	if err := initORT(); err != nil {
		logger.Warn().Err(err).Msg("OnnxService: ORT init failed, falling back to NoopService")
		return NewNoopService(logger)
	}
	logger.Info().Str("modelsDir", modelsDir).Msg("OnnxService: ORT initialised")
	return &OnnxService{
		logger:    logger,
		modelsDir: modelsDir,
		registry:  make(map[string]*onnxModel),
	}
}

// LoadModel resolves the artefact path, opens an ORT session, and
// registers the model. Replaces any existing entry with the same Name
// so operators can hot-swap a new version. Errors surface the
// underlying ORT message — the caller (main.go) typically logs a Warn
// and continues without that model rather than crashing the pod.
func (s *OnnxService) LoadModel(_ context.Context, model Model) error {
	if model.Name == "" {
		return fmt.Errorf("inference: model.Name is required")
	}
	path := model.Path
	if !filepath.IsAbs(path) && s.modelsDir != "" {
		path = filepath.Join(s.modelsDir, path)
	}
	if path == "" {
		return fmt.Errorf("inference: model %q: empty path and no modelsDir", model.Name)
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("inference: model %q artefact %q: %w", model.Name, path, err)
	}

	// Single named input/output convention — keeps the registry generic
	// across regression and classification heads. Models exported with
	// different IO names need their own loader (one will land alongside
	// completion_scorer.go etc. when we have a real bge-style encoder
	// to host here).
	sess, err := ort.NewDynamicAdvancedSession(
		path,
		[]string{"features"},
		[]string{"scores"},
		nil,
	)
	if err != nil {
		return fmt.Errorf("inference: model %q session: %w", model.Name, err)
	}

	loaded := &onnxModel{meta: model, sess: sess}
	s.mu.Lock()
	if prev, ok := s.registry[model.Name]; ok {
		// Destroy the previous session OUTSIDE the registry lock so a
		// slow C-side close doesn't stall concurrent Score callers
		// hitting other models.
		go func(p *onnxModel) {
			p.mu.Lock()
			defer p.mu.Unlock()
			if p.sess != nil {
				_ = p.sess.Destroy()
			}
		}(prev)
	}
	s.registry[model.Name] = loaded
	s.mu.Unlock()

	s.logger.Info().
		Str("model", model.Name).
		Str("version", model.Version).
		Str("path", path).
		Msg("LoadModel: registered")
	return nil
}

// Score runs one inference pass. Features must match the model's
// InputShape (excluding the batch dim, which we always set to 1).
// Returns ErrFeatureShapeMismatch when the shape doesn't line up;
// returns ErrModelUnavailable when the model isn't registered.
func (s *OnnxService) Score(_ context.Context, modelName string, features []float32) ([]float32, error) {
	s.mu.RLock()
	m, ok := s.registry[modelName]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrModelUnavailable, modelName)
	}

	// Compute the expected per-sample feature length from InputShape,
	// skipping a leading dynamic-batch dim. We always run batch=1 here
	// — batching is the caller's concern (the cost predictor and
	// completion scorer both score one execution at a time).
	expected, shape := perSampleAndShape(m.meta.InputShape, len(features))
	if expected > 0 && len(features) != expected {
		return nil, fmt.Errorf("%w: model %q expects %d features, got %d", ErrFeatureShapeMismatch, modelName, expected, len(features))
	}

	in, err := ort.NewTensor(ort.NewShape(shape...), features)
	if err != nil {
		return nil, fmt.Errorf("inference: input tensor: %w", err)
	}
	defer in.Destroy()

	outShape := outputShapeFor(m.meta.OutputShape)
	out, err := ort.NewEmptyTensor[float32](ort.NewShape(outShape...))
	if err != nil {
		return nil, fmt.Errorf("inference: output tensor: %w", err)
	}
	defer out.Destroy()

	m.mu.Lock()
	err = m.sess.Run([]ort.Value{in}, []ort.Value{out})
	m.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("inference: run %q: %w", modelName, err)
	}

	src := out.GetData()
	cp := make([]float32, len(src))
	copy(cp, src)
	return cp, nil
}

// Models returns a stable snapshot. Order is non-deterministic — sort
// at the caller if presentation matters (the /healthz JSON handler
// does).
func (s *OnnxService) Models() []Model {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Model, 0, len(s.registry))
	for _, m := range s.registry {
		out = append(out, m.meta)
	}
	return out
}

// HealthCheck returns the last ORT init error if one occurred. Once
// init has succeeded, the service is healthy even if zero models are
// loaded — a "no models declared" config is a valid degraded mode.
func (s *OnnxService) HealthCheck(_ context.Context) error {
	if ortInitErr != nil {
		return fmt.Errorf("inference: ORT init: %w", ortInitErr)
	}
	return nil
}

// perSampleAndShape extracts the expected flat feature length from an
// InputShape spec and returns the shape to pass to ort.NewShape. Any
// -1 entry is treated as batch=1; remaining positive dims multiply
// into the per-sample length. Returns (0, ...) when the shape is
// empty so the caller skips the length check (truly dynamic models).
func perSampleAndShape(input []int64, flatLen int) (int, []int64) {
	if len(input) == 0 {
		return 0, []int64{1, int64(flatLen)}
	}
	shape := make([]int64, len(input))
	expected := 1
	saw := false
	for i, d := range input {
		if d < 0 {
			shape[i] = 1 // batch dim resolved to 1
			continue
		}
		shape[i] = d
		expected *= int(d)
		saw = true
	}
	if !saw {
		return 0, shape
	}
	return expected, shape
}

// outputShapeFor mirrors perSampleAndShape for the output side. -1
// dims become 1 so ORT can allocate a concrete tensor; positive dims
// are passed through.
func outputShapeFor(output []int64) []int64 {
	if len(output) == 0 {
		return []int64{1, 1}
	}
	shape := make([]int64, len(output))
	for i, d := range output {
		if d < 0 {
			shape[i] = 1
		} else {
			shape[i] = d
		}
	}
	return shape
}

// compile-time interface satisfaction.
var _ Service = (*OnnxService)(nil)
