package finisher

import (
	"context"
	"encoding/json"
	"strings"

	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/operations/appsec"
	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/operations/runtime"
)

// GateEnv carries the per-run environment a Gate needs beyond the project
// itself: an optional runtime client + workspace binding so gates that want
// to execute real build/test commands can do so when a workspace exists.
// Every field except Project may be zero-valued; gates must degrade
// gracefully (e.g. fall back to static checks) when Runtime is nil.
type GateEnv struct {
	Project     *domain.Project
	Runtime     *runtime.Client
	WorkspaceID string
	UserBearer  string
	// Budget is an optional snapshot of the project's billing posture at the
	// start of this gate iteration. Set by the Engine when a BudgetSource is
	// wired; nil otherwise. The Budget gate is the primary consumer.
	Budget *BudgetSnapshot
	// DeployURL is the publicly-reachable preview/production URL for the
	// most recent successful deploy of this project. The LighthouseGate is
	// the primary consumer — it forwards the URL to the PageSpeed Insights
	// API. Empty when no deploy has produced a public URL yet (the gate
	// degrades to a no-op rather than blocking). The gate also falls back
	// to the IRONFLYER_LIGHTHOUSE_TARGET_URL env var when this field is
	// empty, so self-hosted operators can wire one without a deploy store.
	DeployURL string
}

// BudgetSnapshot is the projected billing posture for a project at gate
// evaluation time. CapUSD is the user's plan cap; SpentUSD is what has
// already landed in the ledger for this project; RemainingUSD = CapUSD -
// SpentUSD. HardStop mirrors the plan's HardStop flag — when true, an
// overage MUST block deploy; when false, it warns only.
type BudgetSnapshot struct {
	CapUSD       float64
	SpentUSD     float64
	RemainingUSD float64
	HardStop     bool
	// Reason explains how the snapshot was produced. Used for issue text on
	// the Budget gate, e.g. "no budget source configured".
	Reason string
}

// HasRuntime is the nil-safe predicate gates call before reaching for Exec.
func (e *GateEnv) HasRuntime() bool {
	return e != nil && e.Runtime.Enabled() && e.WorkspaceID != ""
}

// Gate is the contract every completion gate implements. Each gate inspects
// the project (and optionally executes inside the bound workspace) and
// reports issues. When issues are non-empty the orchestrator dispatches the
// gate's repair agent.
type Gate interface {
	Name() domain.GateName
	RepairAgent() agents.Role
	Check(ctx context.Context, env *GateEnv) []domain.Issue
}

type SpecGate struct{}

func (SpecGate) Name() domain.GateName    { return domain.GateSpec }
func (SpecGate) RepairAgent() agents.Role { return agents.RolePlanner }
func (SpecGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	p := env.Project
	var issues []domain.Issue
	if strings.TrimSpace(p.Spec.Idea) == "" {
		issues = append(issues, domain.Issue{Gate: domain.GateSpec, Severity: domain.SeverityError, Message: "spec.idea is empty"})
	}
	if len(p.Spec.UserStories) == 0 {
		issues = append(issues, domain.Issue{Gate: domain.GateSpec, Severity: domain.SeverityError, Message: "no user stories"})
	}
	for _, s := range p.Spec.UserStories {
		if len(s.Acceptance) == 0 {
			issues = append(issues, domain.Issue{Gate: domain.GateSpec, Severity: domain.SeverityWarning, Message: "story missing acceptance: " + s.ID})
		}
	}
	if len(p.Spec.DataModel) == 0 {
		issues = append(issues, domain.Issue{Gate: domain.GateSpec, Severity: domain.SeverityWarning, Message: "data model not sketched"})
	}
	// Acceptance criteria validation — for each criterion, verify the
	// workspace contains code that plausibly addresses it. Unaddressed
	// criteria fail; addressed-but-untested criteria warn.
	issues = append(issues, validateAcceptanceCriteria(ctx, env)...)
	return issues
}

type UXGate struct{}

func (UXGate) Name() domain.GateName    { return domain.GateUX }
func (UXGate) RepairAgent() agents.Role { return agents.RoleUXer }

// knownUXComponents is the canonical set of MUI-aligned components a
// generated screen map may reference. Anything outside this set fails the
// gate so the UXer agent must use the design system instead of inventing
// bespoke widgets. Lower-cased on lookup.
var knownUXComponents = map[string]struct{}{
	"appshell": {}, "topbar": {}, "sidebar": {}, "navrail": {}, "drawer": {},
	"page": {}, "section": {}, "card": {}, "list": {}, "listitem": {},
	"table": {}, "datatable": {}, "form": {}, "field": {}, "input": {},
	"textfield": {}, "select": {}, "switch": {}, "checkbox": {}, "radio": {},
	"button": {}, "iconbutton": {}, "link": {}, "menu": {}, "modal": {},
	"dialog": {}, "drawerpanel": {}, "tabs": {}, "tab": {}, "stepper": {},
	"breadcrumbs": {}, "snackbar": {}, "alert": {}, "toast": {}, "tooltip": {},
	"avatar": {}, "badge": {}, "chip": {}, "tag": {}, "divider": {},
	"empty": {}, "loader": {}, "skeleton": {}, "spinner": {}, "progress": {},
	"chart": {}, "map": {}, "metric": {}, "kpi": {}, "stat": {},
	"hero": {}, "feature": {}, "footer": {}, "header": {}, "search": {},
	"filter": {}, "pagination": {}, "uploader": {}, "datepicker": {}, "timepicker": {},
}

// screenMapDoc is the on-disk schema for .ironflyer/screen_map.json. It is
// produced by the UXer agent and consumed by the Coder + Reviewer.
type screenMapDoc struct {
	Screens []struct {
		ID         string   `json:"id"`
		Name       string   `json:"name"`
		Route      string   `json:"route"`
		Components []string `json:"components"`
		StoryIDs   []string `json:"storyIds,omitempty"`
	} `json:"screens"`
}

// designTokensDoc is the minimum shape every project's tokens file must
// expose. We do not enforce specific palette values — only that the
// declared categories exist and are non-empty.
type designTokensDoc struct {
	Color   map[string]any `json:"color"`
	Spacing map[string]any `json:"spacing"`
	Type    map[string]any `json:"type,omitempty"`
}

const (
	screenMapPath    = ".ironflyer/screen_map.json"
	designTokensPath = ".ironflyer/design_tokens.json"
)

func (UXGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	p := env.Project
	if len(p.Spec.UserStories) == 0 {
		return []domain.Issue{{
			Gate: domain.GateUX, Severity: domain.SeverityWarning,
			Message: "blocked: spec has no stories",
			Hint:    "fix the Spec gate first — UX is downstream of stories",
		}}
	}

	var issues []domain.Issue

	mapRaw, mapOK := fileBody(p, screenMapPath)
	if !mapOK {
		issues = append(issues, domain.Issue{
			Gate: domain.GateUX, Severity: domain.SeverityError,
			Message: "missing screen map", Path: screenMapPath,
			Hint: "UXer must publish " + screenMapPath + " with at least one screen",
		})
	} else {
		var doc screenMapDoc
		if err := json.Unmarshal([]byte(mapRaw), &doc); err != nil {
			issues = append(issues, domain.Issue{
				Gate: domain.GateUX, Severity: domain.SeverityError,
				Message: "screen map is not valid JSON: " + err.Error(),
				Path:    screenMapPath,
			})
		} else if len(doc.Screens) == 0 {
			issues = append(issues, domain.Issue{
				Gate: domain.GateUX, Severity: domain.SeverityError,
				Message: "screen map has zero screens", Path: screenMapPath,
			})
		} else {
			storyIDs := map[string]struct{}{}
			for _, s := range p.Spec.UserStories {
				storyIDs[s.ID] = struct{}{}
			}
			for _, sc := range doc.Screens {
				if strings.TrimSpace(sc.ID) == "" || strings.TrimSpace(sc.Name) == "" {
					issues = append(issues, domain.Issue{
						Gate: domain.GateUX, Severity: domain.SeverityError,
						Message: "screen missing id or name", Path: screenMapPath,
					})
					continue
				}
				if len(sc.Components) == 0 {
					issues = append(issues, domain.Issue{
						Gate: domain.GateUX, Severity: domain.SeverityError,
						Message: "screen " + sc.ID + " has no components", Path: screenMapPath,
					})
				}
				for _, c := range sc.Components {
					if _, ok := knownUXComponents[strings.ToLower(strings.TrimSpace(c))]; !ok {
						issues = append(issues, domain.Issue{
							Gate: domain.GateUX, Severity: domain.SeverityWarning,
							Message: "unknown component on " + sc.ID + ": " + c,
							Path:    screenMapPath,
							Hint:    "use a component from the Ironflyer/MUI design system",
						})
					}
				}
				for _, sid := range sc.StoryIDs {
					if _, ok := storyIDs[sid]; !ok {
						issues = append(issues, domain.Issue{
							Gate: domain.GateUX, Severity: domain.SeverityWarning,
							Message: "screen " + sc.ID + " references unknown storyId: " + sid,
							Path:    screenMapPath,
						})
					}
				}
			}
		}
	}

	tokRaw, tokOK := fileBody(p, designTokensPath)
	if !tokOK {
		issues = append(issues, domain.Issue{
			Gate: domain.GateUX, Severity: domain.SeverityError,
			Message: "missing design tokens", Path: designTokensPath,
			Hint: "publish a tokens file with color + spacing (typography optional)",
		})
	} else {
		var tok designTokensDoc
		if err := json.Unmarshal([]byte(tokRaw), &tok); err != nil {
			issues = append(issues, domain.Issue{
				Gate: domain.GateUX, Severity: domain.SeverityError,
				Message: "design tokens not valid JSON: " + err.Error(),
				Path:    designTokensPath,
			})
		} else {
			if len(tok.Color) == 0 {
				issues = append(issues, domain.Issue{
					Gate: domain.GateUX, Severity: domain.SeverityError,
					Message: "design tokens has no color palette", Path: designTokensPath,
				})
			}
			if len(tok.Spacing) == 0 {
				issues = append(issues, domain.Issue{
					Gate: domain.GateUX, Severity: domain.SeverityError,
					Message: "design tokens has no spacing scale", Path: designTokensPath,
				})
			}
		}
	}

	// Pixel-perfect contract — when the project has VisualTargets we
	// screenshot the live preview and refuse to pass the gate until the
	// rendered output matches within tolerance. This is what turns
	// Lovable's "Visual Edits" into a blocking gate: drift is no longer
	// "show me the diff and hope I notice", it's "ship-blocking until
	// you fix it".
	issues = append(issues, visualTargetChecks(ctx, env)...)

	return issues
}

// visualTargetChecks runs the per-VisualTarget diff against the live
// preview. Returns one Issue per failing target, plus aggregated stats
// in the Hint so the UI / Coder repair prompt sees the full picture
// (diff ratio, mean color delta, perceptual hash distance, base64 diff
// overlay).
//
// Best-effort: when the runtime is absent or no targets are configured,
// we return zero issues so the gate degrades cleanly on dev/self-hosted
// setups that don't have headless chromium wired.
func visualTargetChecks(ctx context.Context, env *GateEnv) []domain.Issue {
	if env == nil || env.Project == nil || len(env.Project.VisualTargets) == 0 {
		return nil
	}
	if !env.HasRuntime() {
		// No runtime → no screenshot → can't enforce.  Surface as a
		// warning so the dashboard shows "visual contract pending" instead
		// of silently passing.
		return []domain.Issue{{
			Gate: domain.GateUX, Severity: domain.SeverityWarning,
			Message: "visual targets configured but runtime is unavailable — pixel-perfect gate is dark",
			Hint:    "start a workspace and re-run; the live preview is required for screenshot diff",
		}}
	}
	var issues []domain.Issue
	for _, t := range env.Project.VisualTargets {
		issues = append(issues, runOneVisualTarget(ctx, env, t)...)
	}
	return issues
}

// runOneVisualTarget captures the live preview matching the target's
// viewport + route and runs CompareScreenshots. A Tolerance of 0 keeps
// the default at 0.02 so the gate isn't a trip-wire on AA flicker.
func runOneVisualTarget(ctx context.Context, env *GateEnv, target domain.VisualTarget) []domain.Issue {
	tol := target.Tolerance
	if tol <= 0 {
		tol = 0.02
	}
	currentB64, err := captureLivePreview(ctx, env, target)
	if err != nil {
		return []domain.Issue{{
			Gate: domain.GateUX, Severity: domain.SeverityWarning,
			Message: "visual target '" + visualTargetLabel(target) + "': could not screenshot preview: " + err.Error(),
			Hint:    "is the dev server running on a forwarded port? does the workspace include chromium-headless?",
		}}
	}
	diff, err := CompareScreenshots(target.ImagePNGBase64, currentB64)
	if err != nil {
		return []domain.Issue{{
			Gate: domain.GateUX, Severity: domain.SeverityError,
			Message: "visual target '" + visualTargetLabel(target) + "': diff failed: " + err.Error(),
		}}
	}
	if diff.DiffRatio <= tol && !diff.SizeMismatch {
		// Within the official tolerance but check for >5% structural drift
		// — a target that's "passing" yet drifted half a percent past the
		// 5% structural-drift mark deserves a warn so the dashboard chip
		// stays honest. Below 5%, return nil (clean).
		if diff.DiffRatio > 0.05 {
			return []domain.Issue{{
				Gate: domain.GateUX, Severity: domain.SeverityWarning,
				Message: "visual target '" + visualTargetLabel(target) +
					"' drifted >5% (within tolerance but worth a review)",
				Hint: visualDiffHint(target, diff, tol),
			}}
		}
		return nil // matched within tolerance and below the structural-drift threshold
	}
	// Encode a structured hint the Coder repair prompt can read.
	hint := visualDiffHint(target, diff, tol)
	return []domain.Issue{{
		Gate: domain.GateUX, Severity: domain.SeverityError,
		Message: "visual target '" + visualTargetLabel(target) + "' does not match preview within tolerance",
		Hint:    hint,
	}}
}

func visualTargetLabel(t domain.VisualTarget) string {
	if t.Name != "" {
		return t.Name
	}
	if t.RouteHint != "" {
		return t.RouteHint
	}
	if t.ID != "" {
		return t.ID
	}
	return "target"
}

// visualDiffHint returns a compact human + machine readable summary so
// the repair Coder sees what changed without the full overlay bytes
// drowning the prompt. The diff overlay itself is shipped to the Coder
// as a vision attachment by the recovery loop, not via this hint.
func visualDiffHint(t domain.VisualTarget, d VisualDiffResult, tol float64) string {
	var b strings.Builder
	b.WriteString("route=")
	if t.RouteHint != "" {
		b.WriteString(t.RouteHint)
	} else {
		b.WriteString("/")
	}
	if t.ViewportW > 0 && t.ViewportH > 0 {
		b.WriteString(" viewport=")
		b.WriteString(itoaPositive(t.ViewportW))
		b.WriteString("x")
		b.WriteString(itoaPositive(t.ViewportH))
	}
	if d.SizeMismatch {
		b.WriteString(" size_mismatch=true")
	}
	b.WriteString(" diff_ratio=")
	b.WriteString(formatRatio(d.DiffRatio))
	b.WriteString(" tolerance=")
	b.WriteString(formatRatio(tol))
	b.WriteString(" within_tolerance=")
	if d.DiffRatio <= tol && !d.SizeMismatch {
		b.WriteString("true")
	} else {
		b.WriteString("false")
	}
	b.WriteString(" mean_delta=")
	b.WriteString(formatRatio(d.MeanColorDelta))
	b.WriteString(" phash_distance=")
	b.WriteString(itoaPositive(d.PerceptualHashDistance))
	return b.String()
}

// VisualReport is the structured per-target diff summary surfaced
// alongside the gate's []Issue output. Callers (GraphQL resolvers, the
// dashboard) consume this to draw the visual drift panel without
// re-parsing the Issue.Hint string. One report per VisualTarget.
type VisualReport struct {
	TargetID        string  `json:"targetId"`
	Label           string  `json:"label,omitempty"`
	Route           string  `json:"route,omitempty"`
	Ratio           float64 `json:"ratio"`
	MeanDelta       float64 `json:"meanDelta"`
	PHashDist       int     `json:"phashDist"`
	WithinTolerance bool    `json:"withinTolerance"`
	SizeMismatch    bool    `json:"sizeMismatch,omitempty"`
	StructuralDrift bool    `json:"structuralDrift,omitempty"` // ratio > 0.05
}

// VisualReportFor returns a structured VisualReport for one target/diff
// pair so callers can render the per-target chip without parsing the
// free-form hint. Pure function; no runtime calls.
func VisualReportFor(t domain.VisualTarget, d VisualDiffResult, tol float64) VisualReport {
	return VisualReport{
		TargetID:        t.ID,
		Label:           visualTargetLabel(t),
		Route:           t.RouteHint,
		Ratio:           d.DiffRatio,
		MeanDelta:       d.MeanColorDelta,
		PHashDist:       d.PerceptualHashDistance,
		WithinTolerance: !d.SizeMismatch && d.DiffRatio <= tol,
		SizeMismatch:    d.SizeMismatch,
		StructuralDrift: d.DiffRatio > 0.05,
	}
}

// formatRatio renders 0..1 as e.g. "0.034". Two-decimal precision is
// enough for gate messages.
func formatRatio(v float64) string {
	if v < 0 {
		v = 0
	}
	thousandths := int(v*1000 + 0.5)
	whole := thousandths / 1000
	frac := thousandths % 1000
	s := itoaPositive(whole) + "."
	if frac < 10 {
		s += "00"
	} else if frac < 100 {
		s += "0"
	}
	s += itoaPositive(frac)
	return s
}

// captureLivePreview shells into the workspace, finds the first running
// preview port the gate-time HasRuntime() guard already validated, then
// asks the runtime to screenshot route+viewport. The runtime endpoint
// uses chromium-headless when available and falls back to a stub.
func captureLivePreview(ctx context.Context, env *GateEnv, target domain.VisualTarget) (string, error) {
	w := target.ViewportW
	if w <= 0 {
		w = 1280
	}
	h := target.ViewportH
	if h <= 0 {
		h = 800
	}
	route := target.RouteHint
	if route == "" {
		route = "/"
	}
	return env.Runtime.Screenshot(ctx, env.UserBearer, env.WorkspaceID, route, w, h)
}

// artifactForPath maps the legacy .ironflyer/<name>.json convention to the
// typed Project.Artifacts name we now dual-write. Returning ("", false)
// means the path has no artifact mirror and we should fall straight to the
// FileNode lookup.
func artifactForPath(path string) (string, bool) {
	switch strings.ToLower(strings.TrimPrefix(path, "/")) {
	case ".ironflyer/plan.json":
		return domain.ArtifactPlan, true
	case ".ironflyer/stack.json":
		return domain.ArtifactStack, true
	case ".ironflyer/screen_map.json":
		return domain.ArtifactScreenMap, true
	case ".ironflyer/design_tokens.json":
		return domain.ArtifactDesignTokens, true
	}
	return "", false
}

// fileBody returns the artifact (preferred) or file contents for a given
// path on a project. We check Project.Artifacts first so the typed mirror
// is the source of truth; the .ironflyer/<name>.json file is the IDE
// transparency layer and backstop for older projects that predate the
// typed field. Match is case-insensitive on the full path.
func fileBody(p *domain.Project, path string) (string, bool) {
	if name, ok := artifactForPath(path); ok {
		if raw, present := p.GetArtifact(name); present {
			return string(raw), true
		}
	}
	want := strings.ToLower(strings.TrimPrefix(path, "/"))
	for _, f := range p.Files {
		if strings.ToLower(strings.TrimPrefix(f.Path, "/")) == want {
			return f.Content, true
		}
	}
	return "", false
}

type ArchGate struct{}

func (ArchGate) Name() domain.GateName    { return domain.GateArch }
func (ArchGate) RepairAgent() agents.Role { return agents.RoleArchitect }
func (ArchGate) Check(_ context.Context, env *GateEnv) []domain.Issue {
	p := env.Project
	var issues []domain.Issue
	if p.Spec.Stack.Frontend == "" || p.Spec.Stack.Backend == "" {
		issues = append(issues, domain.Issue{Gate: domain.GateArch, Severity: domain.SeverityError, Message: "stack not chosen"})
	}
	if p.Spec.Stack.Storage == "" {
		issues = append(issues, domain.Issue{Gate: domain.GateArch, Severity: domain.SeverityWarning, Message: "storage not chosen"})
	}
	return issues
}

type CodeGate struct{}

func (CodeGate) Name() domain.GateName    { return domain.GateCode }
func (CodeGate) RepairAgent() agents.Role { return agents.RoleCoder }
func (CodeGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	if env.HasRuntime() {
		return runBuild(ctx, env)
	}
	if len(env.Project.Files) == 0 {
		return []domain.Issue{{Gate: domain.GateCode, Severity: domain.SeverityError, Message: "no source files yet"}}
	}
	return nil
}

// runBuild detects the project's stack and runs the matching build command
// inside the bound workspace. Non-zero exit is an Issue carrying the tail of
// stderr; timeouts are reported as critical. Repair agent is the Coder.
func runBuild(ctx context.Context, env *GateEnv) []domain.Issue {
	cmd, label, ok := detectBuildCommand(env.Project)
	if !ok {
		return []domain.Issue{{
			Gate: domain.GateCode, Severity: domain.SeverityWarning,
			Message: "code gate has runtime but no known build command for this stack",
		}}
	}
	res, err := env.Runtime.Exec(ctx, env.UserBearer, env.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 120,
	})
	if err != nil {
		return []domain.Issue{{Gate: domain.GateCode, Severity: domain.SeverityError, Message: "exec: " + err.Error(), Hint: label}}
	}
	if res.TimedOut {
		return []domain.Issue{{Gate: domain.GateCode, Severity: domain.SeverityCritical, Message: "build timed out after 120s", Hint: label}}
	}
	if res.ExitCode != 0 {
		return []domain.Issue{{
			Gate: domain.GateCode, Severity: domain.SeverityError,
			Message: "build failed (exit " + itoaPositive(res.ExitCode) + ")",
			Hint:    label + " — " + tail(res.Stderr, 500),
		}}
	}
	return nil
}

// detectBuildCommand picks a build command from the project's declared stack
// (preferred) and file list (fallback). Returns (shellCmd, label, true) or
// false if nothing matches — caller surfaces a soft warning then.
func detectBuildCommand(p *domain.Project) (string, string, bool) {
	backend := strings.ToLower(p.Spec.Stack.Backend)
	frontend := strings.ToLower(p.Spec.Stack.Frontend)
	switch {
	case strings.Contains(backend, "go"):
		return "go build ./...", "go build", true
	case hasFile(p, "go.mod"):
		return "go build ./...", "go build", true
	case strings.Contains(frontend, "next") || strings.Contains(frontend, "react") || strings.Contains(backend, "node"):
		return "npm install --no-audit --no-fund && npm run build --if-present", "npm build", true
	case hasFile(p, "package.json"):
		return "npm install --no-audit --no-fund && npm run build --if-present", "npm build", true
	}
	return "", "", false
}

type TestGate struct{}

func (TestGate) Name() domain.GateName    { return domain.GateTest }
func (TestGate) RepairAgent() agents.Role { return agents.RoleTester }
func (TestGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	if env.HasRuntime() {
		return runTests(ctx, env)
	}
	for _, f := range env.Project.Files {
		if strings.Contains(f.Path, "_test.") || strings.Contains(f.Path, ".test.") || strings.Contains(f.Path, "/tests/") {
			return nil
		}
	}
	return []domain.Issue{{Gate: domain.GateTest, Severity: domain.SeverityError, Message: "no tests detected"}}
}

// LintGate runs language-appropriate linters (go vet, npm run lint) inside
// the bound workspace and surfaces non-zero exits as Issues. Without a
// runtime the gate passes silently — lint is not load-bearing enough to
// fail a project that simply hasn't been cloned to a workspace yet.
type LintGate struct{}

func (LintGate) Name() domain.GateName    { return domain.GateLint }
func (LintGate) RepairAgent() agents.Role { return agents.RoleReviewer }
func (LintGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	if !env.HasRuntime() {
		return nil
	}
	cmd, label, ok := detectLintCommand(env.Project)
	if !ok {
		return nil
	}
	res, err := env.Runtime.Exec(ctx, env.UserBearer, env.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 120,
	})
	if err != nil {
		return []domain.Issue{{Gate: domain.GateLint, Severity: domain.SeverityWarning, Message: "exec: " + err.Error(), Hint: label}}
	}
	if res.TimedOut {
		return []domain.Issue{{Gate: domain.GateLint, Severity: domain.SeverityError, Message: "lint timed out after 120s", Hint: label}}
	}
	if res.ExitCode != 0 {
		return []domain.Issue{{
			Gate: domain.GateLint, Severity: domain.SeverityWarning,
			Message: "lint reported issues (exit " + itoaPositive(res.ExitCode) + ")",
			Hint:    label + " — " + tail(res.Stdout+res.Stderr, 600),
		}}
	}
	return nil
}

func detectLintCommand(p *domain.Project) (string, string, bool) {
	backend := strings.ToLower(p.Spec.Stack.Backend)
	switch {
	case strings.Contains(backend, "go") || hasFile(p, "go.mod"):
		return "go vet ./...", "go vet", true
	case hasFile(p, "package.json"):
		// `--if-present` lets us skip projects that haven't wired up a lint
		// script yet without failing the gate.
		return "npm run lint --if-present --silent", "npm run lint", true
	}
	return "", "", false
}

// SecurityGate delegates to the AppSec core. The gate stays intentionally
// thin: appsec owns inventory, scanner orchestration, runtime adapters, and
// risk policy; finisher only maps the resulting findings into domain issues
// so the existing repair/reporting flow keeps working.
type SecurityGate struct{}

func (SecurityGate) Name() domain.GateName    { return domain.GateSecurity }
func (SecurityGate) RepairAgent() agents.Role { return agents.RoleSecurity }
func (SecurityGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	if env == nil || env.Project == nil {
		return nil
	}
	result := appsec.DefaultEngine().Scan(ctx, appSecTargetFromGateEnv(env))
	return appSecFindingsToIssues(result.Findings)
}

func appSecTargetFromGateEnv(env *GateEnv) appsec.Target {
	target := appsec.Target{
		ProjectID: env.Project.ID,
		Files:     make([]appsec.File, 0, len(env.Project.Files)),
	}
	for _, f := range env.Project.Files {
		target.Files = append(target.Files, appsec.File{Path: f.Path, Content: f.Content})
	}
	if env.HasRuntime() {
		target.Runtime = env.Runtime
		target.WorkspaceID = env.WorkspaceID
		target.UserBearer = env.UserBearer
	}
	return target
}

func appSecFindingsToIssues(findings []appsec.Finding) []domain.Issue {
	out := make([]domain.Issue, 0, len(findings))
	for _, f := range findings {
		out = append(out, domain.Issue{
			Gate:     domain.GateSecurity,
			Severity: appSecSeverityToDomain(f.Severity),
			Message:  f.RuleID + ": " + f.Summary,
			Hint:     f.Remediation,
			Path:     pathWithLine(f.Path, f.Line),
		})
	}
	return out
}

func appSecSeverityToDomain(sev appsec.Severity) domain.Severity {
	switch sev {
	case appsec.SeverityCritical:
		return domain.SeverityCritical
	case appsec.SeverityHigh:
		return domain.SeverityError
	case appsec.SeverityMedium, appsec.SeverityLow:
		return domain.SeverityWarning
	default:
		return domain.SeverityInfo
	}
}

func pathWithLine(path string, line int) string {
	if line <= 0 || path == "" {
		return path
	}
	return path + ":" + itoaPositive(line)
}

// shellQuote wraps a path in single quotes for safe shell interpolation,
// escaping any embedded quote.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// BudgetGate enforces that a project's accumulated provider cost stays
// within the user's plan cap. Unlike Code/Lint/Test/Security, no LLM repair
// can fix an overspend — RepairAgent() returns "" so the engine marks the
// gate blocked instead of looping a Coder. The gate runs after Security
// and before Deploy: if a project burned through its cap during the
// implementation passes, we refuse to ship it.
type BudgetGate struct{}

func (BudgetGate) Name() domain.GateName    { return domain.GateBudget }
func (BudgetGate) RepairAgent() agents.Role { return "" }
func (BudgetGate) Check(_ context.Context, env *GateEnv) []domain.Issue {
	if env == nil || env.Budget == nil {
		// Wiring degraded — emit a warning so the gate is visible in the UI
		// without hard-failing self-hosted setups that haven't connected a
		// vault. The gate still counts as "passed" (zero error-severity
		// issues), so the loop progresses.
		return []domain.Issue{{
			Gate: domain.GateBudget, Severity: domain.SeverityWarning,
			Message: "budget source not configured — spend tracking is dark",
			Hint:    "wire a BudgetSource via Engine.WithBudgetSource at startup",
		}}
	}
	b := env.Budget
	var issues []domain.Issue
	if b.CapUSD <= 0 {
		issues = append(issues, domain.Issue{
			Gate: domain.GateBudget, Severity: domain.SeverityWarning,
			Message: "no cost cap configured for the active plan",
			Hint:    "set plan.CostCapUSD so overspend protection has a ceiling to enforce",
		})
		return issues
	}
	if b.SpentUSD > b.CapUSD {
		sev := domain.SeverityError
		if !b.HardStop {
			sev = domain.SeverityWarning
		}
		issues = append(issues, domain.Issue{
			Gate: domain.GateBudget, Severity: sev,
			Message: "provider cost exceeded the plan cap (spent $" +
				formatUSD(b.SpentUSD) + " of $" + formatUSD(b.CapUSD) + ")",
			Hint: "raise the plan tier, prune iterations, or split the project",
		})
		return issues
	}
	// Soft-warning threshold at 80% of cap so the dashboard can surface a
	// yellow chip before the wall hits.
	if b.SpentUSD > 0 && b.SpentUSD/b.CapUSD > 0.80 {
		issues = append(issues, domain.Issue{
			Gate: domain.GateBudget, Severity: domain.SeverityWarning,
			Message: "approaching plan cap (spent $" +
				formatUSD(b.SpentUSD) + " of $" + formatUSD(b.CapUSD) + ")",
			Hint: "remaining iterations may push spend over budget",
		})
	}
	return issues
}

// formatUSD renders a float as "12.34". Two decimals — we never report
// fractions of a cent to users.
func formatUSD(v float64) string {
	if v < 0 {
		v = 0
	}
	cents := int64(v*100 + 0.5)
	dollars := cents / 100
	hundredths := cents % 100
	d := itoaPositive(int(dollars))
	if hundredths < 10 {
		return d + ".0" + itoaPositive(int(hundredths))
	}
	return d + "." + itoaPositive(int(hundredths))
}

type DeployGate struct{}

func (DeployGate) Name() domain.GateName    { return domain.GateDeploy }
func (DeployGate) RepairAgent() agents.Role { return agents.RoleDeployer }
func (DeployGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	hasDockerfile := false
	hasReadme := false
	var dockerfilePath string
	for _, f := range env.Project.Files {
		if strings.HasSuffix(f.Path, "Dockerfile") {
			hasDockerfile = true
			dockerfilePath = f.Path
		}
		if strings.EqualFold(f.Path, "README.md") {
			hasReadme = true
		}
	}
	var issues []domain.Issue
	if !hasDockerfile {
		issues = append(issues, domain.Issue{Gate: domain.GateDeploy, Severity: domain.SeverityError, Message: "no Dockerfile"})
	}
	if !hasReadme {
		issues = append(issues, domain.Issue{Gate: domain.GateDeploy, Severity: domain.SeverityWarning, Message: "no README"})
	}
	if hasDockerfile && env.HasRuntime() {
		issues = append(issues, dockerfileBuildCheck(ctx, env, dockerfilePath)...)
	}
	return issues
}

// dockerfileBuildCheck runs `docker build --check` (BuildKit linter) and,
// when that is unavailable, falls back to a `hadolint` static analysis
// pass. Either tool gives us a real, enforceable signal that the file
// declares a buildable image rather than a placeholder. We deliberately do
// NOT run a full `docker build` here — that requires Docker-in-Docker and
// can take minutes; the Code gate already exercises the actual build for
// language-specific compilation. This step is purely "is the Dockerfile a
// valid recipe". Both checks degrade silently when the binary is missing.
func dockerfileBuildCheck(ctx context.Context, env *GateEnv, dockerfilePath string) []domain.Issue {
	// Prefer `docker buildx build --check` (BuildKit's first-party linter).
	cmd := "command -v docker >/dev/null 2>&1 && docker buildx build --check -f " + shellQuote(dockerfilePath) + " . 2>&1 || true"
	res, err := env.Runtime.Exec(ctx, env.UserBearer, env.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 60,
	})
	if err == nil && !res.TimedOut && res.ExitCode == 0 && strings.TrimSpace(res.Stdout+res.Stderr) != "" {
		body := res.Stdout + res.Stderr
		// `docker build --check` prints warnings/errors on its stderr; an
		// exit 0 with "warning" lines is still suspect on a deploy gate.
		if strings.Contains(strings.ToLower(body), "warning") || strings.Contains(strings.ToLower(body), "error") {
			return []domain.Issue{{
				Gate: domain.GateDeploy, Severity: domain.SeverityWarning,
				Message: "docker buildx --check flagged issues",
				Path:    dockerfilePath,
				Hint:    tail(body, 600),
			}}
		}
		return nil
	}
	// Fallback to hadolint.
	hadolint := "command -v hadolint >/dev/null 2>&1 && hadolint --no-fail " + shellQuote(dockerfilePath) + " 2>&1 || true"
	res, err = env.Runtime.Exec(ctx, env.UserBearer, env.WorkspaceID, runtime.ExecOpts{
		Shell: hadolint, TimeoutSeconds: 30,
	})
	if err != nil || strings.TrimSpace(res.Stdout+res.Stderr) == "" {
		return nil
	}
	// One issue per finding line. Hadolint format: "<path>:<line> <code> <severity>: <message>"
	var out []domain.Issue
	for _, line := range strings.Split(res.Stdout+res.Stderr, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		sev := domain.SeverityWarning
		low := strings.ToLower(line)
		if strings.Contains(low, " error:") {
			sev = domain.SeverityError
		}
		out = append(out, domain.Issue{
			Gate: domain.GateDeploy, Severity: sev,
			Message: line, Path: dockerfilePath,
			Hint: "hadolint rule — see hadolint docs for remediation",
		})
		if len(out) >= 10 {
			break
		}
	}
	return out
}

func hasFile(p *domain.Project, name string) bool {
	for _, f := range p.Files {
		if strings.EqualFold(f.Path, name) || strings.HasSuffix(f.Path, "/"+name) {
			return true
		}
	}
	return false
}

func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}

func itoaPositive(n int) string {
	if n < 0 {
		return "-" + itoaPositive(-n)
	}
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// DefaultGates returns the ordered gate list. Lint runs after Code so a
// failing build short-circuits the loop before linters complain about a
// half-broken tree. Lighthouse runs LAST: a public preview URL only
// exists after Deploy, and the PSI call is wasted spend if an earlier
// gate is going to roll the project back anyway.
func DefaultGates() []Gate {
	return []Gate{
		SpecGate{}, UXGate{}, ArchGate{},
		CodeGate{}, LintGate{}, TestGate{},
		SecurityGate{}, BudgetGate{}, MobileBuildGate{},
		MobileExpoDoctorGate{},
		MobileSizeBudgetGate{},
		MobileSecurityGate{}, IOSPrivacyManifestGate{},
		PushCredentialsGate{}, DeployGate{},
		LighthouseGate{},
	}
}
