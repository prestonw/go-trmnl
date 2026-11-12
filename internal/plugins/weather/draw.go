package weather

import (
	"fmt"
	"image/color"
	"math"

	"github.com/fogleman/gg"
	"github.com/gesellix/go-trmnl/internal/plugins"
)

// category buckets a WMO weather code into a drawable icon class.
type category int

const (
	catClear category = iota
	catPartly
	catCloudy
	catFog
	catRain
	catSnow
	catStorm
)

// codeInfo maps a WMO weather code to a category, a human label, and an
// accent color for the icon.
func codeInfo(code int) (category, string, color.RGBA) {
	switch {
	case code == 0:
		return catClear, "Clear", color.RGBA{255, 200, 50, 255} // gold sun
	case code == 1, code == 2:
		return catPartly, "Partly cloudy", color.RGBA{200, 200, 200, 255} // light gray cloud
	case code == 3:
		return catCloudy, "Overcast", color.RGBA{150, 150, 155, 255} // mid-gray cloud
	case code == 45, code == 48:
		return catFog, "Fog", color.RGBA{180, 190, 200, 255} // fog blue-gray
	case code >= 51 && code <= 57:
		return catRain, "Drizzle", color.RGBA{80, 140, 200, 255} // soft blue rain
	case code >= 61 && code <= 67:
		return catRain, "Rain", color.RGBA{50, 120, 190, 255} // blue rain
	case code >= 71 && code <= 77:
		return catSnow, "Snow", color.RGBA{220, 230, 240, 255} // white snow
	case code >= 80 && code <= 82:
		return catRain, "Showers", color.RGBA{60, 130, 195, 255} // blue showers
	case code >= 85 && code <= 86:
		return catSnow, "Snow showers", color.RGBA{210, 225, 235, 255} // white snow
	case code >= 95:
		return catStorm, "Thunderstorm", color.RGBA{255, 220, 50, 255} // yellow lightning
	default:
		return catCloudy, "Cloudy", color.RGBA{160, 160, 165, 255} // gray cloud
	}
}

// conditionFor returns a short label for the given category.
func conditionFor(cat category) string {
	switch cat {
	case catClear:
		return "Clear"
	case catPartly:
		return "Partly"
	case catCloudy:
		return "Cloudy"
	case catFog:
		return "Foggy"
	case catRain:
		return "Rain"
	case catSnow:
		return "Snow"
	case catStorm:
		return "Storm"
	default:
		return ""
	}
}

func setFace(dc *gg.Context, fs *plugins.FontSet, size float64, style plugins.FontStyle) {
	if f, err := fs.Face(size, style); err == nil {
		dc.SetFontFace(f)
	}
}

// drawCurrent draws the top section: big icon + temperature, condition, and a
// right-hand info column (feels-like, humidity, wind, sun times).
func drawCurrent(dc *gg.Context, fs *plugins.FontSet, d Data, w int) {
	cat, _, col := codeInfo(d.Code)
	fw := float64(w)

	drawIcon(dc, cat, 120, 150, 80, col)

	// monochrome text stays black
	setFace(dc, fs, 120, plugins.StyleTitle)
	dc.DrawStringAnchored(fmt.Sprintf("%.0f°", d.Temp), 215, 140, 0, 0.5)

	setFace(dc, fs, 34, plugins.StyleSans)
	dc.DrawStringAnchored(d.CondLabel, 40, 235, 0, 0.5)

	// Right-hand info column, right-aligned.
	rx := fw - 40
	setFace(dc, fs, 26, plugins.StyleSans)
	dc.DrawStringAnchored(fmt.Sprintf("Feels like %.0f°%s", d.FeelsLike, d.TempUnit), rx, 60, 1, 0.5)
	dc.DrawStringAnchored(fmt.Sprintf("Humidity %d%%", d.Humidity), rx, 105, 1, 0.5)
	dc.DrawStringAnchored(fmt.Sprintf("Wind %.0f %s", d.Wind, d.WindUnit), rx, 150, 1, 0.5)
	if d.Sunrise != "" || d.Sunset != "" {
		dc.DrawStringAnchored(fmt.Sprintf("Sun %s - %s", d.Sunrise, d.Sunset), rx, 195, 1, 0.5)
	}
}

// drawForecast draws the Today/Tomorrow rows beneath a divider.
func drawForecast(dc *gg.Context, fs *plugins.FontSet, d Data, w int) {
	fw := float64(w)
	// monochrome text stays black
	dc.SetLineWidth(2)
	dc.DrawLine(40, 290, fw-40, 290)
	dc.Stroke()

	y := 350.0
	for _, day := range d.Days {
		cat, _, col := codeInfo(day.Code)
		drawIcon(dc, cat, 70, y-6, 30, col)

		setFace(dc, fs, 28, plugins.StyleTitle)
		dc.DrawStringAnchored(day.Heading, 115, y-14, 0, 0.5)
		setFace(dc, fs, 22, plugins.StyleSans)
		dc.DrawStringAnchored(conditionFor(cat), 115, y+16, 0, 0.5)

		setFace(dc, fs, 24, plugins.StyleSans)
		dc.DrawStringAnchored(fmt.Sprintf("UV %d", day.UV), 380, y, 0, 0.5)
		dc.DrawStringAnchored(fmt.Sprintf("%d%% precip", day.PrecipPct), 480, y, 0, 0.5)
		dc.DrawStringAnchored(fmt.Sprintf("L %.0f°  H %.0f°", day.Lo, day.Hi), 640, y, 0, 0.5)

		y += 70
	}
}

// drawPortrait renders the two-column portrait layout (panel taller than
// wide): a time+date column on the left, the current weather on the right,
// and the two-day forecast full-width at the bottom.
//
// Geometry for 320x480 (margin 20): header y 26; columns 20..160 (time) and
// 160..300 (weather) split by a vertical rule at x 160; forecast divider
// y 372 with one line per day at y 398/432; the battery indicator (bottom
// right) is kept clear — nothing draws below y 449.
func drawPortrait(dc *gg.Context, fs *plugins.FontSet, d Data, w, _ int) {
	fw := float64(w)
	m := 20.0
	// monochrome text stays black

	// Header: label left, place right.
	if text := d.Label; text != "" {
		if f, ok := plugins.FitFace(dc, fs, plugins.StyleSans, text, 22, fw/2-2*m); ok {
			dc.SetFontFace(f)
			dc.DrawStringAnchored(text, m, 26, 0, 0.5)
		}
	}
	if d.Place != "" {
		if f, ok := plugins.FitFace(dc, fs, plugins.StyleSans, d.Place, 22, fw/2-2*m); ok {
			dc.SetFontFace(f)
			dc.DrawStringAnchored(d.Place, fw-m, 26, 1, 0.5)
		}
	}

	// Column geometry. The rule sits at x 160 (midpoint of the 20..300
	// content span); each column's text keeps clearance on both sides.
	cxL, wL := 88.0, 128.0  // left column: text region x 24..152
	cxR, wR := 232.0, 128.0 // right column: text region x 168..296

	// Vertical rule between the columns, spanning the hero region.
	dc.SetLineWidth(2)
	dc.DrawLine(160, 64, 160, 352)
	dc.Stroke()

	// Left column: time, date, sun times, humidity, wind.
	// All text is black; minimum size is 22px (title size) for readability.
	if d.Time != "" {
		if f, ok := plugins.FitFace(dc, fs, plugins.StyleTitle, d.Time, 42, wL); ok {
			dc.SetFontFace(f)
			dc.DrawStringAnchored(d.Time, cxL, 112, 0.5, 0.5)
		}
	}
	if d.Date != "" {
		if f, ok := plugins.FitFace(dc, fs, plugins.StyleSans, d.Date, 22, wL); ok {
			dc.SetFontFace(f)
			dc.DrawStringAnchored(d.Date, cxL, 156, 0.5, 0.5)
		}
	}
	if d.Sunrise != "" || d.Sunset != "" {
		sun := fmt.Sprintf("%s - %s", d.Sunrise, d.Sunset)
		if f, ok := plugins.FitFace(dc, fs, plugins.StyleSans, sun, 22, wL); ok {
			dc.SetFontFace(f)
			dc.DrawStringAnchored(sun, cxL, 196, 0.5, 0.5)
		}
	}
	if f, ok := plugins.FitFace(dc, fs, plugins.StyleSans, fmt.Sprintf("Humidity %d%%", d.Humidity), 22, wL); ok {
		dc.SetFontFace(f)
		dc.DrawStringAnchored(fmt.Sprintf("Humidity %d%%", d.Humidity), cxL, 252, 0.5, 0.5)
	}
	if f, ok := plugins.FitFace(dc, fs, plugins.StyleSans, fmt.Sprintf("Wind %.0f %s", d.Wind, d.WindUnit), 22, wL); ok {
		dc.SetFontFace(f)
		dc.DrawStringAnchored(fmt.Sprintf("Wind %.0f %s", d.Wind, d.WindUnit), cxL, 282, 0.5, 0.5)
	}

	// Right column: icon, temperature, condition, feels-like.
	// Icons are colored; all text is pure black for readability.
	cat, _, col := codeInfo(d.Code)
	drawIcon(dc, cat, cxR, 104, 46, col)
	dc.SetRGB(0, 0, 0) // reset to black after colored icon

	if f, ok := plugins.FitFace(dc, fs, plugins.StyleTitle, fmt.Sprintf("%.0f°", d.Temp), 44, wR); ok {
		dc.SetFontFace(f)
		dc.DrawStringAnchored(fmt.Sprintf("%.0f°", d.Temp), cxR, 172, 0.5, 0.5)
	}
	if f, ok := plugins.FitFace(dc, fs, plugins.StyleSans, conditionFor(cat), 22, wR); ok {
		dc.SetFontFace(f)
		dc.DrawStringAnchored(conditionFor(cat), cxR, 218, 0.5, 0.5)
	}
	if f, ok := plugins.FitFace(dc, fs, plugins.StyleSans, fmt.Sprintf("Feels like %.0f°%s", d.FeelsLike, d.TempUnit), 22, wR); ok {
		dc.SetFontFace(f)
		dc.DrawStringAnchored(fmt.Sprintf("Feels like %.0f°%s", d.FeelsLike, d.TempUnit), cxR, 258, 0.5, 0.5)
	}

	// Forecast: one line per day — heading+condition left, L/H+precip right.
	// UV is dropped on portrait: two lines per row would crowd the battery.
	dc.SetLineWidth(2)
	dc.DrawLine(m, 372, fw-m, 372)
	dc.Stroke()
	y := 398.0
	for _, day := range d.Days {
		c, _, col := codeInfo(day.Code)
		drawIcon(dc, c, 36, y, 18, col)
		if f, ok := plugins.FitFace(dc, fs, plugins.StyleSans, day.Heading+" · "+conditionFor(c), 22, fw/2-46); ok {
			dc.SetFontFace(f)
			dc.DrawStringAnchored(day.Heading+" · "+conditionFor(c), 62, y, 0, 0.5)
		}
		if f, ok := plugins.FitFace(dc, fs, plugins.StyleSans, fmt.Sprintf("L %.0f° H %.0f° · %d%%", day.Lo, day.Hi, day.PrecipPct), 22, fw/2-46); ok {
			dc.SetFontFace(f)
			dc.DrawStringAnchored(fmt.Sprintf("L %.0f° H %.0f° · %d%%", day.Lo, day.Hi, day.PrecipPct), fw-m, y, 1, 0.5)
		}
		y += 34
	}
}

// drawLabel draws the footer: a left label and the place (with no time, which
// the device does not provide here) on the right.
// All text is black, minimum 22px for readability.
func drawLabel(dc *gg.Context, fs *plugins.FontSet, d Data, w, h int) {
	text := d.Label
	if text == "" {
		text = "Weather"
	}
	setFace(dc, fs, 22, plugins.StyleSans)
	dc.DrawStringAnchored(text, 40, float64(h)-20, 0, 0.5)
	if d.Place != "" {
		dc.DrawStringAnchored(d.Place, float64(w-d.FooterReserveRight)-40, float64(h)-20, 1, 0.5)
	}
}

// drawIcon draws a filled weather glyph centered at (cx, cy) sized by s,
// using col for the primary fills.
func drawIcon(dc *gg.Context, cat category, cx, cy, s float64, col color.RGBA) {
	switch cat {
	case catClear:
		drawSun(dc, cx, cy, s, col)
	case catPartly:
		drawSun(dc, cx-s*0.25, cy-s*0.2, s*0.6, col)
		drawCloud(dc, cx+s*0.1, cy+s*0.15, s, col)
	case catCloudy:
		drawCloud(dc, cx, cy, s, col)
	case catFog:
		drawCloud(dc, cx, cy-s*0.1, s*0.9, col)
		drawLines(dc, cx, cy+s*0.45, s, 3, col)
	case catRain:
		drawCloud(dc, cx, cy-s*0.15, s, col)
		drawDrops(dc, cx, cy+s*0.4, s, col)
	case catSnow:
		drawCloud(dc, cx, cy-s*0.15, s, col)
		drawFlakes(dc, cx, cy+s*0.4, s, col)
	case catStorm:
		drawCloud(dc, cx, cy-s*0.15, s, col)
		drawBolt(dc, cx, cy+s*0.35, s, col)
	}
}

func drawSun(dc *gg.Context, cx, cy, s float64, col color.RGBA) {
	r := s * 0.32
	dc.SetColor(col)
	dc.DrawCircle(cx, cy, r)
	dc.Fill()
	dc.SetLineWidth(s * 0.06)
	for i := 0; i < 8; i++ {
		a := float64(i) * math.Pi / 4
		x1, y1 := cx+math.Cos(a)*r*1.4, cy+math.Sin(a)*r*1.4
		x2, y2 := cx+math.Cos(a)*r*1.9, cy+math.Sin(a)*r*1.9
		dc.DrawLine(x1, y1, x2, y2)
	}
	dc.Stroke()
}

func drawCloud(dc *gg.Context, cx, cy, s float64, col color.RGBA) {
	r := s * 0.28
	dc.SetColor(col)
	dc.DrawCircle(cx-r*1.1, cy, r)
	dc.DrawCircle(cx+r*1.1, cy, r)
	dc.DrawCircle(cx, cy-r*0.7, r*1.2)
	dc.DrawRectangle(cx-r*1.5, cy, r*3, r*1.1)
	dc.Fill()
}

func drawDrops(dc *gg.Context, cx, cy, s float64, col color.RGBA) {
	dc.SetColor(col)
	dc.SetLineWidth(s * 0.05)
	for i := -1; i <= 1; i++ {
		x := cx + float64(i)*s*0.25
		dc.DrawLine(x, cy, x-s*0.08, cy+s*0.2)
	}
	dc.Stroke()
}

func drawFlakes(dc *gg.Context, cx, cy, s float64, col color.RGBA) {
	dc.SetColor(col)
	for i := -1; i <= 1; i++ {
		dc.DrawCircle(cx+float64(i)*s*0.25, cy+s*0.05, s*0.05)
	}
	dc.Fill()
}

func drawLines(dc *gg.Context, cx, cy, s float64, n int, col color.RGBA) {
	dc.SetColor(col)
	dc.SetLineWidth(s * 0.06)
	for i := 0; i < n; i++ {
		y := cy + float64(i)*s*0.18
		dc.DrawLine(cx-s*0.5, y, cx+s*0.5, y)
	}
	dc.Stroke()
}

func drawBolt(dc *gg.Context, cx, cy, s float64, col color.RGBA) {
	dc.SetColor(col)
	dc.MoveTo(cx+s*0.1, cy-s*0.15)
	dc.LineTo(cx-s*0.15, cy+s*0.1)
	dc.LineTo(cx, cy+s*0.1)
	dc.LineTo(cx-s*0.1, cy+s*0.35)
	dc.LineTo(cx+s*0.2, cy-s*0.02)
	dc.LineTo(cx+s*0.03, cy-s*0.02)
	dc.ClosePath()
	dc.Fill()
}

// colorToRGB returns the RGB components of a color.RGBA for use in drawColor.
func colorToRGB(c color.RGBA) (r, g, b float64) {
	return float64(c.R) / 255, float64(c.G) / 255, float64(c.B) / 255
}
