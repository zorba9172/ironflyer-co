package agents

import (
	"strings"
	"testing"

	"ironflyer/core/orchestrator/internal/ai/providers"
)

func TestLoadDefaults_AllRolesPresent(t *testing.T) {
	got, err := LoadDefaults()
	if err != nil {
		t.Fatalf("LoadDefaults: %v", err)
	}
	want := []Role{
		RolePlanner, RoleUXer, RoleArchitect, RoleCoder,
		RoleReviewer, RoleTester, RoleSecurity, RoleDeployer,
		RoleCritic, RoleMigrator, Role("figma-translator"),
		Role("mobile-coder"), Role("mobile-deployer"),
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d agents, got %d", len(want), len(got))
	}
	have := map[Role]Agent{}
	for _, a := range got {
		have[a.Role] = a
	}
	for _, role := range want {
		a, ok := have[role]
		if !ok {
			t.Errorf("missing role %q", role)
			continue
		}
		if strings.TrimSpace(a.System) == "" {
			t.Errorf("role %q has empty system prompt", role)
		}
		if len(a.Capabilities) == 0 {
			t.Errorf("role %q has zero capabilities", role)
		}
	}
}

func TestLoadDefaults_PlannerWantsThinking(t *testing.T) {
	got, err := LoadDefaults()
	if err != nil {
		t.Fatalf("LoadDefaults: %v", err)
	}
	for _, a := range got {
		if a.Role != RolePlanner {
			continue
		}
		if !a.EnableThinking {
			t.Errorf("planner should have enableThinking=true")
		}
		hasThinking := false
		for _, c := range a.Capabilities {
			if c == providers.CapThinking {
				hasThinking = true
			}
		}
		if !hasThinking {
			t.Errorf("planner missing CapThinking; got %v", a.Capabilities)
		}
		return
	}
	t.Fatal("planner agent not found")
}

func TestLoadFromBytes_RejectsUnknownCapability(t *testing.T) {
	_, err := LoadFromBytes([]byte(`
agents:
  - role: planner
    system: hi
    capabilities: [reasoning, not-a-real-capability]
`))
	if err == nil {
		t.Fatal("expected error for unknown capability")
	}
	if !strings.Contains(err.Error(), "not-a-real-capability") {
		t.Errorf("error should name the bad tag, got: %v", err)
	}
}

func TestLoadFromBytes_RequiresRole(t *testing.T) {
	_, err := LoadFromBytes([]byte(`
agents:
  - system: missing role
    capabilities: [reasoning]
`))
	if err == nil {
		t.Fatal("expected error for missing role")
	}
}

func TestLoadFromBytes_EmptyFile(t *testing.T) {
	_, err := LoadFromBytes([]byte(`agents: []`))
	if err == nil {
		t.Fatal("expected error for empty agents list")
	}
}
