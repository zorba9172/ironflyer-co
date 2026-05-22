package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"sort"
	"strings"

	"ironflyer/apps/orchestrator/internal/domain"
)

// StackDetector inspects a workspace's tree (over the runtime File API) and
// infers the project's Frontend / Backend / Storage / Auth. The detector is
// deliberately heuristic — the orchestrator's brainstorm pass will refine
// these later. We aim for "confident first guess" rather than "exhaustive".
type StackDetector struct {
	RuntimeURL string
	HTTP       *http.Client
}

// runtimeFileEntry mirrors sandbox.FileEntry on the runtime side. Kept local
// so this package doesn't bend the dependency graph backwards into runtime/.
type runtimeFileEntry struct {
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"isDir"`
}

// Detect walks the workspace, applies the heuristic ruleset, and returns a
// StackDecision plus a list of human-readable warnings ("framework detected
// but DB unknown — Postgres assumed", etc.). The Bearer token is forwarded
// to the runtime so per-user ownership is preserved.
func (d *StackDetector) Detect(ctx context.Context, userBearer, workspaceID string) (domain.StackDecision, []string, []domain.FileNode, error) {
	if d == nil || strings.TrimSpace(d.RuntimeURL) == "" {
		return domain.StackDecision{}, nil, nil, errors.New("stack detector: runtime URL not configured")
	}
	files, err := d.listFiles(ctx, userBearer, workspaceID)
	if err != nil {
		return domain.StackDecision{}, nil, nil, fmt.Errorf("list files: %w", err)
	}

	pathSet := make(map[string]bool, len(files))
	for _, f := range files {
		if !f.IsDir {
			pathSet[strings.ToLower(f.Path)] = true
		}
	}

	stack := domain.StackDecision{}
	warnings := []string{}

	// ---- package.json (Node/TS) -----------------------------------------
	if has(pathSet, "package.json") {
		pj := d.readPackageJSON(ctx, userBearer, workspaceID, "package.json")
		stack.Frontend, stack.Backend = inferFromPackageJSON(pj, &warnings)
	}

	// ---- go.mod ---------------------------------------------------------
	if has(pathSet, "go.mod") {
		gm := d.readText(ctx, userBearer, workspaceID, "go.mod")
		if stack.Backend == "" {
			stack.Backend = inferFromGoMod(gm)
		} else {
			// Hybrid (e.g. Next + Go API) — surface as a backend hint.
			warnings = append(warnings, "Go module detected alongside Node front-end — assuming polyglot monorepo")
		}
	}

	// ---- Rust -----------------------------------------------------------
	if has(pathSet, "cargo.toml") && stack.Backend == "" {
		ct := d.readText(ctx, userBearer, workspaceID, "Cargo.toml")
		stack.Backend = inferFromCargo(ct)
	}

	// ---- Python ---------------------------------------------------------
	if (has(pathSet, "pyproject.toml") || has(pathSet, "requirements.txt")) && stack.Backend == "" {
		py := d.readText(ctx, userBearer, workspaceID, "pyproject.toml")
		if py == "" {
			py = d.readText(ctx, userBearer, workspaceID, "requirements.txt")
		}
		stack.Backend = inferFromPython(py)
	}

	// ---- Ruby / Rails ---------------------------------------------------
	if has(pathSet, "gemfile") && stack.Backend == "" {
		gf := d.readText(ctx, userBearer, workspaceID, "Gemfile")
		stack.Backend = "Ruby"
		if strings.Contains(strings.ToLower(gf), "rails") {
			stack.Backend = "Ruby on Rails"
		}
	}

	// ---- Astro ----------------------------------------------------------
	if has(pathSet, "astro.config.mjs") || has(pathSet, "astro.config.ts") || has(pathSet, "astro.config.js") {
		if stack.Frontend == "" {
			stack.Frontend = "Astro"
		}
	}

	// ---- Java -----------------------------------------------------------
	if (has(pathSet, "pom.xml") || has(pathSet, "build.gradle") || has(pathSet, "build.gradle.kts")) && stack.Backend == "" {
		stack.Backend = "Java"
	}

	// ---- PHP ------------------------------------------------------------
	if has(pathSet, "composer.json") && stack.Backend == "" {
		stack.Backend = "PHP"
	}

	// ---- Storage --------------------------------------------------------
	stack.Storage = inferStorage(ctx, d, userBearer, workspaceID, pathSet, &warnings)

	// ---- Auth (best-effort guess) ---------------------------------------
	stack.Auth = inferAuth(pathSet, &warnings)

	// Fallback labels so a successful import never leaves blank chips.
	if stack.Frontend == "" {
		stack.Frontend = "Unknown frontend"
		warnings = append(warnings, "No obvious frontend manifest found — leaving Frontend blank for manual selection")
	}
	if stack.Backend == "" {
		stack.Backend = "Unknown backend"
		warnings = append(warnings, "No obvious backend manifest found — leaving Backend blank for manual selection")
	}
	if stack.Storage == "" {
		stack.Storage = "Postgres"
		warnings = append(warnings, "No DB binding detected — Postgres assumed as default")
	}
	if stack.Auth == "" {
		stack.Auth = "JWT"
	}

	sample := sampleFiles(files, 64)
	return stack, warnings, sample, nil
}

func has(set map[string]bool, name string) bool {
	if set[name] {
		return true
	}
	// Tolerate detector running against a subdirectory or workspace root.
	for p := range set {
		if path.Base(p) == name {
			return true
		}
	}
	return false
}

// inferFromPackageJSON reads dependencies (+ devDependencies) and returns
// (frontend, backend) labels. We trust framework dependencies more than
// generic ones (React-only is treated as CRA-style SPA).
func inferFromPackageJSON(raw []byte, warnings *[]string) (string, string) {
	if len(raw) == 0 {
		return "", ""
	}
	var pj struct {
		Name            string            `json:"name"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		Scripts         map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(raw, &pj); err != nil {
		*warnings = append(*warnings, "package.json found but failed to parse — frontend detection skipped")
		return "", ""
	}
	deps := map[string]bool{}
	for k := range pj.Dependencies {
		deps[strings.ToLower(k)] = true
	}
	for k := range pj.DevDependencies {
		deps[strings.ToLower(k)] = true
	}
	frontend := ""
	backend := ""
	switch {
	case deps["next"]:
		frontend = "Next.js"
	case deps["nuxt"]:
		frontend = "Nuxt"
	case deps["@remix-run/react"], deps["@remix-run/node"]:
		frontend = "Remix"
	case deps["@sveltejs/kit"]:
		frontend = "SvelteKit"
	case deps["vite"] && deps["react"]:
		frontend = "Vite + React"
	case deps["vite"] && deps["vue"]:
		frontend = "Vite + Vue"
	case deps["vite"]:
		frontend = "Vite"
	case deps["react-scripts"]:
		frontend = "Create React App"
	case deps["react"]:
		frontend = "React (custom)"
	case deps["vue"]:
		frontend = "Vue"
	case deps["@angular/core"]:
		frontend = "Angular"
	case deps["solid-js"]:
		frontend = "Solid"
	}
	switch {
	case deps["@nestjs/core"]:
		backend = "NestJS"
	case deps["express"]:
		backend = "Express (Node)"
	case deps["fastify"]:
		backend = "Fastify (Node)"
	case deps["koa"]:
		backend = "Koa (Node)"
	case deps["hono"]:
		backend = "Hono"
	}
	// Next.js carries its own API routes so we don't double-tag backend
	// unless an explicit server framework is also present.
	if frontend == "Next.js" && backend == "" {
		backend = "Next.js API routes"
	}
	return frontend, backend
}

func inferFromGoMod(text string) string {
	t := strings.ToLower(text)
	switch {
	case strings.Contains(t, "go-chi/chi"):
		return "Go + chi"
	case strings.Contains(t, "gin-gonic/gin"):
		return "Go + Gin"
	case strings.Contains(t, "labstack/echo"):
		return "Go + Echo"
	case strings.Contains(t, "gorilla/mux"):
		return "Go + gorilla/mux"
	case strings.Contains(t, "fiber"):
		return "Go + Fiber"
	}
	return "Go (stdlib)"
}

func inferFromCargo(text string) string {
	t := strings.ToLower(text)
	switch {
	case strings.Contains(t, "actix-web"):
		return "Rust + Actix"
	case strings.Contains(t, "axum"):
		return "Rust + Axum"
	case strings.Contains(t, "rocket"):
		return "Rust + Rocket"
	case strings.Contains(t, "warp"):
		return "Rust + Warp"
	}
	return "Rust"
}

func inferFromPython(text string) string {
	t := strings.ToLower(text)
	switch {
	case strings.Contains(t, "fastapi"):
		return "Python + FastAPI"
	case strings.Contains(t, "django"):
		return "Python + Django"
	case strings.Contains(t, "flask"):
		return "Python + Flask"
	}
	return "Python"
}

func inferStorage(ctx context.Context, d *StackDetector, bearer, ws string, set map[string]bool, warnings *[]string) string {
	// Prisma schema — single source of truth for many TS apps.
	if has(set, "schema.prisma") {
		// Find the actual prisma path so we can read it.
		for p := range set {
			if strings.HasSuffix(p, "prisma/schema.prisma") || strings.HasSuffix(p, "/schema.prisma") || p == "schema.prisma" {
				body := d.readText(ctx, bearer, ws, p)
				switch {
				case strings.Contains(body, `provider = "postgresql"`), strings.Contains(body, `provider="postgresql"`):
					return "Postgres (Prisma)"
				case strings.Contains(body, `provider = "mysql"`), strings.Contains(body, `provider="mysql"`):
					return "MySQL (Prisma)"
				case strings.Contains(body, `provider = "sqlite"`), strings.Contains(body, `provider="sqlite"`):
					return "SQLite (Prisma)"
				case strings.Contains(body, `provider = "mongodb"`), strings.Contains(body, `provider="mongodb"`):
					return "MongoDB (Prisma)"
				}
				return "Prisma (unknown provider)"
			}
		}
	}
	// Drizzle.
	if has(set, "drizzle.config.ts") || has(set, "drizzle.config.js") {
		return "Drizzle ORM"
	}
	// SQLite file checked in.
	for p := range set {
		if strings.HasSuffix(p, ".sqlite") || strings.HasSuffix(p, ".sqlite3") || strings.HasSuffix(p, ".db") {
			return "SQLite"
		}
	}
	// Docker compose hints.
	if has(set, "docker-compose.yml") || has(set, "docker-compose.yaml") || has(set, "compose.yaml") {
		body := d.readText(ctx, bearer, ws, firstMatching(set, "docker-compose.yml", "docker-compose.yaml", "compose.yaml"))
		t := strings.ToLower(body)
		switch {
		case strings.Contains(t, "postgres"):
			return "Postgres"
		case strings.Contains(t, "mysql"), strings.Contains(t, "mariadb"):
			return "MySQL"
		case strings.Contains(t, "mongo"):
			return "MongoDB"
		case strings.Contains(t, "redis"):
			*warnings = append(*warnings, "Redis detected in docker-compose — assuming cache, not primary store")
		}
	}
	// .env hints.
	for p := range set {
		if path.Base(p) == ".env" || path.Base(p) == ".env.example" {
			body := d.readText(ctx, bearer, ws, p)
			t := strings.ToLower(body)
			switch {
			case strings.Contains(t, "postgres://"), strings.Contains(t, "postgresql://"):
				return "Postgres"
			case strings.Contains(t, "mysql://"):
				return "MySQL"
			case strings.Contains(t, "mongodb://"):
				return "MongoDB"
			case strings.Contains(t, "supabase"):
				return "Supabase (Postgres)"
			case strings.Contains(t, "firebase"):
				return "Firebase"
			}
		}
	}
	return ""
}

func inferAuth(set map[string]bool, _ *[]string) string {
	if has(set, "next-auth.config.ts") || has(set, "auth.config.ts") || has(set, "auth.ts") {
		return "NextAuth / Auth.js"
	}
	for p := range set {
		if strings.Contains(p, "clerk") {
			return "Clerk"
		}
		if strings.Contains(p, "supabase") {
			return "Supabase Auth"
		}
		if strings.Contains(p, "firebase") {
			return "Firebase Auth"
		}
	}
	return ""
}

func firstMatching(set map[string]bool, names ...string) string {
	for _, n := range names {
		if set[n] {
			return n
		}
	}
	// fall back to deeper paths.
	for p := range set {
		for _, n := range names {
			if path.Base(p) == n {
				return p
			}
		}
	}
	return names[0]
}

// sampleFiles returns up to `max` representative paths, weighted toward
// shallow paths (more meaningful for the dashboard's file tree summary).
func sampleFiles(files []runtimeFileEntry, max int) []domain.FileNode {
	cleaned := make([]runtimeFileEntry, 0, len(files))
	for _, f := range files {
		if f.IsDir {
			continue
		}
		cleaned = append(cleaned, f)
	}
	sort.SliceStable(cleaned, func(i, j int) bool {
		di := strings.Count(cleaned[i].Path, "/")
		dj := strings.Count(cleaned[j].Path, "/")
		if di != dj {
			return di < dj
		}
		return cleaned[i].Path < cleaned[j].Path
	})
	if len(cleaned) > max {
		cleaned = cleaned[:max]
	}
	out := make([]domain.FileNode, 0, len(cleaned))
	for _, f := range cleaned {
		out = append(out, domain.FileNode{
			Path: f.Path,
			Type: "file",
			Size: int(f.Size),
		})
	}
	return out
}

func (d *StackDetector) httpClient() *http.Client {
	if d.HTTP != nil {
		return d.HTTP
	}
	return http.DefaultClient
}

func (d *StackDetector) listFiles(ctx context.Context, bearer, ws string) ([]runtimeFileEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimRight(d.RuntimeURL, "/")+"/workspaces/"+ws+"/files", nil)
	if err != nil {
		return nil, err
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := d.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("runtime list-files %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out []runtimeFileEntry
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode files: %w", err)
	}
	return out, nil
}

func (d *StackDetector) readPackageJSON(ctx context.Context, bearer, ws, p string) []byte {
	return d.readBytes(ctx, bearer, ws, p)
}

func (d *StackDetector) readText(ctx context.Context, bearer, ws, p string) string {
	if p == "" {
		return ""
	}
	return string(d.readBytes(ctx, bearer, ws, p))
}

func (d *StackDetector) readBytes(ctx context.Context, bearer, ws, p string) []byte {
	if p == "" {
		return nil
	}
	url := strings.TrimRight(d.RuntimeURL, "/") + "/workspaces/" + ws + "/files/" + strings.TrimPrefix(p, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := d.httpClient().Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
	return body
}
