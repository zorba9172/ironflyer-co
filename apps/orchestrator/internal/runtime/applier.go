package runtime

import (
	"bytes"
	"context"
	"encoding/json"
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
		case patch.OpReplace, patch.OpInsertAfter:
			// Anchor-based ops materialise via read-modify-write against
			// the workspace copy of the file. The orchestrator already
			// validated anchor uniqueness against the in-memory project,
			// but we still verify uniqueness here since the workspace tree
			// is the source of truth at apply time.
			body, err := a.C.ReadFile(ctx, userBearer, workspaceID, clean)
			if err != nil {
				return fmt.Errorf("apply read %q: %w", clean, err)
			}
			cur := string(body)
			if strings.Count(cur, ch.Anchor) != 1 {
				return fmt.Errorf("apply anchor mismatch on %q (count != 1)", clean)
			}
			var updated string
			if ch.Op == patch.OpReplace {
				updated = strings.Replace(cur, ch.Anchor, ch.Replacement, 1)
			} else {
				updated = strings.Replace(cur, ch.Anchor, ch.Anchor+ch.Replacement, 1)
			}
			if err := a.C.WriteFile(ctx, userBearer, workspaceID, clean, []byte(updated)); err != nil {
				return fmt.Errorf("apply write %q: %w", clean, err)
			}
		default:
			return fmt.Errorf("apply: unknown op %q on %q", ch.Op, clean)
		}
	}
	// Git-backed persistent snapshot. Initialise the repo on first apply
	// (idempotent: git init is a no-op once .git exists) and stamp a
	// commit per patch. Failures here are non-fatal — they're surfaced as
	// a stderr in logs but don't block the patch lifecycle, since the
	// in-process snapshot store has already captured the previous state.
	_ = a.gitCommitPatch(ctx, userBearer, workspaceID, p)
	return nil
}

// gitCommitPatch initialises a workspace-local git repository (if missing)
// and creates a commit recording this patch. The commit becomes a
// permanent rollback point that survives workspace restarts — strictly
// stronger than the in-process snapshot ring buffer.
func (a *Applier) gitCommitPatch(ctx context.Context, userBearer, workspaceID string, p patch.Patch) error {
	if a == nil || a.C == nil || !a.C.Enabled() {
		return nil
	}
	title := strings.ReplaceAll(strings.ReplaceAll(p.Title, `'`, `'\''`), "\n", " ")
	if title == "" {
		title = "patch"
	}
	cmd := `set -e
if [ ! -d .git ]; then
  git init -q
  git config user.email "ironflyer@local" || true
  git config user.name "Ironflyer Patch Bot" || true
fi
git add -A
git diff --cached --quiet || git commit -q -m "ironflyer: ` + title + `" -m "patch-id: ` + p.ID + `" || true`
	_, err := a.C.Exec(ctx, userBearer, workspaceID, ExecOpts{
		Shell: cmd, TimeoutSeconds: 30,
	})
	return err
}

// ReadFile fetches a file from the runtime workspace. Returns the raw bytes
// or an error if the workspace is offline or the path is missing.
func (c *Client) ReadFile(ctx context.Context, userBearer, workspaceID, path string) ([]byte, error) {
	if !c.Enabled() {
		return nil, errors.New("runtime not configured")
	}
	url := c.BaseURL + "/workspaces/" + workspaceID + "/files/" + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if userBearer != "" {
		req.Header.Set("Authorization", "Bearer "+userBearer)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("runtime read: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		bts, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("runtime read %d: %s", resp.StatusCode, strings.TrimSpace(string(bts)))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("runtime read body: %w", err)
	}
	return body, nil
}

// Screenshot asks the runtime to render the workspace's preview at the
// given route + viewport and return a PNG. The runtime spins up
// chromium-headless when available; on stubbed runtimes it falls back
// to a solid-colour placeholder so callers can still pipeline without
// special-casing dev. Returned string is base64-encoded PNG (no data:
// prefix) — matches the on-wire shape of VisualTarget.ImagePNGBase64.
func (c *Client) Screenshot(ctx context.Context, userBearer, workspaceID, route string, viewportW, viewportH int) (string, error) {
	if !c.Enabled() {
		return "", errors.New("runtime not configured")
	}
	body, _ := json.Marshal(map[string]any{
		"route":     route,
		"viewportW": viewportW,
		"viewportH": viewportH,
	})
	req, err := http.NewRequestWithContext(ctx,
		http.MethodPost,
		c.BaseURL+"/workspaces/"+workspaceID+"/screenshot",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if userBearer != "" {
		req.Header.Set("Authorization", "Bearer "+userBearer)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("runtime screenshot: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("runtime screenshot %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out struct {
		ImagePNGBase64 string `json:"imagePngBase64"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&out); err != nil {
		return "", fmt.Errorf("runtime screenshot decode: %w", err)
	}
	return out.ImagePNGBase64, nil
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
