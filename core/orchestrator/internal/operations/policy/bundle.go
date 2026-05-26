package policy

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// embeddedBundles holds the default Rego policy bundles compiled into
// the orchestrator binary so a fresh deploy is never policy-empty.
//
//go:embed bundles/*.rego
var embeddedBundles embed.FS

// LoadBundles returns the active Rego sources keyed by module name
// (filename without extension) plus a deterministic version hash that
// changes any time a bundle's bytes change.
//
// Source-of-truth precedence:
//  1. cfg.BundleDir, if set and contains .rego files — operator
//     hot-swap path. policy.Reloader (see reload.go) watches the dir
//     with fsnotify and pushes new bundles into the PDP via Rebind so
//     operators no longer need a SIGHUP/restart to pick up changes.
//  2. Embedded bundles — the safe default shipped with the binary.
func LoadBundles(cfg Config) (map[string]string, string, error) {
	if cfg.BundleDir != "" {
		bundles, err := loadFromDisk(cfg.BundleDir)
		if err != nil {
			return nil, "", fmt.Errorf("policy: load bundle dir %q: %w", cfg.BundleDir, err)
		}
		if len(bundles) > 0 {
			return bundles, hashBundles(bundles), nil
		}
		// Fall through to embedded if dir was empty — that's almost
		// certainly an operator mistake; we'd rather boot with the
		// default-deny bundle than refuse to start.
	}
	bundles, err := loadFromEmbedded()
	if err != nil {
		return nil, "", fmt.Errorf("policy: load embedded bundles: %w", err)
	}
	return bundles, hashBundles(bundles), nil
}

func loadFromEmbedded() (map[string]string, error) {
	out := map[string]string{}
	entries, err := fs.ReadDir(embeddedBundles, "bundles")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".rego") {
			continue
		}
		raw, err := embeddedBundles.ReadFile(filepath.ToSlash(filepath.Join("bundles", e.Name())))
		if err != nil {
			return nil, err
		}
		name := strings.TrimSuffix(e.Name(), ".rego")
		out[name] = string(raw)
	}
	return out, nil
}

// ReloadFromDisk is the public form of loadFromDisk + hashBundles used
// by policy.Reloader to fetch the current on-disk bundle snapshot.
// Returns the (bundles, version) pair on success. An empty directory
// produces (nil, "", nil) so callers can distinguish "nothing to load"
// from a hard error.
func ReloadFromDisk(dir string) (map[string]string, string, error) {
	if dir == "" {
		return nil, "", nil
	}
	bundles, err := loadFromDisk(dir)
	if err != nil {
		return nil, "", err
	}
	if len(bundles) == 0 {
		return nil, "", nil
	}
	return bundles, hashBundles(bundles), nil
}

func loadFromDisk(dir string) (map[string]string, error) {
	out := map[string]string{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".rego") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		name := strings.TrimSuffix(e.Name(), ".rego")
		out[name] = string(raw)
	}
	return out, nil
}

// hashBundles returns a stable sha256 of the (name, content) tuples
// after sorting by name. Two orchestrators running the same bundle
// set produce identical hashes; any byte-level change rolls the hash.
func hashBundles(bundles map[string]string) string {
	names := make([]string, 0, len(bundles))
	for n := range bundles {
		names = append(names, n)
	}
	sort.Strings(names)
	h := sha256.New()
	for _, n := range names {
		h.Write([]byte(n))
		h.Write([]byte{0})
		h.Write([]byte(bundles[n]))
		h.Write([]byte{0})
	}
	return "pbv_" + hex.EncodeToString(h.Sum(nil))[:16]
}
