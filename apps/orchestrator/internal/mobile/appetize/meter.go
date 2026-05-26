package appetize

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"ironflyer/apps/orchestrator/internal/ledger"
)

// MeteredClient wraps Client with ledger emission so every embed-URL
// hand-off and every minute of streaming lands as a CategoryAppetizeMin
// ledger entry. The wrapped *Client stays accessible so callers that
// only need the REST surface can pass MeteredClient transparently.
type MeteredClient struct {
	*Client
	ledger ledger.Service
}

// NewMetered returns a MeteredClient. Either argument may be nil — in
// that case the wrapper degrades to "log only, no ledger" so an
// orchestrator booted without a ledger backend still serves embed URLs.
func NewMetered(c *Client, l ledger.Service) *MeteredClient {
	return &MeteredClient{Client: c, ledger: l}
}

// SessionContext carries the identifiers required to attribute Appetize
// minutes to a tenant + execution + workspace. Passed by the resolver
// to StartSession / EndSession so the meter doesn't have to crack
// open the request context.
type SessionContext struct {
	TenantID    uuid.UUID
	ExecutionID uuid.UUID
	ProjectID   string
	WorkspaceID string
}

// StartSession returns the embed URL the operator's browser should
// iframe. The session is identified by the embed URL itself — Appetize
// doesn't issue a separate session token at start; the wall-clock
// duration is recorded later by EndSession.
//
// The session ID we hand back is the publicKey so EndSession can tag
// the ledger entry without re-deriving it from the embed URL.
func (m *MeteredClient) StartSession(ctx context.Context, sc SessionContext, publicKey string, opts EmbedOptions) (string, error) {
	if m == nil || m.Client == nil {
		return "", errors.New("appetize: metered client not configured")
	}
	if strings.TrimSpace(publicKey) == "" {
		return "", errors.New("appetize: StartSession requires publicKey")
	}
	embed := m.Client.EmbedURL(publicKey, opts)
	m.Client.logger.Info().
		Str("project_id", sc.ProjectID).
		Str("workspace_id", sc.WorkspaceID).
		Str("public_key", publicKey).
		Msg("appetize: session started")
	return embed, nil
}

// EndSession records the accumulated streaming minutes against the
// caller's tenant + execution. The Appetize Free tier caps at 100
// minutes / month — we don't enforce the cap here (Appetize does), but
// the ledger entry gives the dashboard the visibility to warn before
// the wall hits.
func (m *MeteredClient) EndSession(ctx context.Context, sc SessionContext, publicKey string, durationMinutes float64) error {
	if m == nil {
		return nil
	}
	if m.ledger == nil || durationMinutes <= 0 {
		// Nothing to record — degrade silently so a free-tier dev box
		// without a ledger keeps working.
		return nil
	}
	_, err := ledger.RecordAppetizeMinutes(ctx, m.ledger, sc.TenantID, sc.ExecutionID, publicKey, durationMinutes)
	return err
}
