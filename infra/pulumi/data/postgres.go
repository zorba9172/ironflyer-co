package data

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/appautoscaling"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/rds"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Postgres holds the Aurora cluster + endpoints we surface as stack outputs.
type Postgres struct {
	Cluster        *rds.Cluster
	Writer         *rds.ClusterInstance
	Reader         *rds.ClusterInstance
	ClusterArn     pulumi.StringOutput
	WriterEndpoint pulumi.StringOutput
	ReaderEndpoint pulumi.StringOutput
	ConnectionURL  pulumi.StringOutput
}

// provisionPostgres builds an Aurora Postgres 16 cluster (multi-AZ, IAM
// auth enabled, KMS-encrypted, 30-day PITR backups) with a writer +
// reader, and an autoscaler that pushes the reader pool to 6 in prod.
func provisionPostgres(ctx *pulumi.Context, env *stackEnv, deps Compute, k *KMSKeys, dataSG *ec2.SecurityGroup, secrets *Secrets) (*Postgres, error) {
	// Allow the EKS cluster SG to talk to the data SG on 5432.
	if _, err := ec2.NewSecurityGroupRule(ctx, name(env, "data-sg-ingress-pg"), &ec2.SecurityGroupRuleArgs{
		Type:                  pulumi.String("ingress"),
		FromPort:              pulumi.Int(5432),
		ToPort:                pulumi.Int(5432),
		Protocol:              pulumi.String("tcp"),
		SecurityGroupId:       dataSG.ID(),
		SourceSecurityGroupId: deps.ClusterSGID,
		Description:           pulumi.String("Postgres from EKS cluster SG"),
	}); err != nil {
		return nil, err
	}

	// Parameter group: tighten logging without going overboard on volume.
	clusterPG, err := rds.NewClusterParameterGroup(ctx, name(env, "pg-cluster-params"), &rds.ClusterParameterGroupArgs{
		Family:      pulumi.String("aurora-postgresql16"),
		Description: pulumi.String("Ironflyer Aurora Postgres 16 cluster params"),
		Parameters: rds.ClusterParameterGroupParameterArray{
			&rds.ClusterParameterGroupParameterArgs{Name: pulumi.String("log_min_duration_statement"), Value: pulumi.String("500")},
			&rds.ClusterParameterGroupParameterArgs{Name: pulumi.String("log_statement"), Value: pulumi.String("ddl")},
			&rds.ClusterParameterGroupParameterArgs{Name: pulumi.String("pgaudit.log"), Value: pulumi.String("ddl,role")},
			&rds.ClusterParameterGroupParameterArgs{Name: pulumi.String("shared_preload_libraries"), Value: pulumi.String("pgaudit"), ApplyMethod: pulumi.String("pending-reboot")},
		},
		Tags: env.tags,
	})
	if err != nil {
		return nil, err
	}

	instancePG, err := rds.NewParameterGroup(ctx, name(env, "pg-instance-params"), &rds.ParameterGroupArgs{
		Family:      pulumi.String("aurora-postgresql16"),
		Description: pulumi.String("Ironflyer Aurora Postgres 16 instance params"),
		Tags:        env.tags,
	})
	if err != nil {
		return nil, err
	}

	// Pull username + password out of the Secrets Manager-backed JSON.
	masterPassword := secrets.PostgresMasterVersion.SecretString.ApplyT(func(s string) (string, error) {
		return extractJSONField(s, "password")
	}).(pulumi.StringOutput)

	backupRetention := 7
	if env.isProd {
		backupRetention = 30
	}

	cluster, err := rds.NewCluster(ctx, name(env, "pg-cluster"), &rds.ClusterArgs{
		ClusterIdentifier:               pulumi.String(name(env, "pg")),
		Engine:                          pulumi.String("aurora-postgresql"),
		EngineVersion:                   pulumi.String("16.4"),
		EngineMode:                      pulumi.String("provisioned"),
		DatabaseName:                    pulumi.String("ironflyer"),
		MasterUsername:                  pulumi.String("ironflyer_master"),
		MasterPassword:                  masterPassword,
		DbSubnetGroupName:               deps.DBSubnetGroupName,
		VpcSecurityGroupIds:             pulumi.StringArray{dataSG.ID()},
		StorageEncrypted:                pulumi.Bool(true),
		KmsKeyId:                        k.RDSKey.Arn,
		BackupRetentionPeriod:           pulumi.Int(backupRetention),
		PreferredBackupWindow:           pulumi.String("02:00-03:00"),
		PreferredMaintenanceWindow:      pulumi.String("sun:03:30-sun:05:00"),
		DeletionProtection:              pulumi.Bool(env.isProd),
		IamDatabaseAuthenticationEnabled: pulumi.Bool(true),
		CopyTagsToSnapshot:              pulumi.Bool(true),
		DbClusterParameterGroupName:     clusterPG.Name,
		EnabledCloudwatchLogsExports:    pulumi.StringArray{pulumi.String("postgresql")},
		SkipFinalSnapshot:               pulumi.Bool(!env.isProd),
		FinalSnapshotIdentifier:         pulumi.String(name(env, "pg-final")),
		ApplyImmediately:                pulumi.Bool(!env.isProd),
		Tags:                            env.tags,
	})
	if err != nil {
		return nil, err
	}

	writer, err := rds.NewClusterInstance(ctx, name(env, "pg-writer"), &rds.ClusterInstanceArgs{
		Identifier:                  pulumi.String(name(env, "pg-writer")),
		ClusterIdentifier:           cluster.ID(),
		InstanceClass:               pulumi.String(env.postgresInstance),
		Engine:                      pulumi.String("aurora-postgresql"),
		EngineVersion:               cluster.EngineVersion,
		DbParameterGroupName:        instancePG.Name,
		DbSubnetGroupName:           deps.DBSubnetGroupName,
		PerformanceInsightsEnabled:  pulumi.Bool(true),
		PerformanceInsightsKmsKeyId: k.RDSKey.Arn,
		PerformanceInsightsRetentionPeriod: pulumi.Int(7),
		PubliclyAccessible:          pulumi.Bool(false),
		PromotionTier:               pulumi.Int(0),
		Tags:                        env.tags,
	})
	if err != nil {
		return nil, err
	}

	reader, err := rds.NewClusterInstance(ctx, name(env, "pg-reader"), &rds.ClusterInstanceArgs{
		Identifier:                  pulumi.String(name(env, "pg-reader")),
		ClusterIdentifier:           cluster.ID(),
		InstanceClass:               pulumi.String(env.postgresInstance),
		Engine:                      pulumi.String("aurora-postgresql"),
		EngineVersion:               cluster.EngineVersion,
		DbParameterGroupName:        instancePG.Name,
		DbSubnetGroupName:           deps.DBSubnetGroupName,
		PerformanceInsightsEnabled:  pulumi.Bool(true),
		PerformanceInsightsKmsKeyId: k.RDSKey.Arn,
		PerformanceInsightsRetentionPeriod: pulumi.Int(7),
		PubliclyAccessible:          pulumi.Bool(false),
		PromotionTier:               pulumi.Int(1),
		Tags:                        env.tags,
	})
	if err != nil {
		return nil, err
	}

	// In prod we autoscale readers 2..6 by CPU. Dev stays at fixed 1+1.
	if env.isProd {
		target, err := appautoscaling.NewTarget(ctx, name(env, "pg-reader-target"), &appautoscaling.TargetArgs{
			ServiceNamespace:  pulumi.String("rds"),
			ScalableDimension: pulumi.String("rds:cluster:ReadReplicaCount"),
			ResourceId:        pulumi.Sprintf("cluster:%s", cluster.ClusterIdentifier),
			MinCapacity:       pulumi.Int(2),
			MaxCapacity:       pulumi.Int(6),
		})
		if err != nil {
			return nil, err
		}
		if _, err := appautoscaling.NewPolicy(ctx, name(env, "pg-reader-cpu"), &appautoscaling.PolicyArgs{
			PolicyType:        pulumi.String("TargetTrackingScaling"),
			ServiceNamespace:  target.ServiceNamespace,
			ScalableDimension: target.ScalableDimension,
			ResourceId:        target.ResourceId,
			TargetTrackingScalingPolicyConfiguration: &appautoscaling.PolicyTargetTrackingScalingPolicyConfigurationArgs{
				TargetValue: pulumi.Float64(60.0),
				PredefinedMetricSpecification: &appautoscaling.PolicyTargetTrackingScalingPolicyConfigurationPredefinedMetricSpecificationArgs{
					PredefinedMetricType: pulumi.String("RDSReaderAverageCPUUtilization"),
				},
			},
		}); err != nil {
			return nil, err
		}
	}

	url := pulumi.All(cluster.Endpoint, masterPassword).ApplyT(func(args []any) string {
		host := args[0].(string)
		pw := args[1].(string)
		return fmt.Sprintf("postgres://ironflyer_master:%s@%s:5432/ironflyer?sslmode=require", pw, host)
	}).(pulumi.StringOutput)

	return &Postgres{
		Cluster:        cluster,
		Writer:         writer,
		Reader:         reader,
		ClusterArn:     cluster.Arn,
		WriterEndpoint: cluster.Endpoint,
		ReaderEndpoint: cluster.ReaderEndpoint,
		ConnectionURL:  url,
	}, nil
}

// extractJSONField is a tiny convenience parser: it pulls a single
// top-level string field out of the placeholder JSON we wrote to Secrets
// Manager. We don't pull in encoding/json + a struct because the secret
// shape is fixed and the resource expects a Pulumi-side string.
func extractJSONField(blob, field string) (string, error) {
	key := fmt.Sprintf("%q:", field)
	i := indexOf(blob, key)
	if i < 0 {
		return "", fmt.Errorf("field %q not found in secret JSON", field)
	}
	i += len(key)
	// skip whitespace + opening quote.
	for i < len(blob) && (blob[i] == ' ' || blob[i] == '\t') {
		i++
	}
	if i >= len(blob) || blob[i] != '"' {
		return "", fmt.Errorf("malformed secret JSON near field %q", field)
	}
	i++
	end := i
	for end < len(blob) && blob[end] != '"' {
		if blob[end] == '\\' {
			end++
		}
		end++
	}
	if end >= len(blob) {
		return "", fmt.Errorf("unterminated string for field %q", field)
	}
	return blob[i:end], nil
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
