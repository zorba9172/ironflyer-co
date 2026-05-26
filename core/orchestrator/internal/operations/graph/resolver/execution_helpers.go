package resolver

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"ironflyer/core/orchestrator/internal/business/execution"
	"ironflyer/core/orchestrator/internal/operations/graph/model"
	"ironflyer/core/orchestrator/internal/business/ledger"
)

// executionToGraphQL converts the internal Execution row to the
// GraphQL Execution model. Pulled out so every resolver returns a
// consistent shape.
func executionToGraphQL(e execution.Execution) *model.Execution {
	out := &model.Execution{
		ID:                e.ID,
		TenantID:          e.TenantID,
		Status:            string(e.Status),
		BudgetUsd:         floatOfDecimal(e.BudgetUSD),
		ReservedUsd:       floatOfDecimal(e.ReservedUSD),
		SpentUsd:          floatOfDecimal(e.SpentUSD),
		RefundedUsd:       floatOfDecimal(e.RefundedUSD),
		RevenueUsd:        floatOfDecimal(e.RevenueUSD),
		ProviderCostUsd:   floatOfDecimal(e.ProviderCostUSD),
		SandboxCostUsd:    floatOfDecimal(e.SandboxCostUSD),
		StorageCostUsd:    floatOfDecimal(e.StorageCostUSD),
		DeploymentCostUsd: floatOfDecimal(e.DeploymentCostUSD),
		CompletionScore:   e.CompletionScore,
		CreatedAt:         e.CreatedAt,
		AdmittedAt:        e.AdmittedAt,
		StartedAt:         e.StartedAt,
		EndedAt:           e.EndedAt,
	}
	if e.ProjectID != "" {
		p := e.ProjectID
		out.ProjectID = &p
	}
	if e.BlueprintID != "" {
		b := e.BlueprintID
		out.BlueprintID = &b
	}
	// A63 — surface the runtime workspace bound to this execution so the
	// frontend can address the workspace API without a second lookup.
	if e.WorkspaceID != "" {
		w := e.WorkspaceID
		out.WorkspaceID = &w
	}
	if e.GrossMarginPct != nil {
		f, _ := e.GrossMarginPct.Float64()
		out.GrossMarginPct = &f
	}
	if e.ExpectedCompletionDelta != nil {
		v := *e.ExpectedCompletionDelta
		out.ExpectedCompletionDelta = &v
	}
	if e.RiskScore != nil {
		v := *e.RiskScore
		out.RiskScore = &v
	}
	if e.StopLossUSD != nil {
		f, _ := e.StopLossUSD.Float64()
		out.StopLossUsd = &f
	}
	if e.PromptSummary != "" {
		v := e.PromptSummary
		out.PromptSummary = &v
	}
	if e.FailureReason != "" {
		v := e.FailureReason
		out.FailureReason = &v
	}
	if len(e.Metadata) > 0 {
		var m map[string]any
		_ = json.Unmarshal(e.Metadata, &m)
		out.Metadata = model.JSON(m)
	} else {
		out.Metadata = model.JSON{}
	}
	return out
}

// tenantUUIDFor returns a UUID for tenant. If tenant is already a
// UUID we parse it; otherwise we deterministically hash it via UUIDv5
// so the ledger key is stable across calls.
func tenantUUIDFor(tenant string) uuid.UUID {
	if id, err := uuid.Parse(tenant); err == nil {
		return id
	}
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(tenant))
}

// commitWalletForExecution releases the unused hold and writes the
// platform-margin ledger entry on terminal status. Idempotent — safe
// to call from multiple terminal-state hooks.
func commitWalletForExecution(ctx context.Context, r *mutationResolver, id string) error {
	if r.ExecutionSvc == nil {
		return nil
	}
	exec, err := r.ExecutionSvc.Get(ctx, id)
	if err != nil {
		return err
	}
	if r.WalletSvc != nil {
		unused := exec.ReservedUSD.Sub(exec.SpentUSD)
		if unused.IsPositive() {
			_ = r.WalletSvc.Release(ctx, exec.TenantID, unused)
		}
		if exec.SpentUSD.IsPositive() {
			_ = r.WalletSvc.Debit(ctx, exec.TenantID, exec.SpentUSD)
		}
	}
	if r.LedgerSvc != nil {
		execUUID, perr := uuid.Parse(id)
		if perr == nil {
			margin := exec.RevenueUSD.Sub(exec.ProviderCostUSD).Sub(exec.SandboxCostUSD).Sub(exec.StorageCostUSD).Sub(exec.DeploymentCostUSD)
			if margin.IsPositive() {
				_, _ = r.LedgerSvc.Write(ctx, ledger.Entry{
					TenantID:       tenantUUIDFor(exec.TenantID),
					ExecutionID:    &execUUID,
					EntryType:      ledger.EntryPlatformMargin,
					Direction:      ledger.CreditDirection,
					AmountUSD:      margin,
					MarginRelevant: true,
				})
			}
			unused := exec.ReservedUSD.Sub(exec.SpentUSD)
			if unused.IsPositive() {
				_, _ = r.LedgerSvc.Write(ctx, ledger.Entry{
					TenantID:    tenantUUIDFor(exec.TenantID),
					ExecutionID: &execUUID,
					EntryType:   ledger.EntryCreditRelease,
					Direction:   ledger.CreditDirection,
					AmountUSD:   unused,
				})
			}
		}
	}
	return nil
}
