// Package config holds the orchestrator runtime config. Sourced from env,
// validated on startup.
package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v10"
	"github.com/go-playground/validator/v10"
)

type Config struct {
	Addr string `env:"IRONFLYER_ADDR" envDefault:":8080"`
	// CORSOrigins is the comma-separated allowlist of browser origins
	// the CORS middleware reflects into Access-Control-Allow-Origin.
	// Empty = reflect any origin (dev convenience). Production MUST
	// set this to the exact origin(s) of the web SPA.
	CORSOrigins []string `env:"IRONFLYER_CORS_ORIGINS" envSeparator:"," envDefault:""`
	Env         string   `env:"IRONFLYER_ENV" envDefault:"dev" validate:"oneof=dev staging prod"`

	// DevWalletSeedUSD — dev-only convenience. When > 0 AND Env == "dev",
	// the SignUp resolver credits the freshly-minted wallet with this
	// USD amount via wallet.TopUp(stripeSessionID="dev-seed-<userID>")
	// so the operator can immediately run describeIdea without going
	// through Stripe. Ignored outside of dev (resolver-level gate on
	// Env=="dev"). Default 50 so any dev orchestrator boots usable.
	DevWalletSeedUSD float64 `env:"IRONFLYER_DEV_WALLET_SEED_USD" envDefault:"50"`
	LogLevel         string  `env:"IRONFLYER_LOG_LEVEL" envDefault:"info" validate:"oneof=debug info warn error"`
	LogFormat        string  `env:"IRONFLYER_LOG_FORMAT" envDefault:"console" validate:"oneof=console json"`

	// Executor: "embedded" runs the finisher in-process (good for local dev,
	// no extra infra). "temporal" runs it as a Temporal Workflow (production).
	Executor string `env:"IRONFLYER_EXECUTOR" envDefault:"embedded" validate:"oneof=embedded temporal"`

	TemporalAddr      string `env:"TEMPORAL_ADDR" envDefault:"localhost:7233"`
	TemporalNamespace string `env:"TEMPORAL_NAMESPACE" envDefault:"default"`
	TemporalTaskQueue string `env:"TEMPORAL_TASK_QUEUE" envDefault:"ironflyer-finisher"`

	// Auth
	JWTSecret    string `env:"IRONFLYER_JWT_SECRET" envDefault:"dev-secret-change-me"`
	JWTIssuer    string `env:"IRONFLYER_JWT_ISSUER" envDefault:"ironflyer"`
	AuthOptional bool   `env:"IRONFLYER_AUTH_OPTIONAL" envDefault:"false"`

	AnthropicAPIKey string `env:"ANTHROPIC_API_KEY"`
	// General-purpose default. The provider's pickModel promotes to Opus
	// 4.7 for `quality`/`thinking`/`reasoning` and demotes to Haiku 4.5
	// for `cheap`/`fast`/`inline_completion` — see anthropic.go.
	AnthropicModel string `env:"ANTHROPIC_MODEL" envDefault:"claude-sonnet-4-6"`

	OpenAIAPIKey string `env:"OPENAI_API_KEY"`
	OpenAIModel  string `env:"OPENAI_MODEL" envDefault:"gpt-4o"`

	// Google Gemini provider. Leaving GEMINI_API_KEY empty disables the
	// provider; the router falls back to Anthropic / OpenAI / mock.
	GeminiAPIKey string `env:"GEMINI_API_KEY"`
	GeminiModel  string `env:"GEMINI_MODEL" envDefault:"gemini-2.5-pro"`

	// Vercel AI Gateway — OpenAI-compatible proxy that fronts multiple
	// LLM vendors with caching + observability. Disabled when the
	// token is empty; when enabled, registers as an additional arm in
	// the router (the bandit can pick it when it wins on reward).
	// VercelAIGatewayModel is vendor-namespaced ("anthropic/...",
	// "openai/..."); see https://vercel.com/docs/ai-gateway for the
	// supported id catalogue.
	VercelAIGatewayToken string `env:"VERCEL_AI_GATEWAY_TOKEN"`
	VercelAIGatewayURL   string `env:"VERCEL_AI_GATEWAY_URL"   envDefault:"https://gateway.ai.vercel.com/v1"`
	VercelAIGatewayModel string `env:"VERCEL_AI_GATEWAY_MODEL" envDefault:"anthropic/claude-sonnet-4-6"`

	// DeepSeek — OpenAI-compatible REST + SSE at api.deepseek.com.
	// DeepSeekEnabled defaults to true so a freshly-supplied API key
	// brings the provider online without an additional flip. Enterprise
	// operators with compliance constraints (DeepSeek is a Chinese
	// provider) hard-disable independent of the API key by setting
	// IRONFLYER_DEEPSEEK_ENABLED=false.
	DeepSeekAPIKey         string `env:"DEEPSEEK_API_KEY"`
	DeepSeekBaseURL        string `env:"DEEPSEEK_BASE_URL"        envDefault:"https://api.deepseek.com/v1"`
	DeepSeekGeneralModel   string `env:"DEEPSEEK_GENERAL_MODEL"   envDefault:"deepseek-chat"`
	DeepSeekReasoningModel string `env:"DEEPSEEK_REASONING_MODEL" envDefault:"deepseek-reasoner"`
	DeepSeekCoderModel     string `env:"DEEPSEEK_CODER_MODEL"     envDefault:"deepseek-coder"`
	// DeepSeekPreferV3ForCode flips CapCode from deepseek-coder to
	// deepseek-chat (V3). V3 outscores the legacy coder SKU on most
	// code benchmarks; the flag exists so operators can opt back into
	// the coder SKU if their own evals show otherwise.
	DeepSeekPreferV3ForCode bool `env:"DEEPSEEK_PREFER_V3_FOR_CODE" envDefault:"false"`
	// DeepSeekEnabled is the operator-level hard kill switch (independent
	// of the API key). Default true — only false if operator explicitly
	// opts out for compliance reasons.
	DeepSeekEnabled bool `env:"IRONFLYER_DEEPSEEK_ENABLED" envDefault:"true"`

	// HuggingFace inference. Powers the memory layer's semantic search
	// (core/orchestrator/internal/embeddings) and any other future
	// HF-backed provider. Leaving HF_API_KEY empty disables the
	// embedder; memory.Query falls back to substring search.
	HFAPIKey string `env:"HF_API_KEY"`
	// HFEmbedModel is the HuggingFace model id used by the memory layer's
	// semantic re-ranker. New code reads HF_EMBEDDINGS_MODEL; the legacy
	// HF_EMBED_MODEL env var is honoured as a fallback so existing
	// deployments don't break on upgrade. Default tracks
	// embeddings.DefaultModel ("BAAI/bge-m3" — multilingual, dense+
	// sparse, current best-in-class open weights for code+prose).
	HFEmbedModel      string `env:"HF_EMBED_MODEL"       envDefault:"BAAI/bge-m3"`
	HFEmbeddingsModel string `env:"HF_EMBEDDINGS_MODEL"`

	// EmbeddingsBackend selects the driver used by the memory layer's
	// semantic re-ranker:
	//   "hf"   — HuggingFace inference API (default; needs HF_API_KEY).
	//   "onnx" — local ONNX Runtime (needs IRONFLYER_ONNX_MODEL +
	//             IRONFLYER_ONNX_VOCAB and a binary built with
	//             `-tags onnx`; see docs/EMBEDDINGS.md).
	//   "auto" — try ONNX when the model artefacts are present, fall
	//             back to HF when they aren't.
	// An unrecognised value behaves as "hf".
	EmbeddingsBackend string `env:"IRONFLYER_EMBEDDINGS_BACKEND" envDefault:"hf"`

	// ONNXModelPath is the absolute path to the ONNX-exported bge-small
	// model file. Empty disables the local backend even when
	// EmbeddingsBackend=onnx is requested (the strategy switch surfaces
	// the misconfiguration in the logs and falls back to HF under
	// "auto"). The file is NOT committed into the repo — operators ship
	// it alongside the container image as a release artefact.
	ONNXModelPath string `env:"IRONFLYER_ONNX_MODEL"`

	// ONNXVocabPath is the absolute path to the WordPiece vocabulary
	// file (vocab.txt) that pairs with ONNXModelPath. Required when
	// running the ONNX backend; ignored otherwise.
	ONNXVocabPath string `env:"IRONFLYER_ONNX_VOCAB"`

	// ONNXDimension overrides the expected output dim. 0 → 384
	// (bge-small-en-v1.5 hidden size). Set this only when running a
	// non-default model.
	ONNXDimension int `env:"IRONFLYER_ONNX_DIM" envDefault:"0"`

	// OpenAIImageAPIKey is the API key used by the built-in
	// generate_image tool (core/orchestrator/internal/imagegen). Leave
	// empty to fall back to OpenAIAPIKey; if both are empty the tool
	// is registered but every call returns "image generation disabled".
	OpenAIImageAPIKey string `env:"OPENAI_IMAGE_API_KEY"`

	// FigmaToken authorises the built-in figma_import tool + the
	// /api/projects/:id/figma-import HTTP endpoint. Leave empty to
	// disable both — the tool registers either way but every call
	// fails with a readable "figma token not configured" rather than
	// crashing the Coder.
	FigmaToken string `env:"FIGMA_TOKEN"`

	// GitHub OAuth + integration. Leaving CLIENT_ID empty disables the
	// /auth/github/* endpoints (they return 503).
	GitHubClientID     string `env:"GITHUB_CLIENT_ID"`
	GitHubClientSecret string `env:"GITHUB_CLIENT_SECRET"`
	GitHubRedirectURL  string `env:"GITHUB_REDIRECT_URL" envDefault:"http://localhost:8080/auth/github/callback"`
	// Where to send the browser after we finish the OAuth exchange.
	GitHubPostLoginURL string `env:"GITHUB_POST_LOGIN_URL" envDefault:"http://localhost:3000/app"`

	// GitHub App (separate from OAuth). The App flavor subscribes to
	// webhooks (pull_request, push) and lets the orchestrator post PR
	// reviews + commit statuses on inbound PRs. Leaving GITHUB_APP_ID
	// empty disables the webhook + /github/app/* endpoints (they
	// return 503). The private key may be passed as a literal PEM
	// (newlines OK) or as `@/path/to/key.pem`.
	GitHubAppID            string `env:"GITHUB_APP_ID"`
	GitHubAppPrivateKey    string `env:"GITHUB_APP_PRIVATE_KEY"`
	GitHubAppWebhookSecret string `env:"GITHUB_APP_WEBHOOK_SECRET"`
	// Slug is the public app slug used to render the install URL on the
	// settings page (https://github.com/apps/<slug>/installations/new).
	GitHubAppSlug string `env:"GITHUB_APP_SLUG"`

	// Runtime service the orchestrator proxies clone calls to.
	RuntimeURL string `env:"IRONFLYER_RUNTIME_URL" envDefault:"http://localhost:8090"`

	// PublicBaseURL is the absolute URL of the web app, used when the
	// orchestrator needs to render an outbound URL (today: project share
	// links). Empty falls back to the request Host header.
	PublicBaseURL string `env:"IRONFLYER_PUBLIC_BASE_URL" envDefault:"http://localhost:3000"`

	// Stripe — leave SecretKey empty to disable /budget/checkout + webhook
	// (the routes return 503). The Pro/Team/Enterprise price IDs map to our
	// PlanTier values; users buy a tier, not a specific price.
	StripeSecretKey       string `env:"STRIPE_SECRET_KEY"`
	StripeWebhookSecret   string `env:"STRIPE_WEBHOOK_SECRET"`
	StripePricePro        string `env:"STRIPE_PRICE_PRO"`
	StripePriceTeam       string `env:"STRIPE_PRICE_TEAM"`
	StripePriceEnterprise string `env:"STRIPE_PRICE_ENTERPRISE"`
	// Metered (usage_type=metered) prices charge per-call overage above
	// each plan's CostCapUSD. Free tier never overages (HardStop). Empty
	// values disable metering for that tier — fine in dev / self-hosted.
	StripeMeteredPricePro  string `env:"STRIPE_METERED_PRICE_PRO"`
	StripeMeteredPriceTeam string `env:"STRIPE_METERED_PRICE_TEAM"`
	// MeteredDisabled is the global kill-switch. Set true to keep the
	// reporter wired but never flush — useful for staging, dry-runs, and
	// incident response when Stripe is having an outage.
	MeteredDisabled bool `env:"IRONFLYER_METERED_DISABLED" envDefault:"false"`
	// MeteredFlushInterval is the buffer flush cadence. Defaults to 60s,
	// matching Stripe's recommended cadence for usage records.
	MeteredFlushInterval time.Duration `env:"IRONFLYER_METERED_FLUSH_INTERVAL" envDefault:"60s"`
	StripeSuccessURL     string        `env:"STRIPE_SUCCESS_URL" envDefault:"http://localhost:3000/app/settings?stripe=success"`
	StripeCancelURL      string        `env:"STRIPE_CANCEL_URL" envDefault:"http://localhost:3000/pricing?stripe=cancel"`

	// Paddle — alternative payment provider. Coexists with Stripe; the
	// user picks at checkout. Without PADDLE_API_KEY the provider is
	// kept in the registry but reports Enabled()=false and the webhook
	// route returns 503. PADDLE_ENV switches the API base between
	// live and Paddle's sandbox.
	PaddleAPIKey          string `env:"PADDLE_API_KEY"`
	PaddleWebhookSecret   string `env:"PADDLE_WEBHOOK_SECRET"`
	PaddleEnv             string `env:"PADDLE_ENV" envDefault:"live"`
	PaddlePricePro        string `env:"PADDLE_PRICE_PRO"`
	PaddlePriceTeam       string `env:"PADDLE_PRICE_TEAM"`
	PaddlePriceEnterprise string `env:"PADDLE_PRICE_ENTERPRISE"`

	// Superuser bootstrap — when both vars are set, main.go idempotently
	// ensures a platform_operator account exists with the given email +
	// password and a verified email. Privileged backdoor; rotate often.
	SuperuserEmail    string `env:"IRONFLYER_SUPERUSER_EMAIL"`
	SuperuserPassword string `env:"IRONFLYER_SUPERUSER_PASSWORD"`

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

	// Memory + audit backends. "memory" keeps the in-process ring buffers
	// (fine for dev / single-node demos); "surreal" persists records +
	// the hash chain to SurrealDB so they survive restarts. "pgvector"
	// persists memory records to Postgres with pgvector embeddings — use
	// it when Aurora Postgres is already provisioned and operators don't
	// want to stand up SurrealDB just for the memory layer. Selecting
	// "surreal" only takes effect when DBDriver also activates SurrealDB
	// (surreal / hybrid); "pgvector" requires PostgresURL to be set
	// (main.go logs a fatal otherwise). Without those prerequisites the
	// orchestrator falls back to the in-process implementation.
	MemoryBackend string `env:"IRONFLYER_MEMORY_BACKEND" envDefault:"memory" validate:"oneof=memory surreal pgvector"`
	AuditBackend  string `env:"IRONFLYER_AUDIT_BACKEND"  envDefault:"memory" validate:"oneof=memory surreal"`

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

	// -------------------- Redis (optional, horizontal scaling) -----------
	// IRONFLYER_REDIS_ENABLED=true switches the orchestrator into multi-pod
	// mode: finisher runs are coordinated through a distributed lock so two
	// pods can't race on the same project, and rate limits become a single
	// source of truth across pods. Leaving it disabled keeps the in-process
	// implementations as the source of truth — fine for single-pod dev.
	RedisEnabled  bool   `env:"IRONFLYER_REDIS_ENABLED" envDefault:"false"`
	RedisAddr     string `env:"REDIS_ADDR"              envDefault:"localhost:6379"`
	RedisPassword string `env:"REDIS_PASSWORD"`
	RedisDB       int    `env:"REDIS_DB"                envDefault:"0"`

	// -------------------- Redpanda event backbone (optional) -------------
	// Durable events always land in Postgres event_outbox when Postgres is
	// enabled. Setting REDPANDA_BROKERS starts the publisher that drains
	// that outbox to Redpanda/Kafka for ClickHouse, SurrealDB projections,
	// notifications, and lag-based worker scaling.
	RedpandaBrokers      string        `env:"REDPANDA_BROKERS"`
	EventPumpBatchSize   int           `env:"IRONFLYER_EVENT_PUMP_BATCH" envDefault:"50"`
	EventPumpInterval    time.Duration `env:"IRONFLYER_EVENT_PUMP_INTERVAL" envDefault:"1s"`
	EventPumpLease       time.Duration `env:"IRONFLYER_EVENT_PUMP_LEASE" envDefault:"30s"`
	EventPumpMaxAttempts int           `env:"IRONFLYER_EVENT_PUMP_MAX_ATTEMPTS" envDefault:"10"`

	// MetricsToken protects the /metrics scrape endpoint. When set, a
	// scraper MUST present `Authorization: Bearer <token>`; the compare
	// is constant-time. In prod this MUST be set or the orchestrator
	// refuses to boot; in dev/staging an unset token leaves /metrics
	// open with a startup warning.
	MetricsToken string `env:"IRONFLYER_METRICS_TOKEN"`

	// CSP overrides the orchestrator's default Content-Security-Policy
	// header. Empty selects a conservative default (self-only sources)
	// in prod and a more permissive policy in dev/staging that admits
	// the Apollo Sandbox CDN.
	CSP string `env:"IRONFLYER_CSP"`

	// AuditRedact toggles PII redaction on audit entries (emails, IPs,
	// provider API keys). Default "on"; "off" disables redaction and
	// logs a warning at startup so operators see the choice in the log.
	AuditRedact string `env:"IRONFLYER_AUDIT_REDACT" envDefault:"on" validate:"oneof=on off"`

	// PromptGuard — defense-in-depth prompt-injection sanitizer. Runs on
	// user-sourced text before it leaves Ironflyer to a provider. Disable
	// in prod is loud (Warn at startup) but allowed for debugging.
	PromptGuardEnabled       bool `env:"IRONFLYER_PROMPTGUARD_ENABLED"         envDefault:"true"`
	PromptGuardBlock         bool `env:"IRONFLYER_PROMPTGUARD_BLOCK"           envDefault:"true"`
	PromptGuardMaxUserChars  int  `env:"IRONFLYER_PROMPTGUARD_MAX_USER_CHARS"  envDefault:"100000"`
	PromptGuardMaxTotalChars int  `env:"IRONFLYER_PROMPTGUARD_MAX_TOTAL_CHARS" envDefault:"400000"`
}

func (c Config) IsProd() bool { return c.Env == "prod" }

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
	if c.IsProd() {
		if c.JWTSecret == "" || c.JWTSecret == "dev-secret-change-me" || len(c.JWTSecret) < 32 {
			return c, fmt.Errorf("IRONFLYER_JWT_SECRET must be set to a non-default value of at least 32 bytes when IRONFLYER_ENV=prod")
		}
		if c.CORSOrigins == nil || len(c.CORSOrigins) == 0 {
			return c, fmt.Errorf("IRONFLYER_CORS_ORIGINS must list explicit origins when IRONFLYER_ENV=prod (open-mode CORS is refused in production)")
		}
		if c.MetricsToken == "" {
			return c, fmt.Errorf("IRONFLYER_METRICS_TOKEN must be set when IRONFLYER_ENV=prod (open /metrics is refused in production)")
		}
	}
	return c, nil
}
