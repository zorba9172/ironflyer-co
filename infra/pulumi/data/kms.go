package data

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/kms"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// KMSKeys is the bundle of customer-managed CMKs the data layer mints.
// Every persistent store + secret uses a CMK (never an AWS-managed key) so
// IAM-based revocation works the same in dev and prod, and the prod DR
// runbook's cross-account key sharing is possible.
type KMSKeys struct {
	RDSKey     *kms.Key
	S3Key      *kms.Key
	SecretsKey *kms.Key
	EBSKey     *kms.Key
}

func provisionKMS(ctx *pulumi.Context, env *stackEnv) (*KMSKeys, error) {
	caller, err := aws.GetCallerIdentity(ctx, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("get caller identity: %w", err)
	}

	policy, err := defaultKeyPolicy(caller.AccountId)
	if err != nil {
		return nil, fmt.Errorf("render key policy: %w", err)
	}

	makeKey := func(logical, alias, desc string) (*kms.Key, error) {
		key, err := kms.NewKey(ctx, name(env, logical), &kms.KeyArgs{
			Description:          pulumi.String(desc),
			DeletionWindowInDays: pulumi.Int(30),
			EnableKeyRotation:    pulumi.Bool(true),
			Policy:               pulumi.String(policy),
			Tags:                 env.tags,
		})
		if err != nil {
			return nil, err
		}
		_, err = kms.NewAlias(ctx, name(env, logical+"-alias"), &kms.AliasArgs{
			Name:        pulumi.String(fmt.Sprintf("alias/ironflyer/%s/%s", env.stack, alias)),
			TargetKeyId: key.KeyId,
		})
		if err != nil {
			return nil, err
		}
		return key, nil
	}

	rdsKey, err := makeKey("kms-rds", "rds", "Ironflyer RDS Aurora Postgres encryption")
	if err != nil {
		return nil, err
	}
	s3Key, err := makeKey("kms-s3", "s3", "Ironflyer S3 buckets encryption")
	if err != nil {
		return nil, err
	}
	secretsKey, err := makeKey("kms-secrets", "secrets", "Ironflyer Secrets Manager encryption")
	if err != nil {
		return nil, err
	}
	ebsKey, err := makeKey("kms-ebs", "ebs", "Ironflyer EBS + EFS encryption")
	if err != nil {
		return nil, err
	}

	return &KMSKeys{
		RDSKey:     rdsKey,
		S3Key:      s3Key,
		SecretsKey: secretsKey,
		EBSKey:     ebsKey,
	}, nil
}

// defaultKeyPolicy grants the account's root principal full control (the
// AWS-recommended baseline) plus the orchestrator IRSA + EKS cluster
// service principals can wrap/unwrap data keys at runtime.
//
// The compute layer is responsible for adding key-grants when it wires
// IRSA roles in; what we set here is the static policy that survives
// stack tear-down + replay.
func defaultKeyPolicy(accountID string) (string, error) {
	doc := map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{
			{
				"Sid":    "EnableRootAccountAdmin",
				"Effect": "Allow",
				"Principal": map[string]any{
					"AWS": fmt.Sprintf("arn:aws:iam::%s:root", accountID),
				},
				"Action":   "kms:*",
				"Resource": "*",
			},
			{
				"Sid":    "AllowAWSServicesEncryption",
				"Effect": "Allow",
				"Principal": map[string]any{
					"Service": []string{
						"rds.amazonaws.com",
						"elasticache.amazonaws.com",
						"s3.amazonaws.com",
						"secretsmanager.amazonaws.com",
						"logs.amazonaws.com",
						"elasticfilesystem.amazonaws.com",
					},
				},
				"Action": []string{
					"kms:Encrypt",
					"kms:Decrypt",
					"kms:ReEncrypt*",
					"kms:GenerateDataKey*",
					"kms:CreateGrant",
					"kms:DescribeKey",
				},
				"Resource": "*",
			},
		},
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
