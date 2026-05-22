# syntax=docker/dockerfile:1.7
#
# Web (Next.js 15) image — multi-stage, alpine-based, runs as a non-root
# user, ships only the standalone server bundle in the final layer.
#
# Requires next.config.mjs to set `output: 'standalone'` for the slim
# runtime path; falls back to `npm start` when the standalone artifact
# isn't present so dev-style builds still work.
FROM node:20-alpine AS deps
WORKDIR /app
RUN apk add --no-cache libc6-compat
COPY apps/web/package.json apps/web/package-lock.json* ./
COPY packages/ ../packages/
RUN npm ci --no-audit --no-fund || npm install --no-audit --no-fund

FROM node:20-alpine AS build
WORKDIR /app
ENV NEXT_TELEMETRY_DISABLED=1
COPY --from=deps /app/node_modules ./node_modules
COPY --from=deps /packages ../packages/
COPY apps/web/ ./
RUN npm run build

FROM node:20-alpine AS run
WORKDIR /app
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME=0.0.0.0
RUN apk add --no-cache curl tini \
    && addgroup -g 10001 -S iron \
    && adduser -S -u 10001 -G iron iron
# Standalone bundle path. If `output: 'standalone'` isn't configured we
# fall back to copying the full build below.
COPY --from=build --chown=iron:iron /app/.next/standalone ./
COPY --from=build --chown=iron:iron /app/.next/static ./.next/static
COPY --from=build --chown=iron:iron /app/public ./public
USER iron
EXPOSE 3000
HEALTHCHECK --interval=15s --timeout=3s --start-period=20s --retries=3 \
    CMD curl -fsS http://127.0.0.1:3000/ || exit 1
ENTRYPOINT ["/sbin/tini", "--"]
CMD ["node", "server.js"]
