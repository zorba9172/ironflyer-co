// Package appconsole backs the studio's Operate surfaces — the post-deploy
// "run the app" plane (Data, Users, Analytics, Automations, API, Marketing,
// Settings). Config surfaces (automations, API keys, webhooks, SEO settings,
// app settings + env vars, user role/suspension) are real mutable state held
// per project. Reflective surfaces (DB schema + rows, end-user roster,
// analytics, endpoint catalogue) are derived deterministically from the
// project id so the GraphQL contract is stable and honest until the runtime
// streams live telemetry into the same shapes.
//
// Every method is project-scoped; the resolver enforces the owner check
// against the project store before calling in.
package appconsole

import "time"

type Column struct {
	Name       string
	Type       string
	Nullable   bool
	PrimaryKey bool
	References string
}

type Table struct {
	Name     string
	RowCount int
	Columns  []Column
}

type TableRows struct {
	Table   string
	Columns []string
	Rows    []map[string]any
	Total   int
}

type EndUser struct {
	ID         string
	Email      string
	Name       string
	Role       string
	Status     string
	Provider   string
	LastSeenAt *time.Time
	CreatedAt  time.Time
}

type RoleCount struct {
	Role  string
	Count int
}

type UserStats struct {
	Total       int
	Active7d    int
	NewThisWeek int
	Suspended   int
	ByRole      []RoleCount
}

type MetricPoint struct {
	TS        time.Time
	Visitors  int
	PageViews int
	Sessions  int
}

type PageStat struct {
	Path       string
	Views      int
	AvgSeconds float64
}

type ReferrerStat struct {
	Source   string
	Visitors int
}

type EventStat struct {
	Name          string
	Count         int
	ConversionPct float64
}

type Analytics struct {
	RangeDays         int
	Visitors          int
	PageViews         int
	Sessions          int
	BounceRatePct     float64
	AvgSessionSeconds float64
	VisitorsDeltaPct  float64
	Series            []MetricPoint
	TopPages          []PageStat
	TopReferrers      []ReferrerStat
	Events            []EventStat
}

type Automation struct {
	ID            string
	ProjectID     string
	Name          string
	TriggerKind   string
	TriggerConfig string
	Action        string
	Enabled       bool
	LastRunAt     *time.Time
	LastStatus    string
	Runs          int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type APIKey struct {
	ID         string
	ProjectID  string
	Name       string
	Prefix     string
	Scopes     []string
	LastUsedAt *time.Time
	CreatedAt  time.Time
	Revoked    bool
}

type Endpoint struct {
	Method      string
	Path        string
	Description string
	Auth        string
}

type Webhook struct {
	ID        string
	ProjectID string
	URL       string
	Events    []string
	Enabled   bool
	CreatedAt time.Time
}

type SeoSettings struct {
	ProjectID      string
	Title          string
	Description    string
	Keywords       []string
	OgImageURL     string
	TwitterHandle  string
	CanonicalURL   string
	Robots         string
	SitemapEnabled bool
	UpdatedAt      time.Time
}

type SeoCheck struct {
	Key    string
	Label  string
	Passed bool
	Detail string
}

type SeoAudit struct {
	Score  int
	Checks []SeoCheck
}

type EnvVar struct {
	Key          string
	ValuePreview string
	Secret       bool
	UpdatedAt    time.Time
	raw          string
}

type Settings struct {
	ProjectID    string
	DisplayName  string
	Visibility   string
	Region       string
	SupportEmail string
	EnvVars      []EnvVar
	UpdatedAt    time.Time
}
