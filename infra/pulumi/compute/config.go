package compute

import (
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// Config captures every setting the program reads from Pulumi config.
// Kept as plain values (not Pulumi Outputs) — they're resolved at startup
// and reused everywhere downstream.
type Config struct {
	Stack            string
	Region           string
	Project          string
	VpcCidr          string
	SingleNatGateway bool
	K8sVersion       string
	PublicAPICidrs   []string
	OrchestratorType string
	RuntimeType      string
	UseKarpenter     bool
	RootDomain       string
	RegionSubdomain  string
	WebSpaBucket     string
	AllowlistedIPs   []string
	DataStackName    string

	// Vercel — drives edge.NewVercel when VercelEnabled is true. Tokens
	// stay out of plain config (operator sets `vercel:apiToken` as a
	// Pulumi secret).
	VercelEnabled      bool
	VercelTeamID       string
	VercelDomain       string
	VercelBranch       string
	VercelFramework    string
	VercelGitRepoOwner string
	VercelGitRepoName  string
	VercelSentryDSN    string
}

// LoadConfig reads every value the program needs from Pulumi config.
// Falling back to sensible defaults that match Pulumi.yaml.
func LoadConfig(ctx *pulumi.Context) *Config {
	c := config.New(ctx, "infra")
	aws := config.New(ctx, "aws")
	iron := config.New(ctx, "ironflyer")

	cfg := &Config{
		Stack:            ctx.Stack(),
		Project:          "ironflyer",
		Region:           aws.Get("region"),
		VpcCidr:          getOr(c, "vpcCidr", "10.0.0.0/16"),
		SingleNatGateway: c.GetBool("singleNatGateway"),
		K8sVersion:       getOr(c, "k8sVersion", "1.30"),
		PublicAPICidrs:   getStringArr(c, "publicApiCidrs", []string{"0.0.0.0/0"}),
		OrchestratorType: getOr(c, "orchestratorInstanceType", "m6i.large"),
		RuntimeType:      getOr(c, "runtimeInstanceType", "m6i.xlarge"),
		UseKarpenter:     c.GetBool("useKarpenter"),
		RootDomain:       getOr(c, "rootDomain", "ironflyer.dev"),
		RegionSubdomain:  c.Get("regionSubdomain"),
		WebSpaBucket:     c.Get("webSpaBucketName"),
		AllowlistedIPs:   csv(c.Get("allowlistedIps")),
		DataStackName:    c.Get("dataStackName"),

		VercelEnabled:      iron.GetBool("vercelEnabled"),
		VercelTeamID:       iron.Get("vercelTeamId"),
		VercelDomain:       iron.Get("vercelDomain"),
		VercelBranch:       getOr(iron, "vercelBranch", "main"),
		VercelFramework:    getOr(iron, "vercelFramework", "nextjs"),
		VercelGitRepoOwner: iron.Get("vercelGitRepoOwner"),
		VercelGitRepoName:  iron.Get("vercelGitRepoName"),
		VercelSentryDSN:    iron.Get("vercelSentryDsn"),
	}
	return cfg
}

// Tags returns the standard tag set every resource gets.
func (c *Config) Tags() pulumi.StringMap {
	return pulumi.StringMap{
		"Project":   pulumi.String(c.Project),
		"Stack":     pulumi.String(c.Stack),
		"Region":    pulumi.String(c.Region),
		"ManagedBy": pulumi.String("pulumi"),
	}
}

// TagsWith returns the standard tag set plus any extras.
func (c *Config) TagsWith(extra map[string]string) pulumi.StringMap {
	out := c.Tags()
	for k, v := range extra {
		out[k] = pulumi.String(v)
	}
	return out
}

// APIHostname returns the public-facing hostname for the orchestrator API
// in this stack — `api.${region}.${root}` in prod, `api.${root}` otherwise.
func (c *Config) APIHostname() string {
	if c.RegionSubdomain != "" {
		return "api." + c.RegionSubdomain + "." + c.RootDomain
	}
	return "api." + c.RootDomain
}

// WebHostname is the front-door SPA hostname (Web SPA via CloudFront).
func (c *Config) WebHostname() string {
	if c.RegionSubdomain != "" {
		return "app." + c.RegionSubdomain + "." + c.RootDomain
	}
	if strings.HasPrefix(c.Stack, "prod") {
		return "app." + c.RootDomain
	}
	return c.RootDomain
}

func getOr(c *config.Config, key, fallback string) string {
	if v := c.Get(key); v != "" {
		return v
	}
	return fallback
}

func getStringArr(c *config.Config, key string, fallback []string) []string {
	var out []string
	if err := c.TryObject(key, &out); err == nil && len(out) > 0 {
		return out
	}
	return fallback
}

func csv(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
