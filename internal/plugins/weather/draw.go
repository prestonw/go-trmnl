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

// drawBox draws a filled white card with a colored outline.
// The white fill masks the vibrant background tint so text stays readable.
func drawBox(dc *gg.Context, x, y, w, h, lw float64, col color.RGBA) {
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(x, y, w, h)
	dc.Fill()
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

// drawPortrait renders a news-style grid on portrait panels:
// top half = large time + date in a centered box,
// bottom half = weather data in colored-card boxes.
// Box outlines are colored by weather condition (gold=sun, blue=rain, red=storm alert).
// Background tint is set by Render().
func drawPortrait(dc *gg.Context, fs *plugins.FontSet, d Data, w, h int) {
	fw := float64(w)
	cat, _, _ := codeInfo(d.Code)
	col := boxOutline(cat)

	cx := fw / 2
	pad := 20.0
	cw := fw - 2*pad

	// ========== TOP HALF: TIME ==========
	topY := pad + 5.0
	topH := 170.0
	drawBox(dc, pad, topY, cw, topH, 6, col)

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
	drawBox(dc, pad, botY, cw, tH, 5, col)
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

	drawBox(dc, pad, rowY, halfW, rowH, 4, col)
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

// inkplateFill is the light-grey card fill for the e-paper layout. The 1-bit
// render pipeline dithers it (Floyd-Steinberg) into a faint dot pattern that
// reads as light grey on the panel.
var inkplateFill = color.RGBA{216, 216, 216, 255}

// inkplateBlack is the single ink color used for all icons, text, and
// outlines in the e-paper layout.
var inkplateBlack = color.RGBA{0, 0, 0, 255}

// inkplateBox draws a light-grey card with a black outline.
func inkplateBox(dc *gg.Context, x, y, w, h, lw float64) {
	dc.SetColor(inkplateFill)
	dc.DrawRectangle(x, y, w, h)
	dc.Fill()
	dc.SetColor(inkplateBlack)
	dc.SetLineWidth(lw)
	dc.Stroke()
}

// drawInkplate renders the 1-bit e-paper dashboard (800x600 landscape):
// white field, light-grey cards with black outlines, all-black text and
// icons. Same content as the Waveshare dashboard, rescaled for the panel.
// The bottom-right corner is left free for the server-drawn battery
// indicator.
func drawInkplate(dc *gg.Context, fs *plugins.FontSet, d Data, w, h int) {
	fw, fh := float64(w), float64(h)
	pad := 20.0
	gap := 20.0

	lx, lw := pad, fw*0.48
	rx := lx + lw + gap
	rw := fw - rx - pad

	// ===== Time card (top left) =====
	topY := pad
	topH := fh*0.42
	inkplateBox(dc, lx, topY, lw, topH, 4)
	cx := lx + lw/2
	// Clock text is drawn locally by the device via RTC. Only date/place
	// are baked into the server image when SkipClock is true.
	if !d.SkipClock && d.Time != "" {
		if f, ok := plugins.FitFace(dc, fs, plugins.StyleTitle, d.Time, 110, lw-40); ok {
			dc.SetFontFace(f)
		}
		dc.SetRGB(0, 0, 0)
		dc.DrawStringAnchored(d.Time, cx, topY+topH*0.36, 0.5, 0.5)
	}
	if d.Date != "" {
		setFace(dc, fs, 34, plugins.StyleSans)
		dc.SetRGB(0, 0, 0)
		dc.DrawStringAnchored(d.Date, cx, topY+topH*0.64, 0.5, 0.5)
	}
	if d.Place != "" {
		if f, ok := plugins.FitFace(dc, fs, plugins.StyleSans, d.Place, 28, lw-40); ok {
			dc.SetFontFace(f)
		}
		dc.SetRGB(0, 0, 0)
		dc.DrawStringAnchored(d.Place, cx, topY+topH*0.85, 0.5, 0.5)
	}

	// ===== Temp card (top right) =====
	cat, _, _ := codeInfo(d.Code)
	inkplateBox(dc, rx, topY, rw, topH, 4)
	drawIcon(dc, cat, rx+62, topY+topH/2, 68, inkplateBlack)
	tempStr := fmt.Sprintf("%.0f°", d.Temp)
	if f, ok := plugins.FitFace(dc, fs, plugins.StyleTitle, tempStr, 96, rw-160); ok {
		dc.SetFontFace(f)
	}
	dc.SetRGB(0, 0, 0)
	dc.DrawStringAnchored(tempStr, rx+150, topY+topH/2-26, 0, 0.5)
	setFace(dc, fs, 30, plugins.StyleSans)
	dc.SetRGB(0, 0, 0)
	dc.DrawStringAnchored(d.CondLabel, rx+150, topY+topH/2+34, 0, 0.5)

	// ===== Middle row: feels/sun card (left), three stat cards (right) =====
	midY := topY + topH + gap
	midH := 88.0

	// The middle row uses its own split, wider on the right: the feels/sun
	// card needs ~260px of text, so the stat cards get 130px each instead
	// of 105px. That lets the stat labels render at body size (28pt, same
	// as "Feels like") and the values big, instead of one shrunken string.
	mLw := 310.0
	mRx := lx + mLw + gap
	mRw := fw - mRx - pad

	inkplateBox(dc, lx, midY, mLw, midH, 3)
	setFace(dc, fs, 28, plugins.StyleSans)
	dc.SetRGB(0, 0, 0)
	dc.DrawStringAnchored(fmt.Sprintf("Feels like %.0f°%s", d.FeelsLike, d.TempUnit), lx+26, midY+30, 0, 0.5)
	if d.Sunrise != "" || d.Sunset != "" {
		dc.DrawStringAnchored(fmt.Sprintf("Sun %s - %s", d.Sunrise, d.Sunset), lx+26, midY+62, 0, 0.5)
	}

	prepPct := 0
	if len(d.Days) > 0 {
		prepPct = d.Days[0].PrecipPct
	}
	type inkStat struct {
		label, value, unit string
	}
	stats := []inkStat{
		{"Humidity", fmt.Sprintf("%d%%", d.Humidity), ""},
		{"Wind", fmt.Sprintf("%.0f", d.Wind), d.WindUnit},
		{"Rain", fmt.Sprintf("%d%%", prepPct), ""},
	}
	sw := (mRw - 2*gap) / 3
	for i, st := range stats {
		x := mRx + float64(i)*(sw+gap)
		inkplateBox(dc, x, midY, sw, midH, 2)
		cx := x + sw/2
		// Label: body size, same 28pt as the "Feels like" line.
		if f, ok := plugins.FitFace(dc, fs, plugins.StyleSans, st.label, 28, sw-16); ok {
			dc.SetFontFace(f)
		}
		dc.SetRGB(0, 0, 0)
		dc.DrawStringAnchored(st.label, cx, midY+24, 0.5, 0.5)
		// Value: big number, unit smaller on the same baseline.
		vf, okV := plugins.FitFace(dc, fs, plugins.StyleTitle, st.value, 40, sw-16)
		dc.SetRGB(0, 0, 0)
		if st.unit == "" || !okV {
			dc.SetFontFace(vf)
			dc.DrawStringAnchored(st.value, cx, midY+62, 0.5, 0.5)
			continue
		}
		vw, _ := dc.MeasureString(st.value)
		uf, errU := fs.Face(20, plugins.StyleSans)
		if errU != nil {
			dc.SetFontFace(vf)
			dc.DrawStringAnchored(st.value, cx, midY+62, 0.5, 0.5)
			continue
		}
		dc.SetFontFace(uf)
		uw, _ := dc.MeasureString(st.unit)
		x0 := cx - (vw+6+uw)/2
		dc.SetFontFace(vf)
		dc.DrawStringAnchored(st.value, x0, midY+62, 0, 0.5)
		dc.SetFontFace(uf)
		dc.DrawStringAnchored(st.unit, x0+vw+6, midY+62, 0, 0.5)
	}

	// ===== Forecast card (bottom, full width) =====
	frY := midY + midH + gap
	frH := fh - frY - pad
	inkplateBox(dc, pad, frY, fw-2*pad, frH, 3)

	for i := range d.Days {
		if i >= 2 {
			break
		}
		day := d.Days[i]
		dcat, _, _ := codeInfo(day.Code)
		ry := frY + 46 + float64(i)*62
		drawIcon(dc, dcat, lx+44, ry, 30, inkplateBlack)

		setFace(dc, fs, 32, plugins.StyleTitle)
		dc.SetRGB(0, 0, 0)
		dc.DrawStringAnchored(day.Heading, lx+96, ry, 0, 0.5)

		setFace(dc, fs, 26, plugins.StyleSans)
		dc.DrawStringAnchored(conditionFor(dcat), lx+300, ry, 0, 0.5)
		dc.DrawStringAnchored(fmt.Sprintf("%.0f° / %.0f°", day.Lo, day.Hi), lx+480, ry, 0, 0.5)
		dc.DrawStringAnchored(fmt.Sprintf("UV %d", day.UV), lx+640, ry, 0, 0.5)
		// right edge stays clear on the last row for the battery indicator
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
