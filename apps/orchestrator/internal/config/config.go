// Package config holds the orchestrator runtime config. Sourced from env,
// validated on startup.
package config

import (
	"fmt"

	"github.com/caarlos0/env/v10"
	"github.com/go-playground/validator/v10"
)

type Config struct {
	Addr      string `env:"IRONFLYER_ADDR" envDefault:":8080"`
	Env       string `env:"IRONFLYER_ENV" envDefault:"dev" validate:"oneof=dev staging prod"`
	LogLevel  string `env:"IRONFLYER_LOG_LEVEL" envDefault:"info" validate:"oneof=debug info warn error"`
	LogFormat string `env:"IRONFLYER_LOG_FORMAT" envDefault:"console" validate:"oneof=console json"`

	// Executor: "embedded" runs the finisher in-process (good for local dev,
	// no extra infra). "temporal" runs it as a Temporal Workflow (production).
	Executor string `env:"IRONFLYER_EXECUTOR" envDefault:"embedded" validate:"oneof=embedded temporal"`

	TemporalAddr      string `env:"TEMPORAL_ADDR" envDefault:"localhost:7233"`
	TemporalNamespace string `env:"TEMPORAL_NAMESPACE" envDefault:"default"`
	TemporalTaskQueue string `env:"TEMPORAL_TASK_QUEUE" envDefault:"ironflyer-finisher"`

	// Auth
	JWTSecret      string `env:"IRONFLYER_JWT_SECRET" envDefault:"dev-secret-change-me"`
	JWTIssuer      string `env:"IRONFLYER_JWT_ISSUER" envDefault:"ironflyer"`
	AuthOptional   bool   `env:"IRONFLYER_AUTH_OPTIONAL" envDefault:"false"`

	AnthropicAPIKey string `env:"ANTHROPIC_API_KEY"`
	AnthropicModel  string `env:"ANTHROPIC_MODEL" envDefault:"claude-opus-4-7"`

	OpenAIAPIKey string `env:"OPENAI_API_KEY"`
	OpenAIModel  string `env:"OPENAI_MODEL" envDefault:"gpt-4o"`

	// Google Gemini provider. Leaving GEMINI_API_KEY empty disables the
	// provider; the router falls back to Anthropic / OpenAI / mock.
	GeminiAPIKey string `env:"GEMINI_API_KEY"`
	GeminiModel  string `env:"GEMINI_MODEL" envDefault:"gemini-2.5-pro"`

	// HuggingFace inference. Powers the memory layer's semantic search
	// (apps/orchestrator/internal/embeddings) and any other future
	// HF-backed provider. Leaving HF_API_KEY empty disables the
	// embedder; memory.Query falls back to substring search.
	HFAPIKey     string `env:"HF_API_KEY"`
	HFEmbedModel string `env:"HF_EMBED_MODEL" envDefault:"BAAI/bge-small-en-v1.5"`

	// OpenAIImageAPIKey is the API key used by the built-in
	// generate_image tool (apps/orchestrator/internal/imagegen). Leave
	// empty to fall back to OpenAIAPIKey; if both are empty the tool
	// is registered but every call returns "image generation disabled".
	OpenAIImageAPIKey string `env:"OPENAI_IMAGE_API_KEY"`

	// GitHub OAuth + integration. Leaving CLIENT_ID empty disables the
	// /auth/github/* endpoints (they return 503).
	GitHubClientID     string `env:"GITHUB_CLIENT_ID"`
	GitHubClientSecret string `env:"GITHUB_CLIENT_SECRET"`
	GitHubRedirectURL  string `env:"GITHUB_REDIRECT_URL" envDefault:"http://localhost:8080/auth/github/callback"`
	// Where to send the browser after we finish the OAuth exchange.
	GitHubPostLoginURL string `env:"GITHUB_POST_LOGIN_URL" envDefault:"http://localhost:3000/app"`

	// Runtime service the orchestrator proxies clone calls to.
	RuntimeURL string `env:"IRONFLYER_RUNTIME_URL" envDefault:"http://localhost:8090"`

	// Stripe — leave SecretKey empty to disable /budget/checkout + webhook
	// (the routes return 503). The Pro/Team/Enterprise price IDs map to our
	// PlanTier values; users buy a tier, not a specific price.
	StripeSecretKey       string `env:"STRIPE_SECRET_KEY"`
	StripeWebhookSecret   string `env:"STRIPE_WEBHOOK_SECRET"`
	StripePricePro        string `env:"STRIPE_PRICE_PRO"`
	StripePriceTeam       string `env:"STRIPE_PRICE_TEAM"`
	StripePriceEnterprise string `env:"STRIPE_PRICE_ENTERPRISE"`
	StripeSuccessURL      string `env:"STRIPE_SUCCESS_URL" envDefault:"http://localhost:3000/app/settings?stripe=success"`
	StripeCancelURL       string `env:"STRIPE_CANCEL_URL" envDefault:"http://localhost:3000/pricing?stripe=cancel"`

	// Storage driver:
	//   memory   — both budget and projects in-process (default, no infra)
	//   postgres — Postgres for budget, memory for projects
	//   surreal  — SurrealDB for projects, memory for budget
	//   hybrid   — Postgres for budget + SurrealDB for projects (production)
	DBDriver    string `env:"IRONFLYER_DB_DRIVER" envDefault:"memory" validate:"oneof=memory postgres surreal hybrid"`
	PostgresURL string `env:"POSTGRES_URL" envDefault:"postgres://ironflyer:ironflyer@localhost:5432/ironflyer?sslmode=disable"`
	SurrealURL  string `env:"SURREAL_URL" envDefault:"ws://localhost:8000/rpc"`
	SurrealNS   string `env:"SURREAL_NS" envDefault:"ironflyer"`
	SurrealDB   string `env:"SURREAL_DB" envDefault:"main"`
	SurrealUser string `env:"SURREAL_USER" envDefault:"root"`
	SurrealPass string `env:"SURREAL_PASS" envDefault:"root"`

	// -------------------- Temporal worker (production) --------------------
	// When TemporalHost is set, main.go boots a Temporal worker on startup so
	// the FinisherWorkflow can run out-of-process. Leaving TemporalHost empty
	// keeps the orchestrator on the embedded executor — no Temporal needed.
	//
	// TemporalAddr above is the legacy key used by the explicit "temporal"
	// executor; TemporalHost is the new opt-in switch wired in main.go. They
	// share TemporalNamespace + TemporalTaskQueue (defined earlier).
	TemporalHost string `env:"IRONFLYER_TEMPORAL_HOST"`

	// Scaffold templates root. Baked into the orchestrator Docker image at
	// /app/templates; dev compose bind-mounts the repo's templates/ over the
	// top so contributors can iterate without rebuilding.
	ScaffoldRoot string `env:"IRONFLYER_SCAFFOLD_ROOT" envDefault:"./templates"`

	// -------------------- Transactional email (optional) ------------------
	// EmailProvider selects the driver: "resend", "sendgrid", or "none".
	// The integration falls back to "none" when API key is missing so the
	// orchestrator stays bootable in dev without secrets configured.
	EmailProvider    string `env:"IRONFLYER_EMAIL_PROVIDER" envDefault:"none" validate:"oneof=resend sendgrid none"`
	EmailAPIKey      string `env:"IRONFLYER_EMAIL_API_KEY"`
	EmailFromAddress string `env:"IRONFLYER_EMAIL_FROM" envDefault:"noreply@ironflyer.dev"`

	// DashboardURL is the public base URL of the Next.js web app. Used to
	// build deep-link CTAs in transactional emails ("Open project →").
	// Defaults to the dev web origin.
	DashboardURL string `env:"IRONFLYER_DASHBOARD_URL" envDefault:"http://localhost:3000"`

	// -------------------- DB provisioner (optional) -----------------------
	// DBProvisioner selects which backend the finisher's DB step calls
	// when a project needs a database. "none" (default) skips provisioning;
	// "supabase" hits the Supabase Management API; "shared-postgres" carves
	// a per-project database out of a single operator-supplied Postgres
	// (works for docker-compose, RDS, internal clusters, etc.).
	DBProvisioner string `env:"IRONFLYER_DB_PROVISIONER" envDefault:"none" validate:"oneof=none supabase shared-postgres"`
	// SharedPostgresAdminDSN points at the admin DSN the shared-postgres
	// provisioner uses to CREATE ROLE / CREATE DATABASE.
	SharedPostgresAdminDSN string `env:"IRONFLYER_SHARED_POSTGRES_ADMIN_DSN"`
	// SharedPostgresPublicHost / SharedPostgresPublicPort override the
	// host:port the per-project DSN returns when the user workspace must
	// reach Postgres on a different network than the orchestrator.
	SharedPostgresPublicHost string `env:"IRONFLYER_SHARED_POSTGRES_PUBLIC_HOST"`
	SharedPostgresPublicPort string `env:"IRONFLYER_SHARED_POSTGRES_PUBLIC_PORT"`
	// SupabasePAT is a Personal Access Token (or service-account token)
	// with project-create scope. Only read when DBProvisioner=="supabase".
	SupabasePAT string `env:"IRONFLYER_SUPABASE_PAT"`
	// SupabaseOrgID is the organization id that owns newly provisioned
	// Supabase projects. Required when DBProvisioner=="supabase".
	SupabaseOrgID string `env:"IRONFLYER_SUPABASE_ORG_ID"`
	// SupabaseRegion is the Supabase region code, e.g. "us-east-1". Empty
	// falls back to the provisioner's own default.
	SupabaseRegion string `env:"IRONFLYER_SUPABASE_REGION"`

	// -------------------- OpenTelemetry tracing (optional) ---------------
	// OTelExporter selects the span exporter:
	//   "none"   — install a no-op TracerProvider (default; zero overhead)
	//   "stdout" — pretty-print spans to stderr (dev / local debugging)
	//   "otlp"   — ship spans over OTLP/HTTP to Honeycomb/Tempo/Jaeger/etc.
	// OTelEndpoint is the OTLP target (host:port or full URL). OTelInsecure
	// disables TLS for plaintext collectors (docker-compose, local dev).
	// OTelHeaders is a comma-separated list of `key=value` pairs sent as HTTP
	// headers on every OTLP request — most vendors use this for auth, e.g.
	// IRONFLYER_OTEL_HEADERS="x-honeycomb-team=YOUR_KEY".
	// OTelSampleRatio caps trace volume: 1.0 = sample everything, 0.1 = 10%,
	// 0 = disable sampling entirely.
	OTelExporter    string  `env:"IRONFLYER_OTEL_EXPORTER" envDefault:"none" validate:"oneof=none stdout otlp"`
	OTelEndpoint    string  `env:"IRONFLYER_OTEL_ENDPOINT"`
	OTelInsecure    bool    `env:"IRONFLYER_OTEL_INSECURE" envDefault:"false"`
	OTelSampleRatio float64 `env:"IRONFLYER_OTEL_SAMPLE_RATIO" envDefault:"1.0"`
	OTelHeaders     string  `env:"IRONFLYER_OTEL_HEADERS"`

	// -------------------- MCP clients (optional) --------------------------
	// MCPServers is a comma-separated list of `name=URL` pairs that
	// the orchestrator should configure as outbound MCP clients —
	// e.g. "notion=https://mcp.notion.com,linear=https://mcp.linear.app".
	// Per-server bearer tokens are read at startup from the env var
	// IRONFLYER_MCP_TOKEN_<UPPERCASE_NAME>. Leaving this empty disables
	// outbound MCP and the Coder runs without external tools.
	MCPServers string `env:"IRONFLYER_MCP_SERVERS"`

	// -------------------- Context7 (default MCP server) -------------------
	// Context7 is a public MCP server that ships fresh library
	// documentation (npm, Go modules, PyPI, etc.). It's enabled by
	// default so the Coder can call `lookup_docs(library, query)` —
	// and the underlying `context7.*` MCP tools — without the
	// operator having to add it to IRONFLYER_MCP_SERVERS. Set
	// IRONFLYER_CONTEXT7_ENABLED=false to disable. The token is
	// optional (Context7 is public) but raises the rate limit.
	Context7Enabled   bool   `env:"IRONFLYER_CONTEXT7_ENABLED" envDefault:"true"`
	Context7AuthToken string `env:"IRONFLYER_CONTEXT7_TOKEN"`
}

func (c Config) UsePostgres() bool {
	return c.DBDriver == "postgres" || c.DBDriver == "hybrid"
}
func (c Config) UseSurreal() bool {
	return c.DBDriver == "surreal" || c.DBDriver == "hybrid"
}

func Load() (Config, error) {
	var c Config
	if err := env.Parse(&c); err != nil {
		return c, fmt.Errorf("env parse: %w", err)
	}
	if err := validator.New().Struct(c); err != nil {
		return c, fmt.Errorf("config validate: %w", err)
	}
	return c, nil
}
