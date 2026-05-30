package resolver

import (
	"context"
	"testing"
	"time"

	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/customer/auth"
	"ironflyer/core/orchestrator/internal/operations/graph/model"
	"ironflyer/core/orchestrator/internal/operations/patch"
	"ironflyer/core/orchestrator/internal/operations/store"
)

func TestPatchMutationsRequireProjectOwner(t *testing.T) {
	resolver, ctxOwner, ctxIntruder := testPatchResolver(t)
	mut := &mutationResolver{resolver}

	content := "package main\n"
	created, err := mut.ProposePatch(ctxOwner, model.ProposePatchInput{
		ProjectID: "owned-project",
		Title:     patchTestStrPtr("add main"),
		Changes: []model.PatchChangeInput{{
			Op:      model.PatchChangeOpCreate,
			Path:    "main.go",
			Content: &content,
		}},
	})
	if err != nil {
		t.Fatalf("owner ProposePatch() error = %v", err)
	}

	if _, err := mut.ApplyPatch(ctxIntruder, created.ID); err == nil {
		t.Fatalf("intruder ApplyPatch() error = nil, want authorization error")
	}
	project, err := resolver.Projects.Get("owned-project")
	if err != nil {
		t.Fatalf("Get(project) error = %v", err)
	}
	if len(project.Files) != 0 {
		t.Fatalf("intruder ApplyPatch() changed project files: %#v", project.Files)
	}

	stage, err := mut.CreateStage(ctxOwner, model.CreateStageInput{
		ProjectID: "owned-project",
		Name:      "stage",
		PatchIds:  []string{created.ID},
	})
	if err != nil {
		t.Fatalf("owner CreateStage() error = %v", err)
	}
	if _, err := mut.RejectStage(ctxIntruder, stage.ID, patchTestStrPtr("nope")); err == nil {
		t.Fatalf("intruder RejectStage() error = nil, want authorization error")
	}
	if _, err := mut.ApplyStage(ctxIntruder, stage.ID); err == nil {
		t.Fatalf("intruder ApplyStage() error = nil, want authorization error")
	}
	stored, ok, err := resolver.Patches.GetStage(stage.ID)
	if err != nil {
		t.Fatalf("GetStage() error = %v", err)
	}
	if !ok {
		t.Fatalf("GetStage() ok = false, want true")
	}
	if stored.Status != patch.StageStatusOpen {
		t.Fatalf("stage status = %q, want %q", stored.Status, patch.StageStatusOpen)
	}
}

func TestProposePatchRequiresOwnedProjectStoreGuard(t *testing.T) {
	projects := store.NewMemoryStore()
	if _, err := projects.Create(domain.Project{
		ID:      "public-seed",
		Name:    "Public Seed",
		OwnerID: "",
	}); err != nil {
		t.Fatalf("Create(public project) error = %v", err)
	}
	resolver := &Resolver{
		Projects: projects,
		Patches:  patch.NewEngine(projects),
	}
	ctx := auth.WithUser(context.Background(), auth.User{ID: "user-a"})

	content := "hi\n"
	if _, err := (&mutationResolver{resolver}).ProposePatch(ctx, model.ProposePatchInput{
		ProjectID: "public-seed",
		Changes: []model.PatchChangeInput{{
			Op:      model.PatchChangeOpCreate,
			Path:    "README.md",
			Content: &content,
		}},
	}); err == nil {
		t.Fatalf("ProposePatch(public project) error = nil, want authorization error")
	}
	if got := resolver.Patches.List("public-seed"); len(got) != 0 {
		t.Fatalf("public project patches = %d, want 0", len(got))
	}
}

func testPatchResolver(t *testing.T) (*Resolver, context.Context, context.Context) {
	t.Helper()

	projects := store.NewMemoryStore()
	now := time.Now().UTC()
	if _, err := projects.Create(domain.Project{
		ID:        "owned-project",
		Name:      "Owned",
		OwnerID:   "owner",
		Files:     []domain.FileNode{},
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("Create(project) error = %v", err)
	}

	resolver := &Resolver{
		Projects: projects,
		Patches:  patch.NewEngine(projects),
	}
	ctxOwner := auth.WithUser(context.Background(), auth.User{ID: "owner"})
	ctxIntruder := auth.WithUser(context.Background(), auth.User{ID: "intruder"})
	return resolver, ctxOwner, ctxIntruder
}

func patchTestStrPtr(s string) *string {
	return &s
}
