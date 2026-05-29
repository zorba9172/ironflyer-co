#!/usr/bin/env bash
# Ironflyer exit-grade policy gate.
#
# Reads the normalized reports emitted by scripts/lint plus Vitest coverage and
# fails the build when the product contract is violated.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
POLICY="${IRONFLYER_QUALITY_POLICY:-${REPO_ROOT}/.ironflyer/quality-policy.json}"
REPORT_DIR="${IRONFLYER_REPORT_DIR:-${REPO_ROOT}/tmp/reports}"

if ! command -v jq >/dev/null 2>&1; then
  echo "FATAL: jq is required for policy enforcement" >&2
  exit 2
fi
if [ ! -f "${POLICY}" ]; then
  echo "FATAL: policy file missing: ${POLICY}" >&2
  exit 2
fi

failures=0

fail() {
  echo "policy: FAIL - $*" >&2
  failures=$((failures + 1))
}

latest_report() {
  local pattern="$1"
  find "${REPORT_DIR}" -maxdepth 1 -type f -name "${pattern}" 2>/dev/null | sort | tail -1
}

check_coverage() {
  local summary=""
  for candidate in \
    "${REPO_ROOT}/coverage/coverage-summary.json" \
    "${REPO_ROOT}/clients/studio/coverage/coverage-summary.json"; do
    if [ -f "${candidate}" ]; then
      summary="${candidate}"
      break
    fi
  done
  if [ -z "${summary}" ]; then
    echo "policy: coverage report missing (skipping until coverage job is wired)"
    return
  fi
  for metric in lines statements functions branches; do
    local min actual
    min="$(jq -r ".coverage.${metric}" "${POLICY}")"
    actual="$(jq -r ".total.${metric}.pct // 0" "${summary}")"
    if awk -v a="${actual}" -v m="${min}" 'BEGIN{exit !(a+0 < m+0)}'; then
      fail "coverage ${metric} ${actual}% is below ${min}%"
    fi
  done
}

check_vulnerabilities() {
  local critical=0 high=0
  while IFS= read -r report; do
    [ -n "${report}" ] || continue
    local c h
    c="$(jq '[
      (.findings[]? | .severity // empty),
      (.Results[]?.Vulnerabilities[]? | .Severity // empty),
      (.Results[]?.Misconfigurations[]? | .Severity // empty),
      (.results[]?.packages[]?.vulnerabilities[]? | .database_specific.severity // empty)
    ] | map(ascii_downcase) | map(select(. == "critical")) | length' "${report}")"
    h="$(jq '[
      (.findings[]? | .severity // empty),
      (.Results[]?.Vulnerabilities[]? | .Severity // empty),
      (.Results[]?.Misconfigurations[]? | .Severity // empty),
      (.results[]?.packages[]?.vulnerabilities[]? | .database_specific.severity // empty)
    ] | map(ascii_downcase) | map(select(. == "high" or . == "error")) | length' "${report}")"
    critical=$((critical + c))
    high=$((high + h))
  done < <(find "${REPORT_DIR}" -maxdepth 1 -type f \( -name 'govulncheck-*.json' -o -name 'osv-*.json' -o -name 'trivy-*.json' \) 2>/dev/null)

  local maxCritical maxHigh
  maxCritical="$(jq -r '.vulnerabilities.critical' "${POLICY}")"
  maxHigh="$(jq -r '.vulnerabilities.high' "${POLICY}")"
  if [ "${critical}" -gt "${maxCritical}" ]; then
    fail "critical vulnerabilities ${critical} exceeds ${maxCritical}"
  fi
  if [ "${high}" -gt "${maxHigh}" ]; then
    fail "high vulnerabilities ${high} exceeds ${maxHigh}"
  fi
}

check_duplication() {
  local report
  report="$(latest_report 'jscpd-*.json')"
  [ -n "${report}" ] || return
  local actual max
  actual="$(jq -r '.summary.duplicationPct // 0' "${report}")"
  max="$(jq -r '.duplicationPct' "${POLICY}")"
  if awk -v a="${actual}" -v m="${max}" 'BEGIN{exit !(a+0 > m+0)}'; then
    fail "duplication ${actual}% exceeds ${max}%"
  fi
}

check_lint() {
  local report errors max
  report="$(latest_report 'eslint-*.json')"
  [ -n "${report}" ] || return
  errors="$(jq '[.[]?.errorCount // 0] | add // 0' "${report}")"
  max="$(jq -r '.lintErrors' "${POLICY}")"
  if [ "${errors}" -gt "${max}" ]; then
    fail "lint errors ${errors} exceeds ${max}"
  fi
}

check_coverage
check_vulnerabilities
check_duplication
check_lint

if [ "${failures}" -gt 0 ]; then
  echo "policy: ${failures} violation(s)"
  exit 1
fi

echo "policy: PASS"
