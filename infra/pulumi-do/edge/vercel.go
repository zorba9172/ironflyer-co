// Vercel surface for the Ironflyer dashboard — DigitalOcean edition.
//
// Mirrors `infra/pulumi/edge/vercel.go` (AWS). The orchestrator runs on
// DOKS (compute + data packages); the dashboard runs on Vercel and is
// wired to the orchestrator's public API via env vars. A Cloudflare
// CNAME (see cloudflare.go) attaches `app.<root>` to Vercel's edge.
package edge

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	"github.com/pulumiverse/pulumi-vercel/sdk/v3/go/vercel"
)

// newVercel provisions the Vercel project, the production env vars the
// Next.js dashboard reads at runtime, and the production domain
// (`app.<root>`). Gated on cfg.VercelEnabled — dev stacks that don't
// want a Vercel project skip the whole thing.
func newVercel(ctx *pulumi.Context, in Inputs) error {
	cfg := in.Config
	if !cfg.VercelEnabled {
		return nil
	}
	host := appHostname(cfg)
	if host == "" {
		ctx.Log.Warn("edge.vercel: vercelEnabled=true but no vercelDomain or rootDomain; skipping", nil)
		return nil
	}

	vConfig := config.New(ctx, "vercel")
	apiToken := vConfig.RequireSecret("apiToken")
	teamID := vConfig.Get("teamId") // optional — personal accounts omit it

	prov, err := vercel.NewProvider(ctx, "vercel-"+cfg.Stack, &vercel.ProviderArgs{
		ApiToken: apiToken,
		Team:     pulumi.String(teamID),
	})
	if err != nil {
		return err
	}
	provOpt := pulumi.Provider(prov)

	framework := "nextjs"
	project, err := vercel.NewProject(ctx, "ironflyer-dashboard-"+cfg.Stack, &vercel.ProjectArgs{
		Name:      pulumi.String(cfg.Stack + "-ironflyer-dashboard"),
		Framework: pulumi.String(framework),
		TeamId:    pulumi.String(teamID),
	}, provOpt)
	if err != nil {
		return err
	}

	// Production runtime env vars. The api/runtime hostnames are
	// deterministic from the root domain so we don't need to wait on a
	// Cloudflare output to know them.
	apiHost := apiHostname(cfg)
	envVars := []struct {
		key   string
		value string
	}{
		{"NEXT_PUBLIC_IRONFLYER_API_URL", maybePrefix("https://", apiHost)},
		{"NEXT_PUBLIC_IRONFLYER_WS_URL", maybePrefix("wss://", apiHost) + maybeSuffix("/graphql", apiHost)},
		// Sentry DSN is operator-set out of band; the placeholder keeps
		// the resource shape stable so a later config-only change adds
		// the real DSN without churning state.
		{"NEXT_PUBLIC_SENTRY_DSN", vConfig.Get("sentryDsn")},
	}
	for _, ev := range envVars {
		if ev.value == "" {
			continue
		}
		if _, err := vercel.NewProjectEnvironmentVariable(ctx, "ironflyer-dashboard-env-"+cfg.Stack+"-"+ev.key, &vercel.ProjectEnvironmentVariableArgs{
			ProjectId: project.ID(),
			TeamId:    pulumi.String(teamID),
			Key:       pulumi.String(ev.key),
			Value:     pulumi.String(ev.value),
			Targets:   pulumi.StringArray{pulumi.String("production")},
			Comment:   pulumi.String("ironflyer.io/stack=" + cfg.Stack + "; ironflyer.io/region=" + cfg.Region),
		}, provOpt); err != nil {
			return err
		}
	}

	// Attach the customer-facing domain. Vercel issues
	// `cname.vercel-dns.com` as the DNS target — the Cloudflare CNAME in
	// cloudflare.go points at it.
	if _, err := vercel.NewProjectDomain(ctx, "ironflyer-dashboard-domain-"+cfg.Stack, &vercel.ProjectDomainArgs{
		ProjectId: project.ID(),
		TeamId:    pulumi.String(teamID),
		Domain:    pulumi.String(host),
	}, provOpt); err != nil {
		return err
	}

	return nil
}

func maybePrefix(p, s string) string {
	if s == "" {
		return ""
	}
	return p + s
}

func maybeSuffix(suffix, s string) string {
	if s == "" {
		return ""
	}
	return suffix
}
