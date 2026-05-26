package ledger

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// recordMobile is the shared shape for every mobile-category record
// helper. It computes amount = minutes * UnitPriceUSD(category), wraps
// the call site's identifiers into Metadata, and routes the entry
// through Service.Write. Marking the entry MarginRelevant = true keeps
// it on the gross-margin axis of the dashboards alongside provider
// and sandbox cost.
func recordMobile(
	ctx context.Context,
	svc Service,
	tenantID, executionID uuid.UUID,
	cat Category,
	units decimal.Decimal,
	metadata map[string]any,
) (Entry, error) {
	if svc == nil {
		return Entry{}, fmt.Errorf("ledger: nil service")
	}
	if units.Sign() <= 0 {
		return Entry{}, ErrZeroAmount
	}
	amount := units.Mul(UnitPriceUSD(cat))
	if amount.Sign() <= 0 {
		return Entry{}, ErrZeroAmount
	}
	meta := map[string]any{
		"category": string(cat),
		"units":    units.String(),
	}
	for k, v := range metadata {
		meta[k] = v
	}
	e := Entry{
		TenantID:       tenantID,
		EntryType:      cat,
		Direction:      DebitDirection,
		AmountUSD:      amount,
		Billable:       true,
		MarginRelevant: true,
		Metadata:       meta,
	}
	if executionID != uuid.Nil {
		eid := executionID
		e.ExecutionID = &eid
	}
	return svc.Write(ctx, e)
}

// RecordMobileBuildMinutes appends a mobile-build-minutes entry to the
// ledger. Caller passes the tenant + execution id, the cumulative
// minutes since the last record, and the platform (android|ios) — the
// platform tag flows through to dashboards so the cost panel can split
// by target.
func RecordMobileBuildMinutes(ctx context.Context, svc Service, tenantID, executionID uuid.UUID, platform string, minutes float64) (Entry, error) {
	return recordMobile(ctx, svc, tenantID, executionID,
		CategoryMobileBuildMin,
		decimal.NewFromFloat(minutes),
		map[string]any{"platform": platform},
	)
}

// RecordEmulatorMinutes appends a one-shot emulator-minute record.
// The workspace ID is the per-user sandbox the emulator booted inside;
// it flows through to the dashboards as a metadata tag.
func RecordEmulatorMinutes(ctx context.Context, svc Service, tenantID, executionID uuid.UUID, workspaceID string, minutes float64) (Entry, error) {
	return recordMobile(ctx, svc, tenantID, executionID,
		CategoryEmulatorMin,
		decimal.NewFromFloat(minutes),
		map[string]any{"workspace_id": workspaceID},
	)
}

// RecordMacWorkspaceMinutes records Mac pool consumption. The hostID
// flows through so we can sum-per-host for capacity planning and
// reconcile against the upstream Mac pool invoice.
func RecordMacWorkspaceMinutes(ctx context.Context, svc Service, tenantID, executionID uuid.UUID, hostID string, minutes float64) (Entry, error) {
	return recordMobile(ctx, svc, tenantID, executionID,
		CategoryMacWorkspaceMin,
		decimal.NewFromFloat(minutes),
		map[string]any{"mac_host_id": hostID},
	)
}

// RecordEASBuild records one EAS build credit. Pass the EAS build ID
// in externalRef so the dashboard can link out to the EAS console
// instead of duplicating their build metadata locally.
func RecordEASBuild(ctx context.Context, svc Service, tenantID, executionID uuid.UUID, externalRef string) (Entry, error) {
	return recordMobile(ctx, svc, tenantID, executionID,
		CategoryEASBuildCredit,
		decimal.NewFromInt(1),
		map[string]any{"eas_build_id": externalRef},
	)
}

// RecordAppetizeMinutes records Appetize.io streaming minutes — the
// iOS-simulator-in-browser fallback used when the project is iOS-bound
// but the user is on the free tier with no dedicated Mac workspace.
func RecordAppetizeMinutes(ctx context.Context, svc Service, tenantID, executionID uuid.UUID, sessionID string, minutes float64) (Entry, error) {
	return recordMobile(ctx, svc, tenantID, executionID,
		CategoryAppetizeMin,
		decimal.NewFromFloat(minutes),
		map[string]any{"appetize_session_id": sessionID},
	)
}

// DeviceCloudMeta carries the metadata RecordDeviceCloudMinutes stamps
// onto the ledger entry. The Manager builds one of these per session
// start/end so the dashboards can split spend per provider, device,
// project, and workspace without parsing a free-form blob.
type DeviceCloudMeta struct {
	Provider    string
	SessionID   string
	DeviceID    string
	ProjectID   string
	WorkspaceID string
	Phase       string // "start" | "end" | "tick"
}

// RecordDeviceCloudMinutes appends a Pro-tier device-cloud minute
// record. minutes can be fractional — the Manager calls this with
// fractional values when reconciling a session that ran less than a
// full minute past the up-front charge.
func RecordDeviceCloudMinutes(ctx context.Context, svc Service, tenantID, executionID uuid.UUID, meta DeviceCloudMeta, minutes float64) (Entry, error) {
	return recordMobile(ctx, svc, tenantID, executionID,
		CategoryDeviceCloudMin,
		decimal.NewFromFloat(minutes),
		map[string]any{
			"provider":     meta.Provider,
			"session_id":   meta.SessionID,
			"device_id":    meta.DeviceID,
			"project_id":   meta.ProjectID,
			"workspace_id": meta.WorkspaceID,
			"phase":        meta.Phase,
		},
	)
}
