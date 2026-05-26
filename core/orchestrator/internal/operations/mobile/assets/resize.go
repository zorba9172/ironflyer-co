// Package assets generates the full set of platform icon + splash
// assets from a single square source logo. The output is deterministic
// for a given input — same logo bytes + same colors produce
// byte-identical PNGs across invocations, which lets the patch engine
// treat a re-generation as a clean no-op when nothing changed.
package assets

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strings"

	xdraw "golang.org/x/image/draw"
)

// ResizeWithPadding draws src centered inside a (targetW x targetH)
// canvas filled with bg, applying a fractional safe-zone padding so the
// logo never bleeds into the device's circular mask area. padding is
// the fraction of the canvas reserved on each side (e.g. 0.165 ==
// Android's 33% total safe-zone, 16.5% per side).
//
// We use CatmullRom for downscaling — it is the highest-quality
// resampler in golang.org/x/image/draw and is well-suited to icon
// rasterization where ringing is acceptable in exchange for sharper
// edges than ApproxBiLinear.
func ResizeWithPadding(src image.Image, targetW, targetH int, bg color.Color, padding float64) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	// Fill background.
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)

	if padding < 0 {
		padding = 0
	}
	if padding > 0.45 {
		padding = 0.45
	}

	innerW := int(float64(targetW) * (1.0 - 2.0*padding))
	innerH := int(float64(targetH) * (1.0 - 2.0*padding))
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}

	// Preserve the source's aspect ratio inside the inner box.
	srcBounds := src.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()
	if srcW == 0 || srcH == 0 {
		return dst
	}
	scaleW := float64(innerW) / float64(srcW)
	scaleH := float64(innerH) / float64(srcH)
	scale := scaleW
	if scaleH < scale {
		scale = scaleH
	}
	drawW := int(float64(srcW) * scale)
	drawH := int(float64(srcH) * scale)
	if drawW < 1 {
		drawW = 1
	}
	if drawH < 1 {
		drawH = 1
	}
	offX := (targetW - drawW) / 2
	offY := (targetH - drawH) / 2

	rect := image.Rect(offX, offY, offX+drawW, offY+drawH)
	xdraw.CatmullRom.Scale(dst, rect, src, srcBounds, xdraw.Over, nil)
	return dst
}

// ResizeSquare scales src to a square (size x size) with bg fill and
// the supplied safe-zone padding.
func ResizeSquare(src image.Image, size int, bg color.Color, padding float64) image.Image {
	return ResizeWithPadding(src, size, size, bg, padding)
}

// RoundedMask returns src clipped to a circle inscribed in the image's
// shorter side. Pixels outside the circle are made transparent. This is
// the cheap stdlib-only mask used for Android's ic_launcher_round.png.
func RoundedMask(src image.Image) image.Image {
	b := src.Bounds()
	w := b.Dx()
	h := b.Dy()
	out := image.NewRGBA(image.Rect(0, 0, w, h))

	// Centre of the inscribed circle, radius is half the shorter side.
	cx := float64(w) / 2.0
	cy := float64(h) / 2.0
	r := cx
	if cy < r {
		r = cy
	}
	r2 := r * r

	for y := 0; y < h; y++ {
		dy := float64(y) + 0.5 - cy
		for x := 0; x < w; x++ {
			dx := float64(x) + 0.5 - cx
			if dx*dx+dy*dy <= r2 {
				out.Set(x, y, src.At(b.Min.X+x, b.Min.Y+y))
			}
		}
	}
	return out
}

// ParseHexColor accepts "#rgb", "#rrggbb", or "#rrggbbaa". Returns a
// fully-opaque colour when alpha is omitted. Whitespace is stripped.
func ParseHexColor(hex string) (color.RGBA, error) {
	s := strings.TrimSpace(hex)
	s = strings.TrimPrefix(s, "#")
	switch len(s) {
	case 3:
		r, err := hexNibble(s[0])
		if err != nil {
			return color.RGBA{}, err
		}
		g, err := hexNibble(s[1])
		if err != nil {
			return color.RGBA{}, err
		}
		b, err := hexNibble(s[2])
		if err != nil {
			return color.RGBA{}, err
		}
		return color.RGBA{R: r*16 + r, G: g*16 + g, B: b*16 + b, A: 255}, nil
	case 6:
		r, err := hexByte(s[0:2])
		if err != nil {
			return color.RGBA{}, err
		}
		g, err := hexByte(s[2:4])
		if err != nil {
			return color.RGBA{}, err
		}
		b, err := hexByte(s[4:6])
		if err != nil {
			return color.RGBA{}, err
		}
		return color.RGBA{R: r, G: g, B: b, A: 255}, nil
	case 8:
		r, err := hexByte(s[0:2])
		if err != nil {
			return color.RGBA{}, err
		}
		g, err := hexByte(s[2:4])
		if err != nil {
			return color.RGBA{}, err
		}
		b, err := hexByte(s[4:6])
		if err != nil {
			return color.RGBA{}, err
		}
		a, err := hexByte(s[6:8])
		if err != nil {
			return color.RGBA{}, err
		}
		return color.RGBA{R: r, G: g, B: b, A: a}, nil
	}
	return color.RGBA{}, fmt.Errorf("invalid hex color %q", hex)
}

func hexNibble(c byte) (uint8, error) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', nil
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, nil
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, nil
	}
	return 0, fmt.Errorf("invalid hex digit %q", string(c))
}

func hexByte(s string) (uint8, error) {
	if len(s) != 2 {
		return 0, errors.New("hex byte must be 2 chars")
	}
	hi, err := hexNibble(s[0])
	if err != nil {
		return 0, err
	}
	lo, err := hexNibble(s[1])
	if err != nil {
		return 0, err
	}
	return hi*16 + lo, nil
}
