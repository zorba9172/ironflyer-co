// Package httpapi exposes the orchestrator over HTTP/SSE using chi + zerolog.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"ironflyer/apps/orchestrator/internal/agents"
	"ironflyer/apps/orchestrator/internal/audit"
	"ironflyer/apps/orchestrator/internal/auth"
	"ironflyer/apps/orchestrator/internal/brainstorm"
	"ironflyer/apps/orchestrator/internal/budget"
	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/figma"
	"ironflyer/apps/orchestrator/internal/finisher"
	"ironflyer/apps/orchestrator/internal/integrations"
	"ironflyer/apps/orchestrator/internal/integrations/github"
	"ironflyer/apps/orchestrator/internal/leads"
	"ironflyer/apps/orchestrator/internal/memory"
	"ironflyer/apps/orchestrator/internal/metrics"
	"ironflyer/apps/orchestrator/internal/notify"
	"ironflyer/apps/orchestrator/internal/patch"
	"ironflyer/apps/orchestrator/internal/projectgraph"
	"ironflyer/apps/orchestrator/internal/providers"
	"ironflyer/apps/orchestrator/internal/retriever"
	"ironflyer/apps/orchestrator/internal/store"
	"ironflyer/apps/orchestrator/internal/webhooks"
)

// Deps groups everything the HTTP layer needs.
type Deps struct {
	Projects           store.Store
	Engine             *finisher.Engine
	Agents             *agents.Registry
	Patches            *patch.Engine
	Billing            *budget.Billing
	Strategist         *brainstorm.Strategist
	BSRunner           *brainstorm.Runner
	Guard              *providers.BillingGuard
	Auth               *auth.Service
	Stripe             *budget.StripeService
	Leads              leads.Store
	AuthOptional       bool // dev convenience: skip auth when true
	GitHub             *github.Service
	GitHubTokens       integrations.TokenStore // for FindByExternal during OAuth login
	GitHubPostLoginURL string                  // browser destination after link/login
	RuntimeURL         string                  // base URL of the workspace runtime, e.g. http://localhost:8090
	Logger             zerolog.Logger
	// Webhooks + notifications wiring. All four may be nil — handlers return
	// 503 when the dependency they need is absent so a partial config does
	// not crash the orchestrator at boot.
	Webhooks          webhooks.Store
	WebhookDispatcher *webhooks.Dispatcher
	NotifyPrefs       notify.PrefsStore
	Notify            *notify.Engine
	// Telemetry is the structured feed of agent calls (provider, model,
	// tokens, latency, cost). Optional; nil disables the /telemetry/agents
	// endpoint but the rest of the API still functions.
	Telemetry providers.TelemetrySink
	// Memory is the persistent intelligence store (Project / Execution /
	// User / Business memories). The finisher loop reads from it to ground
	// agent context; the HTTP API exposes it for dashboards + manual
	// curation. Optional; nil disables the /memory endpoints.
	Memory memory.Store
	// Audit is the immutable hash-chained action log. Powers the
	// production-trust moat — every consequential action lands here so
	// an enterprise customer can prove what happened, when, and to whom.
	// Optional; nil disables /audit endpoints but the rest of the API
	// stays functional.
	Audit audit.Store
	// FigmaTool is the in-process figma_import tool. The HTTP endpoint
	// invokes it directly so an operator can ingest a design from the
	// dashboard without going through the Coder loop. Optional — when
	// nil, /api/projects/:id/figma-import returns 503.
	FigmaTool *figma.Tool
	// Build identity stamped at link time via the orchestrator Makefile's
	// -ldflags target. In dev (`go run`, plain `go build`) these stay at
	// their defaults so /version still answers — it just reports "dev" /
	// "unknown", which is the correct signal that the binary did not
	// come out of the release pipeline.
	Version   string
	Commit    string
	BuildTime string
}

type API struct{ d Deps }

func New(d Deps) http.Handler {
	a := &API{d: d}
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(corsMiddleware)
	r.Use(logMiddleware(d.Logger))
	r.Use(middleware.Recoverer)

	r.Use(metrics.HTTP)
	r.Use(requestIDMiddleware)
	r.Use(accessLogMiddleware(d.Logger))

	r.Get("/health", a.health)
	a.RegisterHealth(r)
	r.Method("GET", "/metrics", metrics.Handler())
	r.Post("/leads/enterprise", a.withSignupRateLimit(a.enterpriseLead))

	// Public auth endpoints.
	r.Route("/auth", func(r chi.Router) {
		r.Post("/signup", a.withSignupRateLimit(a.signup))
		r.Post("/login", a.withSignupRateLimit(a.login))
		// /me uses the protected stack so it returns 401 when unauthenticated.
		r.Group(func(r chi.Router) {
			r.Use(a.authMiddleware())
			r.Get("/me", a.me)
		})
	})

	// GitHub OAuth: /auth/github/start is auth-protected (we need to know
	// which user is connecting). /callback is public because GitHub redirects
	// the browser to it; we recover the user from the state map.
	r.Route("/auth/github", func(r chi.Router) {
		// Login flow is public — anyone can click "Continue with GitHub".
		r.Get("/login/start", a.githubLoginStart)
		// Linking an existing session to GitHub requires auth first.
		r.Group(func(r chi.Router) {
			r.Use(a.authMiddleware())
			r.Get("/start", a.githubStart)
		})
		r.Get("/callback", a.githubCallback)
	})

	// Protected routes — everything project-scoped requires a user.
	r.Group(func(r chi.Router) {
		r.Use(a.authMiddleware())

		r.Route("/integrations/github", func(r chi.Router) {
			r.Get("/me", a.githubMe)
			r.Get("/repos", a.githubRepos)
			r.Delete("/", a.githubDisconnect)
		})

		r.Route("/projects", func(r chi.Router) {
			r.Get("/", a.listProjects)
			r.Post("/", a.createProject)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", a.getProject)
				r.Get("/files", a.listFiles)
				r.Get("/graph", a.projectGraph)
				r.Get("/gates", a.listGates)
				r.Get("/snapshot", a.projectSnapshot)
				r.Post("/run", a.runFinisher)
				r.Get("/stream", a.streamEvents)
				r.Post("/prompt", a.promptPlan)
				r.Post("/chat", a.withChatRateLimit(a.chatStream))
				r.Post("/brainstorm", a.brainstormRun)
				r.Get("/patches", a.listPatches)
				r.Post("/patches", a.proposePatch)
				r.Post("/visual-edit", a.visualEdit)
				r.Get("/visual-targets", a.listVisualTargets)
				r.Post("/visual-targets", a.addVisualTarget)
				r.Delete("/visual-targets/{targetId}", a.deleteVisualTarget)
				r.Get("/subprojects", a.listSubprojects)
				r.Post("/subprojects", a.addSubproject)
				r.Delete("/subprojects/{subId}", a.deleteSubproject)
				r.Get("/search", a.searchProjectCode)
				r.Post("/connect-github", a.projectConnectGitHub)
				r.Delete("/connect-github", a.projectDisconnectGitHub)
				r.Post("/clone-into-workspace", a.projectCloneIntoWorkspace)
				r.Post("/figma-import", a.figmaImport)
			})
		})
		r.Post("/patches/{patchId}/apply", a.applyPatch)
		r.Post("/patches/{patchId}/rollback", a.rollbackPatch)
		r.Get("/agents", a.listAgents)

		// Budget — per-user data is auth-scoped.
		r.Get("/budget/users/me", a.myBudget)
		// Stripe checkout requires an authenticated user (we forward their
		// id + email to Stripe so the webhook can map back).
		r.Post("/budget/checkout", a.startCheckout)

		// Dashboard self-service: per-user vault, bulk-delete projects, close account.
		a.RegisterDashboard(r)

		// MCP (Model Context Protocol) — JSON-RPC at /mcp so external AI
		// clients (Claude Desktop, Cursor, Zed, custom agents) can list
		// projects, read files, and propose patches under the
		// authenticated user's identity.
		a.RegisterMCP(r)

		// Agent telemetry feed — per-call structured record (provider,
		// model, tokens, cost, latency). Operators use it to tune the
		// model router and to spot slow / expensive agents.
		r.Get("/telemetry/agents", a.listAgentTelemetry)

		// Memory engine — persistent project intelligence. Layer 6 of
		// the AI Completion Infrastructure blueprint: the moat that
		// turns a long-lived project into a compounding asset.
		r.Route("/memory", func(r chi.Router) {
			r.Get("/", a.listMemory)
			r.Post("/", a.addMemory)
			r.Delete("/{id}", a.deleteMemory)
		})

		// Immutable hash-chained audit log. Powers the production-trust
		// moat: enterprise customers can replay or verify every
		// consequential action the orchestrator took.
		r.Route("/audit", func(r chi.Router) {
			r.Get("/", a.listAudit)
			r.Get("/verify", a.verifyAudit)
		})

		// Webhooks + notification preferences (Agent N). Appended as a single
		// block so the route group stays diff-friendly for other agents.
		r.Route("/webhooks", func(r chi.Router) {
			r.Get("/", a.listWebhooks)
			r.Post("/", a.createWebhook)
			r.Delete("/{id}", a.deleteWebhook)
			r.Post("/{id}/test", a.testWebhook)
		})
		r.Route("/notifications", func(r chi.Router) {
			r.Get("/preferences", a.getNotificationPrefs)
			r.Put("/preferences", a.setNotificationPrefs)
		})

		// GitHub repo import — turns an external repo into an Ironflyer
		// project + workspace so the user can continue finishing it.
		r.Route("/imports", func(r chi.Router) {
			r.Post("/", a.startImport)
			r.Get("/{projectId}/status", a.importStatus)
		})
	})

	// Public catalogue endpoints (plans/rates/vault snapshot).
	r.Route("/budget", func(r chi.Router) {
		r.Get("/plans", a.listPlans)
		r.Get("/rates", a.listRates)
		r.Get("/vault", a.vaultSnapshot)
	})

	// Stripe webhook is PUBLIC (Stripe calls it server-to-server). Auth is
	// the Stripe-Signature header — verified inside the handler.
	r.Post("/budget/webhook", a.stripeWebhook)

	// Deploy routes (Agent E). Appended last so the deploy package owns
	// its own slice of the router without touching existing groupings.
	a.RegisterDeploy(r)

	return r
}

// authMiddleware honours the AuthOptional dev flag.
func (a *API) authMiddleware() func(http.Handler) http.Handler {
	if a.d.AuthOptional || a.d.Auth == nil {
		return auth.Optional(a.d.Auth)
	}
	return auth.Middleware(a.d.Auth)
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"service": "ironflyer-orchestrator",
		"version": "v13-budget-brainstorm",
	})
}

func (a *API) listProjects(w http.ResponseWriter, r *http.Request) {
	uid := userIDFromCtx(r)
	all := a.d.Projects.List()
	out := make([]domain.Project, 0, len(all))
	for _, p := range all {
		if p.IsAccessibleBy(uid) {
			out = append(out, p)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) createProject(w http.ResponseWriter, r *http.Request) {
	var body struct{ ID, Name, Description, Idea string }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON"))
		return
	}
	if body.ID == "" {
		body.ID = slug(body.Name)
	}
	if body.ID == "" {
		writeJSON(w, http.StatusBadRequest, errJSON("name or id required"))
		return
	}
	now := time.Now().UTC()
	p, err := a.d.Projects.Create(domain.Project{
		ID: body.ID, Name: body.Name, Description: body.Description,
		Status: "draft", Spec: domain.ProductSpec{Idea: body.Idea},
		OwnerID:   userIDFromCtx(r),
		CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		writeJSON(w, http.StatusConflict, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (a *API) getProject(w http.ResponseWriter, r *http.Request) {
	p, ok := a.requireProjectAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (a *API) listFiles(w http.ResponseWriter, r *http.Request) {
	p, ok := a.requireProjectAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, p.Files)
}

// projectGraph returns the derived dependency graph for the project's files.
// See package projectgraph for parser scope and resolution rules.
func (a *API) projectGraph(w http.ResponseWriter, r *http.Request) {
	p, ok := a.requireProjectAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	graph := projectgraph.Build(r.Context(), &p)
	writeJSON(w, http.StatusOK, graph)
}

func (a *API) listGates(w http.ResponseWriter, r *http.Request) {
	p, ok := a.requireProjectAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	out := make([]domain.GateState, 0, len(domain.AllGates()))
	for _, g := range domain.AllGates() {
		out = append(out, p.Gates[g])
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) runFinisher(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, ok := a.requireProjectAccess(w, r, id); !ok {
		return
	}
	// Forward the caller's bearer so build/test gates can authenticate to
	// the workspace runtime under the same identity.
	ctx := finisher.WithBearer(r.Context(), bearerFrom(r))
	report, err := a.d.Engine.Run(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (a *API) streamEvents(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, ok := a.requireProjectAccess(w, r, id); !ok {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	sseHeaders(w)
	ch, unsub := a.d.Engine.Subscribe(id)
	defer unsub()
	hb := time.NewTicker(15 * time.Second)
	defer hb.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			payload, _ := json.Marshal(evt)
			fmt.Fprintf(w, "event: execution\ndata: %s\n\n", payload)
			flusher.Flush()
		case <-hb.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

func (a *API) promptPlan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON"))
		return
	}
	if strings.TrimSpace(body.Prompt) == "" {
		writeJSON(w, http.StatusBadRequest, errJSON("prompt required"))
		return
	}
	p, ok := a.requireProjectAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	res, err := a.d.Agents.Run(r.Context(), agents.Task{
		Role: agents.RolePlanner, Project: &p, Goal: body.Prompt,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// chatStream is the live streaming chat endpoint. Uses the BillingGuard so
// every token is admitted, charged, and accounted to the user's plan.
func (a *API) chatStream(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Prompt string `json:"prompt"`
		Role   string `json:"role"`
		// Effort is a coarse UX dial — Lite biases the router toward cheap+fast
		// models, Power biases toward reasoning+thinking. Empty/"economy" keeps
		// the agent's declared capabilities untouched.
		Effort string `json:"effort"`
		// Attachments are user-supplied images (screenshots / mockups / design
		// references) sent inline with the prompt. Each entry is base64 image
		// bytes + IANA media type. Total decoded size is hard-capped so the
		// caller can't shovel megabytes of garbage at the vision provider.
		Attachments []struct {
			MediaType string `json:"mediaType"`
			Base64    string `json:"base64"`
		} `json:"attachments,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.Prompt) == "" {
		http.Error(w, "prompt required", http.StatusBadRequest)
		return
	}
	role := agents.Role(body.Role)
	if role == "" {
		role = agents.RolePlanner
	}
	userID := userIDFromCtx(r)
	p, ok := a.requireProjectAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	sseHeaders(w)
	turnID := uuid.NewString()
	send := func(eventName string, payload any) {
		data, _ := json.Marshal(payload)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventName, data)
		flusher.Flush()
	}
	send("turn", map[string]string{"id": turnID, "role": string(role), "userId": userID})

	// Resolve the agent's request envelope manually so we can route through
	// the BillingGuard (the Registry uses the bare Router).
	agent, ok := a.d.Agents.Get(role)
	if !ok {
		send("error", errJSON("unknown role"))
		return
	}
	caps, enableThinking := applyEffort(body.Effort, agent.Capabilities, agent.EnableThinking)
	// Validate + normalise attachments. We accept the common web image
	// types only and cap total payload at 8 MiB so a stray screen-recording
	// can't blow the request body up.
	const maxImagePayload = 8 << 20
	var atts []providers.Attachment
	var total int
	allowed := map[string]bool{
		"image/png": true, "image/jpeg": true, "image/webp": true, "image/gif": true,
	}
	for _, a := range body.Attachments {
		if a.Base64 == "" {
			continue
		}
		mt := strings.ToLower(strings.TrimSpace(a.MediaType))
		if !allowed[mt] {
			send("error", errJSON("unsupported attachment type: "+a.MediaType))
			return
		}
		total += (len(a.Base64) * 3) / 4
		if total > maxImagePayload {
			send("error", errJSON("attachments exceed 8 MiB total"))
			return
		}
		atts = append(atts, providers.Attachment{MediaType: mt, Base64: a.Base64})
	}
	req := providers.Request{
		System:         agent.System,
		Prompt:         "# Goal\n" + body.Prompt,
		Capabilities:   caps,
		EnableThinking: enableThinking,
		ProjectContext: projectContextFor(&p),
		TenantID:       userID,
		Attachments:    atts,
	}
	ch, err := a.d.Guard.CompleteStream(r.Context(), req)
	if err != nil {
		send("error", errJSON(err.Error()))
		return
	}
	for d := range ch {
		switch d.Type {
		case providers.DeltaStart:
			send("start", map[string]string{"provider": d.Provider, "model": d.Model, "turn": turnID})
		case providers.DeltaText:
			send("text", map[string]string{"text": d.Text, "turn": turnID})
		case providers.DeltaThinking:
			send("thinking", map[string]string{"text": d.Text, "turn": turnID})
		case providers.DeltaToolUse:
			send("tool_use", d.ToolUse)
		case providers.DeltaDone:
			send("done", map[string]any{
				"turn":     turnID,
				"provider": d.Provider,
				"model":    d.Model,
				"usage":    d.Usage,
			})
		case providers.DeltaError:
			send("error", errJSON(d.Err.Error()))
			return
		}
	}
}

func (a *API) brainstormRun(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Goal string `json:"goal"`
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON"))
		return
	}
	if strings.TrimSpace(body.Goal) == "" {
		writeJSON(w, http.StatusBadRequest, errJSON("goal required"))
		return
	}
	p, ok := a.requireProjectAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	role := agents.Role(body.Role)
	if role == "" {
		role = agents.RolePlanner
	}
	task := agents.Task{Role: role, Project: &p, Goal: body.Goal}
	plan := a.d.Strategist.Decide(task)
	outcome, err := a.d.BSRunner.Execute(r.Context(), plan, task)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan": plan, "outcome": outcome})
}

// visualEdit drives the click-to-edit path: the UI sends a CSS selector
// (or descriptive locator) plus a natural-language instruction, optionally
// with a screenshot of the live preview. We hand the bundle to the Coder
// agent and parse its response into a patch.Patch that the user can then
// review and apply through the normal Patch lifecycle. The endpoint is
// intentionally synchronous (single-request) — unlike /run, which kicks
// off the full finisher loop, this is a surgical "edit just this element"
// operation, so we want the patch back in the same response.
func (a *API) visualEdit(w http.ResponseWriter, r *http.Request) {
	var body struct {
		// Selector is the locator for the element the user clicked. We accept
		// either a CSS selector ("button.cta-primary"), a path-style
		// description ("Hero > Buttons[0]"), or a verbatim text snippet —
		// the Coder is told to treat it as a hint, not a literal query.
		Selector string `json:"selector"`
		// Instruction is the user's request in plain English ("make the
		// padding larger", "change copy to 'Get started'", "use lime accent").
		Instruction string `json:"instruction"`
		// Screenshot, if present, is base64-encoded image bytes of the live
		// preview at the moment of the click. The Coder uses it as a visual
		// reference; vision-capable models only.
		Screenshot          string `json:"screenshot,omitempty"`
		ScreenshotMediaType string `json:"screenshotMediaType,omitempty"`
		// Path narrows the search to a specific file when the UI knows
		// which component owns the selector (faster + cheaper than letting
		// the Coder grep the whole tree).
		Path string `json:"path,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON"))
		return
	}
	if strings.TrimSpace(body.Selector) == "" || strings.TrimSpace(body.Instruction) == "" {
		writeJSON(w, http.StatusBadRequest, errJSON("selector and instruction required"))
		return
	}
	proj, ok := a.requireProjectAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	coder, ok := a.d.Agents.Get(agents.RoleCoder)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("coder agent unavailable"))
		return
	}

	prompt := "# Visual edit\nThe user clicked an element in the live preview and asked for a surgical change.\n\n" +
		"Selector / locator hint: " + body.Selector + "\n"
	if body.Path != "" {
		prompt += "Likely owning file: " + body.Path + "\n"
	}
	prompt += "Instruction: " + body.Instruction + "\n\n" +
		"Reply with the SAME JSON shape the Coder always emits (a single patch with `changes`). " +
		"Prefer `replace` or `insert_after` ops keyed to a unique anchor near the target element — full-file rewrites are wasteful for a single-element tweak. " +
		"If the screenshot reveals layout or copy that differs from the source, trust the SOURCE and only change what the instruction asks for."

	caps := append([]providers.Capability(nil), coder.Capabilities...)
	var atts []providers.Attachment
	if body.Screenshot != "" {
		mt := strings.ToLower(strings.TrimSpace(body.ScreenshotMediaType))
		if mt == "" {
			mt = "image/png"
		}
		atts = []providers.Attachment{{MediaType: mt, Base64: body.Screenshot}}
	}

	req := providers.Request{
		System:         coder.System,
		Prompt:         prompt,
		Capabilities:   caps,
		EnableThinking: coder.EnableThinking,
		ProjectContext: projectContextFor(&proj),
		TenantID:       userIDFromCtx(r),
		Attachments:    atts,
	}
	ch, err := a.d.Guard.CompleteStream(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, errJSON(err.Error()))
		return
	}
	var text strings.Builder
	for d := range ch {
		switch d.Type {
		case providers.DeltaText:
			text.WriteString(d.Text)
		case providers.DeltaError:
			writeJSON(w, http.StatusBadGateway, errJSON(d.Err.Error()))
			return
		}
	}
	raw := strings.TrimSpace(text.String())
	if raw == "" {
		writeJSON(w, http.StatusBadGateway, errJSON("coder returned empty response"))
		return
	}
	// The Coder may wrap the JSON in a fenced block; strip a leading/trailing
	// ``` line if present so json.Unmarshal still sees a clean object.
	raw = stripJSONFence(raw)

	var coderOut struct {
		Title   string `json:"title"`
		Summary string `json:"summary"`
		Changes []struct {
			Op          string `json:"op"`
			Path        string `json:"path"`
			Content     string `json:"content,omitempty"`
			Anchor      string `json:"anchor,omitempty"`
			Replacement string `json:"replacement,omitempty"`
		} `json:"changes"`
	}
	if err := json.Unmarshal([]byte(raw), &coderOut); err != nil {
		writeJSON(w, http.StatusBadGateway, errJSON("coder produced invalid JSON: "+err.Error()))
		return
	}
	p := patch.Patch{
		ProjectID: proj.ID,
		Title:     coderOut.Title,
		Summary:   coderOut.Summary,
		Author:    "visual-edit",
	}
	for _, c := range coderOut.Changes {
		p.Changes = append(p.Changes, patch.FileChange{
			Op:          patch.Op(strings.ToLower(strings.TrimSpace(c.Op))),
			Path:        strings.TrimPrefix(c.Path, "/"),
			Content:     c.Content,
			Anchor:      c.Anchor,
			Replacement: c.Replacement,
		})
	}
	out, err := a.d.Patches.Propose(p)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// listVisualTargets returns the project's pixel-perfect targets. We
// trim ImagePNGBase64 in the response so the listing stays small;
// callers that need the bytes (rendering thumbnails) hit a dedicated
// per-target endpoint — added later if needed. For now we return the
// full payload to keep the API tight.
func (a *API) listVisualTargets(w http.ResponseWriter, r *http.Request) {
	p, ok := a.requireProjectAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"targets": p.VisualTargets,
		"count":   len(p.VisualTargets),
	})
}

// addVisualTarget uploads a reference screenshot the UXGate will diff
// the live preview against. Body: { name, routeHint, viewportW,
// viewportH, imagePngBase64, tolerance? }. We allocate the ID
// server-side so callers can't collide.
func (a *API) addVisualTarget(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireProjectAccess(w, r, chi.URLParam(r, "id")); !ok {
		return
	}
	id := chi.URLParam(r, "id")
	var body struct {
		Name           string  `json:"name"`
		RouteHint      string  `json:"routeHint"`
		ViewportW      int     `json:"viewportW"`
		ViewportH      int     `json:"viewportH"`
		ImagePNGBase64 string  `json:"imagePngBase64"`
		Tolerance      float64 `json:"tolerance"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON or payload > 4 MiB"))
		return
	}
	if strings.TrimSpace(body.ImagePNGBase64) == "" {
		writeJSON(w, http.StatusBadRequest, errJSON("imagePngBase64 required"))
		return
	}
	if body.ViewportW <= 0 {
		body.ViewportW = 1280
	}
	if body.ViewportH <= 0 {
		body.ViewportH = 800
	}
	target := domain.VisualTarget{
		ID:             uuid.NewString(),
		Name:           strings.TrimSpace(body.Name),
		RouteHint:      strings.TrimSpace(body.RouteHint),
		ViewportW:      body.ViewportW,
		ViewportH:      body.ViewportH,
		ImagePNGBase64: body.ImagePNGBase64,
		Tolerance:      body.Tolerance,
	}
	updated, err := a.d.Projects.Update(id, func(p *domain.Project) {
		p.VisualTargets = append(p.VisualTargets, target)
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"target": target,
		"count":  len(updated.VisualTargets),
	})
}

// deleteVisualTarget removes one target by id. Returns 404 when the id
// isn't on the project; idempotent on the wire (re-DELETE returns 404
// instead of erroring louder).
func (a *API) deleteVisualTarget(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireProjectAccess(w, r, chi.URLParam(r, "id")); !ok {
		return
	}
	id := chi.URLParam(r, "id")
	targetID := chi.URLParam(r, "targetId")
	if targetID == "" {
		writeJSON(w, http.StatusBadRequest, errJSON("targetId required"))
		return
	}
	var removed bool
	_, err := a.d.Projects.Update(id, func(p *domain.Project) {
		kept := p.VisualTargets[:0]
		for _, t := range p.VisualTargets {
			if t.ID == targetID {
				removed = true
				continue
			}
			kept = append(kept, t)
		}
		p.VisualTargets = kept
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	if !removed {
		writeJSON(w, http.StatusNotFound, errJSON("target not found"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// listSubprojects returns the project's subprojects (multi-service /
// monorepo layout). Empty slice means single-service (the default).
func (a *API) listSubprojects(w http.ResponseWriter, r *http.Request) {
	p, ok := a.requireProjectAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"subprojects": p.Subprojects,
		"count":       len(p.Subprojects),
	})
}

// addSubproject registers a new service inside the project. Body:
// { name, path, role?, stack? }. The server assigns ID + CreatedAt so
// callers can't collide. Path is required because it's what
// SubprojectByPath uses to claim files at execution time.
func (a *API) addSubproject(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireProjectAccess(w, r, chi.URLParam(r, "id")); !ok {
		return
	}
	id := chi.URLParam(r, "id")
	var body struct {
		Name  string               `json:"name"`
		Path  string               `json:"path"`
		Role  string               `json:"role"`
		Stack domain.StackDecision `json:"stack"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON or payload > 64 KiB"))
		return
	}
	name := strings.TrimSpace(body.Name)
	path := strings.Trim(strings.TrimSpace(body.Path), "/")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, errJSON("name required"))
		return
	}
	if path == "" {
		writeJSON(w, http.StatusBadRequest, errJSON("path required"))
		return
	}
	sub := domain.Subproject{
		ID:        uuid.NewString(),
		Name:      name,
		Path:      path,
		Role:      strings.TrimSpace(body.Role),
		Stack:     body.Stack,
		CreatedAt: time.Now().UTC(),
	}
	updated, err := a.d.Projects.Update(id, func(p *domain.Project) {
		p.Subprojects = append(p.Subprojects, sub)
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"subproject": sub,
		"count":      len(updated.Subprojects),
	})
}

// deleteSubproject removes a subproject by id. Idempotent on the wire
// — re-DELETE returns 200 with ok:false rather than a noisy 404, mirroring
// memory.Delete. Returns 404 when the subproject isn't on the project so
// dashboards can detect the no-op explicitly.
func (a *API) deleteSubproject(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireProjectAccess(w, r, chi.URLParam(r, "id")); !ok {
		return
	}
	id := chi.URLParam(r, "id")
	subID := chi.URLParam(r, "subId")
	if subID == "" {
		writeJSON(w, http.StatusBadRequest, errJSON("subId required"))
		return
	}
	var removed bool
	_, err := a.d.Projects.Update(id, func(p *domain.Project) {
		kept := p.Subprojects[:0]
		for _, s := range p.Subprojects {
			if s.ID == subID {
				removed = true
				continue
			}
			kept = append(kept, s)
		}
		p.Subprojects = kept
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	if !removed {
		writeJSON(w, http.StatusNotFound, errJSON("subproject not found"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// listAudit returns audit entries matching the query filters. Read-only
// — the audit log is append-only by contract. Query params:
//   - projectId, userId
//   - action: patch.proposed | patch.applied | gate.verdict | ...
//   - outcome: success | failure | blocked
//   - since / until (RFC3339)
//   - limit: cap, default 100, ceiling 1000
func (a *API) listAudit(w http.ResponseWriter, r *http.Request) {
	if a.d.Audit == nil {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("audit store not configured"))
		return
	}
	q := audit.Query{
		UserID:    r.URL.Query().Get("userId"),
		ProjectID: r.URL.Query().Get("projectId"),
		Action:    audit.Action(r.URL.Query().Get("action")),
		Outcome:   audit.Outcome(r.URL.Query().Get("outcome")),
	}
	if v := r.URL.Query().Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q.Since = t
		}
	}
	if v := r.URL.Query().Get("until"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q.Until = t
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			q.Limit = n
		}
	}
	if q.Limit > 1000 {
		q.Limit = 1000
	}
	// Ownership scope: project-scoped reads check access; otherwise the
	// caller can only see entries tagged with their own userId.
	if q.ProjectID != "" {
		if _, ok := a.requireProjectAccess(w, r, q.ProjectID); !ok {
			return
		}
	} else {
		q.UserID = userIDFromCtx(r)
	}
	rows, err := a.d.Audit.Query(r.Context(), q)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": rows, "count": len(rows)})
}

// verifyAudit walks the audit log's hash chain and returns the index of
// the first inconsistency, or -1 when the log is intact. Used for
// compliance attestations — the operator pipes the result into a
// monitoring system and fires on chain breakage.
func (a *API) verifyAudit(w http.ResponseWriter, r *http.Request) {
	if a.d.Audit == nil {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("audit store not configured"))
		return
	}
	idx, err := a.d.Audit.Verify(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"intact":          idx < 0,
		"firstBadIndex":   idx,
	})
}

// listMemory returns memory records matching the query filters.
// Query params:
//   - kind: project | execution | user | business
//   - projectId / userId / storyId / gateName: scope filters
//   - tag: single-tag filter
//   - q: substring search across title+body
//   - limit: cap, default 20, hard ceiling 200
// At least one of kind / projectId / userId is required.
func (a *API) listMemory(w http.ResponseWriter, r *http.Request) {
	if a.d.Memory == nil {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("memory store not configured"))
		return
	}
	q := memory.Query{
		Kind:      memory.Kind(strings.ToLower(r.URL.Query().Get("kind"))),
		ProjectID: r.URL.Query().Get("projectId"),
		UserID:    r.URL.Query().Get("userId"),
		StoryID:   r.URL.Query().Get("storyId"),
		GateName:  r.URL.Query().Get("gateName"),
		Tag:       r.URL.Query().Get("tag"),
		Substring: r.URL.Query().Get("q"),
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			q.Limit = n
		}
	}
	if q.Limit > 200 {
		q.Limit = 200
	}
	// Ownership scope: when caller is authenticated AND projectId is
	// supplied, verify access through the standard path. Anonymous /
	// public reads are limited to public projects.
	if q.ProjectID != "" {
		if _, ok := a.requireProjectAccess(w, r, q.ProjectID); !ok {
			return
		}
	} else if q.Kind == "" && q.UserID == "" {
		writeJSON(w, http.StatusBadRequest, errJSON("at least one of kind / projectId / userId is required"))
		return
	}
	// For user memories, enforce that the caller owns the userId scope.
	if q.UserID != "" && q.UserID != userIDFromCtx(r) {
		writeJSON(w, http.StatusForbidden, errJSON("cannot read another user's memory"))
		return
	}
	rows, err := a.d.Memory.Query(r.Context(), q)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"records": rows, "count": len(rows)})
}

// addMemory persists a single record. Body matches memory.Record; the
// server assigns ID + CreatedAt server-side. User-memory writes are
// pinned to the caller's userId regardless of payload (no spoofing).
func (a *API) addMemory(w http.ResponseWriter, r *http.Request) {
	if a.d.Memory == nil {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("memory store not configured"))
		return
	}
	var rec memory.Record
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&rec); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON or payload > 256 KiB"))
		return
	}
	switch rec.Kind {
	case memory.KindProject, memory.KindExecution, memory.KindBusiness:
		if rec.ProjectID == "" {
			writeJSON(w, http.StatusBadRequest, errJSON("projectId required for this kind"))
			return
		}
		if _, ok := a.requireProjectAccess(w, r, rec.ProjectID); !ok {
			return
		}
	case memory.KindUser:
		// Always stamp the authenticated user — never let a payload
		// pretend to write into another user's memory.
		rec.UserID = userIDFromCtx(r)
		if rec.UserID == "" {
			writeJSON(w, http.StatusUnauthorized, errJSON("user memory requires authentication"))
			return
		}
	default:
		writeJSON(w, http.StatusBadRequest, errJSON("kind must be project | execution | user | business"))
		return
	}
	stored, err := a.d.Memory.Record(r.Context(), rec)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusCreated, stored)
}

// deleteMemory removes a record by id. Idempotent — unknown id returns
// 204 so DELETE retries don't surface a 404 in dashboards.
func (a *API) deleteMemory(w http.ResponseWriter, r *http.Request) {
	if a.d.Memory == nil {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("memory store not configured"))
		return
	}
	if err := a.d.Memory.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listAgentTelemetry returns the most recent agent calls. Query params:
//   - limit: max rows (default 100, hard cap 1000)
//   - role / provider / model: optional client-side filters (server filters)
// Returns an array of provider.AgentCall objects, newest first.
func (a *API) listAgentTelemetry(w http.ResponseWriter, r *http.Request) {
	if a.d.Telemetry == nil {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("telemetry sink not configured"))
		return
	}
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 1000 {
		limit = 1000
	}
	wantRole := strings.ToLower(r.URL.Query().Get("role"))
	wantProvider := strings.ToLower(r.URL.Query().Get("provider"))
	wantModel := strings.ToLower(r.URL.Query().Get("model"))

	rows := a.d.Telemetry.Recent(limit * 2) // headroom so filters still hit `limit`
	out := make([]providers.AgentCall, 0, limit)
	for _, c := range rows {
		if wantRole != "" && !strings.EqualFold(c.Role, wantRole) {
			continue
		}
		if wantProvider != "" && !strings.EqualFold(c.Provider, wantProvider) {
			continue
		}
		if wantModel != "" && !strings.EqualFold(c.Model, wantModel) {
			continue
		}
		out = append(out, c)
		if len(out) >= limit {
			break
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"calls": out,
		"count": len(out),
	})
}

// stripJSONFence trims a leading/trailing ```json fence (and a bare ```
// closer) from a model response. The Coder is supposed to emit raw JSON,
// but some prompts produce a fence regardless — fall back gracefully.
func stripJSONFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// Drop the opening fence line.
		if nl := strings.IndexByte(s, '\n'); nl >= 0 {
			s = s[nl+1:]
		}
	}
	if strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}

func (a *API) listPatches(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, ok := a.requireProjectAccess(w, r, id); !ok {
		return
	}
	writeJSON(w, http.StatusOK, a.d.Patches.List(id))
}

func (a *API) proposePatch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, ok := a.requireProjectAccess(w, r, id); !ok {
		return
	}
	var p patch.Patch
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON"))
		return
	}
	p.ProjectID = id
	out, err := a.d.Patches.Propose(p)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (a *API) applyPatch(w http.ResponseWriter, r *http.Request) {
	// Apply must verify the underlying project is accessible to the caller.
	patchID := chi.URLParam(r, "patchId")
	existing, err := a.d.Patches.Get(patchID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errJSON("not found"))
		return
	}
	if _, ok := a.requireProjectAccess(w, r, existing.ProjectID); !ok {
		return
	}
	out, err := a.d.Patches.Apply(patchID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// searchProjectCode exposes the in-process retriever (BM25 + structure-
// aware chunker) over HTTP so the VSCode extension, SDK clients, and
// downstream tools can query the project's source without re-implementing
// retrieval. The same Build()/Query() helpers power the Coder's RAG
// context inside the finisher loop — this endpoint is just a public
// surface for the same index. Per-user ownership is enforced.
//
// Query string:
//   q       — the search query (required)
//   k       — max hits to return (default 8, hard-capped at 32)
//   max_kb  — per-chunk truncation in KiB (default 8) so a 1 MiB file
//             body never blows up the JSON response
//
// Response: {"hits": [{ "path", "startLine", "endLine", "symbols", "score", "text" }, ...]}
func (a *API) searchProjectCode(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	proj, ok := a.requireProjectAccess(w, r, projectID)
	if !ok {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusBadRequest, errJSON("query parameter q is required"))
		return
	}
	k := parsePositiveIntDefault(r.URL.Query().Get("k"), 8)
	if k > 32 {
		k = 32
	}
	maxBytes := parsePositiveIntDefault(r.URL.Query().Get("max_kb"), 8) * 1024
	idx := retriever.Build(&proj, retriever.Options{TopK: k})
	hits := idx.Query(q, k)

	type hit struct {
		Path      string   `json:"path"`
		StartLine int      `json:"startLine"`
		EndLine   int      `json:"endLine"`
		Symbols   []string `json:"symbols,omitempty"`
		Score     float64  `json:"score"`
		Text      string   `json:"text"`
	}
	out := struct {
		Hits []hit `json:"hits"`
	}{Hits: make([]hit, 0, len(hits))}
	for _, h := range hits {
		body := h.Text
		if len(body) > maxBytes {
			body = body[:maxBytes] + "\n…[truncated]"
		}
		out.Hits = append(out.Hits, hit{
			Path:      h.Path,
			StartLine: h.StartLine,
			EndLine:   h.EndLine,
			Symbols:   h.Symbols,
			Score:     h.Score,
			Text:      body,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// parsePositiveIntDefault parses query-string ints with a fallback. We
// reject non-positive values so callers can't pass k=-1 to force the
// retriever's "all chunks" path.
func parsePositiveIntDefault(s string, fallback int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return fallback
		}
		n = n*10 + int(r-'0')
		if n > 100000 {
			return fallback
		}
	}
	if n <= 0 {
		return fallback
	}
	return n
}

// rollbackPatch reverts the project tree to the snapshot taken just before
// the named patch was applied. This is the user-facing escape hatch — it
// exposes the in-process snapshot store that patch.Engine.Apply has been
// populating on every successful apply. Returns 404 if the patch is
// unknown, 409 if no snapshot exists for that patch (e.g. it was never
// applied), or 200 with the restored snapshot's metadata on success.
func (a *API) rollbackPatch(w http.ResponseWriter, r *http.Request) {
	patchID := chi.URLParam(r, "patchId")
	existing, err := a.d.Patches.Get(patchID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errJSON("not found"))
		return
	}
	if _, ok := a.requireProjectAccess(w, r, existing.ProjectID); !ok {
		return
	}
	// Locate the snapshot that this patch produced. Snapshots are tagged
	// with the patch ID on the Engine.Snapshot path so a 1:1 lookup is
	// stable even after multiple patches stack up.
	var targetSnap string
	for _, snap := range a.d.Patches.Snapshots(existing.ProjectID) {
		if snap.PatchID == patchID {
			targetSnap = snap.ID
			break
		}
	}
	if targetSnap == "" {
		writeJSON(w, http.StatusConflict, errJSON("no snapshot for this patch — rollback unavailable"))
		return
	}
	restored, err := a.d.Patches.Rollback(existing.ProjectID, targetSnap)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, restored)
}

// listAgents returns the full agent catalogue — role, system prompt,
// capability tags, enable-thinking — so SDK / VSCode / MCP clients can
// render an agent picker without baking the prompts into their bundle.
func (a *API) listAgents(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.d.Agents.All())
}

func (a *API) listPlans(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.d.Billing.Plans)
}

func (a *API) listRates(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.d.Billing.Rates.All())
}

func (a *API) vaultSnapshot(w http.ResponseWriter, r *http.Request) {
	snap, err := a.d.Billing.Vault.Snapshot(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// myBudget returns the authenticated user's spend + plan tier.
func (a *API) myBudget(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errJSON("unauthenticated"))
		return
	}
	spent, _ := a.d.Billing.Ledger.SpentByUser(r.Context(), u.ID)
	entries, _ := a.d.Billing.Ledger.EntriesByUser(r.Context(), u.ID)
	writeJSON(w, http.StatusOK, map[string]any{
		"userId":  u.ID,
		"email":   u.Email,
		"tier":    u.Plan,
		"spent":   spent,
		"entries": entries,
	})
}

// ---------------- Auth handlers ----------------

func (a *API) signup(w http.ResponseWriter, r *http.Request) {
	if a.d.Auth == nil {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("auth not configured"))
		return
	}
	var body struct{ Email, Name, Password string }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON"))
		return
	}
	u, token, err := a.d.Auth.Signup(r.Context(), auth.SignupInput{
		Email: body.Email, Name: body.Name, Password: body.Password,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON(err.Error()))
		return
	}
	// Bind the user's default plan into the billing facade (records revenue).
	a.d.Billing.AssignPlan(r.Context(), u.ID, budget.PlanTier(u.Plan))
	writeJSON(w, http.StatusCreated, map[string]any{"user": u, "token": token})
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	if a.d.Auth == nil {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("auth not configured"))
		return
	}
	var body struct{ Email, Password string }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON"))
		return
	}
	u, token, err := a.d.Auth.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errJSON(err.Error()))
		return
	}
	// Keep billing's in-memory plan map aligned with the persisted plan.
	a.d.Billing.AssignPlan(r.Context(), u.ID, budget.PlanTier(u.Plan))
	writeJSON(w, http.StatusOK, map[string]any{"user": u, "token": token})
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errJSON("unauthenticated"))
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// ---------------- GitHub OAuth + repos ----------------

func (a *API) githubReady(w http.ResponseWriter) bool {
	if a.d.GitHub == nil || !a.d.GitHub.Enabled() {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("github integration disabled"))
		return false
	}
	return true
}

// githubStart kicks off the LINK flow — the caller is already authenticated
// and wants to connect their existing account to a GitHub identity.
func (a *API) githubStart(w http.ResponseWriter, r *http.Request) {
	if !a.githubReady(w) {
		return
	}
	uid := userIDFromCtx(r)
	url, state, err := a.d.GitHub.AuthURL(github.FlowLink, uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	if r.URL.Query().Get("redirect") == "true" {
		http.Redirect(w, r, url, http.StatusFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"authUrl": url, "state": state})
}

// githubLoginStart kicks off the LOGIN flow — anonymous visitor signs in /
// signs up using GitHub. Always redirects since the caller is not yet a
// SPA-managed session.
func (a *API) githubLoginStart(w http.ResponseWriter, r *http.Request) {
	if !a.githubReady(w) {
		return
	}
	url, _, err := a.d.GitHub.AuthURL(github.FlowLogin, "")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

// githubCallback is the redirect target GitHub sends the browser to. It
// branches by FlowMode: LINK upserts the integration on the already-known
// user; LOGIN finds-or-creates a user and issues a JWT.
func (a *API) githubCallback(w http.ResponseWriter, r *http.Request) {
	if a.d.GitHub == nil || !a.d.GitHub.Enabled() {
		http.Error(w, "github integration disabled", http.StatusServiceUnavailable)
		return
	}
	q := r.URL.Query()
	if errMsg := q.Get("error"); errMsg != "" {
		http.Redirect(w, r, a.postLoginURL()+"?github=error&reason="+errMsg, http.StatusFound)
		return
	}
	code, state := q.Get("code"), q.Get("state")
	if code == "" || state == "" {
		http.Error(w, "missing code/state", http.StatusBadRequest)
		return
	}
	res, err := a.d.GitHub.Exchange(r.Context(), state, code)
	if err != nil {
		a.d.Logger.Warn().Err(err).Msg("github exchange failed")
		http.Redirect(w, r, a.postLoginURL()+"?github=error", http.StatusFound)
		return
	}

	switch res.Mode {
	case github.FlowLink:
		if err := a.d.GitHub.PersistToken(r.Context(), res.UserID, res); err != nil {
			a.d.Logger.Warn().Err(err).Msg("github persist (link) failed")
			http.Redirect(w, r, a.postLoginURL()+"?github=error", http.StatusFound)
			return
		}
		http.Redirect(w, r, a.postLoginURL()+"?github=connected&login="+res.GitHubLogin, http.StatusFound)

	case github.FlowLogin:
		userID, tokenStr, err := a.githubResolveLoginUser(r.Context(), res)
		if err != nil {
			a.d.Logger.Warn().Err(err).Msg("github login resolve failed")
			http.Redirect(w, r, a.postLoginURL()+"?github=error", http.StatusFound)
			return
		}
		if err := a.d.GitHub.PersistToken(r.Context(), userID, res); err != nil {
			a.d.Logger.Warn().Err(err).Msg("github persist (login) failed")
		}
		// Hand the JWT to the SPA via a fragment so it never hits a server
		// log. The /login page extracts it from window.location.hash.
		http.Redirect(w, r,
			a.postLoginURL()+"#github=login&login="+res.GitHubLogin+"&token="+tokenStr,
			http.StatusFound)

	default:
		http.Error(w, "unknown flow", http.StatusInternalServerError)
	}
}

// githubResolveLoginUser maps a GitHub identity to an Ironflyer user, creating
// one if needed. Returns the user's ID and a freshly-minted JWT.
func (a *API) githubResolveLoginUser(ctx context.Context, res github.ExchangeResult) (string, string, error) {
	if a.d.Auth == nil {
		return "", "", errJSONErr("auth not configured")
	}
	// 1. Existing GitHub-linked user?
	if a.d.GitHubTokens != nil {
		if uid, err := a.d.GitHubTokens.FindByExternal(ctx, integrations.KindGitHub, res.GitHubID); err == nil {
			u, err := a.d.Auth.GetByID(ctx, uid)
			if err != nil {
				return "", "", err
			}
			tok, err := a.d.Auth.IssueToken(u)
			if err != nil {
				return "", "", err
			}
			return u.ID, tok, nil
		}
	}
	// 2. Fall back to email match / signup.
	email := strings.ToLower(res.GitHubEmail)
	if email == "" {
		email = res.GitHubLogin + "@users.noreply.github.com"
	}
	u, _, err := a.d.Auth.EnsureUserByEmail(ctx, email, res.GitHubName)
	if err != nil {
		return "", "", err
	}
	// New user → assign default Pro plan so demo experience matches signup.
	a.d.Billing.AssignPlan(ctx, u.ID, budget.PlanTier(u.Plan))
	tok, err := a.d.Auth.IssueToken(u)
	if err != nil {
		return "", "", err
	}
	return u.ID, tok, nil
}

func errJSONErr(msg string) error { return errors.New(msg) }

func (a *API) githubMe(w http.ResponseWriter, r *http.Request) {
	if !a.githubReady(w) {
		return
	}
	status, err := a.d.GitHub.Status(r.Context(), userIDFromCtx(r))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (a *API) githubRepos(w http.ResponseWriter, r *http.Request) {
	if !a.githubReady(w) {
		return
	}
	repos, err := a.d.GitHub.ListRepos(r.Context(), userIDFromCtx(r))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, repos)
}

func (a *API) githubDisconnect(w http.ResponseWriter, r *http.Request) {
	if !a.githubReady(w) {
		return
	}
	if err := a.d.GitHub.Disconnect(r.Context(), userIDFromCtx(r)); err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// projectConnectGitHub binds a repo to a project. Caller must own the
// project and have a live GitHub connection.
func (a *API) projectConnectGitHub(w http.ResponseWriter, r *http.Request) {
	if !a.githubReady(w) {
		return
	}
	id := chi.URLParam(r, "id")
	if _, ok := a.requireProjectAccess(w, r, id); !ok {
		return
	}
	var body struct {
		Owner         string `json:"owner"`
		Repo          string `json:"repo"`
		FullName      string `json:"fullName"`
		DefaultBranch string `json:"defaultBranch"`
		HTMLURL       string `json:"htmlUrl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON"))
		return
	}
	// Accept either {owner, repo} or {fullName}; normalize to all three.
	if body.FullName == "" && body.Owner != "" && body.Repo != "" {
		body.FullName = body.Owner + "/" + body.Repo
	}
	if body.FullName == "" {
		writeJSON(w, http.StatusBadRequest, errJSON("owner+repo or fullName required"))
		return
	}
	if body.Owner == "" || body.Repo == "" {
		parts := strings.SplitN(body.FullName, "/", 2)
		if len(parts) != 2 {
			writeJSON(w, http.StatusBadRequest, errJSON("fullName must be owner/repo"))
			return
		}
		body.Owner, body.Repo = parts[0], parts[1]
	}
	link := &domain.GitHubLink{
		Owner: body.Owner, Repo: body.Repo, FullName: body.FullName,
		DefaultBranch: body.DefaultBranch, HTMLURL: body.HTMLURL,
	}
	updated, err := a.d.Projects.Update(id, func(p *domain.Project) {
		p.GitHub = link
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// projectCloneIntoWorkspace asks the runtime to git-clone the project's
// linked GitHub repo into the user's workspace. The caller's GitHub OAuth
// token is forwarded so the runtime can authenticate against private repos.
//
// Body: { "workspaceId": "ws-...", "ref": "main" (optional), "subdir": "" }
func (a *API) projectCloneIntoWorkspace(w http.ResponseWriter, r *http.Request) {
	if !a.githubReady(w) {
		return
	}
	if strings.TrimSpace(a.d.RuntimeURL) == "" {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("runtime URL not configured"))
		return
	}
	id := chi.URLParam(r, "id")
	p, ok := a.requireProjectAccess(w, r, id)
	if !ok {
		return
	}
	if p.GitHub == nil {
		writeJSON(w, http.StatusBadRequest, errJSON("project not linked to a GitHub repo"))
		return
	}

	var body struct {
		WorkspaceID string `json:"workspaceId"`
		Ref         string `json:"ref"`
		Subdir      string `json:"subdir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON"))
		return
	}
	if strings.TrimSpace(body.WorkspaceID) == "" {
		writeJSON(w, http.StatusBadRequest, errJSON("workspaceId required"))
		return
	}

	uid := userIDFromCtx(r)
	token, err := a.d.GitHub.TokenFor(r.Context(), uid)
	if err != nil {
		writeJSON(w, http.StatusPreconditionRequired, errJSON("connect GitHub first"))
		return
	}

	ref := body.Ref
	if ref == "" {
		ref = p.GitHub.DefaultBranch
	}
	cloneURL := p.GitHub.HTMLURL
	if cloneURL == "" {
		cloneURL = "https://github.com/" + p.GitHub.FullName
	}

	payload := map[string]any{
		"cloneUrl": cloneURL,
		"token":    token,
		"ref":      ref,
		"subdir":   body.Subdir,
	}
	bts, _ := json.Marshal(payload)

	// Forward the caller's Bearer so the runtime authorizes the workspace
	// against the same identity (it owns the per-user ownership check).
	rtURL := strings.TrimRight(a.d.RuntimeURL, "/") + "/workspaces/" + body.WorkspaceID + "/git-clone"
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, rtURL, strings.NewReader(string(bts)))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if h := r.Header.Get("Authorization"); h != "" {
		req.Header.Set("Authorization", h)
	}
	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errJSON("runtime unreachable: "+err.Error()))
		return
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(bodyBytes)
}

// figmaImport ingests a Figma file into the calling user's workspace.
// Body: { "fileKey": "...", "workspaceId": "..." }. The workspaceId is
// required because the underlying figma.Tool writes the extracted
// manifests directly into a runtime sandbox; without it the call would
// fail anyway with "no workspace bound". The operator-configured
// FIGMA_TOKEN is read out of cfg.FigmaToken (held inside the tool's
// Client); when empty the endpoint returns 503 so the dashboard can
// render a "connect Figma" prompt instead of a confusing 500.
func (a *API) figmaImport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, ok := a.requireProjectAccess(w, r, id); !ok {
		return
	}
	if a.d.FigmaTool == nil || a.d.FigmaTool.Client == nil || a.d.FigmaTool.Client.Token == "" {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("figma token not configured"))
		return
	}
	var body struct {
		FileKey     string `json:"fileKey"`
		WorkspaceID string `json:"workspaceId"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON or payload > 64 KiB"))
		return
	}
	if strings.TrimSpace(body.FileKey) == "" {
		writeJSON(w, http.StatusBadRequest, errJSON("fileKey required"))
		return
	}
	if strings.TrimSpace(body.WorkspaceID) == "" {
		writeJSON(w, http.StatusBadRequest, errJSON("workspaceId required"))
		return
	}
	out, err := a.d.FigmaTool.Run(r.Context(), bearerFrom(r), body.WorkspaceID, body.FileKey)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errJSON(err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(out))
}

func (a *API) projectDisconnectGitHub(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, ok := a.requireProjectAccess(w, r, id); !ok {
		return
	}
	updated, err := a.d.Projects.Update(id, func(p *domain.Project) { p.GitHub = nil })
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) postLoginURL() string {
	if a.d.GitHubPostLoginURL != "" {
		return a.d.GitHubPostLoginURL
	}
	return "/app"
}

// projectContextFor mirrors agents.projectContext (kept private there). Small
// duplication beats exporting an internal helper.
func projectContextFor(p *domain.Project) string {
	if p == nil {
		return ""
	}
	out := "# Project\nName: " + p.Name + "\nDescription: " + p.Description + "\n"
	if p.Spec.Idea != "" {
		out += "Idea: " + p.Spec.Idea + "\n"
	}
	if len(p.Files) > 0 {
		out += "\n## Files\n"
		for _, f := range p.Files {
			out += "- " + f.Path + "\n"
		}
	}
	return out
}

func sseHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func errJSON(msg string) map[string]string { return map[string]string{"error": msg} }

// bearerFrom extracts the raw JWT from the Authorization header, if any. We
// re-forward it to the workspace runtime for owner-scoped exec calls.
func bearerFrom(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

// userIDFromCtx returns the authenticated user's ID. Falls back to "demo"
// when AuthOptional is on and no user is present (dev convenience only).
func userIDFromCtx(r *http.Request) string {
	if u, ok := auth.FromContext(r.Context()); ok {
		return u.ID
	}
	return "demo"
}

// requireProjectAccess loads a project and 404s when the caller can neither
// own it nor read it as public. Returning 404 (not 403) avoids leaking
// existence of projects the caller can't see.
func (a *API) requireProjectAccess(w http.ResponseWriter, r *http.Request, projectID string) (domain.Project, bool) {
	p, err := a.d.Projects.Get(projectID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errJSON("not found"))
		return domain.Project{}, false
	}
	if !p.IsAccessibleBy(userIDFromCtx(r)) {
		writeJSON(w, http.StatusNotFound, errJSON("not found"))
		return domain.Project{}, false
	}
	return p, true
}

func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('-')
		}
	}
	return b.String()
}
