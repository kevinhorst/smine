package skillgraph

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/sessiontitle"
	"github.com/kevinhorst/smine/internal/skillenv"
)

// checkRenderedPaths runs before a single file is written.
func checkRenderedPaths(files []RenderedFile) error {
	for _, file := range files {
		if !filepath.IsLocal(file.Path) {
			return fmt.Errorf("checkRenderedPaths: Rendered path %q leaves the skill dir", file.Path)
		}
	}
	return nil
}

// hasContent lets an unchanged file keep its mtime.
func hasContent(data []byte, path string) bool {
	existing, err := os.ReadFile(path)
	return err == nil && bytes.Equal(existing, data)
}

// LeftoverRenders: no checkout holds generated files, so render -check reports each for deletion.
func LeftoverRenders(dir string) ([]string, error) {
	var leftovers []string
	for _, name := range []string{FileAnswer, FileEnforce, FileManifest, FileSmoke, sessiontitle.FileName, skillenv.FileName} {
		path := filepath.Join(dir, name)
		exists, err := fileExists(path)
		if err != nil {
			return nil, fmt.Errorf("LeftoverRenders: %w", err)
		}

		if exists {
			leftovers = append(leftovers, path)
		}
	}
	return leftovers, nil
}

func OrphanPayloads(dir string, files []RenderedFile) ([]string, error) {
	payloadDir := filepath.Join(dir, DirPayloads)
	listing, err := os.ReadDir(payloadDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("OrphanPayloads: Failed to read %s: %w", payloadDir, err)
	}

	rendered := map[string]bool{}
	for _, file := range files {
		rendered[filepath.Clean(file.Path)] = true
	}
	var orphans []string
	for _, item := range listing {
		name := item.Name()
		isPayloadFile := item.Type().IsRegular() && strings.HasSuffix(name, payloadExtension)
		isClaimed := rendered[filepath.Join(DirPayloads, name)]
		if !isPayloadFile || isClaimed {
			continue
		}

		orphans = append(orphans, filepath.Join(payloadDir, name))
	}
	return orphans, nil
}

// Save never writes the vocabulary or a generated file; those exist only where deploy renders them.
func Save(dir string, skill *Skill) error {
	files, err := Render(skill)
	if err != nil {
		return fmt.Errorf("Save: %s: %w", dir, err)
	}

	if err := checkRenderedPaths(files); err != nil {
		return fmt.Errorf("Save: %w", err)
	}

	for _, file := range files {
		path := filepath.Join(dir, file.Path)
		if hasContent(file.Content, path) {
			continue
		}

		if err := fsx.WriteFileAtomic(file.Content, path, 0o644); err != nil {
			return fmt.Errorf("Save: %w", err)
		}
	}

	orphans, err := OrphanPayloads(dir, files)
	if err != nil {
		return fmt.Errorf("Save: %w", err)
	}

	for _, orphan := range orphans {
		if err := os.Remove(orphan); err != nil {
			return fmt.Errorf("Save: Failed to delete orphaned payload %s: %w", orphan, err)
		}
	}
	return nil
}

// SaveVocabulary is the importer's writer; people edit vocabulary.cue as text.
func SaveVocabulary(path string, vocabulary *Vocabulary) error {
	encoded, err := encodeVocabulary(vocabulary)
	if err != nil {
		return fmt.Errorf("SaveVocabulary: %w", err)
	}

	if err := guardVocabularyRoundTrip(encoded, path); err != nil {
		return fmt.Errorf("SaveVocabulary: %w", err)
	}

	if hasContent(encoded, path) {
		return nil
	}

	if err := fsx.WriteFileAtomic(encoded, path, 0o644); err != nil {
		return fmt.Errorf("SaveVocabulary: %w", err)
	}
	return nil
}
