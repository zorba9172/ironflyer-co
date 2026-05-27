package edge

import (
	helmv3 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// newSealedSecrets installs the bitnami-labs sealed-secrets controller.
// Sealed-secrets is how we ship secrets through Git on the DO stack —
// the AWS stack uses AWS Secrets Manager + IRSA; on DO we don't have a
// first-party equivalent, and sealed-secrets is the simplest pattern that
// keeps the GitOps story honest (the sealed payload is safe to commit;
// only the in-cluster controller can decrypt it).
//
// Operator workflow (run once per cluster, after this release is healthy):
//
//	# 1. Fetch the controller's public key and commit it:
//	kubeseal --controller-namespace sealed-secrets \
//	         --controller-name sealed-secrets-controller \
//	         --fetch-cert > infra/sealed-secrets/pub-cert.<stack>.pem
//	git add infra/sealed-secrets/pub-cert.<stack>.pem && git commit
//
//	# 2. Seal a secret offline using the committed cert (no cluster needed
//	#    after this; the CLI uses just the .pem):
//	kubectl create secret generic stripe-secret --dry-run=client \
//	        --from-literal=key=$STRIPE_KEY -o yaml | \
//	  kubeseal --cert infra/sealed-secrets/pub-cert.<stack>.pem -o yaml \
//	  > core/orchestrator/k8s/sealed/stripe-secret.yaml
//
//	# 3. Commit the .yaml. ArgoCD/Helm/kubectl apply it; the controller
//	#    decrypts and creates the matching `Secret` in-cluster.
//
// Key rotation: the controller rotates its sealing key every 30 days but
// keeps old keys for unsealing. To force-rotate (after a suspected
// compromise) delete the `sealed-secrets-key*` secrets in this namespace
// and re-run step 1 above.
func newSealedSecrets(ctx *pulumi.Context, in Inputs) (*helmv3.Release, error) {
	return helmv3.NewRelease(ctx, "sealed-secrets", &helmv3.ReleaseArgs{
		Chart:           pulumi.String("sealed-secrets"),
		Version:         pulumi.String("2.16.0"),
		Namespace:       pulumi.String("sealed-secrets"),
		CreateNamespace: pulumi.Bool(true),
		RepositoryOpts: &helmv3.RepositoryOptsArgs{
			Repo: pulumi.String("https://bitnami-labs.github.io/sealed-secrets"),
		},
		Values: pulumi.Map{
			// Pin the controller's resource + service name so the
			// `kubeseal --controller-name sealed-secrets-controller`
			// invocation in the runbook above keeps working when the
			// chart's default naming changes between minor versions.
			"fullnameOverride": pulumi.String("sealed-secrets-controller"),
			"nodeSelector": pulumi.Map{
				"workload": pulumi.String("system"),
			},
			"metrics": pulumi.Map{
				"serviceMonitor": pulumi.Map{
					// Off by default — we don't ship Prometheus Operator
					// out of the box, so the ServiceMonitor CR isn't
					// available. Flip to true once observability lands.
					"enabled": pulumi.Bool(false),
				},
			},
		},
	}, pulumi.Provider(in.K8sProvider))
}
