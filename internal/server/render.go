package server

import (
	"bytes"
	"html/template"
	"net/http"

	"github.com/kevinhorst/smine/internal/server/respond"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// newMarkdown keeps raw HTML escaped as the injection guard.
func newMarkdown(opts ...goldmark.Option) goldmark.Markdown {
	base := []goldmark.Option{goldmark.WithExtensions(extension.Table)}
	return goldmark.New(append(base, opts...)...)
}

// renderFragment marks every page no-store: a Back navigation refetches live state instead of showing a cached document.
func (s *Server) renderFragment(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		respond.WithInternalServerError(err, w)
	}
}

// renderMarkdown relies on goldmark's raw-HTML-disabled default as the guard for arbitrary SKILL.md content.
func renderMarkdown(src []byte) (template.HTML, error) {
	var buf bytes.Buffer
	if err := newMarkdown().Convert(src, &buf); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}
