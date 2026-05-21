# Ironflyer-branded code-server.
#
# Extends the upstream codercom/code-server image with:
#   - Dark + lime theme baked into the default User settings.json
#   - Pre-installed extensions (Go, Prettier, ESLint, GitLens, EditorConfig)
#   - A branded welcome page available at /home/coder/.config/welcome.html
#   - Sensible defaults: telemetry off, updates off, format on save
#
# Build:
#   docker build -f infra/docker/ironflyer-code.Dockerfile \
#     -t ghcr.io/zorba9172/ironflyer-code:latest .
#
# Run (manually, for inspection):
#   docker run -it --rm -p 8443:8080 \
#     -e PASSWORD=ironflyer-dev \
#     ghcr.io/zorba9172/ironflyer-code:latest
FROM codercom/code-server:latest

# Bake brand assets in before switching back to coder.
USER root
RUN mkdir -p /home/coder/.local/share/code-server/User /home/coder/.config
COPY infra/docker/ironflyer-code/settings.json /home/coder/.local/share/code-server/User/settings.json
COPY infra/docker/ironflyer-code/welcome.html /home/coder/.config/welcome.html
RUN chown -R coder:coder /home/coder/.local /home/coder/.config

USER coder
WORKDIR /home/coder

# Pre-install extensions. `|| true` keeps the build resilient when an
# extension is temporarily missing from the Open VSX registry.
RUN code-server --install-extension golang.go \
 && code-server --install-extension esbenp.prettier-vscode \
 && code-server --install-extension dbaeumer.vscode-eslint \
 && code-server --install-extension editorconfig.editorconfig \
 && code-server --install-extension eamodio.gitlens \
 || true

# Default landing — code-server reads $PASSWORD or hashed-password from config.
# In dev we pass PASSWORD=ironflyer-dev. Production should mount a hashed
# password file via the workspace-runtime ConfigMap.
EXPOSE 8080
ENV CS_DISABLE_GETTING_STARTED_OVERRIDE=1
ENTRYPOINT ["/usr/bin/entrypoint.sh", "--bind-addr", "0.0.0.0:8080", "/home/coder"]
