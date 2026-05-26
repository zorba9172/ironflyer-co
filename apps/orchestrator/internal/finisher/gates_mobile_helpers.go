package finisher

import (
	"encoding/json"
	"strings"

	"ironflyer/apps/orchestrator/internal/domain"
)

// mobileBuildArtifact is the per-target build report MobileBuildGate
// records into Project.Artifacts under ArtifactMobileBuild. The
// downstream gates (MobileSize, MobileSecurity, IOSPrivacyManifest)
// read it back to enforce post-build constraints without re-running
// the build themselves.
type mobileBuildArtifact struct {
	Platform     string `json:"platform"`     // "android" | "ios"
	Profile      string `json:"profile"`      // "debug" | "release" | "preview" | "production"
	ArtifactPath string `json:"artifact_path"`
	SizeBytes    int64  `json:"size_bytes"`
	CompletedAt  string `json:"completed_at"`
	// ArtifactType is set when the path itself doesn't carry the
	// extension we need (Appetize / EAS produce a folder-shaped
	// .app/.aab). One of "apk" | "aab" | "ipa" | "app".
	ArtifactType string `json:"artifact_type,omitempty"`
}

// readMobileBuildReports decodes the per-iteration mobile build
// artifact, returning nil when none was recorded. Each downstream
// mobile gate calls this — no caller should reach into
// p.GetArtifact directly.
func readMobileBuildReports(p *domain.Project) []mobileBuildArtifact {
	if p == nil {
		return nil
	}
	raw, ok := p.GetArtifact(domain.ArtifactMobileBuild)
	if !ok || len(raw) == 0 {
		return nil
	}
	// Tolerate both a single object and an array — early iterations
	// of MobileBuildGate may write a singleton.
	var arr []mobileBuildArtifact
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return arr
	}
	var one mobileBuildArtifact
	if err := json.Unmarshal(raw, &one); err == nil && one.ArtifactPath != "" {
		return []mobileBuildArtifact{one}
	}
	return nil
}

// inferArtifactKind picks "apk" | "aab" | "ipa" | "app" from the
// report's explicit type or the path extension. Returns "" when nothing
// matches — caller should skip size enforcement in that case.
func inferArtifactKind(rep mobileBuildArtifact) string {
	if t := strings.ToLower(strings.TrimSpace(rep.ArtifactType)); t != "" {
		return t
	}
	low := strings.ToLower(rep.ArtifactPath)
	switch {
	case strings.HasSuffix(low, ".apk"):
		return "apk"
	case strings.HasSuffix(low, ".aab"):
		return "aab"
	case strings.HasSuffix(low, ".ipa"):
		return "ipa"
	case strings.HasSuffix(low, ".app") || strings.HasSuffix(low, ".app/"):
		return "app"
	}
	return ""
}

// isReleaseProfile returns true when the mobile build profile is a
// production-grade release. Both "release" (Gradle) and "production"
// (EAS) qualify.
func isReleaseProfile(profile string) bool {
	p := strings.ToLower(strings.TrimSpace(profile))
	return p == "release" || p == "production"
}

// projectMobileKind extracts the StackDecision.Mobile.Kind without
// touching nil pointers — every downstream mobile gate calls this
// before doing anything Kind-specific.
func projectMobileKind(p *domain.Project) domain.MobileKind {
	if p == nil {
		return domain.MobileKindNone
	}
	return p.Spec.Stack.Mobile.Kind
}

// projectIsMobile is the single guard every mobile-additive gate uses
// up-front to short-circuit web-only projects.
func projectIsMobile(p *domain.Project) bool {
	if p == nil {
		return false
	}
	return p.Spec.Stack.IsMobile()
}

// fileBodyAny returns the contents of the first file whose path matches
// any of the supplied predicates. Used by the privacy-manifest +
// security gates to locate config files at unknown depths.
func fileBodyAny(p *domain.Project, predicate func(path string) bool) (string, string, bool) {
	if p == nil {
		return "", "", false
	}
	for _, f := range p.Files {
		if predicate(f.Path) {
			return f.Path, f.Content, true
		}
	}
	return "", "", false
}

// iterFiles is the shared "walk every project file" helper. Inline-able
// at every call site but central here keeps callers shorter.
func iterFiles(p *domain.Project, fn func(path, content string) bool) {
	if p == nil {
		return
	}
	for _, f := range p.Files {
		if !fn(f.Path, f.Content) {
			return
		}
	}
}
