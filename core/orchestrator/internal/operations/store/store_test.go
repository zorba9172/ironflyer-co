package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"ironflyer/core/orchestrator/internal/ai/domain"
)

func TestMemoryStoreClonesProjectsAtBoundaries(t *testing.T) {
	s := NewMemoryStore()
	p := richProject("p1", "owner-a")

	created, err := s.Create(p)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	mutateProject(&p)
	mutateProject(&created)

	got, err := s.Get("p1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	assertRichProjectUnmutated(t, got)

	mutateProject(&got)
	again, err := s.Get("p1")
	if err != nil {
		t.Fatalf("Get again: %v", err)
	}
	assertRichProjectUnmutated(t, again)

	listed := s.List()
	if len(listed) != 1 {
		t.Fatalf("List len = %d, want 1", len(listed))
	}
	mutateProject(&listed[0])
	again, err = s.Get("p1")
	if err != nil {
		t.Fatalf("Get after List mutation: %v", err)
	}
	assertRichProjectUnmutated(t, again)

	batch, err := s.GetByIDs(context.Background(), []string{"p1"})
	if err != nil {
		t.Fatalf("GetByIDs: %v", err)
	}
	got = batch["p1"]
	mutateProject(&got)
	again, err = s.Get("p1")
	if err != nil {
		t.Fatalf("Get after GetByIDs mutation: %v", err)
	}
	assertRichProjectUnmutated(t, again)
}

func TestMemoryStoreUpdateDoesNotRetainCallbackOrReturnAliases(t *testing.T) {
	s := NewMemoryStore()
	if _, err := s.Create(richProject("p1", "owner-a")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	var retained map[domain.GateName]domain.GateState
	updated, err := s.Update("p1", func(p *domain.Project) {
		retained = p.Gates
		p.Name = "Updated"
		p.Artifacts["doc"] = json.RawMessage(`{"ok":false}`)
		p.Secrets["DATABASE_URL"] = "postgres://updated"
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	retained[domain.GateSpec] = domain.GateState{Name: domain.GateSpec, Status: domain.GateStatusFailed}
	updated.Artifacts["doc"][0] = '['
	updated.Secrets["DATABASE_URL"] = "postgres://return-mutated"

	got, err := s.Get("p1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Updated" {
		t.Fatalf("Name = %q, want Updated", got.Name)
	}
	if got.Gates[domain.GateSpec].Status != domain.GateStatusPassed {
		t.Fatalf("gate status = %q, want %q", got.Gates[domain.GateSpec].Status, domain.GateStatusPassed)
	}
	if string(got.Artifacts["doc"]) != `{"ok":false}` {
		t.Fatalf("artifact = %s, want updated JSON", got.Artifacts["doc"])
	}
	if got.Secrets["DATABASE_URL"] != "postgres://updated" {
		t.Fatalf("secret = %q, want update value", got.Secrets["DATABASE_URL"])
	}
}

func TestMemoryStoreListByOwnerPaginationIsDeterministicAndCloned(t *testing.T) {
	s := NewMemoryStore()
	for _, p := range []domain.Project{
		richProject("public", ""),
		richProject("a1", "owner-a"),
		richProject("b1", "owner-b"),
		richProject("a2", "owner-a"),
	} {
		if _, err := s.Create(p); err != nil {
			t.Fatalf("Create %s: %v", p.ID, err)
		}
	}

	got, err := s.ListByOwner(context.Background(), "owner-a", 2, 1)
	if err != nil {
		t.Fatalf("ListByOwner: %v", err)
	}
	assertProjectIDs(t, got, []string{"a1", "a2"})

	all, err := s.ListByOwner(context.Background(), "owner-a", 0, -10)
	if err != nil {
		t.Fatalf("ListByOwner uncapped: %v", err)
	}
	assertProjectIDs(t, all, []string{"public", "a1", "a2"})

	mutateProject(&all[0])
	again, err := s.Get("public")
	if err != nil {
		t.Fatalf("Get public: %v", err)
	}
	assertRichProjectUnmutated(t, again)
}

func richProject(id, ownerID string) domain.Project {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	return domain.Project{
		ID:          id,
		Name:        "Project " + id,
		Description: "description",
		Status:      "ready",
		OwnerID:     ownerID,
		Spec: domain.ProductSpec{
			Idea: "idea",
			UserStories: []domain.UserStory{{
				ID: "story-1", As: "user", IWant: "feature", SoThat: "value",
				Acceptance: []string{"acceptance"},
			}},
			DataModel: []domain.EntityDef{{
				Name: "Account", Fields: []string{"email"},
			}},
			Stack: domain.StackDecision{
				Frontend: "Next.js",
				Mobile: domain.MobileStack{
					Kind:    domain.MobileKindExpo,
					Targets: []domain.MobileTarget{domain.MobileTargetAndroid},
					EAS:     &domain.EASConfig{ProjectID: "eas-original"},
					Signing: &domain.MobileSigning{AndroidKeystoreSecret: "ks-original"},
				},
			},
			Compliance: []string{"soc2"},
		},
		Files: []domain.FileNode{{Path: "README.md", Type: "file", Content: "original"}},
		Artifacts: map[string]json.RawMessage{
			"doc": json.RawMessage(`{"ok":true}`),
		},
		Gates: map[domain.GateName]domain.GateState{
			domain.GateSpec: {
				Name:      domain.GateSpec,
				Status:    domain.GateStatusPassed,
				Issues:    []domain.Issue{{Gate: domain.GateSpec, Message: "original"}},
				UpdatedAt: now,
			},
		},
		Events:        []domain.Event{{ID: "event-1", Message: "original", CreatedAt: now}},
		GitHub:        &domain.GitHubLink{Owner: "octo", Repo: "repo"},
		Secrets:       map[string]string{"DATABASE_URL": "postgres://original"},
		VisualTargets: []domain.VisualTarget{{ID: "visual-1", RouteHint: "/", ViewportW: 1280, ViewportH: 800}},
		Subprojects: []domain.Subproject{{
			ID: "api", Name: "API", Path: "apps/api",
			Stack: domain.StackDecision{
				Mobile: domain.MobileStack{
					Kind:    domain.MobileKindExpo,
					Targets: []domain.MobileTarget{domain.MobileTargetIOS},
					EAS:     &domain.EASConfig{ProjectID: "sub-eas-original"},
				},
			},
			CreatedAt: now,
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func mutateProject(p *domain.Project) {
	p.Files[0].Content = "mutated"
	p.Artifacts["doc"][0] = '['
	g := p.Gates[domain.GateSpec]
	g.Issues[0].Message = "mutated"
	p.Gates[domain.GateSpec] = g
	p.Events[0].Message = "mutated"
	p.GitHub.Owner = "mutated"
	p.Secrets["DATABASE_URL"] = "postgres://mutated"
	p.VisualTargets[0].RouteHint = "/mutated"
	p.Spec.UserStories[0].Acceptance[0] = "mutated"
	p.Spec.DataModel[0].Fields[0] = "mutated"
	p.Spec.Stack.Mobile.Targets[0] = domain.MobileTargetIOS
	p.Spec.Stack.Mobile.EAS.ProjectID = "mutated"
	p.Spec.Stack.Mobile.Signing.AndroidKeystoreSecret = "mutated"
	p.Spec.Compliance[0] = "mutated"
	p.Subprojects[0].Stack.Mobile.Targets[0] = domain.MobileTargetAndroid
	p.Subprojects[0].Stack.Mobile.EAS.ProjectID = "mutated"
}

func assertRichProjectUnmutated(t *testing.T, p domain.Project) {
	t.Helper()
	if p.Files[0].Content != "original" {
		t.Fatalf("file content = %q, want original", p.Files[0].Content)
	}
	if string(p.Artifacts["doc"]) != `{"ok":true}` {
		t.Fatalf("artifact = %s, want original JSON", p.Artifacts["doc"])
	}
	if p.Gates[domain.GateSpec].Issues[0].Message != "original" {
		t.Fatalf("gate issue = %q, want original", p.Gates[domain.GateSpec].Issues[0].Message)
	}
	if p.Events[0].Message != "original" {
		t.Fatalf("event = %q, want original", p.Events[0].Message)
	}
	if p.GitHub.Owner != "octo" {
		t.Fatalf("github owner = %q, want octo", p.GitHub.Owner)
	}
	if p.Secrets["DATABASE_URL"] != "postgres://original" {
		t.Fatalf("secret = %q, want original", p.Secrets["DATABASE_URL"])
	}
	if p.VisualTargets[0].RouteHint != "/" {
		t.Fatalf("visual route = %q, want /", p.VisualTargets[0].RouteHint)
	}
	if p.Spec.UserStories[0].Acceptance[0] != "acceptance" {
		t.Fatalf("acceptance = %q, want original", p.Spec.UserStories[0].Acceptance[0])
	}
	if p.Spec.DataModel[0].Fields[0] != "email" {
		t.Fatalf("field = %q, want email", p.Spec.DataModel[0].Fields[0])
	}
	if p.Spec.Stack.Mobile.Targets[0] != domain.MobileTargetAndroid {
		t.Fatalf("target = %q, want android", p.Spec.Stack.Mobile.Targets[0])
	}
	if p.Spec.Stack.Mobile.EAS.ProjectID != "eas-original" {
		t.Fatalf("eas project = %q, want original", p.Spec.Stack.Mobile.EAS.ProjectID)
	}
	if p.Spec.Stack.Mobile.Signing.AndroidKeystoreSecret != "ks-original" {
		t.Fatalf("signing secret = %q, want original", p.Spec.Stack.Mobile.Signing.AndroidKeystoreSecret)
	}
	if p.Spec.Compliance[0] != "soc2" {
		t.Fatalf("compliance = %q, want soc2", p.Spec.Compliance[0])
	}
	if p.Subprojects[0].Stack.Mobile.Targets[0] != domain.MobileTargetIOS {
		t.Fatalf("subproject target = %q, want ios", p.Subprojects[0].Stack.Mobile.Targets[0])
	}
	if p.Subprojects[0].Stack.Mobile.EAS.ProjectID != "sub-eas-original" {
		t.Fatalf("subproject eas = %q, want original", p.Subprojects[0].Stack.Mobile.EAS.ProjectID)
	}
}

func assertProjectIDs(t *testing.T, got []domain.Project, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), want)
	}
	for i := range want {
		if got[i].ID != want[i] {
			t.Fatalf("project[%d] = %q, want %q", i, got[i].ID, want[i])
		}
	}
}
