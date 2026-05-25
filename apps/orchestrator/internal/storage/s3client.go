// Package storage centralises the S3-compatible client configuration
// used by every orchestrator path that talks to object storage —
// today: nothing (the orchestrator's snapshots live on the runtime
// pod's disk, and audit/memory persistence rides Postgres + SurrealDB),
// but soon: bulk audit exports, encrypted memory backups, and the
// long-term retention bucket for replay artefacts.
//
// The helper exists today as the single source of truth for which
// backend an operator wants to talk to. Four backends are recognised
// via the S3_BACKEND env var:
//
//	aws    — AWS S3 (default; standard region resolution).
//	r2     — Cloudflare R2; endpoint built from R2_ACCOUNT_ID, creds
//	         from R2_ACCESS_KEY_ID / R2_SECRET_ACCESS_KEY, force path
//	         style, region "auto". R2's biggest selling point is zero
//	         egress: replicating snapshots back to a worker is free.
//	spaces — DigitalOcean Spaces; endpoint derived from DO_SPACES_REGION
//	         (defaults to nyc3) as https://<region>.digitaloceanspaces.com,
//	         creds from DO_SPACES_ACCESS_KEY_ID / DO_SPACES_SECRET_ACCESS_KEY,
//	         path style forced for consistency with the R2 path. Spaces is
//	         S3-compatible and pairs naturally with the DigitalOcean
//	         Pulumi stack at infra/pulumi-do/. If S3_BACKEND is unset but
//	         DO_SPACES_ACCESS_KEY_ID is set, this backend auto-activates.
//	minio  — Self-hosted MinIO (or any S3-compatible host); endpoint
//	         from MINIO_ENDPOINT, region "us-east-1" by default
//	         (overridable via MINIO_REGION), path style forced. Useful
//	         for docker-compose dev and air-gapped self-hosted prod.
//
// This file deliberately does NOT import the AWS SDK — the orchestrator
// has zero S3 callers today and pulling a multi-MB SDK in to scaffold
// future work would bloat the build for no gain. Callers that adopt S3
// import the SDK and use the Config struct returned here to wire their
// own *s3.Client (see docs/SCALE.md "Storage backends" for the recipe).
// When a real caller lands, swap this helper's signature to return a
// configured `*s3.Client` and update the doc.
package storage

import (
	"fmt"
	"os"
	"strings"
)

// Backend names. Mirrors the S3_BACKEND env shape: any unrecognised
// value falls back to BackendAWS so a typo doesn't silently route the
// audit log to a half-configured R2 bucket.
const (
	BackendAWS    = "aws"
	BackendR2     = "r2"
	BackendSpaces = "spaces"
	BackendMinIO  = "minio"
)

// Config is the resolved S3-compatible client configuration. Callers
// feed it into the AWS SDK's `aws.Config` via:
//
//	cfg := aws.Config{
//	    Region:      sc.Region,
//	    Credentials: credentials.NewStaticCredentialsProvider(sc.AccessKey, sc.SecretKey, ""),
//	}
//	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
//	    if sc.Endpoint != "" {
//	        o.BaseEndpoint = aws.String(sc.Endpoint)
//	    }
//	    o.UsePathStyle = sc.ForcePathStyle
//	})
//
// AccessKey / SecretKey are intentionally exposed as strings rather than
// a credentials.Provider so this helper stays SDK-free.
type Config struct {
	Backend        string
	Endpoint       string // empty = SDK default (AWS regional endpoint)
	Region         string
	AccessKey      string
	SecretKey      string
	ForcePathStyle bool
}

// LoadConfig reads the relevant env vars and returns the resolved
// config plus a human-readable label suitable for boot logs. Errors
// fire only when a backend was selected but its required env vars are
// missing — operators should fail fast on that rather than silently
// routing reads/writes to the wrong place.
func LoadConfig() (Config, string, error) {
	backend := strings.ToLower(strings.TrimSpace(os.Getenv("S3_BACKEND")))
	if backend == "" {
		// Auto-detect DigitalOcean Spaces when its access key is set but
		// the operator forgot to declare S3_BACKEND explicitly. Saves a
		// boot-time face-plant on the DO Pulumi path.
		if strings.TrimSpace(os.Getenv("DO_SPACES_ACCESS_KEY_ID")) != "" {
			backend = BackendSpaces
		} else {
			backend = BackendAWS
		}
	}
	switch backend {
	case BackendAWS:
		// Standard AWS region resolution: env > IRSA > shared config.
		// We honour AWS_REGION if it's set so the boot log reflects
		// the active region.
		region := strings.TrimSpace(os.Getenv("AWS_REGION"))
		if region == "" {
			region = "us-east-1"
		}
		return Config{
			Backend:   BackendAWS,
			Region:    region,
			AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
			SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		}, "aws (region=" + region + ")", nil

	case BackendR2:
		account := strings.TrimSpace(os.Getenv("R2_ACCOUNT_ID"))
		if account == "" {
			return Config{}, "", fmt.Errorf("storage: S3_BACKEND=r2 requires R2_ACCOUNT_ID")
		}
		accessKey := strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID"))
		secretKey := strings.TrimSpace(os.Getenv("R2_SECRET_ACCESS_KEY"))
		if accessKey == "" || secretKey == "" {
			return Config{}, "", fmt.Errorf("storage: S3_BACKEND=r2 requires R2_ACCESS_KEY_ID and R2_SECRET_ACCESS_KEY")
		}
		endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", account)
		return Config{
			Backend:        BackendR2,
			Endpoint:       endpoint,
			Region:         "auto",
			AccessKey:      accessKey,
			SecretKey:      secretKey,
			ForcePathStyle: true,
		}, "r2 (account=" + account + ")", nil

	case BackendSpaces:
		region := strings.TrimSpace(os.Getenv("DO_SPACES_REGION"))
		if region == "" {
			region = "nyc3"
		}
		accessKey := strings.TrimSpace(os.Getenv("DO_SPACES_ACCESS_KEY_ID"))
		secretKey := strings.TrimSpace(os.Getenv("DO_SPACES_SECRET_ACCESS_KEY"))
		if accessKey == "" || secretKey == "" {
			return Config{}, "", fmt.Errorf("storage: S3_BACKEND=spaces requires DO_SPACES_ACCESS_KEY_ID and DO_SPACES_SECRET_ACCESS_KEY")
		}
		endpoint := fmt.Sprintf("https://%s.digitaloceanspaces.com", region)
		return Config{
			Backend:        BackendSpaces,
			Endpoint:       endpoint,
			Region:         region,
			AccessKey:      accessKey,
			SecretKey:      secretKey,
			ForcePathStyle: true,
		}, "spaces (region=" + region + ")", nil

	case BackendMinIO:
		endpoint := strings.TrimSpace(os.Getenv("MINIO_ENDPOINT"))
		if endpoint == "" {
			return Config{}, "", fmt.Errorf("storage: S3_BACKEND=minio requires MINIO_ENDPOINT")
		}
		// MinIO doesn't care about region but the SDK insists on one.
		region := strings.TrimSpace(os.Getenv("MINIO_REGION"))
		if region == "" {
			region = "us-east-1"
		}
		return Config{
			Backend:        BackendMinIO,
			Endpoint:       endpoint,
			Region:         region,
			AccessKey:      strings.TrimSpace(os.Getenv("MINIO_ACCESS_KEY_ID")),
			SecretKey:      strings.TrimSpace(os.Getenv("MINIO_SECRET_ACCESS_KEY")),
			ForcePathStyle: true,
		}, "minio (endpoint=" + endpoint + ")", nil

	default:
		// Misconfiguration — fail fast so the operator notices.
		return Config{}, "", fmt.Errorf("storage: unknown S3_BACKEND=%q (expected aws|r2|spaces|minio)", backend)
	}
}

// MustLoadConfig is the convenience wrapper for boot-time wiring that
// would rather panic loudly than ship a half-configured object-store
// path. Use LoadConfig from anything called after main().
func MustLoadConfig() (Config, string) {
	cfg, label, err := LoadConfig()
	if err != nil {
		panic(err)
	}
	return cfg, label
}
