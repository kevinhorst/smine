package contextdocs

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/kevinhorst/smine/internal/reach"
)

// sourceTailSegments is the <dir>/<file>.md tail every source keeps under the target's context folder.
const sourceTailSegments = 2

// FilterIndex cuts the source index for one target: entries by reach and language binding, guides by language, sources under contextDir, no self stamp.
func FilterIndex(index *ContextFile, name string, langs []string, contextDir string) *ContextFile {
	selected := toSet(langs)
	scopeLang := map[string]string{}
	for _, aspect := range index.Aspects {
		if aspect.Class == AspectClassScope && aspect.Lang != "" {
			scopeLang[aspect.Name] = aspect.Lang
		}
	}
	allLangs := Langs(index.Aspects)

	filtered := &ContextFile{Aspects: index.Aspects}
	for _, entry := range index.Entries {
		if !reach.DeploysTo(entry.Reach, name) {
			continue
		}
		if lang, isBound := scopeLang[entry.Scope]; isBound && !selected[lang] {
			continue
		}
		entry.Source = rewriteSource(contextDir, entry.Source)
		filtered.Entries = append(filtered.Entries, entry)
	}
	for _, tombstone := range index.Tombstones {
		tombstone.Source = rewriteSource(contextDir, tombstone.Source)
		filtered.Tombstones = append(filtered.Tombstones, tombstone)
	}
	for _, guide := range index.Guides {
		if !isGuideSelected(guide.Name, allLangs, selected) {
			continue
		}
		guide.Source = rewriteSource(contextDir, guide.Source)
		filtered.Guides = append(filtered.Guides, guide)
	}
	return filtered
}

// Langs lists the languages the taxonomy binds, in aspect order, each once.
func Langs(aspects []RuleAspect) []string {
	seen := map[string]bool{}
	var langs []string
	for _, aspect := range aspects {
		isLangScope := aspect.Class == AspectClassScope && aspect.Lang != ""
		if !isLangScope || seen[aspect.Lang] {
			continue
		}
		seen[aspect.Lang] = true
		langs = append(langs, aspect.Lang)
	}
	return langs
}

// MergeRepoOwned appends the target's own entries — files under its context folder without the synced header — validated against the taxonomy; a baseline copy or an id the index holds is skipped.
func MergeRepoOwned(index *ContextFile, contextRoot string, sourceRoot string) error {
	if _, err := os.Stat(contextRoot); err != nil {
		return nil
	}
	parsed, err := ParseContext(contextRoot, true)
	if err != nil {
		return fmt.Errorf("MergeRepoOwned: %w", err)
	}

	known := map[string]bool{}
	for _, entry := range index.Entries {
		known[entry.Id] = true
	}
	var own RuleSet
	for _, entry := range parsed.Entries {
		isOwn := entry.Origin == RuleOriginOverlay && !known[entry.Id]
		if !isOwn {
			continue
		}
		own.Entries = append(own.Entries, entry)
	}
	violations := ValidateRules(own, index.Aspects, sourceRoot)
	if len(violations) > 0 {
		return fmt.Errorf("MergeRepoOwned: %d violation(s) in %s:\n%s", len(violations), contextRoot, strings.Join(violations, "\n"))
	}
	index.Entries = append(index.Entries, own.Entries...)
	return nil
}

// A guide ships with its language, a part guide (go_test) with its parent; a guide no language aspect binds ships always.
func isGuideSelected(guideName string, allLangs []string, selected map[string]bool) bool {
	for _, lang := range allLangs {
		isOwn := guideName == lang || strings.HasPrefix(guideName, lang+"_")
		if isOwn {
			return selected[lang]
		}
	}
	return true
}

// Readers match a source by its last two segments; the prefix differs per producer and never leaves the machine.
func rewriteSource(contextDir string, source string) string {
	segments := strings.Split(filepath.ToSlash(source), "/")
	if len(segments) < sourceTailSegments {
		return path.Join(contextDir, source)
	}
	tail := segments[len(segments)-sourceTailSegments:]
	return path.Join(append([]string{contextDir}, tail...)...)
}

func toSet(values []string) map[string]bool {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	return set
}
