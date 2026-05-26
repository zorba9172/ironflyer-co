package auditexport

import (
	"encoding/csv"
	"encoding/json"
	"io"

	"ironflyer/core/orchestrator/internal/operations/audit"
)

// CSV column order is part of the exporter contract — downstream
// pipelines pin to column indices. NEVER reorder or remove columns;
// only append. The integration tests / customer dashboards rely on
// this exact rectangle:
//
//	id,timestamp,tenant_id,actor,action,outcome,prev_hash,hash,attrs_json
//
// `tenant_id` resolves to Attrs["tenantID"] when present, otherwise
// ProjectID, otherwise UserID — matching the same priority order
// filterByTenant uses.
var csvHeader = []string{
	"id",
	"timestamp",
	"tenant_id",
	"actor",
	"action",
	"outcome",
	"prev_hash",
	"hash",
	"attrs_json",
}

func writeCSV(out io.Writer, rows []audit.Entry, includeAttrs bool) error {
	w := csv.NewWriter(out)
	if err := w.Write(csvHeader); err != nil {
		return err
	}
	for _, r := range rows {
		rec := []string{
			r.ID,
			r.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z07:00"),
			resolveTenant(r),
			r.UserID,
			string(r.Action),
			string(r.Outcome),
			r.PrevHash,
			r.ContentHash,
			attrsField(r, includeAttrs),
		}
		if err := w.Write(rec); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func resolveTenant(e audit.Entry) string {
	if e.Attrs != nil {
		if v, ok := e.Attrs["tenantID"]; ok {
			if s, ok2 := v.(string); ok2 && s != "" {
				return s
			}
		}
		if v, ok := e.Attrs["tenant_id"]; ok {
			if s, ok2 := v.(string); ok2 && s != "" {
				return s
			}
		}
	}
	if e.ProjectID != "" {
		return e.ProjectID
	}
	return e.UserID
}

func attrsField(e audit.Entry, include bool) string {
	if !include || len(e.Attrs) == 0 {
		return ""
	}
	raw, err := json.Marshal(e.Attrs)
	if err != nil {
		return ""
	}
	return string(raw)
}
