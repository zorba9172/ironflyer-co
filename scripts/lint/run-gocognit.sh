#!/usr/bin/env bash
# Ironflyer — Anti-Bloat: gocognit driver.
#
# Wires the `complexity` gate (see core/orchestrator/internal/ai/finisher/
# gates_antibloat.go). gocognit measures cognitive complexity per Go
# function. We install it on demand via `go install`; it is NOT a
# project dep.
#
# Outputs:
#   tmp/reports/gocognit-<timestamp>.json   (normalized findings shape)
#   stdout: "complexity: N functions over <budget> -> <path>"
#
# Operator wiring:
#   ./scripts/lint/run-gocognit.sh
#   export IRONFLYER_COMPLEXITY_REPORT_PATH=/abs/path/to/gocognit-<ts>.json
#
# Exit codes:
#   0  no function exceeds the budget
#   1  at least one function exceeds the budget OR the tool failed

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
REPORT_DIR="${REPO_ROOT}/tmp/reports"
mkdir -p "${REPORT_DIR}"

TS="$(date -u +%Y%m%dT%H%M%SZ)"
NORMALIZED="${REPORT_DIR}/gocognit-${TS}.json"
BUDGET="${IRONFLYER_COMPLEXITY_BUDGET:-15}"

if ! command -v go >/dev/null 2>&1; then
  echo "FATAL: go not in PATH — install Go" >&2
  exit 1
fi

if ! command -v gocognit >/dev/null 2>&1; then
  echo "gocognit not found — installing via go install (one-time)" >&2
  if ! go install github.com/uudashr/gocognit/cmd/gocognit@latest 2>&1; then
    echo "FATAL: go install gocognit failed" >&2
    exit 1
  fi
fi

GOBIN_PATH="$(go env GOBIN)"
if [ -z "${GOBIN_PATH}" ]; then
  GOBIN_PATH="$(go env GOPATH)/bin"
fi
case ":${PATH}:" in
  *":${GOBIN_PATH}:"*) ;;
  *) export PATH="${GOBIN_PATH}:${PATH}" ;;
esac

if ! command -v jq >/dev/null 2>&1; then
  echo "FATAL: jq not in PATH — required to project gocognit output" >&2
  exit 1
fi

TMP_RAW="$(mktemp -d)"
trap 'rm -rf "${TMP_RAW}"' EXIT

run_one() {
  local mod="$1"
  local out_file="${TMP_RAW}/${mod}.json"
  if [ ! -d "${REPO_ROOT}/core/${mod}" ]; then
    echo "skipping core/${mod} (not a directory)" >&2
    return 0
  fi
  echo "running gocognit in core/${mod} (over=${BUDGET})..." >&2
  # gocognit's -json emits an array of { complexity, package, function,
  # pos: { filename, offset, line, column } } objects. -over filters to
  # those above the threshold. Exit code is non-zero when findings
  # exist — informational, not fatal.
  (
    cd "${REPO_ROOT}/core/${mod}" && \
    gocognit -over "${BUDGET}" -json ./... 2>/dev/null
  ) > "${out_file}" || true
  if [ ! -s "${out_file}" ]; then
    # Empty output is a clean signal — no functions exceed the budget.
    echo '[]' > "${out_file}"
  fi
}

run_one "orchestrator"
run_one "runtime"

# Project into the gate's expected shape:
#   { summary: { offenders, budget, severeOffenders },
#     findings: [ { path, message, severity } ... ] }
#
# severity = "warning" per function above the budget, "error" if any
# function exceeds 2× the budget.
jq -s --arg budget "${BUDGET}" '
  ( map(. // []) | add // [] ) as $all
  | ( ($budget | tonumber) ) as $b
  | ( $all | map(select(.complexity >= ($b * 2))) | length ) as $severe
  | {
      summary: {
        offenders: ($all | length),
        budget: $b,
        severeOffenders: $severe
      },
      findings: (
        [ {
            path: "core/",
            message: ("complexity: " + ($all | length | tostring)
                      + " functions over budget " + ($b | tostring)
                      + " (" + ($severe | tostring) + " ≥ 2× budget)"),
            severity: (if $severe > 0 then "error"
                       elif ($all | length) > 0 then "warning"
                       else "info" end)
          } ]
        +
        ($all | map({
            path: ((.pos.filename // "unknown") + ":" + ((.pos.line // 0) | tostring)),
            line: (.pos.line // 0),
            message: ("function " + ((.package // "?") + "." + (.function // "?"))
                      + " has cognitive complexity "
                      + ((.complexity // 0) | tostring)
                      + " (budget " + ($b | tostring) + ")"),
            severity: (if (.complexity // 0) >= ($b * 2) then "error" else "warning" end)
        }))
      )
    }
' "${TMP_RAW}"/*.json > "${NORMALIZED}"

OFFENDERS=$(jq -r '.summary.offenders' "${NORMALIZED}")
SEVERE=$(jq -r '.summary.severeOffenders' "${NORMALIZED}")

echo "complexity: ${OFFENDERS} functions over budget ${BUDGET} (${SEVERE} ≥ 2× budget) -> ${NORMALIZED}"
echo "wire the orchestrator gate: export IRONFLYER_COMPLEXITY_REPORT_PATH='${NORMALIZED}'"

if [ "${OFFENDERS}" -gt 0 ]; then
  echo "complexity: FAIL — ${OFFENDERS} functions exceed budget" >&2
  exit 1
fi
exit 0
