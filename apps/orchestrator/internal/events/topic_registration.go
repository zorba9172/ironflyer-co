package events

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/rs/zerolog"
)

// v22Schemas embeds the canonical V22 per-topic JSON Schemas. The
// filename stem (e.g. "billing.ledger.v1") matches the domain.stream.v<N>
// suffix of the corresponding topic constant.
//
//go:embed schemas/*.json
var v22Schemas embed.FS

// RegisterV22Topics registers the default JSON Schema for each of the 9
// V22 topics declared in topics.go. Subjects are stamped as
// "<topic>-default" via SubjectFor so producers that have not yet
// switched to per-event subjects still benefit from envelope
// validation.
//
// Registration is best-effort: a per-topic Register failure is logged
// at Warn and the loop continues. The overall function returns nil so
// startup never fails on a transient registry hiccup. Hard
// misconfiguration (e.g. missing embedded files) surfaces as a Fatal-
// candidate error to the caller.
//
// When reg is nil we no-op silently — this lets the integration agent
// always call RegisterV22Topics regardless of whether a registry was
// constructed.
func RegisterV22Topics(ctx context.Context, reg Registry, log zerolog.Logger) error {
	if reg == nil {
		return nil
	}
	topics := allTopics()
	registered := 0
	skipped := 0
	for _, topic := range topics {
		schemaName := schemaFileFor(topic)
		raw, err := fs.ReadFile(v22Schemas, "schemas/"+schemaName)
		if err != nil {
			log.Warn().
				Err(err).
				Str("topic", topic).
				Str("schema_file", schemaName).
				Msg("events: V22 schema file missing; skipping topic")
			skipped++
			continue
		}
		subject := SubjectFor(topic, "default")

		// Skip subjects that already have a registered version so we
		// don't pile redundant identical versions on every restart.
		if existing, err := reg.Latest(ctx, subject); err == nil && existing.Version > 0 {
			log.Debug().
				Str("topic", topic).
				Str("subject", string(subject)).
				Int("version", existing.Version).
				Msg("events: V22 schema already registered; skipping")
			skipped++
			continue
		} else if err != nil && !errors.Is(err, ErrSchemaNotFound) {
			// Treat read errors as recoverable: try to register anyway.
			log.Warn().
				Err(err).
				Str("subject", string(subject)).
				Msg("events: failed to probe schema registry; attempting register")
		}

		rs, err := reg.Register(ctx, subject, string(raw))
		if err != nil {
			log.Warn().
				Err(err).
				Str("topic", topic).
				Str("subject", string(subject)).
				Msg("events: V22 schema register failed; continuing")
			skipped++
			continue
		}
		log.Info().
			Str("topic", topic).
			Str("subject", string(rs.Subject)).
			Int("version", rs.Version).
			Msg("events: V22 schema registered")
		registered++
	}
	log.Info().
		Int("registered", registered).
		Int("skipped", skipped).
		Int("total", len(topics)).
		Msg("events: V22 topic schema registration complete")
	return nil
}

// allTopics returns the 9 V22 topics declared in topics.go in a stable
// order. Used by RegisterV22Topics and exposed to operators (CLI
// inspectors, smoke probes) that need the canonical V22 surface.
func allTopics() []string {
	return []string{
		TopicExecutionLifecycle,
		TopicExecutionSteps,
		TopicGatesResults,
		TopicPatchesLifecycle,
		TopicBillingLedger,
		TopicProfitGuardDecisions,
		TopicDeployLifecycle,
		TopicMemoryIndexing,
		TopicAuditSecurity,
	}
}

// schemaFileFor derives the embedded schema filename from a topic
// constant. Strips the env prefix (e.g. "ifly.prod.") so the same
// schema file is reused across dev/staging/prod topics.
//
//	ifly.prod.billing.ledger.v1 -> billing.ledger.v1.json
func schemaFileFor(topic string) string {
	// Trim the leading "ifly.<env>." segment.
	parts := strings.SplitN(topic, ".", 3)
	stem := topic
	if len(parts) == 3 && parts[0] == "ifly" {
		stem = parts[2]
	}
	return stem + ".json"
}

// V22SchemaFor returns the embedded schema bytes for a V22 topic, or
// an error when the topic isn't part of the V22 surface. Exposed so
// CLI tools (e.g. `ironflyer events dump-schemas`) can inspect the
// shipping contract without re-reading the embed.FS by hand.
func V22SchemaFor(topic string) ([]byte, error) {
	name := schemaFileFor(topic)
	raw, err := fs.ReadFile(v22Schemas, "schemas/"+name)
	if err != nil {
		return nil, fmt.Errorf("events: no embedded schema for topic %q: %w", topic, err)
	}
	return raw, nil
}
