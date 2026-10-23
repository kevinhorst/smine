package sessiontitle

import (
	"fmt"
	"path/filepath"
)

// Declared is one skill-declared state as a page shows it.
type Declared struct {
	Meaning string
	Skill   string
}

// Vocabulary's zero value holds the built-in states alone.
type Vocabulary struct {
	declared map[State]*Declared
}

// LoadVocabulary credits a token two skills declare to the first skill in path order.
func LoadVocabulary(skillsDir string) (*Vocabulary, error) {
	paths, err := filepath.Glob(filepath.Join(skillsDir, "*", FileName))
	if err != nil {
		return nil, fmt.Errorf("LoadVocabulary: Invalid skills dir %s: %w", skillsDir, err)
	}

	vocabulary := &Vocabulary{declared: map[State]*Declared{}}
	for _, path := range paths {
		document, err := LoadDocument(path)
		if err != nil {
			return nil, fmt.Errorf("LoadVocabulary: %w", err)
		}

		vocabulary.add(document)
	}
	return vocabulary, nil
}

// Declared returns the declaration behind a skill-declared state.
func (v *Vocabulary) Declared(state State) (*Declared, bool) {
	declared, isDeclared := v.declared[state]
	return declared, isDeclared
}

// IsSettable reports whether `smine-rules title set` accepts the token: WIP, READY or a declared state.
func (v *Vocabulary) IsSettable(state State) bool {
	if state == StateWip || state == StateReady {
		return true
	}
	_, isDeclared := v.declared[state]
	return isDeclared
}

func (v *Vocabulary) add(document *Document) {
	for _, state := range document.Tokens() {
		if _, isKnown := v.declared[state]; isKnown || state.IsBuiltin() {
			continue
		}

		v.declared[state] = &Declared{Meaning: document.States[string(state)].Meaning, Skill: document.Skill}
	}
}

func (v *Vocabulary) state(token string) State {
	state := State(token)
	if state.IsBuiltin() {
		return state
	}
	if _, isDeclared := v.declared[state]; isDeclared {
		return state
	}
	return StateNone
}
