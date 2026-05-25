# syntax=docker/dockerfile:1.7
#
# Web (Next.js 15) image — multi-stage, alpine-based, runs as a non-root
# user. Uses `next start` against the full .next build output, which
# works whether or not next.config.mjs opts in to `output: 'standalone'`.
#
# The web app imports tokens directly from packages/design-tokens via
# relative paths (`../../../../packages/design-tokens`), so the build
# context must include both `apps/web/` and `packages/`. We mirror the
# repo layout inside the image: /app/apps/web + /app/packages.
#
# NEXT_PUBLIC_IRONFLYER_API_URL is read at build-time by Next.js and
# baked into the client bundle. Override with --build-arg when building
# for staging/prod.
ARG NEXT_PUBLIC_IRONFLYER_API_URL=http://localhost:8080

FROM node:20-alpine AS deps
WORKDIR /app/apps/web
RUN apk add --no-cache libc6-compat
# Copy package manifests first so npm ci can cache when only source
# files change. The web package imports packages/design-tokens by
# relative path; bring the package tree along so any postinstall /
# resolution that walks up the tree succeeds.
COPY apps/web/package.json apps/web/package-lock.json* ./
COPY packages/ /app/packages/
RUN --mount=type=cache,target=/root/.npm \
    (npm ci --no-audit --no-fund --legacy-peer-deps \
     || npm install --no-audit --no-fund --legacy-peer-deps)

FROM node:20-alpine AS build
ARG NEXT_PUBLIC_IRONFLYER_API_URL
ENV NEXT_TELEMETRY_DISABLED=1 \
    NEXT_PUBLIC_IRONFLYER_API_URL=${NEXT_PUBLIC_IRONFLYER_API_URL}
WORKDIR /app/apps/web
COPY --from=deps /app/packages /app/packages
COPY --from=deps /app/apps/web/node_modules ./node_modules
COPY apps/web/ ./
# Generated GraphQL types are committed under src/lib/gql/__generated__.ts
# so no codegen step is required during the image build. If a future
# build expects fresh codegen, run `npm run codegen` here before build.
RUN --mount=type=cache,target=/root/.npm \
    npm run build

FROM node:20-alpine AS run
ARG NEXT_PUBLIC_IRONFLYER_API_URL
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME=0.0.0.0 \
    NEXT_PUBLIC_IRONFLYER_API_URL=${NEXT_PUBLIC_IRONFLYER_API_URL}
WORKDIR /app/apps/web
RUN apk add --no-cache curl tini \
    && addgroup -g 10001 -S iron \
    && adduser -S -u 10001 -G iron iron
# Copy the full build output and the production dep tree. We do not
# rely on `output: 'standalone'` because next.config.mjs in this repo
# does not enable it (config is owned by the web team, not infra).
COPY --from=build --chown=iron:iron /app/packages /app/packages
COPY --from=build --chown=iron:iron /app/apps/web/.next ./.next
COPY --from=build --chown=iron:iron /app/apps/web/public ./public
COPY --from=build --chown=iron:iron /app/apps/web/package.json ./package.json
COPY --from=build --chown=iron:iron /app/apps/web/next.config.mjs ./next.config.mjs
COPY --from=build --chown=iron:iron /app/apps/web/node_modules ./node_modules
USER iron
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD curl -fsS http://127.0.0.1:3000/ || exit 1
ENTRYPOINT ["/sbin/tini", "--"]
CMD ["node", "node_modules/next/dist/bin/next", "start", "-p", "3000", "-H", "0.0.0.0"]
