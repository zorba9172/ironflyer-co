package mobile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"ironflyer/core/runtime/internal/pkg/httpclient"
)

// bridgeAllocator is the narrow HTTP client the manager uses to ask
// the scrcpy-bridge service (a sibling Go service in
// `clients/scrcpy-bridge/`) to allocate a streaming session for a freshly
// booted emulator. It is intentionally minimal: a misconfigured
// bridge must not block emulator allocation — the caller falls back
// to the legacy in-runtime placeholder URL when this allocator
// returns an error.
type bridgeAllocator struct {
	baseURL string
	token   string
	client  *http.Client
}

// bridgeAllocatorFromEnv reads IRONFLYER_BRIDGE_URL and
// IRONFLYER_BRIDGE_TOKEN from the runtime env. When either is missing
// the allocator is nil — the manager treats that as "no bridge wired
// up; fall back to the legacy URL".
func bridgeAllocatorFromEnv() *bridgeAllocator {
	url := strings.TrimSpace(os.Getenv("IRONFLYER_BRIDGE_URL"))
	token := strings.TrimSpace(os.Getenv("IRONFLYER_BRIDGE_TOKEN"))
	if url == "" || token == "" {
		return nil
	}
	return &bridgeAllocator{
		baseURL: strings.TrimRight(url, "/"),
		token:   token,
		client:  httpclient.Standard(8 * time.Second),
	}
}

// bridgeSession mirrors the JSON shape the bridge returns. We only
// decode the fields the runtime hands back to the orchestrator.
type bridgeSession struct {
	SessionID  string `json:"sessionId"`
	WSEndpoint string `json:"wsEndpoint"`
}

// Allocate creates a session against the bridge and returns the
// fully-qualified WebSocket URL the frontend should dial. The
// returned URL embeds the shared token as a query param because
// browser WebSocket clients can't set custom headers on the upgrade.
func (b *bridgeAllocator) Allocate(ctx context.Context, workspaceID, emulatorSerial string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"workspaceId":    workspaceID,
		"emulatorSerial": emulatorSerial,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		b.baseURL+"/v1/sessions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Ironflyer-Bridge-Token", b.token)
	resp, err := b.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("bridge POST: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		tail, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("bridge POST: status %d: %s", resp.StatusCode, strings.TrimSpace(string(tail)))
	}
	var session bridgeSession
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<10)).Decode(&session); err != nil {
		return "", fmt.Errorf("bridge decode: %w", err)
	}
	if session.WSEndpoint == "" {
		return "", fmt.Errorf("bridge returned empty wsEndpoint")
	}
	// Build the absolute URL the browser will dial. We swap the
	// http(s) scheme for ws(s) and append the shared token as
	// ?token=... because browsers can't set a custom WS header.
	scheme := "wss"
	if strings.HasPrefix(b.baseURL, "http://") {
		scheme = "ws"
	}
	host := strings.TrimPrefix(strings.TrimPrefix(b.baseURL, "http://"), "https://")
	sep := "?"
	if strings.Contains(session.WSEndpoint, "?") {
		sep = "&"
	}
	return fmt.Sprintf("%s://%s%s%stoken=%s", scheme, host, session.WSEndpoint, sep, b.token), nil
}
