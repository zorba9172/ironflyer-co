package assets

import (
	"image"
	"image/color"
)

// buildExpo emits the icon + adaptive-icon + splash + favicon set the
// default app.json references.
func buildExpo(logo image.Image, bg color.RGBA) (map[string][]byte, []GeneratedEntry, error) {
	files := map[string][]byte{}
	entries := []GeneratedEntry{}

	// icon.png — 1024x1024 master icon used by iOS and as a fallback
	// on Android when no adaptive icon is configured.
	icon := ResizeSquare(logo, 1024, bg, 0.05)
	iconBytes, err := encodePNG(icon)
	if err != nil {
		return nil, nil, err
	}
	files["assets/icon.png"] = iconBytes
	entries = append(entries, GeneratedEntry{
		Path:      "assets/icon.png",
		Width:     1024,
		Height:    1024,
		SizeBytes: len(iconBytes),
		Purpose:   "icon-1024",
	})

	// adaptive-icon.png — 1024x1024 foreground source for Expo's
	// adaptive icon pipeline. Transparent background, 33% safe-zone.
	transparent := color.RGBA{}
	adaptive := ResizeSquare(logo, 1024, transparent, 0.165)
	aBytes, err := encodePNG(adaptive)
	if err != nil {
		return nil, nil, err
	}
	files["assets/adaptive-icon.png"] = aBytes
	entries = append(entries, GeneratedEntry{
		Path:      "assets/adaptive-icon.png",
		Width:     1024,
		Height:    1024,
		SizeBytes: len(aBytes),
		Purpose:   "adaptive-foreground-1024",
	})

	// splash.png — iPhone 14 Pro Max native resolution (1284x2778).
	// Expo scales internally for every device class.
	splash := ResizeWithPadding(logo, 1284, 2778, bg, 0.3)
	sBytes, err := encodePNG(splash)
	if err != nil {
		return nil, nil, err
	}
	files["assets/splash.png"] = sBytes
	entries = append(entries, GeneratedEntry{
		Path:      "assets/splash.png",
		Width:     1284,
		Height:    2778,
		SizeBytes: len(sBytes),
		Purpose:   "splash",
	})

	// favicon.png — used by `expo export --platform web`. 48x48 is the
	// browser default; Expo handles up-scaling for larger contexts.
	favicon := ResizeSquare(logo, 48, bg, 0.05)
	fBytes, err := encodePNG(favicon)
	if err != nil {
		return nil, nil, err
	}
	files["assets/favicon.png"] = fBytes
	entries = append(entries, GeneratedEntry{
		Path:      "assets/favicon.png",
		Width:     48,
		Height:    48,
		SizeBytes: len(fBytes),
		Purpose:   "favicon",
	})

	return files, entries, nil
}
