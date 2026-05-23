// Package imagegen provides image generation providers used by the
// Coder agent's built-in `generate_image` tool. The package is
// deliberately stdlib-only so adding it doesn't grow go.mod.
//
// Two providers ship today:
//
//   - OpenAIImagesProvider — calls https://api.openai.com/v1/images/generations
//     with the first-party "gpt-image-1" model. Falls back to "dall-e-3"
//     on a 4xx that mentions "model not found" so older accounts still
//     work without operator action.
//   - NoopProvider — always errors with "image generation disabled".
//     This is the default when no API key is configured, so the tool
//     surface is wired but disabled rather than crashing.
//
// Callers should not depend on the exact byte length of the returned
// PNG — providers may return different sizes than requested when the
// upstream model decides to clamp.
package imagegen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Provider abstracts an image generation backend. Generate must return
// raw PNG bytes (not base64, not a URL) so the Tool can write them to
// the workspace verbatim.
type Provider interface {
	Name() string
	// Generate returns raw PNG bytes for the given prompt. Size is one
	// of "1024x1024", "1024x1792", "1792x1024" (DALL-E 3 sizes).
	Generate(ctx context.Context, prompt, size string) ([]byte, error)
}

// NoopProvider is the safe default used when no API key is configured.
// Every call returns ErrDisabled so the Coder receives a deterministic
// error string it can surface back to the user.
type NoopProvider struct{}

// ErrDisabled is returned by NoopProvider — exported so callers can
// errors.Is against it if they want to special-case "disabled" vs
// other failures.
var ErrDisabled = errors.New("image generation disabled")

// Name implements Provider.
func (NoopProvider) Name() string { return "noop" }

// Generate implements Provider — always returns ErrDisabled.
func (NoopProvider) Generate(ctx context.Context, prompt, size string) ([]byte, error) {
	return nil, ErrDisabled
}

// OpenAIImagesProvider calls the OpenAI Images API. The struct is
// constructed by the orchestrator at boot from cfg.OpenAIImageAPIKey
// (or cfg.OpenAIAPIKey as a fallback).
//
// Zero-value fields are filled in by Generate:
//   - BaseURL defaults to https://api.openai.com/v1
//   - HTTP defaults to a 60-second http.Client (image generation is
//     latency-heavy; the orchestrator's request context is the real cap)
//   - Model defaults to "gpt-image-1"
type OpenAIImagesProvider struct {
	APIKey  string
	BaseURL string
	HTTP    *http.Client
	Model   string
}

// Name implements Provider.
func (p *OpenAIImagesProvider) Name() string { return "openai-images" }

// Generate calls /images/generations and decodes the base64 PNG out
// of the first data entry. On a 4xx that complains about an unknown
// model and the configured model is not already "dall-e-3", retries
// once against dall-e-3 so older accounts work.
func (p *OpenAIImagesProvider) Generate(ctx context.Context, prompt, size string) ([]byte, error) {
	if p.APIKey == "" {
		return nil, errors.New("openai images: missing API key")
	}
	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("openai images: empty prompt")
	}
	if size == "" {
		size = "1024x1024"
	}
	model := p.Model
	if model == "" {
		model = "gpt-image-1"
	}

	body, err := p.callOnce(ctx, model, prompt, size)
	if err == nil {
		return body, nil
	}
	// Retry once on "model not found" / unknown model for accounts that
	// don't yet have gpt-image-1 access. The check is intentionally
	// loose — OpenAI's error strings vary across error envelopes.
	if model != "dall-e-3" && isModelNotFound(err) {
		return p.callOnce(ctx, "dall-e-3", prompt, size)
	}
	return nil, err
}

func (p *OpenAIImagesProvider) callOnce(ctx context.Context, model, prompt, size string) ([]byte, error) {
	base := p.BaseURL
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	client := p.HTTP
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}

	reqBody, _ := json.Marshal(map[string]any{
		"model":           model,
		"prompt":          prompt,
		"size":            size,
		"n":               1,
		"response_format": "b64_json",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/images/generations", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("openai images: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai images: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if resp.StatusCode/100 != 2 {
		trim := strings.TrimSpace(string(raw))
		if len(trim) > 400 {
			trim = trim[:400]
		}
		return nil, fmt.Errorf("openai images: %d: %s", resp.StatusCode, trim)
	}

	var out struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
			URL     string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("openai images: decode: %w", err)
	}
	if len(out.Data) == 0 || out.Data[0].B64JSON == "" {
		return nil, errors.New("openai images: empty response")
	}
	png, err := base64.StdEncoding.DecodeString(out.Data[0].B64JSON)
	if err != nil {
		return nil, fmt.Errorf("openai images: base64 decode: %w", err)
	}
	return png, nil
}

// isModelNotFound matches the various phrasings OpenAI uses for an
// unknown / unavailable model. Conservative on purpose — we want
// false negatives (no fallback) over false positives (silent fallback
// hiding a real configuration bug).
func isModelNotFound(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "model not found") ||
		strings.Contains(s, "model_not_found") ||
		strings.Contains(s, "does not exist") ||
		strings.Contains(s, "do not have access")
}
