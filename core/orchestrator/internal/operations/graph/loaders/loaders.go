// Package loaders wires per-request graph-gophers/dataloader instances
// for the entities GraphQL resolvers fan out to. The dataloader pattern
// collapses N independent `*ByID` reads into one batched store call per
// request — the classic GraphQL N+1 mitigation.
//
// V22 trims the loader set to the entities the post-purge resolver
// graph actually reaches:
//
//  1. UserByID            — auth.UserStore.GetByIDs
//  2. ProjectByID         — store.Store.GetByIDs
//  3. PlanByTier          — budget.Billing.PlansByTiers (in-memory map)
//
// Each loader is constructed per-request by the middleware below; the
// cache lifetime is intentionally the request, not the process — the
// orchestrator does not have an invalidation channel for user / project
// rows so a longer-lived cache would serve stale data. The dataloader's
// in-memory cache also deduplicates identical keys inside a single
// request, which is the win that matters.
//
// To add a new loader: append a field on Loaders, build the matching
// dataloader.Loader in NewLoaders with a batch func that calls the
// underlying store's `*ByIDs` / `*ByKeys` method, and add a `FromCtx`
// helper in the resolver if you need a typed accessor.
package loaders

import (
	"context"
	"errors"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/customer/auth"
	"ironflyer/core/orchestrator/internal/business/budget"
	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/operations/store"
)

// LoaderDeps carries the underlying stores the batch functions delegate
// to. Each field is optional — nil stores yield loaders that return the
// matching not-found error per key so resolvers always see a stable
// shape regardless of orchestrator configuration.
type LoaderDeps struct {
	Users    auth.UserStore
	Projects store.Store
	Billing  *budget.Billing
	Logger   zerolog.Logger
}

// Loaders bundles every per-request dataloader the resolvers can reach.
//
// Generic type parameters: key type first, value type second. Memory
// allocation is one Loader per field per request, which is the
// graph-gophers idiom — Loader is cheap to construct (one channel +
// one cache + a goroutine spawned on first Load).
type Loaders struct {
	UserByID    *dataloader.Loader[string, auth.User]
	ProjectByID *dataloader.Loader[string, domain.Project]
	PlanByTier  *dataloader.Loader[budget.PlanTier, budget.Plan]
}

// NewLoaders constructs a fresh set of loaders. Call once per request —
// the middleware below handles that lifecycle so resolvers stay
// stateless. Each batch function is logged on error (zerolog) so the
// operator can spot a misbehaving store without a debugger.
func NewLoaders(deps LoaderDeps) *Loaders {
	logger := deps.Logger
	return &Loaders{
		UserByID: dataloader.NewBatchedLoader(
			batchUsers(deps.Users, logger),
			dataloader.WithCache[string, auth.User](dataloader.NewCache[string, auth.User]()),
		),
		ProjectByID: dataloader.NewBatchedLoader(
			batchProjects(deps.Projects, logger),
			dataloader.WithCache[string, domain.Project](dataloader.NewCache[string, domain.Project]()),
		),
		PlanByTier: dataloader.NewBatchedLoader(
			batchPlans(deps.Billing, logger),
			dataloader.WithCache[budget.PlanTier, budget.Plan](dataloader.NewCache[budget.PlanTier, budget.Plan]()),
		),
	}
}

// batchUsers wires the user-by-id batch function. A nil store falls
// back to ErrUserNotFound per key so resolvers behave the same whether
// the orchestrator booted with or without persistent auth.
func batchUsers(s auth.UserStore, logger zerolog.Logger) dataloader.BatchFunc[string, auth.User] {
	return func(ctx context.Context, ids []string) []*dataloader.Result[auth.User] {
		out := make([]*dataloader.Result[auth.User], len(ids))
		if s == nil {
			for i := range ids {
				out[i] = &dataloader.Result[auth.User]{Error: auth.ErrUserNotFound}
			}
			return out
		}
		rows, err := s.GetByIDs(ctx, ids)
		if err != nil {
			logger.Error().Err(err).Int("batch", len(ids)).Msg("loaders: user batch failed")
			for i := range ids {
				out[i] = &dataloader.Result[auth.User]{Error: err}
			}
			return out
		}
		for i, id := range ids {
			if u, ok := rows[id]; ok {
				out[i] = &dataloader.Result[auth.User]{Data: u}
			} else {
				out[i] = &dataloader.Result[auth.User]{Error: auth.ErrUserNotFound}
			}
		}
		return out
	}
}

// batchProjects wires the project-by-id batch function. The store
// interface uses domain.Project; we alias it via domain.Project so the
// dataloader type parameter stays inside the store package's surface.
func batchProjects(s store.Store, logger zerolog.Logger) dataloader.BatchFunc[string, domain.Project] {
	return func(ctx context.Context, ids []string) []*dataloader.Result[domain.Project] {
		out := make([]*dataloader.Result[domain.Project], len(ids))
		if s == nil {
			for i := range ids {
				out[i] = &dataloader.Result[domain.Project]{Error: store.ErrNotFound}
			}
			return out
		}
		rows, err := s.GetByIDs(ctx, ids)
		if err != nil {
			logger.Error().Err(err).Int("batch", len(ids)).Msg("loaders: project batch failed")
			for i := range ids {
				out[i] = &dataloader.Result[domain.Project]{Error: err}
			}
			return out
		}
		for i, id := range ids {
			if p, ok := rows[id]; ok {
				out[i] = &dataloader.Result[domain.Project]{Data: p}
			} else {
				out[i] = &dataloader.Result[domain.Project]{Error: store.ErrNotFound}
			}
		}
		return out
	}
}

// ErrPlanNotFound is returned by PlanByTier.Load when the requested
// tier is not in the orchestrator's plan catalogue. Kept here (and not
// in the budget package) because budget never errored on tier lookup
// before — the dataloader needs an explicit "missing" signal.
var ErrPlanNotFound = errors.New("plan not found")

// batchPlans wires the plan-by-tier batch function. Plans are an
// in-memory catalogue so the batch is a single slice scan — the value
// is per-request de-duplication, not network savings.
func batchPlans(b *budget.Billing, logger zerolog.Logger) dataloader.BatchFunc[budget.PlanTier, budget.Plan] {
	return func(_ context.Context, tiers []budget.PlanTier) []*dataloader.Result[budget.Plan] {
		out := make([]*dataloader.Result[budget.Plan], len(tiers))
		if b == nil {
			for i := range tiers {
				out[i] = &dataloader.Result[budget.Plan]{Error: ErrPlanNotFound}
			}
			return out
		}
		rows := b.PlansByTiers(tiers)
		for i, t := range tiers {
			if p, ok := rows[t]; ok {
				out[i] = &dataloader.Result[budget.Plan]{Data: p}
			} else {
				_ = logger // logger reserved for future probe failures
				out[i] = &dataloader.Result[budget.Plan]{Error: ErrPlanNotFound}
			}
		}
		return out
	}
}
