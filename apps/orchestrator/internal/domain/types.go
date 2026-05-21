// Package domain holds the core Ironflyer types shared across packages.
package domain

import "time"

type GateName string

const (
	GateSpec     GateName = "spec"
	GateUX       GateName = "ux"
	GateArch     GateName = "arch"
	GateCode     GateName = "code"
	GateLint     GateName = "lint"
	GateTest     GateName = "test"
	GateSecurity GateName = "security"
	GateDeploy   GateName = "deploy"
)

func AllGates() []GateName {
	return []GateName{GateSpec, GateUX, GateArch, GateCode, GateLint, GateTest, GateSecurity, GateDeploy}
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
	Gates   map[GateName]GateState `json:"gates"`
	Events  []Event                `json:"events"`
	GitHub  *GitHubLink            `json:"github,omitempty"`
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
