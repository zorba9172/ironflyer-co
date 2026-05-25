// Package postgres provisions the Aurora Postgres cluster used by the
// orchestrator (ACID budget ledger, auth, projects, leads).
//
// Topology:
//   - Dev: serverless v2 (0.5–8 ACU), single writer, 7d backups.
//   - Staging: serverless v2 (0.5–8 ACU), 1 writer + 1 reader, 14d backups.
//   - Prod: provisioned db.r6g.large, 1 writer + 2 readers across AZs,
//     35d backups + PITR.
//
// Parameter group highlights (per CLAUDE.md operations bar):
//   - wal_level = logical   (wal-g + CDC ready)
//   - max_connections = 400
//   - statement_timeout = 30000 ms
//   - idle_in_transaction_session_timeout = 60000 ms
//   - log_min_duration_statement = 500
package postgres

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/rds"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/secretsmanager"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	pkg "ironflyer/infra/pulumi-data/pkg"
	myKMS "ironflyer/infra/pulumi-data/pkg/kms"
	mySecrets "ironflyer/infra/pulumi-data/pkg/secrets"
)

// Postgres is the bundle the data layer exports to other components +
// the stack outputs.
type Postgres struct {
	Cluster        *rds.Cluster
	WriterEndpoint pulumi.StringOutput
	ReaderEndpoint pulumi.StringOutput
	Port           pulumi.IntOutput
	ConnectionURL  pulumi.StringOutput
	SecurityGroup  *ec2.SecurityGroup
	ParameterGroup *rds.ClusterParameterGroup
}

// Provision builds the cluster + parameter group + SG + password.
func Provision(
	ctx *pulumi.Context,
	env *pkg.Env,
	comp *pkg.Compute,
	keys *myKMS.Keys,
	secrets *mySecrets.Secrets,
) (*Postgres, error) {

	// 1. SG that allows EKS workers in on 5432.
	sg, err := ec2.NewSecurityGroup(ctx, env.Name("pg-sg"), &ec2.SecurityGroupArgs{
		Name:        pulumi.String(env.Name("pg-sg")),
		Description: pulumi.String("Aurora Postgres ingress from EKS nodes"),
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
		return nil, fmt.Errorf("pg sg: %w", err)
	}

	_, err = ec2.NewSecurityGroupRule(ctx, env.Name("pg-ingress-nodes"), &ec2.SecurityGroupRuleArgs{
		Type:                  pulumi.String("ingress"),
		SecurityGroupId:       sg.ID(),
		SourceSecurityGroupId: comp.NodeSecurityGroup,
		FromPort:              pulumi.Int(5432),
		ToPort:                pulumi.Int(5432),
		Protocol:              pulumi.String("tcp"),
		Description:           pulumi.String("EKS nodes -> Postgres"),
	})
	if err != nil {
		return nil, fmt.Errorf("pg ingress nodes: %w", err)
	}

	// 2. DB subnet group. Prefer the compute stack's pre-built name but
	// fall back to provisioning our own from PrivateSubnetIDs so the data
	// stack stays self-sufficient on day 1.
	subnetGroup, err := rds.NewSubnetGroup(ctx, env.Name("pg-subnets"), &rds.SubnetGroupArgs{
		Name:        pulumi.String(env.Name("pg-subnets")),
		Description: pulumi.String("Ironflyer Postgres private subnets"),
		SubnetIds:   comp.PrivateSubnetIDs,
		Tags:        env.Tags,
	})
	if err != nil {
		return nil, fmt.Errorf("pg subnet group: %w", err)
	}

	// 3. Cluster parameter group — pinned to the engine major.
	pgFamily := pulumi.String(fmt.Sprintf("aurora-postgresql%s", majorFromEngineVersion(env.PostgresEngineVersion)))
	pgParams, err := rds.NewClusterParameterGroup(ctx, env.Name("pg-cluster-params"), &rds.ClusterParameterGroupArgs{
		Name:        pulumi.String(env.Name("pg-cluster-params")),
		Family:      pgFamily,
		Description: pulumi.String("Ironflyer Aurora Postgres cluster params"),
		Parameters: rds.ClusterParameterGroupParameterArray{
			&rds.ClusterParameterGroupParameterArgs{
				Name:        pulumi.String("rds.logical_replication"),
				Value:       pulumi.String("1"),
				ApplyMethod: pulumi.String("pending-reboot"),
			},
			&rds.ClusterParameterGroupParameterArgs{
				Name:  pulumi.String("log_min_duration_statement"),
				Value: pulumi.String("500"),
			},
		},
		Tags: env.Tags,
	})
	if err != nil {
		return nil, fmt.Errorf("pg cluster params: %w", err)
	}

	// 4. Instance parameter group (statement_timeout + max_connections live
	// at the instance scope on Aurora Postgres).
	instParams, err := rds.NewParameterGroup(ctx, env.Name("pg-instance-params"), &rds.ParameterGroupArgs{
		Name:        pulumi.String(env.Name("pg-instance-params")),
		Family:      pgFamily,
		Description: pulumi.String("Ironflyer Aurora Postgres instance params"),
		Parameters: rds.ParameterGroupParameterArray{
			&rds.ParameterGroupParameterArgs{
				Name:  pulumi.String("max_connections"),
				Value: pulumi.String("400"),
			},
			&rds.ParameterGroupParameterArgs{
				Name:  pulumi.String("statement_timeout"),
				Value: pulumi.String("30000"),
			},
			&rds.ParameterGroupParameterArgs{
				Name:  pulumi.String("idle_in_transaction_session_timeout"),
				Value: pulumi.String("60000"),
			},
		},
		Tags: env.Tags,
	})
	if err != nil {
		return nil, fmt.Errorf("pg instance params: %w", err)
	}

	// 5. Master password — random + stored in Secrets Manager.
	masterPass, err := random.NewRandomPassword(ctx, env.Name("pg-master-pass"), &random.RandomPasswordArgs{
		Length:     pulumi.Int(32),
		Special:    pulumi.Bool(true),
		OverrideSpecial: pulumi.String("_-"),
	})
	if err != nil {
		return nil, fmt.Errorf("pg random pass: %w", err)
	}

	// 6. Cluster. Serverless v2 vs. provisioned diverges on
	// `EngineMode` + `ServerlessV2ScalingConfiguration` vs. instance
	// classes on the instances themselves.
	clusterArgs := &rds.ClusterArgs{
		ClusterIdentifier:           pulumi.String(env.Name("pg")),
		Engine:                      pulumi.String("aurora-postgresql"),
		EngineVersion:               pulumi.String(env.PostgresEngineVersion),
		DatabaseName:                pulumi.String(env.DBName),
		MasterUsername:              pulumi.String(env.DBUser),
		MasterPassword:              masterPass.Result,
		DbSubnetGroupName:           subnetGroup.Name,
		VpcSecurityGroupIds:         pulumi.StringArray{sg.ID().ToStringOutput()},
		StorageEncrypted:            pulumi.Bool(true),
		KmsKeyId:                    keys.RDS.Arn,
		BackupRetentionPeriod:       pulumi.Int(env.BackupRetentionDays),
		PreferredBackupWindow:       pulumi.String(env.BackupWindow),
		PreferredMaintenanceWindow:  pulumi.String(env.MaintenanceWindow),
		DbClusterParameterGroupName: pgParams.Name,
		CopyTagsToSnapshot:          pulumi.Bool(true),
		DeletionProtection:          pulumi.Bool(env.IsProd),
		SkipFinalSnapshot:           pulumi.Bool(!env.IsProd),
		FinalSnapshotIdentifier:     pulumi.String(env.Name("pg-final")),
		EnabledCloudwatchLogsExports: pulumi.StringArray{
			pulumi.String("postgresql"),
		},
		IamDatabaseAuthenticationEnabled: pulumi.Bool(true),
		Tags:                             env.Tags,
	}
	if !env.PostgresProvisioned {
		clusterArgs.EngineMode = pulumi.String("provisioned")
		clusterArgs.Serverlessv2ScalingConfiguration = &rds.ClusterServerlessv2ScalingConfigurationArgs{
			MinCapacity: pulumi.Float64(env.PostgresMinACU),
			MaxCapacity: pulumi.Float64(env.PostgresMaxACU),
		}
	}
	cluster, err := rds.NewCluster(ctx, env.Name("pg"), clusterArgs)
	if err != nil {
		return nil, fmt.Errorf("pg cluster: %w", err)
	}

	// 7. Instances — 1 writer (+N readers).
	instanceClass := env.PostgresInstanceClass
	if !env.PostgresProvisioned {
		instanceClass = "db.serverless"
	}
	writer, err := rds.NewClusterInstance(ctx, env.Name("pg-writer"), &rds.ClusterInstanceArgs{
		Identifier:           pulumi.String(env.Name("pg-writer")),
		ClusterIdentifier:    cluster.ID(),
		InstanceClass:        pulumi.String(instanceClass),
		Engine:               pulumi.String("aurora-postgresql"),
		EngineVersion:        cluster.EngineVersion,
		DbParameterGroupName: instParams.Name,
		PubliclyAccessible:   pulumi.Bool(false),
		Tags:                 env.Tags,
	})
	if err != nil {
		return nil, fmt.Errorf("pg writer: %w", err)
	}
	_ = writer
	for i := 0; i < env.PostgresReaders; i++ {
		name := env.Name(fmt.Sprintf("pg-reader-%d", i))
		_, err := rds.NewClusterInstance(ctx, name, &rds.ClusterInstanceArgs{
			Identifier:           pulumi.String(name),
			ClusterIdentifier:    cluster.ID(),
			InstanceClass:        pulumi.String(instanceClass),
			Engine:               pulumi.String("aurora-postgresql"),
			EngineVersion:        cluster.EngineVersion,
			DbParameterGroupName: instParams.Name,
			PubliclyAccessible:   pulumi.Bool(false),
			Tags:                 env.Tags,
		})
		if err != nil {
			return nil, fmt.Errorf("pg reader %d: %w", i, err)
		}
	}

	// 8. Push master credentials into Secrets Manager. Shape matches the
	// Helm chart's `existingSecret` (postgres-password + POSTGRES_URL +
	// POSTGRES_USER + POSTGRES_PASSWORD + POSTGRES_DB).
	secretJSON := pulumi.All(cluster.Endpoint, cluster.ReaderEndpoint, cluster.Port, masterPass.Result).ApplyT(
		func(args []interface{}) (string, error) {
			writer := args[0].(string)
			port := args[2].(int)
			pass := args[3].(string)
			url := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=require", env.DBUser, pass, writer, port, env.DBName)
			doc := map[string]string{
				"postgres-password": pass,
				"POSTGRES_URL":      url,
				"POSTGRES_HOST":     writer,
				"POSTGRES_PORT":     fmt.Sprintf("%d", port),
				"POSTGRES_USER":     env.DBUser,
				"POSTGRES_PASSWORD": pass,
				"POSTGRES_DB":       env.DBName,
			}
			b, err := jsonMarshal(doc)
			return string(b), err
		},
	).(pulumi.StringOutput)

	_, err = secretsmanager.NewSecretVersion(ctx, env.Name("pg-secret-version"), &secretsmanager.SecretVersionArgs{
		SecretId:     secrets.PostgresMaster.ID(),
		SecretString: secretJSON,
	}, pulumi.DependsOn([]pulumi.Resource{cluster}))
	if err != nil {
		return nil, fmt.Errorf("pg secret version: %w", err)
	}

	connURL := pulumi.All(cluster.Endpoint, cluster.Port, masterPass.Result).ApplyT(
		func(args []interface{}) string {
			writer := args[0].(string)
			port := args[1].(int)
			pass := args[2].(string)
			return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=require", env.DBUser, pass, writer, port, env.DBName)
		},
	).(pulumi.StringOutput)

	return &Postgres{
		Cluster:        cluster,
		WriterEndpoint: cluster.Endpoint,
		ReaderEndpoint: cluster.ReaderEndpoint,
		Port:           cluster.Port,
		ConnectionURL:  connURL,
		SecurityGroup:  sg,
		ParameterGroup: pgParams,
	}, nil
}

// majorFromEngineVersion turns "16.4" -> "16", "15.6" -> "15".
func majorFromEngineVersion(v string) string {
	for i := 0; i < len(v); i++ {
		if v[i] == '.' {
			return v[:i]
		}
	}
	return v
}
