// health.resolver.go projects the Code Health Dashboard
// (playbook §8.11, docs/ANTI_BLOAT_ENGINE.md) into the GraphQL surface.
//
// The resolver is tolerant: every data source is optional. Missing
// reports / nil stores yield empty slices and sentinel zero / -1
// values per health.go so the cockpit panels can render their
// "tool not wired" empty states without surfacing an error.
//
// Data sources (each optional):
//   - r.AtlasStore        → atlasCapabilityCount, lastIndexedAt
//   - r.ArchManifest      → architecture { layers, rules, cycles }
//   - r.AuditStore + r.LedgerSvc → reuseRate, locPerCapability (best-effort)
//   - r.HealthReportPaths → duplicationByDir, complexityHistogram,
//                            bundleByRoute, deadCodeCount, dependencyCycles

package resolver

import (
	"context"
	"encoding/json"
	"os"
	"sort"

	"ironflyer/core/orchestrator/internal/operations/graph/model"
)

// HealthDashboard is the resolver for the healthDashboard field. It
// composes Capability Atlas Stats, the architecture manifest, and the
// configured Anti-Bloat report files into a single HealthMetrics
// payload. The resolver requires an authenticated caller — Code
// Health is operator-facing today and never carries cross-tenant
// signal.
func (r *queryResolver) HealthDashboard(ctx context.Context) (*model.HealthMetrics, error) {
	if _, err := currentUser(ctx); err != nil {
		return nil, err
	}

	out := &model.HealthMetrics{
		ReuseRate:            -1,
		DedupRate:            -1,
		DeadCodeCount:        -1,
		DependencyCycles:     -1,
		ComplexityHistogram:  []model.ComplexityBucket{},
		DuplicationByDir:     []model.DirDup{},
		BundleByRoute:        []model.RouteBundle{},
		AtlasCapabilityCount: 0,
		LocPerCapability:     0,
		Architecture:         model.Architecture{Layers: []string{}, Rules: []model.ArchRule{}, Cycles: ""},
	}

	// Atlas snapshot — counts + last-indexed timestamp.
	if r.AtlasStore != nil {
		if stats, err := r.AtlasStore.Stats(ctx); err == nil {
			out.AtlasCapabilityCount = stats.Total
			if !stats.LastIndexed.IsZero() {
				t := stats.LastIndexed
				out.LastIndexedAt = &t
			}
		}
	}

	// Architecture manifest projection — empty layers means the
	// manifest was missing at boot.
	if len(r.ArchManifest.Layers) > 0 {
		layers := make([]string, len(r.ArchManifest.Layers))
		copy(layers, r.ArchManifest.Layers)
		out.Architecture.Layers = layers
	}
	if len(r.ArchManifest.Rules) > 0 {
		rules := make([]model.ArchRule, 0, len(r.ArchManifest.Rules))
		for _, rl := range r.ArchManifest.Rules {
			rules = append(rules, model.ArchRule{From: rl.From, To: rl.To, Allow: rl.Allow})
		}
		out.Architecture.Rules = rules
	}
	if r.ArchManifest.Cycles != "" {
		out.Architecture.Cycles = r.ArchManifest.Cycles
	}

	// Anti-Bloat report projections — each file is optional. Parse
	// failures degrade silently to the sentinel shape rather than
	// surfacing an error to the cockpit.
	if path := r.HealthReportPaths.Dedup; path != "" {
		if rate, dirs, ok := readDedupReport(path); ok {
			out.DedupRate = rate
			out.DuplicationByDir = dirs
		}
	}
	if path := r.HealthReportPaths.Deadcode; path != "" {
		if n, ok := readDeadcodeReport(path); ok {
			out.DeadCodeCount = n
		}
	}
	if path := r.HealthReportPaths.Complexity; path != "" {
		if hist, ok := readComplexityReport(path); ok {
			out.ComplexityHistogram = hist
		}
	}
	if path := r.HealthReportPaths.DepCycle; path != "" {
		if cycles, ok := readDepCycleReport(path); ok {
			out.DependencyCycles = cycles
		}
	}
	if path := r.HealthReportPaths.Bundle; path != "" {
		if routes, ok := readBundleReport(path); ok {
			out.BundleByRoute = routes
		}
	}

	return out, nil
}

// --- Report parsers ----------------------------------------------------
//
// Each parser accepts either the typed shape Ironflyer's Anti-Bloat
// gate writes (jscpd / knip / gocognit / dependency-cruiser /
// bundle-analyzer post-processed) or its raw upstream shape. We
// project into the dashboard model and ignore unknown fields so the
// resolver stays forward compatible.

// dedupReport mirrors the jscpd post-processed file:
//
//	{
//	  "rate": 0.0123,
//	  "directories": [{"directory":"core/orchestrator","dupPct":0.05,"files":42}]
//	}
type dedupReport struct {
	Rate        float64       `json:"rate"`
	Directories []dedupDirRow `json:"directories"`
}

type dedupDirRow struct {
	Directory string  `json:"directory"`
	DupPct    float64 `json:"dupPct"`
	Files     int     `json:"files"`
}

func readDedupReport(path string) (float64, []model.DirDup, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, nil, false
	}
	var rep dedupReport
	if err := json.Unmarshal(raw, &rep); err != nil {
		return 0, nil, false
	}
	out := make([]model.DirDup, 0, len(rep.Directories))
	for _, d := range rep.Directories {
		out = append(out, model.DirDup{Directory: d.Directory, DupPct: d.DupPct, Files: d.Files})
	}
	// Stable order so the cockpit doesn't flicker between requests.
	sort.SliceStable(out, func(i, j int) bool { return out[i].DupPct > out[j].DupPct })
	return rep.Rate, out, true
}

// deadcodeReport is a single { "count": N } payload (knip / ts-prune
// post-processed into a tally).
type deadcodeReport struct {
	Count int `json:"count"`
}

func readDeadcodeReport(path string) (int, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	var rep deadcodeReport
	if err := json.Unmarshal(raw, &rep); err != nil {
		return 0, false
	}
	return rep.Count, true
}

// complexityReport mirrors the gocognit / sonarjs post-processed file.
// The histogram is one of:
//   - {"bins":[...]} — five fixed buckets, 0..5 / 6..10 / 11..15 /
//     16..20 / 21+ (legacy shape used by health.go).
//   - {"histogram":[{"range":"0-5","count":42}, ...]} — explicit
//     per-bucket labels.
type complexityReport struct {
	Bins      []int                  `json:"bins,omitempty"`
	Histogram []complexityHistBucket `json:"histogram,omitempty"`
}

type complexityHistBucket struct {
	Range string `json:"range"`
	Count int    `json:"count"`
}

func readComplexityReport(path string) ([]model.ComplexityBucket, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var rep complexityReport
	if err := json.Unmarshal(raw, &rep); err != nil {
		return nil, false
	}
	if len(rep.Histogram) > 0 {
		out := make([]model.ComplexityBucket, 0, len(rep.Histogram))
		for _, b := range rep.Histogram {
			out = append(out, model.ComplexityBucket{Range: b.Range, Count: b.Count})
		}
		return out, true
	}
	if len(rep.Bins) > 0 {
		labels := []string{"0-5", "6-10", "11-15", "16-20", "20+"}
		out := make([]model.ComplexityBucket, 0, len(rep.Bins))
		for i, c := range rep.Bins {
			label := "20+"
			if i < len(labels) {
				label = labels[i]
			}
			out = append(out, model.ComplexityBucket{Range: label, Count: c})
		}
		return out, true
	}
	return []model.ComplexityBucket{}, true
}

// depCycleReport is the dependency-cruiser / madge post-processed
// cycle count.
type depCycleReport struct {
	Cycles int `json:"cycles"`
}

func readDepCycleReport(path string) (int, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	var rep depCycleReport
	if err := json.Unmarshal(raw, &rep); err != nil {
		return 0, false
	}
	return rep.Cycles, true
}

// bundleReport mirrors the size-limit + @next/bundle-analyzer
// post-processed file:
//
//	{"routes":[{"route":"/","totalKB":312.5,"firstLoadKB":120.4}]}
type bundleReport struct {
	Routes []bundleRouteRow `json:"routes"`
}

type bundleRouteRow struct {
	Route       string  `json:"route"`
	TotalKB     float64 `json:"totalKB"`
	FirstLoadKB float64 `json:"firstLoadKB"`
}

func readBundleReport(path string) ([]model.RouteBundle, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var rep bundleReport
	if err := json.Unmarshal(raw, &rep); err != nil {
		return nil, false
	}
	out := make([]model.RouteBundle, 0, len(rep.Routes))
	for _, r := range rep.Routes {
		out = append(out, model.RouteBundle{Route: r.Route, TotalKb: r.TotalKB, FirstLoadKb: r.FirstLoadKB})
	}
	return out, true
}
