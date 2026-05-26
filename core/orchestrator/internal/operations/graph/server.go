// Package graph wires the gqlgen runtime — exec schema, transports,
// extensions, sandbox HTML — into a single chi-mountable handler.
//
// Endpoints (all served by Handler):
//
//	POST /graphql          — queries + mutations (JSON)
//	GET  /graphql          — queries via APQ / persisted-query GETs
//	WS   /graphql          — subscriptions over `graphql-transport-ws`
//
// Auth: HTTP requests inherit the chi auth middleware (Authorization
// Bearer). WebSocket subscriptions read the JWT from the `connection_init`
// payload (key `authorization` or `Authorization`); a `?token=` query
// string fallback is honoured for browsers that can't easily set
// connection_init.
package graph

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/apollotracing"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"ironflyer/core/orchestrator/internal/customer/auth"
	"ironflyer/core/orchestrator/internal/operations/graph/generated"
	"ironflyer/core/orchestrator/internal/operations/graph/resolver"
	"ironflyer/core/orchestrator/internal/operations/gqlhardening"
	"ironflyer/core/orchestrator/internal/operations/policy"
	"ironflyer/core/orchestrator/internal/operations/sentryext"
)

// Config is the input wiring the orchestrator passes when constructing
// the GraphQL server. Required fields panic at build time when missing
// (caught by the caller in api.go).
type Config struct {
	Resolver *resolver.Resolver
	Auth     *auth.Service
	Logger   zerolog.Logger

	// Hardening is the V22 Wave-2 production gate set: depth +
	// complexity caps, introspection gate, redacted error presenter.
	// Zero-value installs the gqlhardening defaults; pass nil to skip.
	Hardening *gqlhardening.Config

	// IsOperator decides whether a request principal has operator
	// rights (introspection allowed even in prod mode, persisted-query
	// registration allowed, etc.). Optional — when nil the gate
	// defaults to "no one is operator" in prod and "everyone" in dev.
	IsOperator func(ctx context.Context) bool

	// PolicyPEP is the V22 OPA-backed policy enforcement point. When
	// non-nil every operation runs through GraphQLOperationMiddleware
	// before the resolver fires.
	PolicyPEP *policy.PEP
}

// Handler returns the configured GraphQL HTTP handler. Mount the result
// under the orchestrator's authenticated route group so HTTP requests
// already have the user on context; WebSocket subscriptions resolve
// the user themselves via the InitFunc below.
func Handler(cfg Config) http.Handler {
	es := generated.NewExecutableSchema(generated.Config{Resolvers: cfg.Resolver})
	srv := handler.New(es)

	// Order matters: GET first for APQ + introspection, then JSON POST,
	// then the modern WebSocket subscriber. The multipart / forms /
	// SSE transports are mounted so file uploads + EventSource fallback
	// work even though we standardize on POST + WS.
	srv.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
		Upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			// CORS is enforced by the chi `corsMiddleware` already; the
			// HTTP Origin is allowed through here so the upgrade
			// handshake succeeds. The actual scope check happens in the
			// authMiddleware around the route.
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		InitFunc: makeWebsocketInitFunc(cfg.Auth, cfg.Logger),
	})
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.MultipartForm{})

	srv.SetQueryCache(lru.New[*ast.QueryDocument](envInt("GRAPHQL_QUERY_CACHE_LRU", 1000)))

	// V22 hardening — per-extension logger. The depth + complexity
	// extensions log every reject at WARN with the operation name +
	// measured metric so operators can tune the caps against real
	// traffic before they bite a legitimate dashboard query.
	gqlLogger := cfg.Logger.With().Str("subsystem", "graphql.hardening").Logger()

	// V22 Wave-2 hardening law #8: depth + complexity caps are
	// ALWAYS-ON, regardless of prod vs dev. They are cheap, protect the
	// dev surface from the same fragment-cycle / quadratic-blowup DoS
	// patterns that hit prod, and let dev tooling fail fast on shapes
	// that would later trip the production gate. When the caller does
	// not supply a Hardening config we fall back to the package
	// defaults so the caps still mount.
	hardening := gqlhardening.Defaults()
	if cfg.Hardening != nil {
		hardening = *cfg.Hardening
	}
	if hardening.MaxDepth <= 0 {
		hardening.MaxDepth = gqlhardening.Defaults().MaxDepth
	}
	if hardening.ComplexityLimit <= 0 {
		hardening.ComplexityLimit = gqlhardening.Defaults().ComplexityLimit
	}
	// DEPLOY.md §5 documents these three knobs directly on the
	// GRAPHQL_* prefix; honor them here even when the caller passes a
	// Hardening config that predates the env contract (gqlhardening.Load
	// already covers the same vars but a caller may build a Config
	// manually).
	if v := envInt("GRAPHQL_DEPTH_LIMIT", 0); v > 0 {
		hardening.MaxDepth = v
	}
	if v := envInt("GRAPHQL_COMPLEXITY_LIMIT", 0); v > 0 {
		hardening.ComplexityLimit = v
	}
	if envBool("GRAPHQL_APQ_LOCKED") {
		hardening.APQLocked = true
	}
	if v := strings.TrimSpace(os.Getenv("GRAPHQL_APQ_REGISTRY_DIR")); v != "" {
		hardening.APQRegistryDir = v
	}

	// Introspection: ON in dev (web codegen needs it), OFF in prod
	// unless the operator explicitly flips GRAPHQL_INTROSPECTION=on.
	// The decision is gated on hardening.ProdMode so the toggle stays
	// aligned with the rest of the hardening profile.
	if !hardening.ProdMode || envBool("GRAPHQL_INTROSPECTION") {
		srv.Use(extension.Introspection{})
	}

	// APQ wiring — when GRAPHQL_APQ_LOCKED=true the upstream
	// AutomaticPersistedQuery extension is replaced by gqlhardening's
	// LockedAPQ, which serves only pre-registered hashes from the
	// startup-seeded registry. Operators (per cfg.IsOperator) bypass
	// the lock so Sandbox / CLI still ship ad-hoc queries through the
	// surface.
	if hardening.APQLocked {
		registry := gqlhardening.NewRegistryCache(true)
		regDir := hardening.APQRegistryDir
		if regDir == "" {
			regDir = "clients/web/src/lib/gql/operations"
		}
		seeded, err := gqlhardening.SeedRegistryFromDir(registry, regDir, &gqlLogger)
		if err != nil {
			cfg.Logger.Warn().Err(err).Str("dir", regDir).Msg("graphql: APQ registry seed failed — locked mode will reject ALL requests")
		}
		cfg.Logger.Info().
			Bool("locked", true).
			Str("dir", regDir).
			Int("seeded", seeded).
			Int("registry_size", registry.Len()).
			Msg("graphql: APQ locked mode wired")
		srv.Use(gqlhardening.NewLockedAPQ(registry, &gqlLogger, gqlhardening.OperatorCheck(cfg.IsOperator)))
	} else {
		srv.Use(extension.AutomaticPersistedQuery{
			Cache: lru.New[string](envInt("GRAPHQL_APQ_LRU", 1000)),
		})
	}

	// V22 hardening extensions — depth + complexity + introspection
	// gate + redacted error presenter. Depth and complexity ALWAYS
	// mount; the introspection gate is a no-op in dev and the redacted
	// presenter is overwritten below by the dev-friendly presenter when
	// we're not in prod mode.
	srv.Use(gqlhardening.DepthExtension(hardening.MaxDepth, &gqlLogger))
	srv.Use(gqlhardening.ComplexityExtension(hardening.ComplexityLimit, nil, &gqlLogger))
	srv.Use(gqlhardening.IntrospectionGate(hardening.ProdMode, cfg.IsOperator))
	if hardening.ProdMode {
		srv.SetErrorPresenter(gqlhardening.RedactedErrorPresenter(hardening.ProdMode))
	}
	if cfg.PolicyPEP != nil {
		srv.AroundOperations(policy.GraphQLOperationMiddleware(cfg.PolicyPEP))
	}

	if strings.EqualFold(strings.TrimSpace(os.Getenv("GRAPHQL_TRACING")), "on") {
		srv.Use(apollotracing.Tracer{})
	}
	_ = graphql.OperationContext{} // keep graphql import referenced for older toolchains

	srv.SetRecoverFunc(func(ctx context.Context, err any) error {
		// Render the panic value as a readable string and capture the
		// stacktrace so the operator can see exactly which resolver
		// blew up. zerolog's Interface() serialises empty for a typed
		// error / runtime panic struct, which is why the original
		// "panic={}" log lines were useless during the V22 studio
		// debugging session.
		stack := debug.Stack()
		cfg.Logger.Error().
			Str("panic_value", fmt.Sprintf("%+v", err)).
			Str("panic_type", fmt.Sprintf("%T", err)).
			Str("stack", string(stack)).
			Msg("graphql: panic in resolver")
		sentryext.CaptureRecovered(ctx, err)
		return gqlerror.Errorf("internal server error")
	})
	// In prod we already mounted the redacted presenter above. Keep the
	// dev-friendly presenter (logs + full error string back to the
	// client) only when we are not in prod, so the operator sees real
	// errors during local development and the prod surface stays masked.
	if !hardening.ProdMode {
		srv.SetErrorPresenter(func(ctx context.Context, err error) *gqlerror.Error {
			var gqlErr *gqlerror.Error
			if errors.As(err, &gqlErr) {
				captureGraphQLError(ctx, gqlErr)
				return gqlErr
			}
			cfg.Logger.Warn().Err(err).Msg("graphql: resolver error")
			captureGraphQLError(ctx, err)
			return &gqlerror.Error{Message: err.Error()}
		})
	} else {
		// Prod: keep the redacted presenter mounted above, but wrap it so
		// the resolver error still ships to Sentry before the redaction
		// erases the original message from the wire response.
		inner := gqlhardening.RedactedErrorPresenter(hardening.ProdMode)
		srv.SetErrorPresenter(func(ctx context.Context, err error) *gqlerror.Error {
			captureGraphQLError(ctx, err)
			return inner(ctx, err)
		})
	}

	return srv
}

// captureGraphQLError ships a resolver-side error to Sentry tagged
// with the GraphQL operation name and (when present) the authenticated
// user id, so dashboards can group resolver failures by operation
// instead of by stack trace. No-op when Sentry DSN is unset.
func captureGraphQLError(ctx context.Context, err error) {
	if err == nil {
		return
	}
	tags := map[string]string{}
	if oc := graphql.GetOperationContext(ctx); oc != nil {
		if oc.OperationName != "" {
			tags["graphql.operation"] = oc.OperationName
		} else if oc.Operation != nil && oc.Operation.Name != "" {
			tags["graphql.operation"] = oc.Operation.Name
		}
	}
	if u, ok := auth.FromContext(ctx); ok && u.ID != "" {
		tags["user.id"] = u.ID
	}
	sentryext.CaptureError(ctx, err, tags)
}

func graphEnv() string {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("IRONFLYER_ENV")))
	if env == "" {
		return "dev"
	}
	return env
}

func envBool(key string) bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return raw == "1" || raw == "true" || raw == "on" || raw == "yes"
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}

// makeWebsocketInitFunc returns a transport.WebsocketInitFunc that
// verifies the bearer JWT from connection_init (or ?token= query
// fallback) and stamps the user on ctx so resolvers can use
// auth.FromContext exactly like the HTTP path.
func makeWebsocketInitFunc(svc *auth.Service, logger zerolog.Logger) transport.WebsocketInitFunc {
	return func(ctx context.Context, initPayload transport.InitPayload) (context.Context, *transport.InitPayload, error) {
		if svc == nil {
			// Dev mode (AuthOptional) — pass through without a user.
			return ctx, nil, nil
		}
		var raw string
		if v, ok := initPayload["authorization"].(string); ok {
			raw = v
		} else if v, ok := initPayload["Authorization"].(string); ok {
			raw = v
		}
		raw = strings.TrimSpace(raw)
		const prefix = "Bearer "
		if len(raw) > len(prefix) && strings.EqualFold(raw[:len(prefix)], prefix) {
			raw = strings.TrimSpace(raw[len(prefix):])
		}
		if raw == "" {
			// Fall back to the ?token= query string the SSE path
			// already honours. The token is recovered from the parent
			// HTTP request context if the gqlgen transport copied it
			// over (no public accessor — best effort).
			if t, ok := tokenFromContext(ctx); ok {
				raw = t
			}
		}
		if raw == "" {
			return ctx, nil, errors.New("missing authorization in connection_init payload")
		}
		u, err := svc.Verify(ctx, raw)
		if err != nil {
			logger.Warn().Err(err).Msg("graphql ws: token verify failed")
			return ctx, nil, errors.New("invalid authorization")
		}
		return auth.WithUser(ctx, u), nil, nil
	}
}

// tokenFromContext is a small escape hatch that lets the websocket
// transport read a token the chi layer put on ctx for the `?token=`
// fallback. The httpapi package writes this value via a dedicated
// chi middleware in front of the GraphQL handler when it sees a
// `token` query string parameter.
type tokenCtxKey struct{}

// WithToken stamps a raw JWT on ctx; the websocket InitFunc reads it
// when connection_init.authorization is empty.
func WithToken(ctx context.Context, token string) context.Context {
	if token == "" {
		return ctx
	}
	return context.WithValue(ctx, tokenCtxKey{}, token)
}

func tokenFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(tokenCtxKey{}).(string)
	return v, ok && v != ""
}

// Sandbox returns a tiny HTML page that loads the Apollo Sandbox embed
// against the orchestrator's /graphql endpoint. Mounted at
// GET /graphql/sandbox by api.go. The endpoint is intentionally public
// so it works without a token; the user pastes their token into the
// sandbox's HTTP-headers tab.
//
// TODO: self-host the embed JS so the docs surface stays available even
// when the customer's network blocks Apollo's CDN.
func Sandbox(endpoint string) http.Handler {
	if endpoint == "" {
		endpoint = "/graphql"
	}
	html := strings.ReplaceAll(sandboxHTML, "__ENDPOINT__", endpoint)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(html))
	})
}

const sandboxHTML = `<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <title>Ironflyer — GraphQL Sandbox</title>
  <style>html,body,#embedded-sandbox{height:100%;margin:0;padding:0;background:#0e0e0e;color:#f5f1e6;font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace}</style>
</head>
<body>
  <div id="embedded-sandbox"></div>
  <script src="https://embeddable-sandbox.cdn.apollographql.com/_latest/embeddable-sandbox.umd.production.min.js"></script>
  <script>
    new window.EmbeddedSandbox({
      target: '#embedded-sandbox',
      initialEndpoint: '__ENDPOINT__',
      includeCookies: false,
      hideCookieToggle: false,
    });
  </script>
</body>
</html>`
