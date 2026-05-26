// Package inference hosts local ONNX-Runtime scoring models that the
// orchestrator serves itself — no third-party API call. The models are
// small (single-digit MB each), CPU-friendly, and live alongside the
// orchestrator binary so latency stays inside the request budget that
// gates like Profit Guard already enforce.
//
// Why a dedicated package
//
// The embeddings package already wraps an ONNX session for sentence
// encoding — that's a single, well-known shape (BERT-style encoder ->
// pooled vector). The inference package generalises the same plumbing
// for arbitrary small predictors:
//
//   - completion-score predictor  — "given these execution features,
//     what is the probability this run finishes profitably?"
//   - hallucination detector      — "does this LLM output look made-up
//     vs. grounded in retrieved context?"
//   - intent classifier           — "which workflow does this user
//     prompt belong to (build / refactor / debug / deploy / …)?"
//
// Each model exposes the same Service.Score(modelName, features) ->
// scores contract so callers (CompletionPredictor, RepairMatcher,
// IntentRouter) don't have to special-case any one of them.
//
// Build modes
//
// The default production build (no -tags onnx) wires NewNoopService,
// which records the call in the logger and returns ErrModelUnavailable.
// Every caller therefore degrades gracefully: the completion scorer
// falls back to its heuristic prior, the hallucination detector lets
// the output through with a "could not check" note, the intent
// classifier defers to the lexical router.
//
// The onnx build (`go build -tags onnx ./...`) compiles in
// NewOnnxService, which loads models from disk at startup
// (IRONFLYER_MODELS_DIR) and runs inference through the same
// onnxruntime_go binding the embeddings package already uses.
//
// Privacy boundary
//
// The whole point is that prompt content + execution telemetry NEVER
// leave the tenant boundary for scoring. The Pro tier guarantees this
// in writing: when inference.enabled is true and the model is loaded,
// completion-score / hallucination / intent classification all run
// inside the user's namespace pod. This is what differentiates
// Ironflyer from Lovable / Bolt / v0, which fan everything out to
// third-party LLM APIs for every micro-decision.
//
// Operator runbook lives in docs/DEEP_LEARNING.md.
package inference
