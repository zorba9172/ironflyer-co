// Package auditexport realises 90-day-plan item "enterprise audit
// export": a tenant-scoped, hash-chain-verifiable dump of the
// internal/audit log surfaced both as a streaming download (CSV or
// JSONL) and as a chain-of-custody proof that the tenant's audit
// window has not been tampered with.
//
// The exporter is intentionally thin: it reads from an audit.Store
// (the canonical hash-chained log defined in internal/audit/audit.go)
// and emits the canonical, externally-observable rectangle of fields.
// All tenant scoping is enforced at filter-construction time — the
// exporter refuses an empty TenantID so an operator query without an
// explicit "platform_operator" intent cannot accidentally drain the
// whole log.
package auditexport

import (
	"context"
	"io"
	"time"

	"ironflyer/apps/orchestrator/internal/audit"
)

// Format is the wire format the streamer emits. Two values intentional —
// CSV is the lingua franca for compliance teams that load exports into
// Excel / Splunk; JSONL keeps attrs as structured JSON for SIEM
// ingestion.
type Format string

const (
	FormatCSV   Format = "csv"
	FormatJSONL Format = "jsonl"
)

// Filter is the scoped query the exporter accepts. TenantID is
// mandatory — Stream / ChainProof return ErrTenantRequired when it is
// empty. The audit core does not model tenants directly; we map the
// filter onto audit.Query by treating TenantID as ProjectID when the
// caller wants per-project tenancy, and matching the Attrs["tenantID"]
// envelope when set. Operators who legitimately need to span tenants
// (e.g. platform-wide compliance) must explicitly opt in by setting
// TenantID = TenantWildcard.
type Filter struct {
	TenantID     string
	Since        time.Time
	Until        time.Time
	EventTypes   []string
	Format       Format
	IncludeAttrs bool
}

// TenantWildcard is the sentinel a platform_operator-scoped caller
// supplies when it deliberately needs to dump every tenant. The
// resolver layer is the gatekeeper that ensures only operators may
// pass this sentinel.
const TenantWildcard = "*"

// ChainProof is the verifiable summary the auditor produces for a
// caller. Verified=true means every entry in the window's hash chain
// reproduces; BrokenLinks lists each entry whose recorded PrevHash
// disagrees with the prior entry's ContentHash.
type ChainProof struct {
	From        time.Time
	To          time.Time
	EntryCount  int
	StartHash   string
	EndHash     string
	Verified    bool
	BrokenLinks []BrokenLink
}

// BrokenLink names a single chain inconsistency. AtEntryID identifies
// the entry whose PrevHash does not equal the prior entry's
// ContentHash (or whose own ContentHash failed recomputation).
type BrokenLink struct {
	AtEntryID        string
	ExpectedPrevHash string
	ActualPrevHash   string
}

// Exporter is the canonical surface. Implementations must:
//   - reject empty Filter.TenantID with ErrTenantRequired
//   - stream entries in chronological order (oldest first) so the
//     downstream consumer can fold the hash chain in one pass
//   - never include the audit Attrs map when Filter.IncludeAttrs is
//     false — attrs frequently contain sensitive ProfitGuard /
//     billing detail the customer may have redaction policies on
type Exporter interface {
	Stream(ctx context.Context, out io.Writer, filter Filter) error
	ChainProof(ctx context.Context, from, to time.Time) (ChainProof, error)
}

// StoreExporter is the in-process Exporter that walks an audit.Store.
// It is dependency-injected with a Store rather than constructing one
// itself so the operator can swap a Postgres / SurrealDB / object-store
// backed Store without touching this package.
type StoreExporter struct {
	Store audit.Store
	// MaxEntries caps a single Stream call so a runaway filter cannot
	// exhaust the orchestrator's memory. Defaults to 100k when zero.
	MaxEntries int
}

// NewStoreExporter is the constructor used by main.go (via the
// integration agent). Caller supplies a Store — we never construct one
// here so the audit chain identity remains owned by internal/audit.
func NewStoreExporter(s audit.Store) *StoreExporter {
	return &StoreExporter{Store: s, MaxEntries: 100_000}
}

// Stream writes entries in the requested format to out. Tenant scope
// enforcement and event-type whitelist filtering happen here so format
// implementations stay pure encoders.
func (e *StoreExporter) Stream(ctx context.Context, out io.Writer, f Filter) error {
	if err := f.validate(); err != nil {
		return err
	}
	entries, err := e.fetch(ctx, f)
	if err != nil {
		return err
	}
	switch f.Format {
	case FormatJSONL:
		return writeJSONL(out, entries, f.IncludeAttrs)
	default:
		// CSV is the default — least surprising for compliance teams.
		return writeCSV(out, entries, f.IncludeAttrs)
	}
}

// ChainProof replays the chain in the [from,to] window and reports any
// broken links. The proof intentionally trusts no caller-supplied
// hashes — it recomputes each entry's hash from the canonical fields
// and compares against the stored ContentHash.
func (e *StoreExporter) ChainProof(ctx context.Context, from, to time.Time) (ChainProof, error) {
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return ChainProof{}, ErrInvalidWindow
	}
	q := audit.Query{Since: from, Until: to, Limit: maxInt(e.MaxEntries, 100_000)}
	rows, err := e.Store.Query(ctx, q)
	if err != nil {
		return ChainProof{}, err
	}
	// audit.MemoryStore.Query returns newest-first; the chain walk needs
	// chronological order so PrevHash links resolve correctly.
	sortChronological(rows)
	proof := ChainProof{
		From:       from,
		To:         to,
		EntryCount: len(rows),
		Verified:   true,
	}
	if len(rows) == 0 {
		return proof, nil
	}
	proof.StartHash = rows[0].ContentHash
	proof.EndHash = rows[len(rows)-1].ContentHash

	prev := ""
	for i, row := range rows {
		// The first entry's PrevHash is whatever the store recorded —
		// we don't know the predecessor outside the window, so we
		// accept the recorded PrevHash as the prologue and chain
		// forward from there.
		if i == 0 {
			prev = row.ContentHash
			if recomputed := recomputeHash(row); recomputed != row.ContentHash {
				proof.Verified = false
				proof.BrokenLinks = append(proof.BrokenLinks, BrokenLink{
					AtEntryID:        row.ID,
					ExpectedPrevHash: recomputed,
					ActualPrevHash:   row.ContentHash,
				})
			}
			continue
		}
		if row.PrevHash != prev {
			proof.Verified = false
			proof.BrokenLinks = append(proof.BrokenLinks, BrokenLink{
				AtEntryID:        row.ID,
				ExpectedPrevHash: prev,
				ActualPrevHash:   row.PrevHash,
			})
		}
		if recomputed := recomputeHash(row); recomputed != row.ContentHash {
			proof.Verified = false
			proof.BrokenLinks = append(proof.BrokenLinks, BrokenLink{
				AtEntryID:        row.ID,
				ExpectedPrevHash: row.ContentHash,
				ActualPrevHash:   recomputed,
			})
		}
		prev = row.ContentHash
	}
	return proof, nil
}

// fetch applies the tenant + event-type + time filters on top of the
// audit.Store query API. The Store contract does not understand
// tenant or event-type natively, so we post-filter in process.
func (e *StoreExporter) fetch(ctx context.Context, f Filter) ([]audit.Entry, error) {
	limit := e.MaxEntries
	if limit <= 0 {
		limit = 100_000
	}
	q := audit.Query{
		Since: f.Since,
		Until: f.Until,
		Limit: limit,
	}
	rows, err := e.Store.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	sortChronological(rows)

	if f.TenantID != TenantWildcard {
		rows = filterByTenant(rows, f.TenantID)
	}
	if len(f.EventTypes) > 0 {
		rows = filterByEventTypes(rows, f.EventTypes)
	}
	if !f.IncludeAttrs {
		// Defensive: blank Attrs so the encoder cannot accidentally
		// leak structured detail when IncludeAttrs is false.
		for i := range rows {
			rows[i].Attrs = nil
		}
	}
	return rows, nil
}

func (f Filter) validate() error {
	if f.TenantID == "" {
		return ErrTenantRequired
	}
	if !f.Since.IsZero() && !f.Until.IsZero() && f.Until.Before(f.Since) {
		return ErrInvalidWindow
	}
	switch f.Format {
	case "", FormatCSV, FormatJSONL:
		return nil
	}
	return ErrUnknownFormat
}

// filterByTenant matches Attrs["tenantID"] (string) OR ProjectID OR
// UserID against the supplied tenant. The audit core stores tenant
// scope opportunistically through Attrs, so we accept any of those
// placements.
func filterByTenant(rows []audit.Entry, tenant string) []audit.Entry {
	out := rows[:0]
	for _, r := range rows {
		if matchesTenant(r, tenant) {
			out = append(out, r)
		}
	}
	return out
}

func matchesTenant(e audit.Entry, tenant string) bool {
	if e.ProjectID == tenant || e.UserID == tenant {
		return true
	}
	if e.Attrs != nil {
		if v, ok := e.Attrs["tenantID"]; ok {
			if s, ok2 := v.(string); ok2 && s == tenant {
				return true
			}
		}
		if v, ok := e.Attrs["tenant_id"]; ok {
			if s, ok2 := v.(string); ok2 && s == tenant {
				return true
			}
		}
	}
	return false
}

func filterByEventTypes(rows []audit.Entry, types []string) []audit.Entry {
	allow := make(map[string]struct{}, len(types))
	for _, t := range types {
		allow[t] = struct{}{}
	}
	out := rows[:0]
	for _, r := range rows {
		if _, ok := allow[string(r.Action)]; ok {
			out = append(out, r)
			continue
		}
		// Also match an Attrs["event"] envelope for V22 event names.
		if r.Attrs != nil {
			if v, ok := r.Attrs["event"]; ok {
				if s, ok2 := v.(string); ok2 {
					if _, allowed := allow[s]; allowed {
						out = append(out, r)
					}
				}
			}
		}
	}
	return out
}

func sortChronological(rows []audit.Entry) {
	// Insertion-ordered when coming from audit.MemoryStore.Query,
	// which returns newest-first; reverse in place.
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
