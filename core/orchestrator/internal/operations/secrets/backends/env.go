// Package backends implements the storage-side half of the secret
// broker. Each Backend is responsible for one resolution surface
// (env vars, AWS Secrets Manager, Vault, etc.) and exposes a single
// Load method that returns raw material as []byte.
//
// Backends are the only code in the orchestrator allowed to touch
// secret material directly; the broker mediates every call through
// PDP allow + audit. The interface is intentionally tiny so a new
// backend can be added without touching the broker or any consumer.
package backends

import (
	"context"
	"fmt"
	"os"
	"strings"

	"ironflyer/core/orchestrator/internal/operations/secrets"
)

// Backend is re-exported as a type alias for ergonomics: a backend
// implementation file imports backends and gets the interface name
// it expects. The canonical declaration lives on the secrets package
// (secrets.BackendImpl) so the broker can refer to it without an
// import cycle.
type Backend = secrets.BackendImpl

// Env is the simplest backend: it resolves SecretRefs by reading
// process environment variables. The resolved key is:
//
//	prefix + (BackendRef OR Name)
//
// BackendRef takes precedence so operators can map "STRIPE_SECRET_KEY"
// to "PROD_STRIPE_KEY" without renaming the SecretRef itself.
type Env struct {
	prefix string
}

// NewEnv returns an Env backend that prepends prefix to every lookup.
// Pass "" to disable prefixing.
func NewEnv(prefix string) *Env { return &Env{prefix: prefix} }

func (e *Env) Name() secrets.Backend { return secrets.BackendEnv }

func (e *Env) Load(_ context.Context, ref secrets.SecretRef) ([]byte, error) {
	key := ref.BackendRef
	if key == "" {
		key = ref.Name
	}
	if key == "" {
		return nil, fmt.Errorf("%w: env backend requires name or backend_ref", secrets.ErrSecretNotFound)
	}
	full := e.prefix + key
	v, ok := os.LookupEnv(full)
	if !ok {
		// Be forgiving about leading/trailing whitespace in operator
		// configs — we don't want a stray space to silently drop a
		// credential.
		v, ok = os.LookupEnv(strings.TrimSpace(full))
	}
	if !ok || v == "" {
		return nil, fmt.Errorf("%w: env %q", secrets.ErrSecretNotFound, full)
	}
	out := make([]byte, len(v))
	copy(out, v)
	return out, nil
}
