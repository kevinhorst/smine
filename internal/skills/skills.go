package skills

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/skillgraph"
)

const (
	OriginHome = "home"
	OriginRepo = "repo"

	// toolsGroupKey collects plain (non-Bash, non-MCP) tool names.
	toolsGroupKey = "tools"
)

// repoFields memoizes because loading a model takes tens of milliseconds and a scan reads every skill.
var repoFields = &fieldsCache{entries: map[string]*cachedFields{}}

// cachedFields is one memoized model read with the file state it was read at.
type cachedFields struct {
	fields  frontmatter
	modTime time.Time
	size    int64
}

// isCurrent reports whether the file state still matches the cached read.
func (c *cachedFields) isCurrent(info fs.FileInfo) bool {
	return c.size == info.Size() && c.modTime.Equal(info.ModTime())
}

// fieldsCache is the lock-guarded memo of repo skill fields by model file path.
type fieldsCache struct {
	mu      sync.Mutex
	entries map[string]*cachedFields
}

func (c *fieldsCache) get(modelPath string) frontmatter {
	info, err := os.Stat(modelPath)
	if err != nil {
		return frontmatter{}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	cached, isKnown := c.entries[modelPath]
	if isKnown && cached.isCurrent(info) {
		return cached.fields
	}

	fields := loadModelFields(filepath.Dir(modelPath))
	c.entries[modelPath] = &cachedFields{fields: fields, modTime: info.ModTime(), size: info.Size()}
	return fields
}

type foundSkill struct {
	dir   string
	group string
}

type frontmatter struct {
	Aliases            []string
	AllowedTools       string
	ArgumentHint       string
	Author             string
	Description        string
	Docker             string
	EnableDelegation   string
	EnforcePermissions string
	Name               string
	Requires           []components.ID // repo models only: the model's requires list
	TitleCode          string
	Version            string
}

type Skill struct {
	Aliases            []string   // frontmatter aliases, nil when absent
	AllowedTools       string     // frontmatter allowed-tools, empty when absent
	Args               []SkillArg // parsed from the description's "Args — name: doc; …" segment
	ArgumentHint       string     // frontmatter argument-hint, empty when absent
	Author             string     // frontmatter author, empty when absent
	Description        string
	Docker             string   // rendered docker line, empty when agnostic
	EnableDelegation   string   // rendered enable-delegation line, empty when agnostic
	EnforcePermissions string   // rendered enforce-permissions line, empty when agnostic
	Files              []string // sibling files, relative paths within the skill dir
	Group              string
	Name               string          // directory name
	Origin             string          // OriginRepo | OriginHome
	Path               string          // absolute skill dir
	Requires           []components.ID // repo skills: the components the model needs to deploy (RequiredComponents); nil for home skills
	Summary            string          // description up to the ". Trigger on" marker, full description without one
	Synced             bool            // a same-named skill at the same version exists in the other root
	Version            string          // frontmatter version, empty when absent
}

type SkillArg struct {
	Doc  string
	Name string
}

type ToolGroup struct {
	Key   string
	Tools []string
}

func AllowedTools(skillDir string) string {
	content, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		return ""
	}
	return parseFrontmatter(string(content)).AllowedTools
}

func Version(skillDir string) string {
	content, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		return ""
	}
	return parseFrontmatter(string(content)).Version
}

func DefaultHomePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "skills")
}

func ModelAllowedTools(skillDir string) string {
	return repoFields.get(skillgraph.ModelPath(skillDir)).AllowedTools
}

// Scan finds repo skills by their model file and home skills by SKILL.md.
func Scan(repoRoot, homeRoot string) ([]Skill, error) {
	repoSkills, err := scanRoot(repoRoot, OriginRepo)
	if err != nil {
		return nil, err
	}
	homeSkills, err := scanRoot(homeRoot, OriginHome)
	if err != nil {
		return nil, err
	}

	homeSkills = withoutAliasCopies(homeSkills, repoSkills)
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

func GroupTools(raw string) []ToolGroup {
	byKey := make(map[string][]string)
	for token := range strings.SplitSeq(raw, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}

		key := toolsGroupKey
		display := token
		if spec, ok := strings.CutPrefix(token, "Bash("); ok {
			spec = strings.TrimSuffix(strings.TrimSuffix(spec, ")"), "*")
			display = strings.TrimSpace(spec)
			key, _, _ = strings.Cut(display, " ")
		} else if rest, ok := strings.CutPrefix(token, "mcp__"); ok {
			server, tool, hasTool := strings.Cut(rest, "__")
			if hasTool {
				key = server
				display = tool
			}
		}
		byKey[key] = append(byKey[key], display)
	}

	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		if key != toolsGroupKey {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if _, ok := byKey[toolsGroupKey]; ok {
		keys = append([]string{toolsGroupKey}, keys...)
	}

	groups := make([]ToolGroup, 0, len(keys))
	for _, key := range keys {
		members := byKey[key]
		sort.Strings(members)
		group := ToolGroup{
			Key:   key,
			Tools: members,
		}
		groups = append(groups, group)
	}
	return groups
}

func Find(list []Skill, origin, name string) (*Skill, bool) {
	for i := range list {
		if list[i].Origin == origin && list[i].Name == name {
			return &list[i], true
		}
	}
	return nil, false
}

func TitleCode(skillMd string) string {
	return parseFrontmatter(skillMd).TitleCode
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
		if _, err := os.Stat(markerPath(entry, origin)); err == nil {
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
			if _, err := os.Stat(markerPath(subDir, origin)); err == nil {
				skillDirs = append(skillDirs, foundSkill{dir: subDir, group: dir.Name()})
			}
		}
	}

	var result []Skill
	for _, found := range skillDirs {
		skillDir := found.dir
		fields, isReadable := readFields(origin, skillDir)
		if !isReadable {
			continue
		}

		skill := Skill{
			Group:  found.group,
			Name:   filepath.Base(skillDir),
			Origin: origin,
			Path:   skillDir,
		}
		skill.Aliases = fields.Aliases
		skill.AllowedTools = fields.AllowedTools
		skill.ArgumentHint = fields.ArgumentHint
		skill.Author = fields.Author
		skill.Description = fields.Description
		skill.Docker = fields.Docker
		skill.EnableDelegation = fields.EnableDelegation
		skill.EnforcePermissions = fields.EnforcePermissions
		skill.Requires = fields.Requires
		skill.Version = fields.Version
		skill.Summary, skill.Args = splitDescription(fields.Description)

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

func aliasList(value string) []string {
	var aliases []string
	for alias := range strings.SplitSeq(value, ",") {
		if alias = strings.TrimSpace(alias); alias != "" {
			aliases = append(aliases, alias)
		}
	}
	return aliases
}

// loadModelFields parses the in-memory render so fields read as the SKILL.md states them.
func loadModelFields(skillDir string) frontmatter {
	model, err := skillgraph.Load(skillDir)
	if err != nil {
		return frontmatter{}
	}

	manifest, err := skillgraph.RenderManifest(model)
	if err != nil {
		return frontmatter{}
	}
	fields := parseFrontmatter(string(manifest))
	fields.Requires = model.RequiredComponents()
	return fields
}

// markerPath is the file that makes dir a skill: its model file in the repo, SKILL.md in a home.
func markerPath(dir, origin string) string {
	if origin == OriginRepo {
		return skillgraph.ModelPath(dir)
	}
	return filepath.Join(dir, skillgraph.FileManifest)
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
		if v, ok := strings.CutPrefix(line, "aliases:"); ok {
			fm.Aliases = aliasList(v)
		}
		if v, ok := strings.CutPrefix(line, "allowed-tools:"); ok {
			fm.AllowedTools = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(line, "argument-hint:"); ok {
			fm.ArgumentHint = strings.Trim(strings.TrimSpace(v), `"'`)
		}
		if v, ok := strings.CutPrefix(line, "author:"); ok {
			fm.Author = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(line, "description:"); ok {
			fm.Description = foldedValue(&i, lines, strings.TrimSpace(v))
		}
		if v, ok := strings.CutPrefix(line, "docker:"); ok {
			fm.Docker = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(line, "enable-delegation:"); ok {
			fm.EnableDelegation = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(line, "enforce-permissions:"); ok {
			fm.EnforcePermissions = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(line, "name:"); ok {
			fm.Name = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(line, "title-code:"); ok {
			fm.TitleCode = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(line, "version:"); ok {
			fm.Version = strings.TrimSpace(v)
		}
	}
	return fm
}

func readFields(origin, skillDir string) (frontmatter, bool) {
	if origin == OriginRepo {
		return repoFields.get(skillgraph.ModelPath(skillDir)), true
	}

	data, err := os.ReadFile(filepath.Join(skillDir, skillgraph.FileManifest))
	if err != nil {
		return frontmatter{}, false
	}
	return parseFrontmatter(string(data)), true
}

// splitDescription follows the convention "<sentence>. Trigger on <…>. Args — <n>: <doc>; …".
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

func withoutAliasCopies(homeSkills, repoSkills []Skill) []Skill {
	aliases := map[string]bool{}
	for _, skill := range repoSkills {
		for _, alias := range skill.Aliases {
			aliases[alias] = true
		}
	}

	kept := make([]Skill, 0, len(homeSkills))
	for _, skill := range homeSkills {
		if !aliases[skill.Name] {
			kept = append(kept, skill)
		}
	}
	return kept
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
