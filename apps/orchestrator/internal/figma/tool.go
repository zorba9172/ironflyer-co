// figma_import tool wiring.
//
// Exposes figma.Tool which gets registered with the agents.Registry as
// a built-in tool the Coder can invoke alongside MCP-provided tools.
// The Coder uses it mid-Run to ingest a Figma file (parsed into design
// tokens + a component inventory) and reference the extracted assets
// in the patch it is about to emit. This is what gives us parity with
// Lovable / Bolt's premium "Figma → code" tier.
//
// Engine wiring caveat: same as imagegen. For the tool to actually
// write into a workspace the finisher Engine's tool-loop must thread
// agents.Task.UserBearer + WorkspaceID through to Registry.Run. When
// they're empty the tool fails gracefully with a readable error string
// rather than panicking the Coder.

package figma

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
// figma testable in isolation). Matches imagegen.FileWriter shape on
// purpose so a single runtime client satisfies both.
type FileWriter interface {
	WriteFile(ctx context.Context, userBearer, workspaceID, path string, data []byte) error
}

// Tool wraps a Client + a project store so the Coder can issue a
// single `figma_import` tool call to ingest a design into the current
// project. Constructed at boot:
//
//	figmaTool := &figma.Tool{Client: figmaClient, Writer: runtimeClient}
//	registry.WithBuiltinTool(figmaTool.Spec(), figmaTool.Call)
type Tool struct {
	Client *Client
	Writer FileWriter
}

// Spec returns the providers.ToolSpec the agents registry hands to
// the model. The description doubles as the prompt-time contract —
// keep it concrete so the Coder knows exactly when to reach for it.
func (t *Tool) Spec() providers.ToolSpec {
	return providers.ToolSpec{
		Name:        "figma_import",
		Description: "Import a Figma file into the current project. Returns the extracted design tokens + component inventory the agent should use when generating code. The tool also writes .ironflyer/figma_extract.json and design_tokens.json into the workspace for downstream gates.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"fileKey": map[string]any{
					"type":        "string",
					"description": "Figma file key from the URL: figma.com/file/<KEY>/...",
				},
			},
			"required": []string{"fileKey"},
		},
	}
}

// Call validates the args, calls the configured Figma client, parses
// the file into a tokens + inventory pair, writes both manifests into
// the caller's workspace, and returns the same JSON the Coder echoes
// into its tool_result. The Coder is expected to consume the returned
// tokens + inventory directly when generating the patch.
func (t *Tool) Call(ctx context.Context, userBearer, workspaceID string, args map[string]any) (string, error) {
	if t == nil {
		return "", errors.New("figma_import: tool not configured")
	}
	if t.Client == nil {
		return "", errors.New("figma_import: client not configured")
	}
	if t.Writer == nil {
		return "", errors.New("figma_import: writer not configured")
	}
	if workspaceID == "" {
		// Engine hasn't threaded workspace context through yet —
		// fail readably so the Coder can fall back rather than
		// crashing the run.
		return "", errors.New("figma_import: no workspace bound")
	}
	if t.Client.Token == "" {
		return "", errors.New("figma_import: figma token not configured (set FIGMA_TOKEN)")
	}

	fileKey := strings.TrimSpace(stringArg(args, "fileKey"))
	if fileKey == "" {
		return "", errors.New("figma_import: fileKey is required")
	}

	return t.Run(ctx, userBearer, workspaceID, fileKey)
}

// Run is the underlying ingestion routine, separated from Call so the
// HTTP endpoint can invoke it without re-marshalling args through a
// map[string]any.
func (t *Tool) Run(ctx context.Context, userBearer, workspaceID, fileKey string) (string, error) {
	file, err := t.Client.GetFile(ctx, fileKey)
	if err != nil {
		return "", fmt.Errorf("figma_import: %w", err)
	}
	tokens, inventory := Extract(file)

	result := map[string]any{
		"tokens":    tokens,
		"inventory": inventory,
		"file": map[string]any{
			"name":         file.Name,
			"lastModified": file.LastModified,
		},
	}
	resultBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("figma_import: encode result: %w", err)
	}

	// Persist the full extract so downstream gates (UX, design-tokens)
	// can re-read it without having to re-call Figma. Overwrite is
	// intentional — the latest import is the source of truth.
	if err := t.Writer.WriteFile(ctx, userBearer, workspaceID, ".ironflyer/figma_extract.json", resultBytes); err != nil {
		return "", fmt.Errorf("figma_import: write figma_extract.json: %w", err)
	}
	tokensBytes, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return "", fmt.Errorf("figma_import: encode tokens: %w", err)
	}
	if err := t.Writer.WriteFile(ctx, userBearer, workspaceID, ".ironflyer/design_tokens.json", tokensBytes); err != nil {
		return "", fmt.Errorf("figma_import: write design_tokens.json: %w", err)
	}

	return string(resultBytes), nil
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
