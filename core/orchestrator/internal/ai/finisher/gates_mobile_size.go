package finisher

import (
	"context"
	"strings"

	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/ai/domain"
)

// MobileSizeBudgetGate enforces artifact size sizeBudgets against the
// per-iteration mobile build report produced by MobileBuildGate.
// Oversized APKs hurt install conversion; iOS .ipa over the cellular
// download cap (~200 MiB without WiFi) get rejected by the App Store.
//
// Budgets (per artifact kind):
//
//	APK debug   <= 100 MiB error    <= 60 MiB warn-free
//	APK release <= 50 MiB error     <= 30 MiB warn-free
//	AAB         <= 200 MiB error
//	IPA         <= 200 MiB error    <= 100 MiB warn-free
type MobileSizeBudgetGate struct{}

func (MobileSizeBudgetGate) Name() domain.GateName    { return domain.GateMobileSize }
func (MobileSizeBudgetGate) RepairAgent() agents.Role { return agents.RoleMobileCoder }

func (MobileSizeBudgetGate) Check(_ context.Context, env *GateEnv) []domain.Issue {
	if env == nil || env.Project == nil || !projectIsMobile(env.Project) {
		return nil
	}
	reports := readMobileBuildReports(env.Project)
	if len(reports) == 0 {
		// No build artifact recorded this iteration — nothing to weigh.
		// The MobileBuildGate (upstream) is responsible for failing the
		// "no artifact" case; the size gate stays dark.
		return nil
	}
	var issues []domain.Issue
	for _, rep := range reports {
		issues = append(issues, evaluateSizeBudget(rep)...)
	}
	return issues
}

// sizeBudget describes one size threshold pair. ErrorBytes triggers
// SeverityError, WarnBytes a SeverityWarning. WarnBytes == 0 means "no
// soft warning, only the hard cap".
type sizeBudget struct {
	Label      string
	ErrorBytes int64
	WarnBytes  int64
	Hint       string
}

// sizeBudgetFor picks the right ceiling pair for an artifact kind +
// profile. Returns ok=false when there's nothing to enforce.
func sizeBudgetFor(kind, profile string) (sizeBudget, bool) {
	const (
		mib = int64(1024 * 1024)
	)
	switch strings.ToLower(kind) {
	case "apk":
		if isReleaseProfile(profile) {
			return sizeBudget{
				Label:      "Android release APK",
				ErrorBytes: 50 * mib, WarnBytes: 30 * mib,
				Hint: "shrink the bundle: enable R8 + ProGuard, drop unused locales, ABI-split, audit assets/",
			}, true
		}
		return sizeBudget{
			Label:      "Android debug APK",
			ErrorBytes: 100 * mib, WarnBytes: 60 * mib,
			Hint: "debug builds carry symbol tables; release should be much smaller, but trim large unused assets now",
		}, true
	case "aab":
		return sizeBudget{
			Label:      "Android App Bundle",
			ErrorBytes: 200 * mib,
			Hint:       "AAB is the upload to Play Console — over 200 MiB triggers extra DAI configuration",
		}, true
	case "ipa":
		return sizeBudget{
			Label:      "iOS IPA",
			ErrorBytes: 200 * mib, WarnBytes: 100 * mib,
			Hint: "iOS App Store enforces a 200 MiB cellular-download cap; trim Swift symbols and on-demand resources",
		}, true
	case "app":
		// Simulator .app bundles aren't subject to store caps; we still
		// warn at 200 MiB so the cockpit shows when developer-build
		// bloat is climbing.
		return sizeBudget{
			Label:     "iOS simulator .app",
			WarnBytes: 200 * mib,
			Hint:      "simulator builds carry x86_64+arm64 slices and aren't store-bound, but bloated assets slow Appetize boots",
		}, true
	}
	return sizeBudget{}, false
}

func evaluateSizeBudget(rep mobileBuildArtifact) []domain.Issue {
	kind := inferArtifactKind(rep)
	if kind == "" {
		return nil
	}
	b, ok := sizeBudgetFor(kind, rep.Profile)
	if !ok {
		return nil
	}
	if rep.SizeBytes <= 0 {
		// Build gate didn't record a size — surface as info so the
		// dashboard can spot a degraded build report instead of
		// silently passing.
		return []domain.Issue{{
			Gate: domain.GateMobileSize, Severity: domain.SeverityInfo,
			Message: b.Label + " size unknown — build gate did not record size_bytes",
			Path:    rep.ArtifactPath,
		}}
	}
	if b.ErrorBytes > 0 && rep.SizeBytes > b.ErrorBytes {
		return []domain.Issue{{
			Gate: domain.GateMobileSize, Severity: domain.SeverityError,
			Message: b.Label + " is " + formatMiB(rep.SizeBytes) + " — exceeds the " +
				formatMiB(b.ErrorBytes) + " hard cap",
			Hint: b.Hint,
			Path: rep.ArtifactPath,
		}}
	}
	if b.WarnBytes > 0 && rep.SizeBytes > b.WarnBytes {
		return []domain.Issue{{
			Gate: domain.GateMobileSize, Severity: domain.SeverityWarning,
			Message: b.Label + " is " + formatMiB(rep.SizeBytes) + " — past the " +
				formatMiB(b.WarnBytes) + " warn-free zone",
			Hint: b.Hint,
			Path: rep.ArtifactPath,
		}}
	}
	return nil
}
