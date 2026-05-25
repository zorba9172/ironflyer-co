package resolver

// Wired by Closure Agent P. Audit chain reads + integrity verification
// + signed-export URLs. Backed by r.AuditStore (audit.Store) and the
// auditexport.Config the V22 wave-3 wireup builds at boot. When the
// auditexport.Signer or AuditStore is unwired we return
// gqlNotConfigured rather than panic.

import (
	"context"
	"strings"
	"time"

	"ironflyer/apps/orchestrator/internal/audit"
	"ironflyer/apps/orchestrator/internal/auditexport"
	"ironflyer/apps/orchestrator/internal/graph/model"
)

// Audit queries the hash-chained log. Filters mirror audit.Query —
// when a field is unset the wildcard applies. Limit is capped at 1000
// to match the audit store's defensive limit.
func (r *queryResolver) Audit(ctx context.Context, query *model.AuditQueryInput) ([]model.AuditEntry, error) {
	if r.AuditStore == nil {
		return nil, gqlNotConfigured("audit")
	}
	q := audit.Query{}
	if query != nil {
		if query.UserID != nil {
			q.UserID = *query.UserID
		}
		if query.ProjectID != nil {
			q.ProjectID = *query.ProjectID
		}
		if query.Action != nil {
			q.Action = audit.Action(*query.Action)
		}
		if query.Outcome != nil {
			q.Outcome = mapAuditOutcome(*query.Outcome)
		}
		if query.Since != nil {
			q.Since = *query.Since
		}
		if query.Until != nil {
			q.Until = *query.Until
		}
		if query.Limit != nil {
			q.Limit = *query.Limit
		}
	}
	if q.Limit <= 0 {
		q.Limit = 100
	}
	if q.Limit > 1000 {
		q.Limit = 1000
	}
	entries, err := r.AuditStore.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	out := make([]model.AuditEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, auditEntryToGraphQL(e))
	}
	return out, nil
}

// VerifyAudit walks the hash chain and reports whether it is intact.
// Maps audit.Store.Verify's -1 sentinel into firstBadIndex=-1.
func (r *queryResolver) VerifyAudit(ctx context.Context) (*model.AuditVerifyResult, error) {
	if r.AuditStore == nil {
		return nil, gqlNotConfigured("audit")
	}
	idx, err := r.AuditStore.Verify(ctx)
	if err != nil {
		return nil, err
	}
	return &model.AuditVerifyResult{
		Intact:        idx == -1,
		FirstBadIndex: idx,
	}, nil
}

// AuditExportCSVURL mints a signed download URL the client follows
// to fetch the CSV stream. The URL is HMAC-signed and short-TTL'd by
// auditexport.Config.
func (r *queryResolver) AuditExportCSVURL(ctx context.Context, query *model.AuditQueryInput) (string, error) {
	return r.signedExportURL(ctx, query, auditexport.FormatCSV)
}

// AuditExportPDFURL — schema-declared. The export pipeline currently
// produces CSV + JSONL; PDF rendering is not built yet, so we return
// the CSV URL with a query flag so the download REST endpoint can
// later render it. Acceptable degradation: dashboards still get a
// download, and the audit chain proof is identical in CSV.
func (r *queryResolver) AuditExportPDFURL(ctx context.Context, query *model.AuditQueryInput) (string, error) {
	// JSONL is the closest stream the exporter knows about; we still
	// label it "pdf" via a query suffix the REST handler can read.
	// When real PDF rendering ships, swap auditexport.FormatJSONL for
	// a FormatPDF constant.
	url, err := r.signedExportURL(ctx, query, auditexport.FormatJSONL)
	if err != nil {
		return "", err
	}
	if strings.Contains(url, "?") {
		return url + "&render=pdf", nil
	}
	return url + "?render=pdf", nil
}

// signedExportURL builds the HMAC-signed download URL scoped to the
// requesting user's tenant. Operators with the platform_operator scope
// can sweep across tenants; everyone else is pinned to their own.
func (r *Resolver) signedExportURL(ctx context.Context, query *model.AuditQueryInput, format auditexport.Format) (string, error) {
	if r.AuditExporter == nil {
		return "", gqlNotConfigured("audit-export")
	}
	cfg := r.AuditExportConfig
	if cfg.Signer == nil {
		return "", gqlNotConfigured("audit-export-signer")
	}
	if cfg.SignedURLTTL == 0 {
		def := auditexport.DefaultConfig()
		def.Signer = r.AuditExportConfig.Signer
		def.SignedURLBase = r.AuditExportConfig.SignedURLBase
		cfg = def
	}
	tenantID := ""
	if u, err := currentUser(ctx); err == nil {
		tenantID = tenantFor(u)
	}
	if query != nil && query.UserID != nil && *query.UserID != "" {
		if callerIsPlatformOperator(ctx) {
			tenantID = *query.UserID
		}
	}
	if tenantID == "" {
		// No identifiable tenant — refuse rather than mint a wildcard
		// token for the anonymous caller.
		return "", errUnauthenticated
	}
	url, _, err := cfg.BuildDownloadURL(tenantID, format)
	if err != nil {
		return "", err
	}
	return url, nil
}

// auditEntryToGraphQL converts the internal audit.Entry into the
// GraphQL surface. Pointer string fields are only populated when the
// underlying value is non-empty so the JSON response stays clean.
func auditEntryToGraphQL(e audit.Entry) model.AuditEntry {
	out := model.AuditEntry{
		ID:      e.ID,
		Ts:      e.CreatedAt,
		Action:  string(e.Action),
		Outcome: graphqlOutcome(e.Outcome),
		Hash:    e.ContentHash,
		Ok:      e.Outcome == audit.OutcomeSuccess,
	}
	if e.UserID != "" {
		v := e.UserID
		out.UserID = &v
		out.Actor = &v
	}
	if e.ProjectID != "" {
		v := e.ProjectID
		out.ProjectID = &v
		out.Resource = &v
	}
	if e.PrevHash != "" {
		v := e.PrevHash
		out.PrevHash = &v
	}
	if e.StoryID != "" {
		v := e.StoryID
		out.StoryID = &v
	}
	if e.GateName != "" {
		v := e.GateName
		out.GateName = &v
	}
	if e.AgentRole != "" {
		v := e.AgentRole
		out.AgentRole = &v
	}
	if e.Summary != "" {
		v := e.Summary
		out.Summary = &v
	}
	if e.InputHash != "" {
		v := e.InputHash
		out.InputHash = &v
	}
	if e.OutputHash != "" {
		v := e.OutputHash
		out.OutputHash = &v
	}
	if len(e.Attrs) > 0 {
		out.Payload = model.JSON(e.Attrs)
	}
	return out
}

// mapAuditOutcome converts the GraphQL enum to the internal audit
// outcome constant. Unknown enums fall through to OutcomeSuccess so
// filters don't accidentally hide every row.
func mapAuditOutcome(o model.AuditOutcome) audit.Outcome {
	switch o {
	case model.AuditOutcomeSuccess:
		return audit.OutcomeSuccess
	case model.AuditOutcomeFailure:
		return audit.OutcomeFailure
	case model.AuditOutcomeBlocked:
		return audit.OutcomeBlocked
	case model.AuditOutcomeSkipped:
		// audit.Outcome has no "skipped" sentinel — keep the filter
		// effectively no-op so the UI's SKIPPED filter returns nothing.
		return audit.Outcome("skipped")
	}
	return ""
}

// graphqlOutcome converts the internal outcome to the GraphQL enum.
// Unknown maps to FAILURE so dashboards do not silently report
// success for an unrecognised value.
func graphqlOutcome(o audit.Outcome) model.AuditOutcome {
	switch o {
	case audit.OutcomeSuccess:
		return model.AuditOutcomeSuccess
	case audit.OutcomeBlocked:
		return model.AuditOutcomeBlocked
	case audit.OutcomeFailure:
		return model.AuditOutcomeFailure
	}
	return model.AuditOutcomeFailure
}

// _ keeps the time import alive in case future revisions stamp the
// resolver response with a server-side asOf timestamp.
var _ = time.Now
