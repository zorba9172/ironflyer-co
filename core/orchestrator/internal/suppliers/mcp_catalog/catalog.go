// Package mcp_catalog ships the curated set of Model-Context-Protocol
// servers Ironflyer can spawn on a user's behalf. The catalog is the
// out-of-the-box menu users see in the Settings → MCP panel; an enabled
// entry launches a stdio child process whose JSON-RPC stream is then
// registered against the agents Coder loop via the existing
// providers.MCPClientRegistry surface.
//
// The catalog is intentionally vendor-agnostic — every spec carries a
// declarative description of how to spawn the server (Command + Args),
// what environment variables the underlying CLI needs, and which
// secrets the user must provision before the toggle goes live. The
// catalog does NOT speak to the provider's HTTP/SaaS surface; that's
// what the suppliers/* packages are for.
package mcp_catalog

import "strings"

// ServerSpec is the declarative description of a single MCP server we
// know how to launch. It carries everything the runner needs to spawn
// the process plus the operator-facing metadata the cockpit renders.
type ServerSpec struct {
	// ID is the slug-cased identifier ("github", "google-drive"). Used
	// as the agent-side prefix on every tool name the registry surfaces.
	ID string `json:"id"`
	// Name is the human label rendered in the catalog grid.
	Name string `json:"name"`
	// Description is one-line copy explaining what the server does.
	Description string `json:"description"`
	// Vendor is the operator-recognisable brand ("GitHub", "Linear",
	// "Stripe", ...). Distinct from Name so we can ship two specs from
	// the same vendor without colliding (Stripe Reader vs Stripe Mock).
	Vendor string `json:"vendor"`
	// Command + Args spell out the exec invocation. Today every entry
	// reaches for `npx` because the MCP ecosystem ships as npm packages
	// — but the runner accepts any executable on $PATH so future Go /
	// Python servers slot in without API changes.
	Command string   `json:"command"`
	Args    []string `json:"args"`
	// EnvKeys are the environment variables the child process needs.
	// The runner resolves them from Project.Secrets first, then falls
	// back to the orchestrator's own process env so self-hosted
	// singleton deployments work without per-project provisioning.
	EnvKeys []string `json:"envKeys"`
	// Capabilities is the short list of verbs the server advertises
	// ("read", "write", "search"). Rendered as chips on the card so an
	// operator scanning the catalog can tell at a glance whether the
	// server can mutate external state.
	Capabilities []string `json:"capabilities"`
	// RequiresSecret turns the "Enable" CTA into a "Connect" CTA: the
	// cockpit pops a small secrets dialog before calling Enable. False
	// for spec like `filesystem` and `playwright` that need no creds.
	RequiresSecret bool `json:"requiresSecret"`
	// Category groups the cards. The frontend renders one section per
	// category so the grid stays scannable as the catalog grows.
	Category string `json:"category"`
	// IconURL points at the vendor's logomark hosted on a CDN we trust.
	// Empty string is tolerated — the card falls back to a violet
	// initial tile when absent.
	IconURL string `json:"iconUrl"`
}

// DefaultCatalog is the curated set of 20 vetted MCP servers we
// expose out of the box. The list is intentionally additive: dropping
// an entry breaks anyone who already enabled it, so we prefer to mark
// servers deprecated in-place rather than remove them.
//
// The selection covers the five operator-recognisable categories the
// UI groups by:
//
//   - Productivity     · Notion, Linear, Jira, Asana, Google Drive
//   - Communication    · Slack, Discord
//   - Devtools         · GitHub, Sentry, Figma, Playwright, Brave Search
//   - Cloud & Hosting  · Vercel, Cloudflare, AWS, Resend
//   - Data & Payments  · Postgres, Stripe, Filesystem, PostHog
//
// Every Command is the canonical `npx <package>` invocation documented
// by the vendor. We never bake in a registry mirror; teams that air-
// gap can swap to a private npm mirror via the standard NPM_CONFIG_*
// env vars without us re-publishing the catalog.
func DefaultCatalog() []ServerSpec {
	return []ServerSpec{
		{
			ID: "context7", Name: "Context7", Vendor: "Context7",
			Description:    "Ground every code-writing agent against up-to-date, version-accurate library documentation — cuts hallucinated and stale third-party APIs.",
			Command:        "npx",
			Args:           []string{"-y", "@upstash/context7-mcp"},
			EnvKeys:        nil, // public server; an optional CONTEXT7_API_KEY only raises the rate limit.
			Capabilities:   []string{"read", "search"},
			RequiresSecret: false,
			Category:       "Devtools",
			IconURL:        "https://cdn.simpleicons.org/readme/ffffff",
		},
		{
			ID: "github", Name: "GitHub", Vendor: "GitHub",
			Description:    "Read repositories, open pull requests, comment on issues, manage releases.",
			Command:        "npx",
			Args:           []string{"-y", "@modelcontextprotocol/server-github"},
			EnvKeys:        []string{"GITHUB_PAT"},
			Capabilities:   []string{"read", "write", "search"},
			RequiresSecret: true,
			Category:       "Devtools",
			IconURL:        "https://cdn.simpleicons.org/github/ffffff",
		},
		{
			ID: "linear", Name: "Linear", Vendor: "Linear",
			Description:    "Create, update, and triage Linear issues; route Ironflyer execution output back into team workflows.",
			Command:        "npx",
			Args:           []string{"-y", "mcp-linear"},
			EnvKeys:        []string{"LINEAR_API_KEY"},
			Capabilities:   []string{"read", "write"},
			RequiresSecret: true,
			Category:       "Productivity",
			IconURL:        "https://cdn.simpleicons.org/linear/ffffff",
		},
		{
			ID: "sentry", Name: "Sentry", Vendor: "Sentry",
			Description:    "Fetch issue context, mark resolved, link the Coder's fix patch to the originating event.",
			Command:        "npx",
			Args:           []string{"-y", "mcp-server-sentry"},
			EnvKeys:        []string{"SENTRY_AUTH_TOKEN"},
			Capabilities:   []string{"read", "write"},
			RequiresSecret: true,
			Category:       "Devtools",
			IconURL:        "https://cdn.simpleicons.org/sentry/ffffff",
		},
		{
			ID: "notion", Name: "Notion", Vendor: "Notion",
			Description:    "Read and append to Notion pages so generated specs land alongside human-authored docs.",
			Command:        "npx",
			Args:           []string{"-y", "mcp-notion"},
			EnvKeys:        []string{"NOTION_API_KEY"},
			Capabilities:   []string{"read", "write"},
			RequiresSecret: true,
			Category:       "Productivity",
			IconURL:        "https://cdn.simpleicons.org/notion/ffffff",
		},
		{
			ID: "slack", Name: "Slack", Vendor: "Slack",
			Description:    "Post execution updates and gate verdicts into the team's Slack channel.",
			Command:        "npx",
			Args:           []string{"-y", "@modelcontextprotocol/server-slack"},
			EnvKeys:        []string{"SLACK_BOT_TOKEN", "SLACK_TEAM_ID"},
			Capabilities:   []string{"read", "write"},
			RequiresSecret: true,
			Category:       "Communication",
			IconURL:        "https://cdn.simpleicons.org/slack/ffffff",
		},
		{
			ID: "stripe", Name: "Stripe", Vendor: "Stripe",
			Description:    "Inspect customers, subscriptions, and payments while the Coder edits billing code.",
			Command:        "npx",
			Args:           []string{"-y", "@stripe/mcp"},
			EnvKeys:        []string{"STRIPE_SECRET_KEY"},
			Capabilities:   []string{"read", "write"},
			RequiresSecret: true,
			Category:       "Data & Payments",
			IconURL:        "https://cdn.simpleicons.org/stripe/ffffff",
		},
		{
			ID: "postgres", Name: "Postgres", Vendor: "PostgreSQL",
			Description:    "Run scoped read-only queries against the project database so the Coder can ground its schema decisions.",
			Command:        "npx",
			Args:           []string{"-y", "@modelcontextprotocol/server-postgres"},
			EnvKeys:        []string{"POSTGRES_URL"},
			Capabilities:   []string{"read", "schema"},
			RequiresSecret: true,
			Category:       "Data & Payments",
			IconURL:        "https://cdn.simpleicons.org/postgresql/ffffff",
		},
		{
			ID: "filesystem", Name: "Filesystem", Vendor: "Ironflyer",
			Description:    "Sandboxed file-system access scoped to the project's runtime workspace path.",
			Command:        "npx",
			Args:           []string{"-y", "@modelcontextprotocol/server-filesystem", "/workspace"},
			EnvKeys:        []string{},
			Capabilities:   []string{"read", "write"},
			RequiresSecret: false,
			Category:       "Devtools",
			IconURL:        "",
		},
		{
			ID: "brave-search", Name: "Brave Search", Vendor: "Brave",
			Description:    "Web search grounding for the Planner so spec decisions cite live sources.",
			Command:        "npx",
			Args:           []string{"-y", "@modelcontextprotocol/server-brave-search"},
			EnvKeys:        []string{"BRAVE_API_KEY"},
			Capabilities:   []string{"search"},
			RequiresSecret: true,
			Category:       "Devtools",
			IconURL:        "https://cdn.simpleicons.org/brave/ffffff",
		},
		{
			ID: "google-drive", Name: "Google Drive", Vendor: "Google",
			Description:    "Read documents and spreadsheets so the Planner can ground decisions in the team's existing artifacts.",
			Command:        "npx",
			Args:           []string{"-y", "@modelcontextprotocol/server-gdrive"},
			EnvKeys:        []string{"GDRIVE_OAUTH_TOKEN"},
			Capabilities:   []string{"read"},
			RequiresSecret: true,
			Category:       "Productivity",
			IconURL:        "https://cdn.simpleicons.org/googledrive/ffffff",
		},
		{
			ID: "figma", Name: "Figma", Vendor: "Figma",
			Description:    "Pull frames, components, and tokens out of the team's Figma file straight into the UXer's reference.",
			Command:        "npx",
			Args:           []string{"-y", "mcp-figma"},
			EnvKeys:        []string{"FIGMA_PAT"},
			Capabilities:   []string{"read"},
			RequiresSecret: true,
			Category:       "Devtools",
			IconURL:        "https://cdn.simpleicons.org/figma/ffffff",
		},
		{
			ID: "jira", Name: "Jira", Vendor: "Atlassian",
			Description:    "Create, transition, and comment on Jira tickets when external work-tracking lives there.",
			Command:        "npx",
			Args:           []string{"-y", "mcp-jira"},
			EnvKeys:        []string{"JIRA_USER", "JIRA_API_TOKEN", "JIRA_HOST"},
			Capabilities:   []string{"read", "write"},
			RequiresSecret: true,
			Category:       "Productivity",
			IconURL:        "https://cdn.simpleicons.org/jira/ffffff",
		},
		{
			ID: "asana", Name: "Asana", Vendor: "Asana",
			Description:    "Track project tasks, assignees, and due dates inside Asana while the finisher executes.",
			Command:        "npx",
			Args:           []string{"-y", "mcp-asana"},
			EnvKeys:        []string{"ASANA_PAT"},
			Capabilities:   []string{"read", "write"},
			RequiresSecret: true,
			Category:       "Productivity",
			IconURL:        "https://cdn.simpleicons.org/asana/ffffff",
		},
		{
			ID: "discord", Name: "Discord", Vendor: "Discord",
			Description:    "Post execution updates and approval requests into the team's Discord server.",
			Command:        "npx",
			Args:           []string{"-y", "mcp-discord"},
			EnvKeys:        []string{"DISCORD_BOT_TOKEN"},
			Capabilities:   []string{"read", "write"},
			RequiresSecret: true,
			Category:       "Communication",
			IconURL:        "https://cdn.simpleicons.org/discord/ffffff",
		},
		{
			ID: "vercel", Name: "Vercel", Vendor: "Vercel",
			Description:    "Inspect Vercel projects, deployments, env vars; surface preview URLs back to the orchestrator.",
			Command:        "npx",
			Args:           []string{"-y", "mcp-vercel"},
			EnvKeys:        []string{"VERCEL_TOKEN"},
			Capabilities:   []string{"read", "write"},
			RequiresSecret: true,
			Category:       "Cloud & Hosting",
			IconURL:        "https://cdn.simpleicons.org/vercel/ffffff",
		},
		{
			ID: "cloudflare", Name: "Cloudflare", Vendor: "Cloudflare",
			Description:    "DNS, Workers, R2, KV — manage edge primitives without leaving the cockpit.",
			Command:        "npx",
			Args:           []string{"-y", "mcp-cloudflare"},
			EnvKeys:        []string{"CLOUDFLARE_API_TOKEN"},
			Capabilities:   []string{"read", "write"},
			RequiresSecret: true,
			Category:       "Cloud & Hosting",
			IconURL:        "https://cdn.simpleicons.org/cloudflare/ffffff",
		},
		{
			ID: "aws", Name: "AWS", Vendor: "Amazon Web Services",
			Description:    "S3, Lambda, IAM read/write so the Deployer can land artifacts on the user's AWS account.",
			Command:        "npx",
			Args:           []string{"-y", "mcp-aws"},
			EnvKeys:        []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_REGION"},
			Capabilities:   []string{"read", "write"},
			RequiresSecret: true,
			Category:       "Cloud & Hosting",
			IconURL:        "https://cdn.simpleicons.org/amazonaws/ffffff",
		},
		{
			ID: "resend", Name: "Resend", Vendor: "Resend",
			Description:    "Send transactional email straight from finisher runs (alerts, gate verdicts, signed receipts).",
			Command:        "npx",
			Args:           []string{"-y", "mcp-resend"},
			EnvKeys:        []string{"RESEND_API_KEY"},
			Capabilities:   []string{"write"},
			RequiresSecret: true,
			Category:       "Cloud & Hosting",
			IconURL:        "https://cdn.simpleicons.org/resend/ffffff",
		},
		{
			ID: "posthog", Name: "PostHog", Vendor: "PostHog",
			Description:    "Query funnels, retention, and feature-flag rollouts so the Planner grounds decisions in product data.",
			Command:        "npx",
			Args:           []string{"-y", "mcp-posthog"},
			EnvKeys:        []string{"POSTHOG_API_KEY"},
			Capabilities:   []string{"read"},
			RequiresSecret: true,
			Category:       "Data & Payments",
			IconURL:        "https://cdn.simpleicons.org/posthog/ffffff",
		},
		{
			ID: "playwright", Name: "Playwright", Vendor: "Microsoft",
			Description:    "Headless browser automation that pairs with the Ironflyer Verifier for visual + flow regression.",
			Command:        "npx",
			Args:           []string{"-y", "@playwright/mcp"},
			EnvKeys:        []string{},
			Capabilities:   []string{"automation"},
			RequiresSecret: false,
			Category:       "Devtools",
			IconURL:        "https://cdn.simpleicons.org/playwright/ffffff",
		},
	}
}

// Get looks up a spec by ID. Slug-case is the canonical form but we
// accept upper-case input for resilience against the cockpit
// accidentally posting a Title-Case label.
func Get(id string) (ServerSpec, bool) {
	want := strings.ToLower(strings.TrimSpace(id))
	for _, s := range DefaultCatalog() {
		if s.ID == want {
			return s, true
		}
	}
	return ServerSpec{}, false
}
