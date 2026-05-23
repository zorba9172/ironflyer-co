// GoHTTPScaffolder — chi v5 + pgx + zerolog skeleton for Go HTTP
// services. Triggers when the stack says "go" together with a
// backend/api/http hint, or when stories speak of services or
// microservices. Same shape as the Next.js packs.

package finisher

import (
	"context"
	"strings"

	"ironflyer/apps/orchestrator/internal/domain"
)

type GoHTTPScaffolder struct{}

func (GoHTTPScaffolder) Name() string { return "go-chi" }

func (GoHTTPScaffolder) Applies(p *domain.Project) bool {
	if p == nil {
		return false
	}
	stack := strings.ToLower(p.Spec.Stack.Frontend + " " + p.Spec.Stack.Backend)
	hasGo := strings.Contains(stack, "go") || strings.Contains(stack, "golang") ||
		strings.Contains(stack, "chi") || strings.Contains(stack, "echo") ||
		strings.Contains(stack, "gin")
	hasServerHint := strings.Contains(stack, "backend") || strings.Contains(stack, "api") ||
		strings.Contains(stack, "http") || strings.Contains(stack, "server") ||
		strings.Contains(stack, "service")
	if hasGo && hasServerHint {
		return true
	}
	if strings.Contains(stack, "chi") || strings.Contains(stack, "gin") {
		return true
	}
	for _, s := range p.Spec.UserStories {
		body := strings.ToLower(s.IWant + " " + s.SoThat + " " + strings.Join(s.Acceptance, " "))
		if strings.Contains(body, "microservice") || strings.Contains(body, "grpc service") {
			return true
		}
		if hasGo && strings.Contains(body, "service") {
			return true
		}
	}
	return false
}

// goModuleName turns a project name into a module-friendly slug. Go
// module paths are lowercase with optional hyphens; underscores are
// legal but discouraged. We default to ironflyer/<slug>.
func goModuleName(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return "ironflyer/service"
	}
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "ironflyer/service"
	}
	return "ironflyer/" + out
}

func (GoHTTPScaffolder) Scaffold(_ context.Context, p *domain.Project) (DomainScaffold, error) {
	module := "ironflyer/service"
	if p != nil {
		module = goModuleName(p.Name)
	}

	goMod := "" +
		"module " + module + "\n" +
		"\n" +
		"go 1.23\n" +
		"\n" +
		"require (\n" +
		"\tgithub.com/go-chi/chi/v5 v5.1.0\n" +
		"\tgithub.com/jackc/pgx/v5 v5.7.1\n" +
		"\tgithub.com/joho/godotenv v1.5.1\n" +
		"\tgithub.com/rs/zerolog v1.33.0\n" +
		")\n"

	mainGo := "" +
		"// cmd/server is the service entrypoint. It loads .env, builds the\n" +
		"// pgx pool, wires the chi router, and listens on PORT (default\n" +
		"// 8080). Keep the file thin: handlers live in internal/handlers/.\n" +
		"package main\n" +
		"\n" +
		"import (\n" +
		"\t\"context\"\n" +
		"\t\"net/http\"\n" +
		"\t\"os\"\n" +
		"\t\"time\"\n" +
		"\n" +
		"\t\"github.com/go-chi/chi/v5\"\n" +
		"\t\"github.com/joho/godotenv\"\n" +
		"\t\"github.com/rs/zerolog\"\n" +
		"\t\"github.com/rs/zerolog/log\"\n" +
		"\n" +
		"\t\"" + module + "/internal/handlers\"\n" +
		"\t\"" + module + "/internal/middleware\"\n" +
		"\t\"" + module + "/internal/store\"\n" +
		")\n" +
		"\n" +
		"func main() {\n" +
		"\tzerolog.TimeFieldFormat = zerolog.TimeFormatUnix\n" +
		"\tlog.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})\n" +
		"\n" +
		"\t_ = godotenv.Load(\".env.local\", \".env\")\n" +
		"\n" +
		"\tdsn := os.Getenv(\"DATABASE_URL\")\n" +
		"\tif dsn == \"\" {\n" +
		"\t\tlog.Fatal().Msg(\"DATABASE_URL is required (the DB scaffold writes it to .env.local)\")\n" +
		"\t}\n" +
		"\n" +
		"\tctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)\n" +
		"\tdefer cancel()\n" +
		"\tpool, err := store.NewPool(ctx, dsn)\n" +
		"\tif err != nil {\n" +
		"\t\tlog.Fatal().Err(err).Msg(\"connect to postgres\")\n" +
		"\t}\n" +
		"\tdefer pool.Close()\n" +
		"\n" +
		"\tusers := handlers.NewUsers(pool)\n" +
		"\n" +
		"\tr := chi.NewRouter()\n" +
		"\tr.Use(middleware.Logger)\n" +
		"\tr.Get(\"/health\", handlers.Health)\n" +
		"\tr.Get(\"/version\", handlers.Version)\n" +
		"\tr.Get(\"/users/{id}\", users.Get)\n" +
		"\n" +
		"\tport := os.Getenv(\"PORT\")\n" +
		"\tif port == \"\" {\n" +
		"\t\tport = \"8080\"\n" +
		"\t}\n" +
		"\tsrv := &http.Server{\n" +
		"\t\tAddr:              \":\" + port,\n" +
		"\t\tHandler:           r,\n" +
		"\t\tReadHeaderTimeout: 5 * time.Second,\n" +
		"\t}\n" +
		"\tlog.Info().Str(\"addr\", srv.Addr).Msg(\"listening\")\n" +
		"\tif err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {\n" +
		"\t\tlog.Fatal().Err(err).Msg(\"http server\")\n" +
		"\t}\n" +
		"}\n"

	usersGo := "" +
		"// handlers — example HTTP handlers. Health and Version are\n" +
		"// stateless utilities; Users.Get demonstrates how a handler\n" +
		"// takes a dependency (the pgx pool) and renders JSON.\n" +
		"package handlers\n" +
		"\n" +
		"import (\n" +
		"\t\"encoding/json\"\n" +
		"\t\"net/http\"\n" +
		"\n" +
		"\t\"github.com/go-chi/chi/v5\"\n" +
		"\t\"github.com/jackc/pgx/v5/pgxpool\"\n" +
		"\t\"github.com/rs/zerolog/log\"\n" +
		")\n" +
		"\n" +
		"func Health(w http.ResponseWriter, _ *http.Request) {\n" +
		"\twriteJSON(w, http.StatusOK, map[string]string{\"status\": \"ok\"})\n" +
		"}\n" +
		"\n" +
		"func Version(w http.ResponseWriter, _ *http.Request) {\n" +
		"\twriteJSON(w, http.StatusOK, map[string]string{\"version\": \"0.1.0\"})\n" +
		"}\n" +
		"\n" +
		"type Users struct {\n" +
		"\tpool *pgxpool.Pool\n" +
		"}\n" +
		"\n" +
		"func NewUsers(p *pgxpool.Pool) *Users { return &Users{pool: p} }\n" +
		"\n" +
		"func (u *Users) Get(w http.ResponseWriter, r *http.Request) {\n" +
		"\tid := chi.URLParam(r, \"id\")\n" +
		"\tif id == \"\" {\n" +
		"\t\twriteJSON(w, http.StatusBadRequest, map[string]string{\"error\": \"missing id\"})\n" +
		"\t\treturn\n" +
		"\t}\n" +
		"\tvar email string\n" +
		"\terr := u.pool.QueryRow(r.Context(), \"SELECT email FROM users WHERE id::text = $1\", id).Scan(&email)\n" +
		"\tif err != nil {\n" +
		"\t\tlog.Warn().Err(err).Str(\"id\", id).Msg(\"user lookup failed\")\n" +
		"\t\twriteJSON(w, http.StatusNotFound, map[string]string{\"error\": \"not found\"})\n" +
		"\t\treturn\n" +
		"\t}\n" +
		"\twriteJSON(w, http.StatusOK, map[string]string{\"id\": id, \"email\": email})\n" +
		"}\n" +
		"\n" +
		"func writeJSON(w http.ResponseWriter, status int, body any) {\n" +
		"\tw.Header().Set(\"Content-Type\", \"application/json\")\n" +
		"\tw.WriteHeader(status)\n" +
		"\t_ = json.NewEncoder(w).Encode(body)\n" +
		"}\n"

	storeGo := "" +
		"// store — thin pgx pool wrapper. Keeps pool construction +\n" +
		"// pool tuning in one place so handlers depend on the type, not\n" +
		"// the connection string.\n" +
		"package store\n" +
		"\n" +
		"import (\n" +
		"\t\"context\"\n" +
		"\t\"time\"\n" +
		"\n" +
		"\t\"github.com/jackc/pgx/v5/pgxpool\"\n" +
		")\n" +
		"\n" +
		"func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {\n" +
		"\tcfg, err := pgxpool.ParseConfig(dsn)\n" +
		"\tif err != nil {\n" +
		"\t\treturn nil, err\n" +
		"\t}\n" +
		"\tcfg.MaxConns = 10\n" +
		"\tcfg.MinConns = 1\n" +
		"\tcfg.MaxConnLifetime = 30 * time.Minute\n" +
		"\tcfg.HealthCheckPeriod = 30 * time.Second\n" +
		"\treturn pgxpool.NewWithConfig(ctx, cfg)\n" +
		"}\n"

	middlewareGo := "" +
		"// middleware — zerolog request logger. Logs method, path,\n" +
		"// status, and duration for every request. Add tracing or\n" +
		"// auth middleware in this package as the surface grows.\n" +
		"package middleware\n" +
		"\n" +
		"import (\n" +
		"\t\"net/http\"\n" +
		"\t\"time\"\n" +
		"\n" +
		"\t\"github.com/rs/zerolog/log\"\n" +
		")\n" +
		"\n" +
		"type statusWriter struct {\n" +
		"\thttp.ResponseWriter\n" +
		"\tstatus int\n" +
		"}\n" +
		"\n" +
		"func (s *statusWriter) WriteHeader(code int) {\n" +
		"\ts.status = code\n" +
		"\ts.ResponseWriter.WriteHeader(code)\n" +
		"}\n" +
		"\n" +
		"func Logger(next http.Handler) http.Handler {\n" +
		"\treturn http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\n" +
		"\t\tstart := time.Now()\n" +
		"\t\tsw := &statusWriter{ResponseWriter: w, status: 200}\n" +
		"\t\tnext.ServeHTTP(sw, r)\n" +
		"\t\tlog.Info().\n" +
		"\t\t\tStr(\"method\", r.Method).\n" +
		"\t\t\tStr(\"path\", r.URL.Path).\n" +
		"\t\t\tInt(\"status\", sw.status).\n" +
		"\t\t\tDur(\"dur\", time.Since(start)).\n" +
		"\t\t\tMsg(\"http\")\n" +
		"\t})\n" +
		"}\n"

	dockerfile := "" +
		"# Multi-stage: build the static binary in golang:1.23, ship it on\n" +
		"# distroless static-debian12 so the runtime contains only the binary\n" +
		"# and CA certs.\n" +
		"FROM golang:1.23 AS builder\n" +
		"WORKDIR /src\n" +
		"COPY go.mod ./\n" +
		"RUN go mod download\n" +
		"COPY . .\n" +
		"RUN CGO_ENABLED=0 GOOS=linux go build -ldflags=\"-s -w\" -o /out/server ./cmd/server\n" +
		"\n" +
		"FROM gcr.io/distroless/static-debian12:nonroot AS runtime\n" +
		"WORKDIR /\n" +
		"COPY --from=builder /out/server /server\n" +
		"USER nonroot:nonroot\n" +
		"EXPOSE 8080\n" +
		"ENTRYPOINT [\"/server\"]\n"

	dockerignore := "" +
		".git\n" +
		".env\n" +
		".env.local\n" +
		"*.md\n" +
		"Dockerfile\n" +
		".dockerignore\n" +
		"bin/\n" +
		"dist/\n"

	gitignore := "" +
		"/bin\n" +
		"/dist\n" +
		".env\n" +
		".env.local\n" +
		"*.log\n" +
		"go.sum\n"

	files := map[string]string{
		"go.mod":                          goMod,
		"cmd/server/main.go":              mainGo,
		"internal/handlers/users.go":      usersGo,
		"internal/store/postgres.go":      storeGo,
		"internal/middleware/logger.go":   middlewareGo,
		"Dockerfile":                      dockerfile,
		".dockerignore":                   dockerignore,
		".gitignore":                      gitignore,
	}

	contract := "Go HTTP service scaffold: chi v5 + pgx v5 + zerolog + godotenv.\n" +
		"\n" +
		"Already provisioned:\n" +
		"- go.mod                          module=" + module + ", go 1.23, pinned deps\n" +
		"- cmd/server/main.go              entrypoint: env -> pool -> router -> http.Server\n" +
		"- internal/handlers/users.go      Health, Version, Users{} with .Get\n" +
		"- internal/store/postgres.go      pgxpool builder (max 10, healthchecks)\n" +
		"- internal/middleware/logger.go   zerolog request logger\n" +
		"- Dockerfile                      golang:1.23 -> distroless static-debian12\n" +
		"- .dockerignore                   keeps .env + scratch dirs out of context\n" +
		"- .gitignore                      ignores bin/, dist/, .env*, go.sum\n" +
		"\n" +
		"Run locally with `go run ./cmd/server`. Required env:\n" +
		"  DATABASE_URL  postgres DSN (loaded from .env.local first, then .env)\n" +
		"  PORT          optional, defaults to 8080\n" +
		"\n" +
		"Layout rules: cmd/<binary>/main.go stays thin; HTTP handlers go\n" +
		"in internal/handlers/, persistence in internal/store/, and any\n" +
		"shared HTTP middleware in internal/middleware/. Do not import\n" +
		"internal/* from outside this module.\n"

	return DomainScaffold{Files: files, Contract: contract}, nil
}
