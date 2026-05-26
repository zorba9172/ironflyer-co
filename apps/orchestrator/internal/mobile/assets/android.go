package assets

import (
	"image"
	"image/color"
)

// androidIcon describes one entry in the standard mipmap density set.
type androidIcon struct {
	density string // mdpi / hdpi / ...
	size    int    // pixels (square)
}

// androidMipmapSet is the canonical density ladder. The order is
// stable to keep the generator's output deterministic.
var androidMipmapSet = []androidIcon{
	{density: "mdpi", size: 48},
	{density: "hdpi", size: 72},
	{density: "xhdpi", size: 96},
	{density: "xxhdpi", size: 144},
	{density: "xxxhdpi", size: 192},
}

// buildAndroid emits the full mipmap-* tree, the matching round icons,
// the adaptive foreground PNG with 33% safe-zone, and a splash
// drawable centred on bg.
func buildAndroid(logo image.Image, bg color.RGBA) (map[string][]byte, []GeneratedEntry, error) {
	files := map[string][]byte{}
	entries := make([]GeneratedEntry, 0, len(androidMipmapSet)*2+2)

	for _, ic := range androidMipmapSet {
		// Standard square launcher icon — logo fills almost the whole
		// canvas (5% safe-zone) on top of the background colour.
		square := ResizeSquare(logo, ic.size, bg, 0.05)
		raw, err := encodePNG(square)
		if err != nil {
			return nil, nil, err
		}
		path := "android/app/src/main/res/mipmap-" + ic.density + "/ic_launcher.png"
		files[path] = raw
		entries = append(entries, GeneratedEntry{
			Path:      path,
			Width:     ic.size,
			Height:    ic.size,
			SizeBytes: len(raw),
			Purpose:   "icon-" + ic.density,
		})

		// Round variant — same source clipped to the inscribed circle.
		round := RoundedMask(square)
		rraw, err := encodePNG(round)
		if err != nil {
			return nil, nil, err
		}
		rpath := "android/app/src/main/res/mipmap-" + ic.density + "/ic_launcher_round.png"
		files[rpath] = rraw
		entries = append(entries, GeneratedEntry{
			Path:      rpath,
			Width:     ic.size,
			Height:    ic.size,
			SizeBytes: len(rraw),
			Purpose:   "icon-round-" + ic.density,
		})
	}

	// Adaptive icon foreground — 432x432 with 33% total safe-zone (the
	// logo at the centre 66% of the canvas). Background is transparent
	// because Android composes it on top of the user/system background
	// layer at install time.
	transparent := color.RGBA{}
	adaptive := ResizeSquare(logo, 432, transparent, 0.165)
	araw, err := encodePNG(adaptive)
	if err != nil {
		return nil, nil, err
	}
	apath := "android/app/src/main/res/mipmap-anydpi-v26/ic_launcher_foreground.png"
	files[apath] = araw
	entries = append(entries, GeneratedEntry{
		Path:      apath,
		Width:     432,
		Height:    432,
		SizeBytes: len(araw),
		Purpose:   "adaptive-foreground",
	})

	// Adaptive XML wrapper. Without this the foreground PNG is unused.
	xml := []byte(adaptiveIconXML)
	axmlPath := "android/app/src/main/res/mipmap-anydpi-v26/ic_launcher.xml"
	files[axmlPath] = xml
	entries = append(entries, GeneratedEntry{
		Path:      axmlPath,
		Width:     0,
		Height:    0,
		SizeBytes: len(xml),
		Purpose:   "adaptive-xml",
	})

	// Splash drawable — 1280x1920 portrait, logo centred at ~40% of
	// the shorter side so it does not crowd the device's chrome.
	splash := ResizeWithPadding(logo, 1280, 1920, bg, 0.3)
	sraw, err := encodePNG(splash)
	if err != nil {
		return nil, nil, err
	}
	spath := "android/app/src/main/res/drawable/splash.png"
	files[spath] = sraw
	entries = append(entries, GeneratedEntry{
		Path:      spath,
		Width:     1280,
		Height:    1920,
		SizeBytes: len(sraw),
		Purpose:   "splash",
	})

	return files, entries, nil
}

// adaptiveIconXML is the boilerplate referenced by Android 8+ launchers
// to compose the foreground + background layers. The background layer
// references a colour resource the project's colors.xml must declare —
// the template starters already do.
const adaptiveIconXML = `<?xml version="1.0" encoding="utf-8"?>
<adaptive-icon xmlns:android="http://schemas.android.com/apk/res/android">
    <background android:drawable="@color/ic_launcher_background" />
    <foreground android:drawable="@mipmap/ic_launcher_foreground" />
</adaptive-icon>
`
