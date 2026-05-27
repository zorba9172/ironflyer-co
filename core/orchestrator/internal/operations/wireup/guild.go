package wireup

// V22 FinisherGuild wireup.
//
// Responsibility split with main.go:
//
//   - main.go decides backend (Postgres vs in-memory) and gates the
//     feature on IRONFLYER_GUILD_ENABLED.
//   - WireGuild here assembles Coordinator + GateFailureRouter +
//     Reconciler from the supplied dependencies and returns one
//     bundle so the call site stays a single line.
//   - main.go is responsible for starting the Reconciler (its ctx /
//     supervisor lifecycle differ per binary).
//   - main.go is responsible for installing the GateFailureRouter's
//     Observe callback on the learning.Publisher — we leave a small
//     ObserverHook on the bundle so the wireup is one line at the
//     call site.

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/ai/learning"
	"ironflyer/core/orchestrator/internal/business/guild"
	"ironflyer/core/orchestrator/internal/business/wallet"
	"ironflyer/core/orchestrator/internal/operations/store"
)

// GuildBundle holds everything the resolver / boot path needs.
type GuildBundle struct {
	Service     guild.Service
	Escrow      *guild.Escrow
	Payouts     *guild.Payouts
	Templates   *guild.TemplateRegistry
	Coordinator *guild.Coordinator
	Router      *guild.GateFailureRouter
	Reconciler  *guild.Reconciler
}

// GuildOpts is the wireup input. WalletSvc may be nil (escrow no-ops);
// pool may be nil (memory backend); ProjectStore is required for the
// router's tenant lookup.
type GuildOpts struct {
	Pool         *pgxpool.Pool
	WalletSvc    wallet.Service
	ProjectStore store.Store
	Logger       zerolog.Logger
}

// WireGuild constructs the bundle. Returns nil when GuildOpts.Logger
// is zero — main.go branches on the bundle so the rest of the stack
// stays untouched when the feature is disabled.
func WireGuild(opts GuildOpts) *GuildBundle {
	var svc guild.Service
	if opts.Pool != nil {
		svc = guild.NewPostgresService(opts.Pool)
		opts.Logger.Info().Msg("V22 guild: Postgres backend")
	} else {
		svc = guild.NewMemoryService()
		opts.Logger.Info().Msg("V22 guild: in-memory backend")
	}
	escrow := guild.NewEscrow(opts.WalletSvc)
	payouts := guild.NewPayouts(svc, opts.Logger)
	templates := guild.NewTemplateRegistry(svc, escrow, opts.Logger)
	coord := guild.NewCoordinator(svc, escrow, payouts, templates, opts.Logger)
	router := guild.NewGateFailureRouter(svc, escrow, projectLookup{opts.ProjectStore}, opts.Logger)
	// Sensible default thresholds — wildcard 3, code gate 2 (build
	// failures are the highest-value handoff to a finisher).
	router.Register("*", 3)
	router.Register("code", 2)
	reconciler := guild.NewReconciler(svc, escrow, guild.ReconcilerOpts{Logger: opts.Logger})
	return &GuildBundle{
		Service:     svc,
		Escrow:      escrow,
		Payouts:     payouts,
		Templates:   templates,
		Coordinator: coord,
		Router:      router,
		Reconciler:  reconciler,
	}
}

// AttachRouterObserver wires the router's Observe callback as a fan-
// out of the learning.Publisher's single observer slot. The prior
// observer (the in-memory store's projection) is preserved — both
// callbacks fire on every Publish.
//
// Call this AFTER any other SetObserver call in main.go so we do not
// silently replace the store's projection. The shim runs each
// callback on its own goroutine — the publisher already isolates
// observer failures.
func AttachRouterObserver(pub *learning.Publisher, router *guild.GateFailureRouter, prior func(learning.OutcomeEvent)) {
	if pub == nil || router == nil {
		return
	}
	pub.SetObserver(func(evt learning.OutcomeEvent) {
		if prior != nil {
			prior(evt)
		}
		router.Observe(evt)
	})
}

// projectLookup adapts store.Store to the narrow guild.ProjectLookup
// interface so the router does not import the projects package
// directly.
type projectLookup struct{ s store.Store }

// TenantForProject returns the project owner id as the tenant. Public
// projects (no owner) return ("", false) so the router skips auto-
// task creation for demos.
func (p projectLookup) TenantForProject(_ context.Context, projectID string) (string, bool) {
	if p.s == nil {
		return "", false
	}
	proj, err := p.s.Get(projectID)
	if err != nil {
		return "", false
	}
	if proj.OwnerID == "" {
		return "", false
	}
	return proj.OwnerID, true
}
