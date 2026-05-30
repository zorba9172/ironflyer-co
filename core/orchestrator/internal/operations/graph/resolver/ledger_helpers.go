package resolver

import (
	"strings"

	"ironflyer/core/orchestrator/internal/operations/graph/model"
	"ironflyer/core/orchestrator/internal/business/ledger"
)

// isLLMEntryType reports whether a ledger entry's cost came from an LLM call —
// the only legs whose `provider` is a model vendor we must never name.
func isLLMEntryType(t ledger.EntryType) bool {
	switch string(t) {
	case "provider_inference_cost", "premium_reasoning_charge":
		return true
	default:
		return false
	}
}

// scrubLedgerMetadata drops any vendor/model keys a writer may have stamped into
// a ledger row's metadata, so the JSON passthrough can't leak provider identity.
func scrubLedgerMetadata(m map[string]any) map[string]any {
	if len(m) == 0 {
		return m
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "model", "provider", "recommended_provider", "recommendedprovider", "preferred_provider":
			continue
		}
		out[k] = v
	}
	return out
}

// ledgerRowsToGraphQL maps the internal Entry shape onto the GraphQL
// WalletLedgerEntry model. Pulled out so both list resolvers map
// identically.
func ledgerRowsToGraphQL(rows []ledger.Entry) []model.WalletLedgerEntry {
	out := make([]model.WalletLedgerEntry, 0, len(rows))
	for _, e := range rows {
		entry := model.WalletLedgerEntry{
			ID:             e.ID.String(),
			TenantID:       e.TenantID.String(),
			EntryType:      string(e.EntryType),
			Direction:      string(e.Direction),
			AmountUsd:      floatOfDecimal(e.AmountUSD),
			Billable:       e.Billable,
			MarginRelevant: e.MarginRelevant,
			Metadata:       model.JSON(scrubLedgerMetadata(e.Metadata)),
			CreatedAt:      e.CreatedAt,
		}
		if e.ExecutionID != nil {
			s := e.ExecutionID.String()
			entry.ExecutionID = &s
		}
		// Provider is shown only for non-LLM legs (payments / runtime). The LLM
		// inference vendor is never named to the user — the entry_type already
		// says what the charge was for.
		if e.Provider != "" && !isLLMEntryType(e.EntryType) {
			p := e.Provider
			entry.Provider = &p
		}
		out = append(out, entry)
	}
	return out
}

// itoaInt64 is a tiny shim that avoids pulling strconv into a file
// that uses it once.
func itoaInt64(n int64) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = digits[n%10]
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
