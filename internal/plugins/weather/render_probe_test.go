package weather

import (
	"context"
	"image/png"
	"os"
	"testing"

	"github.com/gesellix/go-trmnl/internal/plugins"
)

// TestProbeInkplateSkipClock renders the new 800x600 SkipClock layout to a
// PNG so the date/place ink can be profiled against the device clock band
// (y58..162). Throwaway probe — delete after the layout is verified.
func TestProbeInkplateSkipClock(t *testing.T) {
	p := New()
	d := Data{Place: "Dublin", TempUnit: "C", WindUnit: "km/h", Temp: 12, FeelsLike: 9,
		Code: 61, CondLabel: "Rain", Humidity: 88, Wind: 22,
		Sunrise: "07:24", Sunset: "16:31",
		Time: "13:37", Date: "Wed, Nov 11", SkipClock: true, Inkplate: true,
		Days: []DayForecast{
			{Heading: "Today", Code: 61, Hi: 14, Lo: 9, UV: 1, PrecipPct: 90},
			{Heading: "Tomorrow", Code: 3, Hi: 13, Lo: 8, UV: 1, PrecipPct: 70},
		}}
	img, err := p.Render(context.Background(), plugins.RenderInput{Width: 800, Height: 600}, d)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	f, err := os.Create("/Users/admin/.hermes/cache/scratch/probe_skipclock.png")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
}
