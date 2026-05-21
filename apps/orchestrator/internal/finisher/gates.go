package finisher

import (
	"context"
	"regexp"
	"strings"

	"ironflyer/apps/orchestrator/internal/agents"
	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/runtime"
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
func (SpecGate) Check(_ context.Context, env *GateEnv) []domain.Issue {
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
	return issues
}

type UXGate struct{}

func (UXGate) Name() domain.GateName    { return domain.GateUX }
func (UXGate) RepairAgent() agents.Role { return agents.RoleUXer }
func (UXGate) Check(_ context.Context, env *GateEnv) []domain.Issue {
	if len(env.Project.Spec.UserStories) == 0 {
		return []domain.Issue{{Gate: domain.GateUX, Severity: domain.SeverityWarning, Message: "blocked: spec has no stories"}}
	}
	// Stub: real check inspects design tokens + screen map.
	return nil
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

func runTests(ctx context.Context, env *GateEnv) []domain.Issue {
	cmd, label, ok := detectTestCommand(env.Project)
	if !ok {
		return []domain.Issue{{
			Gate: domain.GateTest, Severity: domain.SeverityWarning,
			Message: "test gate has runtime but no known test command for this stack",
		}}
	}
	res, err := env.Runtime.Exec(ctx, env.UserBearer, env.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 180,
	})
	if err != nil {
		return []domain.Issue{{Gate: domain.GateTest, Severity: domain.SeverityError, Message: "exec: " + err.Error(), Hint: label}}
	}
	if res.TimedOut {
		return []domain.Issue{{Gate: domain.GateTest, Severity: domain.SeverityCritical, Message: "tests timed out after 180s", Hint: label}}
	}
	if res.ExitCode != 0 {
		return []domain.Issue{{
			Gate: domain.GateTest, Severity: domain.SeverityError,
			Message: "tests failed (exit " + itoaPositive(res.ExitCode) + ")",
			Hint:    label + " — " + tail(res.Stdout+res.Stderr, 800),
		}}
	}
	return nil
}

func detectTestCommand(p *domain.Project) (string, string, bool) {
	backend := strings.ToLower(p.Spec.Stack.Backend)
	switch {
	case strings.Contains(backend, "go") || hasFile(p, "go.mod"):
		return "go test ./...", "go test", true
	case hasFile(p, "package.json"):
		return "npm test --silent --if-present", "npm test", true
	}
	return "", "", false
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

// SecretPattern is one regex + label used by the security scanner.
type SecretPattern struct {
	Name string
	Re   *regexp.Regexp
}

// secretPatterns covers credentials that are unambiguous when matched: long
// random strings with provider-specific prefixes or shapes. Generic
// "password=…" heuristics live separately in suspiciousAssignments so they
// can be downgraded to warnings.
var secretPatterns = []SecretPattern{
	{Name: "AWS access key", Re: regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{Name: "AWS secret access key", Re: regexp.MustCompile(`(?i)aws(.{0,20})?(secret|access).{0,20}?['"]([0-9a-zA-Z/+=]{40})['"]`)},
	{Name: "GitHub personal access token", Re: regexp.MustCompile(`\bghp_[0-9A-Za-z]{36}\b`)},
	{Name: "GitHub OAuth token", Re: regexp.MustCompile(`\bgho_[0-9A-Za-z]{36}\b`)},
	{Name: "GitHub fine-grained PAT", Re: regexp.MustCompile(`\bgithub_pat_[0-9A-Za-z_]{82}\b`)},
	{Name: "Stripe secret key", Re: regexp.MustCompile(`\bsk_(live|test)_[0-9a-zA-Z]{20,}\b`)},
	{Name: "Stripe restricted key", Re: regexp.MustCompile(`\brk_(live|test)_[0-9a-zA-Z]{20,}\b`)},
	{Name: "OpenAI API key", Re: regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{32,}\b`)},
	{Name: "Anthropic API key", Re: regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_-]{32,}\b`)},
	{Name: "Google API key", Re: regexp.MustCompile(`\bAIza[0-9A-Za-z\-_]{35}\b`)},
	{Name: "Slack token", Re: regexp.MustCompile(`\bxox[abprs]-[0-9A-Za-z-]{10,}\b`)},
	{Name: "Private key block", Re: regexp.MustCompile(`-----BEGIN (RSA|EC|OPENSSH|PGP|DSA) PRIVATE KEY-----`)},
	{Name: "JWT", Re: regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`)},
	{Name: "URL with embedded credentials", Re: regexp.MustCompile(`\b(https?|postgres|mysql|mongodb|redis):\/\/[^\s:@/]+:[^\s:@/]+@`)},
}

// suspiciousAssignments matches things like `password = "literal"` — these
// are likely accidents but include too many false positives (test fixtures,
// docs) to be Critical without more context. We surface them as Warnings.
var suspiciousAssignments = regexp.MustCompile(`(?im)^[^#/]*\b(password|passwd|api[_-]?key|secret|token)\s*[:=]\s*['"][^'"\s]{6,}['"]`)

// SecurityGate scans both the in-memory file content and (when available)
// the workspace filesystem for high-confidence credential patterns. The
// in-memory pass covers AI-generated drafts before they ever touch disk;
// the runtime pass catches anything that arrived via git clone.
type SecurityGate struct{}

func (SecurityGate) Name() domain.GateName    { return domain.GateSecurity }
func (SecurityGate) RepairAgent() agents.Role { return agents.RoleSecurity }
func (SecurityGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	var issues []domain.Issue

	for _, f := range env.Project.Files {
		if shouldSkipForSecrets(f.Path) {
			continue
		}
		issues = append(issues, scanForSecrets(f.Path, f.Content)...)
	}

	if env.HasRuntime() {
		issues = append(issues, scanWorkspaceForSecrets(ctx, env)...)
	}
	return issues
}

// scanForSecrets returns one Issue per pattern hit so the user sees what
// kind of secret was detected. Each match is reported once per file even if
// the regex matches multiple times — we don't want to drown the UI.
func scanForSecrets(path, content string) []domain.Issue {
	if content == "" {
		return nil
	}
	var out []domain.Issue
	seen := map[string]bool{}
	for _, p := range secretPatterns {
		if p.Re.MatchString(content) && !seen[p.Name] {
			seen[p.Name] = true
			out = append(out, domain.Issue{
				Gate: domain.GateSecurity, Severity: domain.SeverityCritical,
				Message: "found " + p.Name, Path: path,
				Hint: "remove the credential, rotate it upstream, and store the replacement outside source",
			})
		}
	}
	if !seen["JWT"] && suspiciousAssignments.MatchString(content) {
		out = append(out, domain.Issue{
			Gate: domain.GateSecurity, Severity: domain.SeverityWarning,
			Message: "suspicious credential-shaped assignment", Path: path,
			Hint: "verify this isn't a hardcoded secret",
		})
	}
	return out
}

// scanWorkspaceForSecrets runs a single grep -RIl --binary-files=without-match
// inside the workspace and re-scans the matched files via cat for full
// pattern context. We grep on a coarse OR of high-signal substrings so the
// process is cheap; precise classification happens in scanForSecrets.
func scanWorkspaceForSecrets(ctx context.Context, env *GateEnv) []domain.Issue {
	const coarse = `'AKIA[A-Z0-9]|ghp_|gho_|github_pat_|sk_live_|sk_test_|sk-ant-|BEGIN [A-Z]\+ PRIVATE KEY|eyJ[A-Za-z0-9_-]\{10,\}\.eyJ'`
	listCmd := "grep -RIlE --binary-files=without-match --exclude-dir=.git --exclude-dir=node_modules " + coarse + " . 2>/dev/null | head -50"
	res, err := env.Runtime.Exec(ctx, env.UserBearer, env.WorkspaceID, runtime.ExecOpts{
		Shell: listCmd, TimeoutSeconds: 60,
	})
	if err != nil || res.ExitCode > 1 {
		// Exit 1 from grep = "no matches", which is the happy path.
		return nil
	}
	var issues []domain.Issue
	for _, path := range strings.Split(strings.TrimSpace(res.Stdout), "\n") {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		// Pull file body via cat, capped at 256 KiB so a giant blob can't
		// dominate the scan budget. Skip on read error.
		body, err := env.Runtime.Exec(ctx, env.UserBearer, env.WorkspaceID, runtime.ExecOpts{
			Shell: "head -c 262144 -- " + shellQuote(path), TimeoutSeconds: 20,
		})
		if err != nil || body.ExitCode != 0 {
			continue
		}
		issues = append(issues, scanForSecrets(strings.TrimPrefix(path, "./"), body.Stdout)...)
	}
	return issues
}

// shouldSkipForSecrets prunes known-noisy paths. We never scan binary blobs,
// lock files, or the .git directory — false positives there hide real hits
// in the signal-rich files.
func shouldSkipForSecrets(path string) bool {
	low := strings.ToLower(path)
	switch {
	case strings.Contains(low, "/.git/"), strings.HasPrefix(low, ".git/"):
		return true
	case strings.Contains(low, "/node_modules/"), strings.HasPrefix(low, "node_modules/"):
		return true
	case strings.HasSuffix(low, ".lock"), strings.HasSuffix(low, "-lock.json"), strings.HasSuffix(low, "go.sum"):
		return true
	}
	return false
}

// shellQuote wraps a path in single quotes for safe shell interpolation,
// escaping any embedded quote.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

type DeployGate struct{}

func (DeployGate) Name() domain.GateName    { return domain.GateDeploy }
func (DeployGate) RepairAgent() agents.Role { return agents.RoleDeployer }
func (DeployGate) Check(_ context.Context, env *GateEnv) []domain.Issue {
	hasDockerfile := false
	hasReadme := false
	for _, f := range env.Project.Files {
		if strings.HasSuffix(f.Path, "Dockerfile") {
			hasDockerfile = true
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
	return issues
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
// half-broken tree.
func DefaultGates() []Gate {
	return []Gate{
		SpecGate{}, UXGate{}, ArchGate{},
		CodeGate{}, LintGate{}, TestGate{},
		SecurityGate{}, DeployGate{},
	}
}
