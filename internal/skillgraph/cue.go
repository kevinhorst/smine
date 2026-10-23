package skillgraph

import (
	"bytes"
	_ "embed"
	"fmt"
	"reflect"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/ast/astutil"
	"cuelang.org/go/cue/cuecontext"
	cueerrors "cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/cue/parser"
)

const (
	schemaDefinitionCatalog    = "#Catalog"
	schemaDefinitionSkill      = "#Skill"
	schemaDefinitionVocabulary = "#Vocabulary"
	schemaFileName             = "schema.cue"
)

// endOfFile stands in for a missing line when two texts are diffed.
const endOfFile = "<end of file>"

// schemaSource is never held compiled: a cue.Context only grows, and the config server runs indefinitely.
//
//go:embed schema.cue
var schemaSource string

// countComments counts the comment groups attached anywhere in a parsed file.
func countComments(file *ast.File) int {
	comments := 0
	ast.Walk(file, func(node ast.Node) bool {
		comments += len(ast.Comments(node))
		return true
	}, nil)
	return comments
}

// DecodeSkill leaves payload bodies empty — Load reads them; the merge driver and renumber work on the bytes alone.
func DecodeSkill(data []byte, filename string) (*Skill, error) {
	value, err := validateCue(data, schemaDefinitionSkill, filename)
	if err != nil {
		return nil, fmt.Errorf("DecodeSkill: %w", err)
	}

	skill := &Skill{}
	if err := value.Decode(skill); err != nil {
		return nil, fmt.Errorf("DecodeSkill: Failed to decode %s: %w", filename, err)
	}
	return skill, nil
}

// decodeCatalog loads interfaces.cue bytes against #Catalog.
func decodeCatalog(data []byte, filename string) (*Catalog, error) {
	value, err := validateCue(data, schemaDefinitionCatalog, filename)
	if err != nil {
		return nil, fmt.Errorf("decodeCatalog: %w", err)
	}

	catalog := &Catalog{}
	if err := value.Decode(catalog); err != nil {
		return nil, fmt.Errorf("decodeCatalog: Failed to decode %s: %w", filename, err)
	}
	return catalog, nil
}

// decodeVocabulary loads vocabulary.cue bytes against #Vocabulary.
func decodeVocabulary(data []byte, filename string) (*Vocabulary, error) {
	value, err := validateCue(data, schemaDefinitionVocabulary, filename)
	if err != nil {
		return nil, fmt.Errorf("decodeVocabulary: %w", err)
	}

	vocabulary := &Vocabulary{}
	if err := value.Decode(vocabulary); err != nil {
		return nil, fmt.Errorf("decodeVocabulary: Failed to decode %s: %w", filename, err)
	}

	if violations := checkVocabularySources(vocabulary); len(violations) > 0 {
		return nil, fmt.Errorf("decodeVocabulary: %s: %s", filename, strings.Join(violations, "; "))
	}
	return vocabulary, nil
}

// EncodeSkill does not validate — guardRoundTrip's decode does.
func EncodeSkill(skill *Skill) ([]byte, error) {
	value := cuecontext.New().Encode(skill)
	encoded, err := formatCue(value)
	if err != nil {
		return nil, fmt.Errorf("EncodeSkill: %w", err)
	}
	return encoded, nil
}

// encodeVocabulary renders the shared vocabulary as canonical CUE bytes.
func encodeVocabulary(vocabulary *Vocabulary) ([]byte, error) {
	value := cuecontext.New().Encode(vocabulary)
	encoded, err := formatCue(value)
	if err != nil {
		return nil, fmt.Errorf("encodeVocabulary: %w", err)
	}
	return encoded, nil
}

// firstDifference names the first line where two texts differ.
func firstDifference(got, want []byte) string {
	gotLines := strings.Split(string(got), "\n")
	wantLines := strings.Split(string(want), "\n")
	for index := range max(len(gotLines), len(wantLines)) {
		gotLine, wantLine := lineAt(gotLines, index), lineAt(wantLines, index)
		if gotLine != wantLine {
			return fmt.Sprintf("line %d: want %q, got %q", index+1, wantLine, gotLine)
		}
	}
	return "identical text"
}

func formatCue(value cue.Value) ([]byte, error) {
	if err := value.Err(); err != nil {
		return nil, fmt.Errorf("formatCue: Failed to encode: %w", err)
	}

	node := value.Syntax(cue.Final(), cue.Concrete(true))
	expr, isExpr := node.(ast.Expr)
	if !isExpr {
		return nil, fmt.Errorf("formatCue: Syntax returned %T, want an expression", node)
	}

	file, err := astutil.ToFile(expr)
	if err != nil {
		return nil, fmt.Errorf("formatCue: Failed to wrap the syntax in a file: %w", err)
	}

	formatted, err := format.Node(file)
	if err != nil {
		return nil, fmt.Errorf("formatCue: Failed to format: %w", err)
	}
	return formatted, nil
}

// guardRoundTrip carries payload bodies, the vocabulary, the catalog, the dir and the queries across, since they live outside the model file.
func guardRoundTrip(encoded []byte, skill *Skill) error {
	decoded, err := DecodeSkill(encoded, modelDecodeLabel)
	if err != nil {
		return fmt.Errorf("guardRoundTrip: The encoded model does not load: %w", err)
	}

	decoded.Vocabulary = skill.Vocabulary
	decoded.Catalog = skill.Catalog
	decoded.Dir = skill.Dir
	decoded.Queries = skill.Queries
	bodies := payloadBodies(skill)
	for _, ref := range decoded.Entries() {
		if ref.Entry.Payload != nil {
			ref.Entry.Payload.Body = bodies[ref.Entry.Id]
		}
	}

	if !reflect.DeepEqual(decoded, skill) {
		difference := modelDifference(decoded, skill)
		return fmt.Errorf("guardRoundTrip: The decoded model differs from the saved model at %s", difference)
	}
	return nil
}

func guardVocabularyRoundTrip(encoded []byte, filename string) error {
	decoded, err := decodeVocabulary(encoded, filename)
	if err != nil {
		return fmt.Errorf("guardVocabularyRoundTrip: The encoded vocabulary does not load: %w", err)
	}

	reencoded, err := encodeVocabulary(decoded)
	if err != nil {
		return fmt.Errorf("guardVocabularyRoundTrip: %w", err)
	}

	if !bytes.Equal(reencoded, encoded) {
		difference := firstDifference(reencoded, encoded)
		return fmt.Errorf("guardVocabularyRoundTrip: Re-encoding changes %s at %s", filename, difference)
	}
	return nil
}

// lineAt returns line index, or endOfFile past the end.
func lineAt(lines []string, index int) string {
	if index < len(lines) {
		return lines[index]
	}
	return endOfFile
}

func modelDifference(got, want *Skill) string {
	switch {
	case !reflect.DeepEqual(got.Frontmatter, want.Frontmatter):
		return "frontmatter"
	case !reflect.DeepEqual(got.Changelog, want.Changelog):
		return "changelog"
	case got.Tier != want.Tier:
		return "tier"
	case got.Docker != want.Docker:
		return "docker"
	case got.EnableDelegation != want.EnableDelegation:
		return "enableDelegation"
	case !reflect.DeepEqual(got.EnforcePermissions, want.EnforcePermissions):
		return "enforcePermissions"
	case !reflect.DeepEqual(got.Env, want.Env):
		return "env"
	case !reflect.DeepEqual(got.Atoms, want.Atoms):
		return "atoms"
	case !reflect.DeepEqual(got.Enforce, want.Enforce):
		return "enforce"
	case !reflect.DeepEqual(got.Title, want.Title):
		return "title"
	case !reflect.DeepEqual(got.Smoke, want.Smoke):
		return "smoke"
	case !reflect.DeepEqual(got.Requires, want.Requires):
		return "requires"
	case len(got.Sections) != len(want.Sections):
		return "the section list"
	}
	for index, section := range want.Sections {
		if !reflect.DeepEqual(got.Sections[index], section) {
			return fmt.Sprintf("section %q", section.Title)
		}
	}
	return "exclusive, note or preamble"
}

// payloadBodies maps every payload entry id to its body.
func payloadBodies(skill *Skill) map[string]string {
	bodies := map[string]string{}
	for _, ref := range skill.Entries() {
		if ref.Entry.Payload != nil {
			bodies[ref.Entry.Id] = ref.Entry.Payload.Body
		}
	}
	return bodies
}

// validateCue rejects comments (notes are data) and builds its own context per call.
func validateCue(data []byte, definition, filename string) (cue.Value, error) {
	file, err := parser.ParseFile(filename, data, parser.ParseComments)
	if err != nil {
		return cue.Value{}, fmt.Errorf("validateCue: Failed to parse %s: %w", filename, err)
	}

	if comments := countComments(file); comments > 0 {
		return cue.Value{}, fmt.Errorf("validateCue: %s carries %d comment group(s) — the file is machine-canonical, notes are data", filename, comments)
	}

	ctx := cuecontext.New()
	schemaOption := cue.Filename(schemaFileName)
	schema := ctx.CompileString(schemaSource, schemaOption)
	if err := schema.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("validateCue: Failed to compile the embedded schema: %w", err)
	}

	value := ctx.BuildFile(file)
	if err := value.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("validateCue: Failed to build %s: %w", filename, err)
	}

	definitionPath := cue.ParsePath(definition)
	unified := schema.LookupPath(definitionPath).Unify(value)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		details := cueerrors.Details(err, nil)
		return cue.Value{}, fmt.Errorf("validateCue: %s violates %s:\n%s", filename, definition, details)
	}
	return unified, nil
}
