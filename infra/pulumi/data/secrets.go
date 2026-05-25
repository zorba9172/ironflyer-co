package data

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/secretsmanager"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Secrets is every Secrets Manager secret the orchestrator + runtime + web
// surfaces depend on. Each secret is created with a placeholder JSON value
// flagged TODO so the operator MUST rotate it before going to production.
// The placeholder body is also stable, so re-running Pulumi after a manual
// rotation won't clobber the rotated value (the resource is created with
// IgnoreChanges on SecretString).
type Secrets struct {
	PostgresMaster        *secretsmanager.Secret
	PostgresMasterVersion *secretsmanager.SecretVersion
	RedisAuth             *secretsmanager.Secret
	RedisAuthVersion      *secretsmanager.SecretVersion

	StripeSecretKey     *secretsmanager.Secret
	StripeWebhookSecret *secretsmanager.Secret

	AnthropicAPIKey   *secretsmanager.Secret
	OpenAIAPIKey      *secretsmanager.Secret
	GeminiAPIKey      *secretsmanager.Secret
	HuggingfaceAPIKey *secretsmanager.Secret

	SentryDSN     *secretsmanager.Secret
	JWTSigningKey *secretsmanager.Secret

	GithubAppPrivateKey    *secretsmanager.Secret
	GithubAppWebhookSecret *secretsmanager.Secret

	SurrealRootCreds *secretsmanager.Secret

	// flat index for outputs + ExternalSecrets reflection.
	byLogical map[string]*secretsmanager.Secret
}

const placeholderTODO = `{"value":"TODO-ROTATE-BEFORE-PROD","ironflyer.rotation_required":true}`

func provisionSecrets(ctx *pulumi.Context, env *stackEnv, kms *KMSKeys) (*Secrets, error) {
	s := &Secrets{byLogical: map[string]*secretsmanager.Secret{}}

	// Postgres master password — generated, not a placeholder, because we
	// need it in the RDS cluster too.
	pgPwd, err := random.NewRandomPassword(ctx, name(env, "pg-master-pw"), &random.RandomPasswordArgs{
		Length:  pulumi.Int(40),
		Special: pulumi.Bool(false),
	})
	if err != nil {
		return nil, err
	}
	s.PostgresMaster, s.PostgresMasterVersion, err = managedSecret(ctx, env, kms,
		"postgres-master", "ironflyer/postgres/master",
		pgPwd.Result.ApplyT(func(p string) (string, error) {
			return fmt.Sprintf(`{"username":"ironflyer_master","password":%q}`, p), nil
		}).(pulumi.StringOutput),
	)
	if err != nil {
		return nil, err
	}

	// Redis auth token — same pattern: generated so the cluster + clients
	// share a credential, but exposed via Secrets Manager for rotation.
	redisAuth, err := random.NewRandomPassword(ctx, name(env, "redis-auth"), &random.RandomPasswordArgs{
		Length:  pulumi.Int(64),
		Special: pulumi.Bool(false),
	})
	if err != nil {
		return nil, err
	}
	s.RedisAuth, s.RedisAuthVersion, err = managedSecret(ctx, env, kms,
		"redis-auth", "ironflyer/redis/auth",
		redisAuth.Result.ApplyT(func(p string) (string, error) {
			return fmt.Sprintf(`{"auth_token":%q}`, p), nil
		}).(pulumi.StringOutput),
	)
	if err != nil {
		return nil, err
	}

	// All third-party + product secrets land here. Placeholder body, the
	// operator must rotate them before go-live (the placeholder value is
	// flagged `ironflyer.rotation_required: true`).
	type spec struct {
		field     **secretsmanager.Secret
		logical   string
		secretKey string
	}
	all := []spec{
		{&s.StripeSecretKey, "stripe-secret-key", "ironflyer/stripe/secret-key"},
		{&s.StripeWebhookSecret, "stripe-webhook-secret", "ironflyer/stripe/webhook-secret"},
		{&s.AnthropicAPIKey, "anthropic-api-key", "ironflyer/anthropic/api-key"},
		{&s.OpenAIAPIKey, "openai-api-key", "ironflyer/openai/api-key"},
		{&s.GeminiAPIKey, "gemini-api-key", "ironflyer/gemini/api-key"},
		{&s.HuggingfaceAPIKey, "huggingface-api-key", "ironflyer/huggingface/api-key"},
		{&s.SentryDSN, "sentry-dsn", "ironflyer/sentry/dsn"},
		{&s.JWTSigningKey, "jwt-signing-key", "ironflyer/jwt/signing-key"},
		{&s.GithubAppPrivateKey, "github-app-private-key", "ironflyer/github-app/private-key"},
		{&s.GithubAppWebhookSecret, "github-app-webhook-secret", "ironflyer/github-app/webhook-secret"},
		{&s.SurrealRootCreds, "surreal-root", "ironflyer/surreal/root-credentials"},
	}
	for _, item := range all {
		sec, _, err := managedSecret(ctx, env, kms, item.logical, item.secretKey, pulumi.String(placeholderTODO).ToStringOutput())
		if err != nil {
			return nil, err
		}
		*item.field = sec
	}

	// Build the by-logical index used for outputs + ExternalSecrets.
	s.byLogical["postgres-master"] = s.PostgresMaster
	s.byLogical["redis-auth"] = s.RedisAuth
	s.byLogical["stripe-secret-key"] = s.StripeSecretKey
	s.byLogical["stripe-webhook-secret"] = s.StripeWebhookSecret
	s.byLogical["anthropic-api-key"] = s.AnthropicAPIKey
	s.byLogical["openai-api-key"] = s.OpenAIAPIKey
	s.byLogical["gemini-api-key"] = s.GeminiAPIKey
	s.byLogical["huggingface-api-key"] = s.HuggingfaceAPIKey
	s.byLogical["sentry-dsn"] = s.SentryDSN
	s.byLogical["jwt-signing-key"] = s.JWTSigningKey
	s.byLogical["github-app-private-key"] = s.GithubAppPrivateKey
	s.byLogical["github-app-webhook-secret"] = s.GithubAppWebhookSecret
	s.byLogical["surreal-root"] = s.SurrealRootCreds

	return s, nil
}

// managedSecret centralizes Secrets Manager creation: KMS-encrypted with
// the secrets CMK, project tags applied, and an initial version pinned so
// later operator rotations don't churn Pulumi state.
func managedSecret(ctx *pulumi.Context, env *stackEnv, kms *KMSKeys, logical, awsName string, value pulumi.StringOutput) (*secretsmanager.Secret, *secretsmanager.SecretVersion, error) {
	sec, err := secretsmanager.NewSecret(ctx, name(env, "secret-"+logical), &secretsmanager.SecretArgs{
		Name:                 pulumi.String(fmt.Sprintf("ironflyer/%s/%s", env.stack, awsName[len("ironflyer/"):])),
		Description:          pulumi.String(fmt.Sprintf("Ironflyer %s (%s)", logical, env.stack)),
		KmsKeyId:             kms.SecretsKey.Arn,
		RecoveryWindowInDays: pulumi.Int(7),
		Tags:                 env.tags,
	})
	if err != nil {
		return nil, nil, err
	}
	ver, err := secretsmanager.NewSecretVersion(ctx, name(env, "secret-"+logical+"-v1"), &secretsmanager.SecretVersionArgs{
		SecretId:     sec.ID(),
		SecretString: value,
	}, pulumi.IgnoreChanges([]string{"secretString"}))
	if err != nil {
		return nil, nil, err
	}
	return sec, ver, nil
}

// ArnsByLogicalName produces a map suitable for ctx.Export of the form
// {"postgres-master": "arn:aws:secretsmanager:...", ...}.
func (s *Secrets) ArnsByLogicalName() pulumi.StringMapOutput {
	m := pulumi.StringMap{}
	for k, v := range s.byLogical {
		m[k] = v.Arn
	}
	return m.ToStringMapOutput()
}

// LogicalNames returns the ordered list of logical names used by
// consumers.go to mirror each AWS secret into a K8s Secret.
func (s *Secrets) LogicalNames() []string {
	return []string{
		"postgres-master",
		"redis-auth",
		"stripe-secret-key",
		"stripe-webhook-secret",
		"anthropic-api-key",
		"openai-api-key",
		"gemini-api-key",
		"huggingface-api-key",
		"sentry-dsn",
		"jwt-signing-key",
		"github-app-private-key",
		"github-app-webhook-secret",
		"surreal-root",
	}
}

// SecretByLogical returns the underlying *Secret for ExternalSecret
// wiring (need the Name attribute for the secret-key reference).
func (s *Secrets) SecretByLogical(logical string) *secretsmanager.Secret {
	return s.byLogical[logical]
}
