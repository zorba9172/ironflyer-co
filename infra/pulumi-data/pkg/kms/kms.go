// Package kms provisions one customer-managed CMK per data-layer domain.
//
// Domains: rds, redis, s3, secrets. Each key gets an alias
// "alias/ironflyer/<domain>/<stack>". Key policy grants the account root
// admin (the AWS-recommended baseline so the key never becomes
// unrecoverable) plus encrypt/decrypt/generate-data-key to the
// orchestrator + runtime IRSA roles read from the compute stack.
package kms

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/kms"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	pkg "ironflyer/infra/pulumi-data/pkg"
)

// Keys is the bundle this package mints.
type Keys struct {
	RDS     *kms.Key
	Redis   *kms.Key
	S3      *kms.Key
	Secrets *kms.Key
}

// Provision mints the four CMKs.
func Provision(ctx *pulumi.Context, env *pkg.Env, comp *pkg.Compute) (*Keys, error) {
	caller, err := aws.GetCallerIdentity(ctx, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("get caller identity: %w", err)
	}

	policy := keyPolicy(caller.AccountId, comp)

	make := func(domain, desc string) (*kms.Key, error) {
		k, err := kms.NewKey(ctx, env.Name("kms-"+domain), &kms.KeyArgs{
			Description:          pulumi.String(desc),
			DeletionWindowInDays: pulumi.Int(env.KMSDeletionWindowDays),
			EnableKeyRotation:    pulumi.Bool(true),
			Policy:               policy,
			Tags:                 env.Tags,
		})
		if err != nil {
			return nil, err
		}
		_, err = kms.NewAlias(ctx, env.Name("kms-"+domain+"-alias"), &kms.AliasArgs{
			Name:        pulumi.String(env.Alias(domain)),
			TargetKeyId: k.KeyId,
		})
		if err != nil {
			return nil, err
		}
		return k, nil
	}

	rds, err := make("rds", "Ironflyer RDS Aurora Postgres CMK")
	if err != nil {
		return nil, err
	}
	redis, err := make("redis", "Ironflyer ElastiCache Redis CMK")
	if err != nil {
		return nil, err
	}
	s3, err := make("s3", "Ironflyer S3 buckets CMK")
	if err != nil {
		return nil, err
	}
	secrets, err := make("secrets", "Ironflyer Secrets Manager CMK")
	if err != nil {
		return nil, err
	}
	return &Keys{RDS: rds, Redis: redis, S3: s3, Secrets: secrets}, nil
}

// keyPolicy returns a Pulumi StringOutput rendering the IAM key policy.
// Account root gets full admin (AWS baseline) and the orchestrator +
// runtime IRSA roles get encrypt/decrypt/generate-data-key.
func keyPolicy(accountID string, comp *pkg.Compute) pulumi.StringOutput {
	rootArn := fmt.Sprintf("arn:aws:iam::%s:root", accountID)
	return pulumi.All(comp.OrchestratorRole, comp.RuntimeRole, comp.BackupRole).ApplyT(func(args []interface{}) (string, error) {
		orch, _ := args[0].(string)
		runtime, _ := args[1].(string)
		backup, _ := args[2].(string)

		principals := []string{}
		for _, r := range []string{orch, runtime, backup} {
			if r != "" {
				principals = append(principals, r)
			}
		}

		stmts := []map[string]any{
			{
				"Sid":       "RootAccountAdmin",
				"Effect":    "Allow",
				"Principal": map[string]any{"AWS": rootArn},
				"Action":    "kms:*",
				"Resource":  "*",
			},
			{
				"Sid":    "AllowAWSServicesUse",
				"Effect": "Allow",
				"Principal": map[string]any{"Service": []string{
					"rds.amazonaws.com",
					"elasticache.amazonaws.com",
					"s3.amazonaws.com",
					"secretsmanager.amazonaws.com",
					"logs.amazonaws.com",
				}},
				"Action": []string{
					"kms:Encrypt", "kms:Decrypt", "kms:ReEncrypt*",
					"kms:GenerateDataKey*", "kms:CreateGrant", "kms:DescribeKey",
				},
				"Resource": "*",
			},
		}
		if len(principals) > 0 {
			stmts = append(stmts, map[string]any{
				"Sid":       "AllowIRSARolesUse",
				"Effect":    "Allow",
				"Principal": map[string]any{"AWS": principals},
				"Action": []string{
					"kms:Encrypt", "kms:Decrypt", "kms:ReEncrypt*",
					"kms:GenerateDataKey*", "kms:DescribeKey",
				},
				"Resource": "*",
			})
		}
		doc := map[string]any{"Version": "2012-10-17", "Statement": stmts}
		b, err := json.Marshal(doc)
		return string(b), err
	}).(pulumi.StringOutput)
}
