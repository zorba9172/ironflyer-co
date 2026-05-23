// Package domain holds the core Ironflyer types shared across packages.
package domain

import (
	"encoding/json"
	"time"
)

type GateName string

const (
	GateSpec     GateName = "spec"
	GateUX       GateName = "ux"
	GateArch     GateName = "arch"
	GateCode     GateName = "code"
	GateLint     GateName = "lint"
	GateTest     GateName = "test"
	GateSecurity GateName = "security"
	GateBudget   GateName = "budget"
	GateDeploy   GateName = "deploy"
)

func AllGates() []GateName {
	return []GateName{GateSpec, GateUX, GateArch, GateCode, GateLint, GateTest, GateSecurity, GateBudget, GateDeploy}
}

type GateStatus string

const (
	GateStatusPending  GateStatus = "pending"
	GateStatusRunning  GateStatus = "running"
	GateStatusPassed   GateStatus = "passed"
	GateStatusFailed   GateStatus = "failed"
	GateStatusBlocked  GateStatus = "blocked"
	GateStatusRepaired GateStatus = "repaired"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

type Issue struct {
	Gate     GateName `json:"gate"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Hint     string   `json:"hint,omitempty"`
	Path     string   `json:"path,omitempty"`
}

type GateState struct {
	Name     GateName   `json:"name"`
	Status   GateStatus `json:"status"`
	Issues   []Issue    `json:"issues,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Project struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Status      string                 `json:"status"`
	// OwnerID is the user that owns this project. Empty means "public" —
	// every authenticated user can read it (used for the seed demo project).
	OwnerID string                 `json:"ownerId,omitempty"`
	Spec    ProductSpec            `json:"spec"`
	Files   []FileNode             `json:"files"`
	// Artifacts holds typed, structured documents produced by the finisher
	// pipeline (plan, stack, screen_map, design_tokens, …). Stored as raw
	// JSON so callers can evolve the inner shape without a schema lock.
	// Prefer GetArtifact / SetArtifact over direct map access so nil-safe
	// behaviour is preserved.
	Artifacts map[string]json.RawMessage `json:"artifacts,omitempty"`
	Gates   map[GateName]GateState `json:"gates"`
	Events  []Event                `json:"events"`
	GitHub  *GitHubLink            `json:"github,omitempty"`
	// Secrets holds provisioned per-project credentials — DATABASE_URL,
	// Stripe keys, Supabase service-role tokens, etc. Never serialised to
	// JSON so it cannot leak through API responses; callers that need a
	// value must read it through the store and inject it explicitly (the
	// runtime sandbox is the typical sink).
	Secrets map[string]string `json:"-"`
	// VisualTargets is the pixel-perfect contract: the user uploads one
	// or more reference screenshots (Figma export, Lovable iteration,
	// hand-drawn mockup) and the UXGate refuses to pass until the live
	// preview matches within tolerance. Empty slice = no visual contract
	// (project ships on the regular gate set).
	VisualTargets []VisualTarget `json:"visualTargets,omitempty"`
	CreatedAt time.Time            `json:"createdAt"`
	UpdatedAt time.Time            `json:"updatedAt"`
}

// IsAccessibleBy returns true when userID owns the project or it is public.
func (p Project) IsAccessibleBy(userID string) bool {
	return p.OwnerID == "" || p.OwnerID == userID
}

type ProductSpec struct {
	Idea         string       `json:"idea"`
	UserStories  []UserStory  `json:"userStories"`
	DataModel    []EntityDef  `json:"dataModel"`
	Stack        StackDecision `json:"stack"`
}

type UserStory struct {
	ID          string   `json:"id"`
	As          string   `json:"as"`
	IWant       string   `json:"iWant"`
	SoThat      string   `json:"soThat"`
	Acceptance  []string `json:"acceptance"`
}

type EntityDef struct {
	Name   string   `json:"name"`
	Fields []string `json:"fields"`
}

type StackDecision struct {
	Frontend string `json:"frontend"`
	Backend  string `json:"backend"`
	Storage  string `json:"storage"`
	Auth     string `json:"auth"`
}

// VisualTarget is one reference screenshot the user wants the live
// preview to match. The UXGate fetches a screenshot of the running app
// at RouteHint + viewport, diffs it against ImagePNGBase64, and refuses
// to pass when the difference exceeds Tolerance.
type VisualTarget struct {
	ID              string `json:"id"`
	Name            string `json:"name,omitempty"`
	// RouteHint is the path the runtime should screenshot (e.g. "/",
	// "/pricing", "/app/dashboard"). Empty = "/".
	RouteHint       string `json:"routeHint,omitempty"`
	ViewportW       int    `json:"viewportW"`        // e.g. 1280
	ViewportH       int    `json:"viewportH"`        // e.g. 800
	// ImagePNGBase64 is the target screenshot, base64-encoded PNG bytes
	// (no data: prefix). The orchestrator decodes lazily — keep it small
	// (<= 2 MiB after encoding).
	ImagePNGBase64  string `json:"imagePngBase64"`
	// Tolerance is the fraction of pixels (0..1) that may differ before
	// the gate fails. Default 0.02 = 2% — generous enough that anti-
	// aliasing flicker won't fire false positives.
	Tolerance       float64 `json:"tolerance,omitempty"`
}

// GitHubLink binds a project to a remote GitHub repo so the coder/deploy
// gates can clone, push, and open PRs against it.
type GitHubLink struct {
	Owner         string `json:"owner"`
	Repo          string `json:"repo"`
	FullName      string `json:"fullName"`
	DefaultBranch string `json:"defaultBranch"`
	HTMLURL       string `json:"htmlUrl"`
}

type FileNode struct {
	Path    string `json:"path"`
	Type    string `json:"type"`
	Size    int    `json:"size,omitempty"`
	Content string `json:"content,omitempty"`
}

type Event struct {
	ID        string    `json:"id"`
	Step      string    `json:"step"`
	Agent     string    `json:"agent,omitempty"`
	Gate      GateName  `json:"gate,omitempty"`
	Message   string    `json:"message"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}
