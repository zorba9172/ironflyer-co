# syntax=docker/dockerfile:1.7
#
# Ironflyer workspace image — the per-user sandbox that the runtime's
# Docker driver spawns when `IRONFLYER_RUNTIME_DRIVER=docker`. Replaces
# the legacy `ironflyer-code` image (removed 2026-05-28 along with the
# in-Studio openvscode iframe).
#
# Goals:
#   • Clean, slim base (no VS Code chrome — code editing happens in
#     Monaco in Studio or in the standalone VS Code Extension).
#   • All AppSec scanners that core/orchestrator/internal/operations/appsec/
#     expects to find in PATH:
#       - semgrep        (Python; SAST per OWASP Top 10)
#       - gitleaks       (Go binary; secrets in git history + tree)
#       - trufflehog     (Go binary; entropy + provider verification)
#       - govulncheck    (Go binary; CVE scanner for go.mod projects)
#   • Standard developer toolchain so generated projects build:
#       - git, openssh-client, ca-certificates, curl, jq, bash
#       - python3 + pip
#       - node + npm (Lighthouse, npm audit, Next.js builds)
#       - go (govulncheck, generated Go templates)
#
# Why pre-install instead of `pip install semgrep` on demand: cold
# install is ~40s per workspace boot — ProfitGuard hates that. Bake it
# once, get a ready scanner on every workspace.

FROM golang:1.25-alpine AS gotools
RUN apk add --no-cache git ca-certificates
# AppSec scanners that ship as Go binaries.
RUN go install golang.org/x/vuln/cmd/govulncheck@latest
RUN go install github.com/zricethezav/gitleaks/v8@latest
RUN git clone --depth=1 --branch v3.95.3 https://github.com/trufflesecurity/trufflehog.git /tmp/trufflehog \
 && cd /tmp/trufflehog \
 && go build -trimpath -ldflags="-s -w" -o /go/bin/trufflehog . \
 && rm -rf /tmp/trufflehog

FROM alpine:3.20

# Runtime user matches the runtime service for filesystem ownership.
RUN adduser -D -u 10001 iron

# Tools every generated project + every AppSec scan needs.
# Pinning versions to alpine 3.20's repo keeps the image reproducible.
RUN apk add --no-cache \
    bash \
    ca-certificates \
    curl \
    git \
    jq \
    openssh-client \
    nodejs \
    npm \
    python3 \
    py3-pip \
    py3-virtualenv \
    tzdata

# Semgrep runs inside a venv so it doesn't fight alpine's system pip
# (PEP 668). The wrapper at /usr/local/bin/semgrep keeps the CLI on PATH.
RUN python3 -m venv /opt/semgrep-venv \
 && /opt/semgrep-venv/bin/pip install --no-cache-dir --upgrade pip \
 && /opt/semgrep-venv/bin/pip install --no-cache-dir semgrep \
 && ln -sf /opt/semgrep-venv/bin/semgrep /usr/local/bin/semgrep

# Drop the Go-built scanners alongside the system binaries.
COPY --from=gotools /go/bin/govulncheck /usr/local/bin/govulncheck
COPY --from=gotools /go/bin/gitleaks    /usr/local/bin/gitleaks
COPY --from=gotools /go/bin/trufflehog  /usr/local/bin/trufflehog

# Surface the toolchain summary at build time so a registry pull is
# auditable — operators can confirm the AppSec contract from `docker
# inspect` history.
RUN semgrep --version \
 && gitleaks version \
 && trufflehog --version \
 && govulncheck -version \
 && node --version \
 && npm --version \
 && python3 --version

USER iron
WORKDIR /home/iron
CMD ["/bin/bash", "-l"]
