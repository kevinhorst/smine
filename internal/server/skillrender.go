package server

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// skillEntryIdPattern defers to internal/skills/entries.go for the authoritative full-line parse.
var skillEntryIdPattern = regexp.MustCompile(`^SKILL-[A-Z0-9]{2,24}-[A-Z0-9]{2,12}-[0-9]{3}[a-z]?$`)

var skillEntryTagPattern = regexp.MustCompile(`^[a-z]+$`)

// skillEntryRefPattern over-matches on purpose; on-page set membership is the false-positive guard.
var skillEntryRefPattern = regexp.MustCompile(`\b([A-Z][A-Z0-9]{1,15}-[0-9]{3}[a-z]?)(\.\.[0-9]{3}[a-z]?)?\b`)

var skillFrontmatterPattern = regexp.MustCompile(`(?s)\A---\n.*?\n---\n`)

var skillSlugPattern = regexp.MustCompile(`[^a-z0-9]+`)

// Renderer priorities matter only relative to goldmark's defaults (1000); the transformer runs after tables exist.
const (
	skillEntryRendererPriority  = 90
	permChipRendererPriority    = 100
	skillDocTransformerPriority = 500

	skillEntryTagPlain = "plain"

	contextDirActions = "actions"
	contextDirRules   = "rules"

	// contextDocMinSegments: a context-doc path needs at least <dir>/<file>.
	contextDocMinSegments = 2
	// entryIdParts: SKILL-<NAME>-<rest> — the short form is the third part.
	entryIdParts = 3
)

var kindSkillEntryBlock = ast.NewNodeKind("SkillEntryBlock")

var kindSkillMermaidBlock = ast.NewNodeKind("SkillMermaidBlock")

type skillDocTransformer struct {
	toc *[]skillTocEntry
}

func (t *skillDocTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	source := reader.Source()
	replaceMermaidBlocks(doc, source)
	groupEntryBlocks(doc, source)
	t.collectToc(doc, source)
	rewriteContextLinks(doc)
	linkifyEntryRefs(doc, source, entryAnchors(doc))
}

func (t *skillDocTransformer) collectToc(doc *ast.Document, source []byte) {
	for child := doc.FirstChild(); child != nil; child = child.NextSibling() {
		heading, isHeading := child.(*ast.Heading)
		if !isHeading || heading.Level != 2 {
			continue
		}

		title := nodeText(heading, source)
		entry := skillTocEntry{
			Anchor: headingAnchor(heading, title),
			Title:  title,
		}
		*t.toc = append(*t.toc, entry)
	}
}

// skillEntryBlock absorbs following siblings so an entry's lists, tables and fences render inside its box.
type skillEntryBlock struct {
	ast.BaseBlock

	entryId string
	tag     string
}

func (n *skillEntryBlock) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

func (n *skillEntryBlock) Kind() ast.NodeKind { return kindSkillEntryBlock }

type skillEntryBlockRenderer struct{}

func (r *skillEntryBlockRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindSkillEntryBlock, r.renderBlock)
}

func (r *skillEntryBlockRenderer) renderBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		_, _ = w.WriteString("</div>\n")
		return ast.WalkContinue, nil
	}

	// entryId and tag are pattern-validated (skillEntryInfo), safe to splice.
	block := node.(*skillEntryBlock)
	_, _ = w.WriteString(`<div class="skill-entry skill-entry-` + block.tag + `" id="` + strings.ToLower(block.entryId) + `">`)
	_, _ = w.WriteString(`<div class="skill-entry-head"><span class="skill-entry-id">` + block.entryId + `</span>`)
	if block.tag != skillEntryTagPlain {
		_, _ = w.WriteString(`<span class="skill-entry-chip chip-` + block.tag + `">` + block.tag + `</span>`)
	}
	_, _ = w.WriteString(`</div>`)
	return ast.WalkContinue, nil
}

// skillEntryParagraphRenderer degrades malformed entries to plain paragraphs: SKILL.md structure is arbitrary.
type skillEntryParagraphRenderer struct{}

func (r *skillEntryParagraphRenderer) renderParagraph(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		_, _ = w.WriteString("</p>\n")
		return ast.WalkContinue, nil
	}

	if parent, isBlock := node.Parent().(*skillEntryBlock); isBlock && parent.FirstChild() == node {
		_, _ = w.WriteString(`<p class="skill-entry-title">`)
		return ast.WalkContinue, nil
	}

	_, _ = w.WriteString("<p>")
	return ast.WalkContinue, nil
}

func (r *skillEntryParagraphRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindParagraph, r.renderParagraph)
}

// skillMermaidBlock renders as <pre class="mermaid"> so the vendored mermaid.js draws it client-side.
type skillMermaidBlock struct {
	ast.BaseBlock

	source string
}

func (n *skillMermaidBlock) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

func (n *skillMermaidBlock) Kind() ast.NodeKind { return kindSkillMermaidBlock }

type skillMermaidRenderer struct{}

func (r *skillMermaidRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindSkillMermaidBlock, r.renderMermaid)
}

func (r *skillMermaidRenderer) renderMermaid(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	block := node.(*skillMermaidBlock)
	_, _ = w.WriteString(`<pre class="mermaid">`)
	template.HTMLEscape(w, []byte(block.source))
	_, _ = w.WriteString("</pre>\n")
	return ast.WalkSkipChildren, nil
}

type skillRenderResult struct {
	Body template.HTML
	Toc  []skillTocEntry
}

type skillTocEntry struct {
	Anchor string
	Title  string
}

// renderSkillMarkdown drops only the chip decoration on a settings problem, never the page.
func (s *Server) renderSkillMarkdown(src []byte) (skillRenderResult, error) {
	var result skillRenderResult
	renderers := []util.PrioritizedValue{
		util.Prioritized(&skillEntryBlockRenderer{}, skillEntryRendererPriority),
		util.Prioritized(&skillEntryParagraphRenderer{}, skillEntryRendererPriority),
		util.Prioritized(&skillMermaidRenderer{}, skillEntryRendererPriority),
	}
	if state, ok := s.permState(); ok {
		renderers = append(renderers, util.Prioritized(&permCodeSpanRenderer{state: state, tmpl: s.tmpl}, permChipRendererPriority))
	}

	transformer := &skillDocTransformer{toc: &result.Toc}
	markdown := newMarkdown(
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(util.Prioritized(transformer, skillDocTransformerPriority)),
		),
		goldmark.WithRendererOptions(renderer.WithNodeRenderers(renderers...)),
	)
	var buf bytes.Buffer
	if err := markdown.Convert(src, &buf); err != nil {
		return result, fmt.Errorf("renderSkillMarkdown: %w", err)
	}

	result.Body = template.HTML(buf.String())
	return result, nil
}

// contextDocDestination accepts both the raw $AGENT_CONTEXT_DIR_DEFAULT form and the deploy-substituted one.
func contextDocDestination(destination string) string {
	if strings.HasPrefix(destination, "http://") || strings.HasPrefix(destination, "https://") || strings.HasPrefix(destination, "#") {
		return ""
	}

	trimmed := strings.TrimPrefix(destination, "$AGENT_CONTEXT_DIR_DEFAULT/")
	path, fragment, hasFragment := strings.Cut(trimmed, "#")
	if !strings.HasSuffix(path, ".md") {
		return ""
	}

	segments := strings.Split(path, "/")
	if len(segments) < contextDocMinSegments {
		return ""
	}
	dir := segments[len(segments)-2]
	file := segments[len(segments)-1]
	if dir != contextDirRules && dir != contextDirActions {
		return ""
	}

	route := "/context/" + dir + "/" + file
	if hasFragment {
		route += "#" + fragment
	}
	return route
}

// entryAnchors keys both the full id and its short form without the SKILL-<NAME>- prefix.
func entryAnchors(doc *ast.Document) map[string]string {
	anchors := make(map[string]string)
	for child := doc.FirstChild(); child != nil; child = child.NextSibling() {
		block, isBlock := child.(*skillEntryBlock)
		if !isBlock {
			continue
		}

		anchor := strings.ToLower(block.entryId)
		anchors[block.entryId] = anchor
		if parts := strings.SplitN(block.entryId, "-", entryIdParts); len(parts) == entryIdParts {
			anchors[parts[2]] = anchor
		}
	}
	return anchors
}

// groupEntryBlocks strips the id/tag prefix off the headline because the block head carries them.
func groupEntryBlocks(doc *ast.Document, source []byte) {
	child := doc.FirstChild()
	for child != nil {
		paragraph, isParagraph := child.(*ast.Paragraph)
		if !isParagraph {
			child = child.NextSibling()
			continue
		}
		entryId, tag, isEntry := skillEntryInfo(paragraph, source)
		if !isEntry {
			child = child.NextSibling()
			continue
		}

		block := &skillEntryBlock{
			entryId: entryId,
			tag:     tag,
		}
		doc.ReplaceChild(doc, paragraph, block)
		stripEntryHeadline(paragraph, source)
		block.AppendChild(block, paragraph)
		for sibling := block.NextSibling(); sibling != nil; sibling = block.NextSibling() {
			if isEntryBoundary(sibling, source) {
				break
			}
			block.AppendChild(block, sibling)
		}
		child = block.NextSibling()
	}
}

func headingAnchor(heading *ast.Heading, title string) string {
	if value, hasId := heading.AttributeString("id"); hasId {
		if id, isBytes := value.([]byte); isBytes {
			return string(id)
		}
	}

	anchor := strings.Trim(skillSlugPattern.ReplaceAllString(strings.ToLower(title), "-"), "-")
	heading.SetAttributeString("id", []byte(anchor))
	return anchor
}

// inlineText matches the segment-join in permCodeSpanRenderer.
func inlineText(node ast.Node, source []byte) string {
	var buf bytes.Buffer
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if textNode, isText := child.(*ast.Text); isText {
			buf.Write(textNode.Segment.Value(source))
		}
	}
	return buf.String()
}

func isEntryBoundary(node ast.Node, source []byte) bool {
	if _, isHeading := node.(*ast.Heading); isHeading {
		return true
	}

	paragraph, isParagraph := node.(*ast.Paragraph)
	if !isParagraph {
		return false
	}
	_, _, isEntry := skillEntryInfo(paragraph, source)
	return isEntry
}

// linkifyEntryRefs never touches code spans or existing links.
func linkifyEntryRefs(doc *ast.Document, source []byte, anchors map[string]string) {
	if len(anchors) == 0 {
		return
	}

	var texts []*ast.Text
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node.(type) {
		case *ast.AutoLink, *ast.CodeSpan, *ast.Link:
			return ast.WalkSkipChildren, nil
		}
		if textNode, isText := node.(*ast.Text); isText {
			texts = append(texts, textNode)
		}
		return ast.WalkContinue, nil
	})

	for _, textNode := range texts {
		linkifyTextNode(textNode, source, anchors)
	}
}

// linkifyTextNode keeps every piece a source-segment node so escaping and line-break flags survive.
func linkifyTextNode(textNode *ast.Text, source []byte, anchors map[string]string) {
	content := textNode.Segment.Value(source)
	matches := skillEntryRefPattern.FindAllSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return
	}

	parent := textNode.Parent()
	segmentStart := textNode.Segment.Start
	cursor := 0
	replaced := false
	for _, match := range matches {
		anchor, resolvable := anchors[string(content[match[2]:match[3]])]
		// A '-' before the token marks the tail of a longer id (RULE-PLAN-038 contains PLAN-038).
		if !resolvable || (match[0] > 0 && content[match[0]-1] == '-') {
			continue
		}

		if match[0] > cursor {
			lead := ast.NewTextSegment(text.NewSegment(segmentStart+cursor, segmentStart+match[0]))
			parent.InsertBefore(parent, textNode, lead)
		}
		link := ast.NewLink()
		link.Destination = []byte("#" + anchor)
		token := ast.NewTextSegment(text.NewSegment(segmentStart+match[0], segmentStart+match[1]))
		link.AppendChild(link, token)
		parent.InsertBefore(parent, textNode, link)
		cursor = match[1]
		replaced = true
	}
	if !replaced {
		return
	}

	tail := ast.NewTextSegment(text.NewSegment(segmentStart+cursor, textNode.Segment.Stop))
	tail.SetSoftLineBreak(textNode.SoftLineBreak())
	tail.SetHardLineBreak(textNode.HardLineBreak())
	parent.InsertBefore(parent, textNode, tail)
	parent.RemoveChild(parent, textNode)
}

// nodeText walks all descendants: headings may nest inline nodes inlineText would miss.
func nodeText(node ast.Node, source []byte) string {
	var buf bytes.Buffer
	_ = ast.Walk(node, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if textNode, isText := child.(*ast.Text); isText {
			buf.Write(textNode.Segment.Value(source))
		}
		return ast.WalkContinue, nil
	})
	return buf.String()
}

func replaceMermaidBlocks(doc *ast.Document, source []byte) {
	var fences []*ast.FencedCodeBlock
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if fence, isFence := node.(*ast.FencedCodeBlock); isFence && string(fence.Language(source)) == "mermaid" {
			fences = append(fences, fence)
		}
		return ast.WalkContinue, nil
	})

	for _, fence := range fences {
		var buf bytes.Buffer
		for index := 0; index < fence.Lines().Len(); index++ {
			segment := fence.Lines().At(index)
			buf.Write(segment.Value(source))
		}
		block := &skillMermaidBlock{source: buf.String()}
		fence.Parent().ReplaceChild(fence.Parent(), fence, block)
	}
}

func rewriteContextLinks(doc *ast.Document) {
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if link, isLink := node.(*ast.Link); isLink {
			if destination := contextDocDestination(string(link.Destination)); destination != "" {
				link.Destination = []byte(destination)
			}
		}
		return ast.WalkContinue, nil
	})
}

func skillEntryInfo(paragraph ast.Node, source []byte) (string, string, bool) {
	strong, isStrong := paragraph.FirstChild().(*ast.Emphasis)
	if !isStrong || strong.Level != 2 {
		return "", "", false
	}

	entryId := inlineText(strong, source)
	if !skillEntryIdPattern.MatchString(entryId) {
		return "", "", false
	}

	tag := skillEntryTagPlain
	for sibling := strong.NextSibling(); sibling != nil; sibling = sibling.NextSibling() {
		span, isSpan := sibling.(*ast.CodeSpan)
		if !isSpan {
			continue
		}

		tag = strings.Trim(inlineText(span, source), "[]")
		break
	}

	if !skillEntryTagPattern.MatchString(tag) {
		tag = skillEntryTagPlain
	}
	return entryId, tag, true
}

func stripEntryHeadline(paragraph *ast.Paragraph, source []byte) {
	strong, isStrong := paragraph.FirstChild().(*ast.Emphasis)
	if !isStrong || strong.Level != 2 {
		return
	}

	paragraph.RemoveChild(paragraph, strong)
	trimLeadingSpace(paragraph, source)
	if span, isSpan := paragraph.FirstChild().(*ast.CodeSpan); isSpan {
		if skillEntryTagPattern.MatchString(strings.Trim(inlineText(span, source), "[]")) {
			paragraph.RemoveChild(paragraph, span)
			trimLeadingSpace(paragraph, source)
		}
	}
}

func stripSkillFrontmatter(src []byte) []byte {
	return skillFrontmatterPattern.ReplaceAll(src, nil)
}

func trimLeadingSpace(paragraph *ast.Paragraph, source []byte) {
	for {
		textNode, isText := paragraph.FirstChild().(*ast.Text)
		if !isText || strings.TrimSpace(string(textNode.Segment.Value(source))) != "" {
			return
		}
		paragraph.RemoveChild(paragraph, textNode)
	}
}
