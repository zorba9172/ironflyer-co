package resolver

// guild_helpers.go is the hand-rolled glue between the internal
// `guild.*` value types and the gqlgen-generated `model.*` GraphQL
// shapes. Kept in a *_helpers.go file (mirrors the
// devicecloud_helpers / mcp_helpers conventions) so the next gqlgen
// generate pass does not bury these converters in the auto-generated
// resolver file.

import (
	"context"
	"errors"

	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/business/guild"
	"ironflyer/core/orchestrator/internal/operations/graph/model"
)

// profileToGraphQL projects a finisher profile onto the GraphQL
// shape. Decimal fields render as strings to preserve precision (the
// schema declares hourlyRateUSD / rating as String, mirroring the
// wallet pattern of "render exact decimals as strings").
func profileToGraphQL(p guild.FinisherProfile) model.FinisherProfile {
	return model.FinisherProfile{
		ID:                 p.ID,
		DisplayName:        p.DisplayName,
		Skills:             append([]string(nil), p.Skills...),
		HourlyRateUsd:      p.HourlyRateUSD.String(),
		CompletedTaskCount: p.CompletedTaskCount,
		Rating:             p.Rating.String(),
		Verified:           p.Verified,
	}
}

// taskToGraphQL projects a guild task. Bid count is resolved lazily
// here so the GraphQL caller sees a real number rather than chasing
// a separate field. AssignedTo is hydrated from the finisher store
// when set; failures degrade to nil so the row still renders.
func (r *Resolver) taskToGraphQL(ctx context.Context, t guild.GuildTask, project domain.Project) model.GuildTask {
	out := model.GuildTask{
		ID:            t.ID,
		Project:       projectToGraphQL(project),
		Title:         t.Title,
		Description:   t.Description,
		PriceUSDFloor: t.PriceUSDFloor.String(),
		SLAHours:      t.SLAHours,
		Status:        t.Status,
		CreatedAt:     t.CreatedAt,
	}
	if r.GuildCoord != nil {
		if n, err := r.GuildCoord.Service().CountBidsForTask(ctx, t.ID); err == nil {
			out.BidCount = n
		}
		if t.AssignedTo != nil && *t.AssignedTo != "" {
			if p, err := r.GuildCoord.Service().GetFinisherProfile(ctx, *t.AssignedTo); err == nil {
				gp := profileToGraphQL(p)
				out.AssignedTo = &gp
			}
		}
	}
	return out
}

// bidToGraphQL projects one bid + hydrates the parent task and
// finisher reference fields. Errors propagate because every Bid in
// the schema declares non-null task / finisher — a half-hydrated bid
// is unrenderable.
func (r *Resolver) bidToGraphQL(ctx context.Context, b guild.Bid) (model.Bid, error) {
	if r.GuildCoord == nil {
		return model.Bid{}, errors.New("guild: not configured")
	}
	task, err := r.GuildCoord.Service().GetTask(ctx, b.TaskID)
	if err != nil {
		return model.Bid{}, err
	}
	project, err := r.lookupProject(task.ProjectID)
	if err != nil {
		return model.Bid{}, err
	}
	profile, err := r.GuildCoord.Service().GetFinisherProfile(ctx, b.FinisherID)
	if err != nil {
		return model.Bid{}, err
	}
	return model.Bid{
		ID:             b.ID,
		Task:           r.taskToGraphQL(ctx, task, project),
		Finisher:       profileToGraphQL(profile),
		PriceUsd:       b.PriceUSD.String(),
		EstimatedHours: b.EstimatedHours,
		Note:           b.Note,
		Status:         b.Status,
		CreatedAt:      b.CreatedAt,
	}, nil
}

// templateToGraphQL projects one template + hydrates the author
// profile. When the author profile is missing the template still
// renders with a sentinel zero-valued author so the catalog stays
// browseable.
func (r *Resolver) templateToGraphQL(ctx context.Context, t guild.Template) (model.Template, error) {
	out := model.Template{
		ID:           t.ID,
		Slug:         t.Slug,
		Name:         t.Name,
		Description:  t.Description,
		PriceUsd:     t.PriceUSD.String(),
		GatesPassed:  append([]string(nil), t.GatesPassed...),
		InstallCount: t.InstallCount,
		Verified:     t.Verified,
	}
	if r.GuildCoord != nil {
		if p, err := r.GuildCoord.Service().GetFinisherProfileByUser(ctx, t.AuthorUserID); err == nil {
			out.Author = profileToGraphQL(p)
		}
	}
	return out, nil
}

// requireOwnedProject is the shared owner-check used by every guild
// mutation that scopes a project. 404s on missing / non-owner so the
// existence of someone else's project stays unleakable.
func (r *Resolver) requireOwnedProject(projectID, userID string) (domain.Project, error) {
	if r.Projects == nil {
		return domain.Project{}, errUnauthenticated
	}
	p, err := r.Projects.Get(projectID)
	if err != nil {
		return domain.Project{}, err
	}
	if !p.IsAccessibleBy(userID) {
		return domain.Project{}, errUnauthenticated
	}
	return p, nil
}

// lookupProject is the projection helper used by taskToGraphQL /
// bidToGraphQL when no owner-check is required (read paths). Returns
// the zero value and a non-nil error when the project store is
// missing the row, which the caller treats as a soft skip.
func (r *Resolver) lookupProject(projectID string) (domain.Project, error) {
	if r.Projects == nil {
		return domain.Project{}, errors.New("projects store not configured")
	}
	return r.Projects.Get(projectID)
}
