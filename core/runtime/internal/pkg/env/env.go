// Mirror of core/orchestrator/internal/pkg/env — kept in sync manually; the two-module layout precludes direct sharing.
//
// Package env centralises permissive parsing of process environment
// variables. Every helper trims surrounding whitespace, treats an
// empty / unset / unparseable value as "use the default", and never
// panics on bad operator input.
package env

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// String returns the trimmed value of name, or def when unset or empty.
func String(name, def string) string {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	return v
}

// Int parses name as a base-10 integer. Empty / whitespace-only /
// unparseable values fall back to def.
func Int(name string, def int) int {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// Int64 is the int64 counterpart of Int.
func Int64(name string, def int64) int64 {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}

// Float64 parses name as a float64; falls back to def on any error.
func Float64(name string, def float64) float64 {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return n
}

// Bool returns true for "1", "true", "yes", "on" (case-insensitive),
// false for "0", "false", "no", "off", and def for anything else.
func Bool(name string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	switch v {
	case "":
		return def
	case "1", "t", "true", "y", "yes", "on":
		return true
	case "0", "f", "false", "n", "no", "off":
		return false
	}
	return def
}

// Duration parses name via time.ParseDuration; falls back to def on
// empty / unparseable input.
func Duration(name string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

// StringCSV splits name on "," and trims each segment, dropping empty
// entries. Returns nil when the variable is unset or empty.
func StringCSV(name string) []string {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
