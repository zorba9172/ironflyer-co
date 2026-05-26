package wireup

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	surrealdb "github.com/surrealdb/surrealdb.go"

	"ironflyer/core/orchestrator/internal/operations/events"
	"ironflyer/core/orchestrator/internal/ai/memorygraph"
)

// MemoryGraphResult bundles the graph, writer, and retriever produced
// by BuildMemoryGraph. The retriever feeds the finisher's context
// builder; the writer subscribes to outbox/event publication.
type MemoryGraphResult struct {
	Graph     memorygraph.Graph
	Writer    *memorygraph.Writer
	Retriever *memorygraph.GraphRetriever
}

// BuildMemoryGraph picks between SurrealDB (when surrealDB is wired)
// and the in-process MemoryGraph (dev default). Bootstrap is best-
// effort — failure logs a warning and falls back to the memory graph.
//
// To stream events into the writer, call AttachMemoryGraphWriter
// after the PublisherDaemon is constructed. Both can be passed at
// once via BuildMemoryGraphWithPublisher when the integration site
// has both in scope.
func BuildMemoryGraph(ctx context.Context, surrealDB *surrealdb.DB, log zerolog.Logger) MemoryGraphResult {
	var g memorygraph.Graph
	if surrealDB != nil {
		sg := memorygraph.NewSurrealGraph(surrealDB)
		if err := sg.Bootstrap(ctx); err != nil {
			log.Warn().Err(err).Msg("memorygraph: surreal bootstrap failed; falling back to memory")
			g = memorygraph.NewMemoryGraph()
		} else {
			g = sg
			log.Info().Msg("memorygraph: surreal backend enabled")
		}
	} else {
		mg := memorygraph.NewMemoryGraph()
		_ = mg.Bootstrap(ctx)
		g = mg
		log.Info().Msg("memorygraph: in-process backend enabled")
	}
	writer := memorygraph.NewWriter(g, log.With().Str("svc", "memorygraph-writer").Logger())
	retriever := memorygraph.NewGraphRetriever(g)
	return MemoryGraphResult{Graph: g, Writer: writer, Retriever: retriever}
}

// BuildMemoryGraphWithPublisher is the preferred integration entry
// point: it builds the graph + writer + retriever, then registers the
// writer as an Observer on the publisher daemon so projection fan-out
// rides the same event flow as Redpanda fan-out.
//
// The daemon may be nil (e.g. when REDPANDA_BROKERS is unset). In
// that case the writer is still returned but no events reach it until
// AttachMemoryGraphWriter is called later with a live daemon.
func BuildMemoryGraphWithPublisher(
	ctx context.Context,
	surrealDB *surrealdb.DB,
	daemon *events.PublisherDaemon,
	log zerolog.Logger,
) MemoryGraphResult {
	res := BuildMemoryGraph(ctx, surrealDB, log)
	AttachMemoryGraphWriter(daemon, res.Writer, log)
	return res
}

// AttachMemoryGraphWriter wires the MemoryGraph writer into the
// publisher daemon's Observer slot. Each successfully published outbox
// event is translated into a memorygraph.Event and handed to the
// writer. Projection failures are logged at Warn and never roll back
// the canonical execution per the V22 architecture rule.
//
// Both arguments may be nil — this is a best-effort wire-up so the
// integration site can call it unconditionally.
func AttachMemoryGraphWriter(
	daemon *events.PublisherDaemon,
	writer *memorygraph.Writer,
	log zerolog.Logger,
) {
	if daemon == nil || writer == nil {
		return
	}
	mgLog := log.With().Str("svc", "memorygraph-observer").Logger()
	daemon.SetObserver(func(ctx context.Context, e events.Event) {
		payload := map[string]any{}
		for k, v := range e.Payload {
			payload[k] = v
		}
		graphEvent := memorygraph.Event{
			Kind:    e.Type,
			Payload: payload,
			Provenance: memorygraph.Provenance{
				SourceEventID:   e.ID.String(),
				SourceEventType: e.Type,
				RecordedAt:      time.Now().UTC(),
			},
		}
		if err := writer.Handle(ctx, graphEvent); err != nil {
			mgLog.Warn().Err(err).
				Str("event_type", e.Type).
				Str("event_id", e.ID.String()).
				Msg("memorygraph writer failed")
		}
	})
	mgLog.Info().Msg("memorygraph writer attached to outbox publisher observer")
}
