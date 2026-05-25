// Package docker hosts the runtime's Docker-driver scale-readiness helpers.
// The bulk of the Docker driver still lives in apps/runtime/internal/sandbox/
// (legacy package name kept to avoid churn in callers); this package adds
// the pieces that are only meaningful in a multi-pod deployment — most
// importantly the EFS-backed host mount path used in the production
// `docker run -v <host>:/home/coder` invocation.
package docker

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
)

// HostMount returns the host-side path to bind into a workspace
// container, plus whether EFS mode was used. When the configured EFS
// root is unavailable (no mount, no permissions) the helper falls back
// to a per-pod local directory and logs a warning so operators notice.
//
// efsRoot is typically "/var/lib/ironflyer/workspaces" (the default
// configured in cmd/runtime via RUNTIME_EFS_MOUNT). The caller is
// expected to have created the workspace ID subdirectory beforehand;
// HostMount is a pure path resolver + sanity check.
func HostMount(efsRoot, fallbackRoot, workspaceID string, logger zerolog.Logger) (string, bool, error) {
	if workspaceID == "" {
		return "", false, errors.New("workspace id required")
	}
	if efsRoot == "" {
		efsRoot = "/var/lib/ironflyer/workspaces"
	}
	if usable(efsRoot) {
		path := filepath.Join(efsRoot, workspaceID)
		if err := os.MkdirAll(path, 0o755); err == nil {
			return path, true, nil
		}
	}
	// Fall back to per-pod local storage. Loud warning: this means a
	// container started on this pod will lose its state if the pod dies.
	logger.Warn().
		Str("efsRoot", efsRoot).
		Str("fallback", fallbackRoot).
		Str("workspace", workspaceID).
		Msg("EFS mount unavailable, falling back to per-pod local storage")
	if fallbackRoot == "" {
		fallbackRoot = "/tmp/ironflyer-workspaces"
	}
	path := filepath.Join(fallbackRoot, workspaceID)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", false, err
	}
	return path, false, nil
}

// usable reports whether the EFS root exists and we can write to it.
// We check writability by attempting to create a sentinel file — EFS
// mounts that are present but read-only fail this check, which is the
// right behaviour: the runtime needs read+write to host workspaces.
func usable(root string) bool {
	if root == "" {
		return false
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		// Try to create the directory if missing. EFS volumes are usually
		// pre-mounted; this branch only matters in dev where the operator
		// pointed RUNTIME_EFS_MOUNT at a fresh empty path.
		if err := os.MkdirAll(root, 0o755); err != nil {
			return false
		}
	}
	sentinel := filepath.Join(root, ".ironflyer-write-check")
	f, err := os.Create(sentinel)
	if err != nil {
		return false
	}
	_ = f.Close()
	_ = os.Remove(sentinel)
	return true
}
