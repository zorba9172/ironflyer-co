package guild

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/ai/learning"
)

// TemplateRegistry extends the in-repo blueprints.Registry concept to
// community-authored, rev-share-able starters. The two registries stay
// separate so:
//
//   - The built-in blueprints keep their compile-time guarantees (the
//     files ship in the binary via go:embed, gates run at CI time).
//   - Community templates can be authored without a redeploy and earn
//     their author a payout per install.
//
// At resolve-time the cockpit may surface both — that fan-out belongs
// in the resolver layer (or a future unified-catalog facade), NOT here.
// Keeping templates orthogonal to blueprints means a corrupt template
// upload cannot break the built-in starter path.
type TemplateRegistry struct {
	svc    Service
	escrow *Escrow
	logger zerolog.Logger
}

// NewTemplateRegistry builds the registry bound to a Service + Escrow.
func NewTemplateRegistry(svc Service, escrow *Escrow, logger zerolog.Logger) *TemplateRegistry {
	return &TemplateRegistry{svc: svc, escrow: escrow, logger: logger}
}

// SplitInstallAmount returns (platformCut, authorPayout) for a template
// install at the given price. platformCut = price * 0.15; authorPayout
// = price - platformCut so the two pieces sum to price exactly.
func SplitInstallAmount(price decimal.Decimal) (platformCut, authorPayout decimal.Decimal) {
	platformCut = price.Mul(platformTemplateCutPct).Round(6)
	authorPayout = price.Sub(platformCut)
	return
}

// computeInstallSplit is the documented invariant the Install row
// enforces. Exposed for the resolver/escrow review path.
func computeInstallSplit(price decimal.Decimal) (Install, error) {
	if price.IsNegative() {
		return Install{}, ErrInvalidAmount
	}
	platformCut, authorPayout := SplitInstallAmount(price)
	return Install{
		AmountUSD:       price,
		AuthorPayoutUSD: authorPayout,
		PlatformCutUSD:  platformCut,
	}, nil
}

// Install drives the install flow: idempotency check, wallet hold +
// debit via Escrow.HoldAndDebit, install-row insert, install-count
// bump, OutcomeEvent emission. Returns the resolved template so the
// resolver can echo it back to the caller.
//
// opKey is InstallTemplateOpKey(templateID, projectID) — a project
// installing the same template twice is treated as ONE install (the
// second call short-circuits via RecallOp). A Temporal retry of the
// SAME install also short-circuits.
func (r *TemplateRegistry) Install(ctx context.Context, t Template, projectID, tenantID string) (Template, error) {
	opKey := InstallTemplateOpKey(t.ID, projectID)
	if prior, ok, err := r.svc.RecallOp(ctx, opKey); err != nil {
		return Template{}, err
	} else if ok {
		if prior.Status == "succeeded" {
			return t, nil
		}
		return Template{}, ErrInvalidStatus
	}
	if err := r.escrow.HoldAndDebit(ctx, tenantID, t.PriceUSD); err != nil {
		_ = r.svc.RecordOp(ctx, opKey, string(OpInstallTemplate), t.PriceUSD, "failed", err.Error())
		return Template{}, err
	}
	install, err := computeInstallSplit(t.PriceUSD)
	if err != nil {
		return Template{}, err
	}
	install.TemplateID = t.ID
	install.ProjectID = projectID
	install.TenantID = tenantID
	install.InstalledAt = time.Now().UTC()
	if _, err := r.svc.RecordInstall(ctx, install); err != nil {
		return Template{}, err
	}
	if err := r.svc.IncrementTemplateInstallCount(ctx, t.ID); err != nil {
		// Non-fatal: the install row is the source of truth; the
		// counter is a denormalised dashboard projection.
		r.logger.Warn().Err(err).Str("template_id", t.ID).Msg("guild: install count bump failed")
	}
	_ = r.svc.RecordOp(ctx, opKey, string(OpInstallTemplate), t.PriceUSD, "succeeded", "")
	learning.Publish(ctx, learning.OutcomeEvent{
		Kind: learning.OutcomeKind("guild.template.installed"),
		Attributes: map[string]any{
			"template_id":       t.ID,
			"template_slug":     t.Slug,
			"project_id":        projectID,
			"amount_usd":        install.AmountUSD.String(),
			"author_payout_usd": install.AuthorPayoutUSD.String(),
			"platform_cut_usd":  install.PlatformCutUSD.String(),
		},
		CostUSD:   learning.DecimalPtr(install.AmountUSD),
		MarginUSD: learning.DecimalPtr(install.PlatformCutUSD),
		Success:   learning.BoolPtr(true),
		Tags:      map[string]string{"template_slug": t.Slug},
	})
	r.logger.Info().
		Str("template_id", t.ID).
		Str("project_id", projectID).
		Str("author_payout_usd", install.AuthorPayoutUSD.String()).
		Msg("guild: template installed")
	return t, nil
}
