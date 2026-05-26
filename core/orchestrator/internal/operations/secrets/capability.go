package secrets

import "time"

// Capability is the short-lived, signed-by-the-broker permission token
// that downstream consumers (deploy adapter, workspace mounter, the
// operator break-glass surface) present back to the broker in order to
// fetch raw material via Resolve.
//
// Hard invariants:
//   - Capability NEVER carries the raw secret value. The Resolve call
//     loads from the backend on demand under audit.
//   - Capability is single-purpose: releasedTo, scope, and expiresAt
//     are all stamped at Release time and used by Resolve to enforce
//     "this token is for this consumer, this scope, this short window".
//   - Capability.RedactionProof is sha256("<value>") computed at
//     Release time so the audit chain can prove later that "the
//     bytes a deploy adapter actually used hashed to this digest"
//     without ever storing the bytes.
type Capability struct {
	ID               string       `json:"id"`            // cap_<uuid>
	SecretRefID      string       `json:"secretRefId"`
	Name             string       `json:"name"`          // safe to log
	ReleaseClass     ReleaseClass `json:"releaseClass"`
	ReleasedTo       string       `json:"releasedTo"`
	ExpiresAt        time.Time    `json:"expiresAt"`
	PolicyDecisionID string       `json:"policyDecisionId"`
	Scope            ReleaseScope `json:"scope"`
	RedactionProof   string       `json:"redactionProof"` // "sha256:<hex>" or "sha256:redacted"
}

// ReleaseScope narrows a Capability to a specific delegation context.
// All fields are optional individually but at least one must be set
// for a runtime release — the broker uses the scope to (a) write the
// audit row, (b) reject capabilities that wander out of their context.
type ReleaseScope struct {
	ExecutionID  string `json:"executionId,omitempty"`
	WorkspaceID  string `json:"workspaceId,omitempty"`
	DeployTarget string `json:"deployTarget,omitempty"`
}

// Expired reports whether the capability is past its TTL.
func (c Capability) Expired(now time.Time) bool {
	return !c.ExpiresAt.IsZero() && now.After(c.ExpiresAt)
}
