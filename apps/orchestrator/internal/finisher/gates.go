package finisher

import (
	"context"
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

type SecurityGate struct{}

func (SecurityGate) Name() domain.GateName    { return domain.GateSecurity }
func (SecurityGate) RepairAgent() agents.Role { return agents.RoleSecurity }
func (SecurityGate) Check(_ context.Context, env *GateEnv) []domain.Issue {
	var issues []domain.Issue
	for _, f := range env.Project.Files {
		low := strings.ToLower(f.Content)
		if strings.Contains(low, "api_key=") || strings.Contains(low, "secret=") || strings.Contains(low, "password=") {
			issues = append(issues, domain.Issue{Gate: domain.GateSecurity, Severity: domain.SeverityCritical, Message: "possible secret in file", Path: f.Path})
		}
	}
	return issues
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

// DefaultGates returns the ordered gate list.
func DefaultGates() []Gate {
	return []Gate{SpecGate{}, UXGate{}, ArchGate{}, CodeGate{}, TestGate{}, SecurityGate{}, DeployGate{}}
}
