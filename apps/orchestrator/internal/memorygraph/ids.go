package memorygraph

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// idSafe scrubs an arbitrary canonical id so it's safe to embed in a
// SurrealDB record id without quoting. We collapse anything that isn't
// alphanumeric or in the allow-list down to '_' so paths like
// "apps/web/src/foo.tsx" round-trip into a stable record id.
func idSafe(raw string) string {
	if raw == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '_', r == '-', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// hashed returns a stable short hash for long identifiers (file paths,
// nested symbol refs). Keeps record ids bounded without losing
// idempotence — the hash is deterministic in the canonical input.
func hashed(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:8])
}

// BuildNodeID composes the canonical "<kind>:<tenant>:<id>" SurrealDB
// record id used as the Node.ID. Both tenant and canonical id are
// scrubbed; long canonical ids fall back to a stable hash so traversal
// queries stay cheap.
func BuildNodeID(kind NodeKind, tenantID, canonicalID string) string {
	safeID := idSafe(canonicalID)
	if len(safeID) > 96 || safeID == "" || safeID != canonicalID {
		// Always append a hash when we had to scrub OR truncate so two
		// distinct canonicals can't collide on the same record id.
		if len(safeID) > 96 {
			safeID = safeID[:96]
		}
		safeID = safeID + "_" + hashed(canonicalID)
	}
	return string(kind) + ":" + idSafe(tenantID) + "_" + safeID
}

// BuildEdgeID composes the SurrealDB record id for an edge row. We
// derive it from kind+from+to so re-applying the same projection event
// upserts in place and never creates a duplicate edge.
func BuildEdgeID(kind EdgeKind, fromID, toID string) string {
	key := string(kind) + "|" + fromID + "|" + toID
	return string(kind) + ":" + hashed(key)
}

// SplitNodeID returns the kind segment and the remainder of a node id
// so a caller can route by kind without re-parsing the canonical ref.
// Returns ("", "", false) when the id doesn't match the expected shape.
func SplitNodeID(id string) (NodeKind, string, bool) {
	i := strings.IndexByte(id, ':')
	if i <= 0 || i == len(id)-1 {
		return "", "", false
	}
	return NodeKind(id[:i]), id[i+1:], true
}
