package compute

import (
	"fmt"
	"net"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cloudwatch"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	awsiam "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/rds"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Network bundles every networking resource that downstream modules need
// access to. All fields are Pulumi outputs (or slices of them).
type Network struct {
	VpcID             pulumi.IDOutput
	VpcCidr           pulumi.StringOutput
	PublicSubnetIDs   pulumi.StringArrayOutput
	PrivateSubnetIDs  pulumi.StringArrayOutput
	DBSubnetIDs       pulumi.StringArrayOutput
	DBSubnetGroupName pulumi.StringOutput
	ClusterSGID       pulumi.IDOutput
	AZs               []string
}

// NewVPC provisions a 3-AZ VPC with three subnet tiers (public, private, db),
// NAT gateways (single-NAT mode in dev), VPC flow logs, and the gateway +
// interface endpoints the orchestrator and runtime need.
func NewVPC(ctx *pulumi.Context, cfg *Config) (*Network, error) {
	azs, err := aws.GetAvailabilityZones(ctx, &aws.GetAvailabilityZonesArgs{
		State: pulumi.StringRef("available"),
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("describe AZs: %w", err)
	}
	if len(azs.Names) < 3 {
		return nil, fmt.Errorf("region %s has %d AZs, need 3", cfg.Region, len(azs.Names))
	}
	az := azs.Names[:3]

	vpc, err := ec2.NewVpc(ctx, "ironflyer-vpc", &ec2.VpcArgs{
		CidrBlock:          pulumi.String(cfg.VpcCidr),
		EnableDnsHostnames: pulumi.Bool(true),
		EnableDnsSupport:   pulumi.Bool(true),
		Tags: cfg.TagsWith(map[string]string{
			"Name": "ironflyer-" + cfg.Stack,
			// EKS subnet discovery tags applied to subnets, not VPC.
		}),
	})
	if err != nil {
		return nil, err
	}

	igw, err := ec2.NewInternetGateway(ctx, "ironflyer-igw", &ec2.InternetGatewayArgs{
		VpcId: vpc.ID(),
		Tags:  cfg.TagsWith(map[string]string{"Name": "ironflyer-igw"}),
	})
	if err != nil {
		return nil, err
	}

	// Carve /20 (public + private) and /24 (db) blocks deterministically from
	// the configured /16. Layout for 10.x.0.0/16:
	//   public:  10.x.0.0/20,   10.x.16.0/20,  10.x.32.0/20
	//   private: 10.x.48.0/20,  10.x.64.0/20,  10.x.80.0/20
	//   db:      10.x.96.0/24,  10.x.97.0/24,  10.x.98.0/24
	subnets, err := planSubnets(cfg.VpcCidr)
	if err != nil {
		return nil, err
	}

	publicIDs := make([]pulumi.StringInput, 3)
	publicSubnetRefs := make([]*ec2.Subnet, 3)
	privateIDs := make([]pulumi.StringInput, 3)
	dbIDs := make([]pulumi.StringInput, 3)

	// Public subnets share one route table -> IGW.
	publicRT, err := ec2.NewRouteTable(ctx, "ironflyer-rt-public", &ec2.RouteTableArgs{
		VpcId: vpc.ID(),
		Routes: ec2.RouteTableRouteArray{&ec2.RouteTableRouteArgs{
			CidrBlock: pulumi.String("0.0.0.0/0"),
			GatewayId: igw.ID(),
		}},
		Tags: cfg.TagsWith(map[string]string{"Name": "ironflyer-rt-public"}),
	})
	if err != nil {
		return nil, err
	}

	for i := 0; i < 3; i++ {
		sn, err := ec2.NewSubnet(ctx, fmt.Sprintf("public-%d", i), &ec2.SubnetArgs{
			VpcId:               vpc.ID(),
			CidrBlock:           pulumi.String(subnets.public[i]),
			AvailabilityZone:    pulumi.String(az[i]),
			MapPublicIpOnLaunch: pulumi.Bool(true),
			Tags: cfg.TagsWith(map[string]string{
				"Name":                            fmt.Sprintf("ironflyer-public-%d", i),
				"kubernetes.io/role/elb":          "1",
				"kubernetes.io/cluster/ironflyer": "shared",
				"Tier":                            "public",
			}),
		})
		if err != nil {
			return nil, err
		}
		publicIDs[i] = sn.ID()
		publicSubnetRefs[i] = sn

		if _, err := ec2.NewRouteTableAssociation(ctx, fmt.Sprintf("public-rta-%d", i), &ec2.RouteTableAssociationArgs{
			SubnetId:     sn.ID(),
			RouteTableId: publicRT.ID(),
		}); err != nil {
			return nil, err
		}
	}

	// NAT gateways: one per AZ (HA) unless SingleNatGateway is set.
	natCount := 3
	if cfg.SingleNatGateway {
		natCount = 1
	}
	natGWs := make([]*ec2.NatGateway, natCount)
	for i := 0; i < natCount; i++ {
		eip, err := ec2.NewEip(ctx, fmt.Sprintf("nat-eip-%d", i), &ec2.EipArgs{
			Domain: pulumi.String("vpc"),
			Tags:   cfg.TagsWith(map[string]string{"Name": fmt.Sprintf("ironflyer-nat-eip-%d", i)}),
		})
		if err != nil {
			return nil, err
		}
		ng, err := ec2.NewNatGateway(ctx, fmt.Sprintf("nat-%d", i), &ec2.NatGatewayArgs{
			AllocationId: eip.ID(),
			SubnetId:     publicSubnetRefs[i].ID(),
			Tags:         cfg.TagsWith(map[string]string{"Name": fmt.Sprintf("ironflyer-nat-%d", i)}),
		})
		if err != nil {
			return nil, err
		}
		natGWs[i] = ng
	}

	for i := 0; i < 3; i++ {
		// Private subnets: each AZ has its own route table -> NAT in same AZ
		// (or NAT[0] in single-NAT mode).
		nat := natGWs[0]
		if !cfg.SingleNatGateway {
			nat = natGWs[i]
		}
		rt, err := ec2.NewRouteTable(ctx, fmt.Sprintf("rt-private-%d", i), &ec2.RouteTableArgs{
			VpcId: vpc.ID(),
			Routes: ec2.RouteTableRouteArray{&ec2.RouteTableRouteArgs{
				CidrBlock:    pulumi.String("0.0.0.0/0"),
				NatGatewayId: nat.ID(),
			}},
			Tags: cfg.TagsWith(map[string]string{"Name": fmt.Sprintf("ironflyer-rt-private-%d", i)}),
		})
		if err != nil {
			return nil, err
		}
		sn, err := ec2.NewSubnet(ctx, fmt.Sprintf("private-%d", i), &ec2.SubnetArgs{
			VpcId:            vpc.ID(),
			CidrBlock:        pulumi.String(subnets.private[i]),
			AvailabilityZone: pulumi.String(az[i]),
			Tags: cfg.TagsWith(map[string]string{
				"Name":                            fmt.Sprintf("ironflyer-private-%d", i),
				"kubernetes.io/role/internal-elb": "1",
				"kubernetes.io/cluster/ironflyer": "shared",
				"Tier":                            "private",
			}),
		})
		if err != nil {
			return nil, err
		}
		privateIDs[i] = sn.ID()
		if _, err := ec2.NewRouteTableAssociation(ctx, fmt.Sprintf("private-rta-%d", i), &ec2.RouteTableAssociationArgs{
			SubnetId:     sn.ID(),
			RouteTableId: rt.ID(),
		}); err != nil {
			return nil, err
		}
	}

	// DB subnets: no internet egress. Shared route table is fine.
	dbRT, err := ec2.NewRouteTable(ctx, "ironflyer-rt-db", &ec2.RouteTableArgs{
		VpcId: vpc.ID(),
		Tags:  cfg.TagsWith(map[string]string{"Name": "ironflyer-rt-db"}),
	})
	if err != nil {
		return nil, err
	}
	dbSubnetResources := make([]pulumi.StringInput, 3)
	for i := 0; i < 3; i++ {
		sn, err := ec2.NewSubnet(ctx, fmt.Sprintf("db-%d", i), &ec2.SubnetArgs{
			VpcId:            vpc.ID(),
			CidrBlock:        pulumi.String(subnets.db[i]),
			AvailabilityZone: pulumi.String(az[i]),
			Tags: cfg.TagsWith(map[string]string{
				"Name": fmt.Sprintf("ironflyer-db-%d", i),
				"Tier": "db",
			}),
		})
		if err != nil {
			return nil, err
		}
		dbIDs[i] = sn.ID()
		dbSubnetResources[i] = sn.ID()
		if _, err := ec2.NewRouteTableAssociation(ctx, fmt.Sprintf("db-rta-%d", i), &ec2.RouteTableAssociationArgs{
			SubnetId:     sn.ID(),
			RouteTableId: dbRT.ID(),
		}); err != nil {
			return nil, err
		}
	}

	dbGroup, err := rds.NewSubnetGroup(ctx, "db-subnet-group", &rds.SubnetGroupArgs{
		Description: pulumi.String("Ironflyer DB subnet group (consumed by data stack)"),
		SubnetIds:   pulumi.StringArray(dbSubnetResources),
		Tags:        cfg.TagsWith(map[string]string{"Name": "ironflyer-db-subnet-group"}),
	})
	if err != nil {
		return nil, err
	}

	// VPC flow logs to CloudWatch.
	flowLogGroup, err := cloudwatch.NewLogGroup(ctx, "vpc-flow-logs", &cloudwatch.LogGroupArgs{
		Name:            pulumi.String("/aws/vpc/ironflyer-" + cfg.Stack + "/flow"),
		RetentionInDays: pulumi.Int(30),
		Tags:            cfg.Tags(),
	})
	if err != nil {
		return nil, err
	}
	flowLogRole, err := awsiam.NewRole(ctx, "vpc-flow-log-role", &awsiam.RoleArgs{
		AssumeRolePolicy: pulumi.String(`{
			"Version":"2012-10-17",
			"Statement":[{"Effect":"Allow","Principal":{"Service":"vpc-flow-logs.amazonaws.com"},"Action":"sts:AssumeRole"}]
		}`),
		Tags: cfg.Tags(),
	})
	if err != nil {
		return nil, err
	}
	if _, err := awsiam.NewRolePolicy(ctx, "vpc-flow-log-policy", &awsiam.RolePolicyArgs{
		Role: flowLogRole.ID(),
		Policy: flowLogGroup.Arn.ApplyT(func(arn string) string {
			return `{
				"Version":"2012-10-17",
				"Statement":[{
					"Effect":"Allow",
					"Action":["logs:CreateLogStream","logs:PutLogEvents","logs:DescribeLogStreams","logs:DescribeLogGroups"],
					"Resource":["` + arn + `","` + arn + `:*"]
				}]
			}`
		}).(pulumi.StringOutput),
	}); err != nil {
		return nil, err
	}
	if _, err := ec2.NewFlowLog(ctx, "vpc-flow-log", &ec2.FlowLogArgs{
		IamRoleArn:     flowLogRole.Arn,
		LogDestination: flowLogGroup.Arn,
		TrafficType:    pulumi.String("ALL"),
		VpcId:          vpc.ID(),
		Tags:           cfg.Tags(),
	}); err != nil {
		return nil, err
	}

	// Gateway endpoint: S3.
	if _, err := ec2.NewVpcEndpoint(ctx, "vpce-s3", &ec2.VpcEndpointArgs{
		VpcId:        vpc.ID(),
		ServiceName:  pulumi.String("com.amazonaws." + cfg.Region + ".s3"),
		VpcEndpointType: pulumi.String("Gateway"),
		RouteTableIds:   pulumi.StringArray{dbRT.ID(), publicRT.ID()},
		Tags:            cfg.TagsWith(map[string]string{"Name": "ironflyer-vpce-s3"}),
	}); err != nil {
		return nil, err
	}

	// SG for interface endpoints — allow 443 from the VPC CIDR.
	endpointSG, err := ec2.NewSecurityGroup(ctx, "vpce-sg", &ec2.SecurityGroupArgs{
		VpcId:       vpc.ID(),
		Description: pulumi.String("Allow HTTPS from VPC to interface endpoints"),
		Ingress: ec2.SecurityGroupIngressArray{&ec2.SecurityGroupIngressArgs{
			Protocol:   pulumi.String("tcp"),
			FromPort:   pulumi.Int(443),
			ToPort:     pulumi.Int(443),
			CidrBlocks: pulumi.StringArray{pulumi.String(cfg.VpcCidr)},
		}},
		Egress: ec2.SecurityGroupEgressArray{&ec2.SecurityGroupEgressArgs{
			Protocol:   pulumi.String("-1"),
			FromPort:   pulumi.Int(0),
			ToPort:     pulumi.Int(0),
			CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
		}},
		Tags: cfg.TagsWith(map[string]string{"Name": "ironflyer-vpce-sg"}),
	})
	if err != nil {
		return nil, err
	}

	interfaceServices := []string{"ecr.api", "ecr.dkr", "sts", "logs", "secretsmanager"}
	for _, svc := range interfaceServices {
		if _, err := ec2.NewVpcEndpoint(ctx, "vpce-"+svc, &ec2.VpcEndpointArgs{
			VpcId:             vpc.ID(),
			ServiceName:       pulumi.String("com.amazonaws." + cfg.Region + "." + svc),
			VpcEndpointType:   pulumi.String("Interface"),
			PrivateDnsEnabled: pulumi.Bool(true),
			SubnetIds:         pulumi.StringArray(privateIDs),
			SecurityGroupIds:  pulumi.StringArray{endpointSG.ID()},
			Tags:              cfg.TagsWith(map[string]string{"Name": "ironflyer-vpce-" + svc}),
		}); err != nil {
			return nil, err
		}
	}

	// Cluster security group — EKS will create its own too, but this one is
	// for shared additions (e.g. db security groups reference it as source).
	clusterSG, err := ec2.NewSecurityGroup(ctx, "cluster-sg-shared", &ec2.SecurityGroupArgs{
		VpcId:       vpc.ID(),
		Description: pulumi.String("Shared cluster SG (referenced by data stack RDS/Elasticache)"),
		Egress: ec2.SecurityGroupEgressArray{&ec2.SecurityGroupEgressArgs{
			Protocol:   pulumi.String("-1"),
			FromPort:   pulumi.Int(0),
			ToPort:     pulumi.Int(0),
			CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
		}},
		Tags: cfg.TagsWith(map[string]string{"Name": "ironflyer-cluster-sg-shared"}),
	})
	if err != nil {
		return nil, err
	}

	return &Network{
		VpcID:             vpc.ID(),
		VpcCidr:           pulumi.String(cfg.VpcCidr).ToStringOutput(),
		PublicSubnetIDs:   pulumi.StringArray(publicIDs).ToStringArrayOutput(),
		PrivateSubnetIDs:  pulumi.StringArray(privateIDs).ToStringArrayOutput(),
		DBSubnetIDs:       pulumi.StringArray(dbIDs).ToStringArrayOutput(),
		DBSubnetGroupName: dbGroup.Name,
		ClusterSGID:       clusterSG.ID(),
		AZs:               az,
	}, nil
}

type subnetPlan struct {
	public  []string
	private []string
	db      []string
}

// planSubnets carves /20 and /24 blocks out of the configured /16.
// Layout matches the comment in NewVPC.
func planSubnets(cidr string) (*subnetPlan, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("parse vpc cidr %q: %w", cidr, err)
	}
	if ones, _ := ipnet.Mask.Size(); ones != 16 {
		return nil, fmt.Errorf("vpc cidr must be /16, got /%d", ones)
	}
	b := ip.To4()
	if b == nil {
		return nil, fmt.Errorf("vpc cidr must be IPv4")
	}
	pub := []string{
		fmt.Sprintf("%d.%d.0.0/20", b[0], b[1]),
		fmt.Sprintf("%d.%d.16.0/20", b[0], b[1]),
		fmt.Sprintf("%d.%d.32.0/20", b[0], b[1]),
	}
	pri := []string{
		fmt.Sprintf("%d.%d.48.0/20", b[0], b[1]),
		fmt.Sprintf("%d.%d.64.0/20", b[0], b[1]),
		fmt.Sprintf("%d.%d.80.0/20", b[0], b[1]),
	}
	db := []string{
		fmt.Sprintf("%d.%d.96.0/24", b[0], b[1]),
		fmt.Sprintf("%d.%d.97.0/24", b[0], b[1]),
		fmt.Sprintf("%d.%d.98.0/24", b[0], b[1]),
	}
	return &subnetPlan{public: pub, private: pri, db: db}, nil
}
