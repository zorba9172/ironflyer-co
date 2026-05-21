// Package httpapi exposes the orchestrator over HTTP/SSE using chi + zerolog.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"ironflyer/apps/orchestrator/internal/agents"
	"ironflyer/apps/orchestrator/internal/auth"
	"ironflyer/apps/orchestrator/internal/brainstorm"
	"ironflyer/apps/orchestrator/internal/budget"
	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/finisher"
	"ironflyer/apps/orchestrator/internal/integrations"
	"ironflyer/apps/orchestrator/internal/integrations/github"
	"ironflyer/apps/orchestrator/internal/patch"
	"ironflyer/apps/orchestrator/internal/providers"
	"ironflyer/apps/orchestrator/internal/store"
)

// Deps groups everything the HTTP layer needs.
type Deps struct {
	Projects     store.Store
	Engine       *finisher.Engine
	Agents       *agents.Registry
	Patches      *patch.Engine
	Billing      *budget.Billing
	Strategist   *brainstorm.Strategist
	BSRunner     *brainstorm.Runner
	Guard        *providers.BillingGuard
	Auth         *auth.Service
	Stripe       *budget.StripeService
	AuthOptional bool // dev convenience: skip auth when true
	GitHub       *github.Service
	GitHubTokens integrations.TokenStore // for FindByExternal during OAuth login
	GitHubPostLoginURL string // browser destination after link/login
	RuntimeURL string       // base URL of the workspace runtime, e.g. http://localhost:8090
	Logger       zerolog.Logger
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

	r.Get("/health", a.health)

	// Public auth endpoints.
	r.Route("/auth", func(r chi.Router) {
		r.Post("/signup", a.signup)
		r.Post("/login", a.login)
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
				r.Get("/gates", a.listGates)
				r.Post("/run", a.runFinisher)
				r.Get("/stream", a.streamEvents)
				r.Post("/prompt", a.promptPlan)
				r.Post("/chat", a.chatStream)
				r.Post("/brainstorm", a.brainstormRun)
				r.Get("/patches", a.listPatches)
				r.Post("/patches", a.proposePatch)
				r.Post("/connect-github", a.projectConnectGitHub)
				r.Delete("/connect-github", a.projectDisconnectGitHub)
				r.Post("/clone-into-workspace", a.projectCloneIntoWorkspace)
			})
		})
		r.Post("/patches/{patchId}/apply", a.applyPatch)
		r.Get("/agents", a.listAgents)

		// Budget — per-user data is auth-scoped.
		r.Get("/budget/users/me", a.myBudget)
		// Stripe checkout requires an authenticated user (we forward their
		// id + email to Stripe so the webhook can map back).
		r.Post("/budget/checkout", a.startCheckout)
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
	var body struct{ Prompt string `json:"prompt"` }
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
	req := providers.Request{
		System:         agent.System,
		Prompt:         "# Goal\n" + body.Prompt,
		Capabilities:   caps,
		EnableThinking: enableThinking,
		ProjectContext: projectContextFor(&p),
		TenantID:       userID,
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

func (a *API) listAgents(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.d.Agents.Roles())
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
