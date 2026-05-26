package domain

import "encoding/json"

// Canonical artifact names. The finisher pipeline writes these documents
// onto Project.Artifacts; gates read them back through the same names.
// Adding a new one is additive — never repurpose an existing constant.
const (
	ArtifactPlan         = "plan"
	ArtifactStack        = "stack"
	ArtifactScreenMap    = "screen_map"
	ArtifactDesignTokens = "design_tokens"
	// ArtifactMobileBuild is the per-iteration mobile build report
	// produced by MobileBuildGate. The document is a JSON array (one
	// entry per built target) shaped like:
	//   [{"platform":"android","profile":"debug",
	//     "artifact_path":"android/.../app-debug.apk",
	//     "size_bytes":12345678,
	//     "completed_at":"2026-05-26T00:00:00Z"}]
	// Downstream gates (MobileSize, MobileSecurity) read this to
	// decide what to enforce without re-running the build.
	ArtifactMobileBuild = "mobile_build"
	// ArtifactBundleAnalysis is the per-iteration JS bundle analyzer
	// report produced by BundleAnalyzerGate. The document is a JSON
	// object shaped like:
	//   {
	//     "topPackages":[{"name":"moment","sizeBytes":234567,"severity":"warning"}],
	//     "totalBytes": 1234567,
	//     "generatedAt": "2026-05-26T00:00:00Z"
	//   }
	// The cockpit reads this directly to render the bundle-size chart;
	// the gate's []Issue output drives the repair loop.
	ArtifactBundleAnalysis = "bundle_analysis"
)

// GetArtifact returns the raw JSON document stored under name, if any. A
// nil Artifacts map is treated as "no artifact" rather than a panic so
// callers can pass projects from any source without a guard.
func (p *Project) GetArtifact(name string) (json.RawMessage, bool) {
	if p == nil || p.Artifacts == nil {
		return nil, false
	}
	raw, ok := p.Artifacts[name]
	if !ok || len(raw) == 0 {
		return nil, false
	}
	return raw, true
}

// SetArtifact marshals v to JSON and stores it under name. The map is
// lazily allocated. Passing a json.RawMessage stores the bytes verbatim
// so we don't double-encode pre-marshalled documents.
func (p *Project) SetArtifact(name string, v any) error {
	if p == nil {
		return nil
	}
	var raw json.RawMessage
	switch t := v.(type) {
	case json.RawMessage:
		raw = append(raw, t...)
	case []byte:
		raw = append(raw, t...)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		raw = b
	}
	if p.Artifacts == nil {
		p.Artifacts = make(map[string]json.RawMessage)
	}
	p.Artifacts[name] = raw
	return nil
}
