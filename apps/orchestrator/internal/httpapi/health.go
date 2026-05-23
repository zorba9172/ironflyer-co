package httpapi

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/go-chi/chi/v5"

	"ironflyer/apps/orchestrator/internal/audit"
	"ironflyer/apps/orchestrator/internal/memory"
	"ironflyer/apps/orchestrator/internal/projectgraph"
)

// Version is the human-friendly orchestrator build label surfaced by
// /healthz and /readyz. Bump when shipping a notable release.
const Version = "v14-finisher-polish"

// startedAt is captured at package init so the health endpoints can
// report uptime without threading state through Deps.
var startedAt = time.Now().UTC()

// probeTimeout caps every individual readiness check so a single slow
// dependency can never delay the overall response — a Kubernetes
// readiness probe is on a tight wire and a 30s hang would flap the pod.
const probeTimeout = 1 * time.Second

// RegisterHealth wires the public operational endpoints on the given
// chi router using the API's Deps. Called from api.go via one appended
// line outside any auth middleware groups so external probes
// (Kubernetes, load balancers, uptime monitors) hit them without
// credentials.
func (a *API) RegisterHealth(r chi.Router) {
	r.Get("/healthz", a.healthz)
	r.Get("/livez", a.livez)
	r.Get("/readyz", a.readyz)
	r.Get("/version", a.version)
}

// healthSnapshot probes the orchestrator's external dependencies and
// returns a JSON-ready map. Probes are intentionally cheap and non-blocking
// so the endpoint stays useful during partial outages.
func (a *API) healthSnapshot(ctx context.Context) map[string]any {
	services := map[string]bool{
		"postgres":  a.probePostgres(ctx),
		"anthropic": a.probeAnthropic(),
		"runtime":   a.probeRuntime(ctx),
		"stripe":    a.probeStripe(),
	}
	status := "ok"
	for _, ok := range services {
		if !ok {
			status = "degraded"
			break
		}
	}
	return map[string]any{
		"status":   status,
		"services": services,
		"version":  Version,
		"uptime":   time.Since(startedAt).Round(time.Second).String(),
	}
}

func (a *API) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.healthSnapshot(r.Context()))
}

// livez reports liveness. The orchestrator process is considered alive
// as soon as it can answer this request — a 200 here means "do not kill
// the pod". We deliberately avoid touching downstream dependencies so a
// transient Postgres / provider hiccup never trips a restart loop.
func (a *API) livez(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "alive",
		"uptime": time.Since(startedAt).Round(time.Second).String(),
	})
}

// readyz reports readiness to serve real traffic. Each dependency check
// runs under a 1 s timeout so a stuck backend can never wedge the probe.
// Returns 200 when every required dependency answers, 503 with a list of
// failing checks otherwise.
//
// Required: project store, at least one AI provider (BillingGuard wired).
// Optional (only checked when configured): audit store, memory store.
func (a *API) readyz(w http.ResponseWriter, r *http.Request) {
	checks := map[string]string{}
	failed := []string{}

	// Project store — List() is read-only and cheap; if it panics or
	// blocks past the timeout we consider the store unreachable.
	if a.d.Projects == nil {
		checks["projects"] = "not configured"
		failed = append(failed, "projects")
	} else if err := withTimeout(r.Context(), probeTimeout, func(_ context.Context) error {
		_ = a.d.Projects.List()
		return nil
	}); err != nil {
		checks["projects"] = err.Error()
		failed = append(failed, "projects")
	} else {
		checks["projects"] = "ok"
	}

	// AI providers — the BillingGuard is the gateway every paid model
	// call flows through. Its presence is the operational signal that at
	// least one provider has been registered on the router.
	if a.d.Guard == nil {
		checks["providers"] = "no provider registered"
		failed = append(failed, "providers")
	} else {
		checks["providers"] = "ok"
	}

	// Audit store — optional. Only checked when wired so a partial config
	// (memory-only dev orchestrator) still goes ready.
	if a.d.Audit != nil {
		if err := withTimeout(r.Context(), probeTimeout, func(ctx context.Context) error {
			// Verify() is a fast walk over the in-process hash chain for
			// the memory store; on persistent backends it issues a
			// single bounded read. Errors here mean the chain is
			// unreadable, not that the chain is broken — a broken chain
			// returns idx >= 0 with nil error and we still report ready
			// because the audit feature itself is responsive.
			_, vErr := a.d.Audit.Verify(ctx)
			return vErr
		}); err != nil {
			checks["audit"] = err.Error()
			failed = append(failed, "audit")
		} else {
			checks["audit"] = "ok"
		}
	}

	// Memory store — optional. We issue a no-op Query that returns
	// quickly on every backend; nil filter is rejected, so we scope to
	// KindUser + a sentinel userId that never exists.
	if a.d.Memory != nil {
		if err := withTimeout(r.Context(), probeTimeout, func(ctx context.Context) error {
			_, qErr := a.d.Memory.Query(ctx, memory.Query{
				Kind:   memory.KindUser,
				UserID: "__readyz_probe__",
				Limit:  1,
			})
			return qErr
		}); err != nil {
			checks["memory"] = err.Error()
			failed = append(failed, "memory")
		} else {
			checks["memory"] = "ok"
		}
	}

	body := map[string]any{
		"status": "ready",
		"checks": checks,
		"uptime": time.Since(startedAt).Round(time.Second).String(),
	}
	code := http.StatusOK
	if len(failed) > 0 {
		body["status"] = "not_ready"
		body["failed"] = failed
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, body)
}

// version returns the build identifiers stamped at link time via the
// orchestrator Makefile's -ldflags target. In a `go run` / `go build`
// dev invocation these stay at their defaults ("dev" / "unknown"),
// which is the correct signal that the binary was not produced by the
// release pipeline.
func (a *API) version(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"version":   firstNonEmpty(a.d.Version, "dev"),
		"commit":    firstNonEmpty(a.d.Commit, "unknown"),
		"buildTime": firstNonEmpty(a.d.BuildTime, "unknown"),
		"goVersion": runtime.Version(),
	})
}

// projectSnapshot returns a unified observability bundle for one project:
// gates, recent patches, memory grouped by kind, recent audit trail +
// chain verification, recent telemetry, the derived dependency graph
// summary, and the project's visual targets. Every list is capped at
// snapshotListCap to keep the payload under ~500 KiB even on busy
// projects.
const snapshotListCap = 25

func (a *API) projectSnapshot(w http.ResponseWriter, r *http.Request) {
	p, ok := a.requireProjectAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}

	// Gates — already deterministic order by domain.AllGates().
	gateStates := make([]any, 0, len(p.Gates))
	for _, g := range p.Gates {
		gateStates = append(gateStates, g)
	}
	if len(gateStates) > snapshotListCap {
		gateStates = gateStates[:snapshotListCap]
	}

	// Patches — engine returns newest-first; cap.
	patches := a.d.Patches.List(p.ID)
	if len(patches) > snapshotListCap {
		patches = patches[:snapshotListCap]
	}

	// Memory bucketed per kind. Each bucket independently capped.
	memBuckets := map[string]any{}
	if a.d.Memory != nil {
		for _, k := range memory.AllKinds() {
			ctx, cancel := context.WithTimeout(r.Context(), probeTimeout)
			rows, err := a.d.Memory.Query(ctx, memory.Query{
				Kind:      k,
				ProjectID: p.ID,
				Limit:     snapshotListCap,
			})
			cancel()
			if err != nil || len(rows) == 0 {
				memBuckets[string(k)] = []memory.Record{}
				continue
			}
			if len(rows) > snapshotListCap {
				rows = rows[:snapshotListCap]
			}
			memBuckets[string(k)] = rows
		}
	} else {
		for _, k := range memory.AllKinds() {
			memBuckets[string(k)] = []memory.Record{}
		}
	}

	// Audit — recent project entries + a chain integrity check.
	auditBundle := map[string]any{
		"recent": []audit.Entry{},
		"verifyResult": map[string]any{
			"intact":        true,
			"firstBadIndex": -1,
		},
	}
	if a.d.Audit != nil {
		ctx, cancel := context.WithTimeout(r.Context(), probeTimeout)
		rows, err := a.d.Audit.Query(ctx, audit.Query{
			ProjectID: p.ID,
			Limit:     snapshotListCap,
		})
		cancel()
		if err == nil {
			if len(rows) > snapshotListCap {
				rows = rows[:snapshotListCap]
			}
			auditBundle["recent"] = rows
		}
		ctx2, cancel2 := context.WithTimeout(r.Context(), probeTimeout)
		idx, vErr := a.d.Audit.Verify(ctx2)
		cancel2()
		if vErr == nil {
			auditBundle["verifyResult"] = map[string]any{
				"intact":        idx < 0,
				"firstBadIndex": idx,
			}
		}
	}

	// Telemetry — Recent returns newest-first; cap.
	telemetry := []any{}
	if a.d.Telemetry != nil {
		calls := a.d.Telemetry.Recent(snapshotListCap)
		if len(calls) > snapshotListCap {
			calls = calls[:snapshotListCap]
		}
		for _, c := range calls {
			telemetry = append(telemetry, c)
		}
	}

	// Graph summary — full graph can be heavy, so we only return counts
	// plus the per-language histogram. Clients that want nodes/edges hit
	// /api/projects/{id}/graph directly.
	graphCtx, cancelGraph := context.WithTimeout(r.Context(), 2*time.Second)
	graph := projectgraph.Build(graphCtx, &p)
	cancelGraph()
	languages := map[string]int{}
	for _, n := range graph.Nodes {
		languages[n.Language]++
	}

	// Visual targets — strip image bytes so the payload stays small.
	// Clients that need the bytes hit /visual-targets directly.
	targets := make([]map[string]any, 0, len(p.VisualTargets))
	for _, t := range p.VisualTargets {
		if len(targets) >= snapshotListCap {
			break
		}
		targets = append(targets, map[string]any{
			"id":        t.ID,
			"name":      t.Name,
			"routeHint": t.RouteHint,
			"viewportW": t.ViewportW,
			"viewportH": t.ViewportH,
			"tolerance": t.Tolerance,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"project": p,
		"gates":   gateStates,
		"patches": patches,
		"memory": map[string]any{
			"byKind": memBuckets,
		},
		"audit":         auditBundle,
		"telemetry":     telemetry,
		"graph": map[string]any{
			"nodes":     len(graph.Nodes),
			"edges":     len(graph.Edges),
			"languages": languages,
		},
		"visualTargets": targets,
	})
}

// probePostgres returns true when a Projects store is wired. The Memory
// driver is also a healthy state (dev mode); only an entirely-missing
// store is considered unhealthy.
func (a *API) probePostgres(_ context.Context) bool {
	return a.d.Projects != nil
}

// probeAnthropic reports the BillingGuard's readiness — the guard is the
// gateway every paid Anthropic call flows through.
func (a *API) probeAnthropic() bool {
	return a.d.Guard != nil
}

// probeRuntime pings the workspace runtime's /healthz when configured.
// A 2xx counts as healthy; anything else (incl. unreachable) is reported
// as degraded without failing the orchestrator itself.
func (a *API) probeRuntime(ctx context.Context) bool {
	if a.d.RuntimeURL == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.d.RuntimeURL+"/healthz", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// probeStripe reports whether the StripeService is wired. We avoid
// reaching out to Stripe on every health hit — their API has rate limits
// and we don't want a health probe to itself burn budget.
func (a *API) probeStripe() bool {
	return a.d.Stripe != nil
}

// withTimeout runs fn under a derived context with the supplied timeout
// and returns either fn's error or the context's deadline error,
// whichever fired first.
func withTimeout(parent context.Context, d time.Duration, fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(parent, d)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- fn(ctx) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

