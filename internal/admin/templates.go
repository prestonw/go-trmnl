package admin

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"strconv"
)

//go:embed templates/*.gohtml
var tmplFS embed.FS

//go:embed static/*
var staticFS embed.FS

// pages are the content templates; each is parsed together with the layout.
var pages = []string{
	"dashboard", "devices", "device", "logs",
	"screens", "screen", "playlists", "playlist", "settings",
	"calendar", "calendar_account",
}

type templateSet struct {
	tmpls map[string]*template.Template
}

func mustParseTemplates() *templateSet {
	funcs := template.FuncMap{
		"ns": func(n sql.NullString) string { return n.String },
		"ni": func(n sql.NullInt64) string {
			if !n.Valid {
				return ""
			}
			return strconv.FormatInt(n.Int64, 10)
		},
		"nf": nullFloat,
		"since": func(n sql.NullInt64) string {
			if !n.Valid {
				return "never"
			}
			return humanSince(n.Int64)
		},
		"ago": humanSince,
		"def": func(def, v string) string {
			if v == "" {
				return def
			}
			return v
		},
		"s": func(vals ...string) []string { return vals },
		"slice": func(vals ...string) []string { return vals },
		"parseJSON": func(s string) map[string]any {
			if s == "" {
				return map[string]any{}
			}
			var m map[string]any
			if err := json.Unmarshal([]byte(s), &m); err != nil {
				return map[string]any{}
			}
			return m
		},
		"hasInkplateField": func(key, val string, cfg string) bool {
			if cfg == "" {
				return val == ""
			}
			var m map[string]any
			if err := json.Unmarshal([]byte(cfg), &m); err != nil {
				return val == ""
			}
			return fmt.Sprintf("%v", m[key]) == val
		},
		"inkplateField": func(key, def string, cfg string) string {
			if cfg == "" {
				return def
			}
			var m map[string]any
			if err := json.Unmarshal([]byte(cfg), &m); err != nil {
				return def
			}
			v, ok := m[key]
			if !ok {
				return def
			}
			return fmt.Sprintf("%v", v)
		},
	}
	ts := &templateSet{tmpls: map[string]*template.Template{}}
	for _, p := range pages {
		t := template.New("layout.gohtml").Funcs(funcs)
		t = template.Must(t.ParseFS(tmplFS, "templates/layout.gohtml", "templates/"+p+".gohtml"))
		ts.tmpls[p] = t
	}
	return ts
}

func (ts *templateSet) execute(w io.Writer, page string, data any) error {
	t, ok := ts.tmpls[page]
	if !ok {
		return errNoTemplate(page)
	}
	return t.ExecuteTemplate(w, "layout.gohtml", data)
}

type errNoTemplate string

func (e errNoTemplate) Error() string { return "no such template: " + string(e) }

func nullFloat(n sql.NullFloat64, prec int) string {
	if !n.Valid {
		return ""
	}
	return strconv.FormatFloat(n.Float64, 'f', prec, 64)
}

func staticFileServer() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}
