package secrets

import (
	"context"
	"time"
)

// Backend identifies the storage system that actually holds the secret
// material. Only the broker is allowed to talk to a backend; everything
// else in the system holds a SecretRef + Capability and asks the broker
// to resolve when needed.
type Backend string

const (
	BackendEnv         Backend = "env"
	BackendMemory      Backend = "memory"
	BackendAWSSecrets  Backend = "aws_secrets"
	BackendGCPSecrets  Backend = "gcp_secrets"
	BackendVault       Backend = "vault"
	BackendKV          Backend = "kv"
)

// BackendImpl is the storage abstraction every backend implements.
// Living on the secrets package (not the backends subpackage) breaks
// the otherwise-circular dependency between broker.go and the
// concrete backend files; the backends subpackage imports secrets to
// pick up the interface + ref types.
//
// Implementations MUST be safe for concurrent use and MUST NOT cache
// raw values in process memory — the broker's Capability TTL is the
// only valid lifetime for material.
type BackendImpl interface {
	Name() Backend
	Load(ctx context.Context, ref SecretRef) ([]byte, error)
}

// ReleaseClass mirrors the V22 "Allowed secret release classes" table
// in docs/ARCHITECTURE_POLICY_SECURITY.md. The class is metadata on the
// SecretRef itself — it is the operator's declared intent for how this
// material may ever be released, and the broker refuses Release calls
// that do not match.
type ReleaseClass string

const (
	// ClassBuildTimeReference — only the name is ever observable to AI
	// or generated code (e.g. an env var name baked into a deploy file).
	ClassBuildTimeReference ReleaseClass = "build_time_reference"
	// ClassRuntimeMount — material is mounted into a single execution
	// (deploy run, workspace mount). Requires PDP + deploy approval.
	ClassRuntimeMount ReleaseClass = "runtime_mount"
	// ClassOperatorBreakGlass — incident access by a human operator;
	// requires two-person approval and is never visible to AI.
	ClassOperatorBreakGlass ReleaseClass = "operator_break_glass"
)

// validReleaseClass guards against typos at insert/release time. Mirror
// of the CHECK constraint in migration 00032.
func validReleaseClass(c ReleaseClass) bool {
	switch c {
	case ClassBuildTimeReference, ClassRuntimeMount, ClassOperatorBreakGlass:
		return true
	}
	return false
}

// Allowed release targets — these become the released_to column in the
// audit row and the dimension on the secret_release metric.
const (
	ReleaseToWorkspaceMount   = "workspace_mount"
	ReleaseToDeployProvider   = "deploy_provider"
	ReleaseToOperatorSession  = "operator_session"
)

func validReleaseTo(s string) bool {
	switch s {
	case ReleaseToWorkspaceMount, ReleaseToDeployProvider, ReleaseToOperatorSession:
		return true
	}
	return false
}

// SecretRef is the public, AI-safe handle to a secret. It carries
// everything needed to talk about the secret in prompts, dashboards,
// audit rows, and policy decisions — and nothing that would leak the
// value itself. The struct is intentionally serialisable.
type SecretRef struct {
	ID             string                 `json:"id"`
	TenantID       string                 `json:"tenantId"`
	ProjectID      string                 `json:"projectId,omitempty"`
	Name           string                 `json:"name"`
	Backend        Backend                `json:"backend"`
	BackendRef     string                 `json:"backendRef"`
	ReleaseClass   ReleaseClass           `json:"releaseClass"`
	Version        int                    `json:"version"`
	RotatedAt      *time.Time             `json:"rotatedAt,omitempty"`
	LastReleasedAt *time.Time             `json:"lastReleasedAt,omitempty"`
	Metadata       map[string]any         `json:"metadata,omitempty"`
	CreatedAt      time.Time              `json:"createdAt"`
}
