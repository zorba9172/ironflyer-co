package payments

import "sync"

// Registry holds the set of providers wired at boot. Lookup is by name;
// Active returns just the enabled providers so the web UI can render
// only the buttons the operator can actually click.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewRegistry builds an empty registry.
func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

// Register adds p to the registry. Nil providers are ignored so callers
// can build them unconditionally and let Enabled() gate visibility.
func (r *Registry) Register(p Provider) {
	if p == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.Name()] = p
}

// Get returns the provider with the given name (or nil).
func (r *Registry) Get(name string) Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.providers[name]
}

// Active returns the providers whose Enabled() is true, in unspecified
// order. The slice is a fresh copy — callers may sort it.
func (r *Registry) Active() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		if p.Enabled() {
			out = append(out, p)
		}
	}
	return out
}
