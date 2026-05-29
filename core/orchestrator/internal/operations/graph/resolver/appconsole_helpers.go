package resolver

import (
	"context"

	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/operations/appconsole"
	"ironflyer/core/orchestrator/internal/operations/graph/model"
	"ironflyer/core/orchestrator/internal/operations/store"
)

// requireOperateProject enforces the standard owner check before any Operate
// resolver touches a project's post-deploy state. Returns store.ErrNotFound
// (a 404, not a 403) for projects the caller cannot see, matching the rest of
// the project-scoped resolvers.
func (r *Resolver) requireOperateProject(ctx context.Context, projectID string) (domain.Project, error) {
	if r.AppConsole == nil {
		return domain.Project{}, gqlNotConfigured("operate")
	}
	if r.Projects == nil {
		return domain.Project{}, gqlNotConfigured("projects")
	}
	p, err := r.Projects.Get(projectID)
	if err != nil {
		return domain.Project{}, err
	}
	userID := ""
	if u, err := currentUser(ctx); err == nil {
		userID = u.ID
	}
	if !p.IsAccessibleBy(userID) {
		return domain.Project{}, store.ErrNotFound
	}
	return p, nil
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// requireOperateByAutomation resolves an automation's owning project and runs
// the owner check — used by the id-only automation mutations. Lives here (not
// in the resolver file) so gqlgen's follow-schema regen never evicts it.
func (r *Resolver) requireOperateByAutomation(ctx context.Context, id string) error {
	if r.AppConsole == nil {
		return gqlNotConfigured("operate")
	}
	pid, err := r.AppConsole.AutomationProject(id)
	if err != nil {
		return err
	}
	_, err = r.requireOperateProject(ctx, pid)
	return err
}

// ---- Data ----

func appTableToModel(t appconsole.Table) model.AppTable {
	cols := make([]model.AppColumn, 0, len(t.Columns))
	for _, c := range t.Columns {
		cols = append(cols, model.AppColumn{
			Name: c.Name, Type: c.Type, Nullable: c.Nullable, PrimaryKey: c.PrimaryKey,
			References: strPtrOrNil(c.References),
		})
	}
	return model.AppTable{Name: t.Name, RowCount: t.RowCount, Columns: cols}
}

func appTableRowsToModel(tr appconsole.TableRows) *model.AppTableRows {
	rows := make([]model.JSON, 0, len(tr.Rows))
	for _, row := range tr.Rows {
		rows = append(rows, model.JSON(row))
	}
	return &model.AppTableRows{Table: tr.Table, Columns: tr.Columns, Rows: rows, Total: tr.Total}
}

// ---- Users ----

func appEndUserToModel(u appconsole.EndUser) model.AppEndUser {
	return model.AppEndUser{
		ID: u.ID, Email: u.Email, Name: u.Name, Role: u.Role, Status: u.Status,
		Provider: u.Provider, LastSeenAt: u.LastSeenAt, CreatedAt: u.CreatedAt,
	}
}

func appUserStatsToModel(s appconsole.UserStats) *model.AppUserStats {
	roles := make([]model.RoleCount, 0, len(s.ByRole))
	for _, rc := range s.ByRole {
		roles = append(roles, model.RoleCount{Role: rc.Role, Count: rc.Count})
	}
	return &model.AppUserStats{Total: s.Total, Active7d: s.Active7d, NewThisWeek: s.NewThisWeek, Suspended: s.Suspended, ByRole: roles}
}

// ---- Analytics ----

func appAnalyticsToModel(a appconsole.Analytics) *model.AppAnalytics {
	series := make([]model.AppMetricPoint, 0, len(a.Series))
	for _, p := range a.Series {
		series = append(series, model.AppMetricPoint{Ts: p.TS, Visitors: p.Visitors, PageViews: p.PageViews, Sessions: p.Sessions})
	}
	pages := make([]model.AppPageStat, 0, len(a.TopPages))
	for _, p := range a.TopPages {
		pages = append(pages, model.AppPageStat{Path: p.Path, Views: p.Views, AvgSeconds: p.AvgSeconds})
	}
	refs := make([]model.AppReferrerStat, 0, len(a.TopReferrers))
	for _, p := range a.TopReferrers {
		refs = append(refs, model.AppReferrerStat{Source: p.Source, Visitors: p.Visitors})
	}
	events := make([]model.AppEventStat, 0, len(a.Events))
	for _, e := range a.Events {
		events = append(events, model.AppEventStat{Name: e.Name, Count: e.Count, ConversionPct: e.ConversionPct})
	}
	return &model.AppAnalytics{
		RangeDays: a.RangeDays, Visitors: a.Visitors, PageViews: a.PageViews, Sessions: a.Sessions,
		BounceRatePct: a.BounceRatePct, AvgSessionSeconds: a.AvgSessionSeconds, VisitorsDeltaPct: a.VisitorsDeltaPct,
		Series: series, TopPages: pages, TopReferrers: refs, Events: events,
	}
}

// ---- Automations ----

func automationToModel(a appconsole.Automation) *model.Automation {
	return &model.Automation{
		ID: a.ID, Name: a.Name, TriggerKind: a.TriggerKind, TriggerConfig: a.TriggerConfig,
		Action: a.Action, Enabled: a.Enabled, LastRunAt: a.LastRunAt, LastStatus: a.LastStatus,
		Runs: a.Runs, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

// ---- API ----

func appAPIKeyToModel(k appconsole.APIKey) *model.AppAPIKey {
	return &model.AppAPIKey{
		ID: k.ID, Name: k.Name, Prefix: k.Prefix, Scopes: k.Scopes,
		LastUsedAt: k.LastUsedAt, CreatedAt: k.CreatedAt, Revoked: k.Revoked,
	}
}

func appEndpointToModel(e appconsole.Endpoint) model.AppEndpoint {
	return model.AppEndpoint{Method: e.Method, Path: e.Path, Description: e.Description, Auth: e.Auth}
}

func appWebhookToModel(w appconsole.Webhook) *model.AppWebhook {
	return &model.AppWebhook{ID: w.ID, URL: w.URL, Events: w.Events, Enabled: w.Enabled, CreatedAt: w.CreatedAt}
}

// ---- Marketing ----

func appSeoSettingsToModel(s appconsole.SeoSettings) *model.AppSeoSettings {
	return &model.AppSeoSettings{
		ProjectID: s.ProjectID, Title: s.Title, Description: s.Description, Keywords: s.Keywords,
		OgImageURL: s.OgImageURL, TwitterHandle: s.TwitterHandle, CanonicalURL: s.CanonicalURL,
		Robots: s.Robots, SitemapEnabled: s.SitemapEnabled, UpdatedAt: s.UpdatedAt,
	}
}

func appSeoAuditToModel(a appconsole.SeoAudit) *model.AppSeoAudit {
	checks := make([]model.SeoCheck, 0, len(a.Checks))
	for _, c := range a.Checks {
		checks = append(checks, model.SeoCheck{Key: c.Key, Label: c.Label, Passed: c.Passed, Detail: c.Detail})
	}
	return &model.AppSeoAudit{Score: a.Score, Checks: checks}
}

// ---- Settings ----

func appSettingsToModel(s appconsole.Settings) *model.AppSettings {
	envs := make([]model.AppEnvVar, 0, len(s.EnvVars))
	for _, e := range s.EnvVars {
		envs = append(envs, model.AppEnvVar{Key: e.Key, ValuePreview: e.ValuePreview, Secret: e.Secret, UpdatedAt: e.UpdatedAt})
	}
	return &model.AppSettings{
		ProjectID: s.ProjectID, DisplayName: s.DisplayName, Visibility: s.Visibility,
		Region: s.Region, SupportEmail: s.SupportEmail, EnvVars: envs, UpdatedAt: s.UpdatedAt,
	}
}
