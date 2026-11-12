package deviceapi_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gesellix/go-trmnl/internal/device"
)

// TestDisplayLiveCadence verifies that a screen with a live refresh cadence
// (a) makes /api/display hand out that cadence as refresh_rate (faster than the
// device's base rate, since the client polls at it) and (b) is actually
// re-rendered on that cadence (the clock's seconds change the frame).
func TestDisplayLiveCadence(t *testing.T) {
	ts, st, _ := newTestServerDir(t)

	device.Provision(st, testMAC, "", "")
	do(t, ts, http.MethodGet, "/api/setup", map[string]string{"ID": testMAC}, "").Body.Close()
	d, _ := st.GetDeviceByMAC(testMAC)
	pl, _ := st.CreatePlaylist("default")
	pg, _ := st.CreatePlugin("clock", "Clock")
	sc, _ := st.CreateScreen(pg.ID, "Clock", `{"use_24h":true,"label":"Office"}`)
	st.AddPlaylistItem(pl.ID, sc.ID)
	st.UpdateDeviceSettings(d.ID, "", 60, sql.NullInt64{Int64: pl.ID, Valid: true}, "classic", false, 0, 0)

	poll := func() (filename string, refresh int) {
		resp := do(t, ts, http.MethodGet, "/api/display", map[string]string{
			"ID": d.MAC, "Access-Token": d.APIKey,
		}, "")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		var body struct {
			Filename    string `json:"filename"`
			RefreshRate int    `json:"refresh_rate"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return body.Filename, body.RefreshRate
	}

	// Baseline: no live cadence → device base rate (60) is handed out.
	f0, r0 := poll()
	if f0 == "" || f0 == "placeholder" {
		t.Fatalf("no rendered frame: %q", f0)
	}
	if r0 != 60 {
		t.Fatalf("baseline refresh_rate = %d, want 60 (device base)", r0)
	}

	// Switch the screen to a 5s live cadence (invalidates the cached render).
	if err := st.UpdateScreenRefresh(sc.ID, sql.NullInt64{Int64: 5, Valid: true}); err != nil {
		t.Fatal(err)
	}
	if _, r1 := poll(); r1 != 5 {
		t.Fatalf("live refresh_rate = %d, want 5", r1)
	}
	renderedAt := func() int64 {
		got, err := st.GetScreen(sc.ID)
		if err != nil || !got.RenderedAt.Valid {
			t.Fatalf("screen rendered_at not set: %+v (%v)", got, err)
		}
		return got.RenderedAt.Int64
	}
	// This poll may still serve a render produced by the baseline poll (within
	// the last 5s). Force a fresh render by clearing the record, and anchor the
	// TTL window to that render's time.
	st.UpdateScreenRefresh(sc.ID, sql.NullInt64{Int64: 5, Valid: true})
	poll() // re-render now (record was invalidated)
	at1 := renderedAt()

	// Within the 5s TTL the same frame is served (no pointless re-render).
	f2, _ := poll()
	if f2 != f0 {
		t.Fatalf("frame changed within cadence: %q -> %q", f0, f2)
	}
	if at2 := renderedAt(); at2 != at1 {
		t.Fatalf("rendered_at advanced within cadence: %d -> %d", at1, at2)
	}

	// Past the TTL the screen re-renders (rendered_at advances; the clock is
	// minute-resolution, so the hash only changes on a minute rollover).
	time.Sleep(5500 * time.Millisecond)
	poll()
	if at3 := renderedAt(); at3 <= at1 {
		t.Fatalf("rendered_at not advanced after cadence elapsed: %d -> %d", at1, at3)
	}

	// Clear the cadence: the device base rate is handed out again.
	if err := st.UpdateScreenRefresh(sc.ID, sql.NullInt64{}); err != nil {
		t.Fatal(err)
	}
	if _, r2 := poll(); r2 != 60 {
		t.Fatalf("after clear refresh_rate = %d, want 60", r2)
	}
}
