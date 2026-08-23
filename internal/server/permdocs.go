package server

import (
	"bytes"
	"html/template"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

const tmplPermChip = "_perm_chip.html"

// Permission-rule shapes recognized in doc code spans: Tool(specifier) for a
// known tool name, or a bare mcp__ tool name. Whitelisted tools keep ordinary
// code like Load(path) undecorated (raw.md D5).
var (
	permToolPattern = regexp.MustCompile(`^(Bash|Read|Edit|Write|Glob|Grep|WebFetch|WebSearch|Task|Skill|SlashCommand|NotebookEdit)\(.+\)$`)
	permMCPPattern  = regexp.MustCompile(`^mcp__[A-Za-z0-9_]+$`)
)

func isPermRule(text string) bool {
	return permToolPattern.MatchString(text) || permMCPPattern.MatchString(text)
}

// Chip states: absent (apply button), already live (passive badge), or parked
// in settings.disabled.json (enable button).
const (
	chipAbsent   = "absent"
	chipAllowed  = "allowed"
	chipAsk      = "ask"
	chipDisabled = "disabled"
)

// permRuleState is the live+disabled permission membership a doc render
// decorates against.
type permRuleState struct {
	Allow         []string
	Ask           []string
	DisabledAllow []string
	DisabledAsk   []string
}

type permChipData struct {
	Rule   string
	Status string
	AddURL string
}

// permChip resolves a rule's chip state against live+disabled membership;
// shared by the render-time decoration and the add endpoint's response.
func permChip(rule string, state permRuleState) permChipData {
	status := chipAbsent
	switch {
	case slices.Contains(state.Allow, rule):
		status = chipAllowed
	case slices.Contains(state.Ask, rule):
		status = chipAsk
	case slices.Contains(state.DisabledAllow, rule), slices.Contains(state.DisabledAsk, rule):
		status = chipDisabled
	}
	return permChipData{
		Rule:   rule,
		Status: status,
		AddURL: "/api/permissions/add?rule=" + url.QueryEscape(rule),
	}
}

// permCodeSpanRenderer replaces goldmark's code-span output: matched rules
// render as the chip template, everything else as the stock <code> element.
type permCodeSpanRenderer struct {
	state permRuleState
	tmpl  *template.Template
}

func (r *permCodeSpanRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindCodeSpan, r.renderCodeSpan)
}

func (r *permCodeSpanRenderer) renderCodeSpan(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	// Join the span's text segments; embedded newlines render as spaces,
	// matching goldmark's stock code-span behavior.
	var textBuf bytes.Buffer
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if textNode, ok := child.(*ast.Text); ok {
			textBuf.Write(textNode.Segment.Value(source))
		}
	}
	text := strings.ReplaceAll(textBuf.String(), "\n", " ")
	if !isPermRule(text) {
		_, _ = w.WriteString("<code>")
		template.HTMLEscape(w, []byte(text))
		_, _ = w.WriteString("</code>")
		return ast.WalkSkipChildren, nil
	}
	var buf bytes.Buffer
	if err := r.tmpl.ExecuteTemplate(&buf, tmplPermChip, permChip(text, r.state)); err != nil {
		return ast.WalkStop, err
	}
	_, _ = w.Write(buf.Bytes())
	return ast.WalkSkipChildren, nil
}

// renderDocMarkdown renders doc markdown with permission-rule code spans
// decorated against live-settings state; any settings problem degrades to the
// plain renderer — doc pages never fail over decoration (raw.md D6).
func (s *Server) renderDocMarkdown(src []byte) (template.HTML, error) {
	main, disabled, err := s.loadBoth()
	if err != nil {
		return renderMarkdown(src)
	}
	mainPerms, mainErr := main.Permissions()
	disabledPerms, disabledErr := disabled.Permissions()
	if mainErr != nil || disabledErr != nil {
		return renderMarkdown(src)
	}
	state := permRuleState{
		Allow:         mainPerms.Allow,
		Ask:           mainPerms.Ask,
		DisabledAllow: disabledPerms.Allow,
		DisabledAsk:   disabledPerms.Ask,
	}
	md := goldmark.New(goldmark.WithRendererOptions(
		renderer.WithNodeRenderers(util.Prioritized(&permCodeSpanRenderer{state: state, tmpl: s.tmpl}, 100)),
	))
	var buf bytes.Buffer
	if err := md.Convert(src, &buf); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}
