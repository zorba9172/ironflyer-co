package compute

import (
	"strconv"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// Config captures every setting the program reads from Pulumi config.
// Kept as plain values (not Pulumi Outputs) — they're resolved at startup
// and reused everywhere downstream. Mirrors compute.Config in the AWS
// project; the field set is DO-shaped (droplet sizes, region slugs).
type Config struct {
	Stack         string
	ProjectName   string
	Region        string
	SpacesRegion  string
	RootDomain    string
	K8sVersion    string

	// System node pool (control-plane facing workloads — orchestrator API,
	// edge controllers, observability stack).
	DOKSNodeSize  string
	DOKSNodeCount int
	DOKSMaxNodes  int

	// Runtime node pool (per-user sandbox pods — code-server, runtime
	// driver). Larger droplets, separately scaled, labelled `workload=runtime`
	// and tainted so only runtime pods land there.
	DOKSRuntimeNodeSize  string
	DOKSRuntimeNodeCount int
	DOKSRuntimeMaxNodes  int

	// Data layer sizing — consumed by the sibling `data` package.
	PostgresSize    string
	PostgresVersion string
	RedisSize       string
	RedisVersion   string

	// HA + edge toggles.
	EnableHA      bool
	VercelEnabled bool
	VercelDomain  string
}

// LoadConfig reads every value the program needs from Pulumi config under
// the `ironflyer:` namespace. Defaults match Pulumi.yaml.
func LoadConfig(ctx *pulumi.Context) (*Config, error) {
	c := config.New(ctx, "ironflyer")

	cfg := &Config{
		Stack:                ctx.Stack(),
		ProjectName:          "ironflyer",
		Region:               c.Require("region"),
		SpacesRegion:         getOr(c, "spacesRegion", c.Require("region")),
		RootDomain:           getOr(c, "rootDomain", "ironflyer.dev"),
		K8sVersion:           getOr(c, "k8sVersion", "1.30.6-do.0"),
		DOKSNodeSize:         getOr(c, "doksNodeSize", "s-2vcpu-4gb"),
		DOKSNodeCount:        getInt(c, "doksNodeCount", 2),
		DOKSMaxNodes:         getInt(c, "doksMaxNodes", 6),
		DOKSRuntimeNodeSize:  getOr(c, "doksRuntimeNodeSize", "s-4vcpu-8gb"),
		DOKSRuntimeNodeCount: getInt(c, "doksRuntimeNodeCount", 1),
		DOKSRuntimeMaxNodes:  getInt(c, "doksRuntimeMaxNodes", 8),
		PostgresSize:         getOr(c, "postgresSize", "db-s-1vcpu-1gb"),
		PostgresVersion:      getOr(c, "postgresVersion", "16"),
		RedisSize:            getOr(c, "redisSize", "db-s-1vcpu-1gb"),
		RedisVersion:         getOr(c, "redisVersion", "7"),
		EnableHA:             c.GetBool("enableHA"),
		VercelEnabled:        c.GetBool("vercelEnabled"),
		VercelDomain:         c.Get("vercelDomain"),
	}
	return cfg, nil
}

// Tags returns the standard tag slice every DO resource gets. DigitalOcean
// resources accept a `[]string` of free-form tags (not key/value maps like
// AWS), so we encode our well-known key:value pairs as colon-separated
// strings to keep them searchable in the DO UI + API.
func (c *Config) Tags(extra ...string) pulumi.StringArray {
	base := []string{
		"ironflyer",
		"project:" + c.ProjectName,
		"stack:" + c.Stack,
		"region:" + c.Region,
		"managed-by:pulumi",
	}
	base = append(base, extra...)
	out := make(pulumi.StringArray, 0, len(base))
	for _, t := range base {
		out = append(out, pulumi.String(t))
	}
	return out
}

// ResourceName returns a stable, stack-scoped DO resource name (e.g.
// `ironflyer-dev-vpc`). DigitalOcean enforces lowercase + DNS-label charset
// for most resources, which our stack names already respect.
func (c *Config) ResourceName(suffix string) string {
	return strings.ToLower(c.ProjectName + "-" + c.Stack + "-" + suffix)
}

// APIHostname is the front-door hostname for the orchestrator API in this
// stack — used by edge for ingress + cert wiring.
func (c *Config) APIHostname() string {
	return "api." + c.RootDomain
}

// WebHostname is the dashboard hostname. Falls back to VercelDomain when
// Vercel is enabled (Vercel owns the apex in those stacks).
func (c *Config) WebHostname() string {
	if c.VercelEnabled && c.VercelDomain != "" {
		return c.VercelDomain
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

func getInt(c *config.Config, key string, fallback int) int {
	v := c.Get(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return fallback
	}
	return n
}
