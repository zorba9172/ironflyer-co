// Package ui holds the terminal presentation primitives: colored ANSI
// output, status pills, a spinner, and a small table renderer. None of
// these allocate goroutines unless explicitly started.
package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// isTTY reports whether the given file descriptor is attached to a
// terminal. We use this to suppress ANSI escape codes and spinners when
// stdout is being piped (eg `ironflyer projects | jq`).
//
// We avoid pulling in golang.org/x/term to honour the "minimal deps" bar:
// instead we ask the kernel directly via os.Stat. If the file's mode says
// it's a char device, treat it as a TTY.
func isTTY(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// IsTTY is exported so command code can decide whether to render rich
// output. Spinners + status pills check this internally.
func IsTTY() bool { return isTTY(os.Stdout) }

// IsStderrTTY is used for the spinner which lives on stderr.
func IsStderrTTY() bool { return isTTY(os.Stderr) }

// ANSI color helpers. We hand-roll them rather than depend on fatih/color
// to keep the binary tiny — every dep is a future maintenance bill.
const (
	reset     = "\x1b[0m"
	bold      = "\x1b[1m"
	dim       = "\x1b[2m"
	red       = "\x1b[31m"
	green     = "\x1b[32m"
	yellow    = "\x1b[33m"
	blue      = "\x1b[34m"
	magenta   = "\x1b[35m"
	cyan      = "\x1b[36m"
	white     = "\x1b[37m"
	bgGreen   = "\x1b[42m"
	bgRed     = "\x1b[41m"
	bgYellow  = "\x1b[43m"
	bgBlue    = "\x1b[44m"
	bgDimGray = "\x1b[100m"
)

func paint(code, s string) string {
	if !IsTTY() {
		return s
	}
	return code + s + reset
}

// Red / Green / etc. return the input wrapped in ANSI if stdout is a TTY.
func Red(s string) string     { return paint(red, s) }
func Green(s string) string   { return paint(green, s) }
func Yellow(s string) string  { return paint(yellow, s) }
func Blue(s string) string    { return paint(blue, s) }
func Magenta(s string) string { return paint(magenta, s) }
func Cyan(s string) string    { return paint(cyan, s) }
func Bold(s string) string    { return paint(bold, s) }
func Dim(s string) string     { return paint(dim, s) }

// Errorf writes a red error line to stderr. Used by every command's
// failure path so the output is uniform.
func Errorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if isTTY(os.Stderr) {
		fmt.Fprintln(os.Stderr, red+"error: "+msg+reset)
	} else {
		fmt.Fprintln(os.Stderr, "error: "+msg)
	}
}

// Infof writes an informational line to stderr (so it doesn't pollute
// piped output like `ironflyer projects --json | jq`).
func Infof(format string, args ...any) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf(format, args...))
}

// Pill renders a small status badge — green OK, red FAIL, etc. Used by
// the `status` command.
func Pill(label, status string) string {
	statusUpper := strings.ToUpper(strings.TrimSpace(status))
	var bg string
	switch statusUpper {
	case "OK", "READY", "UP", "PASS", "PASSED", "DEPLOYED":
		bg = bgGreen
	case "FAIL", "FAILED", "DOWN", "ERROR":
		bg = bgRed
	case "PENDING", "RUNNING", "STARTED", "QUEUED":
		bg = bgYellow
	case "INFO":
		bg = bgBlue
	default:
		bg = bgDimGray
	}
	if !IsTTY() {
		return fmt.Sprintf("%s: %s", label, statusUpper)
	}
	return fmt.Sprintf("%s%s %s %s%s %s", bold, bg, statusUpper, reset, reset, label)
}

// RenderTable prints a left-aligned table. Headers in bold, columns
// padded to the widest cell. If non-TTY we drop the ANSI and use plain
// pipe separators so the output stays grep-friendly.
func RenderTable(w io.Writer, headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, r := range rows {
		for i, c := range r {
			if i >= len(widths) {
				continue
			}
			if l := visibleLen(c); l > widths[i] {
				widths[i] = l
			}
		}
	}
	// Header.
	var hb strings.Builder
	for i, h := range headers {
		hb.WriteString(Bold(padRight(h, widths[i])))
		if i < len(headers)-1 {
			hb.WriteString("  ")
		}
	}
	fmt.Fprintln(w, hb.String())
	// Underline (only on TTY — otherwise omit for cleaner pipes).
	if IsTTY() {
		var ub strings.Builder
		for i, wd := range widths {
			ub.WriteString(Dim(strings.Repeat("─", wd)))
			if i < len(widths)-1 {
				ub.WriteString("  ")
			}
		}
		fmt.Fprintln(w, ub.String())
	}
	for _, r := range rows {
		var rb strings.Builder
		for i := range headers {
			cell := ""
			if i < len(r) {
				cell = r[i]
			}
			rb.WriteString(padRight(cell, widths[i]))
			if i < len(headers)-1 {
				rb.WriteString("  ")
			}
		}
		fmt.Fprintln(w, rb.String())
	}
}

// padRight pads s with spaces to the requested visible width. ANSI escape
// sequences don't count toward the width.
func padRight(s string, width int) string {
	pad := width - visibleLen(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

// visibleLen returns the rune-count of s after stripping ANSI escapes.
// Good enough for tabular alignment on Latin scripts.
func visibleLen(s string) int {
	out := 0
	inEsc := false
	for _, r := range s {
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		if r == 0x1b {
			inEsc = true
			continue
		}
		out++
	}
	return out
}

// Spinner is a tiny goroutine-driven progress indicator. It only writes
// when stderr is a TTY — in CI/piped contexts Start/Stop are no-ops.
type Spinner struct {
	msg     atomic.Value // string
	stop    chan struct{}
	wg      sync.WaitGroup
	once    sync.Once
	enabled bool
}

// NewSpinner returns a Spinner ready to be started.
func NewSpinner(msg string) *Spinner {
	s := &Spinner{
		stop:    make(chan struct{}),
		enabled: IsStderrTTY(),
	}
	s.msg.Store(msg)
	return s
}

// Start begins the animation. Safe to call when stderr is not a TTY — it
// becomes a no-op.
func (s *Spinner) Start() {
	if !s.enabled {
		return
	}
	s.wg.Add(1)
	go s.loop()
}

// Update changes the spinner message in place. No-op if not running.
func (s *Spinner) Update(msg string) { s.msg.Store(msg) }

// Stop halts the spinner and erases its line.
func (s *Spinner) Stop() {
	if !s.enabled {
		return
	}
	s.once.Do(func() {
		close(s.stop)
		s.wg.Wait()
		fmt.Fprint(os.Stderr, "\r\x1b[2K") // clear the line
	})
}

func (s *Spinner) loop() {
	defer s.wg.Done()
	frames := []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}
	i := 0
	t := time.NewTicker(80 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			frame := string(frames[i%len(frames)])
			msg, _ := s.msg.Load().(string)
			fmt.Fprintf(os.Stderr, "\r\x1b[2K%s%s%s %s", cyan, frame, reset, msg)
			i++
		}
	}
}

// AgentColor returns a stable color for the given agent role name so
// streamed events stay visually grouped (planner = blue, coder = green,
// architect = magenta, etc.).
func AgentColor(role string) func(string) string {
	switch strings.ToLower(role) {
	case "planner":
		return Blue
	case "architect":
		return Magenta
	case "coder":
		return Green
	case "lint":
		return Yellow
	case "test", "tester":
		return Cyan
	case "security":
		return Red
	case "deploy", "deployer":
		return Bold
	default:
		return Dim
	}
}
