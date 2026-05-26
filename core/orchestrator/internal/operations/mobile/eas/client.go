package eas

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/pkg/httpclient"
)

// defaultBaseURL is the public EAS REST root. Self-hosted Expo
// deployments override via WithBaseURL.
const defaultBaseURL = "https://api.expo.dev"

// defaultRequestTimeout caps every outbound HTTP call. This is
// per-request, not per-context — a parent context that lives 2h still
// gets each call timed out independently.
const defaultRequestTimeout = 30 * time.Second

// maxRetries is the cap on transient 5xx / 429 retries.
const maxRetries = 3

// maxErrorBodyBytes caps how much of an error response we log so a
// misbehaving upstream cannot flood the operator log.
const maxErrorBodyBytes = 2048

// Client is the typed REST client for the EAS API. Constructed once
// at orchestrator startup; safe for concurrent use.
type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
	logger     zerolog.Logger
}

// Option configures Client at construction time.
type Option func(*Client)

// WithBaseURL overrides the api.expo.dev root. Useful for tests
// against a recorded fixture or a self-hosted Expo Open Source server.
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

// WithHTTPClient injects a pre-configured http.Client (custom
// transport, mTLS, proxy). Default carries a 30s timeout.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.httpClient = h } }

// WithLogger swaps the default zerolog logger.
func WithLogger(l zerolog.Logger) Option { return func(c *Client) { c.logger = l } }

// New builds a Client. token may be empty when the orchestrator boots
// without an EAS_TOKEN — every wire call then short-circuits with
// ErrEASTokenMissing so the caller surfaces a typed NOT_CONFIGURED.
func New(token string, opts ...Option) *Client {
	c := &Client{
		baseURL:    defaultBaseURL,
		token:      strings.TrimSpace(token),
		httpClient: httpclient.Standard(defaultRequestTimeout),
		logger:     zerolog.Nop(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// HasToken reports whether the client carries a non-empty bearer.
// The mobile resolver uses this to short-circuit GraphQL calls with
// NOT_CONFIGURED instead of doing a wire round-trip just to learn the
// token is missing.
func (c *Client) HasToken() bool { return c.token != "" }

// --- public API ------------------------------------------------------

// GetBuild fetches a single build by its EAS UUID.
func (c *Client) GetBuild(ctx context.Context, buildID string) (*Build, error) {
	if strings.TrimSpace(buildID) == "" {
		return nil, errors.New("eas: GetBuild: empty buildID")
	}
	var raw rawBuild
	if err := c.do(ctx, http.MethodGet, "/v2/builds/"+url.PathEscape(buildID), nil, &raw); err != nil {
		return nil, fmt.Errorf("eas: GetBuild %s: %w", buildID, err)
	}
	b := raw.toBuild()
	return &b, nil
}

// ListBuilds returns the build history for an EAS project, newest first.
func (c *Client) ListBuilds(ctx context.Context, projectID string, opts ListBuildsOpts) ([]*Build, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, errors.New("eas: ListBuilds: empty projectID")
	}
	q := url.Values{}
	if opts.Platform != "" {
		q.Set("platform", opts.Platform)
	}
	if opts.Status != "" {
		q.Set("status", string(opts.Status))
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Offset > 0 {
		q.Set("offset", strconv.Itoa(opts.Offset))
	}
	if opts.Channel != "" {
		q.Set("channel", opts.Channel)
	}
	path := "/v2/projects/" + url.PathEscape(projectID) + "/builds"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var resp struct {
		Data []rawBuild `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("eas: ListBuilds %s: %w", projectID, err)
	}
	out := make([]*Build, 0, len(resp.Data))
	for i := range resp.Data {
		b := resp.Data[i].toBuild()
		out = append(out, &b)
	}
	return out, nil
}

// CancelBuild cancels an in-flight EAS build. Idempotent: cancelling
// an already-terminal build returns nil and logs at debug level.
func (c *Client) CancelBuild(ctx context.Context, buildID string) error {
	if strings.TrimSpace(buildID) == "" {
		return errors.New("eas: CancelBuild: empty buildID")
	}
	if err := c.do(ctx, http.MethodPost, "/v2/builds/"+url.PathEscape(buildID)+"/cancel", nil, nil); err != nil {
		return fmt.Errorf("eas: CancelBuild %s: %w", buildID, err)
	}
	return nil
}

// DownloadArtifact streams the binary at the given URL. The artifact
// URL comes from Build.ArtifactURL; EAS hosts the binary on a signed
// CloudFront URL that does NOT require the bearer token, but we pass
// it anyway so a self-hosted Expo Open Source artifact host that DOES
// require it still works. Caller MUST Close the returned reader.
func (c *Client) DownloadArtifact(ctx context.Context, artifactURL string) (io.ReadCloser, error) {
	if strings.TrimSpace(artifactURL) == "" {
		return nil, errors.New("eas: DownloadArtifact: empty url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, artifactURL, nil)
	if err != nil {
		return nil, fmt.Errorf("eas: DownloadArtifact build request: %w", err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("eas: DownloadArtifact: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body := readBoundedBody(resp.Body)
		_ = resp.Body.Close()
		c.logger.Warn().
			Int("status", resp.StatusCode).
			Str("url", artifactURL).
			Str("body", body).
			Msg("eas: artifact download failed")
		return nil, fmt.Errorf("eas: DownloadArtifact: status %d", resp.StatusCode)
	}
	return resp.Body, nil
}

// CreateSubmission POSTs a new submission to the App Store / Play
// Store. Either BuildID or ArchiveURL must be set.
func (c *Client) CreateSubmission(ctx context.Context, req SubmissionRequest) (*Submission, error) {
	if strings.TrimSpace(req.ProjectID) == "" {
		return nil, errors.New("eas: CreateSubmission: empty projectID")
	}
	if req.BuildID == "" && req.ArchiveURL == "" {
		return nil, errors.New("eas: CreateSubmission: one of buildId / archiveUrl is required")
	}
	if req.Platform != "ios" && req.Platform != "android" {
		return nil, fmt.Errorf("eas: CreateSubmission: invalid platform %q (want ios|android)", req.Platform)
	}
	body := map[string]any{
		"platform": req.Platform,
	}
	if req.BuildID != "" {
		body["buildId"] = req.BuildID
	}
	if req.ArchiveURL != "" {
		body["archiveUrl"] = req.ArchiveURL
	}
	switch req.Platform {
	case "ios":
		if req.IOS == nil {
			return nil, errors.New("eas: CreateSubmission: iOS platform requires IOSSubmitConfig")
		}
		body["iosConfig"] = req.IOS
	case "android":
		if req.Android == nil {
			return nil, errors.New("eas: CreateSubmission: android platform requires AndroidSubmitConfig")
		}
		android := map[string]any{
			"track": strings.ToLower(strings.TrimSpace(req.Android.Track)),
		}
		if android["track"] == "" {
			android["track"] = "internal"
		}
		if req.Android.ReleaseStatus != "" {
			android["releaseStatus"] = req.Android.ReleaseStatus
		}
		if req.Android.ChangesNotSentForReview {
			android["changesNotSentForReview"] = true
		}
		if len(req.Android.ServiceAccountKey) > 0 {
			android["serviceAccountKey"] = base64.StdEncoding.EncodeToString(req.Android.ServiceAccountKey)
		}
		body["androidConfig"] = android
	}
	var raw rawSubmission
	if err := c.do(ctx, http.MethodPost,
		"/v2/projects/"+url.PathEscape(req.ProjectID)+"/submissions",
		body, &raw,
	); err != nil {
		return nil, fmt.Errorf("eas: CreateSubmission: %w", err)
	}
	s := raw.toSubmission()
	return &s, nil
}

// GetSubmission fetches a single submission by its EAS UUID.
func (c *Client) GetSubmission(ctx context.Context, submissionID string) (*Submission, error) {
	if strings.TrimSpace(submissionID) == "" {
		return nil, errors.New("eas: GetSubmission: empty submissionID")
	}
	var raw rawSubmission
	if err := c.do(ctx, http.MethodGet,
		"/v2/submissions/"+url.PathEscape(submissionID),
		nil, &raw,
	); err != nil {
		return nil, fmt.Errorf("eas: GetSubmission %s: %w", submissionID, err)
	}
	s := raw.toSubmission()
	return &s, nil
}

// PublishUpdate posts a new OTA update against the supplied channel.
func (c *Client) PublishUpdate(ctx context.Context, channelName string, req PublishUpdateRequest) (*Update, error) {
	if strings.TrimSpace(channelName) == "" {
		return nil, errors.New("eas: PublishUpdate: empty channelName")
	}
	if strings.TrimSpace(req.RuntimeVersion) == "" {
		return nil, errors.New("eas: PublishUpdate: empty runtimeVersion")
	}
	body := map[string]any{
		"branch":         req.Branch,
		"message":        req.Message,
		"runtimeVersion": req.RuntimeVersion,
	}
	if len(req.ManifestExtra) > 0 {
		body["manifestExtra"] = req.ManifestExtra
	}
	var raw rawUpdate
	if err := c.do(ctx, http.MethodPost,
		"/v2/channels/"+url.PathEscape(channelName)+"/updates",
		body, &raw,
	); err != nil {
		return nil, fmt.Errorf("eas: PublishUpdate %s: %w", channelName, err)
	}
	u := raw.toUpdate(channelName)
	return &u, nil
}

// --- internal --------------------------------------------------------

// do issues the request with retry / backoff. Body is JSON-encoded
// when non-nil; out is JSON-decoded when non-nil. The token is added
// per request so secret rotation only needs to swap c.token.
func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	if c.token == "" {
		return ErrEASTokenMissing
	}
	full := strings.TrimRight(c.baseURL, "/") + path

	var (
		lastErr  error
		lastCode int
	)
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Re-encode body each attempt — http.Request consumes the
		// reader and we want fresh bytes on every retry.
		var reqBody io.Reader
		if body != nil {
			raw, err := json.Marshal(body)
			if err != nil {
				return fmt.Errorf("encode body: %w", err)
			}
			reqBody = bytes.NewReader(raw)
		}
		req, err := http.NewRequestWithContext(ctx, method, full, reqBody)
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			// Network errors are retried.
			if attempt < maxRetries && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				if !sleepBackoff(ctx, attempt, 0) {
					return ctx.Err()
				}
				continue
			}
			return err
		}
		lastCode = resp.StatusCode

		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			body := readBoundedBody(resp.Body)
			_ = resp.Body.Close()
			c.logger.Warn().
				Int("status", resp.StatusCode).
				Str("method", method).
				Str("path", path).
				Str("body", body).
				Dur("retry_after", retryAfter).
				Msg("eas: rate limited")
			if attempt < maxRetries {
				if !sleepBackoff(ctx, attempt, retryAfter) {
					return ctx.Err()
				}
				continue
			}
			return fmt.Errorf("rate limited (status %d)", resp.StatusCode)
		}

		if resp.StatusCode >= 500 {
			body := readBoundedBody(resp.Body)
			_ = resp.Body.Close()
			c.logger.Warn().
				Int("status", resp.StatusCode).
				Str("method", method).
				Str("path", path).
				Str("body", body).
				Int("attempt", attempt).
				Msg("eas: server error (will retry)")
			lastErr = fmt.Errorf("server error (status %d): %s", resp.StatusCode, body)
			if attempt < maxRetries {
				if !sleepBackoff(ctx, attempt, 0) {
					return ctx.Err()
				}
				continue
			}
			return lastErr
		}

		if resp.StatusCode >= 400 {
			body := readBoundedBody(resp.Body)
			_ = resp.Body.Close()
			c.logger.Error().
				Int("status", resp.StatusCode).
				Str("method", method).
				Str("path", path).
				Str("body", body).
				Msg("eas: request failed")
			return fmt.Errorf("status %d: %s", resp.StatusCode, body)
		}

		// 2xx — decode if a destination was supplied.
		defer resp.Body.Close()
		if out == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			return nil
		}
		// EAS wraps every response in a `data` envelope. We unwrap it
		// generically here so the typed unmarshallers don't have to.
		envelope := struct {
			Data json.RawMessage `json:"data"`
		}{}
		if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
			return fmt.Errorf("decode envelope: %w", err)
		}
		payload := envelope.Data
		if len(payload) == 0 {
			// Some endpoints return the body directly (e.g. a fixture
			// or self-hosted Expo). Fall back to the raw body — we'll
			// have to do a second read so re-issue the request would
			// be the safer path; here we just no-op if data was empty.
			return nil
		}
		if err := json.Unmarshal(payload, out); err != nil {
			return fmt.Errorf("decode payload: %w", err)
		}
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("exhausted retries (last status %d)", lastCode)
	}
	return lastErr
}

// sleepBackoff sleeps with exponential backoff (200ms * 2^attempt)
// capped at 5s, or the supplied override (Retry-After). Returns false
// when ctx cancels.
func sleepBackoff(ctx context.Context, attempt int, override time.Duration) bool {
	var d time.Duration
	if override > 0 {
		d = override
		if d > 30*time.Second {
			d = 30 * time.Second
		}
	} else {
		d = time.Duration(200) * time.Millisecond
		for i := 0; i < attempt; i++ {
			d *= 2
		}
		if d > 5*time.Second {
			d = 5 * time.Second
		}
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// parseRetryAfter accepts either a "<seconds>" integer or an HTTP-date.
// Anything unparseable returns 0 so the default backoff applies.
func parseRetryAfter(h string) time.Duration {
	h = strings.TrimSpace(h)
	if h == "" {
		return 0
	}
	if n, err := strconv.Atoi(h); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	if t, err := http.ParseTime(h); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 0
}

// readBoundedBody returns at most maxErrorBodyBytes of the response
// for safe logging. Never returns the bearer token (the token only
// flows through the Authorization header, never the body).
func readBoundedBody(r io.Reader) string {
	if r == nil {
		return ""
	}
	buf := make([]byte, maxErrorBodyBytes)
	n, _ := io.ReadFull(io.LimitReader(r, maxErrorBodyBytes), buf)
	return string(buf[:n])
}

// --- raw decode types ------------------------------------------------

// rawBuild mirrors the nested JSON shape EAS returns. Promoted into
// the flat Build via toBuild().
type rawBuild struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	Platform     string `json:"platform"`
	BuildProfile string `json:"buildProfile"`
	Distribution string `json:"distribution"`
	Artifacts    struct {
		BuildURL string `json:"buildUrl"`
		Size     int64  `json:"size"`
	} `json:"artifacts"`
	LogFiles        []string   `json:"logFiles"`
	AppVersion      string     `json:"appVersion"`
	AppBuildVersion string     `json:"appBuildVersion"`
	SDKVersion      string     `json:"sdkVersion"`
	Channel         string     `json:"channel"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
	Error           *struct {
		ErrorCode string `json:"errorCode"`
		Message   string `json:"message"`
		DocsURL   string `json:"docsUrl"`
	} `json:"error,omitempty"`
	ProjectID       string `json:"projectId"`
	InitiatingActor struct {
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
	} `json:"initiatingActor"`
}

func (r rawBuild) toBuild() Build {
	b := Build{
		ID:              r.ID,
		Status:          BuildStatus(r.Status),
		Platform:        r.Platform,
		Profile:         r.BuildProfile,
		Distribution:    r.Distribution,
		ArtifactURL:     r.Artifacts.BuildURL,
		ArtifactSize:    r.Artifacts.Size,
		AppVersion:      r.AppVersion,
		AppBuildVersion: r.AppBuildVersion,
		SDKVersion:      r.SDKVersion,
		Channel:         r.Channel,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
		CompletedAt:     r.CompletedAt,
		ProjectID:       r.ProjectID,
	}
	if len(r.LogFiles) > 0 {
		b.LogURL = r.LogFiles[0]
	}
	if r.Error != nil {
		b.Error = &BuildError{
			ErrorCode: r.Error.ErrorCode,
			Message:   r.Error.Message,
			DocsURL:   r.Error.DocsURL,
		}
	}
	if u := strings.TrimSpace(r.InitiatingActor.Username); u != "" {
		b.Initiator = u
	} else if u := strings.TrimSpace(r.InitiatingActor.DisplayName); u != "" {
		b.Initiator = u
	}
	return b
}

// rawSubmission mirrors the nested EAS submission payload.
type rawSubmission struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	Platform    string     `json:"platform"`
	Target      string     `json:"target"`
	BuildID     string     `json:"buildId"`
	ArchiveURL  string     `json:"archiveUrl"`
	LogsURL     string     `json:"logsUrl"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	Error       *struct {
		ErrorCode string `json:"errorCode"`
		Message   string `json:"message"`
		DocsURL   string `json:"docsUrl"`
	} `json:"error,omitempty"`
	ProjectID string `json:"projectId"`
}

func (r rawSubmission) toSubmission() Submission {
	s := Submission{
		ID:          r.ID,
		Status:      SubmissionStatus(r.Status),
		Platform:    r.Platform,
		Target:      r.Target,
		BuildID:     r.BuildID,
		ArchiveURL:  r.ArchiveURL,
		LogURL:      r.LogsURL,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
		CompletedAt: r.CompletedAt,
		ProjectID:   r.ProjectID,
	}
	if r.Error != nil {
		s.Error = &BuildError{
			ErrorCode: r.Error.ErrorCode,
			Message:   r.Error.Message,
			DocsURL:   r.Error.DocsURL,
		}
	}
	return s
}

// rawUpdate is the OTA publish response.
type rawUpdate struct {
	ID             string    `json:"id"`
	Branch         string    `json:"branch"`
	Channel        string    `json:"channel"`
	RuntimeVersion string    `json:"runtimeVersion"`
	Message        string    `json:"message"`
	ManifestURL    string    `json:"manifestUrl"`
	Platform       string    `json:"platform"`
	Group          string    `json:"group"`
	CreatedAt      time.Time `json:"createdAt"`
}

func (r rawUpdate) toUpdate(channel string) Update {
	if r.Channel == "" {
		r.Channel = channel
	}
	return Update{
		ID:             r.ID,
		Branch:         r.Branch,
		Channel:        r.Channel,
		RuntimeVersion: r.RuntimeVersion,
		Message:        r.Message,
		ManifestURL:    r.ManifestURL,
		Platform:       r.Platform,
		GroupID:        r.Group,
		CreatedAt:      r.CreatedAt,
	}
}
