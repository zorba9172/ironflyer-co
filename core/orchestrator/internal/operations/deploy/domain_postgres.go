package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresDomainService struct {
	pool *pgxpool.Pool
	core *MemoryDomainService
}

func NewPostgresDomainService(pool *pgxpool.Pool, providers map[string]DomainProvider, registrars map[string]Registrar, defaultProvider, defaultRegistrar string, opts ...DomainServiceOption) *PostgresDomainService {
	return &PostgresDomainService{
		pool: pool,
		core: NewMemoryDomainService(providers, registrars, defaultProvider, defaultRegistrar, opts...),
	}
}

func (s *PostgresDomainService) ReserveSubdomain(ctx context.Context, in ReserveSubdomainInput) (Domain, error) {
	d, err := s.core.ReserveSubdomain(ctx, in)
	if err != nil {
		return Domain{}, err
	}
	return s.insert(ctx, d)
}

func (s *PostgresDomainService) ConnectDomain(ctx context.Context, in ConnectDomainInput) (Domain, error) {
	d, err := s.core.ConnectDomain(ctx, in)
	if err != nil {
		return Domain{}, err
	}
	return s.insert(ctx, d)
}

func (s *PostgresDomainService) PurchaseDomain(ctx context.Context, in PurchaseDomainInput) (Domain, error) {
	if in.Registrar == "" {
		in.Registrar = s.core.defaultRegistrar
	}
	registrar, ok := s.core.registrars[in.Registrar]
	if !ok {
		return Domain{}, fmt.Errorf("%w: registrar %s", ErrUnknownTarget, in.Registrar)
	}
	purchaseCtx := WithTenant(WithProject(ctx, in.ProjectID), in.TenantID)
	avail, err := s.core.validateDomainPurchase(purchaseCtx, registrar, in)
	if err != nil {
		return Domain{}, err
	}
	if _, err := registrar.Purchase(purchaseCtx, in); err != nil {
		return Domain{}, err
	}
	d, err := s.core.ConnectDomain(ctx, ConnectDomainInput{
		TenantID: in.TenantID, ProjectID: in.ProjectID, DeployID: in.DeployID,
		Hostname: in.Domain, Provider: in.Provider, Primary: in.Primary,
		Metadata: domainPurchaseMetadata(in.Metadata, registrar, avail),
	})
	if err != nil {
		return Domain{}, err
	}
	d.Kind = DomainKindRegistered
	d.Registrar = registrar.Name()
	return s.insert(ctx, d)
}

func (s *PostgresDomainService) CheckDomain(ctx context.Context, tenantID, id string) (Domain, error) {
	d, err := s.get(ctx, tenantID, id)
	if err != nil {
		return Domain{}, err
	}
	provider, _, err := s.core.provider(d.Provider)
	if err != nil {
		return Domain{}, err
	}
	check, err := provider.Check(ctx, d)
	if err != nil {
		return Domain{}, err
	}
	now := time.Now().UTC()
	meta := mergeDomainMetadata(d.Metadata, check.Metadata)
	records := d.DNSRecords
	if len(check.DNSRecords) > 0 {
		records = sortedDNSRecords(check.DNSRecords)
	}
	verifiedAt := d.VerifiedAt
	if check.VerificationStatus == "verified" && verifiedAt == nil {
		t := now
		verifiedAt = &t
	}
	liveAt := d.LiveAt
	if check.Status == DomainStatusLive && liveAt == nil {
		t := now
		liveAt = &t
	}
	if check.CertificateStatus == "" {
		check.CertificateStatus = d.CertificateStatus
	}
	if check.Status == "" {
		check.Status = d.Status
	}
	if check.Instructions == "" {
		check.Instructions = d.Instructions
	}
	if _, err := s.pool.Exec(ctx, `
        UPDATE deploy_domains
           SET status = $3, verification_status = $4, certificate_status = $5,
               dns_records = $6::jsonb, instructions = $7, metadata = $8::jsonb,
               updated_at = $9, verified_at = $10, live_at = $11
         WHERE id = $1 AND tenant_id = $2`,
		id, tenantID, string(check.Status), check.VerificationStatus, string(check.CertificateStatus),
		mustJSON(records), check.Instructions, mustJSON(meta), now, verifiedAt, liveAt); err != nil {
		return Domain{}, fmt.Errorf("deploy domain: check update: %w", err)
	}
	return s.get(ctx, tenantID, id)
}

func (s *PostgresDomainService) SetPrimaryDomain(ctx context.Context, tenantID, id string) (Domain, error) {
	d, err := s.get(ctx, tenantID, id)
	if err != nil {
		return Domain{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Domain{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
        UPDATE deploy_domains SET is_primary = false, updated_at = now()
         WHERE tenant_id = $1 AND project_id = $2`, tenantID, d.ProjectID); err != nil {
		return Domain{}, err
	}
	if _, err := tx.Exec(ctx, `
        UPDATE deploy_domains SET is_primary = true, updated_at = now()
         WHERE tenant_id = $1 AND id = $2`, tenantID, id); err != nil {
		return Domain{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Domain{}, err
	}
	return s.get(ctx, tenantID, id)
}

func (s *PostgresDomainService) ListProjectDomains(ctx context.Context, tenantID, projectID string) ([]Domain, error) {
	rows, err := s.pool.Query(ctx, domainSelect()+`
         WHERE tenant_id = $1 AND project_id = $2 AND status <> 'removed'
         ORDER BY is_primary DESC, created_at DESC`, tenantID, projectID)
	if err != nil {
		return nil, fmt.Errorf("deploy domain: list: %w", err)
	}
	defer rows.Close()
	out := make([]Domain, 0)
	for rows.Next() {
		d, err := scanDomain(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *PostgresDomainService) DomainAvailability(ctx context.Context, registrarName, domain string) (DomainAvailability, error) {
	return s.core.DomainAvailability(ctx, registrarName, domain)
}

func (s *PostgresDomainService) insert(ctx context.Context, d Domain) (Domain, error) {
	if d.ID == "" {
		d.ID = ""
	}
	now := time.Now().UTC()
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now
	}
	if d.UpdatedAt.IsZero() {
		d.UpdatedAt = now
	}
	if d.Metadata == nil {
		d.Metadata = map[string]any{}
	}
	if d.Status == DomainStatusLive {
		if d.VerifiedAt == nil {
			t := now
			d.VerifiedAt = &t
		}
		if d.LiveAt == nil {
			t := now
			d.LiveAt = &t
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Domain{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if d.Primary {
		if _, err := tx.Exec(ctx, `
            UPDATE deploy_domains SET is_primary = false, updated_at = now()
             WHERE tenant_id = $1 AND project_id = $2`, d.TenantID, d.ProjectID); err != nil {
			return Domain{}, err
		}
	}
	var id string
	row := tx.QueryRow(ctx, `
        INSERT INTO deploy_domains(
            tenant_id, project_id, deploy_id, hostname, kind, status,
            provider, registrar, is_primary, dns_records,
            verification_status, certificate_status, instructions, metadata,
            created_at, updated_at, verified_at, live_at
        ) VALUES (
            $1, $2, $3, $4, $5, $6,
            $7, $8, $9, $10::jsonb,
            $11, $12, $13, $14::jsonb,
            $15, $16, $17, $18
        ) RETURNING id`,
		d.TenantID, d.ProjectID, nullStringIfEmpty(d.DeployID), d.Hostname, string(d.Kind), string(d.Status),
		d.Provider, nullStringIfEmpty(d.Registrar), d.Primary, mustJSON(d.DNSRecords),
		d.VerificationStatus, string(d.CertificateStatus), d.Instructions, mustJSON(d.Metadata),
		d.CreatedAt, d.UpdatedAt, d.VerifiedAt, d.LiveAt)
	if err := row.Scan(&id); err != nil {
		if strings.Contains(err.Error(), "deploy_domains_tenant_hostname_live") {
			return Domain{}, fmt.Errorf("%w: domain already connected", ErrInvalidState)
		}
		return Domain{}, fmt.Errorf("deploy domain: insert: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Domain{}, err
	}
	return s.get(ctx, d.TenantID, id)
}

func (s *PostgresDomainService) get(ctx context.Context, tenantID, id string) (Domain, error) {
	row := s.pool.QueryRow(ctx, domainSelect()+` WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	d, err := scanDomain(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Domain{}, ErrNotFound
	}
	return d, err
}

func domainSelect() string {
	return `SELECT
            id, tenant_id, project_id, COALESCE(deploy_id::text, ''),
            hostname, kind, status, provider, COALESCE(registrar, ''),
            is_primary, COALESCE(dns_records::text, '[]'),
            COALESCE(verification_status, ''), certificate_status,
            COALESCE(instructions, ''), COALESCE(metadata::text, '{}'),
            created_at, updated_at, verified_at, live_at
        FROM deploy_domains`
}

func scanDomain(row pgx.Row) (Domain, error) {
	var d Domain
	var kind, status, cert, recordsRaw, metaRaw string
	if err := row.Scan(
		&d.ID, &d.TenantID, &d.ProjectID, &d.DeployID,
		&d.Hostname, &kind, &status, &d.Provider, &d.Registrar,
		&d.Primary, &recordsRaw,
		&d.VerificationStatus, &cert,
		&d.Instructions, &metaRaw,
		&d.CreatedAt, &d.UpdatedAt, &d.VerifiedAt, &d.LiveAt,
	); err != nil {
		return Domain{}, err
	}
	d.Kind = DomainKind(kind)
	d.Status = DomainStatus(status)
	d.CertificateStatus = CertificateStatus(cert)
	_ = json.Unmarshal([]byte(recordsRaw), &d.DNSRecords)
	_ = json.Unmarshal([]byte(metaRaw), &d.Metadata)
	d.DNSRecords = sortedDNSRecords(d.DNSRecords)
	if d.Metadata == nil {
		d.Metadata = map[string]any{}
	}
	return d, nil
}

func mustJSON(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}
