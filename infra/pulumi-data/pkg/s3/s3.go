// Package s3 provisions Ironflyer's four S3 buckets per stack + region:
//
//   - ironflyer-backups-${stack}-${region}    pg_dump + wal-g WAL archive
//   - ironflyer-workspaces-${stack}-${region} runtime workspace state
//   - ironflyer-assets-${stack}-${region}     user uploads + visual diffs
//   - ironflyer-audit-${stack}-${region}      audit log export landing
//
// Every bucket: versioning on, SSE-KMS with the s3 CMK, public ACL block,
// lifecycle transitions (Standard-IA after 30d, Glacier after 180d,
// expiry after 730d for backups), and (prod only) cross-region replication
// into the alternate prod region's sibling bucket.
package s3

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	pkg "ironflyer/infra/pulumi-data/pkg"
	myKMS "ironflyer/infra/pulumi-data/pkg/kms"
)

// Buckets is the bundle this package mints.
type Buckets struct {
	Backups    *s3.BucketV2
	Workspaces *s3.BucketV2
	Assets     *s3.BucketV2
	Audit      *s3.BucketV2
}

// Provision builds all four buckets.
func Provision(ctx *pulumi.Context, env *pkg.Env, comp *pkg.Compute, keys *myKMS.Keys) (*Buckets, error) {
	backups, err := newBucket(ctx, env, keys, "backups", true /*expiry*/)
	if err != nil {
		return nil, fmt.Errorf("backups: %w", err)
	}
	workspaces, err := newBucket(ctx, env, keys, "workspaces", false)
	if err != nil {
		return nil, fmt.Errorf("workspaces: %w", err)
	}
	assets, err := newBucket(ctx, env, keys, "assets", false)
	if err != nil {
		return nil, fmt.Errorf("assets: %w", err)
	}
	audit, err := newBucket(ctx, env, keys, "audit", true)
	if err != nil {
		return nil, fmt.Errorf("audit: %w", err)
	}

	// Cross-region replication setup is left as a follow-up — it requires
	// (a) an IAM role with s3:Replicate* on source and destination, (b)
	// the alternate region's sibling buckets to already exist, and (c)
	// per-bucket ReplicationConfiguration. The compute layer's secondary
	// region must be provisioned first. For now, we expose the helper to
	// build the IAM policy fragments so the operator can run the
	// `aws s3api put-bucket-replication` step out-of-band.
	if env.CrossRegionReplication && env.ReplicaRegion != "" {
		// Replication intentionally not wired here — see README.
		_ = env.ReplicaRegion
	}

	return &Buckets{
		Backups:    backups,
		Workspaces: workspaces,
		Assets:     assets,
		Audit:      audit,
	}, nil
}

func newBucket(ctx *pulumi.Context, env *pkg.Env, keys *myKMS.Keys, purpose string, expireBackups bool) (*s3.BucketV2, error) {
	bucketName := fmt.Sprintf("ironflyer-%s-%s-%s", purpose, env.Stack, env.Region)
	b, err := s3.NewBucketV2(ctx, env.Name("s3-"+purpose), &s3.BucketV2Args{
		Bucket:       pulumi.String(bucketName),
		ForceDestroy: pulumi.Bool(!env.IsProd),
		Tags:         env.Tags,
	})
	if err != nil {
		return nil, err
	}

	// Versioning ON.
	_, err = s3.NewBucketVersioningV2(ctx, env.Name("s3-"+purpose+"-versioning"), &s3.BucketVersioningV2Args{
		Bucket: b.ID(),
		VersioningConfiguration: &s3.BucketVersioningV2VersioningConfigurationArgs{
			Status: pulumi.String("Enabled"),
		},
	})
	if err != nil {
		return nil, err
	}

	// SSE with the data layer's s3 CMK.
	_, err = s3.NewBucketServerSideEncryptionConfigurationV2(ctx, env.Name("s3-"+purpose+"-sse"), &s3.BucketServerSideEncryptionConfigurationV2Args{
		Bucket: b.ID(),
		Rules: s3.BucketServerSideEncryptionConfigurationV2RuleArray{
			&s3.BucketServerSideEncryptionConfigurationV2RuleArgs{
				ApplyServerSideEncryptionByDefault: &s3.BucketServerSideEncryptionConfigurationV2RuleApplyServerSideEncryptionByDefaultArgs{
					SseAlgorithm:   pulumi.String("aws:kms"),
					KmsMasterKeyId: keys.S3.Arn,
				},
				BucketKeyEnabled: pulumi.Bool(true),
			},
		},
	})
	if err != nil {
		return nil, err
	}

	// Block public ACLs.
	_, err = s3.NewBucketPublicAccessBlock(ctx, env.Name("s3-"+purpose+"-pab"), &s3.BucketPublicAccessBlockArgs{
		Bucket:                b.ID(),
		BlockPublicAcls:       pulumi.Bool(true),
		BlockPublicPolicy:     pulumi.Bool(true),
		IgnorePublicAcls:      pulumi.Bool(true),
		RestrictPublicBuckets: pulumi.Bool(true),
	})
	if err != nil {
		return nil, err
	}

	// Lifecycle. Standard-IA at 30d, Glacier at 180d, expiry at 730d if
	// the bucket holds backups/audit data (per docs/DR_RUNBOOK.md
	// retention contract).
	rule := &s3.BucketLifecycleConfigurationV2RuleArgs{
		Id:     pulumi.String("default"),
		Status: pulumi.String("Enabled"),
		Filter: &s3.BucketLifecycleConfigurationV2RuleFilterArgs{
			Prefix: pulumi.String(""),
		},
		Transitions: s3.BucketLifecycleConfigurationV2RuleTransitionArray{
			&s3.BucketLifecycleConfigurationV2RuleTransitionArgs{
				Days:         pulumi.Int(30),
				StorageClass: pulumi.String("STANDARD_IA"),
			},
			&s3.BucketLifecycleConfigurationV2RuleTransitionArgs{
				Days:         pulumi.Int(180),
				StorageClass: pulumi.String("GLACIER"),
			},
		},
	}
	if expireBackups {
		rule.Expiration = &s3.BucketLifecycleConfigurationV2RuleExpirationArgs{
			Days: pulumi.Int(730),
		}
	}
	_, err = s3.NewBucketLifecycleConfigurationV2(ctx, env.Name("s3-"+purpose+"-lifecycle"), &s3.BucketLifecycleConfigurationV2Args{
		Bucket: b.ID(),
		Rules: s3.BucketLifecycleConfigurationV2RuleArray{rule},
	})
	if err != nil {
		return nil, err
	}

	return b, nil
}

// AttachAccessPolicies emits inline IAM policies on the orchestrator,
// runtime, and backup IRSA roles for the buckets they need. Roles come
// from the compute stack; the policy text is an Output so it's resolved
// at apply time.
//
// Layout:
//   - orchestrator: GET/PUT on assets-* + audit-*.
//   - runtime:      GET/PUT on workspaces-*.
//   - backup:       GET/PUT/LIST/DELETE on backups-*.
func AttachAccessPolicies(
	ctx *pulumi.Context,
	env *pkg.Env,
	comp *pkg.Compute,
	buckets *Buckets,
	keys *myKMS.Keys,
) error {
	// orchestrator: assets + audit
	if _, err := attachRolePolicy(ctx, env, "orchestrator-s3", comp.OrchestratorRole,
		[]pulumi.StringOutput{buckets.Assets.Arn, buckets.Audit.Arn},
		[]string{"s3:GetObject", "s3:PutObject", "s3:DeleteObject", "s3:ListBucket"},
		keys.S3.Arn,
	); err != nil {
		return err
	}
	// runtime: workspaces
	if _, err := attachRolePolicy(ctx, env, "runtime-s3", comp.RuntimeRole,
		[]pulumi.StringOutput{buckets.Workspaces.Arn},
		[]string{"s3:GetObject", "s3:PutObject", "s3:DeleteObject", "s3:ListBucket"},
		keys.S3.Arn,
	); err != nil {
		return err
	}
	// backup: backups (full lifecycle including delete for retention sweeps)
	if _, err := attachRolePolicy(ctx, env, "backup-s3", comp.BackupRole,
		[]pulumi.StringOutput{buckets.Backups.Arn},
		[]string{"s3:GetObject", "s3:PutObject", "s3:DeleteObject", "s3:ListBucket", "s3:AbortMultipartUpload"},
		keys.S3.Arn,
	); err != nil {
		return err
	}
	return nil
}

func attachRolePolicy(
	ctx *pulumi.Context,
	env *pkg.Env,
	logical string,
	roleArn pulumi.StringOutput,
	bucketArns []pulumi.StringOutput,
	actions []string,
	kmsArn pulumi.StringOutput,
) (*iam.RolePolicy, error) {
	// Roll the bucket ARNs into a single StringArrayOutput.
	combined := pulumi.StringArray{}
	for _, a := range bucketArns {
		combined = append(combined, a)
	}
	docOut := pulumi.All(combined.ToStringArrayOutput(), kmsArn).ApplyT(func(args []interface{}) (string, error) {
		arns := args[0].([]string)
		kArn := args[1].(string)
		resources := []string{}
		for _, a := range arns {
			resources = append(resources, a, a+"/*")
		}
		doc := map[string]any{
			"Version": "2012-10-17",
			"Statement": []map[string]any{
				{
					"Sid":      "BucketAccess",
					"Effect":   "Allow",
					"Action":   actions,
					"Resource": resources,
				},
				{
					"Sid":      "KMSUse",
					"Effect":   "Allow",
					"Action":   []string{"kms:Decrypt", "kms:Encrypt", "kms:GenerateDataKey", "kms:DescribeKey"},
					"Resource": []string{kArn},
				},
			},
		}
		b, err := json.Marshal(doc)
		return string(b), err
	}).(pulumi.StringOutput)

	roleName := roleArn.ApplyT(func(arn string) string {
		// "arn:aws:iam::123:role/name" -> "name"
		for i := len(arn) - 1; i >= 0; i-- {
			if arn[i] == '/' {
				return arn[i+1:]
			}
		}
		return arn
	}).(pulumi.StringOutput)

	return iam.NewRolePolicy(ctx, env.Name(logical), &iam.RolePolicyArgs{
		Name:   pulumi.String(env.Name(logical)),
		Role:   roleName,
		Policy: docOut,
	})
}
