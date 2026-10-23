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

// Rule shapes match only known tool names, so ordinary code like Load(path) stays undecorated.
var (
	permToolPattern = regexp.MustCompile(`^(Bash|Read|Edit|Write|Glob|Grep|WebFetch|WebSearch|Task|Skill|SlashCommand|NotebookEdit)\(.+\)$`)
	permMCPPattern  = regexp.MustCompile(`^mcp__[A-Za-z0-9_]+$`)
)

func isPermRule(text string) bool {
	return permToolPattern.MatchString(text) || permMCPPattern.MatchString(text)
}

const (
	chipAbsent   = "absent"
	chipAllowed  = "allowed"
	chipAsk      = "ask"
	chipDisabled = "disabled"
)

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

func (s *Server) permState() (permRuleState, bool) {
	main, disabled, err := s.loadBoth()
	if err != nil {
		return permRuleState{}, false
	}

	mainPerms, mainErr := main.Permissions()
	disabledPerms, disabledErr := disabled.Permissions()
	if mainErr != nil || disabledErr != nil {
		return permRuleState{}, false
	}

	state := permRuleState{
		Allow:         mainPerms.Allow,
		Ask:           mainPerms.Ask,
		DisabledAllow: disabledPerms.Allow,
		DisabledAsk:   disabledPerms.Ask,
	}
	return state, true
}

// permChip is shared by the render-time decoration and the add endpoint's response.
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
	// Embedded newlines render as spaces, matching goldmark's stock code-span behavior.
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

// renderDocMarkdown degrades to the plain renderer: doc pages never fail over decoration.
func (s *Server) renderDocMarkdown(src []byte) (template.HTML, error) {
	state, ok := s.permState()
	if !ok {
		return renderMarkdown(src)
	}
	md := newMarkdown(goldmark.WithRendererOptions(
		renderer.WithNodeRenderers(util.Prioritized(&permCodeSpanRenderer{state: state, tmpl: s.tmpl}, 100)),
	))
	var buf bytes.Buffer
	if err := md.Convert(src, &buf); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}
