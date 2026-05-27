// Package data is the data layer for the Ironflyer DigitalOcean Pulumi
// project. It owns:
//
//   - Managed Postgres (with pgvector + pgcrypto installed via a one-shot
//     Kubernetes Job; DO's provider has no first-class
//     `DatabaseExtension` resource at the time of writing).
//   - Managed Valkey/Redis.
//   - Spaces buckets: `backups` (30d), `workspaces` (7d), `audit-exports`
//     (180d, CORS-enabled for browser downloads).
//   - A Spaces API key the orchestrator + backup CronJob use to sign
//     S3-compatible requests.
//   - Kubernetes Secrets in the `ironflyer` namespace surfacing the
//     connection details to the orchestrator (POSTGRES_URL, REDIS_URL,
//     R2_*-aliased Spaces creds — Spaces is wire-compatible with the
//     orchestrator's existing R2 backend, see README in storage/).
//   - Optional kube-prometheus-stack + loki-stack observability bundle.
//
// The Inputs / Outputs structs are the contract main.go holds with the
// data layer; the compute + edge agents own the foundation and the
// public surface, respectively.
package data

import (
	"fmt"

	"github.com/pulumi/pulumi-digitalocean/sdk/v4/go/digitalocean"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"ironflyer/infra/pulumi-do/compute"
)

// Inputs is the compute-layer handoff the data package consumes. Every
// field here is something only the compute layer can produce.
type Inputs struct {
	Config      *compute.Config
	Network     *compute.Network
	Cluster     *digitalocean.KubernetesCluster
	K8sProvider *kubernetes.Provider
}

// Outputs are the data-layer outputs that main.go exports and the edge
// layer consumes. Field set is a superset of the original stub's
// contract so the foundation main.go keeps compiling while richer fields
// (PostgresURI, PrivateHost, the SpacesBuckets map) are also exposed for
// the edge layer + orchestrator Helm values.
type Outputs struct {
	// Postgres.
	PostgresHost          pulumi.StringOutput
	PostgresPrivateHost   pulumi.StringOutput
	PostgresPort          pulumi.IntOutput
	PostgresDatabase      pulumi.StringOutput
	PostgresUser          pulumi.StringOutput
	PostgresPassword      pulumi.StringOutput // secret
	PostgresConnectionURI pulumi.StringOutput // secret — public URI
	PostgresURI           pulumi.StringOutput // alias, kept for the data-layer task contract
	PostgresPrivateURI    pulumi.StringOutput // secret — VPC URI, preferred from in-cluster pods

	// Redis (Valkey).
	RedisHost          pulumi.StringOutput
	RedisPort          pulumi.IntOutput
	RedisPassword      pulumi.StringOutput // secret
	RedisConnectionURI pulumi.StringOutput // secret
	RedisURI           pulumi.StringOutput // alias

	// Spaces.
	SpacesEndpoint  pulumi.StringOutput
	SpacesRegion    pulumi.StringOutput
	SpacesBucket    pulumi.StringOutput            // primary bucket name (backups) — kept for the stub contract
	SpacesBuckets   map[string]pulumi.StringOutput // logical name → bucket domain name
	SpacesAccessKey pulumi.StringOutput            // secret
	SpacesSecretKey pulumi.StringOutput            // secret
}

// Provision is the data layer's single entry point. main.go calls this
// once the compute layer has produced the network, the DOKS cluster,
// and an authenticated kubernetes provider.
//
// Order of operations:
//
//  1. Namespace (`ironflyer`) — every k8s resource the data layer
//     creates lives here.
//  2. Managed Postgres (with pgvector bootstrap Job).
//  3. Managed Valkey/Redis.
//  4. Spaces buckets (backups / workspaces / audit-exports) + API key.
//  5. K8s Secrets that surface the connection strings to the
//     orchestrator pods (POSTGRES_URL, REDIS_URL, R2_* for Spaces).
//  6. Observability add-ons (kube-prometheus + loki) — gated on
//     `ironflyer:observabilityEnabled`.
//
// All steps that touch the cluster are skipped when in.K8sProvider is
// nil, which keeps `pulumi preview` healthy before the compute layer
// has wired the kube provider through.
func Provision(ctx *pulumi.Context, in Inputs) (*Outputs, error) {
	if in.Config == nil {
		return nil, fmt.Errorf("data.Provision: Inputs.Config is required")
	}
	if in.Network == nil {
		return nil, fmt.Errorf("data.Provision: Inputs.Network is required")
	}
	if in.Cluster == nil {
		return nil, fmt.Errorf("data.Provision: Inputs.Cluster is required")
	}

	ns, err := provisionNamespace(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("namespace: %w", err)
	}

	pg, err := provisionPostgres(ctx, in, ns)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}

	redis, err := provisionRedis(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("redis: %w", err)
	}

	spaces, err := provisionSpaces(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("spaces: %w", err)
	}

	if err := provisionSecrets(ctx, in, ns, pg, redis, spaces); err != nil {
		return nil, fmt.Errorf("secrets: %w", err)
	}

	if err := provisionObservability(ctx, in); err != nil {
		return nil, fmt.Errorf("observability: %w", err)
	}

	region := in.Config.SpacesRegion
	if region == "" {
		region = in.Config.Region
	}

	return &Outputs{
		PostgresHost:          pg.Host,
		PostgresPrivateHost:   pg.PrivateHost,
		PostgresPort:          pg.Port,
		PostgresDatabase:      pulumi.String("ironflyer").ToStringOutput(),
		PostgresUser:          pg.Cluster.User,
		PostgresPassword:      pg.Cluster.Password,
		PostgresConnectionURI: pg.URI,
		PostgresURI:           pg.URI,
		PostgresPrivateURI:    pg.PrivateURI,
		RedisHost:             redis.PrivateHost,
		RedisPort:             redis.Port,
		RedisPassword:         redis.Password,
		RedisConnectionURI:    redis.PrivateURI,
		RedisURI:              redis.PrivateURI,
		SpacesEndpoint:        spaces.Endpoint,
		SpacesRegion:          pulumi.String(region).ToStringOutput(),
		SpacesBucket:          spaces.Names["backups"],
		SpacesBuckets:         spaces.Buckets,
		SpacesAccessKey:       spaces.AccessKey,
		SpacesSecretKey:       spaces.SecretKey,
	}, nil
}
