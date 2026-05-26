package secrets

import (
	"os"
	"strings"
)

// Config selects which backend the broker uses by default and how it
// reaches the various managed services. The intent is that ops sets
// IRONFLYER_SECRETS_* env vars on the orchestrator pod and the broker
// initialises itself accordingly; tests set fields directly.
type Config struct {
	// Enabled toggles the entire broker subsystem. When false, the
	// orchestrator should boot with a no-op broker so legacy paths
	// that pre-date V22 secret handling don't break in dev.
	Enabled bool

	// DefaultBackend is the backend used when a SecretRef does not
	// specify one (legacy rows, dev seeding).
	DefaultBackend Backend

	// EnvPrefix is prepended to the secret name when the env backend
	// resolves a SecretRef — e.g. EnvPrefix="IRONFLYER_SECRET_" and
	// name="STRIPE_SECRET_KEY" -> os.Getenv("IRONFLYER_SECRET_STRIPE_SECRET_KEY").
	// An empty prefix means "use the name verbatim".
	EnvPrefix string

	// AWSRegion is the region the AWS Secrets Manager client targets.
	// Real client construction is deferred (see backends/aws_secrets.go);
	// the field is captured here so config is a single source of truth.
	AWSRegion string

	// VaultAddr is the HashiCorp Vault address (e.g. https://vault:8200).
	VaultAddr string

	// VaultToken is the unwrapped token loaded at startup. The
	// orchestrator should treat this string as sensitive — keep it out
	// of structured logs and prefer SecretID-based unwrap in production.
	VaultToken string
}

// LoadConfig reads the standard IRONFLYER_SECRETS_* env vars and
// returns a populated Config. Defaults: Enabled=true, backend=env,
// prefix="IRONFLYER_SECRET_".
func LoadConfig() Config {
	c := Config{
		Enabled:        true,
		DefaultBackend: BackendEnv,
		EnvPrefix:      "IRONFLYER_SECRET_",
	}
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_SECRETS_ENABLED")); v != "" {
		switch strings.ToLower(v) {
		case "0", "false", "off", "no":
			c.Enabled = false
		}
	}
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_SECRETS_BACKEND")); v != "" {
		c.DefaultBackend = Backend(strings.ToLower(v))
	}
	if v, ok := os.LookupEnv("IRONFLYER_SECRETS_ENV_PREFIX"); ok {
		// Allow operators to explicitly clear the prefix.
		c.EnvPrefix = v
	}
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_SECRETS_AWS_REGION")); v != "" {
		c.AWSRegion = v
	}
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_SECRETS_VAULT_ADDR")); v != "" {
		c.VaultAddr = v
	}
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_SECRETS_VAULT_TOKEN")); v != "" {
		c.VaultToken = v
	}
	return c
}
