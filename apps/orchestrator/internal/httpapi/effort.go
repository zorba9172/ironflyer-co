package httpapi

import "ironflyer/apps/orchestrator/internal/providers"

// applyEffort biases the agent's declared capabilities by the caller's
// effort dial. Lite favours cheap + fast models and drops thinking entirely;
// Power adds reasoning + thinking + cache so the router picks the strongest
// available model; Economy (the default) is a no-op so the agent's natural
// capability profile decides the route.
func applyEffort(effort string, caps []providers.Capability, enableThinking bool) ([]providers.Capability, bool) {
	add := func(c providers.Capability) {
		for _, x := range caps {
			if x == c {
				return
			}
		}
		caps = append(caps, c)
	}
	remove := func(c providers.Capability) {
		out := caps[:0]
		for _, x := range caps {
			if x != c {
				out = append(out, x)
			}
		}
		caps = out
	}
	switch effort {
	case "lite":
		add(providers.CapCheap)
		add(providers.CapFast)
		remove(providers.CapThinking)
		remove(providers.CapReasoning)
		enableThinking = false
	case "power":
		add(providers.CapReasoning)
		add(providers.CapThinking)
		add(providers.CapCache)
		enableThinking = true
	}
	return caps, enableThinking
}
