package wireup

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"ironflyer/apps/orchestrator/internal/audit"
	"ironflyer/apps/orchestrator/internal/secrets"
	"ironflyer/apps/orchestrator/internal/secrets/backends"
)

// SecretsResult is the (broker, redactor) pair the secrets wireup
// returns. The broker is wired into the deploy adapter + future
// runtime mounter; the redactor is wired into the providers package so
// LLM prompts never leak a resolved value.
type SecretsResult struct {
	Broker   secrets.Broker
	Redactor *secrets.Redactor
}

// BuildSecrets constructs the secret broker per
// `secrets.LoadConfig`. When the configured backend is unavailable
// (e.g. AWS/Vault stubs) the broker still boots — every backend stub
// returns ErrSecretMissing at runtime so the caller can fall through.
func BuildSecrets(cfg secrets.Config, pool *pgxpool.Pool, audit audit.Store, log zerolog.Logger) SecretsResult {
	var store secrets.Store
	if pool != nil {
		store = secrets.NewPostgresStore(pool)
	} else {
		store = secrets.NewMemoryStore()
	}

	redactor := secrets.NewRedactor()

	bes := []secrets.BackendImpl{
		backends.NewEnv(cfg.EnvPrefix),
		backends.NewMemory(),
		backends.NewAWSSecrets(cfg.AWSRegion),
		backends.NewVault(cfg.VaultAddr, cfg.VaultToken),
	}

	opts := []secrets.Option{
		secrets.WithLogger(log.With().Str("svc", "secrets").Logger()),
		secrets.WithRedactor(redactor),
	}
	if audit != nil {
		opts = append(opts, secrets.WithAudit(audit))
	}
	broker := secrets.New(cfg, store, bes, opts...)
	return SecretsResult{Broker: broker, Redactor: redactor}
}
