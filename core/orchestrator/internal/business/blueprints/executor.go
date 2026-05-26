package blueprints

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Executor materializes a blueprint into a workspace by writing
// every TemplateFile to its workspace-relative path. The executor
// performs NO dependency install and NO build step — the workspace
// runtime (core/runtime) owns that lifecycle and will pick up the
// new files via the watcher.
type Executor interface {
	// Execute writes every file in b into workspaceDir and returns
	// the absolute (host) paths actually created, in the order they
	// were written. workspaceDir must already exist.
	Execute(ctx context.Context, b Blueprint, workspaceDir string) ([]string, error)
}

// NewFSExecutor returns the default Executor — writes to the local
// filesystem rooted at workspaceDir. The integration loop (Agent 8)
// wires this against the runtime workspace mount path.
func NewFSExecutor() Executor { return &fsExecutor{} }

type fsExecutor struct{}

// Execute is intentionally simple: validate the workspace, then
// write each file. We refuse to clobber a file outside workspaceDir
// (defense in depth against malformed template paths).
func (fsExecutor) Execute(ctx context.Context, b Blueprint, workspaceDir string) ([]string, error) {
	if strings.TrimSpace(workspaceDir) == "" {
		return nil, errors.New("blueprints: workspaceDir is empty")
	}
	absRoot, err := filepath.Abs(workspaceDir)
	if err != nil {
		return nil, fmt.Errorf("blueprints: resolve workspace: %w", err)
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("blueprints: stat workspace: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("blueprints: workspace %q is not a directory", absRoot)
	}

	created := make([]string, 0, len(b.Files))
	for _, f := range b.Files {
		if err := ctx.Err(); err != nil {
			return created, err
		}
		rel := filepath.FromSlash(f.Path)
		target := filepath.Join(absRoot, rel)
		// Reject any path that escapes the workspace root.
		cleanedTarget, err := filepath.Abs(target)
		if err != nil {
			return created, fmt.Errorf("blueprints: resolve %q: %w", f.Path, err)
		}
		if !strings.HasPrefix(cleanedTarget+string(os.PathSeparator), absRoot+string(os.PathSeparator)) &&
			cleanedTarget != absRoot {
			return created, fmt.Errorf("blueprints: refuse to write outside workspace: %q", f.Path)
		}
		if err := os.MkdirAll(filepath.Dir(cleanedTarget), 0o755); err != nil {
			return created, fmt.Errorf("blueprints: mkdir %q: %w", filepath.Dir(cleanedTarget), err)
		}
		mode := f.Mode
		if mode == 0 {
			mode = 0o644
		}
		if err := os.WriteFile(cleanedTarget, []byte(f.Content), mode); err != nil {
			return created, fmt.Errorf("blueprints: write %q: %w", cleanedTarget, err)
		}
		created = append(created, cleanedTarget)
	}
	return created, nil
}
