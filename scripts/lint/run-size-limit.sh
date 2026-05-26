#!/usr/bin/env bash
# Ironflyer — Anti-Bloat: size-limit driver.
#
# Wires the `bundle_size` gate (see core/orchestrator/internal/ai/finisher/
# gates_antibloat.go). size-limit is the standard frontend bundle
# budget tool. We run it via `npx --yes` so it is NOT a project dep.
#
# Inputs:
#   - clients/web/.size-limit.cjs (created here on first run if missing).
#     Mirrors the major routes under clients/web/app/ and budgets each
#     to 200 KB gzip by default.
#
# Outputs:
#   tmp/reports/size-limit-<timestamp>.json   (normalized findings shape)
#   stdout: "bundle_size: N routes over budget -> <path>"
#
# Operator wiring:
#   ./scripts/lint/run-size-limit.sh
#   export IRONFLYER_BUNDLE_REPORT_PATH=/abs/path/to/size-limit-<ts>.json
#
# Exit codes:
#   0  every route within its budget
#   1  one or more routes over budget OR the tool failed

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WEB_DIR="${REPO_ROOT}/clients/web"
REPORT_DIR="${REPO_ROOT}/tmp/reports"
mkdir -p "${REPORT_DIR}"

TS="$(date -u +%Y%m%dT%H%M%SZ)"
NORMALIZED="${REPORT_DIR}/size-limit-${TS}.json"
CONFIG="${WEB_DIR}/.size-limit.cjs"

if ! command -v npx >/dev/null 2>&1; then
  echo "FATAL: npx not in PATH — install Node.js + npm" >&2
  exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "FATAL: jq not in PATH — required to project size-limit output" >&2
  exit 1
fi
if [ ! -d "${WEB_DIR}" ]; then
  echo "FATAL: ${WEB_DIR} not found" >&2
  exit 1
fi

# Default config — one budget per major operator surface. Built lazily
# so operators can hand-edit the routes without losing them to a
# regenerate. The cjs extension keeps it Node-friendly without dragging
# ESM resolution into the cjs-default size-limit CLI.
if [ ! -f "${CONFIG}" ]; then
  echo "creating default ${CONFIG}..." >&2
  cat > "${CONFIG}" <<'EOF'
// Anti-Bloat: per-route bundle budgets enforced by size-limit.
// Edit budgets here; do NOT inline raw byte counts in CI.
// First-class routes are budgeted at 200 KB gzip — the heaviest are
// Studio (Monaco) + Cockpit (echarts + xyflow) which lazy-load their
// heavy deps via next/dynamic and so the static budget excludes them.
module.exports = [
  { name: 'home',     path: '.next/static/chunks/app/page*.js',                    limit: '200 KB' },
  { name: 'login',    path: '.next/static/chunks/app/login/page*.js',              limit: '200 KB' },
  { name: 'signup',   path: '.next/static/chunks/app/signup/page*.js',             limit: '200 KB' },
  { name: 'pricing',  path: '.next/static/chunks/app/pricing/page*.js',            limit: '200 KB' },
  { name: 'cockpit',  path: '.next/static/chunks/app/cockpit/page*.js',            limit: '250 KB' },
  { name: 'studio',   path: '.next/static/chunks/app/studio/**/page*.js',          limit: '250 KB' },
  { name: 'wallet',   path: '.next/static/chunks/app/wallet/page*.js',             limit: '200 KB' },
  { name: 'deploy',   path: '.next/static/chunks/app/deploy/**/page*.js',          limit: '200 KB' },
  { name: 'projects', path: '.next/static/chunks/app/projects/page*.js',           limit: '200 KB' },
];
EOF
fi

# size-limit requires a built app to measure the real chunk sizes.
if [ ! -d "${WEB_DIR}/.next" ]; then
  echo "WARN: ${WEB_DIR}/.next not present — running `next build` first" >&2
  (
    cd "${WEB_DIR}" && \
    NEXT_TELEMETRY_DISABLED=1 npm run build
  ) || {
    echo "FATAL: next build failed; size-limit needs a built app" >&2
    # Still write a degrade-to-warning stub so the gate has signal.
    printf '{"findings":[{"path":"clients/web","message":"size-limit: next build failed, no measurement","severity":"warning"}]}\n' \
      > "${NORMALIZED}"
    exit 1
  }
fi

echo "running size-limit in ${WEB_DIR}..." >&2
RAW_OUT="$(mktemp)"
trap 'rm -f "${RAW_OUT}"' EXIT

# size-limit exits non-zero when a route is over budget — that's the
# signal we want. Capture stdout JSON regardless.
(
  cd "${WEB_DIR}" && \
  npx --yes size-limit --json 2>/dev/null
) > "${RAW_OUT}" || true

if [ ! -s "${RAW_OUT}" ]; then
  echo "FATAL: size-limit produced no output" >&2
  printf '{"findings":[{"path":"clients/web","message":"size-limit produced no output","severity":"warning"}]}\n' \
    > "${NORMALIZED}"
  exit 1
fi

# size-limit JSON shape (v11+):
#   [
#     { "name": "home", "passed": true|false, "size": <bytes>,
#       "sizeLimit": <bytes>, "running": <s>, "loading": <s> },
#     ...
#   ]
#
# We project into the gate's expected shape and produce SeverityError
# per over-budget route, SeverityInfo overall when all pass.
jq '
  ( map(select(.passed == false)) ) as $over
  | ( length ) as $total
  | ( $over | length ) as $overCount
  | {
      summary: {
        totalRoutes: $total,
        overBudgetRoutes: $overCount
      },
      findings: (
        [ {
            path: "clients/web",
            message: ("size-limit: " + ($overCount | tostring)
                      + " of " + ($total | tostring)
                      + " routes over budget"),
            severity: (if $overCount > 0 then "error" else "info" end)
          } ]
        +
        ( . | map({
            path: ("clients/web (" + .name + ")"),
            message: ( "route " + .name
                       + ": " + ((.size // 0) | tostring) + " B"
                       + " vs limit " + ((.sizeLimit // 0) | tostring) + " B"
                       + (if .passed then " ✓" else " ✗ OVER" end) ),
            severity: (if .passed then "info" else "error" end)
        }))
      )
    }
' "${RAW_OUT}" > "${NORMALIZED}"

OVER=$(jq -r '.summary.overBudgetRoutes' "${NORMALIZED}")
TOTAL=$(jq -r '.summary.totalRoutes' "${NORMALIZED}")

echo "bundle_size: ${OVER} of ${TOTAL} routes over budget -> ${NORMALIZED}"
echo "wire the orchestrator gate: export IRONFLYER_BUNDLE_REPORT_PATH='${NORMALIZED}'"

if [ "${OVER}" -gt 0 ]; then
  echo "bundle_size: FAIL — ${OVER} routes exceed their budget" >&2
  exit 1
fi
exit 0
