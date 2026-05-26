// Package finisher — pure-Go visual diff. Compares two PNG screenshots
// pixel-by-pixel with anti-aliasing tolerance and returns a structured
// diff report. No new dependency: stdlib's image/png + image packages
// are enough for the resolutions we care about (a 1280×800 frame is
// 1M pixels; the diff finishes in <100ms on commodity hardware).
//
// The diff is the input to UXGate's visual check. When the live preview
// drifts from the target beyond Tolerance, the gate fails and the
// repair Coder receives the diff stats + a base64 diff overlay so it
// can target the wrong elements.

package finisher

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
)

// VisualDiffResult is the outcome of comparing two screenshots.
type VisualDiffResult struct {
	// MatchedPixels / TotalPixels — the headline metric. DiffRatio =
	// 1 - MatchedPixels/TotalPixels. UXGate compares this to Tolerance.
	MatchedPixels int
	TotalPixels   int
	DiffRatio     float64

	// MeanColorDelta is the average per-pixel L2 distance in RGB space
	// over the pixels that differ. Useful signal for "off by a hair"
	// vs. "completely different layout".
	MeanColorDelta float64

	// PerceptualHashDistance is the Hamming distance between aHash
	// (average-hash) signatures of the two images. 0 = identical
	// thumbnail, 64 = unrelated images. Distance >= 12 typically means
	// the high-level layout itself drifted, not just pixel jitter.
	PerceptualHashDistance int

	// DiffPNGBase64 is a tinted overlay highlighting differing pixels
	// in red. Empty when the images had mismatched dimensions and we
	// couldn't compute a per-pixel diff.
	DiffPNGBase64 string

	// SizeMismatch is true when the two images have different
	// dimensions. In that case DiffRatio is set to 1.0 (total
	// mismatch) and the pixel diff is skipped.
	SizeMismatch bool
}

// CompareScreenshots decodes two base64-encoded PNGs and returns the
// diff. The Resize-on-mismatch policy is intentional: we don't resize
// silently — different dimensions almost always mean the live preview
// is rendering at a different viewport than the target was captured
// at, which itself is a finding the gate should surface.
func CompareScreenshots(targetB64, currentB64 string) (VisualDiffResult, error) {
	if targetB64 == "" || currentB64 == "" {
		return VisualDiffResult{}, errors.New("both target and current screenshots are required")
	}
	target, err := decodePNGBase64(targetB64)
	if err != nil {
		return VisualDiffResult{}, errors.New("decode target: " + err.Error())
	}
	current, err := decodePNGBase64(currentB64)
	if err != nil {
		return VisualDiffResult{}, errors.New("decode current: " + err.Error())
	}
	return compareImages(target, current), nil
}

func decodePNGBase64(b64 string) (image.Image, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		// Some clients pad with newlines or use URL-safe alphabet — try
		// the URL variant before giving up. base64.RawStdEncoding handles
		// no-padding.
		raw2, err2 := base64.RawStdEncoding.DecodeString(b64)
		if err2 != nil {
			return nil, err
		}
		raw = raw2
	}
	return png.Decode(bytes.NewReader(raw))
}

// compareImages runs the actual diff. We use a perceptual-tolerance
// threshold of ~20 per-channel: anti-aliasing on a 24-bit RGB grid
// reliably drifts by 5-15 per channel between renderers, so anything
// under 20 is "the same pixel".
const colorTolerancePerChannel = 20

func compareImages(target, current image.Image) VisualDiffResult {
	tBounds := target.Bounds()
	cBounds := current.Bounds()
	if tBounds.Dx() != cBounds.Dx() || tBounds.Dy() != cBounds.Dy() {
		return VisualDiffResult{
			SizeMismatch:           true,
			DiffRatio:              1.0,
			PerceptualHashDistance: hammingDistance(aHash(target), aHash(current)),
		}
	}
	w, h := tBounds.Dx(), tBounds.Dy()
	total := w * h
	matched := 0
	deltaSum := 0.0

	// Build the diff overlay in parallel with the count so we only walk
	// the pixel grid once.
	overlay := image.NewRGBA(image.Rect(0, 0, w, h))

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			tr, tg, tb, _ := target.At(tBounds.Min.X+x, tBounds.Min.Y+y).RGBA()
			cr, cg, cb, _ := current.At(cBounds.Min.X+x, cBounds.Min.Y+y).RGBA()
			// RGBA values are 16-bit; shift down to 8-bit for the
			// perceptual delta and the overlay.
			tr8, tg8, tb8 := int(tr>>8), int(tg>>8), int(tb>>8)
			cr8, cg8, cb8 := int(cr>>8), int(cg>>8), int(cb>>8)
			dR, dG, dB := abs(tr8-cr8), abs(tg8-cg8), abs(tb8-cb8)
			if dR <= colorTolerancePerChannel &&
				dG <= colorTolerancePerChannel &&
				dB <= colorTolerancePerChannel {
				matched++
				// Render the matched pixel as a faded target sample so
				// the overlay still shows the page structure.
				overlay.Set(x, y, color.RGBA{
					R: uint8((tr8 + cr8) / 2),
					G: uint8((tg8 + cg8) / 2),
					B: uint8((tb8 + cb8) / 2),
					A: 70,
				})
			} else {
				deltaSum += math.Sqrt(float64(dR*dR + dG*dG + dB*dB))
				overlay.Set(x, y, color.RGBA{R: 255, G: 80, B: 80, A: 220})
			}
		}
	}
	diffCount := total - matched
	res := VisualDiffResult{
		MatchedPixels:          matched,
		TotalPixels:            total,
		DiffRatio:              float64(diffCount) / float64(total),
		PerceptualHashDistance: hammingDistance(aHash(target), aHash(current)),
	}
	if diffCount > 0 {
		res.MeanColorDelta = deltaSum / float64(diffCount)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, overlay); err == nil {
		res.DiffPNGBase64 = base64.StdEncoding.EncodeToString(buf.Bytes())
	}
	return res
}

// aHash returns the 64-bit average-hash perceptual signature for an
// image. Resize to 8×8 grayscale, average the luminance, emit 1 for
// pixels above average and 0 below. Cheap, robust to small layout
// drift, sensitive to gross structural changes.
func aHash(img image.Image) uint64 {
	const dim = 8
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w == 0 || h == 0 {
		return 0
	}
	// Sample dim×dim grid points with nearest-neighbour selection. Good
	// enough for an aHash and avoids pulling in a real resampler.
	pixels := [dim * dim]int{}
	sum := 0
	for j := 0; j < dim; j++ {
		for i := 0; i < dim; i++ {
			x := bounds.Min.X + (i*w)/dim
			y := bounds.Min.Y + (j*h)/dim
			r, g, b, _ := img.At(x, y).RGBA()
			// ITU-R BT.601 luma — keeps perceptual brightness consistent.
			lum := (int(r>>8)*299 + int(g>>8)*587 + int(b>>8)*114) / 1000
			pixels[j*dim+i] = lum
			sum += lum
		}
	}
	avg := sum / (dim * dim)
	var hash uint64
	for i, v := range pixels {
		if v >= avg {
			hash |= 1 << uint(i)
		}
	}
	return hash
}

func hammingDistance(a, b uint64) int {
	x := a ^ b
	d := 0
	for x != 0 {
		d += int(x & 1)
		x >>= 1
	}
	return d
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
