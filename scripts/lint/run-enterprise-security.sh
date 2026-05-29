#!/usr/bin/env bash
# Unified local/CI security scan driver for the exit-grade pipeline.
#
# Tools are installed outside project deps where possible. Missing tools degrade
# to clear warning reports so the policy layer can decide how strict to be.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
REPORT_DIR="${REPO_ROOT}/tmp/reports"
mkdir -p "${REPORT_DIR}"

TS="$(date -u +%Y%m%dT%H%M%SZ)"

need_go_bin() {
  local bin="$1"
  local pkg="$2"
  if ! command -v "${bin}" >/dev/null 2>&1; then
    echo "${bin} not found - installing ${pkg}" >&2
    go install "${pkg}"
  fi
  local gobin
  gobin="$(go env GOBIN)"
  if [ -z "${gobin}" ]; then
    gobin="$(go env GOPATH)/bin"
  fi
  case ":${PATH}:" in
    *":${gobin}:"*) ;;
    *) export PATH="${gobin}:${PATH}" ;;
  esac
}

write_warning() {
  local file="$1"
  local message="$2"
  jq -n --arg msg "${message}" '{findings:[{path:".",message:$msg,severity:"warning"}]}' > "${file}"
}

run_gitleaks() {
  local out="${REPORT_DIR}/gitleaks-${TS}.json"
  need_go_bin gitleaks github.com/gitleaks/gitleaks/v8@latest
  (cd "${REPO_ROOT}" && gitleaks detect --no-banner --redact --report-format json --report-path "${out}") || true
  if [ ! -s "${out}" ]; then
    echo '[]' > "${out}"
  fi
  echo "gitleaks -> ${out}"
}

run_semgrep() {
  local raw="${REPORT_DIR}/semgrep-${TS}.sarif"
  if ! command -v semgrep >/dev/null 2>&1; then
    if command -v python3 >/dev/null 2>&1; then
      python3 -m pip install --quiet --user semgrep || true
      export PATH="${HOME}/.local/bin:${PATH}"
    fi
  fi
  if ! command -v semgrep >/dev/null 2>&1; then
    write_warning "${REPORT_DIR}/semgrep-${TS}.json" "semgrep not installed"
    return
  fi
  (cd "${REPO_ROOT}" && semgrep scan --config .semgrep.yml --config p/owasp-top-ten --sarif --output "${raw}") || true
  cp "${raw}" "${REPORT_DIR}/semgrep-latest.sarif"
  echo "semgrep -> ${raw}"
}

run_osv() {
  local out="${REPORT_DIR}/osv-${TS}.json"
  need_go_bin osv-scanner github.com/google/osv-scanner/v2/cmd/osv-scanner@latest
  (cd "${REPO_ROOT}" && osv-scanner --format json --recursive --skip-git . > "${out}") || true
  if [ ! -s "${out}" ]; then
    write_warning "${out}" "osv-scanner produced no output"
  fi
  echo "osv-scanner -> ${out}"
}

run_trivy() {
  local out="${REPORT_DIR}/trivy-${TS}.json"
  if ! command -v trivy >/dev/null 2>&1; then
    need_go_bin trivy github.com/aquasecurity/trivy/cmd/trivy@latest || true
  fi
  if ! command -v trivy >/dev/null 2>&1; then
    write_warning "${out}" "trivy not installed"
    return
  fi
  (cd "${REPO_ROOT}" && trivy fs --format json --quiet --scanners vuln,secret,misconfig --skip-dirs node_modules --skip-dirs .git . > "${out}") || true
  echo "trivy -> ${out}"
}

run_syft() {
  local out="${REPORT_DIR}/syft-${TS}.cyclonedx.json"
  if ! command -v syft >/dev/null 2>&1; then
    need_go_bin syft github.com/anchore/syft/cmd/syft@latest || true
  fi
  if ! command -v syft >/dev/null 2>&1; then
    write_warning "${REPORT_DIR}/syft-${TS}.json" "syft not installed"
    return
  fi
  (cd "${REPO_ROOT}" && syft dir:. -o cyclonedx-json --quiet > "${out}") || true
  echo "syft -> ${out}"
}

run_scancode() {
  local out="${REPORT_DIR}/scancode-${TS}.json"
  if ! command -v scancode >/dev/null 2>&1; then
    if command -v python3 >/dev/null 2>&1; then
      python3 -m pip install --quiet --user scancode-toolkit || true
      export PATH="${HOME}/.local/bin:${PATH}"
    fi
  fi
  if ! command -v scancode >/dev/null 2>&1; then
    write_warning "${out}" "scancode not installed"
    return
  fi
  (cd "${REPO_ROOT}" && scancode --license --json-pp "${out}" --ignore node_modules --ignore .git .) || true
  echo "scancode -> ${out}"
}

if ! command -v jq >/dev/null 2>&1; then
  echo "FATAL: jq is required" >&2
  exit 2
fi

run_gitleaks
run_semgrep
run_osv
run_trivy
run_syft
run_scancode
