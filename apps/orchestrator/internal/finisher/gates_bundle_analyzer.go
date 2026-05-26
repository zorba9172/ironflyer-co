package finisher

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"ironflyer/apps/orchestrator/internal/agents"
	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/runtime"
)

// BundleAnalyzerGate runs the JS bundle analyzer for mobile projects
// and flags packages over a per-package size budget. The differentiator
// is opinionated repair hints: when moment.js or full lodash shows up
// on the top-50, we point the Coder at the well-known lighter
// alternative instead of forcing the agent to re-derive it.
//
// Grace path: when react-native-bundle-visualizer isn't installed the
// gate emits a SeverityInfo nudge and passes — bundle analysis is a
// quality enforcement layer, not a hard build prerequisite.
type BundleAnalyzerGate struct{}

func (BundleAnalyzerGate) Name() domain.GateName    { return domain.GateMobileBundleAnalyzer }
func (BundleAnalyzerGate) RepairAgent() agents.Role { return agents.RoleMobileCoder }

// Per-package gzipped thresholds. The 500KB warn line catches "you
// imported the whole library when a single helper would do" mistakes;
// 1MB is the "this is going to dominate the cold-start" hard error.
const (
	bundleWarnBytes  int64 = 500 * 1024
	bundleErrorBytes int64 = 1024 * 1024
)

// bundleAlternatives maps a common offender package name to an
// opinionated repair hint. Keys are matched case-insensitively against
// the package name reported by react-native-bundle-visualizer.
var bundleAlternatives = map[string]string{
	"moment":             "date-fns or dayjs (90% smaller)",
	"lodash":             "lodash-es with named imports, or radash",
	"react-icons":        "@expo/vector-icons or per-icon imports",
	"@material-ui/icons": "individual icon imports",
	"axios":              "fetch (built-in) or ky",
	"rxjs":               "AsyncIterator + plain promises",
	"validator":          "zod (used by tsc) or yup",
	"core-js":            "verify polyfill is needed — RN 0.74+ has wide support",
}

// BundleAnalysisReport is the flat, dashboard-friendly summary the gate
// writes to ArtifactBundleAnalysis. One entry per "top package" so the
// cockpit can render a stacked bar without re-parsing the visualizer's
// tree shape.
type BundleAnalysisReport struct {
	TopPackages []BundleAnalysisPackage `json:"topPackages"`
	TotalBytes  int64                   `json:"totalBytes"`
	GeneratedAt time.Time               `json:"generatedAt"`
}

// BundleAnalysisPackage is a single entry on the top-N list. Severity
// mirrors the gate verdict so the cockpit chip can render the same
// colour without re-deriving it from sizeBytes.
type BundleAnalysisPackage struct {
	Name      string `json:"name"`
	SizeBytes int64  `json:"sizeBytes"`
	Severity  string `json:"severity"` // info | warning | error
	Hint      string `json:"hint,omitempty"`
}

func (BundleAnalyzerGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	if env == nil || env.Project == nil {
		return nil
	}
	p := env.Project
	if !projectIsMobile(p) {
		return nil
	}
	// Gate is build-output dependent — without a mobile build artifact
	// there's nothing to weigh, mirror the MobileSizeBudgetGate stance.
	if len(readMobileBuildReports(p)) == 0 {
		return nil
	}

	if !env.HasRuntime() {
		// Static fallback: parse package.json for known offenders so
		// the gate produces *some* signal on cold starts.
		issues, report := staticBundleAnalysis(p)
		if report != nil {
			_ = p.SetArtifact(domain.ArtifactBundleAnalysis, report)
		}
		return issues
	}

	// Probe for the visualizer binary first — a graceful nudge beats a
	// noisy ENOENT in the gate verdict.
	if !hasBundleVisualizer(ctx, env) {
		return []domain.Issue{{
			Gate: domain.GateMobileBundleAnalyzer, Severity: domain.SeverityInfo,
			Message: "install react-native-bundle-visualizer for bundle-size enforcement",
			Hint:    "npm i -D react-native-bundle-visualizer — then this gate measures top packages and flags > 500KB",
		}}
	}

	rep, err := runBundleVisualizer(ctx, env)
	if err != nil || rep == nil {
		return []domain.Issue{{
			Gate: domain.GateMobileBundleAnalyzer, Severity: domain.SeverityInfo,
			Message: "bundle visualizer did not produce a parseable report",
			Hint:    "rerun once the next build artifact lands, or check workspace logs",
		}}
	}

	issues := evaluateBundleReport(rep)
	if err := p.SetArtifact(domain.ArtifactBundleAnalysis, rep); err != nil {
		// The artifact write is best-effort — the gate's verdict still
		// stands without the dashboard mirror.
		_ = err
	}
	return issues
}

// hasBundleVisualizer probes for react-native-bundle-visualizer via a
// `command -v` lookup. We deliberately don't exec the tool yet — that
// would spend the workspace's bundle minutes for a probe.
func hasBundleVisualizer(ctx context.Context, env *GateEnv) bool {
	res, err := env.Runtime.Exec(ctx, env.UserBearer, env.WorkspaceID, runtime.ExecOpts{
		Shell:          "command -v react-native-bundle-visualizer >/dev/null 2>&1 || ls node_modules/react-native-bundle-visualizer >/dev/null 2>&1",
		TimeoutSeconds: 10,
	})
	if err != nil || res.TimedOut {
		return false
	}
	return res.ExitCode == 0
}

// runBundleVisualizer executes react-native-bundle-visualizer with the
// JSON output flag, then decodes the resulting tree into a flat
// BundleAnalysisReport. The visualizer ships a tree of modules with a
// `gzipSize` / `size` per node; we aggregate by top-level npm package
// and keep the largest 50 for the cockpit.
func runBundleVisualizer(ctx context.Context, env *GateEnv) (*BundleAnalysisReport, error) {
	cmd := "npx --yes react-native-bundle-visualizer --format=json --out=/tmp/ironflyer-bundle.json --no-open 2>&1 && cat /tmp/ironflyer-bundle.json"
	res, err := env.Runtime.Exec(ctx, env.UserBearer, env.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 300,
	})
	if err != nil || res.TimedOut || res.ExitCode != 0 {
		return nil, err
	}
	body := extractTrailingJSON(res.Stdout)
	if body == "" {
		return nil, nil
	}
	var tree visualizerNode
	if err := json.Unmarshal([]byte(body), &tree); err != nil {
		return nil, err
	}
	packages := map[string]int64{}
	var total int64
	walkVisualizer(&tree, packages, &total)

	top := topPackages(packages, 50)
	out := &BundleAnalysisReport{
		TopPackages: make([]BundleAnalysisPackage, 0, len(top)),
		TotalBytes:  total,
		GeneratedAt: time.Now().UTC(),
	}
	for _, pkg := range top {
		sev, hint := classifyPackage(pkg.name, pkg.size)
		out.TopPackages = append(out.TopPackages, BundleAnalysisPackage{
			Name:      pkg.name,
			SizeBytes: pkg.size,
			Severity:  sev,
			Hint:      hint,
		})
	}
	return out, nil
}

// visualizerNode mirrors the bundle-visualizer JSON tree. The schema
// is intentionally permissive — versions differ in field naming, so we
// accept both `gzipSize` and `size`.
type visualizerNode struct {
	Name     string           `json:"name"`
	Path     string           `json:"path"`
	GzipSize int64            `json:"gzipSize"`
	Size     int64            `json:"size"`
	Children []visualizerNode `json:"children"`
}

func walkVisualizer(n *visualizerNode, packages map[string]int64, total *int64) {
	if n == nil {
		return
	}
	sz := n.GzipSize
	if sz == 0 {
		sz = n.Size
	}
	if len(n.Children) == 0 && sz > 0 {
		pkg := packageFromPath(n.Path)
		if pkg == "" {
			pkg = n.Name
		}
		if pkg != "" {
			packages[pkg] += sz
			*total += sz
		}
	}
	for i := range n.Children {
		walkVisualizer(&n.Children[i], packages, total)
	}
}

// packageFromPath extracts the npm package name from a node module
// path: "node_modules/moment/locale/x.js" -> "moment";
// "node_modules/@scope/pkg/index.js" -> "@scope/pkg".
func packageFromPath(p string) string {
	idx := strings.LastIndex(p, "node_modules/")
	if idx < 0 {
		return ""
	}
	rest := p[idx+len("node_modules/"):]
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) == 0 {
		return ""
	}
	if strings.HasPrefix(parts[0], "@") && len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

type packageSize struct {
	name string
	size int64
}

func topPackages(m map[string]int64, n int) []packageSize {
	out := make([]packageSize, 0, len(m))
	for k, v := range m {
		out = append(out, packageSize{name: k, size: v})
	}
	// O(n^2) selection sort capped at n — fine for top-50 from a list
	// of ~hundreds; avoids pulling sort.Slice + a comparator closure.
	if n > len(out) {
		n = len(out)
	}
	for i := 0; i < n; i++ {
		best := i
		for j := i + 1; j < len(out); j++ {
			if out[j].size > out[best].size {
				best = j
			}
		}
		out[i], out[best] = out[best], out[i]
	}
	if n < len(out) {
		out = out[:n]
	}
	return out
}

// classifyPackage returns (severity, hint) for a top-package entry.
// SeverityError above 1MB; SeverityWarning above 500KB; SeverityInfo
// otherwise. Hint pulls from bundleAlternatives when the package name
// matches a known offender.
func classifyPackage(name string, sz int64) (string, string) {
	severity := "info"
	switch {
	case sz > bundleErrorBytes:
		severity = "error"
	case sz > bundleWarnBytes:
		severity = "warning"
	}
	hint := ""
	if alt, ok := bundleAlternatives[strings.ToLower(name)]; ok {
		hint = "consider " + alt
	} else if severity == "warning" {
		hint = "tree-shake the import, lazy-load via React.lazy, or replace with a lighter alternative"
	} else if severity == "error" {
		hint = "this package alone exceeds 1MB gzipped — split or replace before shipping"
	}
	return severity, hint
}

// evaluateBundleReport turns the flat report into the gate's []Issue
// output. Only the top 5 packages get an issue raised; below that the
// dashboard surfaces them via the artifact but we don't drown the
// repair Coder with low-signal warnings.
func evaluateBundleReport(rep *BundleAnalysisReport) []domain.Issue {
	if rep == nil {
		return nil
	}
	var issues []domain.Issue
	for i, pkg := range rep.TopPackages {
		if i >= 5 {
			break
		}
		switch pkg.Severity {
		case "error":
			issues = append(issues, domain.Issue{
				Gate: domain.GateMobileBundleAnalyzer, Severity: domain.SeverityError,
				Message: "package " + pkg.Name + " is " + formatKiB(pkg.SizeBytes) +
					" gzipped — exceeds the 1MB per-package budget",
				Hint: pkg.Hint,
			})
		case "warning":
			issues = append(issues, domain.Issue{
				Gate: domain.GateMobileBundleAnalyzer, Severity: domain.SeverityWarning,
				Message: "package " + pkg.Name + " contributes " + formatKiB(pkg.SizeBytes) +
					" to the bundle; consider tree-shaking, lazy-loading via React.lazy, or replacing with a lighter alternative",
				Hint: pkg.Hint,
			})
		}
	}
	return issues
}

// staticBundleAnalysis is the runtime-less fallback. We can't measure
// the real bundle, but we can scan package.json dependencies for known
// offenders so the cockpit still has *some* signal pre-build.
func staticBundleAnalysis(p *domain.Project) ([]domain.Issue, *BundleAnalysisReport) {
	body, ok := fileBody(p, "package.json")
	if !ok {
		return nil, nil
	}
	var doc struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		return nil, nil
	}
	merged := map[string]struct{}{}
	for k := range doc.Dependencies {
		merged[k] = struct{}{}
	}
	for k := range doc.DevDependencies {
		merged[k] = struct{}{}
	}
	var issues []domain.Issue
	report := &BundleAnalysisReport{GeneratedAt: time.Now().UTC()}
	for name := range merged {
		alt, known := bundleAlternatives[strings.ToLower(name)]
		if !known {
			continue
		}
		report.TopPackages = append(report.TopPackages, BundleAnalysisPackage{
			Name:     name,
			Severity: "warning",
			Hint:     "consider " + alt,
		})
		issues = append(issues, domain.Issue{
			Gate: domain.GateMobileBundleAnalyzer, Severity: domain.SeverityWarning,
			Message: "dependency " + name + " is a known bundle-size offender (static analysis)",
			Hint:    "consider " + alt + " — runtime gate will measure actual size once a workspace is attached",
		})
	}
	if len(report.TopPackages) == 0 {
		return nil, nil
	}
	return issues, report
}

// extractTrailingJSON walks the stdout backwards looking for the last
// balanced { ... } block — the visualizer prepends progress logs before
// the JSON payload, so a straight Unmarshal of the whole stdout fails.
func extractTrailingJSON(stdout string) string {
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return ""
	}
	// Find the last "{" that has a matching "}" at the end.
	end := strings.LastIndex(stdout, "}")
	if end < 0 {
		return ""
	}
	depth := 0
	for i := end; i >= 0; i-- {
		switch stdout[i] {
		case '}':
			depth++
		case '{':
			depth--
			if depth == 0 {
				return stdout[i : end+1]
			}
		}
	}
	return ""
}

// formatKiB renders a byte count as e.g. "612 KiB" or "1.4 MiB" for
// the gate's []Issue messages.
func formatKiB(n int64) string {
	const (
		kib = 1024
		mib = 1024 * 1024
	)
	if n >= mib {
		whole := n / mib
		frac := (n % mib) * 10 / mib
		return itoaPositive(int(whole)) + "." + itoaPositive(int(frac)) + " MiB"
	}
	return itoaPositive(int(n/kib)) + " KiB"
}
