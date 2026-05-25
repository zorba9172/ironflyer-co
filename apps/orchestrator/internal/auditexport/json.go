package auditexport

import (
	"encoding/json"
	"io"

	"ironflyer/apps/orchestrator/internal/audit"
)

// jsonRow is the public NDJSON row shape. We mirror the CSV column
// names (snake_case) instead of audit.Entry's camelCase JSON tags so
// the two formats stay analytically identical. Attrs is emitted as a
// nested object so SIEM ingesters don't have to JSON-decode a string.
type jsonRow struct {
	ID        string         `json:"id"`
	Timestamp string         `json:"timestamp"`
	TenantID  string         `json:"tenant_id"`
	Actor     string         `json:"actor"`
	Action    string         `json:"action"`
	Outcome   string         `json:"outcome"`
	PrevHash  string         `json:"prev_hash,omitempty"`
	Hash      string         `json:"hash"`
	Attrs     map[string]any `json:"attrs,omitempty"`
}

func writeJSONL(out io.Writer, rows []audit.Entry, includeAttrs bool) error {
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	for _, r := range rows {
		row := jsonRow{
			ID:        r.ID,
			Timestamp: r.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z07:00"),
			TenantID:  resolveTenant(r),
			Actor:     r.UserID,
			Action:    string(r.Action),
			Outcome:   string(r.Outcome),
			PrevHash:  r.PrevHash,
			Hash:      r.ContentHash,
		}
		if includeAttrs {
			row.Attrs = r.Attrs
		}
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return nil
}
