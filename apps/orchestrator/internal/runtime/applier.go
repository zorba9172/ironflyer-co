package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"ironflyer/apps/orchestrator/internal/patch"
)

// Applier bridges the finisher loop to a live runtime workspace. It is the
// concrete implementation of finisher.RuntimeApplier: when the loop approves
// a patch, the applier writes every FileChange through the runtime's File
// API so the user sees the change reflected in their workspace (and any
// running dev server picks it up via HMR).
//
// We deliberately avoid the runtime's unified-diff endpoint here because
// the orchestrator's patch model is structured (op + path + content); the
// File API is a more reliable mapping.
type Applier struct {
	C *Client
}

// NewApplier wraps a runtime client. A nil client (runtime disabled) yields
// an applier whose Apply is a no-op so the loop continues without errors.
func NewApplier(c *Client) *Applier { return &Applier{C: c} }

// Apply implements finisher.RuntimeApplier.
//
// Contract: each FileChange is materialised against the workspace through
// the runtime's File API. Paths are sanitised (no leading slash, no
// traversal) defensively — the patch engine validates this upstream too.
// On the first failure, Apply stops and returns the wrapped error.
func (a *Applier) Apply(ctx context.Context, userBearer, workspaceID string, p patch.Patch) error {
	if a == nil || a.C == nil || !a.C.Enabled() {
		return nil
	}
	if workspaceID == "" {
		return errors.New("apply: workspaceID required")
	}
	for _, ch := range p.Changes {
		clean := cleanFilePath(ch.Path)
		if clean == "" {
			return fmt.Errorf("apply: empty path in change")
		}
		switch ch.Op {
		case patch.OpCreate, patch.OpUpdate:
			if err := a.C.WriteFile(ctx, userBearer, workspaceID, clean, []byte(ch.Content)); err != nil {
				return fmt.Errorf("apply write %q: %w", clean, err)
			}
		case patch.OpDelete:
			if err := a.C.DeleteFile(ctx, userBearer, workspaceID, clean); err != nil {
				return fmt.Errorf("apply delete %q: %w", clean, err)
			}
		default:
			return fmt.Errorf("apply: unknown op %q on %q", ch.Op, clean)
		}
	}
	return nil
}

// WriteFile uploads file contents to the runtime workspace at path. Path
// must be relative to the workspace root.
func (c *Client) WriteFile(ctx context.Context, userBearer, workspaceID, path string, data []byte) error {
	if !c.Enabled() {
		return errors.New("runtime not configured")
	}
	url := c.BaseURL + "/workspaces/" + workspaceID + "/files/" + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	if userBearer != "" {
		req.Header.Set("Authorization", "Bearer "+userBearer)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("runtime write: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		bts, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("runtime write %d: %s", resp.StatusCode, strings.TrimSpace(string(bts)))
	}
	return nil
}

// DeleteFile removes a file from the runtime workspace.
func (c *Client) DeleteFile(ctx context.Context, userBearer, workspaceID, path string) error {
	if !c.Enabled() {
		return errors.New("runtime not configured")
	}
	url := c.BaseURL + "/workspaces/" + workspaceID + "/files/" + path
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	if userBearer != "" {
		req.Header.Set("Authorization", "Bearer "+userBearer)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("runtime delete: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode/100 != 2 {
		bts, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("runtime delete %d: %s", resp.StatusCode, strings.TrimSpace(string(bts)))
	}
	return nil
}

func cleanFilePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "/")
	for strings.Contains(p, "../") {
		p = strings.ReplaceAll(p, "../", "")
	}
	return p
}
