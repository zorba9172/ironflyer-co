# AppSec Coverage Runbook

Use this when adding a new scanner, rule, or open-source tool adapter.

## Rule For Customer Boundaries

Every scanner must answer:

1. What exact user project files does it read?
2. Does it execute only inside the customer runtime workspace?
3. Does it send package names, source, paths, or secrets to a third party?
4. Is it disabled or opt-in when network disclosure exists?
5. Does it degrade cleanly when the binary/API is unavailable?

If the answer is unclear, keep it out of the default path.

## Add A Native Rule

1. Add rule code under `apps/orchestrator/internal/appsec`.
2. Emit `Finding` with a stable `RuleID`.
3. Set `Category`, `Severity`, `Path`, `Package`, `Summary`, `Remediation`.
4. Add a test in `appsec_test.go`.
5. Verify:

```bash
cd apps/orchestrator
go test ./internal/appsec ./internal/finisher
```

## Add A Runtime Adapter

1. Run only through `target.Runtime.Exec`.
2. Use `target.WorkspaceID`; never host paths.
3. Keep timeouts small.
4. Treat missing binary as no findings.
5. Parse JSON output when possible.
6. Add a parser unit test with fixture JSON.

## Add A Network Enricher

1. Make it opt-in via `.ironflyer/appsec.json`.
2. Send package metadata only, never source.
3. Use strict timeout and size limit.
4. Never fail the gate because the network service is down.
5. Cache before enabling by default.

## Recommended Scanner Stack

Default native:

- Ironflyer secrets
- Dependency health
- Config/IaC rules
- SBOM/SARIF export

Optional workspace:

- `govulncheck`
- `npm audit`
- `semgrep`
- `gitleaks`
- `trufflehog`
- `syft`
- `grype`
- `trivy`
- `checkov`
- `hadolint`
- `zizmor`

Optional network:

- OSV API
- deps.dev
- OpenSSF Scorecard

