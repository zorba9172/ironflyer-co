package finisher

import (
	"context"
	"strings"

	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/ai/domain"
)

// DeployTargetWriter is the surface a real DeployGate needs in order to
// generate missing deploy artifacts (Dockerfile, fly.toml, railway.json)
// directly into the project workspace. It mirrors the patch lifecycle:
// every write goes through Propose so the existing gates can approve.
//
// Agent A owns the canonical RuntimeApplier interface in patch/runtime.go;
// until that lands we accept any value that satisfies this minimal
// contract. The HTTP deploy handlers wire a concrete implementation via
// the patch engine.
type DeployTargetWriter interface {
	WriteFile(ctx context.Context, projectID, path, contents string) error
}

// DeployArtifact describes a file the DeployGate intends to or did
// generate to make the project shippable. The orchestrator surfaces this
// list to the UI ("we created Dockerfile, fly.toml") so deploys never
// feel like magic.
type DeployArtifact struct {
	Path    string `json:"path"`
	Source  string `json:"source"`  // "existing" | "generated"
	Stack   string `json:"stack"`   // "node" | "go" | "static"
	Purpose string `json:"purpose"` // dockerfile | flyconfig | railwayconfig | workflow
}

// DeployGateV2 is the real deploy gate: it verifies that the project can
// be built into a container and, when artifacts are missing, generates
// them from the templates/deploy/ tree via a DeployTargetWriter.
//
// It does NOT replace finisher.DeployGate inline (gates.go is owned by
// another agent). Instead, it is the canonical deploy-gate implementation
// that the HTTP layer and future DefaultGates() rewires will instantiate.
type DeployGateV2 struct {
	// Writer is optional. When nil the gate degrades to read-only checks:
	// it reports missing artifacts as issues but cannot fix them itself.
	Writer DeployTargetWriter
	// AutoGenerate controls whether the gate writes missing artifacts on
	// pass. When false the gate stays advisory and the Deployer agent
	// owns the actual writes.
	AutoGenerate bool
}

// NewDeployGate constructs the gate. Pass a non-nil writer + AutoGenerate
// to make the gate self-healing; otherwise it is purely diagnostic.
func NewDeployGate(w DeployTargetWriter, autoGenerate bool) *DeployGateV2 {
	return &DeployGateV2{Writer: w, AutoGenerate: autoGenerate}
}

func (g *DeployGateV2) Name() domain.GateName    { return domain.GateDeploy }
func (g *DeployGateV2) RepairAgent() agents.Role { return agents.RoleDeployer }

// Check runs the gate. It returns issues for whatever is still missing
// after a (best-effort) auto-generation pass. A passing gate returns nil.
func (g *DeployGateV2) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	if env == nil || env.Project == nil {
		return []domain.Issue{{
			Gate: domain.GateDeploy, Severity: domain.SeverityError,
			Message: "no project bound to gate environment",
		}}
	}
	p := env.Project

	stack := detectDeployStack(p)
	if stack == "" {
		return []domain.Issue{{
			Gate: domain.GateDeploy, Severity: domain.SeverityError,
			Message: "no recognised deploy stack",
			Hint:    "add a Dockerfile, a package.json with a known framework, or go.mod with a main package",
		}}
	}

	plan := PlanDeployArtifacts(p, stack)
	var issues []domain.Issue

	for _, art := range plan {
		if art.Source == "existing" {
			continue
		}
		if g.AutoGenerate && g.Writer != nil {
			body, ok := DeployTemplateBody(art, p)
			if !ok {
				issues = append(issues, domain.Issue{
					Gate: domain.GateDeploy, Severity: domain.SeverityWarning,
					Message: "no template available for " + art.Purpose + " on stack " + stack,
				})
				continue
			}
			if err := g.Writer.WriteFile(ctx, p.ID, art.Path, body); err != nil {
				issues = append(issues, domain.Issue{
					Gate: domain.GateDeploy, Severity: domain.SeverityError,
					Message: "failed to generate " + art.Path + ": " + err.Error(),
					Path:    art.Path,
				})
			}
			continue
		}
		// Read-only mode: surface a hint so the UI can offer a "generate" CTA.
		issues = append(issues, domain.Issue{
			Gate: domain.GateDeploy, Severity: domain.SeverityWarning,
			Message: "missing deploy artifact " + art.Path,
			Path:    art.Path,
			Hint:    "click Deploy in the project UI to generate " + art.Path + " for stack " + stack,
		})
	}

	// README is a soft requirement — keep parity with the legacy gate.
	if !hasReadme(p) {
		issues = append(issues, domain.Issue{
			Gate: domain.GateDeploy, Severity: domain.SeverityWarning,
			Message: "no README — your deployed app is harder to operate without one",
		})
	}
	return issues
}

// PlanDeployArtifacts returns the canonical artifact list for a stack,
// marking each as existing or to-be-generated based on what's already in
// the project. Pure function so HTTP handlers can preview the plan
// without invoking the gate.
func PlanDeployArtifacts(p *domain.Project, stack string) []DeployArtifact {
	if stack == "" {
		stack = detectDeployStack(p)
	}
	out := []DeployArtifact{
		{Path: "Dockerfile", Purpose: "dockerfile", Stack: stack},
		{Path: "fly.toml", Purpose: "flyconfig", Stack: stack},
		{Path: "railway.json", Purpose: "railwayconfig", Stack: stack},
		{Path: ".github/workflows/deploy.yml", Purpose: "workflow", Stack: stack},
	}
	for i := range out {
		if hasFile(p, out[i].Path) {
			out[i].Source = "existing"
		} else {
			out[i].Source = "generated"
		}
	}
	return out
}

// DeployTemplateBody renders the body for a given artifact. App name +
// region defaults are derived from the project slug so the user doesn't
// have to hand-edit the placeholders for a first deploy.
func DeployTemplateBody(art DeployArtifact, p *domain.Project) (string, bool) {
	appName := deployAppName(p)
	port := defaultPort(art.Stack)
	switch art.Purpose {
	case "dockerfile":
		if art.Stack == "go" {
			return dockerfileGo, true
		}
		return dockerfileNode, true
	case "flyconfig":
		body := flyTomlTemplate
		body = strings.ReplaceAll(body, "{{APP_NAME}}", appName)
		body = strings.ReplaceAll(body, "{{PRIMARY_REGION}}", "iad")
		body = strings.ReplaceAll(body, "{{PORT}}", port)
		return body, true
	case "railwayconfig":
		start := "node node_modules/.bin/next start"
		if art.Stack == "go" {
			start = "/app"
		}
		body := strings.ReplaceAll(railwayJSONTemplate, "{{START_COMMAND}}", start)
		return body, true
	case "workflow":
		return githubWorkflow, true
	}
	return "", false
}

// detectDeployStack picks between go, node, and static. The choice drives
// which Dockerfile template + start command we use.
func detectDeployStack(p *domain.Project) string {
	backend := strings.ToLower(p.Spec.Stack.Backend)
	frontend := strings.ToLower(p.Spec.Stack.Frontend)
	switch {
	case strings.Contains(backend, "go") || hasFile(p, "go.mod"):
		return "go"
	case strings.Contains(frontend, "next"),
		strings.Contains(frontend, "vite"),
		strings.Contains(frontend, "remix"),
		strings.Contains(frontend, "react"),
		strings.Contains(backend, "node"),
		hasFile(p, "package.json"):
		return "node"
	case hasFile(p, "index.html"):
		return "static"
	}
	return ""
}

func hasReadme(p *domain.Project) bool {
	for _, f := range p.Files {
		if strings.EqualFold(f.Path, "README.md") || strings.EqualFold(f.Path, "readme.md") {
			return true
		}
	}
	return false
}

// deployAppName produces a Fly-safe app name from the project ID. Fly
// allows lowercase letters, digits, and hyphens, ≤ 30 chars.
func deployAppName(p *domain.Project) string {
	id := strings.ToLower(p.ID)
	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == ' ':
			b.WriteByte('-')
		}
	}
	name := strings.Trim(b.String(), "-")
	if name == "" {
		name = "ironflyer-app"
	}
	if len(name) > 30 {
		name = name[:30]
	}
	return "iflyr-" + name
}

func defaultPort(stack string) string {
	if stack == "go" {
		return "8080"
	}
	return "3000"
}

// --- inline templates ---
//
// We embed the canonical templates as constants rather than relying on
// `embed.FS`, so the gate can run in tests and one-off Goroutines without
// depending on the build context. The on-disk copies under
// templates/deploy/ are the source of truth — keep them in sync.

const dockerfileNode = `# Multi-stage Dockerfile for Node.js / Next.js / Vite / Remix apps.
# Generated by Ironflyer's DeployGate.

FROM node:20-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json* pnpm-lock.yaml* yarn.lock* ./
RUN if [ -f pnpm-lock.yaml ]; then corepack enable && pnpm install --frozen-lockfile; \
    elif [ -f yarn.lock ]; then corepack enable && yarn install --frozen-lockfile; \
    elif [ -f package-lock.json ]; then npm ci --no-audit --no-fund; \
    else npm install --no-audit --no-fund; fi

FROM node:20-alpine AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build --if-present

FROM node:20-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV PORT=3000
RUN addgroup -S app && adduser -S app -G app
COPY --from=build --chown=app:app /app /app
USER app
EXPOSE 3000
CMD ["sh", "-c", "if [ -f node_modules/.bin/next ]; then node_modules/.bin/next start -p ${PORT}; else npm run start; fi"]
`

const dockerfileGo = `# Multi-stage Dockerfile for Go services. Distroless runtime, static binary,
# non-root. Generated by Ironflyer's DeployGate.

FROM golang:1.23-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -trimpath -ldflags="-s -w" -o /out/app ./...

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/app /app
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app"]
`

const flyTomlTemplate = `app = "{{APP_NAME}}"
primary_region = "{{PRIMARY_REGION}}"

[build]
  dockerfile = "Dockerfile"

[env]
  PORT = "{{PORT}}"

[http_service]
  internal_port = {{PORT}}
  force_https = true
  auto_stop_machines = "stop"
  auto_start_machines = true
  min_machines_running = 0
  processes = ["app"]

  [[http_service.checks]]
    interval = "30s"
    timeout = "5s"
    grace_period = "10s"
    method = "GET"
    path = "/health"

[[vm]]
  cpu_kind = "shared"
  cpus = 1
  memory_mb = 512
`

const railwayJSONTemplate = `{
  "$schema": "https://railway.app/railway.schema.json",
  "build": {
    "builder": "DOCKERFILE",
    "dockerfilePath": "Dockerfile"
  },
  "deploy": {
    "startCommand": "{{START_COMMAND}}",
    "restartPolicyType": "ON_FAILURE",
    "restartPolicyMaxRetries": 5,
    "healthcheckPath": "/health",
    "healthcheckTimeout": 30,
    "numReplicas": 1
  }
}
`

const githubWorkflow = `name: deploy

on:
  push:
    branches: [main]
  workflow_dispatch:

concurrency:
  group: deploy-${{ github.ref }}
  cancel-in-progress: true

jobs:
  fly:
    if: ${{ vars.IRONFLYER_TARGET == 'fly' || vars.IRONFLYER_TARGET == '' }}
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: superfly/flyctl-actions/setup-flyctl@master
      - run: flyctl deploy --remote-only
        env:
          FLY_API_TOKEN: ${{ secrets.FLY_API_TOKEN }}

  railway:
    if: ${{ vars.IRONFLYER_TARGET == 'railway' }}
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with: { node-version: '20' }
      - run: npm install -g @railway/cli
      - run: railway up --service ${{ vars.RAILWAY_SERVICE }}
        env:
          RAILWAY_TOKEN: ${{ secrets.RAILWAY_TOKEN }}
`
