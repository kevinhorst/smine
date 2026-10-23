// Command skillhelp answers a `/<skill> --help` prompt with the skill's help, and any prompt of a hook-answered skill with its answer, and blocks the prompt.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/sessionstate"
	"github.com/kevinhorst/smine/internal/sessiontitle"
	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skills"
)

// HookInput is the UserPromptSubmit envelope; unread fields stay undeclared.
type HookInput struct {
	Cwd    string `json:"cwd"`
	Prompt string `json:"prompt"`
}

type HookOutput struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

// answer is the block reason for a prompt: the help of a `/<leaf> --help`, the answer of a hook-answered `/<leaf>`, empty otherwise.
func answer(dir string, helpPattern *regexp.Regexp, prompt string, section *hookconfig.Skillhelp) (string, error) {
	if match := helpPattern.FindStringSubmatch(prompt); match != nil {
		return renderSkill(dir, match[1])
	}

	match := invokePattern.FindStringSubmatch(prompt)
	if match == nil {
		return "", nil
	}
	return renderAnswer(dir, section.FeaturesLead, match[1])
}

// isHookAnswered reads the frontmatter between the first two fences for the answered-by line.
func isHookAnswered(skillMd string) bool {
	inFrontmatter := false
	for line := range strings.SplitSeq(skillMd, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			if inFrontmatter {
				return false
			}
			inFrontmatter = true
			continue
		}
		if inFrontmatter && trimmed == answeredByLine {
			return true
		}
	}
	return false
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("skillhelp", flag.ContinueOnError)
	flags.SetOutput(stderr)
	claudeHome := flags.String("claude-home", "", "Claude Code home directory (default ~/.claude)")
	skillsDir := flags.String("skills", "", "deployed skills directory (default <claude home>/skills)")
	if err := flags.Parse(args); err != nil {
		return usageExitCode
	}

	home, err := config.ClaudeHomeDir(*claudeHome)
	if err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return 1
	}

	cfg, err := hookconfig.Load(home)
	if err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return 1
	}

	helpPattern, err := regexp.Compile(cfg.Skillhelp.HelpPattern)
	if err != nil {
		fmt.Fprintf(stderr, "run: %s: skillhelp.help_pattern: %v\n", hookconfig.Path(home), err)
		return 1
	}

	input := &HookInput{}
	if err := json.NewDecoder(stdin).Decode(input); err != nil {
		fmt.Fprintf(stderr, "run: Failed to decode hook input: %v\n", err)
		return 1
	}

	dir := *skillsDir
	if dir == "" {
		dir = filepath.Join(home, "skills")
	}

	reason, err := answer(dir, helpPattern, input.Prompt, &cfg.Skillhelp)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if reason == "" {
		return 0
	}

	output := &HookOutput{
		Decision: decisionBlock,
		Reason:   reason,
	}
	if err := json.NewEncoder(stdout).Encode(output); err != nil {
		fmt.Fprintf(stderr, "run: Failed to encode decision: %v\n", err)
		return 1
	}
	return 0
}

// renderAnswer is answer.md plus the feature catalog when the skill's manifest carries the answered-by line; empty otherwise.
func renderAnswer(dir string, featuresLead string, name string) (string, error) {
	skillMd, err := os.ReadFile(filepath.Join(dir, name, skillgraph.FileManifest))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("renderAnswer: Failed to read SKILL.md for %s: %w", name, err)
	}
	if !isHookAnswered(string(skillMd)) {
		return "", nil
	}

	body, err := os.ReadFile(filepath.Join(dir, name, skillgraph.FileAnswer))
	if err != nil {
		return "", fmt.Errorf("renderAnswer: Failed to read %s for %s: %w", skillgraph.FileAnswer, name, err)
	}

	features := make([]string, 0)
	for _, entry := range sessionstate.Help() {
		features = append(features, fmt.Sprintf("- `%s` — %s (%s)", entry.Feature, entry.Does, entry.Hook))
	}
	lines := []string{strings.TrimRight(string(body), "\n"), "## Features", featuresLead, strings.Join(features, "\n\n")}
	return strings.Join(lines, "\n\n") + "\n", nil
}

func renderSkill(dir string, name string) (string, error) {
	skillMd, err := os.ReadFile(filepath.Join(dir, name, "SKILL.md"))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("renderSkill: Failed to read SKILL.md for %s: %w", name, err)
	}

	contextMap, err := os.ReadFile(filepath.Join(dir, name, "context-map.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("renderSkill: Failed to read context-map.json for %s: %w", name, err)
	}

	helpTest, err := os.ReadFile(filepath.Join(dir, name, skillgraph.FileHelpTest))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("renderSkill: Failed to read %s for %s: %w", skillgraph.FileHelpTest, name, err)
	}

	titleDocument, err := os.ReadFile(filepath.Join(dir, name, sessiontitle.FileName))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("renderSkill: Failed to read %s for %s: %w", sessiontitle.FileName, name, err)
	}

	help, err := skills.ParseHelp(string(contextMap), string(helpTest), string(skillMd), string(titleDocument))
	if err != nil {
		return "", fmt.Errorf("renderSkill: %s: %w", name, err)
	}
	return skills.RenderHelp(name, help), nil
}
