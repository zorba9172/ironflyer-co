package data

import (
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// provisionNamespace ensures the `ironflyer` namespace exists. Every
// in-cluster Secret + Job + Helm release the data layer creates lands
// here so the orchestrator + runtime + edge can RBAC against a single
// scoped namespace. We label it `app.kubernetes.io/part-of=ironflyer`
// so observability dashboards can group resources easily.
func provisionNamespace(ctx *pulumi.Context, in Inputs) (*corev1.Namespace, error) {
	if in.K8sProvider == nil {
		return nil, nil
	}
	ns, err := corev1.NewNamespace(ctx, in.Config.ResourceName("ns"), &corev1.NamespaceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("ironflyer"),
			Labels: pulumi.StringMap{
				"app.kubernetes.io/part-of": pulumi.String("ironflyer"),
				"ironflyer.dev/stack":       pulumi.String(in.Config.Stack),
			},
		},
	}, pulumi.Provider(in.K8sProvider))
	if err != nil {
		return nil, err
	}
	return ns, nil
}

// provisionSecrets mirrors the managed-DB + Spaces connection details
// into Kubernetes Secrets the orchestrator reads via standard envFrom.
//
// We intentionally piggy-back on the orchestrator's existing `R2_*` env
// var names for Spaces — Spaces is S3-compatible with an endpoint URL,
// which is exactly the wire shape the `S3_BACKEND=r2` path expects. The
// orchestrator-polish agent will add a first-class `spaces` backend
// later; today this keeps deploys working without a new orchestrator
// release. The `S3_BACKEND` knob is set in the Helm chart, not here.
func provisionSecrets(ctx *pulumi.Context, in Inputs, ns *corev1.Namespace, pg *Postgres, redis *Redis, spaces *Spaces) error {
	if in.K8sProvider == nil || ns == nil {
		return nil
	}

	opts := []pulumi.ResourceOption{
		pulumi.Provider(in.K8sProvider),
		pulumi.DependsOn([]pulumi.Resource{ns}),
	}

	// Postgres — prefer the PrivateUri so k8s pods route over the VPC.
	if _, err := corev1.NewSecret(ctx, in.Config.ResourceName("secret-postgres"), &corev1.SecretArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("ironflyer-postgres"),
			Namespace: ns.Metadata.Name().Elem(),
			Labels: pulumi.StringMap{
				"app.kubernetes.io/part-of":   pulumi.String("ironflyer"),
				"app.kubernetes.io/component": pulumi.String("data-postgres"),
			},
		},
		StringData: pulumi.StringMap{
			"POSTGRES_URL":      pg.PrivateURI,
			"POSTGRES_HOST":     pg.PrivateHost,
			"POSTGRES_DATABASE": pulumi.String("ironflyer").ToStringOutput(),
		},
	}, opts...); err != nil {
		return err
	}

	// Redis / Valkey — same shape; the URI carries auth + scheme.
	if _, err := corev1.NewSecret(ctx, in.Config.ResourceName("secret-redis"), &corev1.SecretArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("ironflyer-redis"),
			Namespace: ns.Metadata.Name().Elem(),
			Labels: pulumi.StringMap{
				"app.kubernetes.io/part-of":   pulumi.String("ironflyer"),
				"app.kubernetes.io/component": pulumi.String("data-redis"),
			},
		},
		StringData: pulumi.StringMap{
			"REDIS_URL":      redis.PrivateURI,
			"REDIS_HOST":     redis.PrivateHost,
			"REDIS_PASSWORD": redis.Password,
		},
	}, opts...); err != nil {
		return err
	}

	// Spaces — exposed under the orchestrator's R2_* keys (S3-compatible
	// path). BACKUP_S3_URI points at the dedicated backups bucket so the
	// backup CronJob doesn't need separate config wiring.
	backupURI := spaces.Names["backups"].ApplyT(func(name string) string {
		return "s3://" + name
	}).(pulumi.StringOutput)

	if _, err := corev1.NewSecret(ctx, in.Config.ResourceName("secret-spaces"), &corev1.SecretArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("ironflyer-spaces"),
			Namespace: ns.Metadata.Name().Elem(),
			Labels: pulumi.StringMap{
				"app.kubernetes.io/part-of":   pulumi.String("ironflyer"),
				"app.kubernetes.io/component": pulumi.String("data-spaces"),
			},
		},
		StringData: pulumi.StringMap{
			// R2_* alias intentional — see package doc above.
			"R2_ACCOUNT_ID":        pulumi.String("digitalocean-spaces").ToStringOutput(),
			"R2_ACCESS_KEY_ID":     spaces.AccessKey,
			"R2_SECRET_ACCESS_KEY": spaces.SecretKey,
			"SPACES_ENDPOINT":      spaces.Endpoint,
			"SPACES_REGION":        pulumi.String(in.Config.SpacesRegion).ToStringOutput(),
			"BACKUP_S3_URI":        backupURI,
			"WORKSPACES_BUCKET":    spaces.Names["workspaces"],
			"AUDIT_EXPORTS_BUCKET": spaces.Names["audit-exports"],
		},
	}, opts...); err != nil {
		return err
	}

	return nil
}
