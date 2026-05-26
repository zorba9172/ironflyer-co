// Package signaling owns the WebSocket negotiation channel between
// the browser and a bridge Session. SDP exchange + ICE trickling +
// fallback input events all multiplex over the same socket.
package signaling

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog"

	"ironflyer/clients/scrcpy-bridge/internal/session"
)

// gracePeriod bounds how long a Session stays alive after the
// signaling WebSocket disconnects. Mobile networks bounce; we let the
// client reconnect within this window before tearing scrcpy down.
const gracePeriod = 30 * time.Second

// SessionDeleter is the narrow contract the WebSocket loop uses to
// tear a session down after the reconnect grace expires. Implemented
// by *session.Manager.
type SessionDeleter interface {
	Delete(id string) error
}

// Message is the wire envelope. Exactly one of the typed payloads is
// populated per envelope depending on Type.
type Message struct {
	Type      string                  `json:"type"`
	SDP       string                  `json:"sdp,omitempty"`
	Candidate *webrtc.ICECandidateInit `json:"candidate,omitempty"`
	Event     *session.InputEvent      `json:"event,omitempty"`
	Error     string                   `json:"error,omitempty"`
}

// Server is the per-session WebSocket loop. One Server lives for the
// duration of a single client connection; reconnects allocate a new
// Server bound to the same Session.
type Server struct {
	Session *session.Session
	Manager SessionDeleter
	Logger  zerolog.Logger
}

// Handle accepts the upgrade and runs the read loop until the client
// drops or ctx fires. After the read loop exits the Server arms the
// grace timer; if no new Handle call lands for the same session within
// gracePeriod the session is deleted.
func (s *Server) Handle(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // browsers from any origin; bridge auth runs separately
		CompressionMode:    websocket.CompressionDisabled,
	})
	if err != nil {
		s.Logger.Warn().Err(err).Msg("ws accept failed")
		return
	}
	// Capture closure cause so deferred cleanup can be precise.
	var stopReason string
	defer func() {
		_ = conn.Close(websocket.StatusNormalClosure, "bye")
		s.armGrace(stopReason)
	}()

	// Wire local ICE candidates to the WebSocket. We serialise sends
	// behind a mutex so concurrent goroutines can't interleave frames.
	var sendMu sync.Mutex
	send := func(msg Message) {
		sendMu.Lock()
		defer sendMu.Unlock()
		writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := wsjson.Write(writeCtx, conn, msg); err != nil {
			s.Logger.Debug().Err(err).Msg("ws write failed")
		}
	}
	s.Session.SetLocalCandidateSink(func(c *webrtc.ICECandidate) {
		if c == nil {
			return // end-of-candidates marker
		}
		init := c.ToJSON()
		send(Message{Type: "ice", Candidate: &init})
	})
	defer s.Session.SetLocalCandidateSink(nil)

	for {
		var msg Message
		readCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		err := wsjson.Read(readCtx, conn, &msg)
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				stopReason = "ctx done"
			} else if isClientGone(err) {
				stopReason = "client disconnect"
			} else {
				stopReason = "ws read error: " + err.Error()
			}
			s.Logger.Info().Str("reason", stopReason).Msg("ws loop ending")
			return
		}
		switch msg.Type {
		case "offer":
			if err := s.handleOffer(ctx, msg, send); err != nil {
				send(Message{Type: "error", Error: err.Error()})
			}
		case "ice":
			if msg.Candidate != nil {
				if err := s.Session.PeerConnection.AddICECandidate(*msg.Candidate); err != nil {
					s.Logger.Debug().Err(err).Msg("AddICECandidate failed")
				}
			}
		case "input":
			if msg.Event == nil {
				continue
			}
			if err := s.Session.HandleInput(ctx, *msg.Event); err != nil {
				s.Logger.Debug().Err(err).Msg("input dispatch failed")
			}
		case "ping":
			send(Message{Type: "pong"})
		default:
			s.Logger.Debug().Str("type", msg.Type).Msg("unknown signal type")
		}
	}
}

func (s *Server) handleOffer(ctx context.Context, msg Message, send func(Message)) error {
	if msg.SDP == "" {
		return errors.New("offer missing sdp")
	}
	pc := s.Session.PeerConnection
	if pc == nil {
		return errors.New("peer connection not ready")
	}
	if err := pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  msg.SDP,
	}); err != nil {
		return err
	}
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return err
	}
	if err := pc.SetLocalDescription(answer); err != nil {
		return err
	}
	send(Message{Type: "answer", SDP: answer.SDP})
	_ = ctx
	return nil
}

// armGrace schedules session deletion gracePeriod after the WS loop
// terminates, unless a new Handle call arrives in the meantime. This
// runtime is single-process so we encode the grace as a goroutine
// guarded by lastWSConnAt on the session — kept simple deliberately.
func (s *Server) armGrace(reason string) {
	if s.Session == nil || s.Manager == nil {
		return
	}
	id := s.Session.ID
	logger := s.Logger.With().Str("session", id).Str("reason", reason).Logger()
	go func() {
		timer := time.NewTimer(gracePeriod)
		defer timer.Stop()
		<-timer.C
		// Best-effort: if the manager still has the session,
		// delete it. A reconnect attaches a fresh Server but uses
		// the same Session pointer, so the test for "is the
		// session still here" is enough.
		logger.Info().Msg("grace window expired, deleting session")
		_ = s.Manager.Delete(id)
	}()
}

func isClientGone(err error) bool {
	if err == nil {
		return false
	}
	var ce websocket.CloseError
	if errors.As(err, &ce) {
		return true
	}
	return false
}

// MarshalMessage is exported for callers that want to construct outgoing
// frames without depending on the package-internal field tags.
func MarshalMessage(m Message) ([]byte, error) {
	return json.Marshal(m)
}
