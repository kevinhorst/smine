package skillgraph

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/sessiontitle"
	"github.com/kevinhorst/smine/internal/skillenv"
)

// RenderOptions' Disable holds entry ids or prefix globs to drop.
type RenderOptions struct {
	Disable      []string
	HasEval      bool
	Components   []components.ID   // nil = every component (canonical deploy)
	Deferred     map[string]bool   // SKILL_<SKILL>_<NAME> rendered as invocation placeholders; nil = none
	DeferredHint string            // the tail of the abort sentence of a required deferred variable
	Deployable   map[string]bool   // leaf -> deployable in this render; nil = every skill deployable (canonical)
	Values       map[string]string // SKILL_<SKILL>_<NAME> -> value; nil = fill nothing, drop nothing (canonical)
}

// RenderedFile's Path is relative to the skill dir and slash-separated.
type RenderedFile struct {
	Content []byte
	Path    string
}

func checkModel(skill *Skill) error {
	violations := Check(skill)
	if len(violations) == 0 {
		return nil
	}

	errs := make([]error, 0, len(violations))
	for _, violation := range violations {
		errs = append(errs, errors.New(violation))
	}
	return errors.Join(errs...)
}

// jsonEscaped maps each value to its JSON string body, so the enforce.json reasons fill without breaking the document.
func jsonEscaped(values map[string]string) map[string]string {
	escaped := make(map[string]string, len(values))
	for name, value := range values {
		encoded, _ := json.Marshal(value)
		escaped[name] = string(encoded[1 : len(encoded)-1])
	}
	return escaped
}

// renderAnswer joins the payload bodies in document order, one blank line between them.
func renderAnswer(skill *Skill) []byte {
	bodies := make([]string, 0)
	for _, ref := range skill.Entries() {
		if ref.Entry.Role == RolePayload && ref.Entry.Payload != nil {
			bodies = append(bodies, strings.TrimRight(ref.Entry.Payload.Body, "\n")+"\n")
		}
	}
	return []byte(strings.Join(bodies, "\n"))
}

// renderDeclarations: enforce.json needs gate and heartbeat, title.json the session helpers and a set title-set variable (any under nil values), env.json the skill environment and a rendered placeholder, answer.md nothing.
func renderDeclarations(deferred *deferral, set map[components.ID]bool, values map[string]string, skill *Skill) ([]RenderedFile, error) {
	var files []RenderedFile
	isEnforced := set[components.Gate] && set[components.Heartbeat]
	if isEnforced && skill.Enforce != nil {
		enforce, err := renderEnforce(skill)
		if err != nil {
			return nil, fmt.Errorf("renderDeclarations: %w", err)
		}

		if values != nil {
			enforce = FillVariables(enforce, jsonEscaped(values))
		}
		enforceFile := RenderedFile{Content: enforce, Path: FileEnforce}
		files = append(files, enforceFile)
	}
	isTitleDeclared := set[components.SessionHelpers] && skill.Title != nil
	isTitleSettable := values == nil || titleSetIsSet(skill, values)
	if isTitleDeclared && isTitleSettable {
		title, err := renderTitle(skill)
		if err != nil {
			return nil, fmt.Errorf("renderDeclarations: %w", err)
		}

		titleFile := RenderedFile{Content: title, Path: sessiontitle.FileName}
		files = append(files, titleFile)
	}
	isEnvDeclared := set[components.SkillEnvironment] && deferred != nil
	if isEnvDeclared && len(deferred.emitted) > 0 {
		env, err := deferred.renderEnv()
		if err != nil {
			return nil, fmt.Errorf("renderDeclarations: %w", err)
		}

		envFile := RenderedFile{Content: env, Path: skillenv.FileName}
		files = append(files, envFile)
	}
	if skill.AnsweredBy != "" {
		answerFile := RenderedFile{Content: renderAnswer(skill), Path: FileAnswer}
		files = append(files, answerFile)
	}
	return files, nil
}

// ManifestLabel names the SKILL.md beside a model file, since findings carry the render's line numbers.
func ManifestLabel(path string) string {
	if !IsModelFile(path) {
		return path
	}
	return filepath.Join(filepath.Dir(path), FileManifest)
}

// ReadManifest renders a model file in memory; any other file is read as it is.
func ReadManifest(path string) ([]byte, error) {
	if !IsModelFile(path) {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("ReadManifest: %w", err)
		}
		return content, nil
	}

	skill, err := Load(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("ReadManifest: %w", err)
	}
	return RenderManifest(skill)
}

// Render refuses a model whose CUE encoding does not round-trip.
func Render(skill *Skill) ([]RenderedFile, error) {
	if err := checkModel(skill); err != nil {
		return nil, fmt.Errorf("Render: Skill %s: %w", skill.Name(), err)
	}

	if _, err := renderMarkdown(nil, false, nil, skill); err != nil {
		return nil, fmt.Errorf("Render: Skill %s: %w", skill.Name(), err)
	}

	encoded, err := EncodeSkill(skill)
	if err != nil {
		return nil, fmt.Errorf("Render: Skill %s: %w", skill.Name(), err)
	}

	if err := guardRoundTrip(encoded, skill); err != nil {
		return nil, fmt.Errorf("Render: Skill %s: %w", skill.Name(), err)
	}

	files := []RenderedFile{{Content: encoded, Path: ModelFile(skill.Name())}}
	for _, ref := range skill.Entries() {
		if ref.Entry.Payload == nil {
			continue
		}

		payload := RenderedFile{
			Content: []byte(ref.Entry.Payload.Body),
			Path:    payloadFile(ref.Entry.Id),
		}
		files = append(files, payload)
	}
	slices.SortFunc(files, func(a, b RenderedFile) int {
		return strings.Compare(a.Path, b.Path)
	})
	return files, nil
}

// RenderDeployed drops absent components' entries, rules, routes and edges; it never encodes CUE, since deploy ships no model file.
func RenderDeployed(opts RenderOptions, skill *Skill) ([]RenderedFile, error) {
	if err := checkModel(skill); err != nil {
		return nil, fmt.Errorf("RenderDeployed: Skill %s: %w", skill.Name(), err)
	}

	disabled, err := resolveDisable(skill, opts.Disable)
	if err != nil {
		return nil, fmt.Errorf("RenderDeployed: Skill %s: %w", skill.Name(), err)
	}

	set := componentSet(opts.Components)
	deferred := newDeferral(opts.DeferredHint, opts.Deferred, skill, opts.Values)
	maps.Copy(disabled, componentDisabled(set, skill))
	if opts.Values != nil {
		maps.Copy(disabled, variableDisabled(deferred, skill, opts.Values))
	}
	view := componentView(deferred, set, opts.Deployable, opts.Values, skill)
	markdown, err := renderMarkdown(deferred, true, disabled, view)
	if err != nil {
		return nil, fmt.Errorf("RenderDeployed: Skill %s: %w", skill.Name(), err)
	}
	if opts.Values != nil {
		markdown = FillVariables(markdown, opts.Values)
	}

	files := []RenderedFile{{Content: markdown, Path: FileManifest}}
	helpTest := renderHelpTest(opts.HasEval, skill)
	if len(helpTest) > 0 && set[components.SessionHelpers] {
		helpTestFile := RenderedFile{Content: helpTest, Path: FileHelpTest}
		files = append(files, helpTestFile)
	}

	declarations, err := renderDeclarations(deferred, set, opts.Values, view)
	if err != nil {
		return nil, fmt.Errorf("RenderDeployed: Skill %s: %w", skill.Name(), err)
	}
	return append(files, declarations...), nil
}

// RenderManifest's text is what the gates, the skill page and the import proof read; no skill dir holds it.
func RenderManifest(skill *Skill) ([]byte, error) {
	if err := checkModel(skill); err != nil {
		return nil, fmt.Errorf("RenderManifest: Skill %s: %w", skill.Name(), err)
	}

	markdown, err := renderMarkdown(nil, false, nil, skill)
	if err != nil {
		return nil, fmt.Errorf("RenderManifest: Skill %s: %w", skill.Name(), err)
	}
	return markdown, nil
}
