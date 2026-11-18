package deviceapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gesellix/go-trmnl/internal/httpx"
	"github.com/gesellix/go-trmnl/internal/store"
)

// setupResponse is returned from GET /api/setup.
type setupResponse struct {
	Status     int    `json:"status"`
	APIKey     string `json:"api_key"`
	FriendlyID string `json:"friendly_id"`
	ImageURL   string `json:"image_url"`
	Filename   string `json:"filename"`
	Message    string `json:"message"`
}

// Setup returns the credentials for a pre-registered device.
func (h *Handler) Setup(w http.ResponseWriter, r *http.Request) {
	mac := macFromCtx(r.Context())
	log.Printf("deviceapi: setup request for MAC %s from %s", mac, r.RemoteAddr)

	d, err := h.store.GetDeviceByMAC(mac)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			log.Printf("deviceapi: setup for unknown MAC %s", mac)
			httpx.WriteJSON(w, http.StatusNotFound, setupResponse{
				Status: http.StatusNotFound,
			})
			return
		}
		httpx.WriteProblem(w, http.StatusInternalServerError, "/problem#device_setup",
			"internal_server_error", "Failed to lookup device.", r.URL.Path, nil)
		return
	}

	name, _ := h.ensurePlaceholder(d)
	httpx.WriteJSON(w, http.StatusOK, setupResponse{
		Status:     http.StatusOK,
		APIKey:     d.APIKey,
		FriendlyID: d.FriendlyID,
		ImageURL:   h.imageURL(name),
		Filename:   strings.TrimSuffix(name, ".bmp"),
		Message:    "Welcome to go-trmnl!",
	})
}

// displayResponse is returned from GET /api/display. Field names match what the
// firmware parses; firmware_url is null when no update is pending.
type displayResponse struct {
	Status             int     `json:"status"`
	ImageURL           string  `json:"image_url"`
	Filename           string  `json:"filename"`
	RefreshRate        int     `json:"refresh_rate"`
	UpdateFirmware     bool    `json:"update_firmware"`
	FirmwareURL        *string `json:"firmware_url"`
	ResetFirmware      bool    `json:"reset_firmware"`
	SpecialFunction    string  `json:"special_function"`
	ImageURLTimeout    int     `json:"image_url_timeout"`
	TemperatureProfile string  `json:"temperature_profile"`
	Clock              *Clock  `json:"clock,omitempty"`
}

// Clock config: when present, the device draws the clock locally using its RTC
// and partial updates instead of relying on the server-rendered image.
//
// Partial-refresh cadence: the device re-arms a deep-sleep timer to the next
// whole minute at this interval (default 60s) and redraws ONLY the time
// digits via a partial e-paper pulse. The radio stays OFF for these wakes —
// they are the cheapest possible minute tick. `wifi` (default false) is the
// future hook for waking with the radio up to refresh live data (ticker,
// YouTube view count, MQTT, …) on the same partial path.
type Clock struct {
	Enabled         bool   `json:"enabled"`
	X               int    `json:"x"`             // top-left X (px)
	Y               int    `json:"y"`             // top-left Y (px)
	Width           int    `json:"w"`             // zone width (px)
	Height          int    `json:"h"`             // zone height (px)
	Font            int    `json:"font_size"`     // base font size
	Format          string `json:"format"`        // "24h" or "12h"
	PartialRefresh  int    `json:"partial_refresh"`  // seconds between local minute ticks (default 60)
	PartialRefreshWiFi bool `json:"partial_refresh_wifi"` // wake with radio up (default false)
}

// Display persists telemetry from request headers and returns the screen to
// render. Until the render pipeline and playlists exist, it serves a
// placeholder image.
func (h *Handler) Display(w http.ResponseWriter, r *http.Request) {
	d := deviceFromCtx(r.Context())

	if err := h.store.UpdateTelemetry(d.ID, telemetryFromHeaders(r)); err != nil {
		httpx.WriteProblem(w, http.StatusInternalServerError, "/problem#internal",
			"internal_server_error", "Failed to persist telemetry.", r.URL.Path, nil)
		return
	}
	// The device was loaded before this poll's telemetry was stored. Anything
	// rendered from it (the battery indicator) would otherwise be one poll
	// behind.
	if fresh, err := h.store.GetDeviceByID(d.ID); err == nil {
		d = fresh
	}

	img := h.currentImage(r.Context(), d)

	special := "none"
	if d.SpecialFunction.Valid && d.SpecialFunction.String != "" {
		special = d.SpecialFunction.String
	}
	var firmwareURL *string
	if d.FirmwareUpdate && d.FirmwareURL.Valid {
		firmwareURL = &d.FirmwareURL.String
	}

	// Build clock config from the screen's settings if present.
	// When draw_clock is false, the server leaves the time card blank (see
	// weather.SkipClock) and the device redraws the time locally via RTC
	// minute ticks — a single draw_clock flag keeps the two in sync so they
	// can never double-draw or both skip.
	var clockCfg *Clock
	if img.screen != nil && img.screen.SettingsJSON != "" {
		clockCfg = clockFromSettings(img.screen.SettingsJSON)
	}

	httpx.WriteJSON(w, http.StatusOK, displayResponse{
		Status:             0,
		ImageURL:           h.imageURL(img.urlName),
		Filename:           img.stem,
		RefreshRate:        pollInterval(d, img.screen),
		UpdateFirmware:     d.FirmwareUpdate && d.FirmwareURL.Valid,
		FirmwareURL:        firmwareURL,
		ResetFirmware:      d.ResetFirmware,
		SpecialFunction:    special,
		ImageURLTimeout:    0,
		TemperatureProfile: "default",
		Clock:              clockCfg,
	})

	// Firmware update and reset are one-shot: clear them after serving so the
	// device does not loop on them.
	if d.FirmwareUpdate {
		_ = h.store.ClearFirmwareUpdate(d.ID)
	}
	if d.ResetFirmware {
		_ = h.store.ClearResetFirmware(d.ID)
	}
}

// clockFromSettings parses the screen's settings JSON and returns the clock
// block for the display response, or nil when the device should not draw the
// clock locally. The single draw_clock flag is the contract shared with the
// weather plugin's SkipClock: true means the device owns the time card (the
// server leaves it blank), false/absent means the server bakes the time in.
//
// Defaults when the flag is set but individual fields are omitted:
//   - zone: top-left time card on the 800x600 landscape panel (20,20 384x252)
//   - font: 110 (matches the server's baked time face)
//   - format: 24h (device clock is 24-hour by default)
//   - partial_refresh: 60s, clamped to whole minutes in [60, 3600]
//   - partial_refresh_wifi: false (radio stays off during the minute tick)
func clockFromSettings(settingsJSON string) *Clock {
	if settingsJSON == "" {
		return nil
	}
	var s struct {
		DrawClock          *bool  `json:"draw_clock"`
		ClockX             *int   `json:"clock_x"`
		ClockY             *int   `json:"clock_y"`
		ClockW             *int   `json:"clock_w"`
		ClockH             *int   `json:"clock_h"`
		ClockFont          *int   `json:"clock_font_size"`
		Clock24h           *bool  `json:"clock_24h"`
		PartialRefresh     *int   `json:"partial_refresh"`
		PartialRefreshWiFi *bool  `json:"partial_refresh_wifi"`
		Timezone           string `json:"timezone"`
	}
	if err := json.Unmarshal([]byte(settingsJSON), &s); err != nil {
		return nil
	}
	if s.DrawClock == nil || !*s.DrawClock {
		return nil
	}
	x, y, w, h := 20, 20, 384, 252
	if s.ClockX != nil && *s.ClockX > 0 {
		x = *s.ClockX
	}
	if s.ClockY != nil && *s.ClockY > 0 {
		y = *s.ClockY
	}
	if s.ClockW != nil && *s.ClockW > 0 {
		w = *s.ClockW
	}
	if s.ClockH != nil && *s.ClockH > 0 {
		h = *s.ClockH
	}
	font := 110
	if s.ClockFont != nil && *s.ClockFont > 0 {
		font = *s.ClockFont
	}
	fm := "24h"
	if s.Clock24h != nil && !*s.Clock24h {
		fm = "12h"
	}
	partial := 60
	if s.PartialRefresh != nil && *s.PartialRefresh > 0 {
		partial = *s.PartialRefresh
		if partial < 60 {
			partial = 60
		}
		if partial > 3600 {
			partial = 3600
		}
	}
	partialWiFi := false
	if s.PartialRefreshWiFi != nil {
		partialWiFi = *s.PartialRefreshWiFi
	}
	return &Clock{
		Enabled:            true,
		X:                  x,
		Y:                  y,
		Width:              w,
		Height:             h,
		Font:               font,
		Format:             fm,
		PartialRefresh:     partial,
		PartialRefreshWiFi: partialWiFi,
	}
}

// logRequest is the body of POST /api/log.
type logRequest struct {
	Logs []logEntry `json:"logs"`
}

type logEntry struct {
	ID              *int64   `json:"id"`
	Message         *string  `json:"message"`
	CreatedAt       *int64   `json:"created_at"`
	WifiStatus      *string  `json:"wifi_status"`
	WifiSignal      *int64   `json:"wifi_signal"`
	SleepDuration   *int64   `json:"sleep_duration"`
	RefreshRate     *int64   `json:"refresh_rate"`
	FreeHeapSize    *int64   `json:"free_heap_size"`
	MaxAllocSize    *int64   `json:"max_alloc_size"`
	SourcePath      *string  `json:"source_path"`
	SourceLine      *int64   `json:"source_line"`
	WakeReason      *string  `json:"wake_reason"`
	FirmwareVersion *string  `json:"firmware_version"`
	BatteryVoltage  *float64 `json:"battery_voltage"`
	SpecialFunction *string  `json:"special_function"`
}

// Log stores a batch of device log entries and returns 204.
func (h *Handler) Log(w http.ResponseWriter, r *http.Request) {
	d := deviceFromCtx(r.Context())

	var req logRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(&req); err != nil {
		httpx.WriteProblem(w, http.StatusUnprocessableEntity, "/problem#log_payload",
			"unprocessable_content", "Invalid log payload.", r.URL.Path, nil)
		return
	}

	rows := make([]*store.DeviceLog, 0, len(req.Logs))
	for _, e := range req.Logs {
		rows = append(rows, e.toRow(d.ID))
	}
	if err := h.store.InsertLogs(rows); err != nil {
		httpx.WriteProblem(w, http.StatusInternalServerError, "/problem#internal",
			"internal_server_error", "Failed to store logs.", r.URL.Path, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// telemetryFromHeaders extracts the device telemetry sent on the display poll.
// The Refresh-Rate header is ignored on purpose; see store.Telemetry.
func telemetryFromHeaders(r *http.Request) store.Telemetry {
	t := store.Telemetry{
		FWVersion:      nullStr(r.Header.Get("FW-Version")),
		Model:          nullStr(r.Header.Get("Model")),
		Width:          nullInt(r.Header.Get("Width")),
		Height:         nullInt(r.Header.Get("Height")),
		BatteryVoltage: nullFloat(r.Header.Get("Battery-Voltage")),
		RSSI:           nullInt(r.Header.Get("RSSI")),
	}
	// The firmware reports charging state via either Battery-Charging or
	// USB-Connected depending on board.
	if v := firstNonEmpty(r.Header.Get("Battery-Charging"), r.Header.Get("USB-Connected")); v != "" {
		if b, ok := parseBool(v); ok {
			t.BatteryCharging = sql.NullBool{Bool: b, Valid: true}
		}
	}
	if v := r.Header.Get("WiFi-Status"); v != "" {
		t.WifiStatus = nullStr(v)
	}
	return t
}

func (e logEntry) toRow(deviceID int64) *store.DeviceLog {
	return &store.DeviceLog{
		DeviceID:        deviceID,
		LogID:           ptrInt(e.ID),
		Message:         ptrStr(e.Message),
		CreatedAt:       ptrInt(e.CreatedAt),
		WifiStatus:      ptrStr(e.WifiStatus),
		WifiSignal:      ptrInt(e.WifiSignal),
		SleepDuration:   ptrInt(e.SleepDuration),
		RefreshRate:     ptrInt(e.RefreshRate),
		FreeHeapSize:    ptrInt(e.FreeHeapSize),
		MaxAllocSize:    ptrInt(e.MaxAllocSize),
		SourcePath:      ptrStr(e.SourcePath),
		SourceLine:      ptrInt(e.SourceLine),
		WakeReason:      ptrStr(e.WakeReason),
		FirmwareVersion: ptrStr(e.FirmwareVersion),
		BatteryVoltage:  ptrFloat(e.BatteryVoltage),
		SpecialFunction: ptrStr(e.SpecialFunction),
	}
}

// --- small parsing/conversion helpers ---

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func parseBool(s string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	}
	return false, false
}

func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullInt(s string) sql.NullInt64 {
	if s == "" {
		return sql.NullInt64{}
	}
	if v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
		return sql.NullInt64{Int64: v, Valid: true}
	}
	// Some numeric headers (e.g. Width) may arrive as floats; truncate.
	if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
		return sql.NullInt64{Int64: int64(f), Valid: true}
	}
	return sql.NullInt64{}
}

func nullFloat(s string) sql.NullFloat64 {
	if s == "" {
		return sql.NullFloat64{}
	}
	if v, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
		return sql.NullFloat64{Float64: v, Valid: true}
	}
	return sql.NullFloat64{}
}

func ptrInt(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

func ptrStr(p *string) sql.NullString {
	if p == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *p, Valid: true}
}

func ptrFloat(p *float64) sql.NullFloat64 {
	if p == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *p, Valid: true}
}
