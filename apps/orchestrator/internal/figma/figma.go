// Package figma is a stdlib-only client for the Figma REST API plus
// the helpers the orchestrator needs to turn a raw Figma file into the
// design-tokens + component inventory the Coder agent can consume.
//
// Surfaces:
//
//   - Client — narrow wrapper around api.figma.com/v1 covering the
//     three endpoints we need: GetFile, GetImages, GetFileStyles.
//   - File / Node / Paint / TextStyle / StyleMeta — JSON shapes that
//     match the upstream wire format (only the fields we actually use
//     are decoded; everything else is dropped on the floor).
//   - Extract (see parse.go) — pure function that walks a File and
//     produces a DesignTokens + ComponentInventory pair.
//   - Tool (see tool.go) — the built-in `figma_import` tool the Coder
//     can invoke mid-patch to ingest a design.
//
// The package deliberately depends on nothing outside the standard
// library so adding it doesn't grow go.mod. Token + base URL are
// caller-supplied — empty token is tolerated; calls then fail with a
// readable HTTP error rather than panicking.
package figma

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBaseURL is the production Figma REST endpoint. Tests + local
// fakes can override Client.BaseURL.
const DefaultBaseURL = "https://api.figma.com/v1"

// Client wraps the Figma REST API. Public docs:
// https://www.figma.com/developers/api
//
// Zero-value fields are filled in on demand: HTTP defaults to a 30s
// http.Client; BaseURL defaults to DefaultBaseURL.
type Client struct {
	Token   string // Figma personal access token
	HTTP    *http.Client
	BaseURL string // default "https://api.figma.com/v1"
}

// New constructs a Client with sensible defaults. An empty token is
// allowed — every subsequent call will fail with a 403 from Figma,
// which we surface verbatim so the operator sees the misconfiguration.
func New(token string) *Client {
	return &Client{
		Token:   token,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
		BaseURL: DefaultBaseURL,
	}
}

// File is the top-level response of GET /files/{key}. Only the fields
// downstream code consumes are kept here; unknown fields are dropped.
type File struct {
	Name         string               `json:"name"`
	LastModified string               `json:"lastModified"`
	Document     Node                 `json:"document"`
	Styles       map[string]StyleMeta `json:"styles"`
}

// Node is a single entry in the Figma document tree. The set of fields
// we decode is intentionally narrow — the rest of the upstream schema
// (effects, layout grids, plugin data, etc.) is ignored.
type Node struct {
	ID                  string     `json:"id"`
	Name                string     `json:"name"`
	Type                string     `json:"type"` // FRAME, COMPONENT, INSTANCE, TEXT, RECTANGLE, GROUP, ...
	Children            []Node     `json:"children,omitempty"`
	AbsoluteBoundingBox *Box       `json:"absoluteBoundingBox,omitempty"`
	Fills               []Paint    `json:"fills,omitempty"`
	Strokes             []Paint    `json:"strokes,omitempty"`
	Characters          string     `json:"characters,omitempty"` // for TEXT
	Style               *TextStyle `json:"style,omitempty"`      // for TEXT
	CornerRadius        float64    `json:"cornerRadius,omitempty"`
	LayoutMode          string     `json:"layoutMode,omitempty"` // HORIZONTAL | VERTICAL | NONE
	ItemSpacing         float64    `json:"itemSpacing,omitempty"`
	// Styles maps the local style slot ("fill", "text", "stroke", ...)
	// to a styleId. We use the "fill" slot to look up named color
	// tokens via File.Styles when extracting design tokens.
	Styles map[string]string `json:"styles,omitempty"`
}

// Box is a Figma bounding box. All units are pixels.
type Box struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Paint covers FILL + STROKE entries. Only SOLID paints are surfaced
// as design tokens; gradients/images are tolerated but ignored.
type Paint struct {
	Type    string `json:"type"`
	Color   *RGBA  `json:"color,omitempty"`
	Visible bool   `json:"visible"`
}

// RGBA mirrors Figma's normalised 0..1 colour channels.
type RGBA struct {
	R float64 `json:"r"`
	G float64 `json:"g"`
	B float64 `json:"b"`
	A float64 `json:"a"`
}

// TextStyle captures the typography fields we surface as tokens.
type TextStyle struct {
	FontFamily          string  `json:"fontFamily"`
	FontPostScriptName  string  `json:"fontPostScriptName,omitempty"`
	FontSize            float64 `json:"fontSize"`
	FontWeight          float64 `json:"fontWeight"`
	LineHeightPx        float64 `json:"lineHeightPx,omitempty"`
	TextAlignHorizontal string  `json:"textAlignHorizontal,omitempty"`
}

// StyleMeta describes a named style attached to the file (colours,
// text styles, effects, grids). Returned both inside File.Styles and
// by GetFileStyles.
type StyleMeta struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	StyleType   string `json:"styleType"` // FILL | TEXT | EFFECT | GRID
	Description string `json:"description,omitempty"`
}

// Style pairs a node-id with its StyleMeta — that is what the file
// styles endpoint returns. Kept as a separate type so callers don't
// have to thread the node id through a map themselves.
type Style struct {
	NodeID string
	Meta   StyleMeta
}

// GetFile fetches the file's node tree.
func (c *Client) GetFile(ctx context.Context, fileKey string) (*File, error) {
	if strings.TrimSpace(fileKey) == "" {
		return nil, errors.New("figma: fileKey is required")
	}
	body, err := c.do(ctx, "/files/"+url.PathEscape(fileKey), nil)
	if err != nil {
		return nil, err
	}
	var out File
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("figma: decode file: %w", err)
	}
	return &out, nil
}

// GetImages renders the supplied nodeIds to PNG/SVG URLs.
// format must be one of "png", "svg", "jpg", "pdf"; empty defaults to
// "png". Returns a map { nodeId → URL }. The URLs are short-lived
// signed S3 links — callers should fetch them promptly.
func (c *Client) GetImages(ctx context.Context, fileKey string, nodeIDs []string, format string) (map[string]string, error) {
	if strings.TrimSpace(fileKey) == "" {
		return nil, errors.New("figma: fileKey is required")
	}
	if len(nodeIDs) == 0 {
		return map[string]string{}, nil
	}
	if format == "" {
		format = "png"
	}
	q := url.Values{}
	q.Set("ids", strings.Join(nodeIDs, ","))
	q.Set("format", format)
	body, err := c.do(ctx, "/images/"+url.PathEscape(fileKey), q)
	if err != nil {
		return nil, err
	}
	var out struct {
		Err    string            `json:"err"`
		Images map[string]string `json:"images"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("figma: decode images: %w", err)
	}
	if out.Err != "" {
		return nil, fmt.Errorf("figma: images: %s", out.Err)
	}
	if out.Images == nil {
		return map[string]string{}, nil
	}
	return out.Images, nil
}

// GetFileStyles returns named color + text + effect styles defined in
// the file. Used to produce the design-tokens manifest with friendly
// names rather than synthesised "color-N" placeholders.
func (c *Client) GetFileStyles(ctx context.Context, fileKey string) ([]Style, error) {
	if strings.TrimSpace(fileKey) == "" {
		return nil, errors.New("figma: fileKey is required")
	}
	body, err := c.do(ctx, "/files/"+url.PathEscape(fileKey)+"/styles", nil)
	if err != nil {
		return nil, err
	}
	// The /styles endpoint nests its rows under meta.styles.
	var out struct {
		Meta struct {
			Styles []struct {
				NodeID      string `json:"node_id"`
				Key         string `json:"key"`
				Name        string `json:"name"`
				StyleType   string `json:"style_type"`
				Description string `json:"description"`
			} `json:"styles"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("figma: decode styles: %w", err)
	}
	styles := make([]Style, 0, len(out.Meta.Styles))
	for _, s := range out.Meta.Styles {
		styles = append(styles, Style{
			NodeID: s.NodeID,
			Meta: StyleMeta{
				Key:         s.Key,
				Name:        s.Name,
				StyleType:   s.StyleType,
				Description: s.Description,
			},
		})
	}
	return styles, nil
}

// do performs a GET against the Figma API with the configured token
// header. Returns the response body on 2xx; on 4xx/5xx returns a
// wrapped error containing the status code and the first 400 bytes of
// the body so misconfiguration (bad token, missing file) is obvious in
// orchestrator logs.
func (c *Client) do(ctx context.Context, path string, query url.Values) ([]byte, error) {
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	full := strings.TrimRight(base, "/") + path
	if len(query) > 0 {
		full += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, fmt.Errorf("figma: build request: %w", err)
	}
	if c.Token != "" {
		req.Header.Set("X-Figma-Token", c.Token)
	}
	req.Header.Set("Accept", "application/json")

	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("figma: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if resp.StatusCode/100 != 2 {
		trim := strings.TrimSpace(string(raw))
		if len(trim) > 400 {
			trim = trim[:400]
		}
		return nil, fmt.Errorf("figma: %d %s: %s", resp.StatusCode, http.StatusText(resp.StatusCode), trim)
	}
	return raw, nil
}
