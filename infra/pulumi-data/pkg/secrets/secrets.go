// Package secrets provisions the AWS Secrets Manager entries the Ironflyer
// workloads consume in-cluster (via the External Secrets Operator bridge
// in pkg/k8s).
//
// Each secret is a JSON blob keyed to match the Helm chart's
// `existingSecret` expectations. Placeholder secrets (Stripe, Anthropic,
// OpenAI, Gemini, HuggingFace, Sentry, GitHub App) are created empty —
// the operator runs `aws secretsmanager put-secret-value` after the
// stack lands with the live credentials.
package secrets

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/secretsmanager"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/kms"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	pkg "ironflyer/infra/pulumi-data/pkg"
	myKMS "ironflyer/infra/pulumi-data/pkg/kms"
)

// Secrets holds every Secrets Manager entry this stack manages.
type Secrets struct {
	PostgresMaster *secretsmanager.Secret
	RedisAuth      *secretsmanager.Secret
	Stripe         *secretsmanager.Secret
	Anthropic      *secretsmanager.Secret
	OpenAI         *secretsmanager.Secret
	Gemini         *secretsmanager.Secret
	HuggingFace    *secretsmanager.Secret
	Sentry         *secretsmanager.Secret
	GithubApp      *secretsmanager.Secret

	// All keeps a deterministic logical-name → Secret map for IAM
	// least-privilege grants downstream.
	All map[string]*secretsmanager.Secret
}

// Provision mints the secret shells. The two infrastructure-owned
// secrets (postgres master + redis auth) are populated by the postgres/
// redis packages once those resources exist.
func Provision(ctx *pulumi.Context, env *pkg.Env, keys *myKMS.Keys) (*Secrets, error) {
	mk := func(logical, suffix string, placeholder map[string]string) (*secretsmanager.Secret, error) {
		s, err := secretsmanager.NewSecret(ctx, env.Name("secret-"+logical), &secretsmanager.SecretArgs{
			Name:                    pulumi.String(fmt.Sprintf("ironflyer/%s/%s", suffix, env.Stack)),
			Description:             pulumi.String(fmt.Sprintf("Ironflyer %s credentials (%s)", suffix, env.Stack)),
			KmsKeyId:                keys.Secrets.Arn,
			RecoveryWindowInDays:    pulumi.Int(7),
			Tags:                    env.Tags,
		})
		if err != nil {
			return nil, err
		}
		if placeholder != nil {
			b, err := json.Marshal(placeholder)
			if err != nil {
				return nil, err
			}
			_, err = secretsmanager.NewSecretVersion(ctx, env.Name("secretver-"+logical), &secretsmanager.SecretVersionArgs{
				SecretId:     s.ID(),
				SecretString: pulumi.String(string(b)),
			})
			if err != nil {
				return nil, err
			}
		}
		return s, nil
	}

	out := &Secrets{All: map[string]*secretsmanager.Secret{}}

	var err error
	if out.PostgresMaster, err = mk("postgres", "postgres", nil); err != nil {
		return nil, err
	}
	out.All["postgres"] = out.PostgresMaster

	if out.RedisAuth, err = mk("redis", "redis", nil); err != nil {
		return nil, err
	}
	out.All["redis"] = out.RedisAuth

	placeholders := map[string]map[string]string{
		"stripe":      {"STRIPE_SECRET_KEY": "", "STRIPE_WEBHOOK_SECRET": ""},
		"anthropic":   {"ANTHROPIC_API_KEY": ""},
		"openai":      {"OPENAI_API_KEY": ""},
		"gemini":      {"GEMINI_API_KEY": ""},
		"huggingface": {"HUGGINGFACE_TOKEN": ""},
		"sentry":      {"SENTRY_DSN": ""},
		"github-app":  {"GITHUB_APP_ID": "", "GITHUB_APP_PRIVATE_KEY": "", "GITHUB_APP_WEBHOOK_SECRET": ""},
	}
	for _, logical := range []string{"stripe", "anthropic", "openai", "gemini", "huggingface", "sentry", "github-app"} {
		s, err := mk(logical, logical, placeholders[logical])
		if err != nil {
			return nil, err
		}
		switch logical {
		case "stripe":
			out.Stripe = s
		case "anthropic":
			out.Anthropic = s
		case "openai":
			out.OpenAI = s
		case "gemini":
			out.Gemini = s
		case "huggingface":
			out.HuggingFace = s
		case "sentry":
			out.Sentry = s
		case "github-app":
			out.GithubApp = s
		}
		out.All[logical] = s
	}

	return out, nil
}

// AttachAccess emits an IAM policy that grants the orchestrator + runtime
// IRSA roles secretsmanager:GetSecretValue + kms:Decrypt on exactly the
// secrets they need. The policy is attached inline to each role.
//
// We attach via aws/iam.RolePolicy keyed by role ARN -> a JSON policy
// rendered from the secret ARNs.
//
// Note: the role ARNs come from the compute stack via StackReference, so
// the policy text is a pulumi.Output, not a static string.
func (s *Secrets) AccessPolicyDoc(secretsKey *kms.Key) pulumi.StringOutput {
	arns := pulumi.StringArray{}
	for _, sec := range s.All {
		arns = append(arns, sec.Arn)
	}
	return pulumi.All(secretsKey.Arn, arns.ToStringArrayOutput()).ApplyT(func(args []interface{}) (string, error) {
		keyArn, _ := args[0].(string)
		secretArns, _ := args[1].([]string)

		doc := map[string]any{
			"Version": "2012-10-17",
			"Statement": []map[string]any{
				{
					"Sid":      "ReadIronflyerSecrets",
					"Effect":   "Allow",
					"Action":   []string{"secretsmanager:GetSecretValue", "secretsmanager:DescribeSecret"},
					"Resource": secretArns,
				},
				{
					"Sid":      "DecryptIronflyerSecrets",
					"Effect":   "Allow",
					"Action":   []string{"kms:Decrypt", "kms:DescribeKey"},
					"Resource": []string{keyArn},
				},
			},
		}
		b, err := json.Marshal(doc)
		return string(b), err
	}).(pulumi.StringOutput)
}
