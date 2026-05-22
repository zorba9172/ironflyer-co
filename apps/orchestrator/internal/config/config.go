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
