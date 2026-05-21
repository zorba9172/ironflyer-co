package finisher

import (
	"strings"

	"ironflyer/apps/orchestrator/internal/agents"
	"ironflyer/apps/orchestrator/internal/domain"
)

// Gate is the contract every completion gate implements. Each gate inspects
// the project and reports issues. When issues are non-empty the orchestrator
// dispatches the gate's repair agent.
type Gate interface {
	Name() domain.GateName
	RepairAgent() agents.Role
	Check(p *domain.Project) []domain.Issue
}

type SpecGate struct{}

func (SpecGate) Name() domain.GateName    { return domain.GateSpec }
func (SpecGate) RepairAgent() agents.Role { return agents.RolePlanner }
func (SpecGate) Check(p *domain.Project) []domain.Issue {
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
func (UXGate) Check(p *domain.Project) []domain.Issue {
	if len(p.Spec.UserStories) == 0 {
		return []domain.Issue{{Gate: domain.GateUX, Severity: domain.SeverityWarning, Message: "blocked: spec has no stories"}}
	}
	// Stub: real check inspects design tokens + screen map.
	return nil
}

type ArchGate struct{}

func (ArchGate) Name() domain.GateName    { return domain.GateArch }
func (ArchGate) RepairAgent() agents.Role { return agents.RoleArchitect }
func (ArchGate) Check(p *domain.Project) []domain.Issue {
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
func (CodeGate) Check(p *domain.Project) []domain.Issue {
	if len(p.Files) == 0 {
		return []domain.Issue{{Gate: domain.GateCode, Severity: domain.SeverityError, Message: "no source files yet"}}
	}
	return nil
}

type TestGate struct{}

func (TestGate) Name() domain.GateName    { return domain.GateTest }
func (TestGate) RepairAgent() agents.Role { return agents.RoleTester }
func (TestGate) Check(p *domain.Project) []domain.Issue {
	hasTest := false
	for _, f := range p.Files {
		if strings.Contains(f.Path, "_test.") || strings.Contains(f.Path, ".test.") || strings.Contains(f.Path, "/tests/") {
			hasTest = true
			break
		}
	}
	if !hasTest {
		return []domain.Issue{{Gate: domain.GateTest, Severity: domain.SeverityError, Message: "no tests detected"}}
	}
	return nil
}

type SecurityGate struct{}

func (SecurityGate) Name() domain.GateName    { return domain.GateSecurity }
func (SecurityGate) RepairAgent() agents.Role { return agents.RoleSecurity }
func (SecurityGate) Check(p *domain.Project) []domain.Issue {
	var issues []domain.Issue
	for _, f := range p.Files {
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
func (DeployGate) Check(p *domain.Project) []domain.Issue {
	hasDockerfile := false
	hasReadme := false
	for _, f := range p.Files {
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

// DefaultGates returns the ordered gate list.
func DefaultGates() []Gate {
	return []Gate{SpecGate{}, UXGate{}, ArchGate{}, CodeGate{}, TestGate{}, SecurityGate{}, DeployGate{}}
}
