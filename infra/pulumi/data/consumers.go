package data

import (
	"fmt"

	kubernetes "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	apiextensions "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apiextensions"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	helmv3 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v3"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// provisionConsumers wires AWS Secrets Manager into the cluster:
//
//  1. Installs the external-secrets operator (Helm).
//  2. Creates a ClusterSecretStore that authenticates against AWS Secrets
//     Manager via the orchestrator IRSA role.
//  3. For each AWS secret created in secrets.go, mints an ExternalSecret
//     that materializes a Kubernetes Secret in the `ironflyer` namespace
//     with a stable name + key layout the existing Helm chart already
//     references via `valueFrom.secretKeyRef`.
//
// Refresh interval: 1 hour. That keeps post-rotation propagation under
// an hour without hammering Secrets Manager's API quota.
func provisionConsumers(ctx *pulumi.Context, env *stackEnv, deps Compute, secrets *Secrets) error {
	if deps.K8sProvider == nil {
		// No cluster yet — compute agent still needs to publish a kube
		// provider. Skip cleanly so dry-runs work.
		return nil
	}

	opts := []pulumi.ResourceOption{pulumi.Provider(deps.K8sProvider)}

	esNs, err := corev1.NewNamespace(ctx, name(env, "external-secrets-ns"), &corev1.NamespaceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("external-secrets"),
		},
	}, opts...)
	if err != nil {
		return fmt.Errorf("external-secrets ns: %w", err)
	}

	esRelease, err := helmv3.NewRelease(ctx, name(env, "external-secrets"), &helmv3.ReleaseArgs{
		Chart:           pulumi.String("external-secrets"),
		Version:         pulumi.String("0.10.4"),
		Namespace:       esNs.Metadata.Name().Elem(),
		CreateNamespace: pulumi.Bool(false),
		RepositoryOpts: &helmv3.RepositoryOptsArgs{
			Repo: pulumi.String("https://charts.external-secrets.io"),
		},
		Values: pulumi.Map{
			"installCRDs": pulumi.Bool(true),
		},
	}, opts...)
	if err != nil {
		return fmt.Errorf("external-secrets helm release: %w", err)
	}

	releaseDep := []pulumi.ResourceOption{
		pulumi.Provider(deps.K8sProvider),
		pulumi.DependsOn([]pulumi.Resource{esRelease}),
	}

	// Make sure the workload namespace exists (chart usually creates it,
	// but we own the K8s Secret targets so we keep this explicit).
	if _, err := corev1.NewNamespace(ctx, name(env, "ironflyer-ns"), &corev1.NamespaceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("ironflyer"),
		},
	}, opts...); err != nil {
		return fmt.Errorf("ironflyer ns: %w", err)
	}

	// ClusterSecretStore -> AWS Secrets Manager via IRSA.
	store, err := apiextensions.NewCustomResource(ctx, name(env, "css-aws"), &apiextensions.CustomResourceArgs{
		ApiVersion: pulumi.String("external-secrets.io/v1beta1"),
		Kind:       pulumi.String("ClusterSecretStore"),
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("ironflyer-aws"),
		},
		OtherFields: map[string]any{
			"spec": map[string]any{
				"provider": map[string]any{
					"aws": map[string]any{
						"service": "SecretsManager",
						"region":  env.region,
						"auth": map[string]any{
							"jwt": map[string]any{
								"serviceAccountRef": map[string]any{
									"name":      "orchestrator-sa",
									"namespace": "ironflyer",
								},
							},
						},
					},
				},
			},
		},
	}, releaseDep...)
	if err != nil {
		return fmt.Errorf("cluster secret store: %w", err)
	}

	storeDep := append(releaseDep, pulumi.DependsOn([]pulumi.Resource{store}))

	// One ExternalSecret per AWS secret. The resulting K8s Secret name is
	// `ironflyer-<logical>` and exposes the JSON payload as well as the
	// top-level scalar keys the orchestrator's Helm chart expects.
	for _, logical := range secrets.LogicalNames() {
		awsSec := secrets.SecretByLogical(logical)
		if awsSec == nil {
			continue
		}
		secretName := "ironflyer-" + logical

		// We pass the AWS secret name via the resolved Name output so we
		// don't hard-code the env.stack prefix in two places.
		_, err := apiextensions.NewCustomResource(ctx, name(env, "es-"+logical), &apiextensions.CustomResourceArgs{
			ApiVersion: pulumi.String("external-secrets.io/v1beta1"),
			Kind:       pulumi.String("ExternalSecret"),
			Metadata: &metav1.ObjectMetaArgs{
				Name:      pulumi.String(secretName),
				Namespace: pulumi.String("ironflyer"),
				Labels: pulumi.StringMap{
					"app.kubernetes.io/part-of":  pulumi.String("ironflyer"),
					"ironflyer.io/secret-source": pulumi.String("aws-secrets-manager"),
				},
			},
			OtherFields: kubernetes.UntypedArgs{
				"spec": pulumi.Map{
					"refreshInterval": pulumi.String("1h"),
					"secretStoreRef": pulumi.Map{
						"name": pulumi.String("ironflyer-aws"),
						"kind": pulumi.String("ClusterSecretStore"),
					},
					"target": pulumi.Map{
						"name":           pulumi.String(secretName),
						"creationPolicy": pulumi.String("Owner"),
						"template": pulumi.Map{
							"type": pulumi.String("Opaque"),
							"metadata": pulumi.Map{
								"labels": pulumi.Map{
									"app.kubernetes.io/part-of": pulumi.String("ironflyer"),
								},
							},
						},
					},
					"dataFrom": pulumi.Array{
						pulumi.Map{
							"extract": pulumi.Map{
								"key": awsSec.Name,
							},
						},
					},
				},
			},
		}, storeDep...)
		if err != nil {
			return fmt.Errorf("external secret %s: %w", logical, err)
		}
	}

	// Companion ConfigMap that lists secret -> mount path for the
	// orchestrator's bootstrap reflection.
	mappings := pulumi.StringMap{}
	for _, logical := range secrets.LogicalNames() {
		mappings[logical] = pulumi.String("ironflyer-" + logical)
	}
	if _, err := corev1.NewConfigMap(ctx, name(env, "secret-map"), &corev1.ConfigMapArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("ironflyer-secret-map"),
			Namespace: pulumi.String("ironflyer"),
		},
		Data: mappings,
	}, opts...); err != nil {
		return fmt.Errorf("secret-map configmap: %w", err)
	}

	return nil
}
