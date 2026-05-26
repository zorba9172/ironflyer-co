package deploy

import "errors"

// Sentinel errors callers can branch on. Wrap with fmt.Errorf("%w: …")
// when adding context — never replace.
var (
	// ErrNotFound is returned when Service.Get / Approval lookups
	// can't resolve the id (or the caller is not the owning tenant —
	// we return 404-style to avoid leaking existence).
	ErrNotFound = errors.New("deploy: not found")

	// ErrInvalidState fires when a transition is requested out of the
	// allowed FSM order (e.g. Promote from `planned` without a preview).
	ErrInvalidState = errors.New("deploy: invalid state")

	// ErrApprovalRequired is returned by Promote when the deploy is a
	// production deploy and no approved approval row backs it.
	ErrApprovalRequired = errors.New("deploy: approval required")

	// ErrApprovalNotPending fires when Decide is called on an approval
	// that has already been decided / expired / withdrawn.
	ErrApprovalNotPending = errors.New("deploy: approval not pending")

	// ErrApprovalExpired fires when an approval's expires_at has
	// already passed — Promote treats this identically to a missing
	// approval, Decide refuses to flip a row that already expired.
	ErrApprovalExpired = errors.New("deploy: approval expired")

	// ErrProfitGuardBlocked is returned by GuardDeploy (and surfaced by
	// the Service before it ever opens a production deploy) when the
	// ProfitGuardChecker returns a hard-stop verdict for the
	// BeforeVercelDeploy enforcement point.
	ErrProfitGuardBlocked = errors.New("deploy: blocked by profit guard")

	// ErrUnknownTarget is returned by Service when PlanInput.Target
	// has no registered adapter.
	ErrUnknownTarget = errors.New("deploy: unknown target")

	// ErrSecretMissing is returned by the Vercel adapter when the
	// SecretResolver could not produce a VERCEL_TOKEN for the tenant.
	ErrSecretMissing = errors.New("deploy: secret missing")

	// ErrProviderFailure is the catch-all the Vercel adapter wraps
	// non-2xx provider responses with so the Service can branch on
	// "the upstream said no".
	ErrProviderFailure = errors.New("deploy: provider failure")
)
