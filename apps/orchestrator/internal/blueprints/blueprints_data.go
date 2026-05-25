package blueprints

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/shopspring/decimal"
)

// templatesFS embeds the verbatim template trees that ship with the
// built-in blueprints. Adding a new blueprint = create a new
// subdirectory under templates/ and a matching entry in
// builtInBlueprints(). All file contents in this tree are emitted
// byte-for-byte into the workspace at execution time.
//
//go:embed all:templates
var templatesFS embed.FS

// builtInBlueprints assembles the v1 catalogue. The CostPriorUSD and
// ExpectedTimeToPreviewSec values are deliberate priors — they will
// be replaced by realised StatsService averages once enough runs
// have landed, but ProfitGuard needs *some* baseline to rank against
// from day one.
func builtInBlueprints() []Blueprint {
	return []Blueprint{
		{
			ID:                       "nextjs-mvp",
			Name:                     "Next.js 15 MVP",
			Description:              "Next.js 15 App Router + MUI 6 + Prisma + Postgres. The default web-app blueprint: server components, MUI theming via AppRouterCacheProvider, and a Prisma schema with User + Item models ready to migrate. Use this when the spec asks for a full web app with persisted data.",
			Category:                 "webapp",
			CostPriorUSD:             decimal.RequireFromString("0.85"),
			ExpectedTimeToPreviewSec: 75,
			SupportedGates: []string{
				"scaffold", "build", "typecheck",
			},
			Files: mustLoadFiles("templates/nextjs-mvp"),
		},
		{
			ID:                       "go-http-api",
			Name:                     "Go HTTP API",
			Description:              "Go + chi + pgx HTTP service with a /healthz probe, an /items CRUD surface, a Postgres-backed store, and a distroless Dockerfile. The default backend-only blueprint — pair with static-landing or nextjs-mvp for a full stack.",
			Category:                 "api",
			CostPriorUSD:             decimal.RequireFromString("0.55"),
			ExpectedTimeToPreviewSec: 45,
			SupportedGates: []string{
				"scaffold", "build", "healthz",
			},
			Files: mustLoadFiles("templates/go-http-api"),
		},
		{
			ID:                       "static-landing",
			Name:                     "Static Landing Page",
			Description:              "Single-page marketing site — index.html + styles.css + vercel.json, no build step. The cheapest blueprint in the catalogue; ProfitGuard reaches for this when the spec is marketing-flavoured or when a fast preview is more valuable than a full stack.",
			Category:                 "static",
			CostPriorUSD:             decimal.RequireFromString("0.10"),
			ExpectedTimeToPreviewSec: 15,
			SupportedGates: []string{
				"scaffold", "build", "deploy_preview",
			},
			Files: mustLoadFiles("templates/static-landing"),
		},
		{
			ID:                       "nextjs-saas",
			Name:                     "Next.js SaaS Starter",
			Description:              "Multi-tenant SaaS scaffold on Next.js 15 + MUI 6 + Prisma + NextAuth v5 (credentials) + Stripe Checkout + Zod. Ships a landing page, a session-gated dashboard, a Stripe Checkout creation endpoint, a verified webhook receiver, and a Prisma schema with Tenant/User/Subscription plus the NextAuth tables. Reach for this when the spec mentions auth, billing, or recurring subscriptions.",
			Category:                 "webapp",
			CostPriorUSD:             decimal.RequireFromString("1.20"),
			ExpectedTimeToPreviewSec: 90,
			SupportedGates: []string{
				"scaffold", "build", "typecheck", "deploy_preview",
			},
			Files: mustLoadFiles("templates/nextjs-saas"),
		},
		{
			ID:                       "python-flask-api",
			Name:                     "Python Flask API",
			Description:              "Flask 3 + SQLAlchemy 2 + Postgres backend with a /healthz probe, an /items CRUD surface, gunicorn-ready Dockerfile, and ruff + mypy config. The default Python-side API blueprint — pair with vue-spa or static-landing for a full stack.",
			Category:                 "api",
			CostPriorUSD:             decimal.RequireFromString("0.45"),
			ExpectedTimeToPreviewSec: 35,
			SupportedGates: []string{
				"scaffold", "build", "healthz",
			},
			Files: mustLoadFiles("templates/python-flask-api"),
		},
		{
			ID:                       "vue-spa",
			Name:                     "Vue 3 SPA",
			Description:              "Vue 3 + Vite + TypeScript + Vue Router + Pinia single-page app. Two routes (Home/About), a Pinia counter store, and vue-tsc-backed typecheck. Reach for this when the spec calls for an interactive UI that doesn't need a Next.js server runtime.",
			Category:                 "webapp",
			CostPriorUSD:             decimal.RequireFromString("0.70"),
			ExpectedTimeToPreviewSec: 60,
			SupportedGates: []string{
				"scaffold", "build", "typecheck",
			},
			Files: mustLoadFiles("templates/vue-spa"),
		},
		{
			ID:                       "expo-react-native",
			Name:                     "Expo React Native App",
			Description:              "Expo SDK 51 + expo-router + TypeScript cross-platform (iOS / Android / web) mobile starter. Ships a Stack root layout, a typed Home screen, and a (tabs) group with a stateful Profile tab. Ships no binary assets — drop a 1024x1024 PNG into ./assets/icon.png before submitting to a store.",
			Category:                 "mobile",
			CostPriorUSD:             decimal.RequireFromString("1.50"),
			ExpectedTimeToPreviewSec: 120,
			SupportedGates: []string{
				"scaffold", "build", "typecheck",
			},
			Files: mustLoadFiles("templates/expo-react-native"),
		},
		{
			ID:                       "discord-bot-py",
			Name:                     "Discord Bot (Python)",
			Description:              "Minimal discord.py v2 bot with both prefix and slash commands, on_ready handler, an admin cog (!echo + /echo), and a slim python:3.11 Dockerfile. The cheapest 'bot' blueprint in the catalogue — pick it when the spec asks for a chat bot or community automation.",
			Category:                 "bot",
			CostPriorUSD:             decimal.RequireFromString("0.40"),
			ExpectedTimeToPreviewSec: 30,
			SupportedGates: []string{
				"scaffold", "build",
			},
			Files: mustLoadFiles("templates/discord-bot-py"),
		},
	}
}

// mustLoadFiles walks the embedded templates FS under root and
// returns every regular file as a TemplateFile with workspace-
// relative paths (i.e. with the root prefix stripped). Panics on any
// I/O error — the embed tree is built into the binary, so a failure
// here means the binary itself is corrupt and refusing to start is
// the only sane response.
//
// Templates that would otherwise be picked up by the surrounding Go
// module's build (a top-level go.mod, a *.go file Go would try to
// compile) are stored on disk with a trailing ".tmpl" suffix; this
// loader strips that suffix so the workspace sees the real filename.
// Example: templates/go-http-api/main.go.tmpl → workspace main.go.
func mustLoadFiles(root string) []TemplateFile {
	out := []TemplateFile{}
	err := fs.WalkDir(templatesFS, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, rerr := templatesFS.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		rel := strings.TrimPrefix(p, root+"/")
		// Use forward slashes regardless of host OS — Executor
		// translates them when writing.
		rel = path.Clean(rel)
		rel = strings.TrimSuffix(rel, ".tmpl")
		out = append(out, TemplateFile{
			Path:    rel,
			Content: string(data),
			Mode:    0o644,
		})
		return nil
	})
	if err != nil {
		panic(fmt.Sprintf("blueprints: load templates from %q: %v", root, err))
	}
	return out
}
