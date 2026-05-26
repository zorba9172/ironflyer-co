package session

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// InputEvent is the wire format the frontend sends over the WebRTC
// data channel (and over the WebSocket as a fallback). Coordinates are
// always normalised into [0, 1] so the client doesn't need to know
// the emulator's pixel resolution.
type InputEvent struct {
	Type    string  `json:"type"`              // "touch" | "swipe" | "key" | "text"
	Action  string  `json:"action,omitempty"`  // "down" | "up" | "move" | "tap"
	X       float64 `json:"x,omitempty"`       // normalised [0, 1]
	Y       float64 `json:"y,omitempty"`       // normalised [0, 1]
	X2      float64 `json:"x2,omitempty"`      // swipe destination
	Y2      float64 `json:"y2,omitempty"`      // swipe destination
	Duration int    `json:"duration,omitempty"` // swipe duration ms
	Keycode int     `json:"keycode,omitempty"`
	Text    string  `json:"text,omitempty"`
}

// Curated Android keycodes the frontend hardware-button row dispatches.
// scrcpy's input mapping is more thorough — we only enumerate the
// constants surfaced through the UI to keep the allow-list tight.
const (
	KeycodeHome       = 3
	KeycodeBack       = 4
	KeycodeRecentApps = 187
	KeycodePower      = 26
	KeycodeVolumeUp   = 24
	KeycodeVolumeDown = 25
	KeycodeMenu       = 82
	KeycodeEnter      = 66
	KeycodeBackspace  = 67
)

// allowedKeycodes is the explicit set the dispatcher will forward to
// adb. Anything else is rejected to keep the surface area predictable.
var allowedKeycodes = map[int]bool{
	KeycodeHome:       true,
	KeycodeBack:       true,
	KeycodeRecentApps: true,
	KeycodePower:      true,
	KeycodeVolumeUp:   true,
	KeycodeVolumeDown: true,
	KeycodeMenu:       true,
	KeycodeEnter:      true,
	KeycodeBackspace:  true,
}

// AdbDispatcher translates InputEvent payloads into `adb shell input`
// invocations against a specific emulator serial. The display size is
// resolved lazily on the first event and cached for the lifetime of
// the dispatcher; emulator resizes mid-session are unusual enough that
// callers can construct a new dispatcher if they need fresh metrics.
type AdbDispatcher struct {
	Serial    string
	AdbServer string // forwarded as ANDROID_ADB_SERVER_PORT-aware -H/-P pair
	AdbPath   string // override for the adb binary; defaults to "adb"

	mu     sync.Mutex
	width  int
	height int
	sized  atomic.Bool
}

// NewAdbDispatcher constructs a dispatcher with sensible defaults for
// the adb binary location and server address.
func NewAdbDispatcher(serial, adbServer string) *AdbDispatcher {
	return &AdbDispatcher{
		Serial:    serial,
		AdbServer: adbServer,
		AdbPath:   "adb",
	}
}

// resolveDisplay caches the emulator's current display geometry. We
// shell out to `wm size` which prints "Physical size: 1080x2400" and
// optionally an "Override size:" line. The override wins when present.
func (d *AdbDispatcher) resolveDisplay(ctx context.Context) (int, int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.sized.Load() {
		return d.width, d.height, nil
	}
	out, err := d.run(ctx, "shell", "wm", "size")
	if err != nil {
		return 0, 0, fmt.Errorf("wm size: %w", err)
	}
	w, h := parseWmSize(out)
	if w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("could not parse wm size output: %q", out)
	}
	d.width, d.height = w, h
	d.sized.Store(true)
	return w, h, nil
}

// Dispatch validates the event, denormalises coordinates against the
// emulator's display metrics, and forwards the corresponding adb
// command. Errors are returned to the caller so the signaling layer
// can log them — they never crash the session.
func (d *AdbDispatcher) Dispatch(ctx context.Context, ev InputEvent) error {
	switch ev.Type {
	case "touch":
		return d.dispatchTouch(ctx, ev)
	case "swipe":
		return d.dispatchSwipe(ctx, ev)
	case "key":
		return d.dispatchKey(ctx, ev)
	case "text":
		return d.dispatchText(ctx, ev)
	default:
		return fmt.Errorf("unknown input event type %q", ev.Type)
	}
}

func (d *AdbDispatcher) dispatchTouch(ctx context.Context, ev InputEvent) error {
	if err := checkNorm(ev.X, ev.Y); err != nil {
		return err
	}
	w, h, err := d.resolveDisplay(ctx)
	if err != nil {
		return err
	}
	x := strconv.Itoa(int(ev.X * float64(w)))
	y := strconv.Itoa(int(ev.Y * float64(h)))
	// adb shell input tap fires a synthetic down+up. For real
	// gestures the client should send a "swipe" with x==x2/y==y2.
	_, err = d.run(ctx, "shell", "input", "tap", x, y)
	return err
}

func (d *AdbDispatcher) dispatchSwipe(ctx context.Context, ev InputEvent) error {
	if err := checkNorm(ev.X, ev.Y); err != nil {
		return err
	}
	if err := checkNorm(ev.X2, ev.Y2); err != nil {
		return err
	}
	w, h, err := d.resolveDisplay(ctx)
	if err != nil {
		return err
	}
	dur := ev.Duration
	if dur <= 0 {
		dur = 120
	}
	if dur > 5000 {
		dur = 5000
	}
	args := []string{
		"shell", "input", "swipe",
		strconv.Itoa(int(ev.X * float64(w))),
		strconv.Itoa(int(ev.Y * float64(h))),
		strconv.Itoa(int(ev.X2 * float64(w))),
		strconv.Itoa(int(ev.Y2 * float64(h))),
		strconv.Itoa(dur),
	}
	_, err = d.run(ctx, args...)
	return err
}

func (d *AdbDispatcher) dispatchKey(ctx context.Context, ev InputEvent) error {
	if !allowedKeycodes[ev.Keycode] {
		return fmt.Errorf("keycode %d not allowed", ev.Keycode)
	}
	_, err := d.run(ctx, "shell", "input", "keyevent", strconv.Itoa(ev.Keycode))
	return err
}

func (d *AdbDispatcher) dispatchText(ctx context.Context, ev InputEvent) error {
	text := strings.TrimSpace(ev.Text)
	if text == "" {
		return errors.New("empty text payload")
	}
	if len(text) > 512 {
		return errors.New("text payload exceeds 512 chars")
	}
	// adb shell input text treats spaces as separators; replace
	// with %s which the shell interprets as space. Quote the rest.
	escaped := strings.ReplaceAll(text, " ", "%s")
	_, err := d.run(ctx, "shell", "input", "text", escaped)
	return err
}

// run executes `adb -s <serial> <args...>` with a short timeout and
// returns the trimmed stdout. Errors carry the stderr tail.
func (d *AdbDispatcher) run(parent context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, 4*time.Second)
	defer cancel()
	full := []string{"-s", d.Serial}
	if d.AdbServer != "" {
		host, port, ok := splitHostPort(d.AdbServer)
		if ok {
			full = append([]string{"-H", host, "-P", port}, full...)
		}
	}
	full = append(full, args...)
	bin := d.AdbPath
	if bin == "" {
		bin = "adb"
	}
	cmd := exec.CommandContext(ctx, bin, full...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("adb %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func checkNorm(x, y float64) error {
	if x < 0 || x > 1 || y < 0 || y > 1 {
		return fmt.Errorf("coordinates out of [0,1] range: x=%.3f y=%.3f", x, y)
	}
	return nil
}

func parseWmSize(out string) (int, int) {
	// Prefer "Override size:" if present, otherwise fall back to
	// "Physical size:". Either line shapes as "...: WIDTHxHEIGHT".
	var phys, override string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Override size:"):
			override = strings.TrimSpace(strings.TrimPrefix(line, "Override size:"))
		case strings.HasPrefix(line, "Physical size:"):
			phys = strings.TrimSpace(strings.TrimPrefix(line, "Physical size:"))
		}
	}
	pick := override
	if pick == "" {
		pick = phys
	}
	parts := strings.SplitN(pick, "x", 2)
	if len(parts) != 2 {
		return 0, 0
	}
	w, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
	return w, h
}

func splitHostPort(s string) (string, string, bool) {
	i := strings.LastIndex(s, ":")
	if i <= 0 || i == len(s)-1 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}
