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
	"strconv"
	"strings"

	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/ai/refactor"
)

// fmtFloat renders a duplication percentage in a stable 2-decimal form
// without bringing in fmt for a single call site. Used by the dedup
// and vuln gates to format summary issue messages.
func fmtFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func fmtInt(n int) string { return strconv.Itoa(n) }

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

// DedupGate consumes jscpd JSON (the canonical cross-file copy/paste
// detector). Wire it by running scripts/lint/run-jscpd.sh and
// exporting the normalized report path:
//
//	./scripts/lint/run-jscpd.sh
//	export IRONFLYER_DEDUP_REPORT_PATH=tmp/reports/jscpd-<ts>.json
//
// The script writes a `{ summary: { duplicationPct, budgetPct,
// cloneCount }, findings: [...] }` document. When `duplicationPct >
// budgetPct` we surface a SeverityError verdict scoped to the overall
// summary AND emit per-clone Warnings; when within budget we emit a
// single SeverityInfo "dup within budget" finding. When the report is
// shaped like a generic `{ findings: [...] }` (e.g. a hand-written
// JSON), we fall back to the evidenceGate parser.
type DedupGate struct{}

func (DedupGate) Name() domain.GateName    { return domain.GateDedup }
func (DedupGate) RepairAgent() agents.Role { return agents.RoleCoder }
func (DedupGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	path := strings.TrimSpace(os.Getenv("IRONFLYER_DEDUP_REPORT_PATH"))
	if path == "" {
		return []domain.Issue{{
			Gate:     domain.GateDedup,
			Severity: domain.SeverityInfo,
			Message:  "dedup tool not installed — run scripts/lint/run-jscpd.sh and export IRONFLYER_DEDUP_REPORT_PATH",
			Hint:     "see docs/ANTI_BLOAT_ENGINE.md§\"Tool wire-up\" + docs/CLOSEOUT_CHECKLIST.md",
		}}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return []domain.Issue{{
			Gate:     domain.GateDedup,
			Severity: domain.SeverityWarning,
			Message:  "dedup report missing at " + path + ": " + err.Error(),
			Hint:     "re-run scripts/lint/run-jscpd.sh or unset IRONFLYER_DEDUP_REPORT_PATH",
		}}
	}
	issues := parseDedupReport(raw)
	// Refactor Proposer hook (playbook §8.6). When the host has wired
	// a refactor service AND the report carries site-rich findings,
	// invoke the proposer per finding and attach the Proposal summary
	// to the corresponding Issue's Hint. The audit chain picks the
	// hints up via the gate.verdict entry's Attrs (see engine.go
	// recordAudit). When the report has no site detail the proposer
	// stays silent and the gate behaves exactly as before.
	if env != nil && env.Refactor != nil {
		findings := parseDedupSiteFindings(raw)
		if len(findings) > 0 {
			issues = annotateDedupIssuesWithProposals(ctx, issues, findings, env.Refactor)
		}
	}
	return issues
}

// dedupSiteFinding mirrors the richer per-clone shape the dedup driver
// emits when run with the site-export flag. Each finding carries the
// content hash + every site in the clone group; this is what the
// Refactor Proposer needs to produce a Proposal.
type dedupSiteFinding struct {
	Hash  string             `json:"hash"`
	Sites []dedupSiteEntry   `json:"sites,omitempty"`
}

type dedupSiteEntry struct {
	Path  string `json:"path"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	Body  string `json:"body"`
}

// dedupSiteReport is the optional richer shape: { siteFindings: [...] }.
// Kept distinct from `dedupReport` so the basic gate path keeps
// working for vanilla jscpd reports.
type dedupSiteReport struct {
	SiteFindings []dedupSiteFinding `json:"siteFindings,omitempty"`
}

func parseDedupSiteFindings(raw []byte) []refactor.Finding {
	var r dedupSiteReport
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil
	}
	if len(r.SiteFindings) == 0 {
		return nil
	}
	out := make([]refactor.Finding, 0, len(r.SiteFindings))
	for _, f := range r.SiteFindings {
		if len(f.Sites) < 2 {
			continue
		}
		sites := make([]refactor.Site, 0, len(f.Sites))
		for _, s := range f.Sites {
			sites = append(sites, refactor.Site{
				Path:  s.Path,
				Lines: [2]int{s.Start, s.End},
				Body:  s.Body,
			})
		}
		out = append(out, refactor.Finding{Hash: f.Hash, Sites: sites})
	}
	return out
}

// annotateDedupIssuesWithProposals runs the Refactor Proposer over
// each rich finding and attaches a one-line proposal summary to the
// Hint field of any Issue that matches one of the finding's site
// paths. We don't replace the Message — the operator still reads the
// original "duplicate block at X:Y" — we just add the proposal so the
// audit chain captures the "we propose extracting it to Z" verdict.
func annotateDedupIssuesWithProposals(
	ctx context.Context,
	issues []domain.Issue,
	findings []refactor.Finding,
	svc *refactor.Service,
) []domain.Issue {
	if svc == nil {
		return issues
	}
	// Build a path → proposal index for O(1) hint patching.
	proposals := make(map[string]*refactor.Proposal, len(findings))
	for _, f := range findings {
		proposal, err := svc.Propose(ctx, f)
		if err != nil || proposal == nil {
			continue
		}
		for _, site := range proposal.Sites {
			if _, exists := proposals[site.Path]; !exists {
				proposals[site.Path] = proposal
			}
		}
	}
	if len(proposals) == 0 {
		return issues
	}
	out := make([]domain.Issue, 0, len(issues))
	for _, issue := range issues {
		if issue.Path != "" {
			if proposal, ok := proposals[issue.Path]; ok {
				if issue.Hint == "" {
					issue.Hint = "Refactor Proposer: extract to " + proposal.TargetUtilPath +
						" — " + proposal.Justification
				} else {
					issue.Hint = issue.Hint + " | Refactor Proposer: extract to " +
						proposal.TargetUtilPath
				}
			}
		}
		out = append(out, issue)
	}
	return out
}

// dedupReport is the jscpd-normalized shape emitted by
// scripts/lint/run-jscpd.sh.
type dedupReport struct {
	Summary struct {
		DuplicationPct float64 `json:"duplicationPct"`
		BudgetPct      float64 `json:"budgetPct"`
		CloneCount     int     `json:"cloneCount"`
	} `json:"summary"`
	Findings []genericFinding `json:"findings,omitempty"`
}

func parseDedupReport(raw []byte) []domain.Issue {
	var r dedupReport
	if err := json.Unmarshal(raw, &r); err != nil {
		return []domain.Issue{{
			Gate:     domain.GateDedup,
			Severity: domain.SeverityWarning,
			Message:  "dedup report parse error: " + err.Error(),
			Hint:     "scripts/lint/run-jscpd.sh writes { summary, findings }; re-run it",
		}}
	}
	// No summary AND no findings → degrade to evidence parser (the
	// report may be a plain {findings:[]} document).
	if r.Summary.BudgetPct == 0 && r.Summary.DuplicationPct == 0 && len(r.Findings) == 0 {
		return parseEvidenceReport(domain.GateDedup, domain.SeverityWarning, raw)
	}
	budget := r.Summary.BudgetPct
	if budget == 0 {
		budget = 2.0 // playbook §8.5 default
	}
	exceeded := r.Summary.DuplicationPct > budget
	out := make([]domain.Issue, 0, len(r.Findings)+1)
	if exceeded {
		out = append(out, domain.Issue{
			Gate:     domain.GateDedup,
			Severity: domain.SeverityError,
			Message: "dedup over budget: " +
				fmtFloat(r.Summary.DuplicationPct) + "% > " +
				fmtFloat(budget) + "% across " +
				fmtInt(r.Summary.CloneCount) + " clone groups",
			Hint: "refactor duplicates into shared modules; the gate is the Anti-Bloat lane's dup signal",
		})
	} else {
		out = append(out, domain.Issue{
			Gate:     domain.GateDedup,
			Severity: domain.SeverityInfo,
			Message: "dedup within budget: " +
				fmtFloat(r.Summary.DuplicationPct) + "% ≤ " +
				fmtFloat(budget) + "% across " +
				fmtInt(r.Summary.CloneCount) + " clone groups",
		})
	}
	for _, f := range r.Findings {
		// Skip the synthetic overall-summary finding emitted by the
		// driver (it shares the summary message we already produced).
		if strings.HasPrefix(f.Message, "overall duplication ") {
			continue
		}
		sev := domain.SeverityWarning
		if exceeded {
			sev = domain.SeverityError
		}
		out = append(out, domain.Issue{
			Gate:     domain.GateDedup,
			Severity: sev,
			Message:  f.Message,
			Path:     f.Path,
		})
	}
	return out
}

// DeadcodeGate consumes knip JSON. Wire it by running
// scripts/lint/run-knip.sh and exporting the normalized report path:
//
//	./scripts/lint/run-knip.sh
//	export IRONFLYER_DEADCODE_REPORT_PATH=tmp/reports/knip-<ts>.json
//
// The driver writes a `{ summary: { unusedFiles, unusedExports,
// unusedDeps, budget, exceeded }, findings: [...] }` document. When
// `exceeded` is true we surface a SeverityError verdict + per-item
// SeverityError findings; when within budget we emit a single
// SeverityInfo. Falls back to evidenceGate's generic parser if the
// report is shaped like a plain `{ findings: [...] }`.
type DeadcodeGate struct{}

func (DeadcodeGate) Name() domain.GateName    { return domain.GateDeadcode }
func (DeadcodeGate) RepairAgent() agents.Role { return agents.RoleCoder }
func (DeadcodeGate) Check(_ context.Context, env *GateEnv) []domain.Issue {
	path := strings.TrimSpace(os.Getenv("IRONFLYER_DEADCODE_REPORT_PATH"))
	if path == "" {
		return []domain.Issue{{
			Gate:     domain.GateDeadcode,
			Severity: domain.SeverityInfo,
			Message:  "deadcode tool not installed — run scripts/lint/run-knip.sh and export IRONFLYER_DEADCODE_REPORT_PATH",
			Hint:     "see docs/ANTI_BLOAT_ENGINE.md§\"Tool wire-up\" + docs/CLOSEOUT_CHECKLIST.md",
		}}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return []domain.Issue{{
			Gate:     domain.GateDeadcode,
			Severity: domain.SeverityWarning,
			Message:  "deadcode report missing at " + path + ": " + err.Error(),
			Hint:     "re-run scripts/lint/run-knip.sh or unset IRONFLYER_DEADCODE_REPORT_PATH",
		}}
	}
	return parseDeadcodeReport(raw)
}

// deadcodeReport mirrors the shape emitted by scripts/lint/run-knip.sh.
type deadcodeReport struct {
	Summary struct {
		UnusedFiles   int  `json:"unusedFiles"`
		UnusedExports int  `json:"unusedExports"`
		UnusedDeps    int  `json:"unusedDeps"`
		Budget        int  `json:"budget"`
		Exceeded      bool `json:"exceeded"`
	} `json:"summary"`
	Findings []genericFinding `json:"findings,omitempty"`
}

func parseDeadcodeReport(raw []byte) []domain.Issue {
	var r deadcodeReport
	if err := json.Unmarshal(raw, &r); err != nil {
		return []domain.Issue{{
			Gate:     domain.GateDeadcode,
			Severity: domain.SeverityWarning,
			Message:  "deadcode report parse error: " + err.Error(),
			Hint:     "scripts/lint/run-knip.sh writes { summary, findings }; re-run it",
		}}
	}
	total := r.Summary.UnusedFiles + r.Summary.UnusedExports + r.Summary.UnusedDeps
	// No summary AND no findings → degrade to evidence parser (plain
	// { findings: [] } passthrough).
	if total == 0 && !r.Summary.Exceeded && len(r.Findings) == 0 {
		return parseEvidenceReport(domain.GateDeadcode, domain.SeverityWarning, raw)
	}
	out := make([]domain.Issue, 0, len(r.Findings)+1)
	if r.Summary.Exceeded {
		out = append(out, domain.Issue{
			Gate:     domain.GateDeadcode,
			Severity: domain.SeverityError,
			Message: "deadcode over budget: " + fmtInt(total) +
				" unused items (" + fmtInt(r.Summary.UnusedFiles) + " files, " +
				fmtInt(r.Summary.UnusedExports) + " exports, " +
				fmtInt(r.Summary.UnusedDeps) + " deps) > budget " +
				fmtInt(r.Summary.Budget),
			Hint: "delete unused exports/files/deps or update IRONFLYER_DEADCODE_BUDGET if intentional",
		})
	} else {
		out = append(out, domain.Issue{
			Gate:     domain.GateDeadcode,
			Severity: domain.SeverityInfo,
			Message: "deadcode within budget: " + fmtInt(total) +
				" unused items (budget " + fmtInt(r.Summary.Budget) + ")",
		})
	}
	for _, f := range r.Findings {
		// Skip the synthetic summary finding the driver leads with so
		// we don't emit it twice (we already wrote our own summary
		// above using the structured Summary block).
		if strings.HasPrefix(f.Message, "knip: ") {
			continue
		}
		sev := domain.SeverityWarning
		switch strings.ToLower(f.Severity) {
		case "error", "high":
			sev = domain.SeverityError
		case "info", "low":
			sev = domain.SeverityInfo
		}
		out = append(out, domain.Issue{
			Gate:     domain.GateDeadcode,
			Severity: sev,
			Message:  f.Message,
			Path:     f.Path,
		})
	}
	return out
}

// ComplexityGate consumes gocognit JSON. Wire it by running
// scripts/lint/run-gocognit.sh and exporting the normalized report
// path:
//
//	./scripts/lint/run-gocognit.sh
//	export IRONFLYER_COMPLEXITY_REPORT_PATH=tmp/reports/gocognit-<ts>.json
//
// Each offender above the budget surfaces as SeverityWarning. Any
// function with complexity ≥ 2× the budget escalates to SeverityError.
type ComplexityGate struct{}

func (ComplexityGate) Name() domain.GateName    { return domain.GateComplexity }
func (ComplexityGate) RepairAgent() agents.Role { return agents.RoleCoder }
func (ComplexityGate) Check(_ context.Context, env *GateEnv) []domain.Issue {
	path := strings.TrimSpace(os.Getenv("IRONFLYER_COMPLEXITY_REPORT_PATH"))
	if path == "" {
		return []domain.Issue{{
			Gate:     domain.GateComplexity,
			Severity: domain.SeverityInfo,
			Message:  "complexity tool not installed — run scripts/lint/run-gocognit.sh and export IRONFLYER_COMPLEXITY_REPORT_PATH",
			Hint:     "see docs/ANTI_BLOAT_ENGINE.md§\"Tool wire-up\" + docs/CLOSEOUT_CHECKLIST.md",
		}}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return []domain.Issue{{
			Gate:     domain.GateComplexity,
			Severity: domain.SeverityWarning,
			Message:  "complexity report missing at " + path + ": " + err.Error(),
			Hint:     "re-run scripts/lint/run-gocognit.sh or unset IRONFLYER_COMPLEXITY_REPORT_PATH",
		}}
	}
	return parseComplexityReport(raw)
}

// complexityReport mirrors scripts/lint/run-gocognit.sh.
type complexityReport struct {
	Summary struct {
		Offenders       int `json:"offenders"`
		Budget          int `json:"budget"`
		SevereOffenders int `json:"severeOffenders"`
	} `json:"summary"`
	Findings []genericFinding `json:"findings,omitempty"`
}

func parseComplexityReport(raw []byte) []domain.Issue {
	var r complexityReport
	if err := json.Unmarshal(raw, &r); err != nil {
		return []domain.Issue{{
			Gate:     domain.GateComplexity,
			Severity: domain.SeverityWarning,
			Message:  "complexity report parse error: " + err.Error(),
			Hint:     "scripts/lint/run-gocognit.sh writes { summary, findings }; re-run it",
		}}
	}
	if r.Summary.Budget == 0 && r.Summary.Offenders == 0 && len(r.Findings) == 0 {
		return parseEvidenceReport(domain.GateComplexity, domain.SeverityWarning, raw)
	}
	out := make([]domain.Issue, 0, len(r.Findings)+1)
	summarySev := domain.SeverityInfo
	if r.Summary.SevereOffenders > 0 {
		summarySev = domain.SeverityError
	} else if r.Summary.Offenders > 0 {
		summarySev = domain.SeverityWarning
	}
	out = append(out, domain.Issue{
		Gate:     domain.GateComplexity,
		Severity: summarySev,
		Message: "complexity: " + fmtInt(r.Summary.Offenders) +
			" functions over budget " + fmtInt(r.Summary.Budget) +
			" (" + fmtInt(r.Summary.SevereOffenders) + " ≥ 2× budget)",
		Hint: "refactor offenders; consider extracting helpers or reducing branching depth",
	})
	for _, f := range r.Findings {
		if strings.HasPrefix(f.Message, "complexity: ") {
			// Driver-emitted summary; we replaced it above.
			continue
		}
		sev := domain.SeverityWarning
		switch strings.ToLower(f.Severity) {
		case "critical":
			sev = domain.SeverityError
		case "error", "high":
			sev = domain.SeverityError
		case "info", "low":
			sev = domain.SeverityInfo
		}
		out = append(out, domain.Issue{
			Gate:     domain.GateComplexity,
			Severity: sev,
			Message:  f.Message,
			Path:     f.Path,
		})
	}
	return out
}

// BundleSizeGate consumes size-limit JSON. Wire it by running
// scripts/lint/run-size-limit.sh and exporting the normalized report
// path:
//
//	./scripts/lint/run-size-limit.sh
//	export IRONFLYER_BUNDLE_REPORT_PATH=tmp/reports/size-limit-<ts>.json
//
// Each over-budget route surfaces as SeverityError; an all-pass run
// emits a single SeverityInfo summary.
type BundleSizeGate struct{}

func (BundleSizeGate) Name() domain.GateName    { return domain.GateBundleSize }
func (BundleSizeGate) RepairAgent() agents.Role { return agents.RoleCoder }
func (BundleSizeGate) Check(_ context.Context, env *GateEnv) []domain.Issue {
	path := strings.TrimSpace(os.Getenv("IRONFLYER_BUNDLE_REPORT_PATH"))
	if path == "" {
		return []domain.Issue{{
			Gate:     domain.GateBundleSize,
			Severity: domain.SeverityInfo,
			Message:  "bundle_size tool not installed — run scripts/lint/run-size-limit.sh and export IRONFLYER_BUNDLE_REPORT_PATH",
			Hint:     "see docs/ANTI_BLOAT_ENGINE.md§\"Tool wire-up\" + docs/CLOSEOUT_CHECKLIST.md",
		}}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return []domain.Issue{{
			Gate:     domain.GateBundleSize,
			Severity: domain.SeverityWarning,
			Message:  "bundle_size report missing at " + path + ": " + err.Error(),
			Hint:     "re-run scripts/lint/run-size-limit.sh or unset IRONFLYER_BUNDLE_REPORT_PATH",
		}}
	}
	return parseBundleReport(raw)
}

// bundleReport mirrors scripts/lint/run-size-limit.sh.
type bundleReport struct {
	Summary struct {
		TotalRoutes      int `json:"totalRoutes"`
		OverBudgetRoutes int `json:"overBudgetRoutes"`
	} `json:"summary"`
	Findings []genericFinding `json:"findings,omitempty"`
}

func parseBundleReport(raw []byte) []domain.Issue {
	var r bundleReport
	if err := json.Unmarshal(raw, &r); err != nil {
		return []domain.Issue{{
			Gate:     domain.GateBundleSize,
			Severity: domain.SeverityWarning,
			Message:  "bundle_size report parse error: " + err.Error(),
			Hint:     "scripts/lint/run-size-limit.sh writes { summary, findings }; re-run it",
		}}
	}
	if r.Summary.TotalRoutes == 0 && r.Summary.OverBudgetRoutes == 0 && len(r.Findings) == 0 {
		return parseEvidenceReport(domain.GateBundleSize, domain.SeverityError, raw)
	}
	out := make([]domain.Issue, 0, len(r.Findings)+1)
	summarySev := domain.SeverityInfo
	if r.Summary.OverBudgetRoutes > 0 {
		summarySev = domain.SeverityError
	}
	out = append(out, domain.Issue{
		Gate:     domain.GateBundleSize,
		Severity: summarySev,
		Message: "bundle_size: " + fmtInt(r.Summary.OverBudgetRoutes) +
			" of " + fmtInt(r.Summary.TotalRoutes) + " routes over budget",
		Hint: "lazy-load heavy modules via next/dynamic; trim shared chunks",
	})
	for _, f := range r.Findings {
		if strings.HasPrefix(f.Message, "size-limit: ") {
			continue
		}
		sev := domain.SeverityInfo
		switch strings.ToLower(f.Severity) {
		case "critical":
			sev = domain.SeverityCritical
		case "error", "high":
			sev = domain.SeverityError
		case "warn", "warning", "medium":
			sev = domain.SeverityWarning
		}
		out = append(out, domain.Issue{
			Gate:     domain.GateBundleSize,
			Severity: sev,
			Message:  f.Message,
			Path:     f.Path,
		})
	}
	return out
}

// MemLeakGate consumes the goleak smoke-harness snapshot produced by
// scripts/lint/run-goleak-smoke.sh. We do NOT use go.uber.org/goleak
// (it is test-only and the repo's no-tests rule is constitutional) —
// the driver curls /debug/leak/snapshot, diffs the goroutine count
// against scripts/health/goleak-baseline.json, and writes a normalized
// report. A confirmed leak escalates to SeverityCritical so the
// gate runner blocks the apply.
type MemLeakGate struct{}

func (MemLeakGate) Name() domain.GateName    { return domain.GateMemLeak }
func (MemLeakGate) RepairAgent() agents.Role { return agents.RoleCoder }
func (MemLeakGate) Check(_ context.Context, env *GateEnv) []domain.Issue {
	path := strings.TrimSpace(os.Getenv("IRONFLYER_MEMLEAK_REPORT_PATH"))
	if path == "" {
		return []domain.Issue{{
			Gate:     domain.GateMemLeak,
			Severity: domain.SeverityInfo,
			Message:  "mem_leak tool not installed — run scripts/lint/run-goleak-smoke.sh and export IRONFLYER_MEMLEAK_REPORT_PATH",
			Hint:     "see docs/ANTI_BLOAT_ENGINE.md§\"Tool wire-up\" + docs/CLOSEOUT_CHECKLIST.md",
		}}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return []domain.Issue{{
			Gate:     domain.GateMemLeak,
			Severity: domain.SeverityWarning,
			Message:  "mem_leak report missing at " + path + ": " + err.Error(),
			Hint:     "re-run scripts/lint/run-goleak-smoke.sh or unset IRONFLYER_MEMLEAK_REPORT_PATH",
		}}
	}
	return parseLeakReport(raw)
}

// leakReport mirrors scripts/lint/run-goleak-smoke.sh.
type leakReport struct {
	Summary struct {
		Goroutines     int  `json:"goroutines"`
		Baseline       int  `json:"baseline"`
		Delta          int  `json:"delta"`
		ThresholdDelta int  `json:"thresholdDelta"`
		Leaked         bool `json:"leaked"`
	} `json:"summary"`
	Findings []genericFinding `json:"findings,omitempty"`
}

func parseLeakReport(raw []byte) []domain.Issue {
	var r leakReport
	if err := json.Unmarshal(raw, &r); err != nil {
		return []domain.Issue{{
			Gate:     domain.GateMemLeak,
			Severity: domain.SeverityWarning,
			Message:  "mem_leak report parse error: " + err.Error(),
			Hint:     "scripts/lint/run-goleak-smoke.sh writes { summary, findings }; re-run it",
		}}
	}
	if r.Summary.Goroutines == 0 && r.Summary.Baseline == 0 && len(r.Findings) == 0 {
		return parseEvidenceReport(domain.GateMemLeak, domain.SeverityError, raw)
	}
	out := make([]domain.Issue, 0, len(r.Findings)+1)
	summarySev := domain.SeverityInfo
	if r.Summary.Leaked {
		summarySev = domain.SeverityCritical
	}
	out = append(out, domain.Issue{
		Gate:     domain.GateMemLeak,
		Severity: summarySev,
		Message: "mem_leak: " + fmtInt(r.Summary.Goroutines) +
			" goroutines (baseline " + fmtInt(r.Summary.Baseline) +
			", delta " + fmtInt(r.Summary.Delta) +
			", threshold +" + fmtInt(r.Summary.ThresholdDelta) + ")",
		Hint: "inspect goroutine stacks at /debug/leak/snapshot; common culprits: unclosed SSE handlers, leaked contexts",
	})
	for _, f := range r.Findings {
		if strings.HasPrefix(f.Message, "goleak: ") {
			continue
		}
		sev := domain.SeverityInfo
		switch strings.ToLower(f.Severity) {
		case "critical":
			sev = domain.SeverityCritical
		case "error", "high":
			sev = domain.SeverityError
		case "warn", "warning", "medium":
			sev = domain.SeverityWarning
		}
		out = append(out, domain.Issue{
			Gate:     domain.GateMemLeak,
			Severity: sev,
			Message:  f.Message,
			Path:     f.Path,
		})
	}
	return out
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

// VulnScanGate consumes govulncheck JSON. Wire it by running
// scripts/lint/run-govulncheck.sh and exporting the normalized
// report path:
//
//	./scripts/lint/run-govulncheck.sh
//	export IRONFLYER_VULN_REPORT_PATH=tmp/reports/govulncheck-<ts>.json
//
// The driver emits `{ findings: [ { path, message, severity } ] }`.
// We map severity strings to the gate-runner severity so the engine's
// Critical-blocks-apply rule kicks in correctly: high+critical →
// SeverityCritical (BLOCKS deploy), medium → SeverityWarning, low →
// SeverityInfo. Unknown severities default to SeverityWarning rather
// than SeverityCritical so a malformed osv entry doesn't accidentally
// block a deploy.
type VulnScanGate struct{}

func (VulnScanGate) Name() domain.GateName    { return domain.GateVulnScan }
func (VulnScanGate) RepairAgent() agents.Role { return agents.RoleSecurity }
func (VulnScanGate) Check(_ context.Context, env *GateEnv) []domain.Issue {
	path := strings.TrimSpace(os.Getenv("IRONFLYER_VULN_REPORT_PATH"))
	if path == "" {
		return []domain.Issue{{
			Gate:     domain.GateVulnScan,
			Severity: domain.SeverityInfo,
			Message:  "vuln_scan tool not installed — run scripts/lint/run-govulncheck.sh and export IRONFLYER_VULN_REPORT_PATH",
			Hint:     "see docs/ANTI_BLOAT_ENGINE.md§\"Tool wire-up\" + docs/CLOSEOUT_CHECKLIST.md",
		}}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return []domain.Issue{{
			Gate:     domain.GateVulnScan,
			Severity: domain.SeverityWarning,
			Message:  "vuln_scan report missing at " + path + ": " + err.Error(),
			Hint:     "re-run scripts/lint/run-govulncheck.sh or unset IRONFLYER_VULN_REPORT_PATH",
		}}
	}
	return parseVulnReport(raw)
}

// parseVulnReport reads a govulncheck-normalized report and projects
// findings into orchestrator gate issues. high/critical findings BLOCK
// (SeverityCritical); medium → Warning; low → Info. A clean report
// (zero findings) emits a single SeverityInfo "no known vulnerabilities".
func parseVulnReport(raw []byte) []domain.Issue {
	if len(raw) == 0 {
		return nil
	}
	var r evidenceReport
	if err := json.Unmarshal(raw, &r); err != nil {
		return []domain.Issue{{
			Gate:     domain.GateVulnScan,
			Severity: domain.SeverityWarning,
			Message:  "vuln_scan report parse error: " + err.Error(),
			Hint:     "scripts/lint/run-govulncheck.sh writes { findings:[...] }; re-run it",
		}}
	}
	findings := r.Findings
	if len(findings) == 0 {
		findings = r.Issues
	}
	if len(findings) == 0 {
		return []domain.Issue{{
			Gate:     domain.GateVulnScan,
			Severity: domain.SeverityInfo,
			Message:  "vuln_scan: no known vulnerabilities",
		}}
	}
	var critical, medium, low int
	out := make([]domain.Issue, 0, len(findings)+1)
	for _, f := range findings {
		sev := domain.SeverityWarning
		switch strings.ToLower(strings.TrimSpace(f.Severity)) {
		case "critical", "high":
			sev = domain.SeverityCritical
			critical++
		case "medium", "warning", "warn":
			sev = domain.SeverityWarning
			medium++
		case "low", "info":
			sev = domain.SeverityInfo
			low++
		default:
			medium++
		}
		out = append(out, domain.Issue{
			Gate:     domain.GateVulnScan,
			Severity: sev,
			Message:  f.Message,
			Path:     f.Path,
		})
	}
	// Lead with a summary issue so the dashboard renders the verdict
	// at the top of the gate panel without scrolling through every
	// finding.
	summarySev := domain.SeverityInfo
	if critical > 0 {
		summarySev = domain.SeverityCritical
	} else if medium > 0 {
		summarySev = domain.SeverityWarning
	}
	summary := domain.Issue{
		Gate:     domain.GateVulnScan,
		Severity: summarySev,
		Message: "vuln_scan: " + fmtInt(critical) + " high/critical, " +
			fmtInt(medium) + " medium, " + fmtInt(low) + " low",
	}
	return append([]domain.Issue{summary}, out...)
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
