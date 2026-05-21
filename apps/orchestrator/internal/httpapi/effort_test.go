package httpapi

import (
	"testing"

	"ironflyer/apps/orchestrator/internal/providers"
)

func TestApplyEffort_Lite_StripsThinkingAddsCheapFast(t *testing.T) {
	caps := []providers.Capability{providers.CapReasoning, providers.CapThinking, providers.CapCache}
	got, thinking := applyEffort("lite", caps, true)
	if thinking {
		t.Error("Lite must disable extended thinking")
	}
	if containsCap(got, providers.CapThinking) {
		t.Error("Lite must drop CapThinking from caps")
	}
	if containsCap(got, providers.CapReasoning) {
		t.Error("Lite must drop CapReasoning from caps")
	}
	if !containsCap(got, providers.CapCheap) || !containsCap(got, providers.CapFast) {
		t.Errorf("Lite must add CapCheap + CapFast, got %v", got)
	}
}

func TestApplyEffort_Power_AddsReasoningThinkingCache(t *testing.T) {
	got, thinking := applyEffort("power", []providers.Capability{providers.CapCode}, false)
	if !thinking {
		t.Error("Power must enable thinking")
	}
	for _, want := range []providers.Capability{
		providers.CapReasoning, providers.CapThinking, providers.CapCache, providers.CapCode,
	} {
		if !containsCap(got, want) {
			t.Errorf("Power must add %s, got %v", want, got)
		}
	}
}

func TestApplyEffort_EconomyAndEmptyAreNoOps(t *testing.T) {
	in := []providers.Capability{providers.CapReasoning, providers.CapJSON}
	for _, e := range []string{"", "economy"} {
		got, _ := applyEffort(e, in, true)
		if len(got) != len(in) {
			t.Errorf("effort %q should be a no-op, got %v", e, got)
		}
	}
}

func containsCap(caps []providers.Capability, want providers.Capability) bool {
	for _, c := range caps {
		if c == want {
			return true
		}
	}
	return false
}
