package policy

import (
	"context"
	"fmt"
)

// Rebind swaps the LocalPDP's bundle set + version atomically. The
// caller is policy.Reloader; the swap holds the same RWMutex as Decide
// so an in-flight Eval sees the old prepared query and any subsequent
// Eval sees the new one — never half.
//
// A bundle that fails to parse is a hard error: we leave the existing
// prepared query in place and return the error so the operator sees a
// loud failure (the reloader logs it). Default-deny is preserved
// because we never tear down the old PDP first.
//
// Lives in its own file (not opa_local.go) so the bundle-loader and
// hot-reload concerns stay visually separate inside the policy
// package.
func (p *LocalPDP) Rebind(_ context.Context, bundles map[string]string, version string) error {
	if p == nil {
		return fmt.Errorf("policy: Rebind on nil LocalPDP")
	}
	if len(bundles) == 0 {
		return fmt.Errorf("policy: Rebind refuses empty bundle set")
	}
	// Stage the new state inside a sibling LocalPDP so prepare()
	// runs against the new bundle set without mutating p. Only after
	// prepare() succeeds do we swap p's fields under the write lock.
	staged := &LocalPDP{
		bundles: bundles,
		version: version,
	}
	if err := staged.prepare(); err != nil {
		return fmt.Errorf("policy: Rebind prepare: %w", err)
	}

	p.mu.Lock()
	p.bundles = bundles
	p.version = version
	p.prepped = staged.prepped
	p.mu.Unlock()

	p.log.Info().
		Str("policy_bundle_version", version).
		Int("modules", len(bundles)).
		Msg("policy: local PDP rebound to new bundle set")
	return nil
}
