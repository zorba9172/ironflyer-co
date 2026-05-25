// Package k8s posts the AWS Secrets Manager entries into the EKS cluster
// as native Kubernetes Secrets using the External Secrets Operator (ESO)
// pattern. The Helm chart's `existingSecret` knobs consume these K8s
// Secrets by name, so the names here are the contract:
//
//   AWS Secret              -> K8s Secret name
//   ironflyer/postgres/...  -> ironflyer-postgres
//   ironflyer/redis/...     -> ironflyer-redis
//   ironflyer/stripe/...    -> ironflyer-stripe
//   ironflyer/anthropic/... -> ironflyer-anthropic
//   ironflyer/openai/...    -> ironflyer-openai
//   ironflyer/gemini/...    -> ironflyer-gemini
//   ironflyer/huggingface/...->ironflyer-huggingface
//   ironflyer/sentry/...    -> ironflyer-sentry
//   ironflyer/github-app/...-> ironflyer-github-app
//
// If `data:kubeconfig` is empty, this entire package is a no-op (AWS-side
// resources still provision). When the kubeconfig is provided, we
// install ESO (if `data:eso.install=true`), create a `ClusterSecretStore`
// pointing at AWS Secrets Manager, and create one `ExternalSecret` per
// AWS Secret we manage.
package k8s

import (
	"fmt"

	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apiextensions"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v3"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	pkg "ironflyer/infra/pulumi-data/pkg"
	mySecrets "ironflyer/infra/pulumi-data/pkg/secrets"
)

// Bridge is the set of K8s objects this package creates.
type Bridge struct {
	Provider       *kubernetes.Provider
	SecretStore    *apiextensions.CustomResource
	ExternalSecrets map[string]*apiextensions.CustomResource
	// Names is the deterministic logical-name → K8s Secret name map that
	// the Helm chart's existingSecret values must reference.
	Names map[string]string
}

// K8sSecretName maps a logical secret to the K8s Secret name the Helm
// chart expects.
func K8sSecretName(logical string) string {
	return "ironflyer-" + logical
}

// Provision creates the ESO bridge. Skips silently if no kubeconfig.
func Provision(
	ctx *pulumi.Context,
	env *pkg.Env,
	comp *pkg.Compute,
	secrets *mySecrets.Secrets,
) (*Bridge, error) {
	if env.Kubeconfig == "" {
		ctx.Log.Info("k8s: skipping (no kubeconfig in stack config)", nil)
		return &Bridge{Names: secretNames(secrets)}, nil
	}

	prov, err := kubernetes.NewProvider(ctx, env.Name("k8s"), &kubernetes.ProviderArgs{
		Kubeconfig:        pulumi.String(env.Kubeconfig),
		EnableServerSideApply: pulumi.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("k8s provider: %w", err)
	}

	bridge := &Bridge{
		Provider:        prov,
		ExternalSecrets: map[string]*apiextensions.CustomResource{},
		Names:           secretNames(secrets),
	}

	// Ensure the app namespace exists (idempotent).
	_, err = corev1.NewNamespace(ctx, env.Name("ns-app"), &corev1.NamespaceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String(env.AppNamespace),
		},
	}, pulumi.Provider(prov))
	if err != nil {
		return nil, fmt.Errorf("app namespace: %w", err)
	}

	// Install ESO if requested.
	var esoRelease *helm.Release
	if env.ESOInstall {
		_, err := corev1.NewNamespace(ctx, env.Name("ns-eso"), &corev1.NamespaceArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String(env.ESONamespace),
			},
		}, pulumi.Provider(prov))
		if err != nil {
			return nil, fmt.Errorf("eso namespace: %w", err)
		}
		esoRelease, err = helm.NewRelease(ctx, env.Name("eso"), &helm.ReleaseArgs{
			Chart:     pulumi.String("external-secrets"),
			Version:   pulumi.String("0.10.4"),
			Namespace: pulumi.String(env.ESONamespace),
			RepositoryOpts: &helm.RepositoryOptsArgs{
				Repo: pulumi.String("https://charts.external-secrets.io"),
			},
			Values: pulumi.Map{
				"installCRDs": pulumi.Bool(true),
			},
		}, pulumi.Provider(prov))
		if err != nil {
			return nil, fmt.Errorf("eso release: %w", err)
		}
	}

	// ClusterSecretStore that authenticates via IRSA (the orchestrator's
	// role has secretsmanager:GetSecretValue + kms:Decrypt on the data
	// layer's secrets).
	storeName := env.Name("eso-store")
	deps := []pulumi.Resource{}
	if esoRelease != nil {
		deps = append(deps, esoRelease)
	}
	store, err := apiextensions.NewCustomResource(ctx, storeName, &apiextensions.CustomResourceArgs{
		ApiVersion: pulumi.String("external-secrets.io/v1beta1"),
		Kind:       pulumi.String("ClusterSecretStore"),
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String(storeName),
		},
		OtherFields: kubernetes.UntypedArgs{
			"spec": pulumi.Map{
				"provider": pulumi.Map{
					"aws": pulumi.Map{
						"service": pulumi.String("SecretsManager"),
						"region":  pulumi.String(env.Region),
						"auth": pulumi.Map{
							"jwt": pulumi.Map{
								"serviceAccountRef": pulumi.Map{
									"name":      pulumi.String("orchestrator-sa"),
									"namespace": pulumi.String(env.AppNamespace),
								},
							},
						},
					},
				},
			},
		},
	}, pulumi.Provider(prov), pulumi.DependsOn(deps))
	if err != nil {
		return nil, fmt.Errorf("ClusterSecretStore: %w", err)
	}
	bridge.SecretStore = store

	// One ExternalSecret per AWS Secret. The ExternalSecret materializes
	// the AWS Secret's JSON payload as a K8s Secret named per
	// K8sSecretName(logical) so the Helm chart's `existingSecret` knobs
	// land cleanly.
	for logical, awsSecret := range secrets.All {
		es, err := apiextensions.NewCustomResource(ctx, env.Name("es-"+logical), &apiextensions.CustomResourceArgs{
			ApiVersion: pulumi.String("external-secrets.io/v1beta1"),
			Kind:       pulumi.String("ExternalSecret"),
			Metadata: &metav1.ObjectMetaArgs{
				Name:      pulumi.String(K8sSecretName(logical)),
				Namespace: pulumi.String(env.AppNamespace),
			},
			OtherFields: kubernetes.UntypedArgs{
				"spec": pulumi.Map{
					"refreshInterval": pulumi.String("1h"),
					"secretStoreRef": pulumi.Map{
						"name": pulumi.String(storeName),
						"kind": pulumi.String("ClusterSecretStore"),
					},
					"target": pulumi.Map{
						"name":           pulumi.String(K8sSecretName(logical)),
						"creationPolicy": pulumi.String("Owner"),
					},
					"dataFrom": pulumi.MapArray{
						pulumi.Map{
							"extract": pulumi.Map{
								"key": awsSecret.Name,
							},
						},
					},
				},
			},
		}, pulumi.Provider(prov), pulumi.DependsOn([]pulumi.Resource{store}))
		if err != nil {
			return nil, fmt.Errorf("ExternalSecret %s: %w", logical, err)
		}
		bridge.ExternalSecrets[logical] = es
	}

	return bridge, nil
}

func secretNames(s *mySecrets.Secrets) map[string]string {
	out := map[string]string{}
	for logical := range s.All {
		out[logical] = K8sSecretName(logical)
	}
	return out
}
