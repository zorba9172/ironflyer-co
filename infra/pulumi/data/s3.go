package data

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Buckets is every S3 bucket the data layer owns. The compute / edge
// layer's CloudFront distribution binds itself to WebSpa via OAC; the
// orchestrator writes backups + workspace snapshots; SurrealDB dumps to
// SurrealExport on a Kubernetes CronJob.
type Buckets struct {
	Backup        *s3.BucketV2
	Workspaces    *s3.BucketV2
	WebSpa        *s3.BucketV2
	CFLogs        *s3.BucketV2
	SurrealExport *s3.BucketV2
}

func provisionS3(ctx *pulumi.Context, env *stackEnv, k *KMSKeys) (*Buckets, error) {
	b := &Buckets{}

	var err error
	b.Backup, err = provisionBackupBucket(ctx, env, k,
		fmt.Sprintf("ironflyer-backup-%s-%s", env.stack, env.region),
		"backup",
		true /* object lock */, 90, 30, 365)
	if err != nil {
		return nil, err
	}

	b.Workspaces, err = provisionWorkspacesBucket(ctx, env, k,
		fmt.Sprintf("ironflyer-workspaces-%s-%s", env.stack, env.region))
	if err != nil {
		return nil, err
	}

	b.WebSpa, err = provisionWebSpaBucket(ctx, env,
		fmt.Sprintf("ironflyer-web-%s", env.stack))
	if err != nil {
		return nil, err
	}

	b.CFLogs, err = provisionLogsBucket(ctx, env,
		fmt.Sprintf("ironflyer-cf-logs-%s", env.stack))
	if err != nil {
		return nil, err
	}

	b.SurrealExport, err = provisionBackupBucket(ctx, env, k,
		fmt.Sprintf("ironflyer-surreal-export-%s", env.stack),
		"surreal-export",
		true, 90, 30, 365)
	if err != nil {
		return nil, err
	}

	return b, nil
}

// provisionBackupBucket builds a KMS-encrypted, versioned, optionally
// object-locked bucket with Glacier-after-30d / expire-at-365d lifecycle.
func provisionBackupBucket(ctx *pulumi.Context, env *stackEnv, k *KMSKeys, bucketName, logical string, objectLock bool, retentionDays, glacierDays, expirationDays int) (*s3.BucketV2, error) {
	args := &s3.BucketV2Args{
		Bucket:        pulumi.String(bucketName),
		ForceDestroy:  pulumi.Bool(!env.isProd),
		Tags:          env.tags,
	}
	if objectLock && env.isProd {
		args.ObjectLockEnabled = pulumi.Bool(true)
	}

	bucket, err := s3.NewBucketV2(ctx, name(env, "bucket-"+logical), args)
	if err != nil {
		return nil, err
	}

	if _, err := s3.NewBucketVersioningV2(ctx, name(env, "bucket-"+logical+"-ver"), &s3.BucketVersioningV2Args{
		Bucket: bucket.ID(),
		VersioningConfiguration: &s3.BucketVersioningV2VersioningConfigurationArgs{
			Status: pulumi.String("Enabled"),
		},
	}); err != nil {
		return nil, err
	}

	if _, err := s3.NewBucketServerSideEncryptionConfigurationV2(ctx, name(env, "bucket-"+logical+"-sse"), &s3.BucketServerSideEncryptionConfigurationV2Args{
		Bucket: bucket.ID(),
		Rules: s3.BucketServerSideEncryptionConfigurationV2RuleArray{
			&s3.BucketServerSideEncryptionConfigurationV2RuleArgs{
				ApplyServerSideEncryptionByDefault: &s3.BucketServerSideEncryptionConfigurationV2RuleApplyServerSideEncryptionByDefaultArgs{
					SseAlgorithm:   pulumi.String("aws:kms"),
					KmsMasterKeyId: k.S3Key.Arn,
				},
				BucketKeyEnabled: pulumi.Bool(true),
			},
		},
	}); err != nil {
		return nil, err
	}

	if _, err := s3.NewBucketPublicAccessBlock(ctx, name(env, "bucket-"+logical+"-pab"), &s3.BucketPublicAccessBlockArgs{
		Bucket:                bucket.ID(),
		BlockPublicAcls:       pulumi.Bool(true),
		BlockPublicPolicy:     pulumi.Bool(true),
		IgnorePublicAcls:      pulumi.Bool(true),
		RestrictPublicBuckets: pulumi.Bool(true),
	}); err != nil {
		return nil, err
	}

	if _, err := s3.NewBucketLifecycleConfigurationV2(ctx, name(env, "bucket-"+logical+"-lifecycle"), &s3.BucketLifecycleConfigurationV2Args{
		Bucket: bucket.ID(),
		Rules: s3.BucketLifecycleConfigurationV2RuleArray{
			&s3.BucketLifecycleConfigurationV2RuleArgs{
				Id:     pulumi.String("archive-then-expire"),
				Status: pulumi.String("Enabled"),
				Filter: &s3.BucketLifecycleConfigurationV2RuleFilterArgs{Prefix: pulumi.String("")},
				Transitions: s3.BucketLifecycleConfigurationV2RuleTransitionArray{
					&s3.BucketLifecycleConfigurationV2RuleTransitionArgs{
						Days:         pulumi.Int(glacierDays),
						StorageClass: pulumi.String("GLACIER"),
					},
				},
				Expiration: &s3.BucketLifecycleConfigurationV2RuleExpirationArgs{
					Days: pulumi.Int(expirationDays),
				},
				AbortIncompleteMultipartUpload: &s3.BucketLifecycleConfigurationV2RuleAbortIncompleteMultipartUploadArgs{
					DaysAfterInitiation: pulumi.Int(1),
				},
			},
		},
	}); err != nil {
		return nil, err
	}

	if objectLock && env.isProd {
		if _, err := s3.NewBucketObjectLockConfigurationV2(ctx, name(env, "bucket-"+logical+"-lock"), &s3.BucketObjectLockConfigurationV2Args{
			Bucket: bucket.ID(),
			Rule: &s3.BucketObjectLockConfigurationV2RuleArgs{
				DefaultRetention: &s3.BucketObjectLockConfigurationV2RuleDefaultRetentionArgs{
					Mode: pulumi.String("COMPLIANCE"),
					Days: pulumi.Int(retentionDays),
				},
			},
		}); err != nil {
			return nil, err
		}
	}

	return bucket, nil
}

// provisionWorkspacesBucket holds per-user durable workspace snapshots
// (the runtime portability story). Versioned + encrypted, but no Object
// Lock — the runtime needs to be able to expire stale checkpoints.
func provisionWorkspacesBucket(ctx *pulumi.Context, env *stackEnv, k *KMSKeys, bucketName string) (*s3.BucketV2, error) {
	bucket, err := s3.NewBucketV2(ctx, name(env, "bucket-workspaces"), &s3.BucketV2Args{
		Bucket:       pulumi.String(bucketName),
		ForceDestroy: pulumi.Bool(!env.isProd),
		Tags:         env.tags,
	})
	if err != nil {
		return nil, err
	}

	if _, err := s3.NewBucketVersioningV2(ctx, name(env, "bucket-workspaces-ver"), &s3.BucketVersioningV2Args{
		Bucket: bucket.ID(),
		VersioningConfiguration: &s3.BucketVersioningV2VersioningConfigurationArgs{Status: pulumi.String("Enabled")},
	}); err != nil {
		return nil, err
	}

	if _, err := s3.NewBucketServerSideEncryptionConfigurationV2(ctx, name(env, "bucket-workspaces-sse"), &s3.BucketServerSideEncryptionConfigurationV2Args{
		Bucket: bucket.ID(),
		Rules: s3.BucketServerSideEncryptionConfigurationV2RuleArray{
			&s3.BucketServerSideEncryptionConfigurationV2RuleArgs{
				ApplyServerSideEncryptionByDefault: &s3.BucketServerSideEncryptionConfigurationV2RuleApplyServerSideEncryptionByDefaultArgs{
					SseAlgorithm:   pulumi.String("aws:kms"),
					KmsMasterKeyId: k.S3Key.Arn,
				},
				BucketKeyEnabled: pulumi.Bool(true),
			},
		},
	}); err != nil {
		return nil, err
	}

	if _, err := s3.NewBucketPublicAccessBlock(ctx, name(env, "bucket-workspaces-pab"), &s3.BucketPublicAccessBlockArgs{
		Bucket:                bucket.ID(),
		BlockPublicAcls:       pulumi.Bool(true),
		BlockPublicPolicy:     pulumi.Bool(true),
		IgnorePublicAcls:      pulumi.Bool(true),
		RestrictPublicBuckets: pulumi.Bool(true),
	}); err != nil {
		return nil, err
	}

	if _, err := s3.NewBucketLifecycleConfigurationV2(ctx, name(env, "bucket-workspaces-lifecycle"), &s3.BucketLifecycleConfigurationV2Args{
		Bucket: bucket.ID(),
		Rules: s3.BucketLifecycleConfigurationV2RuleArray{
			&s3.BucketLifecycleConfigurationV2RuleArgs{
				Id:     pulumi.String("cleanup-multipart"),
				Status: pulumi.String("Enabled"),
				Filter: &s3.BucketLifecycleConfigurationV2RuleFilterArgs{Prefix: pulumi.String("")},
				AbortIncompleteMultipartUpload: &s3.BucketLifecycleConfigurationV2RuleAbortIncompleteMultipartUploadArgs{
					DaysAfterInitiation: pulumi.Int(1),
				},
			},
		},
	}); err != nil {
		return nil, err
	}

	// CORS so the runtime SA (via IRSA) can presign uploads against this
	// bucket from the orchestrator HTTP API. The hosting domain is the
	// stack's rootDomain; * fallback for dev only.
	allowedOrigin := "*"
	if env.dnsZoneRoot != "" {
		allowedOrigin = "https://" + env.dnsZoneRoot
	}
	if _, err := s3.NewBucketCorsConfigurationV2(ctx, name(env, "bucket-workspaces-cors"), &s3.BucketCorsConfigurationV2Args{
		Bucket: bucket.ID(),
		CorsRules: s3.BucketCorsConfigurationV2CorsRuleArray{
			&s3.BucketCorsConfigurationV2CorsRuleArgs{
				AllowedMethods: pulumi.StringArray{pulumi.String("GET"), pulumi.String("PUT"), pulumi.String("POST"), pulumi.String("DELETE")},
				AllowedOrigins: pulumi.StringArray{pulumi.String(allowedOrigin)},
				AllowedHeaders: pulumi.StringArray{pulumi.String("*")},
				ExposeHeaders:  pulumi.StringArray{pulumi.String("ETag")},
				MaxAgeSeconds:  pulumi.Int(3600),
			},
		},
	}); err != nil {
		return nil, err
	}

	return bucket, nil
}

// provisionWebSpaBucket: SPA static bucket, AES256-encrypted so
// CloudFront can read via OAC without crossing into the KMS scope.
func provisionWebSpaBucket(ctx *pulumi.Context, env *stackEnv, bucketName string) (*s3.BucketV2, error) {
	bucket, err := s3.NewBucketV2(ctx, name(env, "bucket-web"), &s3.BucketV2Args{
		Bucket:       pulumi.String(bucketName),
		ForceDestroy: pulumi.Bool(!env.isProd),
		Tags:         env.tags,
	})
	if err != nil {
		return nil, err
	}

	if _, err := s3.NewBucketServerSideEncryptionConfigurationV2(ctx, name(env, "bucket-web-sse"), &s3.BucketServerSideEncryptionConfigurationV2Args{
		Bucket: bucket.ID(),
		Rules: s3.BucketServerSideEncryptionConfigurationV2RuleArray{
			&s3.BucketServerSideEncryptionConfigurationV2RuleArgs{
				ApplyServerSideEncryptionByDefault: &s3.BucketServerSideEncryptionConfigurationV2RuleApplyServerSideEncryptionByDefaultArgs{
					SseAlgorithm: pulumi.String("AES256"),
				},
			},
		},
	}); err != nil {
		return nil, err
	}

	if _, err := s3.NewBucketPublicAccessBlock(ctx, name(env, "bucket-web-pab"), &s3.BucketPublicAccessBlockArgs{
		Bucket:                bucket.ID(),
		BlockPublicAcls:       pulumi.Bool(true),
		BlockPublicPolicy:     pulumi.Bool(true),
		IgnorePublicAcls:      pulumi.Bool(true),
		RestrictPublicBuckets: pulumi.Bool(true),
	}); err != nil {
		return nil, err
	}

	return bucket, nil
}

// provisionLogsBucket: CloudFront access logs, 30-day expiration. Log
// delivery requires bucket ACL "log-delivery-write" prerequisites which
// the compute/edge layer's CloudFront distribution wires in.
func provisionLogsBucket(ctx *pulumi.Context, env *stackEnv, bucketName string) (*s3.BucketV2, error) {
	bucket, err := s3.NewBucketV2(ctx, name(env, "bucket-cflogs"), &s3.BucketV2Args{
		Bucket:       pulumi.String(bucketName),
		ForceDestroy: pulumi.Bool(true),
		Tags:         env.tags,
	})
	if err != nil {
		return nil, err
	}

	if _, err := s3.NewBucketOwnershipControls(ctx, name(env, "bucket-cflogs-ownership"), &s3.BucketOwnershipControlsArgs{
		Bucket: bucket.ID(),
		Rule: &s3.BucketOwnershipControlsRuleArgs{
			ObjectOwnership: pulumi.String("BucketOwnerPreferred"),
		},
	}); err != nil {
		return nil, err
	}

	if _, err := s3.NewBucketServerSideEncryptionConfigurationV2(ctx, name(env, "bucket-cflogs-sse"), &s3.BucketServerSideEncryptionConfigurationV2Args{
		Bucket: bucket.ID(),
		Rules: s3.BucketServerSideEncryptionConfigurationV2RuleArray{
			&s3.BucketServerSideEncryptionConfigurationV2RuleArgs{
				ApplyServerSideEncryptionByDefault: &s3.BucketServerSideEncryptionConfigurationV2RuleApplyServerSideEncryptionByDefaultArgs{
					SseAlgorithm: pulumi.String("AES256"),
				},
			},
		},
	}); err != nil {
		return nil, err
	}

	if _, err := s3.NewBucketLifecycleConfigurationV2(ctx, name(env, "bucket-cflogs-lifecycle"), &s3.BucketLifecycleConfigurationV2Args{
		Bucket: bucket.ID(),
		Rules: s3.BucketLifecycleConfigurationV2RuleArray{
			&s3.BucketLifecycleConfigurationV2RuleArgs{
				Id:     pulumi.String("expire-30d"),
				Status: pulumi.String("Enabled"),
				Filter: &s3.BucketLifecycleConfigurationV2RuleFilterArgs{Prefix: pulumi.String("")},
				Expiration: &s3.BucketLifecycleConfigurationV2RuleExpirationArgs{
					Days: pulumi.Int(30),
				},
			},
		},
	}); err != nil {
		return nil, err
	}

	if _, err := s3.NewBucketPublicAccessBlock(ctx, name(env, "bucket-cflogs-pab"), &s3.BucketPublicAccessBlockArgs{
		Bucket:                bucket.ID(),
		BlockPublicAcls:       pulumi.Bool(true),
		BlockPublicPolicy:     pulumi.Bool(true),
		IgnorePublicAcls:      pulumi.Bool(true),
		RestrictPublicBuckets: pulumi.Bool(true),
	}); err != nil {
		return nil, err
	}

	return bucket, nil
}
