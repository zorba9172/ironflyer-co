package finisher

import (
	"context"
	"strings"

	"ironflyer/apps/orchestrator/internal/agents"
	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/runtime"
)

// MobileExpoDoctorGate runs `npx expo-doctor` inside the bound
// workspace and turns each "Check failed:" line into a domain.Issue.
// Without a runtime it degrades to a static check that confirms
// expo-doctor is at least listed in package.json devDependencies.
//
// Only fires for MobileKindExpo / MobileKindReactNativeBare —
// expo-doctor is JS-stack-specific.
type MobileExpoDoctorGate struct{}

func (MobileExpoDoctorGate) Name() domain.GateName    { return domain.GateMobileExpoDoctor }
func (MobileExpoDoctorGate) RepairAgent() agents.Role { return agents.RoleMobileCoder }

func (MobileExpoDoctorGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	if env == nil || env.Project == nil || !projectIsMobile(env.Project) {
		return nil
	}
	kind := projectMobileKind(env.Project)
	if kind != domain.MobileKindExpo && kind != domain.MobileKindReactNativeBare {
		// JS-stack gate — Flutter / native projects skip cleanly.
		return nil
	}

	if !env.HasRuntime() {
		return expoDoctorStaticFallback(env.Project)
	}

	// Live run. expo-doctor exits non-zero on warnings; we explicitly
	// don't fail the gate on exec error — only on parsed findings.
	cmd := "npx --yes expo-doctor"
	res, err := env.Runtime.Exec(ctx, env.UserBearer, env.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 120,
	})
	if err != nil {
		return []domain.Issue{{
			Gate: domain.GateMobileExpoDoctor, Severity: domain.SeverityWarning,
			Message: "expo-doctor exec error: " + err.Error(),
			Hint:    "verify the workspace has npx + the expo-doctor dependency installed",
		}}
	}
	if res.TimedOut {
		return []domain.Issue{{
			Gate: domain.GateMobileExpoDoctor, Severity: domain.SeverityCritical,
			Message: "expo-doctor timed out after 120s",
			Hint:    "Metro / EAS sub-checks may be blocking on a missing native module",
		}}
	}
	return parseExpoDoctorOutput(res.Stdout + "\n" + res.Stderr)
}

// expoDoctorStaticFallback emits a SeverityInfo nudge when expo-doctor
// isn't even listed in package.json. Returning a single InfoIssue is
// intentional: the goal is visibility on the dashboard, not blocking
// the loop on a self-hosted box without a runtime.
func expoDoctorStaticFallback(p *domain.Project) []domain.Issue {
	body, ok := fileBody(p, "package.json")
	if !ok {
		return []domain.Issue{{
			Gate: domain.GateMobileExpoDoctor, Severity: domain.SeverityInfo,
			Message: "package.json missing — cannot statically verify expo-doctor wiring",
		}}
	}
	if strings.Contains(body, "\"expo-doctor\"") {
		return nil
	}
	return []domain.Issue{{
		Gate: domain.GateMobileExpoDoctor, Severity: domain.SeverityInfo,
		Message: "wire expo-doctor for richer mobile diagnostics",
		Hint:    "npm install --save-dev expo-doctor; the gate runs it whenever a workspace is attached",
		Path:    "package.json",
	}}
}

// parseExpoDoctorOutput scans expo-doctor stdout/stderr for
// "Check failed:" lines and projects each into a domain.Issue. Native-
// dep mismatches are escalated to SeverityError because they break the
// EAS build; everything else is SeverityWarning.
func parseExpoDoctorOutput(body string) []domain.Issue {
	var issues []domain.Issue
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		low := strings.ToLower(line)
		if !strings.Contains(low, "check failed") {
			continue
		}
		sev := domain.SeverityWarning
		// Heuristic: native deps drift breaks the build outright.
		if strings.Contains(low, "native dependencies") ||
			strings.Contains(low, "mismatched") ||
			strings.Contains(low, "package versions") ||
			strings.Contains(low, "incompatible") {
			sev = domain.SeverityError
		}
		issues = append(issues, domain.Issue{
			Gate: domain.GateMobileExpoDoctor, Severity: sev,
			Message: line,
			Hint:    "run `npx expo-doctor` locally for the verbose remediation block",
		})
		if len(issues) >= 20 {
			break
		}
	}
	return issues
}
