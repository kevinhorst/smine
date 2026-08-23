package skills

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	OriginHome = "home"
	OriginRepo = "repo"
)

type ChangelogEntry struct {
	Date    string `json:"date"`
	Text    string `json:"text"`
	Version string `json:"version"`
}

type foundSkill struct {
	dir   string
	group string
}

type frontmatter struct {
	AllowedTools string
	Author       string
	Description  string
	Name         string
	Version      string
}

type Skill struct {
	AllowedTools string     // frontmatter allowed-tools, empty when absent
	Args         []SkillArg // parsed from the description's "Args — name: doc; …" segment
	Author       string     // frontmatter author, empty when absent
	Description  string
	Files        []string // sibling files, relative paths within the skill dir
	Group        string
	Name         string // directory name
	Origin       string // OriginRepo | OriginHome
	Path         string // absolute skill dir
	Summary      string // description up to the ". Trigger on" marker, full description without one
	Synced       bool   // a same-named skill at the same version exists in the other root
	Version      string // frontmatter version, empty when absent
}

type SkillArg struct {
	Doc  string
	Name string
}

func DefaultHomePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "skills")
}

// Scan collects every */SKILL.md under both roots. Frontmatter is parsed
// line-wise between the --- fences for single-line name: and description:
// values.
func Scan(repoRoot, homeRoot string) ([]Skill, error) {
	repoSkills, err := scanRoot(repoRoot, OriginRepo)
	if err != nil {
		return nil, err
	}
	homeSkills, err := scanRoot(homeRoot, OriginHome)
	if err != nil {
		return nil, err
	}

	repoVersions := make(map[string]string, len(repoSkills))
	for _, s := range repoSkills {
		repoVersions[s.Name] = s.Version
	}
	homeVersions := make(map[string]string, len(homeSkills))
	for _, s := range homeSkills {
		homeVersions[s.Name] = s.Version
	}
	for i := range homeSkills {
		version, ok := repoVersions[homeSkills[i].Name]
		homeSkills[i].Synced = ok && version == homeSkills[i].Version
	}
	for i := range repoSkills {
		version, ok := homeVersions[repoSkills[i].Name]
		repoSkills[i].Synced = ok && version == repoSkills[i].Version
	}

	all := append(repoSkills, homeSkills...)
	sort.Slice(all, func(i, j int) bool {
		if all[i].Name != all[j].Name {
			return all[i].Name < all[j].Name
		}
		return all[i].Origin < all[j].Origin
	})
	return all, nil
}

func LoadChangelog(skillDir string) ([]ChangelogEntry, error) {
	path := filepath.Join(skillDir, "changelog.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("LoadChangelog: Failed to read %s: %w", path, err)
	}

	var entries []ChangelogEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("LoadChangelog: Failed to parse %s: %w", path, err)
	}
	return entries, nil
}

func Find(list []Skill, origin, name string) (*Skill, bool) {
	for i := range list {
		if list[i].Origin == origin && list[i].Name == name {
			return &list[i], true
		}
	}
	return nil, false
}

func scanRoot(root, origin string) ([]Skill, error) {
	dirs, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("scanRoot: Failed to read %s: %w", root, err)
	}

	var skillDirs []foundSkill
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		entry := filepath.Join(root, dir.Name())
		if _, err := os.Stat(filepath.Join(entry, "SKILL.md")); err == nil {
			skillDirs = append(skillDirs, foundSkill{dir: entry})
			continue
		}
		subs, err := os.ReadDir(entry)
		if err != nil {
			return nil, fmt.Errorf("scanRoot: Failed to read %s: %w", entry, err)
		}
		for _, sub := range subs {
			if !sub.IsDir() {
				continue
			}
			subDir := filepath.Join(entry, sub.Name())
			if _, err := os.Stat(filepath.Join(subDir, "SKILL.md")); err == nil {
				skillDirs = append(skillDirs, foundSkill{dir: subDir, group: dir.Name()})
			}
		}
	}

	var result []Skill
	for _, found := range skillDirs {
		skillDir := found.dir
		manifest := filepath.Join(skillDir, "SKILL.md")
		data, err := os.ReadFile(manifest)
		if err != nil {
			continue
		}

		skill := Skill{
			Group:  found.group,
			Name:   filepath.Base(skillDir),
			Origin: origin,
			Path:   skillDir,
		}
		fm := parseFrontmatter(string(data))
		skill.AllowedTools = fm.AllowedTools
		skill.Author = fm.Author
		skill.Description = fm.Description
		skill.Version = fm.Version
		skill.Summary, skill.Args = splitDescription(fm.Description)

		err = filepath.WalkDir(skillDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, err := filepath.Rel(skillDir, path)
			if err != nil || rel == "SKILL.md" {
				return err
			}
			skill.Files = append(skill.Files, rel)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("scanRoot: Failed to walk %s: %w", skillDir, err)
		}
		sort.Strings(skill.Files)
		result = append(result, skill)
	}
	return result, nil
}

func parseFrontmatter(content string) frontmatter {
	var fm frontmatter
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fm
	}
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "---" {
			break
		}
		if v, ok := strings.CutPrefix(line, "allowed-tools:"); ok {
			fm.AllowedTools = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(line, "author:"); ok {
			fm.Author = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(line, "description:"); ok {
			fm.Description = foldedValue(&i, lines, strings.TrimSpace(v))
		}
		if v, ok := strings.CutPrefix(line, "name:"); ok {
			fm.Name = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(line, "version:"); ok {
			fm.Version = strings.TrimSpace(v)
		}
	}
	return fm
}

// splitDescription derives the compact display fields from the repo
// description convention: "<sentence>. Trigger on <…>. Args — <n>: <doc>; …".
func splitDescription(description string) (string, []SkillArg) {
	summary := description
	if i := strings.Index(description, ". Trigger on "); i >= 0 {
		summary = description[:i+1]
	}

	var args []SkillArg
	if i := strings.Index(description, "Args — "); i >= 0 {
		segment := strings.TrimSuffix(strings.TrimSpace(description[i+len("Args — "):]), ".")
		for _, part := range strings.Split(segment, ";") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			name, doc, _ := strings.Cut(part, ": ")
			arg := SkillArg{
				Doc:  doc,
				Name: name,
			}
			args = append(args, arg)
		}
	}
	return summary, args
}

func foldedValue(index *int, lines []string, value string) string {
	if value != ">" && value != "|" {
		return value
	}
	var parts []string
	for *index+1 < len(lines) {
		next := lines[*index+1]
		trimmed := strings.TrimSpace(next)
		if trimmed == "---" {
			break
		}
		if trimmed != "" && !isIndented(next) {
			break
		}
		*index++
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, " ")
}

func isIndented(line string) bool {
	return strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
}
