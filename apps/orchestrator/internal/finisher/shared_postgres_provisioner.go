// Package finisher — shared Postgres provisioner. For self-hosted
// Ironflyer deployments that don't want to pay Supabase per project,
// this provisioner connects to a single operator-supplied Postgres
// server and carves out a fresh, isolated database per Ironflyer
// project. Works with any reachable Postgres: docker-compose, RDS,
// Aiven, Neon admin connection, an internal cluster, …
//
// Isolation per project:
//   - new role  `ironflyer_<safeProjectID>` with a random password
//   - new db    `ironflyer_<safeProjectID>` owned by that role
//   - the returned DSN authenticates as the per-project role, so even
//     a misbehaving project can't reach other projects' data.
//
// Safety / cleanup:
//   - This provisioner is INTENTIONALLY one-shot — there is no Destroy
//     hook, because operators usually want the data to outlive
//     ephemeral orchestrator restarts. Run a periodic janitor against
//     the admin DSN to reap projects that no longer exist in the
//     Ironflyer store, if desired.
//   - All identifiers come from sanitiseDBIdentifier so SQL injection
//     through the projectID is impossible.

package finisher

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"

	"ironflyer/apps/orchestrator/internal/domain"
)

// SharedPostgresProvisioner implements DBProvisioner against a single
// admin Postgres connection. The AdminDSN must point to a user with
// CREATEDB + CREATEROLE rights (or be the cluster superuser). The
// returned project DSN authenticates as the per-project role.
type SharedPostgresProvisioner struct {
	// AdminDSN is the connection string the provisioner uses to issue
	// CREATE ROLE / CREATE DATABASE. Typically points at the `postgres`
	// system database with a privileged user.
	AdminDSN string
	// PublicHost overrides the host portion of the per-project DSN. Useful
	// when the orchestrator can reach Postgres on an internal host
	// (`postgres.internal`) but the user workspace must use the public
	// hostname. Leave empty to reuse the AdminDSN host as-is.
	PublicHost string
	// PublicPort overrides the port portion of the per-project DSN. Same
	// internal/external concern as PublicHost.
	PublicPort string
}

// Provision carves out a fresh role + database for projectID.
func (p *SharedPostgresProvisioner) Provision(ctx context.Context, projectID string, _ []domain.EntityDef) (DBProvision, error) {
	if p == nil {
		return DBProvision{}, errors.New("nil SharedPostgresProvisioner")
	}
	if strings.TrimSpace(p.AdminDSN) == "" {
		return DBProvision{}, errors.New("shared-postgres: AdminDSN required")
	}
	conn, err := pgx.Connect(ctx, p.AdminDSN)
	if err != nil {
		return DBProvision{}, fmt.Errorf("shared-postgres: admin connect: %w", err)
	}
	defer conn.Close(ctx)

	ident := sanitiseDBIdentifier(projectID)
	if ident == "" {
		return DBProvision{}, errors.New("shared-postgres: projectID has no usable characters")
	}
	role := "ironflyer_" + ident
	dbName := "ironflyer_" + ident
	password, err := randHex(16)
	if err != nil {
		return DBProvision{}, fmt.Errorf("shared-postgres: password: %w", err)
	}

	// CREATE ROLE — quote both the identifier (with double quotes) AND the
	// literal password (with single quotes + escape). Postgres does not
	// support parameterised CREATE ROLE, so we have to build the SQL by
	// hand. The identifier is alnum-only because of sanitiseDBIdentifier;
	// the password is hex so no escaping concern there.
	createRoleSQL := fmt.Sprintf(
		`CREATE ROLE %s WITH LOGIN PASSWORD '%s' NOCREATEDB NOCREATEROLE NOINHERIT`,
		quoteIdent(role), password,
	)
	if _, err := conn.Exec(ctx, createRoleSQL); err != nil {
		if !isAlreadyExists(err) {
			return DBProvision{}, fmt.Errorf("shared-postgres: create role: %w", err)
		}
		// Role already exists — rotate the password so re-provisioning
		// (which the caller usually avoids via Project.Secrets check) still
		// produces a working DSN. Idempotent in spirit.
		alterSQL := fmt.Sprintf(`ALTER ROLE %s WITH LOGIN PASSWORD '%s'`, quoteIdent(role), password)
		if _, err := conn.Exec(ctx, alterSQL); err != nil {
			return DBProvision{}, fmt.Errorf("shared-postgres: rotate role: %w", err)
		}
	}

	createDBSQL := fmt.Sprintf(`CREATE DATABASE %s OWNER %s`, quoteIdent(dbName), quoteIdent(role))
	if _, err := conn.Exec(ctx, createDBSQL); err != nil && !isAlreadyExists(err) {
		return DBProvision{}, fmt.Errorf("shared-postgres: create database: %w", err)
	}

	// Build the per-project DSN. Reuse the admin DSN as the host template,
	// then swap in role + password + db. The pgconn parser can do this,
	// but a small url.URL walk keeps us off another dependency surface.
	projectDSN, err := buildProjectDSN(p.AdminDSN, role, password, dbName, p.PublicHost, p.PublicPort)
	if err != nil {
		return DBProvision{}, fmt.Errorf("shared-postgres: build dsn: %w", err)
	}
	return DBProvision{
		DSN:      projectDSN,
		Provider: "shared-postgres",
	}, nil
}

// quoteIdent wraps an identifier in double quotes and doubles any embedded
// quote. Standard Postgres identifier-quoting; safe even though we
// sanitise to alnum_/underscore in practice.
func quoteIdent(id string) string {
	return `"` + strings.ReplaceAll(id, `"`, `""`) + `"`
}

// sanitiseDBIdentifier returns a Postgres-safe identifier derived from s.
// Postgres identifiers must start with a letter or underscore and may
// contain only ASCII letters, digits, and underscores when unquoted —
// we additionally hard-cap at 32 chars so the final role / db names
// (with the "ironflyer_" prefix) stay under the 63-byte Postgres limit.
func sanitiseDBIdentifier(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-':
			b.WriteRune('_')
		}
		if b.Len() >= 32 {
			break
		}
	}
	out := b.String()
	if out == "" {
		return ""
	}
	// Identifiers cannot start with a digit. Prepend an underscore if so.
	if out[0] >= '0' && out[0] <= '9' {
		out = "_" + out
		if len(out) > 32 {
			out = out[:32]
		}
	}
	return out
}

// isAlreadyExists turns the Postgres "duplicate role / database" error
// into a boolean. Used so re-provisioning is idempotent even though the
// caller normally avoids the second call entirely via Secrets cache.
func isAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already exists") ||
		strings.Contains(msg, "sqlstate 42710") || // duplicate object
		strings.Contains(msg, "sqlstate 42p04") // duplicate database
}

// buildProjectDSN rewrites the admin DSN to authenticate as the new
// per-project role on the new database. PublicHost / PublicPort override
// the host portion when the project will be reaching Postgres over a
// different network than the orchestrator (very common in K8s setups).
func buildProjectDSN(adminDSN, role, password, dbName, publicHost, publicPort string) (string, error) {
	u, err := url.Parse(adminDSN)
	if err != nil {
		return "", err
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return "", fmt.Errorf("admin DSN scheme must be postgres:// or postgresql://, got %q", u.Scheme)
	}
	u.User = url.UserPassword(role, password)
	if publicHost != "" {
		host := publicHost
		if publicPort != "" {
			host = publicHost + ":" + publicPort
		} else if p := u.Port(); p != "" {
			host = publicHost + ":" + p
		}
		u.Host = host
	} else if publicPort != "" {
		u.Host = u.Hostname() + ":" + publicPort
	}
	u.Path = "/" + dbName
	return u.String(), nil
}

// ensure the SharedPostgresProvisioner satisfies the contract.
var _ DBProvisioner = (*SharedPostgresProvisioner)(nil)

// randHex (shared with supabase_provisioner.go) generates the per-project
// password. Declared there to keep this file dependency-free at the top.
