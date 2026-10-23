// Package workflowgen regenerates the GEN blocks of a skill's stage workflow files and enforces their shape rules.
package workflowgen

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	agentCall            = "agent("
	commonFile           = "common/common.src.js"
	handwrittenCap       = 300
	markerCommon         = "// GEN:common"
	markerEnd            = "// GEN:end"
	markerModels         = "// GEN:models"
	markerPayload        = "// GEN:payload"
	markerSchemas        = "// GEN:schemas"
	payloadConstPrefix   = "PAYLOAD_"
	payloadsDir          = "payloads"
	schemaMarkerFields   = 3
	schemaSuffix         = "_SCHEMA"
	schemasDir           = "schemas"
	severityFixtures     = "severity-fixtures"
	severityFixturesFile = "severity-fixtures.json"
)

// ErrNoStageFiles reports a workflows dir without a stage file.
var ErrNoStageFiles = errors.New("no stage files")

var (
	conductPattern       = regexp.MustCompile(`(?i)token conduct|as few (tool )?calls|never read a file over|do not load tools via toolsearch`)
	copyBackPattern      = regexp.MustCompile(`(?i)EXACTLY as given|byte-for-byte|output you return is that file`)
	generatedMarkers     = []string{markerSchemas, markerCommon, markerModels, markerPayload}
	modelMarkers         = []string{markerModels}
	modelNamePattern     = regexp.MustCompile(`assertModel\(|DEFAULT_SUBAGENT_MODEL`)
	schemaLiteralPattern = regexp.MustCompile(`_(SCHEMA|PROPS|TABLE|EXAMPLES) = [\[{]`)
	// schemaConsts names the consts that are not <NAME>_SCHEMA.
	schemaConsts = map[string]string{
		"finding":        "FINDING_PROPS",
		severityFixtures: "SEVERITY_EXAMPLES",
		"severity-table": "SEVERITY_TABLE",
		"verdict":        "VERDICT_PROPS",
	}
	// SharedProps is resolved by both the generator and the handoff schema compiler.
	SharedProps = []*SharedProp{
		{File: "finding.json", Token: "$FINDING_PROPS"},
		{File: "verdict.json", Token: "$VERDICT_PROPS"},
	}
)

// Generator regenerates the stage files of one workflows dir.
type Generator struct {
	dir    string
	models string
}

// NewGenerator binds a workflows dir to the models block its files carry.
func NewGenerator(dir string, models string) *Generator {
	generator := &Generator{
		dir:    dir,
		models: withNewline(models),
	}
	return generator
}

// common is the shared prelude, empty when the skill has none.
func (g *Generator) common() string {
	data, err := os.ReadFile(filepath.Join(g.dir, commonFile))
	if err != nil {
		return ""
	}
	return withNewline(string(data))
}

// payloadBlock emits the payload file the marker names as one JS string const.
func (g *Generator) payloadBlock(marker string) (string, error) {
	fields := strings.Fields(marker)
	if len(fields) < schemaMarkerFields {
		return "", nil
	}

	entryId := fields[2]
	data, err := os.ReadFile(filepath.Join(g.dir, "..", payloadsDir, entryId+".md"))
	if err != nil {
		return "", fmt.Errorf("Generator.payloadBlock: %w", err) //nolint:staticcheck // ST1005: the Receiver.method prefix is the repo's error-message convention
	}

	text := quoteJson(strings.TrimSuffix(string(data), "\n"))
	return "const " + payloadConst(entryId) + " = " + text + "\n", nil
}

// refresh judges the file on disk under isCheck, else writes and judges the rendered file.
func (g *Generator) refresh(isCheck bool, path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Generator.refresh: %w", err) //nolint:staticcheck // ST1005: the Receiver.method prefix is the repo's error-message convention
	}

	name := filepath.Base(path)
	current := string(data)
	rendered, err := g.render(current)
	if err != nil {
		return nil, fmt.Errorf("Generator.refresh: %s: %w", name, err) //nolint:staticcheck // ST1005: the Receiver.method prefix is the repo's error-message convention
	}

	if rendered == current {
		return shapeFindings(current, name), nil
	}

	if isCheck {
		findings := []string{name + " is stale — run 'make workflow-gen'"}
		return append(findings, shapeFindings(current, name)...), nil
	}

	if err := os.WriteFile(path, []byte(rendered), 0o644); err != nil {
		return nil, fmt.Errorf("Generator.refresh: %w", err) //nolint:staticcheck // ST1005: the Receiver.method prefix is the repo's error-message convention
	}
	return shapeFindings(rendered, name), nil
}

// render returns the stage file with every generated block rebuilt.
func (g *Generator) render(content string) (string, error) {
	var out strings.Builder
	isSkipping := false
	for line := range strings.SplitSeq(strings.TrimSuffix(content, "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, markerSchemas+" "):
			block, err := g.schemaBlock(line)
			if err != nil {
				return "", err
			}
			out.WriteString(line + "\n" + block)
			isSkipping = true
		case strings.HasPrefix(line, markerPayload+" "):
			block, err := g.payloadBlock(line)
			if err != nil {
				return "", err
			}
			out.WriteString(line + "\n" + block)
			isSkipping = true
		case line == markerCommon:
			out.WriteString(line + "\n" + g.common())
			isSkipping = true
		case line == markerModels:
			out.WriteString(line + "\n" + g.models)
			isSkipping = true
		case line == markerEnd:
			out.WriteString(line + "\n")
			isSkipping = false
		case !isSkipping:
			out.WriteString(line + "\n")
		}
	}
	return out.String(), nil
}

// schemaBlock emits one JS const per schema the marker line names.
func (g *Generator) schemaBlock(marker string) (string, error) {
	fields := strings.Fields(marker)
	if len(fields) < schemaMarkerFields {
		return "", nil
	}

	var block strings.Builder
	for name := range strings.SplitSeq(fields[2], ",") {
		document, err := g.schemaJson(name)
		if err != nil {
			return "", err
		}
		block.WriteString("const " + constName(name) + " = " + document + "\n")
	}
	return block.String(), nil
}

func (g *Generator) schemaJson(name string) (string, error) {
	if name == severityFixtures {
		return g.severityExamples()
	}

	dir := filepath.Join(g.dir, "..", schemasDir)
	document, err := compactJson(filepath.Join(dir, name+".json"))
	if err != nil {
		return "", err
	}

	for _, shared := range SharedProps {
		props, err := compactJson(filepath.Join(dir, shared.File))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		document = bytes.ReplaceAll(document, []byte(`"`+shared.Token+`"`), props)
	}

	indented := &bytes.Buffer{}
	if err := json.Indent(indented, document, "", "  "); err != nil {
		return "", fmt.Errorf("Generator.schemaJson: %s: %w", name, err) //nolint:staticcheck // ST1005: the Receiver.method prefix is the repo's error-message convention
	}
	return indented.String(), nil
}

// severityExamples: only example rows travel into a stage file.
func (g *Generator) severityExamples() (string, error) {
	data, err := os.ReadFile(filepath.Join(g.dir, "..", severityFixturesFile))
	if err != nil {
		return "", fmt.Errorf("Generator.severityExamples: %w", err) //nolint:staticcheck // ST1005: the Receiver.method prefix is the repo's error-message convention
	}

	document := &severityDocument{}
	if err := json.Unmarshal(data, document); err != nil {
		return "", fmt.Errorf("Generator.severityExamples: %w", err) //nolint:staticcheck // ST1005: the Receiver.method prefix is the repo's error-message convention
	}

	var rows []string
	for _, fixture := range document.Fixtures {
		if fixture.IsExample {
			rows = append(rows, fixture.row())
		}
	}
	if len(rows) == 0 {
		return "[]", nil
	}
	return "[\n" + strings.Join(rows, ",\n") + "\n]", nil
}

func (g *Generator) Run(isCheck bool) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(g.dir, "*.js"))
	if err != nil {
		return nil, fmt.Errorf("Generator.Run: %w", err) //nolint:staticcheck // ST1005: the Receiver.method prefix is the repo's error-message convention
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("Generator.Run: %s: %w", g.dir, ErrNoStageFiles) //nolint:staticcheck // ST1005: the Receiver.method prefix is the repo's error-message convention
	}

	var findings []string
	for _, path := range paths {
		found, err := g.refresh(isCheck, path)
		if err != nil {
			return nil, err
		}
		findings = append(findings, found...)
	}
	return findings, nil
}

// SharedProp is one placeholder token and the schema-dir file it stands for.
type SharedProp struct {
	File  string
	Token string
}

// severityDocument is the part of severity-fixtures.json the generator reads.
type severityDocument struct {
	Fixtures []*severityFixture `json:"fixtures"`
}

// severityFixture is one fixture row; example rows travel into stage files.
type severityFixture struct {
	Discriminator string `json:"discriminator"`
	Effect        string `json:"effect"`
	Exposure      string `json:"exposure"`
	IsExample     bool   `json:"example"`
	Severity      string `json:"severity"`
}

// row keeps the stage files' key order: effect, exposure, severity, discriminator.
func (f *severityFixture) row() string {
	return fmt.Sprintf(
		"  {\n    \"effect\": %s,\n    \"exposure\": %s,\n    \"severity\": %s,\n    \"discriminator\": %s\n  }",
		quoteJson(f.Effect),
		quoteJson(f.Exposure),
		quoteJson(f.Severity),
		quoteJson(f.Discriminator),
	)
}

// compactJson reads a JSON file without insignificant whitespace.
func compactJson(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("compactJson: %w", err)
	}

	compact := &bytes.Buffer{}
	if err := json.Compact(compact, data); err != nil {
		return nil, fmt.Errorf("compactJson: %s: %w", path, err)
	}
	return compact.Bytes(), nil
}

// constName is the JS const a schema is emitted as.
func constName(schema string) string {
	if name, isNamed := schemaConsts[schema]; isNamed {
		return name
	}
	return strings.ToUpper(strings.ReplaceAll(schema, "-", "_")) + schemaSuffix
}

func handwritten(content string, markers []string) []string {
	var lines []string
	isSkipping := false
	for line := range strings.SplitSeq(strings.TrimSuffix(content, "\n"), "\n") {
		if hasAnyPrefix(line, markers) {
			isSkipping = true
		}

		if line == markerEnd {
			isSkipping = false
			continue
		}

		if !isSkipping {
			lines = append(lines, line)
		}
	}
	return lines
}

func hasAnyPrefix(line string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

// payloadConst is the JS const a payload is emitted as.
func payloadConst(entryId string) string {
	return payloadConstPrefix + strings.ToUpper(strings.ReplaceAll(entryId, "-", "_"))
}

func quoteJson(text string) string {
	quoted := &bytes.Buffer{}
	encoder := json.NewEncoder(quoted)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(text)
	return strings.TrimSuffix(quoted.String(), "\n")
}

// shapeFindings checks one stage file against the workflow-layer shape rules.
func shapeFindings(content string, name string) []string {
	var findings []string
	own := handwritten(content, generatedMarkers)
	if len(own) > handwrittenCap {
		findings = append(findings, fmt.Sprintf("%s: %d handwritten lines — the stage-file cap is %d", name, len(own), handwrittenCap))
	}

	if conductPattern.MatchString(content) {
		findings = append(findings, name+": conduct/limit language is banned — never limit a worker's tool calls or reads")
	}

	if copyBackPattern.MatchString(content) {
		findings = append(findings, name+": copy-back language is banned — no agent re-emits data the script already holds")
	}

	if schemaLiteralPattern.MatchString(strings.Join(own, "\n")) {
		findings = append(findings, name+": schema literal outside a GEN block — schemas live in schemas/*.json")
	}

	if !strings.Contains(content, agentCall) {
		return findings
	}

	if !strings.Contains("\n"+content, "\n"+markerModels+"\n") {
		findings = append(findings, name+": spawns agents without the GEN:models block — every agent() names an asserted model")
	}

	outsideModels := handwritten(content, modelMarkers)
	if !modelNamePattern.MatchString(strings.Join(outsideModels, "\n")) {
		findings = append(findings, name+": spawns agents but never names a model through assertModel or DEFAULT_SUBAGENT_MODEL")
	}
	return findings
}

func withNewline(text string) string {
	if text == "" || strings.HasSuffix(text, "\n") {
		return text
	}
	return text + "\n"
}
