package deviceapi

import (
	"database/sql"
	"testing"

	"github.com/gesellix/go-trmnl/internal/store"
)

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
