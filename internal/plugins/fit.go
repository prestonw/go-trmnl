package plugins

import (
	"github.com/fogleman/gg"
	"golang.org/x/image/font"
)

// FitFace returns a face at which text fits within maxW: the base-size face
// when it fits, otherwise the same style scaled down proportionally (advance
// width scales linearly with point size). ok is false only when the base font
// is unavailable; callers then skip the text rather than draw it clipped.
// The context is left on the returned face, so the caller can draw
// immediately.
func FitFace(dc *gg.Context, fs *FontSet, style FontStyle, text string, base, maxW float64) (font.Face, bool) {
	f, err := fs.Face(base, style)
	if err != nil {
		return nil, false
	}
	dc.SetFontFace(f)
	w, _ := dc.MeasureString(text)
	if w <= maxW {
		return f, true
	}
	fit, err := fs.Face(base*maxW/w, style)
	if err != nil {
		return f, true
	}
	dc.SetFontFace(fit)
	return fit, true
}
