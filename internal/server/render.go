package server

import (
	"bytes"
	"html/template"
	"net/http"

	"github.com/kevinhorst/smine/internal/server/respond"
	"github.com/yuin/goldmark"
)

func (s *Server) renderFragment(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		respond.WithInternalServerError(err, w)
	}
}

// renderMarkdown converts markdown to HTML. goldmark's default sanitization
// (raw HTML disabled) is the injection guard for arbitrary SKILL.md content.
func renderMarkdown(src []byte) (template.HTML, error) {
	var buf bytes.Buffer
	if err := goldmark.Convert(src, &buf); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}
