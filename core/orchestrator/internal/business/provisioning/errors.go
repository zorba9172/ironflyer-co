package provisioning

import "errors"

// ErrConnectorDisabled is the registry's "no enabled connector for this
// kind" sentinel. Resolvers translate it to NOT_CONFIGURED so the UI
// can render a "wire credentials" CTA instead of a hard error.
var ErrConnectorDisabled = errors.New("provisioning: no enabled connector for kind")

// ErrUnknownKind is returned for kinds outside the closed set in
// provisioning.go. Catches typos in mutations before they hit the
// connector layer.
var ErrUnknownKind = errors.New("provisioning: unknown resource kind")

// ErrResourceNotFound is returned when ProvisionedResource lookups
// don't hit a row. Distinct from a permission failure (ErrForbidden)
// so resolvers can pick the right HTTP-equivalent error code.
var ErrResourceNotFound = errors.New("provisioning: resource not found")

// ErrForbidden enforces owner-isolation: a tenant cannot read or
// mutate a ProvisionedResource owned by a different tenant. Resolvers
// translate this into a 404 (NOT a 403) so the response never confirms
// the row's existence to the wrong tenant.
var ErrForbidden = errors.New("provisioning: forbidden")

// ErrPolicyMissing is the PolicyStore sentinel — no RevenuePolicy
// configured for the named Kind. The Stripe Connect path refuses to
// charge without a policy so we never leave Ironflyer cut on the table.
var ErrPolicyMissing = errors.New("provisioning: revenue policy not configured")

// ErrDuplicateEvent is what RecordRevenue returns when the ExternalRef
// has already landed. Connector implementations rely on this to make
// HandleWebhook idempotent without a pre-check round-trip.
var ErrDuplicateEvent = errors.New("provisioning: revenue event already recorded")

// ErrInvalidAmount mirrors wallet.ErrInvalidAmount — non-positive
// gross amounts are refused at the boundary so a buggy connector
// can't quietly land zero-revenue rows.
var ErrInvalidAmount = errors.New("provisioning: invalid amount")
