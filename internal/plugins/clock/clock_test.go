package clock

import (
	"context"
	"testing"
	"time"

	// Embed tzdata so the timezone test passes regardless of system zoneinfo.
	_ "time/tzdata"

	"github.com/gesellix/go-trmnl/internal/plugins"
)

func TestDataModel24hAndLabel(t *testing.T) {
	p := &Plugin{}
	now := time.Date(2026, time.June, 6, 14, 8, 0, 0, time.UTC)
	in := plugins.RenderInput{Now: now, Settings: []byte(`{"use_24h":true,"label":"Office"}`)}

	raw, err := p.DataModel(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	d := raw.(data)
	if d.Time != "14:08" {
		t.Errorf("24h time = %q, want 14:08", d.Time)
	}
	if d.Label != "Office" {
		t.Errorf("label = %q", d.Label)
	}
	if d.Date != "Saturday, June 6" {
		t.Errorf("date = %q", d.Date)
	}
}

func TestDataModel12h(t *testing.T) {
	p := &Plugin{}
	now := time.Date(2026, time.June, 6, 14, 8, 0, 0, time.UTC)
	raw, _ := p.DataModel(context.Background(), plugins.RenderInput{Now: now, Settings: []byte(`{}`)})
	if raw.(data).Time != "2:08 PM" {
		t.Errorf("12h time = %q, want 2:08 PM", raw.(data).Time)
	}
}

func TestDataModelTimezone(t *testing.T) {
	p := &Plugin{}
	// In June, New York is on EDT (UTC-4), so 14:08 UTC is 10:08.
	now := time.Date(2026, time.June, 6, 14, 8, 0, 0, time.UTC)
	in := plugins.RenderInput{Now: now, Settings: []byte(`{"use_24h":true,"timezone":"America/New_York"}`)}
	raw, err := p.DataModel(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if got := raw.(data).Time; got != "10:08" {
		t.Errorf("New York time = %q, want 10:08", got)
	}
}

func TestRenderDimensions(t *testing.T) {
	p := &Plugin{}
	in := plugins.RenderInput{Width: 800, Height: 480, Now: time.Now()}
	d, _ := p.DataModel(context.Background(), in)
	img, err := p.Render(context.Background(), in, d)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 800 || b.Dy() != 480 {
		t.Errorf("bounds = %v", b)
	}
}

// TestRenderFitsNarrowCanvas is the regression test for the 320x480 DIY
// panel: the time must shrink to fit, with no ink within 8px of either edge.
func TestRenderFitsNarrowCanvas(t *testing.T) {
	p := &Plugin{}
	in := plugins.RenderInput{
		Width: 320, Height: 480, Now: time.Date(2026, 11, 11, 14, 8, 0, 0, time.UTC),
		Settings: []byte(`{"use_24h":true,"label":"Kitchen"}`),
		Fonts:    plugins.NewFontSet("", "classic", "", "", ""),
	}
	d, _ := p.DataModel(context.Background(), in)
	img, err := p.Render(context.Background(), in, d)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	minX, maxX := b.Max.X, b.Min.X
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
		// Detect non-background (non-dark) pixels.  Color rendering makes the
		// old black-check insufficient; the gradient sweep is faint but we only
		// care about the bright text (well above 0.4 RGB after blending).
		if float64(r)/65535 > 0.4 || float64(g)/65535 > 0.4 || float64(b)/65535 > 0.4 {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
			}
		}
	}
	if maxX-minX < 40 {
		t.Fatalf("no time rendered (ink span %dpx)", maxX-minX)
	}
	const margin = 8
	if minX < margin || maxX > b.Max.X-margin {
		t.Errorf("ink spans x=[%d,%d], want inside [%d,%d] (text overflows the panel)", minX, maxX, margin, b.Max.X-margin)
	}
}
