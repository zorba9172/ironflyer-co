package auditexport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"ironflyer/core/orchestrator/internal/operations/audit"
)

// recomputeHash recomputes the SHA-256 over the canonical JSON of
// the audit entry with ContentHash zeroed — the exact same algorithm
// internal/audit.hashEntry uses. We re-implement it here (instead of
// exporting it) so the auditor's verification path is intentionally
// independent of the writer's path: a future change to hashEntry that
// silently broke the chain would fail this verifier even though it
// "looked" intact via Store.Verify.
func recomputeHash(e audit.Entry) string {
	clone := e
	clone.ContentHash = ""
	raw, err := json.Marshal(clone)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
