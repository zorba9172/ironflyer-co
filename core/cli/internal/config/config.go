// Package config reads and writes ~/.ironflyer/config.json. The file is the
// CLI's only piece of persistent state — it stores the orchestrator host,
// the bearer token captured during `ironflyer login`, and an optional
// default project id so single-project users don't have to type the id on
// every command.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// File is the on-disk schema. Keep this struct small and additive — older
// CLI binaries should still parse newer configs (unknown keys are dropped
// by encoding/json, which is fine).
type File struct {
	Host           string `json:"host,omitempty"`
	Token          string `json:"token,omitempty"`
	DefaultProject string `json:"defaultProject,omitempty"`
	UserEmail      string `json:"userEmail,omitempty"`
}

// DefaultHost is what we fall back to when nothing else is configured.
// Localhost is the dev orchestrator — production users override this with
// `ironflyer config set host https://api.ironflyer.dev`.
const DefaultHost = "http://localhost:8080"

// Path returns the absolute path to the config file. It does NOT create
// the parent directory — call EnsureDir before writing.
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	return filepath.Join(home, ".ironflyer", "config.json"), nil
}

// EnsureDir creates ~/.ironflyer with 0700 if missing.
func EnsureDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".ironflyer")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}
	return dir, nil
}

// Load reads the config file. A missing file is not an error — we return
// a zero File so commands can supply sensible defaults.
func Load() (File, error) {
	p, err := Path()
	if err != nil {
		return File{}, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return File{}, nil
		}
		return File{}, fmt.Errorf("read %s: %w", p, err)
	}
	var f File
	if len(b) == 0 {
		return f, nil
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return File{}, fmt.Errorf("parse %s: %w", p, err)
	}
	return f, nil
}

// Save atomically writes the config file with 0600 perms. We write to a
// sibling tmp file then rename so an interrupted save can't corrupt the
// previous config.
func Save(f File) error {
	if _, err := EnsureDir(); err != nil {
		return err
	}
	p, err := Path()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, p); err != nil {
		return fmt.Errorf("rename %s: %w", tmp, err)
	}
	return nil
}

// Clear removes the config file. Used by `ironflyer logout`.
func Clear() error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// HostOrDefault returns the configured host or the dev fallback.
func (f File) HostOrDefault() string {
	if v := strings.TrimSpace(os.Getenv("IRONFLYER_HOST")); v != "" {
		return v
	}
	if f.Host == "" {
		return DefaultHost
	}
	return f.Host
}
