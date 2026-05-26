#!/usr/bin/env bash
# Ironflyer — Anti-Bloat: jscpd driver.
#
# Wires the `dedup` gate (see core/orchestrator/internal/ai/finisher/
# gates_antibloat.go). jscpd is the standard cross-file copy/paste
# detector for TypeScript/JS. We run it via `npx --yes` so it is NOT a
# project dep — first run downloads, subsequent runs cache.
#
# Outputs:
#   tmp/reports/jscpd/jscpd-report.json       (raw jscpd report)
#   tmp/reports/jscpd-<timestamp>.json        (normalized findings shape)
#   stdout: "dedup: <pct>% duplication (budget 2%) -> <path>"
#
# Operator wiring:
#   ./scripts/lint/run-jscpd.sh
#   export IRONFLYER_DEDUP_REPORT_PATH=/abs/path/to/jscpd-<ts>.json
#
# Exit codes:
#   0  dup percentage <= budget (2% per playbook §8.5)
#   1  dup percentage > budget OR jscpd produced no parseable output

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
REPORT_DIR="${REPO_ROOT}/tmp/reports"
JSCPD_OUT_DIR="${REPORT_DIR}/jscpd"
mkdir -p "${JSCPD_OUT_DIR}"

TS="$(date -u +%Y%m%dT%H%M%SZ)"
NORMALIZED="${REPORT_DIR}/jscpd-${TS}.json"
BUDGET_PCT="${IRONFLYER_DEDUP_BUDGET_PCT:-2}"

if ! command -v npx >/dev/null 2>&1; then
  echo "FATAL: npx not in PATH — install Node.js + npm" >&2
  exit 1
fi

PATTERN="clients/web/src/**/*.{ts,tsx}"

echo "running jscpd on ${PATTERN}..." >&2
# --silent keeps the console clean; --reporters json drops report into
# the output dir. We intentionally do NOT pass --threshold here because
# jscpd's built-in threshold compares against `--max-lines`/`--min-lines`
# tokens, not overall %. We compute the overall percentage ourselves
# below from `statistics.total.percentage`.
(
  cd "${REPO_ROOT}" && \
  npx --yes jscpd \
    --pattern "${PATTERN}" \
    --reporters json \
    --output "${JSCPD_OUT_DIR}" \
    --silent
) || true

RAW="${JSCPD_OUT_DIR}/jscpd-report.json"
if [ ! -s "${RAW}" ]; then
  echo "FATAL: jscpd produced no report at ${RAW}" >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "FATAL: jq not in PATH — required to compute duplication %" >&2
  exit 1
fi

# jscpd JSON shape (v4): statistics.total.percentage is the overall
# duplication %. duplicates is the array of clone groups.
PCT_RAW=$(jq -r '.statistics.total.percentage // 0' "${RAW}")
# Round to 2 decimals via awk for portability.
PCT=$(awk -v p="${PCT_RAW}" 'BEGIN{printf "%.2f", p+0}')
DUP_COUNT=$(jq -r '(.duplicates // []) | length' "${RAW}")

# Project into the gate's expected shape. We surface one finding per
# clone with severity=high if overall pct > budget, medium otherwise.
OVERALL_SEV="medium"
EXCEEDED=0
if awk -v p="${PCT}" -v b="${BUDGET_PCT}" 'BEGIN{exit !(p+0 > b+0)}'; then
  OVERALL_SEV="high"
  EXCEEDED=1
fi

jq --arg sev "${OVERALL_SEV}" --arg pct "${PCT}" --arg budget "${BUDGET_PCT}" '
  {
    summary: {
      duplicationPct: ($pct | tonumber),
      budgetPct: ($budget | tonumber),
      cloneCount: ((.duplicates // []) | length)
    },
    findings: (
      [ {
          path: "clients/web/src",
          message: ("overall duplication " + $pct + "% (budget " + $budget + "%) — " +
                   (((.duplicates // []) | length) | tostring) + " clone groups"),
          severity: $sev
        } ] +
      ((.duplicates // []) | map({
        path: (.firstFile.name // "unknown"),
        line: (.firstFile.start // 0),
        message: ("duplicate of " + (.secondFile.name // "unknown") +
                  " (lines " + ((.firstFile.start // 0) | tostring) + "-" +
                  ((.firstFile.end // 0) | tostring) + ", " +
                  ((.lines // 0) | tostring) + " lines, " +
                  ((.tokens // 0) | tostring) + " tokens)"),
        severity: "warning"
      }))
    )
  }
' "${RAW}" > "${NORMALIZED}"

echo "dedup: ${PCT}% duplication across ${DUP_COUNT} clone groups (budget ${BUDGET_PCT}%) -> ${NORMALIZED}"
echo "wire the orchestrator gate: export IRONFLYER_DEDUP_REPORT_PATH='${NORMALIZED}'"

if [ "${EXCEEDED}" -eq 1 ]; then
  echo "dedup: FAIL — duplication ${PCT}% exceeds budget ${BUDGET_PCT}%" >&2
  exit 1
fi

exit 0
