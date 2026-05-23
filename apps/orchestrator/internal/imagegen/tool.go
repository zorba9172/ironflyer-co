// generate_image tool wiring.
//
// This file exposes the imagegen.Tool that gets registered with the
// agents.Registry as a built-in tool the Coder can invoke alongside
// MCP-provided tools. The Coder uses it to mid-generation produce an
// asset (e.g. a hero image, a logo) and reference it in the patch it
// is about to emit. This is what gives us parity with Lovable's
// inline image generation.
//
// IMPORTANT — engine wiring caveat:
//
// For the tool to actually write into a workspace, agents.Task must
// carry the caller's user bearer + workspace ID. Those fields were
// added to agents.Task in this commit, but the finisher Engine's
// tool-loop is owned by another worker and may not yet thread them
// through. Until that worker lands, the Coder will call this tool,
// the handler will receive an empty workspaceID, and the call will
// fail gracefully with `"no workspace bound"` — the Coder gets a
// readable error and can continue without the asset rather than
// crashing the run.
//
// To finish the wiring, the Engine (apps/orchestrator/internal/finisher/loop.go
// and any code that constructs agents.Task with Role=coder) needs to
// set Task.UserBearer + Task.WorkspaceID before calling Registry.Run.
package imagegen

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"ironflyer/apps/orchestrator/internal/providers"
)

// FileWriter is the subset of the runtime client the tool needs.
// Implemented by *runtime.Client at the call site so this package
// stays free of any runtime/* import (preventing cycles and keeping
// imagegen testable in isolation).
type FileWriter interface {
	WriteFile(ctx context.Context, userBearer, workspaceID, path string, data []byte) error
}

// Tool is the built-in generate_image tool. Construct it at boot:
//
//	imgTool := &imagegen.Tool{
//	    Gen:       provider,
//	    Writer:    runtimeClient,
//	    AssetsDir: "public/assets",
//	    MaxBytes:  4 << 20,
//	}
//	registry.WithBuiltinTool(imgTool.Spec(), imgTool.Call)
type Tool struct {
	Gen       Provider
	Writer    FileWriter
	AssetsDir string // default "public/assets"
	MaxBytes  int    // hard cap per image; default 4 MiB
}

// defaults
const (
	defaultAssetsDir = "public/assets"
	defaultMaxBytes  = 4 << 20
	maxNameLen       = 60
)

// Spec returns the providers.ToolSpec the agents registry hands to
// the model. Keep the description tight and concrete — it is part of
// the model's working context every Coder turn.
func (t *Tool) Spec() providers.ToolSpec {
	return providers.ToolSpec{
		Name:        "generate_image",
		Description: "Generate a PNG asset from a text prompt and save it to the project. Returns the asset path the calling Coder should reference in its patches. Size defaults to 1024x1024.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"prompt": map[string]any{
					"type":        "string",
					"description": "Plain-English description of the image",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Slug used for the filename (no extension)",
				},
				"size": map[string]any{
					"type":    "string",
					"enum":    []string{"1024x1024", "1024x1792", "1792x1024"},
					"default": "1024x1024",
				},
			},
			"required": []string{"prompt", "name"},
		},
	}
}

// Call validates the args, calls the configured provider, writes the
// resulting PNG to the workspace under AssetsDir, and returns a JSON
// string the Coder echoes back into its tool_result. The Coder is
// expected to reference the returned `path` (web-absolute, e.g.
// `/assets/hero.png`) in the patches it then proposes.
func (t *Tool) Call(ctx context.Context, userBearer, workspaceID string, args map[string]any) (string, error) {
	if t == nil {
		return "", errors.New("generate_image: tool not configured")
	}
	if t.Gen == nil {
		return "", errors.New("generate_image: provider not configured")
	}
	if t.Writer == nil {
		return "", errors.New("generate_image: writer not configured")
	}
	if workspaceID == "" {
		// Engine hasn't threaded workspace context through yet — fail
		// readably so the Coder can fall back to a static asset path
		// instead of crashing the run.
		return "", errors.New("generate_image: no workspace bound")
	}

	prompt := strings.TrimSpace(stringArg(args, "prompt"))
	if prompt == "" {
		return "", errors.New("generate_image: prompt is required")
	}
	rawName := stringArg(args, "name")
	name := sanitiseName(rawName)
	if name == "" {
		return "", errors.New("generate_image: name is required (alphanumeric slug)")
	}

	size := stringArg(args, "size")
	if size == "" {
		size = "1024x1024"
	}
	switch size {
	case "1024x1024", "1024x1792", "1792x1024":
	default:
		return "", fmt.Errorf("generate_image: invalid size %q", size)
	}

	png, err := t.Gen.Generate(ctx, prompt, size)
	if err != nil {
		return "", fmt.Errorf("generate_image: %w", err)
	}

	maxBytes := t.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	if len(png) > maxBytes {
		return "", fmt.Errorf("generate_image: image too large (%d bytes > cap %d)", len(png), maxBytes)
	}

	assets := strings.Trim(t.AssetsDir, "/")
	if assets == "" {
		assets = defaultAssetsDir
	}
	absPath := assets + "/" + name + ".png"
	if err := t.Writer.WriteFile(ctx, userBearer, workspaceID, absPath, png); err != nil {
		return "", fmt.Errorf("generate_image: write %q: %w", absPath, err)
	}

	// Compute the web-absolute path the Coder should reference. Next.js
	// serves the contents of /public at the site root, so a file at
	// public/assets/foo.png is reachable at /assets/foo.png.
	webPath := "/" + strings.TrimPrefix(absPath, "public/")
	if !strings.HasPrefix(absPath, "public/") {
		// Caller picked a non-public assets dir; surface the raw path.
		webPath = "/" + absPath
	}

	out, _ := json.Marshal(map[string]any{
		"path":         webPath,
		"absolutePath": absPath,
		"bytes":        len(png),
	})
	return string(out), nil
}

// stringArg pulls a string out of the args map, tolerating missing
// keys and non-string values.
func stringArg(args map[string]any, key string) string {
	v, ok := args[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// sanitiseName lowercases, collapses non-alphanumerics to `-`, trims
// leading/trailing dashes, and caps the result at maxNameLen. The
// result is safe to drop into a filesystem path without further
// quoting.
func sanitiseName(in string) string {
	in = strings.ToLower(strings.TrimSpace(in))
	var b strings.Builder
	prevDash := false
	for _, r := range in {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > maxNameLen {
		out = strings.TrimRight(out[:maxNameLen], "-")
	}
	return out
}
