// Package figma is a production client for the Figma REST API. It turns a
// Figma file key into a structured Extract — design tokens (colors,
// typography, spacing, radii), a component inventory, and per-frame image
// renders — that the figma-translator agent consumes to materialise a
// pixel-accurate UI in the project's stack.
//
// Auth is a personal access token (FIGMA_TOKEN), sent as the X-Figma-Token
// header. Every method takes a context.Context; a default timeout guards
// against a hung upstream. See https://www.figma.com/developers/api.
package figma

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.figma.com"

// maxRenderNodes caps how many frames/components we ask Figma to render in one
// image call so a huge file can't trigger a giant batch render.
const maxRenderNodes = 60

// Client talks to the Figma REST API.
type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

// Option customises the client.
type Option func(*Client)

// WithBaseURL overrides the API base (tests, proxy).
func WithBaseURL(base string) Option {
	return func(c *Client) {
		if b := strings.TrimRight(strings.TrimSpace(base), "/"); b != "" {
			c.baseURL = b
		}
	}
}

// WithHTTPClient injects a custom http.Client.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) {
		if h != nil {
			c.httpClient = h
		}
	}
}

// New builds a Client. token is required; New returns nil when it is empty so
// callers can treat "no token" as "feature disabled".
func New(token string, opts ...Option) *Client {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	c := &Client{
		token:      token,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// --- Figma REST wire types (decode only what we use) -------------------

type apiColor struct {
	R float64 `json:"r"`
	G float64 `json:"g"`
	B float64 `json:"b"`
	A float64 `json:"a"`
}

type apiPaint struct {
	Type    string    `json:"type"`
	Color   *apiColor `json:"color"`
	Opacity *float64  `json:"opacity"`
	Visible *bool     `json:"visible"`
}

type apiTypeStyle struct {
	FontFamily   string  `json:"fontFamily"`
	FontWeight   float64 `json:"fontWeight"`
	FontSize     float64 `json:"fontSize"`
	LineHeightPx float64 `json:"lineHeightPx"`
}

type apiRect struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type apiNode struct {
	ID                  string        `json:"id"`
	Name                string        `json:"name"`
	Type                string        `json:"type"`
	Fills               []apiPaint    `json:"fills"`
	Children            []apiNode     `json:"children"`
	AbsoluteBoundingBox *apiRect      `json:"absoluteBoundingBox"`
	LayoutMode          string        `json:"layoutMode"`
	ItemSpacing         float64       `json:"itemSpacing"`
	PaddingLeft         float64       `json:"paddingLeft"`
	PaddingRight        float64       `json:"paddingRight"`
	PaddingTop          float64       `json:"paddingTop"`
	PaddingBottom       float64       `json:"paddingBottom"`
	CornerRadius        float64       `json:"cornerRadius"`
	Characters          string        `json:"characters"`
	Style               *apiTypeStyle `json:"style"`
}

type fileResponse struct {
	Name       string  `json:"name"`
	Document   apiNode `json:"document"`
	Components map[string]struct {
		Name string `json:"name"`
	} `json:"components"`
}

type imagesResponse struct {
	Err    *string           `json:"err"`
	Images map[string]string `json:"images"`
}

// --- Public Extract shape ----------------------------------------------

// ColorToken is one resolved solid color. Alpha is 1.0 when fully opaque.
type ColorToken struct {
	Hex   string  `json:"hex"`
	Alpha float64 `json:"alpha"`
}

// TypographyToken is a distinct text style observed in the file.
type TypographyToken struct {
	FontFamily string  `json:"fontFamily"`
	FontSize   float64 `json:"fontSize"`
	FontWeight float64 `json:"fontWeight"`
	LineHeight float64 `json:"lineHeight"`
}

// ComponentInfo is one entry in the component inventory.
type ComponentInfo struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	LayoutMode string  `json:"layoutMode"`
	Width      float64 `json:"width"`
	Height     float64 `json:"height"`
	Children   int     `json:"children"`
}

// FrameInfo is a top-level screen with its rendered image URL.
type FrameInfo struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Width    float64 `json:"width"`
	Height   float64 `json:"height"`
	ImageURL string  `json:"imageUrl"`
}

// Extract is the structured handoff the figma-translator agent consumes.
type Extract struct {
	FileKey    string            `json:"fileKey"`
	Name       string            `json:"name"`
	Colors     []ColorToken      `json:"colors"`
	Typography []TypographyToken `json:"typography"`
	Spacing    []float64         `json:"spacing"`
	Radii      []float64         `json:"radii"`
	Components []ComponentInfo   `json:"components"`
	Frames     []FrameInfo       `json:"frames"`
}

// Extract fetches the file, walks the node tree to derive design tokens + a
// component inventory + the top-level frames, then renders those frames to PNG
// URLs. A failure to render images is non-fatal — the token/component data is
// still returned so the translator can proceed.
func (c *Client) Extract(ctx context.Context, fileKey string) (*Extract, error) {
	fileKey = strings.TrimSpace(fileKey)
	if fileKey == "" {
		return nil, fmt.Errorf("figma: empty file key")
	}
	var file fileResponse
	if err := c.get(ctx, "/v1/files/"+url.PathEscape(fileKey), nil, &file); err != nil {
		return nil, err
	}

	w := &walker{components: make(map[string]bool, len(file.Components))}
	for id := range file.Components {
		w.components[id] = true
	}
	w.walk(&file.Document, 0)

	ex := &Extract{
		FileKey:    fileKey,
		Name:       file.Name,
		Colors:     w.colors(),
		Typography: w.typography(),
		Spacing:    w.sortedSet(w.spacing),
		Radii:      w.sortedSet(w.radii),
		Components: w.componentList,
		Frames:     w.frameList,
	}

	// Render the frames (cap the batch). Non-fatal on error.
	if len(ex.Frames) > 0 {
		ids := make([]string, 0, len(ex.Frames))
		for _, f := range ex.Frames {
			if len(ids) >= maxRenderNodes {
				break
			}
			ids = append(ids, f.ID)
		}
		if imgs, err := c.renderImages(ctx, fileKey, ids); err == nil {
			for i := range ex.Frames {
				if u := imgs[ex.Frames[i].ID]; u != "" {
					ex.Frames[i].ImageURL = u
				}
			}
		}
	}
	return ex, nil
}

// renderImages asks Figma to render the given node ids to PNG at 2x and returns
// nodeID -> image URL.
func (c *Client) renderImages(ctx context.Context, fileKey string, ids []string) (map[string]string, error) {
	if len(ids) == 0 {
		return map[string]string{}, nil
	}
	q := url.Values{}
	q.Set("ids", strings.Join(ids, ","))
	q.Set("format", "png")
	q.Set("scale", "2")
	var out imagesResponse
	if err := c.get(ctx, "/v1/images/"+url.PathEscape(fileKey), q, &out); err != nil {
		return nil, err
	}
	if out.Err != nil && *out.Err != "" {
		return nil, fmt.Errorf("figma: render images: %s", *out.Err)
	}
	return out.Images, nil
}

// get issues an authenticated GET and decodes the JSON body into out.
func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	u := c.baseURL + path
	if enc := q.Encode(); enc != "" {
		u += "?" + enc
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("figma: build request: %w", err)
	}
	req.Header.Set("X-Figma-Token", c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("figma: request %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	switch {
	case resp.StatusCode == http.StatusForbidden, resp.StatusCode == http.StatusUnauthorized:
		return fmt.Errorf("figma: unauthorized — check FIGMA_TOKEN and file access")
	case resp.StatusCode == http.StatusNotFound:
		return fmt.Errorf("figma: file not found")
	case resp.StatusCode == http.StatusTooManyRequests:
		return fmt.Errorf("figma: rate limited (retry-after %s)", resp.Header.Get("Retry-After"))
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return fmt.Errorf("figma: %s returned %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("figma: decode %s: %w", path, err)
	}
	return nil
}

// --- tree walk ---------------------------------------------------------

const maxWalkDepth = 24

type walker struct {
	components map[string]bool

	colorSet map[string]float64 // hex -> alpha
	typeSet  map[string]TypographyToken
	spacing  map[float64]bool
	radii    map[float64]bool

	componentList []ComponentInfo
	frameList     []FrameInfo
}

func (w *walker) walk(n *apiNode, depth int) {
	if n == nil || depth > maxWalkDepth {
		return
	}
	if w.colorSet == nil {
		w.colorSet = map[string]float64{}
		w.typeSet = map[string]TypographyToken{}
		w.spacing = map[float64]bool{}
		w.radii = map[float64]bool{}
	}

	// Solid fills → color tokens.
	for _, p := range n.Fills {
		if p.Visible != nil && !*p.Visible {
			continue
		}
		if p.Type == "SOLID" && p.Color != nil {
			hex := hexOf(*p.Color)
			alpha := p.Color.A
			if p.Opacity != nil {
				alpha *= *p.Opacity
			}
			if _, ok := w.colorSet[hex]; !ok {
				w.colorSet[hex] = alpha
			}
		}
	}

	// Auto-layout spacing + padding → spacing scale.
	if n.LayoutMode == "HORIZONTAL" || n.LayoutMode == "VERTICAL" {
		w.addSpacing(n.ItemSpacing, n.PaddingLeft, n.PaddingRight, n.PaddingTop, n.PaddingBottom)
	}
	if n.CornerRadius > 0 {
		w.radii[n.CornerRadius] = true
	}

	// Text style → typography token.
	if n.Type == "TEXT" && n.Style != nil && n.Style.FontFamily != "" {
		t := TypographyToken{
			FontFamily: n.Style.FontFamily,
			FontSize:   round1(n.Style.FontSize),
			FontWeight: n.Style.FontWeight,
			LineHeight: round1(n.Style.LineHeightPx),
		}
		w.typeSet[fmt.Sprintf("%s|%g|%g", t.FontFamily, t.FontSize, t.FontWeight)] = t
	}

	// Component inventory: published components + component sets.
	if w.components[n.ID] || n.Type == "COMPONENT" || n.Type == "COMPONENT_SET" {
		ci := ComponentInfo{ID: n.ID, Name: n.Name, Type: n.Type, LayoutMode: n.LayoutMode, Children: len(n.Children)}
		if n.AbsoluteBoundingBox != nil {
			ci.Width = round1(n.AbsoluteBoundingBox.Width)
			ci.Height = round1(n.AbsoluteBoundingBox.Height)
		}
		w.componentList = append(w.componentList, ci)
	}

	// Top-level frames (direct children of a CANVAS page) are the screens.
	if n.Type == "CANVAS" {
		for i := range n.Children {
			ch := &n.Children[i]
			if ch.Type == "FRAME" {
				fi := FrameInfo{ID: ch.ID, Name: ch.Name}
				if ch.AbsoluteBoundingBox != nil {
					fi.Width = round1(ch.AbsoluteBoundingBox.Width)
					fi.Height = round1(ch.AbsoluteBoundingBox.Height)
				}
				w.frameList = append(w.frameList, fi)
			}
		}
	}

	for i := range n.Children {
		w.walk(&n.Children[i], depth+1)
	}
}

func (w *walker) addSpacing(vals ...float64) {
	for _, v := range vals {
		if v > 0 {
			w.spacing[round1(v)] = true
		}
	}
}

func (w *walker) colors() []ColorToken {
	out := make([]ColorToken, 0, len(w.colorSet))
	for hex, a := range w.colorSet {
		out = append(out, ColorToken{Hex: hex, Alpha: round2(a)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Hex < out[j].Hex })
	return out
}

func (w *walker) typography() []TypographyToken {
	out := make([]TypographyToken, 0, len(w.typeSet))
	for _, t := range w.typeSet {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FontSize > out[j].FontSize })
	return out
}

func (w *walker) sortedSet(m map[float64]bool) []float64 {
	out := make([]float64, 0, len(m))
	for v := range m {
		out = append(out, v)
	}
	sort.Float64s(out)
	return out
}

// hexOf converts a Figma 0..1 RGB color to #rrggbb.
func hexOf(c apiColor) string {
	to := func(f float64) int {
		v := int(f*255 + 0.5)
		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}
		return v
	}
	return fmt.Sprintf("#%02x%02x%02x", to(c.R), to(c.G), to(c.B))
}

func round1(f float64) float64 { return float64(int(f*10+0.5)) / 10 }
func round2(f float64) float64 { return float64(int(f*100+0.5)) / 100 }
