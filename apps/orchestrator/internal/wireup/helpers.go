package wireup

import (
	"github.com/google/uuid"
)

// temporalTenantUUID mirrors the deterministic tenant→UUID mapping
// used by the resolver layer so ledger rows written by the Temporal
// worker land under the same key as rows written by HTTP requests.
func temporalTenantUUID(tenant string) uuid.UUID {
	if id, err := uuid.Parse(tenant); err == nil {
		return id
	}
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(tenant))
}

// temporalParseExecUUID parses an execution ID into a uuid.UUID for
// the ledger row's optional ExecutionID column. ok=false when the
// caller supplied a non-UUID identifier (e.g. legacy memory rows).
func temporalParseExecUUID(executionID string) (uuid.UUID, bool) {
	if executionID == "" {
		return uuid.Nil, false
	}
	if id, err := uuid.Parse(executionID); err == nil {
		return id, true
	}
	return uuid.Nil, false
}
