package compute

import (
	"github.com/pulumi/pulumi-digitalocean/sdk/v4/go/digitalocean"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// NewK8sProvider returns a kubernetes.Provider authenticated against the
// freshly-provisioned DOKS cluster. Unlike AWS EKS — where we have to
// hand-craft a kubeconfig that shells out to `aws eks get-token` — DOKS
// returns a fully-formed kubeconfig (token + CA + server URL) as part of
// the resource read, so we just pipe it through.
//
// The raw kubeconfig is treated as a Pulumi secret because it contains a
// long-lived service-account token; downstream Helm releases inherit it
// transitively.
func NewK8sProvider(ctx *pulumi.Context, cfg *Config, cluster *digitalocean.KubernetesCluster) (*kubernetes.Provider, error) {
	kubeconfig := cluster.KubeConfigs.Index(pulumi.Int(0)).RawConfig().Elem()

	return kubernetes.NewProvider(ctx, cfg.ResourceName("k8s"), &kubernetes.ProviderArgs{
		Kubeconfig:            pulumi.ToSecret(kubeconfig).(pulumi.StringInput),
		EnableServerSideApply: pulumi.Bool(true),
	})
}
