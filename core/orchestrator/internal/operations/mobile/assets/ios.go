package assets

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"sort"
)

// iosIcon describes one rendered slot in AppIcon.appiconset.
type iosIcon struct {
	idiom string  // iphone / ipad / ios-marketing
	role  string  // notification / settings / spotlight / app / marketing
	pt    int     // logical points (used in the filename + Contents.json)
	scale int     // 1x / 2x / 3x
}

// iosIconSet is the canonical Apple App Icon size table at 2026.
// Sorted by (idiom, role, scale) for deterministic output.
var iosIconSet = []iosIcon{
	// iPhone
	{idiom: "iphone", role: "notification", pt: 20, scale: 2},
	{idiom: "iphone", role: "notification", pt: 20, scale: 3},
	{idiom: "iphone", role: "settings", pt: 29, scale: 2},
	{idiom: "iphone", role: "settings", pt: 29, scale: 3},
	{idiom: "iphone", role: "spotlight", pt: 40, scale: 2},
	{idiom: "iphone", role: "spotlight", pt: 40, scale: 3},
	{idiom: "iphone", role: "app", pt: 60, scale: 2},
	{idiom: "iphone", role: "app", pt: 60, scale: 3},

	// iPad
	{idiom: "ipad", role: "notification", pt: 20, scale: 1},
	{idiom: "ipad", role: "notification", pt: 20, scale: 2},
	{idiom: "ipad", role: "settings", pt: 29, scale: 1},
	{idiom: "ipad", role: "settings", pt: 29, scale: 2},
	{idiom: "ipad", role: "spotlight", pt: 40, scale: 1},
	{idiom: "ipad", role: "spotlight", pt: 40, scale: 2},
	{idiom: "ipad", role: "app", pt: 76, scale: 1},
	{idiom: "ipad", role: "app", pt: 76, scale: 2},
	{idiom: "ipad", role: "app", pt: 83, scale: 2}, // iPad Pro (renders 167px)

	// App Store
	{idiom: "ios-marketing", role: "marketing", pt: 1024, scale: 1},
}

// pixels returns the rendered side length for this slot.
func (i iosIcon) pixels() int { return i.pt * i.scale }

// filename produces the deterministic basename used inside the
// appiconset folder. Apple itself doesn't require a specific name —
// only that Contents.json points to it — so we adopt the convention
// `icon-{pt}x{pt}@{scale}x.png` which is what most generators (and
// the Xcode export) use.
func (i iosIcon) filename() string {
	return fmt.Sprintf("icon-%dx%d@%dx.png", i.pt, i.pt, i.scale)
}

// contentsImage is the JSON shape Xcode expects per asset.
type contentsImage struct {
	Idiom    string `json:"idiom"`
	Size     string `json:"size"`
	Scale    string `json:"scale"`
	Filename string `json:"filename"`
}

type contentsInfo struct {
	Version int    `json:"version"`
	Author  string `json:"author"`
}

type appIconContents struct {
	Images []contentsImage `json:"images"`
	Info   contentsInfo    `json:"info"`
}

// buildIOS emits every AppIcon.appiconset entry, the Contents.json
// that ties them together, and the LaunchScreen.storyboard text.
func buildIOS(
	logo image.Image,
	bg color.RGBA,
	fg color.RGBA,
	appName string,
	bgHex string,
	fgHex string,
) (map[string][]byte, []GeneratedEntry, error) {
	files := map[string][]byte{}
	entries := make([]GeneratedEntry, 0, len(iosIconSet)+2)

	images := make([]contentsImage, 0, len(iosIconSet))
	// Sort the slot list deterministically before iterating.
	slots := make([]iosIcon, len(iosIconSet))
	copy(slots, iosIconSet)
	sort.SliceStable(slots, func(i, j int) bool {
		if slots[i].idiom != slots[j].idiom {
			return slots[i].idiom < slots[j].idiom
		}
		if slots[i].pt != slots[j].pt {
			return slots[i].pt < slots[j].pt
		}
		return slots[i].scale < slots[j].scale
	})

	for _, slot := range slots {
		px := slot.pixels()
		// The marketing slot (1024) must be fully opaque per App Store
		// rules — we render it directly on the bg. Every other slot
		// uses the same approach so the icon family stays visually
		// uniform.
		img := ResizeSquare(logo, px, bg, 0.05)
		raw, err := encodePNG(img)
		if err != nil {
			return nil, nil, err
		}
		path := "ios/Resources/Assets.xcassets/AppIcon.appiconset/" + slot.filename()
		files[path] = raw
		entries = append(entries, GeneratedEntry{
			Path:      path,
			Width:     px,
			Height:    px,
			SizeBytes: len(raw),
			Purpose:   fmt.Sprintf("appicon-%s-%dpt@%dx", slot.role, slot.pt, slot.scale),
		})
		images = append(images, contentsImage{
			Idiom:    slot.idiom,
			Size:     fmt.Sprintf("%dx%d", slot.pt, slot.pt),
			Scale:    fmt.Sprintf("%dx", slot.scale),
			Filename: slot.filename(),
		})
	}

	contents := appIconContents{
		Images: images,
		Info:   contentsInfo{Version: 1, Author: "ironflyer"},
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(contents); err != nil {
		return nil, nil, err
	}
	contentsPath := "ios/Resources/Assets.xcassets/AppIcon.appiconset/Contents.json"
	files[contentsPath] = buf.Bytes()
	entries = append(entries, GeneratedEntry{
		Path:      contentsPath,
		Width:     0,
		Height:    0,
		SizeBytes: buf.Len(),
		Purpose:   "appicon-contents",
	})

	// LaunchScreen.storyboard text.
	storyboard := RenderLaunchStoryboard(StoryboardParams{
		AppName:         appName,
		BackgroundHex:   bgHex,
		ForegroundHex:   fgHex,
		BackgroundRGBA:  bg,
		ForegroundRGBA:  fg,
	})
	sbPath := "ios/Resources/Base.lproj/LaunchScreen.storyboard"
	files[sbPath] = []byte(storyboard)
	entries = append(entries, GeneratedEntry{
		Path:      sbPath,
		Width:     0,
		Height:    0,
		SizeBytes: len(storyboard),
		Purpose:   "launch-storyboard",
	})

	return files, entries, nil
}
