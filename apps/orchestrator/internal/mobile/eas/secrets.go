package eas

import (
	"os"
	"strings"

	"ironflyer/apps/orchestrator/internal/domain"
)

// ProjectSecretEASToken is the canonical key under which a project's
// per-project EAS bearer token is stored in domain.Project.Secrets.
// Keep this string stable — operator dashboards and the secret-upload
// UI reference it directly.
const ProjectSecretEASToken = "EAS_TOKEN"

// EnvEASToken is the global / self-hosted fallback env var.
const EnvEASToken = "EAS_TOKEN"

// ResolveExpoToken returns the EAS bearer for a given project.
// Priority:
//  1. Project.Secrets["EAS_TOKEN"]
//  2. env EAS_TOKEN (global fallback for self-hosted operators)
//  3. ErrEASTokenMissing
//
// Trims surrounding whitespace from each source; an all-whitespace
// secret is treated as missing so a yank-and-replace flow with an
// empty paste doesn't silently fall through to a stale env var.
func ResolveExpoToken(p *domain.Project) (string, error) {
	if p != nil {
		if v, ok := p.Secrets[ProjectSecretEASToken]; ok {
			if t := strings.TrimSpace(v); t != "" {
				return t, nil
			}
		}
	}
	if t := strings.TrimSpace(os.Getenv(EnvEASToken)); t != "" {
		return t, nil
	}
	return "", ErrEASTokenMissing
}
