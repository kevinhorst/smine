package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kevinhorst/smine/internal/acdsl"
	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skillimport"
	"github.com/kevinhorst/smine/internal/skills"
	"github.com/kevinhorst/smine/internal/tokencount"
)

const (
	frontmatterFence     = "---\n"
	legacyAtomsFile      = "atoms.json"
	legacyChangelogFile  = "changelog.json"
	legacyVocabularyFile = "vocabulary.json"

	tokenCacheDir = ".claude/context-report"
)

type exportedSkill struct {
	AllAtoms map[string]*skillgraph.Atom `json:"allAtoms"`
	*skillgraph.Skill
}

func checkRendered(dir string) []string {
	skill, err := skillgraph.Load(dir)
	if err != nil {
		return []string{fmt.Sprintf("%s: invalid model: %v", dir, err)}
	}

	files, err := skillgraph.Render(skill)
	if err != nil {
		return []string{fmt.Sprintf("%s: render failed: %v", dir, err)}
	}

	var drift []string
	for _, file := range files {
		path := filepath.Join(dir, file.Path)
		onDisk, err := os.ReadFile(path)
		if err != nil {
			drift = append(drift, fmt.Sprintf("%s: missing — run: make skill-render SKILL=%s", path, filepath.Base(dir)))
			continue
		}
		if !bytes.Equal(onDisk, file.Content) {
			drift = append(drift, fmt.Sprintf("%s: differs from its canonical form — run: make skill-render SKILL=%s", path, filepath.Base(dir)))
		}
	}
	orphans, err := skillgraph.OrphanPayloads(dir, files)
	if err != nil {
		drift = append(drift, fmt.Sprintf("%s: %v", dir, err))
	}
	for _, orphan := range orphans {
		drift = append(drift, orphan+": orphaned payload — no payload entry owns it")
	}
	leftovers, err := skillgraph.LeftoverRenders(dir)
	if err != nil {
		drift = append(drift, fmt.Sprintf("%s: %v", dir, err))
	}
	for _, leftover := range leftovers {
		drift = append(drift, leftover+": generated file in a skill dir — delete it, deploy renders it into the skill homes")
	}
	for _, legacy := range []string{legacyAtomsFile, legacyChangelogFile} {
		if hasFile(filepath.Join(dir, legacy)) {
			drift = append(drift, filepath.Join(dir, legacy)+": second source next to the model file — delete it")
		}
	}
	if hasFile(filepath.Join(dir, skillgraph.LegacyModelFile)) {
		drift = append(drift, filepath.Join(dir, skillgraph.LegacyModelFile)+": legacy model file — run: git mv "+filepath.Join(dir, skillgraph.LegacyModelFile)+" "+skillgraph.ModelPath(dir))
	}
	return drift
}

func countSkillTokens(ctx context.Context, counter *tokencount.Counter, skill *skillgraph.Skill) error {
	var canonical skillgraph.RenderOptions
	files, err := skillgraph.RenderDeployed(canonical, skill)
	if err != nil {
		return fmt.Errorf("countSkillTokens: %w", err)
	}

	body, ok := deployedBody(files)
	if !ok {
		return errors.New("countSkillTokens: No deployed SKILL.md rendered")
	}
	tokens, err := counter.CountText(ctx, tokencount.SkillCountModel, body)
	if err != nil {
		return fmt.Errorf("countSkillTokens: %w", err)
	}
	skill.Changelog[0].Tokens = tokens
	return nil
}

// deployedBody is the rendered SKILL.md after its closing frontmatter fence.
func deployedBody(files []skillgraph.RenderedFile) (string, bool) {
	for _, file := range files {
		if file.Path != skillgraph.FileManifest {
			continue
		}
		content := string(file.Content)
		if !strings.HasPrefix(content, frontmatterFence) {
			return content, true
		}
		_, body, found := strings.Cut(content[len(frontmatterFence):], "\n"+frontmatterFence)
		if !found {
			return content, true
		}
		return strings.TrimLeft(body, "\n"), true
	}
	return "", false
}

// hasFile reports whether path exists as a regular file.
func hasFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func importPath(path string) error {
	if filepath.Base(path) == legacyVocabularyFile {
		vocab, err := skillimport.ImportVocabulary(path)
		if err != nil {
			return err
		}
		return skillgraph.SaveVocabulary(filepath.Join(filepath.Dir(path), skillgraph.FileVocabulary), vocab)
	}

	skill, err := skillimport.Import(path)
	if err != nil {
		return err
	}

	skill.Vocabulary, err = skillgraph.FindVocabulary(path)
	if err != nil {
		return err
	}

	manifest := filepath.Join(path, skillgraph.FileManifest)
	old, err := os.ReadFile(manifest)
	if err != nil {
		return fmt.Errorf("importPath: %w", err)
	}

	if err := skillimport.Prove(acdsl.StripProjection(manifest, old), skill); err != nil {
		return err
	}
	return skillgraph.Save(path, skill)
}

func newTokenCounter() (*tokencount.Counter, error) {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("newTokenCounter: %w", err)
	}

	//TODO: Fucking struct initialization
	options := &tokencount.Options{CacheDir: filepath.Join(userHomeDir, tokenCacheDir), Mode: tokencount.ModeKeychain}

	tokenCounter, err := tokencount.New(options)
	if err != nil {
		return nil, fmt.Errorf("newTokenCounter: %w", err)
	}

	return tokenCounter, nil
}

func renderRoots(args []string) ([]string, error) {
	seen := map[string]bool{}
	var roots []string
	for _, arg := range args {
		//TODO: shit logic
		root := arg
		if !hasFile(filepath.Join(arg, skillgraph.FileVocabulary)) {
			found, err := skillgraph.FindRoot(arg)
			if err != nil {
				return nil, err
			}

			root = found
		}

		cleaned := filepath.Clean(root)
		if root == "" || seen[cleaned] {
			continue
		}

		seen[cleaned] = true
		roots = append(roots, cleaned)
	}
	return roots, nil
}

func renderTargets(args []string) ([]string, []string, error) {
	//TODO: what dirs?
	var dirs, unmodeled []string

	//TODO: switch inside for loop, super hard to follow
	for _, arg := range args {
		switch {
		case hasFile(skillgraph.ModelPath(arg)):
			dirs = append(dirs, arg)
		case hasFile(filepath.Join(arg, skillgraph.FileManifest)):
			unmodeled = append(unmodeled, arg)
		default:
			found, err := skillgraph.Discover(arg)
			if err != nil {
				return nil, nil, err
			}
			dirs = append(dirs, found...)
		}
	}
	return dirs, unmodeled, nil
}

func runExport(args []string) int {
	flags := flag.NewFlagSet("export", flag.ExitOnError)
	flags.Parse(args)
	if flags.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: skillmodel export <skill dir>")
		return exitError
	}
	skill, err := skillgraph.Load(flags.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillmodel:", err)
		return exitError
	}

	exported := &exportedSkill{AllAtoms: skill.AllAtoms(), Skill: skill}
	data, err := json.MarshalIndent(exported, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillmodel:", err)
		return exitError
	}

	fmt.Println(string(data))
	return exitClean
}

func runImport(args []string) int {
	flags := flag.NewFlagSet("import", flag.ExitOnError)
	flags.Parse(args)
	if flags.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: skillmodel import <skill dir | vocabulary.json>…")
		return exitError
	}
	for _, path := range flags.Args() {
		if err := importPath(path); err != nil {
			fmt.Fprintln(os.Stderr, "skillmodel:", err)
			return exitError
		}
		fmt.Printf("skillmodel: imported %s\n", path)
	}
	return exitClean
}

func runRender(args []string) int {
	flags := flag.NewFlagSet("render", flag.ExitOnError)
	isCheck := flags.Bool("check", false, "compare the rendering with disk instead of writing; exit 1 on drift")
	flags.Parse(args)
	if flags.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: skillmodel render [-check] <skill dir | skills root>…")
		return exitError
	}
	dirs, unmodeled, err := renderTargets(flags.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillmodel:", err)
		return exitError
	}
	var counter *tokencount.Counter
	if !*isCheck {
		counter, err = newTokenCounter()
		if err != nil {
			fmt.Fprintln(os.Stderr, "skillmodel:", err)
			return exitError
		}
	}

	var drift []string
	for _, dir := range unmodeled {
		drift = append(drift, fmt.Sprintf("%s: SKILL.md without a model file — run: make skill-import DIR=%s", dir, dir))
	}
	for _, dir := range dirs {
		if *isCheck {
			drift = append(drift, checkRendered(dir)...)
			continue
		}
		if err := saveRendered(counter, dir); err != nil {
			drift = append(drift, fmt.Sprintf("%s: %v", dir, err))
		}
	}
	roots, err := renderRoots(flags.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillmodel:", err)
		return exitError
	}

	for _, root := range roots {
		drift = append(drift, rootDrift(root)...)
		hits, err := skills.CheckBundle(filepath.Dir(root))
		if err != nil {
			drift = append(drift, fmt.Sprintf("%s: bundle check: %v", root, err))
			continue
		}
		for _, hit := range hits {
			drift = append(drift, "bundle: "+hit)
		}
	}
	for _, line := range drift {
		fmt.Fprintln(os.Stderr, line)
	}
	if len(drift) > 0 {
		return exitViolations
	}

	verb := "rendered"
	if *isCheck {
		verb = "current"
	}
	fmt.Printf("skillmodel: %d skill(s) %s\n", len(dirs), verb)
	return exitClean
}

func rootDrift(root string) []string {
	skills, problems := loadRootSkills(root)
	if len(problems) > 0 {
		return problems
	}

	drift := append(skillgraph.CheckPipelines(skills), skillgraph.CheckSkillGrants(skills)...)
	drift = append(drift, skillgraph.UnreadAtoms(skills)...)
	drift = append(drift, skillgraph.UnresolvedCrossRefs(skills)...)
	return append(drift, skillgraph.MissingSmokes(skills)...)
}

func saveRendered(counter *tokencount.Counter, dir string) error {
	skill, err := skillgraph.Load(dir)
	if err != nil {
		return err
	}
	if err := countSkillTokens(context.Background(), counter, skill); err != nil {
		fmt.Fprintf(os.Stderr, "skillmodel: %s: tokens not counted: %v\n", dir, err)
	}
	return skillgraph.Save(dir, skill)
}

func loadRootSkills(root string) ([]*skillgraph.Skill, []string) {
	dirs, err := skillgraph.Discover(root)
	if err != nil {
		return nil, []string{fmt.Sprintf("%s: %v", root, err)}
	}

	skills := make([]*skillgraph.Skill, 0, len(dirs))
	for _, dir := range dirs {
		skill, err := skillgraph.Load(dir)
		if err != nil {
			return nil, []string{fmt.Sprintf("%s: not checked — %s does not load", root, dir)}
		}
		skills = append(skills, skill)
	}
	return skills, nil
}
