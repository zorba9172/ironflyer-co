# Ironflyer AppSec Core

This document tracks the built-in AppSec capability that protects customer
projects without scanning or exposing Ironflyer's own orchestration code.

## Boundary

AppSec scans only the user project target:

- `appsec.Target.Files`, produced from the customer project.
- The bound runtime workspace for that customer project, when one exists.
- Optional network enrichers only when explicitly enabled in
  `.ironflyer/appsec.json`.

It must not scan:

- The Ironflyer orchestrator repository.
- Host filesystem paths outside the runtime workspace.
- Internal provider credentials, tenant secrets, logs, or memory.
- Ironflyer dependency lists, except during local developer tests.

## What Exists Now

Package: `apps/orchestrator/internal/appsec`

Core:

- `Engine`: scanner orchestration, config, dedupe, verdict, risk graph.
- `Inventory`: file/stack discovery and component extraction.
- `Finding`: canonical issue shape for secrets, deps, code, config, policy.
- `RiskGraph`: project/service/file/package/finding graph.
- `Policy`: blocking thresholds and score.
- `Config`: `.ironflyer/appsec.json` with excludes, disabled scanners,
  severity overrides, waivers, and opt-in network scanners.

Finisher integration:

- `SecurityGate` delegates to `appsec.DefaultEngine()`.
- Finisher maps `appsec.Finding` to `domain.Issue`.
- Existing `gate.security.finding.v1` event projection continues to feed the
  security report.

Dashboard intelligence:

- `apps/web/src/lib/projectIntelligence.ts` derives a compact project profile
  from project files.
- `DashboardPane` shows stack, language percentages, and meaningful components
  without adding a noisy table.

## Current Coverage

| Area | Implemented Coverage | Source |
| --- | --- | --- |
| Secrets | Native high-confidence patterns, high-entropy credential assignments, runtime grep, optional Gitleaks/TruffleHog adapters | Native + optional OSS CLI |
| Dependencies | SBOM components from `go.mod` and `package-lock.json`; deprecated npm metadata; known risky package/version rules; optional `govulncheck`; optional `npm audit`; optional OSV API | Native + Go/npm/OSV |
| SBOM | CycloneDX JSON export using `github.com/CycloneDX/cyclonedx-go`; Package URL identifiers via `github.com/package-url/packageurl-go` | OWASP CycloneDX + PURL |
| Go deprecations | Optional `go list -m -u -json all` runtime enrichment for customer workspace modules | Go toolchain |
| SAST | Optional Semgrep runtime adapter | Semgrep CLI |
| Containers | Native Dockerfile checks: `latest`, missing/root user, remote `ADD`, curl/wget pipe-to-shell | Native |
| Compose | privileged, host network/PID, Docker socket, inline secrets | Native |
| GitHub Actions | `pull_request_target`, `permissions: write-all`, unpinned actions | Native |
| npm scripts | remote install fetches, curl/wget pipe-to-shell | Native |
| Reporting | SARIF export and security report projection | Native |

## Installed Libraries

These were added to avoid building standards from scratch:

- `github.com/CycloneDX/cyclonedx-go`
  - Apache-2.0
  - Used to produce standards-compliant CycloneDX SBOMs.
  - Project: https://github.com/CycloneDX/cyclonedx-go
- `github.com/package-url/packageurl-go`
  - Package URL implementation.
  - Used for package identifiers in SBOMs and vulnerability correlation.
  - Project: https://github.com/package-url/packageurl-go

## Optional Open Source Tool Adapters

The engine is designed to use these when they exist in the customer workspace
image. Missing binaries are non-fatal.

| Tool | Purpose | Default Posture |
| --- | --- | --- |
| `govulncheck` | Go reachable vulnerability analysis | Best-effort runtime adapter |
| `npm audit` | npm advisory scan from lockfile | Best-effort runtime adapter |
| `semgrep` | SAST rules | Best-effort runtime adapter |
| `gitleaks` | Secret scanning | Best-effort runtime adapter |
| `trufflehog` | Secret verification | Best-effort runtime adapter |

Recommended next optional adapters:

| Tool | Why | Notes |
| --- | --- | --- |
| OSV-Scanner | Broad ecosystem vulnerability scan, license scanning | Prefer CLI in workspace for full lockfile support; native OSV API already exists for opt-in network mode |
| Syft | Rich SBOM for filesystems/images | Use as optional workspace adapter; keep CycloneDX library for native export |
| Grype | Vulnerability scan from SBOM | Pair with Syft artifacts |
| Trivy | Container/IaC/secrets/vulns | Use carefully and pin binary/action versions because scanners are themselves supply-chain surface |
| Checkov | Terraform/Kubernetes/CloudFormation/IaC | Good when generated projects include infra-as-code |
| Hadolint | Dockerfile quality/security | Already easy to add as deploy/config signal |
| zizmor | GitHub Actions hardening | Useful for workflow-specific risk |

## Network Privacy

Network scanners can disclose package names and versions to external services.
For that reason, OSV API scanning is opt-in:

```json
{
  "enableNetworkScanners": true
}
```

Without this flag, AppSec still runs native checks, deprecated/risky package
rules, SBOM generation, and local workspace tools.

## Waivers

Waivers are explicit and should include a reason and expiry:

```json
{
  "waivers": [
    {
      "ruleId": "dockerfile-missing-non-root-user",
      "path": "Dockerfile",
      "reason": "builder-only image",
      "expiresAt": "2026-06-30T00:00:00Z"
    }
  ]
}
```

## Remaining Coverage To Build

High priority:

- OSV native cache and rate limits.
- License policy from SBOM components.
- Python dependency parsing: `requirements.txt`, `pyproject.toml`,
  `poetry.lock`, `uv.lock`.
- pnpm/yarn lock parsing.
- Container image scanner adapter: Syft + Grype or Trivy.
- GitHub Actions deep analysis with pinned action validation and risky
  `GITHUB_TOKEN` flows.
- Dependency drift between executions.
- Secret baseline so historical known-test fixtures do not create noise.

Medium priority:

- OpenSSF Scorecard adapter for public GitHub repos.
- SLSA/provenance checks for release workflows.
- VEX support for accepted/not-affected vulnerabilities.
- License compatibility policy per tenant.
- Malware/squatting heuristics: typosquatting, brandjacking, new package age.
- Maintainer health: archived repos, low activity, sudden ownership changes.

Do not build first:

- Full DAST platform.
- Full cloud posture management.
- Enterprise-grade dependency graph UI.
- Heavy scanner libraries embedded directly into orchestrator if they pull a
  large transitive tree. Prefer runtime adapters for those.

## Product UX Principle

The dashboard should show only:

- Main stack.
- Language percentages.
- Important components.
- 1-2 risk hints when actionable.

Everything else belongs in a drilldown or report artifact.

