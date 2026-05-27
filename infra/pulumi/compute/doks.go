package compute

import (
	"github.com/pulumi/pulumi-digitalocean/sdk/v4/go/digitalocean"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// NewDOKS provisions the DOKS control plane plus two node pools:
//
//   - `system` — embedded with the cluster. Hosts the orchestrator API,
//     edge controllers (ingress-nginx, cert-manager, external-dns,
//     sealed-secrets), and observability. Cluster-autoscaler is enabled
//     by setting AutoScale=true; DOKS ships the autoscaler in-cluster, no
//     Helm install needed (see autoscaler.go).
//
//   - `runtime` — a separate KubernetesNodePool with a `workload=runtime`
//     label and a NoSchedule taint so only per-user sandbox pods land
//     there. This is the analogue of the AWS `runtime-pool` EKS managed
//     node group.
//
// HighlyAvailable is gated on cfg.EnableHA. Maintenance is pinned to
// Sunday 03:00 UTC to match the orchestrator's quiet window.
func NewDOKS(ctx *pulumi.Context, cfg *Config, net *Network) (*digitalocean.KubernetesCluster, error) {
	clusterName := cfg.ResourceName("doks")

	cluster, err := digitalocean.NewKubernetesCluster(ctx, clusterName, &digitalocean.KubernetesClusterArgs{
		Name:    pulumi.String(clusterName),
		Region:  pulumi.String(cfg.Region),
		Version: pulumi.String(cfg.K8sVersion),
		VpcUuid: net.VpcID.ToStringOutput(),
		NodePool: &digitalocean.KubernetesClusterNodePoolArgs{
			Name:      pulumi.String("system"),
			Size:      pulumi.String(cfg.DOKSNodeSize),
			NodeCount: pulumi.Int(cfg.DOKSNodeCount),
			AutoScale: pulumi.Bool(true),
			MinNodes:  pulumi.Int(cfg.DOKSNodeCount),
			MaxNodes:  pulumi.Int(cfg.DOKSMaxNodes),
			Labels: pulumi.StringMap{
				"pool":     pulumi.String("system"),
				"workload": pulumi.String("system"),
			},
			Tags: pulumi.StringArray{
				pulumi.String("ironflyer"),
				pulumi.String("doks-system"),
				pulumi.String("stack:" + cfg.Stack),
			},
		},
		MaintenancePolicy: &digitalocean.KubernetesClusterMaintenancePolicyArgs{
			StartTime: pulumi.String("03:00"),
			Day:       pulumi.String("sunday"),
		},
		Ha:           pulumi.Bool(cfg.EnableHA),
		SurgeUpgrade: pulumi.Bool(true),
		AutoUpgrade:     pulumi.Bool(false),
		Tags:            cfg.Tags("doks"),
	})
	if err != nil {
		return nil, err
	}

	// Runtime node pool — separate from the system pool so the cluster
	// autoscaler can scale per-user sandbox capacity independently of the
	// control-plane facing workloads. The NoSchedule taint forces the
	// orchestrator scheduler to be explicit (pod tolerations) when it
	// schedules onto this pool.
	if _, err := digitalocean.NewKubernetesNodePool(ctx, cfg.ResourceName("runtime-pool"), &digitalocean.KubernetesNodePoolArgs{
		ClusterId: cluster.ID(),
		Name:      pulumi.String("runtime"),
		Size:      pulumi.String(cfg.DOKSRuntimeNodeSize),
		NodeCount: pulumi.Int(cfg.DOKSRuntimeNodeCount),
		AutoScale: pulumi.Bool(true),
		MinNodes:  pulumi.Int(cfg.DOKSRuntimeNodeCount),
		MaxNodes:  pulumi.Int(cfg.DOKSRuntimeMaxNodes),
		Labels: pulumi.StringMap{
			"pool":     pulumi.String("runtime"),
			"workload": pulumi.String("runtime"),
		},
		Taints: digitalocean.KubernetesNodePoolTaintArray{
			&digitalocean.KubernetesNodePoolTaintArgs{
				Key:    pulumi.String("dedicated"),
				Value:  pulumi.String("runtime"),
				Effect: pulumi.String("NoSchedule"),
			},
		},
		Tags: pulumi.StringArray{
			pulumi.String("ironflyer"),
			pulumi.String("doks-runtime"),
			pulumi.String("stack:" + cfg.Stack),
		},
	}); err != nil {
		return nil, err
	}

	return cluster, nil
}
