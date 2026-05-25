# Ironflyer-branded code-server.
#
# Extends the upstream codercom/code-server image with:
#   - IronFlyer slim cloud-IDE theme baked into the default User settings.json
#   - Pre-installed essentials only (Go, Prettier, ESLint, EditorConfig)
#   - A branded welcome page available at /home/coder/.config/welcome.html
#   - Sensible defaults: telemetry off, updates off, format on save
#
# Build:
#   docker build -f infra/docker/ironflyer-code.Dockerfile \
#     -t ghcr.io/zorba9172/ironflyer-code:latest .
#
# Run (manually, for inspection):
#   docker run -it --rm -p 8443:8080 \
#     ghcr.io/zorba9172/ironflyer-code:latest --auth none \
#     --bind-addr 0.0.0.0:8080 /home/coder/project
FROM codercom/code-server:latest

# Bake brand assets in before switching back to coder.
USER root
RUN mkdir -p /home/coder/.local/share/code-server/User /home/coder/.config
COPY infra/docker/ironflyer-code/settings.json /home/coder/.local/share/code-server/User/settings.json
COPY infra/docker/ironflyer-code/keybindings.json /home/coder/.local/share/code-server/User/keybindings.json
COPY infra/docker/ironflyer-code/welcome.html /home/coder/.config/welcome.html
RUN chown -R coder:coder /home/coder/.local /home/coder/.config

# Pre-install the toolchain the Ironflyer finisher gates rely on. Each
# tool degrades the gate gracefully when missing, but baking them into
# the image is the difference between "Security gate reports nothing" and
# "Security gate finds an OWASP A03 SQL injection in 4 seconds."
#
#  - semgrep      : multi-language SAST (Security gate)
#  - hadolint     : Dockerfile linter         (Deploy gate)
#  - govulncheck  : Go-specific CVE scanner   (Security gate, Go projects)
#  - go + node    : already standard but pinned here so finisher gates
#                   can `go test`, `npm test`, `npm audit` without a
#                   per-workspace bootstrap step
#  - git, jq, curl: utility belt every workspace assumes
#
# We refuse to install pinned-vulnerable versions: semgrep is taken from
# pip (stable), hadolint from its official static binary (no apt repo).
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      ca-certificates curl git jq python3-pip python3-venv nodejs npm \
 && python3 -m pip install --no-cache-dir --break-system-packages 'semgrep>=1.71' \
 && curl -sSL https://github.com/hadolint/hadolint/releases/download/v2.12.0/hadolint-Linux-x86_64 \
      -o /usr/local/bin/hadolint \
 && chmod +x /usr/local/bin/hadolint \
 && rm -rf /var/lib/apt/lists/*

# Go toolchain (mirrors the orchestrator image so projects with go.mod
# `go build ./...` works the moment the workspace boots). We pin the
# minor version to match orchestrator/go.mod's directive.
ENV GO_VERSION=1.25.1
RUN curl -sSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" \
     | tar -C /usr/local -xz \
 && /usr/local/go/bin/go install golang.org/x/vuln/cmd/govulncheck@latest \
 && mv /root/go/bin/govulncheck /usr/local/bin/ \
 && rm -rf /root/go
ENV PATH=/usr/local/go/bin:/usr/local/bin:${PATH}

USER coder
WORKDIR /home/coder

# Pre-install only the extensions that affect generated-code correctness.
# GitLens and other heavy navigation extensions are intentionally left out:
# the Studio shell already owns project history, patches, and agent context.
# `|| true` keeps the build resilient when an extension is temporarily
# missing from the Open VSX registry.
RUN code-server --install-extension golang.go \
 && code-server --install-extension esbenp.prettier-vscode \
 && code-server --install-extension dbaeumer.vscode-eslint \
 && code-server --install-extension editorconfig.editorconfig \
 || true

# Default landing. Local Studio compose passes `--auth none` so users do not
# see a second IDE login after the IronFlyer app session already allowed them
# into the workspace. Production should front the IDE with a signed runtime or
# edge route scoped to tenant/workspace/expiry instead of exposing this port.
EXPOSE 8080
ENV CS_DISABLE_GETTING_STARTED_OVERRIDE=1
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD curl -fsS http://127.0.0.1:8080/healthz || curl -fsSI http://127.0.0.1:8080/ >/dev/null || exit 1
ENTRYPOINT ["/usr/bin/entrypoint.sh", "--bind-addr", "0.0.0.0:8080", "/home/coder"]
