// Package data provisions the Ironflyer data layer: KMS, RDS Aurora Postgres,
// ElastiCache Redis, S3 buckets, Secrets Manager, EFS, SurrealDB (in-cluster),
// CloudWatch observability, and the External Secrets operator wiring.
//
// It is intentionally split from the compute layer (compute/ + edge/): the
// compute agent owns VPC, EKS, IAM/IRSA, Route53, ACM, CloudFront, and WAF;
// this package consumes that side's outputs through the Compute struct and
// provisions the persistent + secret state the orchestrator runs against.
package data

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	kubernetes "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// Compute holds the public surface the compute/ + edge/ sibling packages
// must publish. The compute agent assembles this struct in main.go and
// hands it to data.Provision. If a field here is not yet produced by the
// compute side, the compute agent must add it before this package can be
// wired up end-to-end.
type Compute struct {
	// Networking — matches compute.Network field names + types so the
	// caller in main.go can pass values straight through.
	VpcID            pulumi.IDOutput
	PrivateSubnetIDs pulumi.StringArrayOutput
	PublicSubnetIDs  pulumi.StringArrayOutput
	// DBSubnetGroupName is the name of an RDS DB subnet group the
	// compute layer prepared from the dedicated DB subnets.
	DBSubnetGroupName pulumi.StringOutput
	// ClusterSGID is the EKS-managed cluster security group (workloads
	// run inside it). All "ingress from the cluster" rules in the data
	// layer reference this.
	ClusterSGID pulumi.IDOutput

	// EKS.
	EksClusterName  pulumi.StringOutput
	OidcProviderArn pulumi.StringOutput
	OidcProviderURL pulumi.StringOutput

	// Kubernetes provider already authenticated against the EKS cluster.
	// The compute agent must construct this with the cluster's
	// kubeconfig and pass it in. When nil, the data layer still works
	// for AWS-side resources (RDS, Redis, S3, Secrets, EFS, KMS,
	// CloudWatch) and skips the in-cluster pieces (SurrealDB,
	// kube-prometheus, loki, external-secrets, ConfigMap mirrors). The
	// compute agent owns adding this; documented in report.
	K8sProvider *kubernetes.Provider

	// IRSA role ARN for the orchestrator service account
	// (ironflyer/orchestrator-sa). External Secrets uses this to read
	// AWS Secrets Manager values into in-cluster Secrets.
	OrchestratorRoleArn pulumi.StringOutput
}

// Outputs is the data-layer half of the unified stack outputs. main.go
// merges it with the compute outputs.
type Outputs struct {
	// KMS.
	KmsRdsKeyArn     pulumi.StringOutput
	KmsS3KeyArn      pulumi.StringOutput
	KmsSecretsKeyArn pulumi.StringOutput
	KmsEbsKeyArn     pulumi.StringOutput

	// Postgres.
	PostgresClusterArn      pulumi.StringOutput
	PostgresWriterEndpoint  pulumi.StringOutput
	PostgresReaderEndpoint  pulumi.StringOutput
	PostgresConnectionURL   pulumi.StringOutput
	PostgresMasterSecretArn pulumi.StringOutput

	// Redis.
	RedisPrimaryEndpoint pulumi.StringOutput
	RedisReaderEndpoint  pulumi.StringOutput
	RedisAuthSecretArn   pulumi.StringOutput

	// S3.
	BackupBucketArn         pulumi.StringOutput
	BackupBucketName        pulumi.StringOutput
	WorkspacesBucketArn     pulumi.StringOutput
	WorkspacesBucketName    pulumi.StringOutput
	WebSpaBucketArn         pulumi.StringOutput
	WebSpaBucketName        pulumi.StringOutput
	CloudFrontLogsBucketArn pulumi.StringOutput
	CloudFrontLogsBucket    pulumi.StringOutput
	SurrealExportBucketArn  pulumi.StringOutput
	SurrealExportBucket     pulumi.StringOutput

	// Secrets.
	SecretArns pulumi.StringMapOutput

	// EFS.
	EfsFileSystemID  pulumi.StringOutput
	EfsAccessPointID pulumi.StringOutput
	EfsDnsName       pulumi.StringOutput

	// SurrealDB.
	SurrealService pulumi.StringOutput
}

// stackEnv captures everything Provision needs that depends on the active
// Pulumi stack name / aws:region / infra:* config flags.
type stackEnv struct {
	stack            string // e.g. "dev", "prod-eu"
	region           string
	isProd           bool
	logRetentionDays int
	postgresInstance string
	postgresReaders  int
	redisShards      int
	surrealEnabled   bool
	datadogApiKey    string
	dnsZoneRoot      string
	tags             pulumi.StringMap
}

func resolveStackEnv(ctx *pulumi.Context) (*stackEnv, error) {
	stack := ctx.Stack()
	awsCfg := config.New(ctx, "aws")
	region := awsCfg.Require("region")
	infraCfg := config.New(ctx, "infra")

	isProd := strings.HasPrefix(stack, "prod")

	retention := 30
	switch {
	case isProd:
		retention = 365
	case stack == "staging":
		retention = 90
	}

	postgresInstance := "db.t4g.medium"
	postgresReaders := 1
	redisShards := 1
	if isProd {
		postgresInstance = "db.r6g.large"
		postgresReaders = 2
		redisShards = 2
	} else if stack == "staging" {
		postgresInstance = "db.t4g.large"
		postgresReaders = 1
		redisShards = 1
	}

	surrealEnabled := isProd
	if v, err := infraCfg.TryBool("surrealEnabled"); err == nil {
		surrealEnabled = v
	}

	return &stackEnv{
		stack:            stack,
		region:           region,
		isProd:           isProd,
		logRetentionDays: retention,
		postgresInstance: postgresInstance,
		postgresReaders:  postgresReaders,
		redisShards:      redisShards,
		surrealEnabled:   surrealEnabled,
		datadogApiKey:    infraCfg.Get("datadogApiKey"),
		dnsZoneRoot:      infraCfg.Get("rootDomain"),
		tags: pulumi.StringMap{
			"Project":   pulumi.String("ironflyer"),
			"Stack":     pulumi.String(stack),
			"ManagedBy": pulumi.String("pulumi"),
			"Layer":     pulumi.String("data"),
		},
	}, nil
}

// Provision builds the entire data layer in dependency order. It is the
// one entry point main.go uses for this package.
func Provision(ctx *pulumi.Context, deps Compute) (*Outputs, error) {
	env, err := resolveStackEnv(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve stack env: %w", err)
	}

	kms, err := provisionKMS(ctx, env)
	if err != nil {
		return nil, fmt.Errorf("kms: %w", err)
	}

	dataSG, err := provisionDataSecurityGroup(ctx, env, deps)
	if err != nil {
		return nil, fmt.Errorf("data security group: %w", err)
	}

	secrets, err := provisionSecrets(ctx, env, kms)
	if err != nil {
		return nil, fmt.Errorf("secrets: %w", err)
	}

	pg, err := provisionPostgres(ctx, env, deps, kms, dataSG, secrets)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}

	redis, err := provisionRedis(ctx, env, deps, kms, dataSG, secrets)
	if err != nil {
		return nil, fmt.Errorf("redis: %w", err)
	}

	buckets, err := provisionS3(ctx, env, kms)
	if err != nil {
		return nil, fmt.Errorf("s3: %w", err)
	}

	efs, err := provisionEFS(ctx, env, deps, kms, dataSG)
	if err != nil {
		return nil, fmt.Errorf("efs: %w", err)
	}

	surreal, err := provisionSurreal(ctx, env, deps, secrets)
	if err != nil {
		return nil, fmt.Errorf("surrealdb: %w", err)
	}

	if err := provisionObservability(ctx, env, deps); err != nil {
		return nil, fmt.Errorf("observability: %w", err)
	}

	if err := provisionConsumers(ctx, env, deps, secrets); err != nil {
		return nil, fmt.Errorf("external-secrets: %w", err)
	}

	return &Outputs{
		KmsRdsKeyArn:            kms.RDSKey.Arn.ToStringOutput(),
		KmsS3KeyArn:             kms.S3Key.Arn.ToStringOutput(),
		KmsSecretsKeyArn:        kms.SecretsKey.Arn.ToStringOutput(),
		KmsEbsKeyArn:            kms.EBSKey.Arn.ToStringOutput(),
		PostgresClusterArn:      pg.ClusterArn,
		PostgresWriterEndpoint:  pg.WriterEndpoint,
		PostgresReaderEndpoint:  pg.ReaderEndpoint,
		PostgresConnectionURL:   pg.ConnectionURL,
		PostgresMasterSecretArn: secrets.PostgresMaster.Arn.ToStringOutput(),
		RedisPrimaryEndpoint:    redis.PrimaryEndpoint,
		RedisReaderEndpoint:     redis.ReaderEndpoint,
		RedisAuthSecretArn:      secrets.RedisAuth.Arn.ToStringOutput(),
		BackupBucketArn:         buckets.Backup.Arn.ToStringOutput(),
		BackupBucketName:        buckets.Backup.Bucket.ToStringOutput(),
		WorkspacesBucketArn:     buckets.Workspaces.Arn.ToStringOutput(),
		WorkspacesBucketName:    buckets.Workspaces.Bucket.ToStringOutput(),
		WebSpaBucketArn:         buckets.WebSpa.Arn.ToStringOutput(),
		WebSpaBucketName:        buckets.WebSpa.Bucket.ToStringOutput(),
		CloudFrontLogsBucketArn: buckets.CFLogs.Arn.ToStringOutput(),
		CloudFrontLogsBucket:    buckets.CFLogs.Bucket.ToStringOutput(),
		SurrealExportBucketArn:  buckets.SurrealExport.Arn.ToStringOutput(),
		SurrealExportBucket:     buckets.SurrealExport.Bucket.ToStringOutput(),
		SecretArns:              secrets.ArnsByLogicalName(),
		EfsFileSystemID:         efs.FileSystemID,
		EfsAccessPointID:        efs.AccessPointID,
		EfsDnsName:              efs.DnsName,
		SurrealService:          surreal.Service,
	}, nil
}

// provisionDataSecurityGroup builds the shared security group that fronts
// stateful services (RDS, Redis, EFS). Ingress rules are layered per
// service; egress is wide open since this SG is inside private subnets.
func provisionDataSecurityGroup(ctx *pulumi.Context, env *stackEnv, deps Compute) (*ec2.SecurityGroup, error) {
	sg, err := ec2.NewSecurityGroup(ctx, name(env, "data-sg"), &ec2.SecurityGroupArgs{
		Name:        pulumi.String(name(env, "data-sg")),
		Description: pulumi.String("Ironflyer data layer (RDS+Redis+EFS) ingress from the EKS cluster SG"),
		VpcId:       deps.VpcID,
		Egress: ec2.SecurityGroupEgressArray{
			&ec2.SecurityGroupEgressArgs{
				Protocol:   pulumi.String("-1"),
				FromPort:   pulumi.Int(0),
				ToPort:     pulumi.Int(0),
				CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
			},
		},
		Tags: env.tags,
	})
	if err != nil {
		return nil, err
	}
	return sg, nil
}

// name builds a deterministic resource name "ironflyer-<stack>-<suffix>".
func name(env *stackEnv, suffix string) string {
	return fmt.Sprintf("ironflyer-%s-%s", env.stack, suffix)
}
