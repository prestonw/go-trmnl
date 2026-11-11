package deviceapi_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gesellix/go-trmnl/internal/device"
	"github.com/gesellix/go-trmnl/internal/deviceapi"
	"github.com/gesellix/go-trmnl/internal/render"
	"github.com/gesellix/go-trmnl/internal/server"
	"github.com/gesellix/go-trmnl/internal/store"

	_ "github.com/gesellix/go-trmnl/internal/plugins/clock"
)

func newTestServerDir(t *testing.T) (*httptest.Server, *store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	r := server.New()
	deviceapi.New(st, "http://test.local", dir, false).Routes(r)
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts, st, dir
}

func TestDisplayRendersClockScreen(t *testing.T) {
	ts, st, dir := newTestServerDir(t)

	// Provision a device and give it a playlist with one clock screen.
	device.Provision(st, testMAC, "", "")
	do(t, ts, http.MethodGet, "/api/setup", map[string]string{"ID": testMAC}, "").Body.Close()
	d, _ := st.GetDeviceByMAC(testMAC)
	pl, _ := st.CreatePlaylist("default")
	pg, _ := st.CreatePlugin("clock", "Clock")
	sc, _ := st.CreateScreen(pg.ID, "Clock", `{"use_24h":true,"label":"Office"}`)
	st.AddPlaylistItem(pl.ID, sc.ID)
	st.UpdateDeviceSettings(d.ID, "", 900, sql.NullInt64{Int64: pl.ID, Valid: true}, "classic", false, 0, 0)

	resp := do(t, ts, http.MethodGet, "/api/display", map[string]string{
		"ID": testMAC, "Access-Token": d.APIKey,
	}, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		ImageURL string `json:"image_url"`
		Filename string `json:"filename"`
	}
	json.NewDecoder(resp.Body).Decode(&body)

	if body.Filename == "placeholder" || body.Filename == "" {
		t.Fatalf("expected a rendered hash filename, got %q", body.Filename)
	}
	if !strings.HasSuffix(body.ImageURL, body.Filename+".bmp") {
		t.Errorf("image_url %q does not match filename %q", body.ImageURL, body.Filename)
	}

	// The rendered BMP and PNG must exist on disk.
	for _, ext := range []string{".bmp", ".png"} {
		if _, err := os.Stat(filepath.Join(dir, body.Filename+ext)); err != nil {
			t.Errorf("rendered file %s missing: %v", body.Filename+ext, err)
		}
	}

	// The screen's rendered hash is recorded.
	got, _ := st.GetScreen(sc.ID)
	if !got.RenderedHash.Valid || got.RenderedHash.String != body.Filename {
		t.Errorf("screen rendered_hash = %+v, want %q", got.RenderedHash, body.Filename)
	}
}

// provisionDevice registers a device, assigns it the playlist and sets whether
// it draws the battery indicator.
func provisionDevice(t *testing.T, ts *httptest.Server, st *store.Store, mac string, playlistID int64, battery bool) *store.Device {
	t.Helper()
	if _, _, err := device.Provision(st, mac, "", ""); err != nil {
		t.Fatalf("provision %s: %v", mac, err)
	}
	do(t, ts, http.MethodGet, "/api/setup", map[string]string{"ID": mac}, "").Body.Close()
	d, err := st.GetDeviceByMAC(mac)
	if err != nil {
		t.Fatalf("get device: %v", err)
	}
	if err := st.UpdateDeviceSettings(d.ID, "", 900, sql.NullInt64{Int64: playlistID, Valid: true}, "classic", battery, 0, 0); err != nil {
		t.Fatalf("settings: %v", err)
	}
	d, _ = st.GetDeviceByMAC(mac)
	return d
}

// displayFilename polls /api/display with a battery voltage and returns the
// served image's content hash.
func displayFilename(t *testing.T, ts *httptest.Server, d *store.Device, volts string) string {
	t.Helper()
	resp := do(t, ts, http.MethodGet, "/api/display", map[string]string{
		"ID": d.MAC, "Access-Token": d.APIKey, "Battery-Voltage": volts,
	}, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Filename == "" || body.Filename == "placeholder" {
		t.Fatalf("no screen rendered: filename = %q", body.Filename)
	}
	return body.Filename
}

// bmpSize reads the width/height from a 1-bit BMP3 header (BITMAPINFOHEADER
// at offset 14: width u32 LE at +4, height u32 LE at +8).
func bmpSize(t *testing.T, path string) (int, int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	w := int(uint32(data[18]) | uint32(data[19])<<8 | uint32(data[20])<<16 | uint32(data[21])<<24)
	h := int(uint32(data[22]) | uint32(data[23])<<8 | uint32(data[24])<<16 | uint32(data[25])<<24)
	return w, h
}

// A device reporting a non-standard panel size must get screens rendered at
// that size, served from its own per-device cache rather than the shared one.
func TestDisplayRendersAtDevicePanelSize(t *testing.T) {
	ts, st, dir := newTestServerDir(t)
	pl, _ := st.CreatePlaylist("default")
	pg, _ := st.CreatePlugin("clock", "Clock")
	sc, _ := st.CreateScreen(pg.ID, "Clock", `{"use_24h":true,"label":"Office"}`)
	st.AddPlaylistItem(pl.ID, sc.ID)

	// Provision both devices, then give one the non-standard portrait panel.
	provisionDevice(t, ts, st, "AA:BB:CC:DD:EE:31", pl.ID, false)
	provisionDevice(t, ts, st, "AA:BB:CC:DD:EE:32", pl.ID, false)
	portrait, _ := st.GetDeviceByMAC("AA:BB:CC:DD:EE:31")
	standard, _ := st.GetDeviceByMAC("AA:BB:CC:DD:EE:32")
	if err := st.UpdateDeviceSettings(portrait.ID, "", 900, sql.NullInt64{Int64: pl.ID, Valid: true}, "classic", false, 320, 480); err != nil {
		t.Fatalf("settings: %v", err)
	}

	standardName := displayFilename(t, ts, standard, "3.95")
	porName := displayFilename(t, ts, portrait, "3.95")

	standardW, standardH := bmpSize(t, filepath.Join(dir, standardName+".bmp"))
	if standardW != render.Width || standardH != render.Height {
		t.Errorf("standard device render = %dx%d, want %dx%d", standardW, standardH, render.Width, render.Height)
	}
	// The non-standard render must be a different image at the panel size.
	if porName == standardName {
		t.Fatalf("both devices shared the same cached image %q", standardName)
	}
	porW, porH := bmpSize(t, filepath.Join(dir, porName+".bmp"))
	if porW != 320 || porH != 480 {
		t.Fatalf("non-standard device render = %dx%d, want 320x480", porW, porH)
	}
}

// A device with the battery indicator sees a different image than one without,
// so the two must not share the per-screen render cache.
func TestBatteryIndicatorRendersPerDevice(t *testing.T) {
	ts, st, _ := newTestServerDir(t)
	pl, _ := st.CreatePlaylist("default")
	pg, _ := st.CreatePlugin("clock", "Clock")
	sc, _ := st.CreateScreen(pg.ID, "Clock", `{"use_24h":true,"label":"Office"}`)
	st.AddPlaylistItem(pl.ID, sc.ID)

	plain := provisionDevice(t, ts, st, "AA:BB:CC:DD:EE:21", pl.ID, false)
	withBattery := provisionDevice(t, ts, st, "AA:BB:CC:DD:EE:22", pl.ID, true)

	plainName := displayFilename(t, ts, plain, "3.95")
	batteryName := displayFilename(t, ts, withBattery, "3.95")
	if plainName == batteryName {
		t.Fatalf("both devices got the same image %q; the indicator was not drawn or the cache is shared", plainName)
	}

	// Serving the same device again must hit its own cache.
	if again := displayFilename(t, ts, withBattery, "3.95"); again != batteryName {
		t.Errorf("second poll re-rendered: %q != %q", again, batteryName)
	}

	// A voltage within the same level keeps the image; a lower level changes it.
	if same := displayFilename(t, ts, withBattery, "4.05"); same != batteryName {
		t.Errorf("4.05 V changed the image although it is the same level: %q != %q", same, batteryName)
	}
	if lower := displayFilename(t, ts, withBattery, "3.35"); lower == batteryName {
		t.Error("an empty battery drew the same image as a full one")
	}
}
