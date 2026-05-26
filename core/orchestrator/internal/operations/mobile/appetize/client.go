package appetize

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/pkg/httpclient"
)

// DefaultBaseURL is the Appetize REST endpoint. Override via WithBaseURL
// when running against the EU region (eu1.appetize.io) or a
// self-hosted proxy.
const DefaultBaseURL = "https://api.appetize.io"

// Client is a thin REST wrapper around the Appetize API. Auth is HTTP
// Basic with the API token as username and an empty password — that's
// the convention documented at https://docs.appetize.io/authentication.
type Client struct {
	baseURL    string
	httpClient *http.Client
	apiToken   string
	logger     zerolog.Logger
}

// Option configures a Client at construction time.
type Option func(*Client)

// WithBaseURL overrides DefaultBaseURL — e.g. "https://eu1.appetize.io".
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

// WithHTTPClient swaps the default *http.Client (60s timeout) for one
// the caller controls. Use to plug a proxy / mTLS / wider timeout.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.httpClient = h } }

// WithLogger threads a zerolog instance into the client so upload /
// delete attempts appear in the orchestrator log stream.
func WithLogger(l zerolog.Logger) Option { return func(c *Client) { c.logger = l } }

// New constructs a Client. apiToken is mandatory; the caller is
// expected to have already resolved per-project vs env precedence via
// ResolveToken below.
func New(apiToken string, opts ...Option) *Client {
	c := &Client{
		baseURL:    DefaultBaseURL,
		httpClient: httpclient.Standard(60 * time.Second),
		apiToken:   apiToken,
		logger:     zerolog.Nop(),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// authHeader returns the Authorization header value for Appetize:
// Basic base64(token+":").
func (c *Client) authHeader() string {
	if c == nil || c.apiToken == "" {
		return ""
	}
	raw := c.apiToken + ":"
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(raw))
}

// UploadApp POSTs a multipart form to /v1/apps containing the artifact
// bytes plus the platform / buildName / note form fields. Streams the
// artifact through io.Pipe so the caller can pass a multi-hundred-MiB
// IPA without buffering.
func (c *Client) UploadApp(ctx context.Context, req UploadRequest) (*App, error) {
	if c == nil || c.apiToken == "" {
		return nil, errors.New("appetize: client missing api token")
	}
	if req.ArtifactReader == nil {
		return nil, errors.New("appetize: UploadRequest.ArtifactReader is nil")
	}
	if req.FileName == "" {
		return nil, errors.New("appetize: UploadRequest.FileName is empty")
	}
	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	if platform != "ios" && platform != "android" {
		return nil, fmt.Errorf("appetize: invalid platform %q (want ios|android)", req.Platform)
	}

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	// Stream the body in a goroutine so the HTTP request can read from
	// the pipe as we write the multipart parts.
	go func() {
		var err error
		defer func() {
			_ = mw.Close()
			_ = pw.CloseWithError(err)
		}()
		if err = mw.WriteField("platform", platform); err != nil {
			return
		}
		if req.BuildName != "" {
			if err = mw.WriteField("buildName", req.BuildName); err != nil {
				return
			}
		}
		if req.Note != "" {
			if err = mw.WriteField("note", req.Note); err != nil {
				return
			}
		}
		var part io.Writer
		part, err = mw.CreateFormFile("file", req.FileName)
		if err != nil {
			return
		}
		_, err = io.Copy(part, struct{ io.Reader }{req.ArtifactReader})
	}()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/apps", pr)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", c.authHeader())
	httpReq.Header.Set("Content-Type", mw.FormDataContentType())

	c.logger.Info().
		Str("file", req.FileName).
		Str("platform", platform).
		Msg("appetize: uploading app")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("appetize: upload: %w", err)
	}
	defer resp.Body.Close()
	return decodeApp(resp)
}

// GetApp fetches an existing app by publicKey.
func (c *Client) GetApp(ctx context.Context, publicKey string) (*App, error) {
	if c == nil || c.apiToken == "" {
		return nil, errors.New("appetize: client missing api token")
	}
	if publicKey == "" {
		return nil, errors.New("appetize: GetApp requires publicKey")
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/v1/apps/"+url.PathEscape(publicKey), nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", c.authHeader())
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("appetize: get: %w", err)
	}
	defer resp.Body.Close()
	return decodeApp(resp)
}

// DeleteApp removes an app from Appetize. Errors propagate; a 404 is
// treated as a successful idempotent delete.
func (c *Client) DeleteApp(ctx context.Context, publicKey string) error {
	if c == nil || c.apiToken == "" {
		return errors.New("appetize: client missing api token")
	}
	if publicKey == "" {
		return errors.New("appetize: DeleteApp requires publicKey")
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		c.baseURL+"/v1/apps/"+url.PathEscape(publicKey), nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Authorization", c.authHeader())
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("appetize: delete: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return fmt.Errorf("appetize: delete %s: status %d: %s", publicKey, resp.StatusCode, bytes.TrimSpace(body))
}

// EmbedURL builds an embeddable simulator URL for the given publicKey
// and options. Pure string concat — no network call.
func (c *Client) EmbedURL(publicKey string, opts EmbedOptions) string {
	if c == nil || publicKey == "" {
		return ""
	}
	host := strings.Replace(c.baseURL, "api.", "", 1)
	if host == "" {
		host = "https://appetize.io"
	}
	q := url.Values{}
	if opts.Device != "" {
		q.Set("device", opts.Device)
	}
	if opts.OSVersion != "" {
		q.Set("osVersion", opts.OSVersion)
	}
	if opts.DeviceColor != "" {
		q.Set("deviceColor", opts.DeviceColor)
	}
	scale := opts.Scale
	if scale <= 0 {
		scale = 75
	}
	if scale < 25 {
		scale = 25
	}
	if scale > 200 {
		scale = 200
	}
	q.Set("scale", strconv.Itoa(scale))
	if opts.Locale != "" {
		q.Set("locale", opts.Locale)
	}
	if opts.Orientation != "" {
		q.Set("orientation", opts.Orientation)
	}
	if opts.Centered {
		q.Set("centered", "true")
	}
	if opts.AutoPlay {
		q.Set("autoplay", "true")
	}
	if opts.RecordSession {
		q.Set("record", "true")
	}
	encoded := q.Encode()
	if encoded == "" {
		return host + "/embed/" + url.PathEscape(publicKey)
	}
	return host + "/embed/" + url.PathEscape(publicKey) + "?" + encoded
}

// decodeApp reads an Appetize JSON response body, distinguishing
// success (2xx) from API errors.
func decodeApp(resp *http.Response) (*App, error) {
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return nil, fmt.Errorf("appetize: read body: %w", readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("appetize: status %d: %s", resp.StatusCode, bytes.TrimSpace(body))
	}
	var app App
	if err := json.Unmarshal(body, &app); err != nil {
		return nil, fmt.Errorf("appetize: decode response: %w", err)
	}
	return &app, nil
}

// ResolveToken returns the Appetize API token for a project, preferring
// a per-project secret over the orchestrator-wide environment variable.
// The project secret key is APPETIZE_TOKEN; the env var fallback is the
// same name. Returns ("", error) when neither is present so the
// resolver can surface a typed configuration error to the operator
// instead of silently uploading to nowhere.
func ResolveToken(secrets map[string]string) (string, error) {
	if secrets != nil {
		if v, ok := secrets["APPETIZE_TOKEN"]; ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v), nil
		}
	}
	if v := strings.TrimSpace(os.Getenv("APPETIZE_TOKEN")); v != "" {
		return v, nil
	}
	return "", errors.New("appetize: no APPETIZE_TOKEN in project secrets or env")
}
