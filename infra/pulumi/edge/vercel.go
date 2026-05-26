// Vercel surface for the Ironflyer dashboard.
//
// Vercel hosts the Ironflyer dashboard (Next.js) — the web frontend the
// product agent is rebuilding under `clients/web`. The orchestrator runs on
// EKS (provisioned by the compute/ + data/ packages); the dashboard runs on
// Vercel and is wired to the orchestrator's public API via env vars. A
// Route53 CNAME (see `AddVercelCNAME`) attaches `app.<region>.ironflyer.dev`
// to Vercel's edge.
//
// All resources flow through an explicit `vercel.Provider` instance so the
// Pulumi program does not depend on a `VERCEL_API_TOKEN` environment
// variable at runtime — the token comes from Pulumi secret config.
package edge

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumiverse/pulumi-vercel/sdk/v3/go/vercel"

	"ironflyer/infra/pulumi/compute"
)

// VercelArgs is the contract main.go passes into NewVercel. Every field is
// a pulumi.Input so secrets (api token, sentry dsn) stay opaque in the
// state file.
type VercelArgs struct {
	// TeamID is the Vercel team slug or numeric ID. Required for any
	// resource that lives under an org team rather than a personal account.
	TeamID pulumi.StringInput

	// APIToken is the Vercel personal access token. MUST be set as a Pulumi
	// secret (see README). The orchestrator's vercel-adapter uses a
	// different token; in single-tenant prod both may be the same value.
	APIToken pulumi.StringInput

	// Domain is the customer-facing dashboard hostname for this stack —
	// e.g. `app.eu.ironflyer.dev`, `app.staging.ironflyer.dev`.
	Domain pulumi.StringInput

	// Branch is the production branch in the linked git repo. Typically
	// `main` for prod stacks; preview deployments still attach
	// automatically to other branches.
	Branch pulumi.StringInput

	// FrameworkPreset is the Vercel framework slug. Defaults to "nextjs"
	// when empty — the dashboard is a Next.js 15 app.
	FrameworkPreset pulumi.StringInput

	// GitRepoOwner + GitRepoName optionally bind the project to a GitHub
	// repo so Vercel auto-deploys on push. When both are empty the
	// project is created standalone (operator imports later).
	GitRepoOwner pulumi.StringInput
	GitRepoName  pulumi.StringInput

	// OrchestratorAPIHost is the orchestrator's public hostname (e.g.
	// `api.eu.ironflyer.dev`). Used to populate
	// NEXT_PUBLIC_IRONFLYER_API_URL + NEXT_PUBLIC_IRONFLYER_WS_URL.
	OrchestratorAPIHost pulumi.StringInput

	// SentryDSN is the in-cluster Sentry DSN. Wired into the dashboard's
	// production env so frontend errors surface in the same project.
	// Typically pulled from the data stack's `data.secrets.arns` output
	// and resolved by the operator out-of-band; a placeholder is fine.
	SentryDSN pulumi.StringInput
}

// VercelOutputs is the slice of state main.go re-exports.
type VercelOutputs struct {
	ProjectID         pulumi.StringOutput
	ProductionURL     pulumi.StringOutput
	PreviewURLPattern pulumi.StringOutput
}

// NewVercel provisions the Vercel project + production domain + runtime
// env vars for one Ironflyer stack. Resources are tagged through the
// shared compute.Config helpers so they show up alongside AWS resources
// in the org-wide tag explorer.
func NewVercel(ctx *pulumi.Context, cfg *compute.Config, args *VercelArgs, opts ...pulumi.ResourceOption) (*VercelOutputs, error) {
	// Dedicated provider so the api token + team default are explicit.
	prov, err := vercel.NewProvider(ctx, "vercel-"+cfg.Stack, &vercel.ProviderArgs{
		ApiToken: args.APIToken,
		Team:     args.TeamID,
	})
	if err != nil {
		return nil, err
	}
	withProv := append(opts, pulumi.Provider(prov))

	// Default framework to nextjs when not supplied — the dashboard is a
	// Next.js 15 app.
	framework := pulumi.StringInput(pulumi.String("nextjs"))
	if args.FrameworkPreset != nil {
		framework = args.FrameworkPreset
	}

	projArgs := &vercel.ProjectArgs{
		Name:      pulumi.String(cfg.Stack + "-ironflyer-dashboard"),
		Framework: framework.ToStringOutput().ApplyT(func(s string) *string { v := s; return &v }).(pulumi.StringPtrInput),
		TeamId:    args.TeamID,
	}

	// Git repo binding is optional — only wire it when both owner + name
	// are supplied. Otherwise the operator hooks the repo manually in the
	// Vercel UI.
	if args.GitRepoOwner != nil && args.GitRepoName != nil {
		repo := pulumi.All(args.GitRepoOwner, args.GitRepoName).ApplyT(func(v []interface{}) string {
			owner, _ := v[0].(string)
			name, _ := v[1].(string)
			if owner == "" || name == "" {
				return ""
			}
			return owner + "/" + name
		}).(pulumi.StringOutput)

		projArgs.GitRepository = vercel.ProjectGitRepositoryArgs{
			Type:             pulumi.String("github"),
			Repo:             repo,
			ProductionBranch: args.Branch.ToStringOutput().ApplyT(func(s string) *string { v := s; return &v }).(pulumi.StringPtrInput),
		}
	}

	project, err := vercel.NewProject(ctx, "ironflyer-dashboard-"+cfg.Stack, projArgs, withProv...)
	if err != nil {
		return nil, err
	}

	// Production runtime env vars. Each value is treated as a Pulumi
	// secret on the resource side (the SDK marks `value` AdditionalSecret).
	apiURL := args.OrchestratorAPIHost.ToStringOutput().ApplyT(func(h string) string {
		if h == "" {
			return ""
		}
		return "https://" + h
	}).(pulumi.StringOutput)
	wsURL := args.OrchestratorAPIHost.ToStringOutput().ApplyT(func(h string) string {
		if h == "" {
			return ""
		}
		return "wss://" + h + "/graphql"
	}).(pulumi.StringOutput)

	envVars := []struct {
		key   string
		value pulumi.StringInput
	}{
		{"NEXT_PUBLIC_IRONFLYER_API_URL", apiURL},
		{"NEXT_PUBLIC_IRONFLYER_WS_URL", wsURL},
		{"NEXT_PUBLIC_SENTRY_DSN", args.SentryDSN},
	}
	for _, ev := range envVars {
		if ev.value == nil {
			continue
		}
		if _, err := vercel.NewProjectEnvironmentVariable(ctx, "ironflyer-dashboard-env-"+cfg.Stack+"-"+ev.key, &vercel.ProjectEnvironmentVariableArgs{
			ProjectId: project.ID(),
			TeamId:    args.TeamID,
			Key:       pulumi.String(ev.key),
			Value:     ev.value,
			Targets:   pulumi.StringArray{pulumi.String("production")},
			Comment:   pulumi.String("ironflyer.io/stack=" + cfg.Stack + "; ironflyer.io/region=" + cfg.Region),
		}, withProv...); err != nil {
			return nil, err
		}
	}

	// Attach the customer-facing domain. Vercel issues
	// `cname.vercel-dns.com` as the target — the AddVercelCNAME helper
	// below wires Route53 to it.
	if _, err := vercel.NewProjectDomain(ctx, "ironflyer-dashboard-domain-"+cfg.Stack, &vercel.ProjectDomainArgs{
		ProjectId: project.ID(),
		TeamId:    args.TeamID,
		Domain:    args.Domain,
		GitBranch: args.Branch.ToStringOutput().ApplyT(func(s string) *string { v := s; return &v }).(pulumi.StringPtrInput),
	}, withProv...); err != nil {
		return nil, err
	}

	productionURL := args.Domain.ToStringOutput().ApplyT(func(d string) string {
		if d == "" {
			return ""
		}
		return "https://" + d
	}).(pulumi.StringOutput)
	previewPattern := pulumi.Sprintf("https://%s-ironflyer-dashboard-*.vercel.app", cfg.Stack)

	return &VercelOutputs{
		ProjectID:         project.ID().ToStringOutput(),
		ProductionURL:     productionURL,
		PreviewURLPattern: previewPattern,
	}, nil
}
