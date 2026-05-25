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
	"net/http"
	"os"
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

	"ironflyer/apps/orchestrator/internal/auth"
	"ironflyer/apps/orchestrator/internal/graph/generated"
	"ironflyer/apps/orchestrator/internal/graph/resolver"
	"ironflyer/apps/orchestrator/internal/gqlhardening"
	"ironflyer/apps/orchestrator/internal/policy"
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

	if graphEnv() != "prod" || envBool("GRAPHQL_INTROSPECTION") {
		srv.Use(extension.Introspection{})
	}
	if limit := envInt("GRAPHQL_COMPLEXITY_LIMIT", 500); limit > 0 {
		srv.Use(extension.FixedComplexityLimit(limit))
	}
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](envInt("GRAPHQL_APQ_LRU", 1000)),
	})

	// V22 hardening: depth + complexity + introspection gate + redacted
	// error presenter. The middleware order matters — the extensions
	// run inside the gqlgen handler; CSRF / persisted-queries /
	// rate-limit run outside in api.go.
	if cfg.Hardening != nil {
		h := *cfg.Hardening
		if h.MaxDepth > 0 {
			srv.Use(gqlhardening.DepthExtension(h.MaxDepth))
		}
		if h.ComplexityLimit > 0 {
			srv.Use(gqlhardening.ComplexityExtension(h.ComplexityLimit, nil))
		}
		srv.Use(gqlhardening.IntrospectionGate(h.ProdMode, cfg.IsOperator))
		srv.SetErrorPresenter(gqlhardening.RedactedErrorPresenter(h.ProdMode))
	}
	if cfg.PolicyPEP != nil {
		srv.AroundOperations(policy.GraphQLOperationMiddleware(cfg.PolicyPEP))
	}

	if strings.EqualFold(strings.TrimSpace(os.Getenv("GRAPHQL_TRACING")), "on") {
		srv.Use(apollotracing.Tracer{})
	}
	_ = graphql.OperationContext{} // keep graphql import referenced for older toolchains

	srv.SetRecoverFunc(func(ctx context.Context, err any) error {
		cfg.Logger.Error().Interface("panic", err).Msg("graphql: panic in resolver")
		return gqlerror.Errorf("internal server error")
	})
	srv.SetErrorPresenter(func(ctx context.Context, err error) *gqlerror.Error {
		var gqlErr *gqlerror.Error
		if errors.As(err, &gqlErr) {
			return gqlErr
		}
		cfg.Logger.Warn().Err(err).Msg("graphql: resolver error")
		if graphEnv() != "prod" || envBool("GRAPHQL_ERROR_DETAILS") {
			return &gqlerror.Error{Message: err.Error()}
		}
		return &gqlerror.Error{Message: "internal server error"}
	})

	return srv
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
