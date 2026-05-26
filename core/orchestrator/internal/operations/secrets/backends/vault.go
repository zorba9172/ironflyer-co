package backends

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	vaultapi "github.com/hashicorp/vault/api"

	"ironflyer/core/orchestrator/internal/operations/secrets"
)

// Vault is the HashiCorp Vault backend. It targets the KV v2 secret
// engine — the canonical way operators store application secrets in
// Vault — and reads at the operator-supplied path
// (SecretRef.BackendRef, with SecretRef.Name as a fallback).
//
// The struct supports a "disabled" mode mirroring AWSSecrets: when
// either the address or the token is empty the client is left nil and
// every Load returns ErrBackendNotConfigured. This keeps dev
// environments and CI bootable without a live Vault.
//
// AuthN today is token-based — production operators are expected to
// inject an AppRole-derived, short-lived token. The struct keeps the
// token off any logged path; broker.go already enforces that token
// strings never reach audit attrs.
type Vault struct {
	Addr   string
	Token  string
	client *vaultapi.Client
	// mount is the KV v2 mount path, default "secret". Operators who
	// run multiple KV engines can extend this via SecretRef metadata
	// (mount=<name>) which we honor per-call.
	mount string
}

// NewVault constructs a Vault backend. Empty addr/token => disabled
// state. Construction failures (URL parse, etc.) also leave the
// backend disabled so the orchestrator stays bootable; Load will
// surface ErrBackendNotConfigured.
func NewVault(addr, token string) *Vault {
	v := &Vault{Addr: addr, Token: token, mount: "secret"}
	if addr == "" || token == "" {
		return v
	}
	cfg := vaultapi.DefaultConfig()
	cfg.Address = addr
	cli, err := vaultapi.NewClient(cfg)
	if err != nil {
		return v
	}
	cli.SetToken(token)
	v.client = cli
	return v
}

func (v *Vault) Name() secrets.Backend { return secrets.BackendVault }

// Load reads from KV v2. The path is taken from SecretRef.BackendRef
// (preferred — "myapp/stripe/secret_key") or SecretRef.Name. The
// returned value resolves by convention:
//   - if the secret data contains a "value" key, that field's value;
//   - else, if there is exactly one field, that field's value;
//   - else an error (ambiguous shape).
//
// The mount path defaults to "secret" but can be overridden per-ref
// via SecretRef.Metadata["mount"] for operators running multiple KV
// engines.
func (v *Vault) Load(ctx context.Context, ref secrets.SecretRef) ([]byte, error) {
	if v.client == nil {
		return nil, secrets.ErrBackendNotConfigured
	}
	path := ref.BackendRef
	if path == "" {
		path = ref.Name
	}
	if path == "" {
		return nil, fmt.Errorf("%w: vault requires name or backend_ref", secrets.ErrSecretNotFound)
	}

	mount := v.mount
	if ref.Metadata != nil {
		if m, ok := ref.Metadata["mount"].(string); ok && m != "" {
			mount = m
		}
	}

	kv := v.client.KVv2(mount)
	var (
		sec *vaultapi.KVSecret
		err error
	)
	if ref.Version > 0 {
		sec, err = kv.GetVersion(ctx, path, ref.Version)
	} else {
		sec, err = kv.Get(ctx, path)
	}
	if err != nil {
		if isVaultNotFound(err) {
			return nil, fmt.Errorf("%w: vault %s/%s", secrets.ErrSecretNotFound, mount, path)
		}
		return nil, fmt.Errorf("vault: get %s/%s: %w", mount, path, err)
	}
	if sec == nil || sec.Data == nil {
		return nil, fmt.Errorf("%w: vault %s/%s empty", secrets.ErrSecretNotFound, mount, path)
	}

	if raw, ok := sec.Data["value"]; ok {
		return coerceBytes(raw)
	}
	if len(sec.Data) == 1 {
		for _, raw := range sec.Data {
			return coerceBytes(raw)
		}
	}
	return nil, fmt.Errorf("vault: ambiguous secret shape at %s/%s (expected 'value' key or single field)", mount, path)
}

// coerceBytes turns the dynamic value Vault returns (string, number,
// bool, []byte) into []byte. JSON-decoded numbers arrive as
// json.Number / float64 depending on SDK version, so we handle both.
func coerceBytes(v any) ([]byte, error) {
	switch t := v.(type) {
	case string:
		return []byte(t), nil
	case []byte:
		cp := make([]byte, len(t))
		copy(cp, t)
		return cp, nil
	case bool:
		return []byte(strconv.FormatBool(t)), nil
	case float64:
		return []byte(strconv.FormatFloat(t, 'f', -1, 64)), nil
	case int:
		return []byte(strconv.Itoa(t)), nil
	case int64:
		return []byte(strconv.FormatInt(t, 10)), nil
	case fmt.Stringer:
		return []byte(t.String()), nil
	case nil:
		return nil, fmt.Errorf("vault: nil secret value")
	default:
		return nil, fmt.Errorf("vault: unsupported secret value type %T", v)
	}
}

// isVaultNotFound recognises the Vault SDK's various "missing path"
// signals. KVv2.Get returns api.ErrSecretNotFound on a clean miss
// (the path was queried successfully but contains no version), and
// the lower-level HTTP path returns a ResponseError with status 404
// when the path itself has never been written.
func isVaultNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errIs(err, vaultapi.ErrSecretNotFound) {
		return true
	}
	if re, ok := err.(*vaultapi.ResponseError); ok {
		if re.StatusCode == 404 {
			return true
		}
	}
	// Fallback: the SDK historically returned plain errors with a
	// "Code: 404" substring on KV v2 misses. Be lenient.
	msg := err.Error()
	if strings.Contains(msg, "Code: 404") || strings.Contains(msg, "secret not found") {
		return true
	}
	return false
}

// errIs avoids pulling in the errors package twice for one call. The
// Vault SDK exposes ErrSecretNotFound as a sentinel.
func errIs(err, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
