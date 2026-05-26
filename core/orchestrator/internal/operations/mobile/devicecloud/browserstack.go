package devicecloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"time"

	"ironflyer/core/orchestrator/internal/pkg/httpclient"
)

// BrowserStack App Live constants. The product caps interactive
// sessions at 30 minutes and exposes device + session endpoints under
// api-cloud.browserstack.com/app-live. We cache the device list for an
// hour because the catalogue rarely changes — every fresh page-load
// otherwise spends 200ms+ on a list-devices round-trip.
const (
	browserStackBaseURL       = "https://api-cloud.browserstack.com/app-live"
	browserStackMaxSession    = 30 * time.Minute
	browserStackDevicesTTL    = time.Hour
	browserStackDefaultClient = 30 * time.Second
)

// BrowserStackClient implements ProviderClient against the App Live
// REST API. The zero value is unusable — call NewBrowserStackClient.
type BrowserStackClient struct {
	username  string
	accessKey string
	http      *http.Client
	baseURL   string

	devicesMu       sync.RWMutex
	devicesCache    []Device
	devicesFetched  time.Time
}

// NewBrowserStackClient constructs a client with the operator's API
// credentials. Pass http.DefaultClient (or a per-request client with
// custom timeouts) — the constructor wraps it in a 30s default if nil.
func NewBrowserStackClient(username, accessKey string, httpClient *http.Client) *BrowserStackClient {
	if httpClient == nil {
		httpClient = httpclient.Standard(browserStackDefaultClient)
	}
	return &BrowserStackClient{
		username:  username,
		accessKey: accessKey,
		http:      httpClient,
		baseURL:   browserStackBaseURL,
	}
}

// Name returns ProviderBrowserStack.
func (c *BrowserStackClient) Name() Provider { return ProviderBrowserStack }

// ListDevices returns the cached catalogue, refreshing in the
// background when the TTL has elapsed. platform is optional — pass
// "android" or "ios" to filter; empty string returns everything.
func (c *BrowserStackClient) ListDevices(ctx context.Context, platform string) ([]Device, error) {
	if devs, ok := c.cachedDevices(); ok {
		return filterByPlatform(devs, platform), nil
	}
	fresh, err := c.fetchDevices(ctx)
	if err != nil {
		// Fall back to whatever we have cached, even if stale — a 502
		// from BrowserStack should not freeze the device picker. Only
		// surface the error when the cache is empty.
		c.devicesMu.RLock()
		stale := append([]Device(nil), c.devicesCache...)
		c.devicesMu.RUnlock()
		if len(stale) > 0 {
			return filterByPlatform(stale, platform), nil
		}
		return nil, err
	}
	c.devicesMu.Lock()
	c.devicesCache = fresh
	c.devicesFetched = time.Now()
	c.devicesMu.Unlock()
	return filterByPlatform(fresh, platform), nil
}

func (c *BrowserStackClient) cachedDevices() ([]Device, bool) {
	c.devicesMu.RLock()
	defer c.devicesMu.RUnlock()
	if c.devicesFetched.IsZero() {
		return nil, false
	}
	if time.Since(c.devicesFetched) > browserStackDevicesTTL {
		return nil, false
	}
	out := append([]Device(nil), c.devicesCache...)
	return out, true
}

// fetchDevices calls GET /devices and translates the BrowserStack
// catalogue into the shared Device shape. The upstream response is a
// flat array of `{device, os, os_version, real_mobile}` objects.
func (c *BrowserStackClient) fetchDevices(ctx context.Context) ([]Device, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/devices.json", nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.username, c.accessKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
		return nil, fmt.Errorf("browserstack: list devices: %s: %s", resp.Status, string(body))
	}
	var raw []struct {
		Device     string `json:"device"`
		OS         string `json:"os"`
		OSVersion  string `json:"os_version"`
		RealMobile bool   `json:"real_mobile"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("browserstack: decode devices: %w", err)
	}
	out := make([]Device, 0, len(raw))
	for _, d := range raw {
		platform := strings.ToLower(d.OS)
		out = append(out, Device{
			ID:           fmt.Sprintf("%s|%s|%s", platform, d.Device, d.OSVersion),
			Provider:     ProviderBrowserStack,
			Platform:     platform,
			OSVersion:    d.OSVersion,
			Model:        d.Device,
			Manufacturer: inferManufacturer(d.Device),
			Real:         d.RealMobile,
		})
	}
	return out, nil
}

func inferManufacturer(model string) string {
	low := strings.ToLower(model)
	switch {
	case strings.HasPrefix(low, "iphone"), strings.HasPrefix(low, "ipad"):
		return "Apple"
	case strings.HasPrefix(low, "pixel"):
		return "Google"
	case strings.HasPrefix(low, "galaxy"):
		return "Samsung"
	case strings.HasPrefix(low, "oneplus"):
		return "OnePlus"
	case strings.HasPrefix(low, "redmi"), strings.HasPrefix(low, "xiaomi"), strings.HasPrefix(low, "mi "):
		return "Xiaomi"
	}
	return ""
}

func filterByPlatform(devs []Device, platform string) []Device {
	p := strings.ToLower(strings.TrimSpace(platform))
	if p == "" {
		return devs
	}
	out := make([]Device, 0, len(devs))
	for _, d := range devs {
		if d.Platform == p {
			out = append(out, d)
		}
	}
	return out
}

// UploadApp performs a multipart POST /upload of an .apk/.ipa. The
// response carries the bs://<hash> URL that StartSession then consumes.
func (c *BrowserStackClient) UploadApp(ctx context.Context, artifactReader io.Reader, fileName string) (string, error) {
	if artifactReader == nil {
		return "", errors.New("browserstack: nil artifact reader")
	}
	if strings.TrimSpace(fileName) == "" {
		return "", errors.New("browserstack: empty file name")
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", fileName)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(fw, artifactReader); err != nil {
		return "", err
	}
	if err := mw.Close(); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/upload", &buf)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(c.username, c.accessKey)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
		return "", fmt.Errorf("browserstack: upload: %s: %s", resp.Status, string(body))
	}
	var out struct {
		AppURL string `json:"app_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("browserstack: decode upload: %w", err)
	}
	if out.AppURL == "" {
		return "", errors.New("browserstack: upload returned empty app_url")
	}
	return out.AppURL, nil
}

// StartSession creates an interactive session against the supplied
// device. SessionLength is clamped to the App Live 30-minute ceiling.
// The returned Session carries the embeddable PublicURL so the cockpit
// can drop it straight into an <iframe>.
func (c *BrowserStackClient) StartSession(ctx context.Context, req StartSessionRequest) (*Session, error) {
	if req.AppURL == "" {
		return nil, errors.New("browserstack: app url required")
	}
	if req.DeviceID == "" {
		return nil, errors.New("browserstack: device id required")
	}
	length := req.SessionLength
	if length <= 0 || length > browserStackMaxSession {
		length = browserStackMaxSession
	}
	device, osVersion := splitDeviceID(req.DeviceID)
	body := map[string]any{
		"app":            req.AppURL,
		"device":         device,
		"os_version":     osVersion,
		"session_length": int(length / time.Minute),
	}
	enc, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/session", bytes.NewReader(enc))
	if err != nil {
		return nil, err
	}
	httpReq.SetBasicAuth(c.username, c.accessKey)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		buf, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
		return nil, fmt.Errorf("browserstack: start session: %s: %s", resp.Status, string(buf))
	}
	var out struct {
		HashedID  string `json:"hashed_id"`
		PublicURL string `json:"public_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("browserstack: decode session: %w", err)
	}
	now := time.Now().UTC()
	return &Session{
		ID:         out.HashedID,
		Provider:   ProviderBrowserStack,
		DeviceID:   req.DeviceID,
		AppURL:     req.AppURL,
		SessionURL: out.PublicURL,
		Status:     "running",
		StartedAt:  now,
		ExpiresAt:  now.Add(length),
	}, nil
}

func splitDeviceID(id string) (device, osVersion string) {
	parts := strings.SplitN(id, "|", 3)
	if len(parts) == 3 {
		return parts[1], parts[2]
	}
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return id, ""
}

// EndSession terminates a running session by hashed_id.
func (c *BrowserStackClient) EndSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return errors.New("browserstack: empty session id")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/session/"+sessionID, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.username, c.accessKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
		return fmt.Errorf("browserstack: end session: %s: %s", resp.Status, string(body))
	}
	return nil
}

// GetSession fetches the current state of a session. BrowserStack's
// session detail endpoint mirrors the start payload plus a status
// field; we map it onto the shared Session shape.
func (c *BrowserStackClient) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	if sessionID == "" {
		return nil, errors.New("browserstack: empty session id")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/session/"+sessionID, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.username, c.accessKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
		return nil, fmt.Errorf("browserstack: get session: %s: %s", resp.Status, string(body))
	}
	var out struct {
		HashedID   string `json:"hashed_id"`
		PublicURL  string `json:"public_url"`
		Status     string `json:"status"`
		Device     string `json:"device"`
		OSVersion  string `json:"os_version"`
		StartedAt  string `json:"started_at"`
		ExpiresAt  string `json:"expires_at"`
		Duration   int    `json:"duration_minutes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("browserstack: decode get session: %w", err)
	}
	started, _ := time.Parse(time.RFC3339, out.StartedAt)
	expires, _ := time.Parse(time.RFC3339, out.ExpiresAt)
	status := strings.ToLower(out.Status)
	if status == "" {
		status = "running"
	}
	deviceID := out.Device
	if out.OSVersion != "" {
		deviceID = out.Device + "|" + out.OSVersion
	}
	return &Session{
		ID:                  out.HashedID,
		Provider:            ProviderBrowserStack,
		DeviceID:            deviceID,
		SessionURL:          out.PublicURL,
		Status:              status,
		StartedAt:           started,
		ExpiresAt:           expires,
		BillableMinutesUsed: float64(out.Duration),
	}, nil
}
