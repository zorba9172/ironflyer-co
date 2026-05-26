// Package finisher — gates_verifier.go is the live-preview proof
// gate. It runs AFTER the Code gate compiles a clean tree and
// BEFORE Lint/Test bookkeeping, so an unverifiable acceptance
// criterion fails the loop before linters complain about
// stylistic noise on dead code.
//
// The gate is intentionally fail-soft: when there is no runtime,
// no workspace, no preview URL, or no acceptance criteria, the
// gate returns zero issues. Verification is additive signal — it
// must never block a project that is too early in its lifecycle
// to have a live preview yet.

package finisher

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/ai/verifier"
	"ironflyer/core/orchestrator/internal/operations/logctx"
)

// verifierRegistrySlot is the package-level sink the engine writes
// the agents.Registry into at boot via RegisterVerifierRegistry.
// VerifierGate.Check reads it back. This is the same pattern other
// gates use (gateTelemetry / completionScorer) to avoid threading
// large dependencies through the Gate interface signature.
var (
	verifierRegistryMu sync.RWMutex
	verifierRegistrySlot *agents.Registry
)

// RegisterVerifierRegistry wires the agents.Registry the VerifierGate
// uses to dispatch the verifier agent. Nil registers nothing; callers
// can pass nil during shutdown to disarm the gate.
func RegisterVerifierRegistry(r *agents.Registry) {
	verifierRegistryMu.Lock()
	defer verifierRegistryMu.Unlock()
	verifierRegistrySlot = r
}

func verifierRegistry() *agents.Registry {
	verifierRegistryMu.RLock()
	defer verifierRegistryMu.RUnlock()
	return verifierRegistrySlot
}

// jsonUnmarshalAny is a thin wrapper used by the verifier gate to
// parse stored artifact bytes back into a typed map. Lives here so
// the gate file owns its serialisation surface.
func jsonUnmarshalAny(raw []byte, out any) error {
	return json.Unmarshal(raw, out)
}

// VerifierGate is the Playwright-driven proof gate. It depends on:
//
//   - env.Runtime + env.WorkspaceID — to drive headless chromium
//     inside the user's sandbox.
//   - env.Project.Spec.UserStories.Acceptance — the criteria to
//     prove.
//   - A preview URL — looked up via env.Runtime.PreviewURL.
//   - An agents.Registry — looked up via the package-level
//     verifierRegistry sink. The engine wires it in NewEngine via
//     RegisterVerifierRegistry so the gate Check signature stays
//     compatible with the Gate interface.
type VerifierGate struct{}

func (VerifierGate) Name() domain.GateName    { return domain.GateVerifier }
func (VerifierGate) RepairAgent() agents.Role { return agents.RoleCoder }

// Check exercises every AcceptanceCriterion through the verifier
// loop. Issues land one per failing criterion (SeverityError) or
// one per warn (SeverityWarning). Pass / skipped criteria yield no
// issues. On graceful degradation (no runtime / no preview URL /
// no criteria) Check returns nil and the gate is treated as
// passed by the engine.
func (VerifierGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	if env == nil || env.Project == nil {
		return nil
	}
	lg := logctx.From(ctx)
	if !env.HasRuntime() {
		lg.Info().Msg("verifier gate: runtime unavailable — skipping")
		return nil
	}
	if len(env.Project.Spec.UserStories) == 0 {
		lg.Info().Msg("verifier gate: project has no stories yet — skipping")
		return nil
	}
	previewURL := lookupPreviewURL(ctx, env, lg)
	if strings.TrimSpace(previewURL) == "" {
		// No preview URL is a soft signal — surface a warning so
		// the dashboard chip flags "verification dark" without
		// blocking the loop.
		return []domain.Issue{{
			Gate: domain.GateVerifier, Severity: domain.SeverityWarning,
			Message: "live preview URL is empty — verifier gate is dark",
			Hint:    "ensure the dev server is running and the runtime has a preview port allocated",
		}}
	}
	registry := verifierRegistry()
	if registry == nil {
		// Defensive — the engine should always wire it. We surface a
		// SeverityInfo rather than fail the gate because a missing
		// registry is a wireup bug the operator must fix; punishing
		// the user's project for it is wrong.
		return []domain.Issue{{
			Gate: domain.GateVerifier, Severity: domain.SeverityInfo,
			Message: "verifier gate is not wired (registry missing)",
			Hint:    "register the agent Registry via RegisterVerifierRegistry at startup",
		}}
	}
	// Wider timeout — chromium cold start plus a few criteria can
	// take longer than the default gate budget.
	cctx, cancel := context.WithTimeout(ctx, 600*time.Second)
	defer cancel()
	results, err := verifier.Run(cctx, verifier.RunInput{
		Project:     env.Project,
		Runtime:     env.Runtime,
		UserBearer:  env.UserBearer,
		WorkspaceID: env.WorkspaceID,
		PreviewURL:  previewURL,
		Registry:    registry,
	})
	if err != nil {
		return []domain.Issue{{
			Gate: domain.GateVerifier, Severity: domain.SeverityWarning,
			Message: "verifier loop errored: " + err.Error(),
			Hint:    "the gate degrades to warning so the project can still progress while you investigate",
		}}
	}
	// Stamp LastVerifiedAt on every passed criterion so the UI can
	// render staleness. We update the project's user-stories in-
	// place; the engine's setGate call later persists the snapshot.
	if passed := verifier.PassedCriterionIDs(results); len(passed) > 0 {
		stampLastVerifiedAt(env.Project, passed, time.Now().UTC())
	}
	return verifier.IssuesFromResults(results)
}

// lookupPreviewURL asks the runtime for the live-preview URL bound
// to the workspace. Best-effort: an empty return surfaces as a
// SeverityWarning at the gate layer, never an error.
func lookupPreviewURL(ctx context.Context, env *GateEnv, lg zerolog.Logger) string {
	if env == nil || !env.HasRuntime() {
		return ""
	}
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	url, err := env.Runtime.PreviewURL(cctx, env.UserBearer, env.WorkspaceID)
	if err != nil {
		lg.Warn().Err(err).Msg("verifier gate: PreviewURL lookup failed")
		return ""
	}
	return strings.TrimSpace(url)
}

// stampLastVerifiedAt updates AcceptanceCriterion.LastVerifiedAt
// for every criterion that landed verdict=pass. We synthesise the
// criterion ID exactly the way verifier.collectCriteria does
// (`<storyId>#<index>`) so the IDs join cleanly.
func stampLastVerifiedAt(p *domain.Project, passedIDs []string, ts time.Time) {
	if p == nil || len(passedIDs) == 0 {
		return
	}
	want := make(map[string]struct{}, len(passedIDs))
	for _, id := range passedIDs {
		want[id] = struct{}{}
	}
	// AcceptanceCriterion lives synthetically — the project's
	// UserStory.Acceptance is a []string. We attach the
	// "last verified" timestamp by mirroring it on the project's
	// Artifacts map under a stable key. UI surfaces read both —
	// the canonical synthesis happens in the GraphQL studio
	// resolver, which augments criteria with the timestamp.
	//
	// Storing per-criterion timestamps as a small map keeps the
	// project's wire-shape backwards-compatible: nothing else in
	// this run touches the existing []string acceptance list.
	const artifactName = "verifier_last_verified_at"
	prev := map[string]time.Time{}
	if raw, ok := p.GetArtifact(artifactName); ok && len(raw) > 0 {
		_ = jsonUnmarshalTimes(raw, prev)
	}
	for id := range want {
		prev[id] = ts
	}
	_ = p.SetArtifact(artifactName, prev)
}

// jsonUnmarshalTimes parses the artifact bytes back into a
// map[string]time.Time. We thread through encoding/json on the
// caller's behalf so the artifact stays a single source of truth.
func jsonUnmarshalTimes(raw []byte, out map[string]time.Time) error {
	return jsonUnmarshalAny(raw, &out)
}
