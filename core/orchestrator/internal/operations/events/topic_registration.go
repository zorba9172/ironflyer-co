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

// EventTypesByTopic enumerates every per-event subject the producers
// stamp via outboxhooks/WriteEventInTx. The schema-registry registration
// loop walks this matrix and registers "<topic>-<eventType>" subjects
// so the producer side never logs "schema subject not registered" for
// the known V22 surface. New eventType strings MUST land here at the
// same time they land in the producer call site.
//
// Domain → eventType strings are derived from:
//   - core/orchestrator/internal/outboxhooks/outboxhooks.go
//   - core/orchestrator/internal/ledger/postgres.go (ledgerEvent)
//   - core/orchestrator/internal/wallet/postgres.go (emitWalletEvent)
//   - core/orchestrator/internal/execution/lifecycle.go (execution.settled.v1)
//   - core/orchestrator/internal/profitguard/store.go (profitguard.decision.v1)
//   - core/orchestrator/internal/finisher/gates_security.go (gate.security.finding.v1)
//   - core/orchestrator/internal/audit/events.go (mandatory audit set)
func EventTypesByTopic() map[string][]string {
	return map[string][]string{
		// execution.lifecycle
		"execution.lifecycle.v1": {
			"execution.admitted.v1",
			"execution.started.v1",
			"execution.settled.v1",
			"execution.failed.v1",
			"execution.cancelled.v1",
			"execution.refunded.v1",
		},
		// execution.steps
		"execution.steps.v1": {
			"execution.step.started.v1",
			"execution.step.completed.v1",
			"execution.step.failed.v1",
		},
		// gates.results
		"gates.results.v1": {
			"gate.result.v1",
			"gate.verdict.v1",
			"gate.waiver.v1",
			"gate.security.finding.v1",
		},
		// patches.lifecycle
		"patches.lifecycle.v1": {
			"patch.proposed.v1",
			"patch.previewed.v1",
			"patch.approved.v1",
			"patch.applied.v1",
			"patch.rolled_back.v1",
		},
		// billing.ledger
		"billing.ledger.v1": {
			// wallet emitter
			"wallet.topup.v1",
			"wallet.hold.v1",
			"wallet.release.v1",
			"wallet.debit.v1",
			"wallet.refund.v1",
			// ledger.<entry_type>.v1 family (kept aligned with budget.EntryType)
			"ledger.credit_reservation.v1",
			"ledger.credit_release.v1",
			"ledger.provider_cost.v1",
			"ledger.sandbox_cost.v1",
			"ledger.storage_cost.v1",
			"ledger.deployment_cost.v1",
			"ledger.revenue.v1",
			"ledger.refund.v1",
			"ledger.wallet_topup.v1",
			// outboxhooks.BillingLedgerEvent emits "billing.ledger.<type>.v1"
			"billing.ledger.wallet_topup.v1",
			"billing.ledger.refund.v1",
			"billing.ledger.revenue.v1",
			"billing.ledger.provider_cost.v1",
			"billing.ledger.sandbox_cost.v1",
			"billing.ledger.storage_cost.v1",
			"billing.ledger.deployment_cost.v1",
		},
		// profitguard.decisions
		"profitguard.decisions.v1": {
			"profitguard.decision.v1",
		},
		// deploy.lifecycle
		"deploy.lifecycle.v1": {
			"deploy.plan.v1",
			"deploy.approval.v1",
			"deploy.provider_action.v1",
			"deploy.smoke_result.v1",
			"deploy.rollback.v1",
		},
		// memory.indexing
		"memory.indexing.v1": {
			"memory.indexed.v1",
			"memory.evicted.v1",
		},
		// audit.security
		"audit.security.v1": {
			"auth.lifecycle.v1",
			"auth.session_change.v1",
			"graphql.high_risk_mutation.v1",
			"graphql.policy_deny.v1",
			"policy.high_risk_allow.v1",
			"policy.deny.v1",
			"provider.dispatch.v1",
			"workspace.command_exec.v1",
			"secret.ref_write.v1",
			"secret.release.v1",
			"secret.rotation.v1",
			"secret.release_deny.v1",
			"operator.break_glass.v1",
			"abuse.escalation.v1",
			"abuse.throttle.v1",
			"abuse.suspension.v1",
		},
	}
}

// envTopicFor builds the env-prefixed topic name for one of the
// canonical domain.stream.v1 stems used by EventTypesByTopic. It mirrors
// TopicFor("<env>", domain, stream, version) but takes the bare stem so
// the matrix is independent of the static "ifly.prod.*" constants.
func envTopicFor(env, stem string) string {
	if env == "" {
		env = CurrentEnv()
	}
	return "ifly." + env + "." + stem
}

// RegisterV22Topics registers the default JSON Schema for each of the 9
// V22 topics declared in topics.go. Subjects are stamped as
// "<topic>-default" via SubjectFor and ALSO as "<topic>-<event_type>"
// for every known producer eventType per EventTypesByTopic so the
// outbox hook never logs "schema subject not registered" for the known
// V22 producer surface.
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
	env := CurrentEnv()
	matrix := EventTypesByTopic()
	registered := 0
	skipped := 0

	// 1) Default per-topic subject: <topic>-default with the V22
	// envelope schema. Walked against the canonical "ifly.prod" topic
	// constants AND the env-prefixed runtime topic so a dev cluster
	// stops complaining when producers stamp env-aware topics.
	for _, topic := range allTopics() {
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

		runtimeTopic := envTopicFor(env, schemaStemFor(topic))
		for _, t := range uniqueTopics(topic, runtimeTopic) {
			subject := SubjectFor(t, "default")
			if r, s := registerOne(ctx, reg, subject, string(raw), log); r {
				registered++
			} else if s {
				skipped++
			}
		}
	}

	// 2) Per-event subjects: <topic>-<event_type> against the runtime
	// (env-aware) topic. The producers stamp event-type-specific
	// subjects via outboxhooks.WriteEventInTx so the registry must too.
	//
	// Per-event subjects intentionally use the envelope schema (3
	// required fields: event_id, tenant_id, occurred_at) — not the
	// richer per-topic schema. Topic-level schemas (e.g. billing.ledger
	// requires `entry_type, direction, amount_usd`) are correct for
	// ledger rows but reject sibling event types in the same domain
	// (wallet.topup/.hold/.release/.debit/.refund don't carry
	// `entry_type`). Per-event payload-shape validation will land in a
	// future pass that ships per-eventType schemas; until then the
	// envelope is the V22 contract.
	envelope := EnvelopeSchema
	for stem, eventTypes := range matrix {
		runtimeTopic := envTopicFor(env, stem)
		for _, et := range eventTypes {
			subject := SubjectFor(runtimeTopic, et)
			if r, s := registerOne(ctx, reg, subject, envelope, log); r {
				registered++
			} else if s {
				skipped++
			}
		}
	}

	log.Info().
		Int("registered", registered).
		Int("skipped", skipped).
		Str("env", env).
		Msg("events: V22 topic schema registration complete")
	return nil
}

// registerOne registers a single subject, returning (registered, skipped).
// Idempotent: an already-present subject increments skipped and logs at
// Debug rather than Info so restart traffic stays quiet.
func registerOne(ctx context.Context, reg Registry, subject Subject, schemaJSON string, log zerolog.Logger) (bool, bool) {
	if existing, err := reg.Latest(ctx, subject); err == nil && existing.Version > 0 {
		log.Debug().
			Str("subject", string(subject)).
			Int("version", existing.Version).
			Msg("events: schema already registered; skipping")
		return false, true
	} else if err != nil && !errors.Is(err, ErrSchemaNotFound) {
		log.Warn().
			Err(err).
			Str("subject", string(subject)).
			Msg("events: failed to probe schema registry; attempting register")
	}
	rs, err := reg.Register(ctx, subject, schemaJSON)
	if err != nil {
		log.Warn().
			Err(err).
			Str("subject", string(subject)).
			Msg("events: schema register failed; continuing")
		return false, true
	}
	log.Debug().
		Str("subject", string(rs.Subject)).
		Int("version", rs.Version).
		Msg("events: schema registered")
	return true, false
}

// schemaStemFor strips the "ifly.<env>." prefix from a fully-stamped
// topic constant so we can re-derive the bare "domain.stream.vN" stem
// used by both the embedded schema filename and EventTypesByTopic.
func schemaStemFor(topic string) string {
	parts := strings.SplitN(topic, ".", 3)
	if len(parts) == 3 && parts[0] == "ifly" {
		return parts[2]
	}
	return topic
}

// uniqueTopics dedupes two topic strings while preserving order.
func uniqueTopics(a, b string) []string {
	if a == b || b == "" {
		return []string{a}
	}
	return []string{a, b}
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
