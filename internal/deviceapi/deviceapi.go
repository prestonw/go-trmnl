// Package deviceapi implements the firmware-facing HTTP endpoints the TRMNL
// device polls: /api/setup, /api/display and /api/log. It is a thin layer over
// the device, store and render packages.
package deviceapi

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gesellix/go-trmnl/internal/playlist"
	"github.com/gesellix/go-trmnl/internal/plugins"
	"github.com/gesellix/go-trmnl/internal/render"
	"github.com/gesellix/go-trmnl/internal/screens"
	"github.com/gesellix/go-trmnl/internal/store"
	"github.com/go-chi/chi/v5"
)

// placeholderName is the filename of the fallback image served when a device
// has no renderable screen yet.
const placeholderName = "placeholder.bmp"

// Handler bundles the dependencies of the device endpoints.
type Handler struct {
	store       *store.Store
	baseURL     string
	uploadsDir  string
	assetsDir   string
	renderer    *render.Renderer
	disableAuth bool

	// Placeholders are written on demand, one file per panel size; the mutex
	// serializes the check-and-write so concurrent polls don't clobber each
	// other.
	placeholderMu  sync.Mutex
	placeholderErr error
}

// New creates a device API handler. baseURL is the public URL prefix used to
// build image URLs; uploadsDir is where rendered images live and are served
// from at /uploads.
func New(st *store.Store, baseURL, uploadsDir string, disableAuth bool) *Handler {
	return &Handler{
		store:       st,
		baseURL:     baseURL,
		uploadsDir:  uploadsDir,
		assetsDir:   filepath.Join(uploadsDir, "assets"),
		renderer:    render.NewRenderer(uploadsDir),
		disableAuth: disableAuth,
	}
}

// screenImage holds the served image's URL filename and its bare stem.
type screenImage struct {
	urlName string // e.g. "<hash>.bmp" or "placeholder.bmp"
	stem    string // e.g. "<hash>" or "placeholder"
}

// currentImage selects the device's next screen, renders it if needed, and
// returns the served image. It falls back to the placeholder when no screen is
// assigned or rendering fails.
func (h *Handler) currentImage(ctx context.Context, d *store.Device) screenImage {
	screen, err := playlist.NextScreen(h.store, d)
	if err != nil { // ErrNoScreen or a store error: fall back gracefully
		name, _ := h.ensurePlaceholder(d)
		return screenImage{urlName: name, stem: trimExt(name)}
	}
	hash, rerr := h.renderScreen(ctx, d, screen)
	if rerr != nil {
		name, _ := h.ensurePlaceholder(d)
		return screenImage{urlName: name, stem: trimExt(name)}
	}
	return screenImage{urlName: hash + ".bmp", stem: hash}
}

// renderScreen returns the content hash of the screen's current image,
// rendering (and caching) it when the cache is stale or missing.
func (h *Handler) renderScreen(ctx context.Context, d *store.Device, screen *store.Screen) (string, error) {
	pluginRow, err := h.store.GetPlugin(screen.PluginID)
	if err != nil {
		return "", err
	}
	ttl := time.Duration(0)
	if p, ok := plugins.Get(pluginRow.Type); ok {
		ttl = p.DefaultRefresh()
	}
	if hash, ok := h.cachedHash(d, screen, ttl); ok {
		return hash, nil
	}

	res, err := screens.Render(ctx, h.store, h.renderer, h.assetsDir, d, screen, h.ditherModeFor(screen))
	if err != nil {
		return "", err
	}
	return res.Hash, nil
}

// cachedHash returns the cached hash for this device's view of the screen, if
// it is still fresh and the file is present on disk. A device drawing the
// battery indicator sees its own image, so it has its own cache entry.
func (h *Handler) cachedHash(d *store.Device, screen *store.Screen, ttl time.Duration) (string, bool) {
	hash, renderedAt := "", int64(0)
	// Per-device cache when the image differs from the screen's shared render:
	// a battery indicator or a non-standard panel size.
	perDevice := d != nil && (d.ShowBattery || screens.DeviceHasNonStandardSize(d))
	if perDevice {
		cachedState, ok, err := 0, false, error(nil)
		hash, renderedAt, cachedState, ok, err = h.store.DeviceScreenRender(d.ID, screen.ID)
		if err != nil || !ok {
			return "", false
		}
		// The indicator is part of the image, so a changed battery level has to
		// invalidate the cache even while the plugin's refresh interval holds.
		level, charging := screens.BatteryState(d)
		if cachedState != render.BatteryState(level, charging) {
			return "", false
		}
	} else {
		if !screen.RenderedHash.Valid {
			return "", false
		}
		hash = screen.RenderedHash.String
		if screen.RenderedAt.Valid {
			renderedAt = screen.RenderedAt.Int64
		}
	}
	if hash == "" {
		return "", false
	}
	if _, err := os.Stat(filepath.Join(h.uploadsDir, hash+".bmp")); err != nil {
		return "", false
	}
	if ttl > 0 && renderedAt > 0 {
		if time.Since(time.Unix(renderedAt, 0)) >= ttl {
			return "", false
		}
	}
	return hash, true
}

// ditherMode reads the global dithering mode, defaulting to Floyd-Steinberg.
func (h *Handler) ditherMode() render.Mode {
	if v, ok, _ := h.store.GetSetting("dither_mode"); ok {
		return render.ParseMode(v)
	}
	return render.FloydSteinberg
}

// ditherModeFor resolves the effective dithering mode for a screen: its
// per-screen override if set, otherwise the global default.
func (h *Handler) ditherModeFor(sc *store.Screen) render.Mode {
	if sc != nil && sc.DitherMode.Valid && sc.DitherMode.String != "" {
		return render.ParseMode(sc.DitherMode.String)
	}
	return h.ditherMode()
}

func trimExt(name string) string {
	if i := len(name) - len(filepath.Ext(name)); i >= 0 {
		return name[:i]
	}
	return name
}

// Routes mounts the device endpoints onto r under /api.
func (h *Handler) Routes(r chi.Router) {
	r.Route("/api", func(r chi.Router) {
		r.With(h.parseMAC).Get("/setup", h.Setup)
		r.With(h.loadDevice, h.requireToken).Get("/display", h.Display)
		r.With(h.loadDevice).Post("/log", h.Log)
	})
}

var placeholderLines = []string{
	"go-trmnl",
	"",
	"Device registered. No screen assigned yet.",
	"Add a screen in the admin UI.",
}

// ensurePlaceholder writes the fallback image to the uploads directory once,
// sized to the device's panel when it is non-standard.
func (h *Handler) ensurePlaceholder(d *store.Device) (string, error) {
	w, hgt := render.Width, render.Height
	name := placeholderName
	if d != nil && screens.DeviceHasNonStandardSize(d) {
		w, hgt = d.Width, d.Height
		name = fmt.Sprintf("placeholder-%dx%d.bmp", w, hgt)
	}
	h.renderPlaceholder(name, w, hgt)
	return name, h.placeholderErr
}

func (h *Handler) renderPlaceholder(name string, w, height int) {
	h.placeholderMu.Lock()
	defer h.placeholderMu.Unlock()
	if _, err := os.Stat(filepath.Join(h.uploadsDir, name)); err == nil {
		return
	}
	img := render.PlaceholderSized(placeholderLines, w, height)
	var buf bytes.Buffer
	if err := render.EncodeBMP1(&buf, img); err != nil {
		h.placeholderErr = err
		return
	}
	h.placeholderErr = os.WriteFile(filepath.Join(h.uploadsDir, name), buf.Bytes(), 0o644)
}

func (h *Handler) imageURL(name string) string {
	return h.baseURL + "/uploads/" + name
}
