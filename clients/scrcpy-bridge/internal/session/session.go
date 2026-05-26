package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/rs/zerolog"
)

// frameDuration is the wall-clock interval assigned to each H.264 NAL
// emitted to Pion. Real scrcpy frames carry no timing in the Annex-B
// stream, so we pace at 30fps which matches scrcpy's default capture
// rate. Pion handles RTP timestamping internally from this hint.
const frameDuration = 33 * time.Millisecond

// dataChannelLabel is the WebRTC data-channel name used for input
// events. The frontend opens the matching label.
const dataChannelLabel = "input"

// Session is one workspace ↔ emulator stream. The lifecycle is:
//   New() → Start(ctx, ...) → Close(). Start spawns scrcpy and the
//   forwarding goroutines; Close kills the child and the PeerConnection.
type Session struct {
	ID             string
	WorkspaceID    string
	EmulatorSerial string

	PeerConnection *webrtc.PeerConnection
	VideoTrack     *webrtc.TrackLocalStaticSample
	InputChannel   *webrtc.DataChannel
	Dispatcher     *AdbDispatcher

	scrcpyCmd    *exec.Cmd
	scrcpyStdout io.ReadCloser
	cancel       context.CancelFunc
	startedAt    time.Time
	lastFrameAt  atomic.Int64 // unix-nano of most recent NAL flushed to Pion
	closed       atomic.Bool

	// onLocalCandidate is set by the signaling layer so trickled ICE
	// candidates the local Pion stack produces can flow out the WS.
	mu               sync.Mutex
	onLocalCandidate func(*webrtc.ICECandidate)

	logger zerolog.Logger
}

// New allocates a Session ID and stamps metadata. The PeerConnection
// itself is created in Start so we can plumb the context cleanly.
func New(workspaceID, emulatorSerial string, logger zerolog.Logger) (*Session, error) {
	if workspaceID == "" {
		return nil, errors.New("workspaceID required")
	}
	if emulatorSerial == "" {
		return nil, errors.New("emulatorSerial required")
	}
	id := "scrcpy-" + uuid.NewString()[:12]
	return &Session{
		ID:             id,
		WorkspaceID:    workspaceID,
		EmulatorSerial: emulatorSerial,
		startedAt:      time.Now().UTC(),
		logger: logger.With().
			Str("session", id).
			Str("workspace", workspaceID).
			Str("serial", emulatorSerial).
			Logger(),
	}, nil
}

// StartedAt returns when New() recorded the session.
func (s *Session) StartedAt() time.Time { return s.startedAt }

// LastFrameAt returns the wall-clock time of the most recent NAL the
// reader pushed to Pion. Zero time means we haven't received the
// first frame yet.
func (s *Session) LastFrameAt() time.Time {
	v := s.lastFrameAt.Load()
	if v == 0 {
		return time.Time{}
	}
	return time.Unix(0, v)
}

// SetLocalCandidateSink installs the callback the signaling layer
// uses to forward locally-discovered ICE candidates to the remote
// peer over the signaling WebSocket.
func (s *Session) SetLocalCandidateSink(fn func(*webrtc.ICECandidate)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onLocalCandidate = fn
}

// Start brings up the PeerConnection, registers the video track and
// input data channel, spawns scrcpy, and begins NAL forwarding. The
// returned ctx is the parent of the spawn — cancelling it tears the
// whole session down.
func (s *Session) Start(ctx context.Context, scrcpyPath, adbServer string) error {
	if s.closed.Load() {
		return errors.New("session closed")
	}

	pcCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	pc, err := newPeerConnection()
	if err != nil {
		cancel()
		return fmt.Errorf("peer connection: %w", err)
	}
	s.PeerConnection = pc

	track, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264},
		"video",
		"scrcpy",
	)
	if err != nil {
		_ = pc.Close()
		cancel()
		return fmt.Errorf("video track: %w", err)
	}
	if _, err := pc.AddTrack(track); err != nil {
		_ = pc.Close()
		cancel()
		return fmt.Errorf("add track: %w", err)
	}
	s.VideoTrack = track

	ordered := true
	lifetime := uint16(100)
	dc, err := pc.CreateDataChannel(dataChannelLabel, &webrtc.DataChannelInit{
		Ordered:           &ordered,
		MaxPacketLifeTime: &lifetime,
	})
	if err != nil {
		_ = pc.Close()
		cancel()
		return fmt.Errorf("data channel: %w", err)
	}
	s.InputChannel = dc
	s.Dispatcher = NewAdbDispatcher(s.EmulatorSerial, adbServer)
	s.wireDataChannel(pcCtx)

	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		s.mu.Lock()
		sink := s.onLocalCandidate
		s.mu.Unlock()
		if sink != nil {
			sink(c)
		}
	})
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		s.logger.Info().Str("state", state.String()).Msg("peer connection state")
		switch state {
		case webrtc.PeerConnectionStateFailed,
			webrtc.PeerConnectionStateClosed,
			webrtc.PeerConnectionStateDisconnected:
			// Don't tear the session down here — the signaling
			// layer keeps a 30s grace window for reconnect.
		}
	})

	if err := s.spawnScrcpy(pcCtx, scrcpyPath, adbServer); err != nil {
		_ = pc.Close()
		cancel()
		return err
	}

	go s.pumpVideo(pcCtx)
	return nil
}

func newPeerConnection() (*webrtc.PeerConnection, error) {
	cfg := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{{
			URLs: []string{"stun:stun.l.google.com:19302"},
		}},
	}
	api := webrtc.NewAPI()
	return api.NewPeerConnection(cfg)
}

// spawnScrcpy forks the scrcpy CLI with flags that:
//   - target a specific emulator serial,
//   - disable the local window and audio,
//   - capture raw H.264 into stdout so we can stream it to Pion
//     without an intermediate file.
//
// The exact flag set is conservative: --no-display and --record with
// --record-format=h264 are the documented way to capture raw NAL units.
// If a deployment lands on a scrcpy version that renamed flags, only
// this function needs to change.
func (s *Session) spawnScrcpy(ctx context.Context, scrcpyPath, adbServer string) error {
	if scrcpyPath == "" {
		scrcpyPath = "scrcpy"
	}
	args := []string{
		"--serial=" + s.EmulatorSerial,
		"--no-audio",
		"--no-control",
		"--no-window",
		"--video-codec=h264",
		"--record=/dev/stdout",
		"--record-format=h264",
	}
	cmd := exec.CommandContext(ctx, scrcpyPath, args...)
	if adbServer != "" {
		// scrcpy honours ADB_SERVER_SOCKET when invoking adb itself.
		cmd.Env = append(cmd.Environ(), "ADB_SERVER_SOCKET=tcp:"+adbServer)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("scrcpy stdout pipe: %w", err)
	}
	cmd.Stderr = scrcpyLogWriter{logger: s.logger}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("scrcpy start: %w", err)
	}
	s.scrcpyCmd = cmd
	s.scrcpyStdout = stdout
	s.logger.Info().Int("pid", cmd.Process.Pid).Msg("scrcpy spawned")
	go func() {
		if err := cmd.Wait(); err != nil && ctx.Err() == nil {
			s.logger.Warn().Err(err).Msg("scrcpy exited")
		}
	}()
	return nil
}

// pumpVideo reads NAL units from scrcpy and writes them as Pion media
// samples. The H.264 parser keeps its own buffer; this function just
// forwards.
func (s *Session) pumpVideo(ctx context.Context) {
	if s.scrcpyStdout == nil {
		return
	}
	err := ParseNALUs(s.scrcpyStdout, func(nalu []byte) {
		if ctx.Err() != nil {
			return
		}
		// WriteSample copies the data, so we can let ParseNALUs
		// reuse its scratch buffer.
		if err := s.VideoTrack.WriteSample(media.Sample{
			Data:     nalu,
			Duration: frameDuration,
		}); err != nil {
			s.logger.Debug().Err(err).Msg("WriteSample failed")
			return
		}
		s.lastFrameAt.Store(time.Now().UnixNano())
	})
	if err != nil && ctx.Err() == nil {
		s.logger.Warn().Err(err).Msg("NAL parser stopped")
	}
}

// wireDataChannel attaches the JSON-decode-then-Dispatch handler the
// frontend uses to send pointer/key events alongside (or instead of)
// the WebSocket signaling channel.
func (s *Session) wireDataChannel(ctx context.Context) {
	if s.InputChannel == nil {
		return
	}
	s.InputChannel.OnOpen(func() {
		s.logger.Info().Msg("input data channel open")
	})
	s.InputChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		var ev InputEvent
		if err := json.Unmarshal(msg.Data, &ev); err != nil {
			s.logger.Debug().Err(err).Msg("input decode failed")
			return
		}
		go s.dispatchInput(ctx, ev)
	})
}

// HandleInput is the entry point the signaling layer calls when an
// "input" message arrives on the WebSocket instead of the data
// channel. The dispatcher is shared, so behaviour is identical.
func (s *Session) HandleInput(ctx context.Context, ev InputEvent) error {
	if s.Dispatcher == nil {
		return errors.New("dispatcher not ready")
	}
	return s.Dispatcher.Dispatch(ctx, ev)
}

func (s *Session) dispatchInput(ctx context.Context, ev InputEvent) {
	if s.Dispatcher == nil {
		return
	}
	if err := s.Dispatcher.Dispatch(ctx, ev); err != nil {
		s.logger.Debug().Err(err).Str("type", ev.Type).Msg("dispatch input failed")
	}
}

// Close tears the session down. It's safe to call multiple times; the
// second call is a no-op.
func (s *Session) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}
	if s.cancel != nil {
		s.cancel()
	}
	if s.scrcpyCmd != nil && s.scrcpyCmd.Process != nil {
		_ = s.scrcpyCmd.Process.Kill()
	}
	if s.PeerConnection != nil {
		_ = s.PeerConnection.Close()
	}
	s.logger.Info().Msg("session closed")
	return nil
}

// scrcpyLogWriter pipes scrcpy stderr into zerolog. We drop the
// trailing newline so individual log entries stay clean.
type scrcpyLogWriter struct {
	logger zerolog.Logger
}

func (w scrcpyLogWriter) Write(p []byte) (int, error) {
	line := p
	for len(line) > 0 && (line[len(line)-1] == '\n' || line[len(line)-1] == '\r') {
		line = line[:len(line)-1]
	}
	if len(line) > 0 {
		w.logger.Debug().Str("source", "scrcpy").Msg(string(line))
	}
	return len(p), nil
}
