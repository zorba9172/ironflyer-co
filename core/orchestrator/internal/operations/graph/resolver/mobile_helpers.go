package resolver

// Helpers for the mobile (EAS) resolver — owner-check + per-project
// EAS client resolution + the eas.Build / eas.Submission / eas.Update
// → gqlgen model converters. Kept in a sibling file so gqlgen's
// regen pass on mobile.resolver.go does not move them around as
// "helper methods in this file".

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/operations/graph/model"
	"ironflyer/core/orchestrator/internal/operations/mobile/eas"
	"ironflyer/core/orchestrator/internal/operations/store"
)

// mobileClientForProject resolves the per-project EAS bearer, enforces
// ownership against the authenticated user, and returns a Client ready
// for the per-call project context. A project-scoped secret wins over
// the orchestrator-wide r.EAS instance so paying customers running on
// their own Expo account get isolation by default.
func (r *Resolver) mobileClientForProject(ctx context.Context, projectID string) (*eas.Client, domain.Project, error) {
	if r.Projects == nil {
		return nil, domain.Project{}, gqlNotConfigured("mobile")
	}
	u, err := currentUser(ctx)
	if err != nil {
		return nil, domain.Project{}, err
	}
	p, err := r.Projects.Get(projectID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, domain.Project{}, fmt.Errorf("project %s: %w", projectID, err)
		}
		return nil, domain.Project{}, err
	}
	if p.OwnerID != "" && p.OwnerID != u.ID {
		// Public seed (OwnerID=="") stays readable; everything else
		// requires owner match.
		return nil, domain.Project{}, fmt.Errorf("project %s: forbidden", projectID)
	}
	tok, err := eas.ResolveExpoToken(&p)
	if err != nil {
		// No per-project secret + no env fallback: defer to the
		// globally configured client when present.
		if r.EAS != nil && r.EAS.HasToken() {
			return r.EAS, p, nil
		}
		return nil, domain.Project{}, gqlNotConfigured("mobile")
	}
	// Per-project token wins over the global client.
	cli := eas.New(tok, eas.WithLogger(r.Logger))
	return cli, p, nil
}

// mobileEASProjectID extracts the Expo project id from the domain
// MobileStack. Returns "" when the project is web-only or has not
// linked an Expo project yet.
func mobileEASProjectID(p domain.Project) string {
	m := p.Spec.Stack.Mobile
	if m.EAS == nil {
		return ""
	}
	return strings.TrimSpace(m.EAS.ProjectID)
}

// mobileTenantUUID returns the canonical tenant uuid for ledger
// attribution. V22 uses the owner uuid as the tenant; returns
// uuid.Nil when the owner id is not a valid uuid so the poller
// skips RecordEASBuild instead of corrupting the ledger.
func mobileTenantUUID(p domain.Project) uuid.UUID {
	if p.OwnerID == "" {
		return uuid.Nil
	}
	id, err := uuid.Parse(p.OwnerID)
	if err != nil {
		return uuid.Nil
	}
	return id
}

// mobileBuildToGraphQL converts an eas.Build into the gqlgen model.
func mobileBuildToGraphQL(b *eas.Build) *model.MobileBuild {
	if b == nil {
		return nil
	}
	out := &model.MobileBuild{
		ID:        b.ID,
		ProjectID: b.ProjectID,
		Platform:  b.Platform,
		Profile:   b.Profile,
		Status:    string(b.Status),
		CreatedAt: b.CreatedAt,
		UpdatedAt: b.UpdatedAt,
	}
	if b.Distribution != "" {
		v := b.Distribution
		out.Distribution = &v
	}
	if b.ArtifactURL != "" {
		v := b.ArtifactURL
		out.ArtifactURL = &v
	}
	if b.ArtifactSize > 0 {
		v := int(b.ArtifactSize)
		out.ArtifactSizeBytes = &v
	}
	if b.LogURL != "" {
		v := b.LogURL
		out.LogURL = &v
	}
	if b.AppVersion != "" {
		v := b.AppVersion
		out.AppVersion = &v
	}
	if b.AppBuildVersion != "" {
		v := b.AppBuildVersion
		out.AppBuildVersion = &v
	}
	if b.SDKVersion != "" {
		v := b.SDKVersion
		out.SdkVersion = &v
	}
	if b.Channel != "" {
		v := b.Channel
		out.Channel = &v
	}
	if b.Initiator != "" {
		v := b.Initiator
		out.Initiator = &v
	}
	if b.Error != nil && b.Error.Message != "" {
		v := b.Error.Message
		out.ErrorMessage = &v
	}
	if b.CompletedAt != nil {
		t := *b.CompletedAt
		out.CompletedAt = &t
	}
	return out
}

// mobileSubmissionToGraphQL converts an eas.Submission into the gqlgen
// model.
func mobileSubmissionToGraphQL(s *eas.Submission) *model.MobileSubmission {
	if s == nil {
		return nil
	}
	out := &model.MobileSubmission{
		ID:        s.ID,
		ProjectID: s.ProjectID,
		Platform:  s.Platform,
		Target:    s.Target,
		Status:    string(s.Status),
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
	if s.BuildID != "" {
		v := s.BuildID
		out.BuildID = &v
	}
	if s.ArchiveURL != "" {
		v := s.ArchiveURL
		out.ArchiveURL = &v
	}
	if s.LogURL != "" {
		v := s.LogURL
		out.LogURL = &v
	}
	if s.Error != nil && s.Error.Message != "" {
		v := s.Error.Message
		out.ErrorMessage = &v
	}
	if s.CompletedAt != nil {
		t := *s.CompletedAt
		out.CompletedAt = &t
	}
	return out
}

// mobileUpdateToGraphQL converts an eas.Update into the gqlgen model.
func mobileUpdateToGraphQL(u *eas.Update) *model.MobileUpdate {
	if u == nil {
		return nil
	}
	out := &model.MobileUpdate{
		ID:             u.ID,
		Branch:         u.Branch,
		Channel:        u.Channel,
		RuntimeVersion: u.RuntimeVersion,
		CreatedAt:      u.CreatedAt,
	}
	if u.Platform != "" {
		v := u.Platform
		out.Platform = &v
	}
	if u.Message != "" {
		v := u.Message
		out.Message = &v
	}
	if u.ManifestURL != "" {
		v := u.ManifestURL
		out.ManifestURL = &v
	}
	if u.GroupID != "" {
		v := u.GroupID
		out.GroupID = &v
	}
	return out
}

// strDeref unwraps a nullable string input or returns "".
func strDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
