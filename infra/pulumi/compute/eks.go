package compute

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/eks"
	awsiam "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Cluster bundles every EKS output downstream modules need.
type Cluster struct {
	Name                   pulumi.StringOutput
	Endpoint               pulumi.StringOutput
	OIDCProviderArn        pulumi.StringOutput
	OIDCProviderURL        pulumi.StringOutput
	ClusterSecurityGroupID pulumi.StringOutput
	K8sProvider            *kubernetes.Provider
	Version                pulumi.StringOutput
}

// NewEKS provisions the EKS control plane, OIDC IAM provider, both node
// groups, and the EKS-native addons (vpc-cni, coredns, kube-proxy, EBS CSI,
// EFS CSI). Helm-based controllers (autoscaler, ALB controller, cert-manager,
// external-dns, optionally Karpenter) live in autoscaling.go so the Pulumi
// dependency graph stays acyclic — they need IRSA roles that need OIDC.
func NewEKS(ctx *pulumi.Context, cfg *Config, net *Network, iam *IAM) (*Cluster, error) {
	clusterName := "ironflyer-" + cfg.Stack

	allSubnets := pulumi.All(net.PublicSubnetIDs, net.PrivateSubnetIDs).ApplyT(func(parts []interface{}) []string {
		pub := parts[0].([]string)
		pri := parts[1].([]string)
		out := make([]string, 0, len(pub)+len(pri))
		out = append(out, pub...)
		out = append(out, pri...)
		return out
	}).(pulumi.StringArrayOutput)

	publicCidrs := pulumi.StringArray{}
	for _, c := range cfg.PublicAPICidrs {
		publicCidrs = append(publicCidrs, pulumi.String(c))
	}

	cluster, err := eks.NewCluster(ctx, "ironflyer-eks", &eks.ClusterArgs{
		Name:    pulumi.String(clusterName),
		RoleArn: iam.ClusterRoleArn,
		Version: pulumi.String(cfg.K8sVersion),
		VpcConfig: &eks.ClusterVpcConfigArgs{
			SubnetIds:             allSubnets,
			EndpointPrivateAccess: pulumi.Bool(true),
			EndpointPublicAccess:  pulumi.Bool(true),
			PublicAccessCidrs:     publicCidrs,
		},
		EnabledClusterLogTypes: pulumi.StringArray{
			pulumi.String("api"),
			pulumi.String("audit"),
			pulumi.String("authenticator"),
			pulumi.String("controllerManager"),
			pulumi.String("scheduler"),
		},
		Tags: cfg.TagsWith(map[string]string{"Name": clusterName}),
	})
	if err != nil {
		return nil, err
	}

	// OIDC provider — needed for IRSA. The thumbprint AWS uses for EKS OIDC
	// endpoints is the well-known root CA thumbprint.
	oidcURL := cluster.Identities.Index(pulumi.Int(0)).Oidcs().Index(pulumi.Int(0)).Issuer().Elem()
	oidcProvider, err := awsiam.NewOpenIdConnectProvider(ctx, "eks-oidc", &awsiam.OpenIdConnectProviderArgs{
		Url:           oidcURL,
		ClientIdLists: pulumi.StringArray{pulumi.String("sts.amazonaws.com")},
		ThumbprintLists: pulumi.StringArray{
			pulumi.String("9e99a48a9960b14926bb7f3b02e22da2b0ab7280"),
		},
		Tags: cfg.Tags(),
	})
	if err != nil {
		return nil, err
	}

	// Node groups -----------------------------------------------------------
	if _, err := eks.NewNodeGroup(ctx, "orchestrator-pool", &eks.NodeGroupArgs{
		ClusterName:   cluster.Name,
		NodeGroupName: pulumi.String("orchestrator-pool"),
		NodeRoleArn:   iam.NodeRoleArn,
		SubnetIds:     net.PrivateSubnetIDs,
		InstanceTypes: pulumi.StringArray{pulumi.String(cfg.OrchestratorType)},
		ScalingConfig: &eks.NodeGroupScalingConfigArgs{
			DesiredSize: pulumi.Int(3),
			MaxSize:     pulumi.Int(10),
			MinSize:     pulumi.Int(2),
		},
		UpdateConfig: &eks.NodeGroupUpdateConfigArgs{
			MaxUnavailablePercentage: pulumi.Int(33),
		},
		Labels: pulumi.StringMap{
			"pool": pulumi.String("orchestrator"),
		},
		Tags: cfg.TagsWith(map[string]string{
			"k8s.io/cluster-autoscaler/enabled":            "true",
			"k8s.io/cluster-autoscaler/" + clusterName:     "owned",
			"Name": "orchestrator-pool",
		}),
	}); err != nil {
		return nil, err
	}

	if _, err := eks.NewNodeGroup(ctx, "runtime-pool", &eks.NodeGroupArgs{
		ClusterName:   cluster.Name,
		NodeGroupName: pulumi.String("runtime-pool"),
		NodeRoleArn:   iam.NodeRoleArn,
		SubnetIds:     net.PrivateSubnetIDs,
		InstanceTypes: pulumi.StringArray{pulumi.String(cfg.RuntimeType)},
		ScalingConfig: &eks.NodeGroupScalingConfigArgs{
			DesiredSize: pulumi.Int(3),
			MaxSize:     pulumi.Int(20),
			MinSize:     pulumi.Int(2),
		},
		UpdateConfig: &eks.NodeGroupUpdateConfigArgs{
			MaxUnavailablePercentage: pulumi.Int(33),
		},
		Labels: pulumi.StringMap{
			"pool": pulumi.String("runtime"),
		},
		Taints: eks.NodeGroupTaintArray{&eks.NodeGroupTaintArgs{
			Key:    pulumi.String("dedicated"),
			Value:  pulumi.String("runtime"),
			Effect: pulumi.String("NO_SCHEDULE"),
		}},
		Tags: cfg.TagsWith(map[string]string{
			"k8s.io/cluster-autoscaler/enabled":            "true",
			"k8s.io/cluster-autoscaler/" + clusterName:     "owned",
			"Name": "runtime-pool",
		}),
	}); err != nil {
		return nil, err
	}

	// Native EKS addons -----------------------------------------------------
	for _, addon := range []struct {
		name string
	}{
		{"vpc-cni"},
		{"coredns"},
		{"kube-proxy"},
		{"aws-ebs-csi-driver"},
		{"aws-efs-csi-driver"},
	} {
		if _, err := eks.NewAddon(ctx, "addon-"+addon.name, &eks.AddonArgs{
			ClusterName:              cluster.Name,
			AddonName:                pulumi.String(addon.name),
			ResolveConflictsOnCreate: pulumi.String("OVERWRITE"),
			ResolveConflictsOnUpdate: pulumi.String("OVERWRITE"),
			Tags:                     cfg.Tags(),
		}); err != nil {
			return nil, err
		}
	}

	// Build a Kubernetes provider authenticated against this EKS cluster.
	// Downstream Helm releases (both compute and data layers) reuse this
	// provider so they don't depend on an ambient kubeconfig.
	k8sProvider, err := newK8sProvider(ctx, cluster)
	if err != nil {
		return nil, err
	}

	return &Cluster{
		Name:                   cluster.Name,
		Endpoint:               cluster.Endpoint,
		OIDCProviderArn:        oidcProvider.Arn,
		OIDCProviderURL:        oidcURL,
		ClusterSecurityGroupID: cluster.VpcConfig.ClusterSecurityGroupId().Elem(),
		K8sProvider:            k8sProvider,
		Version:                cluster.Version,
	}, nil
}

// newK8sProvider returns a kubernetes.Provider configured with a kubeconfig
// templated for this cluster. The kubeconfig uses the `aws eks
// get-token` plugin, which is the recommended approach for pulumi.
func newK8sProvider(ctx *pulumi.Context, cluster *eks.Cluster) (*kubernetes.Provider, error) {
	kubeconfig := pulumi.All(cluster.Endpoint, cluster.CertificateAuthority, cluster.Name).ApplyT(func(parts []interface{}) string {
		endpoint := parts[0].(string)
		// CertificateAuthority is a struct with a `Data` field; Pulumi
		// surfaces it as a map[string]interface{} after the Apply.
		ca := ""
		if m, ok := parts[1].(map[string]interface{}); ok {
			if v, ok := m["data"].(string); ok {
				ca = v
			}
		}
		name := parts[2].(string)
		return `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: ` + endpoint + `
    certificate-authority-data: ` + ca + `
  name: ` + name + `
contexts:
- context:
    cluster: ` + name + `
    user: aws
  name: ` + name + `
current-context: ` + name + `
users:
- name: aws
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: aws
      args:
        - eks
        - get-token
        - --cluster-name
        - ` + name + `
`
	}).(pulumi.StringOutput)

	return kubernetes.NewProvider(ctx, "k8s-cluster", &kubernetes.ProviderArgs{
		Kubeconfig:            kubeconfig,
		EnableServerSideApply: pulumi.Bool(true),
	})
}
