package resolver

// Wired by Closure Agent P. Trivial sentinel surfaces: ping, version,
// _empty, _heartbeat. Version pulls from the package-level build vars
// set via -ldflags in cmd/orchestrator/main.go; when those defaults
// remain we surface "dev" / "unknown" so the dashboard footer is
// honest about the build identity.

import (
	"context"
	"os"
	"time"

	"ironflyer/apps/orchestrator/internal/graph/generated"
	"ironflyer/apps/orchestrator/internal/graph/model"
)

// Build identifiers surfaced by the `version` resolver. Operators can
// populate these via env (preferred for the resolver pkg, since the
// resolver can't see cmd/orchestrator's -ldflags vars) or via
// SetBuildInfo from main. Defaults keep /version honest in `go run`.
// TODO: when the linker -ldflags wire reaches this package, hoist
// these vars to compile-time symbols so env reads become optional.
var (
	buildServiceName = "ironflyer-orchestrator"
	buildVersion     = "dev"
	buildCommit      = "unknown"
	buildTime        = "unknown"
)

func init() {
	if v := os.Getenv("IRONFLYER_BUILD_VERSION"); v != "" {
		buildVersion = v
	}
	if v := os.Getenv("IRONFLYER_BUILD_COMMIT"); v != "" {
		buildCommit = v
	}
	if v := os.Getenv("IRONFLYER_BUILD_TIME"); v != "" {
		buildTime = v
	}
	if v := os.Getenv("IRONFLYER_SERVICE_NAME"); v != "" {
		buildServiceName = v
	}
}

// SetBuildInfo lets the binary entrypoint inject the link-time build
// identifiers so the GraphQL `version` resolver can render them.
// Safe to call before the resolver is mounted; no synchronisation
// required because main runs single-threaded at startup.
func SetBuildInfo(service, version, commit, builtAt string) {
	if service != "" {
		buildServiceName = service
	}
	if version != "" {
		buildVersion = version
	}
	if commit != "" {
		buildCommit = commit
	}
	if builtAt != "" {
		buildTime = builtAt
	}
}

// Empty is the resolver for the _empty field.
func (r *mutationResolver) Empty(ctx context.Context) (*string, error) {
	s := ""
	return &s, nil
}

// Ping is the resolver for the ping field.
func (r *queryResolver) Ping(ctx context.Context) (string, error) {
	return "ok", nil
}

// Version is the resolver for the version field.
func (r *queryResolver) Version(ctx context.Context) (*model.VersionInfo, error) {
	return &model.VersionInfo{
		Service:   buildServiceName,
		Version:   buildVersion,
		Commit:    buildCommit,
		BuildTime: buildTime,
	}, nil
}

// Heartbeat is the resolver for the _heartbeat field. Emits one tick
// per 15s so clients can keep a websocket warm without subscribing to
// a domain-specific feed. Closes when the context cancels.
func (r *subscriptionResolver) Heartbeat(ctx context.Context) (<-chan *model.HeartbeatEvent, error) {
	out := make(chan *model.HeartbeatEvent, 1)
	go func() {
		defer close(out)
		// Send an immediate tick so the client confirms the
		// subscription is live without waiting one full interval.
		first := "alive"
		select {
		case out <- &model.HeartbeatEvent{Ts: time.Now().UTC(), Message: &first}:
		case <-ctx.Done():
			return
		}
		t := time.NewTicker(15 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case ts := <-t.C:
				msg := "alive"
				select {
				case out <- &model.HeartbeatEvent{Ts: ts.UTC(), Message: &msg}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out, nil
}

// Mutation returns generated.MutationResolver implementation.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

// Subscription returns generated.SubscriptionResolver implementation.
func (r *Resolver) Subscription() generated.SubscriptionResolver { return &subscriptionResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
type subscriptionResolver struct{ *Resolver }
