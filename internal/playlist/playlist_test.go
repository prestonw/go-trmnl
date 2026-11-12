package playlist_test

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/gesellix/go-trmnl/internal/playlist"
	"github.com/gesellix/go-trmnl/internal/store"
)

func TestNextScreenRoundRobin(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	pl, _ := st.CreatePlaylist("default")
	var screenIDs []int64
	for _, name := range []string{"a", "b", "c"} {
		p, _ := st.CreatePlugin("clock", name)
		sc, _ := st.CreateScreen(p.ID, name, "{}")
		st.AddPlaylistItem(pl.ID, sc.ID)
		screenIDs = append(screenIDs, sc.ID)
	}

	d, _ := st.CreateDevice(&store.Device{MAC: "AA:BB:CC:DD:EE:01", APIKey: "k", FriendlyID: "F1"})
	if err := st.UpdateDeviceSettings(d.ID, "d.ID", 900, sql.NullInt64{Int64: pl.ID, Valid: true}, "classic", false, false, 0, 0); err != nil {
		t.Fatal(err)
	}

	// Six picks should cycle a, b, c, a, b, c.
	want := []int64{screenIDs[0], screenIDs[1], screenIDs[2], screenIDs[0], screenIDs[1], screenIDs[2]}
	for i, wantID := range want {
		d, _ = st.GetDeviceByID(d.ID) // reload to pick up advanced cursor
		sc, err := playlist.NextScreen(st, d)
		if err != nil {
			t.Fatalf("pick %d: %v", i, err)
		}
		if sc.ID != wantID {
			t.Errorf("pick %d: screen %d, want %d", i, sc.ID, wantID)
		}
	}
}

func TestNextScreenNoPlaylist(t *testing.T) {
	st, _ := store.Open(filepath.Join(t.TempDir(), "t.db"))
	defer st.Close()
	d, _ := st.CreateDevice(&store.Device{MAC: "AA:BB:CC:DD:EE:02", APIKey: "k", FriendlyID: "F2"})
	if _, err := playlist.NextScreen(st, d); !errors.Is(err, playlist.ErrNoScreen) {
		t.Fatalf("err = %v, want ErrNoScreen", err)
	}
}

func TestPeekScreenDoesNotAdvance(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	pl, _ := st.CreatePlaylist("default")
	var screenIDs []int64
	for _, name := range []string{"a", "b"} {
		p, _ := st.CreatePlugin("clock", name)
		sc, _ := st.CreateScreen(p.ID, name, "{}")
		st.AddPlaylistItem(pl.ID, sc.ID)
		screenIDs = append(screenIDs, sc.ID)
	}

	d, _ := st.CreateDevice(&store.Device{MAC: "AA:BB:CC:DD:EE:03", APIKey: "k", FriendlyID: "F3"})
	if err := st.UpdateDeviceSettings(d.ID, "d.ID", 900, sql.NullInt64{Int64: pl.ID, Valid: true}, "classic", false, false, 0, 0); err != nil {
		t.Fatal(err)
	}

	// Peeking twice sees the same screen, and the next pick is that screen too.
	for i := 0; i < 2; i++ {
		d, _ = st.GetDeviceByID(d.ID)
		sc, err := playlist.PeekScreen(st, d)
		if err != nil {
			t.Fatalf("peek %d: %v", i, err)
		}
		if sc.ID != screenIDs[0] {
			t.Errorf("peek %d: screen %d, want %d", i, sc.ID, screenIDs[0])
		}
	}
	d, _ = st.GetDeviceByID(d.ID)
	sc, err := playlist.NextScreen(st, d)
	if err != nil {
		t.Fatal(err)
	}
	if sc.ID != screenIDs[0] {
		t.Errorf("next: screen %d, want %d", sc.ID, screenIDs[0])
	}
}

// TestNextScreenHoldsLiveScreen verifies that a screen with a live refresh
// cadence holds the device's cursor (no playlist advance) while it is showing,
// and that clearing the cadence resumes round-robin.
func TestNextScreenHoldsLiveScreen(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	pl, _ := st.CreatePlaylist("default")
	pA, _ := st.CreatePlugin("clock", "A")
	scA, _ := st.CreateScreen(pA.ID, "A", "{}")
	pB, _ := st.CreatePlugin("clock", "B")
	scB, _ := st.CreateScreen(pB.ID, "B", "{}")
	st.AddPlaylistItem(pl.ID, scA.ID)
	st.AddPlaylistItem(pl.ID, scB.ID)

	d, _ := st.CreateDevice(&store.Device{MAC: "AA:BB:CC:DD:EE:02", APIKey: "testkey-hold-live", FriendlyID: "F2"})
	if err := st.UpdateDeviceSettings(d.ID, "d.ID", 900, sql.NullInt64{Int64: pl.ID, Valid: true}, "classic", false, false, 0, 0); err != nil {
		t.Fatal(err)
	}

	// Make A live.
	if err := st.UpdateScreenRefresh(scA.ID, sql.NullInt64{Int64: 10, Valid: true}); err != nil {
		t.Fatal(err)
	}

	// Three consecutive picks should all return A and the cursor must not move.
	for i := 0; i < 3; i++ {
		d, _ = st.GetDeviceByID(d.ID)
		sc, err := playlist.NextScreen(st, d)
		if err != nil {
			t.Fatalf("pick %d: %v", i, err)
		}
		if sc.ID != scA.ID {
			t.Fatalf("pick %d: got screen %d, want live screen %d (held)", i, sc.ID, scA.ID)
		}
		d, _ = st.GetDeviceByID(d.ID)
		if d.PlaylistCursor != 0 {
			t.Fatalf("pick %d: cursor = %d, want 0 (held while live)", i, d.PlaylistCursor)
		}
	}

	// Clear A's cadence: round-robin resumes from where the cursor was.
	if err := st.UpdateScreenRefresh(scA.ID, sql.NullInt64{}); err != nil {
		t.Fatal(err)
	}
	d, _ = st.GetDeviceByID(d.ID)
	if sc, err := playlist.NextScreen(st, d); err != nil || sc.ID != scA.ID {
		t.Fatalf("after clear: got %v (%v), want A at cursor 0", sc, err)
	}
	d, _ = st.GetDeviceByID(d.ID)
	if d.PlaylistCursor != 1 {
		t.Fatalf("after clear: cursor = %d, want 1 (advanced)", d.PlaylistCursor)
	}
	if sc, err := playlist.NextScreen(st, d); err != nil || sc.ID != scB.ID {
		t.Fatalf("next after clear: got %v (%v), want B", sc, err)
	}
}
