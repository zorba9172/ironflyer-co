package data

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/elasticache"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Redis is the ElastiCache cluster + connection details the orchestrator
// uses for locks, rate limit counters, and pub/sub.
type Redis struct {
	Replication      *elasticache.ReplicationGroup
	PrimaryEndpoint  pulumi.StringOutput
	ReaderEndpoint   pulumi.StringOutput
	SubnetGroup      *elasticache.SubnetGroup
	ParameterGroup   *elasticache.ParameterGroup
}

func provisionRedis(ctx *pulumi.Context, env *stackEnv, deps Compute, k *KMSKeys, dataSG *ec2.SecurityGroup, secrets *Secrets) (*Redis, error) {
	// Cluster SG -> data SG on 6379.
	if _, err := ec2.NewSecurityGroupRule(ctx, name(env, "data-sg-ingress-redis"), &ec2.SecurityGroupRuleArgs{
		Type:                  pulumi.String("ingress"),
		FromPort:              pulumi.Int(6379),
		ToPort:                pulumi.Int(6379),
		Protocol:              pulumi.String("tcp"),
		SecurityGroupId:       dataSG.ID(),
		SourceSecurityGroupId: deps.ClusterSGID,
		Description:           pulumi.String("Redis from EKS cluster SG"),
	}); err != nil {
		return nil, err
	}

	subnetGroup, err := elasticache.NewSubnetGroup(ctx, name(env, "redis-subnets"), &elasticache.SubnetGroupArgs{
		Name:      pulumi.String(name(env, "redis-subnets")),
		SubnetIds: deps.PrivateSubnetIDs,
		Tags:      env.tags,
	})
	if err != nil {
		return nil, err
	}

	pg, err := elasticache.NewParameterGroup(ctx, name(env, "redis-params"), &elasticache.ParameterGroupArgs{
		Name:        pulumi.String(name(env, "redis-params")),
		Family:      pulumi.String("redis7"),
		Description: pulumi.String("Ironflyer Redis 7 params (LRU eviction, idle timeout)"),
		Parameters: elasticache.ParameterGroupParameterArray{
			&elasticache.ParameterGroupParameterArgs{Name: pulumi.String("maxmemory-policy"), Value: pulumi.String("allkeys-lru")},
			&elasticache.ParameterGroupParameterArgs{Name: pulumi.String("timeout"), Value: pulumi.String("300")},
		},
		Tags: env.tags,
	})
	if err != nil {
		return nil, err
	}

	// Auth token comes from the random-generated Secrets Manager entry.
	authToken := secrets.RedisAuthVersion.SecretString.ApplyT(func(s string) (string, error) {
		return extractJSONField(s, "auth_token")
	}).(pulumi.StringOutput)

	// Cluster mode disabled (single logical store per stack memory). We
	// model "shards" as additional replicas in the replication group so
	// the orchestrator sees one writer + N read replicas behind the
	// primary endpoint.
	numCacheClusters := 2 // primary + 1 replica
	if env.isProd {
		numCacheClusters = 1 + env.redisShards // primary + N replicas
	}

	nodeType := "cache.t4g.small"
	if env.isProd {
		nodeType = "cache.r7g.large"
	}

	rg, err := elasticache.NewReplicationGroup(ctx, name(env, "redis"), &elasticache.ReplicationGroupArgs{
		ReplicationGroupId:       pulumi.String(name(env, "redis")),
		Description:              pulumi.String("Ironflyer Redis (orchestrator locks + rate limit + pub/sub)"),
		Engine:                   pulumi.String("redis"),
		EngineVersion:            pulumi.String("7.1"),
		NodeType:                 pulumi.String(nodeType),
		Port:                     pulumi.Int(6379),
		ParameterGroupName:       pg.Name,
		SubnetGroupName:          subnetGroup.Name,
		SecurityGroupIds:         pulumi.StringArray{dataSG.ID()},
		AutomaticFailoverEnabled: pulumi.Bool(true),
		MultiAzEnabled:           pulumi.Bool(true),
		NumCacheClusters:         pulumi.Int(numCacheClusters),
		AtRestEncryptionEnabled:  pulumi.Bool(true),
		TransitEncryptionEnabled: pulumi.Bool(true),
		KmsKeyId:                 k.RDSKey.Arn, // RDS CMK reused; ElastiCache is also covered by its key policy.
		AuthToken:                authToken,
		SnapshotRetentionLimit:   pulumi.Int(7),
		SnapshotWindow:           pulumi.String("01:00-02:00"),
		MaintenanceWindow:        pulumi.String("sun:03:00-sun:04:00"),
		ApplyImmediately:         pulumi.Bool(!env.isProd),
		Tags:                     env.tags,
	})
	if err != nil {
		return nil, err
	}

	return &Redis{
		Replication:     rg,
		PrimaryEndpoint: rg.PrimaryEndpointAddress,
		ReaderEndpoint:  rg.ReaderEndpointAddress,
		SubnetGroup:     subnetGroup,
		ParameterGroup:  pg,
	}, nil
}
