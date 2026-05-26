#!/usr/bin/env bash
# Ironflyer — Anti-Bloat: govulncheck driver.
#
# Wires the `vuln_scan` gate (see core/orchestrator/internal/ai/finisher/
# gates_antibloat.go). govulncheck is the standard Go vuln scanner; it
# is NOT a project dep. We install it on demand via `go install` and
# emit a combined JSON report for the orchestrator + runtime Go
# modules.
#
# Outputs:
#   tmp/reports/govulncheck-<timestamp>.json   (combined JSON)
#   stdout one-liner: "vuln_scan: N findings (H high, M medium, L low) → <path>"
#
# Operator wiring:
#   ./scripts/lint/run-govulncheck.sh
#   export IRONFLYER_VULN_REPORT_PATH=/abs/path/to/govulncheck-<ts>.json
#
# Exit codes:
#   0  scan completed (even if findings exist — gate decides severity)
#   1  govulncheck binary missing AND `go install` failed
#   2  upstream scan errored on every target

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
REPORT_DIR="${REPO_ROOT}/tmp/reports"
mkdir -p "${REPORT_DIR}"

TS="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="${REPORT_DIR}/govulncheck-${TS}.json"

# Ensure govulncheck is available. The tool is installed once into
# $GOBIN (or $HOME/go/bin); we never add it as a project dep.
if ! command -v govulncheck >/dev/null 2>&1; then
  echo "govulncheck not found — installing via go install (one-time)" >&2
  if ! go install golang.org/x/vuln/cmd/govulncheck@latest 2>&1; then
    echo "FATAL: go install golang.org/x/vuln/cmd/govulncheck@latest failed" >&2
    exit 1
  fi
fi

# go install writes to $GOBIN or $HOME/go/bin; make sure PATH sees it.
GOBIN_PATH="$(go env GOBIN)"
if [ -z "${GOBIN_PATH}" ]; then
  GOBIN_PATH="$(go env GOPATH)/bin"
fi
case ":${PATH}:" in
  *":${GOBIN_PATH}:"*) ;;
  *) export PATH="${GOBIN_PATH}:${PATH}" ;;
esac

# Build a combined report. Each module contributes a JSON object;
# we concatenate them into a top-level `{ "findings": [...] }` shape
# that parseEvidenceReport (gates_antibloat.go) understands.
TMP_RAW="$(mktemp -d)"
trap 'rm -rf "${TMP_RAW}"' EXIT

run_one() {
  local mod="$1"
  local out_file="${TMP_RAW}/${mod}.json"
  if [ ! -d "${REPO_ROOT}/core/${mod}" ]; then
    echo "skipping core/${mod} (not a directory)" >&2
    return 0
  fi
  echo "running govulncheck in core/${mod}..." >&2
  # govulncheck emits NDJSON-ish stream of events on -json. Capture
  # raw output; we transform afterward. It exits non-zero when
  # vulnerabilities are found — that is informational, not fatal.
  ( cd "${REPO_ROOT}/core/${mod}" && govulncheck -json ./... ) > "${out_file}" 2>/dev/null || true
  if [ ! -s "${out_file}" ]; then
    echo "WARN: no output from govulncheck in core/${mod}" >&2
    return 1
  fi
}

ANY_OK=0
for mod in orchestrator runtime; do
  if run_one "${mod}"; then
    ANY_OK=1
  fi
done

if [ "${ANY_OK}" -eq 0 ]; then
  echo "FATAL: govulncheck produced no output for any module" >&2
  exit 2
fi

# Transform NDJSON event stream into the
# { "findings": [ { path, message, severity }, ... ] } shape the
# orchestrator's evidence-stub gates parse.
#
# govulncheck emits a stream of `{"osv":{...}}` and `{"finding":{...}}`
# events. We extract findings + osv severity. When jq is missing we
# degrade to a single warning entry so the gate stays informative.
if command -v jq >/dev/null 2>&1; then
  # Combine all module event streams, then project into our schema.
  # We treat any finding whose osv has severity DATABASE_SPECIFIC.severity
  # or affected[].database_specific.severity as that severity; default
  # to "medium" when absent.
  cat "${TMP_RAW}"/*.json | jq -s '
    # Step 1: collect osv records into a map keyed by id.
    (map(select(.osv) | .osv) | reduce .[] as $o ({}; . + { ($o.id // "unknown"): $o })) as $osvs
    | (map(select(.finding) | .finding) | unique_by(.osv)) as $findings
    | { findings: ( $findings | map({
        path: (.trace[0].module // .trace[0].package // "unknown"),
        message: ( ($osvs[.osv].summary // .osv // "vulnerability") + " — " + ($osvs[.osv].details // "" | .[0:200])),
        severity: (
          ( ($osvs[.osv].database_specific.severity // "")
            | ascii_downcase )
          | if . == "" then "medium" else . end
        )
      })) }
  ' > "${OUT}" || {
    # If jq projection fails (schema drift), fall back to a
    # SeverityWarning sentinel so the gate still has SOME signal.
    printf '{"findings":[{"path":"core/","message":"govulncheck completed but jq projection failed; raw at %s","severity":"warning"}]}\n' \
      "${TMP_RAW}" > "${OUT}"
  }
else
  # jq absent — write a sentinel + keep the raw stream alongside.
  cp "${TMP_RAW}"/*.json "${REPORT_DIR}/" 2>/dev/null || true
  printf '{"findings":[{"path":"core/","message":"jq not installed — see raw NDJSON in tmp/reports/","severity":"warning"}]}\n' \
    > "${OUT}"
fi

# One-line summary.
if command -v jq >/dev/null 2>&1; then
  TOTAL=$(jq '.findings | length' "${OUT}")
  HIGH=$(jq '.findings | map(select(.severity=="high" or .severity=="critical")) | length' "${OUT}")
  MED=$(jq '.findings | map(select(.severity=="medium" or .severity=="warning")) | length' "${OUT}")
  LOW=$(jq '.findings | map(select(.severity=="low" or .severity=="info")) | length' "${OUT}")
  echo "vuln_scan: ${TOTAL} findings (${HIGH} high+critical, ${MED} medium, ${LOW} low) -> ${OUT}"
else
  echo "vuln_scan: report written -> ${OUT} (jq missing; summary skipped)"
fi

# Hint operators how to wire the gate.
echo "wire the orchestrator gate: export IRONFLYER_VULN_REPORT_PATH='${OUT}'"
exit 0
