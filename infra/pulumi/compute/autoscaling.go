package compute

import (
	helmv3 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// NewAutoscaling installs the cluster-side controllers responsible for keeping
// the EKS cluster sized appropriately and reachable from the outside.
//
// We split this from NewEKS() because these controllers all need IRSA roles,
// which themselves need the OIDC provider that NewEKS() creates. Installing
// them after NewEKS() returns lets Pulumi resolve the dependency graph
// cleanly.
//
// Installed here:
//   - Cluster Autoscaler (Helm) — scales the managed node groups via tags.
//   - Metrics Server (Helm) — required by HPA / dashboards.
//   - AWS Load Balancer Controller (Helm) — turns Ingress into ALBs.
//   - Karpenter (Helm) — opt-in via infra:useKarpenter=true.
//
// External-DNS and cert-manager are installed in edge/dns.go after the
// Route53 zone exists, so their IRSA policies can reference the zone ARN.
func NewAutoscaling(ctx *pulumi.Context, cfg *Config, cluster *Cluster, _ *IAM) error {
	// Build IRSA roles that the Helm releases below assume into.
	// External-DNS + cert-manager IRSA happens in edge/dns.go, after the
	// hosted zone resource exists.
	irsa, err := buildAutoscalingIRSA(ctx, cfg, cluster)
	if err != nil {
		return err
	}

	// Reuse the shared Kubernetes provider built alongside the cluster.
	opts := pulumi.Provider(cluster.K8sProvider)

	// Cluster Autoscaler
	if _, err := helmv3.NewRelease(ctx, "cluster-autoscaler", &helmv3.ReleaseArgs{
		Chart:           pulumi.String("cluster-autoscaler"),
		Version:         pulumi.String("9.37.0"),
		Namespace:       pulumi.String("kube-system"),
		CreateNamespace: pulumi.Bool(false),
		RepositoryOpts: &helmv3.RepositoryOptsArgs{
			Repo: pulumi.String("https://kubernetes.github.io/autoscaler"),
		},
		Values: pulumi.Map{
			"autoDiscovery": pulumi.Map{
				"clusterName": cluster.Name,
			},
			"awsRegion": pulumi.String(cfg.Region),
			"rbac": pulumi.Map{
				"serviceAccount": pulumi.Map{
					"name": pulumi.String("cluster-autoscaler"),
					"annotations": pulumi.Map{
						"eks.amazonaws.com/role-arn": irsa.ClusterAutoscaler,
					},
				},
			},
			"extraArgs": pulumi.Map{
				"balance-similar-node-groups":   pulumi.String("true"),
				"skip-nodes-with-system-pods":   pulumi.String("false"),
				"skip-nodes-with-local-storage": pulumi.String("false"),
			},
		},
	}, opts); err != nil {
		return err
	}

	// Metrics Server
	if _, err := helmv3.NewRelease(ctx, "metrics-server", &helmv3.ReleaseArgs{
		Chart:           pulumi.String("metrics-server"),
		Version:         pulumi.String("3.12.1"),
		Namespace:       pulumi.String("kube-system"),
		CreateNamespace: pulumi.Bool(false),
		RepositoryOpts: &helmv3.RepositoryOptsArgs{
			Repo: pulumi.String("https://kubernetes-sigs.github.io/metrics-server/"),
		},
	}, opts); err != nil {
		return err
	}

	// AWS Load Balancer Controller
	if _, err := helmv3.NewRelease(ctx, "aws-load-balancer-controller", &helmv3.ReleaseArgs{
		Chart:           pulumi.String("aws-load-balancer-controller"),
		Version:         pulumi.String("1.8.1"),
		Namespace:       pulumi.String("kube-system"),
		CreateNamespace: pulumi.Bool(false),
		RepositoryOpts: &helmv3.RepositoryOptsArgs{
			Repo: pulumi.String("https://aws.github.io/eks-charts"),
		},
		Values: pulumi.Map{
			"clusterName": cluster.Name,
			"region":      pulumi.String(cfg.Region),
			"serviceAccount": pulumi.Map{
				"create": pulumi.Bool(true),
				"name":   pulumi.String("aws-load-balancer-controller"),
				"annotations": pulumi.Map{
					"eks.amazonaws.com/role-arn": irsa.AWSLoadBalancerController,
				},
			},
		},
	}, opts); err != nil {
		return err
	}

	if cfg.UseKarpenter {
		if _, err := helmv3.NewRelease(ctx, "karpenter", &helmv3.ReleaseArgs{
			Chart:           pulumi.String("karpenter"),
			Version:         pulumi.String("0.37.0"),
			Namespace:       pulumi.String("karpenter"),
			CreateNamespace: pulumi.Bool(true),
			RepositoryOpts: &helmv3.RepositoryOptsArgs{
				Repo: pulumi.String("oci://public.ecr.aws/karpenter"),
			},
			Values: pulumi.Map{
				"settings": pulumi.Map{
					"clusterName": cluster.Name,
				},
			},
		}, opts); err != nil {
			return err
		}
	}

	return nil
}

// autoscalingIRSA is the subset of IRSARoles needed by the controllers
// installed in this file. Keeping it small avoids re-running role creation
// later when edge/dns.go builds the DNS/zone-scoped IRSA roles.
type autoscalingIRSA struct {
	ClusterAutoscaler         pulumi.StringOutput
	AWSLoadBalancerController pulumi.StringOutput
}

func buildAutoscalingIRSA(ctx *pulumi.Context, cfg *Config, cluster *Cluster) (*autoscalingIRSA, error) {
	ca, err := MakeIRSARole(ctx, cfg, "cluster-autoscaler-sa", "kube-system", "cluster-autoscaler", cluster.OIDCProviderArn, cluster.OIDCProviderURL)
	if err != nil {
		return nil, err
	}
	if err := attachInline(ctx, "cluster-autoscaler-policy", ca, clusterAutoscalerPolicy()); err != nil {
		return nil, err
	}

	albc, err := MakeIRSARole(ctx, cfg, "aws-load-balancer-controller-sa", "kube-system", "aws-load-balancer-controller", cluster.OIDCProviderArn, cluster.OIDCProviderURL)
	if err != nil {
		return nil, err
	}
	if err := attachInline(ctx, "alb-controller-policy", albc, albControllerPolicy()); err != nil {
		return nil, err
	}

	return &autoscalingIRSA{
		ClusterAutoscaler:         ca.Arn,
		AWSLoadBalancerController: albc.Arn,
	}, nil
}
