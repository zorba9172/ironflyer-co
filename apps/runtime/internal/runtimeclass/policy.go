package runtimeclass

import "sync"

// Policy is the per-tenant allowlist of acceptable RuntimeClass
// names. The selector intersects the tenant's allowlist with the
// risk-derived preference; if the intersection is empty the tenant
// gets the cheapest class still in their allowlist.
type Policy struct {
	mu      sync.RWMutex
	default_ []string                // global default allowlist
	tenants map[string][]string      // tenantID -> allowlist
	forced  map[string]string        // tenantID -> forced class (enterprise)
}

// NewPolicy builds an empty Policy. Without explicit configuration
// every tenant gets the default allowlist (docker + gvisor) — Kata
// and Firecracker are opt-in.
func NewPolicy() *Policy {
	return &Policy{
		default_: []string{ClassDocker, ClassGVisor},
		tenants:  make(map[string][]string),
		forced:   make(map[string]string),
	}
}

// SetDefaults overrides the global default allowlist.
func (p *Policy) SetDefaults(classes []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.default_ = append([]string(nil), classes...)
}

// Allow sets the allowlist for one tenant.
func (p *Policy) Allow(tenantID string, classes ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tenants[tenantID] = append([]string(nil), classes...)
}

// Force pins one tenant to a single class (enterprise / regulated
// workloads where Firecracker is mandatory).
func (p *Policy) Force(tenantID, className string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.forced[tenantID] = className
}

// AllowedFor returns the effective allowlist for the tenant.
func (p *Policy) AllowedFor(tenantID string) []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if forced, ok := p.forced[tenantID]; ok {
		return []string{forced}
	}
	if al, ok := p.tenants[tenantID]; ok && len(al) > 0 {
		return append([]string(nil), al...)
	}
	return append([]string(nil), p.default_...)
}
