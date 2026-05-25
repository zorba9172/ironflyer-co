package data

import (
	"github.com/pulumi/pulumi-digitalocean/sdk/v4/go/digitalocean"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	batchv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/batch/v1"

	"ironflyer/infra/pulumi-do/compute"
)

// Postgres is the managed DO Postgres cluster + the outputs the rest of
// the data layer (secrets, observability) and the edge layer consume.
type Postgres struct {
	Cluster     *digitalocean.DatabaseCluster
	Host        pulumi.StringOutput
	Port        pulumi.IntOutput
	PrivateHost pulumi.StringOutput
	PrivatePort pulumi.IntOutput
	URI         pulumi.StringOutput // public-facing, secret
	PrivateURI  pulumi.StringOutput // VPC-only, secret — preferred by k8s workloads
}

// provisionPostgres creates the managed Postgres cluster, a tenant
// database + app user, a trusted-source firewall that only admits the
// DOKS cluster, and a Kubernetes Job that installs the pgvector
// extension on first apply.
//
// DO managed Postgres exposes pgvector as an available extension; the
// provider SDK has no first-class DatabaseExtension resource at the
// time of writing, so we run a one-shot k8s Job that connects via the
// cluster's PrivateURI and issues `CREATE EXTENSION IF NOT EXISTS vector`.
// The Job is idempotent (the IF NOT EXISTS guard) and lives in the
// `ironflyer` namespace next to the orchestrator.
func provisionPostgres(ctx *pulumi.Context, in Inputs, ns *corev1.Namespace) (*Postgres, error) {
	cfg := in.Config
	clusterName := cfg.ResourceName("pg")

	cluster, err := digitalocean.NewDatabaseCluster(ctx, clusterName, &digitalocean.DatabaseClusterArgs{
		Name:               pulumi.String(clusterName),
		Engine:             pulumi.String("pg"),
		Version:            pulumi.String(cfg.PostgresVersion),
		Size:               pulumi.String(cfg.PostgresSize),
		Region:             pulumi.String(cfg.Region),
		NodeCount:          pulumi.Int(IfHA(cfg, 2, 1)),
		PrivateNetworkUuid: in.Network.VpcID.ToStringOutput(),
		Tags:               cfg.Tags("data", "postgres"),
		MaintenanceWindows: digitalocean.DatabaseClusterMaintenanceWindowArray{
			&digitalocean.DatabaseClusterMaintenanceWindowArgs{
				Day:  pulumi.String("sunday"),
				Hour: pulumi.String("03:00:00"),
			},
		},
	})
	if err != nil {
		return nil, err
	}

	if _, err := digitalocean.NewDatabaseDb(ctx, cfg.ResourceName("pg-db"), &digitalocean.DatabaseDbArgs{
		ClusterId: cluster.ID(),
		Name:      pulumi.String("ironflyer"),
	}); err != nil {
		return nil, err
	}

	if _, err := digitalocean.NewDatabaseUser(ctx, cfg.ResourceName("pg-user"), &digitalocean.DatabaseUserArgs{
		ClusterId: cluster.ID(),
		Name:      pulumi.String("ironflyer_app"),
	}); err != nil {
		return nil, err
	}

	// Trusted-source firewall: only admit the DOKS cluster itself. DO
	// resolves this to the cluster's worker droplets internally.
	if _, err := digitalocean.NewDatabaseFirewall(ctx, cfg.ResourceName("pg-fw"), &digitalocean.DatabaseFirewallArgs{
		ClusterId: cluster.ID(),
		Rules: digitalocean.DatabaseFirewallRuleArray{
			&digitalocean.DatabaseFirewallRuleArgs{
				Type:  pulumi.String("k8s"),
				Value: in.Cluster.ID().ToStringOutput(),
			},
		},
	}); err != nil {
		return nil, err
	}

	// pgvector extension via a one-shot k8s Job. Skipped when the
	// kubernetes provider isn't wired in yet (compute agent still bringing
	// the cluster online), which keeps `pulumi preview` working before
	// the cluster exists.
	if in.K8sProvider != nil && ns != nil {
		if err := installPgVector(ctx, cfg, cluster, in, ns); err != nil {
			return nil, err
		}
	}

	return &Postgres{
		Cluster:     cluster,
		Host:        cluster.Host,
		Port:        cluster.Port,
		PrivateHost: cluster.PrivateHost,
		PrivatePort: cluster.Port,
		URI:         cluster.Uri,
		PrivateURI:  cluster.PrivateUri,
	}, nil
}

// installPgVector creates a Kubernetes Job that runs `psql` against the
// managed cluster's PrivateURI and installs the pgvector extension. The
// Job runs once per `pulumi up` (resource name + image hash are stable);
// the CREATE EXTENSION IF NOT EXISTS guard makes it idempotent if it
// ever re-runs.
func installPgVector(ctx *pulumi.Context, cfg *compute.Config, cluster *digitalocean.DatabaseCluster, in Inputs, ns *corev1.Namespace) error {
	opts := []pulumi.ResourceOption{
		pulumi.Provider(in.K8sProvider),
		pulumi.DependsOn([]pulumi.Resource{cluster, ns}),
	}

	// Wrap the cluster URI in a k8s Secret so we don't bake the password
	// into the Job's command line / pod spec.
	uriSecret, err := corev1.NewSecret(ctx, cfg.ResourceName("pg-bootstrap-uri"), &corev1.SecretArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("ironflyer-pg-bootstrap-uri"),
			Namespace: ns.Metadata.Name().Elem(),
		},
		StringData: pulumi.StringMap{
			"DATABASE_URL": cluster.PrivateUri,
		},
	}, opts...)
	if err != nil {
		return err
	}

	_, err = batchv1.NewJob(ctx, cfg.ResourceName("pg-bootstrap"), &batchv1.JobArgs{
		Metadata: &metav1.ObjectMetaArgs{
			GenerateName: pulumi.String("ironflyer-pg-bootstrap-"),
			Namespace:    ns.Metadata.Name().Elem(),
			Labels: pulumi.StringMap{
				"app.kubernetes.io/name":      pulumi.String("ironflyer-pg-bootstrap"),
				"app.kubernetes.io/component": pulumi.String("data-bootstrap"),
				"app.kubernetes.io/part-of":   pulumi.String("ironflyer"),
			},
		},
		Spec: &batchv1.JobSpecArgs{
			BackoffLimit:            pulumi.Int(6),
			TtlSecondsAfterFinished: pulumi.Int(3600),
			Template: &corev1.PodTemplateSpecArgs{
				Metadata: &metav1.ObjectMetaArgs{
					Labels: pulumi.StringMap{
						"app.kubernetes.io/name": pulumi.String("ironflyer-pg-bootstrap"),
					},
				},
				Spec: &corev1.PodSpecArgs{
					RestartPolicy: pulumi.String("OnFailure"),
					Containers: corev1.ContainerArray{
						&corev1.ContainerArgs{
							Name:  pulumi.String("psql"),
							Image: pulumi.String("postgres:16-alpine"),
							Env: corev1.EnvVarArray{
								&corev1.EnvVarArgs{
									Name: pulumi.String("DATABASE_URL"),
									ValueFrom: &corev1.EnvVarSourceArgs{
										SecretKeyRef: &corev1.SecretKeySelectorArgs{
											Name: uriSecret.Metadata.Name().Elem(),
											Key:  pulumi.String("DATABASE_URL"),
										},
									},
								},
							},
							Command: pulumi.StringArray{
								pulumi.String("/bin/sh"),
								pulumi.String("-c"),
							},
							Args: pulumi.StringArray{
								pulumi.String(`set -e
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -c "CREATE EXTENSION IF NOT EXISTS vector;"
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -c "CREATE EXTENSION IF NOT EXISTS pgcrypto;"
echo "ironflyer: pgvector + pgcrypto installed."`),
							},
						},
					},
				},
			},
		},
	}, opts...)
	return err
}
