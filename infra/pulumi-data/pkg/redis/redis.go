// Package redis provisions an ElastiCache Redis 7.x replication group in
// cluster mode (NumNodeGroups × ReplicasPerNodeGroup).
//
// Dev:     1 shard × 1 replica, single-AZ.
// Staging: 1 shard × 1 replica.
// Prod:    3 shards × 2 replicas, multi-AZ + auto-failover.
//
// In-transit TLS is on. At-rest encryption uses the redis CMK. The AUTH
// token is random + posted into Secrets Manager.
package redis

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/elasticache"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/secretsmanager"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	pkg "ironflyer/infra/pulumi-data/pkg"
	myKMS "ironflyer/infra/pulumi-data/pkg/kms"
	mySecrets "ironflyer/infra/pulumi-data/pkg/secrets"
)

// Redis is the bundle the data layer exports.
type Redis struct {
	ReplicationGroup *elasticache.ReplicationGroup
	PrimaryEndpoint  pulumi.StringOutput
	ReaderEndpoint   pulumi.StringOutput
	ConfigEndpoint   pulumi.StringOutput
	Port             pulumi.IntOutput
	SecurityGroup    *ec2.SecurityGroup
}

func Provision(
	ctx *pulumi.Context,
	env *pkg.Env,
	comp *pkg.Compute,
	keys *myKMS.Keys,
	secrets *mySecrets.Secrets,
) (*Redis, error) {

	sg, err := ec2.NewSecurityGroup(ctx, env.Name("redis-sg"), &ec2.SecurityGroupArgs{
		Name:        pulumi.String(env.Name("redis-sg")),
		Description: pulumi.String("ElastiCache Redis ingress from EKS nodes"),
		VpcId:       comp.VpcID,
		Egress: ec2.SecurityGroupEgressArray{
			&ec2.SecurityGroupEgressArgs{
				Protocol:   pulumi.String("-1"),
				FromPort:   pulumi.Int(0),
				ToPort:     pulumi.Int(0),
				CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
			},
		},
		Tags: env.Tags,
	})
	if err != nil {
		return nil, fmt.Errorf("redis sg: %w", err)
	}
	_, err = ec2.NewSecurityGroupRule(ctx, env.Name("redis-ingress-nodes"), &ec2.SecurityGroupRuleArgs{
		Type:                  pulumi.String("ingress"),
		SecurityGroupId:       sg.ID(),
		SourceSecurityGroupId: comp.NodeSecurityGroup,
		FromPort:              pulumi.Int(6379),
		ToPort:                pulumi.Int(6379),
		Protocol:              pulumi.String("tcp"),
		Description:           pulumi.String("EKS nodes -> Redis"),
	})
	if err != nil {
		return nil, fmt.Errorf("redis ingress: %w", err)
	}

	subnetGroup, err := elasticache.NewSubnetGroup(ctx, env.Name("redis-subnets"), &elasticache.SubnetGroupArgs{
		Name:        pulumi.String(env.Name("redis-subnets")),
		Description: pulumi.String("Ironflyer Redis private subnets"),
		SubnetIds:   comp.PrivateSubnetIDs,
		Tags:        env.Tags,
	})
	if err != nil {
		return nil, fmt.Errorf("redis subnet group: %w", err)
	}

	paramGroup, err := elasticache.NewParameterGroup(ctx, env.Name("redis-params"), &elasticache.ParameterGroupArgs{
		Name:        pulumi.String(env.Name("redis-params")),
		Family:      pulumi.String(fmt.Sprintf("redis%s", majorFromVersion(env.RedisEngineVersion))),
		Description: pulumi.String("Ironflyer Redis params (cluster mode on)"),
		Parameters: elasticache.ParameterGroupParameterArray{
			&elasticache.ParameterGroupParameterArgs{
				Name:  pulumi.String("cluster-enabled"),
				Value: pulumi.String("yes"),
			},
		},
		Tags: env.Tags,
	})
	if err != nil {
		return nil, fmt.Errorf("redis param group: %w", err)
	}

	auth, err := random.NewRandomPassword(ctx, env.Name("redis-auth"), &random.RandomPasswordArgs{
		Length:  pulumi.Int(48),
		Special: pulumi.Bool(false),
	})
	if err != nil {
		return nil, fmt.Errorf("redis random auth: %w", err)
	}

	multiAZ := env.IsProd
	rg, err := elasticache.NewReplicationGroup(ctx, env.Name("redis"), &elasticache.ReplicationGroupArgs{
		ReplicationGroupId:       pulumi.String(env.Name("redis")),
		Description:              pulumi.String("Ironflyer Redis (cluster mode)"),
		Engine:                   pulumi.String("redis"),
		EngineVersion:            pulumi.String(env.RedisEngineVersion),
		NodeType:                 pulumi.String(redisNodeType(env)),
		NumNodeGroups:            pulumi.Int(env.RedisShards),
		ReplicasPerNodeGroup:     pulumi.Int(env.RedisReplicasPerShard),
		ParameterGroupName:       paramGroup.Name,
		SubnetGroupName:          subnetGroup.Name,
		SecurityGroupIds:         pulumi.StringArray{sg.ID().ToStringOutput()},
		AtRestEncryptionEnabled:  pulumi.Bool(true),
		KmsKeyId:                 keys.Redis.Arn,
		TransitEncryptionEnabled: pulumi.Bool(true),
		AuthToken:                auth.Result,
		AutomaticFailoverEnabled: pulumi.Bool(true),
		MultiAzEnabled:           pulumi.Bool(multiAZ),
		Port:                     pulumi.Int(6379),
		SnapshotRetentionLimit:   pulumi.Int(env.RedisSnapshotRetentionDays),
		SnapshotWindow:           pulumi.String("01:00-02:00"),
		MaintenanceWindow:        pulumi.String("sun:04:30-sun:05:30"),
		ApplyImmediately:         pulumi.Bool(!env.IsProd),
		Tags:                     env.Tags,
	})
	if err != nil {
		return nil, fmt.Errorf("redis replication group: %w", err)
	}

	// Post the auth token + URL into Secrets Manager. URL format
	// matches the orchestrator's Redis client expectations
	// (rediss:// for TLS, AUTH baked in).
	secretJSON := pulumi.All(rg.ConfigurationEndpointAddress, auth.Result).ApplyT(func(args []interface{}) (string, error) {
		endpoint := args[0].(string)
		token := args[1].(string)
		url := fmt.Sprintf("rediss://default:%s@%s:6379", token, endpoint)
		doc := map[string]string{
			"REDIS_URL":  url,
			"REDIS_AUTH": token,
			"REDIS_HOST": endpoint,
			"REDIS_PORT": "6379",
		}
		b, err := json.Marshal(doc)
		return string(b), err
	}).(pulumi.StringOutput)
	_, err = secretsmanager.NewSecretVersion(ctx, env.Name("redis-secret-version"), &secretsmanager.SecretVersionArgs{
		SecretId:     secrets.RedisAuth.ID(),
		SecretString: secretJSON,
	}, pulumi.DependsOn([]pulumi.Resource{rg}))
	if err != nil {
		return nil, fmt.Errorf("redis secret version: %w", err)
	}

	return &Redis{
		ReplicationGroup: rg,
		PrimaryEndpoint:  rg.PrimaryEndpointAddress,
		ReaderEndpoint:   rg.ReaderEndpointAddress,
		ConfigEndpoint:   rg.ConfigurationEndpointAddress,
		Port:             rg.Port.Elem(),
		SecurityGroup:    sg,
	}, nil
}

func redisNodeType(env *pkg.Env) string {
	if env.IsProd {
		return "cache.r7g.large"
	}
	return "cache.t4g.small"
}

func majorFromVersion(v string) string {
	for i := 0; i < len(v); i++ {
		if v[i] == '.' {
			return v[:i]
		}
	}
	return v
}
