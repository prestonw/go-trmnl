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
		return catClear, "Clear", color.RGBA{255, 200, 50, 255}
	case code == 1, code == 2:
		return catPartly, "Partly cloudy", color.RGBA{200, 200, 200, 255}
	case code == 3:
		return catCloudy, "Overcast", color.RGBA{150, 150, 155, 255}
	case code == 45, code == 48:
		return catFog, "Fog", color.RGBA{180, 190, 200, 255}
	case code >= 51 && code <= 57:
		return catRain, "Drizzle", color.RGBA{80, 140, 200, 255}
	case code >= 61 && code <= 67:
		return catRain, "Rain", color.RGBA{50, 120, 190, 255}
	case code >= 71 && code <= 77:
		return catSnow, "Snow", color.RGBA{220, 230, 240, 255}
	case code >= 80 && code <= 82:
		return catRain, "Showers", color.RGBA{60, 130, 195, 255}
	case code >= 85 && code <= 86:
		return catSnow, "Snow showers", color.RGBA{210, 225, 235, 255}
	case code >= 95:
		return catStorm, "Thunderstorm", color.RGBA{255, 220, 50, 255}
	default:
		return catCloudy, "Cloudy", color.RGBA{160, 160, 165, 255}
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

// boxOutline returns the stroke color for cards: weather-condition colored
// (gold=sun, blue=rain, etc.) and red for alerts (storm/severe).
func boxOutline(cat category) color.RGBA {
	switch {
	case cat == catStorm:
		return color.RGBA{255, 60, 60, 255} // red alert
	case cat == catSnow:
		return color.RGBA{160, 190, 230, 255} // snow blue
	case cat == catRain:
		return color.RGBA{50, 120, 200, 255} // blue
	case cat == catFog:
		return color.RGBA{140, 170, 200, 255} // fog
	case cat == catCloudy:
		return color.RGBA{140, 140, 145, 255} // grey
	case cat == catPartly:
		return color.RGBA{220, 180, 50, 255} // amber
	default:
		return color.RGBA{255, 200, 50, 255} // gold
	}
}

// drawBox renders a white card with a colored outline at the given width.
func drawBox(dc *gg.Context, x, y, w, h, lw float64, col color.RGBA) {
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(x, y, w, h)
	dc.SetColor(col)
	dc.SetLineWidth(lw)
	dc.Stroke()
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

// drawPortrait renders a clean grid layout on portrait panels:
// top half = large time + date in a centered box,
// bottom half = weather data in colored-card boxes.
// Box outlines are colored by weather condition (gold=sun, blue=rain, red=storm alert).
func drawPortrait(dc *gg.Context, fs *plugins.FontSet, d Data, w, _ int) {
	fw := float64(w)
	cat, _, _ := codeInfo(d.Code)
	col := boxOutline(cat)

	cx := fw / 2
	pad := 10.0
	cw := fw - 2*pad // 10px padding clears the 8px edge guard + 3px box stroke

	// ========== TOP HALF: TIME ==========
	topY := pad + 5.0
	topH := 170.0
	drawBox(dc, pad, topY, cw, topH, 3, col)

	if d.Time != "" {
		if f, ok := plugins.FitFace(dc, fs, plugins.StyleTitle, d.Time, 56, cw); ok {
			dc.SetFontFace(f)
		}
		dc.SetRGB(0, 0, 0)
		dc.DrawStringAnchored(d.Time, cx, topY+70, 0.5, 0.5)
	}
	if d.Date != "" {
		setFace(dc, fs, 22, plugins.StyleSans)
		dc.SetRGB(0, 0, 0)
		dc.DrawStringAnchored(d.Date, cx, topY+108, 0.5, 0.5)
	}
	if d.Place != "" {
		setFace(dc, fs, 18, plugins.StyleSans)
		dc.SetRGB(0, 0, 0)
		dc.DrawStringAnchored(d.Place, cx, topY+140, 0.5, 0.5)
	}

	// ========== BOTTOM HALF: WEATHER ==========
	botY := topY + topH + 10

	// Temp + icon card
	tH := 75.0
	drawBox(dc, pad, botY, cw, tH, 3, col)
	drawIcon(dc, cat, 48, botY+tH/2, 34, col)
	dc.SetRGB(0, 0, 0)
	if f, ok := plugins.FitFace(dc, fs, plugins.StyleTitle, fmt.Sprintf("%.0f°", d.Temp), 40, 140); ok {
		dc.SetFontFace(f)
	}
	dc.DrawStringAnchored(fmt.Sprintf("%.0f°", d.Temp), 180, botY+tH/2-6, 0.5, 0.5)
	setFace(dc, fs, 18, plugins.StyleSans)
	dc.DrawStringAnchored(conditionFor(cat), 180, botY+tH/2+18, 0.5, 0.5)

	// Humidity | Wind row
	rowY := botY + tH + 6
	rowH := 36.0
	halfW := (cw - 12) / 2

	drawBox(dc, pad, rowY, halfW, rowH, 2, col)
	setFace(dc, fs, 22, plugins.StyleSans)
	dc.SetRGB(0, 0, 0)
	dc.DrawStringAnchored(fmt.Sprintf("%d%%", d.Humidity), pad+halfW/2, rowY+20, 0.5, 0.5)

	drawBox(dc, pad+halfW+12, rowY, halfW, rowH, 2, col)
	dc.DrawStringAnchored(fmt.Sprintf("%.0f %s", d.Wind, d.WindUnit), pad+halfW+12+halfW/2, rowY+20, 0.5, 0.5)

	// Feels-like
	fY := rowY + rowH + 6
	fH := 32.0
	drawBox(dc, pad, fY, cw, fH, 2, col)
	setFace(dc, fs, 18, plugins.StyleSans)
	dc.SetRGB(0, 0, 0)
	dc.DrawStringAnchored(fmt.Sprintf("Feels %.0f°", d.FeelsLike), cx, fY+20, 0.5, 0.5)

	// Forecast — compact single line per day, no icons
	frY := fY + fH + 6
	frH := 48.0
	drawBox(dc, pad, frY, cw, frH, 2, col)
	setFace(dc, fs, 17, plugins.StyleSans)
	dc.SetRGB(0, 0, 0)
	for i := range d.Days {
		if i >= 2 {
			break
		}
		y := frY + 12 + float64(i)*17
		dc.DrawStringAnchored(
			fmt.Sprintf("%s %.0f°/%.0f° · %d%%", d.Days[i].Heading, d.Days[i].Lo, d.Days[i].Hi, d.Days[i].PrecipPct),
			cx, y, 0.5, 0.5,
		)
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

// drawLabel draws the footer label for landscape layouts.
func drawLabel(dc *gg.Context, fs *plugins.FontSet, d Data, w, h int) {
	text := d.Label
	if text == "" {
		text = "Weather"
	}
	setFace(dc, fs, 16, plugins.StyleSans)
	dc.SetRGB(0, 0, 0)
	dc.DrawStringAnchored(text, 40, float64(h)-14, 0, 0.5)
	if d.Place != "" {
		dc.DrawStringAnchored(d.Place, float64(w-d.FooterReserveRight)-40, float64(h)-14, 1, 0.5)
	}
}
