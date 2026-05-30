package resolver

// Helpers for the coverage resolvers. Kept OUT of coverage.resolver.go so a
// gqlgen regen (follow-schema) never evicts them into a //!!! WARNING block.

import (
	"encoding/json"
	"sort"
	"time"

	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/operations/graph/model"
)

// artifactCoverage mirrors the JSON the finisher's CoverageGate stores under
// domain.ArtifactCoverage. Declared locally so the resolver layer does not
// couple to the finisher's internal type.
type artifactCoverage struct {
	OverallPct  float64 `json:"overallPct"`
	MinPct      float64 `json:"minPct"`
	Tool        string  `json:"tool"`
	GeneratedAt *string `json:"generatedAt"`
	Files       []struct {
		Path      string  `json:"path"`
		LinePct   float64 `json:"linePct"`
		Uncovered int     `json:"uncovered"`
	} `json:"files"`
}

// coverageReportModel projects a project's coverage settings + latest stored
// report artifact onto the GraphQL model. The toggle + floor are read from
// Project.Settings (authoritative); the measured numbers come from the artifact
// the CoverageGate persisted on its last run (empty when never run).
func coverageReportModel(p domain.Project) *model.CoverageReport {
	out := &model.CoverageReport{
		ProjectID: p.ID,
		Enabled:   p.Settings.CoverageEnabled,
		MinPct:    p.Settings.CoverageMinPct,
		Files:     []model.FileCoverage{},
	}
	raw, ok := p.GetArtifact(domain.ArtifactCoverage)
	if !ok {
		return out
	}
	var a artifactCoverage
	if err := json.Unmarshal(raw, &a); err != nil {
		return out
	}
	out.OverallPct = a.OverallPct
	out.Tool = a.Tool
	if a.MinPct > 0 {
		out.MinPct = a.MinPct
	}
	if a.GeneratedAt != nil {
		if ts, err := time.Parse(time.RFC3339, *a.GeneratedAt); err == nil && !ts.IsZero() {
			out.GeneratedAt = &ts
		}
	}
	files := make([]model.FileCoverage, 0, len(a.Files))
	for _, f := range a.Files {
		files = append(files, model.FileCoverage{Path: f.Path, LinePct: f.LinePct, Uncovered: f.Uncovered})
	}
	// Least-covered first so "what is not closed" reads top-down.
	sort.SliceStable(files, func(i, j int) bool { return files[i].LinePct < files[j].LinePct })
	out.Files = files
	return out
}
