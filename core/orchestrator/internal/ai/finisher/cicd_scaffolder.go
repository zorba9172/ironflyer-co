// Package finisher — CI/CD scaffolder. Ironflyer's promise is finished
// software, and "finished" without deploy automation is a lie. Every
// serious project ships with three GitHub Actions workflows (one per
// supported toolchain), a container build + push to GHCR, an Argo CD
// Application manifest, and a baseline Kubernetes deployment stack
// (Deployment + Service + Ingress + HPA). The operator search-replaces
// <APP_NAME> after the first scaffold; everything else just works.
//
// We deliberately do not introspect the project's filesystem to pick
// one language at scaffold time. Projects mutate, toolchains stack
// (a Next.js frontend + Go API is one repo), and a dynamic matrix in
// a single workflow is fragile. The three-file approach with `paths:`
// guards is boring and reliable — only the relevant workflow fires
// when files in its language tree change.

package finisher

import (
	"context"
	"strings"

	"ironflyer/core/orchestrator/internal/ai/domain"
)

// CICDScaffolder ships a production-grade CI/CD bundle to every
// project that opts in: GitHub Actions for tests + container build
// + push, an Argo CD Application manifest for declarative K8s
// deploys, and a baseline Helm-friendly K8s overlay. Triggers on any
// non-trivial project (>= 3 stories OR an explicit "production" /
// "ci" / "cd" / "deploy" / "kubernetes" / "k8s" / "argo" mention in
// the spec).
type CICDScaffolder struct{}

func (CICDScaffolder) Name() string { return "cicd" }

func (CICDScaffolder) Applies(p *domain.Project) bool {
	if p == nil {
		return false
	}
	if len(p.Spec.UserStories) >= 3 {
		return true
	}
	hay := strings.ToLower(
		p.Description + " " +
			p.Spec.Idea + " " +
			p.Spec.Stack.Frontend + " " +
			p.Spec.Stack.Backend + " " +
			p.Spec.Stack.Storage + " " +
			p.Spec.Stack.Auth,
	)
	needles := []string{
		"production", "ci", "cd", "deploy", "kubernetes", "k8s", "argo",
	}
	for _, n := range needles {
		if strings.Contains(hay, n) {
			return true
		}
	}
	for _, s := range p.Spec.UserStories {
		body := strings.ToLower(s.IWant + " " + s.SoThat + " " + strings.Join(s.Acceptance, " "))
		for _, n := range needles {
			if strings.Contains(body, n) {
				return true
			}
		}
	}
	return false
}

func (CICDScaffolder) Scaffold(_ context.Context, _ *domain.Project) (DomainScaffold, error) {
	files := map[string]string{
		".github/workflows/ci-node.yml":   ciNodeWorkflow,
		".github/workflows/ci-go.yml":     ciGoWorkflow,
		".github/workflows/ci-python.yml": ciPythonWorkflow,
		".github/workflows/deploy.yml":    deployWorkflow,
		"argocd/app.yaml":                 argoApp,
		"infra/k8s/deployment.yaml":       k8sDeployment,
		"infra/k8s/service.yaml":          k8sService,
		"infra/k8s/ingress.yaml":          k8sIngress,
		"infra/k8s/hpa.yaml":              k8sHPA,
	}
	return DomainScaffold{Files: files, Contract: cicdContract}, nil
}

// ============================================================
// GitHub Actions — one workflow per toolchain, gated by paths.
// Only the relevant workflow fires when files in its language
// tree change. This is intentionally boring; dynamic matrices
// look clever in review and break six months later.
// ============================================================

const ciNodeWorkflow = `name: ci-node

on:
  push:
    branches: [main]
    paths:
      - "**/package.json"
      - "**/package-lock.json"
      - "**/pnpm-lock.yaml"
      - "**/yarn.lock"
      - "**/*.ts"
      - "**/*.tsx"
      - "**/*.js"
      - "**/*.jsx"
      - ".github/workflows/ci-node.yml"
  pull_request:
    paths:
      - "**/package.json"
      - "**/package-lock.json"
      - "**/pnpm-lock.yaml"
      - "**/yarn.lock"
      - "**/*.ts"
      - "**/*.tsx"
      - "**/*.js"
      - "**/*.jsx"
      - ".github/workflows/ci-node.yml"

permissions:
  contents: read

jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        node: ["20", "22"]
    steps:
      - uses: actions/checkout@v4
      - name: Setup Node ${{ matrix.node }}
        uses: actions/setup-node@v4
        with:
          node-version: ${{ matrix.node }}
          cache: npm
      - name: Install
        run: |
          if [ -f package-lock.json ]; then npm ci
          elif [ -f pnpm-lock.yaml ]; then corepack enable && pnpm install --frozen-lockfile
          elif [ -f yarn.lock ]; then corepack enable && yarn install --frozen-lockfile
          else npm install
          fi
      - name: Type-check
        run: |
          if npm run | grep -q "^  typecheck"; then npm run typecheck
          elif [ -f tsconfig.json ]; then npx --yes tsc --noEmit
          fi
      - name: Lint
        run: |
          if npm run | grep -q "^  lint"; then npm run lint; fi
      - name: Test
        run: |
          if npm run | grep -q "^  test"; then npm test --silent; fi
`

const ciGoWorkflow = `name: ci-go

on:
  push:
    branches: [main]
    paths:
      - "**/go.mod"
      - "**/go.sum"
      - "**/*.go"
      - ".github/workflows/ci-go.yml"
  pull_request:
    paths:
      - "**/go.mod"
      - "**/go.sum"
      - "**/*.go"
      - ".github/workflows/ci-go.yml"

permissions:
  contents: read

jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        go: ["1.22", "1.23"]
    steps:
      - uses: actions/checkout@v4
      - name: Setup Go ${{ matrix.go }}
        uses: actions/setup-go@v5
        with:
          go-version: ${{ matrix.go }}
          cache: true
      - name: Build
        run: go build ./...
      - name: Vet
        run: go vet ./...
      - name: Test
        run: go test ./... -race -count=1
`

const ciPythonWorkflow = `name: ci-python

on:
  push:
    branches: [main]
    paths:
      - "**/pyproject.toml"
      - "**/poetry.lock"
      - "**/requirements*.txt"
      - "**/*.py"
      - ".github/workflows/ci-python.yml"
  pull_request:
    paths:
      - "**/pyproject.toml"
      - "**/poetry.lock"
      - "**/requirements*.txt"
      - "**/*.py"
      - ".github/workflows/ci-python.yml"

permissions:
  contents: read

jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        python: ["3.11", "3.12"]
    steps:
      - uses: actions/checkout@v4
      - name: Setup Python ${{ matrix.python }}
        uses: actions/setup-python@v5
        with:
          python-version: ${{ matrix.python }}
          cache: pip
      - name: Install
        run: |
          python -m pip install --upgrade pip
          if [ -f pyproject.toml ]; then pip install ".[dev]" || pip install .; fi
          if [ -f requirements.txt ]; then pip install -r requirements.txt; fi
          if [ -f requirements-dev.txt ]; then pip install -r requirements-dev.txt; fi
      - name: Lint
        run: |
          if command -v ruff >/dev/null 2>&1; then ruff check .; fi
      - name: Type-check
        run: |
          if command -v mypy >/dev/null 2>&1; then mypy . || true; fi
      - name: Test
        run: |
          if command -v pytest >/dev/null 2>&1; then pytest -q; fi
`

// ============================================================
// Deploy workflow — runs after CI on push to main. Builds the
// Dockerfile, pushes to GHCR with the commit SHA + `latest`,
// then applies the K8s manifests via kubectl. Operators who
// prefer Argo can disable the kubectl step (it's a no-op once
// the Argo Application is syncing the same paths).
// ============================================================

const deployWorkflow = `name: deploy

on:
  push:
    branches: [main]
  workflow_dispatch: {}

permissions:
  contents: read
  packages: write
  id-token: write

jobs:
  build-and-push:
    runs-on: ubuntu-latest
    outputs:
      image: ${{ steps.meta.outputs.image }}
    steps:
      - uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          # Operator must populate GHCR_PAT (a PAT with write:packages) or
          # rely on the default GITHUB_TOKEN if the repo grants packages: write.
          password: ${{ secrets.GHCR_PAT || secrets.GITHUB_TOKEN }}

      - name: Compute image tags
        id: meta
        run: |
          IMAGE="ghcr.io/${GITHUB_REPOSITORY,,}"
          echo "image=${IMAGE}" >> "$GITHUB_OUTPUT"
          echo "tag_sha=${IMAGE}:${GITHUB_SHA::12}" >> "$GITHUB_OUTPUT"
          echo "tag_latest=${IMAGE}:latest" >> "$GITHUB_OUTPUT"

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: .
          push: true
          tags: |
            ${{ steps.meta.outputs.tag_sha }}
            ${{ steps.meta.outputs.tag_latest }}
          cache-from: type=gha
          cache-to: type=gha,mode=max

      - name: Install cosign
        uses: sigstore/cosign-installer@v3

      - name: Sign image (keyless OIDC)
        env:
          COSIGN_EXPERIMENTAL: "1"
        run: |
          cosign sign --yes ${{ steps.meta.outputs.tag_sha }}
          cosign sign --yes ${{ steps.meta.outputs.tag_latest }}

  rollout:
    needs: build-and-push
    runs-on: ubuntu-latest
    # The operator MUST populate IRONFLYER_KUBE_CONFIG (a base64-encoded
    # kubeconfig pointing at the target cluster). Without it this job is
    # a no-op and Argo CD remains the only deploy path — that's fine.
    if: ${{ vars.IRONFLYER_DEPLOY_MODE != 'argo' }}
    steps:
      - uses: actions/checkout@v4

      - name: Load kubeconfig
        env:
          KUBECONFIG_B64: ${{ secrets.IRONFLYER_KUBE_CONFIG }}
        run: |
          if [ -z "$KUBECONFIG_B64" ]; then
            echo "IRONFLYER_KUBE_CONFIG is not set; skipping kubectl rollout."
            echo "Set IRONFLYER_DEPLOY_MODE=argo in repo vars to silence this."
            exit 0
          fi
          mkdir -p "$HOME/.kube"
          echo "$KUBECONFIG_B64" | base64 -d > "$HOME/.kube/config"

      - name: Apply manifests
        run: |
          if [ ! -f "$HOME/.kube/config" ]; then exit 0; fi
          kubectl apply -f infra/k8s/

      - name: Set image to the new digest
        env:
          IMAGE: ghcr.io/${{ github.repository }}
          SHA: ${{ github.sha }}
        run: |
          if [ ! -f "$HOME/.kube/config" ]; then exit 0; fi
          SHORT="${SHA:0:12}"
          kubectl set image deployment/<APP_NAME> app="${IMAGE}:${SHORT}" --record=false
          kubectl rollout status deployment/<APP_NAME> --timeout=180s
`

// ============================================================
// Argo CD Application — the declarative deploy path. Points at
// this repo's infra/k8s/ tree; auto-prune + self-heal so drift
// is reconciled within the sync interval.
// ============================================================

const argoApp = `apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: <APP_NAME>
  namespace: argocd
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: default
  source:
    # Operator: replace with the HTTPS URL of this repo.
    repoURL: https://github.com/OWNER/REPO.git
    targetRevision: main
    path: infra/k8s
  destination:
    server: https://kubernetes.default.svc
    namespace: default
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
      - ApplyOutOfSyncOnly=true
      - PrunePropagationPolicy=foreground
    retry:
      limit: 5
      backoff:
        duration: 10s
        factor: 2
        maxDuration: 3m
`

// ============================================================
// Kubernetes baseline — Deployment + Service + Ingress + HPA.
// <APP_NAME> placeholders are search-replaced after scaffold.
// ============================================================

const k8sDeployment = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: <APP_NAME>
  labels:
    app.kubernetes.io/name: <APP_NAME>
    app.kubernetes.io/managed-by: ironflyer
spec:
  replicas: 2
  revisionHistoryLimit: 5
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 0
      maxSurge: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: <APP_NAME>
  template:
    metadata:
      labels:
        app.kubernetes.io/name: <APP_NAME>
    spec:
      automountServiceAccountToken: false
      securityContext:
        runAsNonRoot: true
        runAsUser: 10001
        seccompProfile:
          type: RuntimeDefault
      containers:
        - name: app
          image: ghcr.io/<APP_NAME>/<APP_NAME>:latest
          imagePullPolicy: IfNotPresent
          ports:
            - name: http
              containerPort: 8080
              protocol: TCP
          env:
            - name: PORT
              value: "8080"
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 1000m
              memory: 512Mi
          readinessProbe:
            httpGet:
              path: /readyz
              port: http
            initialDelaySeconds: 3
            periodSeconds: 5
            timeoutSeconds: 2
            failureThreshold: 3
          livenessProbe:
            httpGet:
              path: /livez
              port: http
            initialDelaySeconds: 10
            periodSeconds: 15
            timeoutSeconds: 2
            failureThreshold: 3
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop: ["ALL"]
`

const k8sService = `apiVersion: v1
kind: Service
metadata:
  name: <APP_NAME>
  labels:
    app.kubernetes.io/name: <APP_NAME>
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: <APP_NAME>
  ports:
    - name: http
      port: 8080
      targetPort: http
      protocol: TCP
`

const k8sIngress = `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: <APP_NAME>
  annotations:
    # cert-manager issues + renews TLS automatically via Let's Encrypt.
    # Operator: ensure a ClusterIssuer named "letsencrypt-prod" exists.
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - <APP_NAME>.example.com
      secretName: <APP_NAME>-tls
  rules:
    - host: <APP_NAME>.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: <APP_NAME>
                port:
                  number: 8080
`

const k8sHPA = `apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: <APP_NAME>
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: <APP_NAME>
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
        - type: Percent
          value: 50
          periodSeconds: 60
    scaleUp:
      stabilizationWindowSeconds: 30
      policies:
        - type: Percent
          value: 100
          periodSeconds: 30
        - type: Pods
          value: 2
          periodSeconds: 30
      selectPolicy: Max
`

const cicdContract = `CI/CD scaffold: GitHub Actions + GHCR + Argo CD + Kubernetes baseline.

Already provisioned by Ironflyer:
- /.github/workflows/ci-node.yml    -> Node 20/22 matrix, runs only when JS/TS files change
- /.github/workflows/ci-go.yml      -> Go 1.22/1.23 matrix, runs only when Go files change
- /.github/workflows/ci-python.yml  -> Python 3.11/3.12 matrix, runs only when Python files change
- /.github/workflows/deploy.yml     -> on push to main: build, cosign-sign, push to GHCR, kubectl rollout
- /argocd/app.yaml                  -> Argo CD Application syncing infra/k8s/
- /infra/k8s/deployment.yaml        -> 2 replicas, /readyz + /livez probes, non-root, read-only FS
- /infra/k8s/service.yaml           -> ClusterIP on port 8080
- /infra/k8s/ingress.yaml           -> NGINX ingress + cert-manager TLS (letsencrypt-prod)
- /infra/k8s/hpa.yaml               -> HPA 2..10 replicas on CPU 70%

Why three CI workflows instead of one dynamic matrix:
A dynamic matrix that detects the toolchain at runtime looks clean but breaks
the moment a project stacks toolchains (Next.js frontend + Go API in the same
repo is the common case). Three workflows guarded by "paths:" filters keep
each pipeline narrow and only fire the one that actually has work to do.

Required secrets / vars on the GitHub repo:
- GHCR_PAT              (optional) PAT with write:packages; falls back to GITHUB_TOKEN
- IRONFLYER_KUBE_CONFIG (required for kubectl rollout) base64-encoded kubeconfig
- vars.IRONFLYER_DEPLOY_MODE = "argo" to skip the kubectl rollout entirely

Operator search-replace checklist after first scaffold:
1. Replace every <APP_NAME> in /argocd/app.yaml + /infra/k8s/*.yaml with the
   real app name (lowercase, DNS-safe, e.g. "checkout-api").
2. Set the repoURL in /argocd/app.yaml to your repo's HTTPS clone URL.
3. Pick a real hostname in /infra/k8s/ingress.yaml (the placeholder is
   <APP_NAME>.example.com).
4. Confirm a ClusterIssuer named "letsencrypt-prod" exists in the cluster,
   or change the cert-manager.io/cluster-issuer annotation accordingly.

Readiness contract:
The Deployment probes /readyz and /livez. Your app MUST expose these as
plaintext 200-OK endpoints (matches Ironflyer's orchestrator convention):
- /readyz returns 200 once deps (DB, queues) are reachable
- /livez returns 200 while the process is healthy enough to keep serving

Rules for the Coder:
1. Do NOT replace files under /.github/workflows/, /argocd/, or /infra/k8s/
   without first reading this contract. They are the deploy contract.
2. If you add a new language to the project, add a fourth ci-<lang>.yml file
   following the same shape (paths filter, matrix, install/lint/test).
3. Image tags MUST be commit-SHA-prefixed in production; "latest" is for
   humans, never for "kubectl set image" against a live workload.
4. Cluster secrets belong in the cluster (sealed-secrets or external-secrets),
   never in this repo. The workflow secrets above are deploy-time only.
`
