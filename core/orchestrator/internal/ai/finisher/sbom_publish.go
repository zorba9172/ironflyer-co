package finisher

// sbom_publish.go generates a CycloneDX software bill-of-materials for a
// finished run and persists it as a workspace artifact. This makes the
// AppSec/SBOM promise literal: every run that ships dependencies leaves a
// .ironflyer/sbom.json the operator (and the studio SecurityPane) can read
// and export. Best-effort by contract — a failure here never affects the
// run report or settlement.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/business/execution"
	"ironflyer/core/orchestrator/internal/operations/appsec"
)

// publishSBOM builds the dependency inventory from the project's files,
// renders a CycloneDX 1.6 document, writes it to .ironflyer/sbom.json, and
// emits artifact.sbom.published.v1 on the execution feed. No-op when the
// project has no resolvable dependency components (e.g. a docs-only repo).
func (e *Engine) publishSBOM(ctx context.Context, projectID string, p domain.Project) {
	if e == nil || e.projects == nil {
		return
	}

	target := appsec.Target{ProjectID: p.ID, Files: make([]appsec.File, 0, len(p.Files))}
	for _, f := range p.Files {
		target.Files = append(target.Files, appsec.File{Path: f.Path, Content: f.Content})
	}
	inv := appsec.BuildInventory(target)
	if len(inv.Components) == 0 {
		return
	}

	raw, err := appsec.CycloneDXJSON(p.ID, inv, time.Now().UTC())
	if err != nil {
		return
	}
	sum := sha256.Sum256(raw)

	// Persist as a workspace artifact so the studio file tree and the
	// SecurityPane "Export SBOM" action have a real document to read.
	_, _ = e.projects.Update(projectID, func(pp *domain.Project) {
		writeProjectFile(pp, ".ironflyer/sbom.json", string(raw))
	})

	emitExecutionEvent(ctx, e.executionService, execution.EventArtifactSBOMPublishedV1, map[string]any{
		"format":          "cyclonedx",
		"spec_version":    "1.6",
		"component_count": len(inv.Components),
		"size_bytes":      len(raw),
		"sha256":          hex.EncodeToString(sum[:]),
		"path":            ".ironflyer/sbom.json",
		"published_at":    time.Now().UTC().Format(time.RFC3339Nano),
	})
}
