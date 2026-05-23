// Parse: turn a raw Figma file into the structured tokens + component
// inventory the Coder agent consumes.
//
// The Figma document is a deeply nested tree; agents do not reason
// well across thousands of nested nodes. Extract() condenses the tree
// into two small documents:
//
//   - DesignTokens — the colours, typography, spacing, and radii a
//     designer settled on. Mirrors what `.ironflyer/design_tokens.json`
//     already looks like elsewhere in the codebase so the UXer's
//     design-tokens gate finds it without any further translation.
//   - ComponentInventory — every top-level frame/component with its
//     bounding box and a histogram of child types. The Coder uses it
//     to scaffold one React/Vue component per inventory entry without
//     needing to walk the full Figma tree itself.
//
// Implementation is intentionally simple: no heuristics about
// component grouping, no font-weight bucketing, no perceptual color
// dedupe. The agent does the design judgement; we just give it
// concrete numbers to ground on.

package figma

import (
	"fmt"
	"sort"
	"strings"
)

// DesignTokens is the structured token set the UXer can consume. The
// JSON shape matches the conventional `.ironflyer/design_tokens.json`
// the design-tokens gate already looks for.
type DesignTokens struct {
	Colors     map[string]string `json:"colors"`     // name → hex (e.g. "lime-500" → "#c7ff00")
	Typography []TypographyToken `json:"typography"`
	Spacing    []float64         `json:"spacing"` // unique itemSpacing values, sorted
	Radii      []float64         `json:"radii"`   // unique cornerRadius values, sorted
}

// TypographyToken is one entry in the typography token list. Tokens
// are deduplicated by the (family, size, weight, lineHeight) tuple.
type TypographyToken struct {
	Name         string  `json:"name"`
	FontFamily   string  `json:"fontFamily"`
	FontSize     float64 `json:"fontSize"`
	FontWeight   float64 `json:"fontWeight"`
	LineHeightPx float64 `json:"lineHeightPx,omitempty"`
}

// ComponentInventory lists every top-level FRAME / COMPONENT in the
// file with its bounding box + a brief structural description (count
// of children by type) so the Coder can reason about layout without
// walking the full node tree.
type ComponentInventory struct {
	Components []ComponentRef `json:"components"`
}

// ComponentRef is one entry in the inventory.
type ComponentRef struct {
	NodeID      string         `json:"nodeId"`
	Name        string         `json:"name"`
	Type        string         `json:"type"`
	Width       float64        `json:"width"`
	Height      float64        `json:"height"`
	ChildCounts map[string]int `json:"childCounts"` // e.g. {"TEXT":3,"RECTANGLE":2}
}

// Extract walks the document and returns both. Tokens are derived
// from named styles when available; otherwise we synthesise names
// from observed values (e.g. "color-1" through "color-N").
func Extract(f *File) (DesignTokens, ComponentInventory) {
	tokens := DesignTokens{
		Colors:     map[string]string{},
		Typography: []TypographyToken{},
		Spacing:    []float64{},
		Radii:      []float64{},
	}
	inv := ComponentInventory{Components: []ComponentRef{}}
	if f == nil {
		return tokens, inv
	}

	// Track seen sets so we dedupe without doing an O(n^2) scan.
	colorByHex := map[string]string{} // hex → name (so we can prefer named styles over synth)
	typeSeen := map[string]struct{}{} // tuple key → marker
	spacingSeen := map[float64]struct{}{}
	radiiSeen := map[float64]struct{}{}

	// nodeStyleMap maps styleId → friendly name when File.Styles
	// supplies one. We look this up per-node to give colour tokens
	// designer-meaningful names ("brand/lime-500") instead of synth
	// placeholders.
	nodeStyleMap := map[string]string{}
	for id, meta := range f.Styles {
		if meta.Name != "" {
			nodeStyleMap[id] = meta.Name
		}
	}

	// Top-level inventory: the immediate children of the document
	// (Figma's "pages") expose their own children which are the
	// frames + components a designer would call "screens".
	for _, page := range f.Document.Children {
		for _, n := range page.Children {
			switch n.Type {
			case "FRAME", "COMPONENT", "COMPONENT_SET":
				inv.Components = append(inv.Components, summarise(n))
			}
		}
	}

	// Recursively walk every node to harvest tokens.
	var walk func(n Node)
	walk = func(n Node) {
		// Colours: first SOLID visible fill wins.
		if hex := firstSolidHex(n.Fills); hex != "" {
			name := preferredColorName(n, nodeStyleMap)
			if name == "" {
				if existing, ok := colorByHex[hex]; ok {
					name = existing
				} else {
					name = fmt.Sprintf("color-%d", len(colorByHex)+1)
				}
			}
			if _, already := tokens.Colors[name]; !already {
				tokens.Colors[name] = hex
			}
			colorByHex[hex] = name
		}

		// Typography: pull off TEXT nodes with a style block.
		if n.Type == "TEXT" && n.Style != nil {
			t := TypographyToken{
				FontFamily:   n.Style.FontFamily,
				FontSize:     n.Style.FontSize,
				FontWeight:   n.Style.FontWeight,
				LineHeightPx: n.Style.LineHeightPx,
			}
			key := typographyKey(t)
			if _, seen := typeSeen[key]; !seen {
				typeSeen[key] = struct{}{}
				if styleID, ok := n.Styles["text"]; ok {
					if name, ok := nodeStyleMap[styleID]; ok {
						t.Name = name
					}
				}
				if t.Name == "" {
					t.Name = fmt.Sprintf("text-%d", len(tokens.Typography)+1)
				}
				tokens.Typography = append(tokens.Typography, t)
			}
		}

		// Spacing: AutoLayout frames carry ItemSpacing.
		if n.LayoutMode != "" && n.LayoutMode != "NONE" && n.ItemSpacing > 0 {
			if _, seen := spacingSeen[n.ItemSpacing]; !seen {
				spacingSeen[n.ItemSpacing] = struct{}{}
				tokens.Spacing = append(tokens.Spacing, n.ItemSpacing)
			}
		}

		// Radii: any node with a non-zero cornerRadius.
		if n.CornerRadius > 0 {
			if _, seen := radiiSeen[n.CornerRadius]; !seen {
				radiiSeen[n.CornerRadius] = struct{}{}
				tokens.Radii = append(tokens.Radii, n.CornerRadius)
			}
		}

		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(f.Document)

	sort.Float64s(tokens.Spacing)
	sort.Float64s(tokens.Radii)

	return tokens, inv
}

// summarise builds a ComponentRef from a top-level node.
func summarise(n Node) ComponentRef {
	out := ComponentRef{
		NodeID:      n.ID,
		Name:        n.Name,
		Type:        n.Type,
		ChildCounts: map[string]int{},
	}
	if n.AbsoluteBoundingBox != nil {
		out.Width = n.AbsoluteBoundingBox.Width
		out.Height = n.AbsoluteBoundingBox.Height
	}
	// Recursive count so a 3-level-deep TEXT still bumps the
	// TEXT bucket. Without this the histogram is dominated by
	// container groupings.
	var count func(c Node)
	count = func(c Node) {
		out.ChildCounts[c.Type]++
		for _, cc := range c.Children {
			count(cc)
		}
	}
	for _, c := range n.Children {
		count(c)
	}
	return out
}

// firstSolidHex returns the hex string of the first visible SOLID fill,
// or "" when no SOLID fill is present.
func firstSolidHex(fills []Paint) string {
	for _, p := range fills {
		if !p.Visible {
			// Figma omits Visible=true by default; only explicit
			// false should suppress. JSON unmarshal leaves the
			// field at its zero value when missing, so we can't
			// distinguish "missing" from "false". In practice
			// upstream emits Visible only when it's false — we
			// follow that and accept Visible=false as hidden.
			continue
		}
		if p.Type != "SOLID" || p.Color == nil {
			continue
		}
		return toHex(*p.Color)
	}
	// Second pass tolerating the missing-Visible case so we still
	// surface tokens when Figma omits the flag entirely.
	for _, p := range fills {
		if p.Type == "SOLID" && p.Color != nil {
			return toHex(*p.Color)
		}
	}
	return ""
}

// preferredColorName returns the friendly name attached to a node's
// fill style, or "" when no named style applies.
func preferredColorName(n Node, names map[string]string) string {
	if id, ok := n.Styles["fill"]; ok {
		if name, ok := names[id]; ok {
			return name
		}
	}
	if id, ok := n.Styles["fills"]; ok {
		if name, ok := names[id]; ok {
			return name
		}
	}
	return ""
}

// toHex converts a normalised RGBA into a CSS-ready hex string.
// Alpha is dropped when 1.0 (the common case); otherwise we emit an
// 8-digit hex with the alpha channel included.
func toHex(c RGBA) string {
	r := clampByte(c.R)
	g := clampByte(c.G)
	b := clampByte(c.B)
	if c.A == 0 || c.A == 1 {
		return fmt.Sprintf("#%02x%02x%02x", r, g, b)
	}
	a := clampByte(c.A)
	return fmt.Sprintf("#%02x%02x%02x%02x", r, g, b, a)
}

// clampByte projects a normalised 0..1 channel into a 0..255 byte.
// Values outside the range are clamped to the nearest valid byte
// rather than wrapping.
func clampByte(v float64) int {
	x := int(v*255 + 0.5)
	if x < 0 {
		return 0
	}
	if x > 255 {
		return 255
	}
	return x
}

// typographyKey is the dedupe identity for a TypographyToken — the
// designer-meaningful tuple, not the synthesised name.
func typographyKey(t TypographyToken) string {
	return strings.ToLower(strings.TrimSpace(t.FontFamily)) +
		"|" + fmtFloat(t.FontSize) +
		"|" + fmtFloat(t.FontWeight) +
		"|" + fmtFloat(t.LineHeightPx)
}

// fmtFloat formats with no trailing zeros so "16" and "16.0" collide.
func fmtFloat(v float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", v), "0"), ".")
}
