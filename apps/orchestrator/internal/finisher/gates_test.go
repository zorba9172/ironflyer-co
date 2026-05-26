package finisher

import (
	"context"
	"strings"
	"testing"

	"ironflyer/apps/orchestrator/internal/appsec"
	"ironflyer/apps/orchestrator/internal/domain"
)

func TestSecurityGate_CatchesHighConfidenceSecrets(t *testing.T) {
	// Fixtures are assembled at runtime from harmless prefixes + obviously
	// fake bodies. Building them with `+` keeps GitHub push protection's
	// pattern matcher from flagging the test source while still exercising
	// the scanner's regexes end-to-end.
	cases := map[string]string{
		"AWS access key":                "AKIA" + strings.Repeat("X", 16),
		"GitHub personal access token":  "gh" + "p_" + strings.Repeat("x", 36),
		"Stripe secret key":             "sk" + "_test_" + strings.Repeat("0", 24),
		"OpenAI API key":                "s" + "k-" + strings.Repeat("z", 40),
		"Anthropic API key":             "s" + "k-ant-" + strings.Repeat("a", 40),
		"Google API key":                "AIza" + strings.Repeat("Z", 35),
		"Slack token":                   "x" + "oxb-1111111111-2222222222222-" + strings.Repeat("F", 24),
		"Private key block":             "-----BEGIN RSA PRIVATE KEY-----\nfake-body\n-----END RSA PRIVATE KEY-----",
		"URL with embedded credentials": "postgres://user:" + "h" + "unter2@db.example.com:5432/app",
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			issues := SecurityGate{}.Check(context.Background(), &GateEnv{
				Project: &domain.Project{Files: []domain.FileNode{{Path: "config.yaml", Content: payload}}},
			})
			if len(issues) == 0 {
				t.Fatalf("expected %s to be detected, got 0 issues", name)
			}
			found := false
			for _, iss := range issues {
				if strings.Contains(iss.Message, name) {
					found = true
				}
				if iss.Severity != domain.SeverityCritical {
					t.Errorf("expected Critical, got %s for %s", iss.Severity, iss.Message)
				}
			}
			if !found {
				t.Errorf("issues did not name %s; got %+v", name, issues)
			}
		})
	}
}

func TestSecurityGate_IgnoresNoise(t *testing.T) {
	// Documentation snippets shouldn't fire — they look like config but use
	// example/placeholder values that don't match our high-confidence
	// patterns.
	for _, body := range []string{
		"# Set ANTHROPIC_API_KEY in your .env",
		"placeholder=<your-secret-here>",
		"const example = 'short'", // assignment too short to match
	} {
		issues := SecurityGate{}.Check(context.Background(), &GateEnv{
			Project: &domain.Project{Files: []domain.FileNode{{Path: "README.md", Content: body}}},
		})
		// Suspicious assignment is warning-only; documentation snippets above
		// don't hit any high-confidence pattern.
		for _, iss := range issues {
			if iss.Severity == domain.SeverityCritical {
				t.Errorf("documentation triggered Critical issue: %q in %q", iss.Message, body)
			}
		}
	}
}

func TestSecurityGate_FlagsSuspiciousAssignmentsAsWarning(t *testing.T) {
	body := `password = "p@ssw0rd123"`
	issues := SecurityGate{}.Check(context.Background(), &GateEnv{
		Project: &domain.Project{Files: []domain.FileNode{{Path: "app.py", Content: body}}},
	})
	found := false
	for _, iss := range issues {
		if iss.Severity == domain.SeverityWarning && strings.Contains(iss.Message, "suspicious") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a warning-level suspicious assignment issue, got %+v", issues)
	}
}

func TestShouldSkipForSecrets(t *testing.T) {
	cases := map[string]bool{
		"src/.git/config":        true,
		".git/HEAD":              true,
		"node_modules/foo/x.js":  true,
		"deeply/node_modules/x":  true,
		"package-lock.json":      true,
		"go.sum":                 true,
		"src/main.go":            false,
		"infra/k8s/ingress.yaml": false,
	}
	for path, want := range cases {
		if got := appsec.ShouldSkipSecretsPath(path); got != want {
			t.Errorf("ShouldSkipSecretsPath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestLintGate_PassesWithoutRuntime(t *testing.T) {
	p := &domain.Project{Spec: domain.ProductSpec{Stack: domain.StackDecision{Backend: "go"}}}
	issues := LintGate{}.Check(context.Background(), &GateEnv{Project: p})
	if len(issues) != 0 {
		t.Fatalf("LintGate without runtime should pass, got %+v", issues)
	}
}

func TestDetectBuildAndTestCommands(t *testing.T) {
	cases := []struct {
		name      string
		project   domain.Project
		wantBuild string
		wantTest  string
	}{
		{
			name: "go via stack",
			project: domain.Project{Spec: domain.ProductSpec{
				Stack: domain.StackDecision{Backend: "go-chi"},
			}},
			wantBuild: "go build", wantTest: "go test",
		},
		{
			name:      "node via package.json",
			project:   domain.Project{Files: []domain.FileNode{{Path: "package.json"}}},
			wantBuild: "npm", wantTest: "npm",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotBuild, _, _ := detectBuildCommand(&tc.project)
			if !strings.Contains(gotBuild, tc.wantBuild) {
				t.Errorf("build cmd %q missing %q", gotBuild, tc.wantBuild)
			}
			gotTest, _, _ := detectTestCommand(&tc.project)
			if !strings.Contains(gotTest, tc.wantTest) {
				t.Errorf("test cmd %q missing %q", gotTest, tc.wantTest)
			}
		})
	}
}
