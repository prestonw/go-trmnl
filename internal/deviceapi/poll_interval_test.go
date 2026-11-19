package deviceapi

import (
	"database/sql"
	"testing"

	"github.com/gesellix/go-trmnl/internal/store"
)

// TestClockFromSettings pins the deviceapi side of the draw_clock contract:
// only draw_clock:true emits a clock block; the zone/font/format fall back to
// the top-left time card defaults; partial_refresh clamps to whole minutes in
// [60,3600] and partial_refresh_wifi defaults to false (offline tick).
func TestClockFromSettings(t *testing.T) {
	// Flag absent or false: no clock block (server bakes the time).
	for _, s := range []string{`{}`, `{"draw_clock":false}`, `{"latitude":1}`, `{not json`} {
		if c := clockFromSettings(s); c != nil {
			t.Errorf("clockFromSettings(%s) = %+v, want nil", s, c)
		}
	}
	if clockFromSettings("") != nil {
		t.Error("empty settings must not emit a clock block")
	}

	// Flag true, all defaults: time-band only (NOT the whole card) so the
	// device can white-out + redraw just the digits without erasing the baked
	// date/place rows. 24h, 60s, radio off.
	c := clockFromSettings(`{"draw_clock":true}`)
	if c == nil {
		t.Fatal("draw_clock:true must emit a clock block")
	}
	if c.X != 24 || c.Y != 58 || c.Width != 376 || c.Height != 104 {
		t.Errorf("default zone = %d,%d %dx%d, want 24,58 376x104 (time band)", c.X, c.Y, c.Width, c.Height)
	}
	if c.Font != 110 {
		t.Errorf("default font = %d, want 110", c.Font)
	}
	if c.Format != "24h" {
		t.Errorf("default format = %q, want 24h", c.Format)
	}
	if c.PartialRefresh != 60 {
		t.Errorf("default partial_refresh = %d, want 60", c.PartialRefresh)
	}
	if c.PartialRefreshWiFi {
		t.Error("partial_refresh_wifi must default to false (offline tick)")
	}

	// Explicit overrides, including 12h and a radio-up wake.
	c = clockFromSettings(`{"draw_clock":true,"clock_x":10,"clock_y":2,"clock_w":300,"clock_h":180,"clock_font_size":96,"clock_24h":false,"partial_refresh":180,"partial_refresh_wifi":true}`)
	if c == nil {
		t.Fatal("full override settings must emit a clock block")
	}
	if c.X != 10 || c.Y != 2 || c.Width != 300 || c.Height != 180 {
		t.Errorf("override zone = %d,%d %dx%d, want 10,2 300x180", c.X, c.Y, c.Width, c.Height)
	}
	if c.Font != 96 || c.Format != "12h" {
		t.Errorf("override font/format = %d/%q, want 96/12h", c.Font, c.Format)
	}
	if c.PartialRefresh != 180 || !c.PartialRefreshWiFi {
		t.Errorf("override cadence = %d/%v, want 180/true", c.PartialRefresh, c.PartialRefreshWiFi)
	}

	// Cadence clamps to whole minutes in [60,3600].
	clamps := []struct {
		in   string
		want int
	}{
		{`{"draw_clock":true,"partial_refresh":5}`, 60},    // below minimum
		{`{"draw_clock":true,"partial_refresh":90}`, 90},   // valid whole-minute
		{`{"draw_clock":true,"partial_refresh":7200}`, 3600}, // above maximum
		{`{"draw_clock":true,"partial_refresh":0}`, 60},   // zero => default
		{`{"draw_clock":true,"partial_refresh":-5}`, 60},  // negative => default
	}
	for _, tc := range clamps {
		got := clockFromSettings(tc.in)
		if got == nil || got.PartialRefresh != tc.want {
			t.Errorf("clamps %s: partial = %v, want %d", tc.in, got, tc.want)
		}
	}
}

// TestPollInterval is a table test for the cadence precedence: a live screen
// beats the device base rate, sub-5s cadences clamp to 5, and the placeholder
// path (no screen) falls back to the device rate.
func TestPollInterval(t *testing.T) {
	dev := &store.Device{RefreshRate: 60}
	live := &store.Screen{RefreshSeconds: sql.NullInt64{Int64: 10, Valid: true}}
	oneSecond := &store.Screen{RefreshSeconds: sql.NullInt64{Int64: 1, Valid: true}}
	plain := &store.Screen{}

	cases := []struct {
		name string
		d    *store.Device
		sc   *store.Screen
		want int
	}{
		{"live 10s beats base 60s", dev, live, 10},
		{"sub-5s clamps to 5", dev, oneSecond, 5},
		{"no live cadence falls back to base", dev, plain, 60},
		{"placeholder (no screen) uses base rate", dev, nil, 60},
		{"no device falls back to 60", nil, nil, 60},
	}
	for _, tc := range cases {
		if got := pollInterval(tc.d, tc.sc); got != tc.want {
			t.Errorf("%s: pollInterval = %d, want %d", tc.name, got, tc.want)
		}
	}
}
