package events

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Topic taxonomy. The canonical shape is documented in
// docs/ARCHITECTURE_EVENTS.md and reproduced here so the producer
// side never strays from the spec.
//
//	ifly.<env>.<domain>.<stream>.v<major>
//
// Topics below are stamped at the "prod" envelope so the symbol names
// are short and stable across the producer call sites. When the
// orchestrator runs in dev/staging the topic prefix is derived from
// IRONFLYER_ENV via TopicFor — producers that want env-aware names
// call TopicFor("execution", "lifecycle", 1) instead of the const.
const (
	TopicExecutionLifecycle   = "ifly.prod.execution.lifecycle.v1"
	TopicExecutionSteps       = "ifly.prod.execution.steps.v1"
	TopicGatesResults         = "ifly.prod.gates.results.v1"
	TopicPatchesLifecycle     = "ifly.prod.patches.lifecycle.v1"
	TopicBillingLedger        = "ifly.prod.billing.ledger.v1"
	TopicProfitGuardDecisions = "ifly.prod.profitguard.decisions.v1"
	TopicDeployLifecycle      = "ifly.prod.deploy.lifecycle.v1"
	TopicMemoryIndexing       = "ifly.prod.memory.indexing.v1"
	TopicAuditSecurity        = "ifly.prod.audit.security.v1"
)

// AllowedEnvs is the closed set of environment prefixes a topic may
// carry. Anything else is a producer bug — the publisher MUST refuse
// to enqueue topics that fall outside this set so a stray "ifly.test"
// or "ifly." never reaches Redpanda.
var AllowedEnvs = []string{"dev", "staging", "prod"}

// topicRe is the canonical regex for ifly.<env>.<domain>.<stream>.v<N>.
// Major version is one or more digits and must be >= 1; v0 is rejected
// so contract evolution is always a deliberate bump from v1.
var topicRe = regexp.MustCompile(`^ifly\.(dev|staging|prod)\.([a-z][a-z0-9_]*)\.([a-z][a-z0-9_]*)\.v([1-9][0-9]*)$`)

// CurrentEnv reads IRONFLYER_ENV and clamps to the allowed set,
// defaulting to "dev" when unset or invalid. Centralising this keeps
// every producer on the same env literal so cross-env replay never
// fans out by accident.
func CurrentEnv() string {
	e := strings.TrimSpace(strings.ToLower(os.Getenv("IRONFLYER_ENV")))
	if e == "" {
		return "dev"
	}
	for _, ok := range AllowedEnvs {
		if e == ok {
			return e
		}
	}
	return "dev"
}

// TopicFor stamps a topic name. env is normalised through CurrentEnv
// when empty so callers that don't care can pass "" and inherit the
// process default.
func TopicFor(env, domain, stream string, version int) string {
	if env == "" {
		env = CurrentEnv()
	}
	if version < 1 {
		version = 1
	}
	return fmt.Sprintf("ifly.%s.%s.%s.v%d", env, domain, stream, version)
}

// ValidateTopic enforces the ifly.<env>.<domain>.<stream>.v<N> shape.
// Producers call this before enqueueing so a malformed topic name is
// rejected at the Postgres outbox layer rather than at Redpanda where
// the partition router would silently drop or panic.
func ValidateTopic(name string) error {
	if name == "" {
		return fmt.Errorf("events: topic is required")
	}
	if !topicRe.MatchString(name) {
		return fmt.Errorf("events: topic %q does not match ifly.<env>.<domain>.<stream>.v<N>", name)
	}
	return nil
}
