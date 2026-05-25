// Ironflyer compute + edge infrastructure (AWS).
//
// This program is intentionally split across small, focused files so each
// concern (VPC, EKS, IAM, autoscaling, DNS, TLS, CDN, WAF) stays grokkable.
// The data layer (RDS, ElastiCache, S3, KMS, EFS) lives in a sibling Pulumi
// project; this stack publishes the outputs that project needs.
package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"

	"ironflyer/infra/pulumi/compute"
	"ironflyer/infra/pulumi/data"
	"ironflyer/infra/pulumi/edge"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := compute.LoadConfig(ctx)

		// us-east-1 provider — required for CloudFront ACM certificates.
		usEast1, err := aws.NewProvider(ctx, "aws-us-east-1", &aws.ProviderArgs{
			Region: pulumi.String("us-east-1"),
		})
		if err != nil {
			return err
		}

		// --- VPC -----------------------------------------------------------
		net, err := compute.NewVPC(ctx, cfg)
		if err != nil {
			return err
		}

		// --- IAM (roles + IRSA, before EKS so EKS can wire them in) --------
		iam, err := compute.NewIAM(ctx, cfg)
		if err != nil {
			return err
		}

		// --- EKS cluster + node groups + addons + Helm controllers ---------
		cluster, err := compute.NewEKS(ctx, cfg, net, iam)
		if err != nil {
			return err
		}

		// --- Cluster Autoscaler / Karpenter targets ------------------------
		if err := compute.NewAutoscaling(ctx, cfg, cluster, iam); err != nil {
			return err
		}

		// --- Application workload IRSA (orchestrator/runtime/backup) -------
		workload, err := compute.BuildWorkloadIRSA(ctx, cfg, cluster)
		if err != nil {
			return err
		}

		// --- Route53 + ExternalDNS + CertManager IRSA + Helm releases ------
		zone, err := edge.NewDNS(ctx, cfg, cluster)
		if err != nil {
			return err
		}

		// --- ACM (regional + us-east-1 for CloudFront) ---------------------
		certs, err := edge.NewTLS(ctx, cfg, zone, usEast1)
		if err != nil {
			return err
		}

		// --- WAFv2 web ACL (consumed by CloudFront) ------------------------
		acl, err := edge.NewWAF(ctx, cfg, usEast1)
		if err != nil {
			return err
		}

		// --- CloudFront distribution ---------------------------------------
		if err := edge.NewCDN(ctx, cfg, certs, acl); err != nil {
			return err
		}

		// --- Data layer (RDS, Redis, S3, KMS, Secrets, EFS, Surreal) ------
		// The data package consumes the compute outputs through the
		// Compute struct contract documented in data/data.go. If a
		// field below is not yet produced by the compute side, the
		// compute agent must add it (see report).
		dataOut, err := data.Provision(ctx, data.Compute{
			VpcID:             net.VpcID,
			PrivateSubnetIDs:  net.PrivateSubnetIDs,
			PublicSubnetIDs:   net.PublicSubnetIDs,
			DBSubnetGroupName: net.DBSubnetGroupName,
			ClusterSGID:       net.ClusterSGID,
			EksClusterName:    cluster.Name,
			OidcProviderArn:   cluster.OIDCProviderArn,
			OidcProviderURL:   cluster.OIDCProviderURL,
			// K8sProvider intentionally nil: the compute side does not
			// yet expose an authenticated *kubernetes.Provider. When it
			// does, plumb it through here so data layer can install
			// SurrealDB, kube-prometheus-stack, loki-stack, optional
			// Datadog, and the external-secrets operator + mirrors.
			K8sProvider:         nil,
			OrchestratorRoleArn: workload.Orchestrator,
		})
		if err != nil {
			return err
		}
		exportDataOutputs(ctx, dataOut)

		// --- Vercel (dashboard) -------------------------------------------
		// Opt-in per-stack via `ironflyer:vercelEnabled`. When disabled the
		// program builds clean — typical for the `dev` stack where the
		// dashboard is run locally against the orchestrator.
		if cfg.VercelEnabled {
			vercelCfg := config.New(ctx, "vercel")
			sentryDSN := pulumi.StringInput(pulumi.String(cfg.VercelSentryDSN))
			if cfg.VercelSentryDSN == "" {
				// Fall back to the Secrets Manager ARN — the operator
				// rotates the placeholder out-of-band. The string is opaque
				// to Vercel, so an ARN is acceptable as a typed env var
				// value (treated as a Pulumi secret on the resource).
				sentryDSN = dataOut.SecretArns.ApplyT(func(m map[string]string) string {
					return m["sentry-dsn"]
				}).(pulumi.StringOutput)
			}
			vercelOut, err := edge.NewVercel(ctx, cfg, &edge.VercelArgs{
				TeamID:              pulumi.String(cfg.VercelTeamID),
				APIToken:            vercelCfg.RequireSecret("apiToken"),
				Domain:              pulumi.String(cfg.VercelDomain),
				Branch:              pulumi.String(cfg.VercelBranch),
				FrameworkPreset:     pulumi.String(cfg.VercelFramework),
				GitRepoOwner:        pulumi.String(cfg.VercelGitRepoOwner),
				GitRepoName:         pulumi.String(cfg.VercelGitRepoName),
				OrchestratorAPIHost: pulumi.String(cfg.APIHostname()),
				SentryDSN:           sentryDSN,
			})
			if err != nil {
				return err
			}
			if _, err := edge.AddVercelCNAME(ctx, cfg.Stack, &edge.VercelCNAMEArgs{
				Zone:   zone,
				Domain: pulumi.String(cfg.VercelDomain),
			}); err != nil {
				return err
			}
			ctx.Export("vercelProjectId", vercelOut.ProjectID)
			ctx.Export("vercelProductionURL", vercelOut.ProductionURL)
			ctx.Export("vercelPreviewURLPattern", vercelOut.PreviewURLPattern)
		}

		// --- Stack outputs consumed by the data Pulumi project -------------
		ctx.Export("vpcId", net.VpcID)
		ctx.Export("privateSubnetIds", net.PrivateSubnetIDs)
		ctx.Export("publicSubnetIds", net.PublicSubnetIDs)
		ctx.Export("dbSubnetIds", net.DBSubnetIDs)
		ctx.Export("dbSubnetGroupId", net.DBSubnetGroupName)
		ctx.Export("eksClusterName", cluster.Name)
		ctx.Export("eksClusterEndpoint", cluster.Endpoint)
		ctx.Export("oidcProviderArn", cluster.OIDCProviderArn)
		ctx.Export("oidcProviderUrl", cluster.OIDCProviderURL)
		ctx.Export("hostedZoneId", zone.ID())
		ctx.Export("hostedZoneName", zone.Name)
		ctx.Export("certArn", certs.RegionalArn)
		ctx.Export("certArnUsEast1", certs.UsEast1Arn)
		ctx.Export("orchestratorRoleArn", workload.Orchestrator)
		ctx.Export("runtimeRoleArn", workload.Runtime)
		ctx.Export("backupRoleArn", workload.Backup)

		return nil
	})
}

// exportDataOutputs pushes the data-layer outputs onto the stack under a
// `data.*` namespace so they don't collide with the compute/edge exports
// above.
func exportDataOutputs(ctx *pulumi.Context, out *data.Outputs) {
	ctx.Export("data.kms.rdsKeyArn", out.KmsRdsKeyArn)
	ctx.Export("data.kms.s3KeyArn", out.KmsS3KeyArn)
	ctx.Export("data.kms.secretsKeyArn", out.KmsSecretsKeyArn)
	ctx.Export("data.kms.ebsKeyArn", out.KmsEbsKeyArn)

	ctx.Export("data.postgres.clusterArn", out.PostgresClusterArn)
	ctx.Export("data.postgres.writerEndpoint", out.PostgresWriterEndpoint)
	ctx.Export("data.postgres.readerEndpoint", out.PostgresReaderEndpoint)
	ctx.Export("data.postgres.connectionUrl", pulumi.ToSecret(out.PostgresConnectionURL))
	ctx.Export("data.postgres.masterSecretArn", out.PostgresMasterSecretArn)

	ctx.Export("data.redis.primaryEndpoint", out.RedisPrimaryEndpoint)
	ctx.Export("data.redis.readerEndpoint", out.RedisReaderEndpoint)
	ctx.Export("data.redis.authSecretArn", out.RedisAuthSecretArn)

	ctx.Export("data.s3.backup", out.BackupBucketName)
	ctx.Export("data.s3.backupArn", out.BackupBucketArn)
	ctx.Export("data.s3.workspaces", out.WorkspacesBucketName)
	ctx.Export("data.s3.workspacesArn", out.WorkspacesBucketArn)
	ctx.Export("data.s3.web", out.WebSpaBucketName)
	ctx.Export("data.s3.webArn", out.WebSpaBucketArn)
	ctx.Export("data.s3.cflogs", out.CloudFrontLogsBucket)
	ctx.Export("data.s3.cflogsArn", out.CloudFrontLogsBucketArn)
	ctx.Export("data.s3.surrealExport", out.SurrealExportBucket)
	ctx.Export("data.s3.surrealExportArn", out.SurrealExportBucketArn)

	ctx.Export("data.secrets.arns", out.SecretArns)

	ctx.Export("data.efs.fileSystemId", out.EfsFileSystemID)
	ctx.Export("data.efs.accessPointId", out.EfsAccessPointID)
	ctx.Export("data.efs.dnsName", out.EfsDnsName)

	ctx.Export("data.surreal.service", out.SurrealService)
}
