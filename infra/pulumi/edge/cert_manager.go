package edge

import (
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apiextensions"
	helmv3 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// newCertManager installs jetstack/cert-manager v1.16.x in its own
// namespace. CRDs are bundled (`installCRDs=true`) so we don't have to
// shell out to `kubectl apply -f` before the Helm release lands. Leader
// election is pinned to the same namespace so a future multi-replica
// rollout doesn't race against itself.
//
// The ClusterIssuer is created separately by newClusterIssuer once
// ingress-nginx has provisioned the LoadBalancer that ACME HTTP01 will
// solve through — see edge.go for the dependency order.
func newCertManager(ctx *pulumi.Context, in Inputs) (*helmv3.Release, error) {
	return helmv3.NewRelease(ctx, "cert-manager", &helmv3.ReleaseArgs{
		Chart:           pulumi.String("cert-manager"),
		Version:         pulumi.String("v1.16.1"),
		Namespace:       pulumi.String("cert-manager"),
		CreateNamespace: pulumi.Bool(true),
		RepositoryOpts: &helmv3.RepositoryOptsArgs{
			Repo: pulumi.String("https://charts.jetstack.io"),
		},
		Values: pulumi.Map{
			"installCRDs": pulumi.Bool(true),
			"global": pulumi.Map{
				"leaderElection": pulumi.Map{
					"namespace": pulumi.String("cert-manager"),
				},
			},
			// Run cert-manager on the system pool, never on a runtime
			// node — the runtime pool is tainted for sandbox workloads.
			"nodeSelector": pulumi.Map{
				"workload": pulumi.String("system"),
			},
		},
	}, pulumi.Provider(in.K8sProvider))
}

// newClusterIssuer creates the Let's Encrypt production ClusterIssuer.
// We use HTTP01 over ingress-nginx (the dependency chain in Provision
// guarantees the ingress controller's LB exists by the time we get here).
//
// Why HTTP01 over DNS01 — even though Cloudflare also speaks DNS01:
//   - HTTP01 is the simplest reliable path that does not require giving
//     cert-manager Cloudflare API credentials with DNS-edit scope.
//   - Cloudflare proxying is fine for HTTP01 because Let's Encrypt
//     follows redirects + accepts proxied responses; the orchestrator
//     ingress strips proxy-protocol bits before the challenge handler
//     sees them.
//   - When we eventually need wildcard certs (`*.runtime.<root>`) the
//     operator can add a DNS01 solver by editing this CR; the rest of
//     the chart stays unchanged.
func newClusterIssuer(ctx *pulumi.Context, in Inputs, cmRel, ingRel *helmv3.Release) error {
	_, err := apiextensions.NewCustomResource(ctx, "letsencrypt-prod", &apiextensions.CustomResourceArgs{
		ApiVersion: pulumi.String("cert-manager.io/v1"),
		Kind:       pulumi.String("ClusterIssuer"),
		Metadata: &apiextensions.CustomResourceMetadataArgs{
			Name: pulumi.String("letsencrypt-prod"),
		},
		OtherFields: map[string]any{
			"spec": map[string]any{
				"acme": map[string]any{
					"email":  acmeEmail(in.Config),
					"server": "https://acme-v02.api.letsencrypt.org/directory",
					"privateKeySecretRef": map[string]any{
						"name": "letsencrypt-prod-account-key",
					},
					"solvers": []any{
						map[string]any{
							"http01": map[string]any{
								"ingress": map[string]any{
									"class": "nginx",
								},
							},
						},
					},
				},
			},
		},
	},
		pulumi.Provider(in.K8sProvider),
		// Both dependencies must be ready: CRDs from cert-manager, and
		// the ingress controller HTTP01 will route through.
		pulumi.DependsOn([]pulumi.Resource{cmRel, ingRel}),
	)
	return err
}
