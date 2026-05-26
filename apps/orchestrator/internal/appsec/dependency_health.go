package appsec

import (
	"context"
	"strings"
)

type DependencyHealthScanner struct{}

func (DependencyHealthScanner) ID() string { return "ironflyer-dependency-health" }

func (DependencyHealthScanner) Supports(inv Inventory) bool { return len(inv.Components) > 0 }

func (s DependencyHealthScanner) Scan(_ context.Context, _ Target, inv Inventory) ([]Finding, error) {
	var out []Finding
	for _, c := range inv.Components {
		if strings.TrimSpace(c.Deprecated) != "" {
			out = append(out, Finding{
				Tool:        s.ID(),
				Category:    CategoryDeps,
				Severity:    SeverityMedium,
				RuleID:      "deps-deprecated-package",
				Path:        c.Path,
				Package:     componentName(c),
				Summary:     c.Ecosystem + " package is deprecated: " + componentName(c),
				Remediation: strings.TrimSpace(c.Deprecated),
				Metadata: map[string]string{
					"ecosystem":  c.Ecosystem,
					"dependency": c.Name,
					"version":    c.Version,
				},
			})
		}
		if risk, ok := knownDependencyRisk(c); ok {
			out = append(out, Finding{
				Tool:        s.ID(),
				Category:    CategoryDeps,
				Severity:    risk.Severity,
				RuleID:      risk.RuleID,
				Path:        c.Path,
				Package:     componentName(c),
				Summary:     risk.Summary,
				Remediation: risk.Remediation,
				Metadata: map[string]string{
					"ecosystem":  c.Ecosystem,
					"dependency": c.Name,
					"version":    c.Version,
				},
			})
		}
	}
	return out, nil
}

type dependencyRisk struct {
	RuleID      string
	Severity    Severity
	Summary     string
	Remediation string
}

func knownDependencyRisk(c Component) (dependencyRisk, bool) {
	name := strings.ToLower(strings.TrimSpace(c.Name))
	version := normaliseVersion(c.Version)
	switch c.Ecosystem {
	case "npm":
		return knownNpmRisk(name, version)
	case "go":
		return knownGoRisk(name, version)
	default:
		return dependencyRisk{}, false
	}
}

func knownNpmRisk(name, version string) (dependencyRisk, bool) {
	switch name {
	case "flatmap-stream":
		return dependencyRisk{
			RuleID:      "deps-npm-malicious-package",
			Severity:    SeverityCritical,
			Summary:     "flatmap-stream is associated with a known npm supply-chain compromise",
			Remediation: "remove the dependency and inspect the dependency path that pulled it in",
		}, true
	case "event-stream":
		if versionInRange(version, "3.3.6", "3.3.6") {
			return dependencyRisk{
				RuleID:      "deps-npm-compromised-version",
				Severity:    SeverityCritical,
				Summary:     "event-stream 3.3.6 is a known compromised package version",
				Remediation: "remove or pin to a safe dependency tree and review generated bundles for injected code",
			}, true
		}
	case "ua-parser-js":
		if versionInList(version, "0.7.29", "0.8.0", "1.0.0") {
			return dependencyRisk{
				RuleID:      "deps-npm-compromised-version",
				Severity:    SeverityCritical,
				Summary:     "ua-parser-js version " + version + " is a known compromised npm release",
				Remediation: "upgrade to a patched version and rebuild the lockfile",
			}, true
		}
	case "colors":
		if versionInList(version, "1.4.1", "1.4.2") {
			return dependencyRisk{
				RuleID:      "deps-npm-sabotaged-version",
				Severity:    SeverityHigh,
				Summary:     "colors version " + version + " is associated with a sabotaged npm release",
				Remediation: "pin to 1.4.0 or a maintained replacement",
			}, true
		}
	case "faker":
		if version == "6.6.6" {
			return dependencyRisk{
				RuleID:      "deps-npm-sabotaged-version",
				Severity:    SeverityHigh,
				Summary:     "faker 6.6.6 is associated with a sabotaged npm release",
				Remediation: "migrate to @faker-js/faker or a maintained version",
			}, true
		}
	case "node-ipc":
		if versionInList(version, "10.1.1", "10.1.2") {
			return dependencyRisk{
				RuleID:      "deps-npm-protestware-version",
				Severity:    SeverityHigh,
				Summary:     "node-ipc version " + version + " is associated with protestware behavior",
				Remediation: "upgrade or replace node-ipc and rebuild the lockfile",
			}, true
		}
	case "request", "node-sass", "babel-eslint", "tslint":
		return dependencyRisk{
			RuleID:      "deps-npm-unmaintained-package",
			Severity:    SeverityMedium,
			Summary:     name + " is deprecated or unmaintained",
			Remediation: "replace it with a maintained package and refresh the lockfile",
		}, true
	case "core-js":
		if semverLess(version, "3.0.0") {
			return dependencyRisk{
				RuleID:      "deps-npm-legacy-runtime-package",
				Severity:    SeverityMedium,
				Summary:     "core-js v2 is deprecated and no longer a healthy runtime dependency",
				Remediation: "upgrade to core-js v3 through the direct or transitive dependency that requires it",
			}, true
		}
	case "lodash":
		if semverLess(version, "4.17.21") {
			return dependencyRisk{
				RuleID:      "deps-npm-vulnerable-range",
				Severity:    SeverityHigh,
				Summary:     "lodash below 4.17.21 is in a known vulnerable range",
				Remediation: "upgrade lodash to 4.17.21 or newer",
			}, true
		}
	case "minimist":
		if semverLess(version, "1.2.6") {
			return dependencyRisk{
				RuleID:      "deps-npm-vulnerable-range",
				Severity:    SeverityHigh,
				Summary:     "minimist below 1.2.6 is in a known vulnerable range",
				Remediation: "upgrade minimist to 1.2.6 or newer",
			}, true
		}
	case "jsonwebtoken":
		if semverLess(version, "9.0.0") {
			return dependencyRisk{
				RuleID:      "deps-npm-vulnerable-range",
				Severity:    SeverityHigh,
				Summary:     "jsonwebtoken below 9.0.0 has multiple known security advisories",
				Remediation: "upgrade jsonwebtoken to 9.x and re-check signing/verification options",
			}, true
		}
	}
	return dependencyRisk{}, false
}

func knownGoRisk(name, version string) (dependencyRisk, bool) {
	switch name {
	case "github.com/dgrijalva/jwt-go":
		return dependencyRisk{
			RuleID:      "deps-go-abandoned-security-package",
			Severity:    SeverityHigh,
			Summary:     "github.com/dgrijalva/jwt-go is abandoned and affected by known JWT validation risks",
			Remediation: "migrate to github.com/golang-jwt/jwt/v5 and re-check token validation paths",
		}, true
	case "github.com/satori/go.uuid":
		return dependencyRisk{
			RuleID:      "deps-go-unmaintained-package",
			Severity:    SeverityMedium,
			Summary:     "github.com/satori/go.uuid is unmaintained",
			Remediation: "migrate to github.com/google/uuid",
		}, true
	case "github.com/golang/protobuf":
		return dependencyRisk{
			RuleID:      "deps-go-deprecated-package",
			Severity:    SeverityLow,
			Summary:     "github.com/golang/protobuf is deprecated",
			Remediation: "migrate to google.golang.org/protobuf",
		}, true
	case "gopkg.in/yaml.v2":
		if semverLess(version, "2.2.8") {
			return dependencyRisk{
				RuleID:      "deps-go-vulnerable-range",
				Severity:    SeverityHigh,
				Summary:     "gopkg.in/yaml.v2 below 2.2.8 is in a known vulnerable range",
				Remediation: "upgrade gopkg.in/yaml.v2 to 2.2.8 or newer",
			}, true
		}
	}
	return dependencyRisk{}, false
}

func componentName(c Component) string {
	if c.Version == "" {
		return c.Name
	}
	return c.Name + "@" + c.Version
}
