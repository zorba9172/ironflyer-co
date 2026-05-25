package resolver

// Helpers consumed by auth.resolver.go. Live in their own file so
// gqlgen's "regenerate" pass does not strip them when the resolver
// file is rewritten.

import (
	"ironflyer/apps/orchestrator/internal/auth"
	"ironflyer/apps/orchestrator/internal/graph/model"
)

func toModelUser(u auth.User) *model.User {
	out := &model.User{
		ID:              u.ID,
		Email:           u.Email,
		CreatedAt:       u.CreatedAt,
		TelemetryOptOut: u.TelemetryOptOut,
	}
	if u.Name != "" {
		n := u.Name
		out.Name = &n
	}
	if u.Plan != "" {
		p := u.Plan
		out.Plan = &p
	}
	if u.OrgID != "" {
		o := u.OrgID
		out.OrgID = &o
	}
	if u.EmailVerifiedAt != nil {
		t := *u.EmailVerifiedAt
		out.EmailVerifiedAt = &t
	}
	return out
}
