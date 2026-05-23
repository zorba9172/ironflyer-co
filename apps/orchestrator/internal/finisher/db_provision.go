// Package finisher — database provisioning. The finisher pipeline needs a
// real database for any project whose data model is non-empty: without a
// connection string the generated code is unrunnable and the Test gate
// will always fail. This file declares the abstraction the operator wires
// (Supabase admin API, Neon, in-cluster Postgres, …) and a safe default
// that does nothing so dev environments still boot.
//
// The pipeline calls ensureDatabase() right after UXer / before Coder: by
// then the data model is finalised, so the provisioner can target the
// right schema. The resulting DATABASE_URL is stored in Project.Secrets
// (never serialised) and injected into the runtime workspace as an env
// var the generated app reads at startup.

package finisher

import (
	"context"
	"strings"
	"time"

	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/runtime"
)

// DBProvisioner is the contract every provisioner backend implements. It
// returns the connection string for a fresh database scoped to this
// project. Idempotent: calling Provision() twice with the same project +
// data model should yield the same DSN so retries don't leak resources.
//
// Returning an empty DSN with a nil error is the polite way to say "I
// decided this project doesn't need a database" (e.g. static landing
// page); the pipeline treats it as a no-op.
type DBProvisioner interface {
	Provision(ctx context.Context, projectID string, model []domain.EntityDef) (DBProvision, error)
}

// DBProvision is the result of a provisioning call.
type DBProvision struct {
	// DSN is the canonical connection string, e.g.
	// "postgres://user:pass@host:5432/db?sslmode=require".
	DSN string
	// Provider is a human label for logs / events, e.g. "supabase", "neon",
	// "local-postgres". Optional.
	Provider string
	// PublicURL, when set, is a browser-safe URL to the provider's project
	// dashboard so the user can inspect tables and rotate keys. Optional.
	PublicURL string
}

// NoopDBProvisioner is the default: it never provisions anything and
// returns an empty DSN. Used when the operator hasn't wired a real backend
// — the pipeline still runs, but the Test gate will surface the missing
// database as a normal gate failure.
type NoopDBProvisioner struct{}

func (NoopDBProvisioner) Provision(_ context.Context, _ string, _ []domain.EntityDef) (DBProvision, error) {
	return DBProvision{}, nil
}

// dbSecretKey is the canonical secret name we store the DSN under. Keep
// in sync with the runtime side, which exports it to the user's process
// environment as DATABASE_URL.
const dbSecretKey = "DATABASE_URL"

// ensureDatabase wires the provisioned DSN onto the project's secret bag.
// It runs at most once per project: if a DSN is already set we skip the
// provisioner entirely, so calling Run() on a project repeatedly does not
// thrash the operator's external billing. An empty data model is treated
// as "no DB needed" and exits cleanly.
func (e *Engine) ensureDatabase(ctx context.Context, projectID string) {
	if e.dbProvisioner == nil {
		return
	}
	proj, err := e.projects.Get(projectID)
	if err != nil {
		return
	}
	if existing := proj.Secrets[dbSecretKey]; strings.TrimSpace(existing) != "" {
		return // already provisioned — idempotent skip
	}
	if len(proj.Spec.DataModel) == 0 {
		return
	}
	res, err := e.dbProvisioner.Provision(ctx, projectID, proj.Spec.DataModel)
	if err != nil || strings.TrimSpace(res.DSN) == "" {
		return
	}
	_, _ = e.projects.Update(projectID, func(p *domain.Project) {
		if p.Secrets == nil {
			p.Secrets = make(map[string]string, 1)
		}
		p.Secrets[dbSecretKey] = res.DSN
	})
	e.emit(projectID, domain.Event{
		ID: newEventID(), Step: StepRun, Status: StatusDone,
		Message:   "db_provisioned provider=" + res.Provider,
		CreatedAt: time.Now().UTC(),
	})
	// Surface the new secret to the user's workspace so the generated app
	// can read DATABASE_URL at boot. injectSecrets is idempotent — it
	// rewrites .env.local from the full Secrets map every call.
	e.injectSecrets(ctx, projectID)
}

// injectSecrets writes the project's secret bag into the workspace as
// `.env.local`, one KEY=value line per entry. Frameworks (Next.js, Vite,
// Astro, Remix) discover env vars from this file at startup, so the
// generated app gets DATABASE_URL + Supabase keys without any extra
// wiring on the user side.
//
// Safety:
//   - We only emit the file when a runtime is bound; without one there
//     is no workspace to inject into.
//   - Values containing newlines, double-quotes, or backslashes are
//     written using a double-quoted form with escapes so the dotenv
//     parser sees a single value.
//   - The file is owned by Ironflyer: any pre-existing content gets
//     replaced. The Coder is told (via the auth/stripe contracts) not
//     to edit it.
func (e *Engine) injectSecrets(ctx context.Context, projectID string) {
	if e == nil || e.runtime == nil || !e.runtime.Enabled() {
		return
	}
	proj, err := e.projects.Get(projectID)
	if err != nil || len(proj.Secrets) == 0 {
		return
	}
	bearer := bearerFromCtx(ctx)
	if bearer == "" {
		return
	}
	ws, err := e.runtime.FindWorkspaceForProject(ctx, bearer, projectID)
	if err != nil {
		return
	}
	var b strings.Builder
	b.WriteString("# Generated by Ironflyer — do not edit. Managed by the finisher.\n")
	for k, v := range proj.Secrets {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(quoteEnvValue(v))
		b.WriteByte('\n')
	}
	if err := e.runtime.WriteFile(ctx, bearer, ws.ID, ".env.local", []byte(b.String())); err != nil {
		return
	}
	// Nudge the dev server to reload. Modern Node bundlers (Next.js,
	// Vite, Astro) restart when their config file changes mtime; we
	// `touch` the first matching one. `|| true` keeps the chain safe
	// when the file isn't present (different framework / language).
	// HUP also covers long-lived Node processes that wired SIGHUP as a
	// config-reload signal. Everything else is harmless.
	touch := `for f in next.config.js next.config.mjs next.config.ts vite.config.ts vite.config.js astro.config.mjs remix.config.js; do
  if [ -f "$f" ]; then touch "$f"; break; fi
done
pkill -HUP -f 'node|next|vite|astro' 2>/dev/null || true`
	_, _ = e.runtime.Exec(ctx, bearer, ws.ID, runtime.ExecOpts{
		Shell: touch, TimeoutSeconds: 5,
	})
	e.emit(projectID, domain.Event{
		ID: newEventID(), Step: StepRun, Status: StatusDone,
		Message:   "secrets_injected count=" + itoaPositive(len(proj.Secrets)),
		CreatedAt: time.Now().UTC(),
	})
}

// quoteEnvValue produces a dotenv-safe encoding for a value. Plain
// alnum / dash / dot / colon / slash / @ pass through; anything else
// gets double-quoted with backslash escapes for ", \, and newlines.
func quoteEnvValue(v string) string {
	needsQuote := false
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '-' || c == '_' || c == '.' || c == ':' || c == '/' || c == '@' || c == '=' || c == '+' || c == '?' || c == '&':
		default:
			needsQuote = true
		}
		if needsQuote {
			break
		}
	}
	if !needsQuote {
		return v
	}
	out := make([]byte, 0, len(v)+8)
	out = append(out, '"')
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch c {
		case '"', '\\':
			out = append(out, '\\', c)
		case '\n':
			out = append(out, '\\', 'n')
		case '\r':
			out = append(out, '\\', 'r')
		default:
			out = append(out, c)
		}
	}
	out = append(out, '"')
	return string(out)
}
