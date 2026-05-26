# IronFlyer slim OpenVSCode Server.
#
# Default cloud IDE used by the Studio iframe. Upstream openvscode-server
# runtime with four IronFlyer changes baked on top:
#   1. Brand color theme (Ironflyer Dark) shipped as a contributed
#      extension — single source of truth for IDE chrome colors.
#   2. User settings + keybindings copied into the data dir, pointed
#      at the new theme and at the host-side orchestrator URL so the
#      Ironflyer assistant extension talks to the right backend out of
#      the box.
#   3. Upstream GitHub source-control extensions removed so the IDE is
#      pure Ironflyer with no third-party chrome.
#   4. The Ironflyer assistant extension itself (chat / gates / patches
#      / live preview / wallet / inline AI) packaged in stage 1 and
#      installed in stage 2 so it activates on container start.

# -----------------------------------------------------------------------------
# Stage 1 — build the Ironflyer assistant extension into a .vsix.
#
# We can't run vsce inside the runtime image (it ships without npm /
# node toolchain), so a node:20 builder produces the artifact and the
# runtime stage just consumes it.
# -----------------------------------------------------------------------------
FROM node:20-slim AS ext-builder

WORKDIR /build

# Layer the dependency install separately from the source copy so a
# code-only change keeps the npm install cache warm across rebuilds.
COPY apps/vscode-extension/package.json apps/vscode-extension/package-lock.json ./
RUN npm ci

# Now the source. esbuild bundles dependencies into dist/extension.js
# and vsce wraps that + media/ + package.json into the .vsix.
COPY apps/vscode-extension/ ./
RUN npx vsce package --no-dependencies --out /build/ironflyer.vsix

# -----------------------------------------------------------------------------
# Stage 2 — runtime openvscode-server with all IronFlyer assets baked.
# -----------------------------------------------------------------------------
FROM gitpod/openvscode-server:latest

USER root

# 1. User data dir with our settings + keybindings.
RUN mkdir -p /home/.openvscode-server/data/User \
 && chown -R openvscode-server:openvscode-server /home/.openvscode-server
COPY infra/docker/ironflyer-code/settings.json /home/.openvscode-server/data/User/settings.json
COPY infra/docker/ironflyer-code/keybindings.json /home/.openvscode-server/data/User/keybindings.json
RUN chown openvscode-server:openvscode-server \
      /home/.openvscode-server/data/User/settings.json \
      /home/.openvscode-server/data/User/keybindings.json

# 2. Ironflyer Dark theme as a built-in extension. The extensions/
# directory is auto-scanned at startup, so dropping the folder here
# registers the theme without the operator running --install-extension.
COPY --chown=openvscode-server:openvscode-server \
     infra/docker/ironflyer-vscode-theme \
     /home/.openvscode-server/extensions/ironflyer.ironflyer-dark-theme-0.1.0

# 3. Strip the upstream GitHub source-control + auth extensions. The
# Studio handles auth + project sync; these only add chrome we don't
# use. Anything that depended on them surfaces as a missing-extension
# warning the operator can ignore (we suppress notifications anyway).
RUN rm -rf \
      /home/.openvscode-server/extensions/github \
      /home/.openvscode-server/extensions/github-authentication

# 4. Developer language extensions from Open VSX. Pre-bundled so the
# operator never sees the "install ESLint?" prompt mid-flow. List is
# intentionally tight — only extensions that improve the daily editing
# loop for the stack Ironflyer projects use (TypeScript/React, CSS,
# YAML, GraphQL, Markdown). Anything heavier (Prisma, Vue, Rust) stays
# user-installable.
RUN /home/.openvscode-server/bin/openvscode-server \
      --extensions-dir /home/.openvscode-server/extensions \
      --install-extension dbaeumer.vscode-eslint \
      --install-extension esbenp.prettier-vscode \
      --install-extension bradlc.vscode-tailwindcss \
      --install-extension redhat.vscode-yaml \
      --install-extension graphql.vscode-graphql-syntax \
      --install-extension yzhang.markdown-all-in-one

# 5. Ironflyer assistant extension. Pulled from the stage-1 builder
# and installed via openvscode-server's own CLI so the extension's
# package.json metadata + activation events register correctly.
COPY --from=ext-builder /build/ironflyer.vsix /tmp/ironflyer.vsix
RUN /home/.openvscode-server/bin/openvscode-server \
      --install-extension /tmp/ironflyer.vsix \
      --extensions-dir /home/.openvscode-server/extensions \
 && rm -f /tmp/ironflyer.vsix \
 && chown -R openvscode-server:openvscode-server /home/.openvscode-server/extensions

USER openvscode-server
