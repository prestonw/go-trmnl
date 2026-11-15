package render_test

import (
	"image"
	"image/color"
	"image/draw"
	"testing"

	"github.com/gesellix/go-trmnl/internal/render"
)

func blank() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, render.Width, render.Height))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	return img
}

func darkPixels(img *image.RGBA, r image.Rectangle) int {
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			if (cr+cg+cb)/3 < 0x7fff {
				n++
			}
		}
	}
	return n
}

// The indicator must stay inside the space plugins are asked to keep free,
// otherwise it covers their footer text.
func TestBatteryIndicatorStaysInReservedArea(t *testing.T) {
	img := blank()
	render.DrawBatteryIndicator(img, render.BatteryFull, false, 85)

	reserved := image.Rect(render.Width-render.BatteryFooterWidth, render.Height-40, render.Width, render.Height)
	if got := darkPixels(img, reserved); got == 0 {
		t.Fatal("nothing drawn in the reserved area")
	}
	outside := darkPixels(img, img.Bounds()) - darkPixels(img, reserved)
	if outside != 0 {
		t.Errorf("%d dark pixels outside the reserved area", outside)
	}
}

// The percent figure must draw more ink than the icon alone, and only inside
// the reserved strip to the left of the icon.
func TestBatteryPercentDrawsFigure(t *testing.T) {
	plain, withPercent := blank(), blank()
	render.DrawBatteryIndicator(plain, render.BatteryFull, false, 0)
	render.DrawBatteryIndicator(withPercent, render.BatteryFull, false, 85)
	if darkPixels(plain, plain.Bounds()) >= darkPixels(withPercent, withPercent.Bounds()) {
		t.Fatal("the 85% figure did not add ink")
	}
	strip := image.Rect(render.Width-render.BatteryFooterWidth, render.Height-40, render.Width-render.BatteryFooterWidth+40, render.Height)
	if darkPixels(withPercent, strip) == 0 {
		t.Error("no ink in the figure's strip left of the icon")
	}
}

// More charge must mean more ink: the bars are the whole point.
func TestBatteryLevelsDifferVisibly(t *testing.T) {
	prev := -1
	for _, level := range []render.BatteryLevel{
		render.BatteryUnknown, render.BatteryEmpty, render.BatteryLow,
		render.BatteryMedium, render.BatteryFull,
	} {
		img := blank()
		render.DrawBatteryIndicator(img, level, false, 0)
		n := darkPixels(img, img.Bounds())
		if n <= prev {
			t.Errorf("level %d drew %d dark pixels, not more than the previous %d", level, n, prev)
		}
		prev = n
	}
}

func TestBatteryChargingAddsBolt(t *testing.T) {
	plain, charging := blank(), blank()
	render.DrawBatteryIndicator(plain, render.BatteryLow, false, 0)
	render.DrawBatteryIndicator(charging, render.BatteryLow, true, 0)
	if darkPixels(plain, plain.Bounds()) == darkPixels(charging, charging.Bounds()) {
		t.Error("charging state did not change the drawing")
	}
}

func TestBatteryLevelFor(t *testing.T) {
	for _, tc := range []struct {
		volts float64
		want  render.BatteryLevel
	}{
		{0, render.BatteryUnknown},
		{4.15, render.BatteryFull},
		{3.9, render.BatteryFull},
		{3.75, render.BatteryMedium},
		{3.55, render.BatteryLow},
		{3.42, render.BatteryEmpty},
		{3.1, render.BatteryEmpty},
	} {
		if got := render.BatteryLevelFor(tc.volts); got != tc.want {
			t.Errorf("BatteryLevelFor(%.2f) = %d, want %d", tc.volts, got, tc.want)
		}
	}
}

// The image is cached by content hash, so voltages inside the same 5 % step
// must not produce a new image, and crossing a step must.
func TestBatteryDrawingIsStableWithinALevel(t *testing.T) {
	a, b := blank(), blank()
	// Both 3.90 V and 3.91 V are 75 % (same step, same level).
	render.DrawBatteryIndicator(a, render.BatteryLevelFor(3.90), false, render.BatteryPercent(3.90))
	render.DrawBatteryIndicator(b, render.BatteryLevelFor(3.91), false, render.BatteryPercent(3.91))
	for i := range a.Pix {
		if a.Pix[i] != b.Pix[i] {
			t.Fatal("3.90 V and 3.91 V drew different images although both are 75 %")
		}
	}
	// 3.99 V is 85 %: the figure must change the picture.
	c := blank()
	render.DrawBatteryIndicator(c, render.BatteryLevelFor(3.99), false, render.BatteryPercent(3.99))
	if darkPixels(a, a.Bounds()) == darkPixels(c, c.Bounds()) {
		t.Fatal("3.90 V (75 %) and 3.99 V (85 %) drew identical images")
	}
}

func TestBatteryPercent(t *testing.T) {
	for _, tc := range []struct {
		volts float64
		want  int
	}{
		{0, 0},
		{2.5, 0},   // clamped at the floor
		{3.0, 0},   // floor
		{3.07, 5},  // first step
		{3.6, 50},  // middle
		{3.9, 75},  // quarter-point, exactly on a step
		{4.2, 100}, // full
		{4.5, 100}, // clamped at the ceiling
	} {
		if got := render.BatteryPercent(tc.volts); got != tc.want {
			t.Errorf("BatteryPercent(%.2f) = %d, want %d", tc.volts, got, tc.want)
		}
	}
}
