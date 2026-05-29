# Ironflyer IDE — custom-branded Eclipse Theia browser application.
# Multi-stage: (1) build the Theia browser bundle, (2) slim runtime image.
# Intended tag: ironflyer/theia-ide:latest
#
# NOTE: this image builds ONLY the IDE package and its local branding
# extension — it does not pull the whole pnpm monorepo. The build context
# is `clients/ide` (see .dockerignore). Native node-gyp deps for Theia
# (node-pty for the terminal, etc.) require build toolchain in the builder.

# ---------- Stage 1: build ----------
FROM node:20-bookworm AS build

# Toolchain for native modules (node-pty, etc.).
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        make gcc g++ python3 pkg-config libsecret-1-dev libx11-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

# Copy the full IDE sources (app + branding ext src/themes/style + the
# committed generated brand CSS/themes).
COPY . .

# The IDE manifest declares the branding extension as a pnpm `workspace:*`
# dependency. In this standalone npm/Docker build (no pnpm workspace) rewrite it
# to a local `file:` link. This MUST run AFTER `COPY . .` — otherwise the copy
# clobbers the edit and `npm install` fails on `Unsupported URL Type "workspace:"`.
# Build the branding extension first so its lib/ exists before the root install
# resolves the file: dependency, then build the Theia bundle. (The branding
# `build` re-generates themes from brand.ts when present; brand.ts is out of the
# Docker context here, so it no-ops and the committed generated files are used.)
RUN npm pkg set dependencies.@ironflyer/theia-branding="file:./ironflyer-branding" \
    && npm --prefix ironflyer-branding install --no-audit --no-fund \
    && npm --prefix ironflyer-branding run build \
    && npm install --no-audit --no-fund \
    && npx theia build --mode production \
    && mkdir -p plugins

# ---------- Stage 2: runtime ----------
FROM node:20-bookworm-slim AS runtime

# Minimal runtime libs for node-pty / Theia terminal.
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        libsecret-1-0 git openssh-client ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Non-root coder user mirroring the existing code-server setup.
RUN useradd --create-home --home-dir /home/coder --shell /bin/bash coder

# The Theia app lives in /opt/ironflyer-ide — deliberately NOT under
# /home/coder. The runtime's Docker driver bind-mounts the per-workspace volume
# at /home/coder (docker.go), so anything the app stored there would be SHADOWED
# by the mount and the app would fail to start. Keeping the app in /opt leaves
# /home/coder free to be the (mounted) workspace folder the editor opens.
WORKDIR /opt/ironflyer-ide
COPY --from=build --chown=coder:coder /build /opt/ironflyer-ide

RUN chown -R coder:coder /home/coder

USER coder
ENV HOME=/home/coder
EXPOSE 3030

# Theia browser has NO built-in auth ("--auth none" semantics): the runtime
# service is responsible for gating access to this container (network policy /
# reverse-proxy auth / per-workspace token). Opens /home/coder — the mounted
# per-workspace volume — as the workspace folder; runs from /opt/ironflyer-ide.
CMD ["npx", "theia", "start", "--hostname", "0.0.0.0", "--port", "3030", "--plugins=local-dir:plugins", "/home/coder"]
