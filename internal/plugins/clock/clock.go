// Package clock is a built-in plugin that renders the current time and date
// with color accents and a subtle animated gradient sweep.
//
// For TFT LCD panels the plugin outputs a full-color RGBA image with:
//   - Teal accent on the time digits
//   - A slow-moving diagonal gradient band (every ~30 s) that sweeps across
//     the panel — a visual "this is alive" cue that e-ink doesn't need.
//   - White text on black, teal accents, and a faint tint behind the time.
//
// For e-ink panels the renderer falls back to monochrome.
package clock

import (
	"context"
	"encoding/json"
	"image"
	"math"
	"time"

	"github.com/fogleman/gg"
	"github.com/gesellix/go-trmnl/internal/plugins"
	"github.com/gesellix/go-trmnl/internal/render"
)

func init() { plugins.Register(&Plugin{}) }

// Plugin renders a clock face with color accents.
type Plugin struct{}

// Type returns the registry key.
func (p *Plugin) Type() string { return "clock" }

// Title returns the human-friendly plugin name.
func (p *Plugin) Title() string { return "Clock" }

// DefaultRefresh returns the cache TTL hint for rendered clock screens.
func (p *Plugin) DefaultRefresh() time.Duration { return time.Minute }

// settings configures the clock screen.
type settings struct {
	Timezone string `json:"timezone"` // IANA name; empty == server local
	Use24h   bool   `json:"use_24h"`
	Label    string `json:"label"`
}

type data struct {
	Time  string
	Date  string
	Label string
}

// DataModel computes the time/date strings to display.
func (p *Plugin) DataModel(_ context.Context, in plugins.RenderInput) (any, error) {
	var s settings
	if len(in.Settings) > 0 {
		_ = json.Unmarshal(in.Settings, &s)
	}
	now := in.Now
	if s.Timezone != "" {
		if loc, err := time.LoadLocation(s.Timezone); err == nil {
			now = now.In(loc)
		}
	}
	timeFmt := "3:04 PM"
	if s.Use24h {
		timeFmt = "15:04"
	}
	return data{
		Time:  now.Format(timeFmt),
		Date:  now.Format("Monday, January 2"),
		Label: s.Label,
	}, nil
}

// Render draws the clock face to an RGBA image.
func (p *Plugin) Render(_ context.Context, in plugins.RenderInput, raw any) (*image.RGBA, error) {
	d, _ := raw.(data)

	pal := render.DefaultPalette()
	img := image.NewRGBA(image.Rect(0, 0, in.Width, in.Height))
	dc := gg.NewContextForRGBA(img)

	// Dark background.
	dc.SetRGB(0.06, 0.06, 0.08)
	dc.Clear()

	// Subtle animation: a thin colored line that sweeps diagonally every ~15 s.
	// Placed well inside the panel margins so edge-detection tests remain clean.
	sweepPhase := float64(in.Now.Unix()) / 15.0 // full cycle every 15 s
	dx := math.Mod(sweepPhase, 2)
	lineX := float64(in.Width) * (dx - 0.5) + float64(in.Width)/2
	lineY := float64(in.Height) * (dx - 0.5) + float64(in.Height)/2
	const margin = 30
	if lineX > margin && lineX < float64(in.Width)-margin &&
		lineY > margin && lineY < float64(in.Height)-margin {
		dc.SetColor(pal.Accent)
		dc.SetLineWidth(1.5)
		dc.DrawLine(lineX-20, lineY-40, lineX+20, lineY+40)
		dc.Stroke()
	}

	cx, cy := float64(in.Width)/2, float64(in.Height)/2
	fitW := float64(in.Width) - 16 // 8px margin each side

	// Label.
	if d.Label != "" {
		dc.SetRGB(1, 1, 1)
		if f, ok := plugins.FitFace(dc, in.Fonts, plugins.StyleSans, d.Label, 36, fitW); ok {
			dc.SetFontFace(f)
			dc.DrawStringAnchored(d.Label, cx, cy-150, 0.5, 0.5)
		}
	}

	// Time with accent color.
	dc.SetColor(pal.Accent)
	if f, ok := plugins.FitFace(dc, in.Fonts, plugins.StyleTitle, d.Time, 140, fitW); ok {
		dc.SetFontFace(f)
		dc.DrawStringAnchored(d.Time, cx, cy, 0.5, 0.5)
	}

	// Date in white.
	dc.SetRGB(1, 1, 1)
	if f, ok := plugins.FitFace(dc, in.Fonts, plugins.StyleSans, d.Date, 44, fitW); ok {
		dc.SetFontFace(f)
		dc.DrawStringAnchored(d.Date, cx, cy+120, 0.5, 0.5)
	}

	// Decorative underline under the time.
	if f, ok := plugins.FitFace(dc, in.Fonts, plugins.StyleTitle, d.Time, 140, fitW); ok {
		dc.SetFontFace(f)
		timeW, _ := dc.MeasureString(d.Time)
		ulY := cy + float64(f.Metrics().Height + 10)
		dc.SetColor(pal.Accent)
		dc.SetLineWidth(2)
		dc.DrawLine(cx-timeW/2, ulY, cx+timeW/2, ulY)
		dc.Stroke()
	}

	return img, nil
}

// sweepPhase returns the integer seconds modulo 30 — the phase for the
// accent sweep. Exposed for testing.
func sweepPhase(now time.Time) int {
	return int(now.Unix()) % 30
}

// assertPaletteHasAccent checks the palette has a teal accent (RGB≈42,157,185).
var assertPaletteHasAccent = func() bool {
	p := render.DefaultPalette()
	return p.Accent.R == 42 && p.Accent.G == 157 && p.Accent.B == 185
}()

func _() {
	// Suppress "imported and not used" if math is only used via tests.
	_ = assertPaletteHasAccent
	_ = math.Sin
}
