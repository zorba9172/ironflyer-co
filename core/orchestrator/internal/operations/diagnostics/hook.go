package diagnostics

import (
	"strings"
	"time"

	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/operations/logctx"
)

// ZerologHook implements zerolog.Hook by funnelling every WARN+ event
// into the Ring buffer. The hook is intentionally minimal: it captures
// what zerolog hands it (level + message) plus the ids the orchestrator
// stamps on every ctx-aware log call (request_id, tenant_id,
// execution_id via logctx).
//
// The Event struct does not expose its accumulated fields, so the hook
// cannot reconstruct arbitrary structured pairs. Operators get the
// log message + the ctx ids, which is enough to find the matching
// canonical log line in stdout for full detail.
type ZerologHook struct {
	ring *Ring
}

// NewZerologHook returns a hook that appends WARN-and-worse events to
// the supplied ring buffer. INFO and below are dropped so the ring
// stays focused on actionable signal.
func NewZerologHook(ring *Ring) *ZerologHook {
	return &ZerologHook{ring: ring}
}

// Run implements zerolog.Hook. It MUST be non-blocking; the ring's
// Append uses a TryLock so contention drops the entry instead of
// stalling the log call.
func (h *ZerologHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	if h == nil || h.ring == nil {
		return
	}
	if level < zerolog.WarnLevel {
		return
	}
	entry := Entry{
		Time:    time.Now().UTC(),
		Level:   strings.ToLower(level.String()),
		Message: msg,
	}
	if ctx := e.GetCtx(); ctx != nil {
		entry.RequestID = logctx.RequestID(ctx)
		entry.TenantID = logctx.TenantID(ctx)
		entry.ExecutionID = logctx.ExecutionID(ctx)
	}
	h.ring.Append(entry)
}
