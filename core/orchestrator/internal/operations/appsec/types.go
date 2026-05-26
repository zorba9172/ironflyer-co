// Package appsec is Ironflyer's in-process application-security core.
//
// It keeps scanners, inventory, policy, and risk context behind a small Go
// API so finisher gates, CI jobs, CLI commands, and future GraphQL mutations
// can all consume the same security verdict without knowing scanner details.
package appsec

import (
	"context"
	"time"

	"ironflyer/core/orchestrator/internal/operations/runtime"
)

type Category string

const (
	CategorySecrets   Category = "secrets"
	CategoryDeps      Category = "deps"
	CategoryCode      Category = "code"
	CategoryConfig    Category = "config"
	CategoryPolicy    Category = "policy"
	CategoryInventory Category = "inventory"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// File is the static, in-memory view of a project file. Generated projects
// may reach the Security gate before they exist on disk; scanners must support
// both this view and the runtime workspace view.
type File struct {
	Path    string
	Content string
}

// Executor is the small slice of runtime.Client that scanners need. Keeping
// it as an interface lets tests and future CI adapters reuse the engine
// without depending on a live workspace.
type Executor interface {
	Exec(ctx context.Context, userBearer, workspaceID string, opts runtime.ExecOpts) (runtime.ExecResult, error)
}

// Target describes a single appsec scan. Runtime fields are optional; when
// absent, the engine still runs static scanners against Files.
type Target struct {
	ProjectID   string
	TenantID    string
	Files       []File
	Config      *Config
	Runtime     Executor
	WorkspaceID string
	UserBearer  string
}

func (t Target) HasRuntime() bool {
	return t.Runtime != nil && t.WorkspaceID != ""
}

// Finding is the canonical appsec issue shape. It intentionally mirrors the
// securityreport surface while adding scanner/context fields that are useful
// before a report is persisted.
type Finding struct {
	ID          string
	Tool        string
	Category    Category
	Severity    Severity
	RuleID      string
	Path        string
	Line        int
	Package     string
	Summary     string
	Remediation string
	Evidence    string
	Verified    bool
	DetectedAt  time.Time
	Metadata    map[string]string
}

type Scanner interface {
	ID() string
	Supports(inv Inventory) bool
	Scan(ctx context.Context, target Target, inv Inventory) ([]Finding, error)
}

type Result struct {
	Inventory Inventory
	Config    Config
	Findings  []Finding
	Graph     RiskGraph
	Verdict   Verdict
	StartedAt time.Time
	EndedAt   time.Time
}

type Verdict struct {
	Status        string // pass | warning | fail
	BlockedDeploy bool
	OverallScore  float64
}
