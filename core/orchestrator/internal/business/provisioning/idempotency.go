package provisioning

// V22-style op_key dedupe for provisioning operations. Mirrors the
// wallet/idempotency.go shape: opkey-keyed mutation log keyed on
// natural identifiers from the rail (Stripe application_fee.id is the
// canonical key for Connect application-fee.created events) so a
// webhook redelivery folds onto the same row.
//
// The Postgres backend stores rows in `provisioning_operations`
// (PRIMARY KEY op_key, see migrations/00047_provisioning.sql); the
// memory backend keeps an in-process map with identical semantics so
// dev / smoke runs see the same idempotency contract.

// OpType is the closed enum for provisioning_operations.op_type.
// Matches the CHECK constraint in 00047_provisioning.sql; new values
// require both an OpType const and a migration.
type OpType string

const (
	// OpProvision is a Connector.Provision call landing — onboarding a
	// new Stripe Connect account, registering a domain, etc.
	OpProvision OpType = "provision"
	// OpRecordRevenue is a single RevenueEvent insert. The op_key for
	// these is the rail's natural id (Stripe application_fee.id,
	// domain renewal invoice id, …).
	OpRecordRevenue OpType = "record_revenue"
	// OpSuspend is a Connector.Suspend call landing.
	OpSuspend OpType = "suspend"
)
