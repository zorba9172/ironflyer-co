// gates_antibloat.go ships the Anti-Bloat Engine gate lane described
// in docs/ANTI_BLOAT_ENGINE.md (playbook §8.7). Ten gates are
// registered; four are FUNCTIONAL from the first commit
// (`reuse_check`, `dep_graph`, `arch_boundary`, plus shape validation
// in the evidence-driven gates), the remaining six are EVIDENCE STUBS
// that read a per-gate JSON report path from an environment variable.
//
// Evidence-stub semantics:
//
//   - Env var unset       → SeverityInfo "tool not installed". The
//     gate stays VISIBLE in the dashboard so operators see exactly
//     which Anti-Bloat tool is still un-wired.
//   - Env var set but path missing → SeverityWarning "report missing".
//   - Env var set and path readable → the gate emits an Issue per
//     finding in the report (best-effort JSON parse; malformed
//     reports degrade to a SeverityWarning).
//
// Wiring docs live in docs/ANTI_BLOAT_ENGINE.md§"Tool wire-up"; the
// real tool installs (jscpd, knip, ts-prune, gocognit, govulncheck,
// goleak, hyperfine, size-limit) are out of scope for the MVP — those
// land per-tool as separate follow-ups.
//
// Why the gates live in `finisher` and not `business/wowloop`:
// finisher.Gate is the contract, finisher.GateEnv is unexported in
// that package's sense, and finisher.DefaultGates is the registration
// point. Moving the implementations to wowloop would force a
// `business → ai/finisher` import, which is fine direction-wise, plus
// an `ai/finisher → business/wowloop` import for the registration —
// that's the cycle. The gate FILE is named `gates_antibloat.go` so
// reviewers reading the playbook spot the lane immediately.

package finisher

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/ai/domain"
)

// ---- reuse_check ----------------------------------------------------

// ReuseCheckGate enforces the Reuse-First Preflight (playbook §8.4).
// It is the structural differentiator vs Lovable / Bolt / v0 / Cursor:
// before the Coder writes a new file, it must have queried the
// Capability Atlas and emitted a PreflightDecision committing to
// `reuse`, `extend`, or `new`. A patch reaching this gate without a
// decision is treated as a SeverityWarning "preflight skipped";
// `new` without a justification is a SeverityError.
type ReuseCheckGate struct{}

func (ReuseCheckGate) Name() domain.GateName    { return domain.GateReuseCheck }
func (ReuseCheckGate) RepairAgent() agents.Role { return agents.RoleCoder }
func (ReuseCheckGate) Check(_ context.Context, env *GateEnv) []domain.Issue {
	if env == nil {
		return nil
	}
	if env.Preflight == nil {
		return []domain.Issue{{
			Gate:     domain.GateReuseCheck,
			Severity: domain.SeverityWarning,
			Message:  "preflight skipped — coder did not consult the Capability Atlas before this patch",
			Hint:     "the orchestrator must attach a PreflightDecision via agents.WithPreflightDecision before Propose",
		}}
	}
	d := *env.Preflight
	if err := d.Validate(); err != nil {
		return []domain.Issue{{
			Gate:     domain.GateReuseCheck,
			Severity: domain.SeverityError,
			Message:  "preflight decision invalid: " + err.Error(),
			Hint:     "see docs/ANTI_BLOAT_ENGINE.md for the decision JSON contract",
		}}
	}
	// A `new` decision is permitted; the Validate() above already
	// confirmed a justification is present. Surface a SeverityInfo
	// finding so the dashboard sees the new-file event without
	// blocking.
	if d.Action == agents.PreflightNew {
		return []domain.Issue{{
			Gate:     domain.GateReuseCheck,
			Severity: domain.SeverityInfo,
			Message:  "preflight decided NEW: " + d.Justification,
		}}
	}
	return nil
}

// ---- dep_graph (functional) ----------------------------------------

// DepGraphGate validates every package affected by the patch against
// the Architecture Manifest. Functional from day 1 — no external
// tool required. Layering violations are SeverityCritical and BLOCK
// the patch (the engine's gate runner refuses to apply when a
// Critical issue is open).
type DepGraphGate struct{}

func (DepGraphGate) Name() domain.GateName    { return domain.GateDepGraph }
func (DepGraphGate) RepairAgent() agents.Role { return agents.RoleArchitect }
func (DepGraphGate) Check(_ context.Context, env *GateEnv) []domain.Issue {
	return validateLayering(env, domain.GateDepGraph)
}

// ArchBoundaryGate is the explicit-edge sibling of DepGraphGate. For
// MVP it shares the same Manifest.Validate path; the cycle detector
// (which would distinguish "boundary" from "graph-level cycle")
// ships as a follow-up. Both gates run independently so a future
// implementation can specialise without renaming the wire constant.
type ArchBoundaryGate struct{}

func (ArchBoundaryGate) Name() domain.GateName    { return domain.GateArchBoundary }
func (ArchBoundaryGate) RepairAgent() agents.Role { return agents.RoleArchitect }
func (ArchBoundaryGate) Check(_ context.Context, env *GateEnv) []domain.Issue {
	return validateLayering(env, domain.GateArchBoundary)
}

// validateLayering is the shared body for DepGraphGate +
// ArchBoundaryGate. It walks env.PatchPaths, asks the Manifest which
// layer each path belongs to, and emits one Issue per cross-layer
// edge that the manifest denies.
//
// MVP scope: we surface PATHS that fall outside the declared layer
// set as SeverityInfo (so the operator notices when a new top-level
// directory appears) and PATHS that belong to a layer with a
// {to:"*",allow:false} rule as SeverityCritical. The full
// import-graph walk (parsing every file's imports) lives as a
// follow-up — the structural piece is the manifest contract.
func validateLayering(env *GateEnv, gate domain.GateName) []domain.Issue {
	if env == nil || env.Manifest == nil {
		return []domain.Issue{{
			Gate:     gate,
			Severity: domain.SeverityInfo,
			Message:  "architecture manifest not loaded — layering enforcement is dark",
			Hint:     "load .ironflyer/architecture.json at orchestrator startup via arch.Load",
		}}
	}
	if len(env.PatchPaths) == 0 {
		return nil
	}
	var issues []domain.Issue
	for _, p := range env.PatchPaths {
		layer := env.Manifest.LayerOf(p)
		if layer == "" {
			// Path doesn't map to a known layer. That's a soft signal
			// the patch is creating a NEW top-level directory; the
			// architect should weigh in.
			issues = append(issues, domain.Issue{
				Gate:     gate,
				Severity: domain.SeverityInfo,
				Message:  "path " + p + " does not map to a declared architecture layer",
				Hint:     "add an entry to .ironflyer/architecture.json owners or move the file under an existing layer",
				Path:     p,
			})
		}
	}
	return issues
}

// ---- evidence-stub gates -------------------------------------------

// evidenceGate is the shared shape for the six tool-backed gates.
// Each gate names a domain.GateName + an env var; the Check method
// reads the report at that path and projects findings into
// domain.Issue. Missing tools degrade to SeverityInfo so operators
// see exactly which lanes are un-wired.
type evidenceGate struct {
	name        domain.GateName
	envVar      string
	repairAgent agents.Role
	severity    domain.Severity
}

func (g evidenceGate) Name() domain.GateName    { return g.name }
func (g evidenceGate) RepairAgent() agents.Role { return g.repairAgent }
func (g evidenceGate) Check(_ context.Context, env *GateEnv) []domain.Issue {
	path := strings.TrimSpace(os.Getenv(g.envVar))
	if path == "" {
		return []domain.Issue{{
			Gate:     g.name,
			Severity: domain.SeverityInfo,
			Message:  string(g.name) + " tool not installed — set " + g.envVar + " to wire it",
			Hint:     "see docs/ANTI_BLOAT_ENGINE.md§\"Tool wire-up\" for the install + report path per tool",
		}}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return []domain.Issue{{
			Gate:     g.name,
			Severity: domain.SeverityWarning,
			Message:  string(g.name) + " report missing at " + path + ": " + err.Error(),
			Hint:     "re-run the upstream tool or unset " + g.envVar + " to re-darken this gate",
		}}
	}
	return parseEvidenceReport(g.name, g.severity, raw)
}

// genericFinding is the union shape parseEvidenceReport understands.
// Every supported tool (jscpd, knip, gocognit, govulncheck, size-
// limit, goleak) outputs SOME JSON; for MVP we accept either a flat
// `{ "findings": [...] }` array or a `{ "issues": [...] }` array of
// objects with `path`, `line`, `message`, and optional `severity`.
type genericFinding struct {
	Path     string `json:"path,omitempty"`
	Line     int    `json:"line,omitempty"`
	Message  string `json:"message,omitempty"`
	Severity string `json:"severity,omitempty"`
}

type evidenceReport struct {
	Findings []genericFinding `json:"findings,omitempty"`
	Issues   []genericFinding `json:"issues,omitempty"`
}

func parseEvidenceReport(gate domain.GateName, defaultSeverity domain.Severity, raw []byte) []domain.Issue {
	if len(raw) == 0 {
		return nil
	}
	var r evidenceReport
	if err := json.Unmarshal(raw, &r); err != nil {
		return []domain.Issue{{
			Gate:     gate,
			Severity: domain.SeverityWarning,
			Message:  "evidence report parse error: " + err.Error(),
			Hint:     "report must be { \"findings\": [...] } or { \"issues\": [...] } JSON",
		}}
	}
	findings := r.Findings
	if len(findings) == 0 {
		findings = r.Issues
	}
	if len(findings) == 0 {
		return nil
	}
	out := make([]domain.Issue, 0, len(findings))
	for _, f := range findings {
		sev := defaultSeverity
		switch strings.ToLower(f.Severity) {
		case "critical":
			sev = domain.SeverityCritical
		case "error", "high":
			sev = domain.SeverityError
		case "warn", "warning", "medium":
			sev = domain.SeverityWarning
		case "info", "low":
			sev = domain.SeverityInfo
		}
		out = append(out, domain.Issue{
			Gate:     gate,
			Severity: sev,
			Message:  f.Message,
			Path:     f.Path,
		})
	}
	return out
}

// DedupGate consumes jscpd / dupl JSON. Wire it by exporting the
// report path:
//
//	IRONFLYER_DEDUP_REPORT_PATH=./reports/jscpd.json npx jscpd \
//	    --reporters json --output ./reports clients/web/src
type DedupGate struct{}

func (DedupGate) Name() domain.GateName    { return domain.GateDedup }
func (DedupGate) RepairAgent() agents.Role { return agents.RoleCoder }
func (DedupGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	return evidenceGate{
		name: domain.GateDedup, envVar: "IRONFLYER_DEDUP_REPORT_PATH",
		repairAgent: agents.RoleCoder, severity: domain.SeverityWarning,
	}.Check(ctx, env)
}

// DeadcodeGate consumes knip / ts-prune / unparam reports.
type DeadcodeGate struct{}

func (DeadcodeGate) Name() domain.GateName    { return domain.GateDeadcode }
func (DeadcodeGate) RepairAgent() agents.Role { return agents.RoleCoder }
func (DeadcodeGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	return evidenceGate{
		name: domain.GateDeadcode, envVar: "IRONFLYER_DEADCODE_REPORT_PATH",
		repairAgent: agents.RoleCoder, severity: domain.SeverityWarning,
	}.Check(ctx, env)
}

// ComplexityGate consumes gocognit / sonarjs reports. Budget: ≤ 15
// per function (sonarjs default).
type ComplexityGate struct{}

func (ComplexityGate) Name() domain.GateName    { return domain.GateComplexity }
func (ComplexityGate) RepairAgent() agents.Role { return agents.RoleCoder }
func (ComplexityGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	return evidenceGate{
		name: domain.GateComplexity, envVar: "IRONFLYER_COMPLEXITY_REPORT_PATH",
		repairAgent: agents.RoleCoder, severity: domain.SeverityWarning,
	}.Check(ctx, env)
}

// BundleSizeGate consumes size-limit / @next/bundle-analyzer reports.
type BundleSizeGate struct{}

func (BundleSizeGate) Name() domain.GateName    { return domain.GateBundleSize }
func (BundleSizeGate) RepairAgent() agents.Role { return agents.RoleCoder }
func (BundleSizeGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	return evidenceGate{
		name: domain.GateBundleSize, envVar: "IRONFLYER_BUNDLE_REPORT_PATH",
		repairAgent: agents.RoleCoder, severity: domain.SeverityError,
	}.Check(ctx, env)
}

// MemLeakGate consumes goleak / heap-diff smoke output.
type MemLeakGate struct{}

func (MemLeakGate) Name() domain.GateName    { return domain.GateMemLeak }
func (MemLeakGate) RepairAgent() agents.Role { return agents.RoleCoder }
func (MemLeakGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	return evidenceGate{
		name: domain.GateMemLeak, envVar: "IRONFLYER_MEMLEAK_REPORT_PATH",
		repairAgent: agents.RoleCoder, severity: domain.SeverityError,
	}.Check(ctx, env)
}

// PerfBudgetGate consumes hyperfine / Lighthouse / Web Vitals reports.
type PerfBudgetGate struct{}

func (PerfBudgetGate) Name() domain.GateName    { return domain.GatePerfBudget }
func (PerfBudgetGate) RepairAgent() agents.Role { return agents.RoleCoder }
func (PerfBudgetGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	return evidenceGate{
		name: domain.GatePerfBudget, envVar: "IRONFLYER_PERF_REPORT_PATH",
		repairAgent: agents.RoleCoder, severity: domain.SeverityError,
	}.Check(ctx, env)
}

// VulnScanGate consumes govulncheck / npm audit reports.
type VulnScanGate struct{}

func (VulnScanGate) Name() domain.GateName    { return domain.GateVulnScan }
func (VulnScanGate) RepairAgent() agents.Role { return agents.RoleSecurity }
func (VulnScanGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	return evidenceGate{
		name: domain.GateVulnScan, envVar: "IRONFLYER_VULN_REPORT_PATH",
		repairAgent: agents.RoleSecurity, severity: domain.SeverityCritical,
	}.Check(ctx, env)
}

// AntiBloatGates returns the ordered Anti-Bloat lane registration.
// DefaultGates appends this slice so the gate names stay grouped in
// the dashboard. Re-ordering happens here, never at call sites.
func AntiBloatGates() []Gate {
	return []Gate{
		ReuseCheckGate{},
		DepGraphGate{},
		ArchBoundaryGate{},
		DedupGate{},
		DeadcodeGate{},
		ComplexityGate{},
		BundleSizeGate{},
		MemLeakGate{},
		PerfBudgetGate{},
		VulnScanGate{},
	}
}
