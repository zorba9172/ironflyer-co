package appsec

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
)

type ConfigScanner struct{}

func (ConfigScanner) ID() string { return "ironflyer-config" }

func (ConfigScanner) Supports(inv Inventory) bool {
	return inv.FileCount > 0 && (inv.HasDockerfile || inv.HasCompose || inv.HasGitHubAction || inv.HasEnvFile || inv.HasPackageJSON)
}

func (s ConfigScanner) Scan(_ context.Context, target Target, _ Inventory) ([]Finding, error) {
	var out []Finding
	for _, f := range target.Files {
		path := cleanPath(f.Path)
		low := strings.ToLower(path)
		switch {
		case strings.HasSuffix(low, "dockerfile") || strings.Contains(low, ".dockerfile"):
			out = append(out, s.scanDockerfile(path, f.Content)...)
		case strings.HasSuffix(low, "docker-compose.yml") || strings.HasSuffix(low, "docker-compose.yaml") || strings.Contains(low, "/compose/"):
			out = append(out, s.scanCompose(path, f.Content)...)
		case strings.HasPrefix(low, ".github/workflows/") && (strings.HasSuffix(low, ".yml") || strings.HasSuffix(low, ".yaml")):
			out = append(out, s.scanGitHubAction(path, f.Content)...)
		case strings.HasSuffix(low, "package.json"):
			out = append(out, s.scanPackageJSON(path, f.Content)...)
		}
	}
	return out, nil
}

func (s ConfigScanner) scanDockerfile(path, body string) []Finding {
	var out []Finding
	hasUser := false
	for i, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(strings.ToLower(line))
		switch {
		case strings.HasPrefix(trimmed, "from ") && strings.HasSuffix(trimmed, ":latest"):
			out = append(out, Finding{
				Tool:        s.ID(),
				Category:    CategoryConfig,
				Severity:    SeverityMedium,
				RuleID:      "dockerfile-unpinned-latest",
				Path:        path,
				Line:        i + 1,
				Summary:     "Docker base image uses the latest tag",
				Remediation: "pin the base image to an explicit version or digest",
			})
		case strings.HasPrefix(trimmed, "user root") || trimmed == "user 0":
			hasUser = true
			out = append(out, Finding{
				Tool:        s.ID(),
				Category:    CategoryConfig,
				Severity:    SeverityHigh,
				RuleID:      "dockerfile-root-user",
				Path:        path,
				Line:        i + 1,
				Summary:     "container explicitly runs as root",
				Remediation: "create and switch to a non-root runtime user",
			})
		case strings.HasPrefix(trimmed, "user "):
			hasUser = true
		case strings.HasPrefix(trimmed, "run ") && hasPipeToShell(trimmed):
			out = append(out, Finding{
				Tool:        s.ID(),
				Category:    CategoryConfig,
				Severity:    SeverityHigh,
				RuleID:      "dockerfile-curl-pipe-shell",
				Path:        path,
				Line:        i + 1,
				Summary:     "Docker build pipes a remote download into a shell",
				Remediation: "download, checksum, and execute pinned artifacts explicitly",
			})
		case strings.HasPrefix(trimmed, "add http://") || strings.HasPrefix(trimmed, "add https://"):
			out = append(out, Finding{
				Tool:        s.ID(),
				Category:    CategoryConfig,
				Severity:    SeverityMedium,
				RuleID:      "dockerfile-remote-add",
				Path:        path,
				Line:        i + 1,
				Summary:     "Dockerfile ADD downloads a remote URL",
				Remediation: "use explicit download, checksum verification, and COPY the verified artifact",
			})
		}
	}
	if strings.TrimSpace(body) != "" && !hasUser {
		out = append(out, Finding{
			Tool:        s.ID(),
			Category:    CategoryConfig,
			Severity:    SeverityMedium,
			RuleID:      "dockerfile-missing-non-root-user",
			Path:        path,
			Summary:     "Dockerfile does not set a non-root runtime user",
			Remediation: "create a least-privilege user and add USER before the final runtime command",
		})
	}
	return out
}

func (s ConfigScanner) scanGitHubAction(path, body string) []Finding {
	var out []Finding
	for i, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(strings.ToLower(line))
		switch {
		case strings.Contains(trimmed, "pull_request_target"):
			out = append(out, Finding{
				Tool:        s.ID(),
				Category:    CategoryConfig,
				Severity:    SeverityHigh,
				RuleID:      "config-github-action-pull-request-target",
				Path:        path,
				Line:        i + 1,
				Summary:     "workflow uses pull_request_target",
				Remediation: "avoid pull_request_target for untrusted code or tightly restrict checked-out refs and token permissions",
			})
		case strings.HasPrefix(trimmed, "permissions: write-all"):
			out = append(out, Finding{
				Tool:        s.ID(),
				Category:    CategoryConfig,
				Severity:    SeverityHigh,
				RuleID:      "config-github-action-write-all",
				Path:        path,
				Line:        i + 1,
				Summary:     "workflow grants write-all token permissions",
				Remediation: "set minimal per-scope permissions for the job",
			})
		case strings.Contains(trimmed, "uses:") && unpinnedActionRef(trimmed):
			out = append(out, Finding{
				Tool:        s.ID(),
				Category:    CategoryConfig,
				Severity:    SeverityMedium,
				RuleID:      "config-github-action-unpinned-action",
				Path:        path,
				Line:        i + 1,
				Summary:     "workflow uses an action that is not pinned to a commit SHA",
				Remediation: "pin third-party actions to immutable commit SHAs",
			})
		}
	}
	return out
}

func (s ConfigScanner) scanPackageJSON(path, body string) []Finding {
	var doc struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		return nil
	}
	var out []Finding
	for name, script := range doc.Scripts {
		lowScript := strings.ToLower(script)
		if hasPipeToShell(lowScript) {
			out = append(out, Finding{
				Tool:        s.ID(),
				Category:    CategoryConfig,
				Severity:    SeverityHigh,
				RuleID:      "config-package-script-curl-pipe-shell",
				Path:        path,
				Summary:     "npm script " + name + " pipes a remote download into a shell",
				Remediation: "download, verify, and execute pinned artifacts explicitly",
			})
		}
		if (name == "preinstall" || name == "install" || name == "postinstall") && containsNetworkFetch(lowScript) {
			out = append(out, Finding{
				Tool:        s.ID(),
				Category:    CategoryConfig,
				Severity:    SeverityMedium,
				RuleID:      "config-package-install-network-fetch",
				Path:        path,
				Summary:     "npm " + name + " script fetches remote content during install",
				Remediation: "avoid network-fetching install hooks or verify pinned artifacts before execution",
			})
		}
	}
	return out
}

func (s ConfigScanner) scanCompose(path, body string) []Finding {
	var out []Finding
	for i, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(strings.ToLower(line))
		switch {
		case strings.HasPrefix(trimmed, "privileged:") && strings.Contains(trimmed, "true"):
			out = append(out, configLineFinding(s.ID(), path, i+1, SeverityHigh, "config-compose-privileged", "compose service runs privileged", "remove privileged mode and grant only the capabilities the service needs"))
		case strings.HasPrefix(trimmed, "network_mode:") && strings.Contains(trimmed, "host"):
			out = append(out, configLineFinding(s.ID(), path, i+1, SeverityHigh, "config-compose-host-network", "compose service uses host networking", "use an isolated docker network and expose only required ports"))
		case strings.HasPrefix(trimmed, "pid:") && strings.Contains(trimmed, "host"):
			out = append(out, configLineFinding(s.ID(), path, i+1, SeverityHigh, "config-compose-host-pid", "compose service uses the host PID namespace", "remove host PID access unless it is strictly required"))
		case strings.Contains(trimmed, "/var/run/docker.sock"):
			out = append(out, configLineFinding(s.ID(), path, i+1, SeverityCritical, "config-compose-docker-sock", "compose service mounts the Docker socket", "remove Docker socket mounts or isolate them behind a purpose-built proxy"))
		}
		if inlineSecretInConfigLine(trimmed) {
			out = append(out, configLineFinding(s.ID(), path, i+1, SeverityHigh, "config-compose-inline-secret", "compose file contains an inline credential-shaped environment value", "move secrets to a secret manager or runtime-injected environment"))
		}
	}
	return out
}

func configLineFinding(toolID, path string, line int, sev Severity, ruleID, summary, remediation string) Finding {
	return Finding{
		Tool:        toolID,
		Category:    CategoryConfig,
		Severity:    sev,
		RuleID:      ruleID,
		Path:        path,
		Line:        line,
		Summary:     summary,
		Remediation: remediation,
	}
}

func hasPipeToShell(script string) bool {
	return (strings.Contains(script, "curl ") || strings.Contains(script, "wget ")) &&
		(strings.Contains(script, "| sh") || strings.Contains(script, "| bash") || strings.Contains(script, "|sh") || strings.Contains(script, "|bash"))
}

func containsNetworkFetch(script string) bool {
	return strings.Contains(script, "curl ") || strings.Contains(script, "wget ") || strings.Contains(script, "https://") || strings.Contains(script, "http://")
}

var actionRefRe = regexp.MustCompile(`uses:\s*[^@\s]+@([^#\s]+)`)
var shaRefRe = regexp.MustCompile(`^[a-f0-9]{40}$`)

func unpinnedActionRef(line string) bool {
	m := actionRefRe.FindStringSubmatch(line)
	if len(m) != 2 {
		return false
	}
	ref := strings.TrimSpace(m[1])
	if shaRefRe.MatchString(ref) {
		return false
	}
	return ref == "main" || ref == "master" || strings.HasPrefix(ref, "v") || strings.Contains(ref, "/")
}

func inlineSecretInConfigLine(line string) bool {
	if !(strings.Contains(line, "password") || strings.Contains(line, "token") || strings.Contains(line, "secret") || strings.Contains(line, "api_key") || strings.Contains(line, "apikey")) {
		return false
	}
	if strings.Contains(line, "${") || strings.Contains(line, "changeme") || strings.Contains(line, "example") {
		return false
	}
	idx := strings.IndexAny(line, ":=")
	if idx < 0 || idx == len(line)-1 {
		return false
	}
	value := strings.Trim(strings.TrimSpace(line[idx+1:]), `"'`)
	return len(value) >= 8
}
