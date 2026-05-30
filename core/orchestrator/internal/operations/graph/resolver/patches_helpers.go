package resolver

import (
	"context"

	"ironflyer/core/orchestrator/internal/operations/patch"
)

func (r *Resolver) requirePatchProjectOwner(ctx context.Context, projectID string) error {
	if r.Projects == nil {
		return gqlNotConfigured("projects")
	}
	u, err := currentUser(ctx)
	if err != nil {
		return err
	}
	project, err := r.Projects.Get(projectID)
	if err != nil {
		return err
	}
	if project.OwnerID == "" {
		return errUnauthenticated
	}
	if project.OwnerID != u.ID && project.OwnerID != tenantFor(u) {
		return errUnauthenticated
	}
	return nil
}

func (r *Resolver) requirePatchOwner(ctx context.Context, patchID string) (patch.Patch, error) {
	if r.Patches == nil {
		return patch.Patch{}, gqlNotConfigured("patches")
	}
	if _, err := currentUser(ctx); err != nil {
		return patch.Patch{}, err
	}
	p, err := r.Patches.Get(patchID)
	if err != nil {
		return patch.Patch{}, err
	}
	if err := r.requirePatchProjectOwner(ctx, p.ProjectID); err != nil {
		return patch.Patch{}, err
	}
	return p, nil
}

func (r *Resolver) requirePatchStageOwner(ctx context.Context, stageID string) (patch.PatchStage, bool, error) {
	if r.Patches == nil {
		return patch.PatchStage{}, false, gqlNotConfigured("patches")
	}
	if _, err := currentUser(ctx); err != nil {
		return patch.PatchStage{}, false, err
	}
	stage, ok, err := r.Patches.GetStage(stageID)
	if err != nil || !ok {
		return stage, ok, err
	}
	if err := r.requirePatchProjectOwner(ctx, stage.ProjectID); err != nil {
		return patch.PatchStage{}, false, err
	}
	return stage, true, nil
}
