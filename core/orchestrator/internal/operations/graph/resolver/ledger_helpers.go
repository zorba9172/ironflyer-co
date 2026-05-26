package resolver

import (
	"ironflyer/core/orchestrator/internal/operations/graph/model"
	"ironflyer/core/orchestrator/internal/business/ledger"
)

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
			Metadata:       model.JSON(e.Metadata),
			CreatedAt:      e.CreatedAt,
		}
		if e.ExecutionID != nil {
			s := e.ExecutionID.String()
			entry.ExecutionID = &s
		}
		if e.Provider != "" {
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
