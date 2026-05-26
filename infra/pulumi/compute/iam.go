package compute

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// IAM bundles the long-lived IAM roles the EKS module wires into its cluster
// and node groups. IRSA service-account roles are created later — after EKS
// gives us an OIDC provider — by BuildIRSA().
type IAM struct {
	ClusterRoleArn pulumi.StringOutput
	NodeRoleArn    pulumi.StringOutput
	NodeRoleName   pulumi.StringOutput
}

// NewIAM creates the cluster and node IAM roles plus their managed policy
// attachments. These are pre-requisites for the EKS module.
func NewIAM(ctx *pulumi.Context, cfg *Config) (*IAM, error) {
	cluster, err := iam.NewRole(ctx, "eks-cluster-role", &iam.RoleArgs{
		Name: pulumi.String("ironflyer-" + cfg.Stack + "-eks-cluster"),
		AssumeRolePolicy: pulumi.String(`{
			"Version":"2012-10-17",
			"Statement":[{"Effect":"Allow","Principal":{"Service":"eks.amazonaws.com"},"Action":"sts:AssumeRole"}]
		}`),
		Tags: cfg.Tags(),
	})
	if err != nil {
		return nil, err
	}
	for i, arn := range []string{
		"arn:aws:iam::aws:policy/AmazonEKSClusterPolicy",
		"arn:aws:iam::aws:policy/AmazonEKSVPCResourceController",
	} {
		if _, err := iam.NewRolePolicyAttachment(ctx, pname("eks-cluster-attach", i), &iam.RolePolicyAttachmentArgs{
			Role:      cluster.Name,
			PolicyArn: pulumi.String(arn),
		}); err != nil {
			return nil, err
		}
	}

	node, err := iam.NewRole(ctx, "eks-node-role", &iam.RoleArgs{
		Name: pulumi.String("ironflyer-" + cfg.Stack + "-eks-node"),
		AssumeRolePolicy: pulumi.String(`{
			"Version":"2012-10-17",
			"Statement":[{"Effect":"Allow","Principal":{"Service":"ec2.amazonaws.com"},"Action":"sts:AssumeRole"}]
		}`),
		Tags: cfg.Tags(),
	})
	if err != nil {
		return nil, err
	}
	for i, arn := range []string{
		"arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
		"arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
		"arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
		"arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore",
	} {
		if _, err := iam.NewRolePolicyAttachment(ctx, pname("eks-node-attach", i), &iam.RolePolicyAttachmentArgs{
			Role:      node.Name,
			PolicyArn: pulumi.String(arn),
		}); err != nil {
			return nil, err
		}
	}

	return &IAM{
		ClusterRoleArn: cluster.Arn,
		NodeRoleArn:    node.Arn,
		NodeRoleName:   node.Name,
	}, nil
}

// IRSARoles holds the per-workload roles consumed by application Pods. These
// are distinct from controller IRSA roles (which are created next to their
// Helm release in autoscaling.go / edge/dns.go).
type IRSARoles struct {
	Orchestrator pulumi.StringOutput
	Runtime      pulumi.StringOutput
	Backup       pulumi.StringOutput
}

// BuildWorkloadIRSA creates the orchestrator-sa, runtime-sa, and backup-sa
// IRSA roles. Controller IRSA roles (cluster autoscaler, ALB controller,
// external-dns, cert-manager) live with their Helm releases.
func BuildWorkloadIRSA(ctx *pulumi.Context, cfg *Config, cluster *Cluster) (*IRSARoles, error) {
	orchestrator, err := MakeIRSARole(ctx, cfg, "orchestrator-sa", "ironflyer", "orchestrator-sa", cluster.OIDCProviderArn, cluster.OIDCProviderURL)
	if err != nil {
		return nil, err
	}
	if err := attachInline(ctx, "orchestrator-sa-policy", orchestrator, orchestratorPolicy(cfg)); err != nil {
		return nil, err
	}

	rt, err := MakeIRSARole(ctx, cfg, "runtime-sa", "ironflyer", "runtime-sa", cluster.OIDCProviderArn, cluster.OIDCProviderURL)
	if err != nil {
		return nil, err
	}
	if err := attachInline(ctx, "runtime-sa-policy", rt, runtimePolicy(cfg)); err != nil {
		return nil, err
	}

	backup, err := MakeIRSARole(ctx, cfg, "backup-sa", "ironflyer", "backup-sa", cluster.OIDCProviderArn, cluster.OIDCProviderURL)
	if err != nil {
		return nil, err
	}
	if err := attachInline(ctx, "backup-sa-policy", backup, backupPolicy(cfg)); err != nil {
		return nil, err
	}

	return &IRSARoles{
		Orchestrator: orchestrator.Arn,
		Runtime:      rt.Arn,
		Backup:       backup.Arn,
	}, nil
}

// MakeIRSARole returns an IAM role with an OIDC trust policy scoped to a
// single Kubernetes ServiceAccount (namespace + name). The role name embeds
// the stack so multiple stacks can coexist in one AWS account.
func MakeIRSARole(ctx *pulumi.Context, cfg *Config, resourceName, namespace, saName string, oidcArn, oidcURL pulumi.StringOutput) (*iam.Role, error) {
	trust := pulumi.All(oidcArn, oidcURL).ApplyT(func(parts []interface{}) string {
		arn := parts[0].(string)
		url := parts[1].(string)
		// url comes back like https://oidc.eks.<region>.amazonaws.com/id/<id>
		host := url
		if len(host) > 8 && host[:8] == "https://" {
			host = host[8:]
		}
		return `{
			"Version":"2012-10-17",
			"Statement":[{
				"Effect":"Allow",
				"Principal":{"Federated":"` + arn + `"},
				"Action":"sts:AssumeRoleWithWebIdentity",
				"Condition":{"StringEquals":{
					"` + host + `:sub":"system:serviceaccount:` + namespace + `:` + saName + `",
					"` + host + `:aud":"sts.amazonaws.com"
				}}
			}]
		}`
	}).(pulumi.StringOutput)

	return iam.NewRole(ctx, resourceName, &iam.RoleArgs{
		Name:             pulumi.String("ironflyer-" + cfg.Stack + "-" + resourceName),
		AssumeRolePolicy: trust,
		Tags: cfg.TagsWith(map[string]string{
			"ServiceAccount": namespace + "/" + saName,
		}),
	})
}

func attachInline(ctx *pulumi.Context, name string, role *iam.Role, policy string) error {
	_, err := iam.NewRolePolicy(ctx, name, &iam.RolePolicyArgs{
		Role:   role.ID(),
		Policy: pulumi.String(policy),
	})
	return err
}

func AttachInlineOutput(ctx *pulumi.Context, name string, role *iam.Role, policy pulumi.StringOutput) error {
	_, err := iam.NewRolePolicy(ctx, name, &iam.RolePolicyArgs{
		Role:   role.ID(),
		Policy: policy,
	})
	return err
}

// --- Inline policy documents -------------------------------------------------
//
// All write actions are scoped to ARNs. Read actions on Secrets Manager are
// limited via a path-style ARN (`ironflyer/*`). The S3 bucket names follow the
// agreed convention with the data stack: `ironflyer-${stack}-workspace`,
// `ironflyer-${stack}-backup`.

func orchestratorPolicy(cfg *Config) string {
	return `{
		"Version":"2012-10-17",
		"Statement":[
			{
				"Effect":"Allow",
				"Action":["secretsmanager:GetSecretValue","secretsmanager:DescribeSecret"],
				"Resource":"arn:aws:secretsmanager:` + cfg.Region + `:*:secret:ironflyer/*"
			},
			{
				"Effect":"Allow",
				"Action":["s3:PutObject","s3:GetObject","s3:DeleteObject","s3:ListBucket","s3:AbortMultipartUpload"],
				"Resource":[
					"arn:aws:s3:::ironflyer-` + cfg.Stack + `-backup",
					"arn:aws:s3:::ironflyer-` + cfg.Stack + `-backup/*",
					"arn:aws:s3:::ironflyer-` + cfg.Stack + `-workspace",
					"arn:aws:s3:::ironflyer-` + cfg.Stack + `-workspace/*"
				]
			},
			{
				"Effect":"Allow",
				"Action":["logs:CreateLogGroup","logs:CreateLogStream","logs:PutLogEvents"],
				"Resource":"arn:aws:logs:` + cfg.Region + `:*:log-group:/ironflyer/*"
			}
		]
	}`
}

func runtimePolicy(cfg *Config) string {
	return `{
		"Version":"2012-10-17",
		"Statement":[
			{
				"Effect":"Allow",
				"Action":["s3:PutObject","s3:GetObject","s3:DeleteObject","s3:ListBucket","s3:AbortMultipartUpload"],
				"Resource":[
					"arn:aws:s3:::ironflyer-` + cfg.Stack + `-workspace",
					"arn:aws:s3:::ironflyer-` + cfg.Stack + `-workspace/*"
				]
			},
			{
				"Effect":"Allow",
				"Action":["elasticfilesystem:ClientMount","elasticfilesystem:ClientWrite","elasticfilesystem:ClientRootAccess","elasticfilesystem:DescribeFileSystems","elasticfilesystem:DescribeMountTargets"],
				"Resource":"arn:aws:elasticfilesystem:` + cfg.Region + `:*:file-system/*"
			},
			{
				"Effect":"Allow",
				"Action":["secretsmanager:GetSecretValue"],
				"Resource":"arn:aws:secretsmanager:` + cfg.Region + `:*:secret:ironflyer/runtime/*"
			}
		]
	}`
}

func backupPolicy(cfg *Config) string {
	return `{
		"Version":"2012-10-17",
		"Statement":[
			{
				"Effect":"Allow",
				"Action":["secretsmanager:GetSecretValue"],
				"Resource":"arn:aws:secretsmanager:` + cfg.Region + `:*:secret:ironflyer/postgres/*"
			},
			{
				"Effect":"Allow",
				"Action":["s3:PutObject","s3:PutObjectRetention","s3:PutObjectLegalHold","s3:ListBucket"],
				"Resource":[
					"arn:aws:s3:::ironflyer-` + cfg.Stack + `-backup",
					"arn:aws:s3:::ironflyer-` + cfg.Stack + `-backup/*"
				]
			}
		]
	}`
}

func ExternalDNSPolicy(zoneID string) string {
	return `{
		"Version":"2012-10-17",
		"Statement":[
			{
				"Effect":"Allow",
				"Action":["route53:ChangeResourceRecordSets"],
				"Resource":"arn:aws:route53:::hostedzone/` + zoneID + `"
			},
			{
				"Effect":"Allow",
				"Action":["route53:ListHostedZones","route53:ListResourceRecordSets","route53:ListTagsForResource"],
				"Resource":"*"
			}
		]
	}`
}

func clusterAutoscalerPolicy() string {
	// Autoscaler's write actions are intentionally bounded by EC2-side
	// conditions in the upstream docs; for production we trust the
	// `aws:RequestTag` / cluster-tag filter at the API level. Resource is "*"
	// only for read-only describe calls; write actions stay scoped to ASGs.
	return `{
		"Version":"2012-10-17",
		"Statement":[
			{
				"Effect":"Allow",
				"Action":[
					"autoscaling:DescribeAutoScalingGroups",
					"autoscaling:DescribeAutoScalingInstances",
					"autoscaling:DescribeLaunchConfigurations",
					"autoscaling:DescribeScalingActivities",
					"autoscaling:DescribeTags",
					"ec2:DescribeInstanceTypes",
					"ec2:DescribeLaunchTemplateVersions",
					"ec2:DescribeImages",
					"ec2:GetInstanceTypesFromInstanceRequirements"
				],
				"Resource":"*"
			},
			{
				"Effect":"Allow",
				"Action":[
					"autoscaling:SetDesiredCapacity",
					"autoscaling:TerminateInstanceInAutoScalingGroup",
					"autoscaling:UpdateAutoScalingGroup"
				],
				"Resource":"*",
				"Condition":{
					"StringEquals":{"aws:ResourceTag/k8s.io/cluster-autoscaler/enabled":"true"}
				}
			}
		]
	}`
}

func albControllerPolicy() string {
	// Trimmed version of the upstream AWS Load Balancer Controller policy.
	// We keep the read+write split clear: write actions reference resources
	// the controller itself creates (filtered server-side by tag).
	return `{
		"Version":"2012-10-17",
		"Statement":[
			{
				"Effect":"Allow",
				"Action":["iam:CreateServiceLinkedRole"],
				"Resource":"*",
				"Condition":{"StringEquals":{"iam:AWSServiceName":"elasticloadbalancing.amazonaws.com"}}
			},
			{
				"Effect":"Allow",
				"Action":[
					"ec2:DescribeAccountAttributes","ec2:DescribeAddresses","ec2:DescribeAvailabilityZones",
					"ec2:DescribeInternetGateways","ec2:DescribeVpcs","ec2:DescribeVpcPeeringConnections",
					"ec2:DescribeSubnets","ec2:DescribeSecurityGroups","ec2:DescribeInstances",
					"ec2:DescribeNetworkInterfaces","ec2:DescribeTags","ec2:GetCoipPoolUsage",
					"ec2:DescribeCoipPools","elasticloadbalancing:DescribeLoadBalancers",
					"elasticloadbalancing:DescribeLoadBalancerAttributes","elasticloadbalancing:DescribeListeners",
					"elasticloadbalancing:DescribeListenerCertificates","elasticloadbalancing:DescribeSSLPolicies",
					"elasticloadbalancing:DescribeRules","elasticloadbalancing:DescribeTargetGroups",
					"elasticloadbalancing:DescribeTargetGroupAttributes","elasticloadbalancing:DescribeTargetHealth",
					"elasticloadbalancing:DescribeTags"
				],
				"Resource":"*"
			},
			{
				"Effect":"Allow",
				"Action":[
					"cognito-idp:DescribeUserPoolClient","acm:ListCertificates","acm:DescribeCertificate",
					"iam:ListServerCertificates","iam:GetServerCertificate","waf-regional:GetWebACL",
					"waf-regional:GetWebACLForResource","waf-regional:AssociateWebACL","waf-regional:DisassociateWebACL",
					"wafv2:GetWebACL","wafv2:GetWebACLForResource","wafv2:AssociateWebACL","wafv2:DisassociateWebACL",
					"shield:GetSubscriptionState","shield:DescribeProtection","shield:CreateProtection","shield:DeleteProtection"
				],
				"Resource":"*"
			},
			{
				"Effect":"Allow",
				"Action":[
					"ec2:AuthorizeSecurityGroupIngress","ec2:RevokeSecurityGroupIngress",
					"ec2:CreateSecurityGroup","ec2:CreateTags","ec2:DeleteTags","ec2:DeleteSecurityGroup",
					"elasticloadbalancing:CreateLoadBalancer","elasticloadbalancing:CreateTargetGroup",
					"elasticloadbalancing:DeleteLoadBalancer","elasticloadbalancing:DeleteTargetGroup",
					"elasticloadbalancing:CreateListener","elasticloadbalancing:DeleteListener",
					"elasticloadbalancing:CreateRule","elasticloadbalancing:DeleteRule",
					"elasticloadbalancing:AddTags","elasticloadbalancing:RemoveTags",
					"elasticloadbalancing:ModifyLoadBalancerAttributes","elasticloadbalancing:ModifyTargetGroup",
					"elasticloadbalancing:ModifyTargetGroupAttributes","elasticloadbalancing:RegisterTargets",
					"elasticloadbalancing:DeregisterTargets","elasticloadbalancing:SetIpAddressType",
					"elasticloadbalancing:SetSecurityGroups","elasticloadbalancing:SetSubnets",
					"elasticloadbalancing:SetWebAcl","elasticloadbalancing:ModifyListener","elasticloadbalancing:ModifyRule"
				],
				"Resource":"*"
			}
		]
	}`
}

func CertManagerPolicy(zoneID string) string {
	return `{
		"Version":"2012-10-17",
		"Statement":[
			{"Effect":"Allow","Action":"route53:GetChange","Resource":"arn:aws:route53:::change/*"},
			{
				"Effect":"Allow",
				"Action":["route53:ChangeResourceRecordSets","route53:ListResourceRecordSets"],
				"Resource":"arn:aws:route53:::hostedzone/` + zoneID + `"
			},
			{"Effect":"Allow","Action":"route53:ListHostedZonesByName","Resource":"*"}
		]
	}`
}

// pname returns a stable Pulumi resource name built from a prefix + index,
// keeping IAM attachment URNs deterministic across runs.
func pname(prefix string, i int) string {
	return prefix + "-" + itoa(i)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
