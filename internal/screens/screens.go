// Package screens renders a configured screen to cached image files. It is the
// single rendering path shared by the device display endpoint and the admin
// preview, so both produce byte-identical output.
package screens

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/gesellix/go-trmnl/internal/plugins"
	"github.com/gesellix/go-trmnl/internal/render"
	"github.com/gesellix/go-trmnl/internal/store"
)

// Render computes a screen's data model, draws it via its plugin, reduces it to
// 1-bit with mode, writes the cached BMP+PNG, records the content hash on the
// screen row, and returns the render result. device may be nil for previews.
func Render(ctx context.Context, st *store.Store, r *render.Renderer, assetsDir string, device *store.Device, sc *store.Screen, mode render.Mode) (render.Result, error) {
	sans, _, _ := st.GetSetting("font_sans")
	mono, _, _ := st.GetSetting("font_mono")
	title, _, _ := st.GetSetting("font_title")
	bundle := "classic"
	if device != nil && device.FontBundle != "" {
		bundle = device.FontBundle
	}
	fonts := plugins.NewFontSet(assetsDir, bundle, sans, mono, title)

	pluginRow, err := st.GetPlugin(sc.PluginID)
	if err != nil {
		return render.Result{}, err
	}
	p, ok := plugins.Get(pluginRow.Type)
	if !ok {
		return render.Result{}, errors.New("unknown plugin type " + pluginRow.Type)
	}

	battery := device != nil && device.ShowBattery
	// Non-standard panel sizes (e.g. a 320x480 portrait DIY screen) are
	// per-device renders: the shared per-screen cache is keyed on the standard
	// TRMNL size, so mixing sizes would leak one device's image to another.
	own := battery || DeviceHasNonStandardSize(device)
	width, height := render.Width, render.Height
	if device != nil && device.Width > 0 && device.Height > 0 {
		width, height = device.Width, device.Height
	}
	in := plugins.RenderInput{
		Device:    device,
		Screen:    sc,
		Settings:  json.RawMessage(sc.SettingsJSON),
		Now:       time.Now(),
		Width:     width,
		Height:    height,
		AssetsDir: assetsDir,
		Fonts:     fonts,
	}
	if battery {
		in.FooterReserveRight = render.BatteryFooterWidth
	}
	model, err := p.DataModel(ctx, in)
	if err != nil {
		return render.Result{}, err
	}
	img, err := p.Render(ctx, in, model)
	if err != nil {
		return render.Result{}, err
	}
	if battery {
		level, charging := BatteryState(device)
		render.DrawBatteryIndicator(img, level, charging)
	}

	var res render.Result
	if device != nil && device.ColorEnabled {
		res, err = r.ProcessColor(img)
	} else {
		res, err = r.Process(img, mode)
	}
	if err != nil {
		return render.Result{}, err
	}
	// A screen with the indicator or a non-standard size looks different per
	// device, so it cannot go into the shared per-screen cache.
	if own {
		level, charging := BatteryState(device)
		_ = st.SetDeviceScreenRender(device.ID, sc.ID, res.Hash, render.BatteryState(level, charging))
	} else {
		_ = st.SetScreenRendered(sc.ID, res.Hash)
	}
	return res, nil
}

// DeviceHasNonStandardSize reports whether the device's panel differs from the
// standard TRMNL dimensions. Its renders cannot be shared with other devices,
// so they go into the per-device render cache.
func DeviceHasNonStandardSize(d *store.Device) bool {
	return d != nil && d.Width > 0 && d.Height > 0 &&
		(d.Width != render.Width || d.Height != render.Height)
}

// BatteryState reports what the footer indicator would show for a device: its
// coarse charge level and whether it reported itself as charging. Telemetry a
// device has never sent leaves the level unknown.
func BatteryState(d *store.Device) (render.BatteryLevel, bool) {
	if d == nil {
		return render.BatteryUnknown, false
	}
	level := render.BatteryUnknown
	if d.BatteryVoltage.Valid {
		level = render.BatteryLevelFor(d.BatteryVoltage.Float64)
	}
	return level, d.BatteryCharging.Valid && d.BatteryCharging.Bool
}
