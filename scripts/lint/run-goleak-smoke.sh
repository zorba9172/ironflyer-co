#!/usr/bin/env bash
# Ironflyer — Anti-Bloat: goleak smoke driver.
#
# Wires the `mem_leak` gate (see core/orchestrator/internal/ai/finisher/
# gates_antibloat.go). go.uber.org/goleak is the canonical goroutine
# leak detector, BUT it ships as a test-time library and this repo has
# a constitutional no-tests rule. We therefore use the stdlib pprof
# goroutine profile instead, exposed by the orchestrator's
# /debug/leak/snapshot endpoint (see leakprobe.go).
#
# This script:
#   1. Curls /debug/leak/snapshot against IRONFLYER_API_URL.
#   2. Writes the snapshot to tmp/reports/goleak-<timestamp>.json.
#   3. Compares the goroutine count against scripts/health/goleak-baseline.json.
#   4. Exits 1 if current > baseline by > tolerancePct% or > toleranceAbsolute,
#      whichever is SMALLER (the script picks the tighter bound — leaks
#      should not require both to trip).
#
# Operator wiring:
#   export IRONFLYER_LEAK_PROBE_TOKEN=<random-32-bytes>   # on orchestrator
#   export IRONFLYER_API_URL=http://localhost:8080        # on caller
#   ./scripts/lint/run-goleak-smoke.sh
#   export IRONFLYER_MEMLEAK_REPORT_PATH=/abs/path/to/goleak-<ts>.json
#
# Exit codes:
#   0  snapshot within tolerance of baseline (or baseline missing —
#      degrade-to-info; the gate parser will surface SeverityInfo)
#   1  snapshot exceeds tolerance (likely leak)
#   2  could not fetch snapshot (orchestrator down OR token wrong)

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
REPORT_DIR="${REPO_ROOT}/tmp/reports"
mkdir -p "${REPORT_DIR}"

TS="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="${REPORT_DIR}/goleak-${TS}.json"
NORMALIZED="${REPORT_DIR}/goleak-normalized-${TS}.json"
BASELINE="${REPO_ROOT}/scripts/health/goleak-baseline.json"
API_URL="${IRONFLYER_API_URL:-http://localhost:8080}"
TOKEN="${IRONFLYER_LEAK_PROBE_TOKEN:-}"

if [ -z "${TOKEN}" ]; then
  echo "FATAL: IRONFLYER_LEAK_PROBE_TOKEN unset — the orchestrator's /debug/leak/snapshot is 404 without it" >&2
  exit 2
fi
if ! command -v curl >/dev/null 2>&1; then
  echo "FATAL: curl not in PATH" >&2
  exit 2
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "FATAL: jq not in PATH — required to parse the snapshot" >&2
  exit 2
fi

echo "fetching ${API_URL}/debug/leak/snapshot..." >&2
HTTP_CODE=$(curl -sS -o "${OUT}" -w "%{http_code}" \
  -H "Authorization: Bearer ${TOKEN}" \
  "${API_URL}/debug/leak/snapshot" || true)

if [ "${HTTP_CODE}" != "200" ]; then
  echo "FATAL: snapshot fetch returned HTTP ${HTTP_CODE} (body at ${OUT})" >&2
  # Normalize an evidence-stub so the gate can still render a warning.
  printf '{"findings":[{"path":"orchestrator","message":"goleak snapshot fetch failed (HTTP %s)","severity":"warning"}]}\n' \
    "${HTTP_CODE}" > "${NORMALIZED}"
  exit 2
fi

CURRENT=$(jq -r '.goroutines' "${OUT}")
if [ -z "${CURRENT}" ] || [ "${CURRENT}" = "null" ]; then
  echo "FATAL: snapshot missing .goroutines field" >&2
  exit 2
fi

# Baseline + tolerance. Missing baseline → degrade to SeverityInfo so
# operators see the count without a false-positive failure.
BASE=0
TOL_PCT=50
TOL_ABS=200
if [ -f "${BASELINE}" ]; then
  BASE=$(jq -r '.goroutines // 0' "${BASELINE}")
  TOL_PCT=$(jq -r '.tolerancePct // 50' "${BASELINE}")
  TOL_ABS=$(jq -r '.toleranceAbsolute // 200' "${BASELINE}")
fi

# The tighter of the two bounds wins. Example: baseline=80, pct=50,
# abs=200 → pct-bound = 80 * 1.5 = 120 (delta 40), abs-bound = 280
# (delta 200). pct is tighter → threshold delta = 40.
DELTA=$(( CURRENT - BASE ))
PCT_BOUND_DELTA=$(awk -v b="${BASE}" -v p="${TOL_PCT}" 'BEGIN{printf "%d", (b*p/100)}')
THRESHOLD_DELTA=${PCT_BOUND_DELTA}
if [ "${TOL_ABS}" -lt "${PCT_BOUND_DELTA}" ]; then
  THRESHOLD_DELTA=${TOL_ABS}
fi

LEAKED=0
SEVERITY="info"
MESSAGE="goleak: ${CURRENT} goroutines (baseline ${BASE}, delta ${DELTA}, threshold +${THRESHOLD_DELTA})"
if [ "${BASE}" -gt 0 ] && [ "${DELTA}" -gt "${THRESHOLD_DELTA}" ]; then
  LEAKED=1
  SEVERITY="critical"
  MESSAGE="goleak: LEAK SUSPECTED — ${CURRENT} goroutines vs baseline ${BASE} (delta +${DELTA}, threshold +${THRESHOLD_DELTA})"
fi

# Write the normalized report the gate parser consumes.
jq --arg sev "${SEVERITY}" \
   --arg msg "${MESSAGE}" \
   --argjson current "${CURRENT}" \
   --argjson base "${BASE}" \
   --argjson delta "${DELTA}" \
   --argjson threshold "${THRESHOLD_DELTA}" \
   '{
      summary: {
        goroutines: $current,
        baseline: $base,
        delta: $delta,
        thresholdDelta: $threshold,
        leaked: ('"${LEAKED}"' == 1)
      },
      findings: [ {
        path: "orchestrator",
        message: $msg,
        severity: $sev
      } ]
    }' "${OUT}" > "${NORMALIZED}"

echo "${MESSAGE} -> ${NORMALIZED}"
echo "wire the orchestrator gate: export IRONFLYER_MEMLEAK_REPORT_PATH='${NORMALIZED}'"

if [ "${LEAKED}" -eq 1 ]; then
  echo "mem_leak: FAIL — goroutine count exceeds baseline tolerance" >&2
  exit 1
fi
exit 0
