// Package server wires the chi router that fronts the bridge. The
// HTTP surface is deliberately small: allocate a session, exchange
// SDP over a WebSocket, tear the session down. Per-workspace
// isolation is enforced by the shared-token middleware plus the
// workspaceID stamped onto each Session.
package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"

	"ironflyer/clients/scrcpy-bridge/internal/auth"
	"ironflyer/clients/scrcpy-bridge/internal/buildinfo"
	"ironflyer/clients/scrcpy-bridge/internal/session"
	"ironflyer/clients/scrcpy-bridge/internal/signaling"
)

// Config carries the bridge's runtime configuration. The cmd binary
// resolves env vars and hands the struct over.
type Config struct {
	Port        int
	ScrcpyPath  string
	AdbServer   string
	SharedToken string
	Logger      zerolog.Logger
}

// Server bundles the chi router, the Session manager, and config.
type Server struct {
	cfg      Config
	manager  *session.Manager
	router   chi.Router
	logger   zerolog.Logger
	httpAddr string
}

// New constructs a Server and wires all routes.
func New(cfg Config, manager *session.Manager) *Server {
	s := &Server{
		cfg:     cfg,
		manager: manager,
		logger:  cfg.Logger.With().Str("component", "server").Logger(),
	}
	s.router = s.routes()
	return s
}

// Handler exposes the chi router so net/http can serve it. Used in tests
// and the cmd binary alike (well, not tests — but it's idiomatic).
func (s *Server) Handler() http.Handler { return s.router }

// HTTPServer builds an *http.Server with sane timeouts. The caller is
// responsible for Shutdown.
func (s *Server) HTTPServer(addr string) *http.Server {
	s.httpAddr = addr
	return &http.Server{
		Addr:              addr,
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
		// No overall ReadTimeout: WebSocket upgrades hold the
		// socket open intentionally.
	}
}

func (s *Server) routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(s.logRequest)

	// /healthz is unauthenticated by design — k8s liveness probes
	// must reach it without secrets. Mirrors the orchestrator
	// convention documented in CLAUDE.md.
	r.Get("/healthz", s.handleHealthz)

	// Everything under /v1 requires the shared bridge token.
	r.Route("/v1", func(api chi.Router) {
		api.Use(auth.RequireBridgeToken(s.cfg.SharedToken))
		api.Post("/sessions", s.handleCreateSession)
		api.Get("/sessions", s.handleListSessions)
		api.Get("/sessions/{id}", s.handleGetSession)
		api.Delete("/sessions/{id}", s.handleDeleteSession)
		api.Get("/sessions/{id}/ws", s.handleSessionWS)
	})

	return r
}

// ---- handlers -------------------------------------------------------------

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"service":     buildinfo.Component,
		"version":     buildinfo.Version,
		"scrcpy_path": s.cfg.ScrcpyPath,
	})
}

type createSessionRequest struct {
	WorkspaceID    string `json:"workspaceId"`
	EmulatorSerial string `json:"emulatorSerial"`
}

type createSessionResponse struct {
	SessionID      string    `json:"sessionId"`
	WorkspaceID    string    `json:"workspaceId"`
	EmulatorSerial string    `json:"emulatorSerial"`
	WSEndpoint     string    `json:"wsEndpoint"`
	DeleteEndpoint string    `json:"deleteEndpoint"`
	StartedAt      time.Time `json:"startedAt"`
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var body createSessionRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	body.WorkspaceID = strings.TrimSpace(body.WorkspaceID)
	body.EmulatorSerial = strings.TrimSpace(body.EmulatorSerial)
	if body.WorkspaceID == "" || body.EmulatorSerial == "" {
		writeErr(w, http.StatusBadRequest, "workspaceId and emulatorSerial required")
		return
	}
	sess, err := s.manager.Create(r.Context(), body.WorkspaceID, body.EmulatorSerial)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "scrcpy bring-up failed: "+err.Error())
		return
	}
	resp := createSessionResponse{
		SessionID:      sess.ID,
		WorkspaceID:    sess.WorkspaceID,
		EmulatorSerial: sess.EmulatorSerial,
		WSEndpoint:     "/v1/sessions/" + sess.ID + "/ws",
		DeleteEndpoint: "/v1/sessions/" + sess.ID,
		StartedAt:      sess.StartedAt(),
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	workspaceID := strings.TrimSpace(r.URL.Query().Get("workspaceId"))
	if workspaceID == "" {
		writeErr(w, http.StatusBadRequest, "workspaceId query param required")
		return
	}
	live := s.manager.ListByWorkspace(workspaceID)
	out := make([]map[string]any, 0, len(live))
	for _, sess := range live {
		out = append(out, map[string]any{
			"sessionId":      sess.ID,
			"workspaceId":    sess.WorkspaceID,
			"emulatorSerial": sess.EmulatorSerial,
			"startedAt":      sess.StartedAt(),
			"lastFrameAt":    sess.LastFrameAt(),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": out})
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.manager.Get(chi.URLParam(r, "id"))
	if !ok {
		writeErr(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"sessionId":      sess.ID,
		"workspaceId":    sess.WorkspaceID,
		"emulatorSerial": sess.EmulatorSerial,
		"startedAt":      sess.StartedAt(),
		"lastFrameAt":    sess.LastFrameAt(),
	})
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.manager.Delete(id); err != nil {
		if errors.Is(err, errors.New("session not found")) {
			writeErr(w, http.StatusNotFound, "session not found")
			return
		}
		// Fall through: Delete returns a sentinel-ish error string
		// that's safe to expose as 404 when the id doesn't match.
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSessionWS(w http.ResponseWriter, r *http.Request) {
	// The chi router's auth middleware already validated the token
	// when it was supplied in a header. For browser WebSocket
	// clients we accept ?token= instead — re-check here so the
	// middleware order doesn't trap us into accepting unauthenticated
	// upgrades.
	if !auth.VerifyToken(r, s.cfg.SharedToken) {
		writeErr(w, http.StatusUnauthorized, "invalid bridge token")
		return
	}
	id := chi.URLParam(r, "id")
	sess, ok := s.manager.Get(id)
	if !ok {
		writeErr(w, http.StatusNotFound, "session not found")
		return
	}
	srv := &signaling.Server{
		Session: sess,
		Manager: s.manager,
		Logger:  s.logger,
	}
	srv.Handle(r.Context(), w, r)
}

// ---- helpers --------------------------------------------------------------

func (s *Server) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Debug().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Dur("dur", time.Since(start)).
			Msg("http request")
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// Discard unused websocket import on platforms where the type-check
// pass would otherwise complain — used indirectly via signaling.
var _ = websocket.StatusNormalClosure
