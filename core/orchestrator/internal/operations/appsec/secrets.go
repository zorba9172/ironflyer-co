package appsec

import (
	"context"
	"math"
	"regexp"
	"strings"

	"ironflyer/core/orchestrator/internal/operations/runtime"
)

type secretPattern struct {
	Name   string
	RuleID string
	Re     *regexp.Regexp
}

var secretPatterns = []secretPattern{
	{Name: "AWS access key", RuleID: "secret-aws-access-key", Re: regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{Name: "AWS secret access key", RuleID: "secret-aws-secret-access-key", Re: regexp.MustCompile(`(?i)aws(.{0,20})?(secret|access).{0,20}?['"]([0-9a-zA-Z/+=]{40})['"]`)},
	{Name: "GitHub personal access token", RuleID: "secret-github-pat", Re: regexp.MustCompile(`\bghp_[0-9A-Za-z]{36}\b`)},
	{Name: "GitHub OAuth token", RuleID: "secret-github-oauth", Re: regexp.MustCompile(`\bgho_[0-9A-Za-z]{36}\b`)},
	{Name: "GitHub fine-grained PAT", RuleID: "secret-github-fine-grained-pat", Re: regexp.MustCompile(`\bgithub_pat_[0-9A-Za-z_]{82}\b`)},
	{Name: "Stripe secret key", RuleID: "secret-stripe-secret-key", Re: regexp.MustCompile(`\bsk_(live|test)_[0-9a-zA-Z]{20,}\b`)},
	{Name: "Stripe restricted key", RuleID: "secret-stripe-restricted-key", Re: regexp.MustCompile(`\brk_(live|test)_[0-9a-zA-Z]{20,}\b`)},
	{Name: "OpenAI API key", RuleID: "secret-openai-api-key", Re: regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{32,}\b`)},
	{Name: "Anthropic API key", RuleID: "secret-anthropic-api-key", Re: regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_-]{32,}\b`)},
	{Name: "Google API key", RuleID: "secret-google-api-key", Re: regexp.MustCompile(`\bAIza[0-9A-Za-z\-_]{35}\b`)},
	{Name: "Slack token", RuleID: "secret-slack-token", Re: regexp.MustCompile(`\bxox[abprs]-[0-9A-Za-z-]{10,}\b`)},
	{Name: "Private key block", RuleID: "secret-private-key-block", Re: regexp.MustCompile(`-----BEGIN (RSA|EC|OPENSSH|PGP|DSA) PRIVATE KEY-----`)},
	{Name: "JWT", RuleID: "secret-jwt", Re: regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`)},
	{Name: "URL with embedded credentials", RuleID: "secret-url-embedded-credentials", Re: regexp.MustCompile(`\b(https?|postgres|mysql|mongodb|redis):\/\/[^\s:@/]+:[^\s:@/]+@`)},
}

var suspiciousAssignments = regexp.MustCompile(`(?im)^[^#/]*\b(password|passwd|api[_-]?key|secret|token)\s*[:=]\s*['"][^'"\s]{6,}['"]`)
var highEntropyAssignments = regexp.MustCompile(`(?im)^[^#/]*\b(password|passwd|api[_-]?key|secret|token|credential)\s*[:=]\s*['"]([A-Za-z0-9_./+=-]{24,})['"]`)

type NativeSecretScanner struct{}

func (NativeSecretScanner) ID() string { return "ironflyer-native-secrets" }

func (NativeSecretScanner) Supports(inv Inventory) bool { return inv.FileCount > 0 }

func (s NativeSecretScanner) Scan(_ context.Context, target Target, _ Inventory) ([]Finding, error) {
	var out []Finding
	for _, f := range target.Files {
		out = append(out, s.ScanContent(f.Path, f.Content)...)
	}
	return out, nil
}

func (s NativeSecretScanner) ScanContent(path, content string) []Finding {
	path = cleanPath(path)
	if content == "" || ShouldSkipSecretsPath(path) {
		return nil
	}
	var out []Finding
	seen := map[string]bool{}
	for _, p := range secretPatterns {
		loc := p.Re.FindStringIndex(content)
		if loc == nil || seen[p.RuleID] {
			continue
		}
		seen[p.RuleID] = true
		out = append(out, Finding{
			Tool:        s.ID(),
			Category:    CategorySecrets,
			Severity:    SeverityCritical,
			RuleID:      p.RuleID,
			Path:        path,
			Line:        lineForOffset(content, loc[0]),
			Summary:     "found " + p.Name,
			Remediation: "remove the credential, rotate it upstream, and store the replacement outside source",
		})
	}
	if !seen["secret-jwt"] {
		loc := suspiciousAssignments.FindStringIndex(content)
		if loc != nil {
			out = append(out, Finding{
				Tool:        s.ID(),
				Category:    CategorySecrets,
				Severity:    SeverityMedium,
				RuleID:      "secret-suspicious-assignment",
				Path:        path,
				Line:        lineForOffset(content, loc[0]),
				Summary:     "suspicious credential-shaped assignment",
				Remediation: "verify this is not a hardcoded secret; move real credentials to the project secret store",
			})
		}
	}
	for _, match := range highEntropyAssignments.FindAllStringSubmatchIndex(content, -1) {
		if len(match) < 6 {
			continue
		}
		value := content[match[4]:match[5]]
		if looksLikePlaceholder(value) || shannonEntropy(value) < 4.2 {
			continue
		}
		out = append(out, Finding{
			Tool:        s.ID(),
			Category:    CategorySecrets,
			Severity:    SeverityHigh,
			RuleID:      "secret-high-entropy-assignment",
			Path:        path,
			Line:        lineForOffset(content, match[0]),
			Summary:     "high-entropy credential-shaped assignment",
			Remediation: "treat as a likely secret, rotate if real, and move it to the project secret store",
		})
		break
	}
	return out
}

func looksLikePlaceholder(value string) bool {
	low := strings.ToLower(value)
	for _, marker := range []string{"example", "placeholder", "changeme", "your_", "dummy", "fake", "test"} {
		if strings.Contains(low, marker) {
			return true
		}
	}
	return false
}

func shannonEntropy(value string) float64 {
	if value == "" {
		return 0
	}
	counts := map[rune]float64{}
	for _, r := range value {
		counts[r]++
	}
	total := float64(len(value))
	var entropy float64
	for _, count := range counts {
		p := count / total
		entropy -= p * math.Log2(p)
	}
	return entropy
}

func ShouldSkipSecretsPath(path string) bool {
	low := strings.ToLower(cleanPath(path))
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

type RuntimeSecretScanner struct{}

func (RuntimeSecretScanner) ID() string { return "ironflyer-runtime-secrets" }

func (RuntimeSecretScanner) Supports(inv Inventory) bool { return inv.RuntimeEnabled }

func (s RuntimeSecretScanner) Scan(ctx context.Context, target Target, _ Inventory) ([]Finding, error) {
	if !target.HasRuntime() {
		return nil, nil
	}
	const coarse = `'AKIA[A-Z0-9]|ghp_|gho_|github_pat_|sk_live_|sk_test_|sk-ant-|BEGIN [A-Z]\+ PRIVATE KEY|eyJ[A-Za-z0-9_-]\{10,\}\.eyJ'`
	listCmd := "grep -RIlE --binary-files=without-match --exclude-dir=.git --exclude-dir=node_modules " + coarse + " . 2>/dev/null | head -50"
	res, err := target.Runtime.Exec(ctx, target.UserBearer, target.WorkspaceID, runtime.ExecOpts{
		Shell: listCmd, TimeoutSeconds: 60,
	})
	if err != nil || res.ExitCode > 1 {
		return nil, nil
	}
	native := NativeSecretScanner{}
	var out []Finding
	for _, path := range strings.Split(strings.TrimSpace(res.Stdout), "\n") {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		body, err := target.Runtime.Exec(ctx, target.UserBearer, target.WorkspaceID, runtime.ExecOpts{
			Shell: "head -c 262144 -- " + shellQuote(path), TimeoutSeconds: 20,
		})
		if err != nil || body.ExitCode != 0 {
			continue
		}
		for _, f := range native.ScanContent(strings.TrimPrefix(path, "./"), body.Stdout) {
			f.Tool = s.ID()
			out = append(out, f)
		}
	}
	return out, nil
}
