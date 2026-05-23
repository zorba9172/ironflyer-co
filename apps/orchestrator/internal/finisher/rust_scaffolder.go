// RustScaffolder — Axum + Tokio + SQLx skeleton for serious backend
// services that demand low latency and predictable resource use. Same
// shape as the Next.js domain packs: a fixed set of files + a contract
// markdown the Coder reads as context. The pack triggers on explicit
// stack hints ("rust", "axum", "actix", "rocket") and on performance
// language in the description / stories ("high performance", "low
// latency"). False positives are recoverable; false negatives are not.

package finisher

import (
	"context"
	"strings"

	"ironflyer/apps/orchestrator/internal/domain"
)

type RustScaffolder struct{}

func (RustScaffolder) Name() string { return "rust-axum" }

func (RustScaffolder) Applies(p *domain.Project) bool {
	if p == nil {
		return false
	}
	stack := strings.ToLower(p.Spec.Stack.Frontend + " " + p.Spec.Stack.Backend)
	if strings.Contains(stack, "rust") || strings.Contains(stack, "axum") ||
		strings.Contains(stack, "actix") || strings.Contains(stack, "rocket") {
		return true
	}
	desc := strings.ToLower(p.Description + " " + p.Spec.Idea)
	if strings.Contains(desc, "high performance") || strings.Contains(desc, "low latency") ||
		strings.Contains(desc, "high-performance") || strings.Contains(desc, "low-latency") {
		return true
	}
	for _, s := range p.Spec.UserStories {
		body := strings.ToLower(s.IWant + " " + s.SoThat + " " + strings.Join(s.Acceptance, " "))
		if strings.Contains(body, "axum") || strings.Contains(body, "actix") ||
			strings.Contains(body, "rocket") {
			return true
		}
	}
	return false
}

// rustCrateName turns a free-form project name into a snake_case crate
// identifier Cargo accepts. Cargo permits ASCII alphanumeric +
// underscore + hyphen; we normalize to underscores to keep the
// `use crate_name::` style consistent.
func rustCrateName(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return "ironflyer_service"
	}
	var b strings.Builder
	prevUnderscore := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevUnderscore = false
		default:
			if !prevUnderscore && b.Len() > 0 {
				b.WriteByte('_')
				prevUnderscore = true
			}
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "ironflyer_service"
	}
	// Cargo rejects names starting with a digit.
	if out[0] >= '0' && out[0] <= '9' {
		out = "svc_" + out
	}
	return out
}

func (RustScaffolder) Scaffold(_ context.Context, p *domain.Project) (DomainScaffold, error) {
	crate := "ironflyer_service"
	if p != nil {
		crate = rustCrateName(p.Name)
	}

	cargoToml := "" +
		"[package]\n" +
		"name = \"" + crate + "\"\n" +
		"version = \"0.1.0\"\n" +
		"edition = \"2021\"\n" +
		"\n" +
		"[dependencies]\n" +
		"axum = \"0.7.7\"\n" +
		"tokio = { version = \"1.40\", features = [\"full\"] }\n" +
		"serde = { version = \"1.0\", features = [\"derive\"] }\n" +
		"serde_json = \"1.0\"\n" +
		"tower = \"0.5\"\n" +
		"tower-http = { version = \"0.6\", features = [\"trace\", \"cors\"] }\n" +
		"tracing = \"0.1\"\n" +
		"tracing-subscriber = { version = \"0.3\", features = [\"env-filter\"] }\n" +
		"anyhow = \"1.0\"\n" +
		"thiserror = \"1.0\"\n" +
		"sqlx = { version = \"0.8\", features = [\"runtime-tokio\", \"postgres\", \"macros\", \"chrono\", \"uuid\"] }\n" +
		"uuid = { version = \"1.10\", features = [\"v4\", \"serde\"] }\n" +
		"dotenvy = \"0.15\"\n"

	mainRs := "" +
		"// Service entrypoint. Wires the tracing subscriber, builds the\n" +
		"// Postgres pool from DATABASE_URL, mounts the Axum router, and\n" +
		"// listens on PORT (default 8080). Keep this file thin: routes\n" +
		"// live in their own modules once the surface grows past a few\n" +
		"// handlers.\n" +
		"use axum::{\n" +
		"    extract::{Path, State},\n" +
		"    response::Json,\n" +
		"    routing::get,\n" +
		"    Router,\n" +
		"};\n" +
		"use serde_json::{json, Value};\n" +
		"use std::{env, net::SocketAddr, sync::Arc};\n" +
		"use tower_http::trace::TraceLayer;\n" +
		"use tracing_subscriber::EnvFilter;\n" +
		"\n" +
		"mod db;\n" +
		"mod error;\n" +
		"\n" +
		"use crate::error::AppError;\n" +
		"\n" +
		"#[derive(Clone)]\n" +
		"struct AppState {\n" +
		"    pool: Arc<sqlx::PgPool>,\n" +
		"}\n" +
		"\n" +
		"#[tokio::main]\n" +
		"async fn main() -> anyhow::Result<()> {\n" +
		"    let _ = dotenvy::dotenv();\n" +
		"    tracing_subscriber::fmt()\n" +
		"        .with_env_filter(EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new(\"info\")))\n" +
		"        .init();\n" +
		"\n" +
		"    let database_url = env::var(\"DATABASE_URL\")\n" +
		"        .map_err(|_| anyhow::anyhow!(\"DATABASE_URL is required; the DB scaffold populates .env.local\"))?;\n" +
		"    let pool = db::build_pool(&database_url).await?;\n" +
		"    let state = AppState { pool: Arc::new(pool) };\n" +
		"\n" +
		"    let app = Router::new()\n" +
		"        .route(\"/health\", get(health))\n" +
		"        .route(\"/version\", get(version))\n" +
		"        .route(\"/users/:id\", get(get_user))\n" +
		"        .layer(TraceLayer::new_for_http())\n" +
		"        .with_state(state);\n" +
		"\n" +
		"    let port: u16 = env::var(\"PORT\").ok().and_then(|s| s.parse().ok()).unwrap_or(8080);\n" +
		"    let addr = SocketAddr::from(([0, 0, 0, 0], port));\n" +
		"    tracing::info!(%addr, \"listening\");\n" +
		"    let listener = tokio::net::TcpListener::bind(addr).await?;\n" +
		"    axum::serve(listener, app).await?;\n" +
		"    Ok(())\n" +
		"}\n" +
		"\n" +
		"async fn health() -> Json<Value> {\n" +
		"    Json(json!({ \"status\": \"ok\" }))\n" +
		"}\n" +
		"\n" +
		"async fn version() -> Json<Value> {\n" +
		"    Json(json!({ \"version\": env!(\"CARGO_PKG_VERSION\") }))\n" +
		"}\n" +
		"\n" +
		"async fn get_user(\n" +
		"    State(state): State<AppState>,\n" +
		"    Path(id): Path<String>,\n" +
		") -> Result<Json<Value>, AppError> {\n" +
		"    // Example query — swap for a typed sqlx::query_as! once the\n" +
		"    // users table schema is finalized.\n" +
		"    let row: Option<(String, String)> = sqlx::query_as(\n" +
		"        \"SELECT id::text, email FROM users WHERE id::text = $1 LIMIT 1\",\n" +
		"    )\n" +
		"    .bind(&id)\n" +
		"    .fetch_optional(&*state.pool)\n" +
		"    .await\n" +
		"    .map_err(AppError::from)?;\n" +
		"\n" +
		"    match row {\n" +
		"        Some((id, email)) => Ok(Json(json!({ \"id\": id, \"email\": email }))),\n" +
		"        None => Err(AppError::NotFound),\n" +
		"    }\n" +
		"}\n"

	errorRs := "" +
		"// AppError centralizes how the service maps internal failures to\n" +
		"// HTTP responses. Add a variant per category — never let an\n" +
		"// anyhow::Error leak directly to the client.\n" +
		"use axum::{\n" +
		"    http::StatusCode,\n" +
		"    response::{IntoResponse, Response},\n" +
		"    Json,\n" +
		"};\n" +
		"use serde_json::json;\n" +
		"use thiserror::Error;\n" +
		"\n" +
		"#[derive(Debug, Error)]\n" +
		"pub enum AppError {\n" +
		"    #[error(\"not found\")]\n" +
		"    NotFound,\n" +
		"    #[error(\"bad request: {0}\")]\n" +
		"    BadRequest(String),\n" +
		"    #[error(\"unauthorized\")]\n" +
		"    Unauthorized,\n" +
		"    #[error(\"database error\")]\n" +
		"    Database(#[from] sqlx::Error),\n" +
		"    #[error(\"internal error\")]\n" +
		"    Internal(#[from] anyhow::Error),\n" +
		"}\n" +
		"\n" +
		"impl IntoResponse for AppError {\n" +
		"    fn into_response(self) -> Response {\n" +
		"        let (status, message) = match &self {\n" +
		"            AppError::NotFound => (StatusCode::NOT_FOUND, self.to_string()),\n" +
		"            AppError::BadRequest(_) => (StatusCode::BAD_REQUEST, self.to_string()),\n" +
		"            AppError::Unauthorized => (StatusCode::UNAUTHORIZED, self.to_string()),\n" +
		"            AppError::Database(e) => {\n" +
		"                tracing::error!(error = ?e, \"database error\");\n" +
		"                (StatusCode::INTERNAL_SERVER_ERROR, \"internal error\".to_string())\n" +
		"            }\n" +
		"            AppError::Internal(e) => {\n" +
		"                tracing::error!(error = ?e, \"internal error\");\n" +
		"                (StatusCode::INTERNAL_SERVER_ERROR, \"internal error\".to_string())\n" +
		"            }\n" +
		"        };\n" +
		"        (status, Json(json!({ \"error\": message }))).into_response()\n" +
		"    }\n" +
		"}\n"

	dbRs := "" +
		"// Postgres pool builder. Holds the pool config in one place so\n" +
		"// production tuning (max connections, statement timeout) doesn't\n" +
		"// require touching every caller.\n" +
		"use sqlx::postgres::{PgPool, PgPoolOptions};\n" +
		"use std::time::Duration;\n" +
		"\n" +
		"pub async fn build_pool(database_url: &str) -> anyhow::Result<PgPool> {\n" +
		"    let pool = PgPoolOptions::new()\n" +
		"        .max_connections(10)\n" +
		"        .acquire_timeout(Duration::from_secs(5))\n" +
		"        .connect(database_url)\n" +
		"        .await?;\n" +
		"    Ok(pool)\n" +
		"}\n"

	dockerfile := "" +
		"# Multi-stage build: cache cargo deps in the builder, ship a\n" +
		"# slim debian runtime with only the compiled binary.\n" +
		"FROM rust:1.81-slim AS builder\n" +
		"WORKDIR /app\n" +
		"RUN apt-get update && apt-get install -y --no-install-recommends pkg-config libssl-dev ca-certificates && rm -rf /var/lib/apt/lists/*\n" +
		"COPY Cargo.toml ./\n" +
		"COPY src ./src\n" +
		"RUN cargo build --release\n" +
		"\n" +
		"FROM debian:bookworm-slim AS runtime\n" +
		"WORKDIR /app\n" +
		"RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates libssl3 && rm -rf /var/lib/apt/lists/*\n" +
		"COPY --from=builder /app/target/release/" + crate + " /usr/local/bin/service\n" +
		"ENV RUST_LOG=info\n" +
		"EXPOSE 8080\n" +
		"CMD [\"/usr/local/bin/service\"]\n"

	dockerignore := "" +
		"target\n" +
		".git\n" +
		".env\n" +
		".env.local\n" +
		"*.md\n" +
		"Dockerfile\n" +
		".dockerignore\n"

	gitignore := "" +
		"/target\n" +
		".env\n" +
		".env.local\n" +
		"*.log\n" +
		"Cargo.lock\n"

	files := map[string]string{
		"Cargo.toml":    cargoToml,
		"src/main.rs":   mainRs,
		"src/error.rs":  errorRs,
		"src/db.rs":     dbRs,
		"Dockerfile":    dockerfile,
		".dockerignore": dockerignore,
		".gitignore":    gitignore,
	}

	contract := "Rust service scaffold: Axum 0.7 + Tokio + SQLx (Postgres).\n" +
		"\n" +
		"Already provisioned:\n" +
		"- Cargo.toml      package=" + crate + ", pinned deps (axum, tokio, sqlx, serde, tower-http, tracing, anyhow, thiserror)\n" +
		"- src/main.rs     tokio main, routes /health /version /users/:id\n" +
		"- src/error.rs    AppError enum + IntoResponse mapping to HTTP status\n" +
		"- src/db.rs       PgPool builder (max 10, 5s acquire timeout)\n" +
		"- Dockerfile      rust:1.81-slim builder -> debian:bookworm-slim runtime\n" +
		"- .dockerignore   keeps target/ + .env out of build context\n" +
		"- .gitignore      ignores target/, .env*, Cargo.lock\n" +
		"\n" +
		"Run locally with `cargo run`. DATABASE_URL is required; the\n" +
		"Supabase or SharedPostgres provisioner writes it into .env.local\n" +
		"on the first finisher pass and dotenvy loads it on boot.\n" +
		"\n" +
		"Add routes by extending the Router in src/main.rs; once the\n" +
		"surface grows past ~5 handlers, split them into src/routes/<name>.rs\n" +
		"and re-export through a mod.rs. Map every new failure category to\n" +
		"an AppError variant — do not bubble anyhow::Error to the client.\n"

	return DomainScaffold{Files: files, Contract: contract}, nil
}
