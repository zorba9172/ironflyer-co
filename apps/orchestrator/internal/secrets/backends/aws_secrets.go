package backends

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	"ironflyer/apps/orchestrator/internal/secrets"
)

// AWSSecrets is the AWS Secrets Manager backend. It resolves a
// SecretRef.BackendRef (or, falling back, SecretRef.Name) as the
// SecretId — which AWS accepts as either a plain name or a full ARN.
//
// The struct supports a "disabled" mode: when the configured region
// is empty the client is left nil and every Load returns
// ErrBackendNotConfigured. This is what dev environments observe when
// the orchestrator boots without any AWS-side wiring; production
// operators set IRONFLYER_SECRETS_AWS_REGION to enable the backend.
//
// Credentials come from the standard AWS chain
// (env, shared config, IAM role, IRSA) via the SDK's default
// LoadDefaultConfig — we deliberately do NOT accept inline keys here:
// every cred path the orchestrator uses must be operator-managed, and
// the broker should never see raw IAM material.
type AWSSecrets struct {
	Region string
	client *secretsmanager.Client
}

// NewAWSSecrets constructs an AWS Secrets Manager backend. When
// region is empty the backend is left in disabled state — wireup
// always calls this regardless of whether AWS is configured, so an
// empty region is the explicit "AWS path is off" signal. A non-empty
// region triggers SDK config load; if that fails we still return a
// usable (disabled) backend so the orchestrator can boot, but Load
// will surface ErrBackendNotConfigured.
func NewAWSSecrets(region string) *AWSSecrets {
	a := &AWSSecrets{Region: region}
	if region == "" {
		return a
	}
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(region))
	if err != nil {
		// Disabled state on config failure — operator will see
		// ErrBackendNotConfigured at release time. We can't log here
		// without a logger dep, and panicking would take the whole
		// orchestrator down for a backend that may not even be used.
		return a
	}
	a.client = secretsmanager.NewFromConfig(cfg)
	return a
}

func (a *AWSSecrets) Name() secrets.Backend { return secrets.BackendAWSSecrets }

// Load fetches the secret value from AWS Secrets Manager. The
// SecretId is taken from SecretRef.BackendRef (which may be a plain
// name or a full ARN) with SecretRef.Name as a fallback. When a
// version is pinned on the SecretRef we look up the matching version
// stage; otherwise AWS returns AWSCURRENT.
func (a *AWSSecrets) Load(ctx context.Context, ref secrets.SecretRef) ([]byte, error) {
	if a.client == nil {
		return nil, secrets.ErrBackendNotConfigured
	}
	secretID := ref.BackendRef
	if secretID == "" {
		secretID = ref.Name
	}
	if secretID == "" {
		return nil, fmt.Errorf("%w: aws_secrets requires name or backend_ref", secrets.ErrSecretNotFound)
	}

	in := &secretsmanager.GetSecretValueInput{SecretId: aws.String(secretID)}
	if ref.Version > 0 {
		// Operators rotate AWS secrets out-of-band; the broker's
		// version counter maps to the "v<n>" version stage when one
		// has been published. If not present, AWS will return
		// ResourceNotFoundException which we surface distinctly.
		in.VersionStage = aws.String(fmt.Sprintf("v%d", ref.Version))
	}
	out, err := a.client.GetSecretValue(ctx, in)
	if err != nil {
		var rnf *smtypes.ResourceNotFoundException
		if errors.As(err, &rnf) {
			return nil, fmt.Errorf("%w: aws secret %q", secrets.ErrSecretNotFound, secretID)
		}
		var inv *smtypes.InvalidRequestException
		if errors.As(err, &inv) {
			return nil, fmt.Errorf("aws_secrets: invalid request for %q: %w", secretID, err)
		}
		return nil, fmt.Errorf("aws_secrets: get value %q: %w", secretID, err)
	}

	// Binary values take precedence — when an operator stored bytes
	// directly we hand those back; otherwise SecretString is the
	// canonical UTF-8 form.
	if len(out.SecretBinary) > 0 {
		// Defensive copy so callers can zero their buffer without
		// touching SDK-owned memory.
		cp := make([]byte, len(out.SecretBinary))
		copy(cp, out.SecretBinary)
		return cp, nil
	}
	if out.SecretString != nil {
		return []byte(*out.SecretString), nil
	}
	return nil, fmt.Errorf("aws_secrets: empty value for %q", secretID)
}
