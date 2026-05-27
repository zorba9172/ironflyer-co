// Package github_pr — webhook receiver.
//
// The webhook URL is `POST /webhooks/github/{projectID}/{secret}`. The
// `{secret}` path segment is the per-project URL-routing token (NOT the
// HMAC key); GitHub still signs the body with the canonical HMAC key
// stored in Project.Secrets["GITHUB_WEBHOOK_SECRET"]. Splitting the two
// makes URL leaks recoverable without a re-key on the GitHub side.
//
// Flow when a `pull_request` event lands with action opened /
// synchronize / reopened:
//
//  1. Verify the path-routing secret matches Project.Secrets
//     ["GITHUB_WEBHOOK_SECRET_ROUTING"] (falls back to the HMAC key when
//     the routing-only key is unset — single-key bootstrap is legal).
//  2. Verify the HMAC against the same key.
//  3. Pull the PR + changed files via the Client (uses GITHUB_PAT).
//  4. Drive the Reviewer agent with the diff and the project context;
//     compose a structured verdict.
//  5. Post the verdict back to the PR as an Issue-style comment.
//
// Other actions / events are accepted (HTTP 200) and ignored so GitHub
// stops retrying.
package github_pr

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/ai/finisher"
	"ironflyer/core/orchestrator/internal/operations/store"
)

const (
	// SecretKeyHMAC is the Project.Secrets key holding the GitHub
	// webhook HMAC. Used to verify the X-Hub-Signature-256 header on
	// every incoming event.
	SecretKeyHMAC = "GITHUB_WEBHOOK_SECRET"
	// SecretKeyRouting is the OPTIONAL Project.Secrets key holding a
	// URL-routing token used in the webhook path. When unset, the
	// HMAC secret doubles as the routing token (single-key bootstrap).
	SecretKeyRouting = "GITHUB_WEBHOOK_SECRET_ROUTING"
	// SecretKeyPAT is the Project.Secrets key holding the GitHub
	// Personal Access Token / installation token used by the outbound
	// REST client.
	SecretKeyPAT = "GITHUB_PAT"

	// maxBodyBytes caps the inbound payload so a malicious webhook
	// cannot OOM the orchestrator.
	maxBodyBytes = 5 << 20 // 5 MiB
)

// Engine is the subset of *finisher.Engine the webhook handler needs.
// Defining a local interface keeps the package import surface tight and
// makes substitution trivial without circular deps.
type Engine interface {
	RunGate(ctx context.Context, projectID string, gateName string) (domain.GateState, error)
}

// AgentDriver is the subset of the Reviewer agent the webhook uses to
// generate the PR verdict. We do NOT plumb the full *agents.Registry
// here — the per-call API is enough.
type AgentDriver interface {
	Run(ctx context.Context, task agents.Task) (agents.Result, error)
}

// HandlerDeps wires the webhook to the orchestrator's shared services.
// Every field is required at construction time — nil is rejected so
// runtime nil-checks stay local.
type HandlerDeps struct {
	Projects store.Store
	Engine   Engine        // optional; when nil the gate fan-out is skipped
	Agents   AgentDriver   // optional; when nil the Reviewer step is skipped
	Logger   zerolog.Logger
	// ClientFactory builds the GitHub REST client for a given PAT. Tests
	// override this to inject a fake. Nil falls back to the default
	// NewClient() constructor.
	ClientFactory func(pat string) *Client
}

// Handler is the HTTP receiver. Safe for concurrent use.
type Handler struct {
	deps HandlerDeps

	// Per-project mutex so two simultaneous PR events on the same
	// project serialize through the Reviewer rather than racing.
	muMu sync.Mutex
	mus  map[string]*sync.Mutex
}

// NewHandler builds a Handler.
func NewHandler(deps HandlerDeps) *Handler {
	if deps.ClientFactory == nil {
		deps.ClientFactory = func(pat string) *Client { return NewClient(pat) }
	}
	return &Handler{
		deps: deps,
		mus:  map[string]*sync.Mutex{},
	}
}

// Mount registers the webhook route on the chi router. The path uses
// chi URL parameters so the project + routing-secret round-trip
// without query-string parsing.
func (h *Handler) Mount(r chi.Router) {
	r.Post("/webhooks/github/{projectID}/{secret}", h.handle)
}

// projectLock returns the per-project mutex, lazily creating it.
func (h *Handler) projectLock(projectID string) *sync.Mutex {
	h.muMu.Lock()
	defer h.muMu.Unlock()
	mu, ok := h.mus[projectID]
	if !ok {
		mu = &sync.Mutex{}
		h.mus[projectID] = mu
	}
	return mu
}

// handle is the HTTP entry point.
func (h *Handler) handle(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	projectID := strings.TrimSpace(chi.URLParam(r, "projectID"))
	routingSecret := strings.TrimSpace(chi.URLParam(r, "secret"))
	if projectID == "" || routingSecret == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing path params"})
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read body: " + err.Error()})
		return
	}
	if len(body) > maxBodyBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "payload too large"})
		return
	}

	p, err := h.deps.Projects.Get(projectID)
	if err != nil {
		// Return 200 to suppress GitHub retries on misrouted events but
		// log the miss so misconfiguration is visible.
		h.deps.Logger.Warn().Str("project_id", projectID).Err(err).
			Msg("github webhook: unknown project")
		writeJSON(w, http.StatusOK, map[string]string{"received": "ignored", "reason": "unknown project"})
		return
	}

	hmacSecret := strings.TrimSpace(p.Secrets[SecretKeyHMAC])
	if hmacSecret == "" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "webhook not configured for project"})
		return
	}
	wantRouting := strings.TrimSpace(p.Secrets[SecretKeyRouting])
	if wantRouting == "" {
		wantRouting = hmacSecret
	}
	if subtle.ConstantTimeCompare([]byte(routingSecret), []byte(wantRouting)) != 1 {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "routing secret mismatch"})
		return
	}

	if err := verifyGitHubHMAC(r.Header.Get("X-Hub-Signature-256"), body, hmacSecret); err != nil {
		h.deps.Logger.Warn().Str("project_id", projectID).Err(err).
			Msg("github webhook: signature verification failed")
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	event := r.Header.Get("X-GitHub-Event")
	if event == "ping" {
		writeJSON(w, http.StatusOK, map[string]string{"received": "pong"})
		return
	}
	if event != "pull_request" {
		// Accept and ignore — saves GitHub's retry budget without
		// implementing every event type up front.
		writeJSON(w, http.StatusOK, map[string]string{"received": "ignored", "reason": "unsupported event: " + event})
		return
	}

	var payload pullRequestEvent
	if err := json.Unmarshal(body, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "parse payload: " + err.Error()})
		return
	}

	switch payload.Action {
	case "opened", "synchronize", "reopened", "ready_for_review":
		// fall through to processing
	default:
		writeJSON(w, http.StatusOK, map[string]string{"received": "ignored", "reason": "action: " + payload.Action})
		return
	}

	owner := payload.Repository.Owner.Login
	repo := payload.Repository.Name
	prNumber := payload.PullRequest.Number
	if owner == "" || repo == "" || prNumber <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "incomplete repo/PR identity in payload"})
		return
	}

	// The webhook ACK must be quick (GitHub retries when we exceed 10s)
	// — fan the heavy work out to a background goroutine. Detach from
	// the request context but carry a logger correlation.
	go h.processPR(projectID, owner, repo, prNumber)

	writeJSON(w, http.StatusAccepted, map[string]string{
		"received":   "pull_request",
		"action":     payload.Action,
		"project_id": projectID,
		"pr_number":  fmt.Sprintf("%d", prNumber),
	})
}

// processPR runs the Reviewer pipeline asynchronously. Detached from
// the HTTP request so a slow LLM call doesn't make GitHub time out.
func (h *Handler) processPR(projectID, owner, repo string, prNumber int) {
	mu := h.projectLock(projectID)
	mu.Lock()
	defer mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	log := h.deps.Logger.With().
		Str("project_id", projectID).
		Str("repo", owner+"/"+repo).
		Int("pr", prNumber).
		Logger()

	p, err := h.deps.Projects.Get(projectID)
	if err != nil {
		log.Warn().Err(err).Msg("github webhook: project lookup failed")
		return
	}
	pat := strings.TrimSpace(p.Secrets[SecretKeyPAT])
	if pat == "" {
		log.Warn().Msg("github webhook: missing GITHUB_PAT — cannot post review")
		return
	}
	client := h.deps.ClientFactory(pat)

	pr, err := client.GetPullRequest(ctx, owner, repo, prNumber)
	if err != nil {
		log.Warn().Err(err).Msg("github webhook: GetPullRequest failed")
		return
	}
	files, err := client.ListChangedFiles(ctx, owner, repo, prNumber)
	if err != nil {
		log.Warn().Err(err).Msg("github webhook: ListChangedFiles failed")
		return
	}

	verdict := h.runReviewer(ctx, &p, pr, files)

	if err := client.CreatePRComment(ctx, owner, repo, prNumber, verdict.Markdown()); err != nil {
		log.Warn().Err(err).Msg("github webhook: CreatePRComment failed")
		return
	}
	log.Info().Int("findings", len(verdict.Findings)).Msg("github webhook: posted review verdict")
}

// runReviewer drives the Reviewer agent against the diff. When the
// agent driver is nil (degraded boot), we synthesise a structural
// stand-in verdict that names the changed files so the operator gets
// at least a deterministic ACK on the PR.
func (h *Handler) runReviewer(ctx context.Context, p *domain.Project, pr *PullRequest, files []ChangedFile) ReviewerVerdict {
	verdict := ReviewerVerdict{
		ProjectID: p.ID,
		PRNumber:  pr.Number,
		PRTitle:   pr.Title,
		Branch:    pr.Head.Ref,
		Files:     files,
		Generated: time.Now().UTC(),
	}

	if h.deps.Agents == nil {
		verdict.Summary = "Ironflyer received the PR but no Reviewer agent is wired in this deployment."
		verdict.Findings = append(verdict.Findings, ReviewerFinding{
			Severity: "info",
			Message:  "automated review skipped: agent registry not configured",
		})
		return verdict
	}

	prompt := buildReviewerPrompt(p, pr, files)
	task := agents.Task{
		Role:    agents.RoleReviewer,
		Project: p,
		Goal:    fmt.Sprintf("review PR #%d on %s/%s", pr.Number, ownerOf(pr), repoOf(pr)),
		Context: prompt,
	}
	res, err := h.deps.Agents.Run(ctx, task)
	if err != nil {
		verdict.Summary = "Reviewer agent failed: " + err.Error()
		verdict.Findings = append(verdict.Findings, ReviewerFinding{
			Severity: "warning",
			Message:  "reviewer agent error: " + err.Error(),
		})
		return verdict
	}
	parsed, ok := parseReviewerJSON(res.Output)
	if !ok {
		// Treat the raw output as the summary so the operator still
		// gets the agent's reasoning on the PR.
		verdict.Summary = strings.TrimSpace(res.Output)
		return verdict
	}
	verdict.Summary = parsed.Summary
	verdict.Findings = parsed.Findings
	verdict.Verdict = parsed.Verdict
	return verdict
}

// ownerOf / repoOf pull the owner+repo from the PR's HTMLURL — the
// HTMLURL is the only field on PullRequest that carries the repository
// identity in one place.
func ownerOf(pr *PullRequest) string {
	parts := strings.Split(strings.TrimPrefix(pr.HTMLURL, "https://github.com/"), "/")
	if len(parts) >= 1 {
		return parts[0]
	}
	return ""
}

func repoOf(pr *PullRequest) string {
	parts := strings.Split(strings.TrimPrefix(pr.HTMLURL, "https://github.com/"), "/")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

// ReviewerVerdict is the structured output the webhook posts back on
// the PR. The Markdown() rendering pins the dashboard / GitHub UI
// shape so both surfaces read identically.
type ReviewerVerdict struct {
	ProjectID string            `json:"projectId"`
	PRNumber  int               `json:"prNumber"`
	PRTitle   string            `json:"prTitle"`
	Branch    string            `json:"branch"`
	Verdict   string            `json:"verdict"` // approve | request_changes | comment
	Summary   string            `json:"summary"`
	Findings  []ReviewerFinding `json:"findings"`
	Files     []ChangedFile     `json:"files,omitempty"`
	Generated time.Time         `json:"generated"`
}

// ReviewerFinding is one structured note on the PR. Severity is the
// finisher Issue severity vocabulary (info | warning | error |
// critical).
type ReviewerFinding struct {
	Severity string `json:"severity"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
	Hint     string `json:"hint,omitempty"`
}

// Markdown renders the verdict as a GitHub Markdown comment.
func (v ReviewerVerdict) Markdown() string {
	var b strings.Builder
	header := "Ironflyer Reviewer"
	switch strings.ToLower(strings.TrimSpace(v.Verdict)) {
	case "approve", "approved":
		header += " — APPROVED"
	case "request_changes", "request-changes", "changes_requested":
		header += " — CHANGES REQUESTED"
	}
	b.WriteString("### " + header + "\n\n")
	if v.Summary != "" {
		b.WriteString(v.Summary)
		b.WriteString("\n\n")
	}
	if len(v.Findings) > 0 {
		b.WriteString("**Findings**\n\n")
		for _, f := range v.Findings {
			sev := strings.ToUpper(f.Severity)
			if sev == "" {
				sev = "INFO"
			}
			line := "- `" + sev + "`"
			if f.Path != "" {
				line += " `" + f.Path + "`"
			}
			line += " — " + f.Message
			if f.Hint != "" {
				line += "  \n   _" + f.Hint + "_"
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	if len(v.Files) > 0 {
		b.WriteString("<details><summary>Files changed (")
		b.WriteString(fmt.Sprintf("%d", len(v.Files)))
		b.WriteString(")</summary>\n\n")
		for _, f := range v.Files {
			b.WriteString("- `" + f.Filename + "` — +")
			b.WriteString(fmt.Sprintf("%d", f.Additions))
			b.WriteString("/-")
			b.WriteString(fmt.Sprintf("%d", f.Deletions))
			b.WriteString("\n")
		}
		b.WriteString("\n</details>\n\n")
	}
	b.WriteString("---\n_Generated by Ironflyer at ")
	b.WriteString(v.Generated.Format(time.RFC3339))
	b.WriteString("_\n")
	return b.String()
}

// pullRequestEvent is the subset of GitHub's pull_request event payload
// we parse. Fields not used are dropped to keep the type tight.
type pullRequestEvent struct {
	Action      string `json:"action"`
	PullRequest struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		State  string `json:"state"`
		Head   struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		HTMLURL string `json:"html_url"`
	} `json:"pull_request"`
	Repository struct {
		Name  string `json:"name"`
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
		FullName string `json:"full_name"`
	} `json:"repository"`
}

// verifyGitHubHMAC checks the X-Hub-Signature-256 header against the
// HMAC-SHA256 of body using secret. GitHub's header format is
// "sha256=<hex>".
func verifyGitHubHMAC(header string, body []byte, secret string) error {
	header = strings.TrimSpace(header)
	if header == "" {
		return errors.New("missing X-Hub-Signature-256 header")
	}
	if !strings.HasPrefix(header, "sha256=") {
		return errors.New("signature header missing sha256= prefix")
	}
	want, err := hex.DecodeString(strings.TrimPrefix(header, "sha256="))
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	if !hmac.Equal(want, mac.Sum(nil)) {
		return errors.New("signature mismatch")
	}
	return nil
}

// parsedReviewerOutput is the JSON contract the Reviewer agent emits.
type parsedReviewerOutput struct {
	Verdict  string            `json:"verdict"`
	Summary  string            `json:"summary"`
	Findings []ReviewerFinding `json:"findings"`
}

// parseReviewerJSON attempts to extract a JSON object from the agent's
// output. The Reviewer prompt asks for `{"verdict":...,"summary":...,
// "findings":[...]}`; we tolerate optional surrounding markdown.
func parseReviewerJSON(raw string) (parsedReviewerOutput, bool) {
	raw = strings.TrimSpace(raw)
	// Strip a markdown ```json fence when present.
	if strings.HasPrefix(raw, "```") {
		if idx := strings.Index(raw, "\n"); idx > 0 {
			raw = raw[idx+1:]
		}
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)
	}
	// Find the first '{' and the last '}' to tolerate prefixes.
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return parsedReviewerOutput{}, false
	}
	var out parsedReviewerOutput
	if err := json.Unmarshal([]byte(raw[start:end+1]), &out); err != nil {
		return parsedReviewerOutput{}, false
	}
	return out, true
}

// buildReviewerPrompt assembles the Reviewer's input message: the
// project's idea + stack, the PR title/body, and a per-file unified-
// diff hunk so the agent can reason about the actual changes.
func buildReviewerPrompt(p *domain.Project, pr *PullRequest, files []ChangedFile) string {
	var b strings.Builder
	b.WriteString("You are reviewing a GitHub pull request opened against an Ironflyer project.\n")
	b.WriteString("Project idea: ")
	b.WriteString(p.Spec.Idea)
	b.WriteString("\n")
	if p.Spec.Stack.Backend != "" || p.Spec.Stack.Frontend != "" {
		b.WriteString("Stack: backend=")
		b.WriteString(p.Spec.Stack.Backend)
		b.WriteString(" frontend=")
		b.WriteString(p.Spec.Stack.Frontend)
		b.WriteString("\n")
	}
	b.WriteString("\nPR #")
	b.WriteString(fmt.Sprintf("%d", pr.Number))
	b.WriteString(" — ")
	b.WriteString(pr.Title)
	b.WriteString("\nAuthor: ")
	b.WriteString(pr.User.Login)
	b.WriteString("\nBase: ")
	b.WriteString(pr.Base.Ref)
	b.WriteString(" Head: ")
	b.WriteString(pr.Head.Ref)
	b.WriteString("\n\n")
	if strings.TrimSpace(pr.Body) != "" {
		b.WriteString("Description:\n")
		b.WriteString(pr.Body)
		b.WriteString("\n\n")
	}
	b.WriteString("Changed files (")
	b.WriteString(fmt.Sprintf("%d", len(files)))
	b.WriteString("):\n")
	const maxPatchBytes = 32 * 1024
	used := 0
	for _, f := range files {
		header := fmt.Sprintf("\n--- %s (+%d/-%d, status=%s) ---\n", f.Filename, f.Additions, f.Deletions, f.Status)
		b.WriteString(header)
		used += len(header)
		patch := f.Patch
		if used+len(patch) > maxPatchBytes {
			if remaining := maxPatchBytes - used; remaining > 0 {
				patch = patch[:remaining] + "\n…(truncated)\n"
			} else {
				patch = "(truncated)\n"
			}
		}
		used += len(patch)
		b.WriteString(patch)
		if used >= maxPatchBytes {
			b.WriteString("\n(remaining files omitted — prompt budget reached)\n")
			break
		}
	}
	b.WriteString("\nReturn STRICT JSON with this shape:\n")
	b.WriteString(`{"verdict":"approve"|"request_changes"|"comment","summary":"<one-paragraph English>","findings":[{"severity":"info"|"warning"|"error","path":"<file>","message":"<what>","hint":"<how-to-fix>"}]}` + "\n")
	return b.String()
}

// writeJSON is a small helper so the webhook doesn't depend on the
// httpapi package's writeJSON (which is unexported there).
func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

// Compile-time check: *finisher.Engine satisfies the local Engine
// interface. This makes the wiring breakage land at the import site if
// the engine signature ever drifts.
var _ Engine = (*finisher.Engine)(nil)
