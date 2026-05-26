package resolver

import (
	"path"
	"strings"
)

// normalizeWritePath sanitises a user-supplied project-relative path
// before it lands on disk. The rules:
//
//   - empty / whitespace-only paths are rejected.
//   - absolute paths ("/foo", "C:\\foo") are rejected.
//   - any segment equal to ".." is rejected (no parent traversal).
//   - backslashes are normalised to forward slashes for cross-platform
//     callers (the VSCode extension on Windows for example).
//   - the result is path.Clean()ed so duplicate or trailing slashes
//     collapse deterministically.
//
// The function is local to the resolver package so it can be reused by
// writeProjectFiles and the upcoming generateMobileAssets persistence
// step without leaking path policy into the domain layer.
func normalizeWritePath(p string) (string, bool) {
	if p == "" {
		return "", false
	}
	s := strings.TrimSpace(p)
	if s == "" {
		return "", false
	}
	// Normalise Windows-style separators.
	s = strings.ReplaceAll(s, "\\", "/")
	// Reject absolute paths and drive-letter prefixes.
	if strings.HasPrefix(s, "/") {
		return "", false
	}
	if len(s) >= 2 && s[1] == ':' {
		return "", false
	}
	// Reject any explicit parent traversal.
	for _, seg := range strings.Split(s, "/") {
		if seg == ".." {
			return "", false
		}
	}
	cleaned := path.Clean(s)
	if cleaned == "." || cleaned == "" {
		return "", false
	}
	if strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return "", false
	}
	return cleaned, true
}
