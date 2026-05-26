// health.resolver.go hosts the Anti-Bloat report parsers consumed by
// the HealthDashboard resolver in dashboards.resolver.go. The resolver
// method itself lives there; this file only exposes the file-format
// projection helpers.
//
// Each parser accepts the typed shape Ironflyer's Anti-Bloat gate
// writes (jscpd / knip / gocognit / dependency-cruiser / bundle-
// analyzer post-processed) and tolerates missing files + unknown
// fields so the dashboard renders empty-state panels rather than
// surfacing an error.

package resolver

import (
	"encoding/json"
	"os"
	"sort"

	"ironflyer/core/orchestrator/internal/operations/graph/model"
)

// dedupReport mirrors the jscpd post-processed file.
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
	sort.SliceStable(out, func(i, j int) bool { return out[i].DupPct > out[j].DupPct })
	return rep.Rate, out, true
}

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
