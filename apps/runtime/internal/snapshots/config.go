package snapshots

import (
	"os"
	"strings"
)

// Config wires the snapshot Manager. Bucket="" leaves the manager in
// no-op mode, so dev paths that don't have S3 credentials still build
// and run unchanged.
type Config struct {
	// Bucket is the S3/R2 bucket name (WORKSPACE_BUCKET env). Empty
	// disables the manager.
	Bucket string
	// Region is the AWS region (AWS_REGION env). Empty uses SDK
	// default resolution.
	Region string
	// Endpoint overrides the S3 endpoint URL — required for R2 and
	// MinIO; leave empty for AWS S3.
	Endpoint string
	// Prefix is the in-bucket key prefix. Defaults to "snapshots".
	// The full layout becomes:
	//   <prefix>/workspaces/<id>/<ts>.tar.zst
	//   <prefix>/workspaces/<id>/LATEST
	//   <prefix>/archives/<id>.tar.zst
	Prefix string
	// Retention is the number of checkpoint tarballs kept per
	// workspace; older objects are reaped after each successful
	// checkpoint. Defaults to 5.
	Retention int
	// KMSKeyID, when set, requests SSE-KMS with the named key. Empty
	// uses bucket-default encryption.
	KMSKeyID string
	// Excludes is the list of directory names to skip when building
	// a snapshot — read from IRONFLYER_SNAPSHOT_EXCLUDE
	// (comma-separated) and merged with defaults.
	Excludes []string
}

// DefaultExcludes are the transient directories the runtime never
// uploads — they should be rebuilt by package managers / build tools
// after restore.
func DefaultExcludes() []string {
	return []string{
		"node_modules",
		".next",
		"dist",
		"coverage",
		".cache",
		"target",
		"__pycache__",
	}
}

// LoadExcludesFromEnv merges DefaultExcludes() with the optional
// IRONFLYER_SNAPSHOT_EXCLUDE env var (comma-separated). Duplicates
// are de-duped, empty entries dropped.
func LoadExcludesFromEnv() []string {
	out := DefaultExcludes()
	extra := strings.TrimSpace(os.Getenv("IRONFLYER_SNAPSHOT_EXCLUDE"))
	if extra == "" {
		return out
	}
	seen := make(map[string]struct{}, len(out))
	for _, e := range out {
		seen[e] = struct{}{}
	}
	for _, raw := range strings.Split(extra, ",") {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
