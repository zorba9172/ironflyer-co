// ironflyer-data Pulumi project. Provisions the data layer (KMS, Aurora
// Postgres, ElastiCache Redis, S3 buckets, Secrets Manager, ESO bridge)
// on top of outputs from the compute project (ironflyer-infra).
//
// Dependency contract (see infra/pulumi-data/README.md):
//   1. compute (`ironflyer-infra/<stack>`) UP first.
//   2. data    (`ironflyer-data/<stack>`)  UP second.
//   3. data tear-down BEFORE compute tear-down (so the EKS cluster
//      doesn't disappear while ESO ExternalSecrets are still resolving).
package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	pkg "ironflyer/infra/pulumi-data/pkg"
	"ironflyer/infra/pulumi-data/pkg/k8s"
	"ironflyer/infra/pulumi-data/pkg/kms"
	"ironflyer/infra/pulumi-data/pkg/postgres"
	"ironflyer/infra/pulumi-data/pkg/redis"
	"ironflyer/infra/pulumi-data/pkg/s3"
	"ironflyer/infra/pulumi-data/pkg/secrets"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		env, err := pkg.Resolve(ctx)
		if err != nil {
			return err
		}

		comp, err := pkg.LoadCompute(ctx, env.ComputeStackName)
		if err != nil {
			return err
		}

		keys, err := kms.Provision(ctx, env, comp)
		if err != nil {
			return err
		}

		sec, err := secrets.Provision(ctx, env, keys)
		if err != nil {
			return err
		}

		pg, err := postgres.Provision(ctx, env, comp, keys, sec)
		if err != nil {
			return err
		}

		rd, err := redis.Provision(ctx, env, comp, keys, sec)
		if err != nil {
			return err
		}

		buckets, err := s3.Provision(ctx, env, comp, keys)
		if err != nil {
			return err
		}
		if err := s3.AttachAccessPolicies(ctx, env, comp, buckets, keys); err != nil {
			return err
		}

		bridge, err := k8s.Provision(ctx, env, comp, sec)
		if err != nil {
			return err
		}

		// Exports — referenced by the Helm chart values pipeline + ops
		// runbooks.
		ctx.Export("kms.rdsKeyArn", keys.RDS.Arn)
		ctx.Export("kms.redisKeyArn", keys.Redis.Arn)
		ctx.Export("kms.s3KeyArn", keys.S3.Arn)
		ctx.Export("kms.secretsKeyArn", keys.Secrets.Arn)

		ctx.Export("postgres.writerEndpoint", pg.WriterEndpoint)
		ctx.Export("postgres.readerEndpoint", pg.ReaderEndpoint)
		ctx.Export("postgres.port", pg.Port)
		ctx.Export("postgres.databaseName", pulumi.String(env.DBName))
		ctx.Export("postgres.username", pulumi.String(env.DBUser))
		ctx.Export("postgres.connectionUrl", pulumi.ToSecret(pg.ConnectionURL))
		ctx.Export("postgres.masterSecretArn", sec.PostgresMaster.Arn)

		ctx.Export("redis.primaryEndpoint", rd.PrimaryEndpoint)
		ctx.Export("redis.readerEndpoint", rd.ReaderEndpoint)
		ctx.Export("redis.configurationEndpoint", rd.ConfigEndpoint)
		ctx.Export("redis.port", rd.Port)
		ctx.Export("redis.authSecretArn", sec.RedisAuth.Arn)

		ctx.Export("s3.backupsArn", buckets.Backups.Arn)
		ctx.Export("s3.backupsName", buckets.Backups.Bucket)
		ctx.Export("s3.workspacesArn", buckets.Workspaces.Arn)
		ctx.Export("s3.workspacesName", buckets.Workspaces.Bucket)
		ctx.Export("s3.assetsArn", buckets.Assets.Arn)
		ctx.Export("s3.assetsName", buckets.Assets.Bucket)
		ctx.Export("s3.auditArn", buckets.Audit.Arn)
		ctx.Export("s3.auditName", buckets.Audit.Bucket)

		// One stack output per secret ARN, plus the K8s Secret name the
		// Helm chart will consume.
		secretArns := pulumi.StringMap{}
		k8sNames := pulumi.StringMap{}
		for logical, s := range sec.All {
			secretArns[logical] = s.Arn
			k8sNames[logical] = pulumi.String(k8s.K8sSecretName(logical))
		}
		ctx.Export("secrets.arns", secretArns)
		ctx.Export("k8s.secretNames", k8sNames)

		// If the bridge ran, expose the K8s names map (same content).
		_ = bridge
		return nil
	})
}
