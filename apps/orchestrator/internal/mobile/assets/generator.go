package assets

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"sort"
	"time"

	"github.com/rs/zerolog"
)

// ErrLogoTooSmall is returned when the supplied logo is below the
// 1024x1024 minimum we need to downscale cleanly to every target size.
var ErrLogoTooSmall = errors.New("logo must be at least 1024x1024")

// ErrLogoNotSquare is returned when the supplied logo is not 1:1. We
// reject non-square inputs at the API boundary so the operator gets a
// clean error rather than a stretched icon in the App Store listing.
var ErrLogoNotSquare = errors.New("logo must be square")

// GenerateInput is the single argument the public methods take. PNG
// bytes come straight from a multipart upload at the GraphQL layer.
type GenerateInput struct {
	// LogoPNG is the source logo, MUST be square and at least 1024x1024.
	LogoPNG []byte

	// BackgroundColor is the hex (e.g. "#050507") used for the adaptive
	// icon background tile and the splash screen canvas.
	BackgroundColor string

	// AppName is rendered as the splash-screen label on iOS storyboard.
	AppName string

	// SplashFGColor is the optional foreground (label + icon tint) color
	// for the splash screen. Defaults to white when empty.
	SplashFGColor string
}

// GeneratedEntry is one PNG (or text) artifact emitted by the
// generator. Purpose is a stable identifier ("icon-mdpi", "splash-2x",
// "appicon-1024", "adaptive-foreground", ...) the UI uses to render
// per-asset previews.
type GeneratedEntry struct {
	Path      string
	Width     int
	Height    int
	SizeBytes int
	Purpose   string
}

// GenerateManifest accompanies the file map so callers can render a
// summary without re-reading every PNG.
type GenerateManifest struct {
	Platform    string
	FilesCount  int
	TotalBytes  int
	GeneratedAt time.Time
	Entries     []GeneratedEntry
}

// GenerateResult is the full output of a generator invocation. Files
// is keyed by the relative path inside the project tree; the bytes are
// raw PNGs except where Path indicates otherwise (e.g. .xcassets
// Contents.json, LaunchScreen.storyboard).
type GenerateResult struct {
	Files    map[string][]byte
	Manifest GenerateManifest
}

// Generator produces icon + splash asset bundles for Android, iOS, and
// Expo from a single square logo. The service is stateless; one
// instance can be reused for every project.
//
// Storage trade-off: we deliberately return the generated bundle as
// in-memory bytes rather than committing it through patch.Engine
// directly. The patch engine's FileChange.Content is a string field
// and emitting hundreds of kilobytes of base64 through a textual patch
// is noisy and review-hostile. The cleaner path is two-step: the
// resolver returns the manifest + base64 entries to the caller, and a
// follow-up step (the runtime sandbox's WriteFile, or a manifest-only
// patch) lands the binaries in the project tree. The manifest itself
// is small and safe to ship through the patch engine when persistence
// is wired.
type Generator struct {
	logger zerolog.Logger
	now    func() time.Time
}

// New constructs a Generator. The `now` clock is fixed at startup so a
// given GenerateAll invocation has a single GeneratedAt timestamp
// across all platforms even when each helper is called individually.
func New(logger zerolog.Logger) *Generator {
	return &Generator{logger: logger, now: time.Now}
}

// decodeLogo turns the upload bytes into an image.Image with all the
// validity checks the rest of the pipeline assumes.
func (g *Generator) decodeLogo(in GenerateInput) (image.Image, error) {
	if len(in.LogoPNG) == 0 {
		return nil, errors.New("logo png is empty")
	}
	img, err := png.Decode(bytes.NewReader(in.LogoPNG))
	if err != nil {
		return nil, fmt.Errorf("decode logo png: %w", err)
	}
	b := img.Bounds()
	if b.Dx() < 1024 || b.Dy() < 1024 {
		return nil, ErrLogoTooSmall
	}
	if b.Dx() != b.Dy() {
		return nil, ErrLogoNotSquare
	}
	return img, nil
}

func (g *Generator) parseColors(in GenerateInput) (bg, fg color.RGBA, err error) {
	bg, err = ParseHexColor(in.BackgroundColor)
	if err != nil {
		return color.RGBA{}, color.RGBA{}, fmt.Errorf("background color: %w", err)
	}
	if in.SplashFGColor == "" {
		fg = color.RGBA{R: 255, G: 255, B: 255, A: 255}
	} else {
		fg, err = ParseHexColor(in.SplashFGColor)
		if err != nil {
			return color.RGBA{}, color.RGBA{}, fmt.Errorf("splash foreground color: %w", err)
		}
	}
	return bg, fg, nil
}

// encodePNG writes the image as a deterministic PNG. We pin
// CompressionLevel so the output bytes are stable across Go releases
// for the same pixel input — a requirement of the generator's
// determinism contract.
func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	enc := &png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (g *Generator) finishManifest(platform string, files map[string][]byte, entries []GeneratedEntry, at time.Time) GenerateManifest {
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	total := 0
	for _, e := range entries {
		total += e.SizeBytes
	}
	return GenerateManifest{
		Platform:    platform,
		FilesCount:  len(files),
		TotalBytes:  total,
		GeneratedAt: at,
		Entries:     entries,
	}
}

// GenerateAndroid emits the full Android mipmap set + adaptive
// foreground + splash drawable.
func (g *Generator) GenerateAndroid(ctx context.Context, in GenerateInput) (*GenerateResult, error) {
	_ = ctx
	logo, err := g.decodeLogo(in)
	if err != nil {
		return nil, err
	}
	bg, _, err := g.parseColors(in)
	if err != nil {
		return nil, err
	}
	at := g.now().UTC()
	files, entries, err := buildAndroid(logo, bg)
	if err != nil {
		return nil, err
	}
	g.logger.Info().
		Str("platform", "android").
		Int("files", len(files)).
		Msg("mobile-assets generated")
	return &GenerateResult{
		Files:    files,
		Manifest: g.finishManifest("android", files, entries, at),
	}, nil
}

// GenerateIOS emits AppIcon.appiconset + Contents.json + the
// LaunchScreen.storyboard. iOS no longer uses splash PNGs; the
// storyboard is the modern equivalent.
func (g *Generator) GenerateIOS(ctx context.Context, in GenerateInput) (*GenerateResult, error) {
	_ = ctx
	logo, err := g.decodeLogo(in)
	if err != nil {
		return nil, err
	}
	bg, fg, err := g.parseColors(in)
	if err != nil {
		return nil, err
	}
	at := g.now().UTC()
	files, entries, err := buildIOS(logo, bg, fg, in.AppName, in.BackgroundColor, splashFGHex(in))
	if err != nil {
		return nil, err
	}
	g.logger.Info().
		Str("platform", "ios").
		Int("files", len(files)).
		Msg("mobile-assets generated")
	return &GenerateResult{
		Files:    files,
		Manifest: g.finishManifest("ios", files, entries, at),
	}, nil
}

// GenerateExpo emits the four icon + splash assets the Expo app.json
// references by default.
func (g *Generator) GenerateExpo(ctx context.Context, in GenerateInput) (*GenerateResult, error) {
	_ = ctx
	logo, err := g.decodeLogo(in)
	if err != nil {
		return nil, err
	}
	bg, _, err := g.parseColors(in)
	if err != nil {
		return nil, err
	}
	at := g.now().UTC()
	files, entries, err := buildExpo(logo, bg)
	if err != nil {
		return nil, err
	}
	g.logger.Info().
		Str("platform", "expo").
		Int("files", len(files)).
		Msg("mobile-assets generated")
	return &GenerateResult{
		Files:    files,
		Manifest: g.finishManifest("expo", files, entries, at),
	}, nil
}

// GenerateAll runs every platform back-to-back and merges the file
// maps. A failure in any individual platform aborts the bundle.
func (g *Generator) GenerateAll(ctx context.Context, in GenerateInput) (*GenerateResult, error) {
	at := g.now().UTC()
	g.now = func() time.Time { return at }
	defer func() { g.now = time.Now }()

	all := map[string][]byte{}
	entries := []GeneratedEntry{}

	for _, fn := range []func(context.Context, GenerateInput) (*GenerateResult, error){
		g.GenerateAndroid,
		g.GenerateIOS,
		g.GenerateExpo,
	} {
		r, err := fn(ctx, in)
		if err != nil {
			return nil, err
		}
		for k, v := range r.Files {
			all[k] = v
		}
		entries = append(entries, r.Manifest.Entries...)
	}

	return &GenerateResult{
		Files:    all,
		Manifest: g.finishManifest("all", all, entries, at),
	}, nil
}

func splashFGHex(in GenerateInput) string {
	if in.SplashFGColor == "" {
		return "#FFFFFF"
	}
	return in.SplashFGColor
}
